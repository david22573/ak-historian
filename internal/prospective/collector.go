package prospective

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/archiveauthority"
)

var CollectorSourceCommit = "UNSET"

type Collector struct {
	Config Config
	Client *Client
	Clock  ClockChecker
	Now    func() time.Time
	Store  *Store
}

func NewCollector(config Config) (*Collector, error) {
	if CollectorSourceCommit == "UNSET" || CollectorSourceCommit != config.Activation.CollectorSourceCommit {
		return nil, fmt.Errorf("binary collector source commit %s does not match activation %s", CollectorSourceCommit, config.Activation.CollectorSourceCommit)
	}
	return &Collector{Config: config, Client: NewClient(), Clock: SystemClockChecker{}, Now: func() time.Time { return time.Now().UTC() }, Store: NewStore(config.DataRoot, config.Activation)}, nil
}

func (collector *Collector) CollectOnce(ctx context.Context) (CycleResult, error) {
	if collector.Client == nil || collector.Clock == nil || collector.Now == nil || collector.Store == nil {
		return CycleResult{}, errors.New("collector dependencies are incomplete")
	}
	lock, err := AcquireLock(collector.Store.LockPath())
	if err != nil {
		return CycleResult{}, err
	}
	defer lock.Close()
	state, _, err := collector.Store.RebuildState()
	if err != nil {
		return CycleResult{}, fmt.Errorf("verify durable state: %w", err)
	}
	if err := collector.Store.SaveState(state); err != nil {
		return CycleResult{}, err
	}
	started := collector.Now().UTC()
	cycle := CycleResult{SchemaVersion: CycleVersion, StartedAtUTC: started}
	cycle.CycleID = "cycle-" + strings.ReplaceAll(started.Format("20060102T150405.000000000Z"), ".", "")
	clockEvidence, clockErr := collector.Clock.Check(ctx)
	cycle.ClockEvidence = clockEvidence
	if clockErr != nil {
		cycle.CompletedAtUTC = collector.Now().UTC()
		for _, symbol := range UniqueSymbols {
			cycle.Symbols = append(cycle.Symbols, SymbolCycleStatus{Symbol: symbol, Error: clockErr.Error()})
		}
		return collector.finishCycle(cycle, fmt.Errorf("clock evidence: %w", clockErr))
	}
	providerTime, providerTimeHash, _, err := collector.Client.ServerTime(ctx)
	if err != nil {
		cycle.CompletedAtUTC = collector.Now().UTC()
		for _, symbol := range UniqueSymbols {
			cycle.Symbols = append(cycle.Symbols, SymbolCycleStatus{Symbol: symbol, Error: err.Error()})
		}
		return collector.finishCycle(cycle, err)
	}
	cycle.ProviderServerTimeUTC = providerTime
	for _, symbol := range UniqueSymbols {
		status, receipt, raw, fragmentData, receiptRelative, err := collector.collectSymbol(ctx, cycle.CycleID, symbol, providerTime, providerTimeHash, clockEvidence, state)
		if err == nil {
			var added bool
			if added, err = collector.Store.CommitReceipt(receipt, raw, fragmentData, receipt.RawRelativePath, receipt.FragmentRelativePath, receiptRelative); err == nil && added {
				state.LastEnvelopeHash = receipt.CurrentReceiptChainHash
				state.LastAuthorityHash = receipt.AuthorityReceipt.ReceiptHash
				state.NextRegistration++
				state.Cursors[symbol] = Cursor{LastOpenTimeMS: receipt.FinalCandleCloseTimeUTC.UnixMilli() - 59999, LastReceiptHash: receipt.CurrentReceiptChainHash}
				err = collector.Store.SaveState(state)
			} else if err == nil {
				status.Duplicates++
			}
		}
		if err != nil {
			status.Success = false
			status.Error = err.Error()
		}
		cycle.Symbols = append(cycle.Symbols, status)
	}
	cycle.FullUniverseSuccess = len(cycle.Symbols) == len(UniqueSymbols)
	for _, status := range cycle.Symbols {
		if !status.Success {
			cycle.FullUniverseSuccess = false
			break
		}
	}
	cycle.CompletedAtUTC = collector.Now().UTC()
	if cycle.FullUniverseSuccess {
		state.LastSuccessfulCycleID = cycle.CycleID
		state.LastSuccessfulCycleUTC = cycle.CompletedAtUTC
		if err := collector.Store.SaveState(state); err != nil {
			return CycleResult{}, err
		}
	}
	return collector.finishCycle(cycle, nil)
}

