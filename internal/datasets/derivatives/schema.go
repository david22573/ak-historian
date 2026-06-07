package derivatives

type DerivativesRow struct {
	Source   string `json:"source"`
	Dataset  string `json:"dataset"`
	Market   string `json:"market"`
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`

	EventTimeMS   int64 `json:"event_time_ms"`
	AvailableAtMS int64 `json:"available_at_ms"`
	IngestedAtMS  int64 `json:"ingested_at_ms"`

	Value  float64 `json:"value"`
	Extra1 float64 `json:"extra_1,omitempty"`
	Extra2 float64 `json:"extra_2,omitempty"`

	SourceVersion string `json:"source_version"`
}

type Row = DerivativesRow
