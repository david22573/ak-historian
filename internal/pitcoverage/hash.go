package pitcoverage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

func (r *Report) ComputeCoverageHash() (string, error) {
	copyR := *r
	copyR.GeneratedAtUTC = ""
	copyR.Hashes.ReportHash = ""
	copyR.Hashes.CoverageHash = ""

	if copyR.Symbols == nil {
		copyR.Symbols = []SymbolEntry{}
	}
	if copyR.Windows == nil {
		copyR.Windows = []Window{}
	}
	if copyR.Warnings == nil {
		copyR.Warnings = []Warning{}
	}

	sort.SliceStable(copyR.Symbols, func(i, j int) bool {
		return copyR.Symbols[i].Symbol < copyR.Symbols[j].Symbol
	})

	sort.SliceStable(copyR.Windows, func(i, j int) bool {
		return copyR.Windows[i].StartUTC < copyR.Windows[j].StartUTC
	})

	sort.SliceStable(copyR.Warnings, func(i, j int) bool {
		if copyR.Warnings[i].Code == copyR.Warnings[j].Code {
			return copyR.Warnings[i].TargetSymbol < copyR.Warnings[j].TargetSymbol
		}
		return copyR.Warnings[i].Code < copyR.Warnings[j].Code
	})

	for i := range copyR.Symbols {
		if copyR.Symbols[i].Warnings == nil {
			copyR.Symbols[i].Warnings = []Warning{}
		}
		if copyR.Symbols[i].PromotionBlockingReasons == nil {
			copyR.Symbols[i].PromotionBlockingReasons = []string{}
		}
		sort.Strings(copyR.Symbols[i].PromotionBlockingReasons)
		sort.SliceStable(copyR.Symbols[i].Warnings, func(a, b int) bool {
			return copyR.Symbols[i].Warnings[a].Code < copyR.Symbols[i].Warnings[b].Code
		})
	}

	b, err := json.Marshal(copyR)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}

func (r *Report) ComputeReportHash() (string, error) {
	copyR := *r
	copyR.Hashes.ReportHash = ""
	b, err := json.Marshal(copyR)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}
