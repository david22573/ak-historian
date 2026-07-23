package lifecycle

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

func Validate(m *Manifest) {
	m.Validation.IsValid = true
	m.Validation.Status = "PASS"
	m.Validation.WarningCodes = nil

	var warnings []Warning
	warnings = append(warnings, m.Warnings...)
	symbolRE := regexp.MustCompile(`^[A-Z0-9]{3,30}$`)

	if len(m.Symbols) == 0 {
		warnings = append(warnings, Warning{Code: CodeLifecycleEmpty, Message: "Lifecycle manifest contains no symbols"})
		m.Validation.IsValid = false
	}

	seen := map[string]bool{}
	for i := range m.Symbols {
		sym := &m.Symbols[i]
		normalizeSymbol(m, sym)

		if seen[sym.Symbol] {
			warnings = append(warnings, Warning{Code: CodeLifecycleDuplicateSymbol, Target: sym.Symbol, Message: "Duplicate symbol found"})
			m.Validation.IsValid = false
		}
		seen[sym.Symbol] = true

		if !symbolRE.MatchString(sym.Symbol) {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleSymbolMalformed, Target: sym.Symbol, Message: "Malformed symbol"})
			m.Validation.IsValid = false
		}

		listed, listedOK := parseLifecycleTime(sym.ListedAtUTC)
		delisted, delistedOK := parseLifecycleTime(sym.DelistedAtUTC)
		_, firstOK := parseLifecycleTime(sym.FirstSeenUTC)
		_, lastOK := parseLifecycleTime(sym.LastSeenUTC)
		if !listedOK || !delistedOK || !firstOK || !lastOK {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleEvidenceMissing, Target: sym.Symbol, Message: "Lifecycle timestamp is malformed"})
			m.Validation.IsValid = false
		}
		if listedOK && delistedOK && !listed.IsZero() && !delisted.IsZero() && listed.After(delisted) {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleListedAfterDelisted, Target: sym.Symbol, Message: "listed_at_utc is after delisted_at_utc"})
			m.Validation.IsValid = false
		}

		if sym.Status == StatusUnknown {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleStatusUnknown, Target: sym.Symbol, Message: "Lifecycle status is unknown"})
		}
		if sym.ListedAtUTC == StatusUnknown {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleListedDateMissing, Target: sym.Symbol, Message: "Listing date is unknown"})
		}
		if sym.DelistedAtUTC == StatusUnknown {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleDelistedDateMissing, Target: sym.Symbol, Message: "Delisting date is unknown"})
		}

		if !validStatus(sym.Status) {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleStatusUnknown, Target: sym.Symbol, Message: "Lifecycle status value is unsupported"})
			m.Validation.IsValid = false
		}
		if !validEvidenceLevel(sym.EvidenceLevel) {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleEvidenceMissing, Target: sym.Symbol, Message: "Lifecycle evidence level is unsupported"})
			m.Validation.IsValid = false
		}
		if sym.EvidenceLevel == EvidenceUnknown || len(sym.Sources) == 0 {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleEvidenceMissing, Target: sym.Symbol, Message: "Lifecycle evidence is missing"})
		}
		if sym.EvidenceLevel == EvidenceLocalDataFirstSeen {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleLocalDataOnlyNotListingProof, Target: sym.Symbol, Message: "Local data first seen is not exchange listing proof"})
		}
		if sym.EvidenceLevel == EvidenceCurrentActiveOnly {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleCurrentActiveOnlyRisk, Target: sym.Symbol, Message: "Current active evidence is survivorship biased"})
		}
		if sym.EvidenceLevel == EvidenceUserProvidedUnverified {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleUserProvidedUnverified, Target: sym.Symbol, Message: "User-provided evidence is not independently verified"})
		}
		if sym.EvidenceLevel == EvidenceVerifiedExchangeListing && sym.ListedAtUTC == StatusUnknown {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleListedDateMissing, Target: sym.Symbol, Message: "Verified listing evidence requires listed_at_utc"})
			m.Validation.IsValid = false
		}
		if sym.EvidenceLevel == EvidenceVerifiedExchangeDelisting && sym.DelistedAtUTC == StatusUnknown {
			sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleDelistedDateMissing, Target: sym.Symbol, Message: "Verified delisting evidence requires delisted_at_utc"})
			m.Validation.IsValid = false
		}

		for j := range sym.Sources {
			src := &sym.Sources[j]
			normalizeSource(src)
			if _, ok := parseLifecycleTime(src.ObservedAtUTC); !ok {
				sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleEvidenceMissing, Target: sym.Symbol, Message: "Lifecycle source observed_at_utc is malformed"})
				m.Validation.IsValid = false
			}
			if src.SourceHash == StatusUnknown {
				sym.Warnings = append(sym.Warnings, Warning{Code: CodeLifecycleSourceHashMissing, Target: sym.Symbol, Message: "Lifecycle source hash is missing"})
			}
		}
		sym.Warnings = dedupeWarnings(sym.Warnings)
		warnings = append(warnings, sym.Warnings...)
	}

	m.Warnings = dedupeWarnings(warnings)
	m.Validation.WarningCodes = warningCodes(m.Warnings)
	m.Validation.ListingEvidenceStatus = classifyListingEvidence(m.Symbols)
	m.Validation.DelistingEvidenceStatus = classifyDelistingEvidence(m.Symbols)
	m.Validation.SurvivorshipSupportStatus = classifySurvivorshipSupport(m.Symbols)
	m.Validation.LifecycleEvidenceSupported = m.Validation.SurvivorshipSupportStatus == SupportLowSupported
	if m.Validation.SurvivorshipSupportStatus != SupportLowSupported {
		m.Warnings = append(m.Warnings, Warning{
			Code:    CodeLifecycleLowRiskUnproven,
			Message: "Lifecycle evidence does not support LOW survivorship risk",
		})
		m.Warnings = dedupeWarnings(m.Warnings)
		m.Validation.WarningCodes = warningCodes(m.Warnings)
	}
	if !m.Validation.IsValid {
		m.Validation.Status = "FAIL"
	} else if len(m.Warnings) > 0 {
		m.Validation.Status = "WARN"
	}
}

