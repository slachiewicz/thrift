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

// Package audit is the -audit mode of the Thrift compiler, t_audit.cpp: it
// compares a new IDL file against an old one and reports the changes that
// break wire compatibility as failures and the rest as warnings.
package audit

import (
	"fmt"
	"io"

	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the -audit-* flags.
type Options struct {
	// AllowOptionalFieldRemoval is -audit-allow-optional-field-removal.
	AllowOptionalFieldRemoval bool
	// AllowRequiredFieldToDefault is -audit-allow-required-field-to-default.
	AllowRequiredFieldToDefault bool
	// WarnLevel is g_warn: warnings above it are dropped.
	WarnLevel int
	// Stdout and Stderr receive the warnings and the failures, on the same
	// streams the C++ compiler uses.
	Stdout, Stderr io.Writer
}

// Auditor compares two programs. Failed reports whether any failure was
// found, which makes the compiler exit with status 2.
type Auditor struct {
	opts Options
	// path is g_curpath: the new file, the last one parsed.
	path   string
	Failed bool
}

// Audit compares newProgram against oldProgram and returns whether the
// comparison found a failure.
func Audit(newProgram, oldProgram *sema.Program, opts Options) bool {
	a := &Auditor{opts: opts, path: newProgram.Path()}
	if a.opts.Stdout == nil {
		a.opts.Stdout = io.Discard
	}
	if a.opts.Stderr == nil {
		a.opts.Stderr = io.Discard
	}
	a.compareNamespaces(newProgram, oldProgram)
	a.compareServices(newProgram.Services(), oldProgram.Services())
	a.compareEnums(newProgram.Enums(), oldProgram.Enums())
	a.compareStructs(newProgram.Structs(), oldProgram.Structs())
	a.compareStructs(newProgram.Xceptions(), oldProgram.Xceptions())
	a.compareConsts(newProgram.Consts(), oldProgram.Consts())
	return a.Failed
}

func (a *Auditor) warning(level int, format string, args ...interface{}) {
	if a.opts.WarnLevel < level {
		return
	}
	fmt.Fprintf(a.opts.Stdout, "[Thrift Audit Warning:%s] "+format+"\n", append([]interface{}{a.path}, args...)...)
}

func (a *Auditor) failure(format string, args ...interface{}) {
	fmt.Fprintf(a.opts.Stderr, "[Thrift Audit Failure:%s] "+format+"\n", append([]interface{}{a.path}, args...)...)
	a.Failed = true
}

func (a *Auditor) isAllowedFieldRemoval(oldField *sema.Field) bool {
	return a.opts.AllowOptionalFieldRemoval && oldField.Req() == sema.Optional
}

func (a *Auditor) reportFieldRemoval(oldField *sema.Field, structName string) {
	if !a.isAllowedFieldRemoval(oldField) {
		a.failure("Struct Field removed for Id = %d in %s \n", oldField.Key(), structName)
	}
}

func (a *Auditor) compareNamespaces(newProgram, oldProgram *sema.Program) {
	newNamespaces := newProgram.Namespaces()
	for _, language := range oldProgram.NamespaceKeys() {
		oldNamespace := oldProgram.Namespaces()[language]
		if newNamespace, ok := newNamespaces[language]; !ok {
			a.warning(1, "Language %s not found in new thrift file\n", language)
		} else if newNamespace != oldNamespace {
			a.warning(1, "Namespace %s changed in new thrift file\n", oldNamespace)
		}
	}
}

func (a *Auditor) compareEnumValues(newEnum, oldEnum *sema.Enum) {
	for _, oldValue := range oldEnum.Constants() {
		value := oldValue.Value()
		if newValue := newEnum.ConstantByValue(int64(value)); newValue != nil {
			if oldValue.Name() != newValue.Name() {
				a.warning(1, "Name of the value %d changed in enum %s\n", value, oldEnum.Name())
			}
		} else {
			a.failure("Enum value %d missing in %s\n", value, oldEnum.Name())
		}
	}
}

func (a *Auditor) compareEnums(newEnums, oldEnums []*sema.Enum) {
	byName := map[string]*sema.Enum{}
	for _, e := range newEnums {
		byName[e.Name()] = e
	}
	for _, oldEnum := range oldEnums {
		if newEnum, ok := byName[oldEnum.Name()]; !ok {
			a.warning(1, "Enum %s not found in new thrift file\n", oldEnum.Name())
		} else {
			a.compareEnumValues(newEnum, oldEnum)
		}
	}
}

// compareType reports whether two types are the same. Names decide for
// base types, structs and enums; containers have no name, so their element
// types are compared instead.
func compareType(newType, oldType sema.Type) bool {
	if newType.Name() == "" && oldType.Name() == "" {
		switch {
		case newType.IsList() && oldType.IsList():
			return compareType(newType.(*sema.List).ElemType(), oldType.(*sema.List).ElemType())
		case newType.IsMap() && oldType.IsMap():
			n, o := newType.(*sema.Map), oldType.(*sema.Map)
			return compareType(n.KeyType(), o.KeyType()) && compareType(n.ValType(), o.ValType())
		case newType.IsSet() && oldType.IsSet():
			return compareType(newType.(*sema.Set).ElemType(), oldType.(*sema.Set).ElemType())
		}
		return false
	}
	return newType.Name() == oldType.Name()
}

// compareDefaults reports whether two default values are the same.
func compareDefaults(newValue, oldValue *sema.ConstValue) bool {
	if newValue == nil || oldValue == nil {
		return newValue == nil && oldValue == nil
	}
	if newValue.Kind() != oldValue.Kind() {
		return false
	}
	switch newValue.Kind() {
	case sema.CVInteger:
		return newValue.Integer() == oldValue.Integer()
	case sema.CVDouble:
		return newValue.Double() == oldValue.Double()
	case sema.CVString:
		return newValue.String() == oldValue.String()
	case sema.CVList:
		newList, oldList := newValue.List(), oldValue.List()
		if len(newList) != len(oldList) {
			return false
		}
		for i := range newList {
			if !compareDefaults(newList[i], oldList[i]) {
				return false
			}
		}
		return true
	case sema.CVMap:
		newMap, oldMap := newValue.Map(), oldValue.Map()
		if len(newMap) != len(oldMap) {
			return false
		}
		for i := range newMap {
			if !compareDefaults(newMap[i].Key, oldMap[i].Key) || !compareDefaults(newMap[i].Value, oldMap[i].Value) {
				return false
			}
		}
		return true
	case sema.CVIdentifier:
		return newValue.Identifier() == oldValue.Identifier()
	}
	return false
}

func (a *Auditor) compareStructField(newField, oldField *sema.Field, oldStructName string) {
	if !compareType(newField.Type(), oldField.Type()) {
		a.failure("Struct Field Type Changed for Id = %d in %s \n", newField.Key(), oldStructName)
	}

	// A struct member is optional when declared so or when it has a
	// default value.
	newOptional := newField.Req() != sema.Required
	oldOptional := oldField.Req() != sema.Required

	requiredToDefaultAllowed := a.opts.AllowRequiredFieldToDefault &&
		oldField.Req() == sema.Required && newField.Req() == sema.OptInReqOut

	if newOptional != oldOptional && !requiredToDefaultAllowed {
		a.failure("Struct Field Requiredness Changed for Id = %d in %s \n", newField.Key(), oldStructName)
	}
	if newOptional || oldOptional {
		if !compareDefaults(newField.Value(), oldField.Value()) {
			a.warning(1, "Default value changed for Id = %d in %s \n", newField.Key(), oldStructName)
		}
	}
	if newField.Name() != oldField.Name() {
		a.warning(1, "Struct field name changed for Id = %d in %s\n", newField.Key(), oldStructName)
	}
}

// compareSingleStruct walks both member lists in id order together, so
// the ids alone tell which fields were added, removed or kept.
func (a *Auditor) compareSingleStruct(newStruct, oldStruct *sema.Struct, oldStructName string) {
	structName := oldStructName
	if structName == "" {
		structName = oldStruct.Name()
	}
	oldMembers := oldStruct.SortedMembers()
	newMembers := newStruct.SortedMembers()
	o, n := 0, 0
	for o < len(oldMembers) || n < len(newMembers) {
		switch {
		case n == len(newMembers):
			// A field id has been removed from the end.
			a.reportFieldRemoval(oldMembers[o], structName)
			o++
		case o == len(oldMembers):
			// A new field id has been added to the end.
			if newMembers[n].Req() == sema.Required {
				a.failure("Required Struct Field Added for Id = %d in %s \n", newMembers[n].Key(), structName)
			}
			n++
		case newMembers[n].Key() == oldMembers[o].Key():
			a.compareStructField(newMembers[n], oldMembers[o], structName)
			n++
			o++
		case newMembers[n].Key() < oldMembers[o].Key():
			// Adding fields is fine, but adding them in the middle is
			// suspicious.
			a.failure("Struct field is added in the middle with Id = %d in %s\n", newMembers[n].Key(), structName)
			n++
		default:
			// A field is deleted in the new struct.
			a.reportFieldRemoval(oldMembers[o], structName)
			o++
		}
	}
}

func (a *Auditor) compareStructs(newStructs, oldStructs []*sema.Struct) {
	byName := map[string]*sema.Struct{}
	for _, s := range newStructs {
		byName[s.Name()] = s
	}
	for _, oldStruct := range oldStructs {
		if newStruct, ok := byName[oldStruct.Name()]; !ok {
			a.failure("Struct %s not found in new thrift file\n", oldStruct.Name())
		} else {
			a.compareSingleStruct(newStruct, oldStruct, "")
		}
	}
}

func (a *Auditor) compareSingleFunction(newFunction, oldFunction *sema.Function) {
	if newFunction.IsOneway() != oldFunction.IsOneway() {
		a.failure("Oneway attribute changed for function %s\n", oldFunction.Name())
	}
	if !compareType(newFunction.ReturnType(), oldFunction.ReturnType()) {
		a.failure("Return type changed for function %s\n", oldFunction.Name())
	}
	a.compareSingleStruct(newFunction.Arglist(), oldFunction.Arglist(), "")
	a.compareSingleStruct(newFunction.Xceptions(), oldFunction.Xceptions(), oldFunction.Name()+"_exception")
}

func (a *Auditor) compareFunctions(newFunctions, oldFunctions []*sema.Function) {
	byName := map[string]*sema.Function{}
	for _, f := range newFunctions {
		byName[f.Name()] = f
	}
	for _, oldFunction := range oldFunctions {
		if newFunction, ok := byName[oldFunction.Name()]; !ok {
			a.failure("New Thrift File has missing function %s\n", oldFunction.Name())
		} else {
			a.compareSingleFunction(newFunction, oldFunction)
		}
	}
}

func (a *Auditor) compareServices(newServices, oldServices []*sema.Service) {
	byName := map[string]*sema.Service{}
	for _, s := range newServices {
		byName[s.Name()] = s
	}
	for _, oldService := range oldServices {
		newService, ok := byName[oldService.Name()]
		if !ok {
			a.failure("New Thrift file is missing a service %s\n", oldService.Name())
			continue
		}
		oldExtends, newExtends := oldService.Extends(), newService.Extends()
		switch {
		case oldExtends == nil:
			// Adding extends is fine.
		case newExtends == nil:
			a.failure("Change in Service inheritance for %s\n", oldService.Name())
		case newExtends.Name() != oldExtends.Name():
			a.failure("Change in Service inheritance for %s\n", oldService.Name())
		}
		a.compareFunctions(newService.Functions(), oldService.Functions())
	}
}

func (a *Auditor) compareConsts(newConsts, oldConsts []*sema.Const) {
	byName := map[string]*sema.Const{}
	for _, c := range newConsts {
		byName[c.Name()] = c
	}
	for _, oldConst := range oldConsts {
		newConst, ok := byName[oldConst.Name()]
		switch {
		case !ok:
			a.warning(1, "Constants Missing %s \n", oldConst.Name())
		case !compareType(newConst.Type(), oldConst.Type()):
			a.warning(1, "Constant %s is of different type \n", oldConst.Name())
		case !compareDefaults(newConst.Value(), oldConst.Value()):
			a.warning(1, "Constant %s has different value\n", oldConst.Name())
		}
	}
}
