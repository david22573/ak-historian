package prospective

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/david22573/ak-historian/internal/archiveauthority"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (function doerFunc) Do(request *http.Request) (*http.Response, error) { return function(request) }

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func loadAuthorities(t *testing.T) (Protocol, AvailabilityPolicy) {
	t.Helper()
	root := repositoryRoot(t)
	var protocol Protocol
	if err := ReadStrict(filepath.Join(root, "authority", "pr4b0_r1p4_collection_protocol.json"), &protocol); err != nil {
		t.Fatal(err)
	}
	if err := VerifyProtocol(protocol); err != nil {
		t.Fatal(err)
	}
	var policy AvailabilityPolicy
	if err := ReadStrict(filepath.Join(root, "authority", "pr4b0_r1p4_availability_policy.json"), &policy); err != nil {
		t.Fatal(err)
	}
	if err := VerifyAvailabilityPolicy(policy); err != nil {
		t.Fatal(err)
	}
	var supervisor SupervisorContract
	if err := ReadStrict(filepath.Join(root, "authority", "pr4b0_r1p4_supervisor_contract.json"), &supervisor); err != nil {
		t.Fatal(err)
	}
	if err := VerifyCanonicalHash(supervisor, "contract_hash", supervisor.ContractHash); supervisor.SchemaVersion != SupervisorContractVersion || err != nil {
		t.Fatalf("supervisor contract identity failed: %v", err)
	}
	return protocol, policy
}

func testActivation(t *testing.T, protocol Protocol, policy AvailabilityPolicy) Activation {
	t.Helper()
	activation := Activation{SchemaVersion: ActivationVersion, DatasetID: protocol.DatasetID, Generation: "r1-activated-20300101T000000Z", ActivationTimestamp: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), CollectorSourceCommit: strings.Repeat("a", 40), CollectorBuildID: "sha256:" + strings.Repeat("b", 64), ProtocolHash: protocol.ProtocolHash, SourceSchemaVersion: SourceSchemaVersion, SourceSchemaFingerprint: SourceSchemaFingerprint, AvailabilityPolicyVersion: AvailabilityPolicyVersion, AvailabilityPolicyHash: policy.PolicyHash, CoveragePolicyVersion: CoveragePolicyVersion, IngestionReceiptVersion: ReceiptSchemaVersion, IngestionReceiptHash: ReceiptSchemaHash, ManifestContractVersion: ManifestContractVersion, ManifestContractHash: ManifestContractHash, UniqueSymbols: append([]string{}, UniqueSymbols...), Timeframe: "1m", CadenceSeconds: 300, PartitionPolicy: "UTC day", ReceiptLedgerGenesisHash: ZeroHash, CheckpointRule: "immutable"}
	var err error
	activation.ActivationHash, err = HashCanonical(activation, "activation_hash")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyActivation(activation, protocol, policy); err != nil {
		t.Fatal(err)
	}
	return activation
}

