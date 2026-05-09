// DL-1.2 baseline lint — packages/server-go/internal/api/* 直 import
// borgee-server/internal/store 的文件数量不得超过 baseline (115).
//
// 蓝图 data-layer.md §4 B: handler 路径走 datalayer.Repository / Storage /
// Presence / EventBus interface seam, 不直查 store 层. DL-1.3+ 渐进迁移,
// 锁链反 commit drift (新文件偷懒直 import store 不走 datalayer).
//
// 由 release-gate.yml::dl1-no-direct-store grep step 搬到这里 (#717 整治
// — 真行为 test 替临时字符串 grep, workflow 删).
package api_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDL12_DirectStoreImportBaseline(t *testing.T) {
	t.Parallel()

	const baseline = 115

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
		// 这个 test 自身 grep 字面会触发 self-hit, 排除.
		if filepath.Base(p) == "dl12_direct_store_baseline_test.go" {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		// 真 import (避免 _test.go 里 grep 字面被误算)
		if strings.Contains(string(b), `"borgee-server/internal/store"`) {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	t.Logf("DL-1.2: %d files import borgee-server/internal/store directly under internal/api/ (baseline=%d)", count, baseline)

	if count > baseline {
		t.Errorf("DL-1.2 立场 ② — internal/api 直 import internal/store 文件数 %d > baseline %d (handler 应走 datalayer.* interface seam, DL-1.3+ 渐进迁移单调下降, 反 commit drift)", count, baseline)
	}
}
