# A first end-to-end service

> **Applies to Apache Thrift 0.24.0.**

This page builds and runs a complete Thrift service — contract, server,
client — in about ten minutes, using Python for speed of iteration. The
identical contract is later served and consumed in Java and Go in the
[languages chapter](practical-guide/languages/java.html); everything here
transfers.

What you need: the Apache Thrift 0.24.0 compiler (`thrift --version` must
print `Thrift version 0.24.0`; see [installation](/install)) and Python 3
with `pip` (`pip install thrift==0.24.0` — 0.24.0 publishes wheels for
CPython 3.10–3.14 on the common platforms).

## 1. A small contract

`hello_refdata.thrift` — a deliberately tiny slice of the guide's
Reference Data domain:

```thrift
namespace py firstservice

// A market identifier code, e.g. "XNAS".
typedef string MicCode

struct Exchange {
  1: required MicCode mic,
  2: required string name
}

exception LookupError {
  1: required string reason
}

service ExchangeDirectory {
  Exchange getExchange(1: MicCode mic) throws (1: LookupError notFound),

  list<Exchange> allExchanges()
}
```

Three things to notice — each is explained in depth in
[IDL design](practical-guide/idl-design.html):

- every field has an explicit **field ID** (`1:`, `2:`) — its permanent
  wire identifier;
- `required` means a reader of the struct demands the field to be present;
- `throws` declares a **typed exception** that becomes part of the
  service contract.

## 2. Generate

```sh
thrift --gen py hello_refdata.thrift
```

Output lands in `gen-py/firstservice/`:

| File | Role |
|---|---|
| `ttypes.py` | `Exchange`, `LookupError`, with `read`/`write` methods |
| `constants.py` | IDL constants (none here) |
| `ExchangeDirectory.py` | `Iface`, `Client`, `Processor`, `Processor(I).process_*` |

The generated `Iface` is the contract your handler must satisfy:

```python
class Iface:
    def getExchange(self, mic): ...
    def allExchanges(self): ...
```

## 3. Implement the handler and server

`server.py`:

```python
import sys
sys.path.insert(0, "gen-py")

from firstservice import ExchangeDirectory
from firstservice.ttypes import Exchange, LookupError
from thrift.protocol import TBinaryProtocol
from thrift.server import TServer
from thrift.transport import TSocket, TTransport


class Handler(object):
    EXCHANGES = {
        "XNAS": "NASDAQ Stock Market",
        "XNYS": "New York Stock Exchange",
        "XETR": "Deutsche Boerse Xetra",
    }

    def getExchange(self, mic):
        name = self.EXCHANGES.get(mic)
        if name is None:
            raise LookupError(reason="unknown MIC: %s" % mic)
        return Exchange(mic=mic, name=name)

    def allExchanges(self):
        return [Exchange(mic=m, name=n) for m, n in sorted(self.EXCHANGES.items())]


processor = ExchangeDirectory.Processor(Handler())
transport = TSocket.TServerSocket(host="127.0.0.1", port=9090)
tfactory = TTransport.TBufferedTransportFactory()
pfactory = TBinaryProtocol.TBinaryProtocolFactory()
server = TServer.TSimpleServer(processor, transport, tfactory, pfactory)
print("serving on 127.0.0.1:9090")
server.serve()
```

The four server ingredients map exactly onto the
[mental model](practical-guide/mental-model.html): processor, socket,
transport wrapper, protocol.

## 4. The client

`client.py`:

```python
import sys
sys.path.insert(0, "gen-py")

from firstservice import ExchangeDirectory
from firstservice.ttypes import LookupError
from thrift.protocol import TBinaryProtocol
from thrift.transport import TSocket, TTransport

sock = TSocket.TSocket("127.0.0.1", 9090)
sock.setTimeout(5000)  # milliseconds
trans = TTransport.TBufferedTransport(sock)
client = ExchangeDirectory.Client(TBinaryProtocol.TBinaryProtocol(trans))

trans.open()
try:
    print(client.getExchange("XNAS"))
    print(client.allExchanges())
    try:
        client.getExchange("XXXX")
    except LookupError as e:
        print("typed error:", e.reason)
finally:
    trans.close()
```

The client stack **mirrors the server stack**: buffered transport, binary
protocol, in the same order. Change one side and the call hangs — see
[troubleshooting](practical-guide/troubleshooting.html#symptom-hangs).

## 5. Run

```sh
pip install thrift==0.24.0
python3 server.py &        # prints: serving on 127.0.0.1:9090
python3 client.py
```

Expected output:

```
Exchange(mic='XNAS', name='NASDAQ Stock Market')
[Exchange(mic='XETR', name='Deutsche Boerse Xetra'), Exchange(mic='XNAS', name='NASDAQ Stock Market'), ...]
typed error: unknown MIC: XXXX
```

## 6. Follow the data path once

Set a breakpoint (or add prints) in `gen-py/firstservice/ttypes.py`
`Exchange.write()` and in the server's `process_getExchange`, run the
client again, and watch:

1. the client stub writes message `getExchange` type `CALL`;
2. the argument struct writes field `1` (`mic`) with its type;
3. `flush()` releases the buffered bytes;
4. the server processor reads the method name, dispatches to your handler;
5. your `Exchange` result is written back with message type `REPLY`.

Once this path is familiar, everything else in Thrift is configuration
around it.

## Cross-language — same bytes

Because the wire format is language-independent, a Java or Go client
generated from the *same file* talks to this Python server with no
changes. The [examples suite](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples)
does exactly this with the full Reference Data contract, including
validated Java↔Python and Go↔Python pairings.

## Where to next

- Turn the contract into something durable:
  [designing durable IDL contracts](practical-guide/idl-design.html)
- See the same walkthrough with real build systems:
  [Java](practical-guide/languages/java.html),
  [Python](practical-guide/languages/python.html),
  [Go](practical-guide/languages/go.html)
