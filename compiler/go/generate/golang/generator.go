/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements. See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership. The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package golang generates Go code from a resolved Thrift program.
//
// It is a method-for-method port of
// compiler/cpp/src/thrift/generate/t_go_generator.cc and of the validator
// generator it uses. The port writes exactly the bytes the C++ generator
// writes, which is checked against the C++ compiler by the parity tests;
// that is why the emitter builds strings the way an ostream would rather
// than through templates.
package golang

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// DefaultThriftImport is the import path of the runtime library unless
// the thrift_import option overrides it.
const DefaultThriftImport = "github.com/apache/thrift/lib/go/thrift"

// Options are the "--gen go:" generator options.
type Options struct {
	PackagePrefix     string
	ThriftImport      string
	Package           string
	ReadWritePrivate  bool
	IgnoreInitialisms bool
	SkipRemote        bool
	StructKeyEntries  bool
}

// ParseOptions parses the part after "go:" of a --gen argument, with the
// same splitting rules as t_generator::parse_options: options are comma
// separated and a value follows the first "=".
func ParseOptions(spec string) (Options, error) {
	o := Options{ThriftImport: DefaultThriftImport}
	if spec == "" {
		return o, nil
	}
	// The C++ parser stores options in a std::map, so a repeated key keeps
	// the last value and unknown keys are reported in sorted order.
	parsed := map[string]string{}
	for _, option := range strings.Split(spec, ",") {
		if option == "" {
			continue
		}
		key, value := option, ""
		if i := strings.IndexByte(option, '='); i >= 0 {
			key, value = option[:i], option[i+1:]
		}
		parsed[key] = value
	}
	keys := make([]string, 0, len(parsed))
	for k := range parsed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := parsed[key]
		switch key {
		case "package_prefix":
			o.PackagePrefix = value
		case "thrift_import":
			o.ThriftImport = value
		case "package":
			o.Package = value
		case "read_write_private":
			o.ReadWritePrivate = true
		case "ignore_initialisms":
			o.IgnoreInitialisms = true
		case "skip_remote":
			o.SkipRemote = true
		case "struct_key_entries":
			o.StructKeyEntries = true
		default:
			return o, fmt.Errorf("unknown option go:%s", key)
		}
	}
	return o, nil
}

// Error is a generator failure; the C++ generator throws a string.
type Error struct {
	Msg string
}

func (e *Error) Error() string { return e.Msg }

func throw(format string, args ...interface{}) {
	panic(&Error{Msg: fmt.Sprintf(format, args...)})
}

// Generator is t_go_generator for one program.
type Generator struct {
	program     *sema.Program
	opts        Options
	programName string
	serviceName string
	indentLevel int
	tmpCounter  int

	fTypes       strings.Builder
	fTypesName   string
	fConsts      strings.Builder
	fConstsName  string
	fConstValues strings.Builder

	packageName             string
	packageDir              string
	packageIdentifiers      map[string]string
	packageIdentifiersSet   map[string]bool
	lastConstBlock          int
	typesFileHasDeclaration bool
	readMethodName          string
	writeMethodName         string
	equalsMethodName        string

	commonInitialisms map[string]bool
}

// New creates a generator for the program.
func New(program *sema.Program, opts Options) *Generator {
	return &Generator{
		program:               program,
		opts:                  opts,
		programName:           program.Name(),
		packageIdentifiers:    map[string]string{},
		packageIdentifiersSet: map[string]bool{},
	}
}

// Generate writes the Go code for the program. It is
// t_generator::generate_program.
func (g *Generator) Generate() (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*Error); ok {
				err = e
				return
			}
			if e, ok := r.(*sema.Error); ok {
				err = e
				return
			}
			panic(r)
		}
	}()
	g.initGenerator()
	for _, e := range g.program.Enums() {
		g.generateEnum(e)
	}
	for _, t := range g.program.Typedefs() {
		g.generateTypedef(t)
	}
	for _, s := range g.program.Objects() {
		if s.IsXception() {
			g.generateXception(s)
		} else {
			g.generateStruct(s)
		}
	}
	for _, c := range g.program.Consts() {
		g.generateConst(c)
	}
	for _, s := range g.program.Services() {
		g.serviceName = s.Name()
		g.generateService(s)
	}
	g.closeGenerator()
	return nil
}

// ---- t_generator helpers ----

func (g *Generator) indentUp()   { g.indentLevel++ }
func (g *Generator) indentDown() { g.indentLevel-- }

