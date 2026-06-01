//go:build !windows

package askcli

import (
	"math"
	"syscall"
)

func isCharDevice(fd uintptr, name string) bool {
	_ = name
	if fd > math.MaxInt {
		return false
	}
	var stat syscall.Stat_t
	if err := syscall.Fstat(int(fd), &stat); err != nil {
		return false
	}
	return stat.Mode&syscall.S_IFMT == syscall.S_IFCHR
}
