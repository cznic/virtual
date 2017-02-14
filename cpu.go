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

// Operation is the machine code.
type Operation struct {
	Opcode
	N int
}

type cpu struct {
	ap      uintptr // Arguments pointer
	bp      uintptr // Base pointer
	ds      uintptr // Data segment
	ip      uintptr // Instruction pointer
	m       *machine
	rp      uintptr // Results pointer
	rpStack []uintptr
	sp      uintptr // Stack pointer
	stop    chan struct{}
	thread  *thread
	ts      uintptr // Text segment
}

func (c *cpu) addPtr(p uintptr, v uintptr)       { *(*uintptr)(unsafe.Pointer(p)) += v }
func (c *cpu) readC128(p uintptr) complex128     { return *(*complex128)(unsafe.Pointer(p)) }
func (c *cpu) readC64(p uintptr) complex64       { return *(*complex64)(unsafe.Pointer(p)) }
func (c *cpu) readF32(p uintptr) float32         { return *(*float32)(unsafe.Pointer(p)) }
func (c *cpu) readF64(p uintptr) float64         { return *(*float64)(unsafe.Pointer(p)) }
func (c *cpu) readI16(p uintptr) int16           { return *(*int16)(unsafe.Pointer(p)) }
func (c *cpu) readI32(p uintptr) int32           { return *(*int32)(unsafe.Pointer(p)) }
func (c *cpu) readI64(p uintptr) int64           { return *(*int64)(unsafe.Pointer(p)) }
func (c *cpu) readI8(p uintptr) int8             { return *(*int8)(unsafe.Pointer(p)) }
func (c *cpu) readPtr(p uintptr) uintptr         { return *(*uintptr)(unsafe.Pointer(p)) }
func (c *cpu) readU16(p uintptr) uint16          { return *(*uint16)(unsafe.Pointer(p)) }
func (c *cpu) readU32(p uintptr) uint32          { return *(*uint32)(unsafe.Pointer(p)) }
func (c *cpu) readU64(p uintptr) uint64          { return *(*uint64)(unsafe.Pointer(p)) }
func (c *cpu) readU8(p uintptr) uint8            { return *(*uint8)(unsafe.Pointer(p)) }
func (c *cpu) writeC128(p uintptr, v complex128) { *(*complex128)(unsafe.Pointer(p)) = v }
func (c *cpu) writeC64(p uintptr, v complex64)   { *(*complex64)(unsafe.Pointer(p)) = v }
func (c *cpu) writeF32(p uintptr, v float32)     { *(*float32)(unsafe.Pointer(p)) = v }
func (c *cpu) writeF64(p uintptr, v float64)     { *(*float64)(unsafe.Pointer(p)) = v }
func (c *cpu) writeI16(p uintptr, v int16)       { *(*int16)(unsafe.Pointer(p)) = v }
func (c *cpu) writeI32(p uintptr, v int32)       { *(*int32)(unsafe.Pointer(p)) = v }
func (c *cpu) writeI64(p uintptr, v int64)       { *(*int64)(unsafe.Pointer(p)) = v }
func (c *cpu) writeI8(p uintptr, v int8)         { *(*int8)(unsafe.Pointer(p)) = v }
func (c *cpu) writePtr(p uintptr, v uintptr)     { *(*uintptr)(unsafe.Pointer(p)) = v }
func (c *cpu) writeU16(p uintptr, v uint16)      { *(*uint16)(unsafe.Pointer(p)) = v }
func (c *cpu) writeU32(p uintptr, v uint32)      { *(*uint32)(unsafe.Pointer(p)) = v }
func (c *cpu) writeU64(p uintptr, v uint64)      { *(*uint64)(unsafe.Pointer(p)) = v }
func (c *cpu) writeU8(p uintptr, v uint8)        { *(*uint8)(unsafe.Pointer(p)) = v }

func (c *cpu) bool(b bool) {
	if b {
		c.writeI32(c.sp, 1)
		return
	}

	c.writeI32(c.sp, 0)
}

func (c *cpu) builtin(f func()) {
	f()
	n := len(c.rpStack)
	c.sp = c.rp
	c.rp = c.rpStack[n-1]
	c.rpStack = c.rpStack[:n-1]
}

