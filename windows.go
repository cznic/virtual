// Copyright 2017 The Virtual Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// +build windows

package virtual

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

var (
	modkernel32              = syscall.NewLazyDLL("kernel32.dll")
	procCreateFileW          = modkernel32.NewProc("CreateFileW")
	procCloseHandle          = modkernel32.NewProc("CloseHandle")
	procFormatMessageW       = modkernel32.NewProc("FormatMessageW")
	procGetFileAttributesExW = modkernel32.NewProc("GetFileAttributesExW")
	procGetFullPathNameW     = modkernel32.NewProc("GetFullPathNameW")
	procGetLastError         = modkernel32.NewProc("GetLastError")
	procGetSystemInfo        = modkernel32.NewProc("GetSystemInfo")
	procGetVersionExA        = modkernel32.NewProc("GetVersionExA")
	procLocalFree            = modkernel32.NewProc("LocalFree")
	procMultiByteToWideChar  = modkernel32.NewProc("MultiByteToWideChar")
	procReadFile             = modkernel32.NewProc("ReadFile")
	procWideCharToMultiByte  = modkernel32.NewProc("WideCharToMultiByte")
	criticalSections         = criticalSectionsMap{m: map[uintptr]*criticalSection{}}
)

type criticalSectionsMap struct {
	m map[uintptr]*criticalSection
	sync.Mutex
}

func init() {
	registerBuiltins(map[int]Opcode{
		dict.SID("_beginthreadex"):              _beginthreadex,
		dict.SID("_endthreadex"):                _endthreadex,
		dict.SID("_msize"):                      _msize,
		dict.SID("AreFileApisANSI"):             AreFileApisANSI,
		dict.SID("CloseHandle"):                 CloseHandle,
		dict.SID("CreateFileA"):                 CreateFileA,
		dict.SID("CreateFileW"):                 CreateFileW,
		dict.SID("CreateFileMappingA"):          CreateFileMappingA,
		dict.SID("CreateFileMappingW"):          CreateFileMappingW,
		dict.SID("CreateMutexW"):                CreateMutexW,
		dict.SID("DeleteCriticalSection"):       DeleteCriticalSection,
		dict.SID("DeleteFileA"):                 DeleteFileA,
		dict.SID("DeleteFileW"):                 DeleteFileW,
		dict.SID("EnterCriticalSection"):        EnterCriticalSection,
		dict.SID("FlushFileBuffers"):            FlushFileBuffers,
		dict.SID("FlushViewOfFile"):             FlushViewOfFile,
		dict.SID("FormatMessageA"):              FormatMessageA,
		dict.SID("FormatMessageW"):              FormatMessageW,
		dict.SID("FreeLibrary"):                 FreeLibrary,
		dict.SID("GetCurrentProcessId"):         GetCurrentProcessId,
		dict.SID("GetCurrentThreadId"):          GetCurrentThreadId,
		dict.SID("GetDiskFreeSpaceA"):           GetDiskFreeSpaceA,
		dict.SID("GetDiskFreeSpaceW"):           GetDiskFreeSpaceW,
		dict.SID("GetFileAttributesA"):          GetFileAttributesA,
		dict.SID("GetFileAttributesW"):          GetFileAttributesW,
		dict.SID("GetFileAttributesExW"):        GetFileAttributesExW,
		dict.SID("GetFileSize"):                 GetFileSize,
		dict.SID("GetFullPathNameA"):            GetFullPathNameA,
		dict.SID("GetFullPathNameW"):            GetFullPathNameW,
		dict.SID("GetLastError"):                GetLastError,
		dict.SID("GetProcAddress"):              GetProcAddress,
		dict.SID("GetProcessHeap"):              GetProcessHeap,
		dict.SID("GetSystemInfo"):               GetSystemInfo,
		dict.SID("GetSystemTime"):               GetSystemTime,
		dict.SID("GetSystemTimeAsFileTime"):     GetSystemTimeAsFileTime,
		dict.SID("GetTempPathA"):                GetTempPathA,
		dict.SID("GetTempPathW"):                GetTempPathW,
		dict.SID("GetTickCount"):                GetTickCount,
		dict.SID("GetVersionExA"):               GetVersionExA,
		dict.SID("GetVersionExW"):               GetVersionExW,
		dict.SID("HeapAlloc"):                   HeapAlloc,
		dict.SID("HeapCreate"):                  HeapCreate,
		dict.SID("HeapCompact"):                 HeapCompact,
		dict.SID("HeapDestroy"):                 HeapDestroy,
		dict.SID("HeapFree"):                    HeapFree,
		dict.SID("HeapReAlloc"):                 HeapReAlloc,
		dict.SID("HeapSize"):                    HeapSize,
		dict.SID("HeapValidate"):                HeapValidate,
		dict.SID("_InterlockedCompareExchange"): InterlockedCompareExchange,
		dict.SID("InitializeCriticalSection"):   InitializeCriticalSection,
		dict.SID("LoadLibraryA"):                LoadLibraryA,
		dict.SID("LoadLibraryW"):                LoadLibraryW,
		dict.SID("LocalFree"):                   LocalFree,
		dict.SID("LockFile"):                    LockFile,
		dict.SID("LockFileEx"):                  LockFileEx,
		dict.SID("LeaveCriticalSection"):        LeaveCriticalSection,
		dict.SID("MapViewOfFile"):               MapViewOfFile,
		dict.SID("MultiByteToWideChar"):         MultiByteToWideChar,
		dict.SID("OutputDebugStringA"):          OutputDebugStringA,
		dict.SID("OutputDebugStringW"):          OutputDebugStringW,
		dict.SID("QueryPerformanceCounter"):     QueryPerformanceCounter,
		dict.SID("ReadFile"):                    ReadFile,
		dict.SID("SetEndOfFile"):                SetEndOfFile,
		dict.SID("SetFilePointer"):              SetFilePointer,
		dict.SID("Sleep"):                       Sleep,
		dict.SID("SystemTimeToFileTime"):        SystemTimeToFileTime,
		dict.SID("UnlockFile"):                  UnlockFile,
		dict.SID("UnlockFileEx"):                UnlockFileEx,
		dict.SID("UnmapViewOfFile"):             UnmapViewOfFile,
		dict.SID("WaitForSingleObject"):         WaitForSingleObject,
		dict.SID("WaitForSingleObjectEx"):       WaitForSingleObjectEx,
		dict.SID("WideCharToMultiByte"):         WideCharToMultiByte,
		dict.SID("WriteFile"):                   WriteFile,
	})
}

