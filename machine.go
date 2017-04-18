// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/cznic/internal/buffer"
	"github.com/cznic/mathutil"
	"github.com/edsrzf/mmap-go"
)

const (
	c128StackSz  = (16 + stackAlign - 1) &^ (stackAlign - 1)
	c64StackSz   = f64StackSz
	f32StackSz   = i32StackSz
	f64StackSz   = i64StackSz
	i16StackSz   = (2 + stackAlign - 1) &^ (stackAlign - 1)
	i32StackSz   = (4 + stackAlign - 1) &^ (stackAlign - 1)
	i64StackSz   = (8 + stackAlign - 1) &^ (stackAlign - 1)
	i8StackSz    = (1 + stackAlign - 1) &^ (stackAlign - 1)
	intSize      = mathutil.IntBits / 8
	longStackSz  = (longBits/8 + stackAlign - 1) &^ (stackAlign - 1)
	mallocAlign  = 2 * ptrSize
	mmapPage     = 1 << 16
	ptrSize      = mathutil.UintPtrBits / 8
	ptrStackSz   = (ptrSize + stackAlign - 1) &^ (stackAlign - 1)
	stackAlign   = ptrSize
	tlsStackSize = (unsafe.Sizeof(tls{}) + stackAlign - 1) &^ (stackAlign - 1)
)

type memWriter uintptr

func (m *memWriter) WriteByte(b byte) error {
	p := *m
	writeU8(uintptr(p), b)
	*m = p + 1
	return nil
}

func (m *memWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}

	*m += memWriter(movemem(uintptr(*m), uintptr((unsafe.Pointer)(&b[0])), len(b)))
	return len(b), nil
}

func movemem(dst, src uintptr, n int) int {
	return copy((*[math.MaxInt32]byte)(unsafe.Pointer(dst))[:n], (*[math.MaxInt32]byte)(unsafe.Pointer(src))[:n])
}

// GoString returns a string from a C char* s.
func GoString(s uintptr) string {
	var b buffer.Bytes
	for {
		ch := readU8(s)
		if ch == 0 {
			return string(b.Bytes())
		}

		b.WriteByte(ch)
		s++
	}
}

// Machine represents the state of the VM memory and threads.
type Machine struct {
	DS        uintptr
	DSMem     mmap.MMap // Data segment.
	brk       uintptr
	bss       uintptr
	bssSize   int
	code      []Operation
	functions []PCInfo
	lines     []PCInfo
	stderr    io.Writer
	stdin     io.Reader
	stdout    io.Writer
	stop      chan struct{}
	stopMu    sync.Mutex
	stopped   bool
	threadID  uintptr
	Threads   []*Thread
	threadsMu sync.Mutex
	tracePath string
	ts        uintptr
	tsFile    *os.File
	tsMem     mmap.MMap
}

func newMachine(b *Binary, heapSize int, stdin io.Reader, stdout, stderr io.Writer, tracePath string) (*Machine, error) {
	var (
		bssSize      int
		data, text   []byte
		ds, ts       uintptr
		err          error
		tsFile       *os.File
		tsMem, dsMem mmap.MMap
	)
	if b != nil {
		data = b.Data
		text = b.Text
		bssSize = b.BSS
	}
	if len(text) != 0 {
		if tsFile, err = ioutil.TempFile("", "virtual-machine-text-"); err != nil {
			return nil, err
		}

		if _, err := tsFile.Write(text); err != nil {
			tsFile.Close()
			return nil, err
		}

		if tsMem, err = mmap.Map(tsFile, mmap.RDONLY, 0); err != nil {
			tsFile.Close()
			return nil, fmt.Errorf("mmap text segment: %v", err)
		}

		ts = uintptr(unsafe.Pointer(&tsMem[0]))
	}

	dsSize := roundup(len(data), mallocAlign)
	bssSize = roundup(bssSize, mallocAlign)
	brk := dsSize + bssSize
	if n := brk + heapSize; n != 0 {
		if dsMem, err = mmap.MapRegion(nil, roundup(n, mmapPage), mmap.RDWR, mmap.ANON, 0); err != nil {
			return nil, fmt.Errorf("mmap data segment: %v", err)
		}

		copy(dsMem, data)
		ds = uintptr(unsafe.Pointer(&dsMem[0]))
		if b != nil {
			for i, v := range b.TSRelative {
				if v == 0 {
					continue
				}

				mask := byte(1)
				for bit := 0; bit < 8; bit++ {
					if v&mask != 0 {
						addPtr(ds+uintptr(8*i+bit), ts)
					}
					mask <<= 1
				}
			}
			for i, v := range b.DSRelative {
				if v == 0 {
					continue
				}

				mask := byte(1)
				for bit := 0; bit < 8; bit++ {
					if v&mask != 0 {
						addPtr(ds+uintptr(8*i+bit), ds)
					}
					mask <<= 1
				}
			}
		}
	}

	var lines, functions []PCInfo
	if b != nil {
		lines = b.Lines
		functions = b.Functions
	}
	var code []Operation
	if b != nil {
		code = b.Code
	}
	return &Machine{
		DS:        ds,
		DSMem:     dsMem,
		brk:       ds + uintptr(brk),
		bss:       ds + uintptr(dsSize),
		bssSize:   bssSize,
		code:      code,
		functions: functions,
		lines:     lines,
		stderr:    stderr,
		stdin:     stdin,
		stdout:    stdout,
		stop:      make(chan struct{}),
		tracePath: tracePath,
		ts:        ts,
		tsFile:    tsFile,
		tsMem:     tsMem,
	}, nil
}

