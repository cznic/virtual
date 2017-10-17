// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"unsafe"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("htonl"): htonl,
		dict.SID("htons"): htons,
	})
}

var (
	k1234        int64 = 0x1234
	littleEndian bool
)

func init() {
	littleEndian = *(*byte)((unsafe.Pointer)(&k1234)) == 0x34
}

// uint32_t htonl(uint32_t hostlong);
func (c *cpu) htonl() {
	x := readU32(c.sp)
	if littleEndian {
		x = x&0x000000ff<<24 |
			x&0x0000ff00<<8 |
			x&0x00ff0000>>8 |
			x&0xff000000>>24
	}
	writeU32(c.rp, x)
}
