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

// Package decode reads Thrift-encoded bytes without a schema: the binary,
// compact and JSON protocols carry a type tag and an id for every field
// and a length for every string and container, which is enough to walk a
// value and print it as a tree of ids, types and values. Names, enums,
// unions and requiredness live only in the IDL and are not recovered.
package decode

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/apache/thrift/lib/go/thrift"
)

// Protocol names a wire protocol, or Auto to detect one.
type Protocol string

const (
	Auto    Protocol = "auto"
	Binary  Protocol = "binary"
	Compact Protocol = "compact"
	JSON    Protocol = "json"
)

// Tristate is a yes, no or auto-detect setting.
type Tristate string

const (
	AutoDetect Tristate = "auto"
	Yes        Tristate = "yes"
	No         Tristate = "no"
)

// Options control one decode.
type Options struct {
	Protocol Protocol
	// Framed says whether each value is prefixed by a 4-byte big-endian
	// length, as TFramedTransport writes.
	Framed Tristate
	// Message says whether the value starts with a message header (name,
	// type, sequence id) rather than being a bare struct.
	Message Tristate
	// Config bounds the decode; nil uses the runtime defaults.
	Config *thrift.TConfiguration
}

// Value is one decoded value. Exactly the fields for its Type are set.
type Value struct {
	Type thrift.TType
	// Annotation is what the IDL adds, when one was given.
	Annotation Annotation
	Bool       bool
	// Int holds BYTE, I16, I32 and I64.
	Int    int64
	Double float64
	// Bytes holds a STRING; the wire does not say whether it is text or
	// binary, so Text reports whether it is valid UTF-8.
	Bytes []byte
	// fromJSON records that Bytes came off the JSON protocol, where a
	// binary field is base64 text.
	fromJSON bool
	UUID     thrift.Tuuid
	// Fields are the members of a STRUCT in wire order.
	Fields []Field
	// Elems are the members of a LIST or SET, and Entries those of a MAP.
	ElemType thrift.TType
	Elems    []*Value
	KeyType  thrift.TType
	ValType  thrift.TType
	Entries  []Entry
}

// Field is one struct member.
type Field struct {
	ID    int16
	Value *Value
}

// Entry is one map entry.
type Entry struct {
	Key, Value *Value
}

// bytesForBinary is the raw bytes of a string the IDL declares binary.
func (v *Value) bytesForBinary() []byte {
	return binaryBytes(v, v.fromJSON)
}

