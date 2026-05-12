//go:build linux

// Package sandbox applies the Linux Landlock LSM sandbox. It uses raw syscalls
// (SYS_LANDLOCK_CREATE_RULESET=444, SYS_LANDLOCK_ADD_RULE=445,
// SYS_LANDLOCK_RESTRICT_SELF=446) instead of a third-party Landlock wrapper;
// golang.org/x/sys/unix provides the required LANDLOCK_* constants.
//
// hb-2-v0d-spec.md §0.2: kernel >=5.13 applies Landlock; older kernels use
// the documented no-op fallback. daemon startup decides whether Apply errors
// should abort startup.
//
// This layer uses Landlock path restrictions, not cgroups.

package sandbox

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// landlockRulesetAttr mirrors landlock_ruleset_attr from the kernel 5.13 ABI.
type landlockRulesetAttr struct {
	HandledAccessFS uint64
}

// landlockPathBeneathAttr mirrors landlock_path_beneath_attr.
type landlockPathBeneathAttr struct {
	AllowedAccess uint64
	ParentFd      int32
	_             int32 // padding aligned to 4
}

const (
	// LANDLOCK_RULE_PATH_BENEATH = 1 (kernel 5.13).
	landlockRulePathBeneath = 1

	// Read access only; write-class IPC is rejected by ACL.
	allowedReadAccess = unix.LANDLOCK_ACCESS_FS_READ_FILE |
		unix.LANDLOCK_ACCESS_FS_READ_DIR
)

// Apply applies Profile through Landlock LSM path restrictions.
//
// Flow (kernel landlock man page):
//  1. landlock_create_ruleset(attr, sizeof(attr), 0) → ruleset_fd
//  2. for each path: open(path, O_PATH) → fd; landlock_add_rule(ruleset_fd, ...)
//  3. landlock_restrict_self(ruleset_fd, 0)
//  4. close(ruleset_fd)
//
// Error handling: ENOSYS (kernel <5.13) returns nil for the documented fallback;
// other errno values return errors.
func Apply(p Profile) error {
	if len(p.ReadPaths) == 0 {
		// No grants means deny-by-default.
		return restrictEmptyRuleset()
	}

	rulesetFD, err := createRuleset()
	if err != nil {
		if errors.Is(err, syscall.ENOSYS) {
			// Kernel does not support Landlock (<=5.12); use the documented no-op fallback.
			// main.go is responsible for recording the startup warning.
			return nil
		}
		return fmt.Errorf("landlock_create_ruleset: %w", err)
	}
	defer unix.Close(rulesetFD)

	for _, path := range p.ReadPaths {
		if err := addPathBeneathRule(rulesetFD, path); err != nil {
			return fmt.Errorf("landlock_add_rule(%q): %w", path, err)
		}
	}

	if _, _, errno := syscall.Syscall(
		unix.SYS_LANDLOCK_RESTRICT_SELF,
		uintptr(rulesetFD), 0, 0,
	); errno != 0 {
		return fmt.Errorf("landlock_restrict_self: %w", errno)
	}
	return nil
}

func createRuleset() (int, error) {
	attr := landlockRulesetAttr{HandledAccessFS: allowedReadAccess}
	r1, _, errno := syscall.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attr)),
		unsafe.Sizeof(attr),
		0,
	)
	if errno != 0 {
		return -1, errno
	}
	return int(r1), nil
}

func addPathBeneathRule(rulesetFD int, path string) error {
	pathFD, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open(%q, O_PATH): %w", path, err)
	}
	defer unix.Close(pathFD)

	rule := landlockPathBeneathAttr{
		AllowedAccess: allowedReadAccess,
		ParentFd:      int32(pathFD),
	}
	_, _, errno := syscall.Syscall6(
		unix.SYS_LANDLOCK_ADD_RULE,
		uintptr(rulesetFD),
		uintptr(landlockRulePathBeneath),
		uintptr(unsafe.Pointer(&rule)),
		0, 0, 0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// restrictEmptyRuleset starts deny-by-default: create a ruleset without adding
// rules, so daemon reads are rejected fail-closed.
func restrictEmptyRuleset() error {
	attr := landlockRulesetAttr{HandledAccessFS: allowedReadAccess}
	r1, _, errno := syscall.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attr)),
		unsafe.Sizeof(attr),
		0,
	)
	if errno != 0 {
		if errors.Is(errno, syscall.ENOSYS) {
			return nil
		}
		return errno
	}
	defer unix.Close(int(r1))
	if _, _, errno := syscall.Syscall(
		unix.SYS_LANDLOCK_RESTRICT_SELF, r1, 0, 0,
	); errno != 0 {
		return errno
	}
	return nil
}

// Profile describes the sandbox configuration.
type Profile struct {
	ReadPaths    []string // derived from exact host_grants.scope values such as fs:<path>
	AuditLogPath string   // daemon's only write path; OS permissions enforce writes
	TmpCachePath string   // temporary cache path
}

// Platform identifies the Linux implementation selected by this build tag.
const Platform = "linux"
