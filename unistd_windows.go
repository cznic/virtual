// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

// int access(const char *path, int amode);
func (c *cpu) access() { panic("unreachable") }

// int close(int fd);
func (c *cpu) close() { panic("unreachable") }

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
func (c *cpu) read() { panic("unreachable") }

// long sysconf(int name);
func (c *cpu) sysconf() { panic("unreachable") }

// int unlink(const char *path);
func (c *cpu) unlink() { panic("unreachable") }

// ssize_t write(int fd, const void *buf, size_t count);
func (c *cpu) write() { panic("unreachable") }
