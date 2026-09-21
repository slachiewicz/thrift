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

package decode

import (
	"bytes"
	"context"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
)

// writeSample writes the struct the tests decode: every scalar type, a
// nested struct, a list, a set and a map, with ids in wire order.
func writeSample(t *testing.T, p thrift.TProtocol) {
	t.Helper()
	ctx := context.Background()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(p.WriteStructBegin(ctx, "Sample"))
	must(p.WriteFieldBegin(ctx, "b", thrift.BOOL, 1))
	must(p.WriteBool(ctx, true))
	must(p.WriteFieldEnd(ctx))
	must(p.WriteFieldBegin(ctx, "i", thrift.I32, 2))
	must(p.WriteI32(ctx, -7))
	must(p.WriteFieldEnd(ctx))
	must(p.WriteFieldBegin(ctx, "s", thrift.STRING, 3))
	must(p.WriteString(ctx, "héllo"))
	must(p.WriteFieldEnd(ctx))
	must(p.WriteFieldBegin(ctx, "bin", thrift.STRING, 4))
	must(p.WriteBinary(ctx, []byte{0x00, 0xff, 0x10}))
	must(p.WriteFieldEnd(ctx))
	must(p.WriteFieldBegin(ctx, "d", thrift.DOUBLE, 5))
	must(p.WriteDouble(ctx, 1.5))
	must(p.WriteFieldEnd(ctx))
	must(p.WriteFieldBegin(ctx, "inner", thrift.STRUCT, 6))
	must(p.WriteStructBegin(ctx, "Inner"))
	must(p.WriteFieldBegin(ctx, "l", thrift.LIST, 1))
	must(p.WriteListBegin(ctx, thrift.I64, 3))
	for _, v := range []int64{1, 2, 3} {
		must(p.WriteI64(ctx, v))
	}
	must(p.WriteListEnd(ctx))
	must(p.WriteFieldEnd(ctx))
	must(p.WriteFieldBegin(ctx, "m", thrift.MAP, 2))
	must(p.WriteMapBegin(ctx, thrift.STRING, thrift.I16, 1))
	must(p.WriteString(ctx, "k"))
	must(p.WriteI16(ctx, 9))
	must(p.WriteMapEnd(ctx))
	must(p.WriteFieldEnd(ctx))
	must(p.WriteFieldStop(ctx))
	must(p.WriteStructEnd(ctx))
	must(p.WriteFieldEnd(ctx))
	must(p.WriteFieldBegin(ctx, "set", thrift.SET, 7))
	must(p.WriteSetBegin(ctx, thrift.BYTE, 2))
	must(p.WriteByte(ctx, 1))
	must(p.WriteByte(ctx, 2))
	must(p.WriteSetEnd(ctx))
	must(p.WriteFieldEnd(ctx))
	must(p.WriteFieldBegin(ctx, "u", thrift.UUID, 8))
	uuid, err := thrift.ParseTuuid("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	must(err)
	must(p.WriteUUID(ctx, uuid))
	must(p.WriteFieldEnd(ctx))
	must(p.WriteFieldStop(ctx))
	must(p.WriteStructEnd(ctx))
	must(p.Flush(ctx))
}

func encode(t *testing.T, protocol Protocol, message bool) []byte {
	t.Helper()
	buf := thrift.NewTMemoryBuffer()
	var p thrift.TProtocol
	switch protocol {
	case Binary:
		p = thrift.NewTBinaryProtocolConf(buf, nil)
	case Compact:
		p = thrift.NewTCompactProtocolConf(buf, nil)
	case JSON:
		p = thrift.NewTJSONProtocol(buf)
	}
	ctx := context.Background()
	if message {
		if err := p.WriteMessageBegin(ctx, "ping", thrift.CALL, 42); err != nil {
			t.Fatal(err)
		}
	}
	writeSample(t, p)
	if message {
		if err := p.WriteMessageEnd(ctx); err != nil {
			t.Fatal(err)
		}
		if err := p.Flush(ctx); err != nil {
			t.Fatal(err)
		}
	}
	return buf.Bytes()
}

func frame(data ...[]byte) []byte {
	var out []byte
	for _, d := range data {
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(d)))
		out = append(out, n[:]...)
		out = append(out, d...)
	}
	return out
}

