package prospective

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	ApprovedHost  = "fapi.binance.com"
	TimeEndpoint  = "https://fapi.binance.com/fapi/v1/time"
	KlineEndpoint = "https://fapi.binance.com/fapi/v1/klines"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	HTTP        HTTPDoer
	MaxAttempts int
	BaseBackoff time.Duration
	Now         func() time.Time
	Sleep       func(context.Context, time.Duration) error
}

type ResponseEvidence struct {
	RequestStartUTC             time.Time
	ResponseHeadersReceivedUTC  time.Time
	CompleteResponseReceivedUTC time.Time
	HTTPStatus                  int
	RetryNumber                 int
	ProviderHTTPDate            string
	ProviderHTTPDateUTC         time.Time
	Body                        []byte
}

func NewClient() *Client {
	httpClient := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return fmt.Errorf("redirect rejected: %s", request.URL.String())
		},
	}
	return &Client{HTTP: httpClient, MaxAttempts: 3, BaseBackoff: time.Second, Now: func() time.Time { return time.Now().UTC() }, Sleep: sleepContext}
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func validatePublicURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return errors.New("public collector requires HTTPS")
	}
	if parsed.Hostname() != ApprovedHost || parsed.User != nil {
		return fmt.Errorf("unapproved collector host or credentials: %s", parsed.Hostname())
	}
	return nil
}

func (client *Client) get(ctx context.Context, rawURL string) (ResponseEvidence, error) {
	if err := validatePublicURL(rawURL); err != nil {
		return ResponseEvidence{}, err
	}
	if client.HTTP == nil || client.Now == nil || client.Sleep == nil || client.MaxAttempts < 1 || client.MaxAttempts > 5 || client.BaseBackoff < 0 || client.BaseBackoff > 10*time.Second {
		return ResponseEvidence{}, errors.New("invalid bounded HTTP client configuration")
	}
	var lastErr error
	var lastEvidence ResponseEvidence
	for attempt := 0; attempt < client.MaxAttempts; attempt++ {
		started := client.Now().UTC()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return ResponseEvidence{}, err
		}
		request.Header.Set("User-Agent", "ak-historian-prospective/1")
		response, err := client.HTTP.Do(request)
		headersAt := client.Now().UTC()
		if err != nil {
			lastErr = err
		} else {
			body, readErr := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024+1))
			_ = response.Body.Close()
			completeAt := client.Now().UTC()
			if readErr != nil {
				lastErr = readErr
			} else if len(body) > 8*1024*1024 {
				return ResponseEvidence{}, errors.New("provider response exceeds 8 MiB")
			} else {
				dateText := strings.TrimSpace(response.Header.Get("Date"))
				dateUTC, dateErr := http.ParseTime(dateText)
				evidence := ResponseEvidence{RequestStartUTC: started, ResponseHeadersReceivedUTC: headersAt, CompleteResponseReceivedUTC: completeAt, HTTPStatus: response.StatusCode, RetryNumber: attempt, ProviderHTTPDate: dateText, Body: body}
				if dateErr == nil {
					evidence.ProviderHTTPDateUTC = dateUTC.UTC()
				}
				lastEvidence = evidence
				if response.StatusCode == http.StatusOK {
					if dateErr != nil {
						return evidence, fmt.Errorf("provider HTTP Date missing or invalid: %w", dateErr)
					}
					return evidence, nil
				}
				lastErr = fmt.Errorf("provider HTTP status %d", response.StatusCode)
				if response.StatusCode != http.StatusTooManyRequests && response.StatusCode != http.StatusTeapot && response.StatusCode < 500 {
					return evidence, lastErr
				}
			}
		}
		if attempt+1 < client.MaxAttempts {
			backoff := client.BaseBackoff * time.Duration(1<<attempt)
			if err := client.Sleep(ctx, backoff); err != nil {
				return ResponseEvidence{}, err
			}
		}
	}
	return lastEvidence, fmt.Errorf("bounded provider request failed after %d attempts: %w", client.MaxAttempts, lastErr)
}

