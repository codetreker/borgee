// DL-1.2 baseline lint — packages/server-go/internal/api/* 直 import
// borgee-server/internal/store 的 production .go 文件数量不得超过 baseline (50).
//
// 蓝图 data-layer.md §4 B: handler 路径走 datalayer.Repository / Storage /
// Presence / EventBus interface seam, 不直查 store 层. DL-1.3+ 渐进迁移,
// 锁链反 commit drift (新文件偷懒直 import store 不走 datalayer).
//
// 范围只锁 production code (不锁 _test.go): test fixture 直 import store 是
// 正常 (setup / AST scan / fixture seed), 不走 datalayer interface seam.
// 跟 liema #722 Q1 review 对账修.
//
// 由 release-gate.yml::dl1-no-direct-store grep step 搬到这里 (#717 整治
// — 真行为 test 替临时字符串 grep, workflow 删).
//
// hard ratchet: 想加新 handler 不走 datalayer, 必须先 PR 把别处一个 handler
// 迁到 datalayer 把数字降 1, 才能加新的. 单调下降, 不允许纯加.
package api_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDL12_DirectStoreImportBaseline(t *testing.T) {
	t.Parallel()

	const baseline = 50 // production only (production code 路径)

	// 从当前 test 文件位置往上找 internal/api 目录
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	apiDir := wd // 已经在 internal/api/

	count := 0
	err = filepath.Walk(apiDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		// 只锁 production 不锁 test fixture (DL-1.2 §4 B 设计只锁 handler 路径).
		if strings.HasSuffix(p, "_test.go") {
			return nil
		}
		// 这个 test 自身 grep 字面会触发 self-hit, 排除 (虽然此 file 是
		// _test.go 已被上面 skip, 留这条防 rename 后误命中).
		if filepath.Base(p) == "dl12_direct_store_baseline_test.go" {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(b), `"borgee-server/internal/store"`) {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	t.Logf("DL-1.2: %d production .go files import borgee-server/internal/store directly under internal/api/ (baseline=%d)", count, baseline)

	if count > baseline {
		t.Errorf("DL-1.2 §4 B — internal/api 直 import internal/store production 文件数 %d > baseline %d (handler 应走 datalayer.* interface seam, DL-1.3+ 渐进迁移单调下降, 反 commit drift)", count, baseline)
	}
}
