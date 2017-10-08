// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"sync"
	"unsafe"

	"github.com/cznic/ccir/libc/errno"
	"github.com/cznic/ccir/libc/stdio"
	"github.com/cznic/internal/buffer"
	"github.com/cznic/mathutil"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__register_stdfiles"): register_stdfiles,
		dict.SID("fclose"):              fclose,
		dict.SID("ferror"):              ferror,
		dict.SID("fflush"):              fflush,
		dict.SID("fgetc"):               fgetc,
		dict.SID("fgets"):               fgets,
		dict.SID("fopen"):               fopen64,
		dict.SID("fopen64"):             fopen64,
		dict.SID("fprintf"):             fprintf,
		dict.SID("fread"):               fread,
		dict.SID("fseek"):               fseek,
		dict.SID("ftell"):               ftell,
		dict.SID("fwrite"):              fwrite,
		dict.SID("printf"):              printf,
		dict.SID("rewind"):              rewind,
		dict.SID("sprintf"):             sprintf,
		dict.SID("vfprintf"):            vfprintf,
		dict.SID("vprintf"):             vprintf,
	})
}

var (
	stdin, stdout, stderr uintptr
)

// void __register_stdfiles(void *, void *, void *);
func (c *cpu) register_stdfiles() {
	var sp uintptr
	sp, stderr = popPtr(c.sp)
	sp, stdout = popPtr(sp)
	stdin = readPtr(sp)
}

type stream struct {
	*os.File
	eof bool
	err error
}

var (
	files = &fmap{
		m: map[uintptr]*stream{},
	}
	nullReader = bytes.NewBuffer(nil)
)

type fmap struct {
	m  map[uintptr]*stream
	mu sync.Mutex
}

func (m *fmap) add(f *os.File, u uintptr) {
	m.mu.Lock()
	m.m[u] = &stream{File: f}
	m.mu.Unlock()
}

func (m *fmap) get(u uintptr) *stream {
	m.mu.Lock()
	r := m.m[u]
	m.mu.Unlock()
	return r
}

func (m *fmap) reader(u uintptr, c *cpu) io.Reader {
	switch u {
	case stdin:
		return c.m.stdin
	case stdout, stderr:
		return nullReader
	}

	m.mu.Lock()
	f := m.m[u]
	m.mu.Unlock()
	return f
}

func (m *fmap) writer(u uintptr, c *cpu) io.Writer {
	switch u {
	case stdin:
		return ioutil.Discard
	case stdout:
		return c.m.stdout
	case stderr:
		return c.m.stderr
	}

	m.mu.Lock()
	f := m.m[u]
	m.mu.Unlock()
	return f
}

func (m *fmap) extract(u uintptr) *os.File {
	m.mu.Lock()
	f := m.m[u]
	delete(m.m, u)
	m.mu.Unlock()
	return f.File
}

type file struct{ _ int32 }

// int fclose(FILE *stream);
func (c *cpu) fclose() {
	u := readPtr(c.sp)
	switch u {
	case stdin, stdout, stderr:
		c.setErrno(errno.XEIO)
		writeI32(c.rp, stdio.XEOF)
		return
	}

	f := files.extract(u)
	if f == nil {
		c.setErrno(errno.XEBADF)
		writeI32(c.rp, stdio.XEOF)
		return
	}

	c.m.free(u)
	if err := f.Close(); err != nil {
		c.setErrno(errno.XEIO)
		writeI32(c.rp, stdio.XEOF)
		return
	}

	writeI32(c.rp, 0)
}

// int ferror(FILE *stream);
func (c *cpu) ferror() {
	u := readPtr(c.sp)
	s := files.get(u)
	var r int32
	switch {
	case s == nil:
		r = -1
		c.setErrno(errno.XEBADF)
	default:
		if s.err != nil {
			r = 1
		}
	}
	writeI32(c.rp, r)
}

// int fgetc(FILE *stream);
func (c *cpu) fgetc() {
	p := buffer.Get(1)
	if _, err := files.reader(readPtr(c.sp), c).Read(*p); err != nil {
		writeI32(c.rp, stdio.XEOF)
		buffer.Put(p)
		return
	}

	writeI32(c.rp, int32((*p)[0]))
	buffer.Put(p)
}

