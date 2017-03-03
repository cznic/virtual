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
	AddF32
	AddF64
	AddI32
	AddI64
	AddPtr // N
	AddPtrs
	AddSP // N
	And32
	And64
	And8
	Argument   // N + ext: size
	Argument16 // N
	Argument32 // N
	Argument64 // N
	Argument8  // N
	Arguments
	ArgumentsFP
	BP // N
	BoolI32
	BoolI64
	BoolI8
	Call // N
	CallFP
	ConvF32F64
	ConvF32I32
	ConvF64F32
	ConvF64I32
	ConvF64I8
	ConvI16I32
	ConvI16U32
	ConvI32F32
	ConvI32C64
	ConvI32F64
	ConvI32I16
	ConvI32I64
	ConvI32I8
	ConvI64I32
	ConvI64U16
	ConvI8I16
	ConvI8I32
	ConvI8I64
	ConvU16I32
	ConvU16I64
	ConvU16U32
	ConvU32I64
	ConvU32U8
	ConvU8I32
	ConvU8U32
	Copy // N
	Cpl64
	DS    // N
	DSI8  // N
	DSI16 // N
	DSI32 // N
	DSI64 // N
	DivF64
	DivI32
	DivU32
	DivI64
	DivU64
	Dup32
	Dup64
	Dup8
	EqI32
	EqI64
	EqI8
	Ext     // N
	Float32 // N
	Float64 // N
	Func    // N
	GeqF64
	GeqI32
	GeqI64
	GeqU32
	GeqU64
	GtF64
	GtI32
	GtI64
	GtU32
	GtU64
	Index    // N
	IndexI16 // N
	IndexI32 // N
	IndexU32 // N
	IndexU64 // N
	Int32    // N
	Int64    // N
	Jmp      // N
	Jnz      // N
	Jz       // N
	Label    // N
	LeqI32
	LeqU32
	LeqU64
	Load   // N + ext: size
	Load16 // N
	Load32 // N
	Load64 // N
	Load8  // N
	LshI8  // N
	LshI16 // N
	LshI32 // N
	LshI64 // N
	LtF64
	LtI32
	LtI64
	LtU32
	LtU64
	MulF32
	MulF64
	MulI32
	MulI64
	NegI32
	NegIndexI32 // N
	NegIndexU64 // N
	NeqC64
	NeqF64
	NeqI32
	NeqI64
	Not
	Or32
	Or64
	Panic
	PostIncF64 // N
	PostIncI32 // N
	PostIncI64 // N
	PostIncI8  // N
	PostIncPtr // N
	PreIncI32  // N
	PreIncI8   // N
	PreIncPtr  // N
	PtrDiff    // N
	RemI32
	RemU32
	RemU64
	Return
	RshI8  // N
	RshU8  // N
	RshU16 // N
	RshI32 // N
	RshU32 // N
	RshI64 // N
	RshU64 // N
	Store  // N
	Store16
	Store32
	Store64
	Store8
	StoreBits8  // N
	StoreBits32 // N
	StrNCopy    // N
	SubF32
	SubF64
	SubI32
	SubI64
	Text       // N
	Variable   // N + ext: size
	Variable16 // N
	Variable32 // N
	Variable64 // N
	Variable8  // N
	Xor32
	Xor64
	Zero32
	Zero64

	// builtins

	abort
	acos
	asin
	atan
	ceil
	cos
	cosh
	exit
	exp
	fabs
	fclose
	fgetc
	fgets
	floor
	fopen
	fprintf
	fread
	fwrite
	log
	log10
	malloc
	memcmp
	memcpy
	memset
	pow
	printf
	round
	sin
	sinh
	sprintf
	sqrt
	strcat
	strchr
	strcmp
	strcpy
	strlen
	strncmp
	strncpy
	strrchr
	tan
	tanh
	tolower
)
