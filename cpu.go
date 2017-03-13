// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"errors"
	"math"

	"github.com/cznic/internal/buffer"
)

import (
	"fmt"
	"unsafe"
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

type cpu struct {
	ap      uintptr // Arguments pointer
	bp      uintptr // Base pointer
	code    []Operation
	ds      uintptr // Data segment
	fpStack []uintptr
	ip      uintptr // Instruction pointer
	m       *machine
	rp      uintptr // Results pointer
	rpStack []uintptr
	sp      uintptr // Stack pointer
	stop    chan struct{}
	thread  *thread
	ts      uintptr // Text segment
}

func addPtr(p uintptr, v uintptr)       { *(*uintptr)(unsafe.Pointer(p)) += v }
func readC128(p uintptr) complex128     { return *(*complex128)(unsafe.Pointer(p)) }
func readC64(p uintptr) complex64       { return *(*complex64)(unsafe.Pointer(p)) }
func readF32(p uintptr) float32         { return *(*float32)(unsafe.Pointer(p)) }
func readF64(p uintptr) float64         { return *(*float64)(unsafe.Pointer(p)) }
func readI16(p uintptr) int16           { return *(*int16)(unsafe.Pointer(p)) }
func readI32(p uintptr) int32           { return *(*int32)(unsafe.Pointer(p)) }
func readI64(p uintptr) int64           { return *(*int64)(unsafe.Pointer(p)) }
func readI8(p uintptr) int8             { return *(*int8)(unsafe.Pointer(p)) }
func readPtr(p uintptr) uintptr         { return *(*uintptr)(unsafe.Pointer(p)) }
func readU16(p uintptr) uint16          { return *(*uint16)(unsafe.Pointer(p)) }
func readU32(p uintptr) uint32          { return *(*uint32)(unsafe.Pointer(p)) }
func readU64(p uintptr) uint64          { return *(*uint64)(unsafe.Pointer(p)) }
func readU8(p uintptr) uint8            { return *(*uint8)(unsafe.Pointer(p)) }
func writeC128(p uintptr, v complex128) { *(*complex128)(unsafe.Pointer(p)) = v }
func writeC64(p uintptr, v complex64)   { *(*complex64)(unsafe.Pointer(p)) = v }
func writeF32(p uintptr, v float32)     { *(*float32)(unsafe.Pointer(p)) = v }
func writeF64(p uintptr, v float64)     { *(*float64)(unsafe.Pointer(p)) = v }
func writeI16(p uintptr, v int16)       { *(*int16)(unsafe.Pointer(p)) = v }
func writeI32(p uintptr, v int32)       { *(*int32)(unsafe.Pointer(p)) = v }
func writeI64(p uintptr, v int64)       { *(*int64)(unsafe.Pointer(p)) = v }
func writeI8(p uintptr, v int8)         { *(*int8)(unsafe.Pointer(p)) = v }
func writePtr(p uintptr, v uintptr)     { *(*uintptr)(unsafe.Pointer(p)) = v }
func writeU16(p uintptr, v uint16)      { *(*uint16)(unsafe.Pointer(p)) = v }
func writeU32(p uintptr, v uint32)      { *(*uint32)(unsafe.Pointer(p)) = v }
func writeU64(p uintptr, v uint64)      { *(*uint64)(unsafe.Pointer(p)) = v }
func writeU8(p uintptr, v uint8)        { *(*uint8)(unsafe.Pointer(p)) = v }

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

func (c *cpu) stackTrace() error {
	var buf buffer.Bytes
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
			dumpCode(&buf, c.code[ip:ip+1], int(ip))
		default:
			dumpCode(&buf, c.code[ip:ip+1], int(ip))
		}
		sp = bp
		bp = readPtr(sp)
		sp += ptrStackSz
		ap = readPtr(sp)
		sp += ptrStackSz
		if i := sp - c.thread.ss; int(i) >= len(c.thread.stackMem) {
			break
		}

		ip = readPtr(sp) - 1
	}
	return errors.New(string(buf.Bytes()))
}

func (c *cpu) trace() string {
	s := dumpCodeStr(c.code[c.ip:c.ip+1], int(c.ip))
	a := make([]uintptr, 5)
	for i := range a {
		a[i] = readPtr(c.sp + uintptr(i*ptrStackSz))
	}
	return fmt.Sprintf("%s\t%#x: %x; %v", s[:len(s)-1], c.sp, a, c.m.pcInfo(int(c.ip), c.m.lines).Position())
}

