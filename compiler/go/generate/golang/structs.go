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
	v.generateStructValidator(&body, s)
	body.WriteString("return nil\n")
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
	var names, values []string
	for _, m := range s.Members() {
		pointerField := isPointerField(m)
		publicizedName, defValue := g.publicizedNameAndDefValue(m)
		if !pointerField && defValue != nil && !omitInitialization(m) {
			rendered := g.renderFieldInitialValue(m, m.Name(), pointerField)
			names = append(names, publicizedName)
			values = append(values, rendered)
		}
	}
	writeFields(out, names, values)
	if len(names) != 0 {
		out.WriteString("\n")
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
	out.WriteString("type " + structName + " struct {\n")
	numSetable := 0
	if len(sortedMembers) == 0 || sortedMembers[0].Key() >= 0 {
		sortedKeysPos := int32(0)
		for _, m := range sortedMembers {
			if s.IsUnion() {
				m.SetReq(sema.Optional)
			}
		}
		for _, m := range sortedMembers {
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
					out.WriteString("// unused fields # " + itoa(int64(firstUnused)) + " to " + itoa(int64(lastUnused)) + "\n")
				} else if firstUnused == lastUnused {
					out.WriteString("// unused field # " + itoa(int64(firstUnused)) + "\n")
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
			out.WriteString(fieldName + " " + goType + " `thrift:\"" +
				escapeString(m.Name()) + "," + itoa(int64(sortedKeysPos)))
			if m.Req() == sema.Required {
				out.WriteString(",required")
			}
			out.WriteString("\" " + gotag + "`\n")
			sortedKeysPos++
		}
	} else {
		for _, m := range members {
			g.generateDeprecationComment(out, m.Annotations())
			fieldName := g.publicize(m.Name())
			out.WriteString(fieldName + " " + g.typeToGoType(m.Type()) + "\n")
		}
	}
	out.WriteString("}\n\n")
	g.generateDeprecationComment(out, s.Annotations())
	out.WriteString("func New" + structName + "() *" + structName + " {\n")
	out.WriteString("return &")
	g.generateGoStructInitializer(out, s, isResult || isArgs)
	out.WriteString("}\n\n")

	for _, m := range members {
		publicizedName, defValue := g.publicizedNameAndDefValue(m)
		fieldType := m.Type()
		goType := g.typeToGoTypeWithOpt(fieldType, false)
		defVarName := structName + "_" + publicizedName + "_DEFAULT"
		emittedDefault := false
		if m.Req() == sema.Optional || isPointerField(m) {
			g.generateDeprecationComment(out, m.Annotations())
			out.WriteString("var " + defVarName + " " + goType)
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
			out.WriteString("func (p *" + structName + ") Get" + publicizedName + "() " + goType + " {\n")
			out.WriteString("if !p.IsSet" + publicizedName + "() {\n")
			out.WriteString("return " + defVarName + "\n")
			out.WriteString("}\n")
			out.WriteString("return " + maybepointer + "p." + publicizedName + "\n")
			out.WriteString("}\n\n")
		} else {
			g.generateDeprecationComment(out, m.Annotations())
			out.WriteString("func (p *" + structName + ") Get" + publicizedName + "() " + goType + " {\n")
			out.WriteString("return p." + publicizedName + "\n")
			out.WriteString("}\n\n")
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
	out.WriteString("func (p *" + structName + ") String() string {\n")
	out.WriteString("if p == nil {\n")
	out.WriteString("return \"<nil>\"\n")
	out.WriteString("}\n")
	out.WriteString("return fmt.Sprintf(\"" + escapeString(structName) + "(%+v)\", *p)\n")
	out.WriteString("}\n")
	if isException {
		out.WriteString("\n")
		out.WriteString("func (p *" + structName + ") Error() string {\n")
		out.WriteString("return p.String()\n")
		out.WriteString("}\n")
		out.WriteString("\n")
		out.WriteString("func (" + structName + ") TExceptionType() thrift.TExceptionType {\n")
		out.WriteString("return thrift.TExceptionTypeCompiled\n")
		out.WriteString("}\n")
		out.WriteString("\n")
		out.WriteString("var _ thrift.TException = (*" + structName + ")(nil)\n")
	}
	if !g.opts.ReadWritePrivate {
		out.WriteString("\n")
		out.WriteString("func (p *" + structName + ") LogValue() slog.Value {\n")
		out.WriteString("if p == nil {\n")
		out.WriteString("return slog.AnyValue(nil)\n")
		out.WriteString("}\n")
		out.WriteString("v := thrift.SlogTStructWrapper{\n")
		out.WriteString("Type:  \"*" + g.packageName + "." + structName + "\",\n")
		out.WriteString("Value: p,\n")
		out.WriteString("}\n")
		out.WriteString("return slog.AnyValue(v)\n")
		out.WriteString("}\n")
		out.WriteString("\n")
		out.WriteString("var _ slog.LogValuer = (*" + structName + ")(nil)\n")
	}
}

func (g *Generator) generateIssetHelpers(out *strings.Builder, s *sema.Struct, structName string) {
	for _, f := range s.Members() {
		fieldName := g.publicize(escapeString(f.Name()))
		if f.Req() == sema.Optional || isPointerField(f) {
			g.generateDeprecationComment(out, f.Annotations())
			out.WriteString("func (p *" + structName + ") IsSet" + fieldName + "() bool {\n")
			ttype := sema.TrueType(f.Type())
			isByteslice := ttype.IsBinary()
			compareToNilOnly := ttype.IsSet() || ttype.IsList() || ttype.IsMap() || (isByteslice && f.Value() == nil)
			if isPointerField(f) || compareToNilOnly {
				out.WriteString("return p." + fieldName + " != nil\n")
			} else {
				defVarName := structName + "_" + fieldName + "_DEFAULT"
				if isByteslice {
					out.WriteString("return !bytes.Equal(p." + fieldName + ", " + defVarName + ")\n")
				} else {
					out.WriteString("return p." + fieldName + " != " + defVarName + "\n")
				}
			}
			out.WriteString("}\n\n")
		}
	}
}

func (g *Generator) generateCountSetFieldsHelper(out *strings.Builder, s *sema.Struct, structName string) {
	out.WriteString("func (p *" + structName + ") CountSetFields" + structName + "() int {\n")
	out.WriteString("if p == nil {\n")
	out.WriteString("return 0\n")
	out.WriteString("}\n")
	out.WriteString("count := 0\n")
	for _, f := range s.Members() {
		if f.Req() == sema.Required {
			continue
		}
		typ := sema.TrueType(f.Type())
		if !(isPointerField(f) || typ.IsMap() || typ.IsSet() || typ.IsList() || typ.IsBinary()) {
			continue
		}
		fieldName := g.publicize(escapeString(f.Name()))
		out.WriteString("if p.IsSet" + fieldName + "() {\n")
		out.WriteString("count++\n")
		out.WriteString("}\n")
	}
	out.WriteString("return count\n\n")
	out.WriteString("}\n\n")
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
	out.WriteString("func (p *" + structName + ") " + g.readMethodName + "(ctx context.Context, iprot thrift.TProtocol) error {\n")
	out.WriteString("ctx, err := thrift.CheckRecursionDepth(ctx)\n")
	out.WriteString("if err != nil {\n")
	out.WriteString("return err\n")
	out.WriteString("}\n")
	out.WriteString("defer thrift.DecrementRecursionDepth(ctx)\n")
	out.WriteString("if _, err := iprot.ReadStructBegin(ctx); err != nil {\n")
	out.WriteString("return thrift.PrependError(fmt.Sprintf(\"%T read error: \", p), err)\n")
	out.WriteString("}\n")
	for _, f := range fields {
		if f.Req() == sema.Required {
			fieldName := g.publicize(escapeString(f.Name()))
			out.WriteString("var isset" + fieldName + " bool = false\n")
		}
	}
	out.WriteString("for {\n")
	out.WriteString("_, fieldTypeId, fieldId, err := iprot.ReadFieldBegin(ctx)\n")
	out.WriteString("if err != nil {\n")
	out.WriteString("return thrift.PrependError(fmt.Sprintf(\"%T field %d read error: \", p, fieldId), err)\n")
	out.WriteString("}\n")
	out.WriteString("if fieldTypeId == thrift.STOP {\n")
	out.WriteString("break\n")
	out.WriteString("}\n")
	haveSwitch := len(fields) != 0
	if haveSwitch {
		out.WriteString("switch fieldId {\n")
	}
	for _, f := range fields {
		fieldID := f.Key()
		out.WriteString("case " + itoa(int64(fieldID)) + ":\n")
		thriftFieldTypeID := g.typeToEnum(f.Type())
		if thriftFieldTypeID == "thrift.BINARY" {
			thriftFieldTypeID = "thrift.STRING"
		}
		out.WriteString("if fieldTypeId == " + thriftFieldTypeID + " {\n")
		out.WriteString("if err := p." + fieldMethodName("ReadField", fieldID) + "(ctx, iprot); err != nil {\n")
		out.WriteString("return err\n")
		out.WriteString("}\n")
		if f.Req() == sema.Required {
			fieldName := g.publicize(escapeString(f.Name()))
			out.WriteString("isset" + fieldName + " = true\n")
		}
		out.WriteString("} else {\n")
		out.WriteString("if err := iprot.Skip(ctx, fieldTypeId); err != nil {\n")
		out.WriteString("return err\n")
		out.WriteString("}\n")
		out.WriteString("}\n")
	}
	if haveSwitch {
		out.WriteString("default:\n")
	}
	out.WriteString("if err := iprot.Skip(ctx, fieldTypeId); err != nil {\n")
	out.WriteString("return err\n")
	out.WriteString("}\n")
	if haveSwitch {
		out.WriteString("}\n")
	}
	out.WriteString("if err := iprot.ReadFieldEnd(ctx); err != nil {\n")
	out.WriteString("return err\n")
	out.WriteString("}\n")
	out.WriteString("}\n")
	out.WriteString("if err := iprot.ReadStructEnd(ctx); err != nil {\n")
	out.WriteString("return thrift.PrependError(fmt.Sprintf(\"%T read struct end error: \", p), err)\n")
	out.WriteString("}\n")
	for _, f := range fields {
		if f.Req() == sema.Required {
			fieldName := g.publicize(escapeString(f.Name()))
			out.WriteString("if !isset" + fieldName + " {\n")
			out.WriteString("return thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, " +
				"fmt.Errorf(\"Required field " + fieldName + " is not set\"))\n")
			out.WriteString("}\n")
		}
	}
	out.WriteString("return nil\n")
	out.WriteString("}\n\n")
	for _, f := range fields {
		out.WriteString("func (p *" + structName + ") " + fieldMethodName("ReadField", f.Key()) +
			"(ctx context.Context, iprot thrift.TProtocol) error {\n")
		g.generateDeserializeField(out, f, false, "p.", false, false)
		out.WriteString("return nil\n")
		out.WriteString("}\n\n")
	}
}

func (g *Generator) generateGoStructWriter(out *strings.Builder, s *sema.Struct, structName string, usesCountSetFields bool) {
	name := s.Name()
	fields := s.SortedMembers()
	out.WriteString("func (p *" + structName + ") " + g.writeMethodName + "(ctx context.Context, oprot thrift.TProtocol) error {\n")
	out.WriteString("ctx, err := thrift.CheckRecursionDepth(ctx)\n")
	out.WriteString("if err != nil {\n")
	out.WriteString("return err\n")
	out.WriteString("}\n")
	out.WriteString("defer thrift.DecrementRecursionDepth(ctx)\n")
	if s.IsUnion() && usesCountSetFields {
		tstructName := g.publicize(s.Name())
		out.WriteString("if c := p.CountSetFields" + tstructName + "(); c != 1 {\n")
		out.WriteString("return thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, " +
			"fmt.Errorf(\"%T write union: exactly one field must be set (%d set)\", p, c))\n")
		out.WriteString("}\n")
	}
	out.WriteString("if err := oprot.WriteStructBegin(ctx, \"" + name + "\"); err != nil {\n")
	out.WriteString("return thrift.PrependError(fmt.Sprintf(\"%T write struct begin error: \", p), err)\n")
	out.WriteString("}\n")
	out.WriteString("if p != nil {\n")
	for _, f := range fields {
		out.WriteString("if err := p." + fieldMethodName("writeField", f.Key()) + "(ctx, oprot); err != nil {\n")
		out.WriteString("return err\n")
		out.WriteString("}\n")
	}
	out.WriteString("}\n")
	out.WriteString("if err := oprot.WriteFieldStop(ctx); err != nil {\n")
	out.WriteString("return thrift.PrependError(\"write field stop error: \", err)\n")
	out.WriteString("}\n")
	out.WriteString("if err := oprot.WriteStructEnd(ctx); err != nil {\n")
	out.WriteString("return thrift.PrependError(\"write struct stop error: \", err)\n")
	out.WriteString("}\n")
	out.WriteString("return nil\n")
	out.WriteString("}\n\n")
	for _, f := range fields {
		fieldID := f.Key()
		fieldName := f.Name()
		escapeFieldName := escapeString(fieldName)
		fieldRequired := f.Req()
		out.WriteString("func (p *" + structName + ") " + fieldMethodName("writeField", fieldID) +
			"(ctx context.Context, oprot thrift.TProtocol) (err error) {\n")
		// Default requiredness means "write if set" (doc/specs/idl.md), and a
		// pointer field is unset exactly when it is nil.
		checkIfSet := fieldRequired == sema.Optional ||
			(fieldRequired == sema.OptInReqOut && isPointerField(f))
		if checkIfSet {
			out.WriteString("if p.IsSet" + g.publicize(fieldName) + "() {\n")
		}
		out.WriteString("if err := oprot.WriteFieldBegin(ctx, \"" + escapeFieldName + "\", " +
			g.typeToEnum(f.Type()) + ", " + itoa(int64(fieldID)) + "); err != nil {\n")
		out.WriteString("return thrift.PrependError(fmt.Sprintf(\"%T write field begin error " +
			itoa(int64(fieldID)) + ":" + escapeFieldName + ": \", p), err)\n")
		out.WriteString("}\n")
		g.generateSerializeField(out, f, "p.", false)
		out.WriteString("if err := oprot.WriteFieldEnd(ctx); err != nil {\n")
		out.WriteString("return thrift.PrependError(fmt.Sprintf(\"%T write field end error " +
			itoa(int64(fieldID)) + ":" + escapeFieldName + ": \", p), err)\n")
		out.WriteString("}\n")
		if checkIfSet {
			out.WriteString("}\n")
		}
		out.WriteString("return err\n")
		out.WriteString("}\n\n")
	}
}

func (g *Generator) generateGoStructEquals(out *strings.Builder, s *sema.Struct, structName string) {
	fields := s.SortedMembers()
	out.WriteString("func (p *" + structName + ") " + g.equalsMethodName + "(other *" + structName + ") bool {\n")
	out.WriteString("if p == other {\n")
	out.WriteString("return true\n")
	out.WriteString("} else if p == nil || other == nil {\n")
	out.WriteString("return false\n")
	out.WriteString("}\n")
	for _, f := range fields {
		fieldType := f.Type()
		publicizeFieldName := g.publicize(f.Name())
		tgt := "p." + publicizeFieldName
		src := "other." + publicizeFieldName
		ttype := sema.TrueType(fieldType)
		if isPointerField(f) && (ttype.IsBaseType() || ttype.IsEnum() || ttype.IsContainer()) {
			tgtv := "*" + tgt
			srcv := "*" + src
			out.WriteString("if " + tgt + " != " + src + " {\n")
			out.WriteString("if " + tgt + " == nil || " + src + " == nil {\n")
			out.WriteString("return false\n")
			out.WriteString("}\n")
			g.generateGoEquals(out, fieldType, tgtv, srcv)
			out.WriteString("}\n")
		} else {
			g.generateGoEquals(out, fieldType, tgt, src)
		}
	}
	out.WriteString("return true\n")
	out.WriteString("}\n\n")
}
