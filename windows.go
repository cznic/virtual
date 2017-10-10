// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run generator.go

// +build windows

package virtual

import (
	"fmt"
	"os"
	"sync/atomic"
	"syscall"
	"unsafe"
)

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("_InterlockedCompareExchange"): InterlockedCompareExchange,
		dict.SID("_beginthreadex"):              _beginthreadex,
		dict.SID("_endthreadex"):                _endthreadex,
		dict.SID("_msize"):                      _msize,
		dict.SID("GetCurrentThreadId"):          GetCurrentThreadId,
		dict.SID("GetLastError"):                GetLastError,
	})
}

// TODO: implement a generic wide string variant of this
// GoUTF16String converts a wide string to a GOString using
// windows-specific implementations in go's syscall package
func GoUTF16String(s uintptr) string {
	ptr := (*[1 << 20]uint16)(unsafe.Pointer(s))
	return syscall.UTF16ToString(ptr[:])
}

// DWORD WINAPI GetLastError(void);
func (c *cpu) GetLastError() {
	writeI32(c.rp, c.tlsp.errno)
	c.setErrno(0)
}

// DWORD WINAPI GetCurrentThreadId(void);
func (c *cpu) GetCurrentThreadId() {
	writeU32(c.rp, uint32(c.tlsp.threadID))
}

// LONG __cdecl InterlockedCompareExchange(_Inout_ LONG volatile *Destination,_In_ LONG Exchange,_In_ LONG Comparand);
// TODO: figure out if we can bypass a minor race (see below for an explanation)
func (c *cpu) InterlockedCompareExchange() {
	// TODO: memory barrier: https://msdn.microsoft.com/de-de/library/windows/desktop/ms683560(v=vs.85).aspx
	sp, comparand := popI32(c.sp)
	sp, exchange := popI32(sp)
	dest := readPtr(sp)

	if strace {
		fmt.Fprintf(os.Stderr, "InterlockedCompareExchange(%#x, %#x, %#x)\n", comparand, exchange, dest)
	}

	initial := comparand
	if !atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(dest)), comparand, exchange) {
		initial := readI32(dest)
		if initial == comparand {
			// we cannot prevent all cases of races using this implementation, since we have to
			// return the initial value since CompareAndSwapInt32 doesn't return that we have
			// to do a separate read, which is subject to race. such a race did occur here.
			// the caller will compare the return value against initial, which since we didn't
			// swap it has to be different. that's what we enforce here
			// NOTE: this case should only happen very unlikely and won't have any sideffects
			fmt.Fprintln(os.Stderr, "InterlockedCompareExchange: caught race")
			initial = comparand + 1
		}
	}
	writeI32(c.rp, initial)
}

// type mappings
//ty:ptr: HANDLE, LPSECURITY_ATTRIBUTES, LPCRITICAL_SECTION, va_list*, LPCVOID, LPVOID, LPTSTR*, LPDWORD, LPSYSTEM_INFO, LPSYSTEMTIME, LPOSVERSIONINFO, HLOCAL, LPOVERLAPPED, LONG*
//ty:ptr: LARGE_INTEGER*, LPBOOL, HMODULE, FARPROC, LPFILETIME, SIZE_T, PLONG, SYSTEMTIME*
//ty:str: LPCTSTR, LPTSTR, LPWSTR, LPCSTR, LPSTR, LPCWSTR
//ty:int32: DWORD, BOOL, GET_FILEEX_INFO_LEVELS, UINT, int, LONG
//ty:void: void

