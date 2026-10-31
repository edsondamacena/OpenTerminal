package apperr

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

// No Windows a pilha de rede devolve códigos Winsock (WSAE*), não os errno POSIX.
var errnoCodes = map[syscall.Errno]Code{
	windows.WSAECONNREFUSED: ConnectRefused,
	windows.WSAEHOSTUNREACH: HostUnreachable,
	windows.WSAENETUNREACH:  HostUnreachable,
	windows.WSAETIMEDOUT:    ConnectTimeout,
	windows.WSAEADDRINUSE:   PortInUse,
	windows.WSAEACCES:       PermissionDenied,
	windows.WSAECONNRESET:   SessionClosed,
	windows.WSAECONNABORTED: SessionClosed,
}

func errnoCode(err error) (Code, bool) {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return "", false
	}
	c, ok := errnoCodes[errno]
	return c, ok
}
