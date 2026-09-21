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
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

// This file ports go_validator_generator.cc and validator_parser.cc: the
// "vt.*" annotations that turn into a Validate method on every struct.

type validationValueKind int

const (
	vvInteger validationValueKind = iota
	vvDouble
	vvBool
	vvEnum
	vvString
	vvFunction
	vvFieldReference
)

type validationFunction struct {
	name      string
	arguments []*validationValue
}

type validationValue struct {
	kind     validationValueKind
	intVal   int64
	dblVal   float64
	boolVal  bool
	enumVal  *sema.EnumValue
	strVal   string
	function *validationFunction
	field    *sema.Field
}

func (v *validationValue) isFieldReference() bool     { return v.kind == vvFieldReference }
func (v *validationValue) isValidationFunction() bool { return v.kind == vvFunction }

type validationRule struct {
	name   string
	values []*validationValue
	inner  *validationRule
}

// validationParser is validation_parser: it reads the vt.* annotations of
// one field against the type of the field.
type validationParser struct {
	reference *sema.Struct
}

func (p *validationParser) parseField(typ sema.Type, annotations sema.Annotations) []*validationRule {
	if typ.IsTypedef() {
		typ = sema.TrueType(typ)
	}
	switch {
	case typ.IsEnum():
		return p.parseEnumField(typ.(*sema.Enum), annotations)
	case typ.IsBaseType():
		switch typ.(*sema.BaseType).Base() {
		case sema.TypeUUID, sema.TypeVoid:
			return nil
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			return p.parseIntegerField(annotations)
		case sema.TypeDouble:
			return p.parseDoubleField(annotations)
		case sema.TypeString:
			return p.parseStringField(annotations)
		case sema.TypeBool:
			return p.parseBoolField(annotations)
		}
	case typ.IsList():
		return p.parseListField(typ, annotations)
	case typ.IsSet():
		return p.parseListField(typ, annotations)
	case typ.IsMap():
		return p.parseMapField(typ.(*sema.Map), annotations)
	case typ.IsStruct() || typ.IsXception():
		return p.parseStructField(annotations)
	}
	throw("validator error: unsupported type: %s", typ.Name())
	return nil
}

func (p *validationParser) parseBoolField(annotations sema.Annotations) []*validationRule {
	var rules []*validationRule
	p.addBoolRule(&rules, "vt.const", annotations)
	return rules
}

func (p *validationParser) parseEnumField(e *sema.Enum, annotations sema.Annotations) []*validationRule {
	var rules []*validationRule
	p.addBoolRule(&rules, "vt.defined_only", annotations)
	p.addEnumListRule(&rules, e, "vt.in", annotations)
	p.addEnumListRule(&rules, e, "vt.not_in", annotations)
	return rules
}

func (p *validationParser) parseDoubleField(annotations sema.Annotations) []*validationRule {
	var rules []*validationRule
	p.addDoubleRule(&rules, "vt.lt", annotations)
	p.addDoubleRule(&rules, "vt.le", annotations)
	p.addDoubleRule(&rules, "vt.gt", annotations)
	p.addDoubleRule(&rules, "vt.ge", annotations)
	p.addDoubleListRule(&rules, "vt.in", annotations)
	p.addDoubleListRule(&rules, "vt.not_in", annotations)
	return rules
}

func (p *validationParser) parseIntegerField(annotations sema.Annotations) []*validationRule {
	var rules []*validationRule
	p.addIntegerRule(&rules, "vt.lt", annotations)
	p.addIntegerRule(&rules, "vt.le", annotations)
	p.addIntegerRule(&rules, "vt.gt", annotations)
	p.addIntegerRule(&rules, "vt.ge", annotations)
	p.addIntegerListRule(&rules, "vt.in", annotations)
	p.addIntegerListRule(&rules, "vt.not_in", annotations)
	return rules
}

func (p *validationParser) parseStringField(annotations sema.Annotations) []*validationRule {
	var rules []*validationRule
	p.addStringRule(&rules, "vt.const", annotations)
	p.addIntegerRule(&rules, "vt.min_size", annotations)
	p.addIntegerRule(&rules, "vt.max_size", annotations)
	p.addStringRule(&rules, "vt.pattern", annotations)
	p.addStringRule(&rules, "vt.prefix", annotations)
	p.addStringRule(&rules, "vt.suffix", annotations)
	p.addStringRule(&rules, "vt.contains", annotations)
	p.addStringRule(&rules, "vt.not_contains", annotations)
	return rules
}

