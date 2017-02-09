// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"errors"
	"sort"

	"github.com/cznic/internal/buffer"
)

import (
	"fmt"
	"unsafe"
)

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

func (c *cpu) run(code []Operation) (int, error) {
	for i := 0; ; i++ {
		if i&1024 == 0 {
			select {
			case <-c.m.stop:
				return -1, KillError{}
			default:
			}
		}

		fmt.Printf("# cpu\t%s", dumpCodeStr(code[c.ip:c.ip+1], int(c.ip))) //TODO-
		op := code[c.ip]                                                   //TODO bench op := *(*Operation)(unsafe.Address(&code[c.ip]))
		c.ip++
		switch op.Opcode {
		case Abort: // -
			return 1, nil
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
		case BP: // -> val
			c.sp -= ptrSize
			c.writePtr(c.sp, c.bp+uintptr(op.N))
		case Call: // -> results
			c.sp -= ptrStackSz
			c.writePtr(c.sp, c.ip)
			c.ip = uintptr(op.N)
		case Exit: // -
			// void __builtin_exit(int status);
			return int(c.readI32(c.sp)), nil
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
		case Int32: // -> val
			c.sp -= i32StackSz
			c.writeI32(c.sp, int32(op.N))
		case Jmp: // -
			c.ip = uintptr(op.N)
		case Nop: // -
			// nop
		case Panic: // -
			panic(c.trace(code))
		case Store32: // adr, val -> val
			v := c.readI32(c.sp)
			c.sp += i32StackSz
			c.writeI32(c.readPtr(c.sp), v)
			c.sp += ptrStackSz - i32StackSz
			c.writeI32(c.sp, v)
		case Text:
			c.sp -= ptrStackSz
			c.writePtr(c.sp, c.ts+uintptr(op.N))
		case Variable32: // -> val
			c.sp -= i32StackSz
			c.writeI32(c.sp, c.readI32(c.bp+uintptr(op.N)))
		default:
			s := dumpCodeStr(code[c.ip-1:c.ip], int(c.ip)-1)
			panic(fmt.Errorf("%s\t// %s", s[:len(s)-1], op))
		}
	}
}

func (c *cpu) trace(code []Operation) error {
	var buf buffer.Bytes
	bp := c.bp
	ip := c.ip - 1
	sp := c.sp
	for ip < uintptr(len(code)) {
		var fi, li PCInfo
		if i := sort.Search(len(c.m.functions), func(i int) bool { return c.m.functions[i].PC >= int(ip) }); i <= len(c.m.functions) {
			if i > 0 {
				fi = c.m.functions[i-1]
			}
		}
		if i := sort.Search(len(c.m.lines), func(i int) bool { return c.m.lines[i].PC >= int(ip) }); i < len(c.m.lines) {
			li = c.m.lines[i]
		}
		switch p := li.Position(); {
		case p.IsValid():
			fp := fi.Position()
			fp.Filename = p.Filename
			fmt.Fprintf(&buf, "%s: %s\n", fp, dict.S(int(fi.Name)))
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
