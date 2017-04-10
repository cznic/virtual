// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"io"
	"math"
	"os"
	"syscall"
	"unsafe"

	"github.com/cznic/ccir/libc"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("access"):    access,
		dict.SID("close"):     close_,
		dict.SID("fchown"):    fchown,
		dict.SID("fsync"):     fsync,
		dict.SID("ftruncate"): ftruncate,
		dict.SID("getcwd"):    getcwd,
		dict.SID("geteuid"):   geteuid,
		dict.SID("getpid"):    getpid,
		dict.SID("lseek"):     lseek,
		dict.SID("read"):      read,
		dict.SID("readlink"):  readlink,
		dict.SID("rmdir"):     rmdir,
		dict.SID("sleep"):     sleep,
		dict.SID("sysconf"):   sysconf,
		dict.SID("unlink"):    unlink,
		dict.SID("write"):     write,
	})
}

// int close(int fd);
func (c *cpu) close() {
	f := files.extractFd(uintptr(readI32(c.sp)))
	if f == nil {
		writeI32(c.thread.errno, libc.Errno_EBADF)
		writeI32(c.rp, eof)
		return
	}

	if err := f.Close(); err != nil {
		writeI32(c.thread.errno, libc.Errno_EIO)
		writeI32(c.rp, -1)
		return
	}

	writeI32(c.rp, 0)
}

// int fsync(int fildes);
func (c *cpu) fsync() {
	fildes := readI32(c.sp)
	r, _, err := syscall.Syscall(syscall.SYS_FSYNC, uintptr(fildes), 0, 0)
	if err != 0 {
		c.setErrno(err)
	}
	writeLong(c.rp, int64(r))
}

// char *getcwd(char *buf, size_t size);
func (c *cpu) getcwd() {
	sp, size := popLong(c.sp)
	if size == 0 {
		c.setErrno(libc.Errno_EINVAL)
		writePtr(c.rp, 0)
		return
	}

	buf := readPtr(sp)
	cwd, err := os.Getwd()
	if err != nil {
		c.setErrno(err)
		writePtr(c.rp, 0)
		return
	}

	if int64(len(cwd)+1) > int64(size) {
		c.setErrno(libc.Errno_ERANGE)
		writePtr(c.rp, 0)
		return
	}

	b := []byte(cwd)
	movemem(buf, uintptr(unsafe.Pointer(&b[0])), len(b))
	writeI8(buf+uintptr(len(cwd)), 0)
	writePtr(c.rp, buf)
}

// uid_t geteuid(void);
func (c *cpu) geteuid() { writeU32(c.rp, uint32(syscall.Geteuid())) }

// pid_t getpid(void);
func (c *cpu) getpid() { writeI32(c.rp, int32(os.Getpid())) }

// off_t lseek(int fildes, off_t offset, int whence);
func (c *cpu) lseek() {
	sp, whence := popI32(c.sp)
	sp, offset := popLong(sp)
	fildes := readI32(sp)
	r, _, err := syscall.Syscall(syscall.SYS_LSEEK, uintptr(fildes), uintptr(offset), uintptr(whence))
	if err != 0 {
		c.setErrno(err)
	}
	writeLong(c.rp, int64(r))
}

// ssize_t read(int fd, void *buf, size_t count);
func (c *cpu) read() {
	sp, count := popLong(c.sp)
	sp, buf := popPtr(sp)
	fd := readI32(sp)
	f := files.fdReader(uintptr(fd), c)
	n, err := f.Read((*[math.MaxInt32]byte)(unsafe.Pointer(buf))[:count])
	if n != 0 {
		writeULong(c.rp, uint64(n))
		return
	}

	if err == io.EOF {
		writeULong(c.rp, 0)
		return
	}

	c.thread.setErrno(err)
	writeI32(c.rp, -1)
}

// int unlink(const char *path);
func (c *cpu) unlink() {
	path := readPtr(c.sp)
	r, _, err := syscall.Syscall(syscall.SYS_UNLINK, path, 0, 0)
	if err != 0 {
		c.setErrno(err)
	}
	writeLong(c.rp, int64(r))
}

// ssize_t write(int fd, const void *buf, size_t count);
func (c *cpu) write() {
	sp, count := popLong(c.sp)
	sp, buf := popPtr(sp)
	fd := readI32(sp)
	f := files.fdWriter(uintptr(fd), c)
	n, err := f.Write((*[math.MaxInt32]byte)(unsafe.Pointer(buf))[:count])
	if err != nil {
		writeI32(c.thread.errno, libc.Errno_EIO)
	}
	writeULong(c.rp, uint64(n))
}
