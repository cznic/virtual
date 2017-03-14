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
	And16
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
	BP          // N
	BitfieldI8  // N  lshift<<8|rshift
	BitfieldI16 // N  lshift<<8|rshift
	BitfieldI32 // N  lshift<<8|rshift
	BitfieldI64 // N  lshift<<8|rshift
	BitfieldU8  // N: lshift<<8|rshift
	BitfieldU16 // N: lshift<<8|rshift
	BitfieldU32 // N: lshift<<8|rshift
	BitfieldU64 // N: lshift<<8|rshift
	BoolC128
	BoolF32
	BoolF64
	BoolI16
	BoolI32
	BoolI64
	BoolI8
	Call // N
	CallFP
	ConvC64C128
	ConvF32F64
	ConvF32I32
	ConvF32U32
	ConvF64F32
	ConvF64I32
	ConvF64I64
	ConvF64I8
	ConvF64U16
	ConvF64U32
	ConvF64U64
	ConvI16I32
	ConvI16I64
	ConvI16U32
	ConvI32C128
	ConvI32C64
	ConvI32F32
	ConvI32F64
	ConvI32I16
	ConvI32I64
	ConvI32I8
	ConvI64F64
	ConvI64I16
	ConvI64I32
	ConvI64I8
	ConvI64U16
	ConvI8I16
	ConvI8I32
	ConvI8I64
	ConvI8U32
	ConvU16I32
	ConvU16I64
	ConvU16U32
	ConvU32F32
	ConvU32F64
	ConvU32I16
	ConvU32I64
	ConvU32U8
	ConvU8I16
	ConvU8I32
	ConvU8U32
	ConvU8U64
	Copy // N
	Cpl32
	Cpl64
	Cpl8
	DS     // N
	DSC128 // N
	DSI16  // N
	DSI32  // N
	DSI64  // N
	DSI8   // N
	DSN    // N + ext: size
	DivF32
	DivF64
	DivI32
	DivI64
	DivU32
	DivU64
	Dup32
	Dup64
	Dup8
	EqF32
	EqF64
	EqI32
	EqI64
	EqI8
	Ext  // N
	FP   // N
	Func // N
	GeqF32
	GeqF64
	GeqI32
	GeqI64
	GeqU32
	GeqU64
	GtF32
	GtF64
	GtI32
	GtI64
	GtU32
	GtU64
	Index    // N
	IndexI16 // N
	IndexI32 // N
	IndexI64 // N
	IndexU32 // N
	IndexU64 // N
	IndexU8  // N
	Jmp      // N
	JmpP
	Jnz   // N
	Jz    // N
	Label // N
	LeqF32
	LeqF64
	LeqI32
	LeqI64
	LeqU32
	LeqU64
	Load   // N + ext: size
	Load16 // N
	Load32 // N
	Load64 // N
	Load8  // N
	LshI16 // N
	LshI32 // N
	LshI64 // N
	LshI8  // N
	LtF32
	LtF64
	LtI32
	LtI64
	LtU32
	LtU64
	MulC64
	MulF32
	MulF64
	MulI32
	MulI64
	NegF32
	NegF64
	NegI16
	NegI32
	NegI64
	NegI8
	NegIndexI32 // N
	NegIndexI64 // N
	NegIndexU64 // N
	NeqC128
	NeqC64
	NeqF32
	NeqF64
	NeqI32
	NeqI64
	Not
	Or32
	Or64
	Panic
	PostIncF64     // N
	PostIncI16     // N
	PostIncI32     // N
	PostIncI64     // N
	PostIncI8      // N
	PostIncPtr     // N
	PostIncU32Bits // N + ext: bits<<16 | bitoffset<<8 | bitfieldWidth
	PostIncU64Bits // N + ext: bits<<16 | bitoffset<<8 | bitfieldWidth
	PreIncI16      // N
	PreIncI32      // N
	PreIncI64      // N
	PreIncI8       // N
	PreIncPtr      // N
	PreIncU32Bits  // N + ext: bits<<16 | bitoffset<<8 | bitfieldWidth
	PreIncU64Bits  // N + ext: bits<<16 | bitoffset<<8 | bitfieldWidth
	PtrDiff        // N
	Push8          // N
	Push16         // N
	Push32         // N
	Push64         // N
	RemI32
	RemI64
	RemU32
	RemU64
	Return
	RshI16 // N
	RshI32 // N
	RshI64 // N
	RshI8  // N
	RshU16 // N
	RshU32 // N
	RshU64 // N
	RshU8  // N
	Store  // N
	Store16
	Store32
	Store64
	Store8
	StoreBits16 // N
	StoreBits32 // N
	StoreBits64 // N
	StoreBits8  // N
	StrNCopy    // N
	SubF32
	SubF64
	SubI32
	SubI64
	SubPtrs
	Text       // N
	Variable   // N + ext: size
	Variable16 // N
	Variable32 // N
	Variable64 // N
	Variable8  // N
	Xor32
	Xor64
	Zero8
	Zero16
	Zero32
	Zero64

	// builtins

	abort
	abs
	acos
	alloca
	asin
	atan
	calloc
	ceil
	clrsb
	clrsbl
	clrsbll
	clz
	clzl
	clzll
	cos
	cosh
	ctz
	ctzl
	ctzll
	exit
	exp
	fabs
	fclose
	ffs
	ffsl
	ffsll
	fgetc
	fgets
	floor
	fopen
	fprintf
	fread
	free
	fwrite
	isinf
	isinff
	isinfl
	isprint
	log
	log10
	malloc
	memcmp
	memcpy
	memset
	parity
	parityl
	parityll
	popcount
	popcountl
	popcountll
	pow
	printf
	returnAddress
	round
	sign_bit
	sign_bitf
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
	vfprintf
	vprintf
)
