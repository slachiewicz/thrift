# Apache Thrift: A Practical Guide

> **Applies to Apache Thrift 0.24.0.** Behavior, defaults, and generator
> options described in this guide were verified against release 0.24.0.
> Other releases may differ.

Welcome. This guide takes you from "what is a Thrift IDL file, really" to
"how do I run a multi-language Thrift service that survives schema changes,
size limits, and version upgrades." It is written for people who understand
programming, networking, and services, but not Thrift-specific terminology.

## What this guide is

A practical, end-to-end companion to the formal reference documentation.
It complements — does not replace — the canonical pages:

- [Thrift type system](/docs/types)
- [IDL specification](/docs/idl)
- [Language and feature matrix](/docs/Languages)
- [Tutorial](/tutorial)
- [Installation](/install)

Where this guide makes a claim about behavior, it was checked against the
0.24.0 source tree and, where feasible, executed. Where behavior is
implementation-defined or varies per language, the guide says so.

## Scope and support disclaimer

- **Version scope:** Apache Thrift 0.24.0 only. Features from other
  releases are out of scope.
- **Language scope:** deep coverage of Java, Python, and Go; summary
  coverage of Node.js/JavaScript, Rust, PHP, and others in
  [Other targets](practical-guide/languages/other-targets.html).
- **Not exhaustive:** the [language matrix](/docs/Languages) is the
  canonical (but version-lagged — see its header) source for feature
  availability; per-library READMEs under `lib/` are canonical for
  runtime details.
- **Not a replacement for testing:** cross-language behavior must be
  validated on your own contract, toolchain, and traffic. The guide marks
  its own validation status where relevant.

## Contents

1. [The Thrift mental model](practical-guide/mental-model.html) — IDL, compiler,
   generated code, protocols, transports, servers; where interoperability is
   guaranteed and where it is conditional.
2. [A first end-to-end service](practical-guide/first-service.html) — one
   small contract, one server, one client, in about ten minutes.
3. [Designing durable IDL contracts](practical-guide/idl-design.html) —
   field IDs, requiredness, namespaces, types, `uuid`, unions, `oneway`.
4. [Compatibility and schema evolution](practical-guide/compatibility.html) —
   safe vs unsafe changes, the compatibility matrix, `thrift --audit`.
5. [Runtime choices](practical-guide/runtime-selection.html) — protocols,
   transports, server models, framing, TLS, multiplexing; decision tables.
6. [Working with generated code](practical-guide/languages/java.html):
   [Java](practical-guide/languages/java.html) ·
   [Python](practical-guide/languages/python.html) ·
   [Go](practical-guide/languages/go.html) ·
   [other targets](practical-guide/languages/other-targets.html) —
   dependencies, generator options, per-language caveats.
7. [Production concerns](practical-guide/production.html) — size and
   recursion limits, TLS, timeouts, retries and idempotency, error
   taxonomy, observability, upgrades.
8. [Performance engineering](practical-guide/production.html#performance-engineering) —
   what to measure, how to avoid misleading benchmarks.
9. [Diagnostics and troubleshooting](practical-guide/troubleshooting.html) —
   symptom → cause → fix for common failure modes.
10. [Reference appendices](practical-guide/appendices.html) — annotated IDL
    quick reference, compiler flags, glossary, checklists, canonical links.

All examples in this guide use one coherent original domain: a
**Reference Data Query service** (financial instruments, exchange codes,
snapshots). The full IDL and runnable Java/Python/Go implementations live in
the [examples directory](https://github.com/apache/thrift/tree/master/doc/practical-guide/examples)
of the source tree.

## How to read this guide

- **Normative language.** "Must" and "must not" reflect behavior you cannot
  work around (usually the wire format or the compiler). "Should" reflects
  strong recommendation. "May" reflects a valid option among several.
- **Conventions vs rules.** Conventions (naming, which requiredness to
  choose) are marked as conventions; the compiler will usually accept
  either choice.
- **Layer attribution.** Claims state whether behavior belongs to the wire
  format, the compiler, a runtime library, or an application convention.
- Admonition labels used throughout: **Note**, **Caution**, **Security**,
  **Language-specific**, **Version-specific**.
