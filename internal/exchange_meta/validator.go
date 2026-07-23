package exchange_meta

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

func ValidateSnapshot(snapshot *Snapshot) {
	snapshot.Validation = Validation{IsValid: true, Status: "PASS"}
	warnings := append([]Warning{}, snapshot.Warnings...)
	symbolRE := regexp.MustCompile(`^[A-Z0-9]{3,40}$`)

	snapshot.Exchange = normalizeDefault(strings.ToLower(snapshot.Exchange), StatusUnknown)
	snapshot.MarketType = normalizeDefault(strings.ToLower(snapshot.MarketType), StatusUnknown)
	snapshot.QuoteAssetFilter = strings.ToUpper(strings.TrimSpace(snapshot.QuoteAssetFilter))
	snapshot.SourceType = normalizeDefault(snapshot.SourceType, StatusUnknown)
	snapshot.SourceName = normalizeDefault(snapshot.SourceName, StatusUnknown)
	snapshot.SourceURI = normalizeDefault(filepathSlash(snapshot.SourceURI), StatusUnknown)
	snapshot.CollectedAtUTC = normalizeTimestampOrNow(snapshot.CollectedAtUTC)
	snapshot.CollectorGitSHA = normalizeDefault(snapshot.CollectorGitSHA, StatusUnknown)
	if snapshot.RawPayloadSHA256 == "" {
		snapshot.RawPayloadSHA256 = StatusUnknown
	}
	if snapshot.RawPayloadSHA256 == StatusUnknown {
		warnings = append(warnings, Warning{Code: CodeRawPayloadMissing, Message: "Raw exchange metadata payload hash is missing"})
	}
	if snapshot.SourceObservedTimeUTC == nil {
		warnings = append(warnings, Warning{Code: CodeSourceTimeUnknown, Message: "Source observed time is unknown"})
	} else {
		normalized := normalizeTimestampOrUnknown(*snapshot.SourceObservedTimeUTC)
		if normalized == StatusUnknown {
			snapshot.SourceObservedTimeUTC = nil
			warnings = append(warnings, Warning{Code: CodeSourceTimeUnknown, Message: "Source observed time is unknown"})
		} else {
			snapshot.SourceObservedTimeUTC = &normalized
		}
	}
	if isCurrentOnlySource(snapshot.SourceType) {
		snapshot.Validation.CurrentOnly = true
		warnings = append(warnings, Warning{
			Code:    CodeCurrentOnlySource,
			Message: "Current-only exchange metadata snapshot does not prove historical availability before collected_at_utc",
		})
	}
	if len(snapshot.Symbols) == 0 {
		warnings = append(warnings, Warning{Code: CodeEmptySymbolSet, Message: "Exchange metadata snapshot contains no symbols"})
		snapshot.Validation.IsValid = false
	}

	seen := map[string]bool{}
	for i := range snapshot.Symbols {
		sym := &snapshot.Symbols[i]
		sym.Symbol = strings.ToUpper(strings.TrimSpace(sym.Symbol))
		sym.BaseAsset = normalizeDefault(strings.ToUpper(sym.BaseAsset), inferBaseAsset(sym.Symbol, sym.QuoteAsset))
		sym.QuoteAsset = normalizeDefault(strings.ToUpper(sym.QuoteAsset), inferQuoteAsset(sym.Symbol))
		sym.MarketType = normalizeDefault(strings.ToLower(sym.MarketType), snapshot.MarketType)
		sym.Exchange = normalizeDefault(strings.ToLower(sym.Exchange), snapshot.Exchange)
		sym.Status = normalizeDefault(strings.ToUpper(sym.Status), StatusUnknown)
		sym.RawStatus = strings.ToUpper(strings.TrimSpace(sym.RawStatus))
		if sym.Permissions == nil {
			sym.Permissions = []string{}
		}
		sort.Strings(sym.Permissions)
		if sym.SourceFields == nil {
			sym.SourceFields = []string{}
		}
		sort.Strings(sym.SourceFields)
		if sym.Warnings == nil {
			sym.Warnings = []Warning{}
		}

		if seen[sym.Symbol] {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeDuplicateSymbol, Target: sym.Symbol, Message: "Duplicate symbol found in exchange metadata snapshot"})
			snapshot.Validation.IsValid = false
		}
		seen[sym.Symbol] = true
		if !symbolRE.MatchString(sym.Symbol) {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeSymbolMalformed, Target: sym.Symbol, Message: "Malformed symbol"})
			snapshot.Validation.IsValid = false
		}
		if !isAllowedStatus(sym.Status) {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeStatusUnknown, Target: sym.Symbol, Message: "Symbol status is unsupported"})
			snapshot.Validation.IsValid = false
		}
		if sym.Status == StatusUnknown {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeStatusUnknown, Target: sym.Symbol, Message: "Symbol status is unknown"})
			if sym.RawStatus != "" {
				sym.Warnings = append(sym.Warnings, Warning{Code: CodeStatusUnmapped, Target: sym.Symbol, Message: "Raw exchange status is not mapped to an allowed snapshot status"})
			}
		}
		if sym.OnboardDateUTC == nil {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeOnboardDateMissing, Target: sym.Symbol, Message: "Onboard/listing date is missing"})
		} else if !validRFC3339(*sym.OnboardDateUTC) {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeOnboardDateMissing, Target: sym.Symbol, Message: "Onboard/listing date is malformed"})
			snapshot.Validation.IsValid = false
		}
		if sym.DeliveryDateUTC == nil {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeDeliveryDateMissing, Target: sym.Symbol, Message: "Delivery/delisting date is missing"})
		} else if !validRFC3339(*sym.DeliveryDateUTC) {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeDeliveryDateMissing, Target: sym.Symbol, Message: "Delivery/delisting date is malformed"})
			snapshot.Validation.IsValid = false
		}
		sym.Warnings = dedupeWarnings(sym.Warnings)
		warnings = append(warnings, sym.Warnings...)
	}

	sortSnapshot(snapshot)
	snapshot.Warnings = dedupeWarnings(warnings)
	snapshot.Validation.SymbolCount = len(snapshot.Symbols)
	snapshot.Validation.WarningCodes = warningCodes(snapshot.Warnings)
	if !snapshot.Validation.IsValid {
		snapshot.Validation.Status = "FAIL"
	} else if len(snapshot.Warnings) > 0 {
		snapshot.Validation.Status = "WARN"
	}
}

