// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build 386

package virtual

import (
	"math"
	"unsafe"
)

const model = 32

func (c *cpu) pushI64(n, m int) {
	c.sp -= i64StackSz
	writeI64(c.sp, int64(n)|int64(m))
	c.ip++
}

func (c *cpu) pushF64(n, m int) {
	c.sp -= f64StackSz
	writeF64(c.sp, math.Float64frombits(uint64(n)|uint64(m)))
	c.ip++
}

func readSize(p uintptr) { return uint64(*(*uint32)(unsafe.Pointer(p))) }

func writeSize(p uintptr, v uint64) {
	if v > math.MaxUint32 {
		panic("size_t overflow")
	}

	*(*uint32)(unsafe.Pointer(p)) = uint32(v)
}