// subAnnotations collects the annotations under a prefix ("vt.elem",
// "vt.key", "vt.value") renamed back to "vt.*" for the nested type.
func subAnnotations(annotations sema.Annotations, prefix string) sema.Annotations {
	sub := sema.Annotations{}
	for _, k := range annotations.Keys() {
		if strings.HasPrefix(k, prefix) {
			sub["vt"+k[len(prefix):]] = annotations[k]
		}
	}
	return sub
}

func (p *validationParser) parseListField(typ sema.Type, annotations sema.Annotations) []*validationRule {
	var rules []*validationRule
	p.addIntegerRule(&rules, "vt.min_size", annotations)
	p.addIntegerRule(&rules, "vt.max_size", annotations)
	elemAnnotations := subAnnotations(annotations, "vt.elem")
	var elemRules []*validationRule
	if typ.IsList() {
		elemRules = p.parseField(typ.(*sema.List).ElemType(), elemAnnotations)
	} else if typ.IsSet() {
		elemRules = p.parseField(typ.(*sema.Set).ElemType(), elemAnnotations)
	}
	for _, r := range elemRules {
		rules = append(rules, &validationRule{name: "vt.elem", inner: r})
	}
	return rules
}

func (p *validationParser) parseMapField(m *sema.Map, annotations sema.Annotations) []*validationRule {
	var rules []*validationRule
	p.addIntegerRule(&rules, "vt.min_size", annotations)
	p.addIntegerRule(&rules, "vt.max_size", annotations)
	for _, r := range p.parseField(m.KeyType(), subAnnotations(annotations, "vt.key")) {
		rules = append(rules, &validationRule{name: "vt.key", inner: r})
	}
	for _, r := range p.parseField(m.ValType(), subAnnotations(annotations, "vt.value")) {
		rules = append(rules, &validationRule{name: "vt.value", inner: r})
	}
	return rules
}

func (p *validationParser) parseStructField(annotations sema.Annotations) []*validationRule {
	var rules []*validationRule
	p.addBoolRule(&rules, "vt.skip", annotations)
	return rules
}

func (p *validationParser) isReferenceField(value string) bool {
	if value == "" || value[0] != '$' {
		return false
	}
	return p.reference.FieldByName(value[1:]) != nil
}

func isValidationFunction(value string) bool {
	return value != "" && value[0] == '@'
}

func (p *validationParser) referencedField(value string) *sema.Field {
	return p.reference.FieldByName(value[1:])
}

func parseBoolAlpha(s string) bool {
	// istringstream >> boolalpha accepts exactly "true" and "false"; on
	// failure the C++ variable is left uninitialized, taken here as false.
	return s == "true"
}

func (p *validationParser) addBoolRule(rules *[]*validationRule, key string, annotations sema.Annotations) {
	values, ok := annotations[key]
	if !ok || len(values) == 0 {
		return
	}
	for _, v := range values {
		rule := &validationRule{name: key}
		if p.isReferenceField(v) {
			rule.values = append(rule.values, &validationValue{kind: vvFieldReference, field: p.referencedField(v)})
		} else {
			rule.values = append(rule.values, &validationValue{kind: vvBool, boolVal: parseBoolAlpha(values[len(values)-1])})
		}
		*rules = append(*rules, rule)
	}
}

func stod(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); !ok || ne.Err != strconv.ErrRange {
			throw("validator error: invalid floating point value: %s", s)
		}
	}
	return f
}

func stoll(s string) int64 {
	i, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		throw("validator error: invalid integer value: %s", s)
	}
	return i
}

func (p *validationParser) addDoubleRule(rules *[]*validationRule, key string, annotations sema.Annotations) {
	values, ok := annotations[key]
	if !ok || len(values) == 0 {
		return
	}
	for _, v := range values {
		if v == "" {
			continue
		}
		rule := &validationRule{name: key}
		switch {
		case isValidationFunction(v):
			rule.values = append(rule.values, &validationValue{kind: vvFunction, function: p.validationFunction(v)})
		case p.isReferenceField(v):
			rule.values = append(rule.values, &validationValue{kind: vvFieldReference, field: p.referencedField(v)})
		default:
			rule.values = append(rule.values, &validationValue{kind: vvDouble, dblVal: stod(v)})
		}
		*rules = append(*rules, rule)
	}
}

// splitList is strtok with the delimiters "[], ".
func splitList(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '[' || r == ']' || r == ',' || r == ' '
	})
}

