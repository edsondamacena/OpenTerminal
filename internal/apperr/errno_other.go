//go:build !windows

package apperr

import (
	"errors"
	"syscall"
)

var errnoCodes = map[syscall.Errno]Code{
	syscall.ECONNREFUSED: ConnectRefused,
	syscall.EHOSTUNREACH: HostUnreachable,
	syscall.ENETUNREACH:  HostUnreachable,
	syscall.ETIMEDOUT:    ConnectTimeout,
	syscall.EADDRINUSE:   PortInUse,
	syscall.EACCES:       PermissionDenied,
	syscall.EPERM:        PermissionDenied,
	syscall.ECONNRESET:   SessionClosed,
}

func errnoCode(err error) (Code, bool) {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return "", false
	}
	c, ok := errnoCodes[errno]
	return c, ok
}
