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

- The `-audit` mode of the C++ compiler. Overridden on 2026-09-21: ported
  as `compiler/go/audit`, with the cases of `test/audit` as its tests and
  a parity test against the C++ compiler.
- Parity of warning text and error text. Accept and reject decisions must
  match; wording may differ.
- `gofmt`-clean output. Overridden on 2026-09-21: the C++ generator's
  output was already `gofmt`-clean for every shipped IDL file (750 of the
  751 files of the corpus; the exception is a doc-comment edge case), so
  the Go generator formats every file it writes with `go/format` and the
  emitters no longer track indentation. The parity test compares against
  the formatted C++ output.
- Generators for any language other than Go. Overridden on 2026-09-14: the
  Java generator was ported next, on the same terms; see section 12.
- Replacing the C++ compiler for the cross-language test suite. Every other
  language keeps using it.

## 2. Decisions the PMC owns

1. **The single-compiler model.** Thrift has one compiler that emits every
   language. THRIFT-4743 removed the out-of-process generator plugin that
   THRIFT-2835 had added, "to simplify the project". A Go-native generator is
   either a per-language tool outside that model or a re-argument of
   THRIFT-4743. This plan assumes the former: a standalone tool that lives in
   the repository, ships with each release tag, and is optional.
2. **Where the AST comes from.** Settled on 2026-09-21: the own front end in
   Go (`compiler/go/idl`, `compiler/go/sema`), with semantic drift addressed
   in [section 6](#6-test-plan). The alternative, an intermediate emitted by
   the C++ compiler's JSON generator, is withdrawn on evidence:
   `t_json_generator::generate_typedef`
   (`compiler/cpp/src/thrift/generate/t_json_generator.cc:453-454`) writes a
   typedef and every field type through `get_true_type()`, so the JSON says
   `Outer.f` has type `Inner` where the IDL wrote `Alias2`, a typedef of a
   typedef of `Inner`. THRIFT-6197 made the Go generator emit
   `type Alias2 = Alias` and `*Alias2` for that field, which needs the chain
   the JSON has already flattened. Any serialised contract would have to be
   designed from scratch, and then it would be the AST that THRIFT-2835's
   `plugin.thrift` described and THRIFT-4743 removed. The contract is the
   in-process `sema` model instead; see [section 13](#13-from-reference-to-replacement).
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
`-version`, `--gen go:opt,opt=value` and `-audit` with its options. The
language after `--gen` is looked up in `compiler/go/generate`, the
registry that is `t_generator_registry.h`: each generator package
registers its name, long name, option table and constructor from
`init()`, the command blank-imports the packages it ships with, and
`-help` lists the generators and their options from the tables. Any other
generator name is the C++ compiler's "Unable to get a generator" error.

Drop-in syntax is a hard requirement, not a convenience. The behavioural
tests in [section 6](#6-test-plan) work by pointing the existing Makefiles'
`THRIFT` variable at the Go binary, which is only possible if every flag they
use is accepted verbatim.

A second, idiomatic flag set exists beside it: `generate`, `audit`,
`check`, `decode`, `languages`, `version` and `help`, selected by the
first argument and parsed by the standard `flag` package, with each
generator's options as `--<language>.<option>` flags built from the
registry. It is not part of parity; [thrift-go-command-line.md](thrift-go-command-line.md)
describes it. Diagnostics from it read `path:line:col: error: ...` and
`path:line: warning: ...`; the C++ form keeps the C++ prefixes.

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
found 159 files at the time of writing.

| Directory | Notes |
|---|---|
| `compiler/go/testdata/accept` | Hand-written edge cases the unit tests found and the shipped corpus does not cover: doc comments that are empty or end in an empty line, container constants named by identifier, exceptions as types, implicit and nonpositive field ids, keywords as field names. |
| `lib/go/test` | The Go-specific cases and their `common/` includes. `IncludesTest`, `DuplicateImportsTest` and `ConstOptionalField` run with `-r` in the Makefile. |
| `test` | The cross-language cases, including `ThriftTest`, `Recursive`, `Include`, `DocTest`, `AnnotationTest` and `DoubleConstantsTest`, plus the per-language subdirectories. |
| `tutorial` | `tutorial.thrift` includes `shared.thrift`; the Makefile runs it with `-r`. |
| `contrib` | Older idioms. |
| `compiler/cpp/tests/cpp` | Small feature probes. |
| `lib/java/src/test/resources` | The Java library's own test IDL, including the definition-order and annotation-metadata cases that `generateTestThrift.gradle` compiles. |

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
The rule reverses for a language when its Go generator becomes the
reference; [section 13](#13-from-reference-to-replacement) describes the
mechanism.

## 9. Risks

- **Semantic drift in the front end.** The dominant risk. Mitigated by the
  front-end parity layer and the reject corpus, both of which use the C++
  compiler as the oracle rather than hand-written expectations.
- **Two front ends to maintain.** Every grammar change needs two
  implementations while both compilers exist. Measured, the cost is small:
  `thrifty.yy` and `thriftl.ll` together have 9 commits since 2022-01-01
  (uuid, the `cpp_type` syntax, `slist`/`senum` removal, a scanner
  end-of-input fix, error-text and MSVC fixes), against 20 commits to
  `t_go_generator.cc` alone in the last 24 months. The emitters are where
  the double maintenance costs, and each emitter stops costing when its
  language flips; the front end costs a port of a grammar change every
  few months until the C++ compiler is retired.
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
3. Resolved: `-audit` is ported and covered by the parity harness.

## 11. Implementation status

What exists on the branch, and how it was verified. Counts are from the
last local run with the oracle built from the same commit.

| Item | State |
|---|---|
| Scanner, parser, AST, `sema` (`compiler/go/idl`, `compiler/go/sema`) | Done. Front-end parity green on 158 of 159 corpus files; the JSON generator cannot render `ConstEdgeCases.thrift` (a container constant named by identifier) and the test skips it, while output parity covers it. `test/BrokenConstants.thrift` is rejected by both. |
| Generator port (`compiler/go/generate/golang`), including the validator generator | Done. Output parity green for all 159 files on all 8 option rows: 1,272 subtests, of which 8 are the both-reject file and 1,264 are byte-identical trees. |
| `thrift-go` command (`compiler/go/cmd/thrift-go`) | Done; accepts the C++ flag syntax. |
| Version string test (`compiler/go/internal/version`) | Done. `build/veralign.sh` bumps the constant. |
| Token positions | Done. `token.Token.Pos` is the 1-based line and byte column where a token starts; `scanner.Error` and `parser.Error` carry the same, for `file:line:col` diagnostics. `Token.Line` stays the end line the C++ messages name. |
| Scanner, parser and `sema` unit tests | Done: token tables for the flex quirks, one AST test per grammar rule with negative cases pinned to line and message, one `sema` test per rule of section 5.3. They found three differences from the C++ compiler, fixed and pinned by the accept corpus: the `byte` warning level and once-per-run behaviour, whitespace-only doc comments counting as a doc, and a trailing empty doc line printing as `//`. A fourth suspect, a container constant named by identifier, turned out to be identical in both compilers (it stays a reference); the test expectation was wrong and the behaviour is pinned. The commit message of the unit-test commit overstates this as four fixes. |
| Accept corpus (`compiler/go/testdata/accept`) | Done, 3 files, walked by the parity tests. |
| Golden manifests (`compiler/go/internal/parity/testdata/golden`) | Done. `TestGoldenGo`, `TestGoldenJava` and `TestGoldenTrees` run on every checkout without the C++ compiler: a digest per output file for every corpus file and option row (8 Go rows, 19 Java rows), plus the full trees of seven small files for Go and three for Java, so a change there reads as a diff. `-update` rewrites them from the current generator after the oracle tests have passed; the Java `@Generated` date is fixed to 2026-01-01 in these tests. About 2.9 MB of text. |
| `decode` subcommand (`compiler/go/decode`) | Done. `thrift-go decode [flags] [file]` prints Thrift-encoded bytes as a tree of ids, types and values without an IDL, for binary, compact and JSON, messages or bare structs, framed or not, with `--json` output; bounded by a `TConfiguration` and the runtime's recursion depth. With `--idl` (and `--type` for a bare struct, `--service` when the IDL has several) the tree is annotated from the `sema` model: field names, IDL type names, enum members, union and exception kinds, binary versus string, `_args`/`_result`/`TApplicationException` for messages; a field the IDL does not declare or whose wire type differs is marked, not rejected. |
| Reject corpus (`compiler/go/testdata/reject`) | Done, 33 files. `TestRejectGo` runs on every checkout; `TestRejectParity` needs the oracle and fails if the C++ compiler accepts a file. |
| Fuzz targets (`FuzzParse`, `FuzzLoad`) | Done. `FuzzParse` covers the parser; `FuzzLoad` (`compiler/go/sema`) runs the whole front end through `Loader.LoadSource`, constant resolution and the validators, where a `fail()` a parse never reaches would hide. Both seed from the corpora and run in seed mode under `go test`; `.github/workflows/fuzz.yml` runs each for 30 minutes nightly and on demand, uploading any crash reproducer. A 20-second run of `FuzzLoad` by hand found nothing. |
| Behavioural run through make | Done locally with a configured tree: `make -C lib/go check`, `make -C test/go check` and `make -C tutorial/go check`, each with `THRIFT=<path to thrift-go>`, all green; the recipe lines show the Go binary being invoked. |
| CI steps in `lib-go` | Added to `.github/workflows/build.yml` as described in section 6.2. Not yet seen running: the branch has no PR. |
| User documentation | A section in `lib/go/README.md`. |
| PMC decision on dev@ | Not started. |
| Tracking master | Each rebase reruns the parity tests with the oracle rebuilt from the same commit and ports whatever the C++ generators gained. The rebase of 2026-09-17 ported six changes: THRIFT-5807 (`IsDefined` and the `Name(%d)` fallback in `String`, with the validator following), THRIFT-5806 (nil guard in `CountSetFields`), `thrift.PreallocSize` in the container readers, THRIFT-6200 (declaring package in the `-remote` stub), THRIFT-5463 (addressed container literals in constants) and `TBaseHelper.preallocSize` in the Java readers. |

Building the oracle locally needs bison 3; the bison 2.3 that ships with
macOS fails on the `--file-prefix-map` flag the CMake build passes. Point
CMake at Homebrew's bison with `-DBISON_EXECUTABLE`. The autotools
configure needs the same bison on `PATH`.

## 12. The Java generator

A second port on the same terms: `compiler/go/generate/java` is
`t_java_generator.cc` function for function, the C++ compiler stays the
reference, and the parity test holds the output byte-identical. The
front end, the corpus and the harness are shared; only the emitter and
its option table are new. `compiler/go/generate/internal/emit` holds the
`t_generator` pieces both emitters need (conditional file write, string
escaping, the `generate_docstring_comment` loop).

Things the port reproduces on purpose, because parity requires them:

- `t_typedef` overrides only `is_typedef`, so `is_binary`, `is_set` and
  `is_list` on a declared type are false through a typedef. The port
  resolves typedefs only where the C++ code calls `get_true_type`.
- A field of enum type always gets a javadoc, even without a doc comment:
  the text is `"\n@see " + class`, and the class prefix is the namespace
  plus a dot even when the namespace is empty.
- Mid-line `indent()` calls in the C++ code (binary setters, field value
  metadata, the `@Generated` date) emit indentation inside a line.
- `generate_standard_writer` uses `get_sorted_members`; everything else
  uses declaration order.
- The `@Generated` annotation carries the local date unless
  `generated_annotations=undated` or `suppress` is given. Both compilers
  read the clock, so a parity run that straddles midnight can fail once.

Option matrix for the parity test, in `internal/parity/java_test.go`: the
seven invocations from `lib/java/gradle/generateTestThrift.gradle`
verbatim, then `none`, `none` with `-r`, and one row per remaining option
(`android`, `private_members`, `sorted_containers`, `java5`,
`generated_annotations=undated`, `generated_annotations=suppress`,
`rethrow_unhandled_exceptions`, `option_type=thrift`, `fullcamel`,
`nocamel`). `lib/java/src/test/resources` joined the corpus, which also
grew the Go and JSON parity runs.

| Item | State |
|---|---|
| Generator port (`compiler/go/generate/java`) | Done. Output parity green for all 159 corpus files on all 19 rows: 3,021 subtests, of which 19 are the both-reject file and 3,002 are byte-identical trees. Green on the first full run; no compiler difference was found. |
| JSON generator (`compiler/go/generate/json`) | Done. The renderer the front-end parity test had used as `internal/jsondump`, with the `merge` option and file output, registered as `--gen json`. Parity rows `none`, `none -r` and `merge` green; `merge` under `-r` is not a row because the C++ generator merges by mutating the shared program, so its output depends on generation order. |
| `thrift-go --gen java` | Done. The command dispatches on the language and accepts several `--gen` arguments. `beans` writes to `gen-javabean` without `-out`. |
| Unit tests | `ParseOptions` errors and the naming helpers (`constant_name`, `as_camel_case`, `make_valid_java_identifier`). |
| Behavioural run | Done locally once with the Gradle version CI pins (8.4): `gradle -p lib/java -Pthrift.compiler=<thrift-go> compileTestJava` ran all eight generate tasks with the Go binary and compiled the result. The unit tests themselves were not run through gradle. Gradle 9 cannot run this build at all (`exec {}` was removed), which is unrelated to the port. |
| CI | No workflow change: the `lib-go` job's `go test ./compiler/go/...` step runs the Java parity test with the same oracle. |

## 13. From reference to replacement

This section is the proposal for the dev@ thread: how the Go compiler
becomes the reference for a language, and under what condition the C++
compiler is retired. It follows a review of the branch on 2026-09-21 and
the measurements below, read from `upstream/master` at `43cd5e041`.

### 13.1 Flip per language, not per compiler

The C++ front end lives as long as any C++ emitter does, so what flips is
"which emitter is the reference for language X". For Go and Java, which
are at byte parity today:

- **Release N.** `thrift-go` ships from the release tag; `lib/go` and
  `lib/java` build with it in CI. The C++ `--gen go` and `--gen java` print
  a one-line deprecation notice. The parity test inverts: it keeps running,
  but a per-ticket allowlist records fixes that landed in the Go generator
  and were not ported back to C++; unlisted divergence in either direction
  stays red. The rule of [section 8](#8-open-bugs-during-the-migration)
  reverses for that language.
- **Release N+2.** `t_go_generator.cc`, `go_validator_generator.cc`,
  `validator_parser.cc`, `t_java_generator.cc` and the allowlist are deleted.
  Thrift ships about twice a year, so the window is about a year.

Every other language: a byte-parity port while its C++ emitter exists,
then the same two-release flip. A language nobody ports and nobody owns
goes through the deprecate-then-remove path the project used for as3,
cocoa, csharp and netcore: a `CHANGES.md` notice and a warning for one
release, removal the next, on a dev@ vote.

The C++ binary is retired when the last tier-2 emitter below has completed
its window. That is a criterion, not a date.

### 13.2 Tiering

Commits to each emitter in the 24 months before 2026-09-21, and its size.
Churn is the evidence for which emitters are maintained.

| Tier | Emitter | C++ lines | Commits / 24 months |
|---|---|---|---|
| Ported | go | 5,065 | 20 |
| Ported | java | 5,908 | 4 |
| Ported | json | 811 | 0 |
| 2, maintained: port in this order | cpp | 5,188 | 12 |
| 2 | js | 3,296 | 12 |
| 2 | rb | 1,469 | 14 |
| 2 | rs | 3,421 | 11 |
| 2 | erl | 1,460 | 11 |
| 2 | delphi | 4,617 | 11 |
| 2 | py | 3,064 | 10 |
| 2 | php | 3,168 | 9 |
| 2 | netstd | 4,279 | 7 |
| 2 | haxe | 3,188 | 7 |
| 2 | c_glib | 4,596 | 6 |
| 3a, documentation and IR emitters: cheap | markdown | 1,269 | 2 |
| 3a | html | 1,088 | 0 |
| 3a | xml | 704 | 0 |
| 3a | xsd | 369 | 1 |
| 3a | gv | 352 | 0 |
| 3a | mmd | 289 | 1 |
| 3b, dormant: deprecate-then-remove vote rather than a port | javame | 3,337 | 3 |
| 3b | dart | 2,584 | 3 |
| 3b | kotlin | 2,040 | 1 |
| 3b | ocaml | 1,795 | 3 |
| 3b | perl | 1,719 | 3 |
| 3b | lua | 1,207 | 2 |
| 3b | st | 1,066 | 1 |
| 3b | d | 782 | 0 |
| 3b | cl | 564 | 1 |

Tier 2 is 37,746 lines. The Go and Java ports came out at about 0.85 Go
lines per C++ line, and the Java port reached parity in under a week once
the harness existed; the estimate for tier 2 plus 3a is four to six
months for one engineer, most of it the compile check per language in
that language's existing CI job. No port starts before its golden
manifest rows exist, so the oracle build does not multiply in CI.

### 13.3 The contract between front end and emitters

`compiler/go/sema` is the contract, in process: its `Program`, `Type` and
the predicates on them reproduce the ~30 getters and 14 predicates the C++
emitters use ([section 3](#3-what-exists-today)). A third-party generator is
a Go package that imports `sema`, registers itself with
`compiler/go/generate` and ships its own `main`, the way generators for
protobuf import `protogen`. No exec, `plugin` or WASM protocol: THRIFT-4743
removed one (2,474 lines across 31 files) for lack of users, and a library gives the same
extensibility with none of the protocol. `sema` is unstable until the
first flip and additive-only within a minor release after it.

### 13.4 What the PMC decides

1. `thrift-go` is the reference for Go and Java from release N, with the
   removal at N+2.
2. The tiering above, a named owner per tier-2 language, and the freeze
   rule for unowned ones.
3. `sema` as public API with the stability statement in 13.3.
4. The binary is `thrift` once the C++ compiler is retired; `thrift-go` is
   its name during the window.
5. The Windows packaging that landed in 0.26 (THRIFT-6310, THRIFT-6311,
   THRIFT-6313, THRIFT-6314, THRIFT-2208) carries the Go binary at the
   flip: a `go build` replaces the MSVC build in those workflows.

### 13.5 What this plan does not propose

- A rewrite of the emitters on a template engine. Measured on the branch,
  about 10 % of the emitter lines are literal text; templates move those
  and lose compile-time checking of the other 90 %, and they break the
  method-for-method mapping to the C++ source that makes a port reviewable.
- A serialised intermediate representation, for the reason in
  [section 2](#2-decisions-the-pmc-owns).
- A parser generator in place of the hand-written scanner and parser: 57
  rules, conflict-free, and the doc-comment lookahead timing that no
  generator reproduces.
- Any third-party module in the root `go.mod`, or a separate module for the
  compiler.

