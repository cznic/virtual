// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"github.com/cznic/mathutil"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_clz"):     clz, //TODO -> builting.go
		dict.SID("__builtin_ctz"):     ctz, //TODO -> builting.go
		dict.SID("__builtin_ffs"):     ffs,
		dict.SID("__builtin_memcmp"):  memcmp,
		dict.SID("__builtin_memcpy"):  memcpy,
		dict.SID("__builtin_memset"):  memset,
		dict.SID("__builtin_strcmp"):  strcmp,
		dict.SID("__builtin_strcpy"):  strcpy,
		dict.SID("__builtin_strlen"):  strlen,
		dict.SID("__builtin_strncmp"): strncmp,
		dict.SID("ffs"):               ffs,
		dict.SID("memcmp"):            memcmp,
		dict.SID("memcpy"):            memcpy,
		dict.SID("memset"):            memset,
		dict.SID("strcat"):            strcat,
		dict.SID("strchr"):            strchr,
		dict.SID("strcmp"):            strcmp,
		dict.SID("strcpy"):            strcpy,
		dict.SID("strlen"):            strlen,
		dict.SID("strncmp"):           strncmp,
		dict.SID("strncpy"):           strncpy,
		dict.SID("strrchr"):           strrchr,
	})
}

// int __builtin_clz (unsigned x);
//
// Returns the number of leading 0-bits in x, starting at the most significant
// bit position. If x is 0, the result is undefined.
func (c *cpu) clz() {
	writeI32(c.rp, int32(32-mathutil.Log2Uint32(readU32(c.rp-i32StackSz))))
}

// int __builtin_ctz (unsigned x);
//
// Returns the number of trailing 0-bits in x, starting at the least
// significant bit position. If x is 0, the result is undefined.
func (c *cpu) ctz() {
	var r int32
	for x := readU32(c.rp - i32StackSz); r < 32 && x&1 != 0; r++ {
	}
	writeI32(c.rp, r)
}

// int ffs(int i);
func (c *cpu) ffs() {
	var r int32
	for i := readI32(c.rp - i32StackSz); i != 0 && i&1 == 0; r++ {
	}
	writeI32(c.rp, r)
}

// int memcmp(const void *s1, const void *s2, size_t n)
func (c *cpu) memcmp() {
	ap := c.rp - ptrStackSz
	s1 := readPtr(ap)
	ap -= ptrStackSz
	s2 := readPtr(ap)
	ap -= stackAlign
	n := readSize(ap)
	var ch1, ch2 byte
	for n != 0 {
		ch1 = readU8(s1)
		ch2 = readU8(s2)
		if ch1 != ch2 {
			break
		}

		n--
	}
	if n != 0 {
		writeI32(c.rp, int32(ch1)-int32(ch2))
		return
	}

	writeI32(c.rp, 0)
}

// void *memcpy(void *dest, const void *src, size_t n)
func (c *cpu) memcpy() {
	ap := c.rp - ptrStackSz
	dest := readPtr(ap)
	ap -= ptrStackSz
	memcopy(dest, readPtr(ap), int(readSize(ap-stackAlign)))
	writePtr(c.rp, dest)
}

// void *memset(void *s, int c, size_t n)
func (c *cpu) memset() {
	ap := c.rp - ptrStackSz
	s := readPtr(ap)
	ap -= i32StackSz
	ch := readI8(ap)
	ap -= stackAlign
	n := readSize(ap)
	ret := s
	for d := s; n > 0; n-- {
		writeI8(d, ch)
		d++
	}
	writePtr(c.rp, ret)
}

// char *strcat(char *dest, const char *src)
func (c *cpu) strcat() {
	dest := readPtr(c.sp + ptrStackSz)
	src := readPtr(c.sp)
	ret := dest
	for readI8(dest) != 0 {
		dest++
	}
	for {
		ch := readI8(src)
		src++
		writeI8(dest, ch)
		dest++
		if ch == 0 {
			writePtr(c.rp, ret)
			return
		}
	}
}

// char *strchr(const char *s, int c)
func (c *cpu) strchr() {
	s := readPtr(c.sp + ptrStackSz)
	ch := byte(readI32(c.sp))
	for {
		ch2 := readU8(s)
		if ch2 == 0 {
			writePtr(c.rp, 0)
			return
		}

		if ch2 == ch {
			writePtr(c.rp, s)
			return
		}

		s++
	}
}

// int strcmp(const char *s1, const char *s2)
func (c *cpu) strcmp() {
	s1 := readPtr(c.sp + ptrStackSz)
	s2 := readPtr(c.sp)
	for {
		ch1 := readU8(s1)
		s1++
		ch2 := readU8(s2)
		s2++
		if ch1 != ch2 || ch1 == 0 || ch2 == 0 {
			writeI32(c.rp, int32(ch1)-int32(ch2))
			return
		}
	}
}

// char *strcpy(char *dest, const char *src)
func (c *cpu) strcpy() {
	dest := readPtr(c.sp + ptrStackSz)
	src := readPtr(c.sp)
	ret := dest
	for {
		ch := readI8(src)
		src++
		writeI8(dest, ch)
		dest++
		if ch == 0 {
			writePtr(c.rp, ret)
			return
		}
	}
}

// size_t strlen(const char *s)
func (c *cpu) strlen() {
	var n uint64
	for s := readPtr(c.sp); readI8(s) != 0; s++ {
		n++
	}
	writeSize(c.rp, n)
}

// int strncmp(const char *s1, const char *s2, size_t n)
func (c *cpu) strncmp() {
	ap := c.rp - ptrStackSz
	s1 := readPtr(ap)
	ap -= ptrStackSz
	s2 := readPtr(ap)
	ap -= stackAlign
	n := readSize(ap)
	var ch1, ch2 byte
	for n != 0 {
		ch1 = readU8(s1)
		s1++
		ch2 = readU8(s2)
		s2++
		n--
		if ch1 != ch2 || ch1 == 0 || ch2 == 0 {
			break
		}
	}
	if n != 0 {
		writeI32(c.rp, int32(ch1)-int32(ch2))
		return
	}

	writeI32(c.rp, 0)
}

// char *strncpy(char *dest, const char *src, size_t n)
func (c *cpu) strncpy() {
	ap := c.rp - ptrStackSz
	dest := readPtr(ap)
	ap -= ptrStackSz
	src := readPtr(ap)
	ap -= stackAlign
	n := readSize(ap)
	ret := dest
	var ch int8
	for ch = readI8(src); ch != 0 && n > 0; n-- {
		writeI8(dest, ch)
		dest++
		src++
		ch = readI8(src)
	}
	for ; n > 0; n-- {
		writeI8(dest, 0)
		dest++
	}
	writePtr(c.rp, ret)
}

// char *strrchr(const char *s, int c)
func (c *cpu) strrchr() {
	s := readPtr(c.sp + ptrStackSz)
	ch := byte(readI32(c.sp))
	var ret uintptr
	for {
		ch2 := readU8(s)
		if ch2 == 0 {
			writePtr(c.rp, ret)
			return
		}

		if ch2 == ch {
			ret = s
		}
		s++
	}
}
