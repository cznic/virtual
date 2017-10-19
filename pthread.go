// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

import (
	"fmt"
	"os"
	"sync"

	"github.com/cznic/ccir/libc/errno"
	"github.com/cznic/ccir/libc/pthread"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("pthread_cond_broadcast"): pthread_cond_broadcast,
		dict.SID("pthread_cond_destroy"):   pthread_cond_destroy,
		dict.SID("pthread_cond_init"):      pthread_cond_init,
		dict.SID("pthread_cond_signal"):    pthread_cond_signal,
		dict.SID("pthread_cond_wait"):      pthread_cond_wait,
		dict.SID("pthread_create"):         pthread_create,
		/// 		dict.SID("pthread_detach"):            pthread_detach,
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
	*sync.Cond
	attr  int32
	count int
	owner uintptr
	sync.Mutex
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
		r.Cond = sync.NewCond(&r.Mutex)
		m.m[p] = r
	}
	m.Unlock()
	return r
}

type condMap struct {
	m map[uintptr]*sync.Cond
	sync.Mutex
}

func (m *condMap) cond(p uintptr, mu *mu) *sync.Cond {
	m.Lock()
	r := m.m[p]
	if r == nil {
		r = sync.NewCond(&mu.Mutex)
		m.m[p] = r
	}
	m.Unlock()
	return r
}

var (
	conds   = &condMap{m: map[uintptr]*sync.Cond{}}
	mutexes = &mutexMap{m: map[uintptr]*mu{}}
)

// int pthread_cond_broadcast(pthread_cond_t *cond);
func (c *cpu) pthreadCondBroadcast() {
	cond := readPtr(c.sp)
	mu := &mu{}
	conds.cond(cond, mu).Broadcast()
	var r int32
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_cond_broadcast(%#x) %v\n", cond, r)
	}
	writeI32(c.rp, r)
}

// int pthread_cond_destroy(pthread_cond_t *cond);
func (c *cpu) pthreadCondDestroy() {
	cond := readPtr(c.sp)
	conds.Lock()
	delete(conds.m, cond)
	conds.Unlock()
	var r int32
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_cond_destroy(%#x) %v\n", cond, r)
	}
	writeI32(c.rp, r)
}

// int pthread_cond_init(pthread_cond_t *restrict cond, const pthread_condattr_t *restrict attr);
func (c *cpu) pthreadCondInit() {
	sp, attr := popPtr(c.sp)
	cond := readPtr(sp)
	var r int32
	if attr != 0 {
		panic("TODO")
	}
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_cond_init(%#x, %#x) %v\n", cond, attr, r)
	}
	writeI32(c.rp, r)
}

// int pthread_cond_signal(pthread_cond_t *cond);
func (c *cpu) pthreadCondSignal() {
	cond := readPtr(c.sp)
	mu := &mu{}
	conds.cond(cond, mu).Signal()
	var r int32
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_cond_signal(%#x) %v\n", cond, r)
	}
	writeI32(c.rp, r)
}

// extern int pthread_equal(pthread_t __thread1, pthread_t __thread2);
func (c *cpu) pthreadEqual() {
	sp, thread2 := popLong(c.sp)
	thread1 := readLong(sp)
	var r int32
	if thread1 == thread2 {
		r = 1
	}
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_equal(%#x, %#x) %v\n", thread1, thread2, r)
	}
	writeI32(c.rp, r)
}

// extern int pthread_mutex_destroy(pthread_mutex_t * __mutex);
func (c *cpu) pthreadMutexDestroy() {
	mutex := readPtr(c.sp)
	mutexes.Lock()
	delete(mutexes.m, mutex)
	mutexes.Unlock()
	var r int32
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_mutex_destroy(%#x) %v\n", mutex, r)
	}
	writeI32(c.rp, r)
}

// extern int pthread_mutex_init(pthread_mutex_t * __mutex, pthread_mutexattr_t * __mutexattr);
func (c *cpu) pthreadMutexInit() {
	sp, mutexattr := popPtr(c.sp)
	attr := int32(pthread.XPTHREAD_MUTEX_NORMAL)
	if mutexattr != 0 {
		attr = readI32(mutexattr)
	}
	mutex := readPtr(sp)
	mutexes.mu(mutex).attr = attr
	var r int32
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_mutex_init(%#x, %#x) %v\n", mutex, mutexattr, r)
	}
	writeI32(c.rp, r)
}