// defined syscalls
//sys:kernel32: BOOL   	AreFileApisANSI();
//sys:kernel32: HANDLE 	CreateFileA(LPCSTR lpFileName, DWORD dwDesiredAccess, DWORD dwShareMode, LPSECURITY_ATTRIBUTES lpSecurityAttributes, DWORD dwCreationDisposition, DWORD dwFlagsAndAttributes, HANDLE hTemplateFile);
//sys:kernel32: HANDLE 	CreateFileW(LPCWSTR lpFileName, DWORD dwDesiredAccess, DWORD dwShareMode, LPSECURITY_ATTRIBUTES lpSecurityAttributes, DWORD dwCreationDisposition, DWORD dwFlagsAndAttributes, HANDLE hTemplateFile);
//sys:kernel32: HANDLE 	CreateFileMappingA(HANDLE hFile, LPSECURITY_ATTRIBUTES lpAttributes, DWORD flProtect, DWORD dwMaximumSizeHigh, DWORD dwMaximumSizeLow, LPCSTR lpName);
//sys:kernel32: HANDLE 	CreateFileMappingW(HANDLE hFile, LPSECURITY_ATTRIBUTES lpAttributes, DWORD flProtect, DWORD dwMaximumSizeHigh, DWORD dwMaximumSizeLow, LPCWSTR lpName);
//sys:kernel32: HANDLE 	CreateMutexW(LPSECURITY_ATTRIBUTES lpMutexAttributes, BOOL bInitialOwner, LPCTSTR lpName);
//sys:kernel32: BOOL   	CloseHandle(HANDLE hObject);
//sys:kernel32: void   	DeleteCriticalSection(LPCRITICAL_SECTION lpCriticalSection);
//sys:kernel32: BOOL   	DeleteFileA(LPCTSTR lpFileName);
//sys:kernel32: BOOL   	DeleteFileW(LPCTSTR lpFileName);
//sys:kernel32: void   	EnterCriticalSection(LPCRITICAL_SECTION lpCriticalSection);
//sys:kernel32: BOOL   	FlushFileBuffers(HANDLE hFile);
//sys:kernel32: BOOL     FlushViewOfFile(LPCVOID lpBaseAddress, SIZE_T dwNumberOfBytesToFlush);
//sys:kernel32: DWORD  	FormatMessageA(DWORD dwFlags, LPCVOID lpSource, DWORD dwMessageId, DWORD dwLanguageId, LPTSTR lpBuffer, DWORD nSize, va_list* Arguments);
//sys:kernel32: DWORD  	FormatMessageW(DWORD dwFlags, LPCVOID lpSource, DWORD dwMessageId, DWORD dwLanguageId, LPTSTR lpBuffer, DWORD nSize, va_list* Arguments);
//sys:kernel32: BOOL   	FreeLibrary(HMODULE hModule);
//sys:kernel32: DWORD  	GetCurrentProcessId();
//sys:kernel32: BOOL   	GetDiskFreeSpaceA(LPCTSTR lpRootPathName, LPDWORD lpSectorsPerCluster, LPDWORD lpBytesPerSector, LPDWORD lpNumberOfFreeClusters, LPDWORD lpTotalNumberOfClusters);
//sys:kernel32: BOOL   	GetDiskFreeSpaceW(LPCTSTR lpRootPathName, LPDWORD lpSectorsPerCluster, LPDWORD lpBytesPerSector, LPDWORD lpNumberOfFreeClusters, LPDWORD lpTotalNumberOfClusters);
//sys:kernel32: BOOL   	GetFileAttributesExW(LPCTSTR lpFileName, GET_FILEEX_INFO_LEVELS fInfoLevelId, LPVOID lpFileInformation);
//sys:kernel32: DWORD  	GetFileAttributesA(LPCTSTR lpFileName);
//sys:kernel32: DWORD  	GetFileAttributesW(LPCTSTR lpFileName);
//sys:kernel32: DWORD  	GetFileSize(HANDLE hFile, LPDWORD lpFileSizeHigh);
//sys:kernel32: DWORD  	GetFullPathNameA( LPCTSTR lpFileName, DWORD nBufferLength, LPTSTR lpBuffer, LPTSTR* lpFilePart);
//sys:kernel32: DWORD  	GetFullPathNameW( LPCTSTR lpFileName, DWORD nBufferLength, LPTSTR lpBuffer, LPTSTR* lpFilePart);
//sys:kernel32: FARPROC 	GetProcAddress(HMODULE hModule, LPCSTR lpProcName);
//sys:kernel32: HANDLE   GetProcessHeap();
//sys:kernel32: void   	GetSystemInfo(LPSYSTEM_INFO lpSystemInfo);
//sys:kernel32: void   	GetSystemTime(LPSYSTEMTIME lpSystemTime);
//sys:kernel32: void     GetSystemTimeAsFileTime(LPFILETIME lpSystemTimeAsFileTime);
//sys:kernel32: DWORD    GetTempPathA(DWORD nBufferLength, LPTSTR lpBuffer);
//sys:kernel32: DWORD    GetTempPathW(DWORD nBufferLength, LPTSTR lpBuffer);
//sys:kernel32: DWORD  	GetTickCount();
//sys:kernel32: BOOL   	GetVersionExA(LPOSVERSIONINFO lpVersionInfo);
//sys:kernel32: BOOL   	GetVersionExW(LPOSVERSIONINFO lpVersionInfo);
// TODO: we might want to intercept HeapXXX() ourselves? (they are not used by sqlite seemingly btw)
//sys:kernel32: LPVOID 	HeapAlloc(HANDLE hHeap, DWORD dwFlags, SIZE_T dwBytes);
//sys:kernel32: SIZE_T   HeapCompact(HANDLE hHeap, DWORD dwFlags);
//sys:kernel32: HANDLE   HeapCreate(DWORD flOptions, SIZE_T dwInitialSize, SIZE_T dwMaximumSize);
//sys:kernel32: BOOL     HeapDestroy(HANDLE hHeap);
//sys:kernel32: BOOL     HeapFree(HANDLE hHeap, DWORD dwFlags, LPVOID lpMem);
//sys:kernel32: LPVOID   HeapReAlloc(HANDLE hHeap, DWORD dwFlags, LPVOID lpMem, SIZE_T dwBytes);
//sys:kernel32: SIZE_T   HeapSize(HANDLE hHeap, DWORD dwFlags, LPCVOID lpMem);
//sys:kernel32: BOOL     HeapValidate(HANDLE hHeap, DWORD dwFlags, LPCVOID lpMem);
//sys:kernel32: void   	InitializeCriticalSection(LPCRITICAL_SECTION lpCriticalSection);
//sys:kernel32: void   	LeaveCriticalSection(LPCRITICAL_SECTION lpCriticalSection);
//sys:kernel32: HMODULE  LoadLibraryA(LPCTSTR lpFileName);
//sys:kernel32: HMODULE  LoadLibraryW(LPCTSTR lpFileName);
//sys:kernel32: HLOCAL 	LocalFree(HLOCAL hMem);
//sys:kernel32: BOOL     LockFile(HANDLE hFile, DWORD dwFileOffsetLow, DWORD dwFileOffsetHigh, DWORD nNumberOfBytesToLockLow, DWORD nNumberOfBytesToLockHigh);
//sys:kernel32: BOOL   	LockFileEx(HANDLE hFile, DWORD dwFlags, DWORD dwReserved, DWORD nNumberOfBytesToLockLow, DWORD nNumberOfBytesToLockHigh, LPOVERLAPPED lpOverlapped);
//sys:kernel32: LPVOID   MapViewOfFile(HANDLE hFileMappingObject, DWORD dwDesiredAccess, DWORD dwFileOffsetHigh, DWORD dwFileOffsetLow, SIZE_T dwNumberOfBytesToMap);
//sys:kernel32: int 	  	MultiByteToWideChar(UINT CodePage, DWORD dwFlags, LPCSTR lpMultiByteStr,	int cbMultiByte, LPWSTR lpWideCharStr, int cchWideChar);
//sys:kernel32: void     OutputDebugStringA(LPCTSTR lpOutputString);
//sys:kernel32: void     OutputDebugStringW(LPCTSTR lpOutputString);
//sys:kernel32: BOOL   	QueryPerformanceCounter(LARGE_INTEGER* lpPerformanceCount);
//sys:kernel32: BOOL   	ReadFile(HANDLE hFile, LPVOID lpBuffer, DWORD nNumberOfBytesToRead, LPDWORD lpNumberOfBytesRead, LPOVERLAPPED lpOverlapped);
//sys:kernel32: BOOL     SetEndOfFile(HANDLE hFile);
//sys:kernel32: DWORD    SetFilePointer(HANDLE hFile, LONG lDistanceToMove, PLONG lpDistanceToMoveHigh, DWORD dwMoveMethod);
//sys:kernel32: void     Sleep(DWORD dwMilliseconds);
//sys:kernel32: BOOL     SystemTimeToFileTime(SYSTEMTIME* lpSystemTime, LPFILETIME lpFileTime);
//sys:kernel32: BOOL     UnlockFile(HANDLE hFile, DWORD dwFileOffsetLow, DWORD dwFileOffsetHigh, DWORD nNumberOfBytesToUnlockLow, DWORD nNumberOfBytesToUnlockHigh);
//sys:kernel32: BOOL   	UnlockFileEx(HANDLE hFile, DWORD dwReserved, DWORD nNumberOfBytesToUnlockLow, DWORD nNumberOfBytesToUnlockHigh, LPOVERLAPPED lpOverlapped);
//sys:kernel32: BOOL     UnmapViewOfFile(LPCVOID lpBaseAddress);
//sys:kernel32: DWORD    WaitForSingleObject(HANDLE hHandle, DWORD dwMilliseconds);
//sys:kernel32: DWORD    WaitForSingleObjectEx(HANDLE hHandle, DWORD dwMilliseconds, BOOL bAlertable);
//sys:kernel32: int    	WideCharToMultiByte(UINT CodePage, DWORD dwFlags, LPCWSTR lpWideCharStr, int cchWideChar, LPSTR lpMultiByteStr, int cbMultiByte, LPCSTR lpDefaultChar, LPBOOL lpUsedDefaultChar);
//sys:kernel32: BOOL   	WriteFile(HANDLE hFile, LPCVOID lpBuffer, DWORD nNumberOfBytesToWrite, LPDWORD lpNumberOfBytesWritten, LPOVERLAPPED lpOverlapped);
