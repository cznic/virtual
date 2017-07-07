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
		dict.SID("gettimeofday"): gettimeofday,
		dict.SID("utimes"):       utimes,
	})
}

// int gettimeofday(struct timeval *restrict tp, void *restrict tzp);
func (c *cpu) gettimeofday() {
	sp, tzp := popPtr(c.sp)
	tp := readPtr(sp)
	r, _, err := syscall.Syscall(syscall.SYS_GETTIMEOFDAY, tzp, tp, 0)
	if strace {
		fmt.Fprintf(os.Stderr, "gettimeofday(%#x, %#x) %v %v\n", tzp, tp, r, err)
	}
	writeI32(c.rp, int32(r))
}
