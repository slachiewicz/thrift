# Generated code in Go

> **Applies to Apache Thrift 0.24.0.** Verified with Go 1.27 and the
> runtime library provided by module `github.com/apache/thrift` v0.24.0
> (its `go.mod` requires Go ≥ 1.25).

## Prerequisites

- **Compiler:** Apache Thrift 0.24.0.
- **Runtime:** no separate install — the runtime is a *package inside the
  Thrift module*:

```sh
# in your module
go get github.com/apache/thrift@v0.24.0
# import path in code:
import thrift "github.com/apache/thrift/lib/go/thrift"
```

## Generation

```sh
thrift -r --gen go:package_prefix=example.com/refdata/gen-go/ -o go reference_data.thrift
```

- Output: `go/gen-go/refdata/{common,query}/*.go` — package per
  `namespace go` path, plus `reference_data.go` holding types, client,
  processor, and a `-remote` sample binary directory.
- **`package_prefix`** is the key option in module-based projects: it is
  prefixed to cross-file imports (here `common` becomes
  `example.com/refdata/gen-go/refdata/common`), making generated code
  resolve inside your module. Without it, imports use the bare
  `namespace go` path and only compile under GOPATH-style layouts.
- Other options (`thrift --help`): `thrift_import=`, `package=`,
  `ignore_initialisms`, `read_write_private`, `skip_remote`.
- 0.24.0 emits **gofmt-compatible** code (THRIFT-6011) and enforces
  connection liveness checks in `TSocket` including TLS
  (THRIFT-5214/5969).

## A minimal server and client

Complete runnable files:
[`server/main.go`](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/go/server/main.go),
[`client/main.go`](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/go/client/main.go).

```go
processor := query.NewReferenceDataQueryProcessor(handler)
transport, _ := thrift.NewTServerSocket("127.0.0.1:9090")
server := thrift.NewTSimpleServer4(processor, transport,
    thrift.NewTBufferedTransportFactory(8192),
    thrift.NewTBinaryProtocolFactoryDefault())
server.Serve()
```

```go
sock, _ := thrift.NewTSocket("127.0.0.1:9090")
sock.SetConnTimeout(5 * time.Second)
sock.SetSocketTimeout(5 * time.Second)
useTransport, _ := thrift.NewTBufferedTransportFactory(8192).GetTransport(sock)
client := query.NewReferenceDataQueryClientFactory(useTransport,
    thrift.NewTBinaryProtocolFactoryDefault())
useTransport.Open()
// ... defer useTransport.Close()
```

Contexts: every generated method takes `context.Context` first
(`client.GetInstrument(ctx, selector)`), and handlers receive it too —
propagate deadlines from clients into handlers.

Run:

```sh
go run ./server &    # terminal 1
go run ./client      # terminal 2
```

Expected output is in the
[examples README](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/README.md).
Validation status: executed end-to-end; also validated against the Python
server (cross-language).

## Go-specific notes

- **Typedefs are named types.** `typedef string MicCode` yields
  `type MicCode string`; assignment from `string` requires conversion
  (`common.MicCode(mic)`). This surfaces as compile errors the first
  time — by design.
- **Optionality is pointers.** `optional` fields are pointers
  (`*string`); "set" means non-nil. Required fields are plain values.
- **`uuid` → `thrift.Tuuid`** (`[16]byte` value type): parse with
  `thrift.ParseTuuid("…")` (or the generic `thrift.Must` helper), print
  with `.String()`. Because it is a value type there is no nil — an
  all-zero `Tuuid` is the zero value, which matters for unions (below).
- **Unions are not enforced.** Generated Go writes *all* union members,
  including zero values (verified in 0.24.0; see
  [IDL design](practical-guide/idl-design.html#structs-unions-exceptions)).
  Handlers on every language must validate "exactly one non-empty"
  leniently. Prefer plain structs with optional fields when Go is a
  participant.
- **Servers.** 0.24.0 ships exactly **one** server type,
  `TSimpleServer` — it serves one connection at a time. For concurrent
  workloads, run the generated server behind your own accept loop, scale
  with process managers, or wrap the processor with a
  connection-handling strategy of your own; do not expect Java-style
  thread-pool/selector servers.
- **Limits.** `thrift.TConfiguration` defaults: 100 MiB max message,
  16 384 000-byte max frame; recursion-depth checks added in 0.24.0
  (THRIFT-6044). The JSON protocol validates container sizes on read
  (#3604, THRIFT-6071). See
  [production](practical-guide/production.html#size-and-complexity-limits).
- **TLS.** `thrift.NewTSSLSocket` / `NewTSSLServerSocket` wrap
  `tls.Config`; connection checks work over TLS (THRIFT-5969). Configure
  `MinVersion`, roots, and server name explicitly — library defaults are
  conservative.
- **Feature detection.** Go has binary, compact, header, JSON and
  simple-JSON protocols, framed/buffered/http/zlib transports, and
  multiplexed protocol support — but remember the single server type.

## Troubleshooting (Go)

| Symptom | Likely cause | Fix |
|---|---|---|
| `cannot use mic (string) as common.MicCode` | typedef named types | convert: `common.MicCode(s)` |
| imports of included files don't resolve | missing `package_prefix` at generation time | regenerate with `go:package_prefix=<module>/gen-go/` |
| server serves one client at a time | `TSimpleServer` is single-connection by design | add concurrency around it / scale processes |
| `EOF` on first call | transport mismatch (framed vs not) | align stacks on both ends |
| handlers never see context cancellation | deadlines not propagated | pass `ctx` from client through to handler work |
