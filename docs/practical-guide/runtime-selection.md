# Runtime choices: protocols, transports, servers

> **Applies to Apache Thrift 0.24.0.** Feature availability differs per
> language; every claim below was checked against the 0.24.0 runtimes, and
> language caveats are stated inline. The [language matrix](/docs/Languages)
> remains the canonical overview — but note that its header records which
> release it was last verified for, so treat it as a starting point, not a
> guarantee.

## The three dials

Every Thrift endpoint is configured with up to three independent choices:

1. **Protocol** — how messages are encoded (`TBinaryProtocol`,
   `TCompactProtocol`, `TJSONProtocol`, …).
2. **Transport** — how bytes move and whether messages are delimited
   (`TSocket`, `TFramedTransport`, `THttpClient`, `THeaderTransport`, …).
3. **Server model** (server side) — how connections are served
   (thread-per-connection, pools, selectors, …).

Client and server must agree on (1) and (2). Within those constraints the
dials compose freely.

## Framing, explicitly

**Framing** means each message is prefixed with its byte length.

- *Framed transport* (`TFramedTransport` in most runtimes): the
  application writes into a buffer; `flush()` emits
  `<4-byte-length><message>`. Both sides **must** use it, or the receiver
  reads the length prefix as message data (typical symptom: hang on first
  call, then timeout).
- *Unframed socket + buffered wrapper*: no wire framing; bytes stream as
  written and `flush()` marks logical message boundaries. Buffered vs
  plain is a local buffering decision only — a buffered client works
  against a plain server (validated in the example suite:
  Python buffered client ↔ Java plain server).
- *Header transport* (`THeaderTransport`, with `THeaderProtocol`):
  a negotiation header + optional framing + optional compression, with
  backward compatibility to plain unframed binary. Present in **C++, Go,
  Python, Node.js** — not implemented in Java, .NET, PHP, or Rust in
  0.24.0, so it cannot be your universal choice yet.

## Protocols

| Protocol | Encoding | Relative size | Availability in 0.24.0 (verified) | Use when |
|---|---|---|---|---|
| **Binary** | straightforward fixed-width + length-prefixed strings | baseline | all major runtimes incl. Java, Python, Go, .NET, PHP, Rust; browser JS is the notable exception (its stacks are JSON-class) | default choice for service-to-service RPC |
| **Compact** | varints, zig-zag, field-ID deltas | typically smaller | broad: Java, Python, Go, .NET, PHP, Rust, Ruby, Lua, Haxe, Delphi, and others | bandwidth-conscious links; binary-compatible semantics with binary protocol |
| **JSON** | text JSON | largest | Java, Python, Go, .NET, PHP, Node.js, and others | debugging, admin tooling, simple external integration |
| **SimpleJSON** | write-mostly JSON | large | C++, Go, Java(no), PHP, Python(no) — check per runtime | metrics/observability sinks that only write |
| **Header** (`THeader*`) | envelope around binary/compact + transforms | varies | C++, Go, Python, Node.js only | internal fleets standardized on it; needs both ends capable |

