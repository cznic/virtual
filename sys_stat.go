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
	r, _, err := syscall.Syscall(syscall.SYS_FSTAT, uintptr(fildes), buf, 0)
	if strace {
		fmt.Fprintf(os.Stderr, "fstat(%v, %#x) %v %v\n", fildes, buf, r, err)
	}
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(r))
}

// extern int lstat(char *__file, struct stat *__buf);
func (c *cpu) lstat() {
	sp, buf := popPtr(c.sp)
	file := readPtr(sp)
	r, _, err := syscall.Syscall(syscall.SYS_LSTAT, file, buf, 0)
	if strace {
		fmt.Fprintf(os.Stderr, "lstat(%q, %#x) %v %v\n", GoString(file), buf, r, err)
	}
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(r))
}

// extern int stat(char *__file, struct stat *__buf);
func (c *cpu) stat() {
	sp, buf := popPtr(c.sp)
	file := readPtr(sp)
	r, _, err := syscall.Syscall(syscall.SYS_STAT, file, buf, 0)
	if strace {
		fmt.Fprintf(os.Stderr, "stat(%q, %#x) %v %v\n", GoString(file), buf, r, err)
	}
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(r))
}
