# Production concerns

> **Applies to Apache Thrift 0.24.0.** This chapter is about operating
> Thrift services: resource limits, TLS, deadlines, retries, error
> taxonomy, observability, deployment, and performance engineering. Items
> marked *defaults* name 0.24.0 behavior; everything else is guidance.

## Size and complexity limits

Unbounded input is the default failure mode of RPC systems. Thrift
0.24.0 exposes layered limits (all defaults verified in the source tree):

| Limit | Default | Where |
|---|---|---|
| max message size | **100 MiB** (`DEFAULT_MAX_MESSAGE_SIZE`) | Java `TConfiguration`, Go `TConfiguration`; per-transport knobs elsewhere |
| max frame size (framed/header/zlib) | **16 384 000 bytes** ("used consistently across all Thrift libraries") | Java, Go, Python `THeaderTransport`/`TZlibTransport` (aligned to this value in 0.24.0, THRIFT-6024) |
| decompressed payload size | 16 384 000 bytes default (Python header/zlib; #3408) | compression-enabled paths |
| struct read/write recursion depth | **64** (`DEFAULT_RECURSION_DEPTH`) | Java; 0.24.0 added equivalent depth limits to Go, Python skip, Ruby, PHP, Rust, D, Dart, Delphi, Erlang, Haxe, JavaME, JS, Kotlin, Lua, OCaml, Perl, Smalltalk, c_glib |
| compact-protocol varint length | byte-count-bounded readers | all major runtimes (#3410) |
| container element counts | validated against int32 / message bounds (e.g. Go JSON #3604, THRIFT-6071; Ruby clients THRIFT-6025) | per-runtime |

Guidance:

- **Treat defaults as a floor, not a policy.** For services facing
  untrusted input, set explicit, *small* limits: the wire can describe a
  100 MiB map far more cheaply than your handler can ingest it.
- **Validate application-level cardinality too.** The example contract's
  `latestSnapshots(instrumentIds, maxResults)` deliberately documents a
  handler-side cap — the IDL cannot express "too many".
- **Recursion.** Deeply nested structs (or recursive types) can exhaust
  stacks; 0.24.0's depth limits convert that into a protocol exception.
  If you legitimately need deep nesting, raise limits deliberately and
  test stack behavior on your smallest deployment platform.
- **Compression bombs.** zlib-wrapped endpoints inherit the decompressed
  size limit; keep it enabled and sized for your real payloads.

**Version-specific:** several of these limits changed *behavior* in
0.24.0 (recursion limits are new in many runtimes; Python frame defaults
moved from a larger hard max to 16 384 000). Services that "worked" with
huge frames before may now correctly reject oversized peers — re-run
large-payload tests when upgrading.

## TLS

- **Have it.** Internal traffic is traffic; plaintext sockets are the
  exception to justify, not the rule.
- **0.24.0 tightening.** Java: hostname verification enabled in
  `TSSLTransportFactory` and `TNonblockingSSLSocket` (#3390, #3396).
  Python: hostname verification via the `sslcompat` matcher in
  `TSSLSocket` (#3413). c_glib: peer hostname validation added (#3507).
  C++: TLS 1.0/1.1 disabled by default (THRIFT-3165) and RFC 6125
  wildcard placement enforced (#3506).
- **Consequence:** after upgrading, peers with mismatched hostnames or
  legacy TLS versions will fail — by design. Fix certificates and
  protocol levels; resist disabling verification.
- **Rust gap:** the Rust runtime has no TLS transport in 0.24.0 —
  terminate TLS in a sidecar/proxy for Rust services.
- **Authn beyond TLS:** Thrift defines SASL transports in a few runtimes
  (Java `TSasl*`) but no standard per-service authorization story. Treat
  authz as an application/mesh concern and document the boundary in your
  threat model. See the project's
  [threat model](https://github.com/apache/thrift/blob/v0.24.0/doc/thrift-threat-model.md)
  and [SECURITY.md](https://github.com/apache/thrift/blob/v0.24.0/SECURITY.md)
  for the documented trust boundaries.

## Timeouts, deadlines, retries, idempotency

- **Set every timeout explicitly.** Defaults are frequently "infinite":
  - Java: `TSocket(host, port, timeoutMs)` for I/O,
    `setConnectTimeout(ms)` for connect.
  - Python: `TSocket.setTimeout(ms)` (connect and I/O).
  - Go: `SetConnTimeout`, `SetSocketTimeout` (and per-channel timeouts in
    Rust 0.24.0, THRIFT-5954).
- **Propagate deadlines.** Go's context-aware generated methods make
  end-to-end deadlines natural; in other languages, carry a deadline in
  your request struct (the example's `asOf`-style pattern generalizes to
  a `deadline` metadata field) and enforce it in handlers.
- **Retries are an application policy.** The only wire-level rule is:
  `oneway` calls must **not** be retried blindly (the server may have
  processed them), and only *declared* exceptions are safely catchable —
  an `TApplicationException` may mean "method unknown" (safe to retry
  against a newer server) or "handler crashed" (safe only if the method
  is idempotent).
- **Idempotency.** Classify every method: read-only (retry freely),
  idempotent writes (retry with dedup key), non-idempotent (retry only
  with an explicit idempotency key in the contract). Thrift has no
  built-in dedup; design the key into the IDL when needed.
- **Backpressure.** Blocking servers (thread pools) need bounded queues
  and pool sizing tied to handler latency; nonblocking servers need
  frame-size and connection limits. The example's `limit` parameter
  pattern (capped in the handler) is the contract-level complement.

## Input validation

The wire gives you types and requiredness, nothing else. Verified
0.24.0 additions (`go_validator_generator`, `annotations_as_metadata`,
PHP `validate` option) show the project's direction of making
annotation-driven validation possible, but today validation remains
application work:

- range-check numerics (a `double price` can be `-1e308`)
- enumerate-allow list strings/IDs before storage or fan-out
- check container sizes and map key sets
- never trust `optional`-absence as "user said no"; absence is
  indistinguishable from "old client that predates the field"

## Error taxonomy

1. **Declared exceptions** (`throws`) — part of the contract; typed in
   every language; use for *expected* failures (`LookupError`,
   `InvalidQuery`).
2. **`TApplicationException`** — the runtime's generic error channel:
   unknown method, unknown message type, handler crash wrapper. Catch,
   log, and classify; do not encode meaning into it.
3. **`TTransportException`** — connection-level trouble (EOF, timeout,
   not open). Usually retry-with-backoff territory *if* idempotent.
4. **`TProtocolException`** — schema violations (missing required field,
   bad type tag). Almost always a deployment-skew bug; page an engineer,
   don't retry.

Reserve "required" for identity so that #4 stays rare; keep declared
exceptions small and stable so clients can branch on them.

## Observability

Thrift runtimes do not ship a metrics story; standard practice:

- **Hook points:** `TServerEventHandler` (Java: connection/context
  lifecycle, client IP since 0.23.0), processor wrappers (time every
  method), transport factories (connection counts), and your handler
  layer (business metrics).
- **Instrument per method:** request rate, error rate split by exception
  class, latency histogram (handler time vs transport time — time inside
  the handler *and* around the client call; the difference is runtime
  overhead and queueing).
- **Trace propagation:** no standard wire header; propagate trace IDs in
  an application metadata struct, or terminate traces at the edge proxy.
- **Log the method name + sequence id** on errors: the sequence id links
  request/response pairs when debugging hung connections.

## Deployment and lifecycle

- **Regenerate at build time; commit only IDL.** Generated code is a
  build artifact (all walkthroughs in the example suite follow this).
- **Version-skew tolerance is a deployability feature.** Services that
  accept ±N versions of the contract deploy without coordination; keep
  the [compatibility checklist](practical-guide/compatibility.html) in CI
  to preserve that property.
- **Graceful shutdown:** stop accepting, drain in-flight calls (bounded
  by your timeouts), then close listeners. Simple servers serve
  `accept()` until killed — wrap them with signal handling.
- **Upgrade the runtime like a dependency:** read
  [CHANGES.md](https://github.com/apache/thrift/blob/v0.24.0/CHANGES.md)
  for the target release (security + limits sections especially),
  regenerate, diff, and run your cross-language matrix. Recent releases
  tightened defaults (TLS verification, size limits, recursion) — the
  changes are the point; test for them.

## Performance engineering

**Measure, in this order:** handler latency → serialization cost →
transport overhead → server model effects. Optimizing before measuring
is how teams switch to compact protocol for a service whose bottleneck
is a slow disk.

- **Protocol size vs CPU:** compact protocol shrinks integer-heavy
  payloads (varints) at modest CPU cost; binary is the predictable
  baseline. Measure *your* payload distribution — a 20% wire saving on
  200-byte messages rarely matters; on 200 KiB blobs it does.
- **Allocations:** Java allocation pressure (per-call structs, `ByteBuffer`
  copies — `unsafe_binaries` trades safety for copies), Python's
  interpreter overhead (the C extension helps; `slots`/`enum` generator
  options reduce per-object cost), Go's low-allocation steady state.
  Profile per language; do not transfer conclusions between runtimes.
- **Connection reuse:** amortize handshakes (TLS especially). Pool
  clients; keep them alive; respect `TSocket` keepalive options (Python
  `socket_keepalive`).
- **Server models:** match pool size to handler profile (I/O-bound wants
  more concurrency than pool threads; CPU-bound wants pool ≈ cores —
  Python's `TProcessPoolServer` exists for this).
- **Benchmark design pitfalls:**
  - benchmark the *stack*, not the loop: include real framing, timeouts,
    and payload shapes;
  - warm up JIT (Java) and C-extension paths (Python) before measuring;
  - keep payloads representative (int-heavy vs string-heavy flips the
    binary/compact verdict);
  - never report one language's numbers as Thrift's numbers —
    cross-language comparisons measure mostly the languages;
  - state limits explicitly in benchmark configs, since defaults differ
    per runtime.
