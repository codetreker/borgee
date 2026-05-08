# docs/implementation/design/ — 实施设计草稿

> blueprintflow `implementation-design` skill 输出位置. milestone PR 中由 Dev (战马) 出实施设计草稿, 跟 `modules/<id>-spec.md` (Architect 飞马 spec brief) 互补:
> - `modules/<id>-spec.md` — Architect 出, ≤80 行, 锁立场 / 拆段 / 反约束
> - `design/<id>-design.md` — Dev 出, 自由长度, 落地实现细节 (字段名 / SQL / 反向 grep 锚 / 边界条件)
>
> design/ 文档是可选项, 仅当 milestone 真需要详细设计草稿才创建 (e.g. schema 改 / 跨模块 refactor / 多文件 sed). 简单 milestone (单文件 ≤ 200 行 LOC) 不必出 design.
