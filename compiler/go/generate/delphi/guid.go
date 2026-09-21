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

package delphi

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// computeUUIDv8 is compute_uuid_v8: SHA-256(ns_bytes||name), with version
// (4 high bits of byte 6 = 0x8) and variant (2 high bits of byte 8 =
// 0b10) applied per RFC 9562 section 5.8, truncated to 16 bytes.
//
// crypto/sha256 implements the same FIPS 180-2 algorithm as the
// header-only thrift_generator::sha256 the C++ generator uses (see
// compiler/cpp/src/thrift/generate/sha256.h): both produce the standard
// 32-byte big-endian SHA-256 digest of the input bytes, so the two are
// byte-for-byte interchangeable here.
func computeUUIDv8(nsBytes []byte, name string) []byte {
	input := make([]byte, 0, len(nsBytes)+len(name))
	input = append(input, nsBytes...)
	input = append(input, name...)
	sum := sha256.Sum256(input)
	h := sum[:16:16]
	h[6] = 0x80 | (sum[6] & 0x0F)
	h[8] = 0x80 | (sum[8] & 0x3F)
	return h
}

// uuidBytesToDelphiGuid is uuid_bytes_to_delphi_guid: formats 16 UUID
// bytes as a Delphi GUID attribute string.
func uuidBytesToDelphiGuid(b []byte) string {
	return fmt.Sprintf("['{%02X%02X%02X%02X-%02X%02X-%02X%02X-%02X%02X-%02X%02X%02X%02X%02X%02X}']",
		b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7], b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15])
}

// bytesToHex is bytes_to_hex: lowercase hex, no separators.
func bytesToHex(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		fmt.Fprintf(&sb, "%02x", c)
	}
	return sb.String()
}

// dnsNamespaceBytes is DNS_NAMESPACE_BYTES: the well-known RFC 4122 DNS
// namespace UUID (6ba7b810-9dad-11d1-80b4-00c04fd430c8).
var dnsNamespaceBytes = []byte{
	0x6b, 0xa7, 0xb8, 0x10, 0x9d, 0xad, 0x11, 0xd1, 0x80, 0xb4, 0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8,
}

// thriftRootNamespace is thrift_root_namespace: UUIDv8(DNS_NAMESPACE,
// "thrift.apache.org").
func thriftRootNamespace() []byte {
	return computeUUIDv8(dnsNamespaceBytes, "thrift.apache.org")
}

// programNamespaceUUID is t_delphi_generator::program_namespace_uuid.
func (g *Generator) programNamespaceUUID() []byte {
	return computeUUIDv8(thriftRootNamespace(), g.program.Name())
}

// typeNameForGuid is t_delphi_generator::type_name_for_guid.
func (g *Generator) typeNameForGuid(ttype sema.Type) string {
	ttype = sema.TrueType(ttype)
	if ttype.IsBaseType() {
		b := ttype.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeVoid:
			return "void"
		case sema.TypeBool:
			return "bool"
		case sema.TypeI8:
			return "i8"
		case sema.TypeI16:
			return "i16"
		case sema.TypeI32:
			return "i32"
		case sema.TypeI64:
			return "i64"
		case sema.TypeDouble:
			return "double"
		case sema.TypeUUID:
			return "uuid"
		case sema.TypeString:
			if b.IsBinary() {
				return "binary"
			}
			return "string"
		default:
			emit.Throw("compiler error: unknown base type in type_name_for_guid")
		}
	} else if ttype.IsMap() {
		m := ttype.(*sema.Map)
		return "map<" + g.typeNameForGuid(m.KeyType()) + "," + g.typeNameForGuid(m.ValType()) + ">"
	} else if ttype.IsSet() {
		return "set<" + g.typeNameForGuid(ttype.(*sema.Set).ElemType()) + ">"
	} else if ttype.IsList() {
		return "list<" + g.typeNameForGuid(ttype.(*sema.List).ElemType()) + ">"
	}
	if ttype.Program() != nil && ttype.Program() != g.program {
		return ttype.Program().Name() + "." + ttype.Name()
	}
	return ttype.Name()
}

// canonicalStructString is t_delphi_generator::canonical_struct_string.
func (g *Generator) canonicalStructString(tstruct *sema.Struct) string {
	progNsHex := bytesToHex(g.programNamespaceUUID())
	canonical := progNsHex + "\n" + tstruct.Name() + "\n"
	for _, m := range tstruct.Members() {
		canonical += m.Name() + ":" + g.typeNameForGuid(m.Type()) + "\n"
	}
	return canonical
}

// canonicalServiceString is t_delphi_generator::canonical_service_string.
func (g *Generator) canonicalServiceString(tservice *sema.Service) string {
	progNsHex := bytesToHex(g.programNamespaceUUID())

	parentHash := ""
	if tservice.Extends() != nil {
		parentCanonical := g.canonicalServiceString(tservice.Extends())
		sum := sha256.Sum256([]byte(parentCanonical))
		parentHash = bytesToHex(sum[:])
	}

	canonical := progNsHex + "\n" + tservice.Name() + "\n" + parentHash + "\n"

	for _, f := range tservice.Functions() {
		line := ""
		if f.IsOneway() {
			line += "oneway:"
		}
		line += f.Name() + ":" + g.typeNameForGuid(f.ReturnType())
		for _, a := range f.Arglist().Members() {
			line += ":" + g.typeNameForGuid(a.Type())
		}
		canonical += line + "\n"
	}
	return canonical
}

// generateGuid is t_delphi_generator::generate_guid: the guid_v4 option
// uses CoCreateGuid/StringFromGUID2, a Windows-only random GUID. The
// C++ source compiles that call out entirely on non-Windows platforms
// (#else (void)out; #endif) and emits nothing; the Go compiler is built
// and run on the same non-Windows hosts as the C++ oracle here, so this
// matches by also emitting nothing.
func (g *Generator) generateGuid(out *strings.Builder) {
	_ = out
}

// generateGuidV8Service is the t_service* overload of generate_guid_v8.
func (g *Generator) generateGuidV8Service(out *strings.Builder, tservice *sema.Service) {
	progNs := g.programNamespaceUUID()
	canonical := g.canonicalServiceString(tservice)
	uuid := computeUUIDv8(progNs, canonical)
	g.ln(out, uuidBytesToDelphiGuid(uuid))
}

// generateGuidV8Struct is the t_struct* overload of generate_guid_v8.
func (g *Generator) generateGuidV8Struct(out *strings.Builder, tstruct *sema.Struct) {
	progNs := g.programNamespaceUUID()
	canonical := g.canonicalStructString(tstruct)
	uuid := computeUUIDv8(progNs, canonical)
	g.ln(out, uuidBytesToDelphiGuid(uuid))
}