func (p *validationParser) addEnumListRule(rules *[]*validationRule, e *sema.Enum, key string, annotations sema.Annotations) {
	values, ok := annotations[key]
	if !ok || len(values) == 0 {
		return
	}
	for _, v := range values {
		if v == "" {
			continue
		}
		rule := &validationRule{name: key}
		if v[0] == '[' {
			for _, temp := range splitList(v) {
				switch {
				case isValidationFunction(temp):
					rule.values = append(rule.values, &validationValue{kind: vvFunction, function: p.validationFunction(temp)})
				case p.isReferenceField(temp):
					rule.values = append(rule.values, &validationValue{kind: vvFieldReference, field: p.referencedField(temp)})
				default:
					val := temp
					if dot := strings.LastIndexByte(val, '.'); dot >= 0 {
						val = val[dot+1:]
					}
					rule.values = append(rule.values, &validationValue{kind: vvEnum, enumVal: e.ConstantByName(val)})
				}
			}
		} else {
			val := v
			if dot := strings.LastIndexByte(val, '.'); dot >= 0 {
				val = val[dot+1:]
			}
			rule.values = append(rule.values, &validationValue{kind: vvEnum, enumVal: e.ConstantByName(val)})
		}
		*rules = append(*rules, rule)
	}
}

func (p *validationParser) addDoubleListRule(rules *[]*validationRule, key string, annotations sema.Annotations) {
	values, ok := annotations[key]
	if !ok || len(values) == 0 {
		return
	}
	var scalar []string
	for _, v := range values {
		if v == "" {
			continue
		}
		if v[0] == '[' {
			rule := &validationRule{name: key}
			for _, temp := range splitList(v) {
				switch {
				case isValidationFunction(temp):
					rule.values = append(rule.values, &validationValue{kind: vvFunction, function: p.validationFunction(temp)})
				case p.isReferenceField(temp):
					rule.values = append(rule.values, &validationValue{kind: vvFieldReference, field: p.referencedField(temp)})
				default:
					f, err := strconv.ParseFloat(temp, 64)
					if err != nil {
						throw("validator error: validation double list parse failed: %s", temp)
					}
					rule.values = append(rule.values, &validationValue{kind: vvDouble, dblVal: f})
				}
			}
			*rules = append(*rules, rule)
		} else {
			scalar = append(scalar, v)
		}
	}
	if len(scalar) > 0 {
		p.addDoubleRule(rules, key, sema.Annotations{key: scalar})
	}
}

func (p *validationParser) addIntegerRule(rules *[]*validationRule, key string, annotations sema.Annotations) {
	values, ok := annotations[key]
	if !ok || len(values) == 0 {
		return
	}
	for _, v := range values {
		if v == "" {
			continue
		}
		rule := &validationRule{name: key}
		switch {
		case p.isReferenceField(v):
			rule.values = append(rule.values, &validationValue{kind: vvFieldReference, field: p.referencedField(v)})
		case isValidationFunction(v):
			rule.values = append(rule.values, &validationValue{kind: vvFunction, function: p.validationFunction(v)})
		default:
			rule.values = append(rule.values, &validationValue{kind: vvInteger, intVal: stoll(v)})
		}
		*rules = append(*rules, rule)
	}
}

func (p *validationParser) addIntegerListRule(rules *[]*validationRule, key string, annotations sema.Annotations) {
	values, ok := annotations[key]
	if !ok || len(values) == 0 {
		return
	}
	var scalar []string
	for _, v := range values {
		if v == "" {
			continue
		}
		if v[0] == '[' {
			rule := &validationRule{name: key}
			for _, temp := range splitList(v) {
				switch {
				case isValidationFunction(temp):
					rule.values = append(rule.values, &validationValue{kind: vvFunction, function: p.validationFunction(temp)})
				case p.isReferenceField(temp):
					rule.values = append(rule.values, &validationValue{kind: vvFieldReference, field: p.referencedField(temp)})
				default:
					i, err := strconv.ParseInt(temp, 10, 64)
					if err != nil {
						throw("validator error: validation integer list parse failed: %s", temp)
					}
					rule.values = append(rule.values, &validationValue{kind: vvInteger, intVal: i})
				}
			}
			*rules = append(*rules, rule)
		} else {
			scalar = append(scalar, v)
		}
	}
	if len(scalar) > 0 {
		p.addIntegerRule(rules, key, sema.Annotations{key: scalar})
	}
}

