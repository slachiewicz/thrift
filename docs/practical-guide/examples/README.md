# Example suite — "Reference Data Query" service

Original example domain for *Apache Thrift: A Practical Guide*
(Apache Thrift 0.24.0). A neutral reference-data lookup service for
financial instruments: instrument catalog, quote snapshots, exchange
directory.

Generated code is **not committed**. Generate it with the Apache Thrift
0.24.0 compiler as shown below, then run each walkthrough.

## IDL files

| File | Purpose |
|---|---|
| `common.thrift` | shared vocabulary: typedefs, `MarketPhase`, `SnapshotContext`, constants |
| `reference_data.thrift` | contract v1: domain model + `ReferenceDataQuery` service |
| `reference_data_v2.thrift` | contract v2: additive evolution; audit-clean against v1 |

## Generate (compiler 0.24.0)

```sh
# compatibility check between v1 and v2 (exit 0 = compatible)
thrift --audit reference_data.thrift reference_data_v2.thrift

# schema diagram (Mermaid classDiagram)
thrift --gen mmd reference_data.thrift     # -> gen-mmd/reference_data.mmd

# Java
thrift -r --gen java:jakarta_annotations -o java reference_data.thrift

# Python
thrift -r --gen py -o python reference_data.thrift

# Go (package_prefix makes generated imports module-resolvable)
thrift -r --gen go:package_prefix=example.com/refdata/gen-go/ -o go reference_data.thrift
```

## Java (validated: JDK 25, Maven 3.10, `libthrift` 0.24.0 from Maven Central)

```sh
cd java
mvn -q compile
mvn -q exec:java -Dexec.mainClass=com.example.refdata.JavaServer &   # terminal 1
mvn -q exec:java -Dexec.mainClass=com.example.refdata.JavaClient     # terminal 2
```

Stack: `TSimpleServer` + `TServerSocket(9090)` + plain `TTransportFactory`
+ `TBinaryProtocol.Factory`.

## Python (validated: Python 3.14, `thrift` 0.24.0 wheel from PyPI)

```sh
cd python
python3 -m venv .venv && .venv/bin/pip install thrift==0.24.0
.venv/bin/python server.py &    # terminal 1
.venv/bin/python client.py      # terminal 2
```

Stack: `TSimpleServer` + `TServerSocket(127.0.0.1:9090)` +
`TBufferedTransportFactory` + `TBinaryProtocolFactory`.

## Go (validated: Go 1.27, module `github.com/apache/thrift` v0.24.0)

```sh
cd go
go run ./server &    # terminal 1
go run ./client      # terminal 2
```

Stack: Go's single server type `TSimpleServer` + `TServerSocket` +
`TBufferedTransportFactory(8192)` + `TBinaryProtocolFactoryDefault`.

## Cross-language (validated pairs)

- Python client ↔ Java server (buffered client vs plain server transport — compatible)
- Go client ↔ Python server
- (also validated: Java↔Java, Python↔Python, Go↔Go)

## Expected client output (all walkthroughs)

```
getInstrument: AAPL Apple Inc. XNAS
findInstruments: 1 match(es)          # DTE (the only EUR-quoted instrument)
latestSnapshot last: 102.0            # MSFT, deterministic pseudo-price
exchangeDirectory XNAS: NASDAQ Stock Market
LookupError as expected: unknown symbol: NOPE
InvalidQuery as expected: selector must set exactly one field
```