// 	HANDLE WINAPI CreateFile(_In_     LPCTSTR lpFileName, _In_ DWORD dwDesiredAccess,_In_ DWORD dwShareMode, _In_opt_ LPSECURITY_ATTRIBUTES lpSecurityAttributes,
// 		_In_ DWORD dwCreationDisposition,_In_ DWORD dwFlagsAndAttributes, _In_opt_ HANDLE hTemplateFile);
func (c *cpu) CreateFileW() {
	sp, hTemplateFile := popPtr(c.sp)
	sp, dwFlagsAndAttributes := popI32(sp)
	sp, dwCreationDisposition := popI32(sp)
	sp, lpSecurityAttributes := popPtr(sp)
	sp, dwShareMode := popI32(sp)
	sp, dwDesiredAccess := popI32(sp)
	lpFileName := readPtr(sp)

	ret, _, err := syscall.Syscall9(procCreateFileW.Addr(), 7,
		lpFileName,
		uintptr(dwDesiredAccess),
		uintptr(dwShareMode),
		lpSecurityAttributes,
		uintptr(dwCreationDisposition),
		uintptr(dwFlagsAndAttributes),
		hTemplateFile,
		0,
		0)

	if err != 0 {
		c.setErrno(err)
	}
	writePtr(c.rp, ret)
}

// BOOL WINAPI CloseHandle( _In_ HANDLE hObject);
func (c *cpu) CloseHandle() {
	handle := readPtr(c.sp)
	ret, _, err := syscall.Syscall(procCloseHandle.Addr(), 1, handle, 0, 0)
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(ret))
}