func (p *validationParser) addStringRule(rules *[]*validationRule, key string, annotations sema.Annotations) {
	values, ok := annotations[key]
	if !ok || len(values) == 0 {
		return
	}
	for _, v := range values {
		rule := &validationRule{name: key}
		if p.isReferenceField(v) {
			rule.values = append(rule.values, &validationValue{kind: vvFieldReference, field: p.referencedField(v)})
		} else {
			rule.values = append(rule.values, &validationValue{kind: vvString, strVal: v})
		}
		*rules = append(*rules, rule)
	}
}

func (p *validationParser) validationFunction(annotationValue string) *validationFunction {
	value := annotationValue[1:]
	nameEnd := strings.IndexByte(value, '(')
	if nameEnd < 0 {
		throw("validator error: validation function parse failed: %s", annotationValue)
	}
	fn := &validationFunction{name: value[:nameEnd]}
	value = value[nameEnd+1:]
	if fn.name != "len" {
		throw("validator error: validation function parse failed, function not supported: %s", annotationValue)
	}
	argEnd := strings.IndexByte(value, ')')
	if argEnd < 0 {
		throw("validator error: validation function parse failed: %s", annotationValue)
	}
	argument := value[:argEnd]
	if len(argument) > 0 && argument[0] == '$' {
		fn.arguments = append(fn.arguments, &validationValue{kind: vvFieldReference, field: p.referencedField(argument)})
	} else {
		throw("validator error: validation function parse failed, unrecognized argument: %s", annotationValue)
	}
	return fn
}

// validatorGenerator is go_validator_generator for one struct.
type validatorGenerator struct {
	g            *Generator
	structName   string
	patternCache []struct{ pattern, name string }
	tmp          map[string]int
}

func newValidatorGenerator(g *Generator) *validatorGenerator {
	return &validatorGenerator{g: g, tmp: map[string]int{}}
}

type validatorContext struct {
	fieldSymbol string
	tgt         string
	opt         bool
	typ         sema.Type
	rules       []*validationRule
}

func (v *validatorGenerator) genID(id string) string {
	n := v.tmp[id]
	v.tmp[id]++
	return id + strconv.Itoa(n)
}

func (v *validatorGenerator) fieldReferenceName(f *sema.Field) string {
	typ := f.Type()
	tgt, _ := v.g.publicizedNameAndDefValue(f)
	tgt = "p." + tgt
	if isPointerField(f) && (typ.IsBaseType() || typ.IsEnum() || typ.IsContainer()) {
		tgt = "*" + tgt
	}
	return tgt
}

func (v *validatorGenerator) generateStructValidator(out *strings.Builder, s *sema.Struct) {
	parser := &validationParser{reference: s}
	for _, f := range s.Members() {
		rules := parser.parseField(f.Type(), f.Annotations())
		if len(rules) == 0 {
			continue
		}
		v.generateFieldValidator(out, validatorContext{
			fieldSymbol: s.Name() + "." + f.Name(),
			tgt:         v.fieldReferenceName(f),
			opt:         f.Req() == sema.Optional,
			typ:         f.Type(),
			rules:       rules,
		})
	}
}

func (v *validatorGenerator) failure(out *strings.Builder, ctx validatorContext, key string) {
	out.WriteString("return thrift.NewValidationException(thrift.VALIDATION_FAILED, \"" + key + "\", \"" +
		ctx.fieldSymbol + "\", \"" + ctx.fieldSymbol + " not valid, rule " + key + " check failed\")\n")
	out.WriteString("}\n")
}

func (v *validatorGenerator) generateFieldValidator(out *strings.Builder, ctx validatorContext) {
	typ := ctx.typ
	if typ.IsTypedef() {
		typ = sema.TrueType(typ)
	}
	deref := ctx.tgt != "" && ctx.tgt[0] == '*'
	switch {
	case typ.IsEnum():
		if deref {
			out.WriteString("if " + ctx.tgt[1:] + " != nil {\n")
		}
		v.generateEnumFieldValidator(out, ctx)
		if deref {
			out.WriteString("}\n")
		}
	case typ.IsBaseType():
		if deref {
			out.WriteString("if " + ctx.tgt[1:] + " != nil {\n")
		}
		switch typ.(*sema.BaseType).Base() {
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			v.generateIntegerFieldValidator(out, ctx, typ.(*sema.BaseType))
		case sema.TypeDouble:
			v.generateDoubleFieldValidator(out, ctx)
		case sema.TypeString:
			v.generateStringFieldValidator(out, ctx, typ)
		case sema.TypeBool:
			v.generateBoolFieldValidator(out, ctx)
		}
		if deref {
			out.WriteString("}\n")
		}
	case typ.IsList() || typ.IsSet():
		v.generateListFieldValidator(out, ctx, typ)
	case typ.IsMap():
		v.generateMapFieldValidator(out, ctx, typ.(*sema.Map))
	case typ.IsStruct() || typ.IsXception():
		v.generateStructFieldValidator(out, ctx)
	default:
		throw("validator error: unsupported type: %s", typ.Name())
	}
}

