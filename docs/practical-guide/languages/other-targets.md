# Other language targets

> **Applies to Apache Thrift 0.24.0.** This page summarizes targets beyond
> the deep-dive trio ([Java](practical-guide/languages/java.html),
> [Python](practical-guide/languages/python.html),
> [Go](practical-guide/languages/go.html)). Depth here is deliberately
> limited: consult the canonical [language matrix](/docs/Languages) —
> noting that its header records the release it was last verified for and
> therefore **lags** the newest release — and each library's README under
> `lib/` before committing to a target.

## Node.js and JavaScript (`lib/nodejs`, `lib/js`, `lib/ts`)

- **Package:** `thrift` on npm (0.24.0 is `latest`). Node.js runtime in
  `lib/nodejs`; browser runtime in `lib/js`; TypeScript definitions via
  generator options.
- **Generation:** `thrift --gen js:node,ts file.thrift` → `gen-nodejs/`
  with `.js` + `.d.ts` (verified output). Browser code: plain
  `--gen js` (or `js:jquery`); `with_ns` adds namespace objects.
- **0.24.0 changes you will notice:**
  - **Native Promises by default.** Generator and runtime switched from
    the `Q` library to native `Promise` (THRIFT-6040). Client methods
    return real promises; `q` is no longer a dependency. Legacy Q-based
    output is opt-out only: `js:…,native_promise=false`.
  - **Opt-in BigInt for `i64`** via `js:bigint` (#3529). Without it, i64
    is a `Number` and silently loses precision above 2^53 — a classic
    cross-language bug against Java/Go producers. The flag is forced off
    for plain browser output; on Node, regenerate and use
    `bigint_compat`/`int64_util` helpers.
  - WebSocket transport upgraded to `ws` v8 (#3526).
- **Transports/protocols (Node):** buffered, framed, WebSocket, HTTP;
  binary, compact, JSON, and header protocols; multiplexed support.
  Browser: JSON/HTTP-class stacks.
- **Caution:** decide BigInt policy *before* defining any `i64` field in
  a JS-reachable contract; retrofitting is a breaking change.

## Rust (`lib/rs`)

- **Crate:** `thrift` on crates.io — 0.24.0 published (edition 2021;
  default feature `server` pulls `threadpool`+`log`).
- **Generation:** `thrift --gen rs[:crate_prefix=p] file.thrift`.
  0.24.0's `crate_prefix` controls cross-file import paths (THRIFT-6059):
  default `crate`, `super` for submodule layouts.
- **0.24.0 hardening/improvements:** recursion-depth limits
  (THRIFT-6057), forward-compatible union deserialization in structs
  (THRIFT-5953), boxed recursive unions on read (THRIFT-6064),
  `list<union>` fixes (THRIFT-6058), `TTcpChannel` read/write timeouts
  (THRIFT-5954), max-string-size enforcement on non-strict message names
  (#3609).
- **Verified gaps to plan around:** `uuid` is supported by the generator
  (maps to the `uuid` crate's type); the **runtime has no TLS transport**
  — terminate TLS in a proxy in front of Rust services; servers are
  `TSimpleServer`/`TThreadedServer`.
- Protocols: binary, compact, multiplexed (+stored/simple-json
  internals); transports: buffered, framed, memory, TCP channel.

## PHP (`lib/php`)

- **Requires PHP ≥ 8.1** (THRIFT-5956). The 0.24.0 runtime is thoroughly
  modernized: `strict_types`, native parameter/property/return types,
  PSR-12, constructor promotion, phpstan-guarded (THRIFT-5956…6010).
- **Generation:** `thrift --gen php[:nsglobal=NAME] …`; options include
  `validate`, `classmap`, `getters_setters`, `rest`, `server`.
- **New in 0.24.0:** PSR-18 HTTP transport `TPsrHttpClient` (THRIFT-6010)
  and PSR-3 logger hooks in transports (THRIFT-6009).
- Protocols: binary (+accelerated), compact, JSON, simple-JSON;
  transports: socket, SSL socket, socket pool, buffered, framed, HTTP
  (curl/PSR-18), memory, streams; `TMultiplexedProcessor` available.
- **Caution:** if you maintain PHP 7-era code, the 8.1 floor is a real
  migration (types are enforced at runtime, not just declared).

## .NET (`lib/netstd`)

- **Generation:** `thrift --gen netstd[:union,pascal,…]`; 0.24.0 adds
  `net8/net9/net10` feature toggles, `async_postfix`, `no_deepcopy`, and
  emits constants using `const` where possible (THRIFT-5997).
- Protocols: binary, compact, JSON; transports: socket, TLS socket,
  HTTP, named pipes, streams; `TMultiplexedProtocol`; servers:
  simple/threaded. No THeader.
- The `union` option switches on the dedicated union type with static
  `read` — closer to union semantics than struct-based targets.
- CI in the project tests .NET 8/9/10 (THRIFT-6002).

## C++ (`lib/cpp`)

- The reference runtime: binary, compact, JSON, simple-JSON, **THeader**;
  zlib; nonblocking servers; OpenSSL integration. 0.24.0 disables
  TLS 1.0/1.1 by default (THRIFT-3165), enforces RFC 6125 wildcard
  placement in hostname matching (#3506), and allows injecting an
  external `SSL_CTX` (THRIFT-6073).
- Useful generator options: `pure_enums=enum_class`, `moveable_types`,
  `templates`, `include_prefix`, `no_skeleton`.

## Other compiler targets in 0.24.0

Dart, D, Delphi, Erlang, Haxe, Lua, OCaml, Perl, Ruby, Smalltalk,
c_glib (C), Java ME, Common Lisp, plus documentation/model generators
(`html`, `json`, `xml`, `markdown` — now `.md` by default, THRIFT-6038 —
and the new Mermaid generator `--gen mmd`, THRIFT-6026).

Verified `uuid` caveats for these targets: the compiler **rejects**
`uuid` for Dart, Erlang, Perl, OCaml, and D, and aborts for c_glib;
Ruby, Lua, Smalltalk, Common Lisp, Haxe, and Delphi do emit uuid-aware
code (runtime maturity varies — test before relying).

## Choosing targets: honesty checklist

- [ ] check the matrix **header version**, then confirm critical features in the lib README and source
- [ ] test your exact protocol × transport × server pairing cross-language in CI
- [ ] treat young or lightly-tested pairings as unproven until measured
- [ ] prefer the conservative stack (binary + framed + blocking servers) where maturity matters
- [ ] watch i64 in JS, typedef conversions in Go, PHP 8.1 floor, Rust TLS absence
