//go:build linux

package rootd

import (
	"net"
	"syscall"
)

// peerUIDGID returns the connected peer's uid + primary gid via
// SO_PEERCRED. Linux-only; macOS uses getpeereid (peercred_darwin.go).
func peerUIDGID(uc *net.UnixConn) (uint32, uint32, error) {
	raw, err := uc.SyscallConn()
	if err != nil {
		return 0, 0, err
	}
	var ucred *syscall.Ucred
	var sockErr error
	err = raw.Control(func(fd uintptr) {
		ucred, sockErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil {
		return 0, 0, err
	}
	if sockErr != nil {
		return 0, 0, sockErr
	}
	return ucred.Uid, ucred.Gid, nil
}
