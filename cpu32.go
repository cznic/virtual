// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build 386

package virtual

import (
	"math"
	"unsafe"
)

const (
	model    = 32
	longBits = 32
)

func (c *cpu) push64(n, m int) {
	c.sp -= i64StackSz
	writeU64(c.sp, uint64(uint(m))<<32|uint64(uint(n)))
	c.ip++
}

func (c *cpu) pushC128(n, m int) {
	panic("TODO")
}

func readLong(p uintptr) int64   { return int64(*(*int32)(unsafe.Pointer(p))) }
func readULong(p uintptr) uint64 { return uint64(*(*uint32)(unsafe.Pointer(p))) }

func writeULong(p uintptr, v uint64) {
	if v > math.MaxUint32 {
		panic("size_t overflow")
	}

	*(*uint32)(unsafe.Pointer(p)) = uint32(v)
}
