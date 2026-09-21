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

// Package js is a port of t_js_generator.cc. Every function keeps the name
// and the emission order of its C++ original, so that the output is
// byte-identical; the parity test in internal/parity holds it to that.
//
// The generator writes a program's "_types.js" (plus "_types.d.ts" with
// ts), one ".js" file per service (plus one ".d.ts" with ts), and,
// with thrift_package_output_directory, a "thrift.js.episode" file. The
// node, jquery, es6 and with_ns options change the module shape of the
// output; esm and bigint are accepted (the real C++ constructor accepts
// them) but are not documented in THRIFT_REGISTER_GENERATOR's option
// table, so they are left out of the --help text below, matching the
// C++ source.
package js

import (
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// episodeFileName is episode_file_name.
const episodeFileName = "thrift.js.episode"

// maxSafeInteger/minSafeInteger are max_safe_integer/min_safe_integer: the
// largest and smallest integers a double can represent consecutively.
const (
	maxSafeInteger int64 = 0x1fffffffffffff
	minSafeInteger int64 = -maxSafeInteger
)

// Options are the js:... generator options.
type Options struct {
	Node          bool
	Jquery        bool
	TS            bool
	WithNS        bool
	ES6           bool
	ESM           bool
	NativePromise bool
	Bigint        bool

	Imports    string
	HasImports bool

	ThriftPackageOutputDirectory string
	GenEpisodeFile               bool
}

// ParseOptions parses the part after "js:" of a --gen argument, with the
// same splitting rules as t_generator::parse_options: options are comma
// separated, a value follows the first "=", and the C++ compiler iterates
// them in the sorted order of a std::map.
func ParseOptions(spec string) (Options, error) {
	o := Options{NativePromise: true}
	parsed := map[string]string{}
	if spec != "" {
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
	}
	keys := make([]string, 0, len(parsed))
	for k := range parsed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := parsed[key]
		switch key {
		case "node":
			o.Node = true
		case "jquery":
			o.Jquery = true
		case "ts":
			o.TS = true
		case "with_ns":
			o.WithNS = true
		case "es6":
			o.ES6 = true
		case "esm":
			o.ESM = true
		case "imports":
			o.Imports, o.HasImports = value, true
		case "thrift_package_output_directory":
			dir := value
			if strings.HasSuffix(dir, "/") {
				dir = dir[:len(dir)-1]
			}
			if dir == "" {
				return o, &emit.Error{Msg: "the thrift_package_output_directory argument must not be empty"}
			}
			o.ThriftPackageOutputDirectory = dir
			o.GenEpisodeFile = true
		case "native_promise":
			o.NativePromise = value != "false" && value != "0" && value != "no"
		case "bigint":
			o.Bigint = true
		default:
			return o, &emit.Error{Msg: "unknown option js:" + key}
		}
	}
	// BigInt-mode code generation is only meaningful for the node
	// generator. Force it off for plain browser-JS so passing `js:bigint`
	// alongside (or via shared option strings) doesn't affect plain
	// `--gen js`.
	if !o.Node {
		o.Bigint = false
	}
	if o.ES6 && o.Jquery {
		return o, &emit.Error{Msg: "invalid switch: [-gen js:es6,jquery] options not compatible"}
	}
	if o.Node && o.Jquery {
		return o, &emit.Error{Msg: "invalid switch: [-gen js:node,jquery] options not compatible, try: [-gen js:node -gen js:jquery]"}
	}
	if !o.Node && o.WithNS {
		return o, &emit.Error{Msg: "invalid switch: [-gen js:with_ns] is only valid when using node.js"}
	}
	if !o.Node && o.ESM {
		return o, &emit.Error{Msg: "invalid switch: [-gen js:esm] is only valid when using node.js"}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "js",
		LongName: "Javascript",
		Options: []generate.Option{
			{Name: "jquery", Help: "Generate jQuery compatible code."},
			{Name: "node", Help: "Generate node.js compatible code."},
			{Name: "ts", Help: "Generate TypeScript definition files."},
			{Name: "with_ns", Help: "Create global namespace objects when using node.js"},
			{Name: "es6", Help: "Create ES6 code with Promises"},
			{Name: "native_promise", Value: "[true|false]", Help: "Use native Promise (default true). Set to false to\nemit legacy Q-based output (requires the 'q' package)."},
			{Name: "thrift_package_output_directory", Value: "<path>", Help: "Generate episode file and use the <path> as prefix"},
			{Name: "imports", Value: "<paths_to_modules>", Help: "':' separated list of paths of modules that has episode files in their root"},
		},
		Parse: func(spec string) (generate.Runner, error) {
			opts, err := ParseOptions(spec)
			if err != nil {
				return nil, err
			}
			return runner{opts}, nil
		},
	})
}

