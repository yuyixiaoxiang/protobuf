// Copyright (c) 2013, Vastech SA (PTY) LTD. All rights reserved.
// http://code.google.com/p/gogoprotobuf
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

/*
The size plugin generates a Size method for each message.
This is useful with the MarshalTo method generated by the marshalto plugin and the
gogoproto.marshaler and gogoproto.marshaler_all extensions.

It is enabled by the following extensions:

  - sizer
  - sizer_all

The size plugin also generates a test given it is enabled using one of the following extensions:

  - testgen
  - testgen_all

And a benchmark given it is enabled using one of the following extensions:

  - benchgen
  - benchgen_all

Let us look at:

  code.google.com/p/gogoprotobuf/test/example/example.proto

Btw all the output can be seen at:

  code.google.com/p/gogoprotobuf/test/example/*

The following message:

  option (gogoproto.sizer_all) = true;

  message B {
	option (gogoproto.description) = true;
	optional A A = 1 [(gogoproto.nullable) = false, (gogoproto.embed) = true];
	repeated bytes G = 2 [(gogoproto.customtype) = "code.google.com/p/gogoprotobuf/test/custom.Uint128", (gogoproto.nullable) = false];
  }

given to the size plugin, will generate the following code:

  func (m *B) Size() (n int) {
	var l int
	_ = l
	l = m.A.Size()
	n += 1 + l + sovExample(uint64(l))
	if len(m.G) > 0 {
		for _, e := range m.G {
			l = e.Size()
			n += 1 + l + sovExample(uint64(l))
		}
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
  }

and the following test code:

	func TestBSize(t *testing5.T) {
		popr := math_rand5.New(math_rand5.NewSource(time5.Now().UnixNano()))
		p := NewPopulatedB(popr, true)
		data, err := code_google_com_p_gogoprotobuf_proto2.Marshal(p)
		if err != nil {
			panic(err)
		}
		size := p.Size()
		if len(data) != size {
			t.Fatalf("size %v != marshalled size %v", size, len(data))
		}
	}

	func BenchmarkBSize(b *testing5.B) {
		popr := math_rand5.New(math_rand5.NewSource(616))
		total := 0
		pops := make([]*B, 1000)
		for i := 0; i < 1000; i++ {
			pops[i] = NewPopulatedB(popr, false)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			total += pops[i%1000].Size()
		}
		b.SetBytes(int64(total / b.N))
	}

The sovExample function is a size of varint function for the example.pb.go file.

*/
package size

import (
	"code.google.com/p/gogoprotobuf/gogoproto"
	"code.google.com/p/gogoprotobuf/proto"
	descriptor "code.google.com/p/gogoprotobuf/protoc-gen-gogo/descriptor"
	"code.google.com/p/gogoprotobuf/protoc-gen-gogo/generator"
	"fmt"
	"strconv"
)

type size struct {
	*generator.Generator
	generator.PluginImports
	atleastOne bool
	localName  string
}

func NewSize() *size {
	return &size{}
}

func (p *size) Name() string {
	return "size"
}

func (p *size) Init(g *generator.Generator) {
	p.Generator = g
}

func wireToType(wire string) int {
	switch wire {
	case "fixed64":
		return proto.WireFixed64
	case "fixed32":
		return proto.WireFixed32
	case "varint":
		return proto.WireVarint
	case "bytes":
		return proto.WireBytes
	case "group":
		return proto.WireBytes
	case "zigzag32":
		return proto.WireVarint
	case "zigzag64":
		return proto.WireVarint
	}
	panic("unreachable")
}

func keySize(fieldNumber int32, wireType int) int {
	x := uint32(fieldNumber)<<3 | uint32(wireType)
	size := 0
	for size = 0; x > 127; size++ {
		x >>= 7
	}
	size++
	return size
}

func (p *size) sizeVarint() {
	p.P(`
	func sov`, p.localName, `(x uint64) (n int) {
		for {
			n++
			x >>= 7
			if x == 0 {
				break
			}
		}
		return n
	}`)
}

func (p *size) sizeZigZag() {
	p.P(`func soz`, p.localName, `(x uint64) (n int) {
 		return sov`, p.localName, `(uint64((x << 1) ^ uint64((int64(x) >> 63)))) 	
 		return sov`, p.localName, `(uint64((x << 1) ^ uint64((int64(x) >> 63))))
	}`)
}

