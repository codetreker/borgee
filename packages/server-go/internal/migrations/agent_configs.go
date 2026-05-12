package migrations

import (
	"gorm.io/gorm"
)

// al2a1AgentConfigs is migration v=20 — Phase 4 / AL-2a.1.
//
// Blueprint锚: `agent-lifecycle.md` §2.1 (用户完全自主决定 agent 的 name/
// prompt/能力/model) + `plugin-protocol.md` §1.4 (Borgee-owned field boundary) +
// §1.5 (热更新分级 — 字段下发, AL-2a 不含 BPP frame).
// Acceptance: `docs/qa/acceptance-templates/al-2a.md` §数据契约 (#264).
// R3 决议: AL-2 拆 a/b — AL-2a 只落 config 表 + REST update API; agent 端
// reload 走轮询; BPP `agent_config_update` frame 留给 AL-2b 与 BPP-3 同合.
//
// What this migration does:
//   1. CREATE TABLE agent_configs:
//        - agent_id        TEXT    NOT NULL    (FK users.id agent 行;
//                                                逻辑 FK, 跟 al_3_1 /
//                                                al_4_1 / cv_2_1 / dm_2_1 /
//                                                cv_4_1 / chn_3_1 同模式)
//        - schema_version  INTEGER NOT NULL    (单调递增, AL-2a 4.1.a 并发
//                                                update 末次胜出 + 严格递增,
//                                                防丢失)
//        - blob            TEXT    NOT NULL    (JSON; 仅含蓝图 §1.4 "归
//                                                Borgee 管" 字段 — name /
//                                                avatar / prompt / model /
//                                                能力开关 / 启用状态 /
//                                                memory_ref; runtime-only
//                                                字段如 api_key / temperature /
//                                                token_limit / retry_policy
//                                                **不入** blob, AL-2a 4.1.c
//                                                fail-closed 反向断言)
//        - created_at      INTEGER NOT NULL    (Unix ms)
//        - updated_at      INTEGER NOT NULL    (Unix ms; PATCH 时 server
//                                                stamp, schema_version++)
//        - PRIMARY KEY (agent_id)              (单 agent 单 row, blob 整体
//                                                替换 — canonical blob design)
//   2. CREATE INDEX idx_agent_configs_agent_id
//        ON agent_configs(agent_id) — 显式命名让 EXPLAIN QUERY PLAN 可读
//        (跟 AL-4.1 #398 idx_runtime_owner / CHN-3.1 #410
//        idx_user_channel_layout_user_id 同模式).
//
// 反约束 (al-2a-spec.md + acceptance §数据契约 + 蓝图 §1.4 canonical config design):
//   - 蓝图 §1.4 canonical config design: blob 仅含 Borgee 管字段 (name / avatar / prompt /
//     model / capabilities / enabled / memory_ref). 反向: api_key /
//     temperature / token_limit / retry_policy 等 runtime-only 字段不入
//     blob — schema 层无可视化 (blob 是 TEXT JSON, runtime 校验 fail-closed
//     在 AL-2a.2 server REST API 层 + 4.1.c reflect scan).
//   - 蓝图 §1.5 BPP frame agent_config_update **不在** AL-2a 范围: schema
//     不挂 cursor 列 (跟 al_3_1 / al_4_1 / cv_1_1 / cv_2_1 / dm_2_1 / cv_4_1 /
//     chn_3_1 同模式 — RT-1 envelope cursor 是 frame 路径, AL-2a 走轮询
//     reload 不挂 push frame).
//   - org 隔离: agent_id 是 users.id (CM-1 #176 users 表已有 org_id 列), 跨
//     org 隔离走 server-side ACL (AL-2a.2 PATCH 校验 owner.org_id ==
//     target_agent.org_id), schema 不重复持有 org_id (avoid duplicate state).
//   - 不级联: 表无 ON DELETE CASCADE (跟 chn_3_1 同模式 — agent 行删 →
//     config 行 lazy GC 留 v3+ cron, 不阻塞 user delete 路径).
//   - 单 row per agent: PK (agent_id) 而非 composite — 跟 chn_3_1 复合 PK
//     (user_id, channel_id) 不同, AL-2a 设计是 canonical blob replacement (PATCH
//     语义是 atomic blob swap + version++), 不裂多 row by config_key.
//
// Migration policy: forward-only, no Down(). 表本身 v0 新增, IF NOT EXISTS 守
// idempotency. 跟 al_3_1 / al_4_1 / cv_2_1 / dm_2_1 / cv_4_1 / chn_3_1
// 同模式 逻辑 FK.
//
// v=20 sequencing: CV-2.1 v=14 ✅ (#359) / DM-2.1 v=15 ✅ (#361) / AL-4.1
// v=16 ✅ (#398) / CV-3.1 v=17 ✅ (#396) / CV-4.1 v=18 ✅ (#405) / CHN-3.1
// v=19 ✅ (#410) / **AL-2a.1 v=20** (本 migration, Phase 4 起步).
// registry.go 字面锁.
var al2a1AgentConfigs = Migration{
	Version: 20,
	Name:    "al_2a_1_agent_configs",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS agent_configs (
  agent_id       TEXT    NOT NULL,
  schema_version INTEGER NOT NULL,
  blob           TEXT    NOT NULL,
  created_at     INTEGER NOT NULL,
  updated_at     INTEGER NOT NULL,
  PRIMARY KEY (agent_id)
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_agent_configs_agent_id
			ON agent_configs(agent_id)`).Error; err != nil {
			return err
		}
		return nil
	},
}
