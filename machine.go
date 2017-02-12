// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"sync"
	"unsafe"

	"github.com/cznic/mathutil"
	"github.com/edsrzf/mmap-go"
)

const (
	c128StackSz = f64StackSz
	c64StackSz  = f64StackSz
	f32StackSz  = i32StackSz
	f64StackSz  = i64StackSz
	i16StackSz  = (2 + stackAlign - 1) &^ (stackAlign - 1)
	i32StackSz  = (4 + stackAlign - 1) &^ (stackAlign - 1)
	i64StackSz  = (8 + stackAlign - 1) &^ (stackAlign - 1)
	i8StackSz   = (1 + stackAlign - 1) &^ (stackAlign - 1)
	intSize     = mathutil.IntBits / 8
	mallocAlign = ptrSize
	mmapPage    = 1 << 16
	ptrSize     = mathutil.UintPtrBits / 8
	ptrStackSz  = (ptrSize + stackAlign - 1) &^ (stackAlign - 1)
	stackAlign  = ptrSize
)

type machine struct {
	brk       uintptr
	bss       uintptr
	bssSize   int
	ds        uintptr
	dsMem     mmap.MMap
	functions []PCInfo
	lines     []PCInfo
	stderr    io.Writer
	stdin     io.Reader
	stdout    io.Writer
	stop      chan struct{}
	stopMu    sync.Mutex
	stopped   bool
	threads   []*thread
	threadsMu sync.Mutex
	ts        uintptr
	tsFile    *os.File
	tsMem     mmap.MMap
}

func newMachine(data, text []byte, bssSize, heapSize int, stdin io.Reader, stdout, stderr io.Writer) (*machine, error) {
	var (
		ds, ts       uintptr
		err          error
		tsFile       *os.File
		tsMem, dsMem mmap.MMap
	)
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
	}

	return &machine{
		brk:     ds + uintptr(brk),
		bss:     ds + uintptr(dsSize),
		bssSize: bssSize,
		ds:      ds,
		dsMem:   dsMem,
		stderr:  stderr,
		stdin:   stdin,
		stdout:  stdout,
		stop:    make(chan struct{}),
		ts:      ts,
		tsFile:  tsFile,
		tsMem:   tsMem,
	}, nil
}

func (m *machine) CString(s string) uintptr {
	p := m.malloc(len(s) + 1)
	copy(m.dsMem[p-m.ds:], s)
	m.dsMem[p-m.ds+uintptr(len(s))+1] = 0
	return p
}

func (m *machine) close() (err error) {
	m.Kill()
	if m.dsMem != nil {
		if e := m.dsMem.Unmap(); e != nil && err == nil {
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
	for _, v := range m.threads {
		if e := v.close(); e != nil && err == nil {
			err = e
		}
	}
	m.threadsMu.Unlock()
	return err
}

func (m *machine) pcInfo(pc int, infos []PCInfo) PCInfo {
	if i := sort.Search(len(infos), func(i int) bool { return infos[i].PC >= pc }); len(infos) != 0 && i <= len(infos) {
		switch {
		case i == len(infos):
			return infos[i-1]
		default:
			if pc == infos[i].PC {
				return infos[i]
			}

			if i > 0 {
				return infos[i-1]
			}
		}
	}
	return PCInfo{}
}

func (m *machine) Kill() {
	m.stopMu.Lock()
	if !m.stopped {
		close(m.stop)
		m.stopped = true
	}
	m.stopMu.Unlock()
}

func (m *machine) malloc(n int) uintptr { //TODO real malloc
	p := m.brk
	if m.sbrk(n)-m.ds < uintptr(len(m.dsMem)) {
		return p
	}

	panic("vm: out of memory")
}

func (m *machine) newThread(stackSize int) (*thread, error) {
	stackSize = roundup(stackSize, mmapPage)
	stackMem, err := mmap.MapRegion(nil, stackSize, mmap.RDWR, mmap.ANON, 0)
	if err != nil {
		return nil, fmt.Errorf("mmap stack segment: %v", err)
	}

	ss := uintptr(unsafe.Pointer(&stackMem[0]))
	t := &thread{
		cpu: cpu{
			bp:   0xdeadbeef,
			ds:   m.ds,
			m:    m,
			sp:   ss + uintptr(stackSize),
			stop: m.stop,
			ts:   m.ts,
		},
		ss:       ss,
		stackMem: stackMem,
	}
	t.cpu.thread = t
	m.threadsMu.Lock()
	m.threads = append(m.threads, t)
	m.threadsMu.Unlock()
	return t, nil
}

func (m *machine) sbrk(n int) uintptr {
	m.brk += uintptr(roundup(n, mallocAlign))
	return m.brk
}

type thread struct {
	cpu
	errno    int32
	ss       uintptr // Stack segment
	stackMem mmap.MMap
}

func (t *thread) close() error { return t.stackMem.Unmap() }
