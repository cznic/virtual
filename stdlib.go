// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"github.com/cznic/mathutil"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("abort"):  abort,
		dict.SID("abs"):    abs,
		dict.SID("calloc"): calloc,
		dict.SID("exit"):   exit,
		dict.SID("malloc"): malloc,
	})
}

// int abs(int j);
func (c *cpu) abs() {
	j := readI32(c.rp - i32StackSz)
	if j < 0 {
		j = -j
	}
	writeI32(c.rp, j)
}

// void *calloc(size_t nmemb, size_t size);
func (c *cpu) calloc() {
	ap := c.rp - stackAlign
	nmemb := readSize(ap)
	size := readSize(ap - stackAlign)
	hi, lo := mathutil.MulUint128_64(nmemb, size)
	var p uintptr
	if hi == 0 || lo <= mathutil.MaxInt {
		p = c.m.calloc(int(lo))
	}

	writePtr(c.rp, p)
}

// void *malloc(size_t size);
func (c *cpu) malloc() {
	size := readSize(c.rp - stackAlign)
	var p uintptr
	if size <= mathutil.MaxInt {
		p = c.m.malloc(int(size))
	}
	writePtr(c.rp, p)
}
