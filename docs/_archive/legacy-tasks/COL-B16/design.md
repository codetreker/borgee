# COL-B16: 移动端适配 + PWA — 技术设计

日期：2026-04-22 | 状态：Draft

## 1. 响应式布局

### 1.1 断点

```css
/* 移动端 */
@media (max-width: 768px) { ... }
/* 平板 */
@media (max-width: 1024px) { ... }
```

### 1.2 布局结构

**桌面（>768px）**：侧边栏 + 聊天区并排（现有布局不动）

**移动端（≤768px）**：
- 侧边栏默认隐藏，绝对定位覆盖在聊天区上方
- 左上角汉堡菜单图标（☰），点击展开侧边栏
- 选择频道后自动收起侧边栏
- 点击遮罩层也收起

**实现**：React state `sidebarOpen`，CSS `transform: translateX(-100%)` 动画。

### 1.3 改动文件

| 文件 | 改动 |
|------|------|
| `packages/client/src/App.tsx` | 加 `sidebarOpen` state + 汉堡按钮 + 遮罩 |
| `packages/client/src/App.css` | 响应式媒体查询 |
| `packages/client/src/components/Sidebar.tsx` | 移动端绝对定位 |
| `packages/client/src/components/MessageInput.tsx` | 固定底部 + 键盘适配 |
| `packages/client/src/components/EmojiPicker.tsx` | 移动端底部弹出 |
| `packages/client/src/components/MessageActions.tsx` | 长按触发（替代 hover） |

### 1.4 键盘适配

```css
/* iOS Safari 键盘弹出时 */
.message-input-container {
  position: fixed;
  bottom: 0;
  bottom: env(safe-area-inset-bottom); /* iOS notch */
}
```

用 `visualViewport` API 监听键盘弹出/收起，动态调整聊天区高度。

### 1.5 长按操作

移动端 hover 不可用。消息操作改为长按触发：
- `onTouchStart` 启动 500ms timer
- `onTouchEnd` / `onTouchMove` 取消
- 超时弹出操作菜单（底部 sheet）

### 1.6 触摸友好

所有可点击元素最小 44x44px（Apple HIG 标准）。按钮间距加大。

## 2. PWA

### 2.1 manifest.json

```json
{
  "name": "Collab",
  "short_name": "Collab",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#1a1a2e",
  "theme_color": "#16213e",
  "icons": [
    { "src": "/icons/icon-192.png", "sizes": "192x192", "type": "image/png" },
    { "src": "/icons/icon-512.png", "sizes": "512x512", "type": "image/png" }
  ]
}
```

### 2.2 Service Worker

基础 App Shell 缓存：
- 缓存：`index.html` + JS/CSS bundles + 图标
- 网络优先策略（先试网络，失败用缓存）
- 离线 fallback 页面

```javascript
// sw.js
const CACHE = 'collab-v1';
const SHELL = ['/', '/index.html'];

self.addEventListener('install', e => {
  e.waitUntil(caches.open(CACHE).then(c => c.addAll(SHELL)));
});

self.addEventListener('fetch', e => {
  e.respondWith(
    fetch(e.request).catch(() => caches.match(e.request))
  );
});
```

### 2.3 HTML meta tags

```html
<link rel="manifest" href="/manifest.json">
<meta name="theme-color" content="#16213e">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-status-bar-style" content="black-translucent">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<link rel="apple-touch-icon" href="/icons/icon-192.png">
```

## 3. Task Breakdown

### T1: 响应式基础 + 汉堡菜单
- 媒体查询断点
- `sidebarOpen` state + 汉堡按钮 + 遮罩层
- 选择频道自动收起

### T2: 消息区移动端适配
- 消息气泡宽屏适配
- 输入框固定底部 + 键盘适配（visualViewport）
- safe-area-inset 支持

### T3: 交互适配
- Emoji picker 底部弹出
- Slash commands 底部弹出
- 消息操作长按触发
- 触摸目标 44px

### T4: PWA
- manifest.json + 图标
- Service Worker（App Shell 缓存 + 离线 fallback）
- HTML meta tags
- 注册 Service Worker

## 4. 验收

按 PRD 验收标准，**必须在真机上测试**（iOS Safari + Android Chrome）。
