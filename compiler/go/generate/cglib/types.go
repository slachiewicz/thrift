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

package cglib

import (
	"strconv"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// Generate writes the C GLib code for the program. It is
// t_generator::generate_program restricted to what t_c_glib_generator
// overrides.
func (g *Generator) Generate() {
	g.initGenerator()

	for _, e := range g.program.Enums() {
		g.generateEnum(e)
	}
	for _, t := range g.program.Typedefs() {
		g.generateTypedef(t)
	}

	objects := g.program.Objects()
	// generate_forward_declaration is a t_generator no-op that
	// t_c_glib_generator does not override.
	for _, o := range objects {
		if o.IsXception() {
			g.generateXception(o)
		} else {
			g.generateStruct(o)
		}
	}

	g.generateConsts(g.program.Consts())

	for _, s := range g.program.Services() {
		g.serviceName = s.Name()
		g.generateService(s)
	}

	g.closeGenerator()
}

// initGenerator prepares for file generation by opening up the necessary
// file output streams.
func (g *Generator) initGenerator() {
	emit.Mkdir(g.outDir())

	programNameU := initialCapsToUnderscores(g.program.Name())
	programNameUC := toUpperCase(programNameU)
	programNameLC := toLowerCase(programNameU)

	// create output files
	g.fTypesName = g.outDir() + g.nspaceLC + programNameLC + "_types.h"
	g.fTypesImplName = g.outDir() + g.nspaceLC + programNameLC + "_types.c"

	// add thrift boilerplate headers
	g.fTypes.WriteString(autogenComment())
	g.fTypesImpl.WriteString(autogenComment())

	// include inclusion guard
	g.fTypes.WriteString("#ifndef " + g.nspaceUC + programNameUC + "_TYPES_H\n" +
		"#define " + g.nspaceUC + programNameUC + "_TYPES_H\n\n")

	// include base types
	g.fTypes.WriteString("/* base includes */\n" +
		"#include <glib-object.h>\n" +
		"#include <thrift/c_glib/thrift_struct.h>\n" +
		"#include <thrift/c_glib/protocol/thrift_protocol.h>\n")

	// include other thrift includes
	includes := g.program.Includes()
	if len(includes) != 0 {
		g.fTypes.WriteString("/* other thrift includes */\n")
		for _, include := range includes {
			includeNspace := include.Namespace("c_glib")
			includeNspacePrefix := ""
			if includeNspace != "" {
				includeNspacePrefix = initialCapsToUnderscores(includeNspace) + "_"
			}
			g.fTypes.WriteString("#include \"" + includeNspacePrefix +
				initialCapsToUnderscores(include.Name()) + "_types.h\"\n")
		}
		g.fTypes.WriteString("\n")
	}

	// include custom headers
	cIncludes := g.program.CIncludes()
	g.fTypes.WriteString("/* custom thrift includes */\n")
	for _, cInclude := range cIncludes {
		if cInclude[0] == '<' {
			g.fTypes.WriteString("#include " + cInclude + "\n")
		} else {
			g.fTypes.WriteString("#include \"" + cInclude + "\"\n")
		}
	}
	g.fTypes.WriteString("\n")

	// include math.h (for "INFINITY") in the implementation file, in case we
	// encounter a struct with a member of type double
	g.fTypesImpl.WriteString("\n#include <math.h>\n")

	// include the types file
	g.fTypesImpl.WriteString("\n#include \"" + g.nspaceLC + programNameU + "_types.h\"\n" +
		"#include <thrift/c_glib/thrift.h>\n\n")

	g.fTypes.WriteString("/* begin types */\n\n")
}

// closeGenerator finishes up generation and closes all file streams.
func (g *Generator) closeGenerator() {
	programNameUC := toUpperCase(initialCapsToUnderscores(g.program.Name()))

	// end the header inclusion guard
	g.fTypes.WriteString("#endif /* " + g.nspaceUC + programNameUC + "_TYPES_H */\n")

	emit.WriteFile(g.fTypesName, g.fTypes.String())
	emit.WriteFile(g.fTypesImplName, g.fTypesImpl.String())
}

// generateTypedef generates a Thrift typedef in C code. For example:
//
// Thrift:
//
//	typedef map<i32,i32> SomeMap
//
// C:
//
//	typedef GHashTable * ThriftSomeMap;
func (g *Generator) generateTypedef(ttypedef *sema.Typedef) {
	g.fTypes.WriteString(g.indent() + "typedef " + g.typeName(ttypedef.Type(), true, false) + " " +
		g.nspace + ttypedef.Symbolic() + ";\n\n")
}

// generateEnum generates a C enumeration. For example:
//
// Thrift:
//
//	enum MyEnum {
//	  ONE = 1,
//	  TWO
//	}
//
// C:
//
//	enum _ThriftMyEnum {
//	  THRIFT_MY_ENUM_ONE = 1,
//	  THRIFT_MY_ENUM_TWO
//	};
//	typedef enum _ThriftMyEnum ThriftMyEnum;
func (g *Generator) generateEnum(tenum *sema.Enum) {
	name := tenum.Name()
	nameUC := toUpperCase(initialCapsToUnderscores(name))

	g.fTypes.WriteString(g.indent() + "enum _" + g.nspace + name + " {\n")
	g.indentUp()

	constants := tenum.Constants()
	first := true

	// output each of the enumeration elements
	for _, c := range constants {
		if first {
			first = false
		} else {
			g.fTypes.WriteString(",\n")
		}

		g.fTypes.WriteString(g.indent() + g.nspaceUC + nameUC + "_" + c.Name())
		g.fTypes.WriteString(" = " + strconv.FormatInt(int64(c.Value()), 10))
	}

	g.indentDown()
	g.fTypes.WriteString("\n};\ntypedef enum _" + g.nspace + name + " " +
		g.nspace + name + ";\n\n")

	g.fTypes.WriteString("/* return the name of the constant */\n")
	g.fTypes.WriteString("const char *\n")
	g.fTypes.WriteString("toString_" + name + "(int value); \n\n")
	g.fTypesImpl.WriteString("/* return the name of the constant */\n")
	g.fTypesImpl.WriteString("const char *\n")
	g.fTypesImpl.WriteString("toString_" + name + "(int value) \n")
	g.fTypesImpl.WriteString("{\n")
	g.fTypesImpl.WriteString("  static __thread char buf[16] = {0};\n")
	g.fTypesImpl.WriteString("  switch(value) {\n")
	done := map[int32]bool{}
	for _, c := range constants {
		v := c.Value()
		// Skipping duplicate value
		if !done[v] {
			done[v] = true
			g.fTypesImpl.WriteString("  case " + g.nspaceUC + nameUC + "_" + c.Name() + ":" +
				"return \"" + g.nspaceUC + nameUC + "_" + c.Name() + "\";\n")
		}
	}
	g.fTypesImpl.WriteString("  default: g_snprintf(buf, 16, \"%d\", value); return buf;\n")
	g.fTypesImpl.WriteString("  }\n")
	g.fTypesImpl.WriteString("}\n\n")
}

// generateConsts generates Thrift constants in C code.
func (g *Generator) generateConsts(consts []*sema.Const) {
	g.fTypes.WriteString("/* constants */\n")
	g.fTypesImpl.WriteString("/* constants */\n")

	for _, c := range consts {
		name := c.Name()
		nameUC := toUpperCase(name)
		nameLC := toLowerCase(name)
		t := c.Type()
		value := c.Value()

		if g.isComplexType(t) {
			g.fTypes.WriteString(g.typeName(t, false, false) + g.indent() + g.nspaceLC + nameLC +
				"_constant();\n")
		}

		g.fTypes.WriteString(g.indent() + "#define " + g.nspaceUC + nameUC + " " +
			g.constantValue(nameLC, t, value) + "\n")

		g.generateConstInitializer(nameLC, t, value, true)
	}

	g.fTypes.WriteString("\n")
	g.fTypesImpl.WriteString("\n")
}
