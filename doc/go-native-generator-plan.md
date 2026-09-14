# Go-native code generator for Go: design and test plan

**Status: prototype on a branch, nothing has been accepted.** The front end,
the generator port and the parity harness described here exist under
`compiler/go/` on the same branch as this document; see
[section 11](#11-implementation-status) for what is done and what is not.
This document describes how a Go code generator written in Go is built and,
above all, how it is proven equivalent to the C++ generator. It does not
commit the project to shipping it. The decisions in
[section 2](#2-decisions-the-pmc-owns) belong to the PMC and must be settled
on dev@ before anything merges.

Measured sizes in this document were read from the tree at the commit this file
was added in. Effort figures are estimates and are labelled as such.

## 1. Goal and non-goals

### Goal

Ship a tool that Go users can run with `go run` or `go install` from the
`github.com/apache/thrift` module, that reads Thrift IDL and writes the same Go
code as `thrift --gen go`, byte for byte, for every input the C++ compiler
accepts.

Byte parity is the acceptance criterion for every phase. It is stricter than
"compiles and passes the tests", and it is what makes the migration checkable
by a script instead of by judgement.

### Non-goals for the first release

These are deferred, not forgotten. Each one is listed so that nobody
re-litigates it mid-implementation.

- The `-audit` mode of the C++ compiler.
- Parity of warning text and error text. Accept and reject decisions must
  match; wording may differ.
- `gofmt`-clean output. The C++ generator does not produce it today, and
  changing the output breaks parity. Formatting improvements come after the
  C++ generator is retired.
- Generators for any language other than Go.
- Replacing the C++ compiler for the cross-language test suite. Every other
  language keeps using it.

## 2. Decisions the PMC owns

1. **The single-compiler model.** Thrift has one compiler that emits every
   language. THRIFT-4743 removed the out-of-process generator plugin that
   THRIFT-2835 had added, "to simplify the project". A Go-native generator is
   either a per-language tool outside that model or a re-argument of
   THRIFT-4743. This plan assumes the former: a standalone tool that lives in
   the repository, ships with each release tag, and is optional.
2. **Where the AST comes from.** Two options survive analysis:
   - **Own front end in Go.** No runtime dependency on the C++ binary. The
     risk is semantic drift, addressed in [section 6](#6-test-plan).
   - **Intermediate emitted by the C++ compiler.** The existing JSON generator
     (`compiler/cpp/src/thrift/generate/t_json_generator.cc`, 811 lines)
     already emits namespaces, includes, annotations, docs, defaults,
     requiredness, `extends` and typedefs. It is not a versioned interchange
     format and it refers to included types by name, so the Go side still
     needs scope resolution, and users still need the C++ binary installed.

   This plan assumes the own front end. If the PMC prefers the intermediate,
   phase 2 shrinks to a JSON reader plus scope resolution and everything else
   stands.
3. **Deprecation window.** The C++ Go generator stays for at least two minor
   releases after the Go tool reaches parity, and is the reference
   implementation during that window. Bug fixes land in C++ first and cross
   over through the parity test. See [section 8](#8-open-bugs-during-the-migration).

## 3. What exists today

| Component | Measured size | Role |
|---|---|---|
| `compiler/cpp/src/thrift/generate/t_go_generator.cc` | 4,982 lines | The emitter: 101 methods, about 930 stream writes, 7 generator options. |
| `compiler/cpp/src/thrift/generate/t_go_generator.h` | 351 lines | Option parsing, member state. |
| `compiler/cpp/src/thrift/generate/go_validator_generator.cc` | 955 lines | The `Validate()` method every struct gets, driven by `vt.*` annotations. Called from the Go generator, so it is part of the port. |
| `compiler/cpp/src/thrift/generate/validator_parser.cc` | 551 lines | Parses the `vt.*` annotations into rules for the validator generator. |
| `compiler/cpp/src/thrift/thrifty.yy` | 1,341 lines | Bison grammar: 41 tokens, 57 rules. 1,103 of the lines are C++ actions. |
| `compiler/cpp/src/thrift/thriftl.ll` | 347 lines | Flex lexer: 51 rules, no start conditions. |
| `compiler/cpp/src/thrift/parse/*.h` | 2,240 lines | The AST: `t_program`, `t_scope`, `t_struct`, `t_const_value` and friends. |
| `compiler/cpp/src/thrift/main.cc` | 1,334 lines | CLI, include search, two-pass parse driver, `-r`, `-audit`, semantic validation. |

Two observations drive the design:

- **The grammar is small and the actions are large.** Only about a fifth of
  the parser file is grammar shape. The rest builds the AST and validates.
  That work is a rewrite whichever parser technique is chosen, so the choice
  of technique matters less than it looks.
- **The emitter's view of the AST is narrow.** The generator calls about 30
  getters (`get_name`, `get_type`, `get_members`, `get_elem_type`,
  `get_true_type`, `get_key_type`, `get_val_type`, `get_returntype`,
  `get_req`, `get_extends`, `get_arglist`, `get_xceptions`, `get_functions`,
  `get_sorted_members`, `get_doc`, `get_includes`, ...) and 14 predicates
  (`is_map`, `is_set`, `is_list`, `is_enum`, `is_base_type`, `is_struct`,
  `is_binary`, `is_xception`, `is_void`, `is_oneway`, `is_typedef`,
  `is_container`, `is_union`, `is_forward_typedef`). That surface is the
  contract between the front end and the emitter.

### Generator options and their output files

The seven `--gen go:` options are `package_prefix`, `thrift_import`,
`package`, `read_write_private`, `ignore_initialisms`, `skip_remote` and
`struct_key_entries`.

For a program `foo.thrift` the generator writes, under the package directory
derived from `namespace go` or the file name:

- `foo.go` (types and services)
- `foo-consts.go`
- `GoUnusedProtection__.go`
- `<service>-remote/<service>-remote.go` per service, unless `skip_remote`

Files are written through a content-based conditional update, so an unchanged
file keeps its timestamp. The Go tool must do the same, because the Makefiles
depend on it.

Every file starts with a header built from `autogen_summary()` in
`compiler/cpp/src/thrift/generate/t_generator.h`, which embeds
`THRIFT_VERSION`. The Go tool must embed the same string, sourced from the
same place, or parity fails on line one. See [section 5.6](#56-version-string).

## 4. Precedents

Three Go parsers for Thrift IDL exist. None tracks Apache Thrift's grammar
changes, so none is a base for the official tool, but each is evidence of what
works.

| Project | License | Parser | Lexer |
|---|---|---|---|
| thriftrw/thriftrw-go | MIT | goyacc from `thrift.y` | Ragel from `lex.rl` |
| cloudwego/thriftgo | Apache-2.0 | pigeon PEG from `thrift.peg` | scannerless |
| samuel/go-thrift | BSD-3-Clause | pigeon PEG from `grammar.peg` | scannerless |

`goyacc` in golang.org/x/tools is still maintained (last change 2026-05-28)
and would translate `thrifty.yy` rule for rule. This plan chooses a
hand-written recursive-descent parser instead, for the reasons in
[section 5.2](#52-idl-scanner-and-parser). goyacc is the fallback if the PMC
wants a grammar file kept as a fidelity artifact.

## 5. Design

### 5.1 Module and package layout

The tool lives in the root module `github.com/apache/thrift` (`go.mod`,
Go 1.26), not in a nested module. Reasons:

- `go install github.com/apache/thrift/compiler/go/cmd/thrift-go@v0.25.0`
  works with no extra publishing step, because every release tag already
  versions the root module.
- `lib/go/thrift` is in the same module, so the tool and the runtime it
  targets are versioned together.

Constraint: **standard library only**. No third-party dependency enters the
root module for this tool. That keeps the ASF licensing review empty and keeps
`go.sum` unchanged for every existing consumer of `lib/go/thrift`.

```
compiler/go/
  cmd/thrift-go/          CLI, drop-in compatible with the C++ flag syntax
  idl/token/              token kinds and positions
  idl/scanner/            hand-written lexer
  idl/ast/                AST node types
  idl/parser/             recursive-descent parser
  sema/                   include loading, scope, typedef resolution, validation
  generate/golang/        the emitter, ported from t_go_generator.cc
  internal/parity/        parity tests against the C++ compiler (section 6)
  testdata/               positive and negative IDL corpus owned by this tool
```

The `cmd` name `thrift-go` is provisional. See [section 10](#10-open-questions).

### 5.2 IDL scanner and parser

**Scanner.** Hand-written, one `Next()` method returning a token with a
position. The token set is the 41 tokens of `thrifty.yy`, including the
legacy ones that Go ignores (`xsd_all`, `xsd_optional`, `xsd_nillable`,
`xsd_attrs`, `cpp_include`, `cpp_type`, `async`, `reference`). They must lex
and parse so that the tool accepts exactly what the C++ compiler accepts. The
lexical rules to reproduce, from `thriftl.ll`:

- Identifiers may contain dots: `[a-zA-Z_](\.[a-zA-Z_0-9]|[a-zA-Z_0-9])*`.
  Qualified names such as `shared.SharedStruct` arrive as one token.
- Integers are decimal or `0x` hexadecimal with an optional sign. `true` and
  `false` are integer constants 1 and 0.
- Doubles: `[+-]?[0-9]*(\.[0-9]+)?([eE][+-]?[0-9]+)?`.
- String literals in single or double quotes.
- Three comment forms are skipped: `//`, `#`, and `/* */`. A `/** */` block
  is a doc comment and attaches to the next declaration. `/***/` is a plain
  comment.
- A UTF-8 byte-order mark at the start of a file is skipped.

**Parser.** Hand-written recursive descent, one method per bison rule, 57
rules. The grammar has no `%expect` declaration, so it is conflict-free
LALR(1); every construct is decidable with one token of lookahead plus the
dotted-identifier token above. The parser produces an AST only. It does not
resolve names, assign enum values or validate constants. Those are `sema`'s
job, which keeps the parser testable on snippets.

The parser must reproduce two structural behaviours of the C++ driver:

- **Two passes per file.** The C++ compiler parses each file once in
  `INCLUDES` mode, then parses every included file recursively, then parses
  the file again in `PROGRAM` mode. The Go parser can do a single pass, but
  the effect must be identical: included programs are fully loaded before the
  including program's declarations are resolved.
- **Doc comment reset.** Doc-comment state is reset before each file's
  program pass. A dangling doc comment at the end of an included file must
  not attach to the first declaration of the includer.

### 5.3 Semantic analysis (`sema`)

This package reproduces the behaviour of `main.cc` and `t_scope.h`. The rules
below were read from the code and are the parity checklist. Each item gets a
unit test with a positive and a negative input.

**Include resolution** (`include_file` in `main.cc`):

1. An absolute path is used as-is, after `realpath`.
2. A relative path is searched first in the directory of the including file,
   then in each `-I` directory in command-line order. The first hit wins.
3. Paths are canonicalised with `realpath`, so the same file reached through
   two include paths is one program.
4. A missing include is a warning unless `-strict` is 192 or higher, in which
   case it is a failure.
5. Recursion is detected with a set of files currently being parsed; a file
   that includes itself transitively fails with "Recursion detected".
6. With `-r`, the output path of the parent program is propagated to every
   included program and each is generated in turn, children first.

**Scope** (`t_scope.h`): three flat maps, types, constants and services, keyed
by name. Symbols from an included program are registered in the including
program's scope under `<included program name>.<symbol>`. Adding a constant
that already exists is an error. Lookup failures return a null result rather
than an error; the callers decide.

**Constant resolution** (`resolve_all_consts` and `resolve_const_value` in
`t_scope.h`), run once per program before generation:

- Typedefs are followed to the underlying type.
- Map, list, set and struct constants are resolved element-wise. A struct
  constant naming a field the struct does not have is an error.
- An identifier constant whose target type is an enum is bound to that enum.
- An identifier constant of any other type is looked up as a named constant.
  Its value is copied by kind: integer kinds, string, uuid, double, map or
  list. A missing name is an error. A `void` target is an error.
- An integer constant whose target type is an enum is mapped back to the
  enum member with that value. A value with no member is an error.

**Validation** (`main.cc`), each of which must reject the same inputs the C++
compiler rejects:

- `validate_simple_identifier`: no dots in a declared name.
- `validate_const_type` and `validate_const_rec`: the constant's value tree
  matches its declared type, recursively.
- `validate_field_value`: a field default matches the field type.
- `validate_throws`: every `throws` entry is an exception type.
- `validate_input` on the generator: runs the `validate()` method of every
  element before writing anything. The checks in `t_struct`, `t_function`,
  `t_map`, `t_list` and `t_set` that forbid an exception type outside a
  `throws` clause are guarded by `#ifndef ALLOW_EXCEPTIONS_AS_TYPE`, and
  `t_type.h` defines that macro unconditionally (THRIFT-5835), so the C++
  compiler never runs them and the Go front end must not either. Reading
  the headers with preprocessor lines filtered out hid this; the parity
  test caught it.

**Field and enum numbering.** Fields without an explicit id receive negative
ids counting down from -1, with a warning. An explicit non-positive id is
accepted with a warning and resets the counter to one below it. Enum members
without a value get the previous value plus one, starting at 0; an explicit
value resets the counter, and overflow past `INT32_MAX` is an error. All of
this is done in the bison actions today and moves to `sema`.

**Strict mode.** `-strict` sets the level to 255. The level is read in
exactly two places, both against a threshold of 192: a missing include
becomes an error instead of a warning, and an implicit field id becomes an
error instead of a warning. The Go tool copies both and nothing else.

### 5.4 The emitter (`generate/golang`)

A 1:1 port of `t_go_generator.cc`, method for method, writing to a
`strings.Builder` per output file. Templates are deliberately not used in the
first version. Parity is far easier to reach and to debug when the Go method
and the C++ method have the same shape, and a template rewrite is a separate,
later change that the parity test then protects.

Rules the port must follow:

- **Deterministic output.** No iteration over a Go map without sorting the
  keys first. Where the C++ code iterates a `std::map`, the Go code sorts by
  the same key. Where it iterates a `std::vector`, order is declaration order.
- **Same annotation handling.** The generator reads three annotations:
  `cpp.ref` on fields, `go.tag` on fields, and `deprecated` on typedefs,
  enums, enum members, structs, fields and functions. `deprecated` is read
  from the annotation map directly; a bare annotation carries the value `1`,
  which is skipped, and every other value becomes a `// Deprecated:` line.
- **Same number and string formatting.** C++ stream formatting of doubles
  and Go `strconv` formatting differ by default. The port must pick the
  format that reproduces the C++ output, verified by
  `test/DoubleConstantsTest.thrift` and `test/ConstantsDemo.thrift` in the
  parity run.
- **Same file set and same conditional write** as listed in
  [section 3](#generator-options-and-their-output-files).

### 5.5 CLI

`cmd/thrift-go` accepts the C++ syntax unchanged: `-I dir`, `-o dir`,
`-out dir`, `-r` and `-recurse`, `-strict`, `-nowarn`, `-v` and `-verbose`,
`-version`, and `--gen go:opt,opt=value`. Any other generator name after
`--gen` is an error naming the C++ compiler.

Drop-in syntax is a hard requirement, not a convenience. The behavioural
tests in [section 6](#6-test-plan) work by pointing the existing Makefiles'
`THRIFT` variable at the Go binary, which is only possible if every flag they
use is accepted verbatim.

A second, idiomatic flag set can be added later. It is not part of parity.

### 5.6 Version string

The C++ header is `Autogenerated by Thrift Compiler (<THRIFT_VERSION>)`. The
Go tool needs the same string without a build step that reads
`configure.ac`. Proposed mechanism: a generated Go file
`compiler/go/internal/version/version.go` holding the version constant,
updated by the same release procedure that bumps `configure.ac` and the
other per-language version files, and a test that reads `configure.ac` and
fails if they differ. This adds one file to the release checklist in
`doc/ReleaseManagement.md`.

## 6. Test plan

The oracle for every layer is the C++ compiler at the same commit. No
generated code is committed as golden files. Parity tests locate the C++
binary through the `THRIFT_COMPILER` environment variable, and skip with an
explicit message when it is unset, so `go test ./...` stays green on a
checkout without a built compiler while CI always runs the comparison.

| Layer | Command | Oracle | Pass condition | Runs where |
|---|---|---|---|---|
| Scanner | `go test ./compiler/go/idl/scanner` | Table of input to expected token stream, written from `thriftl.ll` | Exact token kinds, values, positions | Every `go test` |
| Parser | `go test ./compiler/go/idl/parser` | Positive and negative snippets per grammar rule | Positive parses to the expected AST; negative fails at the expected token | Every `go test` |
| Sema | `go test ./compiler/go/sema` | One test per rule in [section 5.3](#53-semantic-analysis-sema) | Same accept or reject decision as the C++ compiler for that rule | Every `go test` |
| Front-end parity | `go test ./compiler/go/internal/parity -run AST` | `thrift --gen json` over every corpus file | The Go AST, serialised in the same JSON shape, is byte-identical | CI, and locally with `THRIFT_COMPILER` set |
| Output parity | `go test ./compiler/go/internal/parity -run Generate` | `thrift --gen go:<opts>` over every corpus file, once per option set | Output directory trees are byte-identical, including the file set | CI, and locally with `THRIFT_COMPILER` set |
| Negative corpus | `go test ./compiler/go/internal/parity -run Reject` | The C++ compiler must fail on each file in `compiler/go/testdata/reject` | Both tools reject; the test fails if the C++ compiler accepts a file, so the corpus cannot drift into asserting the wrong thing | CI |
| Behavioural | `make -C lib/go check`, `make -C test/go check`, `make -C tutorial/go` with `THRIFT=<go binary>` | The existing Go test suites and the cross-language test | Green | CI, a second run of the `lib-go` job |
| Fuzz | `go test -fuzz=FuzzParse ./compiler/go/idl/parser` | Seed corpus from the positive and negative corpora | No panic, no hang; `Parse` returns a program or an `*Error` with a line | Seed mode on every `go test`, like `lib/go/test/fuzz`; no workflow in this repository runs on a schedule, so longer runs are by hand with `-fuzztime` |

The front-end parity layer is the important one. It proves the parser and
`sema` independently of the emitter, using a generator that already exists
and that nobody needs to write. When output parity fails, this layer says
whether the front end or the emitter is at fault.

### 6.1 Corpus

Positive corpus: every `.thrift` file under these directories, walked
recursively, skipping `gen-*` and `gopath` output directories. The harness
found 147 files at the time of writing.

| Directory | Notes |
|---|---|
| `compiler/go/testdata/accept` | Hand-written edge cases the unit tests found and the shipped corpus does not cover: doc comments that are empty or end in an empty line, container constants named by identifier, exceptions as types, implicit and nonpositive field ids, keywords as field names. |
| `lib/go/test` | The Go-specific cases and their `common/` includes. `IncludesTest`, `DuplicateImportsTest` and `ConstOptionalField` run with `-r` in the Makefile. |
| `test` | The cross-language cases, including `ThriftTest`, `Recursive`, `Include`, `DocTest`, `AnnotationTest` and `DoubleConstantsTest`, plus the per-language subdirectories. |
| `tutorial` | `tutorial.thrift` includes `shared.thrift`; the Makefile runs it with `-r`. |
| `contrib` | Older idioms. |
| `compiler/cpp/tests/cpp` | Small feature probes. |

The include-heavy files, the `-r` runs and the `ConflictNamespaceTest*`
group in `lib/go/test` are the include-resolution and scope tests. They are
the ones most likely to expose drift in [section 5.3](#53-semantic-analysis-sema).

Option matrix for the output parity layer. Every corpus file runs with every
row, except where the C++ compiler itself rejects the combination.

| Row | Options | Exercised today by |
|---|---|---|
| base | `thrift_import=...,package_prefix=...` | `lib/go/test/Makefile.am`, `test/go/Makefile.am`, `tutorial/go/Makefile.am` |
| base-r | base, with `-r` | The three `-r` files in `lib/go/test` and the tutorial |
| none | no options at all | Nothing in the Makefiles. Covers the default `thrift_import`. |
| skip_remote | base + `skip_remote` | `ProcessorMiddlewareTest.thrift` |
| struct_key_entries | base + `struct_key_entries` | `StructKeyTest.thrift` |
| read_write_private | base + `read_write_private` | `DontExportRWTest.thrift` |
| ignore_initialisms | base + `ignore_initialisms` | `IgnoreInitialismsTest.thrift` |
| package | base + `package=parity` | Nothing in the Makefiles; the harness row is the only coverage. |

Negative corpus: one file per rule in [section 5.3](#53-semantic-analysis-sema),
under `compiler/go/testdata/reject/`, each with a comment naming the rule it
violates. The `Reject` test runs the C++ compiler on each file first and fails
if the C++ compiler accepts it.

### 6.2 CI

The `lib-go` job in `.github/workflows/build.yml` already downloads the
`thrift-compiler` artifact from the `compiler` job, runs `make -C lib/go
check`, `make -C test/go check` and `make -C test/go precross` on a Go
version matrix. Two additions, both in place:

1. Before the existing checks: `go vet ./compiler/go/...`, `go test
   ./compiler/go/...` and `go build -o compiler/go/thrift-go`. The parity
   tests find the oracle at `compiler/cpp/thrift`, where the artifact
   lands, so no environment variable is needed. This runs the unit,
   parity, reject and fuzz-seed layers against the same compiler binary
   the job already has.
2. After the existing checks, on one matrix entry: `make clean` in
   `lib/go/test`, `lib/go/test/fuzz` and `test/go`, then `make -C lib/go
   check` and `make -C test/go check` with `THRIFT=$PWD/compiler/go/thrift-go`.
   The `gopath` stamps from the first pass have to go, or make would not
   regenerate. `THRIFT` is a make prerequisite, so it must be an absolute
   path to an existing file.

Nothing else in the workflow changes. No new job, no new runner, no new
dependency. `tutorial/go` is not built by this job today and stays out.

## 7. Phases and exit criteria

Each exit criterion is a runnable check. Effort figures assume one engineer
who knows the grammar and the Go runtime, and exclude review latency.

| Phase | Work | Exit criterion | Estimate |
|---|---|---|---|
| 0 | dev@ thread on the decisions in [section 2](#2-decisions-the-pmc-owns) | PMC answer recorded in a JIRA ticket | Not engineering time |
| 1 | Parity harness: corpus enumeration, option matrix, `THRIFT_COMPILER` discovery, JSON-shape AST serialiser, directory diff. No parser yet. | `go test ./compiler/go/internal/parity` runs, finds the C++ compiler in CI, and reports "Go tool not built" cleanly. | 1 to 2 weeks |
| 2 | Scanner, parser, AST, `sema`. | Front-end parity green on all corpus files. Reject corpus green. Fuzz target runs for an hour without a crash. | 3 to 5 weeks |
| 3 | Emitter port and CLI. | Output parity green for every corpus file and every option row. | 3 to 4 weeks |
| 4 | Makefile `THRIFT` override, CI second run, version-string test, release checklist entry, user documentation in `lib/go/README.md`. | `lib-go` job green with both compilers. | 1 to 2 weeks |
| 5 | Deprecation window. C++ generator remains the reference; every fix lands there first. | Two minor releases with parity green on every push. | Calendar time |

Phase 1 comes first on purpose. A harness with nothing to compare is cheap,
and it means every later commit is measured from its first day.

## 8. Open bugs during the migration

At the time of writing, four Go generator bugs are open: THRIFT-6200,
THRIFT-6204, THRIFT-6239 and THRIFT-6240. The rule for these and for any bug
filed during phases 2 to 5:

1. The fix lands in `t_go_generator.cc` first, with its test in
   `lib/go/test`, exactly as today.
2. The parity test then fails on the Go side, which is the signal to port the
   fix. The port is a separate commit that references the same ticket.

No fix lands in the Go tool alone while the C++ generator is the reference.

## 9. Risks

- **Semantic drift in the front end.** The dominant risk. Mitigated by the
  front-end parity layer and the reject corpus, both of which use the C++
  compiler as the oracle rather than hand-written expectations.
- **Two front ends to maintain.** Every grammar change now needs two
  implementations until the C++ generator is retired, and the C++ front end
  outlives the C++ Go generator because every other language uses it. This
  is a permanent cost, not a migration cost, and it is the strongest argument
  for the intermediate option in [section 2](#2-decisions-the-pmc-owns).
- **Number formatting.** C++ stream and Go `strconv` disagree on doubles by
  default. Caught by the parity corpus, but it can cost days to match
  precisely.
- **Windows paths.** `include_file` uses `realpath` and `/` joins. The Go
  tool must produce the same package directories on Windows, where the C++
  compiler has its own path code under `compiler/cpp/src/thrift/windows`.
- **Version string skew.** A release that bumps `configure.ac` without the
  Go version file breaks parity on every file. Mitigated by the test in
  [section 5.6](#56-version-string).
- **Reviewer bandwidth.** A 1:1 port of a 5,000-line emitter is a large diff
  for a project with few active Go reviewers. Phase 3 should land as a
  sequence of commits by emitter area (types, constants, structs, services,
  remote) with parity narrowing on each.

## 10. Open questions

1. Name of the binary: `thrift-go`, `thriftgo` (taken by CloudWeGo), or
   `thrift-gen-go` (suggests a protoc-style plugin, which it is not). The
   prototype uses `thrift-go`.
2. Whether the release procedure in `doc/ReleaseManagement.md` gains a step
   or the version file is generated by `bootstrap.sh`.
3. Whether the PMC wants the parity harness to also cover `-audit`, which
   would pull that mode back into scope.

## 11. Implementation status

What exists on the branch, and how it was verified. Counts are from the
last local run with the oracle built from the same commit.

| Item | State |
|---|---|
| Scanner, parser, AST, `sema` (`compiler/go/idl`, `compiler/go/sema`) | Done. Front-end parity green on 149 of 150 corpus files; the JSON generator cannot render `ConstEdgeCases.thrift` (a container constant named by identifier) and the test skips it, while output parity covers it. `test/BrokenConstants.thrift` is rejected by both. |
| Generator port (`compiler/go/generate/golang`), including the validator generator | Done. Output parity green for all 150 files on all 8 option rows: 1,200 subtests, of which 8 are the both-reject file and 1,192 are byte-identical trees. |
| `thrift-go` command (`compiler/go/cmd/thrift-go`) | Done; accepts the C++ flag syntax. |
| Version string test (`compiler/go/internal/version`) | Done. `build/veralign.sh` bumps the constant. |
| Scanner, parser and `sema` unit tests | Done: token tables for the flex quirks, one AST test per grammar rule with negative cases pinned to line and message, one `sema` test per rule of section 5.3. They found three differences from the C++ compiler, fixed and pinned by the accept corpus: the `byte` warning level and once-per-run behaviour, whitespace-only doc comments counting as a doc, and a trailing empty doc line printing as `//`. A fourth suspect, a container constant named by identifier, turned out to be identical in both compilers (it stays a reference); the test expectation was wrong and the behaviour is pinned. The commit message of the unit-test commit overstates this as four fixes. |
| Accept corpus (`compiler/go/testdata/accept`) | Done, 3 files, walked by the parity tests. |
| Reject corpus (`compiler/go/testdata/reject`) | Done, 33 files. `TestRejectGo` runs on every checkout; `TestRejectParity` needs the oracle and fails if the C++ compiler accepts a file. |
| Fuzz target (`FuzzParse`) | Done. Seeds from the corpora; runs in seed mode under `go test`. A 45-second run by hand found nothing. No scheduled workflow exists in this repository, so there is no nightly run. |
| Behavioural run through make | Done locally with a configured tree: `make -C lib/go check`, `make -C test/go check` and `make -C tutorial/go check`, each with `THRIFT=<path to thrift-go>`, all green; the recipe lines show the Go binary being invoked. |
| CI steps in `lib-go` | Added to `.github/workflows/build.yml` as described in section 6.2. Not yet seen running: the branch has no PR. |
| User documentation | A section in `lib/go/README.md`. |
| PMC decision on dev@ | Not started. |

Building the oracle locally needs bison 3; the bison 2.3 that ships with
macOS fails on the `--file-prefix-map` flag the CMake build passes. Point
CMake at Homebrew's bison with `-DBISON_EXECUTABLE`. The autotools
configure needs the same bison on `PATH`.
