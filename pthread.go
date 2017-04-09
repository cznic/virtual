// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package virtual

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
