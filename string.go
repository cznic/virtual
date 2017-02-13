// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_memcmp"):  memcmp,
		dict.SID("__builtin_memcpy"):  memcpy,
		dict.SID("__builtin_memset"):  memset,
		dict.SID("__builtin_strcat"):  strcat,
		dict.SID("__builtin_strchr"):  strchr,
		dict.SID("__builtin_strcmp"):  strcmp,
		dict.SID("__builtin_strcpy"):  strcpy,
		dict.SID("__builtin_strlen"):  strlen,
		dict.SID("__builtin_strncmp"): strncmp,
		dict.SID("__builtin_strncpy"): strncpy,
		dict.SID("__builtin_strrchr"): strrchr,
	})
}

func (c *cpu) strcat() { // char *strcat(char *dest, const char *src)
	dest := c.readPtr(c.sp + ptrStackSz)
	src := c.readPtr(c.sp)
	ret := dest
	for c.readI8(dest) != 0 {
		dest++
	}
	for {
		ch := c.readI8(src)
		src++
		c.writeI8(dest, ch)
		dest++
		if ch == 0 {
			c.writePtr(c.rp, ret)
			return
		}
	}
}

func (c *cpu) strchr() { // char *strchr(const char *s, int c)
	s := c.readPtr(c.sp + ptrStackSz)
	ch := byte(c.readI32(c.sp))
	for {
		ch2 := c.readU8(s)
		if ch2 == 0 {
			c.writePtr(c.rp, 0)
			return
		}

		if ch2 == ch {
			c.writePtr(c.rp, s)
			return
		}

		s++
	}
}

func (c *cpu) strcmp() { // int strcmp(const char *s1, const char *s2)
	s1 := c.readPtr(c.sp + ptrStackSz)
	s2 := c.readPtr(c.sp)
	for {
		ch1 := c.readU8(s1)
		s1++
		ch2 := c.readU8(s2)
		s2++
		if ch1 != ch2 || ch1 == 0 || ch2 == 0 {
			c.writeI32(c.rp, int32(ch1)-int32(ch2))
			return
		}
	}
}

func (c *cpu) strcpy() { // char *strcpy(char *dest, const char *src)
	dest := c.readPtr(c.sp + ptrStackSz)
	src := c.readPtr(c.sp)
	ret := dest
	for {
		ch := c.readI8(src)
		src++
		c.writeI8(dest, ch)
		dest++
		if ch == 0 {
			c.writePtr(c.rp, ret)
			return
		}
	}
}

func (c *cpu) strrchr() { // char *strrchr(const char *s, int c)
	s := c.readPtr(c.sp + ptrStackSz)
	ch := byte(c.readI32(c.sp))
	var ret uintptr
	for {
		ch2 := c.readU8(s)
		if ch2 == 0 {
			c.writePtr(c.rp, ret)
			return
		}

		if ch2 == ch {
			ret = s
		}
		s++
	}
}