// Text reports whether a STRING value is printable UTF-8.
func (v *Value) Text() bool {
	if !utf8.Valid(v.Bytes) {
		return false
	}
	for _, r := range string(v.Bytes) {
		if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

// Message is a decoded message: the header and the struct that follows.
type Message struct {
	Name  string
	Type  thrift.TMessageType
	SeqID int32
	Body  *Value
}

// Result is what one Decode call found: a message or a bare struct, the
// protocol and framing it settled on, and how many bytes were left over.
type Result struct {
	Protocol Protocol
	Framed   bool
	// Schema is set when the result was annotated from an IDL.
	Schema bool
	// Messages or Values holds one entry per frame, or one entry for an
	// unframed input.
	Messages []*Message
	Values   []*Value
	Trailing int
}

// Decode reads every value from data.
func Decode(data []byte, opts Options) (*Result, error) {
	if opts.Protocol == "" {
		opts.Protocol = Auto
	}
	if opts.Framed == "" {
		opts.Framed = AutoDetect
	}
	if opts.Message == "" {
		opts.Message = AutoDetect
	}
	framed, err := detectFramed(data, opts.Framed)
	if err != nil {
		return nil, err
	}
	res := &Result{Framed: framed}
	if !framed {
		n, err := res.decodeOne(data, opts)
		if err != nil {
			return nil, err
		}
		res.Trailing = len(data) - n
		return res, nil
	}
	rest := data
	for len(rest) >= 4 {
		size := int(binary.BigEndian.Uint32(rest))
		if size > len(rest)-4 {
			return nil, fmt.Errorf("frame of %d bytes but only %d remain", size, len(rest)-4)
		}
		if max := int(opts.Config.GetMaxFrameSize()); size > max {
			return nil, fmt.Errorf("frame of %d bytes exceeds the limit of %d", size, max)
		}
		frame := rest[4 : 4+size]
		n, err := res.decodeOne(frame, opts)
		if err != nil {
			return nil, fmt.Errorf("frame %d: %w", len(res.Messages)+len(res.Values), err)
		}
		if n != size {
			return nil, fmt.Errorf("frame %d: %d of %d bytes decoded", len(res.Messages)+len(res.Values), n, size)
		}
		rest = rest[4+size:]
	}
	res.Trailing = len(rest)
	return res, nil
}

// detectFramed decides whether data is a sequence of length-prefixed
// frames: the first prefix has to fit, and the prefixes have to chain to
// the end of the data.
func detectFramed(data []byte, setting Tristate) (bool, error) {
	switch setting {
	case Yes:
		return true, nil
	case No:
		return false, nil
	}
	rest := data
	frames := 0
	for len(rest) >= 4 {
		size := int(binary.BigEndian.Uint32(rest))
		if size == 0 || size > len(rest)-4 {
			return false, nil
		}
		rest = rest[4+size:]
		frames++
	}
	return frames > 0 && len(rest) == 0, nil
}

// decodeOne decodes one value from data and returns how many bytes it
// used.
func (r *Result) decodeOne(data []byte, opts Options) (int, error) {
	protocol, err := detectProtocol(data, opts.Protocol)
	if err != nil {
		return 0, err
	}
	message := opts.Message == Yes || (opts.Message == AutoDetect && looksLikeMessage(data, protocol))
	candidates := []Protocol{protocol}
	if protocol == Auto {
		candidates = []Protocol{Binary, Compact}
	}
	var firstErr error
	for _, p := range candidates {
		n, msg, val, err := decodeWith(data, p, message, opts.Config)
		if err == nil {
			r.Protocol = p
			if msg != nil {
				r.Messages = append(r.Messages, msg)
			} else {
				r.Values = append(r.Values, val)
			}
			return n, nil
		}
		if firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", p, err)
		}
	}
	return 0, firstErr
}

// detectProtocol picks the protocol from the first bytes where they are
// unambiguous: JSON by its first character, a binary or compact message
// by its version magic. A bare binary or compact struct has no magic and
// stays Auto, to be tried in turn.
func detectProtocol(data []byte, setting Protocol) (Protocol, error) {
	if setting != Auto {
		return setting, nil
	}
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) == 0 {
		return Auto, errors.New("no data")
	}
	switch {
	case trimmed[0] == '[' || trimmed[0] == '{':
		return JSON, nil
	case len(data) >= 2 && data[0] == 0x80 && data[1] == 0x01:
		return Binary, nil
	case data[0] == 0x82:
		return Compact, nil
	}
	return Auto, nil
}

// looksLikeMessage reports whether data starts with a message header.
func looksLikeMessage(data []byte, protocol Protocol) bool {
	switch protocol {
	case Binary:
		return len(data) >= 2 && data[0] == 0x80 && data[1] == 0x01
	case Compact:
		return len(data) >= 1 && data[0] == 0x82
	case JSON:
		return len(bytes.TrimLeft(data, " \t\r\n")) > 0 && bytes.TrimLeft(data, " \t\r\n")[0] == '['
	}
	// A bare struct in either protocol starts with a field type byte, which
	// is never 0x80 or 0x82.
	return false
}

func newProtocol(data []byte, protocol Protocol, conf *thrift.TConfiguration) (thrift.TProtocol, *thrift.TMemoryBuffer) {
	buf := thrift.NewTMemoryBufferLen(len(data))
	buf.Write(data)
	switch protocol {
	case Compact:
		return thrift.NewTCompactProtocolConf(buf, conf), buf
	case JSON:
		return thrift.NewTJSONProtocol(buf), buf
	default:
		return thrift.NewTBinaryProtocolConf(buf, conf), buf
	}
}

func decodeWith(data []byte, protocol Protocol, message bool, conf *thrift.TConfiguration) (n int, msg *Message, val *Value, err error) {
	prot, buf := newProtocol(data, protocol, conf)
	ctx := context.Background()
	d := &decoder{prot: prot, ctx: ctx, json: protocol == JSON, depth: 0}
	if message {
		name, typ, seqid, err := prot.ReadMessageBegin(ctx)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("message header: %w", err)
		}
		body, err := d.readStruct()
		if err != nil {
			return 0, nil, nil, err
		}
		if err := prot.ReadMessageEnd(ctx); err != nil {
			return 0, nil, nil, err
		}
		return len(data) - buf.Len(), &Message{Name: name, Type: typ, SeqID: seqid, Body: body}, nil, nil
	}
	body, err := d.readStruct()
	if err != nil {
		return 0, nil, nil, err
	}
	return len(data) - buf.Len(), nil, body, nil
}

