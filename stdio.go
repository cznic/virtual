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

	"syscall"
	"unsafe"

	"github.com/cznic/internal/buffer"
	"github.com/cznic/mathutil"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_fclose"):   fclose,
		dict.SID("__builtin_fgetc"):    fgetc,
		dict.SID("__builtin_fgets"):    fgets,
		dict.SID("__builtin_fopen"):    fopen,
		dict.SID("__builtin_fprintf"):  fprintf,
		dict.SID("__builtin_fread"):    fread,
		dict.SID("__builtin_free"):     free,
		dict.SID("__builtin_fwrite"):   fwrite,
		dict.SID("__builtin_printf"):   printf,
		dict.SID("__builtin_sprintf"):  sprintf,
		dict.SID("__builtin_vfprintf"): vfprintf,
		dict.SID("__builtin_vprintf"):  vprintf,
		dict.SID("fclose"):             fclose,
		dict.SID("fgetc"):              fgetc,
		dict.SID("fgets"):              fgets,
		dict.SID("fopen"):              fopen,
		dict.SID("fprintf"):            fprintf,
		dict.SID("fread"):              fread,
		dict.SID("free"):               free,
		dict.SID("fwrite"):             fwrite,
		dict.SID("printf"):             printf,
		dict.SID("sprintf"):            sprintf,
		dict.SID("vfprintf"):           vfprintf,
		dict.SID("vprintf"):            vprintf,
	})
}

const eof = -1

var (
	files = &fmap{
		fd: map[uintptr]*os.File{},
		m:  map[uintptr]*os.File{},
	}
	nullReader = bytes.NewBuffer(nil)
)

type fmap struct {
	fd map[uintptr]*os.File
	m  map[uintptr]*os.File
	mu sync.Mutex
}

func (m *fmap) add(f *os.File, u uintptr) {
	m.mu.Lock()
	if u != 0 {
		m.m[u] = f
	}
	m.fd[f.Fd()] = f
	m.mu.Unlock()
}

func (m *fmap) reader(u uintptr, c *cpu) io.Reader {
	m.mu.Lock()
	f := m.m[u]
	m.mu.Unlock()
	switch {
	case f == os.Stdin:
		return c.m.stdin
	case f == os.Stdout:
		return nullReader
	case f == os.Stderr:
		return nullReader
	}
	return f
}

func (m *fmap) fdReader(fd uintptr, c *cpu) io.Reader {
	switch fd {
	case 0:
		return c.m.stdin
	case 1:
		return ioutil.NopCloser(&bytes.Buffer{})
	case 2:
		return ioutil.NopCloser(&bytes.Buffer{})
	}

	m.mu.Lock()
	f := m.fd[fd]
	m.mu.Unlock()
	return f
}

func (m *fmap) writer(u uintptr, c *cpu) io.Writer {
	m.mu.Lock()
	f := m.m[u]
	m.mu.Unlock()
	switch {
	case f == os.Stdin:
		return ioutil.Discard
	case f == os.Stdout:
		return c.m.stdout
	case f == os.Stderr:
		return c.m.stderr
	}
	return f
}

func (m *fmap) fdWriter(fd uintptr, c *cpu) io.Writer {
	switch fd {
	case 0:
		return ioutil.Discard
	case 1:
		return c.m.stdout
	case 2:
		return c.m.stderr
	}

	m.mu.Lock()
	f := m.fd[fd]
	m.mu.Unlock()
	return f
}

func (m *fmap) extract(u uintptr) *os.File {
	m.mu.Lock()
	f := m.m[u]
	delete(m.m, u)
	m.mu.Unlock()
	return f
}

func (m *fmap) extractFd(fd uintptr) *os.File {
	m.mu.Lock()
	f := m.fd[fd]
	delete(m.fd, fd)
	m.mu.Unlock()
	return f
}

type file struct{ _ int32 }

