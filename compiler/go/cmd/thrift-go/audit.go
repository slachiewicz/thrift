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
	"fmt"
	"io"
	"os"

	"github.com/apache/thrift/compiler/go/audit"
)

func runAudit(args []string) int {
	var lf loaderFlags
	var opts audit.Options
	fs := newFlagSet("audit", "[flags] old.thrift new.thrift", func(w io.Writer) {
		fmt.Fprintln(w, "Compare new.thrift against old.thrift and report the changes that break wire")
		fmt.Fprintln(w, "compatibility as failures, on stderr, and the rest as warnings, on stdout.")
		fmt.Fprintf(w, "Exit status %d when a failure was found, unless --no-fatal is given.\n", exitPolicy)
	})
	fs.BoolVar(&opts.AllowOptionalFieldRemoval, "allow-optional-field-removal", false, "allow explicitly optional fields to be removed")
	fs.BoolVar(&opts.AllowRequiredFieldToDefault, "allow-required-field-to-default", false, "allow required fields to use default requiredness (binding-dependent; includes service method arguments)")
	noFatal := fs.Bool("no-fatal", false, "exit 0 even when a failure was found")
	includeOld := fs.String("include-old", "", "directory searched for the old file's includes, after -I")
	includeNew := fs.String("include-new", "", "directory searched for the new file's includes, after -I")
	lf.register(fs)
	if code := parseFlags(fs, args); code >= 0 {
		return code
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "audit: exactly two files, old then new")
		return exitError
	}
	loader := lf.loader()
	opts.WarnLevel = loader.Diag.WarnLevel
	opts.Stdout, opts.Stderr = os.Stdout, os.Stderr

	shared := loader.IncludeDirs
	if *includeOld != "" {
		loader.IncludeDirs = append(append([]string{}, shared...), *includeOld)
	}
	oldProgram, err := loader.Load(fs.Arg(0))
	if err != nil {
		report(err)
		return exitError
	}
	loader.IncludeDirs = shared
	if *includeNew != "" {
		loader.IncludeDirs = append(append([]string{}, shared...), *includeNew)
	}
	newProgram, err := loader.Load(fs.Arg(1))
	if err != nil {
		report(err)
		return exitError
	}
	if audit.Audit(newProgram, oldProgram, opts) && !*noFatal {
		return exitPolicy
	}
	return exitOK
}
