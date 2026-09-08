# Designing durable IDL contracts

> **Applies to Apache Thrift 0.24.0.**

A `.thrift` file is not configuration; it is a **contract**. Once two
independently deployed programs rely on it, field IDs and requiredness
become permanent decisions. This chapter explains the parts of the IDL
where those decisions live. The grammar itself is normative in the
[IDL specification](/docs/idl); this page is about using it well.

The running examples are from the guide's example suite:
[`common.thrift`](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/common.thrift)
and
[`reference_data.thrift`](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/reference_data.thrift).

## Documents: headers and definitions

Every Thrift document is *headers followed by definitions*. Headers are
`include`, `cpp_include`, and `namespace`; definitions are `const`,
`typedef`, `enum`, `struct`, `union`, `exception`, and `service`
([IDL spec](/docs/idl)).

### Namespaces

```thrift
namespace java com.example.refdata.query
namespace py   refdata.query
namespace go   refdata.query
namespace netstd RefData.Query
```

- Declare namespaces for every language you ship to, plus `*` as a
  fallback for languages you have not named.
- The scope list is fixed by the compiler (`*`, `c_glib`, `cpp`, `delphi`,
  `haxe`, `go`, `java`, `js`, `lua`, `netstd`, `perl`, `php`, `py`,
  `py.twisted`, `rb`, `st`, `xsd` in 0.24.0). Notably there is **no `rs`
  scope**: Rust module paths come from the generator's `crate_prefix`
  option instead.
- Use each language's own conventions inside its namespace
  (`com.example…` for Java, dotted packages for Python, `path/segments`
  for Go).

**Language-specific (Go):** the `go` namespace defines the *package path
under the output directory*. Generation emits
`gen-go/<go-namespace>/<file-base>`, and cross-file imports use those
paths verbatim — so when the output is inside a Go module, generate with
`--gen go:package_prefix=<module>/gen-go/` so imports resolve.

### Includes

`include "common.thrift"` makes another file's symbols visible under its
name (`common.MarketPhase`) and adds language-appropriate imports to
generated code. Includes are how you share vocabulary; they are **not**
an inheritance mechanism, and included symbols do not merge into your
namespace. Give included files stable, generic names (`common.thrift`,
not `shared_v2_final.thrift`).

## Field IDs are forever

```thrift
struct QuoteSnapshot {
  1: required uuid instrumentId,
  2: required AsOf asOf,
  3: optional double bid,
  ...
}
```

On the wire, a field **is** its ID (plus a type tag). Names are for
humans; IDs are the contract. Therefore:

1. **Assign explicit IDs everywhere.** Implicit numbering works, but
   explicit IDs make insertions safe and reviews possible.
2. **Never reuse a retired ID for different data.** Readers that still
   skip it will silently mis-handle the new meaning.
3. **Never renumber** existing fields, even if "nobody uses the old
   ones".
4. **Document retirement in place** — a comment reservation is the
   convention this guide recommends:

```thrift
  // 12: (reserved, formerly sedol, withdrawn before release)
```

Negative IDs exist only to preserve compatibility with very old files and
require `thrift --allow-neg-keys` to parse; do not introduce them.

## Requiredness: `required`, `optional`, and default

The [IDL spec](/docs/idl) defines three requiredness states. In brief:

| | Write side | Read side | Evolution |
|---|---|---|---|
| `required` | always written | **must** be present; readers error otherwise | cannot be added or removed safely — ever |
| `optional` | written only when set (isset tracking) | may be absent | **safe** to add; safe to stop writing |
| *default* (no keyword) | written when set to a non-default value | may be absent | safe to add, **but** behavior differs per language |

Practical guidance (a convention, not a compiler rule):

- **`required` only for identity fields that will truly never leave**:
  the key of an entity, a timestamp that defines the message. Every
  `required` field is a field you can never remove and can never demote.
- **`optional` for anything that may legitimately be absent** or that you
  might deprecate: advisory metadata, new features.
- **default requiredness** for request-style fields where "absent means
  let the server decide" is the natural reading (filters, page sizes).
  Accept that unset reads as the language's zero value (`0`, `""`,
  `false`, `None`/`nil`) in most runtimes.

