// A bunch of stubs
//
// +build !windows

package virtual

import "fmt"

func unsupportedWin(name string) {
	panic(fmt.Errorf("%s not supported on linux", name))
}

func (c *cpu) CreateFileW() {
	unsupportedWin("CreateFileW")
}

func (c *cpu) CloseHandle() {
	unsupportedWin("CloseHandle")
}

func (c *cpu) DeleteCriticalSection() {
	unsupportedWin("DeleteCriticalSection")
}

func (c *cpu) EnterCriticalSection() {
	unsupportedWin("EnterCriticalSection")
}

func (c *cpu) FormatMessageW() {
	unsupportedWin("FormatMessageW")
}

func (c *cpu) GetCurrentProcessId() {
	unsupportedWin("GetCurrentProcessId")
}

func (c *cpu) GetCurrentThreadId() {
	unsupportedWin("GetCurrentThreadId")
}

func (c *cpu) GetSystemInfo() {
	unsupportedWin("GetSystemInfo")
}

func (c *cpu) GetFileAttributesExW() {
	unsupportedWin("GetFileAttributesExW")
}

func (c *cpu) GetLastError() {
	unsupportedWin("GetLastError")
}

func (c *cpu) GetFullPathNameW() {
	unsupportedWin("GetFullPathNameW")
}

func (c *cpu) GetVersionExA() {
	unsupportedWin("GetVersionExA")
}

func (c *cpu) InterlockedCompareExchange() {
	unsupportedWin("InterlockedCompareExchange")
}

func (c *cpu) InitializeCriticalSection() {
	unsupportedWin("InitializeCriticalSection")
}

func (c *cpu) LeaveCriticalSection() {
	unsupportedWin("LeaveCriticalSection")
}

func (c *cpu) LocalFree() {
	unsupportedWin("LocalFree")
}

func (c *cpu) MultiByteToWideChar() {
	unsupportedWin("MultiByteToWideChar")
}

func (c *cpu) ReadFile() {
	unsupportedWin("ReadFile")
}

func (c *cpu) WideCharToMultiByte() {
	unsupportedWin("WideCharToMultiByte")
}
