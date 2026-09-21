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

package php

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// newTempField builds the throwaway t_field the C++ generator constructs
// inline to drive generate_deserialize_field / generate_serialize_field
// on a bare type (e.g. the protocol's field-type byte in inlined mode).
func newTempField(t sema.Type, name string) *sema.Field {
	return sema.NewField(t, name, 0)
}

// generateStruct makes a struct.
func (g *Generator) generateStruct(s *sema.Struct) {
	g.generatePHPStruct(s, false)
}

// generateXception generates a struct definition for a thrift exception.
// Basically the same as a struct but extends the Exception class.
func (g *Generator) generateXception(s *sema.Struct) {
	g.generatePHPStruct(s, true)
}

// generatePHPStruct: structs can be normal or exceptions.
func (g *Generator) generatePHPStruct(s *sema.Struct, isException bool) {
	var out *strings.Builder
	if g.opts.Classmap {
		out = &g.fTypes
	} else {
		out = &strings.Builder{}
		g.generateProgramHeader(out, nil)
	}
	g.generatePHPStructDefinition(out, s, isException, false)
	if !g.opts.Classmap {
		fStructName := g.packageDir + s.Name() + ".php"
		emit.WriteFile(fStructName, out.String())
	}
}

func (g *Generator) generatePHPTypeSpec(out *strings.Builder, t sema.Type) {
	t = sema.TrueType(t)
	out.WriteString(g.indent() + "'type' => " + typeToEnum(t) + ",\n")

	switch {
	case t.IsBaseType():
		// Noop, type is all we need.
	case t.IsStruct() || t.IsXception() || t.IsEnum():
		out.WriteString(g.indent() + "'class' => '" + g.phpNamespace(t.Program()) + t.Name() + "',\n")
	case t.IsMap():
		m := t.(*sema.Map)
		ktype := sema.TrueType(m.KeyType())
		vtype := sema.TrueType(m.ValType())
		out.WriteString(g.indent() + "'ktype' => " + typeToEnum(ktype) + ",\n")
		out.WriteString(g.indent() + "'vtype' => " + typeToEnum(vtype) + ",\n")
		out.WriteString(g.indent() + "'key' => [\n")
		g.indentUp()
		g.generatePHPTypeSpec(out, ktype)
		g.indentDown()
		out.WriteString(g.indent() + "],\n")
		out.WriteString(g.indent() + "'val' => [\n")
		g.indentUp()
		g.generatePHPTypeSpec(out, vtype)
		g.indentDown()
		out.WriteString(g.indent() + "],\n")
	case t.IsList() || t.IsSet():
		var etype sema.Type
		if t.IsList() {
			etype = sema.TrueType(t.(*sema.List).ElemType())
		} else {
			etype = sema.TrueType(t.(*sema.Set).ElemType())
		}
		out.WriteString(g.indent() + "'etype' => " + typeToEnum(etype) + ",\n")
		out.WriteString(g.indent() + "'elem' => [\n")
		g.indentUp()
		g.generatePHPTypeSpec(out, etype)
		g.indentDown()
		out.WriteString(g.indent() + "],\n")
	default:
		emit.Throw("compiler error: no type for php struct spec field")
	}
}

