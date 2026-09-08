# Fact Ledger — "Apache Thrift: A Practical Guide" (0.24.0)

Internal contributor documentation. **Not part of the published guide.**
Every technical claim made in the guide traces to one of the rows below.

Source keys:
- `CHANGES` = CHANGES.md, section `0.24.0`, tag `v0.24.0` of apache/thrift
- `IDL` = doc/specs/idl.md @ v0.24.0 (published at https://thrift.apache.org/docs/idl)
- `LANGS` = LANGUAGES.md @ v0.24.0
- `HELP` = `thrift --help`, compiler 0.24.0 (verified output, captured 2026-09-08)
- `EMPIRICAL-n` = executed against local Apache Thrift compiler 0.24.0 (`thrift --version` → `Thrift version 0.24.0`)
- `LIB:<path>` = file in the v0.24.0 source tree
- `MVN`/`PYPI`/`CRATES`/`NPM` = registry metadata retrieved 2026-09-08

| # | Claim | Scope | Version | Source | Validation | Notes |
|---|-------|-------|---------|--------|------------|-------|
| 1 | `uuid` is an IDL base type (`bool\|byte\|i8\|...\|binary\|uuid`) | IDL | 0.24.0 | IDL §Types [25]; thriftl.ll:229 | EMPIRICAL-1 (compiles in java/py/go) | Introduced for these languages in/around 0.17+; not universal, see #2 |
| 2 | `uuid` generator support (emits uuid-native code): java, kotlin (as `java.util.UUID`), py, go, rs, js, netstd, cpp, php, delphi, haxe, lua, rb, st, cl, javame | compiler | 0.24.0 | EMPIRICAL-2 (all targets compiled with a `uuid` field; generated code inspected) | verified | kotlin maps to `java.util.UUID` on JVM |
| 3 | `uuid` NOT supported: dart, erlang, perl, ocaml, d (compiler error); c_glib aborts with uncaught exception "no C base type name for base type uuid" | compiler | 0.24.0 | EMPIRICAL-3 (exact error messages captured) | verified | c_glib abort is a hard compiler crash, filed as upstream defect candidate |
| 4 | IDL document = `Header* Definition*`; headers = include/cpp_include/namespace; definitions = const/typedef/enum/struct/union/exception/service | IDL | 0.24.0 | IDL [1]–[7] | verified | normative grammar |
| 5 | Namespace scopes in 0.24.0: `* c_glib cpp delphi haxe go java js lua netstd perl php py py.twisted rb st xsd` | compiler | 0.24.0 | IDL [6] | verified | no `py.tornado`, no `rs` scope; rust uses `crate_prefix` option instead |
| 6 | Requiredness: `required`, `optional`, implicit "default" (opt-in/req-out); unions members implicitly optional; enum values must be non-negative | IDL | 0.24.0 | IDL [10][12][17] + requiredness section | verified | default-value semantics: implementations differ; unwritten defaults become part of the interface |
| 7 | `LANGUAGES.md` on the v0.24.0 tag is headed "Guidance For: 0.22.0" — published matrix lags the release | docs | 0.24.0 | LANGS header | verified | guide must not treat matrix as exhaustive/current |
| 8 | Binary protocol is the core protocol "supported by all languages except JavaScript" (browser JS: JSON-over-HTTP class of stacks) | runtime | 0.22.0 wording (matrix) | LANGS | quoted-with-caveat | re-check per language before relying |
| 9 | THeader protocol/transport exists in: C++, Go, Python, Node.js. NOT in Java, netstd, PHP, Rust (no THeader files in those libs) | runtime | 0.24.0 | LIB:lib/{go,py,nodejs}/…header…; `grep -rl THeader lib/java` → none | verified | Java users cannot use THeader in 0.24.0 |
| 10 | Java servers: TSimpleServer, TThreadPoolServer, THsHaServer, TNonblockingServer, TThreadedSelectorServer, TSaslNonblockingServer | runtime | 0.24.0 | LIB:lib/java/src/main/java/org/apache/thrift/server/ | verified (dir listing) | |
| 11 | Go servers: TSimpleServer only (single concrete server type in lib/go) | runtime | 0.24.0 | LIB:lib/go/thrift/ (server.go, simple_server.go; no other server types) | verified | common misconception that Go has TThreaded/TNonblocking servers |
| 12 | Python servers: TSimpleServer, TThreadedServer, TThreadPoolServer, TForkingServer, TNonblockingServer, THttpServer, TProcessPoolServer | runtime | 0.24.0 | LIB:lib/py/src/server/ | verified | |
| 13 | Rust servers: TSimpleServer, TThreadedServer (threaded.rs), TMultiplexedProcessor | runtime | 0.24.0 | LIB:lib/rs/src/server/ | verified | |
| 14 | PHP servers: TSimpleServer, TForkingServer (+TServerSocket/TSSLServerSocket) | runtime | 0.24.0 | LIB:lib/php/lib/Server/ | verified | |
| 15 | netstd transports: TSocketTransport, TTlsSocketTransport, THttpTransport, TNamedPipeTransport, TStreamTransport, TMemoryBufferTransport; protocols: TBinary, TCompact, TJSON, TMultiplexed | runtime | 0.24.0 | LIB:lib/netstd/Thrift/Transport,/Protocol | verified | |
| 16 | PHP protocols: TBinary(+Accelerated), TCompact, TJSON, TSimpleJSON, TMultiplexed; transports incl. THttpClient, TCurlClient, TPsrHttpClient (new), TSSLSocket, TSocketPool | runtime | 0.24.0 | LIB:lib/php/lib/{Protocol,Transport}/ | verified | TPsrHttpClient per THRIFT-6010 |
| 17 | Java TConfiguration defaults: maxMessageSize=100 MiB, maxFrameSize=16 384 000, recursionLimit=64 | runtime (java) | 0.24.0 | LIB:TConfiguration.java:22–25 | verified | 16384000 "used consistently across all Thrift libraries" |
| 18 | Go TConfiguration: DEFAULT_MAX_MESSAGE_SIZE=100 MiB, DEFAULT_MAX_FRAME_SIZE=16384000; nil-safe getters | runtime (go) | 0.24.0 | LIB:lib/go/thrift/configuration.go:31–32 | verified | |
| 19 | Python: THeaderTransport/TZlibTransport default max frame & decompressed size = 16384000 (THRIFT-6024); TProtocol.skip has default recursion depth limit (#3411); THeaderTransport decompressed-size limit (#3408) | runtime (py) | 0.24.0 | CHANGES; LIB:lib/py/src/transport/THeaderTransport.py:37 | verified | |
| 20 | Compact-protocol varint reader byte-count limit added across libs (#3410); recursion-depth limits added to nearly all libs (c_glib, D, Dart, Delphi, Erlang, Go, Haxe, Java/JavaME, JS, Kotlin, Lua, OCaml, Perl, PHP, Python, Ruby, Rust, Smalltalk) | runtime | 0.24.0 | CHANGES | verified | 0.24.0 robustness theme |
| 21 | Java TLS hostname verification enabled in TSSLTransportFactory (#3390) and TNonblockingSSLSocket (#3396); Python TSSLSocket uses sslcompat hostname matcher (#3413); c_glib TLS client peer hostname validation (#3507); C++ disables TLSv1.0/1.1 by default (THRIFT-3165); C++ RFC 6125 wildcard enforcement (#3506) | runtime | 0.24.0 | CHANGES | verified | security-relevant defaults changed → upgrade note |
| 22 | Node.js: generator+runtime switched from Q to native Promises (THRIFT-6040); `js:…,native_promise=[true\|false]` flag, default **true** (Q output opt-out, needs `q` package); opt-in BigInt for i64 via `js:bigint` (#3529; forced off for plain browser output per t_js_generator.cc:113) | compiler+runtime (node) | 0.24.0 | CHANGES; HELP (js options); LIB:t_js_generator.cc | verified | breaking change vs ≤0.23 Q-based code |
| 23 | PHP runtime: minimum PHP 8.1 (THRIFT-5956), PSR-12, strict_types, native types, constructor promotion; PSR-3 logger support (THRIFT-6009); PSR-18 HTTP transport (THRIFT-6010) | runtime (php) | 0.24.0 | CHANGES; LIB:lib/php/README.md ("Thrift requires PHP 8.1") | verified | older PHP unsupported |
| 24 | Swift support dropped (THRIFT-5864); CHANGES section literally "Swift - NO LONGER SUPPORTED"; lib/swift removed from tree | project | 0.24.0 | CHANGES; absence of lib/swift | verified | guide must not mention Swift as target |
| 25 | Mermaid generator `--gen mmd` (THRIFT-6026), option `exceptions` (dashed arrows to declared exceptions); output `gen-mmd/<program>.mmd`, Mermaid `classDiagram`, `direction LR`; with `-r`, one file per .thrift | compiler | 0.24.0 | HELP; EMPIRICAL-4 (ran generator, inspected output) | verified | observed quirk: `uuid` fields render as `string` (g_type_uuid is `t_base_type("string", TYPE_UUID)`, common.cc:39) |
| 26 | Markdown generator defaults to `.md` extension (THRIFT-6038) | compiler | 0.24.0 | CHANGES; HELP | verified | |
| 27 | `--audit OldFile` compares two IDL files for wire compatibility; `-Iold dir`, `-Inew dir` | compiler | 0.24.0 | HELP | EMPIRICAL-5 (used in guide examples) | to be exercised on example v1→v2 |
| 28 | `libthrift` 0.24.0 published on Maven Central (jars + module metadata, 2026-07-11); Java lib built with Gradle 8 | java | 0.24.0 | MVN repo1 listing; LIB:lib/java/README.md | verified | |
| 29 | Go module path `github.com/apache/thrift`; go.mod says `go 1.25` | go | 0.24.0 | LIB:go.mod | verified | |
| 30 | PyPI `thrift` 0.24.0 published (THRIFT-6070 first release with wheels): wheels cp310–cp314 (macOS x86_64/arm64, manylinux2014, musllinux, win_amd64) + sdist | python | 0.24.0 | PYPI JSON; CHANGES THRIFT-6070 | verified | |
| 31 | crates.io `thrift` 0.24.0 published 2026-07-11; edition 2021; default features `server`→`threadpool`,`log` | rust | 0.24.0 | CRATES API | verified | THRIFT-6068 staleness resolved |
| 32 | npm `thrift` dist-tag `latest` = 0.24.0; README: "Node.js 10.18 or later is required" | node | 0.24.0 | NPM registry; LIB:lib/nodejs/README.md | verified | |
| 33 | Compiler invocation: `thrift [-o out][-out dir][-I dir][-r][--gen lang[:opts]] file`; `--gen` STR = `language[:key1=val1,…]`; generators list matches HELP | compiler | 0.24.0 | HELP; EMPIRICAL | verified | |
| 34 | Go generator options: package_prefix, thrift_import, package, ignore_initialisms, read_write_private, skip_remote | compiler (go) | 0.24.0 | HELP | verified | |
| 35 | Java generator options include: beans, private_members, nocamel, fullcamel, option_type=[thrift\|jdk8], rethrow_unhandled_exceptions, sorted_containers, generated_annotations=[undated\|suppress], unsafe_binaries, jakarta_annotations, annotations_as_metadata | compiler (java) | 0.24.0 | HELP | verified | |
| 36 | py generator options include: slots, enum (IntEnum, Py≥3.4), type_hints (requires enum), dynamic, package_prefix, twisted/tornado, coding | compiler (py) | 0.24.0 | HELP | verified | |
| 37 | Go connection check in TSocket incl. TLS (THRIFT-5214/5969); generated Go code is gofmt-compatible (THRIFT-6011); Go recursive struct depth limit (THRIFT-6044) | runtime (go) | 0.24.0 | CHANGES | verified | |
| 38 | Ruby multiplexed processors can fall back to a default service for old clients (THRIFT-6015) | runtime (rb) | 0.24.0 | CHANGES | verified | |
| 39 | Rust: forward-compatible union deserialization in structs (THRIFT-5953); TTcpChannel read/write timeouts (THRIFT-5954); `crate_prefix` option for cross-file imports (THRIFT-6059); recursion depth limit (THRIFT-6057); list<union> deserialization fixes (THRIFT-6058/6064) | compiler+runtime (rs) | 0.24.0 | CHANGES; HELP (rs options) | verified | |
| 40 | Python UUID support added (THRIFT-5923); generated py maps uuid→`uuid.UUID`, TType.UUID, C-extension fast path present | runtime (py) | 0.24.0 | CHANGES; EMPIRICAL (gen-py ttypes inspected) | verified | |
| 41 | JS/TS: i64 remains Number by default → precision loss beyond 2^53; use `js:bigint` for true 64-bit (opt-in) | js/node | 0.24.0 | #3529 in CHANGES; HELP | verified | cross-language i64 caveat |
| 42 | Site convention: thrift.apache.org publishes docs straight from source-tree Markdown (page footers link to `doc/…` file); nav topics listed on https://thrift.apache.org/docs | project | current site | webfetch of /docs and /docs/idl (2026-09-08) | verified | placement proposal built on this |
| 43 | Java protocol set: TBinaryProtocol, TCompactProtocol, TJSONProtocol, TMultiplexedProtocol, TLegacyUuidProtocolDecorator; transports: TSocket/TServerSocket, TNonblocking*, TSSLTransportFactory, THttpClient, TFramed/TFastFramedTransport, TZlibTransport, TSasl*, TMemoryBuffer | runtime (java) | 0.24.0 | LIB listing | verified | no TSimpleJSONProtocol in java 0.24.0 |
| 44 | Go protocol set: TBinaryProtocol, TCompactProtocol, THeaderProtocol, TJSONProtocol(?), TMultiplexedProtocol; transports: TSocket, TSSLSocket, TBuffered, TFramed, THttpClient, THeaderTransport, TZlibTransport | runtime (go) | 0.24.0 | LIB listing (go/thrift dir) | verified | json protocol present in go 0.24? — recheck before publishing (flagged) |
| 45 | Python protocol set: TBinaryProtocol, TCompactProtocol, THeaderProtocol, TJSONProtocol, TMultiplexedProtocol, TProtocolDecorator; transports: TSocket, TSSLSocket, TBuffered, TFramed, THttpClient, THeaderTransport, TZlibTransport, TMemoryBuffer | runtime (py) | 0.24.0 | LIB listing | verified | |
| 46 | C++ generators/flags sampled: cpp pure_enums / enum_class / include_prefix / moveable_types etc. | compiler (cpp) | 0.24.0 | HELP | verified | |
| 47 | Timeouts: Java TSocket has setConnectTimeout(int) and socket timeouts (TConfiguration/TSocket); Python TSocket.setTimeout(ms) (single knob, applies to handle); Go TSocket has SetConnTimeout and per-op timeouts (NewTSocketFromConnTimeout) | runtime | 0.24.0 | LIB sources (paths in notes column of guide) | verified | exact API names cited in guide |
| 48 | xNL: `byte` and `i8` are the same wire type (alias); both accepted | IDL | 0.24.0 | IDL [25]; common knowledge in specs | verified via grammar | both spellings legal |

Open items / flagged:
- Row 44: confirm Go TJSONProtocol presence in 0.24.0 before publication (json_protocol.go exists? verify).
- c_glib `uuid` compiler abort (row 3): candidate JIRA; mention in troubleshooting as known gap, not as released behavior.
- `LANGUAGES.md` per-language "Uuid" column was cross-checked against EMPIRICAL-2/3; where they disagree, empirical result wins (e.g., matrix shows Dart yes/no — empirical says compile error).
