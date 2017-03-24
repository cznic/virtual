// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"io"
	"math"
	"syscall"
	"unsafe"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_read"):  read,
		dict.SID("read"):            read,
		dict.SID("__builtin_write"): write,
		dict.SID("write"):           write,
	})
}

// ssize_t read(int fd, void *buf, size_t count);
func (c *cpu) read() {
	sp, count := popLong(c.sp)
	sp, buf := popPtr(sp)
	fd := readI32(sp)
	f := files.fdReader(uintptr(fd), c)
	n, err := f.Read((*[math.MaxInt32]byte)(unsafe.Pointer(buf))[:count])
	if n != 0 {
		writeULong(c.rp, uint64(n))
		return
	}

	if err == io.EOF {
		writeULong(c.rp, 0)
		return
	}

	c.thread.setErrno(err)
	writeI32(c.rp, -1)
}

// ssize_t write(int fd, const void *buf, size_t count);
func (c *cpu) write() {
	sp, count := popLong(c.sp)
	sp, buf := popPtr(sp)
	fd := readI32(sp)
	f := files.fdWriter(uintptr(fd), c)
	n, err := f.Write((*[math.MaxInt32]byte)(unsafe.Pointer(buf))[:count])
	if err != nil {
		c.thread.errno = int32(syscall.EIO)
	}
	writeULong(c.rp, uint64(n))
}