func (v *validatorGenerator) enumValueText(val *validationValue) string {
	if val.isFieldReference() {
		return v.fieldReferenceName(val.field)
	}
	return itoa(int64(val.enumVal.Value()))
}

func (v *validatorGenerator) generateEnumFieldValidator(out *strings.Builder, ctx validatorContext) {
	for _, rule := range ctx.rules {
		values := rule.values
		if len(values) == 0 {
			continue
		}
		key := rule.name
		switch key {
		case "vt.in":
			if len(values) > 1 {
				exist := v.genID("_exist")
				out.WriteString("var " + exist + " bool\n")
				src := v.genID("_src")
				out.WriteString(src + " := []int64{")
				for i, val := range values {
					if i > 0 {
						out.WriteString(", ")
					}
					out.WriteString("int64(" + v.enumValueText(val) + ")")
				}
				out.WriteString("}\n")
				out.WriteString("for _, src := range " + src + " {\n")
				out.WriteString("if int64(" + ctx.tgt + ") == src {\n")
				out.WriteString(exist + " = true\n")
				out.WriteString("break\n")
				out.WriteString("}\n")
				out.WriteString("}\n")
				out.WriteString("if " + exist + " == false {\n")
			} else {
				out.WriteString("if int64(" + ctx.tgt + ") != int64(" + v.enumValueText(values[0]) + ") {\n")
			}
			v.failure(out, ctx, "vt.in")
			if len(values) > 1 {
				out.WriteString("}\n")
			}
		case "vt.not_in":
			if len(values) > 1 {
				src := v.genID("_src")
				out.WriteString(src + " := []int64{")
				for i, val := range values {
					if i > 0 {
						out.WriteString(", ")
					}
					out.WriteString("int64(" + v.enumValueText(val) + ")")
				}
				out.WriteString("}\n")
				out.WriteString("for _, src := range " + src + " {\n")
				out.WriteString("if int64(" + ctx.tgt + ") == src {\n")
			} else {
				out.WriteString("if int64(" + ctx.tgt + ") == int64(" + v.enumValueText(values[0]) + ") {\n")
			}
			v.failure(out, ctx, "vt.not_in")
			if len(values) > 1 {
				out.WriteString("}\n")
			}
		case "vt.defined_only":
			if !values[0].boolVal {
				continue
			}
			out.WriteString("if !(" + ctx.tgt + ").IsDefined() {\n")
			v.failure(out, ctx, key)
		}
	}
}

func (v *validatorGenerator) generateBoolFieldValidator(out *strings.Builder, ctx validatorContext) {
	for _, rule := range ctx.rules {
		values := rule.values
		if len(values) == 0 {
			continue
		}
		key := rule.name
		if key == "vt.const" {
			out.WriteString("if " + ctx.tgt + " != ")
			if values[0].isFieldReference() {
				out.WriteString(v.fieldReferenceName(values[0].field))
			} else if values[0].boolVal {
				out.WriteString("true")
			} else {
				out.WriteString("false")
			}
		}
		out.WriteString(" {\n")
		v.failure(out, ctx, key)
	}
}

var comparisonSigns = map[string]string{"vt.lt": ">=", "vt.le": ">", "vt.gt": "<=", "vt.ge": "<"}

func (v *validatorGenerator) doubleValueText(val *validationValue) string {
	if val.isFieldReference() {
		return v.fieldReferenceName(val.field)
	}
	return formatDouble(val.dblVal)
}

