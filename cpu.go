// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"bytes"
	"errors"
	"go/token"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"text/tabwriter"

	"fmt"
	"unsafe"

	"github.com/cznic/internal/buffer"
)

var (
	maxConvF32I32 = math.Nextafter32(math.MaxInt32, 0) // https://github.com/golang/go/issues/19405
	maxConvF32U32 = math.Nextafter32(math.MaxUint32, 0)
)

// Operation is the machine code.
type Operation struct {
	Opcode
	N int
}

// typedef void *__JMP_BUF_TYPE__[7];
type jmpBuf struct {
	ap       uintptr // Arguments pointer
	bp       uintptr // Base pointer
	fpStackP uintptr
	ip       uintptr // Instruction pointer
	rp       uintptr // Results pointer
	rpStackP uintptr
	sp       uintptr // Stack pointer
}

type cpu struct {
	jmpBuf

	code    []Operation
	ds      uintptr // Data segment
	fpStack []uintptr
	m       *Machine
	rpStack []uintptr
	stop    chan struct{}
	thread  *Thread
	tls     uintptr
	tlsp    *tls
	ts      uintptr // Text segment
}

func addPtr(p uintptr, v uintptr)         { *(*uintptr)(unsafe.Pointer(p)) += v }
func popF64(p uintptr) (uintptr, float64) { return p + f64StackSz, readF64(p) }
func popI32(p uintptr) (uintptr, int32)   { return p + i32StackSz, readI32(p) }
func popI64(p uintptr) (uintptr, int64)   { return p + i64StackSz, readI64(p) }
func popLong(p uintptr) (uintptr, int64)  { return p + longStackSz, readLong(p) }
func popPtr(p uintptr) (uintptr, uintptr) { return p + ptrStackSz, readPtr(p) }
func readC128(p uintptr) complex128       { return *(*complex128)(unsafe.Pointer(p)) }
func readC64(p uintptr) complex64         { return *(*complex64)(unsafe.Pointer(p)) }
func readF32(p uintptr) float32           { return *(*float32)(unsafe.Pointer(p)) }
func readF64(p uintptr) float64           { return *(*float64)(unsafe.Pointer(p)) }
func readI16(p uintptr) int16             { return *(*int16)(unsafe.Pointer(p)) }
func readI32(p uintptr) int32             { return *(*int32)(unsafe.Pointer(p)) }
func readI64(p uintptr) int64             { return *(*int64)(unsafe.Pointer(p)) }
func readI8(p uintptr) int8               { return *(*int8)(unsafe.Pointer(p)) }
func readPtr(p uintptr) uintptr           { return *(*uintptr)(unsafe.Pointer(p)) }
func readU16(p uintptr) uint16            { return *(*uint16)(unsafe.Pointer(p)) }
func readU32(p uintptr) uint32            { return *(*uint32)(unsafe.Pointer(p)) }
func readU64(p uintptr) uint64            { return *(*uint64)(unsafe.Pointer(p)) }
func readU8(p uintptr) uint8              { return *(*uint8)(unsafe.Pointer(p)) }
func writeC128(p uintptr, v complex128)   { *(*complex128)(unsafe.Pointer(p)) = v }
func writeC64(p uintptr, v complex64)     { *(*complex64)(unsafe.Pointer(p)) = v }
func writeF32(p uintptr, v float32)       { *(*float32)(unsafe.Pointer(p)) = v }
func writeF64(p uintptr, v float64)       { *(*float64)(unsafe.Pointer(p)) = v }
func writeI16(p uintptr, v int16)         { *(*int16)(unsafe.Pointer(p)) = v }
func writeI32(p uintptr, v int32)         { *(*int32)(unsafe.Pointer(p)) = v }
func writeI64(p uintptr, v int64)         { *(*int64)(unsafe.Pointer(p)) = v }
func writeI8(p uintptr, v int8)           { *(*int8)(unsafe.Pointer(p)) = v }
func writePtr(p uintptr, v uintptr)       { *(*uintptr)(unsafe.Pointer(p)) = v }
func writeU16(p uintptr, v uint16)        { *(*uint16)(unsafe.Pointer(p)) = v }
func writeU32(p uintptr, v uint32)        { *(*uint32)(unsafe.Pointer(p)) = v }
func writeU64(p uintptr, v uint64)        { *(*uint64)(unsafe.Pointer(p)) = v }
func writeU8(p uintptr, v uint8)          { *(*uint8)(unsafe.Pointer(p)) = v }

func (c *cpu) bool(b bool) {
	if b {
		writeI32(c.sp, 1)
		return
	}

	writeI32(c.sp, 0)
}

func (c *cpu) builtin(f func()) {
	f()
	n := len(c.rpStack)
	c.sp = c.rp
	c.rp = c.rpStack[n-1]
	c.rpStack = c.rpStack[:n-1]
}

func (c *cpu) setErrno(err interface{}) {
	switch x := err.(type) {
	case int:
		writeI32(c.tls+unsafe.Offsetof(tls{}.errno), int32(x))
	case *os.PathError:
		c.setErrno(x.Err)
	case syscall.Errno:
		writeI32(c.tls+unsafe.Offsetof(tls{}.errno), int32(x))
	default:
		panic(fmt.Errorf("TODO %T(%#v)", x, x))
	}
}

func (c *cpu) stackTrace() (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("stackTrace: %v", e)
		}
	}()

	var buf, lbuf buffer.Bytes
	m := map[string][][]byte{}
	bp := c.bp
	ip := c.ip - 1
	sp := c.sp
	ap := c.ap
	for ip < uintptr(len(c.code)) {
		fi := c.m.pcInfo(int(ip), c.m.functions)
		li := c.m.pcInfo(int(ip), c.m.lines)
		switch p := li.Position(); {
		case p.IsValid():
			fmt.Fprintf(&buf, "%s.%s(", p.Filename, dict.S(int(fi.Name)))
			for i := 0; ap > bp+3*stackAlign; i++ {
				if i != 0 {
					fmt.Fprintf(&buf, ", ")
				}
				ap -= stackAlign
				fmt.Fprintf(&buf, "%#x", readULong(ap))
			}
			fmt.Fprintf(&buf, ")\n")
			fmt.Fprintf(&buf, "\t%s\t", li.Position())
			dumpCode(&lbuf, c.code[ip:ip+1], int(ip), nil, nil)
			b := lbuf.Bytes()
			buf.Write(b[:len(b)-1])
			lbuf.Reset()
			src, ok := m[p.Filename]
			if !ok {
				f := p.Filename
				if !filepath.IsAbs(f) {
					f = filepath.Join(c.m.tracePath, p.Filename)
				}
				b, err := ioutil.ReadFile(f)
				if err == nil {
					src = bytes.Split(b, []byte{'\n'})
				}
				m[p.Filename] = src
			}
			if p.Line-1 < len(src) {
				fmt.Fprintf(&buf, "\t// %s", bytes.TrimSpace(src[p.Line-1]))
			}
			buf.WriteByte('\n')
		default:
			dumpCode(&buf, c.code[ip:ip+1], int(ip), nil, nil)
		}
		sp = bp
		bp = readPtr(sp)
		sp += ptrStackSz
		ap = readPtr(sp)
		sp += ptrStackSz
		if i := sp - c.thread.ss; int(i) >= len(c.thread.stackMem)-int(tlsStackSize) || bp == 0 || sp == 0 || ap == 0 {
			break
		}

		ip = readPtr(sp) - 1
	}
	return errors.New(string(buf.Bytes()))
}

var prev token.Position

func (c *cpu) trace(w io.Writer) {
	h := c.ip + 1
	for h < uintptr(len(c.code)) && c.code[h].Opcode == Ext {
		h++
	}
	w.Write(dumpCodeStr(c.code[c.ip:h], int(c.ip), c.m.functions, c.m.lines))
}

