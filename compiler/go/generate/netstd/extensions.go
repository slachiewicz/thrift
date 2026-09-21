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

package netstd

import (
	"sort"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// collectExtensionsTypesStruct is collect_extensions_types(t_struct*).
func (g *Generator) collectExtensionsTypesStruct(s *sema.Struct) {
	// make private members with public Properties
	for _, m := range s.Members() {
		g.collectExtensionsTypesType(m.Type())
	}
}

// collectExtensionsTypesType is collect_extensions_types(t_type*).
func (g *Generator) collectExtensionsTypesType(t sema.Type) {
	t = trueType(t)
	key := g.typeName(t, true)

	if t.IsStruct() || t.IsXception() {
		if _, ok := g.checkedExtensionTypes[key]; !ok {
			g.checkedExtensionTypes[key] = t // prevent recursion

			// Only recurse into structs defined in the current program. Structs
			// from included programs are processed by those programs' own
			// generators, which generate extension methods for their internal
			// container types. Recursing into them here would duplicate those
			// extension method signatures (CS0121).
			if t.Program() == g.extensionsOwner {
				g.collectExtensionsTypesStruct(t.(*sema.Struct))
			}
		}
		return
	}

	if t.IsContainer() {
		if _, ok := g.collectedExtensionTypes[key]; !ok {
			g.collectedExtensionTypes[key] = t // prevent recursion

			switch {
			case t.IsMap():
				m := t.(*sema.Map)
				g.collectExtensionsTypesType(m.KeyType())
				g.collectExtensionsTypesType(m.ValType())
			case t.IsSet():
				s := t.(*sema.Set)
				g.collectExtensionsTypesType(s.ElemType())
			case t.IsList():
				l := t.(*sema.List)
				g.collectExtensionsTypesType(l.ElemType())
			default:
				emit.Throw("compiler error: unhandled container type %s", t.Name())
			}
		}
		return
	}
}

// collectExtensionsTypesOfProgram is collect_extensions_types_of_program:
// determine which container types the generator of an included program
// collects for itself. Mirrors the collect_extensions_types() calls made
// while a program's structs, exceptions and services are generated, but
// with that program as the recursion owner.
func (g *Generator) collectExtensionsTypesOfProgram(program *sema.Program) map[string]sema.Type {
	savedCollected := g.collectedExtensionTypes
	savedChecked := g.checkedExtensionTypes
	savedOwner := g.extensionsOwner

	g.collectedExtensionTypes = map[string]sema.Type{}
	g.checkedExtensionTypes = map[string]sema.Type{}
	g.extensionsOwner = program

	for _, o := range program.Objects() {
		g.collectExtensionsTypesStruct(o)
	}

	for _, sv := range program.Services() {
		for _, fn := range sv.Functions() {
			g.collectExtensionsTypesStruct(fn.Arglist())
			g.collectExtensionsTypesStruct(fn.Xceptions())
			g.collectExtensionsTypesType(fn.ReturnType())
		}
	}

	result := g.collectedExtensionTypes

	g.extensionsOwner = savedOwner
	g.collectedExtensionTypes = savedCollected
	g.checkedExtensionTypes = savedChecked

	return result
}

// usesTypeOfProgramType is uses_type_of_program(t_type*, t_program*): true
// if the type - or, for a container, any of its element types - is
// declared in the given program.
func usesTypeOfProgramType(t sema.Type, program *sema.Program) bool {
	t = trueType(t)

	if t.IsMap() {
		m := t.(*sema.Map)
		return usesTypeOfProgramType(m.KeyType(), program) || usesTypeOfProgramType(m.ValType(), program)
	}

	if t.IsSet() {
		return usesTypeOfProgramType(t.(*sema.Set).ElemType(), program)
	}

	if t.IsList() {
		return usesTypeOfProgramType(t.(*sema.List).ElemType(), program)
	}

	if t.IsBaseType() {
		return false
	}

	return t.Program() == program
}

// usesTypeOfProgramStruct is uses_type_of_program(t_struct*, t_program*).
func usesTypeOfProgramStruct(s *sema.Struct, program *sema.Program) bool {
	for _, m := range s.Members() {
		if usesTypeOfProgramType(m.Type(), program) {
			return true
		}
	}
	return false
}

// programDependsOn is program_depends_on: true if the code generated for
// "from" names at least one type declared in "program". Such a pair can
// only ever be compiled together, so "from" may leave extension methods
// to "program" instead of generating a second, ambiguous copy of them.
func programDependsOn(from, program *sema.Program) bool {
	for _, o := range from.Objects() {
		if usesTypeOfProgramStruct(o, program) {
			return true
		}
	}

	for _, c := range from.Consts() {
		if usesTypeOfProgramType(c.Type(), program) {
			return true
		}
	}

	for _, sv := range from.Services() {
		if extends := sv.Extends(); extends != nil && extends.Program() == program {
			return true
		}

		for _, fn := range sv.Functions() {
			if usesTypeOfProgramType(fn.ReturnType(), program) ||
				usesTypeOfProgramStruct(fn.Arglist(), program) ||
				usesTypeOfProgramStruct(fn.Xceptions(), program) {
				return true
			}
		}
	}

	return false
}

// collectInheritedExtensionsTypes is collect_inherited_extensions_types:
// gather the container types generated by the included programs we can
// safely leave them to. That requires two things: their generated code
// has to be part of every build that contains ours, and their extension
// class has to land in the same C# namespace - a class in another
// namespace is out of reach of our generated call sites.
func (g *Generator) collectInheritedExtensionsTypes(program *sema.Program, visited map[*sema.Program]bool, result map[string]*sema.Program) {
	for _, include := range program.Includes() {
		if !programDependsOn(program, include) {
			continue // an include we generate no reference to may not be around at all
		}

		if visited[include] {
			continue // diamond-shaped include graph
		}
		visited[include] = true

		if include.Namespace("netstd") == g.namespaceName {
			types := g.collectExtensionsTypesOfProgram(include)

			keys := make([]string, 0, len(types))
			for k := range types {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				result[k] = include
			}
		}

		g.collectInheritedExtensionsTypes(include, visited, result)
	}
}

// removeInheritedExtensionsTypes is remove_inherited_extensions_types: an
// included program generates extension methods for the container types it
// uses itself. Whenever we use one of the same containers, both extension
// classes end up with the very same signature, and since they also share
// the C# namespace every call site becomes ambiguous (CS0121). Leave those
// to the included program - wherever our extension class is in scope,
// theirs is too.
func (g *Generator) removeInheritedExtensionsTypes(types map[string]sema.Type) {
	for k := range types {
		if _, ok := g.inheritedExtensionOwners[k]; ok {
			delete(types, k)
		}
	}
}

// generateExtensionsFile is generate_extensions_file.
func (g *Generator) generateExtensionsFile() {
	extensionTypes := map[string]sema.Type{}
	for k, v := range g.collectedExtensionTypes {
		extensionTypes[k] = v
	}
	g.removeInheritedExtensionsTypes(extensionTypes)

	if len(extensionTypes) == 0 {
		return
	}

	fExtsName := g.namespaceDir + "/" + g.programName + ".Extensions.cs"
	var f strings.Builder
	g.generateExtensions(&f, extensionTypes)
	emit.WriteFile(fExtsName, f.String())
}

// generateExtensions is generate_extensions.
func (g *Generator) generateExtensions(out *strings.Builder, types map[string]sema.Type) {
	if len(types) == 0 {
		return
	}

	g.resetIndent()
	out.WriteString(g.autogenComment() + g.netstdTypeUsings() +
		"using Thrift.Protocol;\n" +
		"\n\n")

	g.pragmasAndDirectives(out)
	g.startNetstdNamespace(out)

	out.WriteString(g.indent() + "public static class " + makeValidCSharpIdentifier(g.programName) + "Extensions\n")
	g.scopeUp(out)

	keys := make([]string, 0, len(types))
	for k := range types {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		t := types[key]

		out.WriteString(g.indent() + "public static bool Equals(this " + key + " instance, object that)\n")
		g.scopeUp(out)
		if g.opts.TargetNetVersion >= 6 {
			out.WriteString(g.indent() + "if (that is not " + key + " other) return false;\n")
		} else {
			out.WriteString(g.indent() + "if (!(that is " + key + " other)) return false;\n")
		}
		out.WriteString(g.indent() + "if (ReferenceEquals(instance, other)) return true;\n")
		out.WriteString("\n")
		out.WriteString(g.indent() + "return TCollections.Equals(instance, other);\n")
		g.scopeDown(out)
		out.WriteString("\n\n")

		out.WriteString(g.indent() + "public static int GetHashCode(this " + key + " instance)\n")
		g.scopeUp(out)
		out.WriteString(g.indent() + "return TCollections.GetHashCode(instance);\n")
		g.scopeDown(out)
		out.WriteString("\n\n")

		if !g.opts.NoDeepcopy {
			suffix := g.nullableFieldSuffixType(t)
			out.WriteString(g.indent() + "public static " + key + suffix + " " + deepCopyMethodName + "(this " + key + suffix + " source)\n")
			g.scopeUp(out)
			out.WriteString(g.indent() + "if (source == null)\n")
			g.indentUp()
			out.WriteString(g.indent() + "return null;\n\n")
			g.indentDown()

			tmpInstance := g.tmp("tmp")
			if g.opts.TargetNetVersion < 5 && t.IsSet() {
				out.WriteString(g.indent() + "var " + tmpInstance + " = new " + key + "();\n")
			} else {
				out.WriteString(g.indent() + "var " + tmpInstance + " = new " + key + "(source.Count);\n")
			}
			if t.IsMap() {
				tmap := t.(*sema.Map)
				needsTypecast := false
				copyKey := g.deepCopyExpression("pair.Key", tmap.KeyType(), true, &needsTypecast, false)
				copyVal := g.deepCopyExpression("pair.Value", tmap.ValType(), true, &needsTypecast, false)
				nullKey := typeCanBeNull(tmap.KeyType())
				nullVal := typeCanBeNull(tmap.ValType())

				out.WriteString(g.indent() + "foreach (var pair in source)\n")
				g.indentUp()
				if g.opts.TargetNetVersion >= 6 {
					out.WriteString(g.indent() + tmpInstance + ".Add(" + copyKey)
					out.WriteString(", " + copyVal)
				} else {
					out.WriteString(g.indent() + tmpInstance + ".Add(")
					if nullKey {
						out.WriteString("(pair.Key != null) ? " + copyKey + " : null")
					} else {
						out.WriteString(copyKey)
					}
					out.WriteString(", ")
					if nullVal {
						out.WriteString("(pair.Value != null) ? " + copyVal + " : null")
					} else {
						out.WriteString(copyVal)
					}
				}
				out.WriteString(");\n")
				g.indentDown()

			} else if t.IsSet() || t.IsList() {
				var copyElm string
				nullElm := false
				if t.IsSet() {
					tset := t.(*sema.Set)
					needsTypecast := false
					copyElm = g.deepCopyExpression("elem", tset.ElemType(), true, &needsTypecast, false)
					nullElm = typeCanBeNull(tset.ElemType())
				} else {
					tlist := t.(*sema.List)
					needsTypecast := false
					copyElm = g.deepCopyExpression("elem", tlist.ElemType(), true, &needsTypecast, false)
					nullElm = typeCanBeNull(tlist.ElemType())
				}

				out.WriteString(g.indent() + "foreach (var elem in source)\n")
				g.indentUp()
				if g.opts.TargetNetVersion >= 6 {
					out.WriteString(g.indent() + tmpInstance + ".Add(" + copyElm)
				} else {
					out.WriteString(g.indent() + tmpInstance + ".Add(")
					if nullElm {
						out.WriteString("(elem != null) ? " + copyElm + " : null")
					} else {
						out.WriteString(copyElm)
					}
				}
				out.WriteString(");\n")
				g.indentDown()
			}

			out.WriteString(g.indent() + "return " + tmpInstance + ";\n")
			g.scopeDown(out)
			out.WriteString("\n\n")
		}
	}

	g.scopeDown(out)
	g.endNetstdNamespace(out)
}

// extensionsClassName is extensions_class_name: the name of the extension
// class holding the container extension methods of a program.
func extensionsClassName(program *sema.Program) string {
	return makeValidCSharpIdentifier(program.Name()) + "Extensions"
}

// extensionsClassOf is extensions_class_of: the extension class that
// carries the DeepCopy() of the given container type. That is ours,
// unless the container was left to an included program (THRIFT-6198).
func (g *Generator) extensionsClassOf(t sema.Type) string {
	if owner, ok := g.inheritedExtensionOwners[g.typeName(t, true)]; ok {
		return extensionsClassName(owner)
	}
	return extensionsClassName(g.program)
}

// getDeepCopyMethodCall is get_deep_copy_method_call. It returns the call
// expression and sets *suffix and *needsTypecast.
func (g *Generator) getDeepCopyMethodCall(t sema.Type, isNotNull bool, needsTypecast *bool, suffix *string) string {
	t = trueType(t)

	// if is_not_null is set, then the surrounding code already explicitly
	// tests against != null
	nullCheck := ""

	*suffix = ""
	*needsTypecast = false
	if t.IsBaseType() {
		switch baseOf(t) {
		case sema.TypeString:
			if t.IsBinary() {
				*suffix = g.nullableSuffix()
				switch {
				case g.opts.TargetNetVersion >= 8:
					if isNotNull {
						nullCheck = "!"
					} else {
						nullCheck = " ?? []"
					}
				case g.opts.TargetNetVersion >= 6:
					if isNotNull {
						nullCheck = "!"
					} else {
						nullCheck = " ?? Array.Empty<byte>()"
					}
				}
				return ".ToArray()" + nullCheck
			}
			if g.opts.TargetNetVersion >= 6 {
				if isNotNull {
					nullCheck = "!"
				} else {
					nullCheck = " ?? string.Empty"
				}
			}
			return nullCheck // simple assignment will do, strings are immutable in C#
		default:
			return "" // simple assignment will do
		}
	} else if t.IsEnum() {
		return "" // simple assignment will do
	} else if g.isUnionEnabled() && t.IsStruct() && t.(*sema.Struct).IsUnion() {
		*needsTypecast = !t.IsContainer()
		*suffix = g.nullableSuffix()
		if g.opts.TargetNetVersion >= 6 {
			if isNotNull {
				nullCheck = "!"
			} else {
				nullCheck = " ?? new " + t.Name() + ".___undefined()"
			}
		}
		return "." + deepCopyMethodName + "()" + nullCheck
	} else {
		*needsTypecast = !t.IsContainer()
		*suffix = g.nullableSuffix()
		if g.opts.TargetNetVersion >= 8 && t.IsContainer() {
			if isNotNull {
				nullCheck = "!"
			} else {
				nullCheck = " ?? []"
			}
		} else if g.opts.TargetNetVersion >= 6 {
			if isNotNull {
				nullCheck = "!"
			} else {
				nullCheck = " ?? new()"
			}
		}
		return "." + deepCopyMethodName + "()" + nullCheck
	}
}

// deepCopyExpression is deep_copy_expression: builds the expression that
// deep-copies "source".
//
// A container's DeepCopy() is an extension method, and the same signature
// can exist in more than one extension class of a C# namespace -
// THRIFT-6198 removes the duplicates it can see, but two programs without
// an include relation between them cannot know of each other (THRIFT-6199).
// Calling through the class that owns the method rather than through
// extension method syntax keeps the call site unambiguous however many of
// those classes happen to be in scope. Structs and unions have an ordinary
// DeepCopy() instance method and are left alone.
//
// nullConditional picks the "source?.DeepCopy()" form for those, which is
// what the code generated for unions needs.
func (g *Generator) deepCopyExpression(source string, t sema.Type, isNotNull bool, needsTypecast *bool, nullConditional bool) string {
	resolved := trueType(t)

	if !resolved.IsContainer() {
		var suffix string
		copyOp := g.getDeepCopyMethodCall(t, isNotNull, needsTypecast, &suffix)
		extra := ""
		if nullConditional {
			extra = suffix
		}
		return source + extra + copyOp
	}

	*needsTypecast = false

	// if is_not_null is set, then the surrounding code already explicitly
	// tests against != null
	nullCheck := ""
	if g.opts.TargetNetVersion >= 8 {
		if isNotNull {
			nullCheck = "!"
		} else {
			nullCheck = " ?? []"
		}
	} else if g.opts.TargetNetVersion >= 6 {
		if isNotNull {
			nullCheck = "!"
		} else {
			nullCheck = " ?? new()"
		}
	}

	return g.extensionsClassOf(resolved) + "." + deepCopyMethodName + "(" + source + ")" + nullCheck
}