// CRITICAL SECTIONS are mutexes that can only be held by one process (while mutexes can be held by multiple processes)
//     typedef struct _RTL_CRITICAL_SECTION {
//       PRTL_CRITICAL_SECTION_DEBUG DebugInfo;
//		 LONG LockCount;
//       LONG RecursionCount;
//       HANDLE OwningThread;
//       HANDLE LockSemaphore;
//       ULONG_PTR SpinCount;
//    } RTL_CRITICAL_SECTION,*PRTL_CRITICAL_SECTION;
//
// https://msdn.microsoft.com/en-us/library/windows/desktop/ms682530(v=vs.85).aspx
type criticalSection struct {
	debugInfo      uintptr
	lockCount      int32
	recursionCount int32
	owningThread   uintptr
	lockSemaphore  uintptr
	spinCount      uintptr
}

// void WINAPI InitializeCriticalSection(_Out_ LPCRITICAL_SECTION lpCriticalSection);
func (c *cpu) InitializeCriticalSection() {
	// get the pointer to CRITICAL_SECTION
	section := readPtr(c.sp)
	if section == 0 {
		panic("InitializeCriticalSection: got null pointer")
	}
	sec := (*criticalSection)(unsafe.Pointer(section))
	criticalSections.Lock()
	if _, exists := criticalSections.m[section]; exists {
		panic("InitializeCriticalSection: already initialized")
	}
	criticalSections.m[section] = sec
	// TODO: initialize more
	*sec = criticalSection{}
	criticalSections.Unlock()
}

// Waits for ownership of the specified critical section object. The function returns when the calling thread is granted ownership.
// void WINAPI EnterCriticalSection(_Inout_ LPCRITICAL_SECTION lpCriticalSection);
// https://msdn.microsoft.com/de-de/library/windows/desktop/ms682608(v=vs.85).aspx
func (c *cpu) EnterCriticalSection() {
	section := readPtr(c.sp)
	if section == 0 {
		panic("EnterCriticalSection: got null pointer")
	}
	criticalSections.Lock()
	sec, exists := criticalSections.m[section]
	if !exists {
		panic("EnterCriticalSection: uninitialized critical section")
	}
	if sec.owningThread != 0 && sec.owningThread != c.tlsp.threadID {
		panic("EnterCriticalSection: only the trivial case is supported TODO")
	}
	sec.recursionCount++
	sec.owningThread = c.tlsp.threadID
	criticalSections.Unlock()
}

// void WINAPI DeleteCriticalSection(_Inout_ LPCRITICAL_SECTION lpCriticalSection);
func (c *cpu) DeleteCriticalSection() {
	section := readPtr(c.sp)
	if section == 0 {
		panic("InitializeCriticalSection: got null pointer")
	}
	criticalSections.Lock()
	if _, exists := criticalSections.m[section]; !exists {
		panic("InitializeCriticalSection: uninitialized critical section")
	}
	delete(criticalSections.m, section)
	criticalSections.Unlock()
}

// Releases ownership of the specified critical section object.
// https://msdn.microsoft.com/de-de/library/windows/desktop/ms684169(v=vs.85).aspx
// void WINAPI LeaveCriticalSection(_Inout_ LPCRITICAL_SECTION lpCriticalSection);
func (c *cpu) LeaveCriticalSection() {
	section := readPtr(c.sp)
	if section == 0 {
		panic("LeaveCriticalSection: got null pointer")
	}
	criticalSections.Lock()
	sec, exists := criticalSections.m[section]
	if !exists {
		panic("LeaveCriticalSection: uninitialized critical section")
	}
	if sec.owningThread == 0 {
		panic("LeaveCriticalSection: trying to leave unowned critical section")
	}
	sec.recursionCount--
	if sec.recursionCount == 0 {
		sec.owningThread = 0
	}
	criticalSections.Unlock()
}

