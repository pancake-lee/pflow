# 技术设计文档

> 配合 [PRD](./prd.md) 阅读。本文档描述 pflow 的技术架构、模块设计、以及如何友好地复用 cc-connect 的已有能力。

## 1. 与 cc-connect 的关系

### 1.1 定位差异

| | cc-connect | pflow |
|---|-----------|------|
| **核心场景** | AI Agent ↔ IM 平台的桥接器（微信/飞书/Slack 中与 Agent 对话） | 多 Agent 会话的注意力管理与调度（终端内管理并行 Agent） |
| **用户交互** | 通过 IM 消息与 Agent 交互 | 通过终端 Dashboard + tmux/VSCode 与 Agent 交互 |
| **核心抽象** | Engine → Agent + Platform（消息进出双向） | Manager → 多个 Agent Session（状态聚合 + 注意力引导） |
| **依赖方向** | 14 个 IM Platform 适配器 + 多个 Agent 适配器 | 仅 Agent 适配层（以 Claude Code CLI 为主），无 IM 层 |

**结论**：两者设计初衷不同，pflow 不是 cc-connect 的 Fork，而是一个独立项目。但在"与 Claude Code CLI 进程通信"这一底层能力上，cc-connect 已有成熟实现，pflow 将友好地复用——根据实际情况灵活选择 import、复制或向上游贡献。

### 1.2 复用原则

| 原则 | 说明 |
|------|------|
| **按需选择** | 不预先划定哪些模块 import、哪些模块复制。遇到具体问题时，根据耦合度、接口清晰度、修改量逐案决定。如果只是引用类型定义或工具函数，优先 import；如果需要的逻辑紧密耦合在 cc-connect 的内部流程中，考虑复制；如果发现 cc-connect 有可改进之处，考虑向上游贡献。 |
| **完整标注** | 凡引用或复制自 cc-connect 的代码，在文件头部标注原始来源、版权声明、修改说明 |
| **MIT 合规** | 在项目 README 和 NOTICE 文件中声明对 cc-connect 的依赖和感谢 |

## 2. pflow 模块架构

```
pflow/
├── cmd/
│   └── pflow/
│       └── main.go                 # CLI 入口
├── internal/
│   ├── claude/                     # Claude Code CLI 进程监控
│   │   ├── activity.go             # Session 活动扫描（从 transcript 文件读取）
│   │   ├── stream.go               # stream-json 类型定义与解析
│   │   ├── subprocess.go           # CLI 子进程管理
│   │   └── snapshot.go             # SessionSnapshot 状态快照
│   ├── hermes/                     # Hermes Agent 会话监控
│   │   └── activity.go             # Session 活动扫描（从 sessions.json + request_dump 读取）
│   ├── session/                    # Tmux + ttyd 会话管理（阶段三）
│   │   ├── manager.go              # Tmux/ttyd 进程生命周期管理
│   │   ├── claude.go               # Claude statusline 配置 + 启动 + capture-pane 前缀解析
│   │   └── mapping.go              # Tmux↔Claude session 映射持久化
│   ├── api/                        # HTTP API
│   │   └── server.go               # Dashboard API + 终端管理 API
│   └── config/                     # 配置管理
│       └── config.go               # ScanOptions + ParseWindow
├── web/                            # Web Dashboard 前端
│   ├── src/
│   │   ├── components/             # Vue 组件
│   │   ├── composables/            # 组合式函数（useDashboard, usePolling, format）
│   │   ├── types/                  # TypeScript 类型定义
│   │   └── views/                  # 页面视图
│   ├── index.html
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── package.json
├── embed.go                        # //go:embed web/dist/*
├── docs/
│   ├── prd.md
│   ├── tech.md
│   ├── backlog.md
│   ├── changelog.md
│   ├── note.md
│   └── archive/                    # 历史产出归档
├── .local/                         # 个人工作文档（gitignored）
├── Makefile
├── go.mod
├── go.sum
├── LICENSE                         # MIT
├── NOTICE                          # 第三方依赖版权声明
└── README.md
```

