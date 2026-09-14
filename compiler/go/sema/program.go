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
	"io"
	"sort"
	"strings"
)

// Error is a fatal semantic error. The C++ compiler exits on these; the Go
// loader returns them.
type Error struct {
	Msg string
}

func (e *Error) Error() string { return e.Msg }

// fail aborts the current load with an Error. Load recovers it.
func fail(format string, args ...interface{}) {
	panic(&Error{Msg: fmt.Sprintf(format, args...)})
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
}

func (d *Diagnostics) warn(level int, msg string) {
	if d == nil || d.Out == nil || d.WarnLevel < level {
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
// collisions return true; the warnings for possible ones are dropped.
func (p *Program) isCommonNamespace(prog *Program, t Type) bool {
	if prog == t.Program() {
		return true
	}
	match := true
	for _, k := range prog.NamespaceKeys() {
		if prog.namespaces[k] != t.Program().Namespace(k) {
			match = false
		}
	}
	for _, k := range t.Program().NamespaceKeys() {
		if t.Program().namespaces[k] != prog.Namespace(k) {
			match = false
		}
	}
	return match
}
