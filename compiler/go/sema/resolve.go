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

// resolver is the state of t_program::resolve_types in parse.cc. path holds
// the types on the way to the current one, so that a typedef reached again
// through itself is reported instead: a plain cycle would send TrueType
// round forever, and one through a container would send a generator
// recursing into the element type until the stack overflows. done holds
// the types already walked in full.
type resolver struct {
	path map[Type]bool
	done map[Type]bool
}

// ResolveTypes is t_program::resolve_types: it resolves every forward
// typedef the program's types refer to, so that an undeclared target or a
// typedef that refers to itself fails here rather than inside a generator.
func (p *Program) ResolveTypes() {
	r := &resolver{path: map[Type]bool{}, done: map[Type]bool{}}
	for _, td := range p.typedefs {
		r.resolveType(td)
	}
	for _, c := range p.consts {
		r.resolveType(c.Type())
	}
	for _, s := range p.objects {
		r.resolveStruct(s)
	}
	for _, svc := range p.services {
		for _, f := range svc.Functions() {
			r.resolveType(f.ReturnType())
			r.resolveStruct(f.Arglist())
			r.resolveStruct(f.Xceptions())
		}
	}
}

func (r *resolver) resolveType(t Type) {
	if r.done[t] {
		return
	}
	if r.path[t] {
		fail("Type \"%s\" refers to itself", t.Name())
	}
	r.path[t] = true
	switch v := t.(type) {
	case *Typedef:
		r.resolveType(v.Type())
	case *List:
		r.resolveType(v.ElemType())
	case *Set:
		r.resolveType(v.ElemType())
	case *Map:
		r.resolveType(v.KeyType())
		r.resolveType(v.ValType())
	}
	delete(r.path, t)
	r.done[t] = true
}

func (r *resolver) resolveStruct(s *Struct) {
	for _, f := range s.Members() {
		r.resolveType(f.Type())
		// The fields of an xsd_attrs block belong to an anonymous struct
		// that is not one of the program's objects.
		if f.XsdAttrs() != nil {
			r.resolveStruct(f.XsdAttrs())
		}
	}
}
