package sentiment

type Row struct {
	Source   string `json:"source"`
	Dataset  string `json:"dataset"`
	Scope    string `json:"scope"`
	Symbol   string `json:"symbol,omitempty"`
	Interval string `json:"interval"`

	EventTimeMS   int64 `json:"event_time_ms"`
	AvailableAtMS int64 `json:"available_at_ms"`
	IngestedAtMS  int64 `json:"ingested_at_ms"`

	Score     float64 `json:"score"`
	Label     string  `json:"label"`
	Intensity float64 `json:"intensity"`

	Mentions int64 `json:"mentions,omitempty"`
	Positive int64 `json:"positive,omitempty"`
	Negative int64 `json:"negative,omitempty"`
	Neutral  int64 `json:"neutral,omitempty"`

	SourceVersion string `json:"source_version"`
}
