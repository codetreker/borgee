package migrations

import (
	"gorm.io/gorm"
)

// artifacts is migration v=13 — Phase 4 / CV-1.1.
//
// Blueprint锚: `canvas-vision.md` §0 (channel 围 artifact 协作) +
// §1.1-§1.6 (D-lite + workspace per channel + Markdown ONLY v1) + §2
// (v1 做/不做). Spec brief: `docs/implementation/modules/cv-1-spec.md`
// (飞马 v0, 3 条原则 + 3 拆段). Stance: `docs/qa/cv-1-stance-checklist.md`
// (野马 v0+v0.1, 7 条原则 + 5 黑名单 grep + v0/v1 切换三条件).
//
// What this migration does:
//   1. CREATE TABLE artifacts:
//        - id                  TEXT  PRIMARY KEY        (uuid; channel-scoped)
//        - channel_id          TEXT  NOT NULL           (FK channels.id; 设计 ①
//                                                       归属 = channel, 不是
//                                                       author. 软删随 channel)
//        - type                TEXT  NOT NULL CHECK 'markdown'  (设计 ④ Markdown
//                                                       ONLY v1; 代码/图片/PDF/
//                                                       看板留 v2+)
//        - title               TEXT  NOT NULL
//        - body                TEXT  NOT NULL DEFAULT ''  (current rendered body)
//        - current_version     INTEGER NOT NULL DEFAULT 1
//        - created_at          INTEGER NOT NULL          (Unix ms)
//        - archived_at         INTEGER NULL              (软删 — channel archive
//                                                       级联走 list 过滤, 蓝图 §2)
//        - lock_holder_user_id TEXT  NULL                (设计 ② 单文档锁
//                                                       30s TTL, CV-1.2 PATCH
//                                                       conflict 409 路径; NULL
//                                                       = 无人持锁可写)
//        - lock_acquired_at    INTEGER NULL              (设计 ② 锁时间戳, 配合
//                                                       lock_holder_user_id 走
//                                                       30s TTL 过期判断)
//   2. CREATE TABLE artifact_versions:
//        - id                       INTEGER PRIMARY KEY AUTOINCREMENT
//        - artifact_id              TEXT  NOT NULL       (FK artifacts.id)
//        - version                  INTEGER NOT NULL
//        - body                     TEXT  NOT NULL DEFAULT ''
//        - committer_kind           TEXT  NOT NULL CHECK ('agent','human')  (设计 ⑥
//                                                       agent commit fanout
//                                                       system message 路径)
//        - committer_id             TEXT  NOT NULL       (user_id / agent_id)
//        - created_at               INTEGER NOT NULL     (Unix ms)
//        - rolled_back_from_version INTEGER NULL         (设计 ⑦ rollback 触发
//                                                       新 commit 时记原 version,
//                                                       NULL = 普通 commit; CV-1.2
//                                                       POST /rollback 路径写)
//        - UNIQUE(artifact_id, version)                 (版本线性, 设计 ③)
//   3. CREATE INDEX idx_artifacts_channel_id        (channel 内 list 热路径)
//   4. CREATE INDEX idx_artifact_versions_artifact_id (版本侧栏拉取)
//
// 反约束 (cv-1-spec.md §3 + §4):
//   - artifacts 不挂 owner_id 主权列 (设计 ①, 归属 = channel)
//   - artifacts.type 仅 'markdown' (设计 ④, 蓝图 §2 字面)
//   - 不挂 cursor 列 (CV-1.2 ArtifactUpdated frame 走 RT-1.1 cursor 单调,
//     不在数据契约层混淆; 跟 AL-3 presence 拆死同模式)
//   - artifact_versions 无 GC / 不删中间版本 (设计 ③ v0 不限期)
//
// v0 stance: forward-only, no Down(). 表本身 v0 新增, IF NOT EXISTS
// 守 idempotency. SQLite FK 默认禁用, channel_id / artifact_id 走逻辑
// FK (跟 al_3_1 / cm_4_0 同模式).
var artifacts = Migration{
	Version: 13,
	Name:    "cv_1_1_artifacts",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS artifacts (
  id                  TEXT    PRIMARY KEY,
  channel_id          TEXT    NOT NULL,
  type                TEXT    NOT NULL CHECK (type = 'markdown'),
  title               TEXT    NOT NULL,
  body                TEXT    NOT NULL DEFAULT '',
  current_version     INTEGER NOT NULL DEFAULT 1,
  created_at          INTEGER NOT NULL,
  archived_at         INTEGER,
  lock_holder_user_id TEXT,
  lock_acquired_at    INTEGER
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_artifacts_channel_id
			ON artifacts(channel_id)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS artifact_versions (
  id                       INTEGER PRIMARY KEY AUTOINCREMENT,
  artifact_id              TEXT    NOT NULL,
  version                  INTEGER NOT NULL,
  body                     TEXT    NOT NULL DEFAULT '',
  committer_kind           TEXT    NOT NULL CHECK (committer_kind IN ('agent','human')),
  committer_id             TEXT    NOT NULL,
  created_at               INTEGER NOT NULL,
  rolled_back_from_version INTEGER,
  UNIQUE(artifact_id, version)
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_artifact_versions_artifact_id
			ON artifact_versions(artifact_id)`).Error; err != nil {
			return err
		}
		return nil
	},
}
