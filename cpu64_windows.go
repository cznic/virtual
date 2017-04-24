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
	longBits = 32
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

func readLong(p uintptr) int64   { return int64(*(*int32)(unsafe.Pointer(p))) }
func readULong(p uintptr) uint64 { return uint64(*(*uint32)(unsafe.Pointer(p))) }

func writeLong(p uintptr, v int64) {
	if v < math.MinInt32 || v > math.MaxInt32 {
		panic("size_t overflow")
	}

	*(*int32)(unsafe.Pointer(p)) = int32(v)
}

func writeULong(p uintptr, v uint64) {
	if v > math.MaxUint32 {
		panic("size_t overflow")
	}

	*(*uint32)(unsafe.Pointer(p)) = uint32(v)
}