// generatePHPStructSpec generates the struct specification structure,
// which fully qualifies enough type information to generalize
// serialization routines.
func (g *Generator) generatePHPStructSpec(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public static array $tspec = [\n")
	g.indentUp()

	for _, m := range s.Members() {
		t := sema.TrueType(m.Type())
		out.WriteString(g.indent() + itoa(int64(m.Key())) + " => [\n")
		g.indentUp()
		out.WriteString(g.indent() + "'var' => '" + m.Name() + "',\n")
		isRequired := "false"
		if m.Req() == sema.Required {
			isRequired = "true"
		}
		out.WriteString(g.indent() + "'isRequired' => " + isRequired + ",\n")
		g.generatePHPTypeSpec(out, t)
		g.indentDown()
		out.WriteString(g.indent() + "],\n")
	}

	g.indentDown()
	out.WriteString(g.indent() + "];\n")
}

// generateGenericFieldGettersSetters generates necessary accessors and
// mutators for the fields.
func (g *Generator) generateGenericFieldGettersSetters(out *strings.Builder, s *sema.Struct) {
	var getterStream, setterStream strings.Builder

	// build up the bodies of both the getter and setter at once
	for _, field := range s.Members() {
		fieldName := field.Name()
		capName := getCapName(fieldName)

		g.indentUp()
		g.generateReflectionSetters(&setterStream, fieldName, capName)
		g.generateReflectionGetters(&getterStream, fieldName, capName)
		g.indentDown()
	}

	out.WriteString(g.indent() + "\n")
	out.WriteString(getterStream.String())
	out.WriteString(setterStream.String())
	out.WriteString(g.indent() + "\n")
}

// generateReflectionGetters generates a getter for the generated private
// fields.
func (g *Generator) generateReflectionGetters(out *strings.Builder, fieldName, capName string) {
	out.WriteString(g.indent() + "public function " + "get" + capName + "()\n")
	out.WriteString(g.indent() + "{\n")

	g.indentUp()

	out.WriteString(g.indent() + "return $this->" + fieldName + ";\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")
}

// generateReflectionSetters generates a setter for the generated private
// fields.
func (g *Generator) generateReflectionSetters(out *strings.Builder, fieldName, capName string) {
	out.WriteString(g.indent() + "public function set" + capName + "(" + "$" + fieldName + ")\n")
	out.WriteString(g.indent() + "{\n")

	g.indentUp()

	out.WriteString(g.indent() + "$this->" + fieldName + " = $" + fieldName + ";\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")
}

// kExceptionInheritedSlots are the internal C-implementation slots on
// \Exception that PHP forbids subclasses from re-typing ("Type of ...
// must not be defined").
var kExceptionInheritedSlots = map[string]bool{
	"message": true, "code": true, "file": true, "line": true,
}

// generatePHPStructDefinition generates a struct definition for a
// thrift data type. This is nothing in PHP where the objects are all
// just associative arrays (unless of course we decide to start using
// objects for them...).
func (g *Generator) generatePHPStructDefinition(out *strings.Builder, s *sema.Struct, isException, isResult bool) {
	members := s.Members()

	g.phpDoc(out, s)
	out.WriteString("class " + phpNamespaceDeclaration(s))
	if isException {
		out.WriteString(" extends " + "TException")
	} else if g.opts.Oop {
		out.WriteString(" extends " + "TBase")
	}
	if g.opts.JSON {
		out.WriteString(" implements JsonSerializable")
	}
	out.WriteString("\n" + "{\n")
	g.indentUp()

	validate := "false"
	if g.opts.Validate {
		validate = "true"
	}
	out.WriteString(g.indent() + "public static bool $isValidate = " + validate + ";\n\n")

	g.generatePHPStructSpec(out, s)
	out.WriteString("\n")

	for _, m := range members {
		dval := "null"
		t := sema.TrueType(m.Type())
		if m.Value() != nil && !(t.IsStruct() || t.IsXception()) {
			dval = g.renderConstValue(m.Type(), m.Value())
		}
		g.phpDocField(out, m)
		access := "public"
		if g.opts.GettersSetters {
			access = "private"
		}
		fname := m.Name()
		inheritsUntyped := isException && kExceptionInheritedSlots[fname]
		native := ""
		if !inheritsUntyped {
			native = "?" + g.typeToNative(t) + " "
		}
		out.WriteString(g.indent() + access + " " + native + "$" + fname + " = " + dval + ";\n")
	}

	if len(members) > 0 {
		out.WriteString("\n")
	}

	// Generate constructor from array.
	param := ""
	if len(members) > 0 {
		param = "?array $vals = null"
	}
	out.WriteString(g.indent() + "public function __construct(" + param + ")\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	if len(members) > 0 {
		for _, m := range members {
			t := sema.TrueType(m.Type())
			if m.Value() != nil && (t.IsStruct() || t.IsXception()) {
				out.WriteString(g.indent() + "$this->" + m.Name() + " = " +
					g.renderConstValue(t, m.Value()) + ";\n")
			}
		}
		out.WriteString(g.indent() + "if (is_array($vals)) {\n")
		g.indentUp()
		if g.opts.Oop {
			out.WriteString(g.indent() + "parent::__construct(self::$tspec, $vals);\n")
		} else {
			for _, m := range members {
				fname := m.Name()
				// Cast incoming scalar to the declared property type
				// so `new X([...])` tolerates loose-typed user input
				// (e.g. '1' -> true) the way PHP did before
				// declare(strict_types=1) + typed properties became
				// the contract. typeToCast returns "" for
				// struct/container types, leaving them as straight
				// assignments.
				cast := typeToCast(m.Type())
				out.WriteString(g.indent() + "if (isset($vals['" + fname + "'])) {\n")
				g.indentUp()
				out.WriteString(g.indent() + "$this->" + fname + " = " + cast + "$vals['" + fname + "'];\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
			}
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
	g.scopeDown(out)
	out.WriteString("\n")

	out.WriteString(g.indent() + "public function getName(): string\n")
	out.WriteString(g.indent() + "{\n")

	g.indentUp()
	out.WriteString(g.indent() + "return '" + s.Name() + "';\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	if g.opts.GettersSetters {
		g.generateGenericFieldGettersSetters(out, s)
	}
	g.generatePHPStructReader(out, s, isResult)
	out.WriteString("\n")
	g.generatePHPStructWriter(out, s, isResult)
	if g.needsPHPReadValidator(s, isResult) {
		out.WriteString("\n")
		g.generatePHPStructReadValidator(out, s)
	}
	if g.needsPHPWriteValidator(s, isResult) {
		out.WriteString("\n")
		g.generatePHPStructWriteValidator(out, s)
	}
	if g.opts.JSON {
		out.WriteString("\n")
		g.generatePHPStructJSONSerialize(out, s, isResult)
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

// generatePHPStructReader generates the read() method for a struct.
func (g *Generator) generatePHPStructReader(out *strings.Builder, s *sema.Struct, isResult bool) {
	fields := s.Members()

	if g.opts.Inlined {
		// Inline mode reads from a raw TTransport (string buffer), not
		// a TProtocol.
		out.WriteString(g.indent() + "public function read(TTransport $input): int\n")
	} else {
		out.WriteString(g.indent() + "public function read(TProtocol $input): int\n")
	}
	g.scopeUp(out)

	if g.opts.Oop {
		if g.needsPHPReadValidator(s, isResult) {
			out.WriteString(g.indent() + "$tmp = $this->readStruct('" + s.Name() + "', self::$tspec, $input);\n")
			out.WriteString(g.indent() + "$this->validateForRead();\n")
			out.WriteString(g.indent() + "return $tmp;\n")
		} else {
			out.WriteString(g.indent() + "return $this->readStruct('" + s.Name() + "', self::$tspec, $input);\n")
		}
		g.scopeDown(out)
		return
	}

	out.WriteString(g.indent() + "$xfer = 0;\n" + g.indent() + "$fname = null;\n" + g.indent() +
		"$ftype = 0;\n" + g.indent() + "$fid = 0;\n")

	// Declare stack tmp variables.
	if !g.opts.Inlined {
		out.WriteString(g.indent() + "$input->incrementRecursionDepth();\n")
		out.WriteString(g.indent() + "try {\n")
		g.indentUp()
		out.WriteString(g.indent() + "$xfer += $input->readStructBegin($fname);\n")
	}

	// Loop over reading in fields.
	out.WriteString(g.indent() + "while (true) {\n")
	g.indentUp()

	// Read beginning field marker.
	if g.opts.Inlined {
		fftype := newTempField(sema.GlobalI8, "ftype")
		ffid := newTempField(sema.GlobalI16, "fid")
		g.generateDeserializeField(out, fftype, "", false)
		out.WriteString(g.indent() + "if ($ftype == " + "TType::STOP) {\n" + g.indent() + "    break;\n" + g.indent() + "}\n")
		g.generateDeserializeField(out, ffid, "", false)
	} else {
		out.WriteString(g.indent() + "$xfer += $input->readFieldBegin($fname, $ftype, $fid);\n")
		// Check for field STOP marker and break.
		out.WriteString(g.indent() + "if ($ftype == " + "TType::STOP) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "break;\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}

	// Switch statement on the field we are reading.
	out.WriteString(g.indent() + "switch ($fid) {\n")
	g.indentUp()

	// Generate deserialization code for known cases.
	for _, f := range fields {
		out.WriteString(g.indent() + "case " + itoa(int64(f.Key())) + ":\n")
		g.indentUp()
		out.WriteString(g.indent() + "if ($ftype == " + typeToEnum(f.Type()) + ") {\n")
		g.indentUp()
		g.generateDeserializeField(out, f, "this->", false)
		g.indentDown()
		out.WriteString(g.indent() + "} else {\n")
		g.indentUp()
		if g.opts.Inlined {
			out.WriteString(g.indent() + "$xfer += TProtocol::skipBinary($input, $ftype);\n")
		} else {
			out.WriteString(g.indent() + "$xfer += $input->skip($ftype);\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n" + g.indent() + "break;\n")
		g.indentDown()
	}

	// In the default case we skip the field.
	out.WriteString(g.indent() + "default:\n")
	g.indentUp()
	if g.opts.Inlined {
		out.WriteString(g.indent() + "$xfer += " + "TProtocol::skipBinary($input, $ftype);\n")
	} else {
		out.WriteString(g.indent() + "$xfer += $input->skip($ftype);\n")
	}
	out.WriteString(g.indent() + "break;\n")
	g.indentDown()

	g.scopeDown(out)

	if !g.opts.Inlined {
		// Read field end marker.
		out.WriteString(g.indent() + "$xfer += $input->readFieldEnd();\n")
	}

	g.scopeDown(out)

	if !g.opts.Inlined {
		out.WriteString(g.indent() + "$xfer += $input->readStructEnd();\n")
		g.indentDown()
		out.WriteString(g.indent() + "} finally {\n")
		g.indentUp()
		out.WriteString(g.indent() + "$input->decrementRecursionDepth();\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}

	if g.needsPHPReadValidator(s, isResult) {
		out.WriteString(g.indent() + "$this->validateForRead();\n")
	}

	out.WriteString("\n")
	out.WriteString(g.indent() + "return $xfer;\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

// generatePHPStructWriter generates the write() method for a struct.
func (g *Generator) generatePHPStructWriter(out *strings.Builder, s *sema.Struct, isResult bool) {
	name := s.Name()
	fields := s.SortedMembers()

	if g.opts.Inlined {
		out.WriteString(g.indent() + "public function write(string &$output): int\n")
	} else {
		out.WriteString(g.indent() + "public function write(TProtocol $output): int\n")
	}
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	if g.needsPHPWriteValidator(s, isResult) {
		out.WriteString(g.indent() + "$this->validateForWrite();\n")
	}

	if g.opts.Oop {
		out.WriteString(g.indent() + "return $this->writeStruct('" + s.Name() + "', self::$tspec, $output);\n")
		g.scopeDown(out)
		return
	}

	out.WriteString(g.indent() + "$xfer = 0;\n")

	if !g.opts.Inlined {
		out.WriteString(g.indent() + "$output->incrementRecursionDepth();\n")
		out.WriteString(g.indent() + "try {\n")
		g.indentUp()
		out.WriteString(g.indent() + "$xfer += $output->writeStructBegin('" + name + "');\n")
	}

	for _, f := range fields {
		out.WriteString(g.indent() + "if ($this->" + f.Name() + " !== null) {\n")
		g.indentUp()

		t := sema.TrueType(f.Type())
		expect := ""
		if t.IsContainer() {
			expect = "array"
		} else if t.IsStruct() {
			expect = "object"
		}
		if expect != "" {
			out.WriteString(g.indent() + "if (!is_" + expect + "($this->" + f.Name() + ")) {\n")
			g.indentUp()
			out.WriteString(g.indent() + "throw new " + "TProtocolException('Bad type in structure.', " +
				"TProtocolException::INVALID_DATA);\n")
			g.scopeDown(out)
		}

		// Write field header.
		if g.opts.Inlined {
			out.WriteString(g.indent() + "$output .= pack('c', " + typeToEnum(f.Type()) + ");\n" +
				g.indent() + "$output .= pack('n', " + itoa(int64(f.Key())) + ");\n")
		} else {
			out.WriteString(g.indent() + "$xfer += $output->writeFieldBegin(" +
				"'" + f.Name() + "', " + typeToEnum(f.Type()) + ", " + itoa(int64(f.Key())) + ");\n")
		}

		// Write field contents.
		g.generateSerializeField(out, f, "this->")

		// Write field closer.
		if !g.opts.Inlined {
			out.WriteString(g.indent() + "$xfer += $output->writeFieldEnd();\n")
		}

		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}

	if g.opts.Inlined {
		out.WriteString(g.indent() + "$output .= pack('c', " + "TType::STOP);\n")
	} else {
		out.WriteString(g.indent() + "$xfer += $output->writeFieldStop();\n" + g.indent() +
			"$xfer += $output->writeStructEnd();\n")
		g.indentDown()
		out.WriteString(g.indent() + "} finally {\n")
		g.indentUp()
		out.WriteString(g.indent() + "$output->decrementRecursionDepth();\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}

	out.WriteString("\n")
	out.WriteString(g.indent() + "return $xfer;\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generatePHPStructReadValidator(out *strings.Builder, s *sema.Struct) {
	g.generatePHPStructRequiredValidator(out, s, "validateForRead", false)
}

func (g *Generator) generatePHPStructWriteValidator(out *strings.Builder, s *sema.Struct) {
	g.generatePHPStructRequiredValidator(out, s, "validateForWrite", true)
}

func (g *Generator) generatePHPStructRequiredValidator(out *strings.Builder, s *sema.Struct, methodName string, writeMode bool) {
	out.WriteString(g.indent() + "private function " + methodName + "(): void\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	fields := s.Members()
	if len(fields) > 0 {
		for _, field := range fields {
			if field.Req() == sema.Required || (field.Req() == sema.OptInReqOut && writeMode) {
				out.WriteString(g.indent() + "if ($this->" + field.Name() + " === null) {\n")
				g.indentUp()
				out.WriteString(g.indent() + "throw new TProtocolException('Required field " + s.Name() + "." +
					field.Name() + " is unset!');\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
			}
		}
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generatePHPStructJSONSerialize(out *strings.Builder, s *sema.Struct, isResult bool) {
	out.WriteString(g.indent() + "public function jsonSerialize(): mixed\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	if g.needsPHPWriteValidator(s, isResult) {
		out.WriteString(g.indent() + "$this->validateForWrite();\n")
	}

	out.WriteString(g.indent() + "$json = new stdClass();\n")

	fields := s.Members()
	if len(fields) > 0 {
		for _, field := range fields {
			t := field.Type()
			name := field.Name()
			if t.IsMap() {
				keyType := t.(*sema.Map).KeyType()
				if !(keyType.IsBaseType() || keyType.IsEnum()) {
					// JSON object keys must be strings. PHP's
					// json_encode() function will convert any
					// scalar key to strings, but we skip thrift
					// maps with non-scalar keys.
					continue
				}
			}
			out.WriteString(g.indent() + "if ($this->" + name + " !== null) {\n")
			g.indentUp()
			out.WriteString(g.indent() + "$json->" + name + " = ")
			if t.IsMap() {
				out.WriteString("(object)")
			} else {
				out.WriteString(typeToCast(t))
			}
			out.WriteString("$this->" + name + ";\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
	}

	out.WriteString(g.indent() + "return $json;\n")
	g.indentDown()

	out.WriteString(g.indent() + "}\n")
}

func getPHPNumRequiredFields(fields []*sema.Field, writeMode bool) int {
	numReq := 0
	for _, f := range fields {
		if f.Req() == sema.Required || (f.Req() == sema.OptInReqOut && writeMode) {
			numReq++
		}
	}
	return numReq
}

func (g *Generator) needsPHPWriteValidator(s *sema.Struct, isResult bool) bool {
	return g.opts.Validate && !isResult && !s.IsUnion() &&
		getPHPNumRequiredFields(s.Members(), true) > 0
}

func (g *Generator) needsPHPReadValidator(s *sema.Struct, isResult bool) bool {
	return g.opts.Validate && !isResult &&
		getPHPNumRequiredFields(s.Members(), false) > 0
}
