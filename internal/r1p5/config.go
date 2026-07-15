package r1p5

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/david22573/ak-historian/internal/prospective"
)

type Config struct {
	RepositoryRoot  string
	DataRoot        string
	LiveDataRoot    string
	ActivationPath  string
	Protocol        Protocol
	ExposurePolicy  ExposurePolicy
	ReadinessPolicy ReadinessPolicy
	SourceIdentity  SourceIdentity
	Activation      prospective.Activation
}

func LoadConfig(repositoryRoot, dataRoot, liveDataRoot, activationPath string) (Config, error) {
	if repositoryRoot == "" || dataRoot == "" || liveDataRoot == "" || activationPath == "" {
		return Config{}, errors.New("repository, backfill data, live data, and activation paths are required")
	}
	c := Config{RepositoryRoot: filepath.Clean(repositoryRoot), DataRoot: filepath.Clean(dataRoot), LiveDataRoot: filepath.Clean(liveDataRoot), ActivationPath: filepath.Clean(activationPath)}
	read := func(name string, target any) error {
		return prospective.ReadStrict(filepath.Join(c.RepositoryRoot, "authority", name), target)
	}
	if err := read("pr4b0_r1p5_coverage_protocol.json", &c.Protocol); err != nil {
		return Config{}, fmt.Errorf("protocol: %w", err)
	}
	if err := read("pr4b0_r1p5_exposure_eligibility_policy.json", &c.ExposurePolicy); err != nil {
		return Config{}, fmt.Errorf("exposure policy: %w", err)
	}
	if err := read("pr4b0_r1p5_readiness_policy.json", &c.ReadinessPolicy); err != nil {
		return Config{}, fmt.Errorf("readiness policy: %w", err)
	}
	if err := prospective.ReadStrict(filepath.Join(c.RepositoryRoot, filepath.FromSlash(c.Protocol.SourceIdentityPath)), &c.SourceIdentity); err != nil {
		return Config{}, fmt.Errorf("source identity: %w", err)
	}
	if err := prospective.ReadStrict(c.ActivationPath, &c.Activation); err != nil {
		return Config{}, fmt.Errorf("P4 activation: %w", err)
	}
	if err := VerifyConfig(c); err != nil {
		return Config{}, err
	}
	return c, nil
}

func VerifyConfig(c Config) error {
	p := c.Protocol
	if p.SchemaVersion != ProtocolVersion || p.DatasetID == "" || p.AcceptedHistorianCommit != "da9a6db4fad3ee5d47347453e164af6405d21fb8" || p.P4CollectorSourceCommit != "598a9119be828daa7db76dacec017456807ccfed" || p.P4ProtocolHash != "sha256:671a27239d72e163428378dff926acc9f7a22036aff247cc8888ee9f06077311" || p.AvailabilityPolicyHash != "sha256:cbd4c1670830843d233b6b6c3dc3dac0489e3d38fc7854caf388b5e543dfc3e1" || p.SourceSchemaFingerprint != prospective.SourceSchemaFingerprint || p.ManifestContractHash != prospective.ManifestContractHash || p.IngestionReceiptHash != prospective.ReceiptSchemaHash || p.ReceiptLedgerVersion != LedgerVersion || p.ReceiptLedgerGenesisHash != ZeroHash || p.EligibleStartUTC.Format(timeLayout) != "2026-01-01T00:00:00Z" || p.AcquisitionMode != Mode || p.Venue != "Binance" || p.MarketType != "USD-M futures" || p.Timeframe != "1m" || !reflect.DeepEqual(p.Symbols, prospective.UniqueSymbols) || !strings.Contains(strings.ToLower(p.ResearchProhibition), "prohibit") {
		return errors.New("frozen R1P5 protocol authority mismatch")
	}
	if err := prospective.VerifyCanonicalHash(p, "protocol_hash", p.ProtocolHash); err != nil {
		return err
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
	if err := prospective.VerifyCanonicalHash(e, "policy_hash", e.PolicyHash); err != nil {
		return err
	}
	r := c.ReadinessPolicy
	if r.SchemaVersion != ReadinessPolicyVersion || r.MinimumDays != 180 || !reflect.DeepEqual(r.RequiredSymbols, prospective.UniqueSymbols) || !r.CandidateCountsForbidden || !r.FeasibilityOnly {
		return errors.New("readiness policy authority mismatch")
	}
	if err := prospective.VerifyCanonicalHash(r, "policy_hash", r.PolicyHash); err != nil {
		return err
	}
	i := c.SourceIdentity
	if i.SchemaVersion != SourceIdentityVersion || i.SourceCommit != BackfillSourceCommit || i.ProtocolHash != p.ProtocolHash {
		return errors.New("binary/source identity mismatch")
	}
	if err := prospective.VerifyCanonicalHash(i, "identity_hash", i.IdentityHash); err != nil {
		return err
	}
	if c.Activation.ActivationHash != "sha256:37bbb11677d07496b43fee24b4a84f12730713ef89506662015a53c04e8ef187" || c.Activation.CollectorSourceCommit != p.P4CollectorSourceCommit || c.Activation.ProtocolHash != p.P4ProtocolHash || !reflect.DeepEqual(c.Activation.UniqueSymbols, p.Symbols) {
		return errors.New("P4 activation authority mismatch")
	}
	return nil
}

const timeLayout = "2006-01-02T15:04:05Z07:00"