### 2.1 模块分层

```
┌──────────────────────────────────────────────────┐
│  CLI / Web UI（cmd/pflow, web/）                  │  ← 用户界面
├──────────────────────────────────────────────────┤
│  会话管理层（internal/session）                    │  ← tmux + ttyd + statusline 关联
├──────────────────────────────────────────────────┤
│  调度层（internal/manager, attention）             │  ← pflow 核心（规划中）
├──────────────────────────────────────────────────┤
│  Agent 适配层（internal/claude, internal/hermes）  │  ← Agent 状态监控
├──────────────────────────────────────────────────┤
│  公共服务（internal/api, config）                  │  ← 基础设施
└──────────────────────────────────────────────────┘
```

### 2.2 核心数据流

```
Claude Code 进程 → transcript 文件 / Hermes → sessions.json + request_dump
                                            ↓
                                    SessionSummary（统一状态快照）
                                            ↓
                                    API 聚合多 Session
                                            ↓
                                    Dashboard API（JSON）
                                       ↓         ↓
                                  CLI 文本表格   Web Dashboard（Vue 3）
                                                    ↓
                                              Web 终端（ttyd + tmux）
```

`SessionSummary` 是 pflow 的核心数据模型——每个 Agent 会话对外暴露一个只读快照，包含：会话标识、当前状态（busy/waiting/idle/inactive）、最近操作摘要、上下文用量、进程存活状态。Dashboard API 层负责聚合所有 session 的快照并返回。

**Tmux 终端数据流**：

```
pflow claude 启动
  ├── 1. 配置 ~/.claude/settings.json (statusline)
  ├── 2. tmux new-session + Claude
  ├── 3. tmux capture-pane → 提取 8-char session 前缀
  └── 4. 保存映射到 ~/.pflow/mappings.json

Dashboard 打开详情时
  ├── 1. GET /api/v1/terminal/lookup?session_id=<prefix>
  ├── 2. 后端查 mappings.json → 匹配 tmux session
  ├── 3. 返回 ttyd URL → 前端 iframe 嵌入 Web 终端
  └── 4. ttyd → WebSocket → tmux attach → 交互式 Claude 会话
```

## 3. 实施阶段

### 3.1 阶段一：可行性验证 ✅ 已完成

构建最小技术验证链路：启动 Claude Code → 拿到事件流 → 导出状态快照 → CLI 显示。

产出：`pflow status`、`pflow probe`、`pflow serve`（Dashboard API）、双 Agent 支持（Claude + Hermes）。

### 3.2 阶段二：Web Dashboard ✅ 已完成

**产出**：Vue 3 + Naive UI 浏览器端可视化面板。

| 步骤 | 内容 | 产出 |
|------|------|------|
| 1. 项目初始化 | Vite + Vue 3 + TypeScript + Naive UI | `web/` 目录，可运行的空 Dashboard |
| 2. Dashboard 主页面 | DataTable 会话列表、红绿灯渲染、筛选控制栏、统计摘要 | 可用的浏览器面板 |
| 3. 会话详情 | 点击展开/抽屉显示完整 session 信息 | 详情视图（含可调宽度侧边栏） |
| 4. 自动刷新 | 可配置轮询间隔，调用现有 `/api/v1/dashboard` | 准实时更新 |
| 5. Go embed 集成 | `//go:embed web/dist`，`pflow serve` 同时提供 API + 静态资源 | 单二进制部署 |

### 3.3 阶段三：CLI 能力扩展 ← 当前阶段

**目标**：打通"从 Dashboard 看到 Agent 状态"到"一键进入 Agent 会话交互"的闭环。

