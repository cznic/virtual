// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_memcmp"):  memcmp,
		dict.SID("__builtin_memcpy"):  memcpy,
		dict.SID("__builtin_memmove"): memmove,
		dict.SID("__builtin_mempcpy"): mempcpy,
		dict.SID("__builtin_memset"):  memset,
		dict.SID("__builtin_strcat"):  strcat,
		dict.SID("__builtin_strchr"):  strchr,
		dict.SID("__builtin_strcmp"):  strcmp,
		dict.SID("__builtin_strcpy"):  strcpy,
		dict.SID("__builtin_strlen"):  strlen,
		dict.SID("__builtin_strncmp"): strncmp,
		dict.SID("__builtin_strncpy"): strncpy,
		dict.SID("__builtin_strrchr"): strrchr,
		dict.SID("memcmp"):            memcmp,
		dict.SID("memcpy"):            memcpy,
		dict.SID("memmove"):           memmove,
		dict.SID("mempcpy"):           mempcpy,
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

// int memcmp(const void *s1, const void *s2, size_t n)
func (c *cpu) memcmp() {
	sp, n := popLong(c.sp)
	sp, s2 := popPtr(sp)
	s1 := readPtr(sp)
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
	sp, n := popLong(c.sp)
	sp, src := popPtr(sp)
	dest := readPtr(sp)
	movemem(dest, src, int(n))
	writePtr(c.rp, dest)
}

// void *memmove(void *dest, const void *src, size_t n);
func (c *cpu) memmove() {
	sp, n := popLong(c.sp)
	sp, src := popPtr(sp)
	dest := readPtr(sp)
	movemem(dest, src, int(n))
	writePtr(c.rp, dest)
}

// void *mempcpy(void *dest, const void *src, size_t n);
func (c *cpu) mempcpy() {
	sp, n := popLong(c.sp)
	sp, src := popPtr(sp)
	dest := readPtr(sp)
	movemem(dest, src, int(n))
	writePtr(c.rp, dest+uintptr(n))
}

// void *memset(void *s, int c, size_t n)
func (c *cpu) memset() {
	sp, n := popLong(c.sp)
	sp, ch := popI32(sp)
	s := readPtr(sp)
	ret := s
	for d := s; n > 0; n-- {
		writeI8(d, int8(ch))
		d++
	}
	writePtr(c.rp, ret)
}

// char *strcat(char *dest, const char *src)
func (c *cpu) strcat() {
	sp, src := popPtr(c.sp)
	dest := readPtr(sp)
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
	sp, ch := popI32(c.sp)
	s := readPtr(sp)
	for {
		ch2 := readU8(s)
		if ch2 == byte(ch) {
			writePtr(c.rp, s)
			return
		}

		if ch2 == 0 {
			writePtr(c.rp, 0)
			return
		}

		s++
	}
}

// int strcmp(const char *s1, const char *s2)
func (c *cpu) strcmp() {
	sp, s2 := popPtr(c.sp)
	s1 := readPtr(sp)
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
	sp, src := popPtr(c.sp)
	dest := readPtr(sp)
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
	writeULong(c.rp, n)
}

// int strncmp(const char *s1, const char *s2, size_t n)
func (c *cpu) strncmp() {
	sp, n := popLong(c.sp)
	sp, s2 := popPtr(sp)
	s1 := readPtr(sp)
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
	sp, n := popLong(c.sp)
	sp, src := popPtr(sp)
	dest := readPtr(sp)
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
	sp, ch := popI32(c.sp)
	s := readPtr(sp)
	var ret uintptr
	for {
		ch2 := readU8(s)
		if ch2 == 0 {
			writePtr(c.rp, ret)
			return
		}

		if ch2 == byte(ch) {
			ret = s
		}
		s++
	}
}