// int fclose(FILE *stream);
func (c *cpu) fclose() {
	u := readPtr(c.sp)
	f := files.extract(readPtr(u))
	if f == nil {
		c.thread.errno = int32(syscall.EBADF)
		writeI32(c.rp, eof)
		return
	}

	c.m.free(u)
	if err := f.Close(); err != nil {
		c.thread.errno = int32(syscall.EIO)
		writeI32(c.rp, eof)
		return
	}

	writeI32(c.rp, 0)
}

// int fgetc(FILE *stream);
func (c *cpu) fgetc() {
	p := buffer.Get(1)
	if _, err := files.reader(readPtr(c.sp), c).Read(*p); err != nil {
		writeI32(c.rp, eof)
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

// FILE *fopen(const char *path, const char *mode);
func (c *cpu) fopen() {
	sp, mode := popPtr(c.sp)
	path := readPtr(sp)
	p := GoString(path)
	var f *os.File
	var err error
	switch p {
	case os.Stderr.Name():
		f = os.Stderr
	case os.Stdin.Name():
		f = os.Stdin
	case os.Stdout.Name():
		f = os.Stdout
	default:
		switch mode := GoString(mode); mode {
		case "r":
			if f, err = os.OpenFile(p, os.O_RDONLY, 0666); err != nil {
				switch {
				case os.IsNotExist(err):
					c.thread.errno = int32(syscall.ENOENT)
				case os.IsPermission(err):
					c.thread.errno = int32(syscall.EPERM)
				default:
					c.thread.errno = int32(syscall.EACCES)
				}
				writePtr(c.rp, 0)
				return
			}
		case "w":
			if f, err = os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666); err != nil {
				switch {
				case os.IsPermission(err):
					c.thread.errno = int32(syscall.EPERM)
				default:
					c.thread.errno = int32(syscall.EACCES)
				}
				writePtr(c.rp, 0)
				return
			}
		default:
			panic(mode)
		}
	}

	u := c.m.malloc(int(unsafe.Sizeof(file{})))
	files.add(f, u)
	writePtr(c.rp, u)
}

// int fprintf(FILE * stream, const char *format, ...);
func (c *cpu) fprintf() {
	ap := c.rp - ptrStackSz
	stream := readPtr(ap)
	ap -= ptrStackSz
	writeI32(c.rp, goFprintf(files.writer(stream, c), readPtr(ap), ap))
}

// void free(void *ptr);
func (c *cpu) free() { c.m.free(readPtr(c.rp - ptrStackSz)) }

// size_t fread(void *ptr, size_t size, size_t nmemb, FILE *stream);
func (c *cpu) fread() {
	sp, stream := popPtr(c.sp)
	sp, nmemb := popLong(sp)
	sp, size := popLong(sp)
	ptr := readPtr(sp)
	hi, lo := mathutil.MulUint128_64(uint64(size), uint64(nmemb))
	if hi != 0 || lo > math.MaxInt32 {
		c.thread.errno = int32(syscall.E2BIG)
		writeULong(c.rp, 0)
		return
	}

	n, err := files.reader(stream, c).Read((*[math.MaxInt32]byte)(unsafe.Pointer(ptr))[:lo])
	if err != nil {
		c.thread.errno = int32(syscall.EIO)
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
		c.thread.errno = int32(syscall.E2BIG)
		writeULong(c.rp, 0)
		return
	}

	n, err := files.writer(stream, c).Write((*[math.MaxInt32]byte)(unsafe.Pointer(ptr))[:lo])
	if err != nil {
		c.thread.errno = int32(syscall.EIO)
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
					if model == 32 {
						argp -= i32StackSz
						arg = readI32(argp)
						break
					}

					fallthrough
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
					if model == 32 {
						argp -= i32StackSz
						arg = readU32(argp)
						break
					}

					fallthrough
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
					if model == 32 {
						argp -= i32StackSz
						arg = readU32(argp)
						break
					}

					fallthrough
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
	writeI32(c.rp, goFprintf(c.m.stdout, readPtr(c.rp-ptrStackSz), c.rp-ptrStackSz))
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
