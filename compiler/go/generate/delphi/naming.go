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

	"github.com/apache/thrift/compiler/go/sema"
)

// delphiKeywords is DELPHI_KEYWORDS: reserved words and predefined type
// names (lowercase).
var delphiKeywords = newKeywordSet(
	// keywords
	"and", "array", "as", "asm", "at", "automated", "begin", "case", "class", "const", "constructor",
	"destructor", "dispinterface", "div", "do", "downto", "else", "end", "except", "exports", "file",
	"finalization", "finally", "for", "function", "goto", "if", "implementation", "in", "inherited",
	"initialization", "inline", "interface", "is", "label", "library", "mod", "nil", "not", "object",
	"of", "on", "or", "out", "packed", "private", "procedure", "program", "property", "protected",
	"public", "published", "raise", "record", "repeat", "resourcestring", "set", "shl", "shr", "string",
	"then", "threadvar", "to", "try", "type", "unit", "until", "uses", "var", "while", "with", "xor",
	// predefined types (lowercase!)
	"ansistring", "boolean", "double", "int64", "integer", "shortint", "smallint", "string", "unicodestring",
)

// delphiReservedNames is DELPHI_RESERVED_NAMES (lowercase).
var delphiReservedNames = newKeywordSet(
	"result", "system", "sysutils", "types", "texception", "tbytes", "tclass", "thrift", "tinterfacedobject",
	"tobject", "ttask",
)

// delphiReservedMethod is DELPHI_RESERVED_METHOD (lowercase).
var delphiReservedMethod = newKeywordSet(
	"afterconstruction", "beforedestruction", "classinfo", "classname", "classnameis", "classparent",
	"classtype", "cleanupinstance", "create", "defaulthandler", "destroy", "dispatch", "equal", "equals",
	"fieldaddress", "free", "freeinstance", "gethashcode", "getinterface", "getinterfaceentry",
	"getinterfacetable", "inheritsfrom", "initinstance", "instancesize", "methodaddress", "methodname",
	"newinstance", "read", "safecallexception", "tostring", "unitname", "write",
)

// delphiReservedMethodException is DELPHI_RESERVED_METHOD_EXCEPTION (lowercase).
var delphiReservedMethodException = newKeywordSet(
	"setinnerexception", "setstackinfo", "getstacktrace", "raisingexception", "createfmt", "createres",
	"createresfmt", "createhelp", "createfmthelp", "createreshelp", "createresfmthelp", "getbaseexception",
	"baseexception", "helpcontext", "innerexception", "exceptiondata", "message", "stacktrace", "stackinfo",
	"getexceptionstackinfoproc", "getstackinfostringproc", "cleanupstackinfoproc", "raiseouterexception",
	"throwouterexception",
)