**Version-specific (0.24.0 robustness work):** compact-protocol varint
readers gained byte-count limits and many runtimes hardened negative-size
and recursion handling (see
[CHANGES](https://github.com/apache/thrift/blob/v0.24.0/CHANGES.md)). If
you upgrade from an older release, re-run interop tests for compact
protocol specifically.

**Language-specific (JS):** the browser JavaScript runtime's protocol
support differs from Node.js; browser clients are typically used with
JSON over HTTP. Verify your exact pairing on the matrix and in `lib/js`
before designing a browser-facing endpoint.

## Transports

| Transport | Present in (0.24.0, verified) | Notes |
|---|---|---|
| `TSocket` / `TServerSocket` (blocking TCP) | everywhere | the universal baseline; timeouts configurable per runtime |
| TLS variants (`TSSLSocket`, `TSSLTransportFactory`, `TTlsSocketTransport`, …) | Java, Python (TSSLSocket), Go (TSSLSocket/TSSLServerSocket), .NET, PHP, c_glib, Delphi, and others — **not Rust** | see [production](practical-guide/production.html#tls) for 0.24.0 hostname-verification changes |
| Framed wrapper | broadly available | must match on both ends |
| Buffered wrapper | broadly available | local optimization, no wire impact |
| HTTP client/server | Java, Python, Go, Node.js, PHP (incl. new PSR-18 `TPsrHttpClient`), .NET | traversal of HTTP infrastructure; no framing (message per request) |
| WebSocket | Node.js | browser-style deployments |
| THeader transport | C++, Go, Python, Node.js | negotiation + optional compression |
| zlib wrapper | Java, Python, Go, Ruby, and others; defaults aligned to 16 384 000-byte frame/decompressed limits in 0.24.0 (THRIFT-6024) | compression for large payloads; mind decompression bombs → [production](practical-guide/production.html#size-and-complexity-limits) |
| Memory/file/pipe transports | several runtimes (Java `TMemoryBuffer`, Python `TMemoryBuffer`, .NET named pipes, …) | in-process or IPC use |

## Server models (per language, verified in the 0.24.0 trees)

| Language | Server types shipped |
|---|---|
| **Java** | `TSimpleServer`, `TThreadPoolServer`, `THsHaServer`, `TNonblockingServer`, `TThreadedSelectorServer`, `TSaslNonblockingServer` |
| **Python** | `TSimpleServer`, `TThreadedServer`, `TThreadPoolServer`, `TForkingServer`, `TNonblockingServer`, `TProcessPoolServer`, `THttpServer` |
| **Go** | `TSimpleServer` (the only one — concurrency comes from your architecture around it) |
| **Rust** | `TSimpleServer`, `TThreadedServer` (+ multiplexed processor support) |
| **PHP** | `TSimpleServer`, `TForkingServer` |
| **.NET (netstd)** | `TServer`, `TSimpleServer`, `TThreadedServer` in `Thrift.Server` |

**Language-specific (Go):** a common misconception is that Go ships
threaded/nonblocking servers like Java. It does not: `TSimpleServer`
serves one connection at a time, so production Go services typically
front it with their own accept loop or place it behind process managers —
or handle concurrency at the transport level. Plan capacity accordingly.

**Note (Python):** `TForkingServer` uses process forking (POSIX only);
`TProcessPoolServer` gives a worker pool. Both sidestep the GIL for CPU
work, at the cost of shared-state complexity.

## Multiplexing

`TMultiplexedProtocol`/`TMultiplexedProcessor` prefixes method names with
a service name so **multiple services share one port**. Verified available
in 0.24.0: Java, Python, Go, Node.js, .NET, PHP, Ruby (with a
new-in-0.24.0 default-service fallback for old clients, THRIFT-6015),
Rust, and others. Clients must opt in symmetrically. Use it to reduce
port sprawl; skip it when independent scaling or independent TLS
credentials per service matter more.

## Decision table

| Scenario | Protocol | Transport | Server model | Benefits | Trade-offs / caveats |
|---|---|---|---|---|---|
| internal service-to-service RPC, mixed languages | Binary | Framed over TCP | Java: `TThreadPoolServer`; Python: `TThreadPoolServer`; Go/Rust: shipped server + own concurrency | simple, fast, universal | framing must match both ends |
| bandwidth-constrained or high-QPS small messages | Compact | Framed | as above | smaller payloads | verify compact limits on upgrade; CPU slightly higher |
| debug/admin endpoint | JSON | HTTP or buffered TCP | simple/threaded | human-readable, curl-able (HTTP) | larger payloads; do not expose publicly |
| large payloads, compressible | Binary + zlib wrapper, or THeader+zlib | framed | pool | network savings | decompression limits + CPU cost; 0.24.0 default limits apply |
| browser or proxy-fronted clients | JSON | HTTP client/server | language's HTTP server | infrastructure-friendly | no server push; protocol support varies per runtime |
| heterogeneous fleet, Go/Py/C++/Node only | Binary or Compact | THeader | — | negotiation, compression, backward-compatible unframed mode | Java/.NET/PHP lack THeader in 0.24.0 |
| TLS-protected production traffic | Binary | TLS socket (+framed) | pool | encryption + 0.24.0 hostname verification | Rust runtime has no TLS — terminate TLS externally for Rust services |
| many services, one port | Binary | Framed | pool + `TMultiplexedProcessor` | fewer listeners | both ends must multiplex; weaker per-service isolation |
| untrusted/large user input | Binary | Framed | pool, with limits tuned | predictable resource use | always set explicit size limits — [production](practical-guide/production.html#size-and-complexity-limits) |

## Recommendations (conventions, not mandates)

- **Internal RPC:** binary protocol, framed transport, the most
  conservative server model your language offers well (thread pools where
  available). Add compact or THeader later only with a measured reason.
- **Debug/admin:** JSON protocol on a separate, unexposed port.
- **Multiplexing:** adopt when service count makes ports painful; test
  old-client behavior (Ruby's 0.24.0 default-service fallback is a
  model).
- **Large or untrusted payloads:** explicit limits (message, container,
  recursion, decompression) *and* TLS; never rely on defaults alone.
- **Production TLS:** see
  [production](practical-guide/production.html#tls) — 0.24.0 tightened
  hostname verification in Java/Python/c_glib and disabled TLS 1.0/1.1 by
  default in C++; re-test old peers on upgrade.
