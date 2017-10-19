// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("__sysv_signal"): __sysv_signal,
		dict.SID("signal"):        signal_,
	})
}

// sighandler_t signal(int signum, sighandler_t handler);
func (c *cpu) sysvSignal() {
	writePtr(c.rp, 0) //TODO
}
