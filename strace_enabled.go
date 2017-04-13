// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build strace

package virtual

import (
	"fmt"
	"github.com/cznic/ccir/libc"
	"strings"
)

const strace = true

func cmdString(cmd int32) string {
	switch cmd {
	case libc.Fcntl_F_DUPFD:
		return "F_DUPFD"
	case libc.Fcntl_F_GETFD:
		return "F_GETFD"
	case libc.Fcntl_F_GETFL:
		return "F_GETFL"
	case libc.Fcntl_F_GETLK:
		return "F_GETLK"
	case libc.Fcntl_F_GETOWN:
		return "F_GETOWN"
	case libc.Fcntl_F_SETFD:
		return "F_SETFD"
	case libc.Fcntl_F_SETFL:
		return "F_SETFL"
	case libc.Fcntl_F_SETLK:
		return "F_SETLK"
	case libc.Fcntl_F_SETLKW:
		return "F_SETLKW"
	case libc.Fcntl_F_SETOWN:
		return "F_SETOWN"
	default:
		return fmt.Sprintf("%#x", cmd)
	}
}

func modeString(flag int32) string {
	if flag == 0 {
		return "0"
	}

	var a []string
	for _, v := range []struct {
		int32
		string
	}{
		{libc.Fcntl_O_APPEND, "O_APPEND"},
		{libc.Fcntl_O_CREAT, "O_CREAT"},
		{libc.Fcntl_O_DSYNC, "O_DSYNC"},
		{libc.Fcntl_O_EXCL, "O_EXCL"},
		{libc.Fcntl_O_NOCTTY, "O_NOCTTY"},
		{libc.Fcntl_O_NONBLOCK, "O_NONBLOCK"},
		{libc.Fcntl_O_RDONLY, "O_RDONLY"},
		{libc.Fcntl_O_RDWR, "O_RDWR"},
		{libc.Fcntl_O_WRONLY, "O_WRONLY"},
	} {
		if flag&v.int32 != 0 {
			a = append(a, v.string)
		}
	}
	return strings.Join(a, "|")
}
