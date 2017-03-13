// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build amd64

package virtual

import (
	"unsafe"
)

const (
	model    = 64
	longBits = 64
)

func (c *cpu) push64(n, m int) {
	c.sp -= i64StackSz
	writeI64(c.sp, int64(n))
}

func readLong(p uintptr) int64       { return *(*int64)(unsafe.Pointer(p)) }
func readULong(p uintptr) uint64     { return *(*uint64)(unsafe.Pointer(p)) }
func writeULong(p uintptr, v uint64) { *(*uint64)(unsafe.Pointer(p)) = v }
