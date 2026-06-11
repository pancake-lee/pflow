# note

> 活跃技术备忘：当前在用的实现细节、调试技巧、临时方案、已知问题。

## 当前周期（阶段二 Web Dashboard）

### 阶段切换（2026-06-11）

- 阶段一已完成，P1/P2 未完成项回写到 [`backlog.md`](./backlog.md)
- Web Dashboard 从 P3（远期）提前到阶段二，优先于 TUI 方案
- 前端选型确认：**Vue 3 + Naive UI + TypeScript + Vite**
- 理由：模板语法适合 Dashboard，Naive UI DataTable/Tag/Card 开箱即用，暗色主题一流
- 部署方案：Vue SPA 打包后通过 `//go:embed` 嵌入 Go binary，`pflow serve` 单文件部署

### 前端嵌入与部署流程

```
web/src/*  (Vue 3 + TS)
    │  npm run build
    ▼
web/dist/  (纯静态 HTML/JS/CSS)
    │  //go:embed web/dist/*  (embed.go)
    ▼
bin/pflow  (Go 二进制，内含前端静态资源)
    │  pflow serve → http://localhost:8080
    ▼
浏览器访问 → Go HTTP Server 直接返回内嵌的 Vue SPA
```

- `embed.go` 位于项目根目录，import path 为 `github.com/pancake-lee/pflow`
- `cmd/pflow/main.go` 将 `pflow.WebDist`（`embed.FS`）传入 `api.NewServer()`
- `server.go` 的 `spaHandler` 处理 SPA 路由回退：非 `/api` 路径全部返回 `index.html`
- 开发时前后端分离：`make dev` 启动 API server，`cd web && npm run dev` 启动 Vite dev server（带 proxy）
- 生产时单二进制：`make build && bin/pflow serve` 一个命令启动全部

### 待办

- [x] 初始化 `web/` 目录：Vite + Vue 3 + TypeScript + Naive UI
- [x] 确认现有 Dashboard API 字段满足 Web 展示需求
- [ ] API 是否需要 CORS 头（开发阶段 Vite proxy 可规避，生产环境同源部署不需要）

### 实现记录（2026-06-11）

**新增文件**：

| 文件 | 功能 |
|------|------|
| `embed.go` | `//go:embed web/dist/*` 嵌入 Vue SPA，导出 `WebDist embed.FS` |
| `web/src/types/dashboard.ts` | TypeScript 类型定义，与 Go `DashboardEntry` 对齐 |
| `web/src/composables/useDashboard.ts` | Dashboard API 调用 + 响应状态管理 |
| `web/src/composables/usePolling.ts` | 可配置的轮询定时器 |
| `web/src/composables/format.ts` | 时间格式化、文本截断、ID 缩短 |
| `web/src/views/DashboardView.vue` | 核心 Dashboard 页面：控制栏 + DataTable + 统计卡片 + Drawer 详情 |

**修改文件**：

| 文件 | 修改内容 |
|------|---------|
| `internal/api/server.go` | `NewServer(fs.FS)` 接受静态文件系统；新增 `spaHandler`，非 API 路径回退到 `index.html` |
| `cmd/pflow/main.go` | `runServeCmd` 传入 `pflow.WebDist`；修复 `flagSet` 变量命名冲突（`fs` → `flagSet`） |
| `web/vite.config.ts` | 添加 `/api` 开发代理到 `localhost:8080` |
| `web/index.html` | 标题改为 "pflow — Agent Dashboard"，添加 `class="dark"` |

**构建产物**：
- 前端：`web/dist/` — 625KB JS (gzip 178KB) + 1.5KB CSS
- 后端：Go binary 9.3MB（含嵌入前端）
- TypeScript 类型检查 + Go vet 均通过

---

### Hermes Agent 集成可行性（2026-06-11）

**结论：可行。** Hermes 支持 ACP 协议，且本地有丰富的可读数据。

**ACP 协议支持**：
- `hermes acp --check` 通过（v0.14.0，与 Claude Code 使用相同的 Agent Communication Protocol）
- `hermes acp` 以 stdio JSON-RPC 模式启动，供编辑器集成（VS Code、Zed、JetBrains）
- cc-connect 的 `agent/acp/` 包已实现完整的 ACP 适配器（session 管理、权限处理、RPC 通信）

**本地数据源**：
| 数据源 | 路径 | 内容 |
|--------|------|------|
| sessions.json | `~/.hermes/sessions/sessions.json` | 当前活跃 session（session_key → 元数据） |
| state.db | `~/.hermes/state.db` | SQLite：90 sessions、3203 messages、token 统计 |
| gateway_state.json | `~/.hermes/gateway_state.json` | Gateway 平台连接状态（weixin 等） |
| request_dump | `~/.hermes/sessions/request_dump_*.json` | 原始 API request/response JSONL |

