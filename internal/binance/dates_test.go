package binance

import (
	"reflect"
	"testing"
)

func TestExpandDates(t *testing.T) {
	tests := []struct {
		name    string
		period  string
		start   string
		end     string
		want    []string
		wantErr bool
	}{
		{
			name:   "monthly inclusive",
			period: "monthly",
			start:  "2024-01",
			end:    "2024-03",
			want:   []string{"2024-01", "2024-02", "2024-03"},
		},
		{
			name:   "daily inclusive",
			period: "daily",
			start:  "2024-01-01",
			end:    "2024-01-03",
			want:   []string{"2024-01-01", "2024-01-02", "2024-01-03"},
		},
		{
			name:   "single day",
			period: "daily",
			start:  "2024-01-01",
			end:    "2024-01-01",
			want:   []string{"2024-01-01"},
		},
		{
			name:    "invalid format monthly",
			period:  "monthly",
			start:   "2024-01-01",
			end:     "2024-03-01",
			wantErr: true,
		},
		{
			name:    "start after end",
			period:  "monthly",
			start:   "2024-03",
			end:     "2024-01",
			wantErr: true,
		},
		{
			name:   "leap year",
			period: "daily",
			start:  "2024-02-28",
			end:    "2024-03-01",
			want:   []string{"2024-02-28", "2024-02-29", "2024-03-01"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandDates(tt.period, tt.start, tt.end)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExpandDates() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExpandDates() got = %v, want %v", got, tt.want)
			}
		})
	}
}
