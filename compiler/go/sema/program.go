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

package sema

import (
	"fmt"
	"github.com/apache/thrift/compiler/go/idl/ast"
	"io"
	"sort"
	"strings"

	"github.com/apache/thrift/compiler/go/idl/token"
)

// Error is a fatal semantic error. The C++ compiler exits on these; the Go
// loader returns them.
type Error struct {
	// Msg is the message in the C++ compiler's wording, with the path and
	// line where the C++ compiler prints them.
	Msg string
	// Path, Pos and Text are set for a syntax error: the file, where the
	// offending token starts, and the message alone, for a diagnostic of
	// the form path:line:col: text.
	Path string
	Pos  token.Pos
	Text string

	// Line and Last are the C++ lexer's yylineno and yytext when the
	// grammar action that reports the error runs; Line is 0 when the error
	// is not tied to a place in a file. Yacc is set when the C++ compiler
	// reports it through yyerror() as "[ERROR:path:line] (last token was
	// 'Last')" rather than through failure(), and Syntax when it is a
	// parse error, which failure() then follows with "Parser error during
	// include pass.".
	Line   int
	Last   string
	Yacc   bool
	Syntax bool
}

func (e *Error) Error() string { return e.Msg }

// fail aborts the current load with an Error. Load recovers it.
func fail(format string, args ...interface{}) {
	panic(&Error{Msg: fmt.Sprintf(format, args...)})
}

// failYacc is fail for an error the C++ grammar reports with yyerror().
func failYacc(format string, args ...interface{}) {
	panic(&Error{Msg: fmt.Sprintf(format, args...), Yacc: true})
}

// at runs fn and gives an Error raised in it, which has no place yet, the
// place the C++ compiler reports it at: path and the lexer state m.
func at(path string, m ast.Mark, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*Error); ok && e.Line == 0 {
				e.Path, e.Line, e.Last = path, m.Line, m.Text
			}
			panic(r)
		}
	}()
	fn()
}

// Diagnostics collects warnings. Level follows pwarning: 0 always shown
// unless -nowarn, 1 the default, 2 only with -strict.
type Diagnostics struct {
	// Out receives the warnings; nil discards them.
	Out io.Writer
	// WarnLevel is g_warn: warnings above this level are dropped.
	WarnLevel int
	// Path and Line name the position the next warning refers to.
	Path string
	Line int
	// Plain selects the path:line: warning: message form over the C++
	// compiler's [WARNING:path:line] message.
	Plain bool
	// Count is the number of warnings written.
	Count int
}

func (d *Diagnostics) warn(level int, msg string) {
	if d == nil || d.Out == nil || d.WarnLevel < level {
		return
	}
	d.Count++
	if d.Plain {
		// Some C++ messages end in their own newline, which pwarning()
		// follows with another; the plain form is one line.
		fmt.Fprintf(d.Out, "%s:%d: warning: %s\n", d.Path, d.Line, strings.TrimRight(msg, "\n"))
		return
	}
	fmt.Fprintf(d.Out, "[WARNING:%s:%d] %s\n", d.Path, d.Line, msg)
}

// Program is t_program: one parsed and resolved IDL file.
type Program struct {
	docBase
	path          string
	name          string
	outPath       string
	outPathIsAbs  bool
	namespace     string
	includes      []*Program
	includePrefix string
	Scope         *Scope

	typedefs  []*Typedef
	enums     []*Enum
	consts    []*Const
	objects   []*Struct
	structs   []*Struct
	xceptions []*Struct
	services  []*Service

	namespaces           map[string]string
	namespaceAnnotations map[string]Annotations
	cppIncludes          []string
	cIncludes            []string
	recursive            bool

	diag *Diagnostics
}

