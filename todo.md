# TODO

> 当前周期的开发任务，从 [`docs/backlog.md`](./docs/backlog.md) 中挑选。


| 状态    | 序号 | 类别       | 任务                               | 简述                                                                                                     |
| ------- | ---- | ---------- | ---------------------------------- | -------------------------------------------------------------------------------------------------------- |
| Pending | 15   | 交互体验   | 项目折叠/展开                      | 记忆用户对普通项目和归档项目的折叠状态                                                                   |
| ✅ Done | 17   | 智能分析   | 军情哨分析建议（`pflow suggest`）  | 基于会话状态和历史数据，主动给出分析建议                                                                 |
| ✅ Done | 16   | 会话管理 | tmux 会话绑定同步                        | tmux 定期截图刷新 sessionId，`/clear` 和 `/resume` 或重启导致 session 绑定更换，同步到页面做重新绑定 |
| ✅ Done | 18   | Agent 管理 | 多 Agent 类型启动（`pflow hermes`）     | 支持启动不同类型的 AI Agent（Claude Code 以外的其他 Agent）                                    |
| ✅ Done | 27   | 会话管理 | 映射数据结构升级                         | Mapping 增加 agentName / status / lastUpdated / pid 字段，所有创建点已更新                     |
| ✅ Done | 28   | 会话管理 | Claude JSON 目录扫描替代截屏             | `claude -n <name>` + 扫描 `~/.claude/sessions/`，代码开关 `SetClaudeCaptureMode()` 保留旧方案 |
| ✅ Done | 29   | 会话管理 | Claude 后台轮询监控                      | 每 5s 扫描 `~/.claude/sessions/`，sessionId/PID 变化时自动更新映射                             |
| ✅ Done | 30   | 会话管理 | Hermes `/status` 截屏解析               | `tmux send-keys /status` + `capture-pane` 正则解析 Session ID，替代 `hermes sessions export` |
| ✅ Done | 31   | 会话管理 | Tmux 自动销毁优化                        | `trap '' TSTP` 阻止 Ctrl+Z，Agent 退出后自动 `tmux kill-session`                             |
| ✅ Done | 32   | 会话管理 | 孤儿会话清理                             | `CleanOrphanSessions()` 在 serve 启动时扫描并清理无映射的 `pflow-*` tmux 会话                 |
| ✅ Done | 33   | CLI 工具 | `pflow session list`                    | CLI 表格展示所有会话映射（Container / Agent / Name / Session ID / Status / Work Dir）          |
| ✅ Done | 34   | CLI 工具 | `pflow session destroy`                 | `pflow session destroy <containerName>` 销毁 tmux 会话 + 清理映射                              |
| ✅ Done  | 35   | Web 前端 | Dashboard 终端映射优化                   | `SessionMeta` 增加 Name 字段解析，aggregate 优先使用 `-n` name；前端已正确展示                |