func (v *validatorGenerator) generateDoubleFieldValidator(out *strings.Builder, ctx validatorContext) {
	for _, rule := range ctx.rules {
		values := rule.values
		if len(values) == 0 {
			continue
		}
		key := rule.name
		if sign, ok := comparisonSigns[key]; ok {
			out.WriteString("if " + ctx.tgt + " " + sign + " " + v.doubleValueText(values[0]) + " {\n")
			v.failure(out, ctx, key)
			continue
		}
		switch key {
		case "vt.in":
			if len(values) > 1 {
				exist := v.genID("_exist")
				out.WriteString("var " + exist + " bool\n")
				src := v.genID("_src")
				out.WriteString(src + " := []float64{")
				for i, val := range values {
					if i > 0 {
						out.WriteString(", ")
					}
					out.WriteString(v.doubleValueText(val))
				}
				out.WriteString("}\n")
				out.WriteString("for _, src := range " + src + " {\n")
				out.WriteString("if " + ctx.tgt + " == src {\n")
				out.WriteString(exist + " = true\n")
				out.WriteString("break\n")
				out.WriteString("}\n")
				out.WriteString("}\n")
				out.WriteString("if " + exist + " == false {\n")
			} else {
				out.WriteString("if " + ctx.tgt + " != " + v.doubleValueText(values[0]) + " {\n")
			}
			v.failure(out, ctx, "vt.in")
		case "vt.not_in":
			if len(values) > 1 {
				src := v.genID("_src")
				out.WriteString(src + " := []float64{")
				for i, val := range values {
					if i > 0 {
						out.WriteString(", ")
					}
					out.WriteString(v.doubleValueText(val))
				}
				out.WriteString("}\n")
				out.WriteString("for _, src := range " + src + " {\n")
				out.WriteString("if " + ctx.tgt + " == src {\n")
			} else {
				out.WriteString("if " + ctx.tgt + " == " + v.doubleValueText(values[0]) + " {\n")
			}
			v.failure(out, ctx, "vt.not_in")
			if len(values) > 1 {
				out.WriteString("}\n")
			}
		}
	}
}

func goIntType(b *sema.BaseType) string {
	switch b.Base() {
	case sema.TypeI8:
		return "int8"
	case sema.TypeI16:
		return "int16"
	case sema.TypeI32:
		return "int32"
	case sema.TypeI64:
		return "int64"
	}
	throw("validator error: unsupported integer type: %s", b.Name())
	return ""
}

func (v *validatorGenerator) integerValueText(val *validationValue, b *sema.BaseType) string {
	switch {
	case val.isFieldReference():
		return v.fieldReferenceName(val.field)
	case val.isValidationFunction():
		s := goIntType(b) + "("
		if val.function.name == "len" {
			s += "len("
			if val.function.arguments[0].isFieldReference() {
				s += v.fieldReferenceName(val.function.arguments[0].field)
			}
			s += ")"
		}
		return s + ")"
	}
	return itoa(val.intVal)
}

func (v *validatorGenerator) generateIntegerFieldValidator(out *strings.Builder, ctx validatorContext, b *sema.BaseType) {
	for _, rule := range ctx.rules {
		values := rule.values
		if len(values) == 0 {
			continue
		}
		key := rule.name
		if sign, ok := comparisonSigns[key]; ok {
			out.WriteString("if " + ctx.tgt + " " + sign + " " + v.integerValueText(values[0], b) + " {\n")
			v.failure(out, ctx, key)
			continue
		}
		switch key {
		case "vt.in":
			if len(values) > 1 {
				exist := v.genID("_exist")
				out.WriteString("var " + exist + " bool\n")
				src := v.genID("_src")
				out.WriteString(src + " := []" + goIntType(b) + "{")
				for i, val := range values {
					if i > 0 {
						out.WriteString(", ")
					}
					out.WriteString(v.integerValueText(val, b))
				}
				out.WriteString("}\n")
				out.WriteString("for _, src := range " + src + " {\n")
				out.WriteString("if " + ctx.tgt + " == src {\n")
				out.WriteString(exist + " = true\n")
				out.WriteString("break\n")
				out.WriteString("}\n")
				out.WriteString("}\n")
				out.WriteString("if " + exist + " == false {\n")
			} else {
				out.WriteString("if " + ctx.tgt + " != " + v.integerValueText(values[0], b) + " {\n")
			}
			v.failure(out, ctx, "vt.in")
		case "vt.not_in":
			if len(values) > 1 {
				src := v.genID("_src")
				out.WriteString(src + " := []" + goIntType(b) + "{")
				for i, val := range values {
					if i > 0 {
						out.WriteString(", ")
					}
					out.WriteString(v.integerValueText(val, b))
				}
				out.WriteString("}\n")
				out.WriteString("for _, src := range " + src + " {\n")
				out.WriteString("if " + ctx.tgt + " == src {\n")
			} else {
				out.WriteString("if " + ctx.tgt + " == " + v.integerValueText(values[0], b) + " {\n")
			}
			v.failure(out, ctx, "vt.not_in")
			if len(values) > 1 {
				out.WriteString("}\n")
			}
		}
	}
}

