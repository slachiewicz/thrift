# Generated code in Java

> **Applies to Apache Thrift 0.24.0.** Verified with OpenJDK 25, Maven
> 3.10, and `org.apache.thrift:libthrift:0.24.0` from Maven Central.

## Prerequisites

- **Compiler:** Apache Thrift 0.24.0 (`thrift --version`).
- **JDK:** a current LTS (11 or later is the practical range; the example
  was validated on JDK 25).
- **Build tool:** Maven or Gradle. `lib/java` itself is built with Gradle
  8, but consumers typically use the published artifact.

## Dependency

```xml
<dependency>
  <groupId>org.apache.thrift</groupId>
  <artifactId>libthrift</artifactId>
  <version>0.24.0</version>
</dependency>
```

`libthrift` brings SLF4J 1.x API; add a binder such as
`org.slf4j:slf4j-simple:1.7.36` to see server logs.

## Generation

```sh
thrift -r --gen java:jakarta_annotations -o . reference_data.thrift
```

- `-r` also generates included files (`common.thrift`).
- `-o .` writes to `./gen-java/` (without `-o`, to `./gen-java` of the
  current dir; with `-out <dir>`, files land directly in `<dir>`).
- **`jakarta_annotations`** emits `@jakarta.annotation.Generated` instead
  of the `javax.annotation` type removed from modern JDKs — on JDK 11+
  this avoids a familiar compile error. Use
  `generated_annotations=suppress` if you prefer no annotation at all.
- Other useful 0.24.0 options (`thrift --help`): `beans` (private fields +
  void setters), `option_type=jdk8|thrift` (wrap optionals),
  `rethrow_unhandled_exceptions`, `sorted_containers`,
  `fullcamel`, `unsafe_binaries` (no `ByteBuffer` copies — mind
  mutability), `annotations_as_metadata`.

Generated layout: `gen-java/<namespace path>/<Type>.java` and, per
service, `<Service>.java` containing `Iface`, `Client`, `Processor`,
plus argument/result structs (`getInstrument_args`, …).

## Build integration (Maven)

Register the generated sources and compile
([full pom](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/java/pom.xml)):

```xml
<plugin>
  <groupId>org.codehaus.mojo</groupId>
  <artifactId>build-helper-maven-plugin</artifactId>
  <version>3.6.0</version>
  <executions>
    <execution>
      <id>add-generated-sources</id>
      <phase>generate-sources</phase>
      <goals><goal>add-source</goal></goals>
      <configuration><sources><source>gen-java</source></sources></configuration>
    </execution>
  </executions>
</plugin>
```

**Note:** for real projects, invoke the compiler from the build
(`exec-maven-plugin` → `thrift` binary, or a Maven Thrift plugin of your
choice) so regeneration is reproducible in CI.

## A minimal server and client

Runnable, complete implementations are in the example suite:
[`JavaServer.java`](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/java/src/main/java/com/example/refdata/JavaServer.java)
and
[`JavaClient.java`](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/java/src/main/java/com/example/refdata/JavaClient.java).
The stack (deliberately conservative):

```java
TServerSocket serverSocket = new TServerSocket(9090);
TServer server = new TSimpleServer(new TServer.Args(serverSocket)
    .processor(new ReferenceDataQuery.Processor(handler))
    .transportFactory(new TTransportFactory())      // plain streaming
    .protocolFactory(new TBinaryProtocol.Factory()));
server.serve();
```

```java
TTransport transport = new TSocket("127.0.0.1", 9090, 5000); // 5 s I/O timeout
transport.open();
ReferenceDataQuery.Client client =
    new ReferenceDataQuery.Client(new TBinaryProtocol(transport));
```

Run:

```sh
mvn -q compile
mvn -q exec:java -Dexec.mainClass=com.example.refdata.JavaServer   # terminal 1
mvn -q exec:java -Dexec.mainClass=com.example.refdata.JavaClient   # terminal 2
```

Expected client output is listed in the
[examples README](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples/README.md).
Validation status: executed end-to-end (server+client), plus cross-language
against the Python server.

## Java-specific notes

- **Generated types.** `uuid` → `java.util.UUID`; `binary` →
  `ByteBuffer` (copies by default; `unsafe_binaries` opts out); `set` →
  `HashSet` (`sorted_containers` → `TreeSet`).
- **isset tracking.** Optional fields have `isSetX()`/`unsetX()`; only set
  fields are written. `required` fields are validated on read/write —
  missing required fields raise `TProtocolException`.
- **Fluent setters.** Default generation returns `this` from setters
  (`new InstrumentQuery().setLimit(10)`). The `beans` option switches to
  JavaBean style.
- **Timeouts.** `TSocket(host, port, timeout)` sets the I/O timeout;
  `setConnectTimeout(ms)` the connect phase. Size limits live in
  `org.apache.thrift.TConfiguration` (defaults: 100 MiB message,
  16 384 000-byte frame, recursion limit 64) — see
  [production](practical-guide/production.html).
- **TLS.** `TSSLTransportFactory` /
  `TNonblockingSSLSocket`. 0.24.0 enables **hostname verification** in
  these paths ([CHANGES](https://github.com/apache/thrift/blob/v0.24.0/CHANGES.md),
  #3390/#3396) — stricter than older releases; stale certificates or
  hostname mismatches that previously connected now fail (by design).
- **Servers.** `TSimpleServer` (one connection) is for tests; prefer
  `TThreadPoolServer` for blocking production traffic, or the nonblocking
  selectors (`TThreadedSelectorServer`) for many idle connections.
- **No THeader in Java 0.24.0.** Java has no `THeaderTransport`/
  `THeaderProtocol`; header-transport fleets need C++/Go/Python/Node peers
  or an alternative edge.
- **Known footguns.** `ByteBuffer` sharing (see `unsafe_binaries`), and
  forgetting `transport.open()`/`close()` are the two most common
  first-week bugs.

## Troubleshooting (Java)

| Symptom | Likely cause | Fix |
|---|---|---|
| `package javax.annotation does not exist` on generated code | generated without `jakarta_annotations` on JDK 11+ | regenerate with `--gen java:jakarta_annotations` + `jakarta.annotation-api` dep |
| client hangs, times out on first call | transport mismatch (framed vs unframed) or protocol mismatch | align both ends; see [runtime choices](practical-guide/runtime-selection.html) |
| `TTransportException: Read timed out` | server slow vs your timeout, or wrong port | measure handler latency; set explicit timeouts on both phases |
| `TProtocolException: Required field 'x' was not found` | peer writes an older/newer schema missing a `required` field | contract drifted across deployments; re-audit IDL versions |
| SSL handshake fails after upgrade | 0.24.0 hostname verification now on | fix certificates/hostnames (do not disable verification) |