type runner struct{ opts Options }

func (r runner) Run(program *sema.Program, recurse bool) error {
	return Run(program, r.opts, recurse)
}

// Run generates the program and, when recurse is set, every program it
// includes first, each inheriting the output path.
func Run(program *sema.Program, opts Options, recurse bool) error {
	if recurse {
		program.SetRecursive(true)
		for _, inc := range program.Includes() {
			inc.SetOutPath(program.OutPath(), program.IsOutPathAbsolute())
			if err := Run(inc, opts, recurse); err != nil {
				return err
			}
		}
	}
	return generateOne(program, opts)
}

func generateOne(program *sema.Program, opts Options) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*sema.Error); ok {
				err = e
				return
			}
			if e, ok := r.(*emit.Error); ok {
				err = e
				return
			}
			panic(r)
		}
	}()
	program.Scope.ResolveAllConsts()
	sema.ValidateInput(program)
	g := newGenerator(program, opts)
	g.generate()
	return nil
}

// Generator is t_js_generator.
type Generator struct {
	program *sema.Program
	opts    Options

	// serviceName is service_name_, set by t_oop_generator::generate_service
	// before generate_service is called.
	serviceName string

	tsModule   string
	noNS       bool
	outDirBase string

	jsConstType string
	jsLetType   string
	jsVarType   string

	// moduleName2ImportPath is module_name_2_import_path, filled by
	// parseImports from the episode files named by the imports option.
	moduleName2ImportPath map[string]string

	// include2ImportName is include_2_import_name, filled while rendering
	// the TypeScript include statements of the current file.
	include2ImportName map[*sema.Program]string

	indentLevel int
	tmpCounter  int

	fTypes     strings.Builder
	fTypesTS   strings.Builder
	fService   strings.Builder
	fServiceTS strings.Builder
	fEpisode   strings.Builder
}

// newGenerator is the t_js_generator constructor, minus the option
// validation ParseOptions already performed (it needs no program). It
// still performs parse_imports, which does: it can only run once a
// program (and its -r flag) is known.
func newGenerator(program *sema.Program, opts Options) *Generator {
	g := &Generator{
		program:               program,
		opts:                  opts,
		moduleName2ImportPath: map[string]string{},
		include2ImportName:    map[*sema.Program]string{},
	}
	g.jsConstType, g.jsLetType, g.jsVarType = "var ", "var ", "var "
	if opts.ES6 {
		g.jsConstType, g.jsLetType = "const ", "let "
	}
	if opts.Node {
		g.outDirBase = "gen-nodejs"
		g.noNS = !opts.WithNS
	} else {
		g.outDirBase = "gen-js"
		g.noNS = false
	}
	if opts.HasImports {
		g.parseImports(opts.Imports)
	}
	return g
}

func (g *Generator) generate() {
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
}

// ---- t_generator / t_oop_generator helpers ----

func (g *Generator) indentUp()   { g.indentLevel++ }
func (g *Generator) indentDown() { g.indentLevel-- }

func (g *Generator) indent() string {
	return strings.Repeat("  ", g.indentLevel)
}

func (g *Generator) scopeDown(out *strings.Builder) {
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) tmp(name string) string {
	s := name + strconv.Itoa(g.tmpCounter)
	g.tmpCounter++
	return s
}

func (g *Generator) outDir() string {
	if g.program.IsOutPathAbsolute() {
		return g.program.OutPath() + "/"
	}
	return g.program.OutPath() + g.outDirBase + "/"
}

func (g *Generator) autogenComment() string {
	return "//\n" +
		"// Autogenerated by Thrift Compiler (" + version.Version + ")\n" +
		"//\n" +
		"// DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n" +
		"//\n"
}

