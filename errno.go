// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__errno_location"): errno_location,
	})
}

// extern int *__errno_location(void);
func (c *cpu) errnoLocation() { writePtr(c.rp, c.errno) }
