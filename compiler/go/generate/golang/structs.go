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

package golang

import (
	"sort"
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

func (g *Generator) generateStruct(s *sema.Struct)   { g.generateGoStruct(s, false) }
func (g *Generator) generateXception(s *sema.Struct) { g.generateGoStruct(s, true) }

func (g *Generator) generateGoStruct(s *sema.Struct, isException bool) {
	g.beginTypesDeclaration()
	g.generateGoStructDefinition(&g.fTypes, s, isException, false, false)
	g.fTypes.WriteString("\n")
	structName := g.publicizeIn(s.Name(), false, g.serviceName)
	v := newValidatorGenerator(g)
	v.structName = structName
	var body strings.Builder
	g.indentUp()
	v.generateStructValidator(&body, s)
	body.WriteString(g.indent() + "return nil\n")
	g.indentDown()
	v.generateRegexpVars(&g.fTypes)
	g.fTypes.WriteString("func (p *" + structName + ") Validate() error {\n")
	g.fTypes.WriteString(body.String())
	g.fTypes.WriteString("}\n")
}

func (g *Generator) publicizedNameAndDefValue(f *sema.Field) (string, *sema.ConstValue) {
	return g.publicize(escapeString(f.Name())), f.Value()
}

func (g *Generator) generateGoStructInitializer(out *strings.Builder, s *sema.Struct, isArgsOrResult bool) {
	out.WriteString(g.publicizeIn(g.typeName(s), isArgsOrResult, g.serviceName) + "{")
	g.indentUp()
	var names, values []string
	var multiline []bool
	for _, m := range s.Members() {
		pointerField := isPointerField(m)
		publicizedName, defValue := g.publicizedNameAndDefValue(m)
		if !pointerField && defValue != nil && !omitInitialization(m) {
			rendered := g.renderFieldInitialValue(m, m.Name(), pointerField)
			names = append(names, publicizedName)
			values = append(values, rendered)
			multiline = append(multiline, strings.Contains(rendered, "\n"))
		}
	}
	writeAlignedFields(out, g.indent(), names, values, multiline)
	g.indentDown()
	if len(names) != 0 {
		out.WriteString("\n" + g.indent())
	}
	out.WriteString("}\n")
}

func (g *Generator) renderFieldInitialValue(f *sema.Field, name string, optionalField bool) string {
	typ := sema.TrueType(f.Type())
	if optionalField {
		return "new(" + g.typeToGoType(f.Type()) + ")"
	}
	return g.renderConstValue(typ, f.Value(), name, false)
}

func (g *Generator) generateGoStructDefinition(out *strings.Builder, s *sema.Struct, isException, isResult, isArgs bool) {
	members := s.Members()
	sortedMembers := s.SortedMembers()
	structName := g.publicizeIn(s.Name(), isArgs || isResult, g.serviceName)
	g.generateStructDocstring(out, s)
	g.generateDeprecationComment(out, s.Annotations())
	out.WriteString(g.indent() + "type " + structName + " struct {\n")
	g.indentUp()
	numSetable := 0
	if len(sortedMembers) == 0 || sortedMembers[0].Key() >= 0 {
		sortedKeysPos := int32(0)
		for _, m := range sortedMembers {
			if s.IsUnion() {
				m.SetReq(sema.Optional)
			}
		}
		maxFieldNameLen := 0
		maxFieldTypeLen := 0
		groupEnd := 0
		for idx, m := range sortedMembers {
			if sortedKeysPos != m.Key() {
				firstUnused := sortedKeysPos
				sortedKeysPos++
				if firstUnused < 1 {
					firstUnused = 1
				}
				for sortedKeysPos != m.Key() {
					sortedKeysPos++
				}
				lastUnused := sortedKeysPos - 1
				if firstUnused < lastUnused {
					out.WriteString(g.indent() + "// unused fields # " + itoa(int64(firstUnused)) + " to " + itoa(int64(lastUnused)) + "\n")
				} else if firstUnused == lastUnused {
					out.WriteString(g.indent() + "// unused field # " + itoa(int64(firstUnused)) + "\n")
				}
			}
			if idx == groupEnd {
				maxFieldNameLen = 0
				maxFieldTypeLen = 0
				groupKey := m.Key()
				for groupEnd = idx; groupEnd < len(sortedMembers); groupEnd++ {
					gm := sortedMembers[groupEnd]
					if gm.Key() != groupKey {
						break
					}
					if n := len(g.publicize(gm.Name())); n > maxFieldNameLen {
						maxFieldNameLen = n
					}
					if n := len(g.typeToGoTypeWithOpt(gm.Type(), isPointerField(gm))); n > maxFieldTypeLen {
						maxFieldTypeLen = n
					}
					groupKey++
				}
			}
			fieldType := m.Type()
			goType := g.typeToGoTypeWithOpt(fieldType, isPointerField(m))
			tags := map[string]string{}
			tags["db"] = escapeString(m.Name())
			hasDefault := m.Value() != nil
			isOptional := m.Req() == sema.Optional
			if isOptional && !hasDefault {
				tags["json"] = escapeString(m.Name()) + ",omitempty"
			} else {
				tags["json"] = escapeString(m.Name())
			}
			if values, ok := m.Annotations()["go.tag"]; ok && len(values) > 0 {
				parseGoTags(tags, values[len(values)-1])
			}
			tagKeys := make([]string, 0, len(tags))
			for k := range tags {
				tagKeys = append(tagKeys, k)
			}
			sort.Strings(tagKeys)
			gotag := ""
			for _, k := range tagKeys {
				gotag += k + ":\"" + tags[k] + "\" "
			}
			gotag = gotag[:len(gotag)-1]
			g.generateDeprecationComment(out, m.Annotations())
			fieldName := g.publicize(m.Name())
			out.WriteString(g.indent() + fieldName + spaces(maxFieldNameLen-len(fieldName)+1) +
				goType + spaces(maxFieldTypeLen-len(goType)+1) + "`thrift:\"" +
				escapeString(m.Name()) + "," + itoa(int64(sortedKeysPos)))
			if m.Req() == sema.Required {
				out.WriteString(",required")
			}
			out.WriteString("\" " + gotag + "`\n")
			sortedKeysPos++
		}
	} else {
		maxFieldNameLen := 0
		for _, m := range members {
			if n := len(g.publicize(m.Name())); n > maxFieldNameLen {
				maxFieldNameLen = n
			}
		}
		for _, m := range members {
			g.generateDeprecationComment(out, m.Annotations())
			fieldName := g.publicize(m.Name())
			out.WriteString(g.indent() + fieldName + spaces(maxFieldNameLen-len(fieldName)+1) + g.typeToGoType(m.Type()) + "\n")
		}
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	g.generateDeprecationComment(out, s.Annotations())
	out.WriteString(g.indent() + "func New" + structName + "() *" + structName + " {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return &")
	g.generateGoStructInitializer(out, s, isResult || isArgs)
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	for _, m := range members {
		publicizedName, defValue := g.publicizedNameAndDefValue(m)
		fieldType := m.Type()
		goType := g.typeToGoTypeWithOpt(fieldType, false)
		defVarName := structName + "_" + publicizedName + "_DEFAULT"
		emittedDefault := false
		if m.Req() == sema.Optional || isPointerField(m) {
			g.generateDeprecationComment(out, m.Annotations())
			out.WriteString(g.indent() + "var " + defVarName + " " + goType)
			if defValue != nil {
				out.WriteString(" = " + g.renderConstValue(fieldType, defValue, m.Name(), false))
			}
			out.WriteString("\n")
			emittedDefault = true
		}
		if emittedDefault {
			out.WriteString("\n")
		}
		typ := sema.TrueType(fieldType)
		if isPointerField(m) || typ.IsMap() || typ.IsSet() || typ.IsList() || typ.IsBinary() {
			numSetable++
		}
		if isPointerField(m) {
			goOptType := g.typeToGoTypeWithOpt(fieldType, true)
			maybepointer := ""
			if goOptType != goType {
				maybepointer = "*"
			}
			g.generateDeprecationComment(out, m.Annotations())
			out.WriteString(g.indent() + "func (p *" + structName + ") Get" + publicizedName + "() " + goType + " {\n")
			g.indentUp()
			out.WriteString(g.indent() + "if !p.IsSet" + publicizedName + "() {\n")
			g.indentUp()
			out.WriteString(g.indent() + "return " + defVarName + "\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
			out.WriteString(g.indent() + "return " + maybepointer + "p." + publicizedName + "\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n\n")
		} else {
			g.generateDeprecationComment(out, m.Annotations())
			out.WriteString(g.indent() + "func (p *" + structName + ") Get" + publicizedName + "() " + goType + " {\n")
			g.indentUp()
			out.WriteString(g.indent() + "return p." + publicizedName + "\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n\n")
		}
	}
	if s.IsUnion() && numSetable > 0 {
		g.generateCountSetFieldsHelper(out, s, structName)
	}
	g.generateIssetHelpers(out, s, structName)
	g.generateGoStructReader(out, s, structName)
	g.generateGoStructWriter(out, s, structName, numSetable > 0)
	if !isResult && !isArgs {
		g.generateGoStructEquals(out, s, structName)
	}
	out.WriteString(g.indent() + "func (p *" + structName + ") String() string {\n")
	g.indentUp()
	out.WriteString(g.indent() + "if p == nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return \"<nil>\"\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "return fmt.Sprintf(\"" + escapeString(structName) + "(%+v)\", *p)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	if isException {
		out.WriteString("\n")
		out.WriteString(g.indent() + "func (p *" + structName + ") Error() string {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return p.String()\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString("\n")
		out.WriteString(g.indent() + "func (" + structName + ") TExceptionType() thrift.TExceptionType {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return thrift.TExceptionTypeCompiled\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString("\n")
		out.WriteString(g.indent() + "var _ thrift.TException = (*" + structName + ")(nil)\n")
	}
	if !g.opts.ReadWritePrivate {
		out.WriteString("\n")
		out.WriteString(g.indent() + "func (p *" + structName + ") LogValue() slog.Value {\n")
		g.indentUp()
		out.WriteString(g.indent() + "if p == nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return slog.AnyValue(nil)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "v := thrift.SlogTStructWrapper{\n")
		g.indentUp()
		out.WriteString(g.indent() + "Type:  \"*" + g.packageName + "." + structName + "\",\n")
		out.WriteString(g.indent() + "Value: p,\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "return slog.AnyValue(v)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString("\n")
		out.WriteString(g.indent() + "var _ slog.LogValuer = (*" + structName + ")(nil)\n")
	}
}

func (g *Generator) generateIssetHelpers(out *strings.Builder, s *sema.Struct, structName string) {
	for _, f := range s.Members() {
		fieldName := g.publicize(escapeString(f.Name()))
		if f.Req() == sema.Optional || isPointerField(f) {
			g.generateDeprecationComment(out, f.Annotations())
			out.WriteString(g.indent() + "func (p *" + structName + ") IsSet" + fieldName + "() bool {\n")
			g.indentUp()
			ttype := sema.TrueType(f.Type())
			isByteslice := ttype.IsBinary()
			compareToNilOnly := ttype.IsSet() || ttype.IsList() || ttype.IsMap() || (isByteslice && f.Value() == nil)
			if isPointerField(f) || compareToNilOnly {
				out.WriteString(g.indent() + "return p." + fieldName + " != nil\n")
			} else {
				defVarName := structName + "_" + fieldName + "_DEFAULT"
				if isByteslice {
					out.WriteString(g.indent() + "return !bytes.Equal(p." + fieldName + ", " + defVarName + ")\n")
				} else {
					out.WriteString(g.indent() + "return p." + fieldName + " != " + defVarName + "\n")
				}
			}
			g.indentDown()
			out.WriteString(g.indent() + "}\n\n")
		}
	}
}

func (g *Generator) generateCountSetFieldsHelper(out *strings.Builder, s *sema.Struct, structName string) {
	out.WriteString(g.indent() + "func (p *" + structName + ") CountSetFields" + structName + "() int {\n")
	g.indentUp()
	out.WriteString(g.indent() + "if p == nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return 0\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "count := 0\n")
	for _, f := range s.Members() {
		if f.Req() == sema.Required {
			continue
		}
		typ := sema.TrueType(f.Type())
		if !(isPointerField(f) || typ.IsMap() || typ.IsSet() || typ.IsList() || typ.IsBinary()) {
			continue
		}
		fieldName := g.publicize(escapeString(f.Name()))
		out.WriteString(g.indent() + "if p.IsSet" + fieldName + "() {\n")
		g.indentUp()
		out.WriteString(g.indent() + "count++\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
	out.WriteString(g.indent() + "return count\n\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// readMethod returns the ReadFieldN or ReadField_N method name for a key.
func fieldMethodName(prefix string, key int32) string {
	if key < 0 {
		return prefix + "_" + itoa(int64(-key))
	}
	return prefix + itoa(int64(key))
}

func (g *Generator) generateGoStructReader(out *strings.Builder, s *sema.Struct, structName string) {
	fields := s.Members()
	out.WriteString(g.indent() + "func (p *" + structName + ") " + g.readMethodName + "(ctx context.Context, iprot thrift.TProtocol) error {\n")
	g.indentUp()
	out.WriteString(g.indent() + "ctx, err := thrift.CheckRecursionDepth(ctx)\n")
	out.WriteString(g.indent() + "if err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return err\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "defer thrift.DecrementRecursionDepth(ctx)\n")
	out.WriteString(g.indent() + "if _, err := iprot.ReadStructBegin(ctx); err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return thrift.PrependError(fmt.Sprintf(\"%T read error: \", p), err)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	for _, f := range fields {
		if f.Req() == sema.Required {
			fieldName := g.publicize(escapeString(f.Name()))
			out.WriteString(g.indent() + "var isset" + fieldName + " bool = false\n")
		}
	}
	out.WriteString(g.indent() + "for {\n")
	g.indentUp()
	out.WriteString(g.indent() + "_, fieldTypeId, fieldId, err := iprot.ReadFieldBegin(ctx)\n")
	out.WriteString(g.indent() + "if err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return thrift.PrependError(fmt.Sprintf(\"%T field %d read error: \", p, fieldId), err)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "if fieldTypeId == thrift.STOP {\n")
	g.indentUp()
	out.WriteString(g.indent() + "break\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	haveSwitch := len(fields) != 0
	if haveSwitch {
		out.WriteString(g.indent() + "switch fieldId {\n")
	}
	for _, f := range fields {
		fieldID := f.Key()
		out.WriteString(g.indent() + "case " + itoa(int64(fieldID)) + ":\n")
		g.indentUp()
		thriftFieldTypeID := g.typeToEnum(f.Type())
		if thriftFieldTypeID == "thrift.BINARY" {
			thriftFieldTypeID = "thrift.STRING"
		}
		out.WriteString(g.indent() + "if fieldTypeId == " + thriftFieldTypeID + " {\n")
		g.indentUp()
		out.WriteString(g.indent() + "if err := p." + fieldMethodName("ReadField", fieldID) + "(ctx, iprot); err != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return err\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		if f.Req() == sema.Required {
			fieldName := g.publicize(escapeString(f.Name()))
			out.WriteString(g.indent() + "isset" + fieldName + " = true\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "} else {\n")
		g.indentUp()
		out.WriteString(g.indent() + "if err := iprot.Skip(ctx, fieldTypeId); err != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return err\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
	}
	if haveSwitch {
		out.WriteString(g.indent() + "default:\n")
		g.indentUp()
	}
	out.WriteString(g.indent() + "if err := iprot.Skip(ctx, fieldTypeId); err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return err\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	if haveSwitch {
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
	out.WriteString(g.indent() + "if err := iprot.ReadFieldEnd(ctx); err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return err\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "if err := iprot.ReadStructEnd(ctx); err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return thrift.PrependError(fmt.Sprintf(\"%T read struct end error: \", p), err)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	for _, f := range fields {
		if f.Req() == sema.Required {
			fieldName := g.publicize(escapeString(f.Name()))
			out.WriteString(g.indent() + "if !isset" + fieldName + " {\n")
			g.indentUp()
			out.WriteString(g.indent() + "return thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, " +
				"fmt.Errorf(\"Required field " + fieldName + " is not set\"))\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
	}
	out.WriteString(g.indent() + "return nil\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	for _, f := range fields {
		out.WriteString(g.indent() + "func (p *" + structName + ") " + fieldMethodName("ReadField", f.Key()) +
			"(ctx context.Context, iprot thrift.TProtocol) error {\n")
		g.indentUp()
		g.generateDeserializeField(out, f, false, "p.", false, false)
		out.WriteString(g.indent() + "return nil\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
	}
}

func (g *Generator) generateGoStructWriter(out *strings.Builder, s *sema.Struct, structName string, usesCountSetFields bool) {
	name := s.Name()
	fields := s.SortedMembers()
	out.WriteString(g.indent() + "func (p *" + structName + ") " + g.writeMethodName + "(ctx context.Context, oprot thrift.TProtocol) error {\n")
	g.indentUp()
	out.WriteString(g.indent() + "ctx, err := thrift.CheckRecursionDepth(ctx)\n")
	out.WriteString(g.indent() + "if err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return err\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "defer thrift.DecrementRecursionDepth(ctx)\n")
	if s.IsUnion() && usesCountSetFields {
		tstructName := g.publicize(s.Name())
		out.WriteString(g.indent() + "if c := p.CountSetFields" + tstructName + "(); c != 1 {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, " +
			"fmt.Errorf(\"%T write union: exactly one field must be set (%d set)\", p, c))\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
	out.WriteString(g.indent() + "if err := oprot.WriteStructBegin(ctx, \"" + name + "\"); err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return thrift.PrependError(fmt.Sprintf(\"%T write struct begin error: \", p), err)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "if p != nil {\n")
	g.indentUp()
	for _, f := range fields {
		out.WriteString(g.indent() + "if err := p." + fieldMethodName("writeField", f.Key()) + "(ctx, oprot); err != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return err\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "if err := oprot.WriteFieldStop(ctx); err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return thrift.PrependError(\"write field stop error: \", err)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "if err := oprot.WriteStructEnd(ctx); err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return thrift.PrependError(\"write struct stop error: \", err)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "return nil\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	for _, f := range fields {
		fieldID := f.Key()
		fieldName := f.Name()
		escapeFieldName := escapeString(fieldName)
		fieldRequired := f.Req()
		out.WriteString(g.indent() + "func (p *" + structName + ") " + fieldMethodName("writeField", fieldID) +
			"(ctx context.Context, oprot thrift.TProtocol) (err error) {\n")
		g.indentUp()
		if fieldRequired == sema.Optional {
			out.WriteString(g.indent() + "if p.IsSet" + g.publicize(fieldName) + "() {\n")
			g.indentUp()
		}
		out.WriteString(g.indent() + "if err := oprot.WriteFieldBegin(ctx, \"" + escapeFieldName + "\", " +
			g.typeToEnum(f.Type()) + ", " + itoa(int64(fieldID)) + "); err != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return thrift.PrependError(fmt.Sprintf(\"%T write field begin error " +
			itoa(int64(fieldID)) + ":" + escapeFieldName + ": \", p), err)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.generateSerializeField(out, f, "p.", false)
		out.WriteString(g.indent() + "if err := oprot.WriteFieldEnd(ctx); err != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return thrift.PrependError(fmt.Sprintf(\"%T write field end error " +
			itoa(int64(fieldID)) + ":" + escapeFieldName + ": \", p), err)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		if fieldRequired == sema.Optional {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		out.WriteString(g.indent() + "return err\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
	}
}

func (g *Generator) generateGoStructEquals(out *strings.Builder, s *sema.Struct, structName string) {
	fields := s.SortedMembers()
	out.WriteString(g.indent() + "func (p *" + structName + ") " + g.equalsMethodName + "(other *" + structName + ") bool {\n")
	g.indentUp()
	out.WriteString(g.indent() + "if p == other {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return true\n")
	g.indentDown()
	out.WriteString(g.indent() + "} else if p == nil || other == nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return false\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	for _, f := range fields {
		fieldType := f.Type()
		publicizeFieldName := g.publicize(f.Name())
		tgt := "p." + publicizeFieldName
		src := "other." + publicizeFieldName
		ttype := sema.TrueType(fieldType)
		if isPointerField(f) && (ttype.IsBaseType() || ttype.IsEnum() || ttype.IsContainer()) {
			tgtv := "*" + tgt
			srcv := "*" + src
			out.WriteString(g.indent() + "if " + tgt + " != " + src + " {\n")
			g.indentUp()
			out.WriteString(g.indent() + "if " + tgt + " == nil || " + src + " == nil {\n")
			g.indentUp()
			out.WriteString(g.indent() + "return false\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
			g.generateGoEquals(out, fieldType, tgtv, srcv)
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		} else {
			g.generateGoEquals(out, fieldType, tgt, src)
		}
	}
	out.WriteString(g.indent() + "return true\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}
