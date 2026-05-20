//go:build darwin

package rootd

import (
	"net"

	"golang.org/x/sys/unix"
)

// peerUIDGID returns the connected peer's effective uid + gid via
// getpeereid(2). macOS does not expose SO_PEERCRED in the same shape as
// Linux; getpeereid is the supported equivalent and is the same call
// libraries like grpc use for macOS peer-cred checks.
func peerUIDGID(uc *net.UnixConn) (uint32, uint32, error) {
	raw, err := uc.SyscallConn()
	if err != nil {
		return 0, 0, err
	}
	var uid, gid uint32
	var sockErr error
	err = raw.Control(func(fd uintptr) {
		var u, g uint32
		u, g, sockErr = unix.Getpeereid(int(fd))
		uid, gid = u, g
	})
	if err != nil {
		return 0, 0, err
	}
	if sockErr != nil {
		return 0, 0, sockErr
	}
	return uid, gid, nil
}