// char *fgets(char *s, int size, FILE *stream);
func (c *cpu) fgets() {
	sp, stream := popPtr(c.sp)
	sp, size := popI32(sp)
	s := readPtr(sp)
	f := files.reader(stream, c)
	p := buffer.Get(1)
	b := *p
	w := memWriter(s)
	ok := false
	for i := int(size) - 1; i > 0; i-- {
		_, err := f.Read(b)
		if err != nil {
			if !ok {
				writePtr(c.rp, 0)
				buffer.Put(p)
				return
			}

			break
		}

		ok = true
		w.WriteByte(b[0])
		if b[0] == '\n' {
			break
		}
	}
	w.WriteByte(0)
	writePtr(c.rp, s)
	buffer.Put(p)

}

// FILE *fopen64(const char *path, const char *mode);
func (c *cpu) fopen64() {
	sp, mode := popPtr(c.sp)
	path := readPtr(sp)
	p := GoString(path)
	var u uintptr
	switch p {
	case os.Stderr.Name():
		u = stderr
	case os.Stdin.Name():
		u = stdin
	case os.Stdout.Name():
		u = stdout
	default:
		var f *os.File
		var err error
		switch mode := GoString(mode); mode {
		case "r":
			if f, err = os.OpenFile(p, os.O_RDONLY, 0666); err != nil {
				switch {
				case os.IsNotExist(err):
					c.setErrno(errno.XENOENT)
				case os.IsPermission(err):
					c.setErrno(errno.XEPERM)
				default:
					c.setErrno(errno.XEACCES)
				}
			}
		case "w":
			if f, err = os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666); err != nil {
				switch {
				case os.IsPermission(err):
					c.setErrno(errno.XEPERM)
				default:
					c.setErrno(errno.XEACCES)
				}
			}
		default:
			panic(mode)
		}
		if f != nil {
			u = c.m.malloc(int(unsafe.Sizeof(file{})))
			files.add(f, u)
		}
	}
	writePtr(c.rp, u)
}

// int fprintf(FILE * stream, const char *format, ...);
func (c *cpu) fprintf() {
	ap := c.rp - ptrStackSz
	stream := readPtr(ap)
	ap -= ptrStackSz
	writeI32(c.rp, goFprintf(files.writer(stream, c), readPtr(ap), ap))
}

// size_t fread(void *ptr, size_t size, size_t nmemb, FILE *stream);
func (c *cpu) fread() {
	sp, stream := popPtr(c.sp)
	sp, nmemb := popLong(sp)
	sp, size := popLong(sp)
	ptr := readPtr(sp)
	hi, lo := mathutil.MulUint128_64(uint64(size), uint64(nmemb))
	if hi != 0 || lo > math.MaxInt32 {
		c.setErrno(errno.XE2BIG)
		writeULong(c.rp, 0)
		return
	}

	n, err := files.reader(stream, c).Read((*[math.MaxInt32]byte)(unsafe.Pointer(ptr))[:lo])
	if err != nil {
		c.setErrno(errno.XEIO)
	}
	writeLong(c.rp, int64(n)/size)
}

// size_t fwrite(const void *ptr, size_t size, size_t nmemb, FILE *stream);
func (c *cpu) fwrite() {
	sp, stream := popPtr(c.sp)
	sp, nmemb := popLong(sp)
	sp, size := popLong(sp)
	ptr := readPtr(sp)
	hi, lo := mathutil.MulUint128_64(uint64(size), uint64(nmemb))
	if hi != 0 || lo > math.MaxInt32 {
		c.setErrno(errno.XE2BIG)
		writeLong(c.rp, 0)
		return
	}

	n, err := files.writer(stream, c).Write((*[math.MaxInt32]byte)(unsafe.Pointer(ptr))[:lo])
	if err != nil {
		c.setErrno(errno.XEIO)
	}
	writeLong(c.rp, int64(n)/size)
}