// CString allocates a C string initialized from s.
func (m *Machine) CString(s string) uintptr {
	n := len(s)
	p := m.malloc(len(s) + 1)
	i := p - m.DS
	copy(m.DSMem[i:], s)
	m.DSMem[i+uintptr(n)] = 0
	return p
}

// Close frees resources acquired from the OS by m.
func (m *Machine) Close() (err error) {
	m.Kill()
	if m.DSMem != nil {
		if e := m.DSMem.Unmap(); e != nil && err == nil {
			err = e
		}
	}
	if m.tsMem != nil {
		if e := m.tsMem.Unmap(); e != nil && err == nil {
			err = e
		}
	}
	if m.tsFile != nil {
		if e := m.tsFile.Close(); e != nil && err == nil {
			err = e
		}
		nm := m.tsFile.Name()
		if e := os.Remove(nm); e != nil && err == nil {
			err = e
		}
	}
	m.threadsMu.Lock()
	for _, v := range m.Threads {
		if e := v.close(); e != nil && err == nil {
			err = e
		}
	}
	m.threadsMu.Unlock()
	return err
}

func (m *Machine) pcInfo(pc int, infos []PCInfo) *PCInfo {
	if i := sort.Search(len(infos), func(i int) bool { return infos[i].PC >= pc }); len(infos) != 0 && i <= len(infos) {
		switch {
		case i == len(infos):
			return &infos[i-1]
		default:
			if pc == infos[i].PC {
				return &infos[i]
			}

			if i > 0 {
				return &infos[i-1]
			}
		}
	}
	return &PCInfo{}
}

func (m *Machine) Kill() {
	m.stopMu.Lock()
	if !m.stopped {
		close(m.stop)
		m.stopped = true
	}
	m.stopMu.Unlock()
}

func (m *Machine) free(p uintptr) { //TODO
}

func (m *Machine) calloc(n int) uintptr {
	p := m.malloc(n)
	if p != 0 {
		for p := p; n != 0; n-- {
			writeI8(p, 0)
			p++
		}
	}
	return p
}

func (m *Machine) malloc(n int) uintptr { //TODO real malloc
	if n != 0 {
		p := m.brk
		if m.sbrk(n)-m.DS < uintptr(len(m.DSMem)) {
			return p
		}
	}

	return 0
}

func (m *Machine) realloc(p uintptr, n int) uintptr { //TODO real realloc
	q := m.malloc(n)
	if q == 0 {
		return 0
	}

	movemem(q, p, n)
	return q
}

// NewThread returns a newly created Thread or an error, if any. Its Close
// method must be called eventually to free any resources it has acquired from
// the OS.
func (m *Machine) NewThread(stackSize int) (*Thread, error) {
	stackSize = roundup(stackSize, mmapPage)
	stackMem, err := mmap.MapRegion(nil, stackSize, mmap.RDWR, mmap.ANON, 0)
	if err != nil {
		return nil, fmt.Errorf("mmap stack segment: %v", err)
	}

	ss := uintptr(unsafe.Pointer(&stackMem[0]))
	t := &Thread{
		cpu: cpu{
			jmpBuf: jmpBuf{
				bp: 0xdeadbeef,
				sp: ss + uintptr(stackSize) - tlsStackSize,
			},
			ds:   m.DS,
			m:    m,
			stop: m.stop,
			ts:   m.ts,
		},
		ss:       ss,
		stackMem: stackMem,
	}
	t.tls = t.cpu.sp
	t.tlsp = (*tls)(unsafe.Pointer(t.cpu.sp))
	t.tlsp.threadID = atomic.AddUintptr(&m.threadID, 1)
	t.setErrno(0)
	t.thread = t
	m.threadsMu.Lock()
	m.Threads = append(m.Threads, t)
	m.threadsMu.Unlock()
	return t, nil
}

func (m *Machine) sbrk(n int) uintptr {
	m.brk += uintptr(roundup(n, mallocAlign))
	return m.brk
}
