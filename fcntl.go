// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"os"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("fcntl"): fcntl,
		dict.SID("lstat"): lstat,
		dict.SID("open"):  open,
	})
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