// NewProgram creates a program for the file at path. The name is the file
// name without directory and extension, as in program_name().
func NewProgram(path string, diag *Diagnostics) *Program {
	return &Program{
		path:                 path,
		name:                 ProgramName(path),
		outPath:              "./",
		Scope:                NewScope(),
		namespaces:           map[string]string{},
		namespaceAnnotations: map[string]Annotations{},
		diag:                 diag,
	}
}

// ProgramName is program_name() in main.cc.
func ProgramName(filename string) string {
	if slash := strings.LastIndexByte(filename, '/'); slash >= 0 {
		filename = filename[slash+1:]
	}
	if dot := strings.LastIndexByte(filename, '.'); dot >= 0 {
		filename = filename[:dot]
	}
	return filename
}

func (p *Program) Path() string            { return p.path }
func (p *Program) Name() string            { return p.name }
func (p *Program) OutPath() string         { return p.outPath }
func (p *Program) IsOutPathAbsolute() bool { return p.outPathIsAbs }
func (p *Program) IncludePrefix() string   { return p.includePrefix }
func (p *Program) Typedefs() []*Typedef    { return p.typedefs }
func (p *Program) Enums() []*Enum          { return p.enums }
func (p *Program) Consts() []*Const        { return p.consts }
func (p *Program) Structs() []*Struct      { return p.structs }
func (p *Program) Xceptions() []*Struct    { return p.xceptions }
func (p *Program) Objects() []*Struct      { return p.objects }
func (p *Program) Services() []*Service    { return p.services }
func (p *Program) Includes() []*Program    { return p.includes }
func (p *Program) CppIncludes() []string   { return p.cppIncludes }
func (p *Program) CIncludes() []string     { return p.cIncludes }
func (p *Program) Recursive() bool         { return p.recursive }

// SetRecursive marks the program as generated under -r.
func (p *Program) SetRecursive(v bool) { p.recursive = v }

// Diagnostics returns the warning sink.
func (p *Program) Diagnostics() *Diagnostics { return p.diag }

// SetOutPath is t_program::set_out_path; a trailing slash is ensured.
func (p *Program) SetOutPath(outPath string, absolute bool) {
	p.outPath = outPath
	p.outPathIsAbs = absolute
	if !strings.HasSuffix(p.outPath, "/") && !strings.HasSuffix(p.outPath, "\\") {
		p.outPath += "/"
	}
}

func (p *Program) addTypedef(t *Typedef) { p.typedefs = append(p.typedefs, t) }
func (p *Program) addEnum(e *Enum)       { p.enums = append(p.enums, e) }
func (p *Program) addConst(c *Const)     { p.consts = append(p.consts, c) }

func (p *Program) addStruct(s *Struct) {
	p.objects = append(p.objects, s)
	p.structs = append(p.structs, s)
}

func (p *Program) addXception(s *Struct) {
	p.objects = append(p.objects, s)
	p.xceptions = append(p.xceptions, s)
}

func (p *Program) addService(s *Service) {
	s.validateUniqueMembers()
	p.services = append(p.services, s)
}

// AddInclude registers an included program. includeSite is the string
// from the include statement; its directory part becomes the include
// prefix.
func (p *Program) AddInclude(path, includeSite string) *Program {
	prog := NewProgram(path, p.diag)
	prefix := ""
	if slash := strings.LastIndexByte(includeSite, '/'); slash >= 0 {
		prefix = includeSite[:slash]
	}
	prog.SetIncludePrefix(prefix)
	p.includes = append(p.includes, prog)
	return prog
}

// SetIncludePrefix sets the directory the program was included from.
func (p *Program) SetIncludePrefix(prefix string) {
	p.includePrefix = prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		p.includePrefix += "/"
	}
}

// Namespace is get_namespace(language): the namespace for the language,
// falling back to the "*" namespace, or empty.
func (p *Program) Namespace(language string) string {
	if ns, ok := p.namespaces[language]; ok {
		return ns
	}
	if ns, ok := p.namespaces["*"]; ok {
		return ns
	}
	return ""
}

