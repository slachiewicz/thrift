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

package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/apache/thrift/compiler/go/sema"
)

// fixtures returns test/audit, the corpus test/audit/thrift_audit_test.pl
// drives the C++ compiler through.
func fixtures(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "configure.ac")); err == nil {
			return filepath.Join(dir, "test", "audit")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}

// run audits newFile against oldFile and returns the failure flag with
// the audit lines written to both streams.
func run(t *testing.T, oldFile, newFile string, opts Options) (bool, string) {
	t.Helper()
	loader := &sema.Loader{}
	oldProgram, err := loader.Load(oldFile)
	if err != nil {
		t.Fatalf("%s: %v", oldFile, err)
	}
	newProgram, err := loader.Load(newFile)
	if err != nil {
		t.Fatalf("%s: %v", newFile, err)
	}
	var out bytes.Buffer
	opts.WarnLevel = 1
	opts.Stdout, opts.Stderr = &out, &out
	failed := Audit(newProgram, oldProgram, opts)
	return failed, out.String()
}

// The cases of thrift_audit_test.pl: the breaking change each breakN.thrift
// makes to test.thrift, and the name the failure message must carry.
var breakingChanges = []string{
	1:  "base_function3",               // function removed
	2:  "test_struct1",                 // struct field type changed (string to i8)
	3:  "test_struct1",                 // struct field type changed (i8 to string)
	4:  "test_struct1",                 // struct field type changed (i32 to i64)
	5:  "test_struct1",                 // struct field type changed (bool to list<bool>)
	6:  "test_struct2",                 // struct field type changed (list<double> to list<i16>)
	7:  "test_struct6",                 // requiredness removed
	8:  "test_struct5",                 // requiredness added
	9:  "test_struct1",                 // struct field removed
	10: "test_struct2",                 // struct field removed, id 1
	11: "test_struct3",                 // struct field removed, last id
	12: "derived1_function1",           // return type changed (enum1 to enum2)
	13: "derived1_function6",           // return type changed (struct1 to struct2)
	14: "derived1_function4",           // return type changed (string to double)
	15: "derived2_function1",           // return type changed (list<i32> to list<i16>)
	16: "derived2_function5",           // return type changed (map key type)
	17: "derived2_function6",           // return type changed (map value type)
	18: "base_oneway",                  // oneway removed
	19: "base_function1",               // oneway added
	20: "test_enum1",                   // first enum value removed
	21: "test_enum2",                   // last enum value removed
	22: "test_enum1",                   // enum value removed in between
	23: "test_struct4",                 // required struct field added
	24: "derived1",                     // inheritance removed
	25: "derived2",                     // inheritance changed
	26: "base_function1",               // argument type changed
	27: "base_function2_args",          // argument changed (list<enum1> to list<enum3>)
	28: "derived1_function5_args",      // argument type changed (map to list)
	29: "base_function2_args",          // argument type changed (list<string> to string)
	30: "derived1_function6",           // argument changed (struct1 to map)
	31: "base_function2_exception",     // exception removed
	32: "test_exception1",              // exception field type changed
	33: "derived1_function1_exception", // exception type changed
	34: "test_struct3",                 // field added between two existing ids
}

func TestBreakingChanges(t *testing.T) {
	dir := fixtures(t)
	old := filepath.Join(dir, "test.thrift")
	for i := 1; i < len(breakingChanges); i++ {
		name := "break" + strconv.Itoa(i)
		t.Run(name, func(t *testing.T) {
			failed, out := run(t, old, filepath.Join(dir, name+".thrift"), Options{})
			if !failed {
				t.Fatalf("breaking change not detected:\n%s", out)
			}
			if !strings.Contains(out, breakingChanges[i]) {
				t.Fatalf("a failure was detected, but not the expected one (%s):\n%s", breakingChanges[i], out)
			}
		})
	}
}

func TestNonBreakingChanges(t *testing.T) {
	dir := fixtures(t)
	failed, out := run(t, filepath.Join(dir, "test.thrift"), filepath.Join(dir, "warning.thrift"), Options{})
	if failed {
		t.Fatalf("non-breaking changes reported as a failure:\n%s", out)
	}
	if !strings.Contains(out, "[Thrift Audit Warning:") {
		t.Fatalf("no warning for the non-breaking changes:\n%s", out)
	}
}

func TestConfigurableChanges(t *testing.T) {
	dir := fixtures(t)
	cases := []struct {
		name, oldFile, newFile string
		opts                   Options
		failed                 bool
	}{
		{"optional field removal is rejected by default",
			"optional_field_old.thrift", "optional_field_removed.thrift", Options{}, true},
		{"optional field removal is allowed by option",
			"optional_field_old.thrift", "optional_field_removed.thrift", Options{AllowOptionalFieldRemoval: true}, false},
		{"optional field removal from the middle is allowed by option",
			"optional_field_middle_old.thrift", "optional_field_middle_removed.thrift", Options{AllowOptionalFieldRemoval: true}, false},
		{"default field removal remains rejected by optional removal option",
			"default_field_old.thrift", "optional_field_removed.thrift", Options{AllowOptionalFieldRemoval: true}, true},
		{"required field removal remains rejected by optional removal option",
			"required_field_old.thrift", "optional_field_removed.thrift", Options{AllowOptionalFieldRemoval: true}, true},
		{"required to default is rejected by default",
			"required_to_default_old.thrift", "required_to_default_new.thrift", Options{}, true},
		{"required to default is allowed by option",
			"required_to_default_old.thrift", "required_to_default_new.thrift", Options{AllowRequiredFieldToDefault: true}, false},
		{"default to required remains rejected by required-to-default option",
			"required_to_default_new.thrift", "required_to_default_old.thrift", Options{AllowRequiredFieldToDefault: true}, true},
		{"required to optional remains rejected by required-to-default option",
			"required_to_default_old.thrift", "required_to_optional_new.thrift", Options{AllowRequiredFieldToDefault: true}, true},
		{"service argument required to default is rejected by default",
			"required_argument_old.thrift", "required_argument_default.thrift", Options{}, true},
		{"service argument required to default is allowed by option",
			"required_argument_old.thrift", "required_argument_default.thrift", Options{AllowRequiredFieldToDefault: true}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			failed, out := run(t, filepath.Join(dir, c.oldFile), filepath.Join(dir, c.newFile), c.opts)
			if failed != c.failed {
				t.Fatalf("failed = %v, want %v:\n%s", failed, c.failed, out)
			}
		})
	}
}
