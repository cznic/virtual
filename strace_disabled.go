// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !virtual.strace

package virtual

const strace = false

func cmdString(flag int32) string  { return "" }
func modeString(flag int32) string { return "" }
