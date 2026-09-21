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

package py

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

// generateForwardDeclaration writes the "forward declaration" for a Python
// struct: actually the full class definition, so that generateStruct /
// generateXception can add the thrift_spec afterwards. This groups all
// thrift_spec definitions at the end of the file, which enables
// co-recursive structs.
func (g *Generator) generateForwardDeclaration(s *sema.Struct) {
	g.generatePyStruct(s, s.IsXception())
}

// generateStruct is generate_struct.
func (g *Generator) generateStruct(s *sema.Struct) {
	g.generatePyThriftSpec(&g.fTypes, s, false)
}

// generateXception is generate_xception.
func (g *Generator) generateXception(s *sema.Struct) {
	g.generatePyThriftSpec(&g.fTypes, s, true)
}

// generatePyStruct is generate_py_struct.
func (g *Generator) generatePyStruct(s *sema.Struct, isException bool) {
	g.generatePyStructDefinition(&g.fTypes, s, isException)
}

// generatePyThriftSpec generates the thrift_spec for a struct. For
// example:
//
//	all_structs.append(Recursive)
//	Recursive.thrift_spec = (
//	    None,  # 0
//	    (1, TType.LIST, 'Children', (TType.STRUCT, (Recursive, None), False), None, ),  # 1
//	)
func (g *Generator) generatePyThriftSpec(out *strings.Builder, s *sema.Struct, _ bool) {
	sortedMembers := s.SortedMembers()

	out.WriteString(g.indent() + "all_structs.append(" + maybeEscapeIdentifier(s.Name()) + ")\n")

	if len(sortedMembers) == 0 || sortedMembers[0].Key() >= 0 {
		out.WriteString(g.indent() + maybeEscapeIdentifier(s.Name()) + ".thrift_spec = (\n")
		g.indentUp()

		sortedKeysPos := int32(0)
		for _, f := range sortedMembers {
			for ; sortedKeysPos != f.Key(); sortedKeysPos++ {
				out.WriteString(g.indent() + "None,  # " + strconv.FormatInt(int64(sortedKeysPos), 10) + "\n")
			}

			out.WriteString(g.indent() + "(" + strconv.FormatInt(int64(f.Key()), 10) + ", " + g.typeToEnum(f.Type()) +
				", " + "'" + f.Name() + "'" + ", " + g.typeToSpecArgs(f.Type()) + ", " +
				g.renderFieldDefaultValue(f) + ", " + "),  # " + strconv.FormatInt(int64(sortedKeysPos), 10) + "\n")

			sortedKeysPos++
		}

		g.indentDown()
		out.WriteString(g.indent() + ")\n")
	} else {
		out.WriteString(g.indent() + maybeEscapeIdentifier(s.Name()) + ".thrift_spec = ()\n")
	}
}

