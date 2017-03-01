// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"go/token"
	"math"
	"unsafe"

	"github.com/cznic/internal/buffer"
	"github.com/cznic/ir"
	"github.com/cznic/mathutil"
)

var (
	builtins   = map[ir.NameID]Opcode{}
	nonReturns = map[Opcode]struct{}{
		abort: {},
		exit:  {},
		Panic: {},
	}
)

func registerBuiltins(m map[int]Opcode) {
	for k, v := range m {
		nm := ir.NameID(k)
		if _, ok := builtins[nm]; ok {
			panic("internal error")
		}

		builtins[nm] = v
	}
}

// IsBuiltin reports whether an external function is one of the builtins.
func IsBuiltin(nm ir.NameID) bool {
	_, ok := builtins[nm]
	return ok
}

// PCInfo represents a line/function for a particular program counter location.
type PCInfo struct {
	PC     int
	Line   int
	Column int
	Name   ir.NameID // File name or func name.
}

// Position returns a token.Position from p.
func (p *PCInfo) Position() token.Position {
	return token.Position{Line: p.Line, Column: p.Column, Filename: string(dict.S(int(p.Name)))}
}

// Binary represents a loaded program image. It can be run via Exec.
type Binary struct {
	BSS        int
	Code       []Operation
	DSRelative []byte // Bit vector of data segment-relative pointers in Data.
	Data       []byte
	Functions  []PCInfo
	Lines      []PCInfo
	Model      string
	TSRelative []byte // Bit vector of text segment-relative pointers in Data.
	Text       []byte
}

func newBinary(model string) *Binary {
	return &Binary{Model: model}
}

type nfo struct {
	off int
	sz  int
}

type loader struct {
	intSize    int
	m          map[int]int // Object #: {BSS,Code,Data,Text} index.
	model      ir.MemoryModel
	objects    []ir.Object
	out        *Binary
	prev       Operation
	ptrSize    int
	stackAlign int
	strings    map[ir.StringID]int
	tc         ir.TypeCache
}

func newLoader(modelName string, objects []ir.Object) *loader {
	model, ok := ir.MemoryModels[modelName]
	if !ok {
		panic(fmt.Errorf("unknown memory model %q", modelName))
	}

	ptrItem, ok := model[ir.Pointer]
	if !ok {
		panic(fmt.Errorf("invalid memory model %q, missing item for pointer", modelName))
	}

	return &loader{
		m:          map[int]int{},
		model:      model,
		objects:    objects,
		out:        newBinary(modelName),
		prev:       Operation{Opcode: -1},
		ptrSize:    int(ptrItem.Size),
		stackAlign: int(ptrItem.Align),
		strings:    map[ir.StringID]int{},
		tc:         ir.TypeCache{},
	}
}

func (l *loader) loadDataDefinition(d *ir.DataDefinition, off int, v ir.Value) {
	typ := l.tc.MustType(d.TypeID)
	if typ.Kind() == ir.Array && typ.(*ir.ArrayType).Item.Kind() == ir.Int8 {
		switch x := v.(type) {
		case *ir.StringValue:
			copy(l.out.Data[off:], dict.S(int(x.StringID)))
			return
		default:
			panic(fmt.Errorf("%s: TODO %T", d.Position, x))
		}
	}

	var f func(int, ir.TypeID, ir.Value)
	f = func(off int, t ir.TypeID, v ir.Value) {
		b := l.out.Data[off:]
		switch x := v.(type) {
		case *ir.AddressValue:
			*(*uintptr)((unsafe.Pointer)(&b[0])) = uintptr(l.m[x.Index])
			if _, ok := l.objects[x.Index].(*ir.DataDefinition); ok {
				l.out.DSRelative[off>>3] |= 1 << uint(off&7)
			}
		case *ir.CompositeValue:
			switch typ := l.tc.MustType(t); typ.Kind() {
			case ir.Array:
				i := 0
				at := typ.(*ir.ArrayType)
				itemT := at.Item
				itemSz := l.model.Sizeof(itemT)
				for _, v := range x.Values {
					switch y := v.(type) {
					case
						*ir.Int32Value,
						*ir.Int64Value,
						*ir.StringValue:

						f(off+i*int(itemSz), itemT.ID(), y)
					default:
						panic(fmt.Errorf("%s: TODO %T: %v", d.Position, y, y))
					}
					i++
				}
			case ir.Struct:
				i := 0
				st := typ.(*ir.StructOrUnionType)
				fields := st.Fields
				layout := l.model.Layout(st)
				for _, v := range x.Values {
					switch y := v.(type) {
					case nil:
						// nop
					case *ir.Int32Value:
						f(off+int(layout[i].Offset), fields[i].ID(), y)
					default:
						panic(fmt.Errorf("%s: TODO %T: %v", d.Position, y, y))
					}
					i++
				}
			default:
				panic(fmt.Errorf("%s: TODO %v: %v", d.Position, typ.Kind(), v))
			}
		case *ir.Int32Value:
			switch typ := l.tc.MustType(t); typ.Kind() {
			case ir.Int16, ir.Uint16:
				*(*int16)((unsafe.Pointer)(&b[0])) = int16(x.Value)
			case ir.Int32:
				*(*int32)((unsafe.Pointer)(&b[0])) = x.Value
			case ir.Int64:
				*(*int64)((unsafe.Pointer)(&b[0])) = int64(x.Value)
			case ir.Pointer:
				*(*uintptr)((unsafe.Pointer)(&b[0])) = uintptr(x.Value)
			default:
				panic(fmt.Errorf("%s: TODO %v: %v", d.Position, t, v))
			}
		case *ir.Int64Value:
			switch typ := l.tc.MustType(t); typ.Kind() {
			case ir.Int64, ir.Uint64:
				*(*int64)((unsafe.Pointer)(&b[0])) = x.Value
			default:
				panic(fmt.Errorf("%s: TODO %v: %v", d.Position, t, v))
			}
		case *ir.StringValue:
			*(*uintptr)((unsafe.Pointer)(&b[0])) = uintptr(l.text(x.StringID))
			l.out.TSRelative[off>>3] |= 1 << uint(off&7)
		default:
			panic(fmt.Errorf("%s: TODO %T: %v", d.Position, x, x))
		}
	}
	f(off, d.TypeID, v)
}