func (collector *Collector) finishCycle(cycle CycleResult, collectionErr error) (CycleResult, error) {
	cycle.CycleHash = ""
	hash, err := HashCanonical(cycle, "cycle_hash")
	if err != nil {
		return CycleResult{}, err
	}
	cycle.CycleHash = hash
	if err := collector.Store.CommitCycle(cycle); err != nil {
		return CycleResult{}, err
	}
	if _, err := collector.Store.BuildPartitionManifests(cycle.CompletedAtUTC); err != nil {
		return cycle, err
	}
	if collectionErr != nil {
		return cycle, collectionErr
	}
	if !cycle.FullUniverseSuccess {
		return cycle, errors.New("one or more symbols failed; full-cycle success withheld")
	}
	return cycle, nil
}

func (collector *Collector) collectSymbol(ctx context.Context, cycleID, symbol string, providerTime time.Time, providerTimeHash string, clock ClockEvidence, state State) (SymbolCycleStatus, Receipt, []byte, []byte, string, error) {
	status := SymbolCycleStatus{Symbol: symbol}
	cursor := state.Cursors[symbol]
	startTime := int64(0)
	if cursor.LastOpenTimeMS > 0 {
		startTime = cursor.LastOpenTimeMS + 60000
	}
	evidence, params, err := collector.Client.Klines(ctx, symbol, startTime)
	if err != nil {
		if retainErr := collector.Store.RetainIncomplete(cycleID, symbol, evidence, err.Error()); retainErr != nil {
			return status, Receipt{}, nil, nil, "", fmt.Errorf("retain incomplete evidence: %w (request error: %v)", retainErr, err)
		}
		return status, Receipt{}, nil, nil, "", err
	}
	records, err := ParseKlines(evidence.Body, symbol, providerTime)
	if err != nil {
		status.SchemaFailures = 1
		return status, Receipt{}, nil, nil, "", err
	}
	if len(records) == 0 {
		return status, Receipt{}, nil, nil, "", errors.New("no newly completed candle was returned")
	}
	if startTime > 0 && records[0].OpenTimeMS > startTime {
		return status, Receipt{}, nil, nil, "", fmt.Errorf("physical gap before first returned candle: expected %d got %d", startTime, records[0].OpenTimeMS)
	}
	for index := 1; index < len(records); index++ {
		if records[index].OpenTimeMS != records[index-1].OpenTimeMS+60000 {
			return status, Receipt{}, nil, nil, "", fmt.Errorf("physical gap within provider response at record %d", index)
		}
	}
	fragment := Fragment{SchemaVersion: FragmentVersion, NormalizationVersion: FragmentVersion, CycleID: cycleID, Symbol: symbol, SourceSchemaVersion: SourceSchemaVersion, SourceSchemaFingerprint: SourceSchemaFingerprint, Records: records}
	fragment.FragmentHash = ""
	fragmentHash, err := HashCanonical(fragment, "fragment_hash")
	if err != nil {
		return status, Receipt{}, nil, nil, "", err
	}
	fragment.FragmentHash = fragmentHash
	fragmentData, err := CanonicalJSON(fragment)
	if err != nil {
		return status, Receipt{}, nil, nil, "", err
	}
	observed := maxTime(evidence.CompleteResponseReceivedUTC, evidence.ProviderHTTPDateUTC, providerTime)
	created := collector.Now().UTC()
	if created.Before(observed) {
		created = observed
	}
	date := records[0].SourceDate
	base := "cycle=" + cycleID + "/symbol=" + symbol
	rawRelative := filepath.ToSlash(filepath.Join("raw", "date="+date, base+".json"))
	fragmentRelative := filepath.ToSlash(filepath.Join("normalized", "date="+date, base+".json"))
	receiptRelative := filepath.ToSlash(filepath.Join("receipts", "date="+date, base+".json"))
	partitionKey := fmt.Sprintf("futures-um/1m/%s/%d-%d", symbol, records[0].OpenTimeMS, records[len(records)-1].OpenTimeMS)
	authority := archiveauthority.ProspectiveReceipt{
		DatasetID: collector.Config.Activation.DatasetID, DatasetVersion: collector.Config.Activation.ActivationHash, SourceSchemaVersion: SourceSchemaVersion,
		AcquisitionTimestamp: created, SourceAvailabilityTimestamp: &observed, AcquisitionEvidenceType: "PUBLIC_HTTPS_PROVIDER_RESPONSE", AcquisitionEvidenceHash: HashBytes(evidence.Body), ContentHash: fragmentHash,
		ManifestRelativeIdentity: fragmentRelative, PartitionKey: partitionKey, Symbol: symbol, CoveredPeriodStart: time.UnixMilli(records[0].OpenTimeMS).UTC(), CoveredPeriodEnd: time.UnixMilli(records[len(records)-1].OpenTimeMS + 60000).UTC(), ExpectedPartition: true,
		EvaluationCutoff: created, CoveragePolicyVersion: CoveragePolicyVersion, AvailabilityPolicyVersion: AvailabilityPolicyVersion, RegistrationSequence: state.NextRegistration, PreviousReceiptHash: state.LastAuthorityHash,
	}
	authority, err = archiveauthority.SealProspectiveReceipt(authority)
	if err != nil {
		return status, Receipt{}, nil, nil, "", err
	}
	receipt := Receipt{SchemaVersion: ReceiptEnvelopeVersion, CycleID: cycleID, CollectorSourceCommit: collector.Config.Activation.CollectorSourceCommit, ProtocolHash: collector.Config.Protocol.ProtocolHash, RequestID: HashBytes([]byte(cycleID + "\n" + symbol + "\n" + params)), Symbol: symbol, Endpoint: KlineEndpoint, CanonicalRequestParameters: params, RequestStartUTC: evidence.RequestStartUTC, ResponseHeadersReceivedUTC: evidence.ResponseHeadersReceivedUTC, CompleteResponseReceivedUTC: evidence.CompleteResponseReceivedUTC, ProviderHTTPDate: evidence.ProviderHTTPDate, ProviderHTTPDateUTC: evidence.ProviderHTTPDateUTC, ProviderServerTimeUTC: providerTime, ProviderServerTimeHash: providerTimeHash, ClockEvidence: clock, HTTPStatus: evidence.HTTPStatus, RetryNumber: evidence.RetryNumber, ResponseBodyByteLength: len(evidence.Body), RawResponseSHA256: HashBytes(evidence.Body), ParsedRecordCount: len(records), FirstCandleOpenTimeUTC: time.UnixMilli(records[0].OpenTimeMS).UTC(), FinalCandleCloseTimeUTC: time.UnixMilli(records[len(records)-1].CloseTimeMS).UTC(), ReceiptCreationTimeUTC: created, ObservedAvailableAtUTC: observed, AvailabilityStatus: "PIT_ELIGIBLE", RawRelativePath: rawRelative, FragmentRelativePath: fragmentRelative, FragmentHash: fragmentHash, PriorReceiptChainHash: state.LastEnvelopeHash, AuthorityReceipt: authority}
	receipt.CurrentReceiptChainHash, err = HashCanonical(receipt, "current_receipt_chain_hash")
	if err != nil {
		return status, Receipt{}, nil, nil, "", err
	}
	status.Success = true
	status.ReceiptHash = receipt.CurrentReceiptChainHash
	status.Records = len(records)
	return status, receipt, evidence.Body, fragmentData, receiptRelative, nil
}

