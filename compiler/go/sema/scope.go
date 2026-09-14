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

import "sort"

// Scope is t_scope: three flat name tables. Symbols of an included program
// are registered in the including program's scope under
// "<include name>.<symbol>".
type Scope struct {
	types     map[string]Type
	constants map[string]*Const
	services  map[string]*Service
}

// NewScope creates an empty scope.
func NewScope() *Scope {
	return &Scope{
		types:     map[string]Type{},
		constants: map[string]*Const{},
		services:  map[string]*Service{},
	}
}

// AddType registers or replaces a type.
func (s *Scope) AddType(name string, t Type) { s.types[name] = t }

// GetType returns the type or nil.
func (s *Scope) GetType(name string) Type { return s.types[name] }

// AddService registers or replaces a service.
func (s *Scope) AddService(name string, sv *Service) { s.services[name] = sv }

// GetService returns the service or nil.
func (s *Scope) GetService(name string) *Service { return s.services[name] }

// AddConstant registers a constant; a second registration of the same
// name is an error, as in t_scope::add_constant.
func (s *Scope) AddConstant(name string, c *Const) {
	if _, ok := s.constants[name]; ok {
		fail("Enum %s is already defined!", name)
	}
	s.constants[name] = c
}

// GetConstant returns the constant or nil.
func (s *Scope) GetConstant(name string) *Const { return s.constants[name] }

// ResolveAllConsts is t_scope::resolve_all_consts. It visits the constants
// in name order, as the std::map iteration does.
func (s *Scope) ResolveAllConsts() {
	names := make([]string, 0, len(s.constants))
	for n := range s.constants {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		c := s.constants[n]
		s.ResolveConstValue(c.value, c.typ)
	}
}

// ResolveConstValue is t_scope::resolve_const_value. It binds identifier
// values to enums or copies the value of the named constant, and maps
// integer values of enum type back to the member name.
func (s *Scope) ResolveConstValue(cv *ConstValue, ttype Type) {
	for ttype.IsTypedef() {
		ttype = ttype.(*Typedef).Type()
	}
	switch {
	case ttype.IsMap():
		m := ttype.(*Map)
		for _, e := range cv.Map() {
			s.ResolveConstValue(e.Key, m.KeyType())
			s.ResolveConstValue(e.Value, m.ValType())
		}
	case ttype.IsList():
		l := ttype.(*List)
		for _, v := range cv.List() {
			s.ResolveConstValue(v, l.ElemType())
		}
	case ttype.IsSet():
		st := ttype.(*Set)
		for _, v := range cv.List() {
			s.ResolveConstValue(v, st.ElemType())
		}
	case ttype.IsStruct():
		ts := ttype.(*Struct)
		for _, e := range cv.Map() {
			f := ts.FieldByName(e.Key.String())
			if f == nil {
				fail("No field named \"%s\" was found in struct of type \"%s\"", e.Key.String(), ts.Name())
			}
			s.ResolveConstValue(e.Value, f.Type())
		}
	case cv.Kind() == CVIdentifier:
		if ttype.IsEnum() {
			cv.SetEnum(ttype.(*Enum))
			return
		}
		constant := s.GetConstant(cv.Identifier())
		if constant == nil {
			fail("No enum value or constant found named \"%s\"!", cv.Identifier())
		}
		constType := TrueType(constant.Type())
		switch {
		case constType.IsBaseType():
			switch constType.(*BaseType).Base() {
			case TypeI16, TypeI32, TypeI64, TypeBool, TypeI8:
				cv.SetInteger(constant.Value().Integer())
			case TypeString:
				cv.SetString(constant.Value().String())
			case TypeUUID:
				cv.SetUUID(constant.Value().UUID())
			case TypeDouble:
				cv.SetDouble(constant.Value().Double())
			case TypeVoid:
				fail("Constants cannot be of type VOID")
			}
		case constType.IsMap():
			cv.SetMap()
			for _, e := range constant.Value().Map() {
				cv.AddMap(e.Key, e.Value)
			}
		case constType.IsList():
			cv.SetList()
			for _, v := range constant.Value().List() {
				cv.AddList(v)
			}
		}
	case ttype.IsEnum():
		// enum constant with non-identifier value. set the enum and find
		// the value's name.
		te := ttype.(*Enum)
		ev := te.ConstantByValue(cv.Integer())
		if ev == nil {
			fail("Couldn't find a named value in enum %s for value %d", te.Name(), cv.Integer())
		}
		cv.SetIdentifier(te.Name() + "." + ev.Name())
		cv.SetEnum(te)
	}
}