func (v *validatorGenerator) stringArg(val *validationValue) string {
	if val.isFieldReference() {
		return "string(" + v.fieldReferenceName(val.field) + ")"
	}
	return "\"" + val.strVal + "\""
}

func (v *validatorGenerator) generateStringFieldValidator(out *strings.Builder, ctx validatorContext, typ sema.Type) {
	target := ctx.tgt
	if typ.IsBinary() {
		target = v.genID("_tgt")
		out.WriteString(target + " := string(" + ctx.tgt + ")\n")
	}
	for _, rule := range ctx.rules {
		values := rule.values
		if len(values) == 0 {
			continue
		}
		key := rule.name
		switch key {
		case "vt.const":
			out.WriteString("if " + target + " != " + v.stringArg(values[0]))
		case "vt.min_size", "vt.max_size":
			out.WriteString("if len(" + target + ") ")
			if key == "vt.min_size" {
				out.WriteString("<")
			} else {
				out.WriteString(">")
			}
			out.WriteString(" int(")
			switch {
			case values[0].isFieldReference():
				out.WriteString(v.fieldReferenceName(values[0].field))
			case values[0].isValidationFunction():
				if values[0].function.name == "len" {
					out.WriteString("len(")
					if values[0].function.arguments[0].isFieldReference() {
						// The C++ code passes values[0] here, not the
						// function argument; it is a field reference only
						// when the rule value itself is one, so this prints
						// nothing for a function value.
						out.WriteString("string(")
						out.WriteString(v.fieldReferenceName(values[0].field))
						out.WriteString(")")
					}
					out.WriteString(")")
				}
			default:
				out.WriteString(itoa(values[0].intVal))
			}
			out.WriteString(")")
		case "vt.pattern":
			if values[0].isFieldReference() {
				out.WriteString("if ok, _ := regexp.MatchString(string(" + v.fieldReferenceName(values[0].field) + "), " + target + "); !ok")
			} else {
				pattern := values[0].strVal
				varName := ""
				for _, e := range v.patternCache {
					if e.pattern == pattern {
						varName = e.name
						break
					}
				}
				if varName == "" {
					varName = "vtRe" + v.structName + strconv.Itoa(len(v.patternCache))
					v.patternCache = append(v.patternCache, struct{ pattern, name string }{pattern, varName})
				}
				out.WriteString("if !" + varName + ".MatchString(" + target + ")")
			}
		case "vt.prefix":
			out.WriteString("if !strings.HasPrefix(" + target + ", " + v.stringArg(values[0]) + ")")
		case "vt.suffix":
			out.WriteString("if !strings.HasSuffix(" + target + ", " + v.stringArg(values[0]) + ")")
		case "vt.contains":
			out.WriteString("if !strings.Contains(" + target + ", " + v.stringArg(values[0]) + ")")
		case "vt.not_contains":
			out.WriteString("if strings.Contains(" + target + ", " + v.stringArg(values[0]) + ")")
		}
		out.WriteString(" {\n")
		v.failure(out, ctx, key)
	}
}

func (v *validatorGenerator) generateListFieldValidator(out *strings.Builder, ctx validatorContext, typ sema.Type) {
	for _, rule := range ctx.rules {
		values := rule.values
		key := rule.name
		switch key {
		case "vt.min_size", "vt.max_size":
			out.WriteString("if len(" + ctx.tgt + ")")
			if key == "vt.min_size" {
				out.WriteString(" < ")
			} else {
				out.WriteString(" > ")
			}
			if values[0].isFieldReference() {
				out.WriteString("int(" + v.fieldReferenceName(values[0].field) + ")")
			} else {
				out.WriteString(itoa(values[0].intVal))
			}
			out.WriteString(" {\n")
			v.failure(out, ctx, key)
		case "vt.elem":
			out.WriteString("for i := 0; i < len(" + ctx.tgt + "); i++ {\n")
			src := v.genID("_elem")
			out.WriteString(src + " := " + ctx.tgt + "[i]\n")
			var elemType sema.Type
			if typ.IsList() {
				elemType = typ.(*sema.List).ElemType()
			} else {
				elemType = typ.(*sema.Set).ElemType()
			}
			v.generateFieldValidator(out, validatorContext{
				fieldSymbol: ctx.fieldSymbol + ".elem",
				tgt:         src,
				typ:         elemType,
				rules:       []*validationRule{rule.inner},
			})
			out.WriteString("}\n")
		}
	}
}

