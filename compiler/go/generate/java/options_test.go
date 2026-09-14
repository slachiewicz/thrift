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

package java

import "testing"

func TestParseOptions(t *testing.T) {
	o, err := ParseOptions("beans,private-members,reuse-objects,java5,option_type,generated_annotations=undated")
	if err != nil {
		t.Fatal(err)
	}
	if !o.BeanStyle || !o.PrivateMembers || !o.ReuseObjects || !o.Java5 || !o.UseOptionType || o.UseJdk8OptionType || !o.UndatedGeneratedAnnotations {
		t.Fatalf("%+v", o)
	}
	if !o.AndroidLegacy {
		t.Fatal("java5 implies android_legacy")
	}
	if o.OutDirBase() != "gen-javabean" {
		t.Fatalf("beans writes to gen-javabean, got %s", o.OutDirBase())
	}
	if o, _ := ParseOptions(""); o.OutDirBase() != "gen-java" {
		t.Fatalf("default output directory: %s", o.OutDirBase())
	}
	if o, _ := ParseOptions("option_type=jdk8"); !o.UseJdk8OptionType {
		t.Fatal("option_type=jdk8")
	}

	for spec, want := range map[string]string{
		"nope":                        "unknown option java:nope",
		"generated_annotations=x":     "unknown option java:generated_annotations=x",
		"option_type=optional":        "option_type must be 'jdk8' or 'thrift'",
		"beans,generated_annotations": "unknown option java:generated_annotations=",
	} {
		_, err := ParseOptions(spec)
		if err == nil || err.Error() != want {
			t.Errorf("%q: err = %v, want %q", spec, err, want)
		}
	}
}

func TestNaming(t *testing.T) {
	cases := map[string]string{"a": "A", "aB": "A_B", "abCdEF": "AB_CD_EF", "already_upper": "ALREADY_UPPER", "": ""}
	for in, want := range cases {
		if got := constantName(in); got != want {
			t.Errorf("constantName(%q) = %q, want %q", in, got, want)
		}
	}
	camel := map[string]string{"get_foo_bar": "GetFooBar", "_leading": "Leading", "trailing_": "Trailing", "___": "", "a__b": "A_b"}
	for in, want := range camel {
		if got := asCamelCase(in, true); got != want {
			t.Errorf("asCamelCase(%q) = %q, want %q", in, got, want)
		}
	}
	ids := map[string]string{"class": "$class", "1abc": "_1abc", "a-b.c": "a_b_c", "ok_name": "ok_name", "": ""}
	for in, want := range ids {
		if got := makeValidJavaIdentifier(in); got != want {
			t.Errorf("makeValidJavaIdentifier(%q) = %q, want %q", in, got, want)
		}
	}
}
