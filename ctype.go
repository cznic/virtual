// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_tolower"): tolower,
	})
}

// int __builtin_tolower(int c);
func (c *cpu) tolower() {
	ch := readI32(c.rp - i32StackSz)
	if ch >= 'A' && ch <= 'Z' {
		ch ^= ' '
	}
	writeI32(c.rp, ch)
}
