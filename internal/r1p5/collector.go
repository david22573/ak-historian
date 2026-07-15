package r1p5

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/david22573/ak-historian/internal/prospective"
)

type Collector struct {
	Config Config
	Store  *Store
	Client *prospective.Client
	Clock  prospective.ClockChecker
	Now    func() time.Time
	Delay  time.Duration
}

type BackfillStatus struct {
	SchemaVersion  string            `json:"schema_version"`
	GeneratedAtUTC time.Time         `json:"generated_at_utc"`
	Complete       bool              `json:"complete"`
	Requests       int               `json:"request_count"`
	Candles        int               `json:"candle_count"`
	ChainTerminal  string            `json:"receipt_chain_terminal"`
	Cursors        map[string]Cursor `json:"per_symbol_cursors"`
}

func NewCollector(config Config) *Collector {
	return &Collector{Config: config, Store: NewStore(config.DataRoot, config.Protocol, config.SourceIdentity), Client: prospective.NewClient(), Clock: prospective.SystemClockChecker{}, Now: func() time.Time { return time.Now().UTC() }, Delay: 120 * time.Millisecond}
}

func (c *Collector) Status() (BackfillStatus, error) {
	state, entries, err := c.Store.RebuildState()
	if err != nil {
		return BackfillStatus{}, err
	}
	status := BackfillStatus{SchemaVersion: "ak-historian.pr4b0-r1p5.backfill-status.v1", GeneratedAtUTC: c.Now().UTC(), Complete: true, Requests: len(entries), ChainTerminal: state.ChainTerminal, Cursors: state.Cursors}
	for _, cursor := range state.Cursors {
		status.Candles += cursor.Rows
	}
	for _, symbol := range prospective.UniqueSymbols {
		if state.Cursors[symbol].NextOpenUTC != c.Config.Protocol.BackfillEnds[symbol] {
			status.Complete = false
		}
	}
	return status, nil
}

func (c *Collector) CollectAll(ctx context.Context) (BackfillStatus, error) {
	if c.Store == nil || c.Client == nil || c.Clock == nil || c.Now == nil {
		return BackfillStatus{}, errors.New("backfill dependencies incomplete")
	}
	lock, err := prospective.AcquireLock(c.Store.LockPath())
	if err != nil {
		return BackfillStatus{}, err
	}
	defer lock.Close()
	for _, symbol := range prospective.UniqueSymbols {
		for {
			state, _, err := c.Store.RebuildState()
			if err != nil {
				return BackfillStatus{}, err
			}
			start := state.Cursors[symbol].NextOpenUTC
			endBoundary := c.Config.Protocol.BackfillEnds[symbol]
			if !start.Before(endBoundary) {
				break
			}
			end := start.Add(1000 * time.Minute)
			if end.After(endBoundary) {
				end = endBoundary
			}
			if recovered, err := c.recoverOrphan(state, symbol, start); err != nil {
				return BackfillStatus{}, err
			} else if recovered {
				continue
			}
			if _, err := c.collectPage(ctx, state, symbol, start, end); err != nil {
				return BackfillStatus{}, err
			}
			if c.Delay > 0 {
				select {
				case <-ctx.Done():
					return BackfillStatus{}, ctx.Err()
				case <-time.After(c.Delay):
				}
			}
		}
	}
	return c.Status()
}

