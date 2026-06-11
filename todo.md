# todo

> 当前周期：阶段一 可行性验证

## P0

### P0-1

- [x] 项目初始化：`go mod init`，搭好目录结构
- [x] 实现活跃会话检测：扫描 `~/.claude/sessions/` + `history.jsonl`，按 24h 窗口汇总
  - `internal/claude/activity.go` — 读取 session metadata 和 history，聚合输出 SessionSummary
  - `cmd/pflow/main.go` — CLI 入口，打印活跃会话表格（状态/项目/消息数/最后活跃）
- [x] 实现 Hermes Agent 会话监控（额外插入任务）
  - `internal/hermes/activity.go` — 读取 `sessions.json` + `gateway_state.json` + request_dump 文件
  - 确认 Hermes 支持 ACP 协议（v0.14.0），cc-connect 有完整 ACP 适配器，留作后续对接
  - 统一 CLI 输出：Claude Code + Hermes 双面板
  - 数据源：sessions.json（gateway 管理，实时） + request_dump 文件（CLI/cron，事后）
  - 文档记录在 `docs/note.md`
- [ ] 实现 Claude Code CLI 子进程启动与 stdin/stdout 管道通信
- [ ] 实现 stream-json 事件流解析（基于 `--output-format stream-json --input-format stream-json --permission-prompt-tool stdio`）
- [ ] 实现 SessionSnapshot：从事件流推断 busy/waiting/idle 三态

### P0-2

- [ ] 实现 Dashboard API：`GET /api/v1/dashboard` 返回所有 session 的状态快照
- [ ] 实现 `pflow probe` 命令：探测单个 Claude Code 会话的状态
- [ ] 实现 `pflow status` 命令：终端文本表格列出所有会话状态
- [ ] 验证测试：同时启动 2-3 个 Claude Code 会话，验证状态准确性和并发

## P1

- [ ] Shell 补齐脚本（bash/zsh completion）

## 验证目标

| 指标 | 目标 |
|------|------|
| busy/waiting/idle 三态准确率 | > 80% |
| 权限请求检测率 | > 70% |
| Dashboard API 延迟 | < 100ms |
| 多 session 并发 | 3 个 session 无串扰 |