// maxDepth bounds nesting so that a crafted input cannot exhaust the
// stack; the runtime applies the same limit to generated code.
const maxDepth = thrift.DEFAULT_RECURSION_DEPTH

type decoder struct {
	prot  thrift.TProtocol
	ctx   context.Context
	json  bool
	depth int
}

func (d *decoder) enter() error {
	d.depth++
	if d.depth > maxDepth {
		return fmt.Errorf("nesting deeper than %d", maxDepth)
	}
	return nil
}

func (d *decoder) leave() { d.depth-- }

func (d *decoder) readStruct() (*Value, error) {
	if err := d.enter(); err != nil {
		return nil, err
	}
	defer d.leave()
	if _, err := d.prot.ReadStructBegin(d.ctx); err != nil {
		return nil, err
	}
	v := &Value{Type: thrift.STRUCT}
	for {
		_, typ, id, err := d.prot.ReadFieldBegin(d.ctx)
		if err != nil {
			return nil, err
		}
		if typ == thrift.STOP {
			break
		}
		fv, err := d.readValue(typ)
		if err != nil {
			return nil, fmt.Errorf("field %d: %w", id, err)
		}
		if err := d.prot.ReadFieldEnd(d.ctx); err != nil {
			return nil, err
		}
		v.Fields = append(v.Fields, Field{ID: id, Value: fv})
	}
	if err := d.prot.ReadStructEnd(d.ctx); err != nil {
		return nil, err
	}
	return v, nil
}

func (d *decoder) readValue(typ thrift.TType) (*Value, error) {
	v := &Value{Type: typ}
	var err error
	switch typ {
	case thrift.BOOL:
		v.Bool, err = d.prot.ReadBool(d.ctx)
	case thrift.BYTE:
		var b int8
		b, err = d.prot.ReadByte(d.ctx)
		v.Int = int64(b)
	case thrift.I16:
		var i int16
		i, err = d.prot.ReadI16(d.ctx)
		v.Int = int64(i)
	case thrift.I32:
		var i int32
		i, err = d.prot.ReadI32(d.ctx)
		v.Int = int64(i)
	case thrift.I64:
		v.Int, err = d.prot.ReadI64(d.ctx)
	case thrift.DOUBLE:
		v.Double, err = d.prot.ReadDouble(d.ctx)
	case thrift.STRING:
		if d.json {
			// The JSON protocol base64-encodes binary and quotes text; the
			// wire does not say which, so read as text. An IDL that
			// declares the field binary decodes it later.
			var s string
			s, err = d.prot.ReadString(d.ctx)
			v.Bytes = []byte(s)
			v.fromJSON = true
		} else {
			v.Bytes, err = d.prot.ReadBinary(d.ctx)
		}
	case thrift.UUID:
		v.UUID, err = d.prot.ReadUUID(d.ctx)
	case thrift.STRUCT:
		return d.readStruct()
	case thrift.LIST, thrift.SET:
		return d.readList(typ)
	case thrift.MAP:
		return d.readMap()
	default:
		return nil, fmt.Errorf("unknown type %d", typ)
	}
	return v, err
}

func (d *decoder) readList(typ thrift.TType) (*Value, error) {
	if err := d.enter(); err != nil {
		return nil, err
	}
	defer d.leave()
	var elemType thrift.TType
	var size int
	var err error
	if typ == thrift.SET {
		elemType, size, err = d.prot.ReadSetBegin(d.ctx)
	} else {
		elemType, size, err = d.prot.ReadListBegin(d.ctx)
	}
	if err != nil {
		return nil, err
	}
	v := &Value{Type: typ, ElemType: elemType}
	for i := 0; i < size; i++ {
		e, err := d.readValue(elemType)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
		v.Elems = append(v.Elems, e)
	}
	if typ == thrift.SET {
		err = d.prot.ReadSetEnd(d.ctx)
	} else {
		err = d.prot.ReadListEnd(d.ctx)
	}
	return v, err
}

func (d *decoder) readMap() (*Value, error) {
	if err := d.enter(); err != nil {
		return nil, err
	}
	defer d.leave()
	keyType, valType, size, err := d.prot.ReadMapBegin(d.ctx)
	if err != nil {
		return nil, err
	}
	v := &Value{Type: thrift.MAP, KeyType: keyType, ValType: valType}
	for i := 0; i < size; i++ {
		k, err := d.readValue(keyType)
		if err != nil {
			return nil, fmt.Errorf("key %d: %w", i, err)
		}
		val, err := d.readValue(valType)
		if err != nil {
			return nil, fmt.Errorf("value %d: %w", i, err)
		}
		v.Entries = append(v.Entries, Entry{Key: k, Value: val})
	}
	return v, d.prot.ReadMapEnd(d.ctx)
}

