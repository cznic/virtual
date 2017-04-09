// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("ffs"):    ffs,
		dict.SID("ffsl"):   ffsl,
		dict.SID("ffsll"):  ffsll,
		dict.SID("index"):  strchr,
		dict.SID("rindex"): strrchr,
	})
}

// int ffs(int i);
func (c *cpu) ffs() {
	i := readI32(c.sp)
	if i == 0 {
		writeI32(c.rp, 0)
		return
	}

	var r int32
	for ; r < 32 && i&(1<<uint(r)) == 0; r++ {
	}
	writeI32(c.rp, r+1)
}

// int ffsl(long i);
func (c *cpu) ffsl() {
	i := readLong(c.sp)
	if i == 0 {
		writeI32(c.rp, 0)
		return
	}

	var r int32
	for ; r < longBits && i&(1<<uint(r)) == 0; r++ {
	}
	writeI32(c.rp, r+1)
}

// int ffsll(long long i);
func (c *cpu) ffsll() {
	i := readI64(c.sp)
	if i == 0 {
		writeI32(c.rp, 0)
		return
	}

	var r int32
	for ; r < 64 && i&(1<<uint(r)) == 0; r++ {
	}
	writeI32(c.rp, r+1)
}
