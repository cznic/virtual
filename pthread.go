// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"sync"

	"github.com/cznic/ccir/libc/pthread"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("pthread_create"):            pthread_create,
		dict.SID("pthread_equal"):             pthread_equal,
		dict.SID("pthread_join"):              pthread_join,
		dict.SID("pthread_mutex_destroy"):     pthread_mutex_destroy,
		dict.SID("pthread_mutex_init"):        pthread_mutex_init,
		dict.SID("pthread_mutex_lock"):        pthread_mutex_lock,
		dict.SID("pthread_mutex_trylock"):     pthread_mutex_trylock,
		dict.SID("pthread_mutex_unlock"):      pthread_mutex_unlock,
		dict.SID("pthread_mutexattr_destroy"): pthread_mutexattr_destroy,
		dict.SID("pthread_mutexattr_init"):    pthread_mutexattr_init,
		dict.SID("pthread_mutexattr_settype"): pthread_mutexattr_settype,
		dict.SID("pthread_self"):              pthread_self,
	})
}

type mu struct {
	attr  int32
	count int
	inner sync.Mutex
	outer sync.Mutex
	owner uintptr
}

type mutexMap struct {
	m map[uintptr]*mu
	sync.Mutex
}

func (m *mutexMap) mu(p uintptr) *mu {
	m.Lock()
	r := m.m[p]
	if r == nil {
		r = &mu{}
		m.m[p] = r
	}
	m.Unlock()
	return r
}

var (
	mutexes = &mutexMap{m: map[uintptr]*mu{}}
)

// extern int pthread_equal(pthread_t __thread1, pthread_t __thread2);
func (c *cpu) pthreadEqual() {
	sp, thread2 := popLong(c.sp)
	switch {
	case readLong(sp) == thread2:
		writeI32(c.rp, 1)
	default:
		writeI32(c.rp, 0)
	}
}

// extern int pthread_mutex_destroy(pthread_mutex_t * __mutex);
func (c *cpu) pthreadMutexDestroy() {
	mutexes.Lock()
	delete(mutexes.m, readPtr(c.sp))
	mutexes.Unlock()
}

// extern int pthread_mutex_init(pthread_mutex_t * __mutex, pthread_mutexattr_t * __mutexattr);
func (c *cpu) pthreadMutexInit() {
	sp, mutexattr := popPtr(c.sp)
	attr := int32(pthread.XPTHREAD_MUTEX_NORMAL)
	if mutexattr != 0 {
		attr = readI32(mutexattr)
	}
	mutexes.mu(readPtr(sp)).attr = attr
	writeI32(c.rp, 0)
}

// extern int pthread_mutex_lock(pthread_mutex_t * __mutex);
func (c *cpu) pthreadMutexLock() {
	threadID := c.tlsp.threadID
	mu := mutexes.mu(readPtr(c.sp))
	mu.outer.Lock()
	switch mu.attr {
	case pthread.XPTHREAD_MUTEX_NORMAL:
		mu.owner = threadID
		mu.count = 1
		mu.inner.Lock()
	case pthread.XPTHREAD_MUTEX_RECURSIVE:
		switch mu.owner {
		case 0:
			mu.owner = threadID
			mu.count = 1
			mu.inner.Lock()
		case threadID:
			mu.count++
		default:
			panic("TODO105")
		}
	default:
		panic(fmt.Errorf("attr %#x", mu.attr))
	}
	mu.outer.Unlock()
	writeI32(c.rp, 0)
}

// int pthread_mutex_trylock(pthread_mutex_t *mutex);
func (c *cpu) pthreadMutexTryLock() {
	threadID := c.tlsp.threadID
	mu := mutexes.mu(readPtr(c.sp))
	mu.outer.Lock()
	switch mu.attr {
	case pthread.XPTHREAD_MUTEX_NORMAL:
		switch mu.owner {
		case 0:
			mu.owner = threadID
			mu.count = 1
			mu.inner.Lock()
		case threadID:
			panic("TODO127")
		default:
			panic("TODO129")
		}
	default:
		panic(fmt.Errorf("attr %#x", mu.attr))
	}
	mu.outer.Unlock()
	writeI32(c.rp, 0)
}

// extern int pthread_mutex_unlock(pthread_mutex_t * __mutex);
func (c *cpu) pthreadMutexUnlock() {
	threadID := c.tlsp.threadID
	mu := mutexes.mu(readPtr(c.sp))
	var r int32
	mu.outer.Lock()
	switch mu.attr {
	case pthread.XPTHREAD_MUTEX_NORMAL:
		mu.owner = 0
		mu.count = 0
		mu.inner.Unlock()
	case pthread.XPTHREAD_MUTEX_RECURSIVE:
		switch mu.owner {
		case 0:
			panic("TODO140")
		case threadID:
			mu.count--
			if mu.count != 0 {
				break
			}

			mu.owner = 0
			mu.inner.Unlock()
		default:
			panic("TODO144")
		}
	default:
		panic(fmt.Errorf("TODO %#x", mu.attr))
	}
	mu.outer.Unlock()
	writeI32(c.rp, r)
}

// extern int pthread_mutexattr_destroy(pthread_mutexattr_t * __attr);
func (c *cpu) pthreadMutexAttrDestroy() {
	writeI32(readPtr(c.sp), -1)
	writeI32(c.rp, 0)
}

// extern int pthread_mutexattr_init(pthread_mutexattr_t * __attr);
func (c *cpu) pthreadMutexAttrInit() {
	writeI32(readPtr(c.sp), 0)
	writeI32(c.rp, 0)
}

// extern int pthread_mutexattr_settype(pthread_mutexattr_t * __attr, int __kind);
func (c *cpu) pthreadMutexAttrSetType() {
	sp, kind := popI32(c.sp)
	writeI32(readPtr(sp), kind)
	writeI32(c.rp, 0)
}

// pthread_t pthread_self(void);
func (c *cpu) pthreadSelf() { writeULong(c.rp, uint64(c.tlsp.threadID)) }