| 步骤 | 内容 | 状态 |
|------|------|------|
| 1. `pflow claude` 子命令 | 创建 tmux + Claude 托管会话，自动配置 statusline、提取 session 前缀 | ✅ 已完成 |
| 2. Web 终端集成 | Dashboard 侧边栏通过 ttyd + tmux 嵌入交互式终端，通过 statusline 关联 Claude session | ✅ 已完成 |
| 3. Session 映射持久化 | `~/.pflow/mappings.json` 存储 tmux↔Claude 关联，Dashboard API 自动 lookup 并标注可终端连接的 session | ✅ 已完成 |
| 4. `pflow attach` 子命令 | 独立的终端唤醒命令，按 session ID 或项目名查找并 attach | 待实现 |
| 5. `pflow suggest` | 军情哨手动触发分析建议 | 待实现 |
| 6. `pflow focus` | 主攻/侧翼配置 | 待实现 |

### 3.4 后续阶段

| 阶段 | 内容 |
|------|------|
| 阶段四：智能调度 | 军情哨主动推送、统帅偏好学习、战局图 |
| 阶段五：体验层 | TUI Dashboard、游戏化外壳、VSCode 扩展、跨设备同步 |

## 4. MIT 协议合规

### 4.1 对 cc-connect 的版权声明

在项目 `NOTICE` 文件中声明对 cc-connect 的依赖和感谢。凡复制或修改自 cc-connect 的代码，在文件头部标注：

```go
// Copyright (c) 2025 chenhg5 (cc-connect contributors)
// Source: https://github.com/chenhg5/cc-connect/blob/main/<path>
// Licensed under the MIT License.
//
// Modifications for pflow:
//   - ...
```

### 4.2 pflow 自身

pflow 以 MIT License 发布，`LICENSE` 文件已就位。

## 5. 技术选型

### 5.1 后端

| 选择 | 方案 | 理由 |
|------|------|------|
| 语言 | Go | CLI 工具首选，子进程管理和并发模型优秀 |
| TUI 框架 | Bubble Tea (charmbracelet) | 成熟稳定，社区活跃（当前搁置，Web 先行） |
| HTTP 路由 | 标准库 net/http | 端点少，标准库足够 |
| 配置格式 | TOML | Go 生态主流，可读性好 |
| 数据持久化 | JSON 文件（`~/.pflow/mappings.json`） | 简单够用，后续可升级 SQLite |

### 5.2 前端（Web Dashboard）

| 选择 | 方案 | 理由 |
|------|------|------|
| 框架 | **Vue 3** (Composition API) | 模板语法天然适合 Dashboard 类数据展示页面；SFC 直观、学习曲线平缓 |
| 组件库 | **Naive UI** | 树摇优化、暗色主题一流；DataTable / Tag / Card / Drawer 等组件开箱即用；TypeScript 支持完善 |
| 语言 | **TypeScript** | 类型安全，与 Go 后端的 JSON API 对接时有明确的数据契约 |
| 构建 | **Vite** | 秒级 HMR，开发体验极佳；Rollup 生产构建 |
| 部署 | **Go embed** (`//go:embed web/dist`) | 前端打包为静态资源嵌入 Go binary，`pflow serve` 单文件部署 |

### 5.3 前后端交互

```
浏览器                          Go Server
  │                                │
  │  GET /api/v1/dashboard         │
  │  ?window=1d&max_inactive=1     │
  │ ─────────────────────────────> │
  │                                │  claude.Scan() + hermes.Scan()
  │  JSON {sessions:[...]}         │
  │ <───────────────────────────── │
  │                                │
  │  (轮询 10s/30s/60s 可配置)      │
  │                                │
  │  GET / (静态资源)               │
  │ ─────────────────────────────> │
  │  index.html + Vue SPA          │  //go:embed web/dist/*
  │ <───────────────────────────── │
```

- **当前方案**：前端轮询 `/api/v1/dashboard`，间隔可配置
- **后续升级**：WebSocket 实时推送（`GET /api/v1/dashboard/ws`），状态变化时服务端主动推送

### 5.4 Web 终端集成（ttyd + tmux + Claude 关联）✅ 已实现