func (c *cpu) stackTrace(code []Operation) error {
	var buf buffer.Bytes
	bp := c.bp
	ip := c.ip - 1
	sp := c.sp
	ap := c.ap
	for ip < uintptr(len(code)) {
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
				fmt.Fprintf(&buf, "%#x", c.readI64(ap))
			}
			fmt.Fprintf(&buf, ")\n")
			fmt.Fprintf(&buf, "\t%s\t", li.Position())
			dumpCode(&buf, code[ip:ip+1], int(ip))
		default:
			dumpCode(&buf, code[ip:ip+1], int(ip))
		}
		sp = bp
		bp = c.readPtr(sp)
		sp += ptrStackSz
		ap = c.readPtr(sp)
		sp += ptrStackSz
		if i := sp - c.thread.ss; int(i) >= len(c.thread.stackMem) {
			break
		}

		ip = c.readPtr(sp) - 1
	}
	return errors.New(string(buf.Bytes()))
}

func (c *cpu) trace(code []Operation) string {
	s := dumpCodeStr(code[c.ip:c.ip+1], int(c.ip))
	a := make([]uintptr, 5)
	for i := range a {
		a[i] = c.readPtr(c.sp + uintptr(i*ptrStackSz))
	}
	return fmt.Sprintf("%s\t%#x: %x; %v", s[:len(s)-1], c.sp, a, c.m.pcInfo(int(c.ip), c.m.lines).Position())
}

