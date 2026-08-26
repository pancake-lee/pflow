# 阶段一 & 阶段二：可行性验证 + Web Dashboard 基础

> 周期：2026-06-11 | 状态：✅ 已完成

## 阶段切换

- 阶段一已完成，P1/P2 未完成项回写到 [`backlog.md`](../../backlog.md)
- Web Dashboard 从 P3（远期）提前到阶段二，优先于 TUI 方案
- 前端选型确认：**Vue 3 + Naive UI + TypeScript + Vite**
- 理由：模板语法适合 Dashboard，Naive UI DataTable/Tag/Card 开箱即用，暗色主题一流
- 部署方案：Vue SPA 打包后通过 `//go:embed` 嵌入 Go binary，`pflow serve` 单文件部署

## 前端嵌入与部署流程

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

## 实现记录（2026-06-11）

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

## Subprocess 管理 + Stream-json 解析

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

## P0-2 实现：Dashboard API + CLI 子命令 + 参数化查询

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

## 遗留待办

- [ ] `pflow attach` 独立子命令（当前通过 `pflow claude` 自带 attach + Web 终端实现）
- [ ] WebSocket 实时推送（替代轮询）
- [ ] Hermes Last Resp 提取（SQLite 接入）
