// common.thrift — shared vocabulary for the Reference Data example.
//
// Part of "Apache Thrift: A Practical Guide" example suite (Apache Thrift 0.24.0).
// Keep this file small and stable: every service and consumer includes it,
// so changes here propagate to the entire contract surface.

namespace java com.example.refdata.common
namespace py refdata.common
namespace go refdata.common
namespace netstd RefData.Common
namespace js RefData

// ---------------------------------------------------------------------------
// Typedefs: intent-revealing aliases for primitive types.
//
// A typedef is erased at the wire level: it is pure IDL documentation and
// target-language naming. Choose typedef names that are legal identifiers in
// all consumer languages (avoid reserved words such as "type", "class", "def").
//
// Timestamps: Thrift has no timestamp type. Pick ONE convention per contract
// family and encode it in the name. "EpochMillis" states both the epoch and
// the unit, which prevents the classic seconds-vs-millis integration bug.
// ---------------------------------------------------------------------------

typedef i64 EpochMillis
typedef string CurrencyCode   // ISO 4217 alphabetic code, e.g. "USD"
typedef string MicCode        // ISO 10383 market identifier code, e.g. "XNYS"

// ---------------------------------------------------------------------------
// Enumerations.
//
// Thrift enums are i32 on the wire. Assign explicit values to every member so
// that inserting a member never shifts another member's wire value.
// Reserve 0 for a well-defined "default" state, because zero-initialized
// receivers will see 0 for any unset enum field.
// ---------------------------------------------------------------------------

enum MarketPhase {
  CLOSED     = 0,
  PRE_OPEN   = 1,
  OPEN       = 2,
  AUCTION    = 3,
  POST_CLOSE = 4
}

// ---------------------------------------------------------------------------
// Structs.
//
// Requiredness choices (see the compatibility chapter of the guide):
//  - required : field must be present on read. Use ONLY for fields that are
//               part of the identity of the message and will never go away.
//  - optional : written only when set; safe to add and deprecate.
//  - default  (no keyword): written when non-default, tolerated when absent.
// ---------------------------------------------------------------------------

struct SnapshotContext {
  // Identity of a snapshot: when it was taken and from where.
  // Required: every snapshot has an as-of time by definition.
  1: required EpochMillis asOf,

  // Optional: the feeding source is advisory metadata, not identity.
  2: optional string sourceId,

  // Optional: absent when the market state is unknown to the provider.
  3: optional MarketPhase phase,

  // Optional: providers added later can omit this without breaking readers.
  4: optional CurrencyCode quoteCurrency
}

// ---------------------------------------------------------------------------
// Constants.
//
// Constants are materialized in generated code (e.g. a *Constants class) but
// are NOT transmitted on the wire. Treat them as shared literals for
// documentation and validation, not as a configuration distribution channel:
// changing a constant requires re-generating and re-deploying consumers.
// ---------------------------------------------------------------------------

const i32 API_VERSION = 1

const i32 DEFAULT_PAGE_SIZE = 20

const i32 MAX_PAGE_SIZE = 500

const set<string> SUPPORTED_QUOTE_CURRENCIES = ["USD", "EUR", "GBP", "JPY"]
