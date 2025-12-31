//go:build windows

package main

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	procGetUserDefaultLocaleName = kernel32.NewProc("GetUserDefaultLocaleName")
)

const localeNameMaxLength = 85

func detectLocaleWindows() string {
	buf := make([]uint16, localeNameMaxLength)
	r1, _, err := procGetUserDefaultLocaleName.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if r1 == 0 {
		_ = err
		return ""
	}
	n := int(r1)
	if n <= 0 {
		return ""
	}
	if buf[n-1] == 0 {
		n--
	}
	return normalizeLocale(syscall.UTF16ToString(buf[:n]))
}