// ---- text rendering ----

// Format writes the result as an indented tree.
func Format(w io.Writer, r *Result) {
	for i, m := range r.Messages {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "message %s %s seqid=%d\n", messageType(m.Type), strconv.Quote(m.Name), m.SeqID)
		writeValue(w, m.Body, 0)
		fmt.Fprintln(w)
	}
	for i, v := range r.Values {
		if i > 0 || len(r.Messages) > 0 {
			fmt.Fprintln(w)
		}
		writeValue(w, v, 0)
		fmt.Fprintln(w)
	}
	if r.Trailing > 0 {
		fmt.Fprintf(w, "%d trailing byte(s) not decoded\n", r.Trailing)
	}
}

func messageType(t thrift.TMessageType) string {
	switch t {
	case thrift.CALL:
		return "CALL"
	case thrift.REPLY:
		return "REPLY"
	case thrift.EXCEPTION:
		return "EXCEPTION"
	case thrift.ONEWAY:
		return "ONEWAY"
	}
	return "type " + strconv.Itoa(int(t))
}

func typeName(t thrift.TType) string {
	return strings.ToLower(t.String())
}

// Format writes the result as an indented tree.
func writeValue(w io.Writer, v *Value, indent int) {
	pad := strings.Repeat("  ", indent)
	switch v.Type {
	case thrift.STRUCT:
		kind := "struct"
		if v.Annotation.Kind != "" {
			kind = v.Annotation.Kind
		}
		if v.Annotation.TypeName != "" && v.Annotation.Kind != "" {
			kind += " " + v.Annotation.TypeName
		} else if v.Annotation.Name != "" && v.Annotation.Kind != "" {
			kind += " " + v.Annotation.Name
		}
		if len(v.Fields) == 0 {
			fmt.Fprint(w, kind+" {}")
			return
		}
		fmt.Fprint(w, kind+" {\n")
		for _, f := range v.Fields {
			fmt.Fprintf(w, "%s  %d: ", pad, f.ID)
			if name := f.Value.Annotation.Name; name != "" {
				fmt.Fprint(w, name+" ")
			}
			if f.Value.Type != thrift.STRUCT || f.Value.Annotation.Mismatch != "" {
				// A struct value spells its own type.
				fmt.Fprint(w, typeName(f.Value.Type)+" ")
			}
			writeValue(w, f.Value, indent+1)
			writeMarks(w, f.Value)
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s}", pad)
	case thrift.LIST, thrift.SET:
		fmt.Fprintf(w, "<%s> ", typeName(v.ElemType))
		writeSeq(w, v.Elems, indent, "[", "]")
	case thrift.MAP:
		fmt.Fprintf(w, "<%s, %s> ", typeName(v.KeyType), typeName(v.ValType))
		if len(v.Entries) == 0 {
			fmt.Fprint(w, "{}")
			return
		}
		fmt.Fprint(w, "{\n")
		for _, e := range v.Entries {
			fmt.Fprintf(w, "%s  ", pad)
			writeValue(w, e.Key, indent+1)
			fmt.Fprint(w, ": ")
			writeValue(w, e.Value, indent+1)
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s}", pad)
	default:
		fmt.Fprint(w, scalar(v))
	}
}

