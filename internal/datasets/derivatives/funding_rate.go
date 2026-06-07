package derivatives

import (
	"context"
	"time"
)

type FundingRateClient interface {
	FetchFundingRates(ctx context.Context, symbol string, start, end time.Time) ([]Row, error)
}
