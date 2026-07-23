package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/config"
	"github.com/david22573/ak-historian/internal/storage"
	"github.com/david22573/ak-historian/internal/workdir"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(verifyArchiveCmd)
	verifyArchiveCmd.Flags().String("workdir", ".ak-historian/work", "working directory")
	verifyArchiveCmd.Flags().String("source", "r2", "storage source")
	verifyArchiveCmd.Flags().String("market", "futures-um", "market type")
	verifyArchiveCmd.Flags().String("interval", "1m", "interval")
	verifyArchiveCmd.Flags().String("symbols", "", "comma separated symbols")
	verifyArchiveCmd.Flags().String("from", "", "from YYYY-MM")
	verifyArchiveCmd.Flags().String("to", "", "to YYYY-MM")
}

var verifyArchiveCmd = &cobra.Command{
	Use:   "verify-archive",
	Short: "Verify archive against source",
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, _ := cmd.Flags().GetString("workdir")
		source, _ := cmd.Flags().GetString("source")
		market, _ := cmd.Flags().GetString("market")
		interval, _ := cmd.Flags().GetString("interval")
		symbolsStr, _ := cmd.Flags().GetString("symbols")
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")

		return runVerifyArchive(wd, source, market, interval, symbolsStr, from, to)
	},
}

func runVerifyArchive(wd, source, market, interval, symbolsStr, from, to string) error {
	absWd, err := filepath.Abs(wd)
	if err != nil {
		return err
	}

	if source != "r2" {
		fmt.Printf("archive_verification_status: unavailable\nreason: unsupported source %s\n", source)
		return nil
	}

	cfg, err := config.LoadR2Config()
	if err != nil {
		// Missing config
		fmt.Printf("archive_verification_status: unavailable\nreason: missing R2 config\n")
		return nil
	}

	r2, err := storage.NewR2Client(context.Background(), cfg)
	if err != nil {
		fmt.Printf("archive_verification_status: unavailable\nreason: failed to initialize R2 client (%s)\n", err.Error())
		return nil
	}

	manifest, err := workdir.LoadLocalSourceManifest(absWd)
	if err != nil {
		return fmt.Errorf("failed to load local manifest: %w", err)
	}

	targets := strings.Split(symbolsStr, ",")
	if symbolsStr == "" {
		targets = []string{}
	}

	fromYear := ""
	toYear := ""
	if len(from) >= 4 {
		fromYear = from[:4]
	}
	if len(to) >= 4 {
		toYear = to[:4]
	}

	verifiedCount := 0

	err = filepath.Walk(absWd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".parquet") {
			return nil
		}

		rel, err := filepath.Rel(absWd, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil
		}

		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) == 0 || parts[0] != "candles" {
			return nil
		}

		var sym, yr, mo string
		for _, p := range parts {
			if strings.HasPrefix(p, "symbol=") {
				sym = strings.TrimPrefix(p, "symbol=")
			}
			if strings.HasPrefix(p, "year=") {
				yr = strings.TrimPrefix(p, "year=")
			}
			if strings.HasPrefix(p, "month=") {
				mo = strings.TrimPrefix(p, "month=")
			}
		}

		if len(targets) > 0 {
			match := false
			for _, t := range targets {
				if t == sym {
					match = true
					break
				}
			}
			if !match {
				return nil
			}
		}

		if fromYear != "" && yr < fromYear {
			return nil
		}
		if toYear != "" && yr > toYear {
			return nil
		}

		// The key in R2 is expected to be `candles/{market}/{interval}/symbol={SYMBOL}/year={YYYY}/month={MM}/{file}.parquet`
		// Wait, `rel` is already exactly this structure if we use filepath.ToSlash
		r2Key := filepath.ToSlash(rel)

		// Let's verify via r2 client.
		// Actually, we could just do a HeadObject
		exists, err := r2.ObjectExists(context.Background(), r2Key)
		if err == nil && exists {
			// Update manifest
			obj, ok := manifest.Objects[rel]
			if !ok {
				obj = &workdir.LocalParquetObj{
					Market:    market,
					Interval:  interval,
					Symbol:    sym,
					Year:      yr,
					Month:     mo,
					Path:      rel,
					SizeBytes: info.Size(),
				}
				manifest.Objects[rel] = obj
			}
			obj.ArchivedStatus = workdir.ArchivedStatusVerifiedArchive
			obj.ArchiveLocation = "r2://" + r2Key
			obj.LastVerifiedAt = time.Now()
			verifiedCount++
		} else {
			// Mark as unverified or local only
			obj, ok := manifest.Objects[rel]
			if ok && obj.ArchivedStatus == workdir.ArchivedStatusVerifiedArchive {
				// Downgrade
				obj.ArchivedStatus = workdir.ArchivedStatusUnknown
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	err = workdir.SaveLocalSourceManifest(absWd, manifest)
	if err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	fmt.Printf("archive_verification_status: complete\nverified_objects: %d\n", verifiedCount)

	return nil
}
