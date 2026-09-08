# Contribution package — "Apache Thrift: A Practical Guide"

## File manifest (new files)

```
doc/practical-guide/index.md
doc/practical-guide/mental-model.md
doc/practical-guide/first-service.md
doc/practical-guide/idl-design.md
doc/practical-guide/compatibility.md
doc/practical-guide/runtime-selection.md
doc/practical-guide/languages/java.md
doc/practical-guide/languages/python.md
doc/practical-guide/languages/go.md
doc/practical-guide/languages/other-targets.md
doc/practical-guide/production.md
doc/practical-guide/troubleshooting.md
doc/practical-guide/appendices.md
doc/practical-guide/examples/README.md
doc/practical-guide/examples/common.thrift
doc/practical-guide/examples/reference_data.thrift
doc/practical-guide/examples/reference_data_v2.thrift
doc/practical-guide/examples/python/server.py
doc/practical-guide/examples/python/client.py
doc/practical-guide/examples/go/go.mod
doc/practical-guide/examples/go/go.sum
doc/practical-guide/examples/go/server/main.go
doc/practical-guide/examples/go/client/main.go
doc/practical-guide/examples/java/pom.xml
doc/practical-guide/examples/java/src/main/java/com/example/refdata/JavaServer.java
doc/practical-guide/examples/java/src/main/java/com/example/refdata/JavaClient.java
```

Placement note: the site renders source-tree Markdown
(`doc/specs/idl.md` → `/docs/idl`), so `doc/practical-guide/` in the main
repository is the natural home; the docs index gains one link line.
(Apache Thrift has both `doc/` (published) and this proposal's spelling;
final path `doc/` vs `docs/` to be settled with reviewers — all content
is path-agnostic.) Generated code (`gen-*`) is deliberately not
committed; every walkthrough contains its exact generation commands.

## Proposed commit decomposition

One logical change per commit, reviewable independently:

1. `THRIFT-XXXX: add practical guide: core chapters`
   (index, mental-model, first-service, idl-design, compatibility)
2. `THRIFT-XXXX: add practical guide: runtime, languages, production chapters`
   (runtime-selection, languages/*, production, troubleshooting, appendices)
3. `THRIFT-XXXX: add practical guide: example IDL suite and walkthroughs`
   (examples/: IDL files, java/, python/, go/, README)

Squash to a single commit if reviewers prefer (per CONTRIBUTING.md's
one-commit-per-issue norm).

## Proposed PR title

```
THRIFT-XXXX: Add "Apache Thrift: A Practical Guide" (0.24.0) documentation
```

(replace XXXX with the JIRA ticket created for this work; the ticket ID
prefix auto-links the PR from JIRA)

## Proposed PR description

```markdown
THRIFT-XXXX: Add "Apache Thrift: A Practical Guide" (0.24.0) documentation

Client: docs (all languages touched by examples: java, py, go)

### What this adds

A new, version-scoped documentation set under `doc/practical-guide/`,
13 pages + an original, runnable example suite, targeting Apache Thrift
0.24.0. It bridges the gap between the formal IDL/protocol references and
day-to-day engineering: designing durable contracts, schema evolution,
runtime selection (protocols/transports/servers per language), production
concerns (size/recursion limits, TLS, timeouts, retries, observability),
performance methodology, and troubleshooting.

- Pages: index, mental model, first end-to-end service, IDL design,
  compatibility & evolution, runtime choices, language deep-dives
  (Java/Python/Go + summary pages), production, troubleshooting,
  appendices (quick reference, compiler flags, glossary, checklists).
- Examples: a neutral "Reference Data Query" domain (instruments,
  snapshots, exchange directory) with common.thrift, reference_data.thrift,
  an audit-clean v2 evolution, and runnable Java (Maven), Python (pip),
  and Go (modules) server+client walkthroughs.
- Every version-sensitive claim is banner-scoped to 0.24.0 and grounded
  in the v0.24.0 tree; the published language matrix's version lag is
  called out, and per-language caveats (Go union wire behavior, Java
  javax→jakarta, JS i64/BigInt, PHP 8.1 floor, Rust TLS absence,
  Swift's removal) are documented next to the relevant advice.

### Validation performed

- All example IDL compiled with the 0.24.0 compiler for java/py/go/netstd
  /js; `--gen mmd` output verified (incl. documented uuid-rendering quirk).
- `thrift --audit` verified both ways: clean pass on v1→v2; deliberate
  type-change/removal/requiredness breakages each caught with exit 2.
- Walkthroughs executed end-to-end: Java↔Java, Python↔Python, Go↔Go,
  plus cross-language Python↔Java and Go↔Python; expected outputs
  documented in examples/README.md.
- Toolchains recorded (JDK 25/Maven 3.10; CPython 3.14 + thrift 0.24.0
  wheels; Go 1.27 + module v0.24.0).

### Notes for reviewers

- Generated code is intentionally not committed; walkthroughs pin the
  exact generation commands.
- Docs are plain source-tree Markdown (site convention, as with
  doc/specs/*.md); no site build changes required beyond a docs-index
  link if accepted.
- Content is original work based on the official 0.24.0 sources and
  executed validation; it is not derived from any third-party guide.
```

## Reviewer checklist (for the PR)

- [ ] JIRA ticket exists; PR title starts with `THRIFT-NNNN:`
- [ ] `make style` / docs lint (if configured) passes
- [ ] every page carries the 0.24.0 banner and support disclaimer
- [ ] spot-check 3 technical claims against the v0.24.0 tree
- [ ] run at least one walkthrough end-to-end (commands in examples/README.md)
- [ ] confirm no generated code committed; confirm LICENSE/NOTICE need no
      change (no new third-party dependencies introduced)
- [ ] commit message includes the `Client:` line
- [ ] no tool-internal or session URLs anywhere in the commit/PR text
