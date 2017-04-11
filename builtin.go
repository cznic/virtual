// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"github.com/cznic/mathutil"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_bswap64"):        bswap64,
		dict.SID("__builtin_clrsb"):          clrsb,
		dict.SID("__builtin_clrsbl"):         clrsbl,
		dict.SID("__builtin_clrsbll"):        clrsbll,
		dict.SID("__builtin_clz"):            clz,
		dict.SID("__builtin_clzl"):           clzl,
		dict.SID("__builtin_clzll"):          clzll,
		dict.SID("__builtin_ctz"):            ctz,
		dict.SID("__builtin_ctzl"):           ctzl,
		dict.SID("__builtin_ctzll"):          ctzll,
		dict.SID("__builtin_frame_address"):  frameAddress,
		dict.SID("__builtin_parity"):         parity,
		dict.SID("__builtin_parityl"):        parityl,
		dict.SID("__builtin_parityll"):       parityll,
		dict.SID("__builtin_popcount"):       popcount,
		dict.SID("__builtin_popcountl"):      popcountl,
		dict.SID("__builtin_popcountll"):     popcountll,
		dict.SID("__builtin_return_address"): returnAddress,
		dict.SID("__builtin_trap"):           abort,
	})
}

// uint64_t __builtin_bswap64 (uint64_t x)
func (c *cpu) bswap64() {
	x := readU64(c.sp)
	writeU64(
		c.rp,
		x&0x00000000000000ff<<56|
			x&0x000000000000ff00<<40|
			x&0x0000000000ff0000<<24|
			x&0x00000000ff000000<<8|
			x&0x000000ff00000000>>8|
			x&0x0000ff0000000000>>24|
			x&0x00ff000000000000>>40|
			x&0xff00000000000000>>56,
	)
}

// int __builtin_clrsb (int x);
func (c *cpu) clrsb() {
	x := readI32(c.sp)
	i := int32(1)
	n := x >> 31 & 1
	for ; i < 32 && x>>(31-uint(i))&1 == n; i++ {
	}
	writeI32(c.rp, i-1)
}

// int __builtin_clrsbl (long x);
func (c *cpu) clrsbl() {
	x := readLong(c.sp)
	i := int32(1)
	n := x >> (longBits - 1) & 1
	for ; i < longBits && x>>(longBits-1-uint(i))&1 == n; i++ {
	}
	writeI32(c.rp, i-1)
}

// int __builtin_clrsbll (long long x);
func (c *cpu) clrsbll() {
	x := readI64(c.sp)
	i := int32(1)
	n := x >> 63 & 1
	for ; i < 64 && x>>(63-uint(i))&1 == n; i++ {
	}
	writeI32(c.rp, i-1)
}

// int __builtin_clz (unsigned x);
func (c *cpu) clz() {
	x := readU32(c.sp)
	var i int32
	for ; i < 32 && x&(1<<uint(31-i)) == 0; i++ {
	}
	writeI32(c.rp, i)
}

// int __builtin_clzl (unsigned long x);
func (c *cpu) clzl() {
	x := readULong(c.sp)
	var i int32
	for ; i < longBits && x&(1<<uint(longBits-1-i)) == 0; i++ {
	}
	writeI32(c.rp, i)
}

// int __builtin_clzll (unsigned long long x);
func (c *cpu) clzll() {
	x := readU64(c.sp)
	var i int32
	for ; i < 64 && x&(1<<uint(63-i)) == 0; i++ {
	}
	writeI32(c.rp, i)
}

// int __builtin_ctz (unsigned x);
func (c *cpu) ctz() {
	x := readU32(c.sp)
	var i int32
	for ; i < 32 && x&(1<<uint(i)) == 0; i++ {
	}
	writeI32(c.rp, i)
}

// int __builtin_ctzl (unsigned long x);
func (c *cpu) ctzl() {
	x := readULong(c.sp)
	var i int32
	for ; i < longBits && x&(1<<uint(i)) == 0; i++ {
	}
	writeI32(c.rp, i)
}

// int __builtin_ctzll (unsigned long long x);
func (c *cpu) ctzll() {
	x := readU64(c.sp)
	var i int32
	for ; i < 64 && x&(1<<uint(i)) == 0; i++ {
	}
	writeI32(c.rp, i)
}

// void *__builtin_frame_address(unsigned int level);
func (c *cpu) frameAddress() {
	level := readU32(c.sp)
	bp := c.bp
	ip := c.ip - 1
	sp := c.sp
	for level != 0 && ip < uintptr(len(c.code)) {
		sp = bp
		bp = readPtr(sp)
		sp += 2 * ptrStackSz
		if i := sp - c.thread.ss; int(i) >= len(c.thread.stackMem) {
			break
		}

		ip = readPtr(sp) - 1
		level--
	}
	writePtr(c.rp, bp)
}

// int __builtin_parity(unsigned x);
func (c *cpu) parity() { writeI32(c.rp, int32(mathutil.PopCountUint32(readU32(c.sp)))&1) }

// int __builtin_parityl(unsigned long x);
func (c *cpu) parityl() {
	writeI32(c.rp, int32(mathutil.PopCountUint64(readULong(c.sp)))&1)
}

// int __builtin_parityll(unsigned long long x);
func (c *cpu) parityll() { writeI32(c.rp, int32(mathutil.PopCountUint64(readU64(c.sp)))&1) }

// int __builtin_popcount(unsigned x);
func (c *cpu) popcount() { writeI32(c.rp, int32(mathutil.PopCountUint32(readU32(c.sp)))) }

// int __builtin_popcountl(unsigned long x);
func (c *cpu) popcountl() {
	writeI32(c.rp, int32(mathutil.PopCountUint64(readULong(c.sp))))
}

// int __builtin_popcountll(unsigned long long x);
func (c *cpu) popcountll() { writeI32(c.rp, int32(mathutil.PopCountUint64(readU64(c.sp)))) }

// void *__builtin_return_address(unsigned int level);
func (c *cpu) returnAddress() {
	level := readU32(c.sp)
	bp := c.bp
	ip := c.ip - 1
	sp := c.sp
	for ip < uintptr(len(c.code)) {
		sp = bp
		bp = readPtr(sp)
		sp += 2 * ptrStackSz
		if i := sp - c.thread.ss; int(i) >= len(c.thread.stackMem) {
			break
		}

		ip = readPtr(sp) - 1
		if level == 0 {
			break
		}

		level--
	}
	writePtr(c.rp, ip)
}
