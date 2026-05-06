# DOCS-CURRENT-CATCHUP 立场反查表 (反实施漂)

> yema · 2026-05-06 · DOCS-CURRENT-CATCHUP milestone prep
> 配套: 飞马 spec brief (`docs/implementation/modules/docs-current-catchup-spec.md`, in-flight) + liema acceptance template
> 用法: 实施期 zhanma 自查 + 飞马/野马 review 反查; 候选每条对应 1 个反查锚 (文件存在 + 关键字面对照)

## 反查表的范围

DOCS-CURRENT-CATCHUP 是**追平 docs/current/ 跟代码现状的偏差**, 不是历史归档, 不是 narrative 重写.

`docs/current/` 是代码现状的 audit 快照 (跟蓝图 SOT 二分), 偏差就修. 4 立场:

1. **规则 6** — 代码改 → docs/current 必同步 (workflow §关键协议, 这次是补历史欠账)
2. **文件命名按功能不按 milestone** (memory: `file_naming_no_milestone_prefix`)
3. **不投隐私 / 合规** — GDPR / 数据主体权利 / Article 15 立场文档 0 hit (memory: `admin_invisible_to_user`)
4. **admin invisible to user-rail** — admin / 管理员 / 审计 / 监督 在 user-rail 文档 0 hit

每候选: 文件存在 + 关键字段字面对照 + 立场 §X.Y 引用 + grep 锚. 反 narrative.

---

## 候选 1: HB-2 (host-bridge daemon)

**真 drift 已锁** (yema 反扫):
- `docs/current/server/api/host-grants.md` L20 + L124 写 "Rust crate" — 实际真实施 `packages/borgee-helper/` 是 **Go** (`go.mod` 头行确认)

**反查锚**:
```bash
# (a) Rust 字面 0 hit (HB-2 daemon 是 Go 不是 Rust)
grep -rE "Rust crate|rust crate|\.rs\b" docs/current/server/api/host-grants.md docs/current/client/host-grants-panel.md  # 0 hit

# (b) borgee-helper 真路径必引 (Go 真实施 #605 落地)
grep -E "packages/borgee-helper/" docs/current/server/api/host-grants.md  # ≥1 hit

# (c) 蓝图引锚必有
grep -E "host-bridge\.md §1\.[1-5]" docs/current/server/api/host-grants.md docs/current/client/host-grants-panel.md  # ≥2 hit
```

**立场引用**: `host-bridge.md` §1.1 (内部双 daemon, UI 合一) + §1.3 (情境化授权 4 类) + §1.4 (v1 不在 Borgee 跑命令)

---

## 候选 2: HB-1B-INSTALLER (真 installer Go binary)