**当前活跃 session**（2026-06-11）：
- `20260611_144133_0d5a2c1e` — weixin 渠道，status: running
- `20260611_135406_b0a840` — cli 渠道，status: running

**实现策略**：阶段一采用文件系统扫描（与 Claude Code 方案一致），直接读取 `sessions.json` + request_dump 文件。ACP 协议对接留作后续——当需要管理 Hermes session（启动/停止/attach）时再接入。

### Subprocess 管理 + Stream-json 解析（2026-06-11）

**新增文件**：

| 文件 | 功能 |
|------|------|
| `internal/claude/stream.go` | Event/UserEvent/AssistantEvent 类型定义，`ParseEvents()` 流式解析器 |
| `internal/claude/snapshot.go` | `Tracker` 并发安全的状态追踪器，`Snapshot` 三态推断 |
| `internal/claude/subprocess.go` | `Client` 子进程管理：`Start()`, `Send()`, `Events()`, `Close()` |

**状态推断规则**：

| 事件 | 推断状态 | 说明 |
|------|---------|------|
| `type: "user"` | `busy` | 用户发消息，assistant 即将处理 |
| `assistant` + `stop_reason: "tool_use"` | `busy` | 工具执行中 |
| `assistant` + `stop_reason: "end_turn"` | `idle` | 回复完成，等待用户输入 |
| 权限请求（stdio prompt）| `waiting` | 预留，事件格式待确认 |

**统一展示格式**：
- 列：SESSION ID / PROJECT / STATUS / NAME / LAST ACTIVE / LAST REQ / LAST RESP
- NAME 列优先使用 session 标题/名称，否则展示用户第一条消息的前 15 字
  - Claude：从 history.jsonl 的最早 `display` 字段提取
  - Hermes gateway：使用 `DisplayName`（微信用户名等）
  - Hermes dump：使用 dump 中的用户消息
- Hermes PROJECT：从 request_dump 的 system prompt 中解析 `Current working directory: <path>` 获取 cwd；若 cwd 为 `/`（无意义）则回退到 platform 名称
- Last Req/Resp 从 Claude transcript 文件 (`~/.claude/projects/.../<session>.jsonl`) 提取
- busy 状态清除 Last Resp（避免展示不匹配的 req/resp 对）
- Hermes Last Req 从 request_dump body 提取，Last Resp 需 SQLite（暂未实现，见 backlog P2）
- 文本列（NAME, LAST REQ, LAST RESP）中的 `\n` `\r` `\t` 转义为 `\n` `\r` `\t`，防止破坏表格布局

### Hermes 元数据字段参考（2026-06-11）

**sessions.json**（gateway 管理，实时）：

| 字段 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `session_key` | string | 全局唯一键，编码 agent/channel/chat | `agent:main:weixin:dm:o9cq...@im.wechat` |
| `session_id` | string | 会话短 ID | `20260611_144133_0d5a2c1e` |
| `created_at` | time (local, no tz) | 创建时间 | `2026-06-11T14:41:33.639638` |
| `updated_at` | time (local, no tz) | 最后活跃时间 | `2026-06-11T15:11:17.276941` |
| `display_name` | string/null | 显示名称（可手动设置） | `null` |
| `platform` | string | 平台 | `weixin` / `cli` |
| `chat_type` | string | 聊天类型 | `dm` / `group` |
| `origin.platform` | string | 消息来源平台 | `weixin` |
| `origin.chat_name` | string/null | 渠道名称 | `null` |
| `origin.user_name` | string | 用户标识 | `o9cq803...@im.wechat` |
| `origin.chat_topic` | string/null | 群聊主题 | `null` |
| `input_tokens` | int | 累计输入 token | `0`（gateway session 不在此累积） |
| `output_tokens` | int | 累计输出 token | `0` |
| `total_tokens` | int | 累计总 token | `0` |
| `last_prompt_tokens` | int | 最近一次 prompt token 数 | `44169` |
| `estimated_cost_usd` | float | 预估费用 | `0.0` |
| `suspended` | bool | 是否挂起 | `false` |
| `resume_pending` | bool | 是否等待恢复 | `false` |

**request_dump**（事后快照，每次 API 调用一个文件）：

