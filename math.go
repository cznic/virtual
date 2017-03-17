// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"math"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_acos"):      acos,
		dict.SID("__builtin_asin"):      asin,
		dict.SID("__builtin_atan"):      atan,
		dict.SID("__builtin_ceil"):      ceil,
		dict.SID("__builtin_copysign"):  copysign,
		dict.SID("__builtin_cos"):       cos,
		dict.SID("__builtin_cosh"):      cosh,
		dict.SID("__builtin_exp"):       exp,
		dict.SID("__builtin_fabs"):      fabs,
		dict.SID("__builtin_floor"):     floor,
		dict.SID("__builtin_isinf"):     isinf,
		dict.SID("__builtin_isinff"):    isinff,
		dict.SID("__builtin_isinfl"):    isinfl,
		dict.SID("__builtin_log"):       log,
		dict.SID("__builtin_log10"):     log10,
		dict.SID("__builtin_pow"):       pow,
		dict.SID("__builtin_round"):     round,
		dict.SID("__builtin_sign_bit"):  sign_bit,
		dict.SID("__builtin_sign_bitf"): sign_bitf,
		dict.SID("__builtin_sin"):       sin,
		dict.SID("__builtin_sinh"):      sinh,
		dict.SID("__builtin_sqrt"):      sqrt,
		dict.SID("__builtin_tan"):       tan,
		dict.SID("__builtin_tanh"):      tanh,
		dict.SID("acos"):                acos,
		dict.SID("asin"):                asin,
		dict.SID("atan"):                atan,
		dict.SID("ceil"):                ceil,
		dict.SID("copysign"):            copysign,
		dict.SID("cos"):                 cos,
		dict.SID("cosh"):                cosh,
		dict.SID("exp"):                 exp,
		dict.SID("fabs"):                fabs,
		dict.SID("floor"):               floor,
		dict.SID("isinf"):               isinf,
		dict.SID("isinff"):              isinff,
		dict.SID("isinfl"):              isinfl,
		dict.SID("log"):                 log,
		dict.SID("log10"):               log10,
		dict.SID("pow"):                 pow,
		dict.SID("round"):               round,
		dict.SID("sin"):                 sin,
		dict.SID("sinh"):                sinh,
		dict.SID("sqrt"):                sqrt,
		dict.SID("tan"):                 tan,
		dict.SID("tanh"):                tanh,
	})
}

func (c *cpu) isinf() {
	var r int32
	if math.IsInf(readF64(c.sp), 0) {
		r = 1
	}
	writeI32(c.rp, r)
}

func (c *cpu) isinff() {
	var r int32
	if math.IsInf(float64(readF32(c.sp)), 0) {
		r = 1
	}
	writeI32(c.rp, r)
}

func (c *cpu) acos()     { writeF64(c.rp, math.Acos(readF64(c.sp))) }
func (c *cpu) asin()     { writeF64(c.rp, math.Asin(readF64(c.sp))) }
func (c *cpu) atan()     { writeF64(c.rp, math.Atan(readF64(c.sp))) }
func (c *cpu) ceil()     { writeF64(c.rp, math.Ceil(readF64(c.sp))) }
func (c *cpu) copysign() { writeF64(c.rp, math.Copysign(readF64(c.sp+f64StackSz), readF64(c.sp))) }
func (c *cpu) cos()      { writeF64(c.rp, math.Cos(readF64(c.sp))) }
func (c *cpu) cosh()     { writeF64(c.rp, math.Cosh(readF64(c.sp))) }
func (c *cpu) exp()      { writeF64(c.rp, math.Exp(readF64(c.sp))) }
func (c *cpu) fabs()     { writeF64(c.rp, math.Abs(readF64(c.sp))) }
func (c *cpu) floor()    { writeF64(c.rp, math.Floor(readF64(c.sp))) }
func (c *cpu) log()      { writeF64(c.rp, math.Log(readF64(c.sp))) }
func (c *cpu) log10()    { writeF64(c.rp, math.Log10(readF64(c.sp))) }
func (c *cpu) pow()      { writeF64(c.rp, math.Pow(readF64(c.sp+f64StackSz), readF64(c.sp))) }
func (c *cpu) sin()      { writeF64(c.rp, math.Sin(readF64(c.sp))) }
func (c *cpu) sinh()     { writeF64(c.rp, math.Sinh(readF64(c.sp))) }
func (c *cpu) sqrt()     { writeF64(c.rp, math.Sqrt(readF64(c.sp))) }
func (c *cpu) tan()      { writeF64(c.rp, math.Tan(readF64(c.sp))) }
func (c *cpu) tanh()     { writeF64(c.rp, math.Tanh(readF64(c.sp))) }

func (c *cpu) sign_bit() {
	var r int32
	if math.Signbit(readF64(c.sp)) {
		r = 1
	}
	writeI32(c.rp, r)
}

func (c *cpu) sign_bitf() {
	var r int32
	if math.Signbit(float64(readF32(c.sp))) {
		r = 1
	}
	writeI32(c.rp, r)
}

func (c *cpu) round() {
	v := readF64(c.sp)
	switch {
	case v < 0:
		v = math.Ceil(v - 0.5)
	case v > 0:
		v = math.Floor(v + 0.5)
	}
	writeF64(c.rp, v)
}