func TestFrozenAuthorityAndProtocolOrdering(t *testing.T) {
	protocol, policy := loadAuthorities(t)
	activation := testActivation(t, protocol, policy)
	changed := protocol
	changed.CadenceSeconds = 301
	if VerifyProtocol(changed) == nil || VerifyActivation(activation, changed, policy) == nil {
		t.Fatal("changed protocol retained authority")
	}
	previous := CollectorSourceCommit
	t.Cleanup(func() { CollectorSourceCommit = previous })
	CollectorSourceCommit = "UNSET"
	config := Config{Protocol: protocol, Policy: policy, Activation: activation, DataRoot: t.TempDir()}
	if _, err := NewCollector(config); err == nil {
		t.Fatal("real collector ran without committed source identity")
	}
	CollectorSourceCommit = strings.Repeat("c", 40)
	if _, err := NewCollector(config); err == nil {
		t.Fatal("collector accepted pre-source-commit or wrong source identity")
	}
	CollectorSourceCommit = activation.CollectorSourceCommit
	if _, err := NewCollector(config); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPSourceSecurityAndBoundedRetry(t *testing.T) {
	for _, rawURL := range []string{"http://fapi.binance.com/fapi/v1/time", "https://user:pass@fapi.binance.com/fapi/v1/time", "https://example.com/fapi/v1/time"} {
		if validatePublicURL(rawURL) == nil {
			t.Fatalf("unsafe URL accepted: %s", rawURL)
		}
	}
	attempts := 0
	now := time.Date(2030, 1, 1, 0, 0, 2, 0, time.UTC)
	client := &Client{HTTP: doerFunc(func(request *http.Request) (*http.Response, error) {
		attempts++
		return response(http.StatusTooManyRequests, `{"code":-1003}`, now), nil
	}), MaxAttempts: 3, BaseBackoff: 0, Now: func() time.Time { return now }, Sleep: func(context.Context, time.Duration) error { return nil }}
	if _, err := client.get(context.Background(), TimeEndpoint); err == nil || attempts != 3 {
		t.Fatalf("429 retry was not bounded: attempts=%d err=%v", attempts, err)
	}
	redirectClient := NewClient()
	request, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if redirectClient.HTTP.(*http.Client).CheckRedirect(request, nil) == nil {
		t.Fatal("redirect was accepted")
	}
}

func TestClosedSchemaCompletedCandleFiltering(t *testing.T) {
	provider := time.UnixMilli(179999).UTC()
	body := klineBody(60000, 120000)
	records, err := ParseKlines(body, "BTCUSDT", provider)
	if err != nil || len(records) != 1 || records[0].OpenTimeMS != 60000 {
		t.Fatalf("completed filtering failed: %+v %v", records, err)
	}
	var rows [][]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&rows); err != nil {
		t.Fatal(err)
	}
	rows[0] = append(rows[0], "unknown")
	malformed, _ := json.Marshal(rows)
	if _, err := ParseKlines(malformed, "BTCUSDT", provider); err == nil {
		t.Fatal("unknown field shape passed")
	}
	rows[0] = rows[0][:12]
	rows[0][1] = map[string]any{"bad": true}
	malformed, _ = json.Marshal(rows)
	if _, err := ParseKlines(malformed, "BTCUSDT", provider); err == nil {
		t.Fatal("malformed record passed")
	}
}

func TestAvailabilityMaximumCanonicalIdentityAndFailClosed(t *testing.T) {
	a := time.Date(2030, 1, 1, 0, 0, 1, 0, time.UTC)
	b := a.Add(time.Second)
	c := b.Add(time.Second)
	if maxTime(a, c, b) != c {
		t.Fatal("availability maximum not selected")
	}
	left := map[string]any{"b": 2, "a": 1}
	right := map[string]any{"a": 1, "b": 2}
	lh, _ := HashCanonical(left, "none")
	rh, _ := HashCanonical(right, "none")
	if lh != rh {
		t.Fatal("canonical field reordering changed identity")
	}
	protocol, policy := loadAuthorities(t)
	activation := testActivation(t, protocol, policy)
	receipt := minimalReceipt(t, activation, a, b, c)
	if err := VerifyReceipt(receipt, activation, ZeroHash, ZeroHash, 1); err != nil {
		t.Fatal(err)
	}
	missingProvider := receipt
	missingProvider.ProviderServerTimeUTC = time.Time{}
	if VerifyReceipt(missingProvider, activation, ZeroHash, ZeroHash, 1) == nil {
		t.Fatal("missing provider time passed")
	}
	missingClock := receipt
	missingClock.ClockEvidence.Synchronized = false
	if VerifyReceipt(missingClock, activation, ZeroHash, ZeroHash, 1) == nil {
		t.Fatal("missing local clock evidence passed")
	}
	mutated := receipt
	mutated.ResponseBodyByteLength++
	if VerifyReceipt(mutated, activation, ZeroHash, ZeroHash, 1) == nil {
		t.Fatal("receipt mutation retained chain identity")
	}
}

