// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build amd64

package virtual

import (
	"math"
)

func (c *cpu) int64(n, m int) {
	c.sp -= i64StackSz
	c.writeI64(c.sp, int64(n))
}

func (c *cpu) float64(n, m int) {
	c.sp -= f64StackSz
	c.writeF64(c.sp, math.Float64frombits(uint64(n)))
}

// char *strncpy(char *dest, const char *src, size_t n)
func (c *cpu) strncpy() {
	ap := c.rp - ptrStackSz
	dest := c.readPtr(ap)
	ap -= ptrStackSz
	src := c.readPtr(ap)
	ap -= i64StackSz
	n := c.readI64(ap)
	ret := dest
	var ch int8
	for ch = c.readI8(src); ch != 0 && n > 0; n-- {
		c.writeI8(dest, ch)
		dest++
		src++
		ch = c.readI8(src)
	}
	for ; n > 0; n-- {
		c.writeI8(dest, 0)
		dest++
	}
	c.writePtr(c.rp, ret)
}

// size_t strlen(const char *s)
func (c *cpu) strlen() {
	var n uint64
	for s := c.readPtr(c.sp); c.readI8(s) != 0; s++ {
		n++
	}
	c.writeU64(c.rp, n)
}

// int strncmp(const char *s1, const char *s2, size_t n)
func (c *cpu) strncmp() {
	ap := c.rp - ptrStackSz
	s1 := c.readPtr(ap)
	ap -= ptrStackSz
	s2 := c.readPtr(ap)
	ap -= i64StackSz
	n := c.readI64(ap)
	var ch1, ch2 byte
	for n != 0 {
		ch1 = c.readU8(s1)
		s1++
		ch2 = c.readU8(s2)
		s2++
		n--
		if ch1 != ch2 || ch1 == 0 || ch2 == 0 {
			break
		}
	}
	if n != 0 {
		c.writeI32(c.rp, int32(ch1)-int32(ch2))
		return
	}

	c.writeI32(c.rp, 0)
}

// void *memset(void *s, int c, size_t n)
func (c *cpu) memset() {
	ap := c.rp - ptrStackSz
	s := c.readPtr(ap)
	ap -= i32StackSz
	ch := c.readI8(ap)
	ap -= i64StackSz
	n := c.readI64(ap)
	ret := s
	for d := s; n > 0; n-- {
		c.writeI8(d, ch)
		d++
	}
	c.writePtr(c.rp, ret)
}

// void *memcpy(void *dest, const void *src, size_t n)
func (c *cpu) memcpy() {
	ap := c.rp - ptrStackSz
	dest := c.readPtr(ap)
	ap -= ptrStackSz
	src := c.readPtr(ap)
	ap -= i64StackSz
	n := c.readI64(ap)
	ret := dest
	for n > 0 {
		c.writeI8(dest, c.readI8(src))
		dest++
		src++
		n--
	}
	c.writePtr(c.rp, ret)
}

// int memcmp(const void *s1, const void *s2, size_t n)
func (c *cpu) memcmp() {
	ap := c.rp - ptrStackSz
	s1 := c.readPtr(ap)
	ap -= ptrStackSz
	s2 := c.readPtr(ap)
	ap -= i64StackSz
	n := c.readI64(ap)
	var ch1, ch2 byte
	for n != 0 {
		ch1 = c.readU8(s1)
		ch2 = c.readU8(s2)
		if ch1 != ch2 {
			break
		}

		n--
	}
	if n != 0 {
		c.writeI32(c.rp, int32(ch1)-int32(ch2))
		return
	}

	c.writeI32(c.rp, 0)
}
