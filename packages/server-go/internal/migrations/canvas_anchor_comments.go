package migrations

import (
	"gorm.io/gorm"
)

// anchorComments is migration v=14 — Phase 3 / CV-2.1.
//
// Blueprint锚: `canvas-vision.md` §1.4 (artifact 集合) + §1.6 (锚点对话 =
// owner review agent 产物的工具) + §2 v1 不做清单第 5 条 ("段落锚点对话, v2 加").
// Spec brief: `docs/implementation/modules/cv-2-spec.md` (飞马 v0/v1/v2,
// 3 条原则 + 3 拆段). Content lock: `docs/qa/cv-2-content-lock.md` (野马, 设计 ⑤
// 反约束三连 — anchor 仅 owner 视角, agent POST 锚 → 403 anchor.create_owner_only).
// Acceptance skeleton: `docs/qa/acceptance-templates/cv-2.md` (#358).
//
// What this migration does:
//   1. CREATE TABLE artifact_anchors:
//        - id                  TEXT    PRIMARY KEY      (uuid)
//        - artifact_id         TEXT    NOT NULL         (FK artifacts.id;
//                                                       逻辑 FK, 软删随 artifact)
//        - artifact_version_id INTEGER NOT NULL         (FK artifact_versions.id;
//                                                       设计 ② 锚钉死创时 version,
//                                                       artifact 滚下个 version 锚
//                                                       不自动迁移 — review 语境
//                                                       绑死, 否则漂移)
//        - start_offset        INTEGER NOT NULL         (字符索引, ≥0)
//        - end_offset          INTEGER NOT NULL         (字符索引, CHECK end>=start)
//        - created_by          TEXT    NOT NULL         (user_id; 设计 ① 仅 owner /
//                                                       channel 成员可创, 反约束:
//                                                       agent 不能 POST 锚, server
//                                                       403 anchor.create_owner_only)
//        - created_at          INTEGER NOT NULL         (Unix ms)
//        - resolved_at         INTEGER NULL             (NULL = active; 非 NULL =
//                                                       已审, owner / creator 翻)
//        - CHECK (end_offset >= start_offset)           (range 反向校验)
//   2. CREATE TABLE anchor_comments:
//        - id          INTEGER PRIMARY KEY AUTOINCREMENT (全局 audit 序, 同 CV-1.1
//                                                       artifact_versions.id 同模式)
//        - anchor_id   TEXT    NOT NULL                  (FK artifact_anchors.id)
//        - body        TEXT    NOT NULL DEFAULT ''
//        - author_kind TEXT    NOT NULL CHECK ('agent','human')  (注: anchor 是评论
//                                                       作者, 不复用 CV-1
//                                                       artifact_versions.committer_kind
//                                                       命名 — 飞马 spec v2 字面锁;
//                                                       设计 ① 反 agent→agent thread —
//                                                       server 校验 thread 至少有
//                                                       一 author_kind='human' 锚点)
//        - author_id   TEXT    NOT NULL
//        - created_at  INTEGER NOT NULL                  (Unix ms)
//   3. CREATE INDEX idx_anchors_artifact_version
//        ON artifact_anchors(artifact_version_id)        (锚跟 version 绑, 拉
//                                                       per-version active anchors
//                                                       是热路径)
//   4. CREATE INDEX idx_anchor_comments_anchor
//        ON anchor_comments(anchor_id)                   (thread comments 拉取)
//
// 反约束 (cv-2-spec.md §0 + §4):
//   - 不开 agent → agent 锚点对话 (蓝图 §1.6 字面禁; CV-2.2 server 路径校验, schema
//     层不强 enum, 但 author_kind 列名锁让反查 grep 可断 0 hit)
//   - 不做锚点跨版本自动迁移 (设计 ② v3+ 才考虑, schema 用 artifact_version_id FK
//     绑死)
//   - 不做 anchor presence ("谁在看 thread") / typing indicator (留 AL-3 后续 + v3+,
//     schema 不挂 viewer 列)
//   - 不复用 CV-1 committer_kind 列名 (anchor 是评论作者非 commit 提交者; spec v2
//     字面)
//
// v0 stance: forward-only, no Down(). 表本身 v0 新增, IF NOT EXISTS 守 idempotency.
// SQLite FK 默认禁用, artifact_id / artifact_version_id / anchor_id 走逻辑 FK
// (跟 cv_1_1 / al_3_1 / cm_4_0 同模式).
//
// v=14 sequencing 锁 (spec v2 §2): DM-2.1 / CV-2.1 / CHN-2.1 三方候选, 真先到先拿;
// 战马B (DM-2.1) ~6h 未回报, team-lead 派战马A 抢 v=14, DM-2.1 顺延 v=15, AL-4.1
// v=16 (CHN-2.1 无 schema 改, 软约束在 server, 不抢号).
var anchorComments = Migration{
	Version: 14,
	Name:    "cv_2_1_anchor_comments",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS artifact_anchors (
  id                  TEXT    PRIMARY KEY,
  artifact_id         TEXT    NOT NULL,
  artifact_version_id INTEGER NOT NULL,
  start_offset        INTEGER NOT NULL,
  end_offset          INTEGER NOT NULL,
  created_by          TEXT    NOT NULL,
  created_at          INTEGER NOT NULL,
  resolved_at         INTEGER,
  CHECK (end_offset >= start_offset)
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_anchors_artifact_version
			ON artifact_anchors(artifact_version_id)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS anchor_comments (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  anchor_id   TEXT    NOT NULL,
  body        TEXT    NOT NULL DEFAULT '',
  author_kind TEXT    NOT NULL CHECK (author_kind IN ('agent','human')),
  author_id   TEXT    NOT NULL,
  created_at  INTEGER NOT NULL
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_anchor_comments_anchor
			ON anchor_comments(anchor_id)`).Error; err != nil {
			return err
		}
		return nil
	},
}
