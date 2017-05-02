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
		dict.SID("mmap64"): mmap64,
		dict.SID("munmap"): munmap,
	})
}

// void *mmap(void *addr, size_t len, int prot, int flags, int fildes, off_t off);
func (c *cpu) mmap64() {
	sp, off := popLong(c.sp)
	sp, fildes := popI32(sp)
	sp, flags := popI32(sp)
	sp, prot := popI32(sp)
	sp, len := popLong(sp)
	addr := readPtr(sp)
	r, _, err := syscall.Syscall6(syscall.SYS_MMAP, addr, uintptr(len), uintptr(prot), uintptr(flags), uintptr(fildes), uintptr(off))
	if strace {
		fmt.Fprintf(os.Stderr, "mmap(%#x, %#x, %#x, %#x, %#x, %#x) (%#x, %v)\n", addr, len, prot, flags, fildes, off, r, err)
	}
	if err != 0 {
		c.setErrno(err)
	}
	writePtr(c.rp, r)
}

// int munmap(void *addr, size_t len);
func (c *cpu) munmap() {
	sp, len := popLong(c.sp)
	addr := readPtr(sp)
	r, _, err := syscall.Syscall(syscall.SYS_MUNMAP, addr, uintptr(len), 0)
	if strace {
		fmt.Fprintf(os.Stderr, "munmap(%#x, %#x) (%#x, %v)\n", addr, len, r, err)
	}
	if err != 0 {
		c.setErrno(err)
	}
	writePtr(c.rp, r)
}
