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
	"os"
	"path/filepath"
	"strings"

	"github.com/apache/thrift/compiler/go/idl/ast"
	"github.com/apache/thrift/compiler/go/idl/parser"
	"github.com/apache/thrift/compiler/go/idl/scanner"
)

// Loader reads a Thrift file and everything it includes. It is the Go
// counterpart of parse() and include_file() in main.cc.
type Loader struct {
	// IncludeDirs are the -I directories, searched in order after the
	// directory of the including file.
	IncludeDirs []string
	// Strict is g_strict; -strict sets it to 255. Values of 192 and above
	// turn a missing include and an implicit field id into errors.
	Strict int
	// AllowNegFieldKeys is -allow-neg-keys.
	AllowNegFieldKeys bool
	// Allow64BitConsts is -allow-64bit-consts.
	Allow64BitConsts bool
	// Diag receives warnings; nil discards them.
	Diag *Diagnostics

	known      map[string]bool
	byteWarned bool
	// sources holds the text of files given to LoadSource, read instead
	// of the file system.
	sources map[string][]byte
}

// Load parses the file named on the command line and its includes, and
// returns the resolved root program.
func (l *Loader) Load(inputPath string) (prog *Program, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*Error); ok {
				prog, err = nil, e
				return
			}
			panic(r)
		}
	}()
	rp, ok := realpath(inputPath)
	if !ok {
		fail("Could not open input file with realpath: %s", inputPath)
	}
	prog = NewProgram(rp, l.Diag)
	// The include prefix is inferred from the path as given on the
	// command line, not from the real path.
	prefix := ""
	if slash := strings.LastIndexByte(inputPath, '/'); slash >= 0 {
		prefix = inputPath[:slash]
	}
	prog.SetIncludePrefix(prefix)
	// The byte warning is once per process in the C++ compiler, so it is
	// once per Loader here: the audit mode loads two files through one.
	l.known = map[string]bool{}
	l.parse(prog, nil)
	return prog, nil
}

// LoadSource is Load for a file held in memory: path names it, for
// messages and for resolving its includes, and need not exist. Tests and
// the fuzz target use it.
func (l *Loader) LoadSource(path string, src []byte) (prog *Program, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*Error); ok {
				prog, err = nil, e
				return
			}
			panic(r)
		}
	}()
	abs, err := filepath.Abs(path)
	if err != nil {
		fail("Could not open input file with realpath: %s", path)
	}
	abs = filepath.ToSlash(abs)
	if l.sources == nil {
		l.sources = map[string][]byte{}
	}
	l.sources[abs] = src
	defer delete(l.sources, abs)
	prog = NewProgram(abs, l.Diag)
	prefix := ""
	if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
		prefix = path[:slash]
	}
	prog.SetIncludePrefix(prefix)
	l.known = map[string]bool{}
	l.parse(prog, nil)
	return prog, nil
}

// realpath resolves symlinks and requires the file to exist, like the
// C library function.
func realpath(path string) (string, bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	rp, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", false
	}
	return filepath.ToSlash(rp), true
}

func directoryName(path string) string {
	if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
		return path[:slash]
	}
	return "."
}

// includeFile is include_file in main.cc. It returns the empty string
// when the include cannot be found, after warning or failing.
func (l *Loader) includeFile(curdir, filename string) string {
	if strings.HasPrefix(filename, "/") {
		if rp, ok := realpath(filename); ok {
			return rp
		}
		if l.Diag != nil {
			l.Diag.warn(0, "Cannot open include file "+filename)
		}
		return ""
	}
	search := append([]string{curdir}, l.IncludeDirs...)
	for _, dir := range search {
		if rp, ok := realpath(dir + "/" + filename); ok {
			return rp
		}
	}
	if l.Strict >= 192 {
		fail("Could not find include file %s", filename)
	}
	if l.Diag != nil {
		l.Diag.warn(0, "Could not find include file "+filename)
	}
	return ""
}

func (l *Loader) parse(prog *Program, parent *Program) {
	path := prog.Path()
	if l.known[path] {
		fail("Recursion detected, file: \"%s\"", path)
	}
	l.known[path] = true

	src, ok := l.sources[path]
	if !ok {
		var err error
		if src, err = os.ReadFile(path); err != nil {
			fail("Could not open input file: \"%s\"", path)
		}
	}
	if l.Diag != nil {
		l.Diag.Path = path
	}
	warn := func(line, level int, msg string) {
		if msg == scanner.ByteAliasWarning {
			// emit_byte_type_warning fires once per compiler run.
			if l.byteWarned {
				return
			}
			l.byteWarned = true
		}
		if l.Diag != nil {
			l.Diag.Path = path
			l.Diag.Line = line
			l.Diag.warn(level, msg)
		}
	}
	tree, perr := parser.Parse(path, src, warn)
	if perr != nil {
		e := &Error{Msg: path + ":" + perr.Error(), Path: path, Text: perr.Error()}
		if pe, ok := perr.(*parser.Error); ok {
			e.Pos, e.Text = pe.Pos, pe.Msg
		}
		panic(e)
	}

	// Includes pass: register every include, then parse each one.
	curdir := directoryName(path)
	for _, h := range tree.Headers {
		inc, ok := h.(*ast.Include)
		if !ok {
			continue
		}
		if p := l.includeFile(curdir, inc.Path); p != "" {
			prog.AddInclude(p, inc.Path)
		}
	}
	for _, inc := range prog.Includes() {
		l.parse(inc, prog)
	}

	// Program pass.
	if l.Diag != nil {
		l.Diag.Path = path
	}
	b := &builder{
		p:                 prog,
		diag:              l.Diag,
		strict:            l.Strict,
		allowNegFieldKeys: l.AllowNegFieldKeys,
		allow64BitConsts:  l.Allow64BitConsts,
		prefix:            prog.Name() + ".",
	}
	if parent != nil {
		b.parent = parent.Scope
	}
	b.build(tree)

	delete(l.known, path)
}
