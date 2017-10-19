// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//TODO strace

package virtual

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	sockconst "github.com/cznic/ccir/libc/sys/socket"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("connect"):     connect,
		dict.SID("getpeername"): getpeername,
		dict.SID("getsockname"): getsockname,
		dict.SID("getsockopt"):  getsockopt,
		dict.SID("recv"):        recv,
		dict.SID("setsockopt"):  setsockopt,
		dict.SID("shutdown"):    shutdown,
		dict.SID("socket"):      socket,
		dict.SID("writev"):      writev,
	})
}

func socketAF(i int32) string {
	switch i {
	case sockconst.XAF_ALG:
		return "AF_ALG"
	case sockconst.XAF_APPLETALK:
		return "AF_APPLETALK"
	case sockconst.XAF_ATMPVC:
		return "AF_ATMPVC"
	case sockconst.XAF_AX25:
		return "AF_AX25"
	case sockconst.XAF_INET:
		return "AF_INET"
	case sockconst.XAF_INET6:
		return "AF_INET6"
	case sockconst.XAF_IPX:
		return "AF_IPX"
	case sockconst.XAF_NETLINK:
		return "AF_NETLINK"
	case sockconst.XAF_PACKET:
		return "AF_PACKET"
	case sockconst.XAF_UNIX:
		return "AF_UNIX"
	case sockconst.XAF_UNSPEC:
		return "AF_UNSPEC"
	case sockconst.XAF_X25:
		return "AF_X25"
	default:
		return fmt.Sprintf("%#x", i)
	}
}

func socketType(i int32) string {
	var a []string
	if i&sockconst.XSOCK_NONBLOCK != 0 {
		a = append(a, "SOCK_NONBLOCK")
	}
	if i&sockconst.XSOCK_CLOEXEC != 0 {
		a = append(a, "SOCK_CLOEXEC")
	}
	switch i &^ (sockconst.XSOCK_NONBLOCK | sockconst.XSOCK_CLOEXEC) {
	case sockconst.XSOCK_DGRAM:
		a = append(a, "SOCK_DGRAM")
	case sockconst.XSOCK_PACKET:
		a = append(a, "SOCK_PACKET")
	case sockconst.XSOCK_RAW:
		a = append(a, "SOCK_RAW")
	case sockconst.XSOCK_RDM:
		a = append(a, "SOCK_RDM")
	case sockconst.XSOCK_SEQPACKET:
		a = append(a, "SOCK_SEQPACKET")
	case sockconst.XSOCK_STREAM:
		a = append(a, "SOCK_STREAM")
	default:
		a = append(a, fmt.Sprintf("%#x", i))
	}
	sort.Strings(a)
	return strings.Join(a, "|")
}

// int connect(int sockfd, const struct sockaddr *addr, socklen_t addrlen);
func (c *cpu) connect() {
	sp, addrlen := popU32(c.sp)
	sp, addr := popPtr(sp)
	fd := readI32(sp)
	_, _, err := syscall.Syscall(syscall.SYS_CONNECT, uintptr(fd), addr, uintptr(addrlen))
	if strace {
		fmt.Fprintf(os.Stderr, "connext(%#x, %#x, %#x) %v\t; %s\n", fd, addr, addrlen, err, c.pos())
	}
	if err != 0 {
		c.setErrno(err)
		writeI32(c.rp, -1)
		return
	}

	writeI32(c.rp, 0)
}

// int getpeername(int sockfd, struct sockaddr *addr, socklen_t *addrlen);
func (c *cpu) getpeername() {
	sp, addrlen := popPtr(c.sp)
	sp, addr := popPtr(sp)
	fd := readI32(sp)
	_, _, err := syscall.Syscall(syscall.SYS_GETPEERNAME, uintptr(fd), addr, addrlen)
	if strace {
		fmt.Fprintf(os.Stderr, "getpeername(%#x, %#x, %#x) %v\t; %s\n", fd, addr, addrlen, err, c.pos())
	}
	if err != 0 {
		c.setErrno(err)
		writeI32(c.rp, -1)
		return
	}

	writeI32(c.rp, 0)
}

// int getsockname(int sockfd, struct sockaddr *addr, socklen_t *addrlen);
func (c *cpu) getsockname() {
	sp, addrlen := popPtr(c.sp)
	sp, addr := popPtr(sp)
	fd := readI32(sp)
	_, _, err := syscall.Syscall(syscall.SYS_GETSOCKNAME, uintptr(fd), addr, addrlen)
	if strace {
		fmt.Fprintf(os.Stderr, "getsockname(%#x, %#x, %#x) %v\t; %s\n", fd, addr, addrlen, err, c.pos())
	}
	if err != 0 {
		c.setErrno(err)
		writeI32(c.rp, -1)
		return
	}

	writeI32(c.rp, 0)
}

// ssize_t recv(int sockfd, void *buf, size_t len, int flags);
func (c *cpu) recv() {
	sp, flags := popI32(c.sp)
	sp, len := popLong(sp)
	sp, buf := popPtr(sp)
	fd := readI32(sp)
	var b []byte
	h := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	h.Cap = int(len)
	h.Data = buf
	h.Len = int(len)
	n, _, err := syscall.Recvfrom(int(fd), b, int(flags))
	if strace {
		fmt.Fprintf(os.Stderr, "recv(%#x, %#x, %#x, %#x) %v %v\t; %s\n", fd, buf, len, flags, n, err, c.pos())
	}
	if err != nil {
		c.setErrno(err)
		writeLong(c.rp, -1)
		return
	}

	writeLong(c.rp, int64(n))
}

// int shutdown(int sockfd, int how);
func (c *cpu) shutdown() {
	sp, how := popI32(c.sp)
	fd := readI32(sp)
	err := syscall.Shutdown(int(fd), int(how))
	if strace {
		fmt.Fprintf(os.Stderr, "shutdown(%#x, %#x) %v\t; %s\n", fd, how, err, c.pos())
	}
	if err != nil {
		c.setErrno(err)
		writeI32(c.rp, -1)
		return
	}

	writeI32(c.rp, 0)
}

// int socket(int domain, int type, int protocol);
func (c *cpu) socket() {
	sp, protocol := popI32(c.sp)
	sp, typ := popI32(sp)
	domain := readI32(sp)
	fd, err := syscall.Socket(int(domain), int(typ), int(protocol))
	if strace {
		fmt.Fprintf(os.Stderr, "socket(%s, %s, %#x) %v %v\t; %s\n", socketAF(domain), socketType(typ), protocol, fd, err, c.pos())
	}
	if err != nil {
		c.setErrno(err)
		writeI32(c.rp, -1)
		return
	}

	writeI32(c.rp, int32(fd))
}

// ssize_t writev(int fd, const struct iovec *iov, int iovcnt);
func (c *cpu) writev() {
	sp, iovcnt := popI32(c.sp)
	sp, iov := popPtr(sp)
	fd := readI32(sp)
	n, _, err := syscall.Syscall(syscall.SYS_WRITEV, uintptr(fd), iov, uintptr(iovcnt))
	if strace {
		fmt.Fprintf(os.Stderr, "writev(%#x, %#x, %#x) %v %v\t; %s\n", fd, iov, iovcnt, n, err, c.pos())
	}
	if err != 0 {
		c.setErrno(err)
		writeLong(c.rp, -1)
		return
	}

	writeLong(c.rp, int64(n))
}