func (c *cpu) run(ip uintptr) (int, error) {
	var tracew *tabwriter.Writer
	if trace {
		tracew = new(tabwriter.Writer)
		tracew.Init(os.Stderr, 0, 8, 0, '\t', 0)

		defer tracew.Flush()

	}
	c.code = c.m.code
	c.ip = ip
	//fmt.Printf("%#v\n", c)
	defer func() {
		if err := recover(); err != nil {
			switch {
			case Testing:
				panic(fmt.Errorf("%v\n%s\n%s", err, c.stackTrace(), debug.Stack()))
			default:
				panic(fmt.Errorf("%v\n%s", err, c.stackTrace()))
			}
		}
	}()

	for i := 0; ; i++ {
		if i%1024 == 0 {
			select {
			case <-c.m.stop:
				return -1, KillError{}
			default:
			}
		}

		if trace {
			c.trace(tracew)
		}
		op := c.code[c.ip]
		if profile {
			if c.m.ProfileRate == 0 || i%c.m.ProfileRate == 0 {
				if c.m.ProfileFunctions != nil {
					nfo := *c.m.pcInfo(int(c.ip), c.m.functions)
					nfo.PC = 0
					c.m.ProfileFunctions[nfo]++
				}
				if c.m.ProfileLines != nil {
					nfo := *c.m.pcInfo(int(c.ip), c.m.lines)
					nfo.PC = 0
					c.m.ProfileLines[nfo]++
				}
				if c.m.ProfileInstructions != nil {
					c.m.ProfileInstructions[op.Opcode]++
				}
			}
		}
		c.ip++
	main:
		switch op.Opcode {
		case AP: // -> ptr
			c.sp -= ptrStackSz
			writePtr(c.sp, c.ap+uintptr(op.N))
		case AddF32: // a, b -> a + b
			b := readF32(c.sp)
			c.sp += f32StackSz
			writeF32(c.sp, readF32(c.sp)+b)
		case AddF64: // a, b -> a + b
			b := readF64(c.sp)
			c.sp += f64StackSz
			writeF64(c.sp, readF64(c.sp)+b)
		case AddC64: // a, b -> a + b
			b := readC64(c.sp)
			c.sp += c64StackSz
			writeC64(c.sp, readC64(c.sp)+b)
		case AddC128: // a, b -> a + b
			b := readC128(c.sp)
			c.sp += c128StackSz
			writeC128(c.sp, readC128(c.sp)+b)
		case AddI32: // a, b -> a + b
			b := readI32(c.sp)
			c.sp += i32StackSz
			writeI32(c.sp, readI32(c.sp)+b)
		case AddI64: // a, b -> a + b
			b := readI64(c.sp)
			c.sp += i64StackSz
			writeI64(c.sp, readI64(c.sp)+b)
		case AddPtr:
			addPtr(c.sp, uintptr(op.N))
		case AddPtrs:
			v := readPtr(c.sp)
			c.sp += ptrStackSz
			addPtr(c.sp, v)
		case AddSP: // -
			c.sp += uintptr(op.N)
		case And8: // a, b -> a & b
			b := readI8(c.sp)
			c.sp += i8StackSz
			writeI8(c.sp, readI8(c.sp)&b)
		case And16: // a, b -> a & b
			b := readI16(c.sp)
			c.sp += i16StackSz
			writeI16(c.sp, readI16(c.sp)&b)
		case And32: // a, b -> a & b
			b := readI32(c.sp)
			c.sp += i32StackSz
			writeI32(c.sp, readI32(c.sp)&b)
		case And64: // a, b -> a & b
			b := readI64(c.sp)
			c.sp += i64StackSz
			writeI64(c.sp, readI64(c.sp)&b)
		case Argument: // -> val
			off := op.N
			op = c.code[c.ip]
			c.ip++
			sz := op.N
			c.sp -= uintptr(roundup(sz, stackAlign))
			movemem(c.sp, c.ap+uintptr(off), sz)
		case Argument8: // -> val
			c.sp -= i8StackSz
			writeI8(c.sp, readI8(c.ap+uintptr(op.N)))
		case Argument16: // -> val
			c.sp -= i16StackSz
			writeI16(c.sp, readI16(c.ap+uintptr(op.N)))
		case Argument32: // -> val
			c.sp -= i32StackSz
			writeI32(c.sp, readI32(c.ap+uintptr(op.N)))
		case Argument64: // -> val
			c.sp -= i64StackSz
			writeI64(c.sp, readI64(c.ap+uintptr(op.N)))
		case Arguments: // -
			c.rpStack = append(c.rpStack, c.rp)
			c.rp = c.sp
		case ArgumentsFP: // -
			c.rpStack = append(c.rpStack, c.rp)
			c.fpStack = append(c.fpStack, readPtr(c.sp))
			c.sp += ptrStackSz
			c.rp = c.sp
		case BP: // -> ptr
			c.sp -= ptrSize
			writePtr(c.sp, c.bp+uintptr(op.N))
		case BitfieldI8: //  val -> val
			writeI8(c.sp, readI8(c.sp)<<uint(op.N>>8)>>uint(op.N&63))
		case BitfieldI16: //  val -> val
			writeI16(c.sp, readI16(c.sp)<<uint(op.N>>8)>>uint(op.N&63))
		case BitfieldI32: //  val -> val
			writeI32(c.sp, readI32(c.sp)<<uint(op.N>>8)>>uint(op.N&63))
		case BitfieldI64: //  val -> val
			writeI64(c.sp, readI64(c.sp)<<uint(op.N>>8)>>uint(op.N&63))
		case BitfieldU8: //  val -> val
			writeU8(c.sp, readU8(c.sp)<<uint(op.N>>8)>>uint(op.N&63))
		case BitfieldU16: //  val -> val
			writeU16(c.sp, readU16(c.sp)<<uint(op.N>>8)>>uint(op.N&63))
		case BitfieldU32: //  val -> val
			writeU32(c.sp, readU32(c.sp)<<uint(op.N>>8)>>uint(op.N&63))
		case BitfieldU64: //  val -> val
			writeU64(c.sp, readU64(c.sp)<<uint(op.N>>8)>>uint(op.N&63))
		case BoolC128:
			v := readC128(c.sp)
			c.sp += c128StackSz - i32StackSz
			c.bool(v != 0)
		case BoolF32:
			c.bool(readF32(c.sp) != 0)
		case BoolF64:
			v := readF64(c.sp)
			c.sp += f64StackSz - i32StackSz
			c.bool(v != 0)
		case BoolI8:
			v := readI8(c.sp)
			c.sp += i8StackSz - i32StackSz
			c.bool(v != 0)
		case BoolI16:
			v := readI16(c.sp)
			c.sp += i16StackSz - i32StackSz
			c.bool(v != 0)
		case BoolI32:
			c.bool(readI32(c.sp) != 0)
		case BoolI64:
			v := readI64(c.sp)
			c.sp += i64StackSz - i32StackSz
			c.bool(v != 0)
		case Call: // -> results
			c.sp -= ptrStackSz
			writePtr(c.sp, c.ip)
			c.ip = uintptr(op.N)
		case CallFP: // -> results
			c.sp -= ptrStackSz
			writePtr(c.sp, c.ip)
			n := len(c.fpStack)
			c.ip = c.fpStack[n-1]
			c.fpStack = c.fpStack[:n-1]
		case ConvC64C128:
			v := readC64(c.sp)
			c.sp -= c128StackSz - c64StackSz
			writeC128(c.sp, complex128(v))
		case ConvF32C64:
			v := readF32(c.sp)
			c.sp -= c64StackSz - f32StackSz
			writeC64(c.sp, complex(v, 0))
		case ConvF32C128:
			v := readF32(c.sp)
			c.sp -= c128StackSz - f32StackSz
			writeC128(c.sp, complex(float64(v), 0))
		case ConvF32F64:
			v := readF32(c.sp)
			c.sp -= f64StackSz - f32StackSz
			writeF64(c.sp, float64(v))
		case ConvF32I32:
			switch v := readF32(c.sp); {
			case v > maxConvF32I32:
				c.sp += f32StackSz - i32StackSz
				writeI32(c.sp, math.MaxInt32)
			default:
				c.sp += f32StackSz - i32StackSz
				writeI32(c.sp, int32(v))
			}
		case ConvF32I64:
			v := readF32(c.sp)
			c.sp -= i64StackSz - f32StackSz
			writeI64(c.sp, int64(v))
		case ConvF32U32:
			switch v := readF32(c.sp); {
			case v > maxConvF32U32:
				c.sp += f32StackSz - i32StackSz
				writeU32(c.sp, math.MaxUint32)
			default:
				c.sp += f32StackSz - i32StackSz
				writeU32(c.sp, uint32(v))
			}
		case ConvF64F32:
			v := readF64(c.sp)
			c.sp += f64StackSz - f32StackSz
			writeF32(c.sp, float32(v))
		case ConvF64C128:
			v := readF64(c.sp)
			c.sp -= c128StackSz - f64StackSz
			writeC128(c.sp, complex(v, 0))
		case ConvF64U16:
			v := readF64(c.sp)
			c.sp += f64StackSz - i16StackSz
			writeU16(c.sp, uint16(v))
		case ConvF64I32:
			v := readF64(c.sp)
			c.sp += f64StackSz - i32StackSz
			writeI32(c.sp, int32(v))
		case ConvF64U32:
			v := readF64(c.sp)
			c.sp += f64StackSz - i32StackSz
			writeU32(c.sp, uint32(v))
		case ConvF64U64:
			writeU64(c.sp, uint64(readF64(c.sp)))
		case ConvF64I64:
			writeI64(c.sp, int64(readF64(c.sp)))
		case ConvF64I8:
			v := readF64(c.sp)
			c.sp += f64StackSz - i8StackSz
			writeI8(c.sp, int8(v))
		case ConvI32F32:
			v := readI32(c.sp)
			c.sp += i32StackSz - f32StackSz
			writeF32(c.sp, float32(v))
		case ConvI32F64:
			v := readI32(c.sp)
			c.sp -= f64StackSz - i32StackSz
			writeF64(c.sp, float64(v))
		case ConvI32I64:
			v := readI32(c.sp)
			c.sp -= i64StackSz - i32StackSz
			writeI64(c.sp, int64(v))
		case ConvI64:
			v := readI64(c.sp)
			c.sp += i64StackSz
			c.sp -= uintptr(op.N)
			for p, n := c.sp, op.N; n > 0; n -= ptrStackSz {
				writePtr(p, 0)
				p += ptrStackSz
			}
			writeI64(c.sp, v)
		case ConvI64I8:
			v := readI64(c.sp)
			c.sp += i64StackSz - i8StackSz
			writeI8(c.sp, int8(v))
		case ConvI64I16:
			v := readI64(c.sp)
			c.sp += i64StackSz - i16StackSz
			writeI16(c.sp, int16(v))
		case ConvI64I32:
			v := readI64(c.sp)
			c.sp += i64StackSz - i32StackSz
			writeI32(c.sp, int32(v))
		case ConvI64F64:
			v := readI64(c.sp)
			c.sp += i64StackSz - f64StackSz
			writeF64(c.sp, float64(v))
		case ConvI64U16:
			v := readI64(c.sp)
			c.sp += i64StackSz - i16StackSz
			writeU16(c.sp, uint16(v))
		case ConvI16I32:
			writeI32(c.sp, int32(readI16(c.sp)))
		case ConvI16U32:
			writeU32(c.sp, uint32(readI16(c.sp)))
		case ConvI16I64:
			v := readI16(c.sp)
			c.sp -= i64StackSz - i16StackSz
			writeI64(c.sp, int64(v))
		case ConvI32C64:
			v := readI32(c.sp)
			c.sp -= c64StackSz - i32StackSz
			writeC64(c.sp, complex(float32(v), 0))
		case ConvI32C128:
			v := readI32(c.sp)
			c.sp -= c128StackSz - i32StackSz
			writeC128(c.sp, complex(float64(v), 0))
		case ConvI32I8:
			writeI8(c.sp, int8(readI32(c.sp)))
		case ConvI32I16:
			writeI16(c.sp, int16(readI32(c.sp)))
		case ConvI8I16:
			writeI16(c.sp, int16(readI8(c.sp)))
		case ConvI8I32:
			writeI32(c.sp, int32(readI8(c.sp)))
		case ConvI8U32:
			writeU32(c.sp, uint32(readI8(c.sp)))
		case ConvI8I64:
			v := readI8(c.sp)
			c.sp -= i64StackSz - i8StackSz
			writeI64(c.sp, int64(v))
		case ConvI8F64:
			v := readI8(c.sp)
			c.sp -= f64StackSz - i8StackSz
			writeF64(c.sp, float64(v))
		case ConvU8I16:
			writeI16(c.sp, int16(readU8(c.sp)))
		case ConvU8I32:
			writeI32(c.sp, int32(readU8(c.sp)))
		case ConvU8U32:
			writeU32(c.sp, uint32(readU8(c.sp)))
		case ConvU8U64:
			v := readU8(c.sp)
			c.sp -= i64StackSz - i8StackSz
			writeU64(c.sp, uint64(v))
		case ConvU16I32:
			writeI32(c.sp, int32(readU16(c.sp)))
		case ConvU16U32:
			writeU32(c.sp, uint32(readU16(c.sp)))
		case ConvU16U64:
			v := readU16(c.sp)
			c.sp -= i64StackSz - i16StackSz
			writeU64(c.sp, uint64(v))
		case ConvU16I64:
			v := readU16(c.sp)
			c.sp -= i64StackSz - i16StackSz
			writeI64(c.sp, int64(v))
		case ConvU32U8:
			v := readU32(c.sp)
			c.sp += i32StackSz - i8StackSz
			writeU8(c.sp, uint8(v))
		case ConvU32I16:
			v := readU32(c.sp)
			c.sp += i32StackSz - i16StackSz
			writeI16(c.sp, int16(v))
		case ConvU32I64:
			v := readU32(c.sp)
			c.sp -= i64StackSz - i32StackSz
			writeI64(c.sp, int64(v))
		case ConvU32F32:
			v := readU32(c.sp)
			c.sp += i32StackSz - f32StackSz
			writeF32(c.sp, float32(v))
		case ConvU32F64:
			v := readU32(c.sp)
			c.sp -= f64StackSz - i32StackSz
			writeF64(c.sp, float64(v))
		case Copy: // &dst, &src -> &dst
			src := readPtr(c.sp)
			c.sp += ptrStackSz
			movemem(readPtr(c.sp), src, op.N)
		case Cpl8: // a -> ^a
			writeI8(c.sp, ^readI8(c.sp))
		case Cpl32: // a -> ^a
			writeI32(c.sp, ^readI32(c.sp))
		case Cpl64: // a -> ^a
			writeI64(c.sp, ^readI64(c.sp))
		case DS: // -> ptr
			c.sp -= ptrSize
			writePtr(c.sp, c.ds+uintptr(op.N))
		case DSN: // -> val
			off := op.N
			op = c.code[c.ip]
			c.ip++
			sz := op.N
			c.sp -= uintptr(roundup(sz, stackAlign))
			movemem(c.sp, c.ds+uintptr(off), sz)
		case DSI8: // -> val
			c.sp -= i8StackSz
			writeI8(c.sp, readI8(c.ds+uintptr(op.N)))
		case DSI16: // -> val
			c.sp -= i16StackSz
			writeI16(c.sp, readI16(c.ds+uintptr(op.N)))
		case DSI32: // -> val
			c.sp -= i32StackSz
			writeI32(c.sp, readI32(c.ds+uintptr(op.N)))
		case DSI64: // -> val
			c.sp -= i64StackSz
			writeI64(c.sp, readI64(c.ds+uintptr(op.N)))
		case DSC128: // -> val
			c.sp -= c128StackSz
			writeC128(c.sp, readC128(c.ds+uintptr(op.N)))
		case DivF32: // a, b -> a / b
			b := readF32(c.sp)
			c.sp += f32StackSz
			writeF32(c.sp, readF32(c.sp)/b)
		case DivC64: // a, b -> a / b
			b := readC64(c.sp)
			c.sp += c64StackSz
			writeC64(c.sp, readC64(c.sp)/b)
		case DivC128: // a, b -> a / b
			b := readC128(c.sp)
			c.sp += c128StackSz
			writeC128(c.sp, readC128(c.sp)/b)
		case DivF64: // a, b -> a / b
			b := readF64(c.sp)
			c.sp += f64StackSz
			writeF64(c.sp, readF64(c.sp)/b)
		case DivI32: // a, b -> a / b
			b := readI32(c.sp)
			c.sp += i32StackSz
			writeI32(c.sp, readI32(c.sp)/b)
		case DivU32: // a, b -> a / b
			b := readU32(c.sp)
			c.sp += i32StackSz
			writeU32(c.sp, readU32(c.sp)/b)
		case DivI64: // a, b -> a / b
			b := readI64(c.sp)
			c.sp += i64StackSz
			writeI64(c.sp, readI64(c.sp)/b)
		case DivU64: // a, b -> a / b
			b := readU64(c.sp)
			c.sp += i64StackSz
			writeU64(c.sp, readU64(c.sp)/b)
		case Dup8:
			v := readI8(c.sp)
			c.sp -= i8StackSz
			writeI8(c.sp, v)
		case Dup32:
			v := readI32(c.sp)
			c.sp -= i32StackSz
			writeI32(c.sp, v)
		case Dup64:
			v := readI64(c.sp)
			c.sp -= i64StackSz
			writeI64(c.sp, v)
		case EqI8: // a, b -> a == b
			b := readI8(c.sp)
			c.sp += i8StackSz
			a := readI8(c.sp)
			c.bool(a == b)
		case EqI32: // a, b -> a == b
			b := readI32(c.sp)
			c.sp += i32StackSz
			a := readI32(c.sp)
			c.bool(a == b)
		case EqF32: // a, b -> a == b
			b := readF32(c.sp)
			c.sp += f32StackSz
			a := readF32(c.sp)
			c.bool(a == b)
		case EqF64: // a, b -> a == b
			b := readF64(c.sp)
			c.sp += f64StackSz
			a := readF64(c.sp)
			c.sp += f64StackSz - i32StackSz
			c.bool(a == b)
		case EqI64: // a, b -> a == b
			b := readI64(c.sp)
			c.sp += i64StackSz
			a := readI64(c.sp)
			c.sp += i64StackSz - i32StackSz
			c.bool(a == b)
		case FFIReturn:
			return 0, nil
		case FP:
			c.sp -= ptrStackSz
			if ptrStackSz == 4 {
				writeI32(c.sp, int32(op.N))
			} else {
				writeI64(c.sp, int64(op.N))
			}
		case Field8:
			v := readI8(c.sp + uintptr(op.N))
			op = c.code[c.ip]
			c.ip++
			sz := op.N
			c.sp += uintptr(sz) - i8StackSz
			writeI8(c.sp, v)
		case Field16:
			v := readI16(c.sp + uintptr(op.N))
			op = c.code[c.ip]
			c.ip++
			sz := op.N
			c.sp += uintptr(sz) - i16StackSz
			writeI16(c.sp, v)
		case Field64:
			v := readI64(c.sp + uintptr(op.N))
			op = c.code[c.ip]
			c.ip++
			sz := op.N
			c.sp += uintptr(sz) - i64StackSz
			writeI64(c.sp, v)
		case Func: // N: bp offset of variable[n-1])

			// ...higher addresses
			//
			// +--------------------+
			// | result 0           |
			// +--------------------+
			// ...
			// +--------------------+
			// | result n-1         | <- rp
			// +--------------------+
			// | argument 0         |
			// +--------------------+
			// ...
			// +--------------------+
			// | argument n-1       |
			// +--------------------+
			// | return addr        | <- sp
			// +--------------------+
			//
			// ...lower addresses

			c.sp -= ptrStackSz
			writePtr(c.sp, c.ap)
			c.ap = c.rp
			c.sp -= ptrStackSz
			writePtr(c.sp, c.bp)
			c.bp = c.sp
			c.sp = (c.sp + uintptr(op.N)) &^ 0xf // Force 16-byte stack alignment.

			// ...higher addresses
			//
			// +--------------------+
			// | result 0           |
			// +--------------------+
			// ...
			// +--------------------+
			// | result n-1         | <- ap
			// +--------------------+
			// | argument 0         |
			// +--------------------+
			// ...
			// +--------------------+
			// | argument n-1       |
			// +--------------------+
			// | return addr        |
			// +--------------------+
			// | saved ap           |
			// +--------------------+
			// | saved bp           | <- bp
			// +--------------------+
			// | variable 0         |
			// +--------------------+
			// ...
			// +--------------------+
			// | variable n-1       | <- sp
			// +--------------------+
			//
			// ...lower addresses
			//
			// result[i]	ap + sum(stack size result[0..n-1]) - sum(stack size result[0..i])
			// argument[i]	ap - sum(stack size argument[0..i])
			// variable[i]	bp - sum(stack size variable[0..i])

		case GeqF32: // a, b -> a >= b
			b := readF32(c.sp)
			c.sp += f32StackSz
			a := readF32(c.sp)
			c.sp += f32StackSz - i32StackSz
			c.bool(a >= b)
		case GeqF64: // a, b -> a >= b
			b := readF64(c.sp)
			c.sp += f64StackSz
			a := readF64(c.sp)
			c.sp += f64StackSz - i32StackSz
			c.bool(a >= b)
		case GeqI8: // a, b -> a >= b
			b := readI8(c.sp)
			c.sp += i8StackSz
			a := readI8(c.sp)
			c.bool(a >= b)
		case GeqI32: // a, b -> a >= b
			b := readI32(c.sp)
			c.sp += i32StackSz
			a := readI32(c.sp)
			c.bool(a >= b)
		case GeqU32: // a, b -> a >= b
			b := readU32(c.sp)
			c.sp += i32StackSz
			a := readU32(c.sp)
			c.bool(a >= b)
		case GeqI64: // a, b -> a >= b
			b := readI64(c.sp)
			c.sp += i64StackSz
			a := readI64(c.sp)
			c.sp += i64StackSz - i32StackSz
			c.bool(a >= b)
		case GeqU64: // a, b -> a >= b
			b := readU64(c.sp)
			c.sp += i64StackSz
			a := readU64(c.sp)
			c.sp += i64StackSz - i32StackSz
			c.bool(a >= b)
		case GtF32: // a, b -> a > b
			b := readF32(c.sp)
			c.sp += f32StackSz
			a := readF32(c.sp)
			c.sp += f32StackSz - i32StackSz
			c.bool(a > b)
		case GtF64: // a, b -> a > b
			b := readF64(c.sp)
			c.sp += f64StackSz
			a := readF64(c.sp)
			c.sp += f64StackSz - i32StackSz
			c.bool(a > b)
		case GtI32: // a, b -> a > b
			b := readI32(c.sp)
			c.sp += i32StackSz
			a := readI32(c.sp)
			c.bool(a > b)
		case GtI64: // a, b -> a > b
			b := readI64(c.sp)
			c.sp += i64StackSz
			a := readI64(c.sp)
			c.sp += i64StackSz - i32StackSz
			c.bool(a > b)
		case GtU32: // a, b -> a > b
			b := readU32(c.sp)
			c.sp += i32StackSz
			a := readU32(c.sp)
			c.bool(a > b)
		case GtU64: // a, b -> a > b
			b := readU64(c.sp)
			c.sp += i64StackSz
			a := readU64(c.sp)
			c.sp += i64StackSz - i32StackSz
			c.bool(a > b)
		case IndexI8: // addr, index -> addr + n*index
			x := readI8(c.sp)
			c.sp += i8StackSz
			addPtr(c.sp, uintptr(op.N*int(x)))
		case IndexU8: // addr, index -> addr + n*index
			x := readU8(c.sp)
			c.sp += i8StackSz
			addPtr(c.sp, uintptr(op.N*int(x)))
		case IndexI16: // addr, index -> addr + n*index
			x := readI16(c.sp)
			c.sp += i16StackSz
			addPtr(c.sp, uintptr(op.N*int(x)))
		case IndexU16: // addr, index -> addr + n*index
			x := readU16(c.sp)
			c.sp += i16StackSz
			addPtr(c.sp, uintptr(op.N*int(x)))
		case IndexI32: // addr, index -> addr + n*index
			x := readI32(c.sp)
			c.sp += i32StackSz
			addPtr(c.sp, uintptr(op.N*int(x)))
		case IndexU32: // addr, index -> addr + n*index
			x := readU32(c.sp)
			c.sp += i32StackSz
			addPtr(c.sp, uintptr(op.N*int(x)))
		case IndexI64: // addr, index -> addr + n*index
			x := readI64(c.sp)
			c.sp += i64StackSz
			addPtr(c.sp, uintptr(int64(op.N)*x))
		case IndexU64: // addr, index -> addr + n*index
			x := readU64(c.sp)
			c.sp += i64StackSz
			addPtr(c.sp, uintptr(uint64(op.N)*x))
		case Jmp: // -
			c.ip = uintptr(op.N)
		case JmpP: // ip -> -
			c.ip = readPtr(c.sp)
			c.sp -= ptrStackSz
		case Jnz: // val ->
			v := readI32(c.sp)
			c.sp += i32StackSz
			if v != 0 {
				c.ip = uintptr(op.N)
			}
		case Jz: // val ->
			v := readI32(c.sp)
			c.sp += i32StackSz
			if v == 0 {
				c.ip = uintptr(op.N)
			}
		case LeqI8: // a, b -> a <= b
			b := readI8(c.sp)
			c.sp += i8StackSz
			a := readI8(c.sp)
			c.bool(a <= b)
		case LeqI32: // a, b -> a <= b
			b := readI32(c.sp)
			c.sp += i32StackSz
			a := readI32(c.sp)
			c.bool(a <= b)
		case LeqU32: // a, b -> a <= b
			b := readU32(c.sp)
			c.sp += i32StackSz
			a := readU32(c.sp)
			c.bool(a <= b)
		case LeqI64: // a, b -> a <= b
			b := readI64(c.sp)
			c.sp += i64StackSz
			a := readI64(c.sp)
			c.sp += i64StackSz - i32StackSz
			c.bool(a <= b)
		case LeqU64: // a, b -> a <= b
			b := readU64(c.sp)
			c.sp += i64StackSz
			a := readU64(c.sp)
			c.sp += i64StackSz - i32StackSz
			c.bool(a <= b)
		case LeqF32: // a, b -> a <= b
			b := readF32(c.sp)
			c.sp += f32StackSz
			a := readF32(c.sp)
			c.sp += f32StackSz - i32StackSz
			c.bool(a <= b)
		case LeqF64: // a, b -> a <= b
			b := readF64(c.sp)
			c.sp += f64StackSz
			a := readF64(c.sp)
			c.sp += f64StackSz - i32StackSz
			c.bool(a <= b)
		case LshI8: // val, cnt -> val << cnt
			n := readI32(c.sp)
			c.sp += i32StackSz
			writeI8(c.sp, readI8(c.sp)<<uint(n))
		case LshI16: // val, cnt -> val << cnt
			n := readI32(c.sp)
			c.sp += i32StackSz
			writeI16(c.sp, readI16(c.sp)<<uint(n))
		case LshI64: // val, cnt -> val << cnt
			n := readI32(c.sp)
			c.sp += i32StackSz
			writeI64(c.sp, readI64(c.sp)<<uint(n))
		case LtI32: // a, b -> a < b
			b := readI32(c.sp)
			c.sp += i32StackSz
			a := readI32(c.sp)
			c.bool(a < b)
		case LtU32: // a, b -> a < b
			b := readU32(c.sp)
			c.sp += i32StackSz
			a := readU32(c.sp)
			c.bool(a < b)
		case LtI64: // a, b -> a < b
			b := readI64(c.sp)
			c.sp += i64StackSz
			a := readI64(c.sp)
			c.sp += i64StackSz - i32StackSz
			c.bool(a < b)
		case LtF32: // a, b -> a < b
			b := readF32(c.sp)
			c.sp += f32StackSz
			a := readF32(c.sp)
			c.sp += f32StackSz - i32StackSz
			c.bool(a < b)
		case LtF64: // a, b -> a < b
			b := readF64(c.sp)
			c.sp += f64StackSz
			a := readF64(c.sp)
			c.sp += f64StackSz - i32StackSz
			c.bool(a < b)
		case LtU64: // a, b -> a < b
			b := readU64(c.sp)
			c.sp += i64StackSz
			a := readU64(c.sp)
			c.sp += i64StackSz - i32StackSz
			c.bool(a < b)
		case Load: // addr -> (addr+n)
			p := readPtr(c.sp)
			off := op.N
			op = c.code[c.ip]
			c.ip++
			sz := op.N
			c.sp += ptrStackSz - uintptr(roundup(sz, stackAlign))
			movemem(c.sp, p+uintptr(off), sz)
		case Load8: // addr -> (addr+n)
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i8StackSz
			writeI8(c.sp, readI8(p+uintptr(op.N)))
		case Load16: // addr -> (addr+n)
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i16StackSz
			writeI16(c.sp, readI16(p+uintptr(op.N)))
		case Load32: // addr -> (addr+n)
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i32StackSz
			writeI32(c.sp, readI32(p+uintptr(op.N)))
		case Load64: // addr -> (addr+n)
			p := readPtr(c.sp)
			c.sp -= i64StackSz - ptrStackSz
			writeI64(c.sp, readI64(p+uintptr(op.N)))
		case LshI32: // val, cnt -> val << cnt
			n := readI32(c.sp)
			c.sp += i32StackSz
			writeI32(c.sp, readI32(c.sp)<<uint(n))
		case MulF32: // a, b -> a * b
			b := readF32(c.sp)
			c.sp += f32StackSz
			writeF32(c.sp, readF32(c.sp)*b)
		case MulC64: // a, b -> a * b
			b := readC64(c.sp)
			c.sp += c64StackSz
			writeC64(c.sp, readC64(c.sp)*b)
		case MulC128: // a, b -> a * b
			b := readC128(c.sp)
			c.sp += c128StackSz
			writeC128(c.sp, readC128(c.sp)*b)
		case MulF64: // a, b -> a * b
			b := readF64(c.sp)
			c.sp += f64StackSz
			writeF64(c.sp, readF64(c.sp)*b)
		case MulI32: // a, b -> a * b
			b := readI32(c.sp)
			c.sp += i32StackSz
			writeI32(c.sp, readI32(c.sp)*b)
		case MulI64: // a, b -> a * b
			b := readI64(c.sp)
			c.sp += i64StackSz
			writeI64(c.sp, readI64(c.sp)*b)
		case NegI8: // a -> -a
			writeI8(c.sp, -readI8(c.sp))
		case NegI16: // a -> -a
			writeI16(c.sp, -readI16(c.sp))
		case NegI32: // a -> -a
			writeI32(c.sp, -readI32(c.sp))
		case NegI64: // a -> -a
			writeI64(c.sp, -readI64(c.sp))
		case NegF32: // a -> -a
			writeF32(c.sp, -readF32(c.sp))
		case NegF64: // a -> -a
			writeF64(c.sp, -readF64(c.sp))
		case NegIndexU16: // addr, index -> addr - n*index
			x := readU16(c.sp)
			c.sp += i16StackSz
			addPtr(c.sp, uintptr(-op.N*int(x)))
		case NegIndexU32: // addr, index -> addr - n*index
			x := readU32(c.sp)
			c.sp += i32StackSz
			addPtr(c.sp, uintptr(-op.N*int(x)))
		case NegIndexI32: // addr, index -> addr - n*index
			x := readI32(c.sp)
			c.sp += i32StackSz
			addPtr(c.sp, uintptr(-op.N*int(x)))
		case NegIndexI64: // addr, index -> addr - n*index
			x := readI64(c.sp)
			c.sp += i64StackSz
			addPtr(c.sp, uintptr(-op.N*int(x)))
		case NegIndexU64: // addr, index -> addr - n*index
			x := readU64(c.sp)
			c.sp += i64StackSz
			addPtr(c.sp, uintptr(-op.N*int(x)))
		case NeqC64: // a, b -> a |= b
			b := readC64(c.sp)
			c.sp += c64StackSz
			a := readC64(c.sp)
			c.sp += c64StackSz - i32StackSz
			c.bool(a != b)
		case NeqC128: // a, b -> a |= b
			b := readC128(c.sp)
			c.sp += c128StackSz
			a := readC128(c.sp)
			c.sp += c128StackSz - i32StackSz
			c.bool(a != b)
		case NeqI8: // a, b -> a |= b
			b := readI8(c.sp)
			c.sp += i8StackSz
			a := readI8(c.sp)
			c.bool(a != b)
		case NeqI32: // a, b -> a |= b
			b := readI32(c.sp)
			c.sp += i32StackSz
			a := readI32(c.sp)
			c.bool(a != b)
		case NeqI64: // a, b -> a |= b
			b := readI64(c.sp)
			c.sp += i64StackSz
			a := readI64(c.sp)
			c.sp += i64StackSz - i32StackSz
			c.bool(a != b)
		case NeqF32: // a, b -> a |= b
			b := readF32(c.sp)
			c.sp += f32StackSz
			a := readF32(c.sp)
			c.sp += f32StackSz - i32StackSz
			c.bool(a != b)
		case NeqF64: // a, b -> a |= b
			b := readF64(c.sp)
			c.sp += f64StackSz
			a := readF64(c.sp)
			c.sp += f64StackSz - i32StackSz
			c.bool(a != b)
		case Nop: // -
			// nop
		case Not:
			c.bool(readI32(c.sp) == 0)
		case Or32: // a, b -> a | b
			b := readI32(c.sp)
			c.sp += i32StackSz
			writeI32(c.sp, readI32(c.sp)|b)
		case Or64: // a, b -> a | b
			b := readI64(c.sp)
			c.sp += i64StackSz
			writeI64(c.sp, readI64(c.sp)|b)
		case Panic: // -
			return -1, c.stackTrace()
		case PostIncI8: // adr -> (*adr)++
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i8StackSz
			v := readI8(p)
			writeI8(c.sp, v)
			writeI8(p, v+int8(op.N))
		case PostIncI16: // adr -> (*adr)++
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i16StackSz
			v := readI16(p)
			writeI16(c.sp, v)
			writeI16(p, v+int16(op.N))
		case PostIncI32: // adr -> (*adr)++
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i32StackSz
			v := readI32(p)
			writeI32(c.sp, v)
			writeI32(p, v+int32(op.N))
		case PostIncU32Bits: // adr -> (*adr)++
			d := uint64(op.N)
			op = c.code[c.ip]
			c.ip++
			bits := uint(op.N >> 16)
			bitoff := uint(op.N) >> 8 & 0xff
			w := op.N & 0xff
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i32StackSz
			m0 := uint64(1)<<bits - 1
			m := m0 << bitoff
			var u uint64
			switch w {
			case 1:
				u = uint64(readU8(p))
			case 2:
				u = uint64(readU16(p))
			case 4:
				u = uint64(readU32(p))
			case 8:
				u = readU64(p)
			default:
				return -1, fmt.Errorf("internal error: %v\n%s", op, c.stackTrace())
			}
			v := u & m >> bitoff
			writeU32(c.sp, uint32(v))
			v += d
			v &= m0
			u = u&^m | v<<bitoff&m
			switch w {
			case 1:
				writeU8(p, uint8(u))
			case 2:
				writeU16(p, uint16(u))
			case 4:
				writeU32(p, uint32(u))
			case 8:
				writeU64(p, u)
			default:
				return -1, fmt.Errorf("internal error: %v\n%s", op, c.stackTrace())
			}
		case PostIncU64Bits: // adr -> (*adr)++
			d := uint64(op.N)
			op = c.code[c.ip]
			c.ip++
			bits := uint(op.N >> 16)
			bitoff := uint(op.N) >> 8 & 0xff
			w := op.N & 0xff
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i32StackSz
			m0 := uint64(1)<<bits - 1
			m := m0 << bitoff
			var u uint64
			switch w {
			case 1:
				u = uint64(readU8(p))
			case 2:
				u = uint64(readU16(p))
			case 4:
				u = uint64(readU32(p))
			case 8:
				u = readU64(p)
			default:
				return -1, fmt.Errorf("internal error: %v\n%s", op, c.stackTrace())
			}
			v := u & m >> bitoff
			writeU64(c.sp, v)
			v += d
			v &= m0
			u = u&^m | v<<bitoff&m
			switch w {
			case 1:
				writeU8(p, uint8(u))
			case 2:
				writeU16(p, uint16(u))
			case 4:
				writeU32(p, uint32(u))
			case 8:
				writeU64(p, u)
			default:
				return -1, fmt.Errorf("internal error: %v\n%s", op, c.stackTrace())
			}
		case PostIncI64: // adr -> (*adr)++
			p := readPtr(c.sp)
			c.sp -= i64StackSz - ptrStackSz
			v := readI64(p)
			writeI64(c.sp, v)
			writeI64(p, v+int64(op.N))
		case PostIncF64: // adr -> (*adr)++
			p := readPtr(c.sp)
			c.sp -= f64StackSz - ptrStackSz
			v := readF64(p)
			writeF64(c.sp, v)
			writeF64(p, v+float64(op.N))
		case PostIncPtr: // adr -> (*adr)++
			p := readPtr(c.sp)
			v := readPtr(p)
			writePtr(c.sp, v)
			writePtr(p, v+uintptr(op.N))
		case PreIncI8: // adr -> ++(*adr)
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i8StackSz
			v := readI8(p) + int8(op.N)
			writeI8(c.sp, v)
			writeI8(p, v)
		case PreIncI16: // adr -> ++(*adr)
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i16StackSz
			v := readI16(p) + int16(op.N)
			writeI16(c.sp, v)
			writeI16(p, v)
		case PreIncI32: // adr -> ++(*adr)
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i32StackSz
			v := readI32(p) + int32(op.N)
			writeI32(c.sp, v)
			writeI32(p, v)
		case PreIncI64: // adr -> ++(*adr)
			p := readPtr(c.sp)
			c.sp -= i64StackSz - ptrStackSz
			v := readI64(p) + int64(op.N)
			writeI64(c.sp, v)
			writeI64(p, v)
		case PreIncU32Bits: // adr -> ++(*adr)
			d := uint64(op.N)
			op = c.code[c.ip]
			c.ip++
			bits := uint(op.N >> 16)
			bitoff := uint(op.N) >> 8 & 0xff
			w := op.N & 0xff
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i32StackSz
			m0 := uint64(1)<<bits - 1
			m := m0 << bitoff
			var u uint64
			switch w {
			case 1:
				u = uint64(readU8(p))
			case 2:
				u = uint64(readU16(p))
			case 4:
				u = uint64(readU32(p))
			case 8:
				u = readU64(p)
			default:
				return -1, fmt.Errorf("internal error: %v\n%s", op, c.stackTrace())
			}
			v := u&m>>bitoff + d&m0
			writeU32(c.sp, uint32(v))
			u = u&^m | v<<bitoff&m
			switch w {
			case 1:
				writeU8(p, uint8(u))
			case 2:
				writeU16(p, uint16(u))
			case 4:
				writeU32(p, uint32(u))
			case 8:
				writeU64(p, u)
			default:
				return -1, fmt.Errorf("internal error: %v\n%s", op, c.stackTrace())
			}
		case PreIncU64Bits: // adr -> ++(*adr)
			d := uint64(op.N)
			op = c.code[c.ip]
			c.ip++
			bits := uint(op.N >> 16)
			bitoff := uint(op.N) >> 8 & 0xff
			w := op.N & 0xff
			p := readPtr(c.sp)
			c.sp -= i64StackSz - ptrStackSz
			m0 := uint64(1)<<bits - 1
			m := m0 << bitoff
			var u uint64
			switch w {
			case 1:
				u = uint64(readU8(p))
			case 2:
				u = uint64(readU16(p))
			case 4:
				u = uint64(readU32(p))
			case 8:
				u = readU64(p)
			default:
				return -1, fmt.Errorf("internal error: %v\n%s", op, c.stackTrace())
			}
			v := (u&m>>bitoff + d) & m0
			writeU64(c.sp, v)
			u = u&^m | v<<bitoff&m
			switch w {
			case 1:
				writeU8(p, uint8(u))
			case 2:
				writeU16(p, uint16(u))
			case 4:
				writeU32(p, uint32(u))
			case 8:
				writeU64(p, u)
			default:
				return -1, fmt.Errorf("internal error: %v\n%s", op, c.stackTrace())
			}
		case PreIncPtr: // adr -> ++(*adr)
			p := readPtr(c.sp)
			v := readPtr(p) + uintptr(op.N)
			writePtr(c.sp, v)
			writePtr(p, v)
		case Push8: // -> val
			c.sp -= i8StackSz
			writeI8(c.sp, int8(op.N))
		case Push16: // -> val
			c.sp -= i16StackSz
			writeI16(c.sp, int16(op.N))
		case Push32:
			c.sp -= i32StackSz
			writeI32(c.sp, int32(op.N))
		case Push64:
			c.push64(op.N, c.code[c.ip].N)
		case PushC128:
			c.pushC128(op.N, c.code[c.ip].N)
		case PtrDiff: // p q -> p - q
			q := readPtr(c.sp)
			c.sp += ptrStackSz
			writePtr(c.sp, (readPtr(c.sp)-q)/uintptr(op.N))
		case RemI32: // a, b -> a % b
			b := readI32(c.sp)
			c.sp += i32StackSz
			writeI32(c.sp, readI32(c.sp)%b)
		case RemU32: // a, b -> a % b
			b := readU32(c.sp)
			c.sp += i32StackSz
			writeU32(c.sp, readU32(c.sp)%b)
		case RemI64: // a, b -> a % b
			b := readI64(c.sp)
			c.sp += i64StackSz
			writeI64(c.sp, readI64(c.sp)%b)
		case RemU64: // a, b -> a % b
			b := readU64(c.sp)
			c.sp += i64StackSz
			writeU64(c.sp, readU64(c.sp)%b)
		case Return:
			c.sp = c.bp
			c.bp = readPtr(c.sp)
			c.sp += ptrStackSz
			ap := readPtr(c.sp)
			c.sp += ptrStackSz
			c.ip = readPtr(c.sp)
			c.sp += ptrStackSz
			n := len(c.rpStack)
			c.rp = c.rpStack[n-1]
			c.rpStack = c.rpStack[:n-1]
			c.sp = c.ap
			c.ap = ap
		case RshI8: // val, cnt -> val >> cnt
			n := readI32(c.sp)
			c.sp += i32StackSz
			writeI8(c.sp, readI8(c.sp)>>uint(n))
		case RshU8: // val, cnt -> val >> cnt
			n := readI32(c.sp)
			c.sp += i32StackSz
			writeU8(c.sp, readU8(c.sp)>>uint(n))
		case RshI16: // val, cnt -> val >> cnt
			n := readI32(c.sp)
			c.sp += i32StackSz
			writeI16(c.sp, readI16(c.sp)>>uint(n))
		case RshU16: // val, cnt -> val >> cnt
			n := readI32(c.sp)
			c.sp += i32StackSz
			writeU16(c.sp, readU16(c.sp)>>uint(n))
		case RshI32: // val, cnt -> val >> cnt
			n := readI32(c.sp)
			c.sp += i32StackSz
			writeI32(c.sp, readI32(c.sp)>>uint(n))
		case RshU32: // val, cnt -> val >> cnt
			n := readU32(c.sp)
			c.sp += i32StackSz
			writeU32(c.sp, readU32(c.sp)>>uint(n))
		case RshI64: // val, cnt -> val >> cnt
			n := readI32(c.sp)
			c.sp += i32StackSz
			writeI64(c.sp, readI64(c.sp)>>uint(n))
		case RshU64: // val, cnt -> val >> cnt
			n := readI32(c.sp)
			c.sp += i32StackSz
			writeU64(c.sp, readU64(c.sp)>>uint(n))
		case Store8: // adr, val -> val
			v := readI8(c.sp)
			c.sp += i8StackSz
			writeI8(readPtr(c.sp), v)
			c.sp += ptrStackSz - i8StackSz
			writeI8(c.sp, v)
		case Store: // adr, val -> val
			sz := op.N
			adr := readPtr(c.sp + uintptr(roundup(sz, stackAlign)))
			movemem(adr, c.sp, sz)
			movemem(c.sp+ptrStackSz, c.sp, sz)
			c.sp += ptrStackSz
		case Store16: // adr, val -> val
			v := readI16(c.sp)
			c.sp += i16StackSz
			writeI16(readPtr(c.sp), v)
			c.sp += ptrStackSz - i16StackSz
			writeI16(c.sp, v)
		case Store32: // adr, val -> val
			v := readI32(c.sp)
			c.sp += i32StackSz
			writeI32(readPtr(c.sp), v)
			c.sp += ptrStackSz - i32StackSz
			writeI32(c.sp, v)
		case Store64: // adr, val -> val
			v := readI64(c.sp)
			c.sp += i64StackSz
			writeI64(readPtr(c.sp), v)
			c.sp -= i64StackSz - ptrStackSz
			writeI64(c.sp, v)
		case StoreC128: // adr, val -> val
			v := readC128(c.sp)
			c.sp += c128StackSz
			writeC128(readPtr(c.sp), v)
			c.sp -= c128StackSz - ptrStackSz
			writeC128(c.sp, v)
		case StoreBits8: // adr, val -> val
			v := readI8(c.sp)
			c.sp += i8StackSz
			p := readPtr(c.sp)
			v = readI8(p)&^int8(op.N) | v&int8(op.N)
			writeI8(p, v)
			c.sp += ptrStackSz - i8StackSz
			writeI8(c.sp, v)
		case StoreBits16: // adr, val -> val
			v := readI16(c.sp)
			c.sp += i16StackSz
			p := readPtr(c.sp)
			v = readI16(p)&^int16(op.N) | v&int16(op.N)
			writeI16(p, v)
			c.sp += ptrStackSz - i16StackSz
			writeI16(c.sp, v)
		case StoreBits32: // adr, val -> val
			v := readI32(c.sp)
			c.sp += i32StackSz
			p := readPtr(c.sp)
			v = readI32(p)&^int32(op.N) | v&int32(op.N)
			writeI32(p, v)
			c.sp += ptrStackSz - i32StackSz
			writeI32(c.sp, v)
		case StoreBits64: // adr, val -> val
			v := readI64(c.sp)
			c.sp += i64StackSz
			p := readPtr(c.sp)
			v = readI64(p)&^int64(op.N) | v&int64(op.N)
			writeI64(p, v)
			c.sp -= i64StackSz - ptrStackSz
			writeI64(c.sp, v)
		case StrNCopy: // &dst, &src ->
			src := readPtr(c.sp)
			c.sp += ptrStackSz
			dest := readPtr(c.sp)
			c.sp += ptrStackSz
			n := op.N
			var ch int8
			for ch = readI8(src); ch != 0 && n > 0; n-- {
				writeI8(dest, ch)
				dest++
				src++
				ch = readI8(src)
			}
			for ; n > 0; n-- {
				writeI8(dest, 0)
				dest++
			}
		case SubF32: // a, b -> a - b
			b := readF32(c.sp)
			c.sp += f32StackSz
			writeF32(c.sp, readF32(c.sp)-b)
		case SubF64: // a, b -> a - b
			b := readF64(c.sp)
			c.sp += f64StackSz
			writeF64(c.sp, readF64(c.sp)-b)
		case SubI32: // a, b -> a - b
			b := readI32(c.sp)
			c.sp += i32StackSz
			writeI32(c.sp, readI32(c.sp)-b)
		case SubI64: // a, b -> a - b
			b := readI64(c.sp)
			c.sp += i64StackSz
			writeI64(c.sp, readI64(c.sp)-b)
		case SubPtrs:
			v := readPtr(c.sp)
			c.sp += ptrStackSz
			addPtr(c.sp, -v)
		case SwitchI32:
			v := readI32(c.sp)
			c.sp += i32StackSz
			tab := c.ds + uintptr(op.N)
			cases := int(readI32(tab))
			values := tab + i64Size
			labels := roundupP(values+uintptr(cases)*i32Size, ptrSize)
			l := 0
			h := cases - 1
			for l <= h {
				m := (l + h) >> 1
				k := readI32(values + uintptr(m)*i32Size)
				switch {
				case v > k:
					l = m + 1
				case v == k:
					c.ip = readPtr(labels + uintptr(m)*ptrSize)
					break main
				default:
					h = m - 1
				}
			}
			c.ip = readPtr(labels + uintptr(cases)*ptrSize)
		case SwitchI64:
			v := readI64(c.sp)
			c.sp += i64StackSz
			tab := c.ds + uintptr(op.N)
			cases := int(readI32(tab))
			values := tab + i64Size
			labels := values + uintptr(cases)*i64Size
			l := 0
			h := cases - 1
			for l <= h {
				m := (l + h) >> 1
				k := readI64(values + uintptr(m)*i64Size)
				switch {
				case v > k:
					l = m + 1
				case v == k:
					c.ip = readPtr(labels + uintptr(m)*ptrSize)
					break main
				default:
					h = m - 1
				}
			}
			c.ip = readPtr(labels + uintptr(cases)*ptrSize)
		case Text:
			c.sp -= ptrStackSz
			writePtr(c.sp, c.ts+uintptr(op.N))
		case Variable: // -> val
			off := op.N
			op = c.code[c.ip]
			c.ip++
			sz := op.N
			c.sp -= uintptr(roundup(sz, stackAlign))
			movemem(c.sp, c.bp+uintptr(off), sz)
		case Variable8: // -> val
			c.sp -= i8StackSz
			writeI8(c.sp, readI8(c.bp+uintptr(op.N)))
		case Variable16: // -> val
			c.sp -= i16StackSz
			writeI16(c.sp, readI16(c.bp+uintptr(op.N)))
		case Variable32: // -> val
			c.sp -= i32StackSz
			writeI32(c.sp, readI32(c.bp+uintptr(op.N)))
		case Variable64: // -> val
			c.sp -= i64StackSz
			writeI64(c.sp, readI64(c.bp+uintptr(op.N)))
		case Xor32: // a, b -> a ^ b
			b := readI32(c.sp)
			c.sp += i32StackSz
			writeI32(c.sp, readI32(c.sp)^b)
		case Xor64: // a, b -> a ^ b
			b := readI64(c.sp)
			c.sp += i64StackSz
			writeI64(c.sp, readI64(c.sp)^b)
		case Zero32:
			c.sp -= i32StackSz
			writeI32(c.sp, 0)
		case Zero64:
			c.sp -= i64StackSz
			writeI64(c.sp, 0)

		case abort:
			if !Testing {
				return 1, nil
			}

			return 1, c.stackTrace()
		case exit:
			return int(readI32(c.sp)), nil
		case builtin:
			var ip uintptr
			c.sp, ip = popPtr(c.sp)
			es, err := c.run(c.ip)
			if err != nil {
				return es, err
			}

			c.ip = ip

		case sinh:
			c.builtin(c.sinh)
		case cosh:
			c.builtin(c.cosh)
		case tanh:
			c.builtin(c.tanh)
		case sin:
			c.builtin(c.sin)
		case cos:
			c.builtin(c.cos)
		case tan:
			c.builtin(c.tan)
		case asin:
			c.builtin(c.asin)
		case acos:
			c.builtin(c.acos)
		case atan:
			c.builtin(c.atan)
		case atoi:
			c.builtin(c.atoi)
		case exp:
			c.builtin(c.exp)
		case fabs:
			c.builtin(c.fabs)
		case log:
			c.builtin(c.log)
		case log10:
			c.builtin(c.log10)
		case pow:
			c.builtin(c.pow)
		case sqrt:
			c.builtin(c.sqrt)
		case round:
			c.builtin(c.round)
		case ceil:
			c.builtin(c.ceil)
		case floor:
			c.builtin(c.floor)
		case strcpy:
			c.builtin(c.strcpy)
		case strncpy:
			c.builtin(c.strncpy)
		case strcmp:
			c.builtin(c.strcmp)
		case strlen:
			c.builtin(c.strlen)
		case strcat:
			c.builtin(c.strcat)
		case strncmp:
			c.builtin(c.strncmp)
		case strchr:
			c.builtin(c.strchr)
		case strrchr:
			c.builtin(c.strrchr)
		case memset:
			c.builtin(c.memset)
		case memcpy:
			c.builtin(c.memcpy)
		case memcmp:
			c.builtin(c.memcmp)
		case tolower:
			c.builtin(c.tolower)
		case malloc:
			c.builtin(c.malloc)
		case calloc:
			c.builtin(c.calloc)
		case abs:
			c.builtin(c.abs)
		case isprint:
			c.builtin(c.isprint)
		case ffs:
			c.builtin(c.ffs)
		case ffsl:
			c.builtin(c.ffsl)
		case ffsll:
			c.builtin(c.ffsll)
		case clz:
			c.builtin(c.clz)
		case clzl:
			c.builtin(c.clzl)
		case clzll:
			c.builtin(c.clzll)
		case ctz:
			c.builtin(c.ctz)
		case ctzl:
			c.builtin(c.ctzl)
		case ctzll:
			c.builtin(c.ctzll)
		case clrsb:
			c.builtin(c.clrsb)
		case clrsbl:
			c.builtin(c.clrsbl)
		case clrsbll:
			c.builtin(c.clrsbll)
		case popcount:
			c.builtin(c.popcount)
		case popcountl:
			c.builtin(c.popcountl)
		case popcountll:
			c.builtin(c.popcountll)
		case parity:
			c.builtin(c.parity)
		case parityl:
			c.builtin(c.parityl)
		case parityll:
			c.builtin(c.parityll)
		case isinf:
			c.builtin(c.isinf)
		case isinff:
			c.builtin(c.isinff)
		case isinfl:
			c.builtin(c.isinf)
		case returnAddress:
			c.builtin(c.returnAddress)
		case alloca:
			c.builtin(c.alloca)
		case __signbit:
			c.builtin(c.signbit)
		case __signbitf:
			c.builtin(c.signbitf)
		case bswap64:
			c.builtin(c.bswap64)
		case frameAddress:
			c.builtin(c.frameAddress)
		case copysign:
			c.builtin(c.copysign)
		case cimagf:
			c.builtin(c.cimagf)
		case crealf:
			c.builtin(c.crealf)
		case mempcpy:
			c.builtin(c.mempcpy)
		case memmove:
			c.builtin(c.memmove)
		case qsort:
			c.builtin(c.qsort)
		case setjmp:
			c.builtin(c.setjmp)
		case longjmp:
			c.builtin(c.longjmp)
		case pthread_mutex_lock:
			c.builtin(c.pthreadMutexLock)
		case pthread_mutex_unlock:
			c.builtin(c.pthreadMutexUnlock)
		case pthread_mutexattr_init:
			c.builtin(c.pthreadMutexAttrInit)
		case pthread_mutexattr_settype:
			c.builtin(c.pthreadMutexAttrSetType)
		case pthread_mutex_init:
			c.builtin(c.pthreadMutexInit)
		case pthread_mutexattr_destroy:
			c.builtin(c.pthreadMutexAttrDestroy)
		case pthread_mutex_destroy:
			c.builtin(c.pthreadMutexDestroy)
		case errno_location:
			c.builtin(c.errnoLocation)
		case realloc:
			c.builtin(c.realloc)
		case register_stdfiles:
			c.builtin(c.register_stdfiles)
		case printf:
			c.builtin(c.printf)
		case puts:
			c.builtin(c.puts)
		case rewind:
			c.builtin(c.rewind)
		case sprintf:
			c.builtin(c.sprintf)
		case fopen64:
			c.builtin(c.fopen64)
		case fwrite:
			c.builtin(c.fwrite)
		case fclose:
			c.builtin(c.fclose)
		case ferror:
			c.builtin(c.ferror)
		case fread:
			c.builtin(c.fread)
		case fseek:
			c.builtin(c.fseek)
		case ftell:
			c.builtin(c.ftell)
		case fgetc:
			c.builtin(c.fgetc)
		case fgets:
			c.builtin(c.fgets)
		case fprintf:
			c.builtin(c.fprintf)
		case vfprintf:
			c.builtin(c.vfprintf)
		case vprintf:
			c.builtin(c.vprintf)
		case free:
			c.builtin(c.free)
		case lstat64:
			c.builtin(c.lstat64)
		case stat64:
			c.builtin(c.stat64)
		case getcwd:
			c.builtin(c.getcwd)
		case getpid:
			c.builtin(c.getpid)
		case open64:
			c.builtin(c.open64)
		case fcntl:
			c.builtin(c.fcntl)
		case fstat64:
			c.builtin(c.fstat64)
		case lseek64:
			c.builtin(c.lseek64)
		case read:
			c.builtin(c.read)
		case close_:
			c.builtin(c.close)
		case unlink:
			c.builtin(c.unlink)
		case geteuid:
			c.builtin(c.geteuid)
		case write:
			c.builtin(c.write)
		case fsync:
			c.builtin(c.fsync)
		case pthread_self:
			c.builtin(c.pthreadSelf)
		case pthread_equal:
			c.builtin(c.pthreadEqual)
		case pthread_mutex_trylock:
			c.builtin(c.pthreadMutexTryLock)
		case gettimeofday:
			c.builtin(c.gettimeofday)
		case ftruncate64:
			c.builtin(c.ftruncate64)
		case getenv:
			c.builtin(c.getenv)
		case access:
			c.builtin(c.access)
		case mmap64:
			c.builtin(c.mmap64)
		case sysconf:
			c.builtin(c.sysconf)
		case munmap:
			c.builtin(c.munmap)
		case usleep:
			c.builtin(c.usleep)
		case select_:
			c.builtin(c.select_)

		// windows
		case AreFileApisANSI:
			c.builtin(c.AreFileApisANSI)
		case CreateFileA:
			c.builtin(c.CreateFileA)
		case CreateFileW:
			c.builtin(c.CreateFileW)
		case CreateFileMappingA:
			c.builtin(c.CreateFileMappingA)
		case CreateFileMappingW:
			c.builtin(c.CreateFileMappingW)
		case CreateMutexW:
			c.builtin(c.CreateMutexW)
		case CloseHandle:
			c.builtin(c.CloseHandle)
		case DeleteCriticalSection:
			c.builtin(c.DeleteCriticalSection)
		case DeleteFileA:
			c.builtin(c.DeleteFileA)
		case DeleteFileW:
			c.builtin(c.DeleteFileW)
		case EnterCriticalSection:
			c.builtin(c.EnterCriticalSection)
		case FlushFileBuffers:
			c.builtin(c.FlushFileBuffers)
		case FlushViewOfFile:
			c.builtin(c.FlushViewOfFile)
		case FormatMessageA:
			c.builtin(c.FormatMessageA)
		case FormatMessageW:
			c.builtin(c.FormatMessageW)
		case FreeLibrary:
			c.builtin(c.FreeLibrary)
		case GetCurrentProcessId:
			c.builtin(c.GetCurrentProcessId)
		case GetCurrentThreadId:
			c.builtin(c.GetCurrentThreadId)
		case GetDiskFreeSpaceA:
			c.builtin(c.GetDiskFreeSpaceA)
		case GetDiskFreeSpaceW:
			c.builtin(c.GetDiskFreeSpaceW)
		case GetFileAttributesExW:
			c.builtin(c.GetFileAttributesExW)
		case GetFileAttributesA:
			c.builtin(c.GetFileAttributesA)
		case GetFileAttributesW:
			c.builtin(c.GetFileAttributesW)
		case GetFileSize:
			c.builtin(c.GetFileSize)
		case GetFullPathNameA:
			c.builtin(c.GetFullPathNameA)
		case GetFullPathNameW:
			c.builtin(c.GetFullPathNameW)
		case GetLastError:
			c.builtin(c.GetLastError)
		case GetProcAddress:
			c.builtin(c.GetProcAddress)
		case GetProcessHeap:
			c.builtin(c.GetProcessHeap)
		case GetSystemInfo:
			c.builtin(c.GetSystemInfo)
		case GetSystemTime:
			c.builtin(c.GetSystemTime)
		case GetSystemTimeAsFileTime:
			c.builtin(c.GetSystemTimeAsFileTime)
		case GetTempPathA:
			c.builtin(c.GetTempPathA)
		case GetTempPathW:
			c.builtin(c.GetTempPathW)
		case GetTickCount:
			c.builtin(c.GetTickCount)
		case GetVersionExA:
			c.builtin(c.GetVersionExA)
		case GetVersionExW:
			c.builtin(c.GetVersionExW)
		case HeapAlloc:
			c.builtin(c.HeapAlloc)
		case HeapCompact:
			c.builtin(c.HeapCompact)
		case HeapCreate:
			c.builtin(c.HeapCreate)
		case HeapDestroy:
			c.builtin(c.HeapDestroy)
		case HeapFree:
			c.builtin(c.HeapFree)
		case HeapReAlloc:
			c.builtin(c.HeapReAlloc)
		case HeapSize:
			c.builtin(c.HeapSize)
		case HeapValidate:
			c.builtin(c.HeapValidate)
		case InitializeCriticalSection:
			c.builtin(c.InitializeCriticalSection)
		case InterlockedCompareExchange:
			c.builtin(c.InterlockedCompareExchange)
		case LeaveCriticalSection:
			c.builtin(c.LeaveCriticalSection)
		case LoadLibraryA:
			c.builtin(c.LoadLibraryA)
		case LoadLibraryW:
			c.builtin(c.LoadLibraryW)
		case LocalFree:
			c.builtin(c.LocalFree)
		case LockFile:
			c.builtin(c.LockFile)
		case LockFileEx:
			c.builtin(c.LockFileEx)
		case MapViewOfFile:
			c.builtin(c.MapViewOfFile)
		case MultiByteToWideChar:
			c.builtin(c.MultiByteToWideChar)
		case OutputDebugStringA:
			c.builtin(c.OutputDebugStringA)
		case OutputDebugStringW:
			c.builtin(c.OutputDebugStringW)
		case QueryPerformanceCounter:
			c.builtin(c.QueryPerformanceCounter)
		case ReadFile:
			c.builtin(c.ReadFile)
		case SetEndOfFile:
			c.builtin(c.SetEndOfFile)
		case SetFilePointer:
			c.builtin(c.SetFilePointer)
		case Sleep:
			c.builtin(c.Sleep)
		case SystemTimeToFileTime:
			c.builtin(c.SystemTimeToFileTime)
		case UnlockFile:
			c.builtin(c.UnlockFile)
		case UnlockFileEx:
			c.builtin(c.UnlockFileEx)
		case UnmapViewOfFile:
			c.builtin(c.UnmapViewOfFile)
		case WaitForSingleObject:
			c.builtin(c.WaitForSingleObject)
		case WaitForSingleObjectEx:
			c.builtin(c.WaitForSingleObjectEx)
		case WideCharToMultiByte:
			c.builtin(c.WideCharToMultiByte)
		case WriteFile:
			c.builtin(c.WriteFile)

		default:
			return -1, fmt.Errorf("instruction trap: %v\n%s", op, c.stackTrace())
		}
	}
}
