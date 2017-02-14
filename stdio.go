// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"io"

	"github.com/cznic/internal/buffer"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_printf"):  printf,
		dict.SID("__builtin_sprintf"): sprintf,
	})
}

func (c *cpu) fprintf0(w io.Writer, format, argp uintptr) int32 {
	var b buffer.Bytes
	written := 0
	for {
		ch := c.readI8(format)
		format++
		switch ch {
		case 0:
			n, err := b.WriteTo(w)
			b.Close()
			if err != nil {
				return -1
			}

			return int32(written) + int32(n)
		case '%':
			modifiers := ""
		more:
			ch := c.readI8(format)
			format++
			switch ch {
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.':
				modifiers += string(ch)
				goto more
			case 'c':
				argp -= i32StackSz
				arg := c.readI32(argp)
				n, _ := fmt.Fprintf(&b, fmt.Sprintf("%%%sc", modifiers), arg)
				written += n
			case 'd', 'i':
				argp -= i32StackSz
				arg := c.readI32(argp)
				n, _ := fmt.Fprintf(&b, fmt.Sprintf("%%%sd", modifiers), arg)
				written += n
			case 'f':
				argp -= f64StackSz
				arg := c.readF64(argp)
				n, _ := fmt.Fprintf(&b, fmt.Sprintf("%%%sf", modifiers), arg)
				written += n
			case 's':
				argp -= ptrStackSz
				arg := c.readPtr(argp)
				if arg == 0 {
					break
				}

				var b2 buffer.Bytes
				for {
					c := c.readI8(arg)
					arg++
					if c == 0 {
						n, _ := fmt.Fprintf(&b, fmt.Sprintf("%%%ss", modifiers), b2.Bytes())
						b2.Close()
						written += n
						break
					}

					b2.WriteByte(byte(c))
				}
			default:
				panic(fmt.Errorf("TODO %q", "%"+string(ch)))
			}
		default:
			b.WriteByte(byte(ch))
			written++
			if ch == '\n' {
				if _, err := b.WriteTo(w); err != nil {
					b.Close()
					return -1
				}
				b.Reset()
			}
		}
	}
}

// int printf(const char *format, ...);
func (c *cpu) printf() {
	c.writeI32(c.rp, c.fprintf0(c.m.stdout, c.readPtr(c.rp-ptrStackSz), c.rp-ptrStackSz))
}

type memWriter struct {
	p uintptr
	c *cpu
}

func (m *memWriter) Write(p []byte) (int, error) {
	for _, v := range p {
		m.c.writeU8(m.p, v)
		m.p++
	}
	return len(p), nil
}

// int sprintf(char *str, const char *format, ...);
func (c *cpu) sprintf() {
	ap := c.rp - ptrStackSz
	w := memWriter{c.readPtr(ap), c}
	ap -= ptrStackSz
	c.writeI32(c.rp, c.fprintf0(&w, c.readPtr(ap), ap))
	c.writeI8(w.p, 0)
}
