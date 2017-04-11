// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"sort"

	"github.com/cznic/mathutil"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("abort"):   abort,
		dict.SID("abs"):     abs,
		dict.SID("calloc"):  calloc,
		dict.SID("exit"):    exit,
		dict.SID("free"):    free,
		dict.SID("getenv"):  getenv,
		dict.SID("malloc"):  malloc,
		dict.SID("qsort"):   qsort,
		dict.SID("realloc"): realloc,
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

// void *calloc(size_t nmemb, size_t size);
func (c *cpu) calloc() {
	sp, size := popLong(c.sp)
	nmemb := readLong(sp)
	hi, lo := mathutil.MulUint128_64(uint64(nmemb), uint64(size))
	var p uintptr
	if hi == 0 || lo <= mathutil.MaxInt {
		p = c.m.calloc(int(lo))
	}

	writePtr(c.rp, p)
}

// void *malloc(size_t size);
func (c *cpu) malloc() {
	size := readLong(c.sp)
	var p uintptr
	if size <= mathutil.MaxInt {
		p = c.m.malloc(int(size))
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
	c.ip = s.compar
	_, err := c.run(c.code)
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
	writePtr(c.rp, c.m.realloc(ptr, int(size)))
}