func (l *loader) emitOne(op Operation) {
	prev := l.prev
	if _, ok := nonReturns[prev.Opcode]; ok {
		switch op.Opcode {
		case Func, Label:
		default:
			return
		}
	}

	l.prev = op
	switch op.Opcode {
	case AddSP:
		if prev.Opcode == AddSP {
			i := len(l.out.Code) - 1
			l.out.Code[i].N += op.N
			if l.out.Code[i].N == 0 {
				l.out.Code = l.out.Code[:i]
			}
			break
		}

		l.out.Code = append(l.out.Code, op)
	case Return:
		switch {
		case prev.Opcode == AddSP:
			l.out.Code[len(l.out.Code)-1] = op
		default:
			l.out.Code = append(l.out.Code, op)
		}
	case Label:
		// nop
	default:
		l.out.Code = append(l.out.Code, op)
	}
}

func (l *loader) emit(li PCInfo, op ...Operation) {
	if li.Line != 0 {
		li.Column = 1
		if n := len(l.out.Lines); n == 0 || l.out.Lines[n-1].Line != li.Line || l.out.Lines[n-1].Name != li.Name {
			l.out.Lines = append(l.out.Lines, li)
		}
	}
	for _, v := range op {
		l.emitOne(v)
	}
}

func (l *loader) sizeof(tid ir.TypeID) int {
	sz := l.model.Sizeof(l.tc.MustType(tid))
	if sz > mathutil.MaxInt {
		panic(fmt.Errorf("size of %s out of limits", tid))
	}

	return int(sz)
}

func (l *loader) stackSize(tid ir.TypeID) int { return roundup(l.sizeof(tid), l.stackAlign) }

func (l *loader) text(s ir.StringID) int {
	if p, ok := l.strings[s]; ok {
		return p
	}

	p := len(l.out.Text)
	l.strings[s] = p
	l.out.Text = append(l.out.Text, dict.S(int(s))...)
	sz := roundup(len(l.out.Text)+1, mallocAlign)
	l.out.Text = append(l.out.Text, make([]byte, sz-len(l.out.Text))...)
	return p
}

func (l *loader) pos(op ir.Operation) PCInfo {
	p := op.Pos()
	if !p.IsValid() {
		return PCInfo{}
	}

	return PCInfo{PC: len(l.out.Code), Line: p.Line, Column: p.Column, Name: ir.NameID(dict.SID(p.Filename))}
}

func (l *loader) ip() int { return len(l.out.Code) }

func (l *loader) int32(x ir.Operation, n int32) {
	switch {
	case n == 0:
		l.emit(l.pos(x), Operation{Opcode: Zero32})
	default:
		l.emit(l.pos(x), Operation{Opcode: Int32, N: int(n)})
	}
}

func (l *loader) int64(x ir.Operation, n int64) {
	if n == 0 {
		l.emit(l.pos(x), Operation{Opcode: Zero64})
		return
	}

	switch intSize {
	case 4:
		l.emit(l.pos(x), Operation{Opcode: Int64, N: int(n)})
		l.emit(l.pos(x), Operation{Opcode: Ext, N: int(n >> 32)})
	case 8:
		l.emit(l.pos(x), Operation{Opcode: Int64, N: int(n)})
	default:
		panic("internal error")
	}
}

func (l *loader) uintptr32(x ir.Operation, n int32) {
	switch l.ptrSize {
	default:
	case 4:
		l.int32(x, n)
	case 8:
		l.int64(x, int64(n))
	}
}

func (l *loader) float32(x ir.Operation, n float32) {
	l.emit(l.pos(x), Operation{Opcode: Float32, N: int(math.Float32bits(n))})
}

func (l *loader) float64(x ir.Operation, n float64) {
	bits := math.Float64bits(n)
	switch intSize {
	case 4:
		l.emit(l.pos(x), Operation{Opcode: Float64, N: int(bits)})
		l.emit(l.pos(x), Operation{Opcode: Ext, N: int(bits >> 32)})
	case 8:
		l.emit(l.pos(x), Operation{Opcode: Float64, N: int(bits)})
	default:
		panic("internal error")
	}
}

func (l *loader) int32Literal(dest []byte, t ir.Type, lit int32) {
	switch t.Kind() {
	case ir.Int8:
		*(*int8)((unsafe.Pointer)(&dest[0])) = int8(lit)
	case ir.Int32:
		*(*int32)((unsafe.Pointer)(&dest[0])) = lit
	case ir.Int64:
		*(*int64)((unsafe.Pointer)(&dest[0])) = int64(lit)
	default:
		panic(fmt.Errorf("TODO %s", t.Kind()))
	}
}

func (l *loader) arrayLiteral(t ir.Type, v ir.Value) *[]byte {
	p := buffer.CGet(l.sizeof(t.ID()))
	b := *p
	item := t.(*ir.ArrayType).Item
	itemSz := l.sizeof(item.ID())
	switch x := v.(type) {
	case *ir.CompositeValue:
		i := 0 // Item index
		for _, v := range x.Values {
			switch y := v.(type) {
			case *ir.Int32Value:
				l.int32Literal(b[i*itemSz:], item, y.Value)
				i++
			case *ir.CompositeValue:
				l.compositeValue(b[i*itemSz:], item, y)
				i++
			default:
				panic(fmt.Errorf("TODO %T", y))
			}
		}
	default:
		panic(fmt.Errorf("TODO %T", x))
	}
	return p
}

func (l *loader) compositeValue(dest []byte, t ir.Type, lit ir.Value) {
	var p *[]byte
	switch t.Kind() {
	case ir.Array:
		p = l.arrayLiteral(t, lit)
	case ir.Struct:
		p = l.structLiteral(t, lit)
	default:
		panic(fmt.Errorf("TODO %s", t.Kind()))
	}
	copy(dest, *p)
	buffer.Put(p)
}