// getlineSplit repeats what a loop of std::getline(stream, s, delim) does:
// it never emits a trailing empty piece produced only by consuming a
// final delimiter, and an empty input yields no pieces at all.
func getlineSplit(s string, delim byte) []string {
	var out []string
	for len(s) > 0 {
		if i := strings.IndexByte(s, delim); i >= 0 {
			out = append(out, s[:i])
			s = s[i+1:]
		} else {
			out = append(out, s)
			s = ""
		}
	}
	return out
}

// ---- namespaces ----

// jsNamespacePieces is js_namespace_pieces.
func (g *Generator) jsNamespacePieces(p *sema.Program) []string {
	if g.noNS {
		return nil
	}
	ns := p.Namespace("js")
	var pieces []string
	for ns != "" {
		if loc := strings.IndexByte(ns, '.'); loc >= 0 {
			pieces = append(pieces, ns[:loc])
			ns = ns[loc+1:]
		} else {
			break
		}
	}
	if ns != "" {
		pieces = append(pieces, ns)
	}
	return pieces
}

// jsTypeNamespace is js_type_namespace.
func (g *Generator) jsTypeNamespace(p *sema.Program) string {
	if g.opts.Node {
		if p != nil && p != g.program {
			return makeValidNodeJsIdentifier(p.Name()) + "_ttypes."
		}
		return "ttypes."
	}
	return g.jsNamespace(p)
}

// hasJsNamespace is has_js_namespace.
func (g *Generator) hasJsNamespace(p *sema.Program) bool {
	if g.noNS {
		return false
	}
	return p.Namespace("js") != ""
}

// jsNamespace is js_namespace.
func (g *Generator) jsNamespace(p *sema.Program) string {
	if g.noNS {
		return ""
	}
	ns := p.Namespace("js")
	if ns != "" {
		ns += "."
	}
	return ns
}

// ---- TypeScript module helpers ----

// tsIndent is ts_indent.
func (g *Generator) tsIndent() string {
	if g.tsModule != "" {
		return g.indent() + "  "
	}
	return g.indent()
}

// tsDeclare is ts_declare.
func (g *Generator) tsDeclare() string {
	if g.tsModule != "" {
		return ""
	}
	if g.opts.Node {
		return "declare "
	}
	return "export declare "
}

// tsGetReq is ts_get_req.
func tsGetReq(f *sema.Field) string {
	if f.Req() == sema.Optional || f.Value() != nil {
		return "?"
	}
	return ""
}

// tsPrintDoc is ts_print_doc. It uses the free-function
// std::getline(istream&, string&), which has no 1024-character line-length
// quirk, unlike t_generator::generate_docstring_comment.
func (g *Generator) tsPrintDoc(doc docHolder) string {
	result := "\n"
	if doc.HasDoc() {
		result += g.tsIndent() + "/**" + "\n"
		for _, item := range getlineSplit(doc.Doc(), '\n') {
			result += g.tsIndent() + " * " + item + "\n"
		}
		result += g.tsIndent() + " */" + "\n"
	}
	return result
}

type docHolder interface {
	HasDoc() bool
	Doc() string
}

// makeValidNodeJsIdentifier is make_valid_nodeJs_identifier.
func makeValidNodeJsIdentifier(name string) string {
	if name == "" {
		return name
	}
	b := []byte(name)
	if c := b[0]; '0' <= c && c <= '9' {
		b = append([]byte{'_'}, b...)
	}
	for i, c := range b {
		if !(('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z') || ('0' <= c && c <= '9') || c == '_' || c == '$') {
			b[i] = '_'
		}
	}
	return string(b)
}

// findField is find_field.
func findField(fields []*sema.Field, name string) bool {
	for _, f := range fields {
		if f.Name() == name {
			return true
		}
	}
	return false
}

// nextIdentifierName is next_identifier_name.
func (g *Generator) nextIdentifierName(fields []*sema.Field, baseName string) string {
	current := makeValidNodeJsIdentifier(baseName)
	for findField(fields, current) {
		current = makeValidNodeJsIdentifier("_" + current)
	}
	return current
}

