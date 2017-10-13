// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"os"
	"sort"

	"github.com/cznic/ccir/libc/errno"
	"github.com/cznic/mathutil"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("abort"):   abort,
		dict.SID("abs"):     abs,
		dict.SID("atoi"):    atoi,
		dict.SID("calloc"):  calloc,
		dict.SID("exit"):    exit,
		dict.SID("free"):    free,
		dict.SID("getenv"):  getenv,
		dict.SID("malloc"):  malloc,
		dict.SID("qsort"):   qsort,
		dict.SID("realloc"): realloc,
		dict.SID("strtoul"): strtoul,
		dict.SID("system"):  system,
	})
}

// int abs(int j);
func (c *cpu) abs() {
	j := readI32(c.sp)
	if j < 0 {
		j = -j
	}
	writeI32(c.rp, j)
}

// int atoi(char *__nptr);
func (c *cpu) atoi() {
	nptr := readPtr(c.sp)
	var r int32
	for {
		ch := readU8(nptr)
		nptr++
		if ch < '0' || ch > '9' {
			writeI32(c.rp, r)
			return
		}

		r = 10*r + int32(ch) - '0'
	}
}

// void *calloc(size_t nmemb, size_t size);
func (c *cpu) calloc() {
	sp, size := popLong(c.sp)
	nmemb := readLong(sp)
	hi, lo := mathutil.MulUint128_64(uint64(nmemb), uint64(size))
	var p uintptr
	if hi == 0 || lo <= mathutil.MaxInt {
		p = c.m.calloc(int(lo))
	}
	if strace {
		fmt.Fprintf(os.Stderr, "calloc(%#x) %#x\n", size, p)
	}
	if p == 0 {
		c.setErrno(errno.XENOMEM)
	}
	writePtr(c.rp, p)
}

// void free(void *ptr);
func (c *cpu) free() {
	ptr := readPtr(c.sp)
	if strace {
		fmt.Fprintf(os.Stderr, "freep(%#x)\n", ptr)
	}
	c.m.free(ptr)
}

// void *malloc(size_t size);
func (c *cpu) malloc() {
	size := readLong(c.sp)
	var p uintptr
	if size <= mathutil.MaxInt {
		p = c.m.malloc(int(size))
	}
	if strace {
		fmt.Fprintf(os.Stderr, "malloc(%#x) %#x\n", size, p)
	}
	if p == 0 {
		c.setErrno(errno.XENOMEM)
	}
	writePtr(c.rp, p)
}

type sorter struct {
	c      *cpu
	base   uintptr
	nmemb  int
	size   int
	compar uintptr
}

func (s *sorter) Len() int { return s.nmemb }

func (s *sorter) ptr(i int) uintptr { return s.base + uintptr(i)*uintptr(s.size) }

func (s *sorter) Less(i, j int) bool {
	c := s.c
	// Alloc result
	c.sp -= i32StackSz
	// Arguments
	c.rpStack = append(c.rpStack, c.rp)
	c.rp = c.sp
	// Argument #1
	c.sp -= ptrStackSz
	writePtr(c.sp, s.ptr(i))
	// Argument #2
	c.sp -= ptrStackSz
	writePtr(c.sp, s.ptr(j))
	// C callout
	_, err := c.run(s.compar)
	if err != nil {
		panic(err)
	}

	// Pop result
	r := readI32(c.sp)
	c.sp += i32StackSz
	return r < 0
}

func (s *sorter) Swap(i, j int) {
	p := s.ptr(i)
	q := s.ptr(j)
	switch size := s.size; size {
	case 1:
		a := readI8(p)
		writeI8(p, readI8(q))
		writeI8(q, a)
	case 2:
		a := readI16(p)
		writeI16(p, readI16(q))
		writeI16(q, a)
	case 4:
		a := readI32(p)
		writeI32(p, readI32(q))
		writeI32(q, a)
	case 8:
		a := readI64(p)
		writeI64(p, readI64(q))
		writeI64(q, a)
	default:
		buf := s.c.sp - uintptr(size)
		movemem(buf, p, size)
		movemem(p, q, size)
		movemem(q, buf, size)
	}
}

// char *getenv(const char *name);
func (c *cpu) getenv() {
	name := GoString(readPtr(c.sp))
	v := os.Getenv(name)
	var p uintptr
	if v != "" {
		p = c.m.CString(v) //TODO memory leak
	}
	writePtr(c.rp, p)
}

// void qsort(void *base, size_t nmemb, size_t size, int (*compar)(const void *, const void *));
func (c *cpu) qsort() {
	sp, compar := popPtr(c.sp)
	sp, size := popLong(sp)
	sp, nmemb := popLong(sp)
	base := readPtr(sp)
	if size > mathutil.MaxInt {
		panic("size overflow")
	}

	if nmemb > mathutil.MaxInt {
		panic("nmemb overflow")
	}

	s := &sorter{c, base, int(nmemb), int(size), compar - ffiProlog}
	ip := c.ip
	sort.Sort(s)
	c.ip = ip
}

// void *realloc(void *ptr, size_t size);
func (c *cpu) realloc() {
	sp, size := popLong(c.sp)
	ptr := readPtr(sp)
	r := c.m.realloc(ptr, int(size))
	if strace {
		fmt.Fprintf(os.Stderr, "realloc(%#x, %#x) %#x\n", ptr, size, r)
	}
	if size != 0 && r == 0 {
		c.setErrno(errno.XENOMEM)
	}
	writePtr(c.rp, r)
}
