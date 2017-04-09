// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("cimagf"): cimagf,
		dict.SID("crealf"): crealf,
	})
}

// float cimagf(float complex z);
func (c *cpu) cimagf() { writeF32(c.rp, imag(readC64(c.sp))) }

// float crealf(float complex z);
func (c *cpu) crealf() { writeF32(c.rp, real(readC64(c.sp))) }
