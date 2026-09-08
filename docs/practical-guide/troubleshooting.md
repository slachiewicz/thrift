# Diagnostics and troubleshooting

> **Applies to Apache Thrift 0.24.0.** Symptom → likely causes →
> diagnostic steps → remediation, for the failure modes that account for
> most real-world Thrift pain. Layer attribution follows the
> [mental model](practical-guide/mental-model.html): wire, compiler,
> runtime, or application.

## Connection-level symptoms

### Symptom: client hangs, then times out on the first call

**Likely causes** (in order of frequency):
1. **Transport mismatch** — framed on one side, unframed on the other
   (the receiver reads the 4-byte length prefix as message data).
2. **Protocol mismatch** — compact client against binary server.
3. Server model saturated — e.g. Go's `TSimpleServer` or Python's
   `TSimpleServer` busy with a previous connection.

**Diagnostics:** capture the first bytes from the client (framed begins
with a plausible little-endian length; unframed binary begins with the
protocol's message-type bytes); check both sides' transport/protocol
factory lines; check server concurrency.

**Remediation:** mirror the stacks exactly
([runtime choices](practical-guide/runtime-selection.html)); for
saturation, use a concurrent server model or add capacity.

### Symptom: `TTransportException: EOF` / connection reset immediately after connect

**Likely causes:** server process died on accept (check its log);
TLS-vs-plaintext mismatch (plaintext client → TLS server port);
firewall/proxy with short idle timeouts; framed client against plain
HTTP endpoint.

**Diagnostics:** `telnet host port` — a TLS port answers with a TLS
alert/garbage to plaintext probes; try swapping the client's transport to
TLS or plain accordingly; check server logs at error level.

**Remediation:** match TLS on both ends; for idle-timeout kills, enable
TCP keepalive (Python `socket_keepalive=True`, others per-runtime) or
proxy-level idle exemptions.

### Symptom: works locally, fails through a proxy/LB

**Likely causes:** LB health-check speaking partial Thrift (confuses
blocking servers); proxy buffering framing; connection reuse across
handlers that assume session state.

**Remediation:** use HTTP transport class through proxies, or
TCP-pass-through LB config; make handlers stateless; align timeouts with
LB idle policy.

## Protocol/message-level symptoms

### Symptom: `TProtocolException: Required field 'x' was not present`

**Meaning:** the *reader's* schema marks field `x` `required`; the
*writer* didn't send it. **This is deployment skew** — two sides running
different contract revisions.

**Diagnostics:** log the peer identity on connect; diff the deployed
IDL/generation stamps; run `thrift --audit old new` between them.

**Remediation:** redeploy in the compatibility-safe order (additive
fields only); see [compatibility](practical-guide/compatibility.html).
Do not "fix" by demoting `required` casually — old readers still demand
the field.

### Symptom: `TApplicationException: Internal error` on one method only

**Meaning:** the handler raised an exception *not declared* in `throws`;
the runtime converted it to the generic error.

**Diagnostics:** server log at the moment of the call (most runtimes log
the underlying exception); reproduce the handler path with hostile
inputs.

**Remediation:** declare the expected error type in `throws`; reserve
`TApplicationException` for true surprises. Note: a `oneway` method can
never deliver this signal — see below.

### Symptom: client method returns but server never did the work (or errors vanish)

**Likely cause:** the method is `oneway`. No reply is expected; handler
exceptions are not propagated; delivery is best-effort. (Also possible:
an HTTP-transport oneway quirk — 0.24.0 fixed the C++ HTTP client to
*not* wait for a response on oneway, THRIFT-6021; older pairings may
hang or misbehave.)

**Remediation:** use oneway only for loss-tolerant notifications; for
anything else use a normal method
([IDL design, oneway](practical-guide/idl-design.html#oneway--use-rarely-know-exactly-what-you-are-buying)).

### Symptom: wrong data after a schema change (values shifted between fields)

**Likely cause:** field ID reuse/renumbering, or a type change under the
same ID. The wire matched IDs, so data lands in the "wrong" variable
without any error.

**Diagnostics:** `thrift --audit` the two contract revisions; inspect the
evolution history for the affected struct.

**Remediation:** revert the reuse; add a reserved-ID comment; publish a
correct additive revision.

## Schema/generation symptoms

### Symptom: compiler error `INVALID TYPE IN type_to_enum` or "unsupported base type uuid"

**Meaning:** the IDL uses `uuid` and the **target generator doesn't
support it**. Verified 0.24.0: rejected by dart, erlang, perl, ocaml, d;
c_glib aborts ("no C base type name for base type uuid").

**Remediation:** keep `uuid` only in contracts whose targets all support
it ([IDL design](practical-guide/idl-design.html#uuid-is-a-base-type--but-not-everywhere));
otherwise use `binary` with a documented canonical encoding.

### Symptom: generated code doesn't compile

| Error | Cause | Fix |
|---|---|---|
| Java: `package javax.annotation does not exist` | JDK 11+ lacks `javax.annotation` | regenerate `--gen java:jakarta_annotations` (+ jakarta dependency) |
| Go: `cannot use x (string) as common.MicCode` | Go typedefs are named types | convert explicitly |
| Go: included-file imports don't resolve | generated without `package_prefix` | regenerate with the option |
| Python: `ModuleNotFoundError` for generated pkg | gen dir not on path | fix `sys.path`/packaging |
| PHP 7.x: syntax errors in generated/runtime code | 0.24.0 requires PHP ≥ 8.1 (strict types, native types) | upgrade PHP or pin the older runtime (plan migration) |

### Symptom: `i64` values wrong only for JavaScript clients

**Cause:** JS `Number` precision above 2^53. **Fix:** Node clients:
regenerate with `js:bigint` (0.24.0 opt-in); otherwise encode large IDs
as `string` in JS-facing contracts. See
[other targets](practical-guide/languages/other-targets.html#nodejs-and-javascript-lib_nodejs-lib_js-lib_ts).

## Runtime/resource symptoms

### Symptom: server dies / thrashes under hostile or large input

**Likely causes (0.24.0-specific):** you *upgraded* from an older
release, and the new limits (recursion depth 64, 16 384 000-byte frames,
negative-size rejection, varint bounds) now correctly reject traffic your
old build silently accepted — **or** you predate limits and are seeing
resource exhaustion.

**Diagnostics:** match the exception class (`TProtocolException` size/
depth violations = limits working); check payload sizes against
[TConfiguration](practical-guide/production.html#size-and-complexity-limits)
defaults; check whether a proxy is fragmenting frames.

**Remediation:** tune limits explicitly to real payload profiles; never
disable limits wholesale; validate container cardinality in handlers.

### Symptom: TLS connections fail after runtime upgrade

**Cause (0.24.0):** hostname verification enabled in Java/Python/c_glib
client paths; C++ disables TLS 1.0/1.1 by default; wildcard matching
enforces RFC 6125 placement.

**Remediation:** fix certificates/hostnames/TLS versions. Verification
failures are the feature, not a bug
([production, TLS](practical-guide/production.html#tls)).

### Symptom: performance cliff after an upgrade

**First suspects:** new validation/limits on hot paths (measure, don't
assume); changed default server model; regenerated-code API changes
forcing extra copies (e.g. Java buffer handling). Re-run the
[performance method](practical-guide/production.html#performance-engineering)
— handler time vs wire time — before rolling back.

## A 5-minute triage script

1. Which layer? (connection / message / schema / resource)
2. Do both sides log? Get server-side truth first.
3. Same contract revision? (`--audit`, generation stamps)
4. Same protocol *and* transport factory lines on both ends?
5. Limits and timeouts explicit on both ends?
6. Reproduce with the walkthrough clients (they are known-good): if a
   known-good client fails, the server is wrong; if it succeeds, the
   suspect client's stack/config is wrong.
