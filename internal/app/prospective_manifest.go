package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/david22573/ak-historian/internal/archiveauthority"
	"github.com/spf13/cobra"
)

func newValidateProspectiveManifestCommand() *cobra.Command {
	var path string
	command := &cobra.Command{Use: "validate-prospective-manifest", Short: "Fail-closed validation of an immutable prospective manifest", RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		var manifest archiveauthority.ProspectiveManifest
		if err := decoder.Decode(&manifest); err != nil {
			return err
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			if err == nil {
				return fmt.Errorf("manifest contains trailing JSON")
			}
			return err
		}
		if err := archiveauthority.VerifyProspectiveManifest(manifest); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "PROSPECTIVE_MANIFEST_VALID")
		return err
	}}
	command.Flags().StringVar(&path, "manifest", "", "prospective manifest JSON")
	_ = command.MarkFlagRequired("manifest")
	return command
}

func init() { rootCmd.AddCommand(newValidateProspectiveManifestCommand()) }
