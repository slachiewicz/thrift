// reference_data_v2.thrift — Reference Data Query service, contract v2.
//
// Part of "Apache Thrift: A Practical Guide" example suite (Apache Thrift 0.24.0).
//
// This file demonstrates SAFE, IN-PLACE EVOLUTION of reference_data.thrift.
// The service keeps its name and namespaces; all changes are additive:
//
//   1. Existing field IDs keep their numbers, types, and requiredness.
//   2. New data arrives as new OPTIONAL fields with previously unused IDs.
//   3. A retired field's ID is explicitly RESERVED in a comment so nobody
//      ever reuses the number for different data.
//   4. New functionality is added as a new method on the SAME service, so
//      v1 clients keep working against a v2 server and vice versa.
//
// The 0.24.0 compiler can mechanically check old-vs-new compatibility:
//
//   thrift --audit reference_data.thrift reference_data_v2.thrift

include "common.thrift"

// Same namespaces as v1: this is the next revision of the same contract,
// not a separate API. (A separate v2 package would fork the contract.)
namespace java com.example.refdata.query
namespace py refdata.query
namespace go refdata.query
namespace netstd RefData.Query
namespace js RefData.Query

typedef common.EpochMillis AsOf
typedef common.CurrencyCode QuoteCurrency

enum AssetClass {
  EQUITY       = 0,
  FIXED_INCOME = 1,
  FUND         = 2,
  FX           = 3,
  COMMODITY    = 4
}

struct Instrument {
  // Unchanged from v1: the identity triple stays required at IDs 1-3.
  1: required uuid instrumentId,
  2: required string symbol,
  3: required string name,
  4: required AssetClass assetClass,
  5: required common.MicCode primaryMic,

  // Unchanged optional fields 6-10.
  6: optional string isin,
  7: optional set<QuoteCurrency> quoteCurrencies,
  8: optional list<string> tags,
  9: optional map<string, string> attributes,
  10: optional common.SnapshotContext context,

  // NEW in v2: additive, optional, previously unused ID.
  11: optional string legalEntityId,

  // Field 12 was `optional string sedol` in an internal draft that never
  // shipped. RESERVED — never assign this number to different data.
  // 12: (reserved, formerly sedol, withdrawn before release)
}

struct InstrumentSelector {
  // Unchanged.
  1: uuid instrumentId,
  2: string symbol,
  3: string isin
}

struct InstrumentQuery {
  1: optional string symbolPrefix,
  2: optional AssetClass assetClass,
  3: optional common.MicCode mic,
  4: optional QuoteCurrency quoteCurrency,

  // CAUTION: the default value 20 is part of the interface — old clients
  // that omit field 5 rely on the receiver applying it. Changing the IDL
  // default silently changes v1 clients' page size once they regenerate
  // against this file. Keep it at 20; consumers wanting more pages set
  // the field explicitly.
  5: i32 limit = 20,

  6: bool includeSnapshots
}

struct QuoteSnapshot {
  1: required uuid instrumentId,
  2: required AsOf asOf,
  3: optional double bid,
  4: optional double ask,
  5: optional double last,
  6: optional i64 volume,
  7: optional common.MarketPhase phase,

  // NEW in v2: additive, optional, previously unused IDs.
  8: optional QuoteCurrency quoteCurrency,
  9: optional double changePercent
}

exception LookupError {
  1: required string reason,
  2: optional uuid instrumentId
}

exception InvalidQuery {
  1: required string reason,
  2: optional list<string> violations
}

// Same service name as v1, with one NEW method appended. v1 servers never
// see the new method; v1 clients never call it. Existing methods are
// untouched, so v1 clients interoperate with v2 servers and vice versa.
service ReferenceDataQuery {

  Instrument getInstrument(
    1: InstrumentSelector selector
  ) throws (1: LookupError lookupError, 2: InvalidQuery invalidQuery),

  list<Instrument> findInstruments(
    1: InstrumentQuery query
  ) throws (1: InvalidQuery invalidQuery),

  QuoteSnapshot latestSnapshot(
    1: uuid instrumentId
  ) throws (1: LookupError lookupError),

  map<string, string> exchangeDirectory(),

  oneway void reportUsage(
    1: string clientApplication
  ),

  // NEW in v2: batch snapshot access. Note the input cap argument:
  // handlers must enforce an upper bound on `instrumentIds.size()` because
  // the wire contract cannot express "too many" — see the production chapter.
  list<QuoteSnapshot> latestSnapshots(
    1: list<uuid> instrumentIds,
    2: i32 maxResults
  ) throws (1: InvalidQuery invalidQuery)
}
