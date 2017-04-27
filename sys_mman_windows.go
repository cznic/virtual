// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

// void *mmap(void *addr, size_t len, int prot, int flags, int fildes, off_t off);
func (c *cpu) mmap64() { panic("unreachable") }

// int munmap(void *addr, size_t len);
func (c *cpu) munmap() { panic("unreachable") }
