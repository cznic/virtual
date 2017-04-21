// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

// int fcntl(int fildes, int cmd, ...);
func (c *cpu) fcntl() { panic("unreachable") }

// int open64(const char *pathname, int flags, ...);
func (c *cpu) open64() { panic("unreachable") }
