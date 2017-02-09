// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"io"
	"strings"

	"github.com/cznic/internal/buffer"
)

type KillError struct{}

func (e KillError) Error() string { return "SIGKILL" }

// if n%m != 0 { n += m-n%m }. m must be a power of 2.
func roundup(n, m int) int { return (n + m - 1) &^ (m - 1) }

func DumpCode(w io.Writer, code []Operation, start int) error {
	return dumpCode(w, code, 0)
}

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
	const width = 12
	for i, op := range code {
		lo := strings.ToLower(op.Opcode.String())
		switch op.Opcode {
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
		case Call: // default
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s%#x\n", start+i, width, lo, op.N); err != nil {
				return err
			}
		case // no N
			Abort,
			Arguments,
			Exit,
			Panic,
			Return,
			Store32:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s\n", start+i, width, lo); err != nil {
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
		case Int32:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s%#x\n", start+i, width, "push32", uint(op.N)); err != nil {
				return err
			}
		case RP:
			switch {
			case op.N == 0:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*srp\n", start+i, width, "push"); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%#05x\t\t%-*srp%+#x\n", start+i, width, "push", op.N); err != nil {
					return err
				}
			}
		case Text:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*sts%+#x\n", start+i, width, "push", op.N); err != nil {
				return err
			}
		case Variable32:
			if _, err := fmt.Fprintf(w, "%#05x\t\t%-*s(bp%+#x)\n", start+i, width, "push32", op.N); err != nil {
				return err
			}
		default:
			panic(fmt.Errorf("%#05x\t\t%-*s%#x\n", start+i, width, lo, op.N))
		}
	}
	return nil
}
