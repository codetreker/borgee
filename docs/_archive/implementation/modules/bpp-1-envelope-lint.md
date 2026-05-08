# BPP-1 envelope CI lint — spec stub (Phase 4 占号)

> **状态**: 占号 stub (Phase 2 G2.6 留账行 PR 锁用), 实施留 Phase 4 BPP-1。
> **蓝图锚**: [`plugin-protocol.md`](../../blueprint/current/plugin-protocol.md) §2 + §2.1 控制面 + §2.2 数据面; [`realtime.md`](../../blueprint/current/realtime.md) §2.3 (`/ws` ↔ BPP envelope 等同性, 飞马 R3)。
> **关联**: PR #237 envelope 模板 + PR #267 §5 + PR #269 RT-1 §0 守门。

## 1. envelope schema (Phase 4 锁)

```
{ cursor:int64, frame_kind:string, payload:object }
```

`payload` 字段名 / 顺序 / 类型与 `internal/ws/event_schemas.go` 同 frame_kind 行 **byte-identical 或 type alias**, 任一分歧 CI fail。

## 2. CI lint workflow placeholder

- 文件: `.github/workflows/bpp-envelope-lint.yml` (Phase 4 落)
- 触发: PR 改 `internal/bpp/frame_schemas.go` ∪ `internal/ws/event_schemas.go`
- 步骤: AST 对比同 frame_kind 字段, 不一致 fail; 缺对应行也 fail (反向防 BPP frame 单方落)

## 3. 范围 / 反约束

- ✅ 占号 G2.6 留账行 PR # (规则 6)
- ❌ 不在本 PR: BPP-1 协议骨架 / frame 实现 / 直连 flag (留 BPP-1)
- 反约束: 此 stub merge 后 G2.6 行写本 PR #, **不**视作 lint 已落; 实际启用以 BPP-1 实施 PR 为准

## Test plan: BPP-1 CI lint workflow — 内容真做留 Phase 4 实施
