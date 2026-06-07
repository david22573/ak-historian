package derivatives

import (
	"reflect"
	"testing"
)

func TestDerivativesRowTimingTags(t *testing.T) {
	rowType := reflect.TypeOf(DerivativesRow{})
	for field, want := range map[string]string{
		"EventTimeMS":   "event_time_ms",
		"AvailableAtMS": "available_at_ms",
		"IngestedAtMS":  "ingested_at_ms",
	} {
		sf, ok := rowType.FieldByName(field)
		if !ok {
			t.Fatalf("missing field %s", field)
		}
		if got := sf.Tag.Get("json"); got != want {
			t.Fatalf("%s tag = %q, want %q", field, got, want)
		}
	}
}