| 字段 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `timestamp` | time | 快照时间 | `2026-06-11T14:41:42` |
| `session_id` | string | 所属 session | `20260611_144133_0d5a2c1e` |
| `reason` | string | 快照原因 | `max_retries_exhausted` / `non_retryable_client_error` |
| `request.method` | string | HTTP 方法 | `POST` |
| `request.url` | string | API 端点 | `https://api.kimi.com/coding/chat/completions` |
| `request.body.model` | string | 使用的模型 | `kimi-for-coding` |
| `request.body.system` | string | **系统 prompt（含 cwd）** | `Current working directory: /root/code/pancake` |
| `request.body.messages` | array | 对话消息 | 最后一条为用户消息 |
| `error` | object/null | 错误信息 | `{"type":"invalid_request_error",...}` |

**gateway_state.json**（gateway 进程状态）：

| 字段 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `pid` | int | gateway 进程 PID | `14` |
| `gateway_state` | string | gateway 状态 | `running` / `stopped` |
| `active_agents` | int | 活跃 agent 数 | `0` |
| `platforms.<name>.state` | string | 各平台连接状态 | `connected` / `paused` |
| `platforms.<name>.error_message` | string/null | 平台错误信息 | `null` |

**cron jobs**（`cron/jobs.json`）：

| 字段 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `id` | string | cron job ID | `3910e8758c9b` |
| `name` | string | 任务名称 | `knowledge-nudge-daily` |
| `schedule.kind` | string | 调度类型 | `cron` |
| `schedule.expr` | string | cron 表达式 | `0 9 * * *` |
| `enabled` | bool | 是否启用 | `true` |
| `workdir` | string/null | **工作目录（可配置）** | `null`（当前未设置） |
| `origin.platform` | string | 投递平台 | `weixin` |
| `last_run_at` | time | 上次运行时间 | `2026-06-11T09:00:55+08:00` |
| `last_status` | string | 上次状态 | `error` / `success` |

**结论**：
- Hermes **没有**原生的 cwd/project 字段（sessions.json 不含、无 `.hermes/config` project 设置）
- **cwd 的唯一可靠来源**：request_dump 文件内 system prompt 中的 `Current working directory: <path>` 行
  - CLI session → 真实项目目录（如 `/root/code/pancake`）✅ 已实现提取
  - weixin/cron session → `/`（无意义，回退 platform 名）✅ 已实现
- cron job 配置中 `workdir` 字段为 `null`（当前未使用），后续若 cron 配置了 workdir 可优先使用
- assistant 回复内容在 `state.db` SQLite 中（`messages` 表），Last Resp 暂未接入（见 backlog P2）

### P0-2 实现：Dashboard API + CLI 子命令 + 参数化查询（2026-06-11）

**新增/修改文件**：

| 文件 | 功能 |
|------|------|
| `internal/config/config.go` | `ScanOptions` 配置类型、`ParseWindow()` 解析 `1h`/`3h`/`1d` 等时间窗口 |
| `internal/api/server.go` | HTTP API 服务器，`GET /api/v1/dashboard?window=1d&max_inactive=1` |
| `cmd/pflow/main.go` | CLI 子命令重构：`status`/`probe`/`serve` + flags |
| `internal/claude/activity.go` | 新增 `IsActive()`/`TrafficLight()`/`StatusLabel()` 方法，`Scan()` 接受 `ScanOptions` |
| `internal/hermes/activity.go` | 同上，字段 `IsActive` → `IsGatewayTracked`（避免与方法名冲突） |

**状态枚举与红绿灯映射**：

| Agent | 状态 | 红绿灯 | IsActive |
|-------|------|--------|----------|
| Claude | `busy` | 🟢 | true |
| Claude | `waiting` | 🟡 | true |
| Claude | `idle` | ⚪ | true |
| Claude | `inactive` | ⚫ | false |
| Hermes | running | 🟢 | true |
| Hermes | inactive | ⚫ | false |

**CLI 子命令**：

| 命令 | 说明 | 参数 |
|------|------|------|
| `pflow` (无参数) | 等同于 `pflow status`，使用默认值 | — |
| `pflow status` | 终端表格展示所有 session | `--window 1d`、`--max-inactive 0` |
| `pflow probe <id>` | 探测单个 session 详细状态 | session ID 或前缀（支持 Claude + Hermes） |
| `pflow serve` | 启动 HTTP API 服务器 | `--port 8080` |

**max_inactive 过滤逻辑**：
- 按 PROJECT 分组，active session 全部保留，inactive 仅保留最近 N 个
- `max_inactive=0` 表示不限制（展示全部）
- 适用于两个 agent 类型，各自使用自己的 `IsActive()` 判断

**ParseWindow 支持格式**：`30s`、`90m`、`3h`、`1d`、`2d6h`、`1w`、`90`（裸数字=小时），以及 Go 标准 `time.ParseDuration` 格式