func (c *cpu) run(code []Operation) (int, error) {
	//fmt.Printf("%#v\n", c)
	defer func() {
		if err := recover(); err != nil {
			panic(fmt.Errorf("%v\n%s", err, c.stackTrace(code)))
		}
	}()

	for i := 0; ; i++ {
		if i&1024 == 0 {
			select {
			case <-c.m.stop:
				return -1, KillError{}
			default:
			}
		}

		//fmt.Println(c.trace(code)) //TODO-
		op := code[c.ip] //TODO bench op := *(*Operation)(unsafe.Address(&code[c.ip]))
		c.ip++
		switch op.Opcode {
		case AP: // -> ptr
			c.sp -= i32StackSz
			c.writePtr(c.sp, c.ap+uintptr(op.N))
		case AddF64: // a, b -> a + b
			b := c.readF64(c.sp)
			c.sp += f64StackSz
			c.writeF64(c.sp, c.readF64(c.sp)+b)
		case AddI32: // a, b -> a + b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			c.writeI32(c.sp, c.readI32(c.sp)+b)
		case AddPtr:
			c.addPtr(c.sp, uintptr(op.N))
		case AddPtrs:
			v := c.readPtr(c.sp)
			c.sp += ptrStackSz
			c.addPtr(c.sp, v)
		case AddSP: // -
			c.sp += uintptr(op.N)
		case And32: // a, b -> a & b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			c.writeI32(c.sp, c.readI32(c.sp)&b)
		case Argument8: // -> val
			c.sp -= i8StackSz
			c.writeI8(c.sp, c.readI8(c.ap+uintptr(op.N)))
		case Argument32: // -> val
			c.sp -= i32StackSz
			c.writeI32(c.sp, c.readI32(c.ap+uintptr(op.N)))
		case Argument64: // -> val
			c.sp -= i64StackSz
			c.writeI64(c.sp, c.readI64(c.ap+uintptr(op.N)))
		case Arguments: // -
			c.rpStack = append(c.rpStack, c.rp)
			c.rp = c.sp
		case BP: // -> ptr
			c.sp -= ptrSize
			c.writePtr(c.sp, c.bp+uintptr(op.N))
		case BoolI64:
			v := c.readI64(c.sp)
			c.sp += i64StackSz - i32StackSz
			c.bool(v != 0)
		case DS: // -> ptr
			c.sp -= ptrSize
			c.writePtr(c.sp, c.ds+uintptr(op.N))
		case Call: // -> results
			c.sp -= ptrStackSz
			c.writePtr(c.sp, c.ip)
			c.ip = uintptr(op.N)
		case ConvF32F64:
			v := c.readF32(c.sp)
			c.sp += f32StackSz - f64StackSz
			c.writeF64(c.sp, float64(v))
		case ConvF64F32:
			v := c.readF64(c.sp)
			c.sp += f64StackSz - f32StackSz
			c.writeF32(c.sp, float32(v))
		case ConvF64I32:
			v := c.readF64(c.sp)
			c.sp += f64StackSz - i32StackSz
			c.writeI32(c.sp, int32(v))
		case ConvF64I8:
			v := c.readF64(c.sp)
			c.sp += f64StackSz - i8StackSz
			c.writeI8(c.sp, int8(v))
		case ConvI32F32:
			v := c.readI32(c.sp)
			c.sp += i32StackSz - f32StackSz
			c.writeF32(c.sp, float32(v))
		case ConvI32F64:
			v := c.readI32(c.sp)
			c.sp += i32StackSz - f64StackSz
			c.writeF64(c.sp, float64(v))
		case ConvI32I64:
			v := c.readI32(c.sp)
			c.sp += i32StackSz - i64StackSz
			c.writeI64(c.sp, int64(v))
		case ConvI64I32:
			v := c.readI64(c.sp)
			c.sp += i64StackSz - i32StackSz
			c.writeI32(c.sp, int32(v))
		case ConvI32I8:
			c.writeI8(c.sp, int8(c.readI32(c.sp)))
		case ConvI8I32:
			c.writeI32(c.sp, int32(c.readI8(c.sp)))
		case Copy: // &dst, &src -> &dst
			src := c.readPtr(c.sp)
			c.sp += ptrStackSz
			dst := c.readPtr(c.sp)
			for i := 0; i < op.N; i++ {
				c.writeI8(dst, c.readI8(src))
				src++
				dst++
			}
		case DivF64: // a, b -> a / b
			b := c.readF64(c.sp)
			c.sp += f64StackSz
			c.writeF64(c.sp, c.readF64(c.sp)/b)
		case DivI32: // a, b -> a / b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			c.writeI32(c.sp, c.readI32(c.sp)/b)
		case DivU64: // a, b -> a / b
			b := c.readU64(c.sp)
			c.sp += i64StackSz
			c.writeU64(c.sp, c.readU64(c.sp)/b)
		case Dup32:
			v := c.readI32(c.sp)
			c.sp -= i32StackSz
			c.writeI32(c.sp, v)
		case Dup64:
			v := c.readI64(c.sp)
			c.sp -= i64StackSz
			c.writeI64(c.sp, v)
		case EqI32: // a, b -> a == b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			a := c.readI32(c.sp)
			c.bool(a == b)
		case EqI64: // a, b -> a == b
			b := c.readI64(c.sp)
			c.sp += i64StackSz
			a := c.readI64(c.sp)
			c.bool(a == b)
		case Float32:
			c.sp -= f32StackSz
			c.writeF32(c.sp, math.Float32frombits(uint32(op.N)))
		case Float64:
			c.float64(op.N, code[c.ip].N)
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
			c.writePtr(c.sp, c.ap)
			c.ap = c.rp
			c.sp -= ptrStackSz
			c.writePtr(c.sp, c.bp)
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
		case GeqI32: // a, b -> a >= b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			a := c.readI32(c.sp)
			c.bool(a >= b)
		case GtI32: // a, b -> a > b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			a := c.readI32(c.sp)
			c.bool(a > b)
		case IndexI32: // addr, index -> addr + n*index
			x := c.readI32(c.sp)
			c.sp += i32StackSz
			c.addPtr(c.sp, uintptr(op.N*int(x)))
		case Int32: // -> val
			c.sp -= i32StackSz
			c.writeI32(c.sp, int32(op.N))
		case Int64: // -> val
			c.int64(op.N, code[c.ip].N)
		case Jmp: // -
			c.ip = uintptr(op.N)
		case Jnz: // val ->
			v := c.readI32(c.sp)
			c.sp += i32StackSz
			if v != 0 {
				c.ip = uintptr(op.N)
			}
		case Jz: // val ->
			v := c.readI32(c.sp)
			c.sp += i32StackSz
			if v == 0 {
				c.ip = uintptr(op.N)
			}
		case LeqI32: // a, b -> a <= b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			a := c.readI32(c.sp)
			c.bool(a <= b)
		case LtI32: // a, b -> a < b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			a := c.readI32(c.sp)
			c.bool(a < b)
		case Load8: // addr -> (addr+n)
			p := c.readPtr(c.sp)
			c.sp += ptrStackSz - i8StackSz
			c.writeI8(c.sp, c.readI8(p+uintptr(op.N)))
		case Load32: // addr -> (addr+n)
			p := c.readPtr(c.sp)
			c.sp += ptrStackSz - i32StackSz
			c.writeI32(c.sp, c.readI32(p+uintptr(op.N)))
		case Load64: // addr -> (addr+n)
			p := c.readPtr(c.sp)
			c.sp += ptrStackSz - i64StackSz
			c.writeI64(c.sp, c.readI64(p+uintptr(op.N)))
		case MulF64: // a, b -> a * b
			b := c.readF64(c.sp)
			c.sp += f64StackSz
			c.writeF64(c.sp, c.readF64(c.sp)*b)
		case MulI32: // a, b -> a * b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			c.writeI32(c.sp, c.readI32(c.sp)*b)
		case NeqI32: // a, b -> a |= b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			a := c.readI32(c.sp)
			c.bool(a != b)
		case NeqI64: // a, b -> a |= b
			b := c.readI64(c.sp)
			c.sp += i64StackSz
			a := c.readI64(c.sp)
			c.bool(a != b)
		case Nop: // -
			// nop
		case Or32: // a, b -> a | b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			c.writeI32(c.sp, c.readI32(c.sp)|b)
		case Panic: // -
			return -1, c.stackTrace(code)
		case PostIncI32: // adr -> (*adr)++
			p := c.readPtr(c.sp)
			c.sp += ptrStackSz - i32StackSz
			v := c.readI32(p)
			c.writeI32(c.sp, v)
			c.writeI32(p, v+int32(op.N))
		case PostIncPtr: // adr -> (*adr)++
			p := c.readPtr(c.sp)
			v := c.readPtr(p)
			c.writePtr(c.sp, v)
			c.writePtr(p, v+uintptr(op.N))
		case RemU64: // a, b -> a % b
			b := c.readU64(c.sp)
			c.sp += i64StackSz
			c.writeU64(c.sp, c.readU64(c.sp)%b)
		case Return:
			c.sp = c.bp
			c.bp = c.readPtr(c.sp)
			c.sp += ptrStackSz
			ap := c.readPtr(c.sp)
			c.sp += ptrStackSz
			c.ip = c.readPtr(c.sp)
			c.sp += ptrStackSz
			n := len(c.rpStack)
			c.rp = c.rpStack[n-1]
			c.rpStack = c.rpStack[:n-1]
			c.sp = c.ap
			c.ap = ap
		case Store8: // adr, val -> val
			v := c.readI8(c.sp)
			c.sp += i8StackSz
			c.writeI8(c.readPtr(c.sp), v)
			c.sp += ptrStackSz - i8StackSz
			c.writeI8(c.sp, v)
		case Store32: // adr, val -> val
			v := c.readI32(c.sp)
			c.sp += i32StackSz
			c.writeI32(c.readPtr(c.sp), v)
			c.sp += ptrStackSz - i32StackSz
			c.writeI32(c.sp, v)
		case Store64: // adr, val -> val
			v := c.readI64(c.sp)
			c.sp += i64StackSz
			c.writeI64(c.readPtr(c.sp), v)
			c.sp += ptrStackSz - i64StackSz
			c.writeI64(c.sp, v)
		case SubF64: // a, b -> a - b
			b := c.readF64(c.sp)
			c.sp += f64StackSz
			c.writeF64(c.sp, c.readF64(c.sp)-b)
		case SubI32: // a, b -> a - b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			c.writeI32(c.sp, c.readI32(c.sp)-b)
		case Text:
			c.sp -= ptrStackSz
			c.writePtr(c.sp, c.ts+uintptr(op.N))
		case Variable8: // -> val
			c.sp -= i8StackSz
			c.writeI8(c.sp, c.readI8(c.bp+uintptr(op.N)))
		case Variable32: // -> val
			c.sp -= i32StackSz
			c.writeI32(c.sp, c.readI32(c.bp+uintptr(op.N)))
		case Variable64: // -> val
			c.sp -= i64StackSz
			c.writeI64(c.sp, c.readI64(c.bp+uintptr(op.N)))
		case Xor32: // a, b -> a ^ b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			c.writeI32(c.sp, c.readI32(c.sp)^b)
		case Zero32:
			c.sp -= i32StackSz
			c.writeI32(c.sp, 0)
		case Zero64:
			c.sp -= i64StackSz
			c.writeI64(c.sp, 0)

		case abort:
			return 1, nil
		case exit:
			return int(c.readI32(c.sp)), nil
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

		default:
			return -1, fmt.Errorf("instruction trap: %v\n%s", op, c.stackTrace(code))
		}
	}
}
