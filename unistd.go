// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__builtin_read"):  read,
		dict.SID("read"):            read,
		dict.SID("__builtin_write"): write,
		dict.SID("write"):           write,
	})
}

// ssize_t read(int fd, void *buf, size_t count);
func (c *cpu) read() {
	panic("TODO")
}

// ssize_t write(int fd, const void *buf, size_t count);
func (c *cpu) write() {
	panic("TODO")
}
