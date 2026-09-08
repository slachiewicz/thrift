// Go walkthrough — Reference Data Query server (Apache Thrift 0.24.0)
//
// Layer stack: TServerSocket -> buffered transport -> binary protocol,
// served by Go's single concrete server type, TSimpleServer.
package main

import (
	"context"
	"fmt"
	"log"

	thrift "github.com/apache/thrift/lib/go/thrift"
	query "example.com/refdata/gen-go/refdata/query"
	common "example.com/refdata/gen-go/refdata/common"
)

const nowMs = 1767000000000 // fixed as-of for reproducible output

func mustTuuid(s string) thrift.Tuuid {
	return thrift.Must(thrift.ParseTuuid(s))
}

func newInstrument(id, symbol, name, isin, mic string, currencies []string) *query.Instrument {
	phase := common.MarketPhase_OPEN
	return &query.Instrument{
		InstrumentId:    mustTuuid(id),
		Symbol:          symbol,
		Name:            name,
		AssetClass:      query.AssetClass_EQUITY,
		PrimaryMic:      common.MicCode(mic),
		Isin:            &isin,
		QuoteCurrencies: toQuoteCurrencies(currencies),
		Context: &common.SnapshotContext{
			AsOf:          nowMs,
			SourceId:      strPtr("walkthrough-static"),
			Phase:         &phase,
			QuoteCurrency: currencyPtr("USD"),
		},
	}
}

func strPtr(s string) *string { return &s }

func currencyPtr(s string) *common.CurrencyCode {
	c := common.CurrencyCode(s)
	return &c
}

func toQuoteCurrencies(list []string) []query.QuoteCurrency {
	out := make([]query.QuoteCurrency, 0, len(list))
	for _, s := range list {
		out = append(out, query.QuoteCurrency(s))
	}
	return out
}

type handler struct {
	instruments map[thrift.Tuuid]*query.Instrument
}

func (h *handler) GetInstrument(ctx context.Context, selector *query.InstrumentSelector) (*query.Instrument, error) {
	// Union convention: exactly one member set. Generated Go code writes any
	// non-zero fields and does not enforce the one-member rule, so the
	// handler must validate.
	var zeroTuuid thrift.Tuuid
	set := 0
	if selector.InstrumentId != zeroTuuid {
		set++
	}
	if selector.Symbol != "" {
		set++
	}
	if selector.Isin != "" {
		set++
	}
	if set != 1 {
		return nil, &query.InvalidQuery{Reason: "selector must set exactly one field"}
	}
	if selector.Symbol != "" {
		for _, instr := range h.instruments {
			if instr.Symbol == selector.Symbol {
				return instr, nil
			}
		}
		return nil, &query.LookupError{Reason: fmt.Sprintf("unknown symbol: %s", selector.Symbol)}
	}
	if instr, ok := h.instruments[selector.InstrumentId]; ok {
		return instr, nil
	}
	return nil, &query.LookupError{Reason: "unknown instrumentId", InstrumentId: &selector.InstrumentId}
}

func (h *handler) FindInstruments(ctx context.Context, q *query.InstrumentQuery) ([]*query.Instrument, error) {
	limit := q.Limit
	if limit == 0 {
		limit = common.DEFAULT_PAGE_SIZE
	}
	if limit > common.MAX_PAGE_SIZE {
		return nil, &query.InvalidQuery{
			Reason:     "limit exceeds maximum",
			Violations: []string{fmt.Sprintf("limit>%d", common.MAX_PAGE_SIZE)},
		}
	}
	out := make([]*query.Instrument, 0, limit)
	for _, instr := range h.instruments {
		if q.SymbolPrefix != nil && !startsWith(instr.Symbol, *q.SymbolPrefix) {
			continue
		}
		if q.AssetClass != nil && instr.AssetClass != *q.AssetClass {
			continue
		}
		if q.Mic != nil && instr.PrimaryMic != *q.Mic {
			continue
		}
		if q.QuoteCurrency != nil && !containsQuoteCurrency(instr.QuoteCurrencies, *q.QuoteCurrency) {
			continue
		}
		out = append(out, instr)
		if len(out) >= int(limit) {
			break
		}
	}
	return out, nil
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func containsQuoteCurrency(list []query.QuoteCurrency, want query.QuoteCurrency) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func (h *handler) LatestSnapshot(ctx context.Context, instrumentId thrift.Tuuid) (*query.QuoteSnapshot, error) {
	if _, ok := h.instruments[instrumentId]; !ok {
		return nil, &query.LookupError{Reason: "unknown instrumentId", InstrumentId: &instrumentId}
	}
	seed := float64(instrumentId[len(instrumentId)-1])
	phase := common.MarketPhase_OPEN
	return &query.QuoteSnapshot{
		InstrumentId: instrumentId,
		AsOf:         nowMs,
		Bid:          float64Ptr(100.0 + seed - 0.25),
		Ask:          float64Ptr(100.0 + seed + 0.25),
		Last:         float64Ptr(100.0 + seed),
		Volume:       int64Ptr(10000 + int64(100*seed)),
		Phase:        &phase,
	}, nil
}

func float64Ptr(f float64) *float64 { return &f }
func int64Ptr(i int64) *int64       { return &i }

func (h *handler) ExchangeDirectory(ctx context.Context) (map[string]string, error) {
	return map[string]string{
		"XNAS": "NASDAQ Stock Market",
		"XETR": "Deutsche Boerse Xetra",
		"XNYS": "New York Stock Exchange",
	}, nil
}

func (h *handler) ReportUsage(ctx context.Context, clientApplication string) error {
	// oneway: no reply, no error path back to the caller.
	log.Printf("usage report from %s", clientApplication)
	return nil
}

func main() {
	h := &handler{instruments: map[thrift.Tuuid]*query.Instrument{
		mustTuuid("00000000-0000-4000-8000-000000000001"): newInstrument(
			"00000000-0000-4000-8000-000000000001", "AAPL", "Apple Inc.", "US0378331005", "XNAS", []string{"USD"}),
		mustTuuid("00000000-0000-4000-8000-000000000002"): newInstrument(
			"00000000-0000-4000-8000-000000000002", "MSFT", "Microsoft Corporation", "US5949181045", "XNAS", []string{"USD"}),
		mustTuuid("00000000-0000-4000-8000-000000000003"): newInstrument(
			"00000000-0000-4000-8000-000000000003", "DTE", "Deutsche Telekom AG", "DE0005557508", "XETR", []string{"EUR"}),
	}}

	processor := query.NewReferenceDataQueryProcessor(h)
	transport, err := thrift.NewTServerSocket("127.0.0.1:9090")
	if err != nil {
		log.Fatalf("server socket: %v", err)
	}
	server := thrift.NewTSimpleServer4(
		processor,
		transport,
		thrift.NewTBufferedTransportFactory(8192),
		thrift.NewTBinaryProtocolFactoryDefault(),
	)
	log.Println("ReferenceDataQuery listening on 127.0.0.1:9090")
	if err := server.Serve(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
