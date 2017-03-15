// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_alloca"): alloca,
		dict.SID("alloca"):           alloca,
	})
}

// void *alloca(size_t size);
func (c *cpu) alloca() {
	dest := readPtr(c.rp + ptrStackSz)
	size := roundupULong(readULong(c.rp-longStackSz), stackAlign)
	r := c.sp - uintptr(size)
	sp := r - 2*ptrStackSz
	writePtr(sp+ptrStackSz, dest)
	writePtr(sp, r)
	c.rp = sp
}
