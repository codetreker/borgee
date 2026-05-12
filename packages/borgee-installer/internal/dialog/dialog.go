// Package dialog builds the HB-1B-INSTALLER permission confirmation dialog.
//
// Per hb-1b-installer-spec §0.2 required item 3: the four grant_type literals must
// stay byte-identical with the HB-3 #520 host_grants schema CHECK enum
// (read/write/exec/network). Any change must also update the server migration
// host_grants v=24 CHECK constraint and the GrantTypes list below.
//
// Installer commands use platform-native tools through os/exec: zenity or
// kdialog on Linux, and osascript on macOS. Unit tests use Confirm with
// injected io.Reader/io.Writer values so they do not block on GUI prompts.
package dialog

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// GrantTypes is the four-value source of truth that matches the HB-3 #520
// host_grants CHECK enum byte-for-byte. Changes must update server migrations,
// this slice, and the REG-HB1B-005 source-text check.
var GrantTypes = []string{
	"read",
	"write",
	"exec",
	"network",
}

// PromptText renders the native dialog body with all four grant_type values and
// an explicit user confirmation. REG-HB1B-005 checks source text for
// `grant_type.*read|grant_type.*write|grant_type.*exec|grant_type.*network` in
// dialog.go.
func PromptText() string {
	var b strings.Builder
	b.WriteString("Borgee Helper 安装 — 权限确认\n\n")
	b.WriteString("borgee-helper daemon 将获得以下宿主能力:\n")
	for _, gt := range GrantTypes {
		switch gt {
		case "read":
			b.WriteString("  • grant_type=read    : 读用户 home + project 目录文件\n")
		case "write":
			b.WriteString("  • grant_type=write   : 写指定 sandbox 目录 (启动后 landlock 限定)\n")
		case "exec":
			b.WriteString("  • grant_type=exec    : 启动 plugin 子进程 (sandbox-exec 限定)\n")
		case "network":
			b.WriteString("  • grant_type=network : 出站 HTTPS 到 Borgee server (无入站)\n")
		}
	}
	b.WriteString("\n输入 'y' 确认安装, 任意其他键取消:\n")
	return b.String()
}

// Confirm uses injected io.Reader/io.Writer values. Installer command packages
// pass os.Stdin/Stdout through the native dialog wrapper, while unit tests pass
// strings.Reader values.
func Confirm(in io.Reader, out io.Writer) (bool, error) {
	if _, err := fmt.Fprint(out, PromptText()); err != nil {
		return false, err
	}
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	resp := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return resp == "y" || resp == "yes", nil
}