// ---- includes ----

// jsIncludes is js_includes.
func (g *Generator) jsIncludes() string {
	if g.opts.Node {
		result := ""
		if g.opts.ESM {
			if g.opts.Bigint {
				// Bigint codegen references `thrift.toBigInt` /
				// `thrift.fromBigInt` at deserialize / serialize sites, so
				// the ESM include path needs a namespace binding alongside
				// the named `Thrift` import.
				result += "import * as thrift from 'thrift';\n"
				result += "const { Thrift } = thrift;\n"
			} else {
				result += "import { Thrift } from 'thrift';\n"
			}
		} else {
			result += g.jsConstType + "thrift = require('thrift');\n" +
				g.jsConstType + "Thrift = thrift.Thrift;\n"
		}
		if !g.opts.NativePromise && !g.opts.ES6 {
			if g.opts.ESM {
				result += "import Q from 'q';\n"
			} else {
				result += g.jsConstType + "Q = require('q');\n"
			}
		}
		if g.opts.ESM {
			if !g.opts.Bigint {
				result += "import Int64 from 'node-int64';\n"
			}
		} else {
			if !g.opts.Bigint {
				result += g.jsConstType + "Int64 = require('node-int64');\n"
			}
		}
		return result
	}
	return "if (typeof Int64 === 'undefined' && typeof require === 'function') {\n  " + g.jsConstType + "Int64 = require('node-int64');\n}\n"
}

// tsIncludes is ts_includes.
func (g *Generator) tsIncludes() string {
	if g.opts.Node {
		result := "import thrift = require('thrift');\n" +
			"import Thrift = thrift.Thrift;\n"
		if !g.opts.NativePromise {
			result += "import Q = require('q');\n"
		}
		if !g.opts.Bigint {
			result += "import Int64 = require('node-int64');\n"
		}
		result += "type uuid = string;"
		return result
	}
	return "import Int64 = require('node-int64');\n" +
		"type uuid = string;"
}

// tsServiceIncludes is ts_service_includes. It is defined but never
// called in the C++ source; ported for fidelity.
func (g *Generator) tsServiceIncludes() string {
	if g.opts.Node {
		result := "import thrift = require('thrift');\n" +
			"import Thrift = thrift.Thrift;"
		if !g.opts.NativePromise {
			result += "\nimport Q = require('q');"
		}
		if !g.opts.Bigint {
			result += "\nimport Int64 = require('node-int64');"
		}
		return result
	}
	return "import Int64 = require('node-int64');"
}

// renderIncludes is render_includes.
func (g *Generator) renderIncludes() string {
	result := ""
	if g.opts.Node {
		includes := g.program.Includes()
		for _, inc := range includes {
			if g.opts.ESM {
				result += "import * as " + makeValidNodeJsIdentifier(inc.Name()) + "_ttypes from '" + g.getImportPathProgram(inc) + "';\n"
			} else {
				result += g.jsConstType + makeValidNodeJsIdentifier(inc.Name()) + "_ttypes = require('" + g.getImportPathProgram(inc) + "');\n"
			}
		}
		if len(includes) > 0 {
			result += "\n"
		}
	}
	return result
}

// renderTsIncludes is render_ts_includes.
func (g *Generator) renderTsIncludes() string {
	result := ""
	if !g.opts.Node {
		return result
	}
	includes := g.program.Includes()
	for _, inc := range includes {
		includeName := makeValidNodeJsIdentifier(inc.Name()) + "_ttypes"
		g.include2ImportName[inc] = includeName
		result += "import " + includeName + " = require('" + g.getImportPathProgram(inc) + "');\n"
	}
	if len(includes) > 0 {
		result += "\n"
	}
	return result
}

// getImportPathProgram is get_import_path(t_program*).
func (g *Generator) getImportPathProgram(program *sema.Program) string {
	importFileName := program.Name() + "_types"
	ext := ".js"
	if g.opts.ESM {
		ext = ".mjs"
	}
	importFileNameWithExtension := importFileName + ext
	if program.Recursive() {
		return "./" + importFileNameWithExtension
	}
	if p, ok := g.moduleName2ImportPath[importFileName]; ok {
		return p
	}
	return "./" + importFileNameWithExtension
}

