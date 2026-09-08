// Go walkthrough — Reference Data Query client (Apache Thrift 0.24.0)
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	thrift "github.com/apache/thrift/lib/go/thrift"
	query "example.com/refdata/gen-go/refdata/query"
)

func mustTuuid(s string) thrift.Tuuid {
	return thrift.Must(thrift.ParseTuuid(s))
}

func main() {
	// Layer stack must MIRROR the server: socket -> buffered -> binary.
	transport, err := thrift.NewTSocket("127.0.0.1:9090")
	if err != nil {
		log.Fatalf("socket: %v", err)
	}
	if err := transport.SetConnTimeout(5 * time.Second); err != nil {
		log.Fatalf("conn timeout: %v", err)
	}
	if err := transport.SetSocketTimeout(5 * time.Second); err != nil {
		log.Fatalf("socket timeout: %v", err)
	}
	buffered := thrift.NewTBufferedTransportFactory(8192)
	useTransport, err := buffered.GetTransport(transport)
	if err != nil {
		log.Fatalf("transport: %v", err)
	}
	protocolFactory := thrift.NewTBinaryProtocolFactoryDefault()
	client := query.NewReferenceDataQueryClientFactory(useTransport, protocolFactory)

	if err := useTransport.Open(); err != nil {
		log.Fatalf("open: %v", err)
	}
	defer useTransport.Close()

	ctx := context.Background()

	apple, err := client.GetInstrument(ctx, &query.InstrumentSelector{Symbol: "AAPL"})
	if err != nil {
		log.Fatalf("GetInstrument: %v", err)
	}
	fmt.Println("GetInstrument:", apple.Symbol, apple.Name, apple.PrimaryMic)

	eq := query.AssetClass_EQUITY
	eur := query.QuoteCurrency("EUR")
	matches, err := client.FindInstruments(ctx, &query.InstrumentQuery{
		AssetClass:    &eq,
		QuoteCurrency: &eur,
		Limit:         10,
	})
	if err != nil {
		log.Fatalf("FindInstruments: %v", err)
	}
	fmt.Println("FindInstruments:", len(matches), "match(es)")

	snap, err := client.LatestSnapshot(ctx, mustTuuid("00000000-0000-4000-8000-000000000002"))
	if err != nil {
		log.Fatalf("LatestSnapshot: %v", err)
	}
	fmt.Println("LatestSnapshot last:", *snap.Last)

	dir, err := client.ExchangeDirectory(ctx)
	if err != nil {
		log.Fatalf("ExchangeDirectory: %v", err)
	}
	fmt.Println("ExchangeDirectory XNAS:", dir["XNAS"])

	// oneway: returns as soon as the request is on the wire.
	if err := client.ReportUsage(ctx, "go-walkthrough"); err != nil {
		log.Fatalf("ReportUsage: %v", err)
	}

	// Typed exception on the happy-path contract.
	if _, err := client.GetInstrument(ctx, &query.InstrumentSelector{Symbol: "NOPE"}); err != nil {
		if lookup, ok := err.(*query.LookupError); ok {
			fmt.Println("LookupError as expected:", lookup.Reason)
		} else {
			log.Fatalf("unexpected error type: %v", err)
		}
	}
}
