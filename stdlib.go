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
		dict.SID("exit"):   exit,
		dict.SID("malloc"): malloc,
	})
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
