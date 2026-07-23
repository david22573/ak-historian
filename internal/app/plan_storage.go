package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(planStorageCmd)
	planStorageCmd.Flags().String("workdir", ".ak-historian/work", "working directory")
	planStorageCmd.Flags().String("market", "futures-um", "market type")
	planStorageCmd.Flags().String("interval", "1m", "interval")
	planStorageCmd.Flags().String("symbols", "", "comma separated symbols")
	planStorageCmd.Flags().String("from", "", "from YYYY-MM")
	planStorageCmd.Flags().String("to", "", "to YYYY-MM")
	planStorageCmd.Flags().Int("disk-budget-gb", 20, "disk budget in gb")
	planStorageCmd.Flags().Int("min-free-gb", 5, "min free gb")
}

var planStorageCmd = &cobra.Command{
	Use:   "plan-storage",
	Short: "Plan storage staging",
	RunE: func(cmd *cobra.Command, args []string) error {
		workdir, _ := cmd.Flags().GetString("workdir")
		market, _ := cmd.Flags().GetString("market")
		interval, _ := cmd.Flags().GetString("interval")
		symbolsStr, _ := cmd.Flags().GetString("symbols")
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		budget, _ := cmd.Flags().GetInt("disk-budget-gb")
		minFree, _ := cmd.Flags().GetInt("min-free-gb")

		return runPlanStorage(workdir, market, interval, symbolsStr, from, to, budget, minFree)
	},
}

type StagingGroup struct {
	GroupID                    int      `json:"group_id"`
	Symbols                    []string `json:"symbols"`
	Year                       string   `json:"year"`
	EstimatedSizeBytes         int64    `json:"estimated_size_bytes"`
	EstimatedSizeHuman         string   `json:"estimated_size_human"`
	FitsBudget                 bool     `json:"fits_budget"`
	RequiresFetch              bool     `json:"requires_fetch"`
	RequiresArchiveBeforeFetch bool     `json:"requires_archive_before_fetch"`
	RequiresCleanupBeforeFetch bool     `json:"requires_cleanup_before_fetch"`
	FetchCommands              []string `json:"fetch_commands"`
	VerifyCommands             []string `json:"verify_commands"`
	EnginePrepCommands         []string `json:"engine_prep_commands"`
	CleanupCommandsAfterEngine []string `json:"cleanup_commands_after_engine"`
}

type StoragePlan struct {
	Groups []StagingGroup `json:"groups"`
}

func runPlanStorage(workdir, market, interval, symbolsStr, from, to string, budgetGB, minFreeGB int) error {
	budgetBytes := int64(budgetGB) * 1024 * 1024 * 1024

	symbols := strings.Split(symbolsStr, ",")
	var targetSymbols []string
	var coreSymbols []string
	for _, s := range symbols {
		if s == "BTCUSDT" || s == "ETHUSDT" {
			coreSymbols = append(coreSymbols, s)
		} else {
			targetSymbols = append(targetSymbols, s)
		}
	}

	fromYear := from[:4]
	toYear := to[:4]
	fromYearInt, _ := strconv.Atoi(fromYear)
	toYearInt, _ := strconv.Atoi(toYear)
	var years []string
	for y := fromYearInt; y <= toYearInt; y++ {
		years = append(years, strconv.Itoa(y))
	}

	plan := StoragePlan{}
	groupID := 1

	for _, y := range years {
		for _, t := range targetSymbols {
			// group: BTCUSDT + ETHUSDT + t
			groupSymbols := append(coreSymbols, t)

			// 1m data for 1 year is approx 500MB per symbol in Parquet. Let's estimate 1GB per symbol for safety.
			// Actually, let's estimate 1GB per symbol per year.
			estBytes := int64(len(groupSymbols)) * 1024 * 1024 * 1024

			grp := StagingGroup{
				GroupID:                    groupID,
				Symbols:                    groupSymbols,
				Year:                       y,
				EstimatedSizeBytes:         estBytes,
				EstimatedSizeHuman:         fmt.Sprintf("%d GB", len(groupSymbols)),
				FitsBudget:                 estBytes <= budgetBytes,
				RequiresFetch:              true,
				RequiresArchiveBeforeFetch: false,
				RequiresCleanupBeforeFetch: groupID > 1, // If we have multiple groups, we need to clean up previous ones
			}

			// Generate commands
			symStr := strings.Join(groupSymbols, ",")
			grp.FetchCommands = []string{
				fmt.Sprintf("ak-historian fetch --workdir %s --market %s --interval %s --symbols %s --period monthly --start %s-01 --end %s-12", workdir, market, interval, symStr, y, y),
			}
			grp.VerifyCommands = []string{
				fmt.Sprintf("ak-historian verify-archive --workdir %s --source r2 --market %s --interval %s --symbols %s --from %s-01 --to %s-12", workdir, market, interval, symStr, y, y),
			}
			grp.EnginePrepCommands = []string{
				fmt.Sprintf("ak-engine phase10-low-resource-prep --market %s --interval %s --symbols %s --start %s-01-01 --end %s-12-31 --max-symbols 1 --max-months 1 --retain-policy reports_only", market, interval, symStr, y, y),
			}
			grp.CleanupCommandsAfterEngine = []string{
				fmt.Sprintf("ak-historian cleanup-workdir --workdir %s --market %s --interval %s --symbols %s --from %s-01 --to %s-12 --force --only-verified-archive --retain-symbols BTCUSDT,ETHUSDT", workdir, market, interval, t, y, y),
			}

			plan.Groups = append(plan.Groups, grp)
			groupID++
		}
	}

	reportsDir := filepath.Join(workdir, "reports")
	os.MkdirAll(reportsDir, 0755)

	jBytes, _ := json.MarshalIndent(plan, "", "  ")
	os.WriteFile(filepath.Join(reportsDir, "phase10_5e_storage_plan.json"), jBytes, 0644)

	md := "# Storage Plan\n\n"
	for _, g := range plan.Groups {
		md += fmt.Sprintf("## Group %d\n", g.GroupID)
		md += fmt.Sprintf("- Symbols: %s\n", strings.Join(g.Symbols, ", "))
		md += fmt.Sprintf("- Year: %s\n", g.Year)
		md += fmt.Sprintf("- Estimated Size: %s\n", g.EstimatedSizeHuman)
		md += fmt.Sprintf("- Fits Budget: %t\n\n", g.FitsBudget)
	}
	os.WriteFile(filepath.Join(reportsDir, "phase10_5e_storage_plan.md"), []byte(md), 0644)

	return nil
}
