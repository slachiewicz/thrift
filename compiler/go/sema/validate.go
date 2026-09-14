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

package sema

import "strings"

// validateSimpleIdentifier is validate_simple_identifier in main.cc.
func validateSimpleIdentifier(id string) {
	if strings.Contains(id, ".") {
		fail("Identifier %s can't have a dot.", id)
	}
}

// validateConstRec is validate_const_rec in main.cc: the value tree must
// match the declared type.
func validateConstRec(name string, typ Type, value *ConstValue) {
	typ = TrueType(typ)
	if typ.IsVoid() {
		fail("type error: cannot declare a void const: %s", name)
	}
	switch {
	case typ.IsBaseType():
		switch typ.(*BaseType).Base() {
		case TypeString:
			if value.Kind() != CVString {
				fail("type error: const \"%s\" was declared as string", name)
			}
		case TypeUUID:
			if value.Kind() != CVString {
				fail("type error: const \"%s\" was declared as uuid", name)
			}
			value.SetUUID(value.UUID())
		case TypeBool:
			if value.Kind() != CVInteger {
				fail("type error: const \"%s\" was declared as bool", name)
			}
		case TypeI8:
			if value.Kind() != CVInteger {
				fail("type error: const \"%s\" was declared as byte", name)
			}
		case TypeI16:
			if value.Kind() != CVInteger {
				fail("type error: const \"%s\" was declared as i16", name)
			}
		case TypeI32:
			if value.Kind() != CVInteger {
				fail("type error: const \"%s\" was declared as i32", name)
			}
		case TypeI64:
			if value.Kind() != CVInteger {
				fail("type error: const \"%s\" was declared as i64", name)
			}
		case TypeDouble:
			if value.Kind() != CVInteger && value.Kind() != CVDouble {
				fail("type error: const \"%s\" was declared as double", name)
			}
		default:
			fail("compiler error: no const of base type %s%s", BaseName(typ.(*BaseType).Base()), name)
		}
	case typ.IsEnum():
		if value.Kind() != CVIdentifier {
			fail("type error: const \"%s\" was declared as enum", name)
		}
		namePortion := value.IdentifierName()
		if typ.(*Enum).ConstantByName(namePortion) == nil {
			fail("type error: const %s was declared as type %s which is an enum, but %s is not a valid value for that enum",
				name, typ.Name(), value.Identifier())
		}
	case typ.IsStruct() || typ.IsXception():
		if value.Kind() != CVMap {
			fail("type error: const \"%s\" was declared as struct/xception", name)
		}
		fields := typ.(*Struct).Members()
		for _, e := range value.Map() {
			if e.Key.Kind() != CVString {
				fail("type error: %s struct key must be string", name)
			}
			var fieldType Type
			for _, f := range fields {
				if f.Name() == e.Key.String() {
					fieldType = f.Type()
				}
			}
			if fieldType == nil {
				fail("type error: %s has no field %s", typ.Name(), e.Key.String())
			}
			validateConstRec(name+"."+e.Key.String(), fieldType, e.Value)
		}
	case typ.IsMap():
		m := typ.(*Map)
		for _, e := range value.Map() {
			validateConstRec(name+"<key>", m.KeyType(), e.Key)
			validateConstRec(name+"<val>", m.ValType(), e.Value)
		}
	case typ.IsList() || typ.IsSet():
		var elem Type
		if typ.IsList() {
			elem = typ.(*List).ElemType()
		} else {
			elem = typ.(*Set).ElemType()
		}
		for _, v := range value.List() {
			validateConstRec(name+"<elem>", elem, v)
		}
	}
}

func validateConstType(c *Const) {
	validateConstRec(c.Name(), c.Type(), c.Value())
}

func validateFieldValue(f *Field, cv *ConstValue) {
	validateConstRec(f.Name(), f.Type(), cv)
}

// validateThrows is validate_throws: every member must be an exception.
func validateThrows(throws *Struct) bool {
	for _, f := range throws.Members() {
		if !TrueType(f.Type()).IsXception() {
			return false
		}
	}
	return true
}

// ValidateInput is t_generator::validate_input for a generator with no
// reserved words: it runs the Validate method of every element before
// code generation.
func ValidateInput(p *Program) {
	for _, e := range p.Enums() {
		e.Validate()
		for _, v := range e.Constants() {
			v.Validate()
		}
	}
	for _, t := range p.Typedefs() {
		t.Validate()
	}
	for _, s := range p.Objects() {
		validateStruct(s)
	}
	for _, c := range p.Consts() {
		c.Validate()
	}
	for _, s := range p.Services() {
		s.Validate()
		for _, f := range s.Functions() {
			f.Validate()
			validateStruct(f.Arglist())
			validateStruct(f.Xceptions())
		}
	}
}

func validateStruct(s *Struct) {
	s.Validate()
	for _, f := range s.Members() {
		f.Validate()
	}
}
