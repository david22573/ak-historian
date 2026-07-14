package prospective

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

type Config struct {
	RepositoryRoot string
	DataRoot       string
	ProtocolPath   string
	PolicyPath     string
	ActivationPath string
	Protocol       Protocol
	Policy         AvailabilityPolicy
	Activation     Activation
}

func LoadConfig(repositoryRoot, dataRoot, activationPath string) (Config, error) {
	if repositoryRoot == "" || dataRoot == "" {
		return Config{}, errors.New("repository root and data root are required")
	}
	config := Config{
		RepositoryRoot: filepath.Clean(repositoryRoot),
		DataRoot:       filepath.Clean(dataRoot),
		ProtocolPath:   filepath.Join(repositoryRoot, "authority", "pr4b0_r1p4_collection_protocol.json"),
		PolicyPath:     filepath.Join(repositoryRoot, "authority", "pr4b0_r1p4_availability_policy.json"),
		ActivationPath: filepath.Clean(activationPath),
	}
	if err := ReadStrict(config.ProtocolPath, &config.Protocol); err != nil {
		return Config{}, fmt.Errorf("load frozen protocol: %w", err)
	}
	if err := VerifyProtocol(config.Protocol); err != nil {
		return Config{}, err
	}
	if err := ReadStrict(config.PolicyPath, &config.Policy); err != nil {
		return Config{}, fmt.Errorf("load availability policy: %w", err)
	}
	if err := VerifyAvailabilityPolicy(config.Policy); err != nil {
		return Config{}, err
	}
	if err := ReadStrict(config.ActivationPath, &config.Activation); err != nil {
		return Config{}, fmt.Errorf("load activation identity: %w", err)
	}
	if err := VerifyActivation(config.Activation, config.Protocol, config.Policy); err != nil {
		return Config{}, err
	}
	return config, nil
}

func VerifyProtocol(protocol Protocol) error {
	if protocol.SchemaVersion != ProtocolVersion || protocol.DatasetID == "" || protocol.Venue != "Binance" || protocol.MarketType != "USD-M futures" || protocol.SourceSchemaVersion != SourceSchemaVersion || protocol.SourceSchemaFingerprint != SourceSchemaFingerprint || protocol.SourceSchemaAuthorityHash != SourceSchemaAuthorityHash || protocol.IngestionReceiptVersion != ReceiptSchemaVersion || protocol.IngestionReceiptHash != ReceiptSchemaHash || protocol.ManifestContractVersion != ManifestContractVersion || protocol.ManifestContractHash != ManifestContractHash || protocol.Timeframe != "1m" || protocol.CadenceSeconds != 300 {
		return errors.New("frozen collection protocol identity or source authority mismatch")
	}
	if !reflect.DeepEqual(protocol.PrimarySymbols, PrimarySymbols) || !reflect.DeepEqual(protocol.ContextSymbols, ContextSymbols) || !reflect.DeepEqual(protocol.UniqueSymbols, UniqueSymbols) {
		return errors.New("frozen protocol symbol universe mismatch")
	}
	if !strings.Contains(protocol.CompletedCandleRule, "provider") || !strings.Contains(protocol.ResearchProhibition, "prohibited") {
		return errors.New("frozen protocol completion or research boundary is incomplete")
	}
	return VerifyCanonicalHash(protocol, "protocol_hash", protocol.ProtocolHash)
}

func VerifyAvailabilityPolicy(policy AvailabilityPolicy) error {
	if policy.SchemaVersion != AvailabilityPolicyVersion || policy.IncompleteEvidenceStatus != "AVAILABILITY_EVIDENCE_INCOMPLETE" || !strings.Contains(policy.ObservedAvailableCalculation, "maximum") || !strings.Contains(policy.LocalClockRequirement, "NTPSynchronized=yes") {
		return errors.New("availability policy identity or fail-closed rules mismatch")
	}
	return VerifyCanonicalHash(policy, "policy_hash", policy.PolicyHash)
}

func VerifyActivation(activation Activation, protocol Protocol, policy AvailabilityPolicy) error {
	if err := VerifyProtocol(protocol); err != nil {
		return fmt.Errorf("activation protocol: %w", err)
	}
	if err := VerifyAvailabilityPolicy(policy); err != nil {
		return fmt.Errorf("activation availability policy: %w", err)
	}
	if activation.SchemaVersion != ActivationVersion || activation.DatasetID != protocol.DatasetID || activation.Generation == "" || strings.Contains(strings.ToLower(activation.Generation), "latest") || activation.ActivationTimestamp.IsZero() || !validGitCommit(activation.CollectorSourceCommit) || !validHash(activation.CollectorBuildID) || activation.ProtocolHash != protocol.ProtocolHash || activation.SourceSchemaVersion != SourceSchemaVersion || activation.SourceSchemaFingerprint != SourceSchemaFingerprint || activation.AvailabilityPolicyVersion != AvailabilityPolicyVersion || activation.AvailabilityPolicyHash != policy.PolicyHash || activation.CoveragePolicyVersion != CoveragePolicyVersion || activation.IngestionReceiptVersion != ReceiptSchemaVersion || activation.IngestionReceiptHash != ReceiptSchemaHash || activation.ManifestContractVersion != ManifestContractVersion || activation.ManifestContractHash != ManifestContractHash || !reflect.DeepEqual(activation.UniqueSymbols, UniqueSymbols) || activation.Timeframe != "1m" || activation.CadenceSeconds != 300 || activation.ReceiptLedgerGenesisHash != ZeroHash {
		return errors.New("activation identity does not bind the frozen collection authority")
	}
	return VerifyCanonicalHash(activation, "activation_hash", activation.ActivationHash)
}
