// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/cznic/internal/buffer"
	"github.com/cznic/ir"
)

var (
	idInt32  = ir.TypeID(dict.SID("int32"))
	idInt32P = ir.TypeID(dict.SID("*int32"))
	idInt64  = ir.TypeID(dict.SID("int64"))
	idInt8P  = ir.TypeID(dict.SID("*int8"))
	idStart  = ir.NameID(dict.SID("_start"))
	idUint32 = ir.TypeID(dict.SID("uint32"))
	idUint64 = ir.TypeID(dict.SID("uint64"))
	idVoidP  = ir.TypeID(dict.SID("*struct{}"))
)

// KillError is the error returned by the CPU of a killed machine.
type KillError struct{}

// Error implements error.
func (e KillError) Error() string { return "SIGKILL" }

// if n%m != 0 { n += m-n%m }. m must be a power of 2.
func roundup(n, m int) int { return (n + m - 1) &^ (m - 1) }

// if n%m != 0 { n += m-n%m }. m must be a power of 2.
func roundupP(n, m uintptr) uintptr { return (n + m - 1) &^ (m - 1) }

// if n%m != 0 { n += m-n%m }. m must be a power of 2.
func roundupULong(n, m uint64) uint64 { return (n + m - 1) &^ (m - 1) }

// DumpCode outputs code to w, assuming it is located at start.
func DumpCode(w io.Writer, code []Operation, start int, funcs, lines []PCInfo) error {
	return dumpCode(w, code, 0, funcs, lines)
}

// DumpCodeStr is like DumpCode but it returns a buffer.Bytes instead. Recycle
// the result using its Close method.
func DumpCodeStr(code []Operation, start int, funcs, lines []PCInfo) buffer.Bytes {
	var buf buffer.Bytes
	dumpCode(&buf, code, start, funcs, lines)
	s := bytes.Replace(buf.Bytes(), []byte("\n\n"), []byte("\n"), -1)
	buf.Close()
	buf.Write(s)
	return buf
}

func dumpCodeStr(code []Operation, start int, funcs, lines []PCInfo) []byte {
	var buf buffer.Bytes
	dumpCode(&buf, code, start, funcs, lines)
	return bytes.Replace(buf.Bytes(), []byte("\n\n"), []byte("\n"), -1)
}

func pcInfo(pc int, infos []PCInfo) *PCInfo {
	if i := sort.Search(len(infos), func(i int) bool { return infos[i].PC >= pc }); len(infos) != 0 && i <= len(infos) {
		switch {
		case i == len(infos):
			return &infos[i-1]
		default:
			if pc == infos[i].PC {
				return &infos[i]
			}

			if i > 0 {
				return &infos[i-1]
			}
		}
	}
	return &PCInfo{}
}

