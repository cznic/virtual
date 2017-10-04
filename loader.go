// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"go/token"
	"io"
	"math"
	"runtime"
	"sort"
	"strconv"
	tm "time"
	"unicode/utf16"
	"unsafe"

	"github.com/cznic/internal/buffer"
	"github.com/cznic/ir"
	"github.com/cznic/mathutil"
)

var (
	_ io.ReaderFrom = (*Binary)(nil)
	_ io.Writer     = (*counter)(nil)
	_ io.WriterTo   = (*Binary)(nil)
)

const (
	binaryVersion = 1 // Compatibility version of Binary.
	ffiProlog     = 2 // Call $+2, FFIReturn, Func, ...
)

var (
	builtins   = map[ir.NameID]Opcode{}
	magic      = []byte{0x03, 0x91, 0x7a, 0xef, 0x55, 0xad, 0xcc, 0xce}
	nonReturns = map[Opcode]struct{}{
		abort: {},
		exit:  {},
		Panic: {},
		JmpP:  {},
	}
	uu            = []byte("__")
	builtinPrefix = []byte("__builtin_")
)

func registerBuiltins(m map[int]Opcode) {
	for k, v := range m {
		nm := ir.NameID(k)
		if _, ok := builtins[nm]; ok {
			panic(fmt.Errorf("internal error %q", nm))
		}

		builtins[nm] = v
		if !bytes.HasPrefix(dict.S(k), uu) {
			nm := ir.NameID(dict.ID(append(builtinPrefix, dict.S(k)...)))
			if _, ok := builtins[nm]; ok {
				panic(fmt.Errorf("internal error %q", nm))
			}

			builtins[nm] = v
		}
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

type counter int64

func (c *counter) Write(b []byte) (int, error) {
	*c += counter(len(b))
	return len(b), nil
}

// Binary represents a loaded program image. It can be run via Exec.
type Binary struct {
	BSS        int
	Code       []Operation
	DSRelative []byte // Bit vector of data segment-relative pointers in Data.
	Data       []byte
	Functions  []PCInfo
	Lines      []PCInfo
	TSRelative []byte // Bit vector of text segment-relative pointers in Data.
	Text       []byte
	Sym        map[ir.NameID]int // External function: Code index.
}

func newBinary() *Binary {
	return &Binary{
		Sym: map[ir.NameID]int{},
	}
}

// ReadFrom reads b from r.
func (b *Binary) ReadFrom(r io.Reader) (n int64, err error) {
	var c counter
	*b = Binary{}
	b.Sym = map[ir.NameID]int{}

	r = io.TeeReader(r, &c)
	gr, err := gzip.NewReader(r)
	if err != nil {
		return 0, err
	}

	if len(gr.Header.Extra) < len(magic) || !bytes.Equal(gr.Header.Extra[:len(magic)], magic) {
		return int64(c), fmt.Errorf("unrecognized file format")
	}

	buf := gr.Header.Extra[len(magic):]
	a := bytes.Split(buf, []byte{'|'})
	if len(a) != 3 {
		return int64(c), fmt.Errorf("corrupted file")
	}

	if s := string(a[0]); s != runtime.GOOS {
		return int64(c), fmt.Errorf("invalid platform %q", s)
	}

	if s := string(a[1]); s != runtime.GOARCH {
		return int64(c), fmt.Errorf("invalid architecture %q", s)
	}

	v, err := strconv.ParseUint(string(a[2]), 10, 64)
	if err != nil {
		return int64(c), err
	}

	if v != binaryVersion {
		return int64(c), fmt.Errorf("invalid version number %v", v)
	}

	err = gob.NewDecoder(gr).Decode(b)
	return int64(c), err
}

// WriteTo writes b to w.
func (b *Binary) WriteTo(w io.Writer) (n int64, err error) {
	var c counter
	gw := gzip.NewWriter(io.MultiWriter(w, &c))
	gw.Header.Comment = "VM binary/executable"
	var buf buffer.Bytes
	buf.Write(magic)
	fmt.Fprintf(&buf, fmt.Sprintf("%s|%s|%v", runtime.GOOS, runtime.GOARCH, binaryVersion))
	gw.Header.Extra = buf.Bytes()
	buf.Close()
	gw.Header.ModTime = tm.Now()
	gw.Header.OS = 255 // Unknown OS.
	enc := gob.NewEncoder(gw)
	if err := enc.Encode(b); err != nil {
		return int64(c), err
	}

	if err := gw.Close(); err != nil {
		return int64(c), err
	}

	return int64(c), nil
}

type nfo struct {
	align int
	off   int
	sz    int
}

type labelNfo struct {
	index int // fn
	nm    ir.NameID
}

type loader struct {
	csLabels    map[int]*ir.AddressValue
	dsLabels    map[int]*ir.AddressValue
	m           map[int]int // Object #: {BSS,Code,Data,Text} index.
	model       ir.MemoryModel
	namedLabels map[labelNfo]int // nfo: ip
	objects     []ir.Object
	out         *Binary
	prev        Operation
	ptrSize     int
	stackAlign  int
	strings     map[ir.StringID]int
	switches    map[*ir.Switch]int
	tc          ir.TypeCache
	tsLabels    map[int]*ir.AddressValue
	wstrings    map[ir.StringID]int
}

func newLoader(objects []ir.Object) *loader {
	model, err := ir.NewMemoryModel()
	if err != nil {
		panic(err)
	}

	ptrItem := model[ir.Pointer]
	return &loader{
		csLabels:    map[int]*ir.AddressValue{},
		dsLabels:    map[int]*ir.AddressValue{},
		m:           map[int]int{},
		model:       model,
		namedLabels: map[labelNfo]int{},
		objects:     objects,
		out:         newBinary(),
		prev:        Operation{Opcode: -1},
		ptrSize:     int(ptrItem.Size),
		stackAlign:  int(ptrItem.Align),
		strings:     map[ir.StringID]int{},
		switches:    map[*ir.Switch]int{},
		tc:          ir.TypeCache{},
		tsLabels:    map[int]*ir.AddressValue{},
		wstrings:    map[ir.StringID]int{},
	}
}

func (l *loader) loadDataDefinition(d *ir.DataDefinition, off int, v ir.Value) {
	brk := off + l.sizeof(d.TypeID)

	malloc := func(dst, n int, b []byte) {
		copy(l.out.Data[brk:], b)
		l.out.DSRelative[dst>>3] |= 1 << uint(dst&7)
		*(*uintptr)(unsafe.Pointer(&l.out.Data[dst])) = uintptr(brk)
		brk += roundup(n, mallocAlign)
	}

	var f func(int, ir.TypeID, ir.Value)
	f = func(off int, t ir.TypeID, v ir.Value) {
		b := l.out.Data[off:]
		switch x := v.(type) {
		case nil:
			// nop
		case *ir.AddressValue:
			if x.Label != 0 {
				l.dsLabels[off] = x
				break
			}

			*(*uintptr)(unsafe.Pointer(&b[0])) = uintptr(l.m[x.Index]) + x.Offset
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
					f(off+i*int(itemSz), itemT.ID(), v)
					i++
				}
			case ir.Struct, ir.Union:
				i := 0
				st := typ.(*ir.StructOrUnionType)
				fields := st.Fields
				layout := l.model.Layout(st)
				for _, v := range x.Values {
					f(off+int(layout[i].Offset), fields[i].ID(), v)
					i++
				}
			case ir.Pointer:
				switch elem := typ.(*ir.PointerType).Element; elem.Kind() {
				case ir.Int8:
					var buf buffer.Bytes
					for _, v := range x.Values {
						switch x := v.(type) {
						case *ir.Int32Value:
							buf.WriteByte(byte(x.Value))
						default:
							panic(fmt.Errorf("%s: TODO %T: %v", d.Position, x, v))
						}
					}
					malloc(off, buf.Len(), buf.Bytes())
				case ir.Int32:
					var buf buffer.Bytes
					for _, v := range x.Values {
						switch x := v.(type) {
						case *ir.Int32Value:
							var b4 [4]byte
							*(*int32)(unsafe.Pointer(&b4)) = x.Value
							buf.Write(b4[:])
						default:
							panic(fmt.Errorf("%s: TODO %T: %v", d.Position, x, v))
						}
					}
					malloc(off, buf.Len(), buf.Bytes())
				case ir.Pointer:
					switch elem := elem.(*ir.PointerType).Element; elem.Kind() {
					case ir.Function:
						var buf buffer.Bytes
						for _, v := range x.Values {
							switch x := v.(type) {
							case *ir.AddressValue:
								if x.Label != 0 {
									panic("TODO")
								}

								switch x.Linkage {
								case ir.ExternalLinkage:
									switch ex := l.objects[x.Index].(type) {
									case *ir.FunctionDefinition:
										switch l.ptrSize {
										case 4:
											var b [4]byte
											*(*uintptr)(unsafe.Pointer(&b)) = uintptr(l.m[x.Index]) + x.Offset
											buf.Write(b[:])
										case 8:
											var b [8]byte
											*(*uintptr)(unsafe.Pointer(&b)) = uintptr(l.m[x.Index]) + x.Offset
											buf.Write(b[:])
										default:
											panic(fmt.Errorf("internal error %v", l.ptrSize))
										}
									default:
										panic(fmt.Errorf("internal error %T", ex))
									}
								default:
									panic(fmt.Errorf("internal error %v", x.Linkage))
								}
							default:
								panic(fmt.Errorf("%s: TODO %T: %v", d.Position, x, v))
							}
						}
						id := ir.StringID(dict.ID(buf.Bytes()))
						f(off, t, &ir.StringValue{StringID: id})
					default:
						panic(fmt.Errorf("%s: TODO %v %v %v: %v", d.Position, t, elem, elem.Kind(), v))
					}
				default:
					panic(fmt.Errorf("%s: TODO %v %v %v: %v", d.Position, t, elem, elem.Kind(), v))
				}
			default:
				panic(fmt.Errorf("%s: TODO %v %v: %v", d.Position, t, typ.Kind(), v))
			}
		case *ir.Int32Value:
			switch typ := l.tc.MustType(t); typ.Kind() {
			case ir.Int8, ir.Uint8:
				*(*int8)(unsafe.Pointer(&b[0])) = int8(x.Value)
			case ir.Int16, ir.Uint16:
				*(*int16)(unsafe.Pointer(&b[0])) = int16(x.Value)
			case ir.Int32, ir.Uint32:
				*(*int32)(unsafe.Pointer(&b[0])) = x.Value
			case ir.Int64, ir.Uint64:
				*(*int64)(unsafe.Pointer(&b[0])) = int64(x.Value)
			case ir.Float32:
				*(*float32)(unsafe.Pointer(&b[0])) = float32(x.Value)
			case ir.Float64:
				*(*float64)(unsafe.Pointer(&b[0])) = float64(x.Value)
			case ir.Pointer:
				*(*uintptr)(unsafe.Pointer(&b[0])) = uintptr(x.Value)
			case ir.Struct:
				f(off, typ.(*ir.StructOrUnionType).Fields[0].ID(), x)
			default:
				panic(fmt.Errorf("%s: TODO %v: %v", d.Position, t, v))
			}
		case *ir.Int64Value:
			switch typ := l.tc.MustType(t); typ.Kind() {
			case ir.Int32, ir.Uint32:
				*(*int32)(unsafe.Pointer(&b[0])) = int32(x.Value)
			case ir.Int64, ir.Uint64:
				*(*int64)(unsafe.Pointer(&b[0])) = x.Value
			default:
				panic(fmt.Errorf("%s: TODO %v: %v", d.Position, t, v))
			}
		case *ir.Float32Value:
			switch typ := l.tc.MustType(t); typ.Kind() {
			case ir.Float32:
				*(*float32)(unsafe.Pointer(&b[0])) = x.Value
			default:
				panic(fmt.Errorf("%s: TODO %v: %v", d.Position, t, v))
			}
		case *ir.Float64Value:
			switch typ := l.tc.MustType(t); typ.Kind() {
			case ir.Float32:
				*(*float32)(unsafe.Pointer(&b[0])) = float32(x.Value)
			case ir.Float64:
				*(*float64)(unsafe.Pointer(&b[0])) = x.Value
			default:
				panic(fmt.Errorf("%s: TODO %v: %v", d.Position, t, v))
			}
		case *ir.Complex64Value:
			switch typ := l.tc.MustType(t); typ.Kind() {
			case ir.Complex64:
				*(*complex64)(unsafe.Pointer(&b[0])) = x.Value
			case ir.Complex128:
				*(*complex128)(unsafe.Pointer(&b[0])) = complex128(x.Value)
			default:
				panic(fmt.Errorf("%s: TODO %v: %v", d.Position, t, v))
			}
		case *ir.Complex128Value:
			switch typ := l.tc.MustType(t); typ.Kind() {
			case ir.Complex128:
				*(*complex128)(unsafe.Pointer(&b[0])) = x.Value
			default:
				panic(fmt.Errorf("%s: TODO %v: %v", d.Position, t, v))
			}
		case *ir.StringValue:
			delta := int(x.Offset)
			switch typ := l.tc.MustType(t); typ.Kind() {
			case ir.Pointer:
				switch typ := typ.(*ir.PointerType).Element; typ.Kind() {
				case ir.Int8, ir.Uint8, ir.Int32:
					*(*uintptr)(unsafe.Pointer(&b[0])) = uintptr(l.text(x.StringID, true, delta))
					l.out.TSRelative[off>>3] |= 1 << uint(off&7)
				case ir.Pointer:
					switch typ := typ.(*ir.PointerType).Element; typ.Kind() {
					case ir.Function:
						*(*uintptr)(unsafe.Pointer(&b[0])) = uintptr(l.text(x.StringID, true, delta))
						l.out.TSRelative[off>>3] |= 1 << uint(off&7)
					default:
						panic(fmt.Errorf("%s: TODO %v: %q", d.Position, typ, x.StringID))
					}
				default:
					panic(fmt.Errorf("%s: TODO %v: %q", d.Position, typ, x.StringID))
				}
			case ir.Array:
				switch typ := typ.(*ir.ArrayType).Item; typ.Kind() {
				case ir.Int8, ir.Uint8:
					copy(l.out.Data[off:], dict.S(int(x.StringID)))
				default:
					panic(fmt.Errorf("%s: TODO %v: %q", d.Position, typ, x.StringID))
				}
			default:
				panic(fmt.Errorf("%s: TODO %v: %q", d.Position, typ, x.StringID))
			}
		case *ir.WideStringValue:
			switch typ := l.tc.MustType(t); typ.Kind() {
			case ir.Array:
				switch typ := typ.(*ir.ArrayType).Item; typ.Kind() {
				// in use for e.g. windows wide strings
				case ir.Uint16:
					for i, v := range utf16.Encode(x.Value) {
						*(*uint16)(unsafe.Pointer(&l.out.Data[off+2*i])) = v
					}
				case ir.Int32:
					for i, v := range x.Value {
						*(*rune)(unsafe.Pointer(&l.out.Data[off+4*i])) = v
					}
				default:
					panic(fmt.Errorf("%s: TODO %v: %q", d.Position, typ, string(x.Value)))
				}
			default:
				panic(fmt.Errorf("%s: TODO %v: %q", d.Position, typ, string(x.Value)))
			}
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

func (l *loader) text(s ir.StringID, null bool, off int) int {
	if p, ok := l.strings[s]; ok {
		return p + off
	}

	p := len(l.out.Text)
	l.strings[s] = p
	l.out.Text = append(l.out.Text, dict.S(int(s))...)
	more := 0
	if null {
		more++
	}
	sz := roundup(len(l.out.Text)+more, mallocAlign)
	l.out.Text = append(l.out.Text, make([]byte, sz-len(l.out.Text))...)
	return p + off
}

func (l *loader) wtext(s ir.StringID) int {
	if p, ok := l.wstrings[s]; ok {
		return p
	}

	p := len(l.out.Text)
	l.wstrings[s] = p
	ws := []rune(string(dict.S(int(s))))
	sz := roundup(4*(len(ws)+1), mallocAlign)
	l.out.Text = append(l.out.Text, make([]byte, sz)...)
	for i, v := range ws {
		*(*rune)(unsafe.Pointer(&l.out.Text[p+4*i])) = v
	}
	return p
}

func (l *loader) pos(op ir.Operation) PCInfo {
	if op == nil {
		return PCInfo{}
	}

	p := op.Pos()
	if !p.IsValid() {
		return PCInfo{}
	}

	return PCInfo{PC: len(l.out.Code), Line: p.Line, Column: p.Column, Name: ir.NameID(dict.SID(p.Filename))}
}

func (l *loader) ip() int { return len(l.out.Code) }

func (l *loader) int8(x ir.Operation, n int8) {
	switch {
	case n == 0:
		l.emit(l.pos(x), Operation{Opcode: Zero8})
	default:
		l.emit(l.pos(x), Operation{Opcode: Push8, N: int(n)})
	}
}

func (l *loader) int16(x ir.Operation, n int16) {
	switch {
	case n == 0:
		l.emit(l.pos(x), Operation{Opcode: Zero16})
	default:
		l.emit(l.pos(x), Operation{Opcode: Push16, N: int(n)})
	}
}

func (l *loader) int32(x ir.Operation, n int32) {
	switch {
	case n == 0:
		l.emit(l.pos(x), Operation{Opcode: Zero32})
	default:
		l.emit(l.pos(x), Operation{Opcode: Push32, N: int(n)})
	}
}

func (l *loader) int64(x ir.Operation, n int64) {
	if n == 0 {
		l.emit(l.pos(x), Operation{Opcode: Zero64})
		return
	}

	switch intSize {
	case 4:
		l.emit(l.pos(x), Operation{Opcode: Push64, N: int(n)})
		l.emit(l.pos(x), Operation{Opcode: Ext, N: int(n >> 32)})
	case 8:
		l.emit(l.pos(x), Operation{Opcode: Push64, N: int(n)})
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
	l.emit(l.pos(x), Operation{Opcode: Push32, N: int(math.Float32bits(n))})
}

func (l *loader) float64(x ir.Operation, n float64) {
	bits := math.Float64bits(n)
	switch intSize {
	case 4:
		l.emit(l.pos(x), Operation{Opcode: Push64, N: int(bits)})
		l.emit(l.pos(x), Operation{Opcode: Ext, N: int(bits >> 32)})
	case 8:
		l.emit(l.pos(x), Operation{Opcode: Push64, N: int(bits)})
	default:
		panic("internal error")
	}
}

func (l *loader) complex64(x ir.Operation, n complex64) {
	bits := *(*int64)(unsafe.Pointer(&n))
	switch intSize {
	case 4:
		l.emit(l.pos(x), Operation{Opcode: Push64, N: int(bits)})
		l.emit(l.pos(x), Operation{Opcode: Ext, N: int(bits) >> 32})
	case 8:
		l.emit(l.pos(x), Operation{Opcode: Push64, N: int(bits)})
	default:
		panic("internal error")
	}
}

func (l *loader) complex128(x ir.Operation, n complex128) {
	re := real(n)
	im := imag(n)
	switch intSize {
	case 4:
		re := math.Float64bits(re)
		im := math.Float64bits(im)
		l.emit(l.pos(x),
			Operation{Opcode: PushC128, N: int(re)},
			Operation{Opcode: Ext, N: int(re >> 32)},
			Operation{Opcode: Ext, N: int(im)},
			Operation{Opcode: Ext, N: int(im >> 32)},
		)
	case 8:
		l.emit(l.pos(x),
			Operation{Opcode: PushC128, N: int(math.Float64bits(re))},
			Operation{Opcode: Ext, N: int(math.Float64bits(im))},
		)
	default:
		panic(fmt.Errorf("%s: internal error", x.Pos()))
	}
}

func (l *loader) int32Literal(dest []byte, t ir.Type, lit int32) {
	switch t.Kind() {
	case ir.Int8, ir.Uint8:
		*(*int8)(unsafe.Pointer(&dest[0])) = int8(lit)
	case ir.Int16, ir.Uint16:
		*(*int16)(unsafe.Pointer(&dest[0])) = int16(lit)
	case ir.Int32, ir.Uint32:
		*(*int32)(unsafe.Pointer(&dest[0])) = lit
	case ir.Int64, ir.Uint64:
		*(*int64)(unsafe.Pointer(&dest[0])) = int64(lit)
	case ir.Float32:
		*(*float32)(unsafe.Pointer(&dest[0])) = float32(lit)
	case ir.Float64:
		*(*float64)(unsafe.Pointer(&dest[0])) = float64(lit)
	case ir.Pointer:
		*(*uintptr)(unsafe.Pointer(&dest[0])) = uintptr(lit)
	default:
		panic(fmt.Errorf("TODO %s", t.Kind()))
	}
}

func (l *loader) int64Literal(dest []byte, t ir.Type, lit int64) {
	switch t.Kind() {
	case ir.Int8, ir.Uint8:
		*(*int8)(unsafe.Pointer(&dest[0])) = int8(lit)
	case ir.Int16, ir.Uint16:
		*(*int16)(unsafe.Pointer(&dest[0])) = int16(lit)
	case ir.Int32, ir.Uint32:
		*(*int32)(unsafe.Pointer(&dest[0])) = int32(lit)
	case ir.Int64, ir.Uint64:
		*(*int64)(unsafe.Pointer(&dest[0])) = lit
	case ir.Float32:
		*(*float32)(unsafe.Pointer(&dest[0])) = float32(lit)
	case ir.Pointer:
		*(*uintptr)(unsafe.Pointer(&dest[0])) = uintptr(lit)
	default:
		panic(fmt.Errorf("TODO %s", t.Kind()))
	}
}

func (l *loader) float32Literal(dest []byte, t ir.Type, lit float32) {
	switch t.Kind() {
	case ir.Float32:
		*(*float32)(unsafe.Pointer(&dest[0])) = lit
	default:
		panic(fmt.Errorf("TODO %s", t.Kind()))
	}
}

func (l *loader) float64Literal(dest []byte, t ir.Type, lit float64) {
	switch t.Kind() {
	case ir.Float64:
		*(*float64)(unsafe.Pointer(&dest[0])) = lit
	default:
		panic(fmt.Errorf("TODO %s", t.Kind()))
	}
}

func (l *loader) arrayLiteral(t ir.Type, v ir.Value, o int) *[]byte {
	p := buffer.CGet(l.sizeof(t.ID()))
	b := *p
	item := t.(*ir.ArrayType).Item
	itemSz := l.sizeof(item.ID())
	switch x := v.(type) {
	case *ir.CompositeValue:
		i := 0 // Item index
		for _, v := range x.Values {
			off := i * itemSz
			switch y := v.(type) {
			case *ir.Int32Value:
				l.int32Literal(b[off:], item, y.Value)
				i++
			case *ir.Int64Value:
				l.int64Literal(b[off:], item, y.Value)
				i++
			case *ir.Float32Value:
				l.float32Literal(b[off:], item, y.Value)
				i++
			case *ir.CompositeValue:
				l.compositeValue(b[off:], item, y, o)
				i++
			case *ir.AddressValue:
				if y.Label != 0 {
					l.tsLabels[o+off] = y
					i++
					break
				}

				panic(fmt.Errorf("TODO %T", y))
			default:
				panic(fmt.Errorf("TODO %T", y))
			}
		}
	default:
		panic(fmt.Errorf("TODO %T", x))
	}
	return p
}

func (l *loader) compositeValue(dest []byte, t ir.Type, lit ir.Value, o int) {
	var p *[]byte
	switch t.Kind() {
	case ir.Array:
		p = l.arrayLiteral(t, lit, o)
	case ir.Struct, ir.Union:
		p = l.structLiteral(t, lit, o)
	default:
		panic(fmt.Errorf("TODO %s", t.Kind()))
	}
	copy(dest, *p)
	buffer.Put(p)
}

func (l *loader) structLiteral(t ir.Type, v ir.Value, o int) *[]byte {
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
			case *ir.Int64Value:
				l.int64Literal(b[layout[i].Offset:], fields[i], y.Value)
				i++
			case *ir.Float64Value:
				l.float64Literal(b[layout[i].Offset:], fields[i], y.Value)
				i++
			case *ir.CompositeValue:
				l.compositeValue(b[layout[i].Offset:], fields[i], y, o)
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
		p = l.arrayLiteral(t, v, len(l.out.Text))
	case ir.Struct, ir.Union:
		p = l.structLiteral(t, v, len(l.out.Text))
	default:
		panic(fmt.Errorf("TODO %s", t.Kind()))
	}

	r := l.text(ir.StringID(dict.ID(*p)), false, 0)
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
		sz := l.sizeof(v.ID())
		if v.Kind() == ir.Array {
			sz = l.ptrSize
		}
		arguments = append(arguments, nfo{sz: sz})
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
		off += roundup(results[i].sz, l.stackAlign)
	}

	for _, v := range f.Body {
		switch x := v.(type) {
		case *ir.VariableDeclaration:
			t := l.tc.MustType(x.TypeID)
			variables = append(variables, nfo{align: l.model.Alignof(t), sz: l.sizeof(x.TypeID)})
		}
	}
	off = 0
	for i := range variables {
		n := roundup(variables[i].sz, variables[i].align)
		off -= roundup(n, l.stackAlign)
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
			case ir.Complex64:
				l.emit(l.pos(x), Operation{Opcode: AddC64})
			case ir.Complex128:
				l.emit(l.pos(x), Operation{Opcode: AddC128})
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
			case 2:
				l.emit(l.pos(x), Operation{Opcode: And16})
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
			case ir.Int8, ir.Uint8:
				l.emit(l.pos(x), Operation{Opcode: BoolI8})
			case ir.Int16, ir.Uint16:
				l.emit(l.pos(x), Operation{Opcode: BoolI16})
			case ir.Int32, ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: BoolI32})
			case ir.Int64, ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: BoolI64})
			case ir.Float32:
				l.emit(l.pos(x), Operation{Opcode: BoolF32})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: BoolF64})
			case ir.Complex128:
				l.emit(l.pos(x), Operation{Opcode: BoolC128})
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
				body := l.objects[x.Index].(*ir.FunctionDefinition).Body
				if len(body) == 1 {
					if _, ok := body[0].(*ir.Panic); ok {
						l.emit(l.pos(x), Operation{Opcode: opcode})
						break
					}
				}
			}

			l.emit(l.pos(x), Operation{Opcode: Call, N: x.Index})
		case *ir.CallFP:
			l.emit(l.pos(x), Operation{Opcode: CallFP})
		case *ir.Convert:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int8:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int8, ir.Uint8:
					// ok
				case ir.Int16, ir.Uint16:
					l.emit(l.pos(x), Operation{Opcode: ConvI8I16})
				case ir.Int32, ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: ConvI8I32})
				case ir.Int64, ir.Uint64:
					l.emit(l.pos(x), Operation{Opcode: ConvI8I64})
				case ir.Float64:
					l.emit(l.pos(x), Operation{Opcode: ConvI8F64})
				default:
					panic(fmt.Errorf("TODO %v", u.Kind()))
				}
			case ir.Uint8:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int8, ir.Uint8:
					// ok
				case ir.Int16, ir.Uint16:
					l.emit(l.pos(x), Operation{Opcode: ConvU8I16})
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: ConvU8I32})
				case ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: ConvU8U32})
				case ir.Int64, ir.Uint64:
					l.emit(l.pos(x), Operation{Opcode: ConvU8U64})
				case ir.Pointer:
					switch l.ptrSize {
					case 4:
						l.emit(l.pos(x), Operation{Opcode: ConvU8U32})
					case 8:
						l.emit(l.pos(x), Operation{Opcode: ConvU8U64})
					default:
						panic(fmt.Errorf("%s: TODO %v", x.Position, l.ptrSize))
					}
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, u.Kind()))
				}
			case ir.Int16:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int8, ir.Uint8, ir.Int16, ir.Uint16:
					// ok
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: ConvI16I32})
				case ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: ConvI16U32})
				case ir.Int64, ir.Uint64:
					l.emit(l.pos(x), Operation{Opcode: ConvI16I64})
				case ir.Pointer:
					switch l.ptrSize {
					case 4:
						l.emit(l.pos(x), Operation{Opcode: ConvU16U32})
					case 8:
						l.emit(l.pos(x), Operation{Opcode: ConvU16U64})
					default:
						panic(fmt.Errorf("%s: TODO %v", x.Position, l.ptrSize))
					}
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, u.Kind()))
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
				case ir.Complex64:
					l.emit(l.pos(x), Operation{Opcode: ConvI32C64})
				case ir.Complex128:
					l.emit(l.pos(x), Operation{Opcode: ConvI32C128})
				case ir.Pointer, ir.Function:
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
				case ir.Int8, ir.Uint8:
					// ok
				case ir.Int16, ir.Uint16:
					// ok
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: ConvU16I32})
				case ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: ConvU16U32})
				case ir.Int64, ir.Uint64:
					l.emit(l.pos(x), Operation{Opcode: ConvU16I64})
				case ir.Pointer:
					switch l.ptrSize {
					case 4:
						l.emit(l.pos(x), Operation{Opcode: ConvU16U32})
					case 8:
						l.emit(l.pos(x), Operation{Opcode: ConvU16U64})
					default:
						panic(fmt.Errorf("%s: TODO %v", x.Position, l.ptrSize))
					}
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, u.Kind()))
				}
			case ir.Uint32:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int8, ir.Uint8:
					l.emit(l.pos(x), Operation{Opcode: ConvU32U8})
				case ir.Int16, ir.Uint16:
					l.emit(l.pos(x), Operation{Opcode: ConvU32I16})
				case ir.Int32, ir.Uint32:
					// ok
				case ir.Int64, ir.Uint64:
					l.emit(l.pos(x), Operation{Opcode: ConvU32I64})
				case ir.Float32:
					l.emit(l.pos(x), Operation{Opcode: ConvU32F32})
				case ir.Float64:
					l.emit(l.pos(x), Operation{Opcode: ConvU32F64})
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
				case ir.Int8, ir.Uint8:
					l.emit(l.pos(x), Operation{Opcode: ConvI64I8})
				case ir.Int16, ir.Uint16:
					l.emit(l.pos(x), Operation{Opcode: ConvI64I16})
				case ir.Int32, ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: ConvI64I32})
				case ir.Int64, ir.Uint64:
					// ok
				case ir.Float64:
					l.emit(l.pos(x), Operation{Opcode: ConvI64F64})
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
				case ir.Int8, ir.Uint8:
					l.emit(l.pos(x), Operation{Opcode: ConvI64I8})
				case ir.Int16, ir.Uint16:
					l.emit(l.pos(x), Operation{Opcode: ConvI64I16})
				case ir.Int32, ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: ConvI64I32})
				case ir.Int64, ir.Uint64:
					// ok
				case ir.Float64:
					l.emit(l.pos(x), Operation{Opcode: ConvI64F64})
				case ir.Pointer:
					switch l.ptrSize {
					case 4:
						l.emit(l.pos(x), Operation{Opcode: ConvI64I32})
					case 8:
						// ok
					}
				case ir.Union:
					l.emit(l.pos(x), Operation{Opcode: ConvI64, N: l.sizeof(x.Result)})
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, u.Kind()))
				}
			case ir.Float32:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: ConvF32I32})
				case ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: ConvF32U32})
				case ir.Int64:
					l.emit(l.pos(x), Operation{Opcode: ConvF32I64})
				case ir.Float64:
					l.emit(l.pos(x), Operation{Opcode: ConvF32F64})
				case ir.Complex64:
					l.emit(l.pos(x), Operation{Opcode: ConvF32C64})
				case ir.Complex128:
					l.emit(l.pos(x), Operation{Opcode: ConvF32C128})
				default:
					panic(fmt.Errorf("TODO %v", u.Kind()))
				}
			case ir.Float64:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int8:
					l.emit(l.pos(x), Operation{Opcode: ConvF64I8})
				case ir.Uint16:
					l.emit(l.pos(x), Operation{Opcode: ConvF64U16})
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: ConvF64I32})
				case ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: ConvF64U32})
				case ir.Int64:
					l.emit(l.pos(x), Operation{Opcode: ConvF64I64})
				case ir.Uint64:
					l.emit(l.pos(x), Operation{Opcode: ConvF64U64})
				case ir.Float32:
					l.emit(l.pos(x), Operation{Opcode: ConvF64F32})
				case ir.Float64:
					// ok
				case ir.Complex128:
					l.emit(l.pos(x), Operation{Opcode: ConvF64C128})
				default:
					panic(fmt.Errorf("TODO %v", u.Kind()))
				}
			case ir.Complex64:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Complex64:
					// ok
				case ir.Complex128:
					l.emit(l.pos(x), Operation{Opcode: ConvC64C128})
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, u.Kind()))
				}
			case ir.Pointer:
				switch u := l.tc.MustType(x.Result); u.Kind() {
				case ir.Int32, ir.Uint32:
					switch l.ptrSize {
					case 4:
						// ok
					case 8:
						l.emit(l.pos(x), Operation{Opcode: ConvI64I32})
					}
				case ir.Int64, ir.Uint64:
					switch l.ptrSize {
					case 4:
						l.emit(l.pos(x), Operation{Opcode: ConvU32I64})
					case 8:
						// ok
					}
				case ir.Pointer, ir.Array:
					// ok
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, u.Kind()))
				}
			default:
				panic(fmt.Errorf("%s: TODO %v -> %v", x.Position, x.TypeID, x.Result))
			}
		case *ir.Copy:
			l.emit(l.pos(x), Operation{Opcode: Copy, N: l.sizeof(x.TypeID)})
		case *ir.Cpl:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int8, ir.Uint8:
				l.emit(l.pos(x), Operation{Opcode: Cpl8})
			case ir.Int32, ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: Cpl32})
			case ir.Int64, ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: Cpl64})
			default:
				panic(fmt.Errorf("%s: TODO %v", x.Position, t.Kind()))
			}
		case *ir.Div:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32:
				l.emit(l.pos(x), Operation{Opcode: DivI32})
			case ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: DivU32})
			case ir.Int64:
				l.emit(l.pos(x), Operation{Opcode: DivI64})
			case ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: DivU64})
			case ir.Float32:
				l.emit(l.pos(x), Operation{Opcode: DivF32})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: DivF64})
			case ir.Complex64:
				l.emit(l.pos(x), Operation{Opcode: DivC64})
			case ir.Complex128:
				l.emit(l.pos(x), Operation{Opcode: DivC128})
			default:
				panic(fmt.Errorf("internal error %s", x.TypeID))
			}
		case *ir.Drop:
			l.emit(l.pos(x), Operation{Opcode: AddSP, N: l.stackSize(x.TypeID)})
		case *ir.Dup:
			switch l.sizeof(x.TypeID) {
			case 1:
				l.emit(l.pos(x), Operation{Opcode: Dup8})
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
				case ir.Uint16:
					l.emit(l.pos(x), Operation{Opcode: NegIndexU16, N: sz})
				case ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: NegIndexU32, N: sz})
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: NegIndexI32, N: sz})
				case ir.Int64:
					l.emit(l.pos(x), Operation{Opcode: NegIndexI64, N: sz})
				case ir.Uint64:
					l.emit(l.pos(x), Operation{Opcode: NegIndexU64, N: sz})
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, xt.Kind()))
				}
				if !x.Address {
					panic(fmt.Errorf("TODO %v", xt.Kind()))
				}
			default:
				switch xt.Kind() {
				case ir.Int8:
					l.emit(l.pos(x), Operation{Opcode: IndexI8, N: sz})
				case ir.Uint8:
					l.emit(l.pos(x), Operation{Opcode: IndexU8, N: sz})
				case ir.Int16:
					l.emit(l.pos(x), Operation{Opcode: IndexI16, N: sz})
				case ir.Uint16:
					l.emit(l.pos(x), Operation{Opcode: IndexU16, N: sz})
				case ir.Int32:
					l.emit(l.pos(x), Operation{Opcode: IndexI32, N: sz})
				case ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: IndexU32, N: sz})
				case ir.Int64:
					l.emit(l.pos(x), Operation{Opcode: IndexI64, N: sz})
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
						l.emit(l.pos(x),
							Operation{Opcode: Load},
							Operation{Opcode: Ext, N: sz},
						)
					}
				}
			}
		case *ir.EndScope:
			// nop
		case *ir.Eq:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int8, ir.Uint8:
				l.emit(l.pos(x), Operation{Opcode: EqI8})
			case ir.Int32, ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: EqI32})
			case ir.Int64, ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: EqI64})
			case ir.Float32:
				l.emit(l.pos(x), Operation{Opcode: EqF32})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: EqF64})
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
					case ir.Int8, ir.Uint8:
						l.emit(l.pos(x), Operation{Opcode: DSI8, N: l.m[x.Index]})
					case ir.Int16, ir.Uint16:
						l.emit(l.pos(x), Operation{Opcode: DSI16, N: l.m[x.Index]})
					case ir.Int32, ir.Uint32, ir.Float32:
						l.emit(l.pos(x), Operation{Opcode: DSI32, N: l.m[x.Index]})
					case ir.Int64, ir.Uint64, ir.Float64, ir.Complex64:
						l.emit(l.pos(x), Operation{Opcode: DSI64, N: l.m[x.Index]})
					case ir.Complex128:
						l.emit(l.pos(x), Operation{Opcode: DSC128, N: l.m[x.Index]})
					case ir.Pointer:
						switch l.ptrSize {
						case 4:
							l.emit(l.pos(x), Operation{Opcode: DSI32, N: l.m[x.Index]})
						case 8:
							l.emit(l.pos(x), Operation{Opcode: DSI64, N: l.m[x.Index]})
						default:
							panic(fmt.Errorf("internal error %s, %v", x.TypeID, l.ptrSize))
						}
					case ir.Struct, ir.Union:
						l.emit(l.pos(x),
							Operation{Opcode: DSN, N: l.m[x.Index]},
							Operation{Opcode: Ext, N: l.sizeof(x.TypeID)},
						)
					default:
						panic(fmt.Errorf("%s: TODO %v", x.Position, t.Kind()))
					}
				}
			case *ir.FunctionDefinition:
				switch {
				case x.Address:
					l.emit(l.pos(x), Operation{Opcode: FP, N: x.Index})
				default:
					panic(fmt.Errorf("%s: TODO %T(%v)", x.Position, ex, ex))
				}
			default:
				panic(fmt.Errorf("%s: TODO %T(%v)", x.Position, ex, ex))
			}
		case *ir.Field:
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
		case *ir.FieldValue:
			t := l.tc.MustType(x.TypeID).(*ir.StructOrUnionType)
			tsz := l.model.Sizeof(t)
			fields := l.model.Layout(t)
			switch sz := fields[x.Index].Size; sz {
			case 1:
				l.emit(l.pos(x),
					Operation{Opcode: Field8, N: int(fields[x.Index].Offset)},
					Operation{Opcode: Ext, N: int(tsz)},
				)
			case 2:
				l.emit(l.pos(x),
					Operation{Opcode: Field16, N: int(fields[x.Index].Offset)},
					Operation{Opcode: Ext, N: int(tsz)},
				)
			case 8:
				l.emit(l.pos(x),
					Operation{Opcode: Field64, N: int(fields[x.Index].Offset)},
					Operation{Opcode: Ext, N: int(tsz)},
				)
			default:
				panic(fmt.Errorf("%s: TODO %v", x.Position, sz))
			}
		case *ir.Geq:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int8:
				l.emit(l.pos(x), Operation{Opcode: GeqI8})
			case ir.Int32:
				l.emit(l.pos(x), Operation{Opcode: GeqI32})
			case ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: GeqU32})
			case ir.Int64:
				l.emit(l.pos(x), Operation{Opcode: GeqI64})
			case ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: GeqU64})
			case ir.Float32:
				l.emit(l.pos(x), Operation{Opcode: GeqF32})
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
			case ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: GtU64})
			case ir.Float32:
				l.emit(l.pos(x), Operation{Opcode: GtF32})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: GtF64})
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
		case *ir.Const:
			switch v := x.Value.(type) {
			case *ir.AddressValue:
				if v.Label != 0 {
					l.csLabels[len(l.out.Code)] = v
					switch l.ptrSize {
					case 4:
						l.emit(l.pos(x), Operation{Opcode: Push32})
					case 8:
						l.emit(l.pos(x), Operation{Opcode: Push64})
					default:
						panic("internal error")
					}
					break
				}

				panic(fmt.Errorf("%s: TODO %T", x.Position, v))
			default:
				panic(fmt.Errorf("%s: TODO %T", x.Position, v))
			}
		case *ir.Const32:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int8, ir.Uint8, ir.Int16, ir.Uint16, ir.Int32, ir.Uint32:
				l.int32(x, x.Value)
			case ir.Int64, ir.Uint64:
				l.int64(x, int64(x.Value))
			case ir.Float32:
				l.float32(x, math.Float32frombits(uint32(x.Value)))
			case ir.Pointer:
				l.uintptr32(x, x.Value)
			default:
				panic(fmt.Errorf("TODO %v", t.Kind()))
			}
		case *ir.Const64:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int8, ir.Uint8:
				l.int8(x, int8(x.Value))
			case ir.Int16, ir.Uint16:
				l.int16(x, int16(x.Value))
			case ir.Int32, ir.Uint32:
				l.int32(x, int32(x.Value))
			case ir.Int64, ir.Uint64:
				l.int64(x, x.Value)
			case ir.Float64:
				l.float64(x, math.Float64frombits(uint64(x.Value)))
			case ir.Complex64:
				real := math.Float32frombits(uint32(x.Value >> 32))
				imag := math.Float32frombits(uint32(x.Value))
				l.complex64(x, complex(real, imag))
			default:
				panic(fmt.Errorf("TODO %v", t.Kind()))
			}
		case *ir.ConstC128:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Complex128:
				l.complex128(x, x.Value)
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
			var nfo labelNfo
			switch {
			case n != 0:
				nfo = labelNfo{index: index, nm: x.NameID}
			default:
				n = x.Number
				nfo = labelNfo{index: index, nm: ir.NameID(-n)}
			}
			l.namedLabels[nfo] = len(l.out.Code)
			labels[n] = len(l.out.Code)
			l.emit(l.pos(x), Operation{Opcode: Label, N: n})
		case *ir.Leq:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int8:
				l.emit(l.pos(x), Operation{Opcode: LeqI8})
			case ir.Int32:
				l.emit(l.pos(x), Operation{Opcode: LeqI32})
			case ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: LeqU32})
			case ir.Int64:
				l.emit(l.pos(x), Operation{Opcode: LeqI64})
			case ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: LeqU64})
			case ir.Float32:
				l.emit(l.pos(x), Operation{Opcode: LeqF32})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: LeqF64})
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
			switch sz := l.sizeof(l.tc.MustType(x.TypeID).(*ir.PointerType).Element.ID()); sz {
			case 1:
				l.emit(l.pos(x), Operation{Opcode: Load8})
			case 2:
				l.emit(l.pos(x), Operation{Opcode: Load16})
			case 4:
				l.emit(l.pos(x), Operation{Opcode: Load32})
			case 8:
				l.emit(l.pos(x), Operation{Opcode: Load64})
			default:
				l.emit(l.pos(x),
					Operation{Opcode: Load},
					Operation{Opcode: Ext, N: sz},
				)
			}
		case *ir.Lsh:
			switch l.sizeof(x.TypeID) {
			case 1:
				l.emit(l.pos(x), Operation{Opcode: LshI8})
			case 2:
				l.emit(l.pos(x), Operation{Opcode: LshI16})
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
			case ir.Float32:
				l.emit(l.pos(x), Operation{Opcode: LtF32})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: LtF64})
			case ir.Pointer:
				switch l.ptrSize {
				case 4:
					l.emit(l.pos(x), Operation{Opcode: LtU32})
				case 8:
					l.emit(l.pos(x), Operation{Opcode: LtU64})
				default:
					panic(fmt.Errorf("internal error %s", x.TypeID))
				}
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
			case ir.Complex64:
				l.emit(l.pos(x), Operation{Opcode: MulC64})
			case ir.Complex128:
				l.emit(l.pos(x), Operation{Opcode: MulC128})
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
			case ir.Int8, ir.Uint8:
				l.emit(l.pos(x), Operation{Opcode: NegI8})
			case ir.Int16, ir.Uint16:
				l.emit(l.pos(x), Operation{Opcode: NegI16})
			case ir.Int32, ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: NegI32})
			case ir.Int64, ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: NegI64})
			case ir.Float32:
				l.emit(l.pos(x), Operation{Opcode: NegF32})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: NegF64})
			default:
				panic(fmt.Errorf("%s: TODO %v", x.Position, t.Kind()))
			}
		case *ir.Neq:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int8, ir.Uint8:
				l.emit(l.pos(x), Operation{Opcode: NeqI8})
			case ir.Int32, ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: NeqI32})
			case ir.Int64, ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: NeqI64})
			case ir.Float32:
				l.emit(l.pos(x), Operation{Opcode: NeqF32})
			case ir.Float64:
				l.emit(l.pos(x), Operation{Opcode: NeqF64})
			case ir.Complex64:
				l.emit(l.pos(x), Operation{Opcode: NeqC64})
			case ir.Complex128:
				l.emit(l.pos(x), Operation{Opcode: NeqC128})
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
			case 8:
				l.emit(l.pos(x), Operation{Opcode: Or64})
			default:
				panic(fmt.Errorf("internal error %s", x.TypeID))
			}
		case *ir.Panic:
			l.emit(l.pos(x), Operation{Opcode: Panic})
		case *ir.PostIncrement:
			switch {
			case x.Bits != 0:
				switch t := l.tc.MustType(x.BitFieldType); t.Kind() {
				case ir.Int32, ir.Uint32:
					l.emit(l.pos(x),
						Operation{Opcode: PostIncU32Bits, N: x.Delta},
						Operation{Opcode: Ext, N: x.Bits<<16 | x.BitOffset<<8 | l.sizeof(x.BitFieldType)},
					)
				case ir.Int64, ir.Uint64:
					l.emit(l.pos(x),
						Operation{Opcode: PostIncU64Bits, N: x.Delta},
						Operation{Opcode: Ext, N: x.Bits<<16 | x.BitOffset<<8 | l.sizeof(x.BitFieldType)},
					)
				default:
					panic(fmt.Errorf("TODO %v", t.Kind()))
				}
			default:
				switch t := l.tc.MustType(x.TypeID); t.Kind() {
				case ir.Int8, ir.Uint8:
					l.emit(l.pos(x), Operation{Opcode: PostIncI8, N: x.Delta})
				case ir.Int16, ir.Uint16:
					l.emit(l.pos(x), Operation{Opcode: PostIncI16, N: x.Delta})
				case ir.Int32, ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: PostIncI32, N: x.Delta})
				case ir.Int64, ir.Uint64:
					l.emit(l.pos(x), Operation{Opcode: PostIncI64, N: x.Delta})
				case ir.Float64:
					l.emit(l.pos(x), Operation{Opcode: PostIncF64, N: x.Delta})
				case ir.Pointer:
					l.emit(l.pos(x), Operation{Opcode: PostIncPtr, N: x.Delta})
				default:
					panic(fmt.Errorf("TODO %v", t.Kind()))
				}
			}
		case *ir.PreIncrement:
			switch {
			case x.Bits != 0:
				switch t := l.tc.MustType(x.BitFieldType); t.Kind() {
				case ir.Int32, ir.Uint32:
					l.emit(l.pos(x),
						Operation{Opcode: PreIncU32Bits, N: x.Delta},
						Operation{Opcode: Ext, N: x.Bits<<16 | x.BitOffset<<8 | l.sizeof(x.BitFieldType)},
					)
				case ir.Int64, ir.Uint64:
					l.emit(l.pos(x),
						Operation{Opcode: PreIncU64Bits, N: x.Delta},
						Operation{Opcode: Ext, N: x.Bits<<16 | x.BitOffset<<8 | l.sizeof(x.BitFieldType)},
					)
				default:
					panic(fmt.Errorf("TODO %v", t.Kind()))
				}
			default:
				switch t := l.tc.MustType(x.TypeID); t.Kind() {
				case ir.Int8, ir.Uint8:
					l.emit(l.pos(x), Operation{Opcode: PreIncI8, N: x.Delta})
				case ir.Int16, ir.Uint16:
					l.emit(l.pos(x), Operation{Opcode: PreIncI16, N: x.Delta})
				case ir.Int32, ir.Uint32:
					l.emit(l.pos(x), Operation{Opcode: PreIncI32, N: x.Delta})
				case ir.Int64, ir.Uint64:
					l.emit(l.pos(x), Operation{Opcode: PreIncI64, N: x.Delta})
				case ir.Pointer:
					l.emit(l.pos(x), Operation{Opcode: PreIncPtr, N: x.Delta})
				default:
					panic(fmt.Errorf("%s: TODO %v", x.Position, t.Kind()))
				}
			}
		case *ir.PtrDiff:
			sz := 1
			if x.PtrType != idVoidP {
				sz = l.sizeof(l.tc.MustType(x.PtrType).(*ir.PointerType).Element.ID())
			}
			l.emit(l.pos(x), Operation{Opcode: PtrDiff, N: sz})
		case *ir.Rem:
			switch t := l.tc.MustType(x.TypeID); t.Kind() {
			case ir.Int32:
				l.emit(l.pos(x), Operation{Opcode: RemI32})
			case ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: RemU32})
			case ir.Int64:
				l.emit(l.pos(x), Operation{Opcode: RemI64})
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
			case ir.Uint8:
				l.emit(l.pos(x), Operation{Opcode: RshU8})
			case ir.Int16:
				l.emit(l.pos(x), Operation{Opcode: RshI16})
			case ir.Uint16:
				l.emit(l.pos(x), Operation{Opcode: RshU16})
			case ir.Int32:
				l.emit(l.pos(x), Operation{Opcode: RshI32})
			case ir.Uint32:
				l.emit(l.pos(x), Operation{Opcode: RshU32})
			case ir.Int64:
				l.emit(l.pos(x), Operation{Opcode: RshI64})
			case ir.Uint64:
				l.emit(l.pos(x), Operation{Opcode: RshU64})
			default:
				panic(fmt.Errorf("%s: internal error %s", x.Position, x.TypeID))
			}
		case *ir.Store:
			if x.Bits != 0 {
				if x.BitOffset != 0 {
					l.emit(l.pos(x), Operation{Opcode: Push32, N: x.BitOffset})
					switch t := l.tc.MustType(x.TypeID); t.Kind() {
					case ir.Int8, ir.Uint8:
						l.emit(l.pos(x), Operation{Opcode: LshI8})
					case ir.Int16, ir.Uint16:
						l.emit(l.pos(x), Operation{Opcode: LshI16})
					case ir.Int32, ir.Uint32:
						l.emit(l.pos(x), Operation{Opcode: LshI32})
					case ir.Int64, ir.Uint64:
						l.emit(l.pos(x), Operation{Opcode: LshI64})
					default:
						panic(fmt.Errorf("%s: internal error %s", x.Position, x.TypeID))
					}
				}
				mask := (uint64(1)<<uint(x.Bits) - 1) << uint(x.BitOffset)
				switch l.sizeof(x.TypeID) {
				case 1:
					l.emit(l.pos(x), Operation{Opcode: StoreBits8, N: int(mask)})
				case 2:
					l.emit(l.pos(x), Operation{Opcode: StoreBits16, N: int(mask)})
				case 4:
					l.emit(l.pos(x), Operation{Opcode: StoreBits32, N: int(mask)})
				case 8:
					l.emit(l.pos(x), Operation{Opcode: StoreBits64, N: int(mask)})
				default:
					panic(fmt.Errorf("%s: internal error %s", x.Position, x.TypeID))
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
			switch x.TypeID {
			case idInt8P:
				l.emit(l.pos(x), Operation{Opcode: Text, N: l.text(x.Value, true, 0)})
			case idInt32P:
				l.emit(l.pos(x), Operation{Opcode: Text, N: l.wtext(x.Value)})
			default:
				panic(fmt.Errorf("%s: TODO %v", x.Position, x.TypeID))
			}
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
			case ir.Pointer:
				l.emit(l.pos(x), Operation{Opcode: SubPtrs})
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
				if v.Label != 0 {
					l.csLabels[len(l.out.Code)] = v
					switch l.ptrSize {
					case 4:
						l.emit(l.pos(x), Operation{Opcode: Push32})
					case 8:
						l.emit(l.pos(x), Operation{Opcode: Push64})
					default:
						panic("internal error")
					}
					break
				}

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
						panic(fmt.Errorf("%s: %s.%05x: TODO %T(%v)", x.Position, f.NameID, ip, ex, ex))
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
					l.int64(x, int64(v.Value))
					l.emit(l.pos(x), Operation{Opcode: Store64})
				case ir.Float32:
					l.float32(x, float32(v.Value))
					l.emit(l.pos(x), Operation{Opcode: Store32})
				case ir.Float64:
					l.float64(x, float64(v.Value))
					l.emit(l.pos(x), Operation{Opcode: Store64})
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

					switch l.ptrSize {
					case 4:
						l.int32(x, v.Value)
						l.emit(l.pos(x), Operation{Opcode: Store32})
					case 8:
						l.int64(x, int64(v.Value))
						l.emit(l.pos(x), Operation{Opcode: Store64})
					default:
						panic(fmt.Errorf("internal error %s", x.TypeID))
					}
				default:
					panic(fmt.Errorf("%s: %v", x.Position, x.TypeID))
				}
				l.emit(l.pos(x), Operation{Opcode: AddSP, N: l.stackSize(x.TypeID)})
			case *ir.Int64Value:
				l.emit(l.pos(x), Operation{Opcode: BP, N: variables[x.Index].off})
				switch t := l.tc.MustType(x.TypeID); t.Kind() {
				case ir.Int32, ir.Uint32:
					l.int32(x, int32(v.Value))
					l.emit(l.pos(x), Operation{Opcode: Store32})
				case ir.Int64, ir.Uint64:
					l.int64(x, v.Value)
					l.emit(l.pos(x), Operation{Opcode: Store64})
				case ir.Pointer:
					if v.Value == 0 {
						panic(fmt.Errorf("%s: %v", x.Position, x.TypeID))
					}

					switch l.ptrSize {
					case 4:
						l.int32(x, int32(v.Value))
						l.emit(l.pos(x), Operation{Opcode: Store32})
					case 8:
						l.int64(x, v.Value)
						l.emit(l.pos(x), Operation{Opcode: Store64})
					default:
						panic(fmt.Errorf("internal error %s", x.TypeID))
					}
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
				case ir.Float64:
					l.float64(x, v.Value)
					l.emit(l.pos(x), Operation{Opcode: Store64})
				default:
					panic(fmt.Errorf("%s: %v", x.Position, x.TypeID))
				}
				l.emit(l.pos(x), Operation{Opcode: AddSP, N: l.stackSize(x.TypeID)})
			case *ir.Complex128Value:
				l.emit(l.pos(x), Operation{Opcode: BP, N: variables[x.Index].off})
				switch t := l.tc.MustType(x.TypeID); t.Kind() {
				case ir.Complex128:
					l.complex128(x, v.Value)
					l.emit(l.pos(x), Operation{Opcode: StoreC128})
				default:
					panic(fmt.Errorf("%s: %v", x.Position, x.TypeID))
				}
				l.emit(l.pos(x), Operation{Opcode: AddSP, N: l.stackSize(x.TypeID)})
			case *ir.StringValue:
				switch vt := l.tc.MustType(x.TypeID); {
				case vt.Kind() == ir.Array:
					l.emit(l.pos(x), Operation{Opcode: BP, N: variables[x.Index].off})
					l.emit(l.pos(x), Operation{Opcode: Text, N: l.text(v.StringID, true, 0)})
					l.emit(l.pos(x), Operation{Opcode: StrNCopy, N: l.sizeof(x.TypeID)})
				default:
					l.emit(l.pos(x), Operation{Opcode: BP, N: variables[x.Index].off})
					l.emit(l.pos(x), Operation{Opcode: Text, N: l.text(v.StringID, true, 0)})
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
			case 8:
				l.emit(l.pos(x), Operation{Opcode: Xor64})
			default:
				panic(fmt.Errorf("internal error %s", x.TypeID))
			}
		case *ir.JmpP:
			l.emit(l.pos(x), Operation{Opcode: JmpP})
		case *ir.Switch:
			switch x.TypeID {
			case idInt32, idUint32:
				l.emit(l.pos(x), Operation{Opcode: SwitchI32, N: l.m[l.switches[x]]})
			case idInt64, idUint64:
				l.emit(l.pos(x), Operation{Opcode: SwitchI64, N: l.m[l.switches[x]]})
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

func (l *loader) loadBuiltin(op Opcode, f *ir.FunctionDefinition) {
	l.prev = Operation{}
	fp := f.Position
	fi := PCInfo{PC: len(l.out.Code), Line: fp.Line, Name: f.NameID}
	switch op {
	case exit:
		l.emit(fi,
			Operation{Opcode: AddSP, N: ptrStackSz},
			Operation{Opcode: op},
		)
	default:
		l.emit(fi,
			Operation{Opcode: builtin},
			Operation{Opcode: op},
			Operation{Opcode: FFIReturn},
		)
	}
}

func (l *loader) vsize(v ir.Value, t ir.Type) (r int) {
	switch t.Kind() {
	case ir.Array:
		u := t.(*ir.ArrayType).Item
		switch x := v.(type) {
		case *ir.CompositeValue:
			for _, v := range x.Values {
				r += l.vsize(v, u)
			}
		case
			nil,
			*ir.StringValue:
			// nop
		default:
			panic(fmt.Errorf("%v:%v, %T", t.ID(), t.Kind(), x))
		}
	case
		ir.Int8, ir.Int16, ir.Int32, ir.Int64,
		ir.Uint8, ir.Uint16, ir.Uint32, ir.Uint64,
		ir.Float32, ir.Float64,
		ir.Complex64, ir.Complex128:
		// nop
	case ir.Pointer:
		u := t.(*ir.PointerType).Element
		switch x := v.(type) {
		case
			*ir.AddressValue,
			*ir.Int32Value,
			*ir.StringValue:
			// nop
		case *ir.CompositeValue:
			r = len(x.Values) * l.sizeof(u.ID())
			for _, v := range x.Values {
				r += l.vsize(v, u)
			}
		default:
			panic(fmt.Errorf("%v:%v, %T", t.ID(), t.Kind(), x))
		}
	case ir.Struct, ir.Union:
		u := t.(*ir.StructOrUnionType)
		switch x := v.(type) {
		case nil:
			// nop
		case *ir.CompositeValue:
			if t.Kind() == ir.Union && len(x.Values) > 1 {
				panic("internal error")
			}

			for i, v := range x.Values {
				r += l.vsize(v, u.Fields[i])
			}
		case
			*ir.AddressValue,
			*ir.Int32Value:
			// nop
		default:
			panic(fmt.Errorf("%v:%v, %T", t.ID(), t.Kind(), x))
		}
	default:
		panic(fmt.Errorf("%v:%v", t.ID(), t.Kind()))
	}
	return roundup(r, mallocAlign)
}

func (l *loader) size(v ir.Value, t ir.Type) (r int) {
	r = l.sizeof(t.ID())
	switch t.Kind() {
	case ir.Array:
		u := t.(*ir.ArrayType)
		switch x := v.(type) {
		case *ir.CompositeValue:
			for _, v := range x.Values {
				r += l.vsize(v, u.Item)
			}
		case
			*ir.StringValue,
			*ir.WideStringValue:
			// nop
		default:
			panic(fmt.Errorf("%v:%v, %T", t.ID(), t.Kind(), x))
		}
	case
		ir.Int8, ir.Int16, ir.Int32, ir.Int64,
		ir.Uint8, ir.Uint16, ir.Uint32, ir.Uint64,
		ir.Float32, ir.Float64,
		ir.Complex64, ir.Complex128,
		ir.Pointer:
		// nop
	case ir.Struct, ir.Union:
		switch x := v.(type) {
		case *ir.CompositeValue:
			if t.Kind() == ir.Union && len(x.Values) > 1 {
				panic("internal error")
			}

			u := t.(*ir.StructOrUnionType)
			for i, v := range x.Values {
				r += l.vsize(v, u.Fields[i])
			}
		default:
			panic(fmt.Errorf("%v:%v, %T", t.ID(), t.Kind(), x))
		}
	default:
		panic(fmt.Errorf("%v:%v", t.ID(), t.Kind()))
	}
	return r
}

func (l *loader) load() error {
	var buf buffer.Bytes
	var swo []ir.Object
	for fi, v := range l.objects {
		switch x := v.(type) {
		case *ir.FunctionDefinition:
			fn := x.NameID
			for _, v := range x.Body {
				switch x := v.(type) {
				case *ir.Switch:
					var a switchPairs
					for i, v := range x.Values {
						a = append(a, switchPair{v, &x.Labels[i]})
					}
					sort.Sort(a)
					buf.Reset()
					fmt.Fprintf(&buf, "struct{ int64")
					for _, v := range a {
						switch x := v.Value.(type) {
						case *ir.Int32Value:
							fmt.Fprintf(&buf, ", int32")
						case *ir.Int64Value:
							fmt.Fprintf(&buf, ", int64")
						default:
							panic(fmt.Sprintf("%T", x))
						}
					}
					for i := 0; i <= len(a); i++ {
						fmt.Fprintf(&buf, ", *struct{}")
					}
					buf.WriteByte('}')
					cv := &ir.CompositeValue{Values: []ir.Value{&ir.Int32Value{Value: int32(len(a))}}}
					d := &ir.DataDefinition{
						ObjectBase: ir.ObjectBase{
							Position: x.Position,
							Linkage:  ir.InternalLinkage,
							TypeID:   ir.TypeID(dict.ID(buf.Bytes())),
						},
						Value: cv,
					}
					for _, v := range a {
						cv.Values = append(cv.Values, v.Value)
					}
					for _, v := range a {
						cv.Values = append(cv.Values, &ir.AddressValue{
							Index:  fi,
							Label:  ir.NameID(-v.Label.Number),
							NameID: fn,
						})
					}
					cv.Values = append(cv.Values, &ir.AddressValue{
						Index:  fi,
						Label:  ir.NameID(-x.Default.Number),
						NameID: fn,
					})
					index := len(l.objects) + len(swo)
					l.switches[x] = index
					swo = append(swo, d)
				}
			}
		}
	}
	l.objects = append(l.objects, swo...)
	var ds int
	for i, v := range l.objects { // Allocate global initialized data.
		switch x := v.(type) {
		case *ir.DataDefinition:
			if x.Value != nil {
				l.m[i] = ds
				ds += roundup(l.size(x.Value, l.tc.MustType(x.TypeID)), mallocAlign)
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
			if x.Linkage == ir.ExternalLinkage {
				l.out.Sym[x.NameID] = len(l.out.Code) // FFI address.
			}
			if op, ok := builtins[x.NameID]; ok && len(x.Body) == 1 {
				if _, ok := x.Body[0].(*ir.Panic); ok {
					l.m[i] = len(l.out.Code)
					l.loadBuiltin(op, x)
					break
				}
			}

			l.out.Code = append(l.out.Code, Operation{Call, i}, Operation{FFIReturn, 0})
			l.m[i] = len(l.out.Code)
			l.loadFunctionDefinition(i, x)
		}
	}
	for i, v := range l.out.Code {
		switch v.Opcode {
		case Call:
			n, ok := l.m[v.N]
			if !ok {
				return fmt.Errorf("%#05x: undefined object #%v", i, v.N)
			}

			l.out.Code[i].N = n
		case FP:
			n, ok := l.m[v.N]
			if !ok {
				return fmt.Errorf("%#05x: undefined object #%v", i, v.N)
			}

			l.out.Code[i].N = n
		case Push32, Push64:
			v, ok := l.csLabels[i]
			if !ok {
				break
			}

			nfo := labelNfo{index: v.Index, nm: v.Label}
			ip, ok := l.namedLabels[nfo]
			if !ok {
				return fmt.Errorf("%s: undefined label %s", l.objects[v.Index].Base().NameID, v.Label)
			}

			l.out.Code[i].N = ip
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
	for off, v := range l.dsLabels {
		nfo := labelNfo{index: v.Index, nm: v.Label}
		ip, ok := l.namedLabels[nfo]
		if !ok {
			return fmt.Errorf("%s: undefined label %s", l.objects[v.Index].Base().NameID, v.Label)
		}

		*(*uintptr)(unsafe.Pointer(&l.out.Data[off])) = uintptr(ip)
	}
	for off, v := range l.tsLabels {
		nfo := labelNfo{index: v.Index, nm: v.Label}
		ip, ok := l.namedLabels[nfo]
		if !ok {
			return fmt.Errorf("%s: undefined label %s", l.objects[v.Index].Base().NameID, v.Label)
		}

		*(*uintptr)(unsafe.Pointer(&l.out.Text[off])) = uintptr(ip)
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
	return nil
}

// LoadMain translates program in objects into a Binary or an error, if any.
// It's the caller responsibility to ensure the objects were produced for this
// architecture and platform.
func LoadMain(objects []ir.Object) (_ *Binary, err error) {
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

	l := newLoader(objects)
	if err := l.load(); err != nil {
		return nil, err
	}

	return l.out, nil
}