func VerifyReceipt(receipt Receipt, activation Activation, priorEnvelope, priorAuthority string, sequence uint64) error {
	if receipt.SchemaVersion != ReceiptEnvelopeVersion || receipt.CollectorSourceCommit != activation.CollectorSourceCommit || receipt.ProtocolHash != activation.ProtocolHash || !contains(UniqueSymbols, receipt.Symbol) || receipt.Endpoint != KlineEndpoint || receipt.HTTPStatus != 200 || receipt.ResponseBodyByteLength <= 0 || !validHash(receipt.RawResponseSHA256) || receipt.ParsedRecordCount <= 0 || receipt.ProviderHTTPDateUTC.IsZero() || receipt.ProviderServerTimeUTC.IsZero() || !receipt.ClockEvidence.Synchronized || receipt.PriorReceiptChainHash != priorEnvelope || receipt.AuthorityReceipt.PreviousReceiptHash != priorAuthority || receipt.AuthorityReceipt.RegistrationSequence != sequence || receipt.ObservedAvailableAtUTC != maxTime(receipt.CompleteResponseReceivedUTC, receipt.ProviderHTTPDateUTC, receipt.ProviderServerTimeUTC) || receipt.AvailabilityStatus != "PIT_ELIGIBLE" {
		return errors.New("receipt required authority or chain fields are invalid")
	}
	if err := archiveauthority.VerifyProspectiveReceipt(receipt.AuthorityReceipt); err != nil {
		return err
	}
	return VerifyCanonicalHash(receipt, "current_receipt_chain_hash", receipt.CurrentReceiptChainHash)
}

func maxTime(values ...time.Time) time.Time {
	var maximum time.Time
	for _, value := range values {
		if value.After(maximum) {
			maximum = value.UTC()
		}
	}
	return maximum
}