// 	DWORD WINAPI FormatMessage(_In_ DWORD dwFlags,_In_opt_ LPCVOID lpSource,_In_ DWORD dwMessageId,
//		_In_ DWORD dwLanguageId,_Out_ LPTSTR lpBuffer,_In_ DWORD nSize, _In_opt_ va_list *Arguments);
func (c *cpu) FormatMessageW() {
	sp, arguments := popPtr(c.sp)
	sp, nSize := popI32(sp)
	sp, lpBuffer := popPtr(sp)
	sp, dwLanguageId := popI32(sp)
	sp, dwMessageId := popI32(sp)
	sp, lpSource := popPtr(sp)
	dwFlags := readI32(sp)

	ret, _, err := syscall.Syscall9(procFormatMessageW.Addr(),
		7,
		uintptr(dwFlags),
		lpSource,
		uintptr(dwMessageId),
		uintptr(dwLanguageId),
		lpBuffer,
		uintptr(nSize),
		arguments,
		0, 0)

	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(ret))
}

// DWORD WINAPI GetCurrentProcessId(void);
func (c *cpu) GetCurrentProcessId() {
	// TODO
	writeU32(c.rp, uint32(666))
}

// DWORD WINAPI GetCurrentThreadId(void);
func (c *cpu) GetCurrentThreadId() {
	writeU32(c.rp, uint32(c.tlsp.threadID))
}

// BOOL WINAPI GetFileAttributesEx( _In_  LPCTSTR lpFileName, _In_  GET_FILEEX_INFO_LEVELS fInfoLevelId, _Out_ LPVOID  lpFileInformation);
func (c *cpu) GetFileAttributesExW() {
	sp, lpFileInformation := popPtr(c.sp)
	sp, fInfoLevelId := popI32(sp)
	sp, lpFileName := popPtr(sp)

	ret, _, err := syscall.Syscall6(procGetFileAttributesExW.Addr(), 3, lpFileName, uintptr(fInfoLevelId), lpFileInformation, 0, 0, 0)
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(ret))
}

// DWORD WINAPI GetFullPathNameW(_In_  LPCTSTR lpFileName, _In_  DWORD   nBufferLength,_Out_ LPTSTR  lpBuffer,_Out_ LPTSTR  *lpFilePart);
func (c *cpu) GetFullPathNameW() {
	sp, lpFilePart := popPtr(c.sp)
	sp, lpBuffer := popPtr(sp)
	sp, nBufferLength := popI32(sp)
	lpFileName := readPtr(sp)

	ret, _, err := syscall.Syscall6(procGetFullPathNameW.Addr(), 4, lpFileName, uintptr(nBufferLength), lpBuffer, lpFilePart, 0, 0)
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(ret))
}

// DWORD WINAPI GetLastError(void);
func (c *cpu) GetLastError() {
	ret, _, err := syscall.Syscall(procGetLastError.Addr(), 0, 0, 0, 0)
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(ret))
}

// void WINAPI GetSystemInfo(_Out_ LPSYSTEM_INFO lpSystemInfo);
func (c *cpu) GetSystemInfo() {
	lpSystemInfo := readPtr(c.sp)
	_, _, err := syscall.Syscall(procGetSystemInfo.Addr(), 1, lpSystemInfo, 0, 0)
	if err != 0 {
		c.setErrno(err)
	}
}

// BOOL WINAPI GetVersionEx(_Inout_ LPOSVERSIONINFO lpVersionInfo);
func (c *cpu) GetVersionExA() {
	lpVersionInfo := readPtr(c.sp)
	ret, _, err := syscall.Syscall(procGetVersionExA.Addr(), 1, lpVersionInfo, 0, 0)
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(ret))
}

// HLOCAL WINAPI LocalFree(_In_ HLOCAL hMem);
func (c *cpu) LocalFree() {
	hMem := readPtr(c.sp)
	ret, _, err := syscall.Syscall(procLocalFree.Addr(), 1, hMem, 0, 0)
	if err != 0 {
		c.setErrno(err)
	}
	writePtr(c.rp, ret)
}

