// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build amd64

package virtual

import (
	"math"
	"unsafe"
)

func (c *cpu) pushI64(n, m int) {
	c.sp -= i64StackSz
	writeI64(c.sp, int64(n))
}

func (c *cpu) pushF64(n, m int) {
	c.sp -= f64StackSz
	writeF64(c.sp, math.Float64frombits(uint64(n)))
}

func readSize(p uintptr) uint64     { return *(*uint64)(unsafe.Pointer(p)) }
func writeSize(p uintptr, v uint64) { *(*uint64)(unsafe.Pointer(p)) = v }
