package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
)

type VerifyReport struct {
	Market             string  `json:"market"`
	Symbol             string  `json:"symbol"`
	Interval           string  `json:"interval"`
	From               string  `json:"from"`
	To                 string  `json:"to"`
	ExpectedCandles    int     `json:"expected_candles"`
	ActualCandles      int     `json:"actual_candles"`
	UniqueOpenTimes    int     `json:"unique_open_times"`
	DuplicateOpenTimes int     `json:"duplicate_open_times"`
	MissingCandles     int     `json:"missing_candles"`
	FirstOpenTime      string  `json:"first_open_time"`
	LastOpenTime       string  `json:"last_open_time"`
	FirstGap           *string `json:"first_gap"`
	LastGap            *string `json:"last_gap"`
	Status             string  `json:"status"`
}

type FinalReport struct {
	Symbol             string  `json:"symbol"`
	CoverageStatus     string  `json:"coverage_status"`
	ExpectedCandles    int     `json:"expected_candles"`
	ActualCandles      int     `json:"actual_candles"`
	MissingCandles     int     `json:"missing_candles"`
	FirstOpenTime      string  `json:"first_open_time"`
	LastOpenTime       string  `json:"last_open_time"`
	FirstGap           *string `json:"first_gap"`
	LastGap            *string `json:"last_gap"`
	Usable2024         bool    `json:"usable_2024"`
	Usable2025         bool    `json:"usable_2025"`
	UsableFull20242025 bool    `json:"usable_full_2024_2025"`
}

func main() {
	symbols := []string{"BTCUSDT", "ETHUSDT", "LINKUSDT", "SOLUSDT", "AVAXUSDT"}
	var reports []FinalReport

	for _, sym := range symbols {
		cmd := exec.Command("go", "run", "./cmd/ak-historian", "verify-coverage", "--market", "futures-um", "--symbol", sym, "--interval", "1m", "--from", "2024-01-01", "--to", "2025-12-31", "--source", "local", "--path", ".ak-historian/work")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Run()

		output := out.String()
		startIndex := strings.Index(output, "{")
		if startIndex == -1 {
			fmt.Println("No JSON found for", sym)
			continue
		}
		
		endIndex := strings.LastIndex(output, "}")
		if endIndex == -1 {
			continue
		}
		
		jsonStr := output[startIndex : endIndex+1]

		var r VerifyReport
		err := json.Unmarshal([]byte(jsonStr), &r)
		if err != nil {
			fmt.Println("Error unmarshalling:", err)
			continue
		}

		usable := r.MissingCandles == 0
		fr := FinalReport{
			Symbol:             r.Symbol,
			CoverageStatus:     r.Status,
			ExpectedCandles:    r.ExpectedCandles,
			ActualCandles:      r.ActualCandles,
			MissingCandles:     r.MissingCandles,
			FirstOpenTime:      r.FirstOpenTime,
			LastOpenTime:       r.LastOpenTime,
			FirstGap:           r.FirstGap,
			LastGap:            r.LastGap,
			Usable2024         : usable,
			Usable2025         : usable,
			UsableFull20242025 : usable,
		}
		reports = append(reports, fr)
	}

	b, _ := json.MarshalIndent(reports, "", "  ")
	ioutil.WriteFile("../runs/reports/phase10_5_extended_oos_candle_coverage.json", b, 0644)

	var md strings.Builder
	md.WriteString("# Phase 10.5 Extended OOS Candle Coverage\n\n")
	md.WriteString("| Symbol | Status | Expected | Actual | Missing | 2024 Usable | 2025 Usable | Full 24-25 Usable |\n")
	md.WriteString("|---|---|---|---|---|---|---|---|\n")
	for _, r := range reports {
		md.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %d | %v | %v | %v |\n", r.Symbol, r.CoverageStatus, r.ExpectedCandles, r.ActualCandles, r.MissingCandles, r.Usable2024, r.Usable2025, r.UsableFull20242025))
	}

	ioutil.WriteFile("../runs/reports/phase10_5_extended_oos_candle_coverage.md", []byte(md.String()), 0644)
	fmt.Println("Generated coverage report")
}
