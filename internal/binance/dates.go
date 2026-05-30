package binance

import (
	"fmt"
	"time"
)

func ExpandDates(period, start, end string) ([]string, error) {
	var dates []string
	var layout string

	switch period {
	case "monthly":
		layout = "2006-01"
	case "daily":
		layout = "2006-01-02"
	default:
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	startTime, err := time.Parse(layout, start)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}

	endTime, err := time.Parse(layout, end)
	if err != nil {
		return nil, fmt.Errorf("invalid end date: %w", err)
	}

	if startTime.After(endTime) {
		return nil, fmt.Errorf("start date must be before or equal to end date")
	}

	current := startTime
	for !current.After(endTime) {
		dates = append(dates, current.Format(layout))

		if period == "monthly" {
			current = current.AddDate(0, 1, 0)
		} else {
			current = current.AddDate(0, 0, 1)
		}
	}

	return dates, nil
}
