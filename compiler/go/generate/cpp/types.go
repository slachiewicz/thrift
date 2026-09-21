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

package cpp

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

// generateTypedef is t_cpp_generator::generate_typedef. In C++, a typedef
// is just a simple 1-liner.
func (g *Generator) generateTypedef(t *sema.Typedef) {
	g.generateJavaDoc(&g.fTypes, t.HasDoc(), t.Doc())
	g.fTypes.WriteString(g.indent() + "typedef " + g.typeName(t.Type(), true, false) + " " + t.Symbolic() + ";\n\n")
}

// generateEnumConstantList is t_cpp_generator::generate_enum_constant_list.
func (g *Generator) generateEnumConstantList(f *strings.Builder, constants []*sema.EnumValue, prefix, suffix string, includeValues bool) {
	f.WriteString(" {\n")
	g.indentUp()

	first := true
	for _, c := range constants {
		if first {
			first = false
		} else {
			f.WriteString(",\n")
		}
		g.generateJavaDoc(f, c.HasDoc(), c.Doc())
		f.WriteString(g.indent() + prefix + c.Name() + suffix)
		if includeValues {
			f.WriteString(" = " + strconv.FormatInt(int64(c.Value()), 10))
		}
	}

	f.WriteString("\n")
	g.indentDown()
	f.WriteString(g.indent() + "};\n")
}

// generateEnum is t_cpp_generator::generate_enum. In C++, this is
// essentially the same as the Thrift definition itself, using the enum
// keyword in C++.
func (g *Generator) generateEnum(e *sema.Enum) {
	constants := e.Constants()

	enumName := e.Name()
	if !g.opts.PureEnums {
		enumName = "type"
		g.generateJavaDoc(&g.fTypes, e.HasDoc(), e.Doc())
		g.fTypes.WriteString(g.indent() + "struct " + e.Name() + " {\n")
		g.indentUp()
	}
	if g.opts.PureEnums && g.opts.EnumClass {
		g.fTypes.WriteString(g.indent() + "enum class " + enumName)
	} else {
		g.fTypes.WriteString(g.indent() + "enum " + enumName)
	}

	g.generateEnumConstantList(&g.fTypes, constants, "", "", true)

	if !g.opts.PureEnums {
		g.indentDown()
		g.fTypes.WriteString("};\n")
	}

	g.fTypes.WriteString("\n")

	// Generate a character array of enum names for debugging purposes.
	intValuePrefix := ""
	intValueSuffix := ""
	if g.opts.EnumClass {
		intValuePrefix = "static_cast<int>(" + e.Name() + "::"
		intValueSuffix = ")"
	} else if !g.opts.PureEnums {
		intValuePrefix = e.Name() + "::"
	}

	g.fTypesImpl.WriteString(g.indent() + "int _k" + e.Name() + "Values[] =")
	g.generateEnumConstantList(&g.fTypesImpl, constants, intValuePrefix, intValueSuffix, false)

	g.fTypesImpl.WriteString(g.indent() + "const char* _k" + e.Name() + "Names[] =")
	g.generateEnumConstantList(&g.fTypesImpl, constants, "\"", "\"", false)

	g.fTypes.WriteString(g.indent() + "extern const std::map<int, const char*> _" + e.Name() + "_VALUES_TO_NAMES;\n\n")

	g.fTypesImpl.WriteString(g.indent() + "const std::map<int, const char*> _" + e.Name() +
		"_VALUES_TO_NAMES(::apache::thrift::TEnumIterator(" + strconv.Itoa(len(constants)) + ", _k" + e.Name() + "Values" +
		", _k" + e.Name() + "Names), " +
		"::apache::thrift::TEnumIterator(-1, nullptr, nullptr));\n\n")

	g.generateEnumOstreamOperatorDecl(&g.fTypes, e)
	g.generateEnumOstreamOperator(&g.fTypesImpl, e)

	g.generateEnumToStringHelperFunctionDecl(&g.fTypes, e)
	g.generateEnumToStringHelperFunction(&g.fTypesImpl, e)

	// Generate the template printTo specialization for enums when
	// template_streamop is enabled.
	if g.opts.TemplateStreamop {
		g.generateEnumPrintToHelperFunctionDecl(&g.fTypes, e)
		g.generateEnumPrintToHelperFunction(&g.fTypesTcc, e)
	}

	g.hasMembers = true
}

func (g *Generator) enumValArgType(e *sema.Enum) string {
	if g.opts.PureEnums {
		return e.Name()
	}
	return e.Name() + "::type&"
}

func (g *Generator) generateEnumOstreamOperatorDecl(out *strings.Builder, e *sema.Enum) {
	out.WriteString("std::ostream& operator<<(std::ostream& out, const " + g.enumValArgType(e) + " val);\n\n")
}

func (g *Generator) generateEnumOstreamOperator(out *strings.Builder, e *sema.Enum) {
	// If told the consuming application will provide an ostream operator
	// definition, only make a declaration.
	if g.hasCustomOstream(e.Annotations()) {
		return
	}
	out.WriteString("std::ostream& operator<<(std::ostream& out, const " + g.enumValArgType(e) + " val) ")
	g.scopeUp(out)
	g.generateEnumFindBody(out, e, "out << it->second;", "out << static_cast<int>(val);")
	out.WriteString(g.indent() + "return out;\n")
	g.scopeDown(out)
	out.WriteString("\n")
}

func (g *Generator) generateEnumToStringHelperFunctionDecl(out *strings.Builder, e *sema.Enum) {
	out.WriteString("std::string to_string(const " + g.enumValArgType(e) + " val);\n\n")
}

func (g *Generator) generateEnumToStringHelperFunction(out *strings.Builder, e *sema.Enum) {
	if g.hasCustomOstream(e.Annotations()) {
		return
	}
	out.WriteString("std::string to_string(const " + g.enumValArgType(e) + " val) ")
	g.scopeUp(out)
	g.generateEnumFindBody(out, e, "return std::string(it->second);", "return std::to_string(static_cast<int>(val));")
	g.scopeDown(out)
	out.WriteString("\n")
}

func (g *Generator) generateEnumPrintToHelperFunctionDecl(out *strings.Builder, e *sema.Enum) {
	out.WriteString("template <typename OStream_>\n")
	out.WriteString("void printTo(OStream_& out, const " + g.enumValArgType(e) + " val);\n\n")
}

func (g *Generator) generateEnumPrintToHelperFunction(out *strings.Builder, e *sema.Enum) {
	if g.hasCustomOstream(e.Annotations()) {
		return
	}
	out.WriteString("template <typename OStream_>\n")
	out.WriteString("void printTo(OStream_& out, const " + g.enumValArgType(e) + " val) ")
	g.scopeUp(out)
	g.generateEnumFindBody(out, e, "out << it->second;", "out << static_cast<int>(val);")
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateEnumFindBody writes the shared _VALUES_TO_NAMES lookup body that
// operator<<, to_string and printTo all use.
func (g *Generator) generateEnumFindBody(out *strings.Builder, e *sema.Enum, found, notFound string) {
	out.WriteString(g.indent() + "std::map<int, const char*>::const_iterator it = _" + e.Name() + "_VALUES_TO_NAMES.find(")
	if g.opts.EnumClass {
		out.WriteString("static_cast<int>(val));\n")
	} else {
		out.WriteString("val);\n")
	}
	out.WriteString(g.indent() + "if (it != _" + e.Name() + "_VALUES_TO_NAMES.end()) {\n")
	g.indentUp()
	out.WriteString(g.indent() + found + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "} else {\n")
	g.indentUp()
	out.WriteString(g.indent() + notFound + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}
