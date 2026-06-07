package datasets

import (
	"testing"
)

func TestObjectKey_Sentiment(t *testing.T) {
	spec := DatasetSpec{
		Kind:     KindSentiment,
		Source:   "alternative_me",
		Dataset:  "fear_greed",
		Scope:    "global",
		Interval: "1d",
		Date:     "2023-01",
	}

	key, err := ObjectKey(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "datasets/sentiment/source=alternative_me/dataset=fear_greed/scope=global/interval=1d/year=2023/month=01/fear_greed-2023-01.parquet"
	if key != expected {
		t.Errorf("expected %s, got %s", expected, key)
	}
}

func TestManifestKey_Sentiment(t *testing.T) {
	spec := DatasetSpec{
		Kind:     KindSentiment,
		Source:   "alternative_me",
		Dataset:  "fear_greed",
		Scope:    "global",
		Interval: "1d",
	}

	key, err := ManifestKey(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "manifests/datasets/sentiment/alternative_me/fear_greed/scope=global/manifest.json"
	if key != expected {
		t.Errorf("expected %s, got %s", expected, key)
	}
}

func TestPathRejects(t *testing.T) {
	specs := []DatasetSpec{
		{
			Kind:     KindSentiment,
			Source:   "../malicious",
			Dataset:  "fear_greed",
			Scope:    "global",
			Interval: "1d",
			Date:     "2023-01",
		},
		{
			Kind:     KindSentiment,
			Source:   "alternative_me",
			Dataset:  "fear/greed",
			Scope:    "global",
			Interval: "1d",
			Date:     "2023-01",
		},
	}

	for _, spec := range specs {
		_, err := ObjectKey(spec)
		if err == nil {
			t.Errorf("expected error for spec %v", spec)
		}
		_, err = ManifestKey(spec)
		if err == nil {
			t.Errorf("expected error for spec %v", spec)
		}
	}
}
