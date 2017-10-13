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
		dict.SID("connect"):     connect,
		dict.SID("getpeername"): getpeername,
		dict.SID("getsockname"): getsockname,
		dict.SID("getsockopt"):  getsockopt,
		dict.SID("recv"):        recv,
		dict.SID("setsockopt"):  setsockopt,
		dict.SID("shutdown"):    shutdown,
		dict.SID("socket"):      socket,
		dict.SID("writev"):      writev,
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

// ssize_t writev(int fd, const struct iovec *iov, int iovcnt);
func (c *cpu) writev() {
	sp, iovcnt := popI32(c.sp)
	sp, iov := popPtr(sp)
	fd := readI32(sp)
	n, _, err := syscall.Syscall(syscall.SYS_WRITEV, uintptr(fd), iov, uintptr(iovcnt))
	if err != 0 {
		c.setErrno(err)
		writeLong(c.rp, -1)
		return
	}

	writeLong(c.rp, int64(n))
}
