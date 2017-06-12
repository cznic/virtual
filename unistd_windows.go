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
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("read"):   read,
		dict.SID("close"):  close_,
		dict.SID("usleep"): usleep,
	})
}

// int access(const char *path, int amode);
func (c *cpu) access() { panic("unreachable") }

// int close(int fd);
func (c *cpu) close() {
	fd := readI32(c.sp)
	err := syscall.Close(syscall.Handle(fd))
	if strace {
		fmt.Fprintf(os.Stderr, "close(%v) %v\n", fd, err)
	}
	if err != nil {
		c.setErrno(err)
	}
	writeI32(c.rp, 0)
}

// int fsync(int fildes);
func (c *cpu) fsync() { panic("unreachable") }

// int ftruncate(int fildes, off_t length);
func (c *cpu) ftruncate64() { panic("unreachable") }

// char *getcwd(char *buf, size_t size);
func (c *cpu) getcwd() { panic("unreachable") }

// uid_t geteuid(void);
func (c *cpu) geteuid() { panic("unreachable") }

// pid_t getpid(void);
func (c *cpu) getpid() { panic("unreachable") }

// off_t lseek64(int fildes, off_t offset, int whence);
func (c *cpu) lseek64() { panic("unreachable") }

// ssize_t read(int fd, void *buf, size_t count);
func (c *cpu) read() {
	sp, count := popLong(c.sp)
	sp, buf := popPtr(sp)
	fd := readI32(sp)
	slice := (*[math.MaxInt32]byte)(unsafe.Pointer(buf))[:count]
	r, err := syscall.Read(syscall.Handle(uintptr(fd)), slice)
	if strace {
		fmt.Fprintf(os.Stderr, "read(%v, %#x, %v) %v %v\n", fd, buf, count, r, err)
	}
	if err != nil {
		c.thread.setErrno(err)
	}
	writeLong(c.rp, int64(r))
}

// long sysconf(int name);
func (c *cpu) sysconf() { panic("unreachable") }

// int unlink(const char *path);
func (c *cpu) unlink() { panic("unreachable") }

// ssize_t write(int fd, const void *buf, size_t count);
func (c *cpu) write() { panic("unreachable") }

// int usleep(useconds_t usec);
func (c *cpu) usleep() {
	usec := readU32(c.sp)
	tim.Sleep(tim.Duration(usec) * tim.Microsecond)
	if strace {
		fmt.Fprintf(os.Stderr, "usleep(%#x)", usec)
	}
	writeI32(c.rp, 0)
}
