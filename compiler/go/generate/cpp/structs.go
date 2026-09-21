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

// generateCppStruct is t_cpp_generator::generate_cpp_struct: a struct
// definition for a Thrift data type. This is a class with data members
// and read/write() functions, plus a mirroring isset inner class.
func (g *Generator) generateCppStruct(s *sema.Struct, isException bool) {
	g.generateStructDeclaration(&g.fTypes, s, isException, false, true, true, true, true)
	g.generateStructDefinition(&g.fTypesImpl, &g.fTypesImpl, s, true, true, false)

	out := &g.fTypesImpl
	if g.opts.Templates {
		out = &g.fTypesTcc
	}
	g.generateStructReader(out, s, false)
	g.generateStructWriter(out, s, false)

	// Generate forward setter template implementations in the .tcc file.
	if g.opts.ForwardSetter {
		g.generateStructForwardSetterImpls(&g.fTypesTcc, s)
	}

	g.generateStructSwap(&g.fTypesImpl, s)
	if !g.opts.NoDefaultOperators {
		g.generateEqualityOperator(&g.fTypesImpl, s)
	}
	if !g.opts.NoConstructors {
		g.generateCopyConstructor(&g.fTypesImpl, s, isException)
		if g.opts.Moveable {
			g.generateMoveConstructor(&g.fTypesImpl, s, isException)
		}
		g.generateAssignmentOperator(&g.fTypesImpl, s)
		if g.opts.Moveable {
			g.generateMoveAssignmentOperator(&g.fTypesImpl, s)
		}
	}

	if !g.hasCustomOstream(s.Annotations()) {
		// When template_streamop is enabled, the printTo implementation
		// goes to the .tcc file.
		printOut := &g.fTypesImpl
		if g.opts.TemplateStreamop {
			printOut = &g.fTypesTcc
		}
		g.generateStructPrintMethod(printOut, s)
	}

	if isException {
		g.generateExceptionWhatMethod(&g.fTypesImpl, s)
	}

	g.hasMembers = true
}

func (g *Generator) generateEqualityOperator(out *strings.Builder, s *sema.Struct) {
	members := s.Members()
	rhs := "/* rhs */"
	if len(members) > 0 {
		rhs = "rhs"
	}
	out.WriteString(g.indent() + "bool " + s.Name() + "::operator==(const " + s.Name() + " & " + rhs + ") const\n")
	g.scopeUp(out)
	for _, m := range members {
		name := m.Name()
		// Most existing Thrift code does not use isset or
		// optional/required, so "default" fields are treated as required.
		if m.Req() != sema.Optional {
			out.WriteString(g.indent() + "if (!(" + name + " == rhs." + name + "))\n" + g.indent() + "  return false;\n")
		} else {
			out.WriteString(g.indent() + "if (__isset." + name + " != rhs.__isset." + name + ")\n" + g.indent() + "  return false;\n" +
				g.indent() + "else if (__isset." + name + " && !(" + name + " == rhs." + name + "))\n" +
				g.indent() + "  return false;\n")
		}
	}
	out.WriteString(g.indent() + "return true;\n")
	g.scopeDown(out)
	out.WriteString("\n")
}

func (g *Generator) generateDefaultConstructor(out *strings.Builder, s *sema.Struct, isException bool) {
	members := s.Members()
	hasDefaultValue := hasFieldWithDefaultValue(s)

	clsnameCtor := s.Name() + "::" + s.Name() + "()"
	out.WriteString(g.indent() + clsnameCtor)
	if !hasDefaultValue {
		out.WriteString(" noexcept")
	}

	// Start generating the initializer list.
	initCtor := false
	argsIndent := "   "

	// Default-initialize TException, if it is the base type.
	if isException {
		out.WriteString("\n")
		out.WriteString(g.indent() + " : ")
		out.WriteString("TException()")
		initCtor = true
	}

	// Default-initialize all members that should be initialized in the
	// initializer block.
	for _, m := range members {
		t := sema.TrueType(m.Type())
		if t.IsBaseType() || t.IsEnum() || isReference(m) {
			dval := ""
			if cv := m.Value(); cv != nil {
				dval = g.renderConstValue(out, m.Name(), t, cv)
			} else if t.IsEnum() {
				dval = "static_cast<" + g.typeName(t, false, false) + ">(0)"
			} else if !(t.IsString() || isReference(m) || t.IsUUID()) {
				dval = "0"
			}
			if !initCtor {
				initCtor = true
				if hasDefaultValue {
					out.WriteString(" : ")
				} else {
					out.WriteString("\n" + argsIndent + ": ")
					argsIndent += "  "
				}
			} else {
				out.WriteString(",\n" + argsIndent)
			}
			out.WriteString(m.Name() + "(" + dval + ")")
		}
	}

	// Start generating the body.
	out.WriteString(" {\n")
	g.indentUp()
	// TODO(dreiss): When everything else in Thrift is perfect, do more of
	// these in the initializer list.
	for _, m := range members {
		t := sema.TrueType(m.Type())
		if !t.IsBaseType() && !t.IsEnum() && !isReference(m) {
			if cv := m.Value(); cv != nil {
				g.printConstValue(out, m.Name(), t, cv)
			}
		}
	}
	g.scopeDown(out)
}

