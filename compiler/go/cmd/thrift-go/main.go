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

// Command thrift-go compiles Thrift IDL, audits two IDL files for
// compatibility, and decodes Thrift-encoded bytes.
//
// It has two command lines. The first word selects a subcommand:
//
//	thrift-go generate --lang go --out ./gen -I ./idl file.thrift...
//	thrift-go audit [flags] old.thrift new.thrift
//	thrift-go check [flags] file.thrift...
//	thrift-go decode [flags] [file]
//	thrift-go languages [--json]
//	thrift-go version
//	thrift-go help [command]
//
// Anything else is the C++ thrift compiler's command line, accepted
// unchanged so that the existing build files can point their THRIFT
// variable at this binary:
//
//	thrift-go [options] --gen go[:option,...] file.thrift
//
// The generators come from the registry in compiler/go/generate.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	_ "github.com/apache/thrift/compiler/go/generate/golang"   // registers go
	_ "github.com/apache/thrift/compiler/go/generate/gv"       // registers gv
	_ "github.com/apache/thrift/compiler/go/generate/html"     // registers html
	_ "github.com/apache/thrift/compiler/go/generate/java"     // registers java
	_ "github.com/apache/thrift/compiler/go/generate/json"     // registers json
	_ "github.com/apache/thrift/compiler/go/generate/markdown" // registers markdown
	_ "github.com/apache/thrift/compiler/go/generate/mmd"      // registers mmd
	_ "github.com/apache/thrift/compiler/go/generate/rs"       // registers rs
	_ "github.com/apache/thrift/compiler/go/generate/xml"      // registers xml
	_ "github.com/apache/thrift/compiler/go/generate/xsd"      // registers xsd
)

// Exit statuses, shared by both command lines.
const (
	exitOK        = 0
	exitError     = 1 // usage, unreadable input, parse or validation error
	exitPolicy    = 2 // an audit failure, or a warning under check --strict
	exitGenerator = 3 // a generator failed on a valid program
)

// command is one subcommand.
type command struct {
	name    string
	summary string
	run     func(args []string) int
}

var commands []command

// The table is filled in init because help refers back to it.
func init() {
	commands = []command{
		{"generate", "generate code for one or more languages", runGenerate},
		{"audit", "compare a new IDL file against an old one for wire compatibility", runAudit},
		{"check", "parse and validate IDL files without generating", runCheck},
		{"decode", "print Thrift-encoded bytes as a tree, without an IDL", runDecode},
		{"languages", "list the generators and their options", runLanguages},
		{"version", "print the compiler version", runVersion},
		{"help", "describe a command, or the exit statuses with 'help exit-codes'", runHelp},
	}
}

// aliases map short forms to commands.
var aliases = map[string]string{"gen": "generate"}

func lookupCommand(name string) (command, bool) {
	if full, ok := aliases[name]; ok {
		name = full
	}
	for _, c := range commands {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	if c, ok := lookupCommand(os.Args[1]); ok {
		os.Exit(c.run(os.Args[2:]))
	}
	// A legacy invocation starts with an option; a bare file name with no
	// options is a usage error in both forms and gets both usages.
	if !strings.HasPrefix(os.Args[1], "-") && len(os.Args) == 2 {
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		overview(os.Stderr)
		os.Exit(exitError)
	}
	runLegacy()
}

// overview prints the subcommand summary.
func overview(w *os.File) {
	fmt.Fprintf(w, "Usage: %s <command> [flags] [arguments]\n", os.Args[0])
	fmt.Fprintf(w, "       %s [options] --gen <language>[:option,...] file.thrift   (the C++ compiler's form)\n\n", os.Args[0])
	fmt.Fprintln(w, "Commands:")
	for _, c := range commands {
		fmt.Fprintf(w, "  %-10s %s\n", c.name, c.summary)
	}
	fmt.Fprintf(w, "\nUse %s help <command> for the flags of a command.\n", os.Args[0])
}

func runHelp(args []string) int {
	if len(args) == 0 {
		overview(os.Stdout)
		return exitOK
	}
	switch args[0] {
	case "exit-codes":
		fmt.Println("Exit statuses:")
		fmt.Printf("  %d  success; warnings may have been printed\n", exitOK)
		fmt.Printf("  %d  usage error, unreadable input, parse or validation error\n", exitError)
		fmt.Printf("  %d  audit found an incompatible change; check --strict found a warning\n", exitPolicy)
		fmt.Printf("  %d  a generator failed on a valid program\n", exitGenerator)
		return exitOK
	case "legacy":
		help()
		return exitOK
	}
	c, ok := lookupCommand(args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		overview(os.Stderr)
		return exitError
	}
	// Each command prints its usage and exits 0 on -h; route through it.
	return c.run([]string{"-h"})
}

func runVersion(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "version takes no arguments")
		return exitError
	}
	fmt.Println(versionString())
	return exitOK
}

// languageNames lists the registered generators.
func languageNames() []string {
	var names []string
	for _, info := range generate.All() {
		names = append(names, info.Name)
	}
	sort.Strings(names)
	return names
}