// getImportPathService is get_import_path(t_service*).
func (g *Generator) getImportPathService(service *sema.Service) string {
	importFileName := service.Name()
	ext := ".js"
	if g.opts.ESM {
		ext = ".mjs"
	}
	importFileNameWithExtension := importFileName + ext
	if p, ok := g.moduleName2ImportPath[importFileName]; ok {
		return p
	}
	return "./" + importFileNameWithExtension
}

// ---- init and close ----

func (g *Generator) initGenerator() {
	outdir := g.outDir()
	emit.Mkdir(outdir)

	if g.opts.GenEpisodeFile {
		typesModule := g.program.Name() + "_types"
		g.fEpisode.WriteString(typesModule + ":" + g.opts.ThriftPackageOutputDirectory + "/" + typesModule + "\n")
	}

	g.fTypes.WriteString(g.autogenComment())

	if (g.opts.Node || g.opts.ES6) && g.noNS {
		g.fTypes.WriteString("\"use strict\";\n\n")
	}

	g.fTypes.WriteString(g.jsIncludes() + "\n" + g.renderIncludes() + "\n")

	if g.opts.TS {
		g.fTypesTS.WriteString(g.autogenComment() + g.tsIncludes() + "\n" + g.renderTsIncludes() + "\n")
	}

	if g.opts.Node {
		if g.opts.ESM {
			g.fTypes.WriteString("import * as ttypes from './" + g.program.Name() + "_types.mjs';\n")
		} else {
			g.fTypes.WriteString(g.jsConstType + "ttypes = module.exports = {};\n")
		}
	}

	pns := ""
	nsPieces := g.jsNamespacePieces(g.program)
	if len(nsPieces) > 0 {
		for i, piece := range nsPieces {
			if i > 0 {
				pns += "."
			}
			pns += piece
			g.fTypes.WriteString("if (typeof " + pns + " === 'undefined') {\n")
			g.fTypes.WriteString("  " + pns + " = {};\n")
			g.fTypes.WriteString("}\n")
			g.fTypes.WriteString("if (typeof module !== 'undefined' && module.exports) {\n")
			g.fTypes.WriteString("  module.exports." + pns + " = " + pns + ";\n}\n")
		}
		if g.opts.TS {
			g.tsModule = pns
			g.fTypesTS.WriteString("declare module " + g.tsModule + " {")
		}
	}
}

func (g *Generator) closeGenerator() {
	outdir := g.outDir()
	ext := ".js"
	if g.opts.ESM {
		ext = ".mjs"
	}
	emit.WriteFile(outdir+g.program.Name()+"_types"+ext, g.fTypes.String())

	if g.opts.TS {
		if g.tsModule != "" {
			g.fTypesTS.WriteString("}")
		}
		emit.WriteFile(outdir+g.program.Name()+"_types.d.ts", g.fTypesTS.String())
	}
	if g.opts.GenEpisodeFile {
		emit.WriteFile(outdir+episodeFileName, g.fEpisode.String())
	}
}

// parseImports is parse_imports.
func (g *Generator) parseImports(importsString string) {
	if g.program.Recursive() {
		emit.Throw("[-gen js:imports=] option is not usable in recursive code generation mode")
	}
	imports := getlineSplit(importsString, ':')
	if len(imports) == 0 {
		emit.Throw("invalid usage: [-gen js:imports=] requires at least one path (multiple paths are separated by ':')")
	}
	for _, imp := range imports {
		if strings.HasSuffix(imp, "/") {
			imp = imp[:len(imp)-1]
		}
		if imp == "" {
			emit.Throw("empty paths are not allowed in imports")
		}
		episodeFilePath := imp + "/" + episodeFileName
		content, err := os.ReadFile(episodeFilePath)
		if err != nil {
			emit.Throw("failed to open the file '%s'", episodeFilePath)
		}
		for _, line := range getlineSplit(string(content), '\n') {
			sep := strings.IndexByte(line, ':')
			if sep < 0 {
				emit.Throw("the episode file '%s' is malformed, the line '%s' does not have a key:value separator ':'", episodeFilePath, line)
			}
			moduleName := line[:sep]
			importPath := line[sep+1:]
			if moduleName == "" {
				emit.Throw("the episode file '%s' is malformed, the module name is empty", episodeFilePath)
			}
			if importPath == "" {
				emit.Throw("the episode file '%s' is malformed, the import path is empty", episodeFilePath)
			}
			base := imp
			if i := strings.LastIndexByte(imp, '/'); i >= 0 {
				base = imp[i+1:]
			}
			moduleImportPath := base + "/" + importPath
			if existing, ok := g.moduleName2ImportPath[moduleName]; ok {
				emit.Throw("multiple providers of import path found for %s\n\t%s\n\t%s", moduleName, moduleImportPath, existing)
			}
			g.moduleName2ImportPath[moduleName] = moduleImportPath
		}
	}
}