func (l *loader) structLiteral(t ir.Type, v ir.Value) *[]byte {
	st := t.(*ir.StructOrUnionType)
	fields := st.Fields
	layout := l.model.Layout(st)
	p := buffer.CGet(l.sizeof(t.ID()))
	b := *p
	switch x := v.(type) {
	case *ir.CompositeValue:
		i := 0 // field#
		for _, v := range x.Values {
			switch y := v.(type) {
			case nil:
				i++
			case *ir.Int32Value:
				l.int32Literal(b[layout[i].Offset:], fields[i], y.Value)
				i++
			case *ir.CompositeValue:
				l.compositeValue(b[layout[i].Offset:], fields[i], y)
				i++
			default:
				panic(fmt.Errorf("TODO %T", y))
			}
		}
	default:
		panic(fmt.Errorf("TODO %T", x))
	}
	return p
}

func (l *loader) compositeLiteral(tid ir.TypeID, v ir.Value) int {
	var p *[]byte
	switch t := l.tc.MustType(tid); t.Kind() {
	case ir.Array:
		p = l.arrayLiteral(t, v)
	case ir.Struct:
		p = l.structLiteral(t, v)
	default:
		panic(fmt.Errorf("TODO %s", t.Kind()))
	}

	r := l.text(ir.StringID(dict.ID(*p)))
	buffer.Put(p)
	return r
}

