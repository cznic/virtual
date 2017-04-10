// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"os"
	"syscall"

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
	cmd := readI32(ap)
	ap -= i32StackSz
	arg := readPtr(ap)
	switch fildes {
	case libc.Unistd_STDIN_FILENO:
		panic(fmt.Errorf("TODO30 %v %v", fildes, cmd))
	case libc.Unistd_STDOUT_FILENO:
		panic(fmt.Errorf("TODO30 %v %v", fildes, cmd))
	case libc.Unistd_STDERR_FILENO:
		panic(fmt.Errorf("TODO30 %v %v", fildes, cmd))
	}

	r, _, err := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fildes), uintptr(cmd), arg)
	if r != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(r))
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
