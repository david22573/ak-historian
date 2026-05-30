package binance

import (
	"testing"
)

func TestBuildArchiveURL(t *testing.T) {
	tests := []struct {
		name    string
		spec    ArchiveSpec
		want    string
		wantErr bool
	}{
		{
			name: "futures-um monthly",
			spec: ArchiveSpec{
				Market:   "futures-um",
				Symbol:   "BTCUSDT",
				Interval: "1m",
				Period:   "monthly",
				Date:     "2024-01",
			},
			want: "https://data.binance.vision/data/futures/um/monthly/klines/BTCUSDT/1m/BTCUSDT-1m-2024-01.zip",
		},
		{
			name: "futures-um daily",
			spec: ArchiveSpec{
				Market:   "futures-um",
				Symbol:   "BTCUSDT",
				Interval: "1m",
				Period:   "daily",
				Date:     "2024-01-01",
			},
			want: "https://data.binance.vision/data/futures/um/daily/klines/BTCUSDT/1m/BTCUSDT-1m-2024-01-01.zip",
		},
		{
			name: "spot monthly",
			spec: ArchiveSpec{
				Market:   "spot",
				Symbol:   "ETHUSDT",
				Interval: "1h",
				Period:   "monthly",
				Date:     "2023-12",
			},
			want: "https://data.binance.vision/data/spot/monthly/klines/ETHUSDT/1h/ETHUSDT-1h-2023-12.zip",
		},
		{
			name: "invalid market",
			spec: ArchiveSpec{
				Market: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildArchiveURL(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildArchiveURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("BuildArchiveURL() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObjectKey(t *testing.T) {
	tests := []struct {
		name    string
		spec    ArchiveSpec
		want    string
		wantErr bool
	}{
		{
			name: "monthly key",
			spec: ArchiveSpec{
				Market:   "futures-um",
				Symbol:   "BTCUSDT",
				Interval: "1m",
				Period:   "monthly",
				Date:     "2024-01",
			},
			want: "candles/futures-um/1m/symbol=BTCUSDT/year=2024/month=01/BTCUSDT-1m-2024-01.parquet",
		},
		{
			name: "daily key",
			spec: ArchiveSpec{
				Market:   "spot",
				Symbol:   "ETHUSDT",
				Interval: "1h",
				Period:   "daily",
				Date:     "2024-01-05",
			},
			want: "candles/spot/1h/symbol=ETHUSDT/year=2024/month=01/ETHUSDT-1h-2024-01-05.parquet",
		},
		{
			name: "lowercase symbol normalization",
			spec: ArchiveSpec{
				Market:   "futures-um",
				Symbol:   "btcusdt",
				Interval: "1m",
				Period:   "monthly",
				Date:     "2024-01",
			},
			want: "candles/futures-um/1m/symbol=BTCUSDT/year=2024/month=01/BTCUSDT-1m-2024-01.parquet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ObjectKey(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ObjectKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ObjectKey() got = %v, want %v", got, tt.want)
			}
		})
	}
}
