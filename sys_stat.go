// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"os"
	//"github.com/cznic/ccir/libc"
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
		switch {
		case os.IsNotExist(err):
			panic(fmt.Errorf("TODO 32 %q %#x", file, buf))
			//TODO must set errno
			//TODO writeI32(c.rp, libc.Xerrno_ENOENT)
			//TODO return
		}
	}
	panic(fmt.Errorf("TODO 33 %q %#x", file, buf))
}
