# HB-1b INSTALLER stance checklist — install-butler installer (Linux .deb + macOS .pkg) v1

> 7 立场 byte-identical 跟 hb-1b-installer-spec.md (飞马待 commit). **真兑现 G4.audit 交叉核验项 3 (第三方抓的 installer 真漏 P0)** — 蓝图 host-bridge.md §1.1+§1.4 明示 Linux systemd + macOS launchd 真 v1, Windows v2 留账. 真有 prod code (.deb + .pkg installer 真启 + sudo install + systemd unit + launchd plist + 卸载脚本) 但 0 server schema 改 + 0 既有 endpoint shape 改 (installer = client side). content-lock 不需 (installer 命令行/系统弹窗字面跟 OS 标准承袭).

## 1. Linux .deb installer 真启 (蓝图 §1.4 systemd + cgroups)
- [ ] `.deb` package 真生成 (`dpkg-deb --build` + `borgee-helper` binary + systemd unit `borgee-helper.service` byte-identical 跟蓝图 §1.4)
- [ ] systemd unit `User=borgee-helper Group=borgee-helper` (反 root 跑 host-bridge, 蓝图 §1.4 字面承袭)
- [ ] sudo install 首次 prompt + 创建 `borgee-helper` OS user/group (蓝图 §1.1 字面承袭)
- [ ] cgroups 限制 (蓝图 §1.4 — sandbox 跨 OS user 隔离)

## 2. macOS .pkg installer 真启 (蓝图 §1.4 launchd + sandbox-exec)
- [ ] `.pkg` 真生成 (`productbuild` + `borgee-helper` binary + launchd plist `com.borgee.helper.plist`)
- [ ] launchd plist `UserName=_borgee` (反 root, 跟蓝图 §1.4 字面)
- [ ] sandbox-exec profile 真启 (留 HB-2 v0(D) 真 sandbox 协同)
- [ ] sudo install 首次 prompt + 创建 `_borgee` OS user (跟蓝图 §1.1 字面)

## 3. 一键完全卸载 (信任底线, 蓝图 §1.2)
- [ ] `borgee-helper-uninstall` 命令真启 (跟蓝图 §1.2 6 项删除清单 byte-identical: 二进制 / 配置 / 状态 / runtime / Borgee server 注册 / OS user/group / launchd / systemd unit)
- [ ] 6 项删除字面 byte-identical 跟蓝图 §1.2 (反"漏删" 用户感知信任底线破)
- [ ] 反 user data 残留 (反向 grep cleanup 路径 byte-identical)

## 4. ed25519 verify on install (跟 HB-1 #589 endpoint 对接)
- [ ] installer 真启 fetch `/api/v1/plugin-manifest` + ed25519 verify (跟 HB-1 #589 byte-identical)
- [ ] verify 失败 → 安装阻塞 (反 HB-4 release-gate 第 3 行 "签名 0% fail")
- [ ] 反 install 阶段偷传 telemetry (sandbox 不偷传立场延伸)

## 5. PM 必修 #10 #599 borgee.cloud 一键安装真兑现
- [ ] borgee.cloud 一键安装路径承袭 #599 PM 必修 #10 (用户从 borgee.cloud 一键下载 + sudo install)
- [ ] 跨 #599+CS-1/2/3 字面承袭一致 (`Borgee Helper` 命名 byte-identical)
- [ ] daemon 命名 `Borgee Helper` 在 about 页 + 进程列表 + setting byte-identical 跟 HB-2 content-lock §2

## 6. Windows installer 留账 v2 (蓝图 §1.4 明示)
- [ ] Windows MSI 留 v2 (蓝图 §1.4 明示 "Windows: v2 才支持, 需重新设计")
- [ ] 反 v1 强行做 Windows MSI (反蓝图明示)
- [ ] HB-1b spec 显式声明 Windows v2 留账 (反 future drift)

## 7. 0 server schema / 0 既有 endpoint shape 改 + admin god-mode 不挂
- [ ] 反向 grep `migrations/hb_1b_` 在 packages/server-go/ 0 hit
- [ ] 反向 grep `admin.*installer|admin.*install-butler` 0 hit (ADM-0 §1.3 红线)
- [ ] installer 走用户 sudo, 反 admin override (anchor #360 owner-only ACL 锁链立场延伸)

## 反约束 — 真不在范围
- ❌ Windows MSI 真启 (蓝图 §1.4 明示 v2)
- ❌ 自动更新 (蓝图 §1.2.3 明示 "自动更新 = 反模式, 绝不在 v1")
- ❌ 加 schema / endpoint shape / admin override
- ❌ install 偷传 telemetry / metrics / phone-home (反沙箱不偷传立场)

## 跨 milestone byte-identical 锁链 (5 链)
- HB stack 4 步路径 (#599+#605+#606+HB-2 v0(D) stance) → 本 PR 第 5 步 installer 真兑现
- HB-1 #589 manifest endpoint ed25519 verify (installer 真接)
- 蓝图 host-bridge.md §1.1+§1.2+§1.4 字面 byte-identical (双 daemon 拆分 + 完整卸载 + sandbox 跨 OS)
- PM 必修 #10 #599 borgee.cloud 一键安装真兑现路径
- anchor #360 owner-only ACL 22+ PRs + ADM-0 §1.3 红线

## PM 拆死决策 (3 段)
- **Linux+macOS v1 vs Windows v2 拆死** — 蓝图 §1.4 明示 (本 PR Linux+macOS), Windows 蓝图明示 v2
- **完整卸载 vs 残留拆死** — 6 项全删跟蓝图 §1.2 byte-identical (本 PR), 反"漏删"破信任底线
- **OS user/group + sandbox vs root 拆死** — 独立 user (Linux `borgee-helper` / macOS `_borgee`) (本 PR), 反 root 跑 daemon

## 用户主权红线 (5 项)
- ✅ borgee.cloud 一键安装真兑现 (PM 必修 #10)
- ✅ 完整卸载真兑现 (蓝图 §1.2 信任底线)
- ✅ ed25519 signed install (反 supply chain attack)
- ✅ 反自动更新 (蓝图 §1.2.3 红线)
- ✅ admin god-mode 不挂 installer (ADM-0 §1.3 红线)

## PR 出来 5 核对疑点
1. .deb + .pkg installer 真生成 + sudo install 真启
2. systemd unit + launchd plist 真启 (User=borgee-helper / _borgee, 反 root)
3. 一键完全卸载 6 项字面 byte-identical 跟蓝图 §1.2
4. ed25519 verify on install (跟 HB-1 #589 byte-identical)
5. cov ≥85% (#613 gate) + admin grep 0 hit + Windows v2 留账显式声明
