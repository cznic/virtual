// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package virtual implements a machine that isn't. (Work In Progress)
//
// For supported platforms and architectures please see [0].
//
// Links
//
// Referenced from elsewhere
//
//  [0]: https://github.com/cznic/ccir
package virtual

import (
	"fmt"
	"io"

	"github.com/cznic/xc"
)

var (
	// Testing amends things for tests.
	Testing bool

	dict = xc.Dict
)

// New runs the program in b and returns its exit status or an error, if any.
// It's the caller responsibility to ensure the binary was produced for the
// correct architecture and platform.
//
// If a stack trace is produced on error, the PCInfo is interpreted relative to
// tracePath and if a corresponding source file is available the trace is
// extended with the respective source code lines.
//
// The returned machine is ready, if applicable, for calling individual
// external functions. Its Close method must be called eventually to free any
// resources it has acquired from the OS.
func New(b *Binary, args []string, stdin io.Reader, stdout, stderr io.Writer, heapSize, stackSize int, tracePath string) (m *Machine, exitStatus int, err error) {
	pc, ok := b.Sym[idStart]
	if !ok {
		return nil, -1, fmt.Errorf("missing symbol: %s", idStart)
	}

	if m, err = newMachine(b, heapSize, stdin, stdout, stderr, tracePath); err != nil {
		return nil, -1, err
	}

	t, err := m.NewThread(stackSize)
	if err != nil {
		return nil, -1, err
	}

	argv := make([]uintptr, len(args)+1)
	for i, v := range args {
		argv[i] = m.CString(v)
	}
	pargv := m.malloc(len(argv) * ptrSize)
	for i, v := range argv {
		writePtr(pargv+uintptr(i*ptrSize), v)
	}

	// void _start(int args, char **argv);
	t.rp = t.sp
	t.sp -= i32StackSz
	writeI32(t.sp, int32(len(args))) // argc
	t.sp -= ptrStackSz
	writePtr(t.sp, pargv) // argv
	t.sp -= ptrStackSz
	writePtr(t.sp, 0xcafebabe) // return address, not used
	if exitStatus, err = t.run(uintptr(pc) + ffiProlog); err != nil {
		return nil, exitStatus, err
	}

	return m, exitStatus, nil
}

// Exec is a convenience wrapper around New. It takes care of calling the
// Close method of the Machine returned by New.
func Exec(b *Binary, args []string, stdin io.Reader, stdout, stderr io.Writer, heapSize, stackSize int, tracePath string) (exitStatus int, err error) {
	var m *Machine
	m, exitStatus, err = New(b, args, stdin, stdout, stderr, heapSize, stackSize, tracePath)
	if m != nil {
		if e := m.Close(); e != nil && err == nil {
			err = e
		}
	}
	return exitStatus, err
}