func (v *validatorGenerator) generateMapFieldValidator(out *strings.Builder, ctx validatorContext, m *sema.Map) {
	for _, rule := range ctx.rules {
		values := rule.values
		key := rule.name
		switch key {
		case "vt.min_size", "vt.max_size":
			out.WriteString("if len(" + ctx.tgt + ")")
			if key == "vt.min_size" {
				out.WriteString(" < ")
			} else {
				out.WriteString(" > ")
			}
			if values[0].isFieldReference() {
				out.WriteString("int(" + v.fieldReferenceName(values[0].field) + ")")
			} else {
				out.WriteString(itoa(values[0].intVal))
			}
			out.WriteString(" {\n")
			v.failure(out, ctx, key)
		case "vt.key":
			src := v.genID("_key")
			if v.g.isContainerKeyedMap(ctx.typ) {
				entry := v.genID("_entry")
				out.WriteString("for _, " + entry + " := range " + ctx.tgt + " {\n")
				out.WriteString(src + " := " + entry + ".Key\n")
			} else {
				out.WriteString("for " + src + " := range " + ctx.tgt + " {\n")
			}
			v.generateFieldValidator(out, validatorContext{
				fieldSymbol: ctx.fieldSymbol + ".key",
				tgt:         src,
				typ:         m.KeyType(),
				rules:       []*validationRule{rule.inner},
			})
			out.WriteString("}\n")
		case "vt.value":
			src := v.genID("_value")
			if v.g.isContainerKeyedMap(ctx.typ) {
				entry := v.genID("_entry")
				out.WriteString("for _, " + entry + " := range " + ctx.tgt + " {\n")
				out.WriteString(src + " := " + entry + ".Value\n")
			} else {
				out.WriteString("for _, " + src + " := range " + ctx.tgt + " {\n")
			}
			v.generateFieldValidator(out, validatorContext{
				fieldSymbol: ctx.fieldSymbol + ".value",
				tgt:         src,
				typ:         m.ValType(),
				rules:       []*validationRule{rule.inner},
			})
			out.WriteString("}\n")
		}
	}
}

func (v *validatorGenerator) generateRegexpVars(out *strings.Builder) {
	if len(v.patternCache) == 0 {
		return
	}
	out.WriteString("// Precompiled regex patterns for " + v.structName + " vt.pattern validation\n")
	if len(v.patternCache) == 1 {
		out.WriteString("var " + v.patternCache[0].name + " = regexp.MustCompile(`" + v.patternCache[0].pattern + "`)\n\n")
		return
	}
	out.WriteString("var (\n")
	for _, e := range v.patternCache {
		out.WriteString(e.name + " = regexp.MustCompile(`" + e.pattern + "`)\n")
	}
	out.WriteString(")\n\n")
}

func (v *validatorGenerator) generateStructFieldValidator(out *strings.Builder, ctx validatorContext) {
	generateValid := true
	var lastValidRule *validationRule
	for _, rule := range ctx.rules {
		if len(rule.values) == 0 {
			continue
		}
		if rule.name == "vt.skip" {
			if rule.values[0].isFieldReference() || !rule.values[0].boolVal {
				generateValid = true
			} else if rule.values[0].boolVal {
				generateValid = false
			}
			lastValidRule = rule
		}
	}
	if !generateValid {
		return
	}
	if lastValidRule == nil {
		out.WriteString("if err := " + ctx.tgt + ".Validate(); err != nil {\n")
		out.WriteString("return err\n")
		out.WriteString("}\n")
		return
	}
	values := lastValidRule.values
	if !values[0].boolVal {
		out.WriteString("if err := " + ctx.tgt + ".Validate(); err != nil {\n")
		out.WriteString("return err\n")
		out.WriteString("}\n")
	} else if values[0].isFieldReference() {
		out.WriteString("if !" + v.fieldReferenceName(values[0].field) + " {\n")
		out.WriteString("if err := " + ctx.tgt + ".Validate(); err != nil {\n")
		out.WriteString("return err\n")
		out.WriteString("}\n")
		out.WriteString("}\n")
	}
}
