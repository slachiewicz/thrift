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

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// stringList is a repeatable string flag.
type stringList []string

func (l *stringList) String() string     { return strings.Join(*l, ",") }
func (l *stringList) Set(v string) error { *l = append(*l, v); return nil }

// newFlagSet makes a flag set that prints its usage on -h and exits 0,
// and returns a non-zero status through exitCode on any other parse
// error.
func newFlagSet(name, args string, describe func(w io.Writer)) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s %s %s\n\n", os.Args[0], name, args)
		describe(fs.Output())
		fmt.Fprintln(fs.Output(), "\nFlags:")
		fs.PrintDefaults()
	}
	return fs
}

// parseFlags parses args and reports the exit status to use when parsing
// ended the command: 0 for -h, 1 for a bad flag, -1 to continue.
func parseFlags(fs *flag.FlagSet, args []string) int {
	err := fs.Parse(args)
	switch {
	case err == flag.ErrHelp:
		fs.SetOutput(os.Stdout)
		fs.Usage()
		return exitOK
	case err != nil:
		return exitError
	}
	return -1
}

// loaderFlags are the front-end flags every IDL-reading command shares.
type loaderFlags struct {
	includes stringList
	strict   bool
	noWarn   bool
	negKeys  bool
	consts64 bool
}

func (f *loaderFlags) register(fs *flag.FlagSet) {
	fs.Var(&f.includes, "I", "directory searched for include directives; repeatable")
	fs.BoolVar(&f.strict, "strict", false, "strict warnings, and a missing include is an error")
	fs.BoolVar(&f.noWarn, "no-warn", false, "suppress warnings")
	fs.BoolVar(&f.negKeys, "allow-neg-keys", false, "allow negative field keys")
	fs.BoolVar(&f.consts64, "allow-64bit-consts", false, "do not warn about 64-bit constants")
}

// loader builds the front end for the flags, with plain diagnostics on
// stderr.
func (f *loaderFlags) loader() *sema.Loader {
	diag := &sema.Diagnostics{Out: os.Stderr, WarnLevel: 1, Plain: true, Path: "arguments"}
	if f.noWarn {
		diag.WarnLevel = 0
	}
	l := &sema.Loader{IncludeDirs: f.includes, Diag: diag, AllowNegFieldKeys: f.negKeys, Allow64BitConsts: f.consts64}
	if f.strict {
		l.Strict = 255
		diag.WarnLevel = 2
	}
	return l
}

// report prints a load or generation error in the path:line:col form
// when the error carries a position.
func report(err error) {
	if e, ok := err.(*sema.Error); ok && e.Path != "" {
		if e.Pos.Line > 0 {
			fmt.Fprintf(os.Stderr, "%s:%d:%d: error: %s\n", e.Path, e.Pos.Line, e.Pos.Col, e.Text)
		} else {
			fmt.Fprintf(os.Stderr, "%s: error: %s\n", e.Path, e.Text)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
}

// versionString is the release version, with the module version and the
// VCS revision when the Go toolchain recorded them.
func versionString() string {
	s := "Thrift version " + version.Version
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return s
	}
	var extra []string
	if v := info.Main.Version; v != "" && v != "(devel)" {
		extra = append(extra, "module "+v)
	}
	var rev, modified string
	for _, kv := range info.Settings {
		switch kv.Key {
		case "vcs.revision":
			rev = kv.Value
		case "vcs.modified":
			modified = kv.Value
		}
	}
	if rev != "" {
		if len(rev) > 12 {
			rev = rev[:12]
		}
		if modified == "true" {
			rev += " (modified)"
		}
		extra = append(extra, "revision "+rev)
	}
	if len(extra) > 0 {
		s += " (" + strings.Join(extra, ", ") + ")"
	}
	return s
}
