# Cross-PR drift anchor — DL-4 ↔ HB-1 manifest scope split

> 战马A 架构师视角 spec 双向锁 (跟 zhanma-e DL-4 spec 实施视角互校).
> 触发: HB-1 spec brief v0 (81a41fa, PR #491) 锁了 `GET /api/v1/plugin-
> manifest` 字面 contract; DL-4 spec brief (PR #490 ca3891b) 锁了
> `GET /api/v1/manifest/plugins`. **两个 endpoint, 两个范围, drift 警报.**
>
> 本 doc 是 ≤30 行 anchor — 锁住分工边界, 不重写两边 spec, 防 implementation
> 时跨 PR 撞车.

## 范围拆 (drift 解决方案)

| Endpoint                              | Owner        | 范围                                                     | Sig 要求            |
|---------------------------------------|--------------|----------------------------------------------------------|---------------------|
| `GET /api/v1/manifest/plugins`        | **DL-4** (zhanma-e) | PWA installable plugin 元数据 list (name/icon/version/runtime); 给 client 设置页选 plugin 用 | manifest 体内不签 (走 HTTPS + bearer auth, PWA 路径) |
| `GET /api/v1/plugin-manifest`         | **DL-4 follow-up (新 ticket)** | install-butler 消费的**签名 binary manifest** (含 binary_url + SHA256 + 二进制 GPG signature + manifest 自身 GPG signature) | 双签 (manifest GPG + 每 binary GPG, 蓝图 host-bridge §1.2 ①) |

## 立场

- DL-4 当前 spec (#490 ca3891b, 7 段 v0) 范围是 **Web Push (蓝图
  client-shape.md L22)**, 不含 install-butler 签名 manifest. zhanma-e
  实施的 DL-4.4 manifest API 是 PWA 用, 不是 install-butler 用.
- HB-1 spec (#491 81a41fa) §3.2 字面锁了 install-butler 消费的 manifest
  schema. 但**当前没有 milestone 拍下 该 endpoint 的 server 端实施**.
- **解决**: 加 DL-4 follow-up ticket (DL-4.8 plugin signing manifest
  endpoint) 或新建 milestone DL-5; 不动 DL-4 当前 7 段范围 (zhanma-e
  已实施 DL-4.1+DL-4.2 in flight).

## 不动他人 WIP

- 本 anchor 不改 zhanma-e 的 dl-4-spec.md (PR #490) — 那是 author 视角
  spec, 范围 lock 完整.
- 本 anchor 不改 hb-1-spec.md §3.2 — install-butler 消费的 contract
  锁定不变, 只是 server 端实施方迁到新 ticket.
- 真补丁: HB-1 实施时 (DL-4.8/DL-5 落地后) 才需要回头 cross-ref. 现在
  只锁分工边界.

## Action items

1. team-lead 派 DL-4.8 ticket: `GET /api/v1/plugin-manifest` 签名 manifest
   endpoint (host-bridge §1.2 ① 双签), 不入 DL-4 当前 PR 范围.
2. 或直接挂在 HB-1 实施 PR (DL-4.8 + HB-1.1 同 PR) — 新协议 一 milestone
   一 PR, 但跨 server-go + Rust crate 拆是合理例外.
3. 本 anchor 是 stub — 真 contract 锁仍在 hb-1-spec.md §3.2 字面.

## 跨 PR drift 防御 (本 anchor 守的)

- `GET /api/v1/manifest/plugins` (DL-4 PWA) ≠ `GET /api/v1/plugin-
  manifest` (HB-1 install-butler signed) — 命名相近, 范围不同, **这两个
  endpoint 不能合**.
- HB-1 install-butler 调 DL-4 PWA endpoint = 安全裸奔 (PWA endpoint
  不签 binary, install-butler 消费即 fail-closed 的 §4.5 "未签 binary
  100% reject" 全过, 但 manifest 自身没签也 reject 不到 — 0 防御).
- 字面承袭: DL-4 PWA endpoint 走 bearer auth + HTTPS 即够 (PWA list);
  install-butler endpoint 必须双签 (binary 落本机 sudo 路径, 安全模型
  不同).
