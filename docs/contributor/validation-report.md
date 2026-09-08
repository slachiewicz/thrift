# Validation report — "Apache Thrift: A Practical Guide"

Date of validation: **2026-09-08**
Environment: macOS (darwin/arm64); local Apache Thrift compiler
`Thrift version 0.24.0`; sources cross-read from git tag `v0.24.0`
(6d2ec95f4) via a dedicated worktree.

## 1. Compiler-behavior validation (executed)

| Check | Command | Result |
|---|---|---|
| compiler identity | `thrift --version` | `Thrift version 0.24.0` |
| example IDL parses for all walkthrough targets | `thrift -r --gen {java,py,go,netstd,js:node,ts} reference_data.thrift` | all succeed; output trees inspected |
| `mmd` generator | `thrift --gen mmd[:exceptions]` | `gen-mmd/reference_data.mmd`, Mermaid `classDiagram`; `uuid` renders as `string` (quirk documented) |
| audit: v1→v2 compatible | `thrift --audit reference_data.thrift reference_data_v2.thrift` | exit 0, no output |
| audit: type change caught | deliberate v2 with `last: double→i64` | exit 2, `Struct Field Type Changed for Id = 5 in QuoteSnapshot` |
| audit: removal caught | deliberate v2 with field 7 removed | exit 2, `Struct Field removed for Id = 7 in QuoteSnapshot` |
| audit: requiredness change caught | deliberate v2 with `6 optional→required` | exit 2, `Struct Field Requiredness Changed for Id = 6 in QuoteSnapshot` |
| audit include-path sensitivity | audit from dir lacking `common.thrift` | false "type changed" failures; guide documents running with includes resolvable / `-Iold`,`-Inew` |
| `uuid` support matrix | generated a `uuid`-field struct for every 0.24.0 generator | supported (native types): java, kotlin, py, go, rs, js, netstd, cpp, php, delphi, haxe, lua, rb, st, cl, javame. Rejected with errors: dart, erl, perl, ocaml, d. c_glib: compiler abort (crash) |
| Go union wire behavior | inspected generated `writeField*`; cross-language test | **all union members written unconditionally** (incl. zero values) — documented as a verified caveat |
| `--help` generator options | `thrift --help` | captured; quoted options verified (incl. `js:bigint`, `native_promise` default true, `jakarta_annotations`, `mmd`) |

## 2. Walkthrough validation (executed end-to-end)

| Pairing | Client | Server | Result |
|---|---|---|---|
| Python↔Python | `client.py` | `server.py` | full expected output incl. typed errors + oneway log |
| Go↔Go | `go run ./client` | `go run ./server` | full expected output |
| Java↔Java | `JavaClient` (mvn exec) | `JavaServer` (mvn exec) | full expected output |
| Python client ↔ Java server | `client.py` snippets | `JavaServer` | getInstrument + latestSnapshot correct (buffered client ↔ plain server transport interop) |
| Go client ↔ Python server | `go run ./client` | `server.py` | full expected output (after documenting the Go union quirk & lenient validation) |

Toolchains used (recorded for reproduction):
- Python 3.14.7 + `thrift==0.24.0` from PyPI (venv; PEP 668 respected)
- Go 1.27.1 + module `github.com/apache/thrift v0.24.0` (go.sum committed)
- OpenJDK 25.0.4 + Maven 3.10.0-rc-1 + `org.apache.thrift:libthrift:0.24.0`
  (Maven Central), `jakarta.annotation-api` 2.1.1, `slf4j-simple` 1.7.36

Issues discovered during validation (all reflected in the guide):
1. Java without `jakarta_annotations` fails on modern JDKs
   (`javax.annotation` absent) → walkthrough uses the 0.24.0 generator
   flag + dependency.
2. Java has no `TBufferedTransport`; plain `TSocket` ↔ `TTransportFactory`
   used (and shown to interop with Python's buffered transport).
3. Go union write path violates "exactly one" on the wire → documented;
   example handlers validate leniently.
4. c_glib `uuid` → compiler crash (uncaught exception). Candidate JIRA
   for upstream; the guide documents it as a known gap, not released
   behavior.

## 3. Documentation checks

- Markdown files: consistent H1 per page, version banner present, admonition
  labels used as specified (**Note/Caution/Security/Language-specific/Version-specific**).
- Relative cross-links verified to match file names in this tree.
- External links pinned to `v0.24.0` where version matters; registry
  claims verified 2026-09-08 (Maven Central listing, PyPI JSON,
  crates.io API, npm dist-tag).
- **Originality check:** this work was authored from the 0.24.0 source
  tree and executed behavior only. The historic third-party guide was
  not consulted for content, structure, examples, or wording; no part of
  this guide copies or adapts it. (The only reference to it anywhere is
  the existing third-party link already present on the site's docs
  index, which this change does not touch.)

## 4. Known limitations / honesty notes

- Matrix-dependent claims (which runtime supports which feature) were
  verified against the 0.24.0 source trees for the languages named;
  secondary targets rely on generator/run tests plus `LANGUAGES.md`, and
  the guide says so.
- Walkthroughs were validated on macOS/arm64; Windows-specific steps are
  not covered.
- The guide intentionally excludes: C++ deep dive, Delphi/Erlang/OCaml
  specifics, and any unreleased behavior from master.
