//go:build linux

// Package sandbox — Linux landlock LSM sandbox (v0(D) 真启). 替代 v0(C)
// stub. 走 raw syscall (SYS_LANDLOCK_CREATE_RULESET=444, SYS_LANDLOCK_ADD_RULE=445,
// SYS_LANDLOCK_RESTRICT_SELF=446) 不依赖 landlock-lsm/go-landlock 第三方包
// — golang.org/x/sys/unix 提供 LANDLOCK_* 常量足够.
//
// hb-2-v0d-spec.md §0.2: kernel ≥5.13 真 landlock; <5.13 fallback no-op
// + warn (生产 daemon 启动时 main.go 检 sandbox.Apply 错误决是否 abort).
//
// 反约束 (grep 检查 0 hit): cgroups, cgroupv2 — landlock 限 path 已足.

package sandbox

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// landlockRulesetAttr 是 landlock_ruleset_attr struct (kernel 5.13 ABI).
type landlockRulesetAttr struct {
	HandledAccessFS uint64
}

// landlockPathBeneathAttr 是 landlock_path_beneath_attr struct.
type landlockPathBeneathAttr struct {
	AllowedAccess uint64
	ParentFd      int32
	_             int32 // padding aligned to 4
}

const (
	// LANDLOCK_RULE_PATH_BENEATH = 1 (kernel 5.13).
	landlockRulePathBeneath = 1

	// 全 read 类访问 (HB-2 v0(D) 仅 read scope, 反约束 §1.1 写类 100% reject).
	allowedReadAccess = unix.LANDLOCK_ACCESS_FS_READ_FILE |
		unix.LANDLOCK_ACCESS_FS_READ_DIR
)

// Apply 应用 Profile 走 landlock LSM 限制 daemon 真路径访问.
//
// 流程 (kernel man landlock):
//   1. landlock_create_ruleset(attr, sizeof(attr), 0) → ruleset_fd
//   2. for each path: open(path, O_PATH) → fd; landlock_add_rule(ruleset_fd, ...)
//   3. landlock_restrict_self(ruleset_fd, 0)
//   4. close(ruleset_fd)
//
// 错误处理: ENOSYS (kernel <5.13) → return nil + 调用方记 warn; 其他 errno → return err.
func Apply(p Profile) error {
	if len(p.ReadPaths) == 0 {
		// 无 grant 时 fail-closed 起 — 反约束 deny-by-default.
		return restrictEmptyRuleset()
	}

	rulesetFD, err := createRuleset()
	if err != nil {
		if errors.Is(err, syscall.ENOSYS) {
			// kernel 不支持 landlock (≤5.12) — fallback no-op.
			// 生产 main.go 应记 audit log; 此处仅返 nil 让 daemon 起.
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

// restrictEmptyRuleset deny-by-default: 创建 ruleset 但不加任何 rule
// → daemon 真 read 任何路径 都 reject (fail-closed start).
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

// Profile 描述 sandbox 配置 (跨平台 byte-identical struct).
type Profile struct {
	ReadPaths    []string // grants 注入 (host_grants.path 真接)
	AuditLogPath string   // daemon 唯一允许写 (走 OS perms; landlock 限 read 已足)
	TmpCachePath string   // 临时缓存
}

// Platform 锚 — 单测断 build tag 选对.
const Platform = "linux"
