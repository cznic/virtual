// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"math"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_sinh"):  sinh,
		dict.SID("__builtin_cosh"):  cosh,
		dict.SID("__builtin_tanh"):  tanh,
		dict.SID("__builtin_sin"):   sin,
		dict.SID("__builtin_cos"):   cos,
		dict.SID("__builtin_tan"):   tan,
		dict.SID("__builtin_asin"):  asin,
		dict.SID("__builtin_acos"):  acos,
		dict.SID("__builtin_atan"):  atan,
		dict.SID("__builtin_exp"):   exp,
		dict.SID("__builtin_fabs"):  fabs,
		dict.SID("__builtin_log"):   log,
		dict.SID("__builtin_log10"): log10,
		dict.SID("__builtin_pow"):   pow,
		dict.SID("__builtin_sqrt"):  sqrt,
		dict.SID("__builtin_round"): round,
		dict.SID("__builtin_ceil"):  ceil,
		dict.SID("__builtin_floor"): floor,
	})
}

func (c *cpu) sinh()  { c.writeF64(c.rp, math.Sinh(c.readF64(c.sp))) }
func (c *cpu) cosh()  { c.writeF64(c.rp, math.Cosh(c.readF64(c.sp))) }
func (c *cpu) tanh()  { c.writeF64(c.rp, math.Tanh(c.readF64(c.sp))) }
func (c *cpu) sin()   { c.writeF64(c.rp, math.Sin(c.readF64(c.sp))) }
func (c *cpu) cos()   { c.writeF64(c.rp, math.Cos(c.readF64(c.sp))) }
func (c *cpu) tan()   { c.writeF64(c.rp, math.Tan(c.readF64(c.sp))) }
func (c *cpu) asin()  { c.writeF64(c.rp, math.Asin(c.readF64(c.sp))) }
func (c *cpu) acos()  { c.writeF64(c.rp, math.Acos(c.readF64(c.sp))) }
func (c *cpu) atan()  { c.writeF64(c.rp, math.Atan(c.readF64(c.sp))) }
func (c *cpu) exp()   { c.writeF64(c.rp, math.Exp(c.readF64(c.sp))) }
func (c *cpu) fabs()  { c.writeF64(c.rp, math.Abs(c.readF64(c.sp))) }
func (c *cpu) log()   { c.writeF64(c.rp, math.Log(c.readF64(c.sp))) }
func (c *cpu) log10() { c.writeF64(c.rp, math.Log10(c.readF64(c.sp))) }
func (c *cpu) pow()   { c.writeF64(c.rp, math.Pow(c.readF64(c.sp+f64StackSz), c.readF64(c.sp))) }
func (c *cpu) sqrt()  { c.writeF64(c.rp, math.Sqrt(c.readF64(c.sp))) }
func (c *cpu) ceil()  { c.writeF64(c.rp, math.Ceil(c.readF64(c.sp))) }
func (c *cpu) floor() { c.writeF64(c.rp, math.Floor(c.readF64(c.sp))) }

func (c *cpu) round() {
	v := c.readF64(c.sp)
	switch {
	case v < 0:
		v = math.Ceil(v - 0.5)
	case v > 0:
		v = math.Floor(v + 0.5)
	}
	c.writeF64(c.rp, v)
}