func (p *size) Generate(file *generator.FileDescriptor) {
	p.PluginImports = generator.NewPluginImports(p.Generator)
	p.atleastOne = false
	p.localName = generator.FileName(file)
	protoPkg := p.NewImport("code.google.com/p/gogoprotobuf/proto")
	for _, message := range file.Messages() {
		if !gogoproto.IsSizer(file.FileDescriptorProto, message.DescriptorProto) {
			continue
		}
		p.atleastOne = true

		ccTypeName := generator.CamelCaseSlice(message.TypeName())
		p.P(`func (m *`, ccTypeName, `) Size() (n int) {`)
		p.In()
		p.P(`var l int`)
		p.P(`_ = l`)
		for _, field := range message.Field {
			fieldname := p.GetFieldName(message, field)
			nullable := gogoproto.IsNullable(field)
			repeated := field.IsRepeated()
			if repeated {
				p.P(`if len(m.`, fieldname, `) > 0 {`)
				p.In()
			} else if nullable {
				p.P(`if m.`, fieldname, ` != nil {`)
				p.In()
			}
			packed := field.IsPacked()
			_, wire := p.GoType(message, field)
			wireType := wireToType(wire)
			fieldNumber := field.GetNumber()
			if packed {
				wireType = proto.WireBytes
			}
			key := keySize(fieldNumber, wireType)
			switch *field.Type {
			case descriptor.FieldDescriptorProto_TYPE_DOUBLE,
				descriptor.FieldDescriptorProto_TYPE_FIXED64,
				descriptor.FieldDescriptorProto_TYPE_SFIXED64:
				if packed {
					p.P(`n+=`, strconv.Itoa(key), `+sov`, p.localName, `(uint64(len(m.`, fieldname, `)*8))`, `+len(m.`, fieldname, `)*8`)
				} else if repeated {
					p.P(`n+=`, strconv.Itoa(key+8), `*len(m.`, fieldname, `)`)
				} else if nullable {
					p.P(`n+=`, strconv.Itoa(key+8))
				} else {
					p.P(`n+=`, strconv.Itoa(key+8))
				}
			case descriptor.FieldDescriptorProto_TYPE_FLOAT,
				descriptor.FieldDescriptorProto_TYPE_FIXED32,
				descriptor.FieldDescriptorProto_TYPE_SFIXED32:
				if packed {
					p.P(`n+=`, strconv.Itoa(key), `+sov`, p.localName, `(uint64(len(m.`, fieldname, `)*4))`, `+len(m.`, fieldname, `)*4`)
				} else if repeated {
					p.P(`n+=`, strconv.Itoa(key+4), `*len(m.`, fieldname, `)`)
				} else if nullable {
					p.P(`n+=`, strconv.Itoa(key+4))
				} else {
					p.P(`n+=`, strconv.Itoa(key+4))
				}
			case descriptor.FieldDescriptorProto_TYPE_INT64,
				descriptor.FieldDescriptorProto_TYPE_UINT64,
				descriptor.FieldDescriptorProto_TYPE_INT32,
				descriptor.FieldDescriptorProto_TYPE_UINT32,
				descriptor.FieldDescriptorProto_TYPE_ENUM:
				if packed {
					p.P(`l = 0`)
					p.P(`for _, e := range m.`, fieldname, ` {`)
					p.In()
					p.P(`l+=sov`, p.localName, `(uint64(e))`)
					p.Out()
					p.P(`}`)
					p.P(`n+=`, strconv.Itoa(key), `+sov`, p.localName, `(uint64(l))+l`)
				} else if repeated {
					p.P(`for _, e := range m.`, fieldname, ` {`)
					p.In()
					p.P(`n+=`, strconv.Itoa(key), `+sov`, p.localName, `(uint64(e))`)
					p.Out()
					p.P(`}`)
				} else if nullable {
					p.P(`n+=`, strconv.Itoa(key), `+sov`, p.localName, `(uint64(*m.`, fieldname, `))`)
				} else {
					p.P(`n+=`, strconv.Itoa(key), `+sov`, p.localName, `(uint64(m.`, fieldname, `))`)
				}
			case descriptor.FieldDescriptorProto_TYPE_BOOL:
				if packed {
					p.P(`n+=`, strconv.Itoa(key), `+sov`, p.localName, `(uint64(len(m.`, fieldname, `)))`, `+len(m.`, fieldname, `)*1`)
				} else if repeated {
					p.P(`n+=`, strconv.Itoa(key+1), `*len(m.`, fieldname, `)`)
				} else if nullable {
					p.P(`n+=`, strconv.Itoa(key+1))
				} else {
					p.P(`n+=`, strconv.Itoa(key+1))
				}
			case descriptor.FieldDescriptorProto_TYPE_STRING:
				if repeated {
					p.P(`for _, s := range m.`, fieldname, ` { `)
					p.In()
					p.P(`l = len(s)`)
					p.P(`n+=`, strconv.Itoa(key), `+l+sov`, p.localName, `(uint64(l))`)
					p.Out()
					p.P(`}`)
				} else if nullable {
					p.P(`l=len(*m.`, fieldname, `)`)
					p.P(`n+=`, strconv.Itoa(key), `+l+sov`, p.localName, `(uint64(l))`)
				} else {
					p.P(`l=len(m.`, fieldname, `)`)
					p.P(`n+=`, strconv.Itoa(key), `+l+sov`, p.localName, `(uint64(l))`)
				}
			case descriptor.FieldDescriptorProto_TYPE_GROUP:
				panic(fmt.Errorf("size does not support group %v", fieldname))
			case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
				if repeated {
					p.P(`for _, e := range m.`, fieldname, ` { `)
					p.In()
					p.P(`l=e.Size()`)
					p.P(`n+=`, strconv.Itoa(key), `+l+sov`, p.localName, `(uint64(l))`)
					p.Out()
					p.P(`}`)
				} else {
					p.P(`l=m.`, fieldname, `.Size()`)
					p.P(`n+=`, strconv.Itoa(key), `+l+sov`, p.localName, `(uint64(l))`)
				}
			case descriptor.FieldDescriptorProto_TYPE_BYTES:
				if !gogoproto.IsCustomType(field) {
					if repeated {
						p.P(`for _, b := range m.`, fieldname, ` { `)
						p.In()
						p.P(`l = len(b)`)
						p.P(`n+=`, strconv.Itoa(key), `+l+sov`, p.localName, `(uint64(l))`)
						p.Out()
						p.P(`}`)
					} else {
						p.P(`l=len(m.`, fieldname, `)`)
						p.P(`n+=`, strconv.Itoa(key), `+l+sov`, p.localName, `(uint64(l))`)
					}
				} else {
					if repeated {
						p.P(`for _, e := range m.`, fieldname, ` { `)
						p.In()
						p.P(`l=e.Size()`)
						p.P(`n+=`, strconv.Itoa(key), `+l+sov`, p.localName, `(uint64(l))`)
						p.Out()
						p.P(`}`)
					} else {
						p.P(`l=m.`, fieldname, `.Size()`)
						p.P(`n+=`, strconv.Itoa(key), `+l+sov`, p.localName, `(uint64(l))`)
					}
				}
			case descriptor.FieldDescriptorProto_TYPE_SINT32,
				descriptor.FieldDescriptorProto_TYPE_SINT64:
				if packed {
					p.P(`l = 0`)
					p.P(`for _, e := range m.`, fieldname, ` {`)
					p.In()
					p.P(`l+=soz`, p.localName, `(uint64(e))`)
					p.Out()
					p.P(`}`)
					p.P(`n+=`, strconv.Itoa(key), `+sov`, p.localName, `(uint64(l))+l`)
				} else if repeated {
					p.P(`for _, e := range m.`, fieldname, ` {`)
					p.In()
					p.P(`n+=`, strconv.Itoa(key), `+soz`, p.localName, `(uint64(e))`)
					p.Out()
					p.P(`}`)
				} else if nullable {
					p.P(`n+=`, strconv.Itoa(key), `+soz`, p.localName, `(uint64(*m.`, fieldname, `))`)
				} else {
					p.P(`n+=`, strconv.Itoa(key), `+soz`, p.localName, `(uint64(m.`, fieldname, `))`)
				}
			default:
				panic("not implemented")
			}
			if nullable || repeated {
				p.Out()
				p.P(`}`)
			}
		}
		if message.DescriptorProto.HasExtension() {
			p.P(`if m.XXX_extensions != nil {`)
			p.In()
			if gogoproto.HasExtensionsMap(file.FileDescriptorProto, message.DescriptorProto) {
				p.P(`n += `, protoPkg.Use(), `.SizeOfExtensionMap(m.XXX_extensions)`)
			} else {
				p.P(`n+=len(m.XXX_extensions)`)
			}
			p.Out()
			p.P(`}`)
		}
		p.P(`if m.XXX_unrecognized != nil {`)
		p.In()
		p.P(`n+=len(m.XXX_unrecognized)`)
		p.Out()
		p.P(`}`)
		p.P(`return n`)
		p.Out()
		p.P(`}`)
	}

	if !p.atleastOne {
		return
	}

	p.sizeVarint()
	p.sizeZigZag()

}

func init() {
	generator.RegisterPlugin(NewSize())
}
