// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

type Opcode int

const (
	Nop Opcode = iota

	Abort
	AddSP      // N
	Argument32 // N
	Argument64 // N
	Arguments
	BP   // N
	Call // N
	Exit
	Func  // N
	Int32 // N
	Jmp   // N
	Panic
	RP // N
	Return
	Store32
	Text       // N
	Variable32 // N
)
