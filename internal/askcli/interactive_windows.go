//go:build windows

package askcli

import "syscall"

func isCharDevice(fd uintptr, name string) bool {
	_ = name
	typ, err := syscall.GetFileType(syscall.Handle(fd))
	if err != nil {
		return false
	}
	return typ == syscall.FILE_TYPE_CHAR
}