func goFprintf(w io.Writer, format, argp uintptr) int32 {
	var b buffer.Bytes
	written := 0
	for {
		ch := readI8(format)
		format++
		switch ch {
		case 0:
			_, err := b.WriteTo(w)
			b.Close()
			if err != nil {
				return -1
			}

			return int32(written)
		case '%':
			modifiers := ""
			long := 0
			var w []interface{}
		more:
			ch := readI8(format)
			format++
			switch ch {
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.':
				modifiers += string(ch)
				goto more
			case '*':
				argp -= i32StackSz
				w = append(w, readI32(argp))
				modifiers += string(ch)
				goto more
			case 'c':
				argp -= i32StackSz
				arg := readI32(argp)
				n, _ := fmt.Fprintf(&b, fmt.Sprintf("%%%sc", modifiers), append(w, arg)...)
				written += n
			case 'd', 'i':
				var arg interface{}
				switch long {
				case 0:
					argp -= i32StackSz
					arg = readI32(argp)
				case 1:
					argp -= longStackSz
					arg = readLong(argp)
				default:
					argp -= i64StackSz
					arg = readI64(argp)
				}
				n, _ := fmt.Fprintf(&b, fmt.Sprintf("%%%sd", modifiers), append(w, arg)...)
				written += n
			case 'u':
				var arg interface{}
				switch long {
				case 0:
					argp -= i32StackSz
					arg = readU32(argp)
				case 1:
					argp -= longStackSz
					arg = readULong(argp)
				default:
					argp -= i64StackSz
					arg = readU64(argp)
				}
				n, _ := fmt.Fprintf(&b, fmt.Sprintf("%%%sd", modifiers), append(w, arg)...)
				written += n
			case 'x':
				var arg interface{}
				switch long {
				case 0:
					argp -= i32StackSz
					arg = readU32(argp)
				case 1:
					argp -= longStackSz
					arg = readULong(argp)
				default:
					argp -= i64StackSz
					arg = readU64(argp)
				}
				n, _ := fmt.Fprintf(&b, fmt.Sprintf("%%%sx", modifiers), append(w, arg)...)
				written += n
			case 'l':
				long++
				goto more
			case 'f':
				argp -= f64StackSz
				arg := readF64(argp)
				n, _ := fmt.Fprintf(&b, fmt.Sprintf("%%%sf", modifiers), append(w, arg)...)
				written += n
			case 'p':
				argp -= ptrStackSz
				arg := readPtr(argp)
				n, _ := fmt.Fprintf(&b, fmt.Sprintf("%%%sp", modifiers), append(w, unsafe.Pointer(arg))...)
				written += n
			case 'g':
				argp -= f64StackSz
				arg := readF64(argp)
				n, _ := fmt.Fprintf(&b, fmt.Sprintf("%%%sg", modifiers), append(w, arg)...)
				written += n
			case 's':
				argp -= ptrStackSz
				arg := readPtr(argp)
				if arg == 0 {
					break
				}

				var b2 buffer.Bytes
				for {
					c := readI8(arg)
					arg++
					if c == 0 {
						n, _ := fmt.Fprintf(&b, fmt.Sprintf("%%%ss", modifiers), append(w, b2.Bytes())...)
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
	ap := c.rp - ptrStackSz
	writeI32(c.rp, goFprintf(c.m.stdout, readPtr(ap), ap))
}

// int sprintf(char *str, const char *format, ...);
func (c *cpu) sprintf() {
	ap := c.rp - ptrStackSz
	w := memWriter(readPtr(ap))
	ap -= ptrStackSz
	writeI32(c.rp, goFprintf(&w, readPtr(ap), ap))
	writeI8(uintptr(w), 0)
}

// int vfprintf(FILE *stream, const char *format, va_list ap);
func (c *cpu) vfprintf() {
	sp, ap := popPtr(c.sp)
	sp, format := popPtr(sp)
	stream := readPtr(sp)
	writeI32(c.rp, goFprintf(files.writer(stream, c), format, ap))
}

// int vprintf(const char *format, va_list ap);
func (c *cpu) vprintf() {
	sp, ap := popPtr(c.sp)
	format := readPtr(sp)
	writeI32(c.rp, goFprintf(c.m.stdout, format, ap))
}