func (c *cpu) run(code []Operation) (int, error) {
	//fmt.Printf("%#v\n", c)
	defer func() {
		if err := recover(); err != nil {
			panic(fmt.Errorf("%v\n%s", err, c.stackTrace()))
		}
	}()

	c.code = code
	for i := 0; ; i++ {
		if i&1024 == 0 {
			select {
			case <-c.m.stop:
				return -1, KillError{}
			default:
			}
		}

		//fmt.Println(c.trace()) //TODO-
		op := code[c.ip] //TODO bench op := *(*Operation)(unsafe.Address(&code[c.ip]))
		c.ip++
		switch op.Opcode {
		case AP: // -> ptr
			c.sp -= i32StackSz
			writePtr(c.sp, c.ap+uintptr(op.N))
		case AddF32: // a, b -> a + b
			b := readF32(c.sp)
			c.sp += f32StackSz
			writeF32(c.sp, readF32(c.sp)+b)
		case AddF64: // a, b -> a + b
			b := readF64(c.sp)
			c.sp += f64StackSz
			writeF64(c.sp, readF64(c.sp)+b)
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
			op = code[c.ip]
			c.ip++
			sz := op.N
			c.sp -= uintptr(roundup(sz, stackAlign))
			memcopy(c.sp, c.ap+uintptr(off), sz)
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
		case ConvF32F64:
			v := readF32(c.sp)
			c.sp += f32StackSz - f64StackSz
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
		case ConvF64I32:
			v := readF64(c.sp)
			c.sp += f64StackSz - i32StackSz
			writeI32(c.sp, int32(v))
		case ConvF64U32:
			v := readF64(c.sp)
			c.sp += f64StackSz - i32StackSz
			writeU32(c.sp, uint32(v))
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
			c.sp += i32StackSz - f64StackSz
			writeF64(c.sp, float64(v))
		case ConvI32I64:
			v := readI32(c.sp)
			c.sp += i32StackSz - i64StackSz
			writeI64(c.sp, int64(v))
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
			c.sp += i16StackSz - i64StackSz
			writeI64(c.sp, int64(v))
		case ConvI32C64:
			v := readI32(c.sp)
			c.sp += i32StackSz - c64StackSz
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
			c.sp += i8StackSz - i64StackSz
			writeI64(c.sp, int64(v))
		case ConvU8I16:
			writeI16(c.sp, int16(readU8(c.sp)))
		case ConvU8I32:
			writeI32(c.sp, int32(readU8(c.sp)))
		case ConvU8U32:
			writeU32(c.sp, uint32(readU8(c.sp)))
		case ConvU8U64:
			v := readI8(c.sp)
			c.sp += i8StackSz - i64StackSz
			writeU64(c.sp, uint64(v))
		case ConvU16I32:
			writeI32(c.sp, int32(readU16(c.sp)))
		case ConvU16U32:
			writeU32(c.sp, uint32(readU16(c.sp)))
		case ConvU16I64:
			v := readU16(c.sp)
			c.sp += i16StackSz - i64StackSz
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
			c.sp += i32StackSz - i64StackSz
			writeI64(c.sp, int64(v))
		case ConvU32F32:
			v := readU32(c.sp)
			c.sp += i32StackSz - f32StackSz
			writeF32(c.sp, float32(v))
		case ConvU32F64:
			v := readU32(c.sp)
			c.sp += i32StackSz - f64StackSz
			writeF64(c.sp, float64(v))
		case Copy: // &dst, &src -> &dst
			src := readPtr(c.sp)
			c.sp += ptrStackSz
			memcopy(readPtr(c.sp), src, op.N)
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
			op = code[c.ip]
			c.ip++
			sz := op.N
			c.sp -= uintptr(roundup(sz, stackAlign))
			memcopy(c.sp, c.ds+uintptr(off), sz)
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
		case FP:
			c.sp -= ptrStackSz
			if ptrStackSz == 4 {
				writeI32(c.sp, int32(op.N))
			} else {
				writeI64(c.sp, int64(op.N))
			}
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
			c.sp += uintptr(op.N)

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
		case IndexU8: // addr, index -> addr + n*index
			x := readU8(c.sp)
			c.sp += i8StackSz
			addPtr(c.sp, uintptr(op.N*int(x)))
		case IndexI16: // addr, index -> addr + n*index
			x := readI16(c.sp)
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
			op = code[c.ip]
			c.ip++
			sz := op.N
			c.sp += ptrStackSz - uintptr(roundup(sz, stackAlign))
			memcopy(c.sp, p+uintptr(off), sz)
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
			c.sp += ptrStackSz - i64StackSz
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
		case NegIndexI32: // addr, index -> addr - n*index
			x := readI32(c.sp)
			c.sp += i32StackSz
			addPtr(c.sp, uintptr(-op.N*int(x)))
		case NegIndexI64: // addr, index -> addr - n*index
			x := readI64(c.sp)
			c.sp += i64StackSz
			addPtr(c.sp, uintptr(int64(-op.N)*x))
		case NegIndexU64: // addr, index -> addr - n*index
			x := readU64(c.sp)
			c.sp += i64StackSz
			addPtr(c.sp, uintptr(uint64(-op.N)*x))
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
			d := uint32(op.N)
			op = code[c.ip]
			c.ip++
			bits := uint(op.N >> 16)
			bitoff := uint(op.N) >> 8 & 0xff
			w := op.N & 0xff
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i32StackSz
			m := (uint64(1)<<bits - 1) << bitoff
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
			v := uint32(u & m >> bitoff)
			writeU32(c.sp, v)
			v += d
			u = u&^m | uint64(v)<<bitoff&m
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
			op = code[c.ip]
			c.ip++
			bits := uint(op.N >> 16)
			bitoff := uint(op.N) >> 8 & 0xff
			w := op.N & 0xff
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i32StackSz
			m := (uint64(1)<<bits - 1) << bitoff
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
			c.sp += ptrStackSz - i64StackSz
			v := readI64(p)
			writeI64(c.sp, v)
			writeI64(p, v+int64(op.N))
		case PostIncF64: // adr -> (*adr)++
			p := readPtr(c.sp)
			c.sp += ptrStackSz - f64StackSz
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
			c.sp += ptrStackSz - i64StackSz
			v := readI64(p) + int64(op.N)
			writeI64(c.sp, v)
			writeI64(p, v)
		case PreIncU32Bits: // adr -> (*adr)++
			d := uint32(op.N)
			op = code[c.ip]
			c.ip++
			bits := uint(op.N >> 16)
			bitoff := uint(op.N) >> 8 & 0xff
			w := op.N & 0xff
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i32StackSz
			m := (uint64(1)<<bits - 1) << bitoff
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
			v := uint32(u&m>>bitoff) + d
			writeU32(c.sp, v)
			u = u&^m | uint64(v)<<bitoff&m
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
		case PreIncU64Bits: // adr -> (*adr)++
			d := uint64(op.N)
			op = code[c.ip]
			c.ip++
			bits := uint(op.N >> 16)
			bitoff := uint(op.N) >> 8 & 0xff
			w := op.N & 0xff
			p := readPtr(c.sp)
			c.sp += ptrStackSz - i64StackSz
			m := (uint64(1)<<bits - 1) << bitoff
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
			v := u&m>>bitoff + d
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
			c.push64(op.N, code[c.ip].N)
		case PtrDiff: // p q -> p - q
			q := readPtr(c.sp)
			c.sp += ptrStackSz
			writeU64(c.sp, uint64(readPtr(c.sp)-q)/uint64(op.N))
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
			memcopy(adr, c.sp, sz)
			memcopy(c.sp+ptrStackSz, c.sp, sz)
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
			c.sp += ptrStackSz - i64StackSz
			writeI64(c.sp, v)
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
			c.sp += ptrStackSz - i64StackSz
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
		case Text:
			c.sp -= ptrStackSz
			writePtr(c.sp, c.ts+uintptr(op.N))
		case Variable: // -> val
			off := op.N
			op = code[c.ip]
			c.ip++
			sz := op.N
			c.sp -= uintptr(roundup(sz, stackAlign))
			memcopy(c.sp, c.bp+uintptr(off), sz)
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
			return 1, nil
		case exit:
			return int(readI32(c.sp)), nil
		case printf:
			c.builtin(c.printf)
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
		case sprintf:
			c.builtin(c.sprintf)
		case fopen:
			c.builtin(c.fopen)
		case fwrite:
			c.builtin(c.fwrite)
		case fclose:
			c.builtin(c.fclose)
		case fread:
			c.builtin(c.fread)
		case fgetc:
			c.builtin(c.fgetc)
		case fgets:
			c.builtin(c.fgets)
		case fprintf:
			c.builtin(c.fprintf)
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
		case vfprintf:
			c.builtin(c.vfprintf)
		case vprintf:
			c.builtin(c.vprintf)
		case free:
			c.builtin(c.free)
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

		default:
			return -1, fmt.Errorf("instruction trap: %v\n%s", op, c.stackTrace())
		}
	}
}
