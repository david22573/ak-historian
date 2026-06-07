package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type FearGreedClient struct {
	HTTPClient *http.Client
	BaseURL    string
}

func NewFearGreedClient() *FearGreedClient {
	return &FearGreedClient{
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type rawFearGreedResponse struct {
	Name string               `json:"name"`
	Data []rawFearGreedRecord `json:"data"`
}

type rawFearGreedRecord struct {
	Value               string `json:"value"`
	ValueClassification string `json:"value_classification"`
	Timestamp           string `json:"timestamp"`
}

func (c *FearGreedClient) Fetch(ctx context.Context, start, end time.Time) ([]Row, error) {
	url := c.BaseURL
	if url == "" {
		url = "https://api.alternative.me/fng/?limit=0&format=json"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp rawFearGreedResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode JSON response: %w", err)
	}

	ingestedAt := time.Now().UnixMilli()
	var rows []Row
	seen := make(map[int64]bool)

	for _, rec := range apiResp.Data {
		tsSec, err := strconv.ParseInt(rec.Timestamp, 10, 64)
		if err != nil {
			continue // Skip malformed timestamps
		}
		eventTimeMS := tsSec * 1000
		eventTime := time.Unix(tsSec, 0)

		// Filter by start and end inclusive
		// Start date check (00:00:00 of start date)
		if eventTime.Before(start) && !eventTime.Equal(start) {
			continue
		}
		if eventTime.After(end) && !eventTime.Equal(end) {
			continue
		}

		// De-duplicate by EventTimeMS
		if seen[eventTimeMS] {
			continue
		}
		seen[eventTimeMS] = true

		score, err := strconv.ParseFloat(rec.Value, 64)
		if err != nil || score < 0 || score > 100 {
			continue // Skip invalid scores
		}

		intensity := math.Abs(score-50.0) / 50.0

		// Normalize label to snake_case
		label := strings.ToLower(strings.TrimSpace(rec.ValueClassification))
		label = strings.ReplaceAll(label, " ", "_")

		rows = append(rows, Row{
			Source:        "alternative_me",
			Dataset:       "fear_greed",
			Scope:         "global",
			Interval:      "1d",
			EventTimeMS:   eventTimeMS,
			AvailableAtMS: eventTimeMS + (24 * 3600 * 1000), // EventTimeMS + 24h
			IngestedAtMS:  ingestedAt,
			Score:         score,
			Label:         label,
			Intensity:     intensity,
			SourceVersion: "alternative_me_fng_v1",
		})
	}

	// Sort rows by EventTimeMS
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].EventTimeMS < rows[j].EventTimeMS
	})

	return rows, nil
}