// 	int MultiByteToWideChar(_In_ UINT CodePage, _In_  DWORD  dwFlags,_In_  LPCSTR lpMultiByteStr,_In_
// 		int cbMultiByte,_Out_opt_ LPWSTR lpWideCharStr,_In_  int cchWideChar);
func (c *cpu) MultiByteToWideChar() {
	sp, cchWideChar := popI32(c.sp)
	sp, lpWideCharStr := popPtr(sp)
	sp, cbMultiByte := popI32(sp)
	sp, lpMultiByteStr := popPtr(sp)
	sp, dwFlags := popI32(sp)
	codePage := readI32(sp)

	ret, _, err := syscall.Syscall6(procMultiByteToWideChar.Addr(),
		6,
		uintptr(codePage),
		uintptr(dwFlags),
		lpMultiByteStr,
		uintptr(cbMultiByte),
		lpWideCharStr,
		uintptr(cchWideChar))
	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(ret))
}

// 	BOOL WINAPI ReadFile(_In_ HANDLE hFile,_Out_ LPVOID lpBuffer, _In_ DWORD nNumberOfBytesToRead,
// 		_Out_opt_ LPDWORD  lpNumberOfBytesRead,_Inout_opt_ LPOVERLAPPED lpOverlapped);
func (c *cpu) ReadFile() {
	sp, lpOverlapped := popPtr(c.sp)
	sp, lpNumberOfBytesRead := popPtr(sp)
	sp, nNumberOfBytesToRead := popI32(sp)
	sp, lpBuffer := popPtr(sp)
	hFile := readPtr(sp)

	ret, _, err := syscall.Syscall6(procReadFile.Addr(),
		5,
		hFile,
		lpBuffer,
		uintptr(nNumberOfBytesToRead),
		lpNumberOfBytesRead,
		lpOverlapped,
		0)
	if err != 0 {
		c.setErrno(err)
	}
	fmt.Println("ReadFile ", ret)
	writeI32(c.rp, int32(ret))
}

// LONG __cdecl InterlockedCompareExchange(_Inout_ LONG volatile *Destination,_In_ LONG Exchange,_In_ LONG Comparand);
func (c *cpu) InterlockedCompareExchange() {
	// TODO: memory barrier: https://msdn.microsoft.com/de-de/library/windows/desktop/ms683560(v=vs.85).aspx
	sp, comparand := popI32(c.sp)
	sp, exchange := popI32(sp)
	dest := readPtr(sp)

	// TODO: currently we don't seem to support multiple threads, so we don't need to ensure
	// atomicity here

	initial := readI32(dest)
	if initial == comparand {
		writeI32(dest, exchange)
	}
	writeI32(c.rp, initial)
}

//	int WideCharToMultiByte(_In_ UINT CodePage,_In_ DWORD dwFlags, _In_ LPCWSTR lpWideCharStr, _In_ int cchWideChar,_Out_opt_ LPSTR lpMultiByteStr,
//		_In_ int cbMultiByte, _In_opt_ LPCSTR lpDefaultChar, _Out_opt_ LPBOOL lpUsedDefaultChar);
func (c *cpu) WideCharToMultiByte() {
	sp, lpUsedDefaultChar := popI32(c.sp)
	sp, lpDefaultChar := popPtr(sp)
	sp, cbMultiByte := popI32(sp)
	sp, lpMultiByteStr := popPtr(sp)
	sp, cchWideChar := popI32(sp)
	sp, lpWideCharStr := popPtr(sp)
	sp, dwFlags := popI32(sp)
	CodePage := readI32(sp)

	ret, _, err := syscall.Syscall9(procWideCharToMultiByte.Addr(),
		8,
		uintptr(CodePage),
		uintptr(dwFlags),
		lpWideCharStr,
		uintptr(cchWideChar),
		lpMultiByteStr,
		uintptr(cbMultiByte),
		lpDefaultChar,
		uintptr(lpUsedDefaultChar),
		0)

	if err != 0 {
		c.setErrno(err)
	}
	writeI32(c.rp, int32(ret))
}
