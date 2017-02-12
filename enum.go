// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

// Opcode encodes the particular operation.
type Opcode int

// Values of Opcode.
const (
	Nop Opcode = iota

	AP // N
	AddF64
	AddI32
	AddPtr // N
	AddSP  // N
	And32
	Argument8  // N
	Argument32 // N
	Argument64 // N
	Arguments
	BP   // N
	Call // N
	ConvF32F64
	ConvF64F32
	ConvF64I32
	ConvF64I8
	ConvI32F32
	ConvI32F64
	ConvI32I8
	ConvI8I32
	DS // N
	DivF64
	DivI32
	Dup32
	Dup64
	EqI32
	EqI64
	Ext      // N
	Float32  // N
	Float64  // N
	Func     // N
	Index    // N
	IndexI32 // N
	Int32    // N
	Jmp      // N
	Jnz      // N
	Jz       // N
	Label    // N
	LeqI32
	Load32 // N
	Load8  // N
	LtI32
	MulF64
	MulI32
	NeqI32
	NeqI64
	Or32
	Panic
	PostIncI32
	PostIncPtr // N
	Return
	Store32
	Store64
	Store8
	SubF64
	SubI32
	Text       // N
	Variable32 // N
	Variable64 // N
	Variable8  // N
	Xor32
	Zero32
	Zero64

	// builtins

	abort
	exit
	printf
	ceil
	floor
	round
	sqrt
	pow
	log10
	log
	fabs
	exp
	sinh
	cosh
	tanh
	sin
	cos
	tan
	asin
	acos
	atan
)
