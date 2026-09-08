# Generated code in Python

> **Applies to Apache Thrift 0.24.0.** Verified with CPython 3.14 and the
> `thrift` 0.24.0 package from PyPI (0.24.0 is the first release publishing
> binary wheels: CPython 3.10–3.14 on macOS x86_64/arm64, manylinux,
> musllinux, and Windows — [CHANGES, THRIFT-6070](https://github.com/apache/thrift/blob/v0.24.0/CHANGES.md)).

## Prerequisites

- **Compiler:** Apache Thrift 0.24.0.
- **Runtime:** the `thrift` package from PyPI; pure-Python works
  everywhere, with an optional C extension accelerating encode/decode on
  CPython (wheels include it; sdists build it when a toolchain exists).

```sh
python3 -m venv .venv
.venv/bin/pip install thrift==0.24.0
```

## Generation

```sh
thrift -r --gen py -o python reference_data.thrift
```

Output: `python/gen-py/refdata/{common,query}/` — per module `ttypes.py`,
`constants.py`, and per service `<Service>.py` (`Iface`, `Client`,
`Processor`, plus a `-remote` sample shell client).

Useful 0.24.0 generator options (`thrift --help`): `slots`
(`__slots__` on generated classes), `enum` (Python `IntEnum` members),
`type_hints` (requires `enum`), `package_prefix=<pkg.>` (prepend package
path), `twisted`/`tornado` (framework integration), `dynamic` (smaller
code, slower).

**Note:** put the generated directory on `sys.path` (or install it); the
generated code imports `thrift` and sibling modules by package path.

## A minimal server and client

Complete runnable files:
[`server.py`](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/python/server.py),
[`client.py`](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/python/client.py).

```python
from thrift.protocol import TBinaryProtocol
from thrift.server import TServer
from thrift.transport import TSocket, TTransport

processor  = ReferenceDataQuery.Processor(Handler())
transport  = TSocket.TServerSocket(host="127.0.0.1", port=9090)
tfactory   = TTransport.TBufferedTransportFactory()
pfactory   = TBinaryProtocol.TBinaryProtocolFactory()
server     = TServer.TSimpleServer(processor, transport, tfactory, pfactory)
server.serve()
```

```python
sock    = TSocket.TSocket("127.0.0.1", 9090)
sock.setTimeout(5000)                      # ms; applies to connect and I/O
trans   = TTransport.TBufferedTransport(sock)
client  = ReferenceDataQuery.Client(TBinaryProtocol.TBinaryProtocol(trans))
trans.open()
... ; trans.close()
```

Run and expect the standard walkthrough output (see the
[examples README](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/README.md)).
Validation status: executed end-to-end; also validated as the server for
the Go client and as the client for the Java server.

## Python-specific notes

- **Generated types.** `uuid` → `uuid.UUID` (added in 0.24.0,
  THRIFT-5923); `binary` → `bytes`; `set` → `set`; optional fields are
  `None` until set; IDL defaults materialize in
  `<module>-constants`/`__init__` values.
- **Requiredness on read.** A missing `required` field raises
  `TProtocolException`; unknown incoming fields are skipped by ID.
- **Fast path.** With the C extension present, `_fast_encode`/
  `_fast_decode` handle struct (de)serialization for buffered-compatible
  transports; the pure-Python path implements the same wire format.
- **Timeouts.** `TSocket.setTimeout(ms)` is the single knob (Python
  sockets apply it to connect and I/O alike). For header transport,
  per-direction limits are configurable; see below.
- **Limits (0.24.0).** `THeaderTransport` and `TZlibTransport` default to
  `DEFAULT_MAX_FRAME_SIZE` = 16 384 000 bytes for frames **and**
  decompressed payloads (THRIFT-6024/#3408); `TProtocol.skip` gained a
  default recursion depth limit (#3411). Binary-protocol negative sizes
  are rejected (#3404). Tune explicitly for large-payload services — see
  [production](practical-guide/production.html#size-and-complexity-limits).
- **TLS.** `TSSLSocket` / `TSSLServerSocket` with `ssl` contexts;
  0.24.0 enables hostname verification via the `sslcompat` matcher
  (#3413). Older releases that skipped verification are not a safe
  baseline.
- **Servers.** `TSimpleServer` (blocking, one connection) for tests;
  `TThreadedServer` / `TThreadPoolServer` for blocking production;
  `TNonblockingServer` for many connections; `TProcessPoolServer` for
  CPU-bound handlers (POSIX).
- **THeader.** Python is one of the four 0.24.0 runtimes with
  `THeaderProtocol`/`THeaderTransport` (with C++, Go, Node.js).

**Version-specific:** 0.23.0 restored Python 3.12+ support (THRIFT-5915);
the 0.24.0 wheel matrix makes interpreter pairing straightforward — pin
`thrift==0.24.0` and test on your target interpreter.

## Troubleshooting (Python)

| Symptom | Likely cause | Fix |
|---|---|---|
| `ModuleNotFoundError: refdata` | gen-py not on `sys.path` | insert the generated dir, or install it |
| `AttributeError: module 'thrift' has no attribute …` | an unrelated `thrift` package shadowing PyPI's | `pip` uninstall the other dist; check `thrift.__file__` |
| client hangs on first call | framed vs buffered mismatch / protocol mismatch | mirror the server stack exactly |
| `socket.timeout` under load | server concurrency too low (`TSimpleServer` serializes!) | move to `TThreadPoolServer`/nonblocking; keep client timeouts |
| `TApplicationException: Internal error` | handler raised an undeclared exception | check server logs; declare expected errors in `throws` |
| UnicodeDecodeError on strings | peer writes non-UTF-8 into `string` | fix producer; use `binary` for non-text |
