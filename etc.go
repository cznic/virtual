// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"io"
	"strings"

	"github.com/cznic/internal/buffer"
	"github.com/cznic/ir"
)

var (
	idInt32P = ir.TypeID(dict.SID("*int32"))
	idInt8P  = ir.TypeID(dict.SID("*int8"))
	idUint8P = ir.TypeID(dict.SID("*uint8"))
)

// KillError is the error returned by the CPU of a killed machine.
type KillError struct{}

// Error implements error.
func (e KillError) Error() string { return "SIGKILL" }

// if n%m != 0 { n += m-n%m }. m must be a power of 2.
func roundup(n, m int) int { return (n + m - 1) &^ (m - 1) }

// if n%m != 0 { n += m-n%m }. m must be a power of 2.
func roundupULong(n, m uint64) uint64 { return (n + m - 1) &^ (m - 1) }

// DumpCode outputs code to w, assuming it is located at start.
func DumpCode(w io.Writer, code []Operation, start int) error {
	return dumpCode(w, code, 0)
}

// DumpCodeStr is like DumpCode but it returns a buffer.Bytes instead. Recycle
// the result using its Close method.
func DumpCodeStr(code []Operation, start int) buffer.Bytes {
	var buf buffer.Bytes
	dumpCode(&buf, code, start)
	return buf
}

func dumpCodeStr(code []Operation, start int) []byte {
	var buf buffer.Bytes
	dumpCode(&buf, code, start)
	return buf.Bytes()
}

