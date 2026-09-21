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

package delphi

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// docable is anything with a doc comment, the common ground of t_doc:
// Program, Struct, Enum, EnumValue, Const, Field, Function, Service and
// Typedef all satisfy it through their embedded doc-comment state.
type docable interface {
	HasDoc() bool
	Doc() string
}

// xmlEncode is t_delphi_generator::xml_encode.
func xmlEncode(contents string) string {
	str := strings.ReplaceAll(contents, "&", "&amp;")
	str = strings.ReplaceAll(str, "<", "&lt;")
	str = strings.ReplaceAll(str, ">", "&gt;")
	return str
}

// xmlattribEncode is t_delphi_generator::xmlattrib_encode.
func xmlattribEncode(contents string) string {
	str := xmlEncode(contents)
	str = strings.ReplaceAll(str, "\"", "\\\"")
	return str
}

// xmldocEncode is t_delphi_generator::xmldoc_encode.
func xmldocEncode(contents string) string {
	str := xmlEncode(contents)
	str = strings.ReplaceAll(str, "\r\n", "\r")
	str = strings.ReplaceAll(str, "\n", "\r")
	str = strings.ReplaceAll(str, "\r", "</para>\n<para>")
	return str
}

// generateDelphiDocstringComment is t_delphi_generator::generate_delphi_docstring_comment.
func (g *Generator) generateDelphiDocstringComment(out *strings.Builder, contents string) {
	if !g.opts.XMLDoc {
		return
	}
	emit.DocstringComment(out, g.indent(),
		"{$REGION 'XMLDoc'}/// <summary>\n",
		"/// ",
		"<para>"+contents+"</para>",
		"/// </summary>\n{$ENDREGION}\n")
}

// generateDelphiDocField is the t_field* overload of generate_delphi_doc.
func (g *Generator) generateDelphiDocField(out *strings.Builder, field *sema.Field) {
	if !g.opts.XMLDoc {
		return
	}
	if field.Type().IsEnum() {
		combined := xmldocEncode(field.Doc()) + "\n<seealso cref=\"" + xmldocEncode(g.typeName(field.Type(), false, false)) + "\"/>"
		g.generateDelphiDocstringComment(out, combined)
	} else {
		g.generateDelphiDoc(out, field)
	}
}

// generateDelphiDoc is the t_doc* overload of generate_delphi_doc.
func (g *Generator) generateDelphiDoc(out *strings.Builder, tdoc docable) {
	if tdoc.HasDoc() && g.opts.XMLDoc {
		g.generateDelphiDocstringComment(out, xmldocEncode(tdoc.Doc()))
	}
}

// generateDelphiDocFunction is the t_function* overload of generate_delphi_doc.
func (g *Generator) generateDelphiDocFunction(out *strings.Builder, tfunction *sema.Function) {
	if !(tfunction.HasDoc() && g.opts.XMLDoc) {
		return
	}
	var ps strings.Builder
	for _, p := range tfunction.Arglist().Members() {
		ps.WriteString("\n<param name=\"" + xmlattribEncode(p.Name()) + "\">")
		if p.HasDoc() {
			str := strings.ReplaceAll(p.Doc(), "\n", "")
			ps.WriteString(xmldocEncode(str))
		}
		ps.WriteString("</param>")
	}
	emit.DocstringComment(out, g.indent(),
		"{$REGION 'XMLDoc'}",
		"/// ",
		"<summary><para>"+xmldocEncode(tfunction.Doc())+"</para></summary>"+ps.String(),
		"{$ENDREGION}\n")
}

// isDeprecated is t_delphi_generator::is_deprecated.
func isDeprecated(annotations sema.Annotations) bool {
	_, ok := annotations["deprecated"]
	return ok
}

// renderDeprecationAttribute is t_delphi_generator::render_deprecation_attribute.
func renderDeprecationAttribute(annotations sema.Annotations, prefix, postfix string) string {
	vals, ok := annotations["deprecated"]
	if !ok {
		return ""
	}
	result := prefix + "deprecated"
	last := vals[len(vals)-1]
	if len(last) > 0 && last != "1" {
		result += " " + makePascalStringLiteral(last)
	}
	result += postfix
	return result
}