**真 drift 已锁**:
- `packages/borgee-installer/` Go 真实施 (#589 / #627 全 merged) — `docs/current/` **0 引用**, 用户认知不到 installer 已落地

**反查锚**:
```bash
# (a) borgee-installer 路径必出现 (server-side download endpoint + Linux .deb / macOS .pkg)
grep -rE "packages/borgee-installer/|borgee-installer" docs/current/  # ≥1 hit (新建 docs/current/borgee-installer/README.md 或 server/api/installer.md)

# (b) 不许写 "TODO / coming soon / 待实施" — 已 merged
grep -rE "TODO|coming soon|will be|待实施|pending" docs/current/borgee-installer/ docs/current/server/api/installer.md 2>/dev/null  # 0 hit

# (c) 蓝图引锚
grep -E "host-bridge\.md §1\.[1-5]" docs/current/borgee-installer/README.md docs/current/server/api/installer.md 2>/dev/null  # ≥1 hit
```

**立场引用**: `host-bridge.md` §1.1 + §1.5 (v1 release 硬指标)

---

## 候选 3: CS-1 / CS-2 / CS-3 (client-shape 三栏 + 故障三态 + PWA)

**真 drift 风险**:
- CS-1 三栏 + Artifact 分级 (#601 merged): `docs/current/client/ui/main-desktop.md` 是否引 §X.Y client-shape, 4 态 state machine 字面是否对齐
- CS-2 故障三态 + 4 层 UX (#595): `docs/current/client/failure-ux.md` 已存在, 字面是否跟实施代码 byte-identical
- CS-3 PWA install + Web Push UI (#598): `docs/current/client/pwa-install.md` + `push-subscribe.md` 已存在, 真实施跟文档对齐?

**反查锚**:
```bash
# (a) CS-1 三栏 + Artifact 分级: client-shape §X.Y 必引
grep -E "client-shape\.md §[0-9]" docs/current/client/ui/main-desktop.md docs/current/client/README.md  # ≥1 hit

# (b) CS-2 故障三态 字面对照 (3 态字面 byte-identical 跟实施)
grep -cE "online|offline|error|degraded|stale" docs/current/client/failure-ux.md  # ≥3 hit (3 态 + 关键 UX 词)

# (c) CS-3 PWA install + Web Push: VAPID / service-worker 字面对照
grep -cE "VAPID|service[ -]worker|Web Push|manifest\.webmanifest" docs/current/client/pwa-install.md docs/current/client/push-subscribe.md  # ≥4 hit
```

**立场引用**: `client-shape.md` §1 (一份 SPA + Tauri 壳 + Mobile PWA), 14 立场 §10 (remote-agent 升级为安装管家)

---

## 候选 4: DL-2 / DL-3 (events 双流 + 阈值哨)

**真 drift 已锁**:
- `docs/current/server/abac.md` L122: `## capabilities-enum (AP-4-enum #TBD)` — PR# 真值是 #591 (NAMING-1 spec 提到), `#TBD` 字面是 drift
- `docs/current/server/dl-2.md` / `dl-3.md` 已存在, 跟 #615/#618 真实施字面对齐?

**反查锚**:
```bash
# (a) #TBD / TBD PR# 0 残留 (真值已知就填)
grep -rnE "#TBD|\\(TBD\\)|TBD\\)" docs/current/server/  # 0 hit

# (b) DL-2 events 双流: messages + events 双流字面
grep -cE "messages.*events|events.*messages|双流|dual stream" docs/current/server/dl-2.md  # ≥1 hit

# (c) DL-3 阈值哨 4 metric 字面 byte-identical 跟蓝图 §5
grep -cE "db_size|wal_pending|write_lock|row_count" docs/current/server/dl-3.md  # ≥4 hit (4 metric 全字面)

# (d) 蓝图引锚
grep -cE "data-layer\.md §[0-9]" docs/current/server/dl-2.md docs/current/server/dl-3.md  # ≥2 hit
```

**立场引用**: `data-layer.md` §3.1 (Q10.2 events 双流) + §3.4 (阈值哨), 14 立场 §12 (凭指标切, 不凭感觉切)

---

## 候选 5: ADM-3 (audit_events RENAME)

**真 drift 风险**:
- ADM-3 #586 merged (admin_actions → audit_events RENAME + alias view): `docs/current/server/adm-3.md` / `data-model.md` / `migrations.md` 是否反映 v=43 真值
- 别名 view `admin_actions` 留 backward compat — docs 是否说清两个名字关系

**反查锚**:
```bash
# (a) audit_events 表名出现 + alias view 关系说明
grep -cE "audit_events|alias view|admin_actions.*backward" docs/current/server/adm-3.md docs/current/server/data-model.md  # ≥2 hit

# (b) v=43 migration 真值
grep -cE "v=43|version.*43" docs/current/server/migrations.md docs/current/server/adm-3.md  # ≥1 hit

# (c) admin god-mode endpoint 不返回内容字段 (ADM-0 §1.3 红线 + memory admin_invisible)
grep -rnE "message\.body|artifact\.content|\.api_key" docs/current/server/admin*.md docs/current/server/adm-3.md 2>/dev/null \
  | grep -v "不返回\|绝不\|禁\|red line"  # 0 hit (除非是反约束陈述)
```

**立场引用**: `admin-model.md` §1.3 (硬隔离 + god-mode 不返内容) + §2 (audit 100% 留痕)

---

## 4 立场跨候选硬反查

不绑特定候选, 5 候选全适用:

### 立场 ① 规则 6 (docs/current 必同步)

```bash
# 每个 catchup 文件必引代码真路径 (反"光说不锚")
for f in docs/current/server/api/host-grants.md docs/current/client/host-grants-panel.md docs/current/server/dl-2.md docs/current/server/dl-3.md docs/current/server/adm-3.md; do
  grep -cE "packages/(server-go|client|borgee-helper|borgee-installer|remote-agent)/" "$f" 2>/dev/null
done | awk '{ if ($1 < 1) exit 1 }'  # 每文件 ≥1 hit
```

### 立场 ② 文件命名按功能不按 milestone

```bash
# docs/current/server/ 11 milestone-id 文件已存在 (naming-1.md / refactor-2.md / dl-2.md / dl-3.md / adm-3.md / ap-2.md / cm-5.md / rt-3.md / wire-1.md / ulid-migration.md / refactor-1.md)
# CATCHUP 期间不许新增更多 milestone-prefix 文件; 改造方案让 spec brief 拍板
ls docs/current/server/ | grep -cE "^(naming|refactor|dl|adm|ap|cm|rt|wire|ulid)-[0-9]+\.md$"  # baseline (不增, 走 spec 拍删/合并/迁出)
```

### 立场 ③ 不投隐私 / 合规 (memory: admin_invisible_to_user)

```bash
# GDPR / 数据主体权利 / Article 15 在 docs/current/ + 立场文档 0 hit
grep -rnE "GDPR|Article 15|数据主体权利" docs/current/ docs/blueprint/current/ docs/qa/ docs/implementation/modules/ 2>/dev/null  # 0 hit
```

### 立场 ④ admin invisible to user-rail

```bash
# user-rail 类 docs (非 admin/ 路径) 不出现 admin / 管理员 / 审计 / 监督
grep -rnE "管理员|审计|监督" docs/current/client/ \
  | grep -v "/admin/" | grep -v "// " | wc -l  # 0
# admin 字面在 user-rail docs 也 0 (除引用 admin-model.md 的章节)
grep -rnE "\\badmin\\b" docs/current/client/ \
  | grep -v "admin-model\.md" | grep -v "/admin/" | grep -v "// " | wc -l  # 0
```

---

## 用法

1. **zhanma 实施期自查**: 每个候选改完跑对应 5 段 grep + 4 立场跨候选 grep
2. **飞马 PR review**: 5 候选段一段不漏, 任一 grep 命中即 NOT-LGTM
3. **野马 review**: 重点 4 立场跨候选段 (③ + ④ admin/GDPR 红线 + ① 规则 6 锚到代码)
4. **CI 集成 (可选)**: 4 立场跨候选段 grep 串行跑, 任一非 0 整段 fail (跟 DOCS-CURRENT-CATCHUP PR check 挂)

## 反模式

- ❌ catchup 文档写 narrative 不锚代码路径 (反规则 6, 失去 audit 真值意义)
- ❌ 借 catchup 把 admin 字眼带进 user-rail 文档 (新写 `docs/current/client/admin-actions.md` 类文件)
- ❌ 借 catchup 把 GDPR / 合规字眼带进立场文档
- ❌ 新增更多 milestone-prefix 命名的 docs/current 文件 (现有 11 个已是债, 不增量)
- ❌ "TODO / coming soon / 待实施"字面写在 docs/current (catchup 范围必须是已 merged 真实施, 不写未来)

## 5 候选拍板待飞马 spec brief

> 飞马 spec brief in-flight, 候选范围以 spec brief 拍板为准. 上面 5 候选 (HB-2 / HB-1B-INSTALLER / CS-1/2/3 / DL-2/3 / ADM-3) 是 yema 按最近 4-30 ~ 5-01 merged 高漂概率反推的, 待飞马拍板对齐.
