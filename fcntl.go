// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"os"

	"github.com/cznic/ccir/libc"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("fcntl"): fcntl,
		dict.SID("open"):  open,
	})
}

// int fcntl(int fildes, int cmd, ...);
func (c *cpu) fcntl() {
	ap := c.rp - i32StackSz
	fildes := readI32(ap)
	ap -= i32StackSz
	switch cmd := readI32(ap); cmd {
	case libc.Fcntl_F_GETFD:
		f := files.fdReader(uintptr(fildes), c)
		if f == nil {
			c.setErrno(libc.Errno_EBADF)
			writeI32(c.rp, -1)
			return
		}

		panic(fmt.Errorf("TODO35 %v", f.(*os.File).Name()))
	default:
		panic(fmt.Errorf("TODO37 %v %v", fildes, cmd))
	}
}

// int open(const char *pathname, int flags, ...);
func (c *cpu) open() {
	ap := c.rp - ptrStackSz
	pathname := GoString(readPtr(ap))
	ap -= i32StackSz
	flags := readI32(ap)
	f, err := os.OpenFile(pathname, int(flags), 0600)
	if err != nil {
		c.thread.setErrno(err)
		writeI32(c.rp, -1)
		return
	}

	files.add(f, 0)
	writeI32(c.rp, int32(f.Fd()))
}