func dumpCode(w io.Writer, code []Operation, start int) error {
	const width = 15
	for i, op := range code {
		lo := strings.ToLower(op.Opcode.String())
		switch op.Opcode {
		case AP:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sap\n", start+i, width, "push"); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sap%+#x\n", start+i, width, "push", op.N); err != nil {
					return err
				}
			}
		case AddSP:
			switch {
			case op.N > 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*ssp, %#x\n", start+i, width, "add", op.N); err != nil {
					return err
				}
			case op.N < 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*ssp, %#x\n", start+i, width, "sub", -op.N); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*ssp, %#x\t//TODO optimize\n", start+i, width, "add", op.N); err != nil { //TODOOK
					return err
				}
			}
		case BP:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sbp\n", start+i, width, "push"); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sbp%+#x\n", start+i, width, "push", op.N); err != nil {
					return err
				}
			}
		case DS:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sds\n", start+i, width, "push"); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sds%+#x\n", start+i, width, "push", op.N); err != nil {
					return err
				}
			}
		case DSN:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds)\n", start+i, width, "push"); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds%+#x)\n", start+i, width, "push", op.N); err != nil {
					return err
				}
			}
		case DSI8:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds)\n", start+i, width, "push8"); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds%+#x)\n", start+i, width, "push8", op.N); err != nil {
					return err
				}
			}
		case DSI16:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds)\n", start+i, width, "push16"); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds%+#x)\n", start+i, width, "push16", op.N); err != nil {
					return err
				}
			}
		case DSI32:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds)\n", start+i, width, "push32"); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds%+#x)\n", start+i, width, "push32", op.N); err != nil {
					return err
				}
			}
		case DSI64:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds)\n", start+i, width, "push64"); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds%+#x)\n", start+i, width, "push64", op.N); err != nil {
					return err
				}
			}
		case DSC128:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds)\n", start+i, width, "pushC128"); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds%+#x)\n", start+i, width, "pushC128", op.N); err != nil {
					return err
				}
			}
		case // default format
			AddPtr,
			BitfieldI8,
			BitfieldI16,
			BitfieldI32,
			BitfieldI64,
			BitfieldU8,
			BitfieldU16,
			BitfieldU32,
			BitfieldU64,
			Call,
			Copy,
			Ext,
			FP,
			Field8,
			Field16,
			Field64,
			IndexI16,
			IndexI32,
			IndexI64,
			IndexU32,
			IndexU64,
			IndexI8,
			IndexU8,
			Jmp,
			Jnz,
			Jz,
			Load,
			Load16,
			Load32,
			Load64,
			Load8,
			NegIndexU32,
			NegIndexI32,
			NegIndexI64,
			NegIndexU64,
			PostIncF64,
			PostIncI16,
			PostIncI32,
			PostIncI64,
			PostIncI8,
			PostIncPtr,
			PostIncU32Bits,
			PostIncU64Bits,
			PreIncI16,
			PreIncI32,
			PreIncI64,
			PreIncI8,
			PreIncPtr,
			PreIncU32Bits,
			PreIncU64Bits,
			PtrDiff,
			Push16,
			Push32,
			Push64,
			PushC128,
			Store,
			StoreBits16,
			StoreBits32,
			StoreBits64,
			StoreBits8,
			StrNCopy:

			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s%#x\n", start+i, width, lo, uint(op.N)); err != nil {
				return err
			}
		case Label:
			switch {
			case op.N < 0:
				if _, err := fmt.Fprintf(w, "%#05x\t%v:\n", start+i, ir.NameID(-op.N)); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t%v:\n", start+i, op.N); err != nil {
					return err
				}
			}
		case // no N
			AddF32,
			AddF64,
			AddC64,
			AddC128,
			AddI32,
			AddI64,
			AddPtrs,
			And16,
			And32,
			And64,
			And8,
			Arguments,
			ArgumentsFP,
			BoolC128,
			BoolF32,
			BoolF64,
			BoolI16,
			BoolI32,
			BoolI64,
			BoolI8,
			CallFP,
			ConvC64C128,
			ConvF32C64,
			ConvF32C128,
			ConvF32F64,
			ConvF32I32,
			ConvF32I64,
			ConvF32U32,
			ConvF64F32,
			ConvF64C128,
			ConvF64I32,
			ConvF64I64,
			ConvF64I8,
			ConvF64U16,
			ConvF64U32,
			ConvF64U64,
			ConvI16I32,
			ConvI16I64,
			ConvI16U32,
			ConvI32C128,
			ConvI32C64,
			ConvI32F32,
			ConvI32F64,
			ConvI32I16,
			ConvI32I64,
			ConvI32I8,
			ConvI64F64,
			ConvI64I16,
			ConvI64I32,
			ConvI64I8,
			ConvI64U16,
			ConvI8I16,
			ConvI8I32,
			ConvI8I64,
			ConvI8U32,
			ConvU16I32,
			ConvU16I64,
			ConvU16U32,
			ConvU32F32,
			ConvU32F64,
			ConvU32I16,
			ConvU32I64,
			ConvU32U8,
			ConvU8I16,
			ConvU8I32,
			ConvU8U32,
			ConvU8U64,
			Cpl32,
			Cpl64,
			Cpl8,
			DivF32,
			DivC64,
			DivC128,
			DivF64,
			DivI32,
			DivI64,
			DivU32,
			DivU64,
			Dup32,
			Dup64,
			Dup8,
			EqF32,
			EqF64,
			EqI32,
			EqI64,
			EqI8,
			GeqF32,
			GeqF64,
			GeqI8,
			GeqI32,
			GeqI64,
			GeqU32,
			GeqU64,
			GtF32,
			GtF64,
			GtI32,
			GtI64,
			GtU32,
			GtU64,
			JmpP,
			LeqF32,
			LeqF64,
			LeqI8,
			LeqI32,
			LeqI64,
			LeqU32,
			LeqU64,
			LshI16,
			LshI32,
			LshI64,
			LshI8,
			LtF32,
			LtF64,
			LtI32,
			LtI64,
			LtU32,
			LtU64,
			MulF32,
			MulC64,
			MulC128,
			MulF64,
			MulI32,
			MulI64,
			NegF32,
			NegF64,
			NegI8,
			NegI16,
			NegI32,
			NegI64,
			NeqC64,
			NeqC128,
			NeqF32,
			NeqF64,
			NeqI8,
			NeqI32,
			NeqI64,
			Not,
			Or32,
			Or64,
			Panic,
			RemI32,
			RemU32,
			RemI64,
			RemU64,
			Return,
			RshI16,
			RshI32,
			RshI64,
			RshI8,
			RshU16,
			RshU32,
			RshU64,
			RshU8,
			Store16,
			Store32,
			Store64,
			StoreC128,
			Store8,
			SubF32,
			SubF64,
			SubI32,
			SubI64,
			SubPtrs,
			Xor32,
			Xor64,
			Zero8,
			Zero16,
			Zero32,
			Zero64,

			abort,
			abs,
			acos,
			alloca,
			asin,
			atan,
			bswap64,
			calloc,
			ceil,
			cimagf,
			clrsb,
			clrsbl,
			clrsbll,
			clz,
			clzl,
			clzll,
			copysign,
			cos,
			cosh,
			crealf,
			ctz,
			ctzl,
			ctzll,
			exit,
			exp,
			fabs,
			fclose,
			ffs,
			ffsl,
			ffsll,
			fgetc,
			fgets,
			floor,
			fopen,
			fprintf,
			frameAddress,
			fread,
			free,
			fwrite,
			isinf,
			isinff,
			isinfl,
			isprint,
			log,
			log10,
			malloc,
			memcmp,
			memcpy,
			memset,
			open,
			parity,
			parityl,
			parityll,
			popcount,
			popcountl,
			popcountll,
			pow,
			read,
			printf,
			returnAddress,
			round,
			sign_bit,
			sign_bitf,
			sin,
			sinh,
			sprintf,
			sqrt,
			strcat,
			strchr,
			strcmp,
			strcpy,
			strlen,
			strncmp,
			strncpy,
			strrchr,
			tan,
			tanh,
			tolower,
			vfprintf,
			vprintf,
			write:

			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s\n", start+i, width, lo); err != nil {
				return err
			}
		case Argument:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ap%+#x)\n", start+i, width, "push", op.N); err != nil {
				return err
			}
		case Argument8:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ap%+#x)\n", start+i, width, "push8", op.N); err != nil {
				return err
			}
		case Argument16:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ap%+#x)\n", start+i, width, "push16", op.N); err != nil {
				return err
			}
		case Argument32:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ap%+#x)\n", start+i, width, "push32", op.N); err != nil {
				return err
			}
		case Argument64:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ap%+#x)\n", start+i, width, "push64", op.N); err != nil {
				return err
			}
		case Func:
			if i != 0 {
				fmt.Fprintln(w)
			}
			switch {
			case op.N != 0:
				if _, err := fmt.Fprintf(w, "%#05x\t%s\t%-*s[%#x]byte\n", start+i, lo, width, "variables", -op.N); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t%s\n", start+i, lo); err != nil {
					return err
				}
			}
		case Text:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sts%+#x\n", start+i, width, "push", op.N); err != nil {
				return err
			}
		case Variable8:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(bp%+#x)\n", start+i, width, "push8", op.N); err != nil {
				return err
			}
		case Variable16:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(bp%+#x)\n", start+i, width, "push16", op.N); err != nil {
				return err
			}
		case Variable32:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(bp%+#x)\n", start+i, width, "push32", op.N); err != nil {
				return err
			}
		case Variable64:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(bp%+#x)\n", start+i, width, "push64", op.N); err != nil {
				return err
			}
		case Variable:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(bp%+#x)\n", start+i, width, "push", op.N); err != nil {
				return err
			}
		default:
			panic(fmt.Errorf("%#05x\t\t%-*s%#x", start+i, width, lo, op.N))
		}
	}
	return nil
}
