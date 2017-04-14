// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"os"
	"syscall"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("fcntl"):  fcntl,
		dict.SID("open"):   open64,
		dict.SID("open64"): open64,
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
	r, _, err := syscall.Syscall(syscall.SYS_FCNTL64, uintptr(fildes), uintptr(cmd), arg)
	if strace {
		fmt.Fprintf(os.Stderr, "fcntl(%v, %v, %#x) %v %v\n", fildes, cmdString(cmd), arg, r, err)
	}
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(r))
}

// int open64(const char *pathname, int flags, ...);
func (c *cpu) open64() {
	ap := c.rp - ptrStackSz
	pathname := readPtr(ap)
	ap -= i32StackSz
	flags := readI32(ap)
	ap -= i32StackSz
	mode := readU32(ap)
	r, _, err := syscall.Syscall(syscall.SYS_OPEN, pathname, uintptr(flags), uintptr(mode))
	if strace {
		fmt.Fprintf(os.Stderr, "open(%q, %v, %#o) %v %v\n", GoString(pathname), modeString(flags), mode, r, err)
	}
	if err != 0 {
		c.thread.setErrno(err)
	}
	writeI32(c.rp, int32(r))
}
