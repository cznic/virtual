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
		dict.SID("fchmod"): fchmod,
		dict.SID("fstat"):  fstat,
		dict.SID("lstat"):  lstat,
		dict.SID("mkdir"):  mkdir,
		dict.SID("stat"):   stat,
	})
}

// int fstat(int fildes, struct stat *buf);
func (c *cpu) fstat() {
	sp, buf := popPtr(c.sp)
	fildes := readI32(sp)
	switch fildes {
	case libc.Unistd_STDIN_FILENO:
		panic(fmt.Errorf("TODO30 %v %#x", fildes, buf))
	case libc.Unistd_STDOUT_FILENO:
		panic(fmt.Errorf("TODO30 %v %#x", fildes, buf))
	case libc.Unistd_STDERR_FILENO:
		panic(fmt.Errorf("TODO30 %v %#x", fildes, buf))
	}

	r, _, err := syscall.Syscall(syscall.SYS_FSTAT, uintptr(fildes), buf, 0)
	if r != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(r))
}

// extern int lstat(char *__file, struct stat *__buf);
func (c *cpu) lstat() {
	sp, buf := popPtr(c.sp)
	file := GoString(readPtr(sp))
	_, err := os.Lstat(file)
	if err != nil {
		c.thread.setErrno(err)
		writeI32(c.rp, -1)
		return
	}

	panic(fmt.Errorf("TODO 34 %q %#x", file, buf))
}

// extern int stat(char *__file, struct stat *__buf);
func (c *cpu) stat() {
	sp, buf := popPtr(c.sp)
	file := readPtr(sp)
	r, _, err := syscall.Syscall(syscall.SYS_STAT, file, buf, 0)
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(r))
}