func TestCollectionStateRestartDurabilityFullUniverseAndVerification(t *testing.T) {
	protocol, policy := loadAuthorities(t)
	activation := testActivation(t, protocol, policy)
	root := t.TempDir()
	config := Config{Protocol: protocol, Policy: policy, Activation: activation, DataRoot: root}
	previous := CollectorSourceCommit
	CollectorSourceCommit = activation.CollectorSourceCommit
	t.Cleanup(func() { CollectorSourceCommit = previous })
	collector, err := NewCollector(config)
	if err != nil {
		t.Fatal(err)
	}
	provider := time.Date(2030, 1, 1, 0, 3, 0, 0, time.UTC)
	local := provider.Add(time.Second)
	collector.Now = func() time.Time { return local }
	collector.Client = fixtureClient(provider, "")
	collector.Clock = StaticClockChecker{Evidence: syncedClock(local)}
	cycle, err := collector.CollectOnce(context.Background())
	if err != nil || !cycle.FullUniverseSuccess || len(cycle.Symbols) != 9 {
		t.Fatalf("full cycle failed: %+v %v", cycle, err)
	}
	state, receipts, err := collector.Store.RebuildState()
	if err != nil || len(receipts) != 9 || state.NextRegistration != 10 || len(state.Cursors) != 9 {
		t.Fatalf("durable state invalid: %+v receipts=%d err=%v", state, len(receipts), err)
	}
	first := receipts[0]
	rawPath, _ := collector.Store.absolute(first.RawRelativePath)
	fragmentPath, _ := collector.Store.absolute(first.FragmentRelativePath)
	raw, _ := os.ReadFile(rawPath)
	fragment, _ := os.ReadFile(fragmentPath)
	receiptRelative := collector.Store.relative("receipts", "date="+first.FirstCandleOpenTimeUTC.Format("2006-01-02"), "cycle="+first.CycleID, "symbol="+first.Symbol+".json")
	if added, err := collector.Store.CommitReceipt(first, raw, fragment, first.RawRelativePath, first.FragmentRelativePath, receiptRelative); err != nil || added {
		t.Fatalf("identical duplicate was not idempotent: added=%v err=%v", added, err)
	}
	conflict := first
	conflict.ParsedRecordCount++
	if _, err := collector.Store.CommitReceipt(conflict, raw, fragment, first.RawRelativePath, first.FragmentRelativePath, receiptRelative); err == nil {
		t.Fatal("conflicting immutable receipt did not fail closed")
	}
	if err := os.Remove(collector.Store.StatePath()); err != nil {
		t.Fatal(err)
	}
	restarted, rebuilt, err := collector.Store.RebuildState()
	if err != nil || !reflect.DeepEqual(state.Cursors, restarted.Cursors) || len(rebuilt) != 9 {
		t.Fatalf("restart did not recover ledger state: %v", err)
	}
	summary, err := collector.Store.VerifyAll(local)
	if err != nil || !summary.Valid || summary.ReceiptCount != 9 || summary.RawResponseCount != 9 || summary.FragmentCount != 9 {
		t.Fatalf("verification failed: %+v %v", summary, err)
	}
	if err := os.Chmod(rawPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rawPath, []byte("altered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Store.VerifyAll(local); err == nil {
		t.Fatal("altered raw response bytes passed verification")
	}
}

func TestOneSymbolFailureWithholdsFullCycleAndLockRejectsConcurrency(t *testing.T) {
	protocol, policy := loadAuthorities(t)
	activation := testActivation(t, protocol, policy)
	previous := CollectorSourceCommit
	CollectorSourceCommit = activation.CollectorSourceCommit
	t.Cleanup(func() { CollectorSourceCommit = previous })
	collector, _ := NewCollector(Config{Protocol: protocol, Policy: policy, Activation: activation, DataRoot: t.TempDir()})
	provider := time.Date(2030, 1, 1, 0, 3, 0, 0, time.UTC)
	collector.Now = func() time.Time { return provider.Add(time.Second) }
	collector.Client = fixtureClient(provider, "SOLUSDT")
	collector.Clock = StaticClockChecker{Evidence: syncedClock(provider)}
	cycle, err := collector.CollectOnce(context.Background())
	if err == nil || cycle.FullUniverseSuccess {
		t.Fatal("symbol failure produced a full-cycle success claim")
	}
	incomplete := filepath.Join(collector.Store.Root, "quarantine", "availability-incomplete", "cycle="+cycle.CycleID, "symbol=SOLUSDT.json")
	retained, readErr := os.ReadFile(incomplete)
	if readErr != nil || !strings.Contains(string(retained), `"status":"AVAILABILITY_EVIDENCE_INCOMPLETE"`) {
		t.Fatalf("fetched ineligible bytes were not classified and retained: %v", readErr)
	}
	lock, err := AcquireLock(collector.Store.LockPath())
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, err := AcquireLock(collector.Store.LockPath()); err == nil {
		t.Fatal("concurrent collector lock was accepted")
	}
}

func TestPartitionsManifestPathsMutationAndOpenDay(t *testing.T) {
	protocol, policy := loadAuthorities(t)
	activation := testActivation(t, protocol, policy)
	day := "2030-01-01"
	start := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	partition := &accumulatedPartition{records: map[int64]NormalizedCandle{}, fragments: []string{"sha256:" + strings.Repeat("1", 64)}, receipts: []string{"sha256:" + strings.Repeat("2", 64)}, availability: []time.Time{start.Add(time.Minute)}}
	for index := 0; index < 2; index++ {
		opened := start.Add(time.Duration(index) * time.Minute)
		partition.records[opened.UnixMilli()] = NormalizedCandle{Symbol: "BTCUSDT", SourceDate: day, OpenTimeMS: opened.UnixMilli(), CloseTimeMS: opened.UnixMilli() + 59999}
	}
	openManifest, err := buildPartitionManifest(activation, "BTCUSDT", day, partition, start.Add(2*time.Minute))
	if err != nil || openManifest.PartitionStatus != "PARTITION_OPEN" || openManifest.ObservedRows != 2 || len(openManifest.MissingIntervals) == 0 {
		t.Fatalf("open partition invalid: %+v %v", openManifest, err)
	}
	closed, _ := buildPartitionManifest(activation, "BTCUSDT", day, partition, start.Add(25*time.Hour))
	if closed.PhysicalStatus != "PHYSICAL_INCOMPLETE" || closed.PartitionStatus == "PHYSICAL_COMPLETE" {
		t.Fatal("missing candles permitted physical completeness")
	}
	again, _ := buildPartitionManifest(activation, "BTCUSDT", day, partition, start.Add(2*time.Minute))
	if openManifest.PartitionHash != again.PartitionHash {
		t.Fatal("partition hash is nondeterministic")
	}
	mutated := openManifest
	mutated.ObservedRows++
	mutatedHash, _ := HashCanonical(mutated, "partition_hash")
	if mutatedHash == openManifest.PartitionHash {
		t.Fatal("manifest mutation retained identity")
	}
	left := NewStore(filepath.Join(t.TempDir(), "left"), activation)
	right := NewStore(filepath.Join(t.TempDir(), "right"), activation)
	if left.Activation.ActivationHash != right.Activation.ActivationHash || filepath.IsAbs(left.relative("manifests", "x.json")) {
		t.Fatal("absolute path affected dataset identity")
	}
	if err := WriteAtomic(filepath.Join(left.Root, "normalized", "undeclared.json"), []byte("{}\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if manifests, err := left.BuildPartitionManifests(start); err != nil || len(manifests) != 0 {
		t.Fatalf("undeclared file entered manifests: count=%d err=%v", len(manifests), err)
	}
}

func TestAtomicPartialWriteAndConflictingIdentityFailClosed(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "artifact.json")
	if err := WriteAtomic(path, []byte("complete"), 0o444); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "complete" {
		t.Fatal("atomic write accepted partial content")
	}
	ledger := filepath.Join(root, "ledger.jsonl")
	if err := AppendDurable(ledger, []byte("{partial\n")); err != nil {
		t.Fatal(err)
	}
	if err := ReadJSONLines(ledger, func(line []byte) error { var value map[string]any; return StrictDecode(line, &value) }); err == nil {
		t.Fatal("partial ledger entry was accepted")
	}
	protocol, policy := loadAuthorities(t)
	store := NewStore(filepath.Join(root, "orphan-store"), testActivation(t, protocol, policy))
	if err := WriteAtomic(filepath.Join(store.Root, "raw", "orphan"), []byte("not durable"), 0o444); err != nil {
		t.Fatal(err)
	}
	state, _, err := store.RebuildState()
	if err != nil || len(state.Cursors) != 0 || state.NextRegistration != 1 {
		t.Fatal("cursor advanced before a durable receipt")
	}
}

func TestSecurityBoundarySourceHasNoCredentialTraderOrOrderSurface(t *testing.T) {
	root := repositoryRoot(t)
	for _, relative := range []string{"internal/prospective", "internal/app/prospective_collection.go", "config/systemd"} {
		path := filepath.Join(root, relative)
		err := filepath.Walk(path, func(current string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() || strings.HasSuffix(current, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(current)
			if err != nil {
				return err
			}
			lower := strings.ToLower(string(data))
			for _, forbidden := range []string{"api_secret", "api_key", "ak-trader", "/order", "placeorder", "downtrendmidvolrelieflong240m"} {
				if strings.Contains(lower, forbidden) {
					t.Fatalf("forbidden collector surface %q in %s", forbidden, current)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func fixtureClient(provider time.Time, failSymbol string) *Client {
	date := provider.Format(http.TimeFormat)
	doer := doerFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/fapi/v1/time" {
			return response(http.StatusOK, `{"serverTime":`+jsonInt(provider.UnixMilli())+`}`, provider), nil
		}
		symbol := request.URL.Query().Get("symbol")
		if symbol == failSymbol {
			return response(http.StatusInternalServerError, `{"code":-1}`, provider), nil
		}
		start := provider.Add(-3 * time.Minute).UnixMilli()
		body := klineBody(start, start+60000, start+120000, start+180000)
		result := response(http.StatusOK, string(body), provider)
		result.Header.Set("Date", date)
		return result, nil
	})
	return &Client{HTTP: doer, MaxAttempts: 1, BaseBackoff: 0, Now: func() time.Time { return provider.Add(500 * time.Millisecond) }, Sleep: func(context.Context, time.Duration) error { return nil }}
}

func response(status int, body string, date time.Time) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Date": []string{date.Format(http.TimeFormat)}}, Body: io.NopCloser(strings.NewReader(body)), Request: &http.Request{URL: &url.URL{Scheme: "https", Host: ApprovedHost}}}
}

func klineBody(openTimes ...int64) []byte {
	rows := make([][]any, 0, len(openTimes))
	for _, opened := range openTimes {
		rows = append(rows, []any{opened, "1.0", "2.0", "0.5", "1.5", "10.0", opened + 59999, "15.0", 5, "4.0", "6.0", "0"})
	}
	data, _ := json.Marshal(rows)
	return data
}

func jsonInt(value int64) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func syncedClock(at time.Time) ClockEvidence {
	return ClockEvidence{CheckedAtUTC: at, Method: "synthetic timedatectl fixture", Synchronized: true, EvidenceHash: "sha256:" + strings.Repeat("c", 64), Diagnostic: "yes"}
}

func minimalReceipt(t *testing.T, activation Activation, complete, header, provider time.Time) Receipt {
	t.Helper()
	observed := maxTime(complete, header, provider)
	created := observed.Add(time.Second)
	available := observed
	authority := archiveauthorityReceipt(t, activation, created, available)
	receipt := Receipt{SchemaVersion: ReceiptEnvelopeVersion, CycleID: "cycle-test", CollectorSourceCommit: activation.CollectorSourceCommit, ProtocolHash: activation.ProtocolHash, RequestID: "sha256:" + strings.Repeat("1", 64), Symbol: "BTCUSDT", Endpoint: KlineEndpoint, CanonicalRequestParameters: "interval=1m&limit=5&symbol=BTCUSDT", RequestStartUTC: complete.Add(-time.Second), ResponseHeadersReceivedUTC: complete.Add(-time.Millisecond), CompleteResponseReceivedUTC: complete, ProviderHTTPDate: header.Format(http.TimeFormat), ProviderHTTPDateUTC: header, ProviderServerTimeUTC: provider, ProviderServerTimeHash: "sha256:" + strings.Repeat("2", 64), ClockEvidence: syncedClock(complete), HTTPStatus: 200, ResponseBodyByteLength: 10, RawResponseSHA256: "sha256:" + strings.Repeat("3", 64), ParsedRecordCount: 1, FirstCandleOpenTimeUTC: complete.Add(-time.Minute), FinalCandleCloseTimeUTC: complete.Add(-time.Millisecond), ReceiptCreationTimeUTC: created, ObservedAvailableAtUTC: observed, AvailabilityStatus: "PIT_ELIGIBLE", RawRelativePath: "raw/x", FragmentRelativePath: "normalized/x", FragmentHash: "sha256:" + strings.Repeat("4", 64), PriorReceiptChainHash: ZeroHash, AuthorityReceipt: authority}
	receipt.CurrentReceiptChainHash, _ = HashCanonical(receipt, "current_receipt_chain_hash")
	return receipt
}

func archiveauthorityReceipt(t *testing.T, activation Activation, created, available time.Time) archiveauthority.ProspectiveReceipt {
	t.Helper()
	receipt := archiveauthority.ProspectiveReceipt{DatasetID: activation.DatasetID, DatasetVersion: activation.ActivationHash, SourceSchemaVersion: SourceSchemaVersion, AcquisitionTimestamp: created, SourceAvailabilityTimestamp: &available, AcquisitionEvidenceType: "PUBLIC_HTTPS_PROVIDER_RESPONSE", AcquisitionEvidenceHash: "sha256:" + strings.Repeat("3", 64), ContentHash: "sha256:" + strings.Repeat("4", 64), ManifestRelativeIdentity: "normalized/x", PartitionKey: "x", Symbol: "BTCUSDT", CoveredPeriodStart: created.Add(-time.Minute), CoveredPeriodEnd: created, ExpectedPartition: true, EvaluationCutoff: created, CoveragePolicyVersion: CoveragePolicyVersion, AvailabilityPolicyVersion: AvailabilityPolicyVersion, RegistrationSequence: 1, PreviousReceiptHash: ZeroHash}
	sealed, err := archiveauthority.SealProspectiveReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}