### Default values are part of the interface

```thrift
5: i32 limit = 20,
```

A default is applied by the *receiver* when the field is absent (in most
implementations it is materialized at initialization and omitted from the
wire when equal). That means changing `20` to `50` in a later revision
changes the behavior of **old clients that omit the field**, as soon as
*they* regenerate or the *server* does. Treat IDL defaults as wire-adjacent
API surface: change them only with a migration note
([IDL spec, "Semantics of Default Values"](/docs/idl)).

## Types and containers

Base types: `bool`, `byte` (alias `i8`), `i16`, `i32`, `i64`, `double`,
`string`, `binary`, `uuid`. Containers: `map<K,V>`, `set<T>`, `list<T>`.
There is no single-precision float, no unsigned integers, no date/time
type — encode intent with typedefs (`EpochMillis`) and choose one
convention per contract family.

- `map`, `set`, and `list` all carry explicit element counts on the wire;
  treat unbounded containers as a production hazard (see
  [production](practical-guide/production.html#size-and-complexity-limits)).
- `set` semantics differ per language (sorted vs hashed); do not rely on
  iteration order anywhere.
- `binary` is bytes without encoding guarantees — the right type for
  hashes and opaque blobs; `string` is (conventionally) UTF-8 text.
- Map keys may be any base type including `uuid` where supported.

**Language-specific (Go):** typedefs become *distinct named types*
(`type MicCode string`); conversions (`common.MicCode(s)`) are required at
assignment boundaries. **Language-specific (JavaScript):** `i64` is a
Number by default and silently loses precision above 2^53; 0.24.0 adds
opt-in BigInt via `js:bigint` ([changes, #3529](https://github.com/apache/thrift/blob/v0.24.0/CHANGES.md)).

### `uuid` is a base type — but not everywhere

`uuid` encodes as 16 bytes and maps to native UUID types where supported
(`java.util.UUID`, Python `uuid.UUID`, Go `thrift.Tuuid`, Rust
`uuid::Uuid`, …). In 0.24.0 the **compiler** generates `uuid`-aware code
for Java, Kotlin (via `java.util.UUID`), Python, Go, Rust, JavaScript,
.NET, C++, PHP, Delphi, Haxe, Lua, Ruby, and more — but **rejects** it for
Dart, Erlang, Perl, OCaml, and D, and aborts for c_glib. Verified by
compiling a `uuid`-bearing IDL against every 0.24.0 generator.

**Version-specific:** `uuid` support is recent; consumers on older Thrift
versions cannot read it. If your ecosystem includes older runtimes, ship
`binary` + a documented canonical encoding instead until all consumers
upgrade. Always check the [language matrix](/docs/Languages) (and note its
version header) before adopting `uuid` in a new contract.

## Enums, constants, typedefs

- **Enums** are `i32` on the wire. Assign explicit values to every member
  (`EQUITY = 0`) so insertions never shift existing values; values must be
  non-negative. Unknown incoming enum values raise errors in strict
  runtimes and arrive as raw integers elsewhere — handlers should not
  assume they know every value a peer may send.
- **Constants** (`const`) are materialized in generated code; they are not
  transmitted on the wire and changing them requires re-generation — they
  are shared literals, not a configuration channel.
- **Typedefs** are pure documentation/naming; they vanish on the wire.
  Use them to encode units and intent (`EpochMillis`, `MicCode`).
  **Language-specific (Go):** as noted above, typedefs are distinct named
  types there.

## Structs, unions, exceptions

**Structs** are the workhorse. Field names must be unique within a struct;
both `,` and `;` are accepted as field separators.

**Unions** transport *exactly one* member, and members are implicitly
optional. But — verified in the 0.24.0 sources and cross-language tests —
generated code does **not** enforce one-member semantics uniformly:

- Java/Python treat unions like structs with optional fields: they write
  whatever you set, without complaining if you set two.
- **Go 0.24.0 writes *all* union members unconditionally**, including
  zero values (a value-type field like `uuid` or `string` cannot express
  "unset"), so a Go-sent union can put three members on the wire.

**Caution:** if a contract involving Go uses `union`, receivers that
validate strictly may reject Go traffic, and Go receivers cannot tell a
deliberately-empty string from an unset one. Prefer a plain struct with
explicit `optional` fields + documented validation when Go participates;
otherwise enforce "exactly one, non-empty" in handlers on both ends. The
guide's example suite demonstrates the lenient-handler pattern
(`examples/`).

**Exceptions** are structs that integrate with native exception
mechanisms. In `throws` they become typed RPC errors with their own wire
message types. Keep them small and stable; never name a field `message`
(shadows the conventional base-exception attribute in several
languages), and remember that *undeclared* server-side failures still
reach clients as the generic application exception — declared exceptions
are the part of your error taxonomy you design (see
[production](practical-guide/production.html#error-taxonomy)).

## Services

```thrift
service ReferenceDataQuery {
  Instrument getInstrument(1: InstrumentSelector s)
    throws (1: LookupError notFound, 2: InvalidQuery invalid),
  oneway void reportUsage(1: string clientApp)
}
```

- Methods are identified **by name** on the wire (with a type-checked
  argument struct and, on replies, a typed result struct or declared
  exception).
- Parameter lists are anonymous structs: give every parameter an explicit
  ID, and treat parameter-list changes like struct changes.
- A service may `extends` another service; the subtype's interface
  includes the base's methods. This is occasionally useful (a "superset"
  service), but for in-place API growth the convention in this guide is to
  **add methods to the same service** — old clients keep working, and
  `thrift --audit` keeps validating the pair.

### `oneway` — use rarely, know exactly what you are buying

`oneway void f(...)` sends the call and expects **no reply**. Consequences
(verified against the wire spec and runtime behavior):

- The client returns as soon as the request is handed to the transport;
  **no delivery, processing, or ordering guarantee** is implied.
- **Declared exceptions are meaningless on a oneway method** — there is no
  reply to carry them. The compiler permits `oneway` only for `void`
  methods; do not combine `oneway` with `throws`.
- Errors inside the handler are invisible to the caller (some runtimes log
  them; some transport combinations swallow them).
- Use oneway only for loss-tolerant notifications (the example's
  `reportUsage` telemetry). For "at least once" or "processed" semantics,
  use a normal method — possibly returning quickly with a queued handle.

## Naming and portability

- Identifiers are letters, digits, `_`, and `.` (the IDL also allows `-`
  in service-scope names); both `:`-style field IDs and `,`/`;` separators
  are parsed. The compiler maintains a keyword list spanning many target
  languages — a name legal in the IDL may still be reserved in some
  generated language. Generators escape what they must, but predictable,
  language-neutral names avoid the issue: `UpperCamel` types,
  `lowerCamel` fields, no target-language keywords, no leading
  underscores.
- A typedef name that collides with a target-language builtin is a
  classic portability trap (`type`, `string`, `class`, `def`…) — check
  your targets' generated output once, then encode the convention in
  review checklists.

## A worked contract

The full example suite applies every rule above with explanatory
comments: [`common.thrift`](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/common.thrift)
(vocabulary),
[`reference_data.thrift`](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/reference_data.thrift)
(v1 contract), and
[`reference_data_v2.thrift`](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/reference_data_v2.thrift)
(evolution). Its schema diagrams can be regenerated with 0.24.0's Mermaid
generator: `thrift --gen mmd reference_data.thrift` (adds `exceptions`
option for error arrows; note that in 0.24.0 `uuid` fields render as
`string` in the diagram — a cosmetic generator quirk).

## Checklist for a new contract

- [ ] namespaces for every shipped language + `*` fallback
- [ ] explicit field IDs everywhere; no negative IDs
- [ ] `required` only on identity fields
- [ ] `optional` for everything advisory or new
- [ ] defaults reviewed as API surface
- [ ] enums with explicit non-negative values
- [ ] typedefs used for units and encodings (`EpochMillis`)
- [ ] `uuid` only if all consumers are on capable runtimes
- [ ] unions avoided if Go participates (or validated leniently in handlers)
- [ ] exceptions small, stable, not named `message`
- [ ] `oneway` only for loss-tolerant notifications, never with `throws`
- [ ] `thrift --audit` clean against the previous contract revision
