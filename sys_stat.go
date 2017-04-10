// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"os"
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
	file := GoString(readPtr(sp))
	_, err := os.Stat(file)
	if err != nil {
		c.thread.setErrno(err)
		writeI32(c.rp, -1)
		return
	}

	panic(fmt.Errorf("TODO 34 %q %#x", file, buf))
}
