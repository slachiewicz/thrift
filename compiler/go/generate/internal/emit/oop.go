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

package emit

import (
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

// The helpers t_oop_generator.h gives every object-oriented generator.

// DocTypeName is t_oop_generator::get_doc_type_name: the type's name
// qualified by its program's java namespace. Generators that override it
// pass their own function to ThrowsDoc.
func DocTypeName(t sema.Type) string {
	pkg := ""
	if p := t.Program(); p != nil {
		pkg = p.Namespace("java")
	}
	if pkg == "" {
		return t.Name()
	}
	return pkg + "." + t.Name()
}

// ThrowsDoc is t_oop_generator::generate_throws_doc: one "@throws" line per
// exception the function declares, appended to out, which holds the doc
// written so far, with the exception field's doc stripped of trailing line
// breaks.
func ThrowsDoc(out *strings.Builder, f *sema.Function, docTypeName func(sema.Type) string) {
	for _, x := range f.Xceptions().Members() {
		if out.Len() > 0 {
			out.WriteString("\n")
		}
		out.WriteString("@throws " + docTypeName(x.Type()))
		if x.HasDoc() {
			out.WriteString(" " + strings.TrimRight(x.Doc(), "\r\n"))
		}
	}
}

// XMLEncode is t_oop_generator::xml_encode.
func XMLEncode(contents string) string {
	contents = strings.ReplaceAll(contents, "&", "&amp;")
	contents = strings.ReplaceAll(contents, "<", "&lt;")
	return strings.ReplaceAll(contents, ">", "&gt;")
}