const wantSample = `struct {
  1: bool true
  2: i32 -7
  3: string "héllo"
  4: string 0x00ff10 (3 bytes)
  5: double 1.5
  6: struct {
    1: list <i64> [1, 2, 3]
    2: map <string, i16> {
      "k": 9
    }
  }
  7: set <byte> [1, 2]
  8: uuid 6ba7b810-9dad-11d1-80b4-00c04fd430c8
}
`

func TestDecodeEveryProtocol(t *testing.T) {
	for _, protocol := range []Protocol{Binary, Compact, JSON} {
		for _, message := range []bool{false, true} {
			name := string(protocol)
			if message {
				name += "-message"
			}
			t.Run(name, func(t *testing.T) {
				data := encode(t, protocol, message)
				res, err := Decode(data, Options{})
				if err != nil {
					t.Fatal(err)
				}
				if res.Protocol != protocol {
					t.Errorf("detected %s, want %s", res.Protocol, protocol)
				}
				if res.Trailing != 0 || res.Framed {
					t.Errorf("trailing=%d framed=%v", res.Trailing, res.Framed)
				}
				var out bytes.Buffer
				Format(&out, res)
				want := wantSample
				if protocol == JSON {
					// The JSON protocol base64-encodes binary and the wire
					// does not say so; the decoder reads it as text.
					want = strings.Replace(want, `0x00ff10 (3 bytes)`, `"AP8Q"`, 1)
				}
				if message {
					want = "message CALL \"ping\" seqid=42\n" + want
				}
				if out.String() != want {
					t.Errorf("output:\n%s\nwant:\n%s", out.String(), want)
				}
			})
		}
	}
}

func TestDecodeFramedStream(t *testing.T) {
	a := encode(t, Compact, true)
	b := encode(t, Compact, true)
	res, err := Decode(frame(a, b), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Framed || len(res.Messages) != 2 || res.Trailing != 0 {
		t.Fatalf("framed=%v messages=%d trailing=%d", res.Framed, len(res.Messages), res.Trailing)
	}
	if res.Messages[1].SeqID != 42 || len(res.Messages[1].Body.Fields) != 8 {
		t.Errorf("second frame decoded as %+v", res.Messages[1])
	}
}

func TestDecodeReportsTrailingBytes(t *testing.T) {
	data := append(encode(t, Binary, false), 0xde, 0xad)
	res, err := Decode(data, Options{Protocol: Binary})
	if err != nil {
		t.Fatal(err)
	}
	if res.Trailing != 2 {
		t.Errorf("trailing = %d, want 2", res.Trailing)
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	cases := map[string][]byte{
		"empty":            {},
		"unknown type":     {0x7f, 0x00, 0x01},
		"truncated string": {0x0b, 0x00, 0x01, 0x00, 0x00, 0x10, 0x00, 'a'},
		"huge frame":       {0x7f, 0xff, 0xff, 0xff, 0x00},
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if res, err := Decode(data, Options{}); err == nil {
				t.Errorf("accepted: %+v", res)
			}
		})
	}
	// A short frame prefix decodes as an empty struct with trailing bytes
	// unless framing is forced, and then the frame does not fit.
	short := []byte{0x00, 0x00, 0x00, 0x02, 0x00}
	if res, err := Decode(short, Options{}); err != nil || res.Framed || res.Trailing != 4 {
		t.Errorf("unforced: res=%+v err=%v", res, err)
	}
	if _, err := Decode(short, Options{Framed: Yes}); err == nil {
		t.Error("forced framing accepted a frame that does not fit")
	}
}

func TestDecodeBoundsNesting(t *testing.T) {
	// A binary struct whose field 1 is a struct whose field 1 is a struct...
	var data []byte
	for i := 0; i < maxDepth+2; i++ {
		data = append(data, byte(thrift.STRUCT), 0x00, 0x01)
	}
	if _, err := Decode(data, Options{Protocol: Binary, Framed: No}); err == nil || !strings.Contains(err.Error(), "nesting deeper") {
		t.Errorf("got %v, want a nesting error", err)
	}
}
