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

package parity

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/apache/thrift/compiler/go/audit"
	"github.com/apache/thrift/compiler/go/sema"
)

// auditCase is one invocation of thrift_audit_test.pl: an old and a new
// file and the -audit-* options.
type auditCase struct {
	name, oldFile, newFile string
	options                []string
}

func auditCases() []auditCase {
	var cases []auditCase
	for i := 1; i <= 34; i++ {
		name := "break" + strconv.Itoa(i)
		cases = append(cases, auditCase{name, "test.thrift", name + ".thrift", nil})
	}
	cases = append(cases, auditCase{"warning", "test.thrift", "warning.thrift", nil})
	optional := []string{"-audit-allow-optional-field-removal"}
	required := []string{"-audit-allow-required-field-to-default"}
	cases = append(cases,
		auditCase{"optional-removal-rejected", "optional_field_old.thrift", "optional_field_removed.thrift", nil},
		auditCase{"optional-removal-allowed", "optional_field_old.thrift", "optional_field_removed.thrift", optional},
		auditCase{"optional-middle-removal-allowed", "optional_field_middle_old.thrift", "optional_field_middle_removed.thrift", optional},
		auditCase{"default-removal-rejected", "default_field_old.thrift", "optional_field_removed.thrift", optional},
		auditCase{"required-removal-rejected", "required_field_old.thrift", "optional_field_removed.thrift", optional},
		auditCase{"required-to-default-rejected", "required_to_default_old.thrift", "required_to_default_new.thrift", nil},
		auditCase{"required-to-default-allowed", "required_to_default_old.thrift", "required_to_default_new.thrift", required},
		auditCase{"default-to-required-rejected", "required_to_default_new.thrift", "required_to_default_old.thrift", required},
		auditCase{"required-to-optional-rejected", "required_to_default_old.thrift", "required_to_optional_new.thrift", required},
		auditCase{"argument-required-to-default-rejected", "required_argument_old.thrift", "required_argument_default.thrift", nil},
		auditCase{"argument-required-to-default-allowed", "required_argument_old.thrift", "required_argument_default.thrift", required},
	)
	return cases
}

// auditLines keeps the audit messages of an output, sorted: the C++
// compiler writes warnings to stdout and failures to stderr, so their
// relative order is not part of the contract.
func auditLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "[Thrift Audit ") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	sort.Strings(lines)
	return lines
}

// TestAuditParity runs every case of test/audit through the C++ compiler's
// -audit and through the Go audit, and compares the exit status and the
// audit messages.
func TestAuditParity(t *testing.T) {
	root := RepoRoot(t)
	thrift := Compiler(t, root)
	dir := filepath.Join(root, "test", "audit")
	for _, c := range auditCases() {
		t.Run(c.name, func(t *testing.T) {
			oldFile := filepath.Join(dir, c.oldFile)
			newFile := filepath.Join(dir, c.newFile)

			args := append([]string{"-audit", oldFile}, c.options...)
			args = append(args, newFile)
			cmd := exec.Command(thrift, args...)
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			cppFailed := false
			if err := cmd.Run(); err != nil {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
					t.Fatalf("C++ compiler: %v\n%s", err, output.String())
				}
				cppFailed = true
			}

			opts := audit.Options{WarnLevel: 1}
			for _, o := range c.options {
				switch o {
				case "-audit-allow-optional-field-removal":
					opts.AllowOptionalFieldRemoval = true
				case "-audit-allow-required-field-to-default":
					opts.AllowRequiredFieldToDefault = true
				}
			}
			loader := &sema.Loader{}
			oldProgram, err := loader.Load(oldFile)
			if err != nil {
				t.Fatal(err)
			}
			newProgram, err := loader.Load(newFile)
			if err != nil {
				t.Fatal(err)
			}
			var goOutput bytes.Buffer
			opts.Stdout, opts.Stderr = &goOutput, &goOutput
			goFailed := audit.Audit(newProgram, oldProgram, opts)

			if cppFailed != goFailed {
				t.Fatalf("failed: cpp=%v go=%v\ncpp:\n%s\ngo:\n%s", cppFailed, goFailed, output.String(), goOutput.String())
			}
			want, got := auditLines(output.String()), auditLines(goOutput.String())
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				t.Fatalf("audit messages differ:\ncpp:\n%s\ngo:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
			}
		})
	}
}