// writeSeq writes short scalar sequences on one line and the rest one
// element per line.
func writeSeq(w io.Writer, elems []*Value, indent int, open, close string) {
	pad := strings.Repeat("  ", indent)
	if len(elems) == 0 {
		fmt.Fprint(w, open+close)
		return
	}
	scalars := true
	for _, e := range elems {
		if e.Type == thrift.STRUCT || e.Type == thrift.MAP || e.Type == thrift.LIST || e.Type == thrift.SET {
			scalars = false
		}
	}
	if scalars && len(elems) <= 8 {
		parts := make([]string, len(elems))
		for i, e := range elems {
			parts[i] = scalar(e)
		}
		fmt.Fprint(w, open+strings.Join(parts, ", ")+close)
		return
	}
	fmt.Fprint(w, open+"\n")
	for _, e := range elems {
		fmt.Fprintf(w, "%s  ", pad)
		writeValue(w, e, indent+1)
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%s%s", pad, close)
}

// writeMarks appends what the IDL had to say that the value itself does
// not show.
func writeMarks(w io.Writer, v *Value) {
	a := v.Annotation
	switch {
	case a.Unknown:
		fmt.Fprint(w, "  (not in the IDL)")
	case a.Mismatch != "":
		fmt.Fprintf(w, "  (the IDL says %s)", a.Mismatch)
	case a.EnumName != "":
		fmt.Fprintf(w, " (%s %s)", a.TypeName, a.EnumName)
	case v.Type == thrift.I32 && a.TypeName != "" && a.TypeName != "i32" && a.Mismatch == "":
		// An enum value the IDL does not declare.
		if t := a.TypeName; t != "" {
			fmt.Fprintf(w, " (%s, not a declared member)", t)
		}
	}
}

func scalar(v *Value) string {
	switch v.Type {
	case thrift.BOOL:
		return strconv.FormatBool(v.Bool)
	case thrift.BYTE, thrift.I16, thrift.I32, thrift.I64:
		return strconv.FormatInt(v.Int, 10)
	case thrift.DOUBLE:
		return strconv.FormatFloat(v.Double, 'g', -1, 64)
	case thrift.STRING:
		if v.Annotation.Binary {
			b := v.bytesForBinary()
			return fmt.Sprintf("0x%s (%d bytes)", hex.EncodeToString(b), len(b))
		}
		if v.Text() {
			return strconv.Quote(string(v.Bytes))
		}
		return fmt.Sprintf("0x%s (%d bytes)", hex.EncodeToString(v.Bytes), len(v.Bytes))
	case thrift.UUID:
		return v.UUID.String()
	}
	return "?"
}

// ---- JSON rendering ----

// ToJSON renders the result as plain data for encoding/json: a struct is
// an object keyed by field id, a list or set an array, a map an array of
// {key, value} pairs since keys need not be strings, and a string either
// the text or {"binary": "<hex>"}.
func ToJSON(r *Result) map[string]interface{} {
	out := map[string]interface{}{
		"protocol": string(r.Protocol),
		"framed":   r.Framed,
	}
	if len(r.Messages) > 0 {
		msgs := make([]interface{}, 0, len(r.Messages))
		for _, m := range r.Messages {
			msgs = append(msgs, map[string]interface{}{
				"name":  m.Name,
				"type":  messageType(m.Type),
				"seqid": m.SeqID,
				"body":  valueJSON(m.Body),
			})
		}
		out["messages"] = msgs
	}
	if len(r.Values) > 0 {
		vals := make([]interface{}, 0, len(r.Values))
		for _, v := range r.Values {
			vals = append(vals, valueJSON(v))
		}
		out["values"] = vals
	}
	if r.Trailing > 0 {
		out["trailing"] = r.Trailing
	}
	return out
}

func valueJSON(v *Value) interface{} {
	switch v.Type {
	case thrift.BOOL:
		return v.Bool
	case thrift.BYTE, thrift.I16, thrift.I32, thrift.I64:
		return v.Int
	case thrift.DOUBLE:
		return v.Double
	case thrift.STRING:
		if v.Annotation.Binary {
			return map[string]string{"binary": hex.EncodeToString(v.bytesForBinary())}
		}
		if v.Text() {
			return string(v.Bytes)
		}
		return map[string]string{"binary": hex.EncodeToString(v.Bytes)}
	case thrift.UUID:
		return v.UUID.String()
	case thrift.STRUCT:
		obj := map[string]interface{}{}
		for _, f := range v.Fields {
			entry := map[string]interface{}{"type": typeName(f.Value.Type), "value": valueJSON(f.Value)}
			a := f.Value.Annotation
			if a.Name != "" {
				entry["name"] = a.Name
			}
			if a.TypeName != "" {
				entry["idl_type"] = a.TypeName
			}
			if a.EnumName != "" {
				entry["enum"] = a.EnumName
			}
			if a.Unknown {
				entry["unknown"] = true
			}
			if a.Mismatch != "" {
				entry["mismatch"] = a.Mismatch
			}
			obj[strconv.Itoa(int(f.ID))] = entry
		}
		if v.Annotation.Kind != "" {
			return map[string]interface{}{"kind": v.Annotation.Kind, "name": v.Annotation.Name, "fields": obj}
		}
		return obj
	case thrift.LIST, thrift.SET:
		arr := make([]interface{}, 0, len(v.Elems))
		for _, e := range v.Elems {
			arr = append(arr, valueJSON(e))
		}
		return arr
	case thrift.MAP:
		arr := make([]interface{}, 0, len(v.Entries))
		for _, e := range v.Entries {
			arr = append(arr, map[string]interface{}{"key": valueJSON(e.Key), "value": valueJSON(e.Value)})
		}
		return arr
	}
	return nil
}
