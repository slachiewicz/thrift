# Reference appendices

> **Applies to Apache Thrift 0.24.0.** This appendix condenses the
> normative sources for quick reference. The canonical documents — the
> [IDL specification](/docs/idl), the [type system](/docs/types), the
> [binary](https://github.com/apache/thrift/blob/v0.24.0/doc/specs/thrift-binary-protocol.md)
> and
> [compact](https://github.com/apache/thrift/blob/v0.24.0/doc/specs/thrift-compact-protocol.md)
> protocol specifications, and the
> [RPC message spec](https://github.com/apache/thrift/blob/v0.24.0/doc/specs/thrift-rpc.md)
> — remain authoritative.

## A. Annotated IDL quick reference

```thrift
// ---- headers -----------------------------------------------------------
include "common.thrift"          // symbols become common.X; adds imports
cpp_include "boost/optional.hpp" // (C++ only) raw include in C++ output

namespace java com.example.svc   // per-language package/namespace
namespace py   svc
namespace go   svc
namespace *    svc               // fallback for unnamed languages

// ---- definitions -------------------------------------------------------
typedef i64 EpochMillis          // alias; erased on the wire (named type in Go)

enum Phase {                     // i32 on the wire; values non-negative
  CLOSED = 0,                    // explicit values: insertions stay safe
  OPEN   = 2
}

const i32 MAX_RESULTS = 500      // generated constant; NOT a wire value
const set<string> CCS = ["USD","EUR"]

struct Quote {                   // the compositional workhorse
  1: required uuid  id,          // required: identity; reader errors if absent
  2: required i64   asOfMs,      // (EpochMillis in real contracts)
  3: optional double last,       // optional: written only when set
  4: i32           qty = 100,   // default requiredness + default value
  5: map<string,double> fx,      // containers: map<K,V> set<T> list<T>
  6: optional set<string> ccs
}

union Selector {                 // exactly one member — convention, and
  1: uuid id,                    // NOT uniformly enforced by generated code
  2: string symbol               // (Go 0.24.0 writes all members; validate
}                                //  in handlers / prefer structs w/ Go)

exception NotFound {             // struct + native exception semantics
  1: required string reason
}

service Quotes {
  Quote get(1: Selector s)                 // args are anonymous structs:
    throws (1: NotFound nf),               //   explicit IDs required here too
  list<Quote> scan(1: i32 limit),
  oneway void ping()                       // no reply ever; no throws allowed;
}                                          //   loss-tolerant notifications only
```

Grammar notes (from the [spec](/docs/idl)): a document is `Header*
Definition*`; field separators may be `,` or `;`; identifiers allow
letters, digits, `_`, `.`; negative field IDs need
`--allow-neg-keys` (legacy only); `xsd_*` options are historical
no-ops — do not use.

Base types: `bool`, `byte` (= `i8`), `i16`, `i32`, `i64`, `double`,
`string`, `binary`, `uuid`. No float32, unsigned ints, dates, decimals,
or null — model them (typedefs, sentinel structs, presence via
`optional`).

## B. Compiler flags (0.24.0)

```
thrift [options] file
  -o dir          output base dir; creates gen-<lang>/ inside
  -out dir        output directly into dir (no gen-<lang> wrapper)
  -I dir          add include search dir
  -r, --recurse   also generate included files
  --allow-neg-keys        accept negative field IDs (legacy files)
  --allow-64bit-consts    silence 64-bit constant warnings
  -strict         warnings become errors
  -v, -debug      verbosity
  --audit OldFile compare OldFile vs file for compatibility
                  (with -Iold dir / -Inew dir include paths)
  --gen STR       STR = language[:key=val[,key2=val2]]
```

Selected generator options (`thrift --help` for the full list):

| Target | Options worth knowing |
|---|---|
| `java` | `beans`, `option_type=jdk8|thrift`, `rethrow_unhandled_exceptions`, `sorted_containers`, `generated_annotations=undated|suppress`, `jakarta_annotations`, `unsafe_binaries`, `fullcamel`, `annotations_as_metadata` |
| `py` | `slots`, `enum`, `type_hints`, `package_prefix=`, `twisted`, `tornado`, `dynamic`, `coding=` |
| `go` | `package_prefix=`, `thrift_import=`, `package=`, `ignore_initialisms`, `read_write_private`, `skip_remote` |
| `js` | `node`, `ts`, `es6`, `with_ns`, `jquery`, `native_promise=` (default true), `bigint` (opt-in), `imports=` |
| `rs` | `crate_prefix=` (default `crate`) |
| `netstd` | `union`, `pascal`, `wcf`, `serial`, `net8/net9/net10`, `async_postfix`, `no_deepcopy` |
| `cpp` | `pure_enums[=enum_class]`, `moveable_types`, `templates`, `include_prefix`, `no_skeleton`, `no_ostream_operators` |
| `php` | `inlined`, `oop`, `nsglobal=`, `validate`, `classmap`, `getters_setters`, `json`, `rest`, `server` |
| `markdown` | `suffix=`, `noescape` (`.md` is the default extension) |
| `mmd` | `exceptions` (dashed arrows service→declared exception) |
| `html`/`json`/`xml` | `merge`, plus `standalone`/`noescape` (html) |

**Note (`--gen mmd`):** emits `gen-mmd/<file>.mmd` Mermaid
`classDiagram` per document (`-r`: per included file). Verified quirk in
0.24.0: `uuid` fields render as `string` in diagrams.

## C. Glossary

| Term | Meaning |
|---|---|
| IDL | Interface Definition Language: the `.thrift` schema language |
| Field ID | the integer wire identifier of a field (`1:`); permanent once shipped |
| Requiredness | `required` / `optional` / implicit default — read & write behavior per [IDL design](practical-guide/idl-design.html) |
| isset | runtime flag marking an optional field as present (Java `isSetX()`, Go non-nil, Python non-None) |
| Protocol | message encoding (binary, compact, JSON, header…) |
| Transport | byte movement + optional delimiting (socket, framed, buffered, HTTP…) |
| Framing | length-prefixing each message; must match on both ends |
| THeader | negotiation envelope (protocols/compression) supported by C++, Go, Python, Node.js in 0.24.0 |
| Server model | connection→execution strategy on the server (simple/threaded/pool/selector) |
| Processor | generated dispatcher: message → handler method call |
| Multiplexing | several services sharing one port via name prefixes |
| Message types | `CALL` / `REPLY` / `EXCEPTION` / `ONEWAY` on the wire |
| Sequence id | per-message integer linking replies to requests |
| Skew | two deployed sides running different contract revisions |

## D. Canonical links

- Downloads: <https://thrift.apache.org/download>
- Install: <https://thrift.apache.org/install>
- Types: <https://thrift.apache.org/docs/types> · IDL: <https://thrift.apache.org/docs/idl>
- Language matrix: <https://thrift.apache.org/docs/Languages> (check its version header)
- Tutorial: <https://thrift.apache.org/tutorial>
- Binary protocol spec: <https://github.com/apache/thrift/blob/v0.24.0/doc/specs/thrift-binary-protocol.md>
  (see also `doc/specs/thrift-compact-protocol.md`, `doc/specs/thrift-rpc.md`)
- Official source for this release: <https://github.com/apache/thrift/tree/v0.24.0>
- Release notes: <https://github.com/apache/thrift/blob/v0.24.0/CHANGES.md>
- Artifacts: Maven Central `org.apache.thrift:libthrift:0.24.0` ·
  PyPI `thrift==0.24.0` · npm `thrift@0.24.0` · crates.io `thrift 0.24.0`
- Security policy: <https://github.com/apache/thrift/blob/v0.24.0/SECURITY.md>
- Example suite for this guide:
  <https://github.com/apache/thrift/tree/master/doc/practical-guide/examples>

## E. Checklists

### E.1 Test checklist (every snippet/example)

- [ ] compiles against Thrift 0.24.0 (`thrift --version`)
- [ ] generated output inspected before use
- [ ] end-to-end run produces documented output
- [ ] cross-language pairing tested where claimed
- [ ] validation status marked (executed vs illustrative) in prose

### E.2 Link-check checklist

- [ ] all `thrift.apache.org` links resolve
- [ ] source links pinned to `v0.24.0` tag where version matters
- [ ] JIRA links use `THRIFT-NNNN` form
- [ ] no links to unofficial docs, blogs, or AI content

### E.3 Compatibility-review checklist

See [compatibility chapter](practical-guide/compatibility.html#compatibility-review-checklist)
(run per contract change; `--audit` must be in CI).

### E.4 Release-upgrade review checklist

See [compatibility chapter](practical-guide/compatibility.html#release-upgrade-review-thrift-runtime-upgrades)
(run per runtime upgrade; covers TLS/limits/defaults re-validation).

## F. Documentation maintenance note

This guide is version-scoped by design: each fact names its release, and
portable statements avoid "latest" language. When a new Thrift release
ships:

1. Re-run the example suite's generation + audit commands against the new
   compiler; fix drift.
2. Re-validate the three walkthroughs (newer toolchains okay; record
   versions used).
3. Re-check the fact-sensitive spots: per-language feature lists,
   defaults tables, security/limits sections, and every "0.24.0" banner —
   banners must advance **only** with a full re-verification pass.
4. Re-check links (they pin `v0.24.0`; update or add new-tag links
   deliberately).
5. Record the verification date and toolchain in the validation notes of
   the PR that advances the banner.
