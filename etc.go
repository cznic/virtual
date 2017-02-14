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
	const width = 13
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
		case // default format
			AddPtr,
			Call,
			Copy,
			Ext,
			Float32,
			Float64,
			IndexI32,
			Int32,
			Int64,
			Jmp,
			Jnz,
			Jz,
			Load32,
			Load64,
			Load8,
			PostIncI32,
			PostIncPtr:

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
			AddPtrs,
			AddF64,
			AddI32,
			And32,
			Arguments,
			BoolI64,
			ConvF32F64,
			ConvF64F32,
			ConvF64I32,
			ConvF64I8,
			ConvI32F32,
			ConvI32F64,
			ConvI32I64,
			ConvI32I8,
			ConvI64I32,
			ConvI8I32,
			DivF64,
			DivI32,
			DivU64,
			Dup32,
			Dup64,
			EqI32,
			EqI64,
			GeqI32,
			GtI32,
			LeqI32,
			LtI32,
			MulF64,
			MulI32,
			NeqI32,
			NeqI64,
			Or32,
			Panic,
			RemU64,
			Return,
			Store32,
			Store64,
			Store8,
			SubF64,
			SubI32,
			Xor32,
			Zero32,
			Zero64,

			abort,
			acos,
			asin,
			atan,
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
			fread,
			fwrite,
			log,
			log10,
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
			tanh:

			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s\n", start+i, width, lo); err != nil {
				return err
			}
		case Argument8:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ap%+#x)\n", start+i, width, "push8", op.N); err != nil {
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
		case Variable32:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(bp%+#x)\n", start+i, width, "push32", op.N); err != nil {
				return err
			}
		case Variable64:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(bp%+#x)\n", start+i, width, "push64", op.N); err != nil {
				return err
			}
		default:
			panic(fmt.Errorf("%#05x\t\t%-*s%#x\n", start+i, width, lo, op.N))
		}
	}
	return nil
}