// extern int pthread_mutex_lock(pthread_mutex_t * __mutex);
func (c *cpu) pthreadMutexLock() {
	threadID := c.tlsp.threadID
	mutex := readPtr(c.sp)
	mu := mutexes.mu(mutex)
	var r int32
	mu.Lock()
	switch mu.attr {
	case pthread.XPTHREAD_MUTEX_NORMAL:
		if mu.count == 0 {
			mu.owner = threadID
			mu.count = 1
			break
		}

		for mu.count != 0 {
			mu.Cond.Wait()
		}
		mu.owner = threadID
		mu.count = 1
	case pthread.XPTHREAD_MUTEX_RECURSIVE:
		if mu.count == 0 {
			mu.owner = threadID
			mu.count = 1
			break
		}

		if mu.owner == threadID {
			mu.count++
			break
		}

		panic("TODO")
	default:
		panic(fmt.Errorf("attr %#x", mu.attr))
	}
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_mutex_lock(%#x: %+v [thread id %v]) %v\n", mutex, mu, threadID, r)
	}
	mu.Unlock()
	writeI32(c.rp, r)
}

// int pthread_mutex_trylock(pthread_mutex_t *mutex);
func (c *cpu) pthreadMutexTryLock() {
	threadID := c.tlsp.threadID
	mutex := readPtr(c.sp)
	mu := mutexes.mu(mutex)
	var r int32
	mu.Lock()
	switch mu.attr {
	case pthread.XPTHREAD_MUTEX_NORMAL:
		if mu.count == 0 {
			mu.count = 1
			mu.owner = threadID
			break
		}

		r = errno.XEBUSY
	default:
		panic(fmt.Errorf("attr %#x", mu.attr))
	}
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_mutex_trylock(%#x: %+v [thread id %v]) %v\n", mutex, mu, threadID, r)
	}
	mu.Unlock()
	writeI32(c.rp, r)
}

// extern int pthread_mutex_unlock(pthread_mutex_t * __mutex);
func (c *cpu) pthreadMutexUnlock() {
	threadID := c.tlsp.threadID
	mutex := readPtr(c.sp)
	mu := mutexes.mu(mutex)
	var r int32
	mu.Lock()
	switch mu.attr {
	case pthread.XPTHREAD_MUTEX_NORMAL:
		if mu.count == 0 {
			panic("TODO")
		}

		mu.owner = 0
		mu.count = 0
		mu.Cond.Broadcast()
	case pthread.XPTHREAD_MUTEX_RECURSIVE:
		if mu.count == 0 {
			panic("TODO")
		}

		if mu.owner == threadID {
			mu.count--
			if mu.count != 0 {
				break
			}

			mu.owner = 0
			mu.Cond.Broadcast()
			break
		}

		panic("TODO")
	default:
		panic(fmt.Errorf("TODO %#x", mu.attr))
	}
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_mutex_unlock(%#x: %+v [thread id %v]) %v\n", mutex, mu, threadID, r)
	}
	mu.Unlock()
	writeI32(c.rp, r)
}

// extern int pthread_mutexattr_destroy(pthread_mutexattr_t * __attr);
func (c *cpu) pthreadMutexAttrDestroy() {
	var r int32
	attr := readPtr(c.sp)
	writeI32(attr, -1)
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_mutexattr_destroy(%#x) %v\n", attr, r)
	}
	writeI32(c.rp, r)
}

// extern int pthread_mutexattr_init(pthread_mutexattr_t * __attr);
func (c *cpu) pthreadMutexAttrInit() {
	var r int32
	attr := readPtr(c.sp)
	writeI32(attr, 0)
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_mutexattr_init(%#x) %v\n", attr, r)
	}
	writeI32(c.rp, r)
}

// extern int pthread_mutexattr_settype(pthread_mutexattr_t * __attr, int __kind);
func (c *cpu) pthreadMutexAttrSetType() {
	var r int32
	sp, kind := popI32(c.sp)
	attr := readPtr(sp)
	writeI32(attr, kind)
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_mutexattr_settype(%#x, %v) %v\n", attr, kind, r)
	}
	writeI32(c.rp, r)
}

// pthread_t pthread_self(void);
func (c *cpu) pthreadSelf() {
	threadID := uint64(c.tlsp.threadID)
	writeULong(c.rp, threadID)
	if ptrace {
		fmt.Fprintf(os.Stderr, "pthread_self() %v\n", threadID)
	}
}
