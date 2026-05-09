// CS-3.1 — Push permission 三态文案 (蓝图 client-shape.md §1.4 + DL-4 #485 同源).
//
// 设计 ② (cs-3-stance-checklist):
//   - 4-enum byte-identical 跟 DL-4 PushPermissionState (granted/denied/default/unsupported)
//   - 文案 byte-identical 跟蓝图字面 (改 = 改两处 + content-lock §1)
//
// 反约束 (cs-3-content-lock §1):
//   - 同义词漂禁: '下载客户端' / '装个 app' / '接收推送' / '订阅通知' / '权限被拒' 0 hit

import type { PushPermissionState } from './pushSubscribe';

/**
 * PUSH_PERMISSION_LABELS — 4-enum → toggle UI label byte-identical.
 *
 * - granted   : `已开启通知` (toggle on)
 * - denied    : `通知已被浏览器拒绝, 请到浏览器设置开启`
 * - default   : `开启通知` (toggle off, click → subscribe)
 * - unsupported: 空 string (UI 不渲染, 沉默胜于假活物感)
 */
export const PUSH_PERMISSION_LABELS: Record<PushPermissionState, string> = {
  granted: '已开启通知',
  denied: '通知已被浏览器拒绝, 请到浏览器设置开启',
  default: '开启通知',
  unsupported: '',
};

/**
 * INSTALL_BUTTON_LABEL — Install button 文案 byte-identical 跟蓝图 §1.1.
 */
export const INSTALL_BUTTON_LABEL = '安装 Borgee 桌面应用';