// ---- shared field / signature helpers ----

// declareField is declare_field.
func (g *Generator) declareField(f *sema.Field, init, obj bool) string {
	result := "this." + f.Name()
	if !obj {
		result = g.jsLetType + f.Name()
	}
	if init {
		t := sema.TrueType(f.Type())
		switch {
		case t.IsBaseType():
			switch t.(*sema.BaseType).Base() {
			case sema.TypeVoid:
			case sema.TypeString, sema.TypeUUID, sema.TypeBool, sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64, sema.TypeDouble:
				result += " = null"
			default:
				emit.Throw("compiler error: no JS initializer for base type %s", sema.BaseName(t.(*sema.BaseType).Base()))
			}
		case t.IsEnum():
			result += " = null"
		case t.IsMap():
			result += " = null"
		case t.IsContainer():
			result += " = null"
		case t.IsStruct() || t.IsXception():
			if obj {
				result += " = new " + g.jsTypeNamespace(t.Program()) + t.Name() + "()"
			} else {
				result += " = null"
			}
		}
	} else {
		result += " = null"
	}
	return result
}

// functionSignature is function_signature.
func (g *Generator) functionSignature(f *sema.Function, prefix string, includeCallback bool) string {
	return prefix + f.Name() + " = function(" + g.argumentList(f.Arglist(), includeCallback) + ")"
}

// argumentList is argument_list.
func (g *Generator) argumentList(s *sema.Struct, includeCallback bool) string {
	result := ""
	first := true
	for _, f := range s.Members() {
		if first {
			first = false
		} else {
			result += ", "
		}
		result += f.Name()
	}
	if includeCallback {
		if len(s.Members()) > 0 {
			result += ", "
		}
		result += "callback"
	}
	return result
}

// typeToEnum is type_to_enum.
func typeToEnum(t sema.Type) string {
	t = sema.TrueType(t)
	switch {
	case t.IsBaseType():
		switch t.(*sema.BaseType).Base() {
		case sema.TypeVoid:
			emit.Throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			return "Thrift.Type.STRING"
		case sema.TypeUUID:
			return "Thrift.Type.UUID"
		case sema.TypeBool:
			return "Thrift.Type.BOOL"
		case sema.TypeI8:
			return "Thrift.Type.BYTE"
		case sema.TypeI16:
			return "Thrift.Type.I16"
		case sema.TypeI32:
			return "Thrift.Type.I32"
		case sema.TypeI64:
			return "Thrift.Type.I64"
		case sema.TypeDouble:
			return "Thrift.Type.DOUBLE"
		default:
			emit.Throw("compiler error: unhandled js type")
		}
	case t.IsEnum():
		return "Thrift.Type.I32"
	case t.IsStruct(), t.IsXception():
		return "Thrift.Type.STRUCT"
	case t.IsMap():
		return "Thrift.Type.MAP"
	case t.IsSet():
		return "Thrift.Type.SET"
	case t.IsList():
		return "Thrift.Type.LIST"
	}
	emit.Throw("INVALID TYPE IN type_to_enum: %s", t.Name())
	return ""
}

// getContainedType is get_contained_type: the element type of a container,
// or for a map the value type (keys are always strings in js).
func getContainedType(t sema.Type) sema.Type {
	switch {
	case t.IsList():
		return t.(*sema.List).ElemType()
	case t.IsSet():
		return t.(*sema.Set).ElemType()
	default:
		return t.(*sema.Map).ValType()
	}
}
