// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
)

func caller(s string, va ...interface{}) {
	if s == "" {
		s = strings.Repeat("%v ", len(va))
	}
	_, fn, fl, _ := runtime.Caller(2)
	fmt.Fprintf(os.Stderr, "# caller: %s:%d: ", path.Base(fn), fl)
	fmt.Fprintf(os.Stderr, s, va...)
	fmt.Fprintln(os.Stderr)
	_, fn, fl, _ = runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "# \tcallee: %s:%d: ", path.Base(fn), fl)
	fmt.Fprintln(os.Stderr)
	os.Stderr.Sync()
}

func dbg(s string, va ...interface{}) {
	if s == "" {
		s = strings.Repeat("%v ", len(va))
	}
	_, fn, fl, _ := runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "# dbg %s:%d: ", path.Base(fn), fl)
	fmt.Fprintf(os.Stderr, s, va...)
	fmt.Fprintln(os.Stderr)
	os.Stderr.Sync()
}

func TODO(...interface{}) string { //TODOOK
	_, fn, fl, _ := runtime.Caller(1)
	return fmt.Sprintf("# TODO: %s:%d:\n", path.Base(fn), fl) //TODOOK
}

func use(...interface{}) {}

func init() {
	use(caller, dbg, TODO) //TODOOK
}

// ============================================================================

func TestAbort(t *testing.T) {
	m, err := newMachine(nil, 0, nil, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()

	thread, err := m.NewThread(mmapPage)
	if err != nil {
		t.Fatal(err)
	}

	m.code = []Operation{
		{abort, 0},
	}
	if g, _ := thread.cpu.run(0); g == 0 {
		t.Fatal("expected non zero exit code")
	}
}

func TestExit(t *testing.T) {
	m, err := newMachine(nil, 0, nil, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()

	thread, err := m.NewThread(mmapPage)
	if err != nil {
		t.Fatal(err)
	}

	e := 42
	m.code = []Operation{
		{Push32, e},
		{exit, 0},
	}
	if g, _ := thread.cpu.run(0); g != e {
		t.Fatal("exit code", g, e)
	}
}

func TestKill(t *testing.T) {
	m, err := newMachine(nil, 0, nil, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()

	thread, err := m.NewThread(mmapPage)
	if err != nil {
		t.Fatal(err)
	}

	ch := make(chan int)
	go func() {
		m.code = []Operation{
			{Jmp, 0},
		}
		es, _ := thread.cpu.run(0)
		ch <- es
	}()

	m.Kill()
	if g, e := <-ch, -1; g != e {
		t.Fatal("kill", g, e)
	}
}