func (c *Collector) recoverOrphan(state State, symbol string, start time.Time) (bool, error) {
	relative := receiptPath(symbol, start)
	absolute, err := c.Store.absolute(relative)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(absolute); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	receipt, err := c.Store.readReceipt(relative)
	if err != nil {
		return false, err
	}
	if receipt.PriorReceiptChainHash != state.ChainTerminal {
		return false, errors.New("orphan receipt cannot join current chain")
	}
	if _, err := c.Store.commitLedger(receipt, relative); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Collector) collectPage(ctx context.Context, state State, symbol string, start, end time.Time) (LedgerEntry, error) {
	clock, err := c.Clock.Check(ctx)
	if err != nil {
		return LedgerEntry{}, fmt.Errorf("clock evidence: %w", err)
	}
	providerTime, providerTimeHash, _, err := c.Client.ServerTime(ctx)
	if err != nil {
		return LedgerEntry{}, err
	}
	evidence, params, err := c.Client.KlinesRange(ctx, symbol, start.UnixMilli(), end.Add(-time.Minute).UnixMilli())
	if err != nil {
		return LedgerEntry{}, err
	}
	records, err := prospective.ParseKlines(evidence.Body, symbol, providerTime)
	if err != nil {
		return LedgerEntry{}, err
	}
	expected := int(end.Sub(start) / time.Minute)
	if len(records) != expected {
		return LedgerEntry{}, fmt.Errorf("partial response for %s [%s,%s): got %d want %d", symbol, start, end, len(records), expected)
	}
	for index, record := range records {
		if record.OpenTimeMS != start.Add(time.Duration(index)*time.Minute).UnixMilli() {
			return LedgerEntry{}, fmt.Errorf("non-contiguous response for %s at row %d", symbol, index)
		}
	}
	observed := maxTime(evidence.CompleteResponseReceivedUTC, evidence.ProviderHTTPDateUTC, providerTime)
	acquired := c.Now().UTC()
	if acquired.Before(observed) {
		acquired = observed
	}
	requestID := prospective.HashBytes([]byte(c.Config.Protocol.ProtocolHash + "\n" + symbol + "\n" + start.Format(time.RFC3339) + "\n" + end.Format(time.RFC3339)))
	normalized := make([]NormalizedRecord, len(records))
	for index, record := range records {
		normalized[index] = NormalizedRecord{NormalizedCandle: record, MarketEventTimeUTC: time.UnixMilli(record.OpenTimeMS).UTC(), ProviderCandleCloseTimeUTC: time.UnixMilli(record.CloseTimeMS).UTC(), ObservedAvailableAtUTC: observed, AcquiredAtUTC: acquired, AcquisitionReceiptID: requestID}
	}
	fragment := Fragment{SchemaVersion: FragmentVersion, RequestID: requestID, Symbol: symbol, SourceSchemaVersion: c.Config.Protocol.SourceSchemaVersion, SourceSchemaFingerprint: c.Config.Protocol.SourceSchemaFingerprint, Records: normalized}
	fragment.FragmentHash, err = prospective.HashCanonical(fragment, "fragment_hash")
	if err != nil {
		return LedgerEntry{}, err
	}
	fragmentData, err := prospective.CanonicalJSON(fragment)
	if err != nil {
		return LedgerEntry{}, err
	}
	base := filepath.Join("symbol="+symbol, "start="+start.Format("20060102T150405Z")+".json.gz")
	receipt := Receipt{SchemaVersion: ReceiptVersion, AcquisitionMode: Mode, RequestID: requestID, Symbol: symbol, RequestedStartUTC: start, RequestedEndExclusiveUTC: end, Endpoint: prospective.KlineEndpoint, CanonicalRequestParameters: params, RequestStartUTC: evidence.RequestStartUTC, ResponseHeadersReceivedUTC: evidence.ResponseHeadersReceivedUTC, CompleteResponseReceivedUTC: evidence.CompleteResponseReceivedUTC, ProviderHTTPDate: evidence.ProviderHTTPDate, ProviderHTTPDateUTC: evidence.ProviderHTTPDateUTC, ProviderServerTimeUTC: providerTime, ProviderServerTimeHash: providerTimeHash, ClockEvidence: clock, HTTPStatus: evidence.HTTPStatus, RetryNumber: evidence.RetryNumber, RawByteLength: len(evidence.Body), RawHash: prospective.HashBytes(evidence.Body), RawPath: filepath.ToSlash(filepath.Join("raw", base)), FragmentByteLength: len(fragmentData), FragmentHash: fragment.FragmentHash, FragmentPath: filepath.ToSlash(filepath.Join("fragments", base)), ParsedRowCount: len(records), FirstCandleOpenUTC: start, LastCandleCloseUTC: time.UnixMilli(records[len(records)-1].CloseTimeMS).UTC(), ObservedAvailableAtUTC: observed, AcquiredAtUTC: acquired, PriorReceiptChainHash: state.ChainTerminal, BackfillSourceCommit: c.Config.SourceIdentity.SourceCommit, ProtocolHash: c.Config.Protocol.ProtocolHash, P4CollectorSourceCommit: c.Config.Protocol.P4CollectorSourceCommit, AvailabilityPolicyVersion: c.Config.Protocol.AvailabilityPolicyVersion, AvailabilityPolicyHash: c.Config.Protocol.AvailabilityPolicyHash}
	receipt.ReceiptHash, err = prospective.HashCanonical(receipt, "receipt_hash")
	if err != nil {
		return LedgerEntry{}, err
	}
	return c.Store.Commit(receipt, evidence.Body, fragmentData)
}