func (l *loader) loadFunctionDefinition(index int, f *ir.FunctionDefinition) {
	var (
		arguments []nfo
		labels    = map[int]int{}
		results   []nfo
		variables []nfo
	)

	t := l.tc.MustType(f.TypeID).(*ir.FunctionType)
	for _, v := range t.Arguments {
		arguments = append(arguments, nfo{sz: l.sizeof(v.ID())})
	}
	off := 0
	for i := range arguments {
		off -= roundup(arguments[i].sz, l.stackAlign)
		arguments[i].off = off
	}

	for _, v := range t.Results {
		results = append(results, nfo{sz: l.sizeof(v.ID())})
	}
	off = 0
	for i := len(results) - 1; i >= 0; i-- {
		results[i].off = off
		off += roundup(results[i].sz, stackAlign)
	}

	for _, v := range f.Body {
		switch x := v.(type) {
		case *ir.VariableDeclaration:
			variables = append(variables, nfo{sz: l.sizeof(x.TypeID)})
		}
	}
	off = 0
	for i := range variables {
		off -= roundup(variables[i].sz, l.stackAlign)
		variables[i].off = off
	}

	n := 0
	if m := len(variables); m != 0 {
		n = variables[m-1].off
	}
	fp := f.Position
	fi := PCInfo{PC: len(l.out.Code), Line: fp.Line, Column: len(arguments), Name: f.NameID}
	l.out.Functions = append(l.out.Functions, fi)
	l.emit(l.pos(f.Body[0]), Operation{Opcode: Func, N: n})
	ip0 := l.ip()
	for ip, v := range f.Body {
		switch x := v.(type) {
		case *ir.Add:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32, ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: AddI32})
			case ir.Int64, ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: AddI64})
			case ir.Float32:
				l.emit(l.pos(x), Operation{Opcode: AddF32})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: AddF64})
			case ir.Pointer:
				l.emit(l.pos(x), Operation{Opcode: AddPtrs})
			default:
				panic(fmt.Errorf("internal error %s", x.TypeID))
			}
		case *ir.AllocResult:
			l.emit(l.pos(x), Operation{Opcode: AddSP, N: -l.stackSize(x.TypeID)})
		case *ir.And:
			switch l.sizeof(x.TypeID) {
			case 1:
				l.emit(l.pos(x), Operation{Opcode: And8})
			case 4:
				l.emit(l.pos(x), Operation{Opcode: And32})
			case 8:
				l.emit(l.pos(x), Operation{Opcode: And64})
			default:
				panic(fmt.Errorf("internal error %s", x.TypeID))
			}
		case *ir.Argument:
			switch {
			case x.Address:
				l.emit(l.pos(x), Operation{Opcode: AP, N: arguments[x.Index].off})
			default:
				switch val := arguments[x.Index]; val.sz {
				case 1:
					l.emit(l.pos(x), Operation{Opcode: Argument8, N: val.off})
				case 2:
					l.emit(l.pos(x), Operation{Opcode: Argument16, N: val.off})
				case 4:
					l.emit(l.pos(x), Operation{Opcode: Argument32, N: val.off})
				case 8:
					l.emit(l.pos(x), Operation{Opcode: Argument64, N: val.off})
				case 16:
					panic(fmt.Errorf("%s: internal error %v", x.Position, val))
				case 32:
					panic(fmt.Errorf("%s: internal error %v", x.Position, val))
				default:
					l.emit(l.pos(x),
						Operation{Opcode: Argument, N: val.off},
						Operation{Opcode: Ext, N: val.sz},
					)
				}
			}
		case *ir.Arguments:
			switch {
			case x.FunctionPointer:
				l.emit(l.pos(x), Operation{Opcode: ArgumentsFP})
			default:
				l.emit(l.pos(x), Operation{Opcode: Arguments})
			}
		case *ir.BeginScope:
			// nop
		case *ir.Bool:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int8:
				l.emit(l.pos(x), Operation{Opcode: BoolI8})
			case ir.Int32, ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: BoolI32})
			case ir.Int64, ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: BoolI64})
			case ir.Pointer:
				switch l.ptrSize {
				case 4:
					l.emit(l.pos(x), Operation{Opcode: BoolI32})
				case 8:
					l.emit(l.pos(x), Operation{Opcode: BoolI64})
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, l.ptrSize))
				}
			default:
				panic(fmt.Errorf("%s: TODO %v", x.Position, t.Kind()))
			}
		case *ir.Call:
			if opcode, ok := builtins[l.objects[x.Index].(*ir.FunctionDefinition).NameID]; ok {
				l.emit(l.pos(x), Operation{Opcode: opcode})
				break
			}

			l.emit(l.pos(x), Operation{Opcode: Call, N: x.Index})
		case *ir.CallFP:
			l.emit(l.pos(x), Operation{Opcode: CallFP})
		case *ir.Convert:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int8:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: ConvI8I32})
				case ir.Int64:
					l.emit(l.pos(x), Operation{Opcode: ConvI8I64})
				default:
					panic(fmt.Errorf("TODO %v", u.Kind()))
				}
			case ir.Uint8:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: ConvU8I32})
				case ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: ConvU8U32})
				default:
					panic(fmt.Errorf("TODO %v", u.Kind()))
				}
			case ir.Int16:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: ConvI16I32})
				case ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: ConvI16U32})
				default:
					panic(fmt.Errorf("TODO %v", u.Kind()))
				}
			case ir.Int32:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int8, ir.Uint8:
					l.emit(l.pos(x), Operation{Opcode: ConvI32I8})
				case ir.Int16, ir.Uint16:
					l.emit(l.pos(x), Operation{Opcode: ConvI32I16})
				case ir.Int32, ir.Uint32:
					// ok
				case ir.Int64, ir.Uint64:
					l.emit(l.pos(x), Operation{Opcode: ConvI32I64})
				case ir.Float32:
					l.emit(l.pos(x), Operation{Opcode: ConvI32F32})
				case ir.Float64:
					l.emit(l.pos(x), Operation{Opcode: ConvI32F64})
				case ir.Pointer:
					switch l.ptrSize {
					case 4:
						// ok
					case 8:
						l.emit(l.pos(x), Operation{Opcode: ConvI32I64})
					default:
						panic(fmt.Errorf("%s: TODO %v", x.Position, l.ptrSize))
					}
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, u.Kind()))
				}
			case ir.Uint16:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: ConvU16I32})
				case ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: ConvU16U32})
				case ir.Uint64:
					l.emit(l.pos(x), Operation{Opcode: ConvU16I64})
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, u.Kind()))
				}
			case ir.Uint32:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Uint8:
					l.emit(l.pos(x), Operation{Opcode: ConvU32U8})
				case ir.Int32:
					// ok
				case ir.Int64:
					l.emit(l.pos(x), Operation{Opcode: ConvU32I64})
				case ir.Pointer:
					switch l.ptrSize {
					case 4:
						// ok
					case 8:
						l.emit(l.pos(x), Operation{Opcode: ConvU32I64})
					}
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, u.Kind()))
				}
			case ir.Int64:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Uint16:
					l.emit(l.pos(x), Operation{Opcode: ConvI64U16})
				case ir.Int32, ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: ConvI64I32})
				case ir.Uint64:
					// ok
				case ir.Pointer:
					switch l.ptrSize {
					case 4:
						l.emit(l.pos(x), Operation{Opcode: ConvI64I32})
					case 8:
						// ok
					}
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, u.Kind()))
				}
			case ir.Uint64:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int32, ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: ConvI64I32})
				case ir.Int64, ir.Uint64:
					// ok
				default:
					panic(fmt.Errorf("TODO %v", u.Kind()))
				}
			case ir.Float32:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: ConvF32I32})
				case ir.Float64:
					l.emit(l.pos(x), Operation{Opcode: ConvF32F64})
				default:
					panic(fmt.Errorf("TODO %v", u.Kind()))
				}
			case ir.Float64:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int8:
					l.emit(l.pos(x), Operation{Opcode: ConvF64I8})
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: ConvF64I32})
				case ir.Float32:
					l.emit(l.pos(x), Operation{Opcode: ConvF64F32})
				case ir.Float64:
					// ok
				default:
					panic(fmt.Errorf("TODO %v", u.Kind()))
				}
			case ir.Pointer:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int64:
					switch l.ptrSize {
					case 4:
						l.emit(l.pos(x), Operation{Opcode: ConvU32I64})
					case 8:
						// ok
					}
				case ir.Pointer:
					// ok
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, u.Kind()))
				}
			default:
				panic(fmt.Errorf("%s: TODO %v", x.Position, t.Kind()))
			}
		case *ir.Copy:
			l.emit(l.pos(x), Operation{Opcode: Copy, N: l.sizeof(x.TypeID)})
		case *ir.Div:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32:
				l.emit(l.pos(x), Operation{Opcode: DivI32})
			case ir.Int64:
				l.emit(l.pos(x), Operation{Opcode: DivI64})
			case ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: DivU64})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: DivF64})
			default:
				panic(fmt.Errorf("internal error %s", x.TypeID))
			}
		case *ir.Drop:
			l.emit(l.pos(x), Operation{Opcode: AddSP, N: l.stackSize(x.TypeID)})
		case *ir.Dup:
			switch l.sizeof(x.TypeID) {
			case 4:
				l.emit(l.pos(x), Operation{Opcode: Dup32})
			case 8:
				l.emit(l.pos(x), Operation{Opcode: Dup64})
			default:
				panic(fmt.Errorf("internal error %s %v", x.TypeID, l.sizeof(x.TypeID)))
			}
		case *ir.Element:
			t := l.tc.MustType(x.TypeID).(*ir.PointerType).Element
			sz := l.sizeof(t.ID())
			xt := l.tc.MustType(x.IndexType)
			switch {
			case x.Neg:
				switch xt.Kind() {
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: NegIndexI32, N: sz})
				case ir.Uint64:
					l.emit(l.pos(x), Operation{Opcode: NegIndexU64, N: sz})
				default:
					panic(fmt.Errorf("TODO %v", xt.Kind()))
				}
				if !x.Address {
					panic(fmt.Errorf("TODO %v", xt.Kind()))
				}
			default:
				switch xt.Kind() {
				case ir.Int16:
					l.emit(l.pos(x), Operation{Opcode: IndexI16, N: sz})
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: IndexI32, N: sz})
				case ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: IndexU32, N: sz})
				case ir.Uint64:
					l.emit(l.pos(x), Operation{Opcode: IndexU64, N: sz})
				default:
					panic(fmt.Errorf("TODO %v", xt.Kind()))
				}
				if !x.Address {
					switch sz {
					case 1:
						l.emit(l.pos(x), Operation{Opcode: Load8})
					case 2:
						l.emit(l.pos(x), Operation{Opcode: Load16})
					case 4:
						l.emit(l.pos(x), Operation{Opcode: Load32})
					case 8:
						l.emit(l.pos(x), Operation{Opcode: Load64})
					default:
						panic(fmt.Errorf("%s: TODO %v", x.Position, sz))
					}
				}
			}
		case *ir.EndScope:
			// nop
		case *ir.Eq:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32:
				l.emit(l.pos(x), Operation{Opcode: EqI32})
			case ir.Int64, ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: EqI64})
			case ir.Pointer:
				switch l.ptrSize {
				case 4:
					l.emit(l.pos(x), Operation{Opcode: EqI32})
				case 8:
					l.emit(l.pos(x), Operation{Opcode: EqI64})
				default:
					panic(fmt.Errorf("internal error %s", x.TypeID))
				}
			default:
				panic(fmt.Errorf("TODO %v", t.Kind()))
			}
		case *ir.Global:
			switch ex := l.objects[x.Index].(type) {
			case *ir.DataDefinition:
				switch {
				case x.Address:
					l.emit(l.pos(x), Operation{Opcode: DS, N: l.m[x.Index]})
				default:
					switch t := l.tc.MustType(x.TypeID); t.Kind() {
					case ir.Int16, ir.Uint16:
						l.emit(l.pos(x), Operation{Opcode: DSI16, N: l.m[x.Index]})
					case ir.Int32:
						l.emit(l.pos(x), Operation{Opcode: DSI32, N: l.m[x.Index]})
					case ir.Int64, ir.Uint64, ir.Float64:
						l.emit(l.pos(x), Operation{Opcode: DSI64, N: l.m[x.Index]})
					case ir.Pointer:
						switch l.ptrSize {
						case 4:
							l.emit(l.pos(x), Operation{Opcode: DSI32, N: l.m[x.Index]})
						case 8:
							l.emit(l.pos(x), Operation{Opcode: DSI64, N: l.m[x.Index]})
						default:
							panic(fmt.Errorf("internal error %s, %v", x.TypeID, l.ptrSize))
						}
					default:
						panic(fmt.Errorf("%s: TODO %v", x.Position, t.Kind()))
					}
				}
			default:
				panic(fmt.Errorf("TODO %T(%v)", ex, ex))
			}
		case *ir.Field:
			if x.Bits != 0 {
				panic(fmt.Errorf("%#05x\t%s:%s: internal error", ip, f.NameID, x.Position))
			}

			fields := l.model.Layout(l.tc.MustType(x.TypeID).(*ir.PointerType).Element.(*ir.StructOrUnionType))
			switch {
			case x.Address:
				if n := int(fields[x.Index].Offset); n != 0 {
					l.emit(l.pos(x), Operation{Opcode: AddPtr, N: n})
				}
			default:
				switch sz := fields[x.Index].Size; sz {
				case 1:
					l.emit(l.pos(x), Operation{Opcode: Load8, N: int(fields[x.Index].Offset)})
				case 2:
					l.emit(l.pos(x), Operation{Opcode: Load16, N: int(fields[x.Index].Offset)})
				case 4:
					l.emit(l.pos(x), Operation{Opcode: Load32, N: int(fields[x.Index].Offset)})
				case 8:
					l.emit(l.pos(x), Operation{Opcode: Load64, N: int(fields[x.Index].Offset)})
				default:
					l.emit(l.pos(x),
						Operation{Opcode: Load, N: int(fields[x.Index].Offset)},
						Operation{Opcode: Ext, N: int(sz)},
					)
				}
			}
		case *ir.Geq:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32:
				l.emit(l.pos(x), Operation{Opcode: GeqI32})
			case ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: GeqU32})
			case ir.Int64:
				l.emit(l.pos(x), Operation{Opcode: GeqI64})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: GeqF64})
			case ir.Pointer:
				switch l.ptrSize {
				case 4:
					l.emit(l.pos(x), Operation{Opcode: GeqU32})
				case 8:
					l.emit(l.pos(x), Operation{Opcode: GeqU64})
				default:
					panic(fmt.Errorf("%s: internal error %v", x.Position, l.ptrSize))
				}
			default:
				panic(fmt.Errorf("%s: TODO %v", x.Position, t.Kind()))
			}
		case *ir.Gt:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32:
				l.emit(l.pos(x), Operation{Opcode: GtI32})
			case ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: GtU32})
			case ir.Int64:
				l.emit(l.pos(x), Operation{Opcode: GtI64})
			case ir.Pointer:
				switch l.ptrSize {
				case 4:
					l.emit(l.pos(x), Operation{Opcode: GtU32})
				case 8:
					l.emit(l.pos(x), Operation{Opcode: GtU64})
				default:
					panic(fmt.Errorf("%s: internal error %v", x.Position, l.ptrSize))
				}
			default:
				panic(fmt.Errorf("%s: TODO %v", x.Position, t.Kind()))
			}
		case *ir.Const32:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int8, ir.Uint8, ir.Uint16, ir.Int32, ir.Uint32:
				l.int32(x, x.Value)
			case ir.Pointer:
				l.uintptr32(x, x.Value)
			default:
				panic(fmt.Errorf("TODO %v", t.Kind()))
			}
		case *ir.Const64:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int64, ir.Uint64:
				l.int64(x, x.Value)
			case ir.Float64:
				l.float64(x, math.Float64frombits(uint64(x.Value)))
			default:
				panic(fmt.Errorf("TODO %v", t.Kind()))
			}
		case *ir.Jmp:
			n := -int(x.NameID)
			if n == 0 {
				n = x.Number
			}
			l.emit(l.pos(x), Operation{Opcode: Jmp, N: n})
		case *ir.Jnz:
			n := -int(x.NameID)
			if n == 0 {
				n = x.Number
			}
			l.emit(l.pos(x), Operation{Opcode: Jnz, N: n})
		case *ir.Jz:
			n := -int(x.NameID)
			if n == 0 {
				n = x.Number
			}
			l.emit(l.pos(x), Operation{Opcode: Jz, N: n})
		case *ir.Label:
			n := -int(x.NameID)
			if n == 0 {
				n = x.Number
			}
			labels[n] = len(l.out.Code)
			l.emit(l.pos(x), Operation{Opcode: Label, N: n})
		case *ir.Leq:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32:
				l.emit(l.pos(x), Operation{Opcode: LeqI32})
			case ir.Pointer:
				switch l.ptrSize {
				case 4:
					l.emit(l.pos(x), Operation{Opcode: LeqU32})
				case 8:
					l.emit(l.pos(x), Operation{Opcode: LeqU64})
				default:
					panic("internal error")
				}
			default:
				panic(fmt.Errorf("TODO %v", t.Kind()))
			}
		case *ir.Load:
			if x.Bits != 0 {
				panic(fmt.Errorf("%#05x\t%s:%s: internal error", ip, f.NameID, x.Position))
			}

			switch l.sizeof(l.tc.MustType(x.TypeID).(*ir.PointerType).Element.ID()) {
			case 1:
				l.emit(l.pos(x), Operation{Opcode: Load8})
			case 2:
				l.emit(l.pos(x), Operation{Opcode: Load16})
			case 4:
				l.emit(l.pos(x), Operation{Opcode: Load32})
			case 8:
				l.emit(l.pos(x), Operation{Opcode: Load64})
			default:
				panic(fmt.Errorf("%s: internal error %s", x.Position, x.TypeID))
			}
		case *ir.Lsh:
			switch l.sizeof(x.TypeID) {
			case 4:
				l.emit(l.pos(x), Operation{Opcode: LshI32})
			case 8:
				l.emit(l.pos(x), Operation{Opcode: LshI64})
			default:
				panic(fmt.Errorf("%s: internal error %s", x.Position, x.TypeID))
			}
		case *ir.Lt:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32:
				l.emit(l.pos(x), Operation{Opcode: LtI32})
			case ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: LtU32})
			case ir.Int64:
				l.emit(l.pos(x), Operation{Opcode: LtI64})
			case ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: LtU64})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: LtF64})
			case ir.Pointer:
				l.emit(l.pos(x), Operation{Opcode: LtU64})
			default:
				panic(fmt.Errorf("%s: TODO %v", x.Position, t.Kind()))
			}
		case *ir.Mul:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32, ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: MulI32})
			case ir.Int64, ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: MulI64})
			case ir.Float32:
				l.emit(l.pos(x), Operation{Opcode: MulF32})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: MulF64})
			case ir.Pointer:
				switch l.ptrSize {
				case 4:
					l.emit(l.pos(x), Operation{Opcode: MulI32})
				case 8:
					l.emit(l.pos(x), Operation{Opcode: MulI64})
				default:
					panic(fmt.Errorf("internal error %s", x.TypeID))
				}
			default:
				panic(fmt.Errorf("internal error %s", x.TypeID))
			}
		case *ir.Nil:
			switch l.ptrSize {
			case 4:
				l.emit(l.pos(x), Operation{Opcode: Zero32})
			case 8:
				l.emit(l.pos(x), Operation{Opcode: Zero64})
			default:
				panic(fmt.Errorf("internal error %s", x.TypeID))
			}
		case *ir.Neg:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32, ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: NegI32})
			default:
				panic(fmt.Errorf("%s: TODO %v", x.Position, t.Kind()))
			}
		case *ir.Neq:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32, ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: NeqI32})
			case ir.Int64, ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: NeqI64})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: NeqF64})
			case ir.Pointer:
				switch l.ptrSize {
				case 4:
					l.emit(l.pos(x), Operation{Opcode: NeqI32})
				case 8:
					l.emit(l.pos(x), Operation{Opcode: NeqI64})
				default:
					panic(fmt.Errorf("internal error %s", x.TypeID))
				}
			default:
				panic(fmt.Errorf("%s: TODO %v", x.Position, t.Kind()))
			}
		case *ir.Not:
			l.emit(l.pos(x), Operation{Opcode: Not})
		case *ir.Or:
			switch l.sizeof(x.TypeID) {
			case 4:
				l.emit(l.pos(x), Operation{Opcode: Or32})
			default:
				panic(fmt.Errorf("internal error %s", x.TypeID))
			}
		case *ir.Panic:
			l.emit(l.pos(x), Operation{Opcode: Panic})
		case *ir.PostIncrement:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int8:
				l.emit(l.pos(x), Operation{Opcode: PostIncI8, N: x.Delta})
			case ir.Int32, ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: PostIncI32, N: x.Delta})
			case ir.Int64:
				l.emit(l.pos(x), Operation{Opcode: PostIncI64, N: x.Delta})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: PostIncF64, N: x.Delta})
			case ir.Pointer:
				l.emit(l.pos(x), Operation{Opcode: PostIncPtr, N: x.Delta})
			default:
				panic(fmt.Errorf("TODO %v", t.Kind()))
			}
		case *ir.PreIncrement:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32:
				l.emit(l.pos(x), Operation{Opcode: PreIncI32, N: x.Delta})
			case ir.Pointer:
				l.emit(l.pos(x), Operation{Opcode: PreIncPtr, N: x.Delta})
			default:
				panic(fmt.Errorf("%s: TODO %v", x.Position, t.Kind()))
			}
		case *ir.PtrDiff:
			sz := l.sizeof(l.tc.MustType(x.PtrType).(*ir.PointerType).Element.ID())
			l.emit(l.pos(x), Operation{Opcode: PtrDiff, N: sz})
		case *ir.Rem:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32:
				l.emit(l.pos(x), Operation{Opcode: RemI32})
			case ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: RemU64})
			default:
				panic(fmt.Errorf("%s: internal error %s", x.Position, x.TypeID))
			}
		case *ir.Result:
			var r nfo
			switch {
			case len(results) == 0 && x.Index == 0:
				// nop
			default:
				r = results[x.Index]
			}
			switch {
			case x.Address:
				l.emit(l.pos(x), Operation{Opcode: AP, N: r.off})
			default:
				panic("TODO")
			}
		case *ir.Return:
			l.emit(l.pos(x), Operation{Opcode: Return})
		case *ir.Rsh:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int8:
				l.emit(l.pos(x), Operation{Opcode: RshI8})
			case ir.Int64:
				l.emit(l.pos(x), Operation{Opcode: RshI64})
			default:
				panic(fmt.Errorf("%s: internal error %s", x.Position, x.TypeID))
			}
		case *ir.Store:
			if x.Bits != 0 {
				if x.BitOffset != 0 {
					l.emit(l.pos(x), Operation{Opcode: Int32, N: x.BitOffset})
					switch t := l.tc.MustType(x.TypeID); t.Kind() {
					case ir.Int32:
						l.emit(l.pos(x), Operation{Opcode: LshI32})
					default:
						panic(fmt.Errorf("%s: internal error %s", x.Position, x.TypeID))
					}
				}
				mask := (uint64(1)<<uint(x.Bits) - 1) << uint(x.BitOffset)
				switch l.sizeof(x.BitFieldType) {
				case 1:
					l.emit(l.pos(x), Operation{Opcode: StoreBits8, N: int(mask)})
				case 4:
					l.emit(l.pos(x), Operation{Opcode: StoreBits32, N: int(mask)})
				default:
					panic(fmt.Errorf("%s: internal error %s", x.Position, x.BitFieldType))
				}
				break
			}

			switch sz := l.sizeof(x.TypeID); sz {
			case 1:
				l.emit(l.pos(x), Operation{Opcode: Store8})
			case 2:
				l.emit(l.pos(x), Operation{Opcode: Store16})
			case 4:
				l.emit(l.pos(x), Operation{Opcode: Store32})
			case 8:
				l.emit(l.pos(x), Operation{Opcode: Store64})
			default:
				l.emit(l.pos(x), Operation{Opcode: Store, N: sz})
			}
		case *ir.StringConst:
			l.emit(l.pos(x), Operation{Opcode: Text, N: l.text(x.Value)})
		case *ir.Sub:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32, ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: SubI32})
			case ir.Int64, ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: SubI64})
			case ir.Float32:
				l.emit(l.pos(x), Operation{Opcode: SubF32})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: SubF64})
			default:
				panic(fmt.Errorf("internal error %s", x.TypeID))
			}
		case *ir.Variable:
			switch {
			case x.Address:
				l.emit(l.pos(x), Operation{Opcode: BP, N: variables[x.Index].off})
			default:
				switch val := variables[x.Index]; val.sz {
				case 1:
					l.emit(l.pos(x), Operation{Opcode: Variable8, N: val.off})
				case 2:
					l.emit(l.pos(x), Operation{Opcode: Variable16, N: val.off})
				case 4:
					l.emit(l.pos(x), Operation{Opcode: Variable32, N: val.off})
				case 8:
					l.emit(l.pos(x), Operation{Opcode: Variable64, N: val.off})
				default:
					l.emit(l.pos(x),
						Operation{Opcode: Variable, N: val.off},
						Operation{Opcode: Ext, N: val.sz},
					)
				}
			}
		case *ir.VariableDeclaration:
			switch v := x.Value.(type) {
			case nil:
				// nop
			case *ir.AddressValue:
				l.emit(l.pos(x), Operation{Opcode: BP, N: variables[x.Index].off})
				switch v.Linkage {
				case ir.ExternalLinkage:
					switch ex := l.objects[v.Index].(type) {
					case *ir.DataDefinition:
						switch {
						case ex.Value != nil:
							panic("TODO")
						default:
							l.emit(l.pos(x), Operation{Opcode: DS, N: l.m[v.Index] + len(l.out.Data)})
							switch l.ptrSize {
							case 4:
								l.emit(l.pos(x), Operation{Opcode: Store32})
							case 8:
								l.emit(l.pos(x), Operation{Opcode: Store64})
							default:
								panic("internal error")
							}
							l.emit(l.pos(x), Operation{Opcode: AddSP, N: l.ptrSize})
						}
					default:
						panic(fmt.Errorf("%s.%05x: TODO %T(%v)", f.NameID, ip, ex, ex))
					}
				case ir.InternalLinkage:
					panic(fmt.Errorf("%s.%05x: TODO %T(%v)", f.NameID, ip, v, v))
				default:
					panic(fmt.Errorf("%s.%05x: internal error %T(%v)", f.NameID, ip, v, v))
				}
			case *ir.Int32Value:
				l.emit(l.pos(x), Operation{Opcode: BP, N: variables[x.Index].off})
				switch t := l.tc.MustType(x.TypeID); t.Kind() {
				case ir.Int8, ir.Uint8:
					l.int32(x, v.Value)
					l.emit(l.pos(x), Operation{Opcode: Store8})
				case ir.Int16, ir.Uint16:
					l.int32(x, v.Value)
					l.emit(l.pos(x), Operation{Opcode: Store16})
				case ir.Int32, ir.Uint32:
					l.int32(x, v.Value)
					l.emit(l.pos(x), Operation{Opcode: Store32})
				case ir.Int64, ir.Uint64:
					l.int32(x, v.Value)
					l.emit(l.pos(x), Operation{Opcode: Store64})
				case ir.Float32:
					l.float32(x, float32(v.Value))
					l.emit(l.pos(x), Operation{Opcode: Store32})
				case ir.Pointer:
					if v.Value == 0 {
						switch l.ptrSize {
						case 4:
							l.emit(l.pos(x), Operation{Opcode: Zero32})
							l.emit(l.pos(x), Operation{Opcode: Store32})
						case 8:
							l.emit(l.pos(x), Operation{Opcode: Zero64})
							l.emit(l.pos(x), Operation{Opcode: Store64})
						default:
							panic(fmt.Errorf("internal error %s", x.TypeID))
						}
						break
					}

					panic(fmt.Errorf("%s: %v", x.Position, x.TypeID))
				default:
					panic(fmt.Errorf("%s: %v", x.Position, x.TypeID))
				}
				l.emit(l.pos(x), Operation{Opcode: AddSP, N: l.stackSize(x.TypeID)})
			case *ir.Int64Value:
				l.emit(l.pos(x), Operation{Opcode: BP, N: variables[x.Index].off})
				switch t := l.tc.MustType(x.TypeID); t.Kind() {
				case ir.Int64, ir.Uint64:
					l.int64(x, v.Value)
					l.emit(l.pos(x), Operation{Opcode: Store64})
				default:
					panic(fmt.Errorf("%s: %v", x.Position, x.TypeID))
				}
				l.emit(l.pos(x), Operation{Opcode: AddSP, N: l.stackSize(x.TypeID)})
			case *ir.Float64Value:
				l.emit(l.pos(x), Operation{Opcode: BP, N: variables[x.Index].off})
				switch t := l.tc.MustType(x.TypeID); t.Kind() {
				case ir.Int8:
					l.int32(x, int32(v.Value))
					l.emit(l.pos(x), Operation{Opcode: Store8})
				case ir.Int32:
					l.int32(x, int32(v.Value))
					l.emit(l.pos(x), Operation{Opcode: Store32})
				case ir.Float32:
					l.float32(x, float32(v.Value))
					l.emit(l.pos(x), Operation{Opcode: Store32})
				default:
					panic(fmt.Errorf("%s: %v", x.Position, x.TypeID))
				}
				l.emit(l.pos(x), Operation{Opcode: AddSP, N: l.stackSize(x.TypeID)})
			case *ir.StringValue:
				switch vt := l.tc.MustType(x.TypeID); {
				case vt.Kind() == ir.Array:
					l.emit(l.pos(x), Operation{Opcode: BP, N: variables[x.Index].off})
					l.emit(l.pos(x), Operation{Opcode: Text, N: l.text(v.StringID)})
					l.emit(l.pos(x), Operation{Opcode: StrNCopy, N: l.sizeof(x.TypeID)})
				default:
					l.emit(l.pos(x), Operation{Opcode: BP, N: variables[x.Index].off})
					l.emit(l.pos(x), Operation{Opcode: Text, N: l.text(v.StringID)})
					switch l.ptrSize {
					case 4:
						l.emit(l.pos(x), Operation{Opcode: Store32})
					case 8:
						l.emit(l.pos(x), Operation{Opcode: Store64})
					default:
						panic("internal error")
					}
					l.emit(l.pos(x), Operation{Opcode: AddSP, N: l.ptrSize})
				}
			case *ir.CompositeValue:
				l.emit(l.pos(x), Operation{Opcode: BP, N: variables[x.Index].off})
				l.emit(l.pos(x), Operation{Opcode: Text, N: l.compositeLiteral(x.TypeID, x.Value)})
				l.emit(l.pos(x), Operation{Opcode: Copy, N: l.sizeof(x.TypeID)})
				l.emit(l.pos(x), Operation{Opcode: AddSP, N: l.ptrSize})
			default:
				panic(fmt.Errorf("%05x: TODO %T(%v)", ip, v, v))
			}
		case *ir.Xor:
			switch l.sizeof(x.TypeID) {
			case 4:
				l.emit(l.pos(x), Operation{Opcode: Xor32})
			default:
				panic(fmt.Errorf("internal error %s", x.TypeID))
			}
		default:
			panic(fmt.Errorf("TODO %T\n\t%#05x\t%v", x, ip, x))
		}
	}
	for i, v := range l.out.Code[ip0:] {
		switch v.Opcode {
		case Jmp, Jnz, Jz:
			n, ok := labels[v.N]
			if !ok {
				switch {
				case n < 0:
					panic(fmt.Errorf("%#x: internal error: undefined label %v", ip0+i, ir.NameID(-n)))
				default:
					panic(fmt.Errorf("%#x: internal error: undefined label %v", ip0+i, n))
				}
			}
			l.out.Code[ip0+i].N = labels[v.N]
		}
	}
}

