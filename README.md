# pflow

多 Agent 会话的注意力管理与调度工具。

## 是什么

pflow 帮助你在同时使用多个 AI 编程 Agent（Claude Code、Cline 等）时，降低多任务切换的心智负担——就像一个"战报 + 传令系统"：在军帐中看到各部战况，在合适的时机被引导到需要你的前线。

## 核心概念

| 概念 | 说明 |
|------|------|
| Agent Session | 一个正在执行任务的 AI Agent 实例（一支部队） |
| Dashboard（军帐战报）| 红绿灯状态表：🟢交战中 / 🟡待命 / ⚪休整 |
| 军情哨 | 监控各部战况，在合适时机给出调度建议 |
| 赴前线 | 一键从 Dashboard 唤起终端，直接与 Agent 对话 |

## 状态

当前处于**阶段一：可行性验证**。详见 [`todo.md`](./todo.md)。

## 术语

- **主攻方向**：当前深度聚焦的项目
- **侧翼战场**：可在等待时间填充的轻量任务
- **传令兵**：军情哨的用户通知机制

## 文档索引

| 文档 | 用途 |
|------|------|
| [`todo.md`](./todo.md) | 当前周期任务清单 |
| [`docs/prd.md`](./docs/prd.md) | 产品需求文档 |
| [`docs/tech.md`](./docs/tech.md) | 技术设计方案 |
| [`docs/backlog.md`](./docs/backlog.md) | 全部需求池 |
| [`docs/changelog.md`](./docs/changelog.md) | 版本历史 |
| [`docs/note.md`](./docs/note.md) | 活跃技术备忘 |
| [`docs/testing.md`](./docs/testing.md) | 测试策略 |

## 许可

MIT License