func (g *Generator) indent() string {
	return strings.Repeat("\t", g.indentLevel)
}

// tmp is t_generator::tmp: a name with a running number appended.
func (g *Generator) tmp(name string) string {
	s := name + strconv.Itoa(g.tmpCounter)
	g.tmpCounter++
	return s
}

// escapeString is t_generator::escape_string.
func escapeString(in string) string {
	var sb strings.Builder
	for i := 0; i < len(in); i++ {
		switch c := in[i]; c {
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

func lowercase(in string) string { return strings.ToLower(in) }

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
func isLower(c byte) bool { return c >= 'a' && c <= 'z' }
func toUpper(c byte) byte {
	if isLower(c) {
		return c - 'a' + 'A'
	}
	return c
}
func toLower(c byte) byte {
	if isUpper(c) {
		return c - 'A' + 'a'
	}
	return c
}

// underscore is t_generator::underscore.
func underscore(in string) string {
	b := []byte(in)
	if len(b) > 0 {
		b[0] = toLower(b[0])
	}
	var out []byte
	for i, c := range b {
		if i > 0 && isUpper(c) {
			out = append(out, '_', toLower(c))
		} else {
			out = append(out, c)
		}
	}
	return string(out)
}

// spaces returns n spaces; n below zero yields none.
func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

// formatDouble mimics `ostream << double` with the default precision of
// six significant digits.
func formatDouble(f float64) string {
	return strconv.FormatFloat(f, 'g', 6, 64)
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

// ---- naming ----

// camelcase is t_go_generator::camelcase.
func (g *Generator) camelcase(value string) string {
	b := []byte(value)
	b = g.fixCommonInitialism(b, 0)
	for i := 1; i < len(b)-1; i++ {
		if b[i] == '_' {
			if isLower(b[i+1]) {
				b[i] = toUpper(b[i+1])
				b = append(b[:i+1], b[i+2:]...)
			}
			b = g.fixCommonInitialism(b, i)
		}
	}
	return string(b)
}

// fixCommonInitialism upper-cases the word starting at i when it is a
// known initialism.
func (g *Generator) fixCommonInitialism(value []byte, i int) []byte {
	if g.opts.IgnoreInitialisms {
		return value
	}
	end := len(value)
	if j := strings.IndexByte(string(value[i:]), '_'); j >= 0 {
		end = i + j
	}
	word := strings.ToUpper(string(value[i:end]))
	if g.commonInitialisms[word] {
		copy(value[i:end], word)
	}
	return value
}

// publicizeIn is the three-argument publicize.
func (g *Generator) publicizeIn(value string, isArgsOrResult bool, serviceName string) string {
	if len(value) == 0 {
		return value
	}
	value2, prefix := value, ""
	if dot := strings.LastIndexByte(value, '.'); dot >= 0 {
		prefix = value[:dot+1]
		value2 = value[dot+1:]
	}
	b := []byte(value2)
	if !isUpper(b[0]) {
		b[0] = toUpper(b[0])
	}
	value2 = g.camelcase(string(b))
	lenBefore := len(value2)
	if lenBefore >= 3 && value2[:3] == "New" {
		value2 += "_"
	}
	if lenBefore >= 5 && value2[:5] == "IsSet" {
		value2 += "_"
	}
	if !isArgsOrResult {
		endsWithArgs := lenBefore >= 4 && value2[lenBefore-4:lenBefore] == "Args"
		endsWithRslt := lenBefore >= 6 && value2[lenBefore-6:lenBefore] == "Result"
		if endsWithArgs || endsWithRslt {
			value2 += "_"
		}
	}
	if isArgsOrResult {
		prefix += g.publicize(serviceName)
	}
	return prefix + value2
}

// publicize is the one-argument publicize.
func (g *Generator) publicize(value string) string {
	return g.publicizeIn(value, false, g.serviceName)
}

// publicizeArgs is publicize(value, true).
func (g *Generator) publicizeArgs(value string) string {
	return g.publicizeIn(value, true, g.serviceName)
}

func (g *Generator) newPrefix(value string) string {
	if len(value) == 0 {
		return value
	}
	if dot := strings.LastIndexByte(value, '.'); dot >= 0 {
		return value[:dot+1] + "New" + g.publicize(value[dot+1:])
	}
	return "New" + g.publicize(value)
}

func (g *Generator) privatize(value string) string {
	if len(value) == 0 {
		return value
	}
	b := []byte(value)
	if !isLower(b[0]) {
		b[0] = toLower(b[0])
	}
	return g.camelcase(string(b))
}

var goKeywordsByInitial = map[byte][]string{
	'b': {"break"},
	'c': {"case", "chan", "const", "continue"},
	'd': {"default", "defer"},
	'e': {"else", "error"},
	'f': {"fallthrough", "for", "func"},
	'g': {"go", "goto"},
	'i': {"if", "import", "interface"},
	'm': {"map"},
	'p': {"package"},
	'r': {"range", "return"},
	's': {"select", "struct", "switch"},
	't': {"type"},
	'v': {"var"},
}

// variableNameToGoName appends "_a1" to names that are Go keywords.
func variableNameToGoName(value string) string {
	if len(value) == 0 {
		return value
	}
	value2 := strings.ToLower(value)
	words, ok := goKeywordsByInitial[toLower(value[0])]
	if !ok {
		return value
	}
	for _, w := range words {
		if value2 == w {
			return value2 + "_a1"
		}
	}
	return value
}

func packageNameToGoName(value string) string {
	if value == "error" {
		return value
	}
	for i := 0; i < len(value); i++ {
		if !isLower(value[i]) {
			return value
		}
	}
	return variableNameToGoName(value)
}

// realGoModule is get_real_go_module.
func (g *Generator) realGoModule(program *sema.Program) string {
	if g.opts.Package != "" {
		return g.opts.Package
	}
	if m := program.Namespace("go"); m != "" {
		return m
	}
	return lowercase(program.Name())
}

// ---- files ----

// outDir is t_generator::get_out_dir.
func (g *Generator) outDir() string {
	if g.program.IsOutPathAbsolute() {
		return g.program.OutPath() + "/"
	}
	return g.program.OutPath() + "gen-go" + "/"
}

// writeFile stores the content unless the file already holds it, like
// ofstream_with_content_based_conditional_update.
func writeFile(path, content string) {
	if old, err := os.ReadFile(path); err == nil && string(old) == content {
		return
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		throw("failed to write the output to the file '%s', details: '%s'", path, err.Error())
	}
}

func mkdir(path string) {
	_ = os.MkdirAll(path, 0o755)
}

var commonInitialismList = []string{
	"API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID", "HTML", "HTTP", "HTTPS",
	"ID", "IP", "JSON", "LHS", "QPS", "RAM", "RHS", "RPC", "SLA", "SMTP", "SSH",
	"TCP", "TLS", "TTL", "UDP", "UI", "UID", "UUID", "URI", "URL", "UTF8", "VM",
	"XML", "XSRF", "XSS",
}

func (g *Generator) initGenerator() {
	module := g.realGoModule(g.program)
	g.packageDir = g.outDir()
	g.lastConstBlock = 0
	g.commonInitialisms = map[string]bool{}
	for _, s := range commonInitialismList {
		g.commonInitialisms[s] = true
	}
	if g.opts.ReadWritePrivate {
		g.readMethodName = "read"
		g.writeMethodName = "write"
	} else {
		g.readMethodName = "Read"
		g.writeMethodName = "Write"
	}
	g.equalsMethodName = "Equals"
	for {
		mkdir(g.packageDir)
		if module == "" {
			break
		}
		if pos := strings.IndexByte(module, '.'); pos < 0 {
			g.packageDir += "/" + module
			g.packageName = packageNameToGoName(module)
			module = ""
		} else {
			g.packageDir += "/" + module[:pos]
			module = module[pos+1:]
		}
	}

	g.fTypesName = g.packageDir + "/" + g.programName + ".go"
	g.fConstsName = g.packageDir + "/" + g.programName + "-consts.go"

	g.fTypes.WriteString(g.autogenComment() + g.goPackage() + g.renderIncludes(false))
	g.typesFileHasDeclaration = true
	g.fConsts.WriteString(g.autogenComment() + g.goPackage() + g.renderIncludes(true))
	g.fConsts.WriteString("\n")
	g.fConstValues.WriteString("func init() {\n")

	unusedProtName := g.packageDir + "/" + "GoUnusedProtection__.go"
	writeFile(unusedProtName, g.autogenComment()+g.goPackage()+g.renderImportProtection())
}

func (g *Generator) closeGenerator() {
	g.fConstValues.WriteString("}\n")
	if len(g.program.Consts()) != 0 {
		g.fConsts.WriteString("\n")
	}
	g.fConsts.WriteString(g.fConstValues.String())
	writeFile(g.fConstsName, g.fConsts.String())
	writeFile(g.fTypesName, g.fTypes.String())
}

func (g *Generator) beginTypesDeclaration() {
	if g.typesFileHasDeclaration {
		g.fTypes.WriteString("\n")
	}
	g.typesFileHasDeclaration = true
}

func (g *Generator) autogenComment() string {
	return "// Code generated by Thrift Compiler (" + version.Version + "). DO NOT EDIT.\n\n"
}

func (g *Generator) goPackage() string {
	return "package " + g.packageName + "\n\n"
}

// ---- imports ----

func (g *Generator) renderIncludedPrograms(unusedProt *string) string {
	localNamespace := g.realGoModule(g.program)
	included := map[string]*sema.Program{}
	for _, inc := range g.program.Includes() {
		includeModule := g.realGoModule(inc)
		if localNamespace != "" && localNamespace == includeModule {
			continue
		}
		if _, ok := included[includeModule]; !ok {
			included[includeModule] = inc
		}
	}
	keys := make([]string, 0, len(included))
	for k := range included {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := ""
	for _, k := range keys {
		result += g.renderProgramImport(included[k], unusedProt)
	}
	return result
}

func (g *Generator) renderProgramImport(program *sema.Program, unusedProtection *string) string {
	goModule := g.realGoModule(program)
	goPath := strings.ReplaceAll(goModule, ".", "/")
	found := 0
	if i := strings.LastIndexByte(goModule, '.'); i >= 0 {
		found = i + 1
	}
	lastComponent := goModule[found:]
	packageIdentifier, ok := g.packageIdentifiers[goModule]
	if !ok {
		value := variableNameToGoName(lastComponent)
		if g.packageIdentifiersSet[value] {
			value = g.tmp(value)
		}
		g.packageIdentifiersSet[value] = true
		g.packageIdentifiers[goModule] = value
		packageIdentifier = value
	}
	result := "\t"
	if lastComponent != packageIdentifier {
		result += packageIdentifier + " "
	}
	result += "\"" + g.opts.PackagePrefix + goPath + "\"\n"
	*unusedProtection += "var _ = " + packageIdentifier + ".GoUnusedProtection__\n"
	return result
}

func (g *Generator) renderSystemPackages(systemPackages []string) string {
	result := ""
	for _, pkg := range systemPackages {
		identifier := pkg
		if space := strings.IndexByte(pkg, ' '); space >= 0 {
			result += "\t" + pkg + "\n"
			identifier = pkg[:space]
		} else {
			result += "\t\"" + pkg + "\"\n"
			if slash := strings.LastIndexByte(pkg, '/'); slash >= 0 {
				identifier = pkg[slash+1:]
			}
		}
		g.packageIdentifiersSet[identifier] = true
		if _, ok := g.packageIdentifiers[pkg]; !ok {
			g.packageIdentifiers[pkg] = identifier
		}
	}
	return result
}

func (g *Generator) renderIncludes(consts bool) string {
	var unusedProt string
	result := g.goImportsBegin(consts)
	result += "\n" + g.renderSystemPackages([]string{"thrift \"" + g.opts.ThriftImport + "\""})
	includedPrograms := g.renderIncludedPrograms(&unusedProt)
	if includedPrograms != "" {
		result += "\n" + includedPrograms
	}
	return result + g.goImportsEnd() + unusedProt
}

func (g *Generator) renderImportProtection() string {
	return "var GoUnusedProtection__ int\n"
}

func (g *Generator) goImportsBegin(consts bool) string {
	systemPackages := []string{"bytes", "context"}
	if !consts && len(g.program.Enums()) > 0 {
		systemPackages = append(systemPackages, "database/sql/driver")
	}
	systemPackages = append(systemPackages, "errors", "fmt", "iter", "log/slog", "regexp", "strings", "time")
	return "import (\n" + g.renderSystemPackages(systemPackages)
}

func (g *Generator) goImportsEnd() string {
	return ")\n\n" +
		"// (needed to ensure safety because of naive import list construction.)\n" +
		"var _ = bytes.Equal\n" +
		"var _ = context.Background\n" +
		"var _ = errors.New\n" +
		"var _ = fmt.Printf\n" +
		"var _ = iter.Pull[int]\n" +
		"var _ = slog.Log\n" +
		"var _ = time.Now\n" +
		"var _ = thrift.ZERO\n" +
		"\n" +
		"// (needed by validator.)\n" +
		"var _ = strings.Contains\n" +
		"var _ = regexp.MatchString\n"
}

// ---- docstrings and deprecation ----

func (g *Generator) generateDocstringFor(out *strings.Builder, doc interface {
	HasDoc() bool
	Doc() string
}, members *sema.Struct, subheader string) {
	hasDoc := false
	var ss strings.Builder
	if doc.HasDoc() {
		hasDoc = true
		ss.WriteString(doc.Doc())
	}
	fields := members.Members()
	if len(fields) > 0 {
		if hasDoc {
			ss.WriteString("\n")
		}
		hasDoc = true
		ss.WriteString(subheader + ":\n")
		for _, p := range fields {
			ss.WriteString("  - " + g.publicize(p.Name()))
			if p.HasDoc() {
				ss.WriteString(": " + p.Doc())
			} else {
				ss.WriteString("\n")
			}
		}
	}
	if hasDoc {
		g.generateDocstringComment(out, ss.String())
	}
}

func (g *Generator) generateStructDocstring(out *strings.Builder, s *sema.Struct) {
	g.generateDocstringFor(out, s, s, "Attributes")
}

func (g *Generator) generateFunctionDocstring(out *strings.Builder, f *sema.Function) {
	g.generateDocstringFor(out, f, f.Arglist(), "Parameters")
}

func (g *Generator) generateDocstring(out *strings.Builder, doc interface {
	HasDoc() bool
	Doc() string
}) {
	if doc.HasDoc() {
		g.generateDocstringComment(out, doc.Doc())
	}
}

// generateDocstringComment reproduces the getline loop of the C++ code,
// including its behaviour on a line of 1024 characters or more, where
// the stream fails and the rest of the comment is dropped.
func (g *Generator) generateDocstringComment(out *strings.Builder, contents string) {
	if strings.Trim(contents, " \t\r\n") == "" {
		return
	}
	rest := contents
	for rest != "" {
		var line string
		nl := strings.IndexByte(rest, '\n')
		truncated := false
		switch {
		case nl >= 0 && nl < 1023:
			line, rest = rest[:nl], rest[nl+1:]
		case nl < 0 && len(rest) < 1024:
			line, rest = rest, ""
		default:
			line, rest = rest[:1023], ""
			truncated = true
		}
		// getline sets eofbit only when it runs into the end of the
		// stream, not when it consumes the final newline. A trailing empty
		// line therefore still prints as "//".
		eof := nl < 0
		if len(line) > 0 {
			docLine := line
			if !strings.HasPrefix(docLine, "  -") {
				if i := strings.IndexFunc(docLine, func(r rune) bool { return r != ' ' && r != '\t' }); i >= 0 {
					docLine = docLine[i:]
				} else {
					docLine = ""
				}
			}
			docLine = strings.ReplaceAll(docLine, "''", "\"\"")
			out.WriteString(g.indent() + "// " + docLine + "\n")
		} else if !eof {
			out.WriteString(g.indent() + "//\n")
		}
		if truncated {
			return
		}
	}
}

func (g *Generator) generateDeprecationComment(out *strings.Builder, annotations sema.Annotations) bool {
	values, ok := annotations["deprecated"]
	if !ok {
		return false
	}
	out.WriteString(g.indent() + "// Deprecated: ")
	first := true
	for _, v := range values {
		if v == "1" {
			continue
		}
		if first {
			first = false
		} else {
			out.WriteString("; ")
		}
		out.WriteString(v)
	}
	if first {
		out.WriteString("(no reason given)")
	}
	out.WriteString("\n")
	return true
}

// parseGoTags is parse_go_tags, including its index arithmetic.
func parseGoTags(tags map[string]string, in string) {
	key, value := "", ""
	mode := 0
	for index := 0; index < len(in); index++ {
		if index == 0 && mode == 0 && in[index] == ' ' {
			mode = 2
		}
		if mode == 2 {
			if in[index] == ' ' {
				continue
			}
			mode = 0
		}
		if mode == 0 {
			if in[index] == ':' {
				mode = 1
				index++
				continue
			}
			key += string(in[index])
		} else if mode == 1 {
			if in[index] == '"' {
				tags[key] = value
				key, value = "", ""
				mode = 2
				continue
			}
			value += string(in[index])
		}
	}
}