func ValidateManifest(manifest *SnapshotManifest) {
	manifest.Validation = Validation{IsValid: true, Status: "PASS", SymbolCount: len(manifest.SymbolLifecycleEvidenceSummary)}
	warnings := append([]Warning{}, manifest.Warnings...)
	if manifest.SnapshotCount == 0 {
		warnings = append(warnings, Warning{Code: CodeEmptySymbolSet, Message: "Exchange metadata snapshot manifest contains no snapshots"})
		manifest.Validation.IsValid = false
	}
	for _, ref := range manifest.Snapshots {
		if ref.SymbolCount == 0 {
			warnings = append(warnings, Warning{Code: CodeEmptySymbolSet, Target: ref.SnapshotID, Message: "Snapshot reference contains no symbols"})
		}
	}
	manifest.Warnings = dedupeWarnings(warnings)
	manifest.Validation.WarningCodes = warningCodes(manifest.Warnings)
	if !manifest.Validation.IsValid {
		manifest.Validation.Status = "FAIL"
	} else if len(manifest.Warnings) > 0 {
		manifest.Validation.Status = "WARN"
	}
}

func isCurrentOnlySource(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "current", "public_endpoint_current", "file_import_current":
		return true
	default:
		return strings.Contains(value, "current")
	}
}

func isAllowedStatus(value string) bool {
	switch value {
	case StatusActive, StatusTrading, StatusBreak, StatusHalt, StatusPreTrading, StatusSettling, StatusDelivered, StatusDelisted, StatusExpired, StatusUnknown:
		return true
	default:
		return false
	}
}

func validRFC3339(value string) bool {
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

func warningCodes(warnings []Warning) []string {
	set := map[string]bool{}
	for _, w := range warnings {
		if w.Code != "" {
			set[w.Code] = true
		}
	}
	out := make([]string, 0, len(set))
	for code := range set {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

func dedupeWarnings(warnings []Warning) []Warning {
	sort.SliceStable(warnings, func(i, j int) bool {
		if warnings[i].Code != warnings[j].Code {
			return warnings[i].Code < warnings[j].Code
		}
		if warnings[i].Target != warnings[j].Target {
			return warnings[i].Target < warnings[j].Target
		}
		return warnings[i].Message < warnings[j].Message
	})
	out := warnings[:0]
	var prev Warning
	for i, w := range warnings {
		if i == 0 || w != prev {
			out = append(out, w)
			prev = w
		}
	}
	return out
}

func filepathSlash(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}