func newKeywordSet(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// findKeyword is find_keyword: a name ending in one or more trailing
// underscores is also checked with those underscores stripped.
func findKeyword(keywords map[string]bool, name string) bool {
	if len(name) <= 0 {
		return false
	}
	nlast := strings.LastIndexByte(name, '_')
	if nlast >= 1 && nlast == len(name)-1 {
		return findKeyword(keywords, name[:nlast])
	}
	return keywords[name]
}

// normalizeNameSimple is normalize_name(name) with every optional
// argument at its default (false).
func (g *Generator) normalizeNameSimple(name string) string {
	return g.normalizeName(name, false, false, false)
}

// normalizeName is t_delphi_generator::normalize_name.
func (g *Generator) normalizeName(name string, bMethod, bExceptionMethod, bForceUnderscore bool) string {
	tmp := lowercaseASCII(name)

	bReserved := false
	bKeyword := false

	switch {
	case findKeyword(delphiKeywords, tmp):
		bKeyword = true
	case findKeyword(delphiReservedNames, tmp):
		bReserved = true
	case bMethod && findKeyword(delphiReservedMethod, tmp):
		bReserved = true
	case bExceptionMethod && findKeyword(delphiReservedMethodException, tmp):
		bReserved = true
	}

	if !(bReserved || bKeyword) {
		return name
	}

	if !bKeyword || g.opts.OldNames || bForceUnderscore {
		return name + "_"
	}
	return "&" + name
}

// addDelphiUsesList is add_delphi_uses_list.
func (g *Generator) addDelphiUsesList(unitname string) {
	for _, u := range g.usesList {
		if u == unitname {
			return
		}
	}
	g.usesList = append(g.usesList, unitname)
}

// normalizeClsnm is t_delphi_generator::normalize_clsnm with
// b_no_check_keyword defaulted to false.
func (g *Generator) normalizeClsnm(clsnm, prefix string) string {
	return g.normalizeClsnmFull(clsnm, prefix, false)
}

// normalizeClsnmFull is the full three-argument t_delphi_generator::normalize_clsnm.
func (g *Generator) normalizeClsnmFull(clsnm, prefix string, noCheckKeyword bool) string {
	if len(clsnm) > 0 {
		clsnm = strings.ToUpper(clsnm[:1]) + clsnm[1:]
	}
	if noCheckKeyword {
		return prefix + clsnm
	}
	return g.normalizeNameSimple(prefix + clsnm)
}

// propName is the (name, is_xception, prefix) overload of
// t_delphi_generator::prop_name.
func (g *Generator) propName(name string, isXception bool, prefix string) string {
	ret := name
	if len(ret) > 0 {
		ret = strings.ToUpper(ret[:1]) + ret[1:]
	}
	return g.normalizeName(prefix+ret, true, isXception, false)
}

// propNameF is the (t_field*, is_xception, prefix) overload of prop_name.
func (g *Generator) propNameF(f *sema.Field, isXception bool, prefix string) string {
	return g.propName(f.Name(), isXception, prefix)
}

// constructorParamName is t_delphi_generator::constructor_param_name.
func (g *Generator) constructorParamName(name string) string {
	ret := name
	if len(ret) > 0 {
		ret = strings.ToUpper(ret[:1]) + ret[1:]
	}
	return g.normalizeName("a"+ret, false, false, false)
}

// makeValidDelphiIdentifier is t_delphi_generator::make_valid_delphi_identifier.
func makeValidDelphiIdentifier(fromName string) string {
	if fromName == "" {
		return fromName
	}
	b := []byte(fromName)
	if b[0] >= '0' && b[0] <= '9' {
		b = append([]byte{'_'}, b...)
	}
	for i, c := range b {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			b[i] = '_'
		}
	}
	return string(b)
}

// makePascalStringLiteral is t_delphi_generator::make_pascal_string_literal.
// It walks the value as signed bytes, as the C++ `signed char const c`
// loop does, so bytes >= 0x80 (UTF-8 continuation bytes) are left alone
// rather than treated as control characters.
func makePascalStringLiteral(value string) string {
	if len(value) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteByte('\'')
	for i := 0; i < len(value); i++ {
		c := int8(value[i])
		switch {
		case c >= 0 && c < 32:
			sb.WriteByte('#')
			sb.WriteString(itoa64(int64(c)))
		case value[i] == '\'':
			sb.WriteString("''")
		default:
			sb.WriteByte(value[i])
		}
	}
	sb.WriteByte('\'')
	return sb.String()
}

// delphiEscapeString is get_escaped_string as the Delphi generator's
// constructor configures escape_: only "'" is escaped, to "”"; every
// other byte, including control characters, passes through unchanged.
func delphiEscapeString(in string) string {
	if !strings.ContainsRune(in, '\'') {
		return in
	}
	var sb strings.Builder
	for i := 0; i < len(in); i++ {
		if in[i] == '\'' {
			sb.WriteString("''")
		} else {
			sb.WriteByte(in[i])
		}
	}
	return sb.String()
}
