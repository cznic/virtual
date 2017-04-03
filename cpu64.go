// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build amd64 amd64p32 arm64 mips64 mips64le mips64p32 mips64p32le ppc64 sparc64

package virtual

import (
	"math"
	"unsafe"
)

const (
	longBits = 64
)

func (c *cpu) push64(n, m int) {
	c.sp -= i64StackSz
	writeI64(c.sp, int64(n))
}

func (c *cpu) pushC128(n, m int) {
	c.sp -= c128StackSz
	writeC128(c.sp, complex(math.Float64frombits(uint64(n)), math.Float64frombits(uint64(m))))
	c.ip++
}

func readLong(p uintptr) int64       { return *(*int64)(unsafe.Pointer(p)) }
func readULong(p uintptr) uint64     { return *(*uint64)(unsafe.Pointer(p)) }
func writeLong(p uintptr, v int64)   { *(*int64)(unsafe.Pointer(p)) = v }
func writeULong(p uintptr, v uint64) { *(*uint64)(unsafe.Pointer(p)) = v }
