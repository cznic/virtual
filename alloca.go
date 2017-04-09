// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("alloca"): alloca,
	})
}

// void *alloca(size_t size);
func (c *cpu) alloca() {

	// dest = alloca(size);
	//
	// +--------------------+
	// | &dest              |
	// +--------------------+
	// | space for result   | <- rp
	// +--------------------+
	// | size               | <- sp
	// +--------------------+

	dest := readPtr(c.rp + ptrStackSz)
	size := roundupULong(readULong(c.sp), stackAlign)
	sp := c.rp - uintptr(size)
	r := sp + 2*ptrStackSz
	writePtr(sp+ptrStackSz, dest)
	writePtr(sp, r)
	c.rp = sp

	// +--------------------+
	// | allocated space    |
	// +--------------------+
	// | &dest              |
	// +--------------------+
	// | &allocated space   | <- sp
	// +--------------------+
}
