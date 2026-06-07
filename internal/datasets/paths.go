package datasets

import (
	"fmt"
	"regexp"
	"strings"
)

var dateRegex = regexp.MustCompile(`^\d{4}-\d{2}$`)

func ObjectKey(spec DatasetSpec) (string, error) {
	if err := validateSpec(spec); err != nil {
		return "", err
	}

	if !dateRegex.MatchString(spec.Date) {
		return "", fmt.Errorf("invalid date format: %q (expected YYYY-MM)", spec.Date)
	}

	year := spec.Date[:4]
	month := spec.Date[5:7]

	switch spec.Kind {
	case KindSentiment:
		if !isSafeID(spec.Scope) {
			return "", fmt.Errorf("invalid scope: %q", spec.Scope)
		}
		return fmt.Sprintf(
			"datasets/sentiment/source=%s/dataset=%s/scope=%s/interval=%s/year=%s/month=%s/%s-%s.parquet",
			spec.Source, spec.Dataset, spec.Scope, spec.Interval, year, month, spec.Dataset, spec.Date,
		), nil

	case KindDerivatives:
		if !isSafeID(spec.Market) {
			return "", fmt.Errorf("invalid market: %q", spec.Market)
		}
		if !isSafeID(spec.Symbol) {
			return "", fmt.Errorf("invalid symbol: %q", spec.Symbol)
		}
		return fmt.Sprintf(
			"datasets/derivatives/source=%s/dataset=%s/market=%s/symbol=%s/interval=%s/year=%s/month=%s/%s-%s-%s.parquet",
			spec.Source, spec.Dataset, spec.Market, spec.Symbol, spec.Interval, year, month, spec.Symbol, spec.Dataset, spec.Date,
		), nil

	default:
		return "", fmt.Errorf("unsupported dataset kind: %q", spec.Kind)
	}
}

func ManifestKey(spec DatasetSpec) (string, error) {
	if spec.Source == "" {
		return "", fmt.Errorf("empty source")
	}
	if !isSafeID(spec.Source) {
		return "", fmt.Errorf("invalid source: %q", spec.Source)
	}
	if spec.Dataset == "" {
		return "", fmt.Errorf("empty dataset")
	}
	if !isSafeID(spec.Dataset) {
		return "", fmt.Errorf("invalid dataset: %q", spec.Dataset)
	}

	switch spec.Kind {
	case KindSentiment:
		if !isSafeID(spec.Scope) {
			return "", fmt.Errorf("invalid scope: %q", spec.Scope)
		}
		return fmt.Sprintf(
			"manifests/datasets/sentiment/%s/%s/scope=%s/manifest.json",
			spec.Source, spec.Dataset, spec.Scope,
		), nil

	case KindDerivatives:
		if !isSafeID(spec.Market) {
			return "", fmt.Errorf("invalid market: %q", spec.Market)
		}
		if !isSafeID(spec.Symbol) {
			return "", fmt.Errorf("invalid symbol: %q", spec.Symbol)
		}
		return fmt.Sprintf(
			"manifests/datasets/derivatives/%s/%s/market=%s/symbol=%s/interval=%s/manifest.json",
			spec.Source, spec.Dataset, spec.Market, spec.Symbol, spec.Interval,
		), nil

	default:
		return "", fmt.Errorf("unsupported dataset kind: %q", spec.Kind)
	}
}

func validateSpec(spec DatasetSpec) error {
	if spec.Source == "" {
		return fmt.Errorf("empty source")
	}
	if !isSafeID(spec.Source) {
		return fmt.Errorf("invalid source: %q", spec.Source)
	}
	if spec.Dataset == "" {
		return fmt.Errorf("empty dataset")
	}
	if !isSafeID(spec.Dataset) {
		return fmt.Errorf("invalid dataset: %q", spec.Dataset)
	}
	if spec.Interval == "" {
		return fmt.Errorf("empty interval")
	}
	if !isSafeID(spec.Interval) {
		return fmt.Errorf("invalid interval: %q", spec.Interval)
	}
	return nil
}

func isSafeID(s string) bool {
	if s == "" {
		return false
	}
	if strings.Contains(s, "..") || strings.Contains(s, "/") || strings.Contains(s, "\\") {
		return false
	}
	return true
}
