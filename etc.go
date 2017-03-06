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

// KillError is the error returned by the CPU of a killed machine.
type KillError struct{}

// Error implements error.
func (e KillError) Error() string { return "SIGKILL" }

// if n%m != 0 { n += m-n%m }. m must be a power of 2.
func roundup(n, m int) int { return (n + m - 1) &^ (m - 1) }

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
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*ssp, %#x\t//TODO optimize\n", start+i, width, "add", op.N); err != nil {
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
		case // default format
			AddPtr,
			Call,
			Copy,
			Ext,
			FP,
			Float32,
			Float64,
			IndexI16,
			IndexI32,
			IndexU32,
			IndexU64,
			IndexU8,
			Int32,
			Int64,
			Jmp,
			Jnz,
			Jz,
			Load,
			Load16,
			Load32,
			Load64,
			Load8,
			NegIndexI32,
			NegIndexU64,
			PostIncF64,
			PostIncI32,
			PostIncI64,
			PostIncI8,
			PostIncPtr,
			PostIncU32Bits,
			PreIncI32,
			PreIncI8,
			PreIncPtr,
			PreIncU32Bits,
			PtrDiff,
			Store,
			StoreBits32,
			StoreBits8,
			StrNCopy:

			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s%#x\n", start+i, width, lo, op.N); err != nil {
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
			AddI32,
			AddI64,
			AddPtrs,
			And32,
			And64,
			And8,
			Arguments,
			ArgumentsFP,
			BoolI16,
			BoolI32,
			BoolI64,
			BoolI8,
			CallFP,
			ConvF32F64,
			ConvF32I32,
			ConvF64F32,
			ConvF64I32,
			ConvF64I64,
			ConvF64U64,
			ConvF64I8,
			ConvI16I32,
			ConvI16I64,
			ConvI16U32,
			ConvI32C64,
			ConvI32F32,
			ConvI32F64,
			ConvI32I16,
			ConvI32I64,
			ConvI32I8,
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
			ConvU32I64,
			ConvU32I16,
			ConvU32U8,
			ConvU8I32,
			ConvU8U32,
			Cpl32,
			Cpl64,
			DivF64,
			DivI32,
			DivI64,
			DivU32,
			DivU64,
			Dup32,
			Dup64,
			Dup8,
			EqI32,
			EqI64,
			EqI8,
			GeqF64,
			GeqI32,
			GeqI64,
			GeqU32,
			GeqU64,
			GtF64,
			GtI32,
			GtI64,
			GtU32,
			GtU64,
			LeqI32,
			LeqI64,
			LeqU32,
			LeqU64,
			LshI16,
			LshI32,
			LshI64,
			LshI8,
			LtF64,
			LtI32,
			LtI64,
			LtU32,
			LtU64,
			MulF32,
			MulF64,
			MulI32,
			MulI64,
			NegF64,
			NegI32,
			NegI64,
			NeqC64,
			NeqF32,
			NeqF64,
			NeqI32,
			NeqI64,
			Not,
			Or32,
			Or64,
			Panic,
			RemI32,
			RemU32,
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
			Store8,
			SubF32,
			SubF64,
			SubI32,
			SubI64,
			Xor32,
			Xor64,
			Zero32,
			Zero64,

			abort,
			abs,
			acos,
			asin,
			atan,
			calloc,
			ceil,
			cos,
			cosh,
			exit,
			exp,
			fabs,
			fclose,
			fgetc,
			fgets,
			floor,
			fopen,
			fprintf,
			fread,
			fwrite,
			log,
			log10,
			malloc,
			memcmp,
			memcpy,
			memset,
			pow,
			printf,
			round,
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
			tolower:

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
