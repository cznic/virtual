// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_isprint"): isprint,
		dict.SID("__builtin_tolower"): tolower,
		dict.SID("isprint"):           isprint,
		dict.SID("tolower"):           tolower,
	})
}

// int isprint(int c);
func (c *cpu) isprint() {
	ch := readI32(c.sp)
	var r int32
	if ch >= ' ' && ch <= '~' {
		r = 1
	}
	writeI32(c.rp, r)
}

// int tolower(int c);
func (c *cpu) tolower() {
	ch := readI32(c.sp)
	if ch >= 'A' && ch <= 'Z' {
		ch |= ' '
	}
	writeI32(c.rp, ch)
}
