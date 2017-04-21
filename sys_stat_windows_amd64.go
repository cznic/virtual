// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

// int fstat64(int fildes, struct stat64 *buf);
func (c *cpu) fstat64() { panic("unreachable") }

// extern int lstat64(char *__file, struct stat64 *__buf);
func (c *cpu) lstat64() { panic("unreachable") }

// extern int stat64(char *__file, struct stat64 *__buf);
func (c *cpu) stat64() { panic("unreachable") }
