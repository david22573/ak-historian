package duckdbquery

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// QuoteString quotes a string for inclusion in DuckDB SQL literals by wrapping in single quotes and escaping embedded single quotes.
func QuoteString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// RunQuery executes a duckdb CLI subprocess with the provided query string.
func RunQuery(ctx context.Context, query string, extraArgs ...string) ([]byte, error) {
	args := append([]string{}, extraArgs...)
	args = append(args, "-c", query)
	cmd := exec.CommandContext(ctx, "duckdb", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("duckdb query failed: %w, output: %s", err, string(output))
	}
	return output, nil
}
