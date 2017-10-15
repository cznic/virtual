// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"math"
	"os"
	"syscall"
	tim "time"
	"unsafe"

	"github.com/cznic/ccir/libc/unistd"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("access"):      access,
		dict.SID("close"):       close_,
		dict.SID("fchown"):      fchown,
		dict.SID("fsync"):       fsync,
		dict.SID("ftruncate64"): ftruncate64,
		dict.SID("getcwd"):      getcwd,
		dict.SID("geteuid"):     geteuid,
		dict.SID("gethostname"): gethostname,
		dict.SID("getpid"):      getpid,
		dict.SID("lseek64"):     lseek64,
		dict.SID("read"):        read,
		dict.SID("readlink"):    readlink,
		dict.SID("rmdir"):       rmdir,
		dict.SID("sleep"):       sleep,
		dict.SID("sysconf"):     sysconf,
		dict.SID("unlink"):      unlink,
		dict.SID("usleep"):      usleep,
		dict.SID("write"):       write,
	})
}

// int access(const char *path, int amode);
func (c *cpu) access() {
	sp, amode := popI32(c.sp)
	path := readPtr(sp)
	r, _, err := syscall.Syscall(syscall.SYS_ACCESS, path, uintptr(amode), 0)
	if strace {
		fmt.Fprintf(os.Stderr, "access(%q) %v %v\t; %s\t; %s\n", GoString(path), r, err, c.pos(), c.pos())
	}
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(r))
}

// int close(int fd);
func (c *cpu) close() {
	fd := readI32(c.sp)
	r, _, err := syscall.Syscall(syscall.SYS_CLOSE, uintptr(fd), 0, 0)
	if strace {
		fmt.Fprintf(os.Stderr, "close(%v) %v %v\t; %s\n", fd, r, err, c.pos())
	}
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(r))
}

// int fsync(int fildes);
func (c *cpu) fsync() {
	fildes := readI32(c.sp)
	r, _, err := syscall.Syscall(syscall.SYS_FSYNC, uintptr(fildes), 0, 0)
	if strace {
		fmt.Fprintf(os.Stderr, "fsync(%v) %v %v\t; %s\n", fildes, r, err, c.pos())
	}
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(r))
}

// int ftruncate(int fildes, off_t length);
func (c *cpu) ftruncate64() {
	sp, length := popI64(c.sp)
	fildes := readI32(sp)
	r, _, err := syscall.Syscall(syscall.SYS_FTRUNCATE, uintptr(fildes), uintptr(length), 0)
	if strace {
		fmt.Fprintf(os.Stderr, "ftruncate(%#x, %#x) %v, %v\t; %s\n", fildes, length, r, err, c.pos())
	}
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(r))
}

// char *getcwd(char *buf, size_t size);
func (c *cpu) getcwd() {
	sp, size := popLong(c.sp)
	buf := readPtr(sp)
	r, _, err := syscall.Syscall(syscall.SYS_GETCWD, buf, uintptr(size), 0)
	if strace {
		fmt.Fprintf(os.Stderr, "getcwd(%#x, %#x) %v %v %q\t; %s\n", buf, size, r, err, GoString(buf), c.pos())
	}
	if err != 0 {
		c.setErrno(err)
	}
	writePtr(c.rp, r)
}

// uid_t geteuid(void);
func (c *cpu) geteuid() {
	r, _, _ := syscall.RawSyscall(syscall.SYS_GETEUID, 0, 0, 0)
	if strace {
		fmt.Fprintf(os.Stderr, "geteuid() %v\t; %s\n", r, c.pos())
	}
	writeU32(c.rp, uint32(r))
}

// pid_t getpid(void);
func (c *cpu) getpid() {
	r, _, _ := syscall.RawSyscall(syscall.SYS_GETPID, 0, 0, 0)
	if strace {
		fmt.Fprintf(os.Stderr, "getpid() %v\t; %s\n", r, c.pos())
	}
	writeI32(c.rp, int32(r))
}

// off_t lseek64(int fildes, off_t offset, int whence);
func (c *cpu) lseek64() {
	sp, whence := popI32(c.sp)
	sp, offset := popI64(sp)
	fildes := readI32(sp)
	r, _, err := syscall.Syscall(syscall.SYS_LSEEK, uintptr(fildes), uintptr(offset), uintptr(whence))
	if strace {
		fmt.Fprintf(os.Stderr, "lseek(%v, %v, %v) %v %v\t; %s\n", fildes, offset, whence, r, err, c.pos())
	}
	if err != 0 {
		c.setErrno(err)
	}
	writeLong(c.rp, int64(r))
}

// ssize_t read(int fd, void *buf, size_t count);
func (c *cpu) read() { //TODO stdin
	sp, count := popLong(c.sp)
	sp, buf := popPtr(sp)
	fd := readI32(sp)
	r, _, err := syscall.Syscall(syscall.SYS_READ, uintptr(fd), buf, uintptr(count))
	if strace {
		fmt.Fprintf(os.Stderr, "read(%v, %#x, %v) %v %v\t; %s\n", fd, buf, count, r, err, c.pos())
	}
	if err != 0 {
		c.thread.setErrno(err)
	}
	writeLong(c.rp, int64(r))
}

// long sysconf(int name);
func (c *cpu) sysconf() {
	switch n := readI32(c.sp); n {
	case unistd.X_SC_PAGESIZE:
		writeLong(c.rp, int64(os.Getpagesize()))
	default:
		panic(fmt.Errorf("%v(%#x)", n, n))
	}
}

// int unlink(const char *path);
func (c *cpu) unlink() {
	path := readPtr(c.sp)
	r, _, err := syscall.Syscall(syscall.SYS_UNLINK, path, 0, 0)
	if strace {
		fmt.Fprintf(os.Stderr, "unlink(%q) %v %v\t; %s\n", GoString(path), r, err, c.pos())
	}
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(r))
}

// ssize_t write(int fd, const void *buf, size_t count);
func (c *cpu) write() {
	sp, count := popLong(c.sp)
	sp, buf := popPtr(sp)
	fd := readI32(sp)
	switch fd {
	case unistd.XSTDOUT_FILENO:
		n, err := c.m.stdout.Write((*[math.MaxInt32]byte)(unsafe.Pointer(buf))[:count])
		if err != nil {
			c.thread.setErrno(err)
		}
		writeLong(c.rp, int64(n))
		return
	case unistd.XSTDERR_FILENO:
		n, err := c.m.stderr.Write((*[math.MaxInt32]byte)(unsafe.Pointer(buf))[:count])
		if err != nil {
			c.thread.setErrno(err)
		}
		writeLong(c.rp, int64(n))
		return
	}
	r, _, err := syscall.Syscall(syscall.SYS_WRITE, uintptr(fd), buf, uintptr(count))
	if strace {
		fmt.Fprintf(os.Stderr, "write(%v, %#x, %v) %v %v\t; %s\n", fd, buf, count, r, err, c.pos())
	}
	if err != 0 {
		c.thread.setErrno(err)
	}
	writeLong(c.rp, int64(r))
}

// int usleep(useconds_t usec);
func (c *cpu) usleep() {
	usec := readU32(c.sp)
	tim.Sleep(tim.Duration(usec) * tim.Microsecond)
	if strace {
		fmt.Fprintf(os.Stderr, "usleep(%#x)", usec)
	}
	writeI32(c.rp, 0)
}
