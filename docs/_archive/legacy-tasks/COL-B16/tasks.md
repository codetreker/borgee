Task list 已写入 `docs/tasks/COL-B16/tasks.md`，共 7 个任务，总预估 ~383 行改动。

**关键发现**：App.tsx 和 index.css 已有 sidebar 响应式基础（汉堡按钮、slide 动画、overlay、safe-area），所以 T1 只需完善触摸目标尺寸。真正的新工作集中在：

- **T2** 键盘适配（visualViewport API）
- **T3/T4** Emoji + Slash picker 底部弹出
- **T5** 长按操作（需新建 useLongPress hook + action sheet）
- **T6/T7** PWA 全部从零开始（manifest、SW、离线页面）

建议先并行做 T1+T2+T5+T6，再串行 T3→T4→T7。
