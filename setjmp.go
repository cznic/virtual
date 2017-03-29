// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"unsafe"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_longjmp"): longjmp,
		dict.SID("__builtin_setjmp"):  setjmp,
		dict.SID("longjmp"):           longjmp,
		dict.SID("setjmp"):            setjmp,
	})
}

// void longjmp(jmp_buf env, int val);
func (c *cpu) longjmp() {
	sp, val := popI32(c.sp)
	c.jmpBuf = *(*jmpBuf)(unsafe.Pointer(readPtr(sp)))
	c.fpStack = c.fpStack[:c.fpStackP]
	c.rpStack = c.rpStack[:c.rpStackP]
	if val == 0 {
		val = 1
	}
	writeI32(c.rp, val)
}

// int setjmp(jmp_buf env);
func (c *cpu) setjmp() {
	c.fpStackP = uintptr(len(c.fpStack))
	c.rpStackP = uintptr(len(c.rpStack))
	*(*jmpBuf)(unsafe.Pointer(readPtr(c.sp))) = c.jmpBuf
	writeI32(c.rp, 0)
}
