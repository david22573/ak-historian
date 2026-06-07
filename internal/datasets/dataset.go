package datasets

type DatasetKind string

const (
	KindSentiment   DatasetKind = "sentiment"
	KindDerivatives DatasetKind = "derivatives"
	KindNews        DatasetKind = "news"
)

type DatasetSpec struct {
	Kind     DatasetKind
	Source   string
	Dataset  string
	Market   string
	Symbol   string
	Scope    string
	Interval string
	Date     string // YYYY-MM
}

type RowStats struct {
	RowCount         int64 `json:"row_count"`
	MinEventTimeMS   int64 `json:"min_event_time_ms"`
	MaxEventTimeMS   int64 `json:"max_event_time_ms"`
	MinAvailableAtMS int64 `json:"min_available_at_ms"`
	MaxAvailableAtMS int64 `json:"max_available_at_ms"`
}