**核心思路**：通过 Claude 的 `/statusline` 功能，在终端状态行显示 session ID 的前 8 个字符作为前缀。pflow 管理的 tmux session 可以通过 `tmux capture-pane` 解析出这个前缀，从而建立 **tmux session ↔ Claude session** 的关联。

**关联流程**：

```
pflow claude 启动流程:
  1. 检查并配置 Claude statusline（~/.claude/settings.json）
     → 状态行格式: "sid8 | model | ctx | tok | session"
  2. 创建 tmux session + 启动 Claude
  3. 异步 wait + tmux capture-pane 提取 8-char session 前缀
  4. 保存映射到 ~/.pflow/mappings.json

Dashboard 打开详情时:
  1. GET /api/v1/terminal/lookup?session_id=<uuid>
  2. 后端读取 mappings.json，按前缀匹配
  3. 找到 → 返回 tmux 会话信息（含 ttyd URL），前端可启动/连接 Web 终端
  4. 未找到 → 提示用户使用 pflow claude 启动
```

**架构**：

```
浏览器 Dashboard 侧边栏
  │  iframe 嵌入 ttyd 终端
  ▼
ttyd 进程 (端口 10000+)
  │  WebSocket + PTY
  ▼
tmux session (pflow-<name>)
  │  Claude Code（statusline 显示 session ID 前缀）
  ▼
项目工作目录
```

**组件实现**：

| 组件 | 角色 | 文件 |
|------|------|------|
| Session Manager | tmux + ttyd 进程管理器：创建/销毁会话、分配端口、追踪进程状态 | `internal/session/manager.go` |
| Claude Session | Claude statusline 配置、Claude 进程启动、capture-pane 前缀解析 | `internal/session/claude.go` |
| Mapping | tmux↔Claude session 映射持久化（`~/.pflow/mappings.json`） | `internal/session/mapping.go` |
| CLI: `pflow claude` | 一键创建 tmux + Claude 托管会话 | `cmd/pflow/main.go:runClaudeCmd` |
| API: terminal/start | 启动 ttyd 并连接到指定 tmux session | `internal/api/server.go` |
| API: terminal/stop | 停止 ttyd 进程（可选保留 tmux） | `internal/api/server.go` |
| API: terminal/list | 列出当前活跃的终端会话 | `internal/api/server.go` |
| API: terminal/lookup | 按 Claude session ID 查找关联的 tmux 会话 | `internal/api/server.go` |

**CLI 使用**：

```bash
# 默认：在当前目录创建托管 session
pflow claude

# 指定项目目录
pflow claude -dir /path/to/project

# 同一项目启动多个 Claude（不同 session 名）
pflow claude -name fix-bug

# 后台启动（不 attach）
pflow claude -no-attach
```

**Statusline 配置**：

pflow 会自动配置 `~/.claude/settings.json` 中的 statusline。如果用户已有自定义配置，会给出提示。用户可以：
1. `pflow claude -force` 覆盖为 pflow 的配置
2. 手动在自己的 statusline 最前面添加 `sid8` 变量（8 位 session ID 前缀）

**外部依赖**：
- `tmux`（必须）— 终端多路复用
- `ttyd`（必须）— Web 终端网关，用户需自行安装：`dnf install ttyd` 或 `apt install ttyd`
- `jq`（必须）— Claude statusline 命令中用于解析 JSON

**安全设计**：
- ttyd 绑定 `127.0.0.1` 仅监听本地，不暴露到公网
- 每个会话分配独立端口（默认从 10000 开始递增）
- 生产环境建议通过 nginx 反向代理 + 认证保护

**设计权衡**：

- 只处理通过 `pflow claude` 启动 + 配合 statusline 配置的会话。原生终端启动、自建 tmux、Claude 退出又重进等路径不保证关联成功。
- 关联失败时不影响 Dashboard 的核心功能——Terminal 面板是可选的增强功能。
- 8 位前缀并非完整 UUID，但同一项目内足以区分不同会话。
