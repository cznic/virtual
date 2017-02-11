// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"errors"

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
	bss     uintptr // Zero data in data segment
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

func (c *cpu) trace(code []Operation) error {
	var buf buffer.Bytes
	bp := c.bp
	ip := c.ip - 1
	sp := c.sp
	ap := c.ap
	rpStack := c.rpStack
	for ip < uintptr(len(code)) {
		fi := c.m.pcInfo(int(ip), c.m.functions)
		li := c.m.pcInfo(int(ip), c.m.lines)
		switch p := li.Position(); {
		case p.IsValid():
			fmt.Fprintf(&buf, "%s.%s(", p.Filename, dict.S(int(fi.Name)))
			for i := 0; i < fi.C; i++ {
				if i != 0 {
					fmt.Fprintf(&buf, ", ")
				}
				ap -= stackAlign
				fmt.Fprintf(&buf, "%#x", c.readI64(ap))
			}
			ap = rpStack[len(rpStack)-1] - stackAlign
			rpStack = rpStack[:len(rpStack)-1]
			fmt.Fprintf(&buf, ")\n")
			fmt.Fprintf(&buf, "\t%s\t", li.Position())
			dumpCode(&buf, code[ip:ip+1], int(ip))
		default:
			dumpCode(&buf, code[ip:ip+1], int(ip))
		}
		sp = bp
		bp = c.readPtr(sp)
		sp += 2 * ptrStackSz
		if i := sp - c.thread.ss; int(i) >= len(c.thread.stackMem) {
			break
		}

		ip = c.readPtr(sp) - 1
	}
	return errors.New(string(buf.Bytes()))
}

func (c *cpu) run(code []Operation) (int, error) {
	defer func() {
		if err := recover(); err != nil {
			panic(fmt.Errorf("%v\n%s", err, c.trace(code)))
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

		// fmt.Printf("# cpu\t%s", dumpCodeStr(code[c.ip:c.ip+1], int(c.ip))) //TODO-
		op := code[c.ip] //TODO bench op := *(*Operation)(unsafe.Address(&code[c.ip]))
		c.ip++
		switch op.Opcode {
		case AP: // -> ptr
			c.sp -= i32StackSz
			c.writePtr(c.sp, c.ap+uintptr(op.N))
		case AddI32: // a, b -> a + b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			c.writeI32(c.sp, c.readI32(c.sp)+b)
		case AddPtr:
			c.addPtr(c.sp, uintptr(op.N))
		case AddSP: // -
			c.sp += uintptr(op.N)
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
		case BSS: // -> ptr
			c.sp -= ptrSize
			c.writePtr(c.sp, c.bss+uintptr(op.N))
		case Call: // -> results
			c.sp -= ptrStackSz
			c.writePtr(c.sp, c.ip)
			c.ip = uintptr(op.N)
		case Dup32:
			v := c.readI32(c.sp)
			c.sp -= i32StackSz
			c.writeI32(c.sp, v)
		case EqI32: // a, b -> a == b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			a := c.readI32(c.sp)
			c.bool(a == b)
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
		case IndexI32: // addr, index -> addr + n*index
			x := c.readI32(c.sp)
			c.sp += i32StackSz
			c.addPtr(c.sp, uintptr(op.N*int(x)))
		case Int32: // -> val
			c.sp -= i32StackSz
			c.writeI32(c.sp, int32(op.N))
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
		//TODO- case Label: // -
		//TODO- 	// nop
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
		case Load32: // addr -> (addr+n)
			p := c.readPtr(c.sp)
			c.sp += ptrStackSz - i32StackSz
			c.writeI32(c.sp, c.readI32(p+uintptr(op.N)))
		case MulI32: // a, b -> a * b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			c.writeI32(c.sp, c.readI32(c.sp)*b)
		case Nop: // -
			// nop
		case Panic: // -
			return -1, c.trace(code)
		case PostIncI32: // adr -> (*adr)++
			p := c.readPtr(c.sp)
			c.sp += ptrStackSz - i32StackSz
			v := c.readI32(p)
			c.writeI32(c.sp, v)
			c.writeI32(p, v+1)
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
		case SubI32: // a, b -> a - b
			b := c.readI32(c.sp)
			c.sp += i32StackSz
			c.writeI32(c.sp, c.readI32(c.sp)-b)
		case Text:
			c.sp -= ptrStackSz
			c.writePtr(c.sp, c.ts+uintptr(op.N))
		case Variable32: // -> val
			c.sp -= i32StackSz
			c.writeI32(c.sp, c.readI32(c.bp+uintptr(op.N)))
		case Variable64: // -> val
			c.sp -= i64StackSz
			c.writeI64(c.sp, c.readI64(c.bp+uintptr(op.N)))

		case abort:
			return 1, nil
		case exit:
			return int(c.readI32(c.sp)), nil
		case printf:
			c.builtin(c.printf)

		default:
			return -1, fmt.Errorf("instruction trap: %v\n%s", op, c.trace(code))
		}
	}
}
