# pflow

多 Agent 会话的注意力管理与调度工具。

## 是什么

pflow 帮助你在同时使用多个 AI 编程 Agent（Claude Code、Hermes 等）时，降低多任务切换的心智负担——就像一个"战报 + 传令系统"：在军帐中看到各部战况，在合适的时机被引导到需要你的前线。

## 核心概念

| 概念 | 说明 |
|------|------|
| Agent Session | 一个正在执行任务的 AI Agent 实例（一支部队） |
| Dashboard（军帐战报）| 红绿灯状态表：🟢交战中 / 🟡待命 / ⚪休整 / ⚫静默 |
| `pflow claude` | 一键启动 tmux + Claude 托管会话，自动关联 Dashboard |
| 军情哨 | 监控各部战况，在合适时机给出调度建议（规划中） |
| 赴前线 | 从 Dashboard 一键打开 Web 终端，直接与 Agent 交互 |

## 快速开始

```bash
# 查看所有 Agent 会话状态（CLI 文本表格）
pflow status

# 启动 Web Dashboard（浏览器访问 http://localhost:8080）
pflow serve

# 创建 tmux + Claude 托管会话
pflow claude -dir /path/to/project

# 查看单个会话详情
pflow probe <session-id>
```

## 当前状态

**阶段三：CLI 能力扩展** — 已打通"从 Dashboard 看到 Agent 状态 → Web 终端交互"的完整闭环。

已实现：
- ✅ CLI 状态仪表盘 + Web Dashboard（Vue 3 + Naive UI）
- ✅ Claude + Hermes 双 Agent 状态监控
- ✅ `pflow claude` — tmux + Claude 托管会话管理
- ✅ Web 终端集成（ttyd + tmux），Dashboard 侧边栏一键连接
- ✅ Tmux↔Claude session 关联映射（通过 statusline 前缀）

规划中：
- `pflow attach` 独立子命令
- `pflow suggest` 军情哨分析建议
- 多 Agent 类型启动（Hermes）
- 军情哨主动推送 + 偏好学习

## 文档索引

| 文档 | 用途 |
|------|------|
| [`docs/prd.md`](./docs/prd.md) | 产品需求文档 |
| [`docs/tech.md`](./docs/tech.md) | 技术设计方案 |
| [`docs/backlog.md`](./docs/backlog.md) | 全部需求池 |
| [`docs/changelog.md`](./docs/changelog.md) | 版本历史 |
| [`docs/note.md`](./docs/note.md) | 活跃技术备忘 |
| [`docs/testing.md`](./docs/testing.md) | 测试策略 |

## 许可

MIT License
