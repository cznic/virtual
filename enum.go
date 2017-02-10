// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

type Opcode int

const (
	Nop Opcode = iota

	AP // N
	AddI32
	AddPtr     // N
	AddSP      // N
	Argument32 // N
	Argument64 // N
	Arguments
	BP   // N
	Call // N
	Dup32
	EqI32
	Func     // N
	Index    // N
	IndexI32 // N
	Int32    // N
	Jmp      // N
	Jnz      // N
	Jz       // N
	Label    // N //TODO-
	LeqI32
	Load32 // N
	LtI32
	MulI32
	Panic
	PostIncI32
	Return
	Store32
	SubI32
	Text       // N
	Variable32 // N

	// builtins

	abort
	exit
	printf
)
