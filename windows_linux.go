// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run generator.go

// +build !windows

package virtual

func (c *cpu) InterlockedCompareExchange() {
	winStub("InterlockedCompareExchange")
}

func (c *cpu) GetLastError() {
	winStub("GetLastError")
}
