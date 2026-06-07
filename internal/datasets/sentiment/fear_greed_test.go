package sentiment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFearGreedClient_Fetch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock response
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"name": "Fear and Greed Index",
			"data": [
				{
					"value": "40",
					"value_classification": "Fear",
					"timestamp": "1672531200",
					"time_until_update": "12345"
				},
				{
					"value": "20",
					"value_classification": "Extreme Fear",
					"timestamp": "1672444800",
					"time_until_update": "12345"
				}
			]
		}`))
	}))
	defer ts.Close()

	client := NewFearGreedClient()
	client.BaseURL = ts.URL

	start := time.Unix(1672444800, 0)
	end := time.Unix(1672531200, 0)

	rows, err := client.Fetch(context.Background(), start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Check sorting and normalization
	if rows[0].Score != 20 || rows[0].Label != "extreme_fear" || rows[0].EventTimeMS != 1672444800000 {
		t.Errorf("row 0 incorrect: %+v", rows[0])
	}

	if rows[1].Score != 40 || rows[1].Label != "fear" || rows[1].EventTimeMS != 1672531200000 {
		t.Errorf("row 1 incorrect: %+v", rows[1])
	}
}

func TestFearGreedClient_Filter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": [
				{
					"value": "40",
					"value_classification": "Fear",
					"timestamp": "1672531200"
				},
				{
					"value": "20",
					"value_classification": "Extreme Fear",
					"timestamp": "1672444800"
				}
			]
		}`))
	}))
	defer ts.Close()

	client := NewFearGreedClient()
	client.BaseURL = ts.URL

	// only fetch the later one
	start := time.Unix(1672531200, 0)
	end := time.Unix(1672531200, 0)

	rows, err := client.Fetch(context.Background(), start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}