func (l *loader) load() {
	var ds int
	for i, v := range l.objects { // Allocate global initialized data.
		switch x := v.(type) {
		case *ir.DataDefinition:
			if x.Value != nil {
				l.m[i] = ds
				ds += roundup(l.sizeof(x.TypeID), mallocAlign)
			}
		}
	}
	for i, v := range l.objects { // Allocate global zero-initialized data.
		switch x := v.(type) {
		case *ir.DataDefinition:
			if x.Value == nil {
				l.m[i] = ds
				sz := roundup(l.sizeof(x.TypeID), mallocAlign)
				ds += sz
				l.out.BSS += sz
			}
		}
	}
	for i, v := range l.objects {
		switch x := v.(type) {
		case *ir.FunctionDefinition:
			if _, ok := builtins[x.NameID]; ok {
				break
			}

			l.m[i] = len(l.out.Code)
			l.loadFunctionDefinition(i, x)
		}
	}
	for i, v := range l.out.Code {
		switch v.Opcode {
		case Call:
			l.out.Code[i].N = l.m[v.N]
		}
	}
	l.out.Data = *buffer.CGet(ds - l.out.BSS)
	l.out.TSRelative = *buffer.CGet((len(l.out.Data) + 7) / 8)
	l.out.DSRelative = *buffer.CGet((len(l.out.Data) + 7) / 8)
	for i, v := range l.objects {
		switch x := v.(type) {
		case *ir.DataDefinition:
			if x.Value != nil {
				l.loadDataDefinition(x, l.m[i], x.Value)
			}
		}
	}
	h := -1
	for i, v := range l.out.TSRelative {
		if v != 0 {
			h = i
		}
	}
	l.out.TSRelative = l.out.TSRelative[:h+1]
	h = -1
	for i, v := range l.out.DSRelative {
		if v != 0 {
			h = i
		}
	}
	l.out.DSRelative = l.out.DSRelative[:h+1]
}

// LoadMain translates program in objects into a Binary or an error, if any.
func LoadMain(model string, objects []ir.Object) (_ *Binary, err error) {
	if !Testing {
		defer func() {
			switch x := recover().(type) {
			case nil:
				// nop
			default:
				err = fmt.Errorf("Load: %v", x)
			}
		}()
	}

	l := newLoader(model, objects)
	l.load()
	return l.out, nil
}
