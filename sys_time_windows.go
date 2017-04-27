// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

// int gettimeofday(struct timeval *restrict tp, void *restrict tzp);
func (c *cpu) gettimeofday() { panic("unreachable") }