func (client *Client) ServerTime(ctx context.Context) (time.Time, string, ResponseEvidence, error) {
	evidence, err := client.get(ctx, TimeEndpoint)
	if err != nil {
		return time.Time{}, "", evidence, err
	}
	var payload struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := StrictDecode(evidence.Body, &payload); err != nil {
		return time.Time{}, "", evidence, fmt.Errorf("provider server-time response: %w", err)
	}
	if payload.ServerTime <= 0 {
		return time.Time{}, "", evidence, errors.New("provider server time is invalid")
	}
	return time.UnixMilli(payload.ServerTime).UTC(), HashBytes(evidence.Body), evidence, nil
}

func canonicalKlineParams(symbol string, startTime int64) (string, string, error) {
	if !contains(UniqueSymbols, symbol) {
		return "", "", fmt.Errorf("symbol %s is not in the frozen universe", symbol)
	}
	values := url.Values{}
	values.Set("interval", "1m")
	values.Set("limit", "1000")
	values.Set("symbol", symbol)
	if startTime > 0 {
		values.Set("startTime", strconv.FormatInt(startTime, 10))
	} else {
		values.Set("limit", "5")
	}
	params := values.Encode()
	return KlineEndpoint + "?" + params, params, nil
}

func (client *Client) Klines(ctx context.Context, symbol string, startTime int64) (ResponseEvidence, string, error) {
	endpoint, params, err := canonicalKlineParams(symbol, startTime)
	if err != nil {
		return ResponseEvidence{}, "", err
	}
	evidence, err := client.get(ctx, endpoint)
	return evidence, params, err
}

func ParseKlines(body []byte, symbol string, providerTime time.Time) ([]NormalizedCandle, error) {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	var rows [][]any
	if err := decoder.Decode(&rows); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New("trailing provider JSON")
		}
		return nil, err
	}
	result := make([]NormalizedCandle, 0, len(rows))
	var previous int64
	for index, row := range rows {
		if len(row) != 12 {
			return nil, fmt.Errorf("row %d has unknown field shape %d", index, len(row))
		}
		openTime, err := jsonNumberInt64(row[0])
		if err != nil {
			return nil, fmt.Errorf("row %d open time: %w", index, err)
		}
		closeTime, err := jsonNumberInt64(row[6])
		if err != nil {
			return nil, fmt.Errorf("row %d close time: %w", index, err)
		}
		trades, err := jsonNumberInt64(row[8])
		if err != nil {
			return nil, fmt.Errorf("row %d trade count: %w", index, err)
		}
		if closeTime != openTime+59999 {
			return nil, fmt.Errorf("row %d one-minute close boundary mismatch", index)
		}
		if index > 0 && openTime <= previous {
			return nil, fmt.Errorf("row %d is duplicate or out of order", index)
		}
		previous = openTime
		if closeTime >= providerTime.UnixMilli() {
			continue
		}
		values := make([]string, 0, 8)
		for _, position := range []int{1, 2, 3, 4, 5, 7, 9, 10} {
			value, ok := row[position].(string)
			if !ok || !validDecimal(value) {
				return nil, fmt.Errorf("row %d field %d is malformed", index, position)
			}
			values = append(values, value)
		}
		if ignored, ok := row[11].(string); !ok || !validDecimal(ignored) {
			return nil, fmt.Errorf("row %d ignored field malformed", index)
		}
		opened := time.UnixMilli(openTime).UTC()
		result = append(result, NormalizedCandle{Market: "futures-um", Symbol: symbol, Interval: "1m", Period: "daily", SourceDate: opened.Format("2006-01-02"), OpenTimeMS: openTime, Open: values[0], High: values[1], Low: values[2], Close: values[3], Volume: values[4], CloseTimeMS: closeTime, QuoteAssetVolume: values[5], NumberOfTrades: trades, TakerBuyBaseVolume: values[6], TakerBuyQuoteVolume: values[7]})
	}
	return result, nil
}

func jsonNumberInt64(value any) (int64, error) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, errors.New("not an integer JSON number")
	}
	return number.Int64()
}

func validDecimal(value string) bool {
	if strings.TrimSpace(value) != value || value == "" {
		return false
	}
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
