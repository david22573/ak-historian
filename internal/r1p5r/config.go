package r1p5r

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/david22573/ak-historian/internal/prospective"
)

type Config struct {
	RepositoryRoot     string
	DataRoot           string
	LiveDataRoot       string
	ActivationPath     string
	Protocol           Protocol
	ExposurePolicy     ExposurePolicy
	ReadinessPolicy    ReadinessPolicy
	SourceIdentity     SourceIdentity
	AbandonedRegistry  AbandonedEvidenceRegistry
	PreacquisitionSeal PreacquisitionSeal
	Activation         prospective.Activation
}

func LoadConfig(repositoryRoot, dataRoot, liveDataRoot, activationPath string) (Config, error) {
	if repositoryRoot == "" || dataRoot == "" || liveDataRoot == "" || activationPath == "" {
		return Config{}, errors.New("repository, backfill data, live data, and activation paths are required")
	}
	c := Config{RepositoryRoot: filepath.Clean(repositoryRoot), DataRoot: filepath.Clean(dataRoot), LiveDataRoot: filepath.Clean(liveDataRoot), ActivationPath: filepath.Clean(activationPath)}
	read := func(name, hashField string, target any) error {
		path := filepath.Join(c.RepositoryRoot, "authority", name)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var complete map[string]any
		if err := prospective.StrictDecode(data, &complete); err != nil {
			return err
		}
		recorded, ok := complete[hashField].(string)
		if !ok {
			return fmt.Errorf("%s is missing", hashField)
		}
		if err := prospective.VerifyCanonicalHash(complete, hashField, recorded); err != nil {
			return err
		}
		return prospective.StrictDecode(data, target)
	}
	if err := read("pr4b0_r1p5r_reacquisition_protocol.json", "protocol_hash", &c.Protocol); err != nil {
		return Config{}, fmt.Errorf("protocol: %w", err)
	}
	if err := read("pr4b0_r1p5_exposure_eligibility_policy.json", "policy_hash", &c.ExposurePolicy); err != nil {
		return Config{}, fmt.Errorf("exposure policy: %w", err)
	}
	if err := read("pr4b0_r1p5_readiness_policy.json", "policy_hash", &c.ReadinessPolicy); err != nil {
		return Config{}, fmt.Errorf("readiness policy: %w", err)
	}
	if err := prospective.ReadStrict(filepath.Join(c.RepositoryRoot, filepath.FromSlash(c.Protocol.SourceIdentityPath)), &c.SourceIdentity); err != nil {
		return Config{}, fmt.Errorf("source identity: %w", err)
	}
	if err := read(filepath.Base(c.Protocol.AbandonedRegistryPath), "registry_hash", &c.AbandonedRegistry); err != nil {
		return Config{}, fmt.Errorf("abandoned evidence registry: %w", err)
	}
	if err := read(filepath.Base(c.Protocol.PreacquisitionSealPath), "seal_hash", &c.PreacquisitionSeal); err != nil {
		return Config{}, fmt.Errorf("preacquisition seal: %w", err)
	}
	if err := prospective.ReadStrict(c.ActivationPath, &c.Activation); err != nil {
		return Config{}, fmt.Errorf("P4 activation: %w", err)
	}
	if err := VerifyConfig(c); err != nil {
		return Config{}, err
	}
	if err := VerifyRunningBinary(c.PreacquisitionSeal.BinarySHA256); err != nil {
		return Config{}, fmt.Errorf("sealed executable: %w", err)
	}
	return c, nil
}