// maybeMove converts a variable to an rvalue, when move is enabled.
func maybeMove(other string, move bool) string {
	if move {
		return "std::move(" + other + ")"
	}
	return other
}

func (g *Generator) generateCopyConstructor(out *strings.Builder, s *sema.Struct, isException bool) {
	g.generateConstructorHelper(out, s, isException, false)
}

func (g *Generator) generateMoveConstructor(out *strings.Builder, s *sema.Struct, isException bool) {
	g.generateConstructorHelper(out, s, isException, true)
}

func (g *Generator) generateConstructorHelper(out *strings.Builder, s *sema.Struct, isException, isMove bool) {
	tmpName := g.tmp("other")

	out.WriteString(g.indent() + s.Name() + "::" + s.Name())
	if isMove {
		out.WriteString("(" + s.Name() + "&& ")
	} else {
		out.WriteString("(const " + s.Name() + "& ")
	}
	out.WriteString(tmpName + ") ")
	if isMove || isStructStorageNotThrowing(s) {
		out.WriteString("noexcept ")
	}
	if isException {
		out.WriteString(": TException() ")
	}
	out.WriteString("{\n")
	g.indentUp()

	members := s.Members()
	// Eliminate a compiler unused warning.
	if len(members) == 0 {
		out.WriteString(g.indent() + "(void) " + tmpName + ";\n")
	}

	hasNonrequiredFields := false
	for _, f := range members {
		if f.Req() != sema.Required {
			hasNonrequiredFields = true
		}
		out.WriteString(g.indent() + f.Name() + " = " +
			maybeMove(tmpName+"."+f.Name(), isMove && g.isComplexType(f.Type())) + ";\n")
	}

	if hasNonrequiredFields {
		out.WriteString(g.indent() + "__isset = " + maybeMove(tmpName+".__isset", false) + ";\n")
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateAssignmentOperator(out *strings.Builder, s *sema.Struct) {
	g.generateAssignmentHelper(out, s, false)
}

func (g *Generator) generateMoveAssignmentOperator(out *strings.Builder, s *sema.Struct) {
	g.generateAssignmentHelper(out, s, true)
}

func (g *Generator) generateAssignmentHelper(out *strings.Builder, s *sema.Struct, isMove bool) {
	tmpName := g.tmp("other")

	out.WriteString(g.indent() + s.Name() + "& " + s.Name() + "::operator=(")
	if isMove {
		out.WriteString(s.Name() + "&& ")
	} else {
		out.WriteString("const " + s.Name() + "& ")
	}
	out.WriteString(tmpName + ") ")
	if isMove || isStructStorageNotThrowing(s) {
		out.WriteString("noexcept ")
	}
	out.WriteString("{\n")
	g.indentUp()

	members := s.Members()
	if len(members) == 0 {
		out.WriteString(g.indent() + "(void) " + tmpName + ";\n")
	}

	hasNonrequiredFields := false
	for _, f := range members {
		if f.Req() != sema.Required {
			hasNonrequiredFields = true
		}
		out.WriteString(g.indent() + f.Name() + " = " +
			maybeMove(tmpName+"."+f.Name(), isMove && g.isComplexType(f.Type())) + ";\n")
	}
	if hasNonrequiredFields {
		out.WriteString(g.indent() + "__isset = " + maybeMove(tmpName+".__isset", false) + ";\n")
	}

	out.WriteString(g.indent() + "return *this;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

// generateStructDeclaration is t_cpp_generator::generate_struct_declaration:
// writes the struct declaration into the header file.
func (g *Generator) generateStructDeclaration(out *strings.Builder, s *sema.Struct, isException, pointers, read, write, swap, isUserStruct bool) {
	extends := ""
	if isException {
		extends = " : public ::apache::thrift::TException"
	} else if isUserStruct && !g.opts.Templates {
		extends = " : public virtual ::apache::thrift::TBase"
	}

	members := s.Members()

	// Isset struct has boolean fields, but only for non-required fields.
	hasNonrequiredFields := false
	for _, m := range members {
		if m.Req() != sema.Required {
			hasNonrequiredFields = true
		}
	}

	// Write the isset structure declaration outside the class. This
	// makes the generated code amenable to processing by SWIG. Only
	// declare the struct if it gets used in the class.
	if hasNonrequiredFields && (!pointers || read) {
		out.WriteString(g.indent() + "typedef struct _" + s.Name() + "__isset {\n")
		g.indentUp()

		out.WriteString(g.indent() + "_" + s.Name() + "__isset() ")
		first := true
		for _, m := range members {
			if m.Req() == sema.Required {
				continue
			}
			isSet := "false"
			if m.Value() != nil {
				isSet = "true"
			}
			if first {
				first = false
				out.WriteString(": " + m.Name() + "(" + isSet + ")")
			} else {
				out.WriteString(", " + m.Name() + "(" + isSet + ")")
			}
		}
		out.WriteString(" {}\n")

		for _, m := range members {
			if m.Req() != sema.Required {
				out.WriteString(g.indent() + "bool " + m.Name() + " :1;\n")
			}
		}

		g.indentDown()
		out.WriteString(g.indent() + "} _" + s.Name() + "__isset;\n")
	}

	out.WriteString("\n")
	g.generateJavaDoc(out, s.HasDoc(), s.Doc())

	// Open the struct definition.
	out.WriteString(g.indent() + "class " + s.Name() + extends + " {\n" + g.indent() + " public:\n\n")
	g.indentUp()

	if !g.opts.NoConstructors && !pointers {
		noexceptSuffix := ""
		if isStructStorageNotThrowing(s) {
			noexceptSuffix = " noexcept"
		}
		// Copy constructor
		out.WriteString(g.indent() + s.Name() + "(const " + s.Name() + "&)" + noexceptSuffix + ";\n")

		// Move constructor
		if g.opts.Moveable {
			out.WriteString(g.indent() + s.Name() + "(" + s.Name() + "&&) noexcept;\n")
		}

		// Assignment operator
		out.WriteString(g.indent() + s.Name() + "& operator=(const " + s.Name() + "&)" + noexceptSuffix + ";\n")

		// Move assignment operator
		if g.opts.Moveable {
			out.WriteString(g.indent() + s.Name() + "& operator=(" + s.Name() + "&&) noexcept;\n")
		}

		hasDefaultValue := hasFieldWithDefaultValue(s)

		// Default constructor
		clsnameCtor := s.Name() + "()"
		noexceptCtor := " noexcept"
		if hasDefaultValue {
			noexceptCtor = ""
		}
		out.WriteString(g.indent() + clsnameCtor + noexceptCtor + ";\n")
	}

	if !g.opts.NoConstructors && !s.Annotations().Has("final") {
		out.WriteString("\n" + g.indent())
		if !g.opts.Templates {
			out.WriteString("virtual ")
		}
		out.WriteString("~" + s.Name() + "() noexcept;\n")
	}

	// Declare all fields.
	if g.opts.PrivateOptional && !pointers {
		fieldsArePublic := true

		for _, m := range members {
			fieldIsPublic := m.Req() != sema.Optional
			if fieldIsPublic != fieldsArePublic {
				g.indentDown()
				access := " private:"
				if fieldIsPublic {
					access = " public:"
				}
				out.WriteString("\n" + g.indent() + access + "\n")
				g.indentUp()
				fieldsArePublic = fieldIsPublic
			}

			g.generateJavaDocField(out, m)
			out.WriteString(g.indent() + g.declareField(m, g.opts.NoConstructors, false, !read, false) + "\n")
		}

		if !fieldsArePublic {
			g.indentDown()
			out.WriteString("\n" + g.indent() + " public:\n")
			g.indentUp()
		}
	} else {
		// Default behavior: all fields in the public section.
		for _, m := range members {
			g.generateJavaDocField(out, m)
			out.WriteString(g.indent() + g.declareField(m, !pointers && g.opts.NoConstructors, pointers && !m.Type().IsXception(), !read, false) + "\n")
		}
	}

	// Add the __isset data member if needed, using the definition above.
	if hasNonrequiredFields && (!pointers || read) {
		out.WriteString("\n" + g.indent() + "_" + s.Name() + "__isset __isset;\n")
	}

	// Create a setter function for each field.
	for _, m := range members {
		if pointers {
			continue
		}
		if isReference(m) {
			out.WriteString("\n" + g.indent() + "void __set_" + m.Name() + "(::std::shared_ptr<" +
				g.typeName(m.Type(), false, false) + ">")
			out.WriteString(" val);\n")
		} else if g.opts.ForwardSetter && g.isComplexType(m.Type()) {
			// Use a template for perfect forwarding with forward_setter
			// on complex types.
			out.WriteString("\n" + g.indent() + "template <typename T_>\n")
			out.WriteString(g.indent() + "void __set_" + m.Name() + "(T_&& val);\n")
		} else {
			out.WriteString("\n" + g.indent() + "void __set_" + m.Name() + "(" +
				g.typeName(m.Type(), false, true))
			out.WriteString(" val);\n")
		}
	}

	// Generate getter methods when private_optional is enabled.
	if g.opts.PrivateOptional && !pointers {
		for _, m := range members {
			fieldType := g.typeName(m.Type(), false, false)
			if isReference(m) {
				fieldType = "::std::shared_ptr<" + fieldType + ">"
			}
			out.WriteString("\n" + g.indent() + "const " + fieldType + "& __get_" + m.Name() +
				"() const { return " + m.Name() + "; }\n")
		}
	}
	out.WriteString("\n")

	if !pointers {
		if !g.opts.NoDefaultOperators {
			rhs := "/* rhs */"
			if len(members) > 0 {
				rhs = "rhs"
			}
			out.WriteString(g.indent() + "bool operator == (const " + s.Name() + " & " + rhs + ") const;\n")

			out.WriteString(g.indent() + "bool operator != (const " + s.Name() + " &rhs) const {\n" +
				g.indent() + "  return !(*this == rhs);\n" + g.indent() + "}\n\n")

			// Declare the less-than operator. This must be implemented by
			// the application developer, if desired, with a link error
			// otherwise.
			out.WriteString(g.indent() + "bool operator < (const " + s.Name() + " & ) const;\n\n")
		}
	}

	if read {
		if g.opts.Templates {
			out.WriteString(g.indent() + "template <class Protocol_>\n" + g.indent() + "uint32_t read(Protocol_* iprot);\n")
		} else {
			out.WriteString(g.indent() + "uint32_t read(::apache::thrift::protocol::TProtocol* iprot)")
			if !isException && extends != "" {
				out.WriteString(" override")
			}
			out.WriteString(";\n")
		}
	}
	if write {
		if g.opts.Templates {
			out.WriteString(g.indent() + "template <class Protocol_>\n" + g.indent() + "uint32_t write(Protocol_* oprot) const;\n")
		} else {
			out.WriteString(g.indent() + "uint32_t write(::apache::thrift::protocol::TProtocol* oprot) const")
			if !isException && extends != "" {
				out.WriteString(" override")
			}
			out.WriteString(";\n")
		}
	}
	out.WriteString("\n")

	if isUserStruct && !g.hasCustomOstream(s.Annotations()) {
		out.WriteString(g.indent())
		// Template methods cannot be virtual, so skip the virtual keyword
		// when using template_streamop.
		if !g.opts.Templates && !g.opts.TemplateStreamop {
			out.WriteString("virtual ")
		}
		g.generateStructPrintMethodDecl(out, nil)
		out.WriteString(";\n")
	}

	// std::exception::what()
	if isException {
		out.WriteString(g.indent() + "mutable std::string thriftTExceptionMessageHolder_;\n")
		out.WriteString(g.indent())
		g.generateExceptionWhatMethodDecl(out, s, false)
		out.WriteString(";\n")
	}

	// When private_optional is enabled, optional members may be private.
	// The generated namespace-scope swap() needs friend access.
	if swap && g.opts.PrivateOptional {
		out.WriteString(g.indent() + "friend ")
		g.generateStructSwapDecl(out, s)
	}

	// When private_optional is enabled, optional members may be private.
	// The generated namespace-scope operator<< needs friend access.
	if isUserStruct && g.opts.PrivateOptional {
		if !g.opts.TemplateStreamop {
			out.WriteString(g.indent() + "friend ")
		}
		g.generateStructOstreamOperatorDecl(out, s)
	}

	g.indentDown()
	out.WriteString(g.indent() + "};\n\n")

	if swap {
		// Generate a namespace-scope swap() function.
		out.WriteString(g.indent())
		g.generateStructSwapDecl(out, s)
	}

	// When both private_optional and template_streamop are enabled, the
	// friend function template declared inside the class body is
	// sufficient (it is findable via ADL). Emitting a second
	// namespace-scope declaration would place the 'friend' keyword
	// outside a class, which is ill-formed.
	if isUserStruct && !(g.opts.PrivateOptional && g.opts.TemplateStreamop) {
		g.generateStructOstreamOperatorDecl(out, s)
	}
}

func (g *Generator) generateStructDefinition(out, forceCppOut *strings.Builder, s *sema.Struct, setters, isUserStruct, pointers bool) {
	members := s.Members()

	// Destructor
	if !g.opts.NoConstructors && !s.Annotations().Has("final") {
		forceCppOut.WriteString("\n" + g.indent() + s.Name() + "::~" + s.Name() + "() noexcept {\n")
		g.indentUp()
		g.indentDown()
		forceCppOut.WriteString(g.indent() + "}\n\n")
	}

	if !g.opts.NoConstructors && !pointers {
		// force_cpp_out always goes into the .cpp file, never a .tcc file
		// when templates are involved. Since the constructor is not
		// templated, putting it into the (later included) .tcc file would
		// cause ODR violations.
		g.generateDefaultConstructor(forceCppOut, s, false)
	}

	// Create a setter function for each field.
	if setters {
		for _, m := range members {
			// Skip implementation for forwarding setters (they're inline
			// in the header).
			if g.opts.ForwardSetter && !isReference(m) && g.isComplexType(m.Type()) {
				continue
			}

			if isReference(m) {
				out.WriteString("\n" + g.indent() + "void " + s.Name() + "::__set_" + m.Name() +
					"(::std::shared_ptr<" + g.typeName(m.Type(), false, false) + ">")
				out.WriteString(" val) {\n")
			} else {
				out.WriteString("\n" + g.indent() + "void " + s.Name() + "::__set_" + m.Name() +
					"(" + g.typeName(m.Type(), false, true))
				out.WriteString(" val) {\n")
			}
			g.indentUp()
			out.WriteString(g.indent() + "this->" + m.Name() + " = val;\n")
			g.indentDown()

			// Assume all fields are required except optional fields; for
			// optional fields change __isset.name to true.
			if m.Req() == sema.Optional {
				out.WriteString(g.indent() + g.indent() + "__isset." + m.Name() + " = true;\n")
			}
			out.WriteString(g.indent() + "}\n")
		}
	}
	if isUserStruct {
		// When template_streamop is enabled, the operator<< implementation
		// goes to the .tcc file.
		ostreamOpOut := out
		if g.opts.TemplateStreamop {
			ostreamOpOut = &g.fTypesTcc
		}
		g.generateStructOstreamOperator(ostreamOpOut, s)
	}
	out.WriteString("\n")
}

// generateStructForwardSetterImpls generates template setter
// implementations for forward_setter mode, output to the .tcc file.
func (g *Generator) generateStructForwardSetterImpls(out *strings.Builder, s *sema.Struct) {
	if !g.opts.ForwardSetter {
		return
	}

	for _, m := range s.Members() {
		// Only generate implementations for complex types with forward_setter.
		if isReference(m) || !g.isComplexType(m.Type()) {
			continue
		}

		out.WriteString("\n" + g.indent() + "template <typename T_>\n")
		out.WriteString(g.indent() + "void " + s.Name() + "::__set_" + m.Name() + "(T_&& val) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "this->" + m.Name() + " = ::std::forward<T_>(val);\n")

		if m.Req() == sema.Optional {
			out.WriteString(g.indent() + "__isset." + m.Name() + " = true;\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
}

// generateStructReader is t_cpp_generator::generate_struct_reader: a
// helper function to generate a struct reader.
func (g *Generator) generateStructReader(out *strings.Builder, s *sema.Struct, pointers bool) {
	if g.opts.Templates {
		out.WriteString(g.indent() + "template <class Protocol_>\n" + g.indent() + "uint32_t " +
			s.Name() + "::read(Protocol_* iprot) {\n")
	} else {
		out.WriteString(g.indent() + "uint32_t " + s.Name() + "::read(::apache::thrift::protocol::TProtocol* iprot) {\n")
	}
	g.indentUp()

	fields := s.Members()

	// Declare stack tmp variables.
	out.WriteString("\n" +
		g.indent() + "::apache::thrift::protocol::TInputRecursionTracker tracker(*iprot);\n" +
		g.indent() + "uint32_t xfer = 0;\n" +
		g.indent() + "std::string fname;\n" +
		g.indent() + "::apache::thrift::protocol::TType ftype;\n" +
		g.indent() + "int16_t fid;\n" +
		"\n" +
		g.indent() + "xfer += iprot->readStructBegin(fname);\n" +
		"\n" +
		g.indent() + "using ::apache::thrift::protocol::TProtocolException;\n" +
		"\n")

	// Required variables aren't in __isset, so tmp vars are needed to
	// check them.
	for _, f := range fields {
		if f.Req() == sema.Required {
			out.WriteString(g.indent() + "bool isset_" + f.Name() + " = false;\n")
		}
	}
	out.WriteString("\n")

	// Loop over reading fields.
	out.WriteString(g.indent() + "while (true)\n")
	g.scopeUp(out)

	// Read the beginning field marker.
	out.WriteString(g.indent() + "xfer += iprot->readFieldBegin(fname, ftype, fid);\n")

	// Check for the field STOP marker.
	out.WriteString(g.indent() + "if (ftype == ::apache::thrift::protocol::T_STOP) {\n" +
		g.indent() + "  break;\n" + g.indent() + "}\n")

	if len(fields) == 0 {
		out.WriteString(g.indent() + "xfer += iprot->skip(ftype);\n")
	} else {
		// Switch statement on the field being read.
		out.WriteString(g.indent() + "switch (fid)\n")
		g.scopeUp(out)

		for _, f := range fields {
			out.WriteString(g.indent() + "case " + strconv.Itoa(int(f.Key())) + ":\n")
			g.indentUp()
			out.WriteString(g.indent() + "if (ftype == " + typeToEnum(f.Type()) + ") {\n")
			g.indentUp()

			issetPrefix := "isset_"
			if f.Req() != sema.Required {
				issetPrefix = "this->__isset."
			}

			if pointers && !f.Type().IsXception() {
				g.generateDeserializeField(out, f, "(*(this->", "))")
			} else {
				g.generateDeserializeField(out, f, "this->", "")
			}
			out.WriteString(g.indent() + issetPrefix + f.Name() + " = true;\n")
			g.indentDown()
			out.WriteString(g.indent() + "} else {\n" + g.indent() + "  xfer += iprot->skip(ftype);\n" +
				g.indent() + "}\n" + g.indent() + "break;\n")
			g.indentDown()
		}

		// The default case skips the field.
		out.WriteString(g.indent() + "default:\n" + g.indent() + "  xfer += iprot->skip(ftype);\n" +
			g.indent() + "  break;\n")

		g.scopeDown(out)
	}
	// Read the field end marker.
	out.WriteString(g.indent() + "xfer += iprot->readFieldEnd();\n")

	g.scopeDown(out)

	out.WriteString("\n" + g.indent() + "xfer += iprot->readStructEnd();\n")

	// Throw if any required fields are missing. This happens after
	// reading the struct end, so there might possibly be a chance of
	// continuing.
	out.WriteString("\n")
	for _, f := range fields {
		if f.Req() == sema.Required {
			out.WriteString(g.indent() + "if (!isset_" + f.Name() + ")\n" + g.indent() +
				"  throw TProtocolException(TProtocolException::INVALID_DATA);\n")
		}
	}

	out.WriteString(g.indent() + "return xfer;\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateStructWriter is t_cpp_generator::generate_struct_writer.
func (g *Generator) generateStructWriter(out *strings.Builder, s *sema.Struct, pointers bool) {
	name := s.Name()
	fields := s.SortedMembers()

	if g.opts.Templates {
		out.WriteString(g.indent() + "template <class Protocol_>\n" + g.indent() + "uint32_t " +
			s.Name() + "::write(Protocol_* oprot) const {\n")
	} else {
		out.WriteString(g.indent() + "uint32_t " + s.Name() + "::write(::apache::thrift::protocol::TProtocol* oprot) const {\n")
	}
	g.indentUp()

	out.WriteString(g.indent() + "uint32_t xfer = 0;\n")

	out.WriteString(g.indent() + "::apache::thrift::protocol::TOutputRecursionTracker tracker(*oprot);\n")
	out.WriteString(g.indent() + "xfer += oprot->writeStructBegin(\"" + name + "\");\n")

	for _, f := range fields {
		checkIfSet := f.Req() == sema.Optional || f.Type().IsXception()
		if checkIfSet {
			out.WriteString("\n" + g.indent() + "if (this->__isset." + f.Name() + ") {\n")
			g.indentUp()
		} else {
			out.WriteString("\n")
		}

		// Write the field header.
		out.WriteString(g.indent() + "xfer += oprot->writeFieldBegin(\"" + f.Name() + "\", " +
			typeToEnum(f.Type()) + ", " + strconv.Itoa(int(f.Key())) + ");\n")
		// Write the field contents.
		if pointers && !f.Type().IsXception() {
			g.generateSerializeField(out, f, "(*(this->", "))")
		} else {
			g.generateSerializeField(out, f, "this->", "")
		}
		// Write the field closer.
		out.WriteString(g.indent() + "xfer += oprot->writeFieldEnd();\n")
		if checkIfSet {
			g.indentDown()
			out.WriteString(g.indent() + "}")
		}
	}

	out.WriteString("\n")

	// Write the struct map.
	out.WriteString(g.indent() + "xfer += oprot->writeFieldStop();\n" + g.indent() +
		"xfer += oprot->writeStructEnd();\n" + g.indent() + "return xfer;\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateStructResultWriter is t_cpp_generator::generate_struct_result_writer:
// a struct writer for a function's result, which can have only one field
// set, and does a conditional if/else lookup into __isset.
func (g *Generator) generateStructResultWriter(out *strings.Builder, s *sema.Struct, pointers bool) {
	name := s.Name()
	fields := s.SortedMembers()

	if g.opts.Templates {
		out.WriteString(g.indent() + "template <class Protocol_>\n" + g.indent() + "uint32_t " +
			s.Name() + "::write(Protocol_* oprot) const {\n")
	} else {
		out.WriteString(g.indent() + "uint32_t " + s.Name() + "::write(::apache::thrift::protocol::TProtocol* oprot) const {\n")
	}
	g.indentUp()

	out.WriteString("\n" + g.indent() + "uint32_t xfer = 0;\n\n")

	out.WriteString(g.indent() + "xfer += oprot->writeStructBegin(\"" + name + "\");\n")

	first := true
	for _, f := range fields {
		if first {
			first = false
			out.WriteString("\n" + g.indent() + "if ")
		} else {
			out.WriteString(" else if ")
		}

		out.WriteString("(this->__isset." + f.Name() + ") {\n")

		g.indentUp()

		out.WriteString(g.indent() + "xfer += oprot->writeFieldBegin(\"" + f.Name() + "\", " +
			typeToEnum(f.Type()) + ", " + strconv.Itoa(int(f.Key())) + ");\n")
		if pointers {
			g.generateSerializeField(out, f, "(*(this->", "))")
		} else {
			g.generateSerializeField(out, f, "this->", "")
		}
		out.WriteString(g.indent() + "xfer += oprot->writeFieldEnd();\n")

		g.indentDown()
		out.WriteString(g.indent() + "}")
	}

	out.WriteString("\n" + g.indent() + "xfer += oprot->writeFieldStop();\n" + g.indent() +
		"xfer += oprot->writeStructEnd();\n" + g.indent() + "return xfer;\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) generateStructSwap(out *strings.Builder, s *sema.Struct) {
	name := s.Name()
	if name == "a" || name == "b" {
		out.WriteString(g.indent() + "void swap(" + name + " &a1, " + name + " &a2) noexcept {\n")
	} else {
		out.WriteString(g.indent() + "void swap(" + name + " &a, " + name + " &b) noexcept {\n")
	}

	g.indentUp()

	// Argument-dependent name lookup finds the right swap() to use based
	// on the argument types, falling back to ::std::swap() otherwise.
	out.WriteString(g.indent() + "using ::std::swap;\n")

	hasNonrequiredFields := false
	fields := s.Members()
	for _, f := range fields {
		if f.Req() != sema.Required {
			hasNonrequiredFields = true
		}
		if name == "a" || name == "b" {
			out.WriteString(g.indent() + "swap(a1." + f.Name() + ", a2." + f.Name() + ");\n")
		} else {
			out.WriteString(g.indent() + "swap(a." + f.Name() + ", b." + f.Name() + ");\n")
		}
	}

	if hasNonrequiredFields {
		if name == "a" || name == "b" {
			out.WriteString(g.indent() + "swap(a1.__isset, a2.__isset);\n")
		} else {
			out.WriteString(g.indent() + "swap(a.__isset, b.__isset);\n")
		}
	}

	// Handle empty structs.
	if len(fields) == 0 {
		if name == "a" || name == "b" {
			out.WriteString(g.indent() + "(void) a1;\n")
			out.WriteString(g.indent() + "(void) a2;\n")
		} else {
			out.WriteString(g.indent() + "(void) a;\n")
			out.WriteString(g.indent() + "(void) b;\n")
		}
	}

	g.scopeDown(out)
	out.WriteString("\n")
}

func (g *Generator) generateStructSwapDecl(out *strings.Builder, s *sema.Struct) {
	name := s.Name()
	if name == "a" || name == "b" {
		out.WriteString("void swap(" + name + " &a1, " + name + " &a2) noexcept;")
	} else {
		out.WriteString("void swap(" + name + " &a, " + name + " &b) noexcept;")
	}
	out.WriteString("\n\n")
}

func (g *Generator) generateStructOstreamOperatorDecl(out *strings.Builder, s *sema.Struct) {
	if g.opts.TemplateStreamop {
		out.WriteString("template <typename OStream_>\n")
		if g.opts.PrivateOptional {
			out.WriteString(g.indent() + "friend ")
		}
		out.WriteString("OStream_& operator<<(OStream_& out, const " + s.Name() + "& obj);\n")
	} else {
		out.WriteString("std::ostream& operator<<(std::ostream& out, const " + s.Name() + "& obj);\n")
	}
	out.WriteString("\n")
}

func (g *Generator) generateStructOstreamOperator(out *strings.Builder, s *sema.Struct) {
	if g.hasCustomOstream(s.Annotations()) {
		return
	}
	// Thrift defines this behavior.
	if g.opts.TemplateStreamop {
		out.WriteString("template <typename OStream_>\n")
		out.WriteString("OStream_& operator<<(OStream_& out, const " + s.Name() + "& obj)\n")
	} else {
		out.WriteString("std::ostream& operator<<(std::ostream& out, const " + s.Name() + "& obj)\n")
	}
	g.scopeUp(out)
	out.WriteString(g.indent() + "obj.printTo(out);\n" + g.indent() + "return out;\n")
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructPrintMethodDecl is t_cpp_generator::generate_struct_print_method_decl.
// s is nil for the declaration written inside the class body.
func (g *Generator) generateStructPrintMethodDecl(out *strings.Builder, s *sema.Struct) {
	if g.opts.TemplateStreamop {
		// For the template version, the method itself is templated.
		out.WriteString("template <typename OStream_>\n" + g.indent() + "void ")
		if s != nil {
			out.WriteString(s.Name() + "::")
		}
		out.WriteString("printTo(OStream_& out) const")
	} else {
		out.WriteString("void ")
		if s != nil {
			out.WriteString(s.Name() + "::")
		}
		out.WriteString("printTo(std::ostream& out) const")
	}
}

func (g *Generator) generateExceptionWhatMethodDecl(out *strings.Builder, s *sema.Struct, external bool) {
	out.WriteString("const char* ")
	if external {
		out.WriteString(s.Name() + "::")
	}
	out.WriteString("what() const noexcept")
	if !external {
		out.WriteString(" override")
	}
}

// ---- operator<< field rendering ----

func writeRequiredFieldValue(out *strings.Builder, name string, usePrintto bool) {
	if usePrintto {
		// For template_streamop, use printTo for direct streaming without
		// a temporary string: out << "x=", printTo(out, x)
		out.WriteString(", printTo(out, " + name + ")")
		return
	}
	// For std::ostream, use to_string (backward compatible).
	out.WriteString(" << to_string(" + name + ")")
}

func writeOptionalFieldValue(out *strings.Builder, name string, usePrintto bool) {
	out.WriteString("; (__isset." + name + " ? ")
	if usePrintto {
		// printTo() returns void. Both ternary branches must have the
		// same type, so the false branch is cast to void as well. The
		// non-printTo path does not need the cast because both of its
		// branches return the same stream reference.
		out.WriteString("printTo(out, " + name + ")")
		out.WriteString(" : (void)(out << \"<null>\"))")
	} else {
		// Both branches return std::ostream&, so no cast is needed.
		out.WriteString("(out << to_string(" + name + "))")
		out.WriteString(" : (out << \"<null>\"))")
	}
}

func writeFieldValue(out *strings.Builder, f *sema.Field, usePrintto bool) {
	if f.Req() == sema.Optional {
		writeOptionalFieldValue(out, f.Name(), usePrintto)
	} else {
		writeRequiredFieldValue(out, f.Name(), usePrintto)
	}
}

func writeFieldName(out *strings.Builder, f *sema.Field) {
	out.WriteString(`"` + f.Name() + `="`)
}

func writeOstreamField(out *strings.Builder, f *sema.Field, usePrintto bool) {
	writeFieldName(out, f)
	writeFieldValue(out, f, usePrintto)
}

func writeOstreamFields(out *strings.Builder, fields []*sema.Field, indent string, usePrintto bool) {
	for i, f := range fields {
		out.WriteString(indent + "out << ")
		if i != 0 {
			out.WriteString(`", " << `)
		}
		writeOstreamField(out, f, usePrintto)
		out.WriteString(";\n")
	}
}

// generateStructPrintMethod is t_cpp_generator::generate_struct_print_method:
// generates operator<<.
func (g *Generator) generateStructPrintMethod(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent())
	g.generateStructPrintMethodDecl(out, s)
	out.WriteString(" {\n")

	g.indentUp()

	usePrintto := g.opts.TemplateStreamop
	if usePrintto {
		// For template_streamop, use printTo for direct streaming (better
		// performance).
		out.WriteString(g.indent() + "using ::apache::thrift::printTo;\n")
	}
	// Always include to_string as well, for compatibility.
	out.WriteString(g.indent() + "using ::apache::thrift::to_string;\n")

	out.WriteString(g.indent() + "out << \"" + s.Name() + "(\";\n")
	writeOstreamFields(out, s.Members(), g.indent(), usePrintto)
	out.WriteString(g.indent() + "out << \")\";\n")

	g.indentDown()
	out.WriteString("}\n\n")
}

// generateExceptionWhatMethod generates the what() method for exceptions.
func (g *Generator) generateExceptionWhatMethod(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent())
	g.generateExceptionWhatMethodDecl(out, s, true)
	out.WriteString(" {\n")

	g.indentUp()
	out.WriteString(g.indent() + "try {\n")

	g.indentUp()
	out.WriteString(g.indent() + "std::stringstream ss;\n")
	out.WriteString(g.indent() + "ss << \"TException - service has thrown: \" << *this;\n")
	out.WriteString(g.indent() + "this->thriftTExceptionMessageHolder_ = ss.str();\n")
	out.WriteString(g.indent() + "return this->thriftTExceptionMessageHolder_.c_str();\n")
	g.indentDown()

	out.WriteString(g.indent() + "} catch (const std::exception&) {\n")

	g.indentUp()
	out.WriteString(g.indent() + "return \"TException - service has thrown: " + s.Name() + "\";\n")
	g.indentDown()

	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString("}\n\n")
}
