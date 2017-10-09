// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"github.com/edsrzf/mmap-go"
)

var (
	_ FFIArgument = Float64(0)
	_ FFIArgument = Int32(0)
	_ FFIArgument = Int64(0)
	_ FFIResult   = Float64Result{}
	_ FFIResult   = Int32Result{}
	_ FFIResult   = Int64Result{}
	_ FFIResult   = PtrResult{}
)

// FFIArgument is immplemented by all types of FFI arguments.
type FFIArgument interface {
	arg()
}

// FFIResult is immplemented by all types of FFI results.
type FFIResult interface {
	result()
}

// Float64 is an float64 FFI argument.
type Float64 int64

func (Float64) arg() {}

// Int32 is an int32 FFI argument.
type Int32 int32

func (Int32) arg() {}

// Int64 is an int64 FFI argument.
type Int64 int64

func (Int64) arg() {}

// Ptr is a pointer FFI argument.
type Ptr uintptr

func (Ptr) arg() {}

// Int32Result is an FFI int32 result.
type Int32Result struct{ Value *int32 }

func (Int32Result) result() {}

// Int64Result is an FFI int64 result.
type Int64Result struct{ Value *int64 }

func (Int64Result) result() {}

// Float64Result is an FFI float64 result.
type Float64Result struct{ Value *float64 }

func (Float64Result) result() {}

// PtrResult is an FFI pointer result.
type PtrResult struct{ Value *uintptr }

func (PtrResult) result() {}

type tls struct {
	errno    int32
	threadID uintptr
}

// Thread is a thread of VM execution.
type Thread struct {
	cpu
	ss       uintptr // Stack segment
	stackMem mmap.MMap
}

func (t *Thread) close() error { return t.stackMem.Unmap() }

// Close frees resources acquired from the OS by t.
func (t *Thread) Close() error {
	t.m.threadsMu.Lock()
	for i, v := range t.m.Threads {
		if v == t {
			n := len(t.m.Threads)
			copy(t.m.Threads[:i], t.m.Threads[i+1:])
			t.m.Threads = t.m.Threads[:n-1]
			break
		}
	}
	t.m.threadsMu.Unlock()
	return t.stackMem.Unmap()
}

// FFI0 executes a void function fn using 'in' as arguments.  The number and
// types of arguments must match the number and types of the function
// arguments. Variadic functions are supported.
func (t *Thread) FFI0(fn int, in ...FFIArgument) (int, error) {
	return t.FFI(fn, nil, in...)
}

// FFI1 executes function fn, having one result, using 'in' as arguments.  The
// number and types of 'in' items must match the number and types of the
// function arguments. Variadic functions are supported.
func (t *Thread) FFI1(fn int, out FFIResult, in ...FFIArgument) (int, error) {
	return t.FFI(fn, []FFIResult{out}, in...)
}

// FFI executes function fn using 'in' as arguments.  The number and types  of
// 'out' and 'in' items must match the number and types of the function results
// and arguments. Variadic functions are supported.
func (t *Thread) FFI(fn int, out []FFIResult, in ...FFIArgument) (int, error) {
	rpStack := t.rpStack
	rp := t.rp
	sp := t.sp

	// Alloc result(s)
	for _, v := range out {
		switch x := v.(type) {
		case Int32Result:
			t.sp -= i32StackSz
		case Int64Result:
			t.sp -= i64StackSz
		case Float64Result:
			t.sp -= f64StackSz
		case PtrResult:
			t.sp -= ptrStackSz
		default:
			panic(fmt.Errorf("%T", x))
		}
	}
	// Arguments
	t.rpStack = append(t.rpStack, t.rp)
	t.rp = t.sp
	r := t.rp
	for _, v := range in {
		switch x := v.(type) {
		case Float64:
			t.sp -= f64StackSz
			writeF64(t.sp, float64(x))
		case Int32:
			t.sp -= i32StackSz
			writeI32(t.sp, int32(x))
		case Int64:
			t.sp -= i64StackSz
			writeI64(t.sp, int64(x))
		case Ptr:
			t.sp -= ptrStackSz
			writePtr(t.sp, uintptr(x))
		default:
			panic(fmt.Errorf("%T", x))
		}
	}
	s, err := t.run(uintptr(fn))
	if err != nil {
		t.rpStack = rpStack
		t.rp = rp
		t.sp = sp
		return s, err
	}

	for _, v := range out {
		switch x := v.(type) {
		case Int32Result:
			if p := x.Value; p != nil {
				r, *p = popI32(r)
			}
		case Int64Result:
			if p := x.Value; p != nil {
				r, *p = popI64(r)
			}
		case Float64Result:
			if p := x.Value; p != nil {
				r, *p = popF64(r)
			}
		case PtrResult:
			if p := x.Value; p != nil {
				r, *p = popPtr(r)
			}
		default:
			panic(fmt.Errorf("%T", x))
		}
	}
	t.rpStack = rpStack
	t.rp = rp
	t.sp = sp
	return s, err
}
