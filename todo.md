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
- [x] 实现 Claude Code CLI 子进程启动与 stdin/stdout 管道通信
  - `internal/claude/subprocess.go` — `Start()`, `Send()`, `Events()`, `Close()`, `PID()`
  - 使用 `--output-format stream-json --input-format stream-json --permission-prompt-tool stdio` 标志
- [x] 实现 stream-json 事件流解析（基于 `--output-format stream-json --input-format stream-json --permission-prompt-tool stdio`）
  - `internal/claude/stream.go` — Event/UserEvent/AssistantEvent 类型，`ParseEvents()` 流式解析器
- [x] 实现 SessionSnapshot：从事件流推断 busy/waiting/idle 三态
  - `internal/claude/snapshot.go` — `Tracker` 并发安全的状态追踪器，基于 stop_reason 推断状态
  - busy: 用户刚发消息 或 assistant 正在执行工具 (stop_reason=tool_use)
  - idle: assistant 完成回复 (stop_reason=end_turn)，等待用户输入
  - waiting: 权限请求场景（预留，stdio permission prompt 事件待补充）
- [x] 统一展示格式：Session ID / Project / Status / Last Active / Last Req / Last Resp
  - Last Req / Last Resp 从 transcript 文件 (`~/.claude/projects/.../<session>.jsonl`) 提取，截取前 15 字
  - busy 状态的 session 清除 Last Resp（避免展示不匹配的 req/resp 对）
  - Hermes Last Req 从 request_dump body 提取，Last Resp 暂无（需 SQLite）

### P0-2

- [x] 实现 Dashboard API：`GET /api/v1/dashboard?window=1d&max_inactive=1` 返回所有 session 的状态快照
  - `internal/api/server.go` — net/http 标准库，JSON 响应含 session_id/agent_type/status/traffic_light 等字段
- [x] 实现 `pflow probe` 命令：探测单个会话的详细状态（支持 Claude + Hermes，模糊匹配 session ID 前缀）
- [x] 实现 `pflow status` 命令：终端文本表格列出所有会话状态（支持 `--window` 和 `--max-inactive` 参数）
- [x] 实现 `pflow serve` 命令：启动 HTTP Dashboard API 服务器（`--port` 参数，默认 8080）
- [x] 新增时间窗口参数：`--window` 支持 `1h`/`3h`/`1d`/`2d` 等格式（`internal/config/config.go` — ParseWindow）
- [x] 新增非活跃限制参数：`--max-inactive` 按 PROJECT 分组限制 unknown/completed 等非活跃状态的展示数量
- [x] Agent 各自定义状态枚举及红绿灯映射：Claude (busy🟢/waiting🟡/idle⚪/unknown⚫)，Hermes (running🟢/suspended🟡/completed⚫)
- [x] 验证测试：build/vet 通过，CLI + API 功能验证正常（status/probe/serve 均可运行）
  - `pflow status --window 1h --max-inactive 1` 正确过滤非活跃 session
  - `pflow probe <id>` 支持 Claude 和 Hermes 两种 agent，支持前缀模糊匹配
  - `pflow serve` API 返回 JSON 含完整 traffic_light 和 is_active 字段

## P1

- [ ] Shell 补齐脚本（bash/zsh completion）

## P2

- [ ] **Hermes Last Resp 提取**：接入 `~/.hermes/state.db` SQLite，读取 assistant 回复内容填充 `LastResp` 字段。当前仅从 request_dump body 提取了 `LastReq`，回复内容需查询 SQLite messages 表。

## 验证目标

| 指标 | 目标 |
|------|------|
| busy/waiting/idle 三态准确率 | > 80% |
| 权限请求检测率 | > 70% |
| Dashboard API 延迟 | < 100ms |
| 多 session 并发 | 3 个 session 无串扰 |
