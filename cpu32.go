// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build 386

package virtual

import (
	"math"
)

func (c *cpu) float64(n, m int) {
	c.sp -= f64StackSz
	c.writeF64(c.sp, math.Float64frombits(uint64(n)|uint64(m)))
	c.ip++
}
