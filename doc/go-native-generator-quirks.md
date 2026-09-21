# Quirks reproduced for parity, worth fixing after the flip

Companion to [go-native-generator-plan.md](go-native-generator-plan.md).
Byte parity with the C++ compiler is the acceptance criterion of every
port, so each port reproduces what the C++ generator does, including what
it does wrong. This file is the register of those findings: what it is,
where it is, what it costs, and what to do about it once the Go
generator is the reference for that language and the output may change.
Started 2026-09-21 during the tier-3a ports; add to it as ports go on.

Legend for the last column: **now** means a fix in the C++ generator can
land today without breaking parity (the Go port follows at the next
rebase); **after flip** means the fix changes output, so it waits until
the Go generator is the reference; **front end** means it belongs to the
compiler as a whole, not one emitter.

## Crashes and undefined behaviour

| Where | What | Effect | When |
|---|---|---|---|
| `t_gv_generator.cc`, `print_const_value` | Casts the constant's declared type to `t_map*` or `t_list*`/`t_set*` unchecked. A struct literal is a `CV_MAP` value over a `t_struct`, so a struct-valued constant under a container, a struct-typed map key, or a struct field whose value is a container reads a `t_struct` as a `t_map`. | Segfault on 18 of 20 runs of `--gen gv lib/go/test/ConstOptionalField.thrift`; empty type names on the runs that survive. The Go port rejects these inputs; the oracle test skips them (`undefinedInCpp` in `lang_test.go`) and the golden manifest pins the rejection. | now: check `is_struct` and walk the struct's fields |
| `t_xsd_generator.cc`, `generate_service` | Stores each exception's field type in `map<string, t_struct*>` through a C-style cast even when the type is a `t_typedef`. Works only because every reader checks `is_typedef` before touching anything struct-specific. | No visible effect today; the Go port keeps the value as `sema.Type`. | now: store `t_type*` |

## Output the port had to match

| Where | What | Effect | When |
|---|---|---|---|
| `t_go_generator.cc`, `generate_go_docstring_comment` | Writes one `//` per empty doc line, including leading, trailing and doubled ones, which gofmt rewrites. | Generated code failed the gofmt check on such IDL. | now: [apache/thrift#3921](https://github.com/apache/thrift/pull/3921) |
| `t_json_generator.cc`, `merge_includes` | Merges by appending the includes' declarations to the shared `t_program` objects in place. | Under `-r` the output of a program depends on which programs were generated before it; an include reached twice is written twice. Not a parity row. | after flip: merge into a copy, or a set |
| `t_json_generator.cc`, `generate_typedef` and `write_type_spec` | Writes every typedef and field type through `get_true_type()`. | The JSON cannot express a typedef chain, which is why it cannot be the front-end/emitter contract (plan §2). | after flip: emit the declared type and a `true_type` |
| `t_json_generator.cc` | Cannot render a container constant named by an identifier (`compiler/go/testdata/accept/ConstEdgeCases.thrift`): fails with `[FAILURE:generation:...]`. | Both compilers reject; the AST parity test skips the file. | after flip |
| `t_xml_generator.cc`, `write_doc` | The trailing-newline trim erases only when the backward scan breaks on a non-newline character, so a doc that is entirely newlines is written untrimmed. | An empty `<doc>` element with newlines inside. | after flip |
| `t_xsd_generator.cc`, `generate_element` | Checks `is_void` and `is_list` on the declared type, which `t_typedef` does not override, so a typedef of a list is written as a plain typed element rather than expanded. | Arguably right; documented so nobody "fixes" it by accident. | keep |
| `t_markdown_generator.cc` | Keeps its own HTML-entity escape table that overrides `t_generator::get_escaped_string`; escapes struct-field docs regardless of `noescape`; `suffix=` takes the extension without the dot and prepends one. | Inconsistent with `noescape` and with the html generator. | after flip |
| `t_html_generator.cc`, `generate_program_toc_row` | Builds the TOC maps with `std::map::insert`, which keeps the first value on a name collision. | A second declaration with the same name in the TOC is silently dropped. | after flip: warn, or key by qualified name |
| `t_mmd_generator.cc`, `emit_program_types` | Assigns `service_name_` and never reads it. | None. | now: delete |
| `t_java_generator.cc`, `generate_javax_generated_annotation` | Writes the local date into `@Generated(date = ...)` by default. | Generated Java differs from day to day; builds are not reproducible unless `generated_annotations=undated` is given. The Go golden tests pin the clock to 2026-01-01. | after flip: undated by default, or take `SOURCE_DATE_EPOCH` |

## Front end and command line

| Where | What | Effect | When |
|---|---|---|---|
| `main.cc`, `validate_simple_identifier` | The grammar accepts a dotted identifier (`a.B`) in every identifier position, and declarations are rejected afterwards with "Identifier %s can't have a dot." through `yyerror` and `exit(1)`; the comment says it was easier than fixing the grammar. | The error skips `failure()`, so it has no `[FAILURE:` prefix and no cleanup; the Go parser carries the same two-step check for message parity. | front end: two token kinds |
| `main.cc`, `pwarning` and `t_typedef.cc` | Warnings, and the lazily detected "Type ... not defined", go to stdout; failures go to stderr. | Warnings end up in redirected output; the audit's warnings and failures interleave unpredictably in a combined capture. The new `thrift-go` subcommands send every diagnostic to stderr; the legacy form keeps the C++ streams. | after flip |
| `main.cc`, `parse_args` | Every argument but the last is split on spaces with `strtok`; `--x` is rewritten to `-x`; `-o` writes `gen-<lang>/` under the directory while `-out` writes into it. | A directory with a space cannot be an option value; two flags a letter apart with different layouts. Kept verbatim in the legacy form; the subcommand form does not have them. | keep in legacy |
| `t_typedef::get_type()` | Resolves a forward typedef on first use, from the generator, and exits the process from inside a generator when it cannot. | A type error surfaces during generation, after other files may have been written. | front end: resolve in the front end |
| `lib/go/test/Makefile.am`, `test/go/Makefile.am` | Write filtered copies of `test/ThriftTest.thrift` into the source tree as gitignored `ThriftTest.thrift`, and `NamespacedTest.thrift`/`IncludesTest.thrift` include them. | A checkout's IDL corpus depends on whether a build has run; those files had to leave the parity corpus (`buildArtifacts` in `corpus.go`). | now: include `../../../test/ThriftTest.thrift` directly, or generate into the build directory |
| `lib/haxe/codegen/*.ps1`, `lib/netstd/Tests/codegen/*.ps1` | Sweep every `*.thrift` under the repository and expect each to compile. | Any negative-test IDL anywhere in the tree breaks two CI jobs; fixed on the branch with a directory skip list. | now |
| `main.cc`, `-audit` | Failure messages end in `" \n"` plus the trailing newline of `failure()`, so every message is followed by a blank line. | Cosmetic; reproduced. | after flip |
