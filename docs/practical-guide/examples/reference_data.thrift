// reference_data.thrift — Reference Data Query service, contract v1.
//
// Part of "Apache Thrift: A Practical Guide" example suite (Apache Thrift 0.24.0).
//
// FIELD IDs ARE PERMANENT WIRE IDENTIFIERS.
// Once a field ID ships to consumers it must never be reused for a different
// field, and never renumbered. Retired IDs are explicitly reserved in
// comments (see reference_data_v2.thrift for the evolution example).

include "common.thrift"

namespace java com.example.refdata.query
namespace py refdata.query
namespace go refdata.query
namespace netstd RefData.Query
namespace js RefData.Query

// ---------------------------------------------------------------------------
// Typedefs built on the included vocabulary.
//
// Symbols from an included file are reached through the include name
// (common.EpochMillis), and may be re-aliased locally.
// ---------------------------------------------------------------------------

typedef common.EpochMillis AsOf
typedef common.CurrencyCode QuoteCurrency

// ---------------------------------------------------------------------------
// Domain model.
// ---------------------------------------------------------------------------

enum AssetClass {
  EQUITY       = 0,
  FIXED_INCOME = 1,
  FUND         = 2,
  FX           = 3,
  COMMODITY    = 4
}

struct Instrument {
  // 1-3 are the identity triple: they are required and will never move.
  // uuid is a 0.24.0 base type; every target we support here handles it
  // natively (verify per-target support before adding new consumers).
  1: required uuid instrumentId,

  2: required string symbol,

  3: required string name,

  // 4-5 classify the instrument. Required: always known at listing time.
  4: required AssetClass assetClass,

  5: required common.MicCode primaryMic,

  // 6-9 are advisory attributes: optional, so they can be added by some
  // providers, populated lazily, and deprecated later without a flag day.
  6: optional string isin,

  7: optional set<QuoteCurrency> quoteCurrencies,

  8: optional list<string> tags,

  9: optional map<string, string> attributes,

  // 10 groups provenance in a nested struct so provenance fields can evolve
  // inside SnapshotContext without touching Instrument's ID space.
  10: optional common.SnapshotContext context
}

// A union carries EXACTLY ONE of its members. Union members are implicitly
// optional; never declare them `required` (the compiler rejects that).
struct InstrumentSelector {
  1: uuid instrumentId,
  2: string symbol,
  3: string isin
}

struct InstrumentQuery {
  // All filters are optional — an empty query means "match everything",
  // which the handler must cap via `limit`.
  1: optional string symbolPrefix,

  2: optional AssetClass assetClass,

  3: optional common.MicCode mic,

  4: optional QuoteCurrency quoteCurrency,

  // Default requiredness WITH an explicit default value.
  // The default is applied by receivers when the field is absent, which
  // makes this value part of the interface contract: changing 20 to 50
  // later changes behavior for old clients that omit the field.
  5: i32 limit = 20,

  // Default requiredness without a default value: written when true,
  // tolerated when absent, read as false by zero-initializing receivers.
  6: bool includeSnapshots
}

struct QuoteSnapshot {
  1: required uuid instrumentId,

  2: required AsOf asOf,

  // Price fields are optional: for illiquid instruments the provider may
  // legitimately have no bid/ask/last, and `0.0` would be wrong.
  3: optional double bid,

  4: optional double ask,

  5: optional double last,

  6: optional i64 volume,

  7: optional common.MarketPhase phase
}

// ---------------------------------------------------------------------------
// Exceptions.
//
// Exceptions are structs wired through the RPC layer: declaring one in
// `throws` gives it a dedicated message type ID, and generated clients
// raise/return it as a typed error. Do not use them as control flow for
// expected outcomes; use them for genuinely exceptional results.
// Field name note: avoid the name "message" — it collides visually with the
// base-exception message in several languages.
// ---------------------------------------------------------------------------

exception LookupError {
  1: required string reason,

  // Present when the failure is about a specific instrument.
  2: optional uuid instrumentId
}

exception InvalidQuery {
  1: required string reason,

  // Machine-readable per-constraint violations, e.g. ["limit>500"].
  2: optional list<string> violations
}

// ---------------------------------------------------------------------------
// Service.
// ---------------------------------------------------------------------------

service ReferenceDataQuery {

  // Normal request/reply. `throws` declarations are part of the contract:
  // clients can catch LookupError / InvalidQuery as typed exceptions.
  Instrument getInstrument(
    1: InstrumentSelector selector
  ) throws (1: LookupError lookupError, 2: InvalidQuery invalidQuery),

  list<Instrument> findInstruments(
    1: InstrumentQuery query
  ) throws (1: InvalidQuery invalidQuery),

  QuoteSnapshot latestSnapshot(
    1: uuid instrumentId
  ) throws (1: LookupError lookupError),

  // MIC -> human-readable market name, e.g. "XNYS" -> "New York Stock Exchange".
  map<string, string> exchangeDirectory(),

  // ONEWAY — fire-and-forget, with caveats (guide, "oneway" section):
  //  - no response is ever sent; the client continues without confirmation;
  //  - no delivery or ordering guarantee across implementations;
  //  - the handler MUST NOT throw meaningful errors back to the caller;
  //  - only valid because usage reports are loss-tolerant telemetry.
  // If you need "at least once" or "processed" semantics, this is NOT oneway.
  oneway void reportUsage(
    1: string clientApplication
  )
}
