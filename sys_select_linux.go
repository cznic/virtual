// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"syscall"
	"unsafe"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("select"): select_,
	})
}

// int select(int nfds, fd_set *readfds, fd_set *writefds, fd_set *exceptfds, struct timeval *timeout);
func (c *cpu) select_() {
	sp, timeout := popPtr(c.sp)
	sp, exceptfds := popPtr(sp)
	sp, writefds := popPtr(sp)
	sp, readfds := popPtr(sp)
	nfds := readI32(sp)
	n, err := syscall.Select(
		int(nfds),
		(*syscall.FdSet)(unsafe.Pointer(readfds)),
		(*syscall.FdSet)(unsafe.Pointer(writefds)),
		(*syscall.FdSet)(unsafe.Pointer(exceptfds)),
		(*syscall.Timeval)(unsafe.Pointer(timeout)),
	)
	if err != nil {
		c.setErrno(err)
		writeI32(c.rp, -1)
		return
	}

	writeI32(c.rp, int32(n))
}
