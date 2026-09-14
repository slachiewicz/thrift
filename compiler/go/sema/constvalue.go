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

// ConstValueKind is t_const_value::t_const_value_type.
type ConstValueKind int

// Constant value kinds, in the C++ enum order. The order matters: it is
// the primary key when map constants are sorted.
const (
	CVInteger ConstValueKind = iota
	CVDouble
	CVString
	CVMap
	CVList
	CVIdentifier
	CVUnknown
)

// MapEntry is one key/value pair of a map constant.
type MapEntry struct {
	Key, Value *ConstValue
}

// ConstValue is t_const_value. Map entries are kept sorted by key with the
// same ordering the C++ std::map uses, and a key inserted twice replaces
// the earlier value.
type ConstValue struct {
	mapVal        []MapEntry
	listVal       []*ConstValue
	stringVal     string
	intVal        int64
	doubleVal     float64
	identifierVal string
	enum          *Enum
	valType       ConstValueKind
}

// NewConstValue creates a value of unknown kind.
func NewConstValue() *ConstValue { return &ConstValue{valType: CVUnknown} }

// NewIntConstValue creates an integer value.
func NewIntConstValue(v int64) *ConstValue {
	c := NewConstValue()
	c.SetInteger(v)
	return c
}

// NewStringConstValue creates a string value.
func NewStringConstValue(v string) *ConstValue {
	c := NewConstValue()
	c.SetString(v)
	return c
}

func (c *ConstValue) SetString(v string) {
	c.valType = CVString
	c.stringVal = v
}

func (c *ConstValue) String() string { return c.stringVal }

func (c *ConstValue) SetInteger(v int64) {
	c.valType = CVInteger
	c.intVal = v
}

// Integer is t_const_value::get_integer: for an identifier bound to an
// enum it returns the value of the named member.
func (c *ConstValue) Integer() int64 {
	if c.valType == CVIdentifier {
		if c.enum == nil {
			fail("have identifier \"%s\", but unset enum on line!", c.identifierVal)
		}
		identifier := c.identifierVal
		if dot := strings.LastIndexByte(identifier, '.'); dot >= 0 {
			identifier = identifier[dot+1:]
		}
		v := c.enum.ConstantByName(identifier)
		if v == nil {
			fail("Unable to find enum value \"%s\" in enum \"%s\"", identifier, c.enum.Name())
		}
		return int64(v.Value())
	}
	return c.intVal
}

// SetUUID validates and stores a UUID string.
func (c *ConstValue) SetUUID(v string) {
	v = validateUUID(v)
	c.valType = CVString
	c.stringVal = v
}

// UUID returns the value validated as a UUID.
func (c *ConstValue) UUID() string {
	return validateUUID(c.stringVal)
}

func (c *ConstValue) SetDouble(v float64) {
	c.valType = CVDouble
	c.doubleVal = v
}

func (c *ConstValue) Double() float64 { return c.doubleVal }

func (c *ConstValue) SetMap() { c.valType = CVMap }

// AddMap inserts an entry keeping the entries sorted by key.
func (c *ConstValue) AddMap(key, val *ConstValue) {
	lo, hi := 0, len(c.mapVal)
	for lo < hi {
		mid := (lo + hi) / 2
		if c.mapVal[mid].Key.Less(key) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(c.mapVal) && !key.Less(c.mapVal[lo].Key) {
		c.mapVal[lo].Value = val
		return
	}
	c.mapVal = append(c.mapVal, MapEntry{})
	copy(c.mapVal[lo+1:], c.mapVal[lo:])
	c.mapVal[lo] = MapEntry{Key: key, Value: val}
}

// Map returns the entries in key order.
func (c *ConstValue) Map() []MapEntry { return c.mapVal }

func (c *ConstValue) SetList() { c.valType = CVList }

func (c *ConstValue) AddList(v *ConstValue) { c.listVal = append(c.listVal, v) }

// List returns the elements in source order.
func (c *ConstValue) List() []*ConstValue { return c.listVal }

func (c *ConstValue) SetIdentifier(v string) {
	c.valType = CVIdentifier
	c.identifierVal = v
}

func (c *ConstValue) Identifier() string { return c.identifierVal }

// IdentifierName is get_identifier_name: the identifier with its first
// one or two dotted prefixes removed.
func (c *ConstValue) IdentifierName() string {
	ret := c.identifierVal
	s := strings.IndexByte(ret, '.')
	if s < 0 {
		fail("error: identifier %s is unqualified!", ret)
	}
	ret = ret[s+1:]
	if s = strings.IndexByte(ret, '.'); s >= 0 {
		ret = ret[s+1:]
	}
	return ret
}

// IdentifierWithParent is get_identifier_with_parent: the identifier with
// its include prefix removed when it has one.
func (c *ConstValue) IdentifierWithParent() string {
	ret := c.identifierVal
	s := strings.IndexByte(ret, '.')
	if s < 0 {
		fail("error: identifier %s is unqualified!", ret)
	}
	if s2 := strings.IndexByte(ret[s+1:], '.'); s2 >= 0 {
		ret = ret[s+1:]
	}
	return ret
}

func (c *ConstValue) SetEnum(e *Enum) { c.enum = e }

// Enum returns the enum an identifier value was bound to, or nil.
func (c *ConstValue) Enum() *Enum { return c.enum }

// Kind is get_type. It fails on a value that was never set.
func (c *ConstValue) Kind() ConstValueKind {
	if c.valType == CVUnknown {
		fail("unknown t_const_value")
	}
	return c.valType
}

// Less is t_const_value::operator<.
func (c *ConstValue) Less(o *ConstValue) bool {
	t1, t2 := c.Kind(), o.Kind()
	if t1 != t2 {
		return t1 < t2
	}
	switch t1 {
	case CVInteger:
		return c.intVal < o.intVal
	case CVDouble:
		return c.doubleVal < o.doubleVal
	case CVString:
		return c.stringVal < o.stringVal
	case CVIdentifier:
		return c.identifierVal < o.identifierVal
	case CVMap:
		return lexicographicalMap(c.mapVal, o.mapVal)
	case CVList:
		return lexicographicalList(c.listVal, o.listVal)
	}
	fail("unknown value type")
	return false
}

func lexicographicalList(a, b []*ConstValue) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i].Less(b[i]) {
			return true
		}
		if b[i].Less(a[i]) {
			return false
		}
	}
	return len(a) < len(b)
}

func mapEntryLess(l, r MapEntry) bool {
	if l.Key.Less(r.Key) {
		return true
	}
	if r.Key.Less(l.Key) {
		return false
	}
	return l.Value.Less(r.Value)
}

func lexicographicalMap(a, b []MapEntry) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if mapEntryLess(a[i], b[i]) {
			return true
		}
		if mapEntryLess(b[i], a[i]) {
			return false
		}
	}
	return len(a) < len(b)
}

// validateUUID accepts the canonical 36-character form and the braced
// Windows GUID form, returning the canonical form.
func validateUUID(uuid string) string {
	if len(uuid) == 38 && uuid[0] == '{' && uuid[37] == '}' {
		uuid = uuid[1:37]
	}
	valid := len(uuid) == 36
	for i := 0; valid && i < len(uuid); i++ {
		switch i {
		case 8, 13, 18, 23:
			if uuid[i] != '-' {
				valid = false
			}
		default:
			if strings.IndexByte("0123456789ABCDEFabcdef", uuid[i]) < 0 {
				valid = false
			}
		}
	}
	if !valid {
		fail("invalid uuid %s", uuid)
	}
	return uuid
}
