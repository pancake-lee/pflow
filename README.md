# pflow

[English](./README.en.md) | 中文

多 Agent 会话的注意力管理与调度工具

## 你是否也遇到过这些问题？

- Claude 在处理代码，Hermes 在写文档。你在等谁先完成，该去哪个窗口看？
- 切到其他窗口忙了一会儿，回来忘了哪个 Agent 正在等你授权，只能逐屏翻找。
- 多个项目的 AI 会话散落在不同终端、不同 tmux 中，没有一个统一的视图能看到“全盘战况”。

pflow 把这些 AI 会话的状态汇聚到一张仪表盘上，让你像将军看沙盘一样，一眼看清哪路兵在打、哪路兵在等、该去哪路支援。

## Dashboard 预览

![Dashboard 截图](./docs/screenshot.png)

## 快速开始

```bash
# 打开 Web Dashboard（浏览器访问 http://localhost:8080）
pflow serve

# 在当前项目启动 Claude 托管会话（tmux 自动配置）
pflow claude -dir .

# 查看今日军情建议（专注/切换/授权提醒）
pflow suggest
```

## 核心体验

| 场景 | pflow 怎么做 |
|------|-------------|
| 启动一个 Agent 会话 | `pflow claude -dir .` → tmux + Agent 自动配置，Dashboard 立即出现 |
| 多会话状态一目了然 | Dashboard 红绿灯：🟢交战中 / 🟡待命 / ⚪休整 |
| Agent 需要授权 | 点一下侧边栏，Web 终端直接 attach 到同一 tmux 会话，按 1 确认 |
| 今天该专注什么 | 主线/支线策略 + 提醒分数算法 → `pflow suggest` 告诉你下一步 |

## 为什么用 pflow

- **不是又一个 Agent**：pflow 不做对话，只做调度。它让你同时用好多个 Agent，而不是在它们之间消耗精力。
- **无感工作流**：在终端里正常用 Claude Code、Hermes，它们自动出现在 Dashboard 上——不需要改变你的使用习惯。
- **一键直达**：从“看到状态”到“操作会话”只需要一次点击，不用复制粘贴命令或手动 tmux attach。

## 已实现的核心能力

- **多 Agent 监控**：Claude Code、Hermes 双引擎支持，状态实时同步
- **项目策略管理**：主线/支线任务映射 + 提醒分数算法 + 注意力遮罩
- **Web 终端集成**：Dashboard 侧边栏一键 attach 到 Agent 会话（基于 ttyd + tmux）
- **注意力引导**：专注模式保护期 + 军情哨建议引擎（~20 个静态场景）
- **完整文档**：PRD、技术设计、需求池、测试策略齐全

## 技术栈

| 层级 | 技术 |
|------|------|
| CLI 框架 | Cobra + Bubble Tea |
| Web Dashboard | Vue 3 + Naive UI（Go embed 单二进制部署） |
| 后端 | Go（tmux 管理、状态扫描、session 映射） |
| Agent 集成 | Claude Code（statusline + JSON 扫描）、Hermes（export 解析） |

## 路线图

- [x] 双 Agent 状态监控 + Web Dashboard
- [x] 项目策略管理 + 提醒分数算法 + 注意力遮罩
- [x] Web 终端集成 + 专注模式
- [x] 军情哨建议引擎（静态场景）
- [ ] 桌面通知（浏览器 Notification API）
- [ ] 军情哨主动推送 + 偏好学习
- [ ] 双层换肤系统

## 文档

| 文档 | 用途 |
|------|------|
| [`docs/prd.md`](./docs/prd.md) | 产品需求文档 |
| [`docs/tech.md`](./docs/tech.md) | 技术设计方案 |
| [`docs/backlog.md`](./docs/backlog.md) | 全部需求池 |
| [`docs/changelog.md`](./docs/changelog.md) | 版本历史 |
| [`docs/screenshot.png`](./docs/screenshot.png) | 最新截图 |

## License

MIT
