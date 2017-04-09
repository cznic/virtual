// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"sync"

	"github.com/cznic/ccir/libc"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("pthread_create"):            pthread_create,
		dict.SID("pthread_join"):              pthread_join,
		dict.SID("pthread_mutex_destroy"):     pthread_mutex_destroy,
		dict.SID("pthread_mutex_init"):        pthread_mutex_init,
		dict.SID("pthread_mutex_lock"):        pthread_mutex_lock,
		dict.SID("pthread_mutex_trylock"):     pthread_mutex_trylock,
		dict.SID("pthread_mutex_unlock"):      pthread_mutex_unlock,
		dict.SID("pthread_mutexattr_destroy"): pthread_mutexattr_destroy,
		dict.SID("pthread_mutexattr_init"):    pthread_mutexattr_init,
		dict.SID("pthread_mutexattr_settype"): pthread_mutexattr_settype,
	})
}

type mu struct {
	sync.Mutex
	t *thread

	cnt int

	attr int32
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

// extern int pthread_mutex_destroy(pthread_mutex_t * __mutex);
func (c *cpu) pthreadMutexDestroy() {
	mutexes.Lock()
	delete(mutexes.m, readPtr(c.sp))
	mutexes.Unlock()
}

// extern int pthread_mutex_init(pthread_mutex_t * __mutex, pthread_mutexattr_t * __mutexattr);
func (c *cpu) pthreadMutexInit() {
	sp, mutexattr := popPtr(c.sp)
	mutexes.mu(readPtr(sp)).attr = readI32(mutexattr)
	writeI32(c.rp, 0)
}

// extern int pthread_mutex_lock(pthread_mutex_t * __mutex);
func (c *cpu) pthreadMutexLock() {
	mu := mutexes.mu(readPtr(c.sp))
	switch mu.attr {
	case libc.Xpthread_PTHREAD_MUTEX_NORMAL:
		mu.Lock()
		mu.t = c.thread
		mu.cnt = 0
	case libc.Xpthread_PTHREAD_MUTEX_RECURSIVE:
		switch {
		case c.thread == mu.t:
			mu.cnt++
		default:
			mu.Lock()
			mu.t = c.thread
			mu.cnt = 0
		}
	default:
		panic(fmt.Errorf("attr %#x", mu.attr))
	}
	writeI32(c.rp, 0)
}

// extern int pthread_mutex_unlock(pthread_mutex_t * __mutex);
func (c *cpu) pthreadMutexUnlock() {
	mu := mutexes.mu(readPtr(c.sp))
	var r int32
	switch mu.attr {
	case libc.Xpthread_PTHREAD_MUTEX_NORMAL:
		switch {
		case c.thread == mu.t:
			mu.Unlock()
			mu.t = nil
		default:
			panic("TODO")
		}
	case libc.Xpthread_PTHREAD_MUTEX_RECURSIVE:
		switch {
		case c.thread == mu.t:
			mu.cnt--
			if mu.cnt != 0 {
				break
			}

			mu.Unlock()
			mu.t = nil
		default:
			panic("TODO")
		}
	default:
		panic(fmt.Errorf("TODO %#x", mu.attr))
	}
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
