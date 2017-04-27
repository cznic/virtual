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
			// NOTE: this case should only hapen very unlikely and won't have any sideffects
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
//sys: BOOL   	AreFileApisANSI();
//sys: HANDLE 	CreateFileA(LPCTSTR lpFileName, DWORD dwDesiredAccess, DWORD dwShareMode, LPSECURITY_ATTRIBUTES lpSecurityAttributes, DWORD dwCreationDisposition, DWORD dwFlagsAndAttributes, HANDLE hTemplateFile);
//sys: HANDLE 	CreateFileW(LPCTSTR lpFileName, DWORD dwDesiredAccess, DWORD dwShareMode, LPSECURITY_ATTRIBUTES lpSecurityAttributes, DWORD dwCreationDisposition, DWORD dwFlagsAndAttributes, HANDLE hTemplateFile);
//sys: HANDLE 	CreateFileMappingA(HANDLE hFile, LPSECURITY_ATTRIBUTES lpAttributes, DWORD flProtect, DWORD dwMaximumSizeHigh, DWORD dwMaximumSizeLow, LPCTSTR lpName);
//sys: HANDLE 	CreateFileMappingW(HANDLE hFile, LPSECURITY_ATTRIBUTES lpAttributes, DWORD flProtect, DWORD dwMaximumSizeHigh, DWORD dwMaximumSizeLow, LPCTSTR lpName);
//sys: HANDLE 	CreateMutexW(LPSECURITY_ATTRIBUTES lpMutexAttributes, BOOL bInitialOwner, LPCTSTR lpName);
//sys: BOOL   	CloseHandle(HANDLE hObject);
//sys: void   	DeleteCriticalSection(LPCRITICAL_SECTION lpCriticalSection);
//sys: BOOL   	DeleteFileA(LPCTSTR lpFileName);
//sys: BOOL   	DeleteFileW(LPCTSTR lpFileName);
//sys: void   	EnterCriticalSection(LPCRITICAL_SECTION lpCriticalSection);
//sys: BOOL   	FlushFileBuffers(HANDLE hFile);
//sys: BOOL     FlushViewOfFile(LPCVOID lpBaseAddress, SIZE_T dwNumberOfBytesToFlush);
//sys: DWORD  	FormatMessageA(DWORD dwFlags, LPCVOID lpSource, DWORD dwMessageId, DWORD dwLanguageId, LPTSTR lpBuffer, DWORD nSize, va_list* Arguments);
//sys: DWORD  	FormatMessageW(DWORD dwFlags, LPCVOID lpSource, DWORD dwMessageId, DWORD dwLanguageId, LPTSTR lpBuffer, DWORD nSize, va_list* Arguments);
//sys: BOOL   	FreeLibrary(HMODULE hModule);
//sys: DWORD  	GetCurrentProcessId();
//sys: BOOL   	GetDiskFreeSpaceA(LPCTSTR lpRootPathName, LPDWORD lpSectorsPerCluster, LPDWORD lpBytesPerSector, LPDWORD lpNumberOfFreeClusters, LPDWORD lpTotalNumberOfClusters);
//sys: BOOL   	GetDiskFreeSpaceW(LPCTSTR lpRootPathName, LPDWORD lpSectorsPerCluster, LPDWORD lpBytesPerSector, LPDWORD lpNumberOfFreeClusters, LPDWORD lpTotalNumberOfClusters);
//sys: BOOL   	GetFileAttributesExW(LPCTSTR lpFileName, GET_FILEEX_INFO_LEVELS fInfoLevelId, LPVOID lpFileInformation);
//sys: DWORD  	GetFileAttributesA(LPCTSTR lpFileName);
//sys: DWORD  	GetFileAttributesW(LPCTSTR lpFileName);
//sys: DWORD  	GetFileSize(HANDLE hFile, LPDWORD lpFileSizeHigh);
//sys: DWORD  	GetFullPathNameA( LPCTSTR lpFileName, DWORD nBufferLength, LPTSTR lpBuffer, LPTSTR* lpFilePart);
//sys: DWORD  	GetFullPathNameW( LPCTSTR lpFileName, DWORD nBufferLength, LPTSTR lpBuffer, LPTSTR* lpFilePart);
//sys: FARPROC 	GetProcAddress(HMODULE hModule, LPCSTR lpProcName);
//sys: HANDLE   GetProcessHeap();
//sys: void   	GetSystemInfo(LPSYSTEM_INFO lpSystemInfo);
//sys: void   	GetSystemTime(LPSYSTEMTIME lpSystemTime);
//sys: void     GetSystemTimeAsFileTime(LPFILETIME lpSystemTimeAsFileTime);
//sys: DWORD    GetTempPathA(DWORD nBufferLength, LPTSTR lpBuffer);
//sys: DWORD    GetTempPathW(DWORD nBufferLength, LPTSTR lpBuffer);
//sys: DWORD  	GetTickCount();
//sys: BOOL   	GetVersionExA(LPOSVERSIONINFO lpVersionInfo);
//sys: BOOL   	GetVersionExW(LPOSVERSIONINFO lpVersionInfo);
// TODO: we might want to intercept HeapXXX() ourselves? (they are not used by sqlite seemingly btw)
//sys: LPVOID 	HeapAlloc(HANDLE hHeap, DWORD dwFlags, SIZE_T dwBytes);
//sys: SIZE_T   HeapCompact(HANDLE hHeap, DWORD dwFlags);
//sys: HANDLE   HeapCreate(DWORD flOptions, SIZE_T dwInitialSize, SIZE_T dwMaximumSize);
//sys: BOOL     HeapDestroy(HANDLE hHeap);
//sys: BOOL     HeapFree(HANDLE hHeap, DWORD dwFlags, LPVOID lpMem);
//sys: LPVOID   HeapReAlloc(HANDLE hHeap, DWORD dwFlags, LPVOID lpMem, SIZE_T dwBytes);
//sys: SIZE_T   HeapSize(HANDLE hHeap, DWORD dwFlags, LPCVOID lpMem);
//sys: BOOL     HeapValidate(HANDLE hHeap, DWORD dwFlags, LPCVOID lpMem);
//sys: void   	InitializeCriticalSection(LPCRITICAL_SECTION lpCriticalSection);
//sys: void   	LeaveCriticalSection(LPCRITICAL_SECTION lpCriticalSection);
//sys: HMODULE  LoadLibraryA(LPCTSTR lpFileName);
//sys: HMODULE  LoadLibraryW(LPCTSTR lpFileName);
//sys: HLOCAL 	LocalFree(HLOCAL hMem);
//sys: BOOL     LockFile(HANDLE hFile, DWORD dwFileOffsetLow, DWORD dwFileOffsetHigh, DWORD nNumberOfBytesToLockLow, DWORD nNumberOfBytesToLockHigh);
//sys: BOOL   	LockFileEx(HANDLE hFile, DWORD dwFlags, DWORD dwReserved, DWORD nNumberOfBytesToLockLow, DWORD nNumberOfBytesToLockHigh, LPOVERLAPPED lpOverlapped);
//sys: LPVOID   MapViewOfFile(HANDLE hFileMappingObject, DWORD dwDesiredAccess, DWORD dwFileOffsetHigh, DWORD dwFileOffsetLow, SIZE_T dwNumberOfBytesToMap);
//sys: int 	  	MultiByteToWideChar(UINT CodePage, DWORD dwFlags, LPCSTR lpMultiByteStr,	int cbMultiByte, LPWSTR lpWideCharStr, int cchWideChar);
//sys: void     OutputDebugStringA(LPCTSTR lpOutputString);
//sys: void     OutputDebugStringW(LPCTSTR lpOutputString);
//sys: BOOL   	QueryPerformanceCounter(LARGE_INTEGER* lpPerformanceCount);
//sys: BOOL   	ReadFile(HANDLE hFile, LPVOID lpBuffer, DWORD nNumberOfBytesToRead, LPDWORD lpNumberOfBytesRead, LPOVERLAPPED lpOverlapped);
//sys: BOOL     SetEndOfFile(HANDLE hFile);
//sys: DWORD    SetFilePointer(HANDLE hFile, LONG lDistanceToMove, PLONG lpDistanceToMoveHigh, DWORD dwMoveMethod);
//sys: void     Sleep(DWORD dwMilliseconds);
//sys: BOOL     SystemTimeToFileTime(SYSTEMTIME* lpSystemTime, LPFILETIME lpFileTime);
//sys: BOOL     UnlockFile(HANDLE hFile, DWORD dwFileOffsetLow, DWORD dwFileOffsetHigh, DWORD nNumberOfBytesToUnlockLow, DWORD nNumberOfBytesToUnlockHigh);
//sys: BOOL   	UnlockFileEx(HANDLE hFile, DWORD dwReserved, DWORD nNumberOfBytesToUnlockLow, DWORD nNumberOfBytesToUnlockHigh, LPOVERLAPPED lpOverlapped);
//sys: BOOL     UnmapViewOfFile(LPCVOID lpBaseAddress);
//sys: DWORD    WaitForSingleObject(HANDLE hHandle, DWORD dwMilliseconds);
//sys: DWORD    WaitForSingleObjectEx(HANDLE hHandle, DWORD dwMilliseconds, BOOL bAlertable);
//sys: int    	WideCharToMultiByte(UINT CodePage, DWORD dwFlags, LPCWSTR lpWideCharStr, int cchWideChar, LPSTR lpMultiByteStr, int cbMultiByte, LPCSTR lpDefaultChar, LPBOOL lpUsedDefaultChar);
//sys: BOOL   	WriteFile(HANDLE hFile, LPCVOID lpBuffer, DWORD nNumberOfBytesToWrite, LPDWORD lpNumberOfBytesWritten, LPOVERLAPPED lpOverlapped);