func VerifyConfig(c Config) error {
	p := c.Protocol
	checks := []struct {
		name string
		ok   bool
	}{
		{"schema_version", p.SchemaVersion == ProtocolVersion}, {"dataset_id", p.DatasetID != ""}, {"accepted_historian_commit", p.AcceptedHistorianCommit == "3864a0c4066b7859b821b534b79a9cc3ae012fa2"},
		{"repair_source_commit", isCommit(p.RepairSourceCommit) && p.RepairSourceCommit != "59951efd756a8024455608d298c5534c778e5121"},
		{"p4_collector_source_commit", p.P4CollectorSourceCommit == "598a9119be828daa7db76dacec017456807ccfed"}, {"p4_protocol_hash", p.P4ProtocolHash == "sha256:671a27239d72e163428378dff926acc9f7a22036aff247cc8888ee9f06077311"},
		{"availability_policy_hash", p.AvailabilityPolicyHash == "sha256:cbd4c1670830843d233b6b6c3dc3dac0489e3d38fc7854caf388b5e543dfc3e1"}, {"source_schema_fingerprint", p.SourceSchemaFingerprint == prospective.SourceSchemaFingerprint},
		{"manifest_contract_hash", p.ManifestContractHash == prospective.ManifestContractHash}, {"ingestion_receipt_hash", p.IngestionReceiptHash == prospective.ReceiptSchemaHash}, {"receipt_ledger_version", p.ReceiptLedgerVersion == LedgerVersion},
		{"receipt_ledger_genesis_hash", p.ReceiptLedgerGenesisHash == ZeroHash}, {"receipt_chain_id", p.ReceiptChainID != "" && !strings.Contains(p.ReceiptChainID, "r1p5-")}, {"storage_namespace", p.StorageNamespace == "pr4b0-r1p5r"},
		{"eligible_start", p.EligibleStartUTC.Format(timeLayout) == "2026-01-01T00:00:00Z"}, {"acquisition_mode", p.AcquisitionMode == Mode},
		{"venue", p.Venue == "Binance"}, {"market_type", p.MarketType == "USD-M futures"}, {"timeframe", p.Timeframe == "1m"}, {"symbol_universe", reflect.DeepEqual(p.Symbols, prospective.UniqueSymbols)},
		{"research_prohibition", strings.Contains(strings.ToLower(p.ResearchProhibition), "do not")}, {"old_evidence_prohibition", strings.Contains(strings.ToLower(p.AbandonedEvidenceProhibition), "do not import")},
	}
	for _, check := range checks {
		if !check.ok {
			return fmt.Errorf("frozen R1P5R protocol authority mismatch: %s", check.name)
		}
	}
	if len(p.BarredIntervals) != 1 || p.BarredIntervals[0].StartUTC.Format(timeLayout) != "2024-01-01T00:00:00Z" || p.BarredIntervals[0].EndUTC != p.EligibleStartUTC {
		return errors.New("barred exposure boundary mismatch")
	}
	for _, symbol := range prospective.UniqueSymbols {
		end, ok := p.BackfillEnds[symbol]
		if !ok || !end.After(p.EligibleStartUTC) || end.Second() != 0 || end.Nanosecond() != 0 {
			return fmt.Errorf("invalid frozen end for %s", symbol)
		}
	}
	e := c.ExposurePolicy
	if e.SchemaVersion != ExposurePolicyVersion || e.ExposureLedgerHash != "sha256:5756897fe8f38591a0b181433242667b4f0fe477b6aaa92aa13cf2ae61f2bab2" || e.InspectionAuditHash != "sha256:68b25e70267ea1459520f3fb545b4247dbf03be6b041269284ce6529165878c2" || e.EligibleFloorUTC != p.EligibleStartUTC {
		return errors.New("exposure eligibility authority mismatch")
	}
	r := c.ReadinessPolicy
	if r.SchemaVersion != ReadinessPolicyVersion || r.MinimumDays != 180 || !reflect.DeepEqual(r.RequiredSymbols, prospective.UniqueSymbols) || !r.CandidateCountsForbidden || !r.FeasibilityOnly {
		return errors.New("readiness policy authority mismatch")
	}
	i := c.SourceIdentity
	if i.SchemaVersion != SourceIdentityVersion || i.RepairSourceCommit != p.RepairSourceCommit || i.ProtocolHash != p.ProtocolHash || i.AbandonedRegistryHash != p.AbandonedRegistryHash {
		return errors.New("binary/source identity mismatch")
	}
	if err := prospective.VerifyCanonicalHash(i, "identity_hash", i.IdentityHash); err != nil {
		return err
	}
	registry := c.AbandonedRegistry
	if registry.SchemaVersion != AbandonedRegistryVersion || registry.OldSourceCommit != "59951efd756a8024455608d298c5534c778e5121" || registry.OldCheckpointID != "r1p5-checkpoint-20260715T094258Z" || registry.OldCheckpointHash != "sha256:7d2cb161941deab10896ef95ecc04db7455501fb2621eca4a3ff4b87fa82e1b7" || registry.OldBackfillTerminal != "sha256:645567987688724cc29d0472b30634aaae74b5688880bb9e3ab6363516154e7f" || registry.OmittedSourceForensicHash != "sha256:554e0514f2cb65ccd1c2da543de9983f95580605f9fdbd21d574285f73de128c" || registry.RegistryHash != p.AbandonedRegistryHash {
		return errors.New("abandoned evidence registry mismatch")
	}
	for _, reason := range []string{"SOURCE_COMMIT_NOT_REPRODUCIBLE", "CHECKPOINT_NOT_RESEARCH_ELIGIBLE", "RECEIPT_CHAIN_NOT_REUSABLE"} {
		if !contains(registry.Reasons, reason) {
			return fmt.Errorf("abandoned evidence registry missing reason %s", reason)
		}
	}
	seal := c.PreacquisitionSeal
	if seal.SchemaVersion != PreacquisitionSealVersion || seal.RepairSourceCommit != p.RepairSourceCommit || !isCommit(seal.SourceSealCommit) || seal.SourceSealCommit == registry.OldSourceCommit || seal.SourceSealCommit != SourceSealCommit || seal.ProtocolHash != p.ProtocolHash || seal.SourceIdentityHash != i.IdentityHash || seal.AbandonedRegistryHash != registry.RegistryHash || !seal.FreshCloneChecksPassed || !seal.SafetyScansPassed || !seal.NoAcquisitionReceiptsAtSeal || seal.VerificationStartedAtUTC.IsZero() || !seal.VerificationCompletedAtUTC.After(seal.VerificationStartedAtUTC) || !strings.HasPrefix(seal.BinarySHA256, "sha256:") {
		return errors.New("preacquisition source seal mismatch")
	}
	if filepath.Base(c.DataRoot) != p.StorageNamespace || pathsOverlap(c.DataRoot, c.LiveDataRoot) {
		return errors.New("R1P5R storage namespace is not isolated")
	}
	if c.Activation.ActivationHash != "sha256:37bbb11677d07496b43fee24b4a84f12730713ef89506662015a53c04e8ef187" || c.Activation.CollectorSourceCommit != p.P4CollectorSourceCommit || c.Activation.ProtocolHash != p.P4ProtocolHash || !reflect.DeepEqual(c.Activation.UniqueSymbols, p.Symbols) {
		return errors.New("P4 activation authority mismatch")
	}
	return nil
}

func VerifyBinaryFile(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("binary identity mismatch: got %s want %s", actual, expected)
	}
	return nil
}

func VerifyRunningBinary(expected string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return VerifyBinaryFile(executable, expected)
}

func isCommit(value string) bool {
	if len(value) != 40 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func pathsOverlap(one, two string) bool {
	one, _ = filepath.Abs(one)
	two, _ = filepath.Abs(two)
	return one == two || strings.HasPrefix(one+string(os.PathSeparator), two+string(os.PathSeparator)) || strings.HasPrefix(two+string(os.PathSeparator), one+string(os.PathSeparator))
}

const timeLayout = "2006-01-02T15:04:05Z07:00"