func dumpCode(w io.Writer, code []Operation, start int, funcs, lines []PCInfo) error {
	const width = 15
	for i, op := range code {
		pos := pcInfo(start+i, lines).Position()
		lo := strings.ToLower(op.Opcode.String())
		switch op.Opcode {
		case AP:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sap\t; %v\n", start+i, width, "push", pos); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sap%+#x\t; %v\n", start+i, width, "push", op.N, pos); err != nil {
					return err
				}
			}
		case AddSP:
			switch {
			case op.N > 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*ssp, %#x\t; %v\n", start+i, width, "add", op.N, pos); err != nil {
					return err
				}
			case op.N < 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*ssp, %#x\t; %v\n", start+i, width, "sub", -op.N, pos); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*ssp, %#x\t//TODO optimize\t; %v\n", start+i, width, "add", op.N, pos); err != nil { //TODOOK
					return err
				}
			}
		case BP:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sbp\t; %v\n", start+i, width, "push", pos); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sbp%+#x\t; %v\n", start+i, width, "push", op.N, pos); err != nil {
					return err
				}
			}
		case DS:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sds\t; %v\n", start+i, width, "push", pos); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sds%+#x\t; %v\n", start+i, width, "push", op.N, pos); err != nil {
					return err
				}
			}
		case DSN:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds)\t; %v\n", start+i, width, "push", pos); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds%+#x)\t; %v\n", start+i, width, "push", op.N, pos); err != nil {
					return err
				}
			}
		case DSI8:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds)\t; %v\n", start+i, width, "push8", pos); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds%+#x)\t; %v\n", start+i, width, "push8", op.N, pos); err != nil {
					return err
				}
			}
		case DSI16:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds)\t; %v\n", start+i, width, "push16", pos); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds%+#x)\t; %v\n", start+i, width, "push16", op.N, pos); err != nil {
					return err
				}
			}
		case DSI32:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds)\t; %v\n", start+i, width, "push32", pos); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds%+#x)\t; %v\n", start+i, width, "push32", op.N, pos); err != nil {
					return err
				}
			}
		case DSI64:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds)\t; %v\n", start+i, width, "push64", pos); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds%+#x)\t; %v\n", start+i, width, "push64", op.N, pos); err != nil {
					return err
				}
			}
		case DSC128:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds)\t; %v\n", start+i, width, "pushc128", pos); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ds%+#x)\t; %v\n", start+i, width, "pushc128", op.N, pos); err != nil {
					return err
				}
			}
		case // default format
			AddPtr,
			BitfieldI8,
			BitfieldI16,
			BitfieldI32,
			BitfieldI64,
			BitfieldU16,
			BitfieldU32,
			BitfieldU64,
			BitfieldU8,
			Call,
			ConvI64,
			Copy,
			Ext,
			FP,
			Field16,
			Field64,
			Field8,
			IndexI16,
			IndexU16,
			IndexI32,
			IndexI64,
			IndexI8,
			IndexU32,
			IndexU64,
			IndexU8,
			Load,
			Load16,
			Load32,
			Load64,
			Load8,
			NegIndexI32,
			NegIndexI64,
			NegIndexU16,
			NegIndexU32,
			NegIndexU64,
			Nop,
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
			Push8,
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

			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s%#x\t; %v\n", start+i, width, lo, uint(op.N), pos); err != nil {
				return err
			}
		case Label:
			switch {
			case op.N < 0:
				if _, err := fmt.Fprintf(w, "%#05x\t%v:\t; %v\n", start+i, ir.NameID(-op.N), pos); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t%v:\t; %v\n", start+i, op.N, pos); err != nil {
					return err
				}
			}
		case JmpP:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(sp)\t; %v\n\n", start+i, width, "jmp", pos); err != nil {
				return err
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
			ConvI8F64,
			ConvI8U32,
			ConvU16I32,
			ConvU16I64,
			ConvU16U32,
			ConvU16U64,
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
			DivC128,
			DivC64,
			DivF32,
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
			GeqI32,
			GeqI64,
			GeqI8,
			GeqU32,
			GeqU64,
			GtF32,
			GtF64,
			GtI32,
			GtI64,
			GtU32,
			GtU64,
			LeqF32,
			LeqF64,
			LeqI32,
			LeqI64,
			LeqI8,
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
			MulC128,
			MulC64,
			MulF32,
			MulF64,
			MulI32,
			MulI64,
			NegF32,
			NegF64,
			NegI16,
			NegI32,
			NegI64,
			NegI8,
			NeqC128,
			NeqC64,
			NeqF32,
			NeqF64,
			NeqI32,
			NeqI64,
			NeqI8,
			Not,
			Or32,
			Or64,
			Panic,
			RemI32,
			RemI64,
			RemU32,
			RemU64,
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
			Zero64:

			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s\t; %v\n", start+i, width, lo, pos); err != nil {
				return err
			}
		case Argument:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ap%+#x)\t; %v\n", start+i, width, "push", op.N, pos); err != nil {
				return err
			}
		case Argument8:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ap%+#x)\t; %v\n", start+i, width, "push8", op.N, pos); err != nil {
				return err
			}
		case Argument16:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ap%+#x)\t; %v\n", start+i, width, "push16", op.N, pos); err != nil {
				return err
			}
		case Argument32:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ap%+#x)\t; %v\n", start+i, width, "push32", op.N, pos); err != nil {
				return err
			}
		case Argument64:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(ap%+#x)\t; %v\n", start+i, width, "push64", op.N, pos); err != nil {
				return err
			}
		case builtin:
			if i != 0 {
				fmt.Fprintln(w)
			}
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s\t; %v\n", start+i, width, lo, pos); err != nil {
				return err
			}
		case exit, abort:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s\t; %v\n\n\n", start+i, width, "#"+lo, pos); err != nil {
				return err
			}
		case FFIReturn, Return:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s\t; %v\n\n\n", start+i, width, lo, pos); err != nil {
				return err
			}
		case Jmp, Jz, Jnz:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s%#x\t; %v\n\n\n", start+i, width, lo, uint(op.N), pos); err != nil {
				return err
			}
		case Func:
			if i != 0 {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "# %v\n", pcInfo(start+i, funcs).Name)
			switch {
			case op.N != 0:
				if _, err := fmt.Fprintf(w, "%#05x\t%s\t%-*s[%#x]byte\t; %v\n", start+i, lo, width, "variables", -op.N, pos); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t%s\t; %v\n", start+i, lo, pos); err != nil {
					return err
				}
			}
		case SwitchI32, SwitchI64:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sds%+#x\t; %v\n", start+i, width, lo, op.N, pos); err != nil {
				return err
			}
		case Text:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sts%+#x\t; %v\n", start+i, width, "push", op.N, pos); err != nil {
				return err
			}
		case Variable8:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(bp%+#x)\t; %v\n", start+i, width, "push8", op.N, pos); err != nil {
				return err
			}
		case Variable16:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(bp%+#x)\t; %v\n", start+i, width, "push16", op.N, pos); err != nil {
				return err
			}
		case Variable32:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(bp%+#x)\t; %v\n", start+i, width, "push32", op.N, pos); err != nil {
				return err
			}
		case Variable64:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(bp%+#x)\t; %v\n", start+i, width, "push64", op.N, pos); err != nil {
				return err
			}
		case Variable:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(bp%+#x)\t; %v\n", start+i, width, "push", op.N, pos); err != nil {
				return err
			}
		default:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s\t; %v\n", start+i, width, "#"+lo, pos); err != nil {
				return err
			}
		}
	}
	return nil
}

type switchPair struct {
	ir.Value
	*ir.Label
}

type switchPairs []switchPair

func (s switchPairs) Len() int { return len(s) }

func (s switchPairs) Less(i, j int) bool {
	switch x := s[i].Value.(type) {
	case *ir.Int32Value:
		return x.Value < s[j].Value.(*ir.Int32Value).Value
	case *ir.Int64Value:
		return x.Value < s[j].Value.(*ir.Int64Value).Value
	default:
		panic(fmt.Errorf("%T", x))
	}
}

func (s switchPairs) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
