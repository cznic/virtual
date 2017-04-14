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
		dict.SID("fchmod"):  fchmod,
		dict.SID("fstat64"): fstat64,
		dict.SID("lstat64"): lstat64,
		dict.SID("mkdir"):   mkdir,
		dict.SID("stat64"):  stat64,
	})
}

// int fstat64(int fildes, struct stat64 *buf);
func (c *cpu) fstat64() {
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

// extern int lstat64(char *__file, struct stat64 *__buf);
func (c *cpu) lstat64() {
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

// extern int stat64(char *__file, struct stat64 *__buf);
func (c *cpu) stat64() {
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
