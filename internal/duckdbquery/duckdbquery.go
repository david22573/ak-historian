package duckdbquery

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DefaultMemoryLimit is the hard bound applied when no explicit memory limit is specified.
const DefaultMemoryLimit = "512MB"

// QuoteString quotes a string for inclusion in DuckDB SQL literals by wrapping in single quotes and escaping embedded single quotes.
func QuoteString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// MemoryLimit returns the configured or default DuckDB memory limit.
func MemoryLimit() string {
	if lim := os.Getenv("AK_DUCKDB_MEMORY_LIMIT"); lim != "" {
		return lim
	}
	if lim := os.Getenv("DUCKDB_MEMORY_LIMIT"); lim != "" {
		return lim
	}
	return DefaultMemoryLimit
}

// WrapWithPragmas wraps any query with strict memory and resource bounds.
func WrapWithPragmas(query string) string {
	limit := MemoryLimit()
	trimmed := strings.TrimSpace(query)
	// If query already sets memory_limit, avoid duplicating
	if strings.Contains(strings.ToUpper(trimmed), "MEMORY_LIMIT") {
		return trimmed
	}
	return fmt.Sprintf("PRAGMA memory_limit='%s';\nPRAGMA preserve_insertion_order=false;\n%s", limit, trimmed)
}

// RunQuery executes a duckdb CLI subprocess with the provided query string and enforced memory bounds.
func RunQuery(ctx context.Context, query string, extraArgs ...string) ([]byte, error) {
	boundedQuery := WrapWithPragmas(query)
	args := append([]string{}, extraArgs...)
	args = append(args, "-c", boundedQuery)
	cmd := exec.CommandContext(ctx, "duckdb", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("duckdb query failed: %w, output: %s", err, string(output))
	}
	return output, nil
}

