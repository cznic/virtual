// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"reflect"
	"syscall"
	"unsafe"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("recv"): recv,
	})
}

// ssize_t recv(int sockfd, void *buf, size_t len, int flags);
func (c *cpu) recv() {
	sp, flags := popI32(c.sp)
	sp, len := popLong(sp)
	sp, buf := popPtr(sp)
	fd := readI32(sp)
	var b []byte
	h := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	h.Cap = int(len)
	h.Data = buf
	h.Len = int(len)
	n, _, err := syscall.Recvfrom(int(fd), b, int(flags))
	if err != nil {
		c.setErrno(err)
		writeLong(c.rp, -1)
		return
	}

	writeLong(c.rp, int64(n))
}