// generatePyStructDefinition is generate_py_struct_definition: a struct
// definition for a thrift data type.
func (g *Generator) generatePyStructDefinition(out *strings.Builder, s *sema.Struct, isException bool) {
	members := s.Members()
	sortedMembers := s.SortedMembers()

	out.WriteString("\n\nclass " + maybeEscapeIdentifier(s.Name()))
	if isException {
		if g.opts.Dynamic {
			if isImmutable(s) {
				out.WriteString("(" + g.opts.DynbaseClassFrozenExc + ")")
			} else {
				out.WriteString("(" + g.opts.DynbaseClassExc + ")")
			}
		} else {
			out.WriteString("(TException)")
		}
	} else if g.opts.Dynamic {
		if isImmutable(s) {
			out.WriteString("(" + g.opts.DynbaseClassFrozen + ")")
		} else {
			out.WriteString("(" + g.opts.DynbaseClass + ")")
		}
	} else if g.opts.NewStyle {
		out.WriteString("(object)")
	}
	out.WriteString(":\n")
	g.indentUp()
	g.generatePythonDocstringStruct(out, s)
	thriftSpecType := ""
	if g.opts.TypeHints {
		thriftSpecType = ": typing.Any"
	}
	out.WriteString(g.indent() + "thrift_spec" + thriftSpecType + " = None\n")

	if g.opts.TypeHints && isImmutable(s) && len(members) > 0 {
		out.WriteString("\n")
		for _, f := range members {
			out.WriteString(g.indent() + maybeEscapeIdentifier(f.Name()) + g.memberHint(f.Type(), f.Req()) + "\n")
		}
	}

	out.WriteString("\n")

	// Here we generate the structure specification for the fastbinary
	// codec. These specifications have the following structure:
	//   thrift_spec -> tuple of item_spec
	//   item_spec -> None | (tag, type_enum, name, spec_args, default)
	//   tag -> integer
	//   type_enum -> TType.I32 | TType.STRING | TType.STRUCT | ...
	//   name -> string_literal
	//   default -> None  # Handled by __init__
	//   spec_args -> None  # For simple types
	//              | (type_enum, spec_args)  # Value type for list/set
	//              | (type_enum, spec_args, type_enum, spec_args)
	//                # Key and value for map
	//              | (class_name, spec_args_ptr) # For struct/exception
	//   class_name -> identifier  # Basically a pointer to the class
	//   spec_args_ptr -> expression  # just class_name.spec_args

	if g.opts.Slots {
		out.WriteString(g.indent() + "__slots__ = (\n")
		g.indentUp()
		for _, f := range sortedMembers {
			out.WriteString(g.indent() + "'" + f.Name() + "',\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + ")\n\n")
	}

	if len(members) > 0 {
		out.WriteString("\n")
		out.WriteString(g.indent() + "def __init__(self,")

		for _, f := range members {
			out.WriteString(" " + g.declareArgument(f) + ",")
		}
		out.WriteString("):\n")

		g.indentUp()

		for _, f := range members {
			typ := f.Type()
			if !typ.IsBaseType() && !typ.IsEnum() && f.Value() != nil {
				out.WriteString(g.indent() + "if " + maybeEscapeIdentifier(f.Name()) + " is " +
					"self.thrift_spec[" + strconv.FormatInt(int64(f.Key()), 10) + "][4]:\n")
				g.indentUp()
				out.WriteString(g.indent() + maybeEscapeIdentifier(f.Name()) + " = " + g.renderFieldDefaultValue(f) + "\n")
				g.indentDown()
			}

			if isImmutable(s) {
				if g.opts.Enum && typ.IsEnum() {
					out.WriteString(g.indent() + "super(" + maybeEscapeIdentifier(s.Name()) + ", self).__setattr__('" +
						f.Name() + "', " + maybeEscapeIdentifier(f.Name()) +
						" if hasattr(" + maybeEscapeIdentifier(f.Name()) + ", 'value') else " +
						g.typeName(typ) + ".__members__.get(" + maybeEscapeIdentifier(f.Name()) + "))\n")
				} else if g.opts.NewStyle || g.opts.Dynamic {
					out.WriteString(g.indent() + "super(" + maybeEscapeIdentifier(s.Name()) + ", self).__setattr__('" +
						f.Name() + "', " + maybeEscapeIdentifier(f.Name()) + ")\n")
				} else {
					out.WriteString(g.indent() + "self.__dict__['" + f.Name() + "'] = " + maybeEscapeIdentifier(f.Name()) + "\n")
				}
			} else {
				out.WriteString(g.indent() + "self." + maybeEscapeIdentifier(f.Name()) + g.memberHint(f.Type(), f.Req()) +
					" = " + maybeEscapeIdentifier(f.Name()) + "\n")
			}
		}

		g.indentDown()
	}

	if isImmutable(s) {
		out.WriteString("\n")
		out.WriteString(g.indent() + "def __setattr__(self, *args):\n")
		g.indentUp()

		// Fields the user did not provide should stay editable so that the
		// Python Standard Library can edit internal fields of std library
		// base classes; for example Python 3.11's ContextManager edits
		// __traceback__ on exceptions. __slots__ makes this trivial, since
		// the user-provided fields are known; without it a way to know
		// which fields are user-provided would be needed.
		if g.opts.Slots && !g.opts.Dynamic {
			out.WriteString(g.indent() + "if args[0] not in self.__slots__:\n")
			g.indentUp()
			out.WriteString(g.indent() + "super().__setattr__(*args)\n" + g.indent() + "return\n")
			g.indentDown()
		}
		out.WriteString(g.indent() + "raise TypeError(\"can't modify immutable instance\")\n")
		g.indentDown()
		out.WriteString("\n")
		out.WriteString(g.indent() + "def __delattr__(self, *args):\n")
		g.indentUp()

		if g.opts.Slots && !g.opts.Dynamic {
			out.WriteString(g.indent() + "if args[0] not in self.__slots__:\n")
			g.indentUp()
			out.WriteString(g.indent() + "super().__delattr__(*args)\n" + g.indent() + "return\n")
			g.indentDown()
		}
		out.WriteString(g.indent() + "raise TypeError(\"can't modify immutable instance\")\n")
		g.indentDown()
		out.WriteString("\n")

		// Hash all of the members in order, and also hash in the class to
		// avoid collisions for stuff like single-field structures.
		out.WriteString(g.indent() + "def __hash__(self):\n" +
			g.indent() + indentStr + "return hash(self.__class__) ^ hash((")

		for _, f := range members {
			out.WriteString("self." + maybeEscapeIdentifier(f.Name()) + ", ")
		}

		out.WriteString("))\n")
	} else if g.opts.Enum {
		hasEnum := false
		for _, f := range members {
			if f.Type().IsEnum() {
				hasEnum = true
				break
			}
		}

		if hasEnum {
			out.WriteString("\n")
			out.WriteString(g.indent() + "def __setattr__(self, name, value):\n")
			g.indentUp()
			for _, f := range members {
				typ := f.Type()
				if typ.IsEnum() {
					out.WriteString(g.indent() + "if name == \"" + f.Name() + "\":\n" +
						g.indent() + indentStr + "super().__setattr__(name, value if hasattr(value, 'value') or value is None else " +
						g.typeName(typ) + "(value))\n" +
						g.indent() + indentStr + "return\n")
				}
			}
			out.WriteString(g.indent() + "super().__setattr__(name, value)\n\n")
			g.indentDown()
		}
	}

	if !g.opts.Dynamic {
		out.WriteString("\n")
		g.generatePyStructReader(out, s)
		g.generatePyStructWriter(out, s)
	}

	// For exceptions only, generate a __str__ method. When raised
	// exceptions are printed to the console, __repr__ is not used; see
	// Python bug #5882.
	if isException {
		out.WriteString("\n")
		out.WriteString(g.indent() + "def __str__(self):\n" +
			g.indent() + indentStr + "return repr(self)\n")
	}

	if !g.opts.Slots {
		out.WriteString("\n")
		// Printing utilities so that on the command line thrift structs
		// look pretty like dictionaries.
		out.WriteString(g.indent() + "def __repr__(self):\n")
		g.indentUp()
		out.WriteString(g.indent() + "L = ['%s=%r' % (key, value)\n" +
			g.indent() + "     for key, value in self.__dict__.items()]\n" +
			g.indent() + "return '%s(%s)' % (self.__class__.__name__, ', '.join(L))\n" +
			"\n")
		g.indentDown()

		// Equality and inequality methods that compare by value.
		out.WriteString(g.indent() + "def __eq__(self, other):\n")
		g.indentUp()
		out.WriteString(g.indent() + "return isinstance(other, self.__class__) and self.__dict__ == other.__dict__\n")
		g.indentDown()
		out.WriteString("\n")

		out.WriteString(g.indent() + "def __ne__(self, other):\n")
		g.indentUp()
		out.WriteString(g.indent() + "return not (self == other)\n")
		g.indentDown()
	} else if !g.opts.Dynamic {
		out.WriteString("\n")
		// No base class is available to implement __eq__, __repr__ and
		// __ne__, so one is provided that uses __slots__.
		out.WriteString(g.indent() + "def __repr__(self):\n")
		g.indentUp()
		out.WriteString(g.indent() + "L = ['%s=%r' % (key, getattr(self, key))\n" +
			g.indent() + "     for key in self.__slots__]\n" +
			g.indent() + "return '%s(%s)' % (self.__class__.__name__, ', '.join(L))\n" +
			"\n")
		g.indentDown()

		// Equality method that compares each attribute by value and type,
		// walking __slots__.
		out.WriteString(g.indent() + "def __eq__(self, other):\n")
		g.indentUp()
		out.WriteString(g.indent() + "if not isinstance(other, self.__class__):\n" +
			g.indent() + indentStr + "return False\n" +
			g.indent() + "for attr in self.__slots__:\n" +
			g.indent() + indentStr + "my_val = getattr(self, attr)\n" +
			g.indent() + indentStr + "other_val = getattr(other, attr)\n" +
			g.indent() + indentStr + "if my_val != other_val:\n" +
			g.indent() + indentStr + indentStr + "return False\n" +
			g.indent() + "return True\n" +
			"\n")
		g.indentDown()

		out.WriteString(g.indent() + "def __ne__(self, other):\n" +
			g.indent() + indentStr + "return not (self == other)\n")
	}
	g.indentDown()
}

// generatePyStructReader is generate_py_struct_reader.
func (g *Generator) generatePyStructReader(out *strings.Builder, s *sema.Struct) {
	fields := s.Members()

	if isImmutable(s) {
		out.WriteString(g.indent() + "@classmethod\n" + g.indent() + "def read(cls, iprot):\n")
	} else {
		out.WriteString(g.indent() + "def read(self, iprot):\n")
	}
	g.indentUp()

	id := "self"
	if isImmutable(s) {
		id = "cls"
	}

	out.WriteString(g.indent() + "if iprot._fast_decode is not None " +
		"and isinstance(iprot.trans, TTransport.CReadableTransport) " +
		"and " + id + ".thrift_spec is not None:\n")
	g.indentUp()

	if isImmutable(s) {
		out.WriteString(g.indent() + "return iprot._fast_decode(None, iprot, [cls, cls.thrift_spec])\n")
	} else {
		out.WriteString(g.indent() + "iprot._fast_decode(self, iprot, [self.__class__, self.thrift_spec])\n")
		out.WriteString(g.indent() + "return\n")
	}
	g.indentDown()

	out.WriteString(g.indent() + "iprot.increment_recursion_depth()\n")
	out.WriteString(g.indent() + "try:\n")
	g.indentUp()
	out.WriteString(g.indent() + "iprot.readStructBegin()\n")

	if isImmutable(s) {
		for _, f := range fields {
			result := maybeEscapeIdentifier(f.Name()) + " = "
			if f.Value() != nil {
				result += g.renderFieldDefaultValue(f)
			} else {
				result += "None"
			}
			out.WriteString(g.indent() + result + "\n")
		}
	}

	// Loop over reading in fields.
	out.WriteString(g.indent() + "while True:\n")
	g.indentUp()

	// Read beginning field marker.
	out.WriteString(g.indent() + "(fname, ftype, fid) = iprot.readFieldBegin()\n")

	// Check for the field STOP marker and break.
	out.WriteString(g.indent() + "if ftype == TType.STOP:\n")
	g.indentUp()
	out.WriteString(g.indent() + "break\n")
	g.indentDown()

	// Switch statement on the field being read: generate deserialization
	// code for known cases.
	first := true
	for _, f := range fields {
		if first {
			first = false
			out.WriteString(g.indent() + "if ")
		} else {
			out.WriteString(g.indent() + "elif ")
		}
		out.WriteString("fid == " + strconv.FormatInt(int64(f.Key()), 10) + ":\n")
		g.indentUp()
		out.WriteString(g.indent() + "if ftype == " + g.typeToEnum(f.Type()) + ":\n")
		g.indentUp()
		if isImmutable(s) {
			g.generateDeserializeField(out, f, "")
		} else {
			g.generateDeserializeField(out, f, "self.")
		}
		g.indentDown()
		out.WriteString(g.indent() + "else:\n" + g.indent() + indentStr + "iprot.skip(ftype)\n")
		g.indentDown()
	}

	// In the default case, skip the field.
	out.WriteString(g.indent() + "else:\n" + g.indent() + indentStr + "iprot.skip(ftype)\n")

	// Read the field end marker.
	out.WriteString(g.indent() + "iprot.readFieldEnd()\n")

	g.indentDown()

	out.WriteString(g.indent() + "iprot.readStructEnd()\n")

	if isImmutable(s) {
		out.WriteString(g.indent() + "return cls(\n")
		g.indentUp()
		for _, f := range fields {
			out.WriteString(g.indent() + maybeEscapeIdentifier(f.Name()) + "=" + maybeEscapeIdentifier(f.Name()) + ",\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + ")\n")
	}

	g.indentDown()
	out.WriteString(g.indent() + "finally:\n")
	g.indentUp()
	out.WriteString(g.indent() + "iprot.decrement_recursion_depth()\n")
	g.indentDown()

	g.indentDown()
	out.WriteString("\n")
}

// generatePyStructWriter is generate_py_struct_writer.
func (g *Generator) generatePyStructWriter(out *strings.Builder, s *sema.Struct) {
	name := s.Name()
	fields := s.SortedMembers()

	out.WriteString(g.indent() + "def write(self, oprot):\n")
	g.indentUp()
	out.WriteString(g.indent() + "self.validate()\n")
	out.WriteString(g.indent() + "if oprot._fast_encode is not None and self.thrift_spec is not None:\n")
	g.indentUp()

	out.WriteString(g.indent() + "oprot.trans.write(oprot._fast_encode(self, [self.__class__, self.thrift_spec]))\n")
	out.WriteString(g.indent() + "return\n")
	g.indentDown()

	out.WriteString(g.indent() + "oprot.increment_recursion_depth()\n")
	out.WriteString(g.indent() + "try:\n")
	g.indentUp()
	out.WriteString(g.indent() + "oprot.writeStructBegin('" + name + "')\n")

	for _, f := range fields {
		// Write the field header.
		out.WriteString(g.indent() + "if self." + maybeEscapeIdentifier(f.Name()) + " is not None:\n")
		g.indentUp()
		out.WriteString(g.indent() + "oprot.writeFieldBegin(" +
			"'" + f.Name() + "', " + g.typeToEnum(f.Type()) + ", " + strconv.FormatInt(int64(f.Key()), 10) + ")\n")

		// Write the field contents.
		g.generateSerializeField(out, f, "self.")

		// Write the field closer.
		out.WriteString(g.indent() + "oprot.writeFieldEnd()\n")

		g.indentDown()
	}

	// Write the struct map.
	out.WriteString(g.indent() + "oprot.writeFieldStop()\n" + g.indent() + "oprot.writeStructEnd()\n")

	g.indentDown()
	out.WriteString(g.indent() + "finally:\n")
	g.indentUp()
	out.WriteString(g.indent() + "oprot.decrement_recursion_depth()\n")
	g.indentDown()

	out.WriteString("\n")

	g.indentDown()
	g.generatePyStructRequiredValidator(out, s)
}

// generatePyStructRequiredValidator is generate_py_struct_required_validator.
func (g *Generator) generatePyStructRequiredValidator(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "def validate(self):\n")
	g.indentUp()

	fields := s.Members()
	if len(fields) > 0 {
		for _, f := range fields {
			if f.Req() == sema.Required {
				out.WriteString(g.indent() + "if self." + maybeEscapeIdentifier(f.Name()) + " is None:\n")
				out.WriteString(g.indent() + indentStr + "raise TProtocolException(message='Required field " + f.Name() + " is unset!')\n")
			}
		}
	}

	out.WriteString(g.indent() + "return\n")
	g.indentDown()
}