func normalizeSymbol(m *Manifest, sym *SymbolEntry) {
	sym.Symbol = strings.ToUpper(strings.TrimSpace(sym.Symbol))
	sym.BaseAsset = normalizeDefault(sym.BaseAsset, inferBaseAsset(sym.Symbol, sym.QuoteAsset))
	sym.QuoteAsset = normalizeDefault(sym.QuoteAsset, m.QuoteAsset)
	sym.MarketType = normalizeDefault(sym.MarketType, m.MarketType)
	sym.Exchange = normalizeDefault(sym.Exchange, m.Exchange)
	sym.Status = normalizeDefault(strings.ToUpper(strings.TrimSpace(sym.Status)), StatusUnknown)
	sym.ListedAtUTC = normalizeTimestampOrUnknown(sym.ListedAtUTC)
	sym.DelistedAtUTC = normalizeTimestampOrUnknown(sym.DelistedAtUTC)
	sym.FirstSeenUTC = normalizeTimestampOrUnknown(sym.FirstSeenUTC)
	sym.LastSeenUTC = normalizeTimestampOrUnknown(sym.LastSeenUTC)
	sym.EvidenceLevel = normalizeDefault(strings.ToUpper(strings.TrimSpace(sym.EvidenceLevel)), EvidenceUnknown)
	if sym.Sources == nil {
		sym.Sources = []SourceEntry{}
	}
	if sym.Warnings == nil {
		sym.Warnings = []Warning{}
	}
}

func normalizeSource(src *SourceEntry) {
	src.SourceType = normalizeDefault(src.SourceType, StatusUnknown)
	src.SourceName = normalizeDefault(src.SourceName, StatusUnknown)
	src.SourceURIOrPath = normalizeSourcePath(src.SourceURIOrPath)
	src.SourceHash = normalizeDefault(src.SourceHash, StatusUnknown)
	src.ObservedAtUTC = normalizeTimestampOrUnknown(src.ObservedAtUTC)
	if src.EvidenceFields == nil {
		src.EvidenceFields = []string{}
	}
	sort.Strings(src.EvidenceFields)
	src.Confidence = normalizeDefault(strings.ToUpper(strings.TrimSpace(src.Confidence)), StatusUnknown)
}

func parseLifecycleTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, StatusUnknown) {
		return time.Time{}, true
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func validStatus(value string) bool {
	switch value {
	case StatusActive, StatusDelist, StatusExpired, StatusRenamed, StatusUnknown:
		return true
	default:
		return false
	}
}

func validEvidenceLevel(value string) bool {
	switch value {
	case EvidenceVerifiedExchangeListing, EvidenceVerifiedExchangeDelisting, EvidenceHistoricalSnapshot, EvidenceLocalDataFirstSeen, EvidenceCurrentActiveOnly, EvidenceUserProvidedUnverified, EvidenceUnknown:
		return true
	default:
		return false
	}
}

func classifyListingEvidence(symbols []SymbolEntry) string {
	if len(symbols) == 0 {
		return ListingEvidenceMissing
	}
	allVerified := true
	anyLocal := false
	anyCurrent := false
	anyUser := false
	for _, sym := range symbols {
		if sym.EvidenceLevel == EvidenceLocalDataFirstSeen {
			anyLocal = true
		}
		if sym.EvidenceLevel == EvidenceCurrentActiveOnly {
			anyCurrent = true
		}
		if sym.EvidenceLevel == EvidenceUserProvidedUnverified {
			anyUser = true
		}
		if sym.ListedAtUTC == StatusUnknown || (sym.EvidenceLevel != EvidenceVerifiedExchangeListing && sym.EvidenceLevel != EvidenceVerifiedExchangeDelisting && sym.EvidenceLevel != EvidenceHistoricalSnapshot) {
			allVerified = false
		}
	}
	switch {
	case allVerified:
		return ListingEvidenceVerified
	case anyLocal:
		return ListingEvidenceFirstSeenOnly
	case anyCurrent:
		return ListingEvidenceCurrentOnly
	case anyUser:
		return ListingEvidenceUserProvided
	default:
		return ListingEvidenceMissing
	}
}

func classifyDelistingEvidence(symbols []SymbolEntry) string {
	if len(symbols) == 0 {
		return DelistingEvidenceMissing
	}
	for _, sym := range symbols {
		if sym.DelistedAtUTC == StatusUnknown || (sym.EvidenceLevel != EvidenceVerifiedExchangeDelisting && sym.EvidenceLevel != EvidenceHistoricalSnapshot) {
			return DelistingEvidenceMissing
		}
	}
	return DelistingEvidenceVerified
}

func classifySurvivorshipSupport(symbols []SymbolEntry) string {
	if len(symbols) == 0 {
		return SupportUnknown
	}
	for _, sym := range symbols {
		if sym.ListedAtUTC == StatusUnknown || sym.DelistedAtUTC == StatusUnknown {
			return SupportElevated
		}
		if sym.EvidenceLevel != EvidenceVerifiedExchangeDelisting && sym.EvidenceLevel != EvidenceHistoricalSnapshot {
			return SupportElevated
		}
		for _, src := range sym.Sources {
			if src.SourceHash == StatusUnknown {
				return SupportElevated
			}
		}
	}
	return SupportLowSupported
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