// Namespaces returns every namespace declaration keyed by language.
func (p *Program) Namespaces() map[string]string { return p.namespaces }

// NamespaceKeys returns the declared namespace languages in sorted order.
func (p *Program) NamespaceKeys() []string {
	keys := make([]string, 0, len(p.namespaces))
	for k := range p.namespaces {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SetNamespace is t_program::set_namespace. The generator existence check
// of the C++ compiler only produces warnings and is not reproduced.
func (p *Program) SetNamespace(language, namespace string) {
	if language != "*" {
		base := language
		if i := strings.IndexByte(language, '.'); i >= 0 {
			base = language[:i]
		}
		if base == "smalltalk" {
			p.diag.warn(1, "Namespace 'smalltalk' is deprecated. Use 'st' instead")
		}
	}
	p.namespaces[language] = namespace
}

// NamespaceAnnotations returns the annotations on a namespace declaration.
func (p *Program) NamespaceAnnotations(language string) Annotations {
	return p.namespaceAnnotations[language]
}

func (p *Program) setNamespaceAnnotations(language string, a Annotations) {
	p.namespaceAnnotations[language] = a
}

func (p *Program) addCppInclude(path string) { p.cppIncludes = append(p.cppIncludes, path) }

// IsUniqueTypename is t_program::is_unique_typename.
func (p *Program) IsUniqueTypename(t Type) bool {
	occurrences := p.programTypenameCount(p, t)
	for _, inc := range p.includes {
		occurrences += p.programTypenameCount(inc, t)
	}
	return occurrences == 0
}

func (p *Program) programTypenameCount(prog *Program, t Type) int {
	n := 0
	for _, x := range prog.typedefs {
		n += p.typenameCollision(prog, Type(x), t)
	}
	for _, x := range prog.enums {
		n += p.typenameCollision(prog, Type(x), t)
	}
	for _, x := range prog.objects {
		n += p.typenameCollision(prog, Type(x), t)
	}
	for _, x := range prog.services {
		n += p.typenameCollision(prog, Type(x), t)
	}
	return n
}

func (p *Program) typenameCollision(prog *Program, existing, t Type) int {
	if existing != t && t.Name() == existing.Name() && p.isCommonNamespace(prog, t) {
		return 1
	}
	return 0
}

// isCommonNamespace is t_program::is_common_namespace. Only guaranteed
// collisions return true; possible ones are reported as warnings.
func (p *Program) isCommonNamespace(prog *Program, t Type) bool {
	tp := t.Program()
	// Case 1: the typenames are in the same program [collision].
	if prog == tp {
		p.diag.warn(1, fmt.Sprintf("Duplicate typename %s found in %s", t.Name(), tp.Name()))
		return true
	}
	// Case 2: both programs have identical namespace scope/name
	// declarations [collision].
	match := true
	for _, k := range prog.NamespaceKeys() {
		if prog.namespaces[k] == tp.Namespace(k) {
			p.diag.warn(1, fmt.Sprintf("Duplicate typename %s found in %s,%s,%s and %s,%s,%s [file,scope,ns]",
				t.Name(), tp.Name(), k, prog.namespaces[k], prog.Name(), k, prog.namespaces[k]))
		} else {
			match = false
		}
	}
	for _, k := range tp.NamespaceKeys() {
		if tp.namespaces[k] == prog.Namespace(k) {
			p.diag.warn(1, fmt.Sprintf("Duplicate typename %s found in %s,%s,%s and %s,%s,%s [file,scope,ns]",
				t.Name(), tp.Name(), k, tp.namespaces[k], prog.Name(), k, tp.namespaces[k]))
		} else {
			match = false
		}
	}
	if len(prog.namespaces) == 0 && len(tp.namespaces) == 0 {
		p.diag.warn(1, fmt.Sprintf("Duplicate typename %s found in %s and %s", t.Name(), tp.Name(), prog.Name()))
	}
	return match
}
