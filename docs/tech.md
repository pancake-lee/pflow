# 技术设计文档

> 配合 [PRD](./prd.md) 阅读。本文档描述 pflow 的技术架构、模块设计、以及如何友好地复用 cc-connect 的已有能力。

## 1. 与 cc-connect 的关系

### 1.1 定位差异

| | cc-connect | pflow |
|---|-----------|------|
| **核心场景** | AI Agent ↔ IM 平台的桥接器（微信/飞书/Slack 中与 Agent 对话） | 多 Agent 会话的注意力管理与调度（信息聚合 + 策略管理） |
| **用户交互** | 通过 IM 消息与 Agent 交互 | 通过 Dashboard 获取信息，用户自行切换到终端/VSCode 操作 |
| **核心抽象** | Engine → Agent + Platform（消息进出双向） | Project → Session（项目维度的状态聚合 + 优先级策略） |
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
│   ├── project/                    # 项目管理（阶段三新增）
│   │   ├── store.go                # ~/.pflow/projects.json 读写
│   │   └── strategy.go             # 主线/支线策略校验与切换
│   ├── session/                    # Tmux + ttyd 会话管理
│   │   ├── manager.go              # Tmux/ttyd 进程生命周期管理
│   │   ├── claude.go               # Claude statusline 配置 + 启动 + capture-pane 前缀解析
│   │   └── mapping.go              # Tmux↔Claude session 映射持久化
│   ├── api/                        # HTTP API
│   │   └── server.go               # Dashboard API + 项目 CRUD + 策略 API
│   └── config/                     # 配置管理
│       └── config.go               # ScanOptions + ParseWindow
├── web/                            # Web Dashboard 前端
│   ├── src/
│   │   ├── components/             # Vue 组件
│   │   ├── composables/            # 组合式函数
│   │   ├── types/                  # TypeScript 类型定义
│   │   └── views/                  # 页面视图
│   ├── index.html
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── package.json
├── embed.go                        # //go:embed web/dist/*
├── docs/                           # 文档
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
│  策略层（internal/project）                       │  ← 主线/支线策略、项目优先级管理
├──────────────────────────────────────────────────┤
│  会话管理层（internal/session）                    │  ← tmux + ttyd + statusline 关联
├──────────────────────────────────────────────────┤
│  Agent 适配层（internal/claude, internal/hermes）  │  ← Agent 状态监控
├──────────────────────────────────────────────────┤
│  公共服务（internal/api, config）                  │  ← 基础设施
└──────────────────────────────────────────────────┘
```

### 2.2 核心数据流

```
                    ┌──────────────────────┐
                    │  projects.json       │  ← 项目定义 + 优先级策略
                    └──────┬───────────────┘
                           │ projectId 关联
                    ┌──────▼───────────────┐
Claude transcript ──┤                      │
Hermes sessions ─── │  SessionSummary      │──→ Dashboard API (JSON)
Mappings ────────── │                      │
                    └──────────────────────┘
                           │
                    ┌──────▼───────────────┐
                    │  Web Dashboard       │
                    │  项目卡片 → session  │
                    │  主线/支线/普通分区   │
                    └──────────────────────┘
```

### 2.3 项目数据模型

```json
// ~/.pflow/projects.json
{
  "version": 1,
  "projects": [
    {
      "id": "proj-1",
      "name": "pflow",
      "path": "/home/user/code/pflow",
      "priority": "primary",
      "created_at": "2026-06-13T00:00:00Z",
      "sort_order": 0
    },
    {
      "id": "proj-2",
      "name": "周报",
      "path": "",
      "priority": "secondary",
      "created_at": "2026-06-13T00:00:00Z",
      "sort_order": 1
    },
    {
      "id": "proj-3",
      "name": "未分类",
      "path": "",
      "priority": "normal",
      "created_at": "2026-06-13T00:00:00Z",
      "is_default": true
    }
  ]
}
```

**Session ↔ Project 关联**：每个 session 通过 `projectId` 字段关联到项目。对于本地 Agent（Claude Code），可按工作目录自动匹配；未匹配的 session 归入"未分类"默认项目。

**优先级语义**：

| priority | 含义 | 数量限制 | Dashboard 展示 |
|----------|------|---------|---------------|
| `primary` | 今日主线 | 1 | ⭐ 独立区域，始终展开 |
| `secondary` | 支线 | 最多 3 | 🚩 独立区域，始终展开 |
| `normal` | 普通关注 | 不限 | 📁 可折叠区域 |
| `archived` | 已归档 | 不限 | 📦 可折叠，默认折叠 |

**优先级切换规则**：
- 设为主线：原主线降为 normal，目标升为 primary
- 设为支线：若支线数 < 3 则直接加入；已满则拒绝并提示
- 设为普通：从 primary/secondary 移除
- 归档：从当前区域移入 archived

## 3. 实施阶段

### 3.1 阶段一：可行性验证 ✅ 已完成

构建最小技术验证链路。产出：`pflow status`、`pflow probe`、`pflow serve`、双 Agent 支持。

### 3.2 阶段二：Web Dashboard ✅ 已完成

Vue 3 + Naive UI 浏览器端面板。产出：`pflow serve` 单二进制部署、`pflow claude` tmux 托管、Web 终端集成。

### 3.3 阶段三：项目策略管理 ← 当前阶段

**目标**：从"扁平 session 列表"升级为"项目 → session"两级结构，支持用户设定主线/支线策略。

| 步骤 | 内容 | 产出 |
|------|------|------|
| 1. 数据层 | `internal/project/` 包：`~/.pflow/projects.json` 读写、CRUD、策略校验 | Go 包 + 测试 |
| 2. Session 关联 | 扩展现有 scan 流程，session 增加 `projectId`；按工作目录自动归类；历史数据迁移逻辑 | 兼容旧数据 |
| 3. API 层 | Dashboard API 返回 projects + sessions 两级结构；新增项目 CRUD 端点 | REST API |
| 4. 前端重构 | 替换扁平表格为项目卡片视图，按优先级分区展示，优先级切换交互 | Vue 组件 |
| 5. 策略引擎 | 优先级切换规则、数量校验（1 主线 + 最多 3 支线）、边界条件处理 | 后端校验 |

### 3.4 后续阶段

| 阶段 | 内容 |
|------|------|
| 阶段四：智能调度 | 沉默提醒、军情哨主动推送、偏好学习 |
| 阶段五：体验层 | TUI Dashboard、游戏化外壳、跨设备同步 |

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
| HTTP 路由 | 标准库 net/http（Go 1.22+ 增强路由） | 端点少，标准库足够 |
| 数据持久化 | JSON 文件（`~/.pflow/`） | 简单够用，无需外部数据库 |
| 配置格式 | JSON（与 Claude settings.json 风格一致） | 用户可手动编辑 |

### 5.2 前端（Web Dashboard）

| 选择 | 方案 | 理由 |
|------|------|------|
| 框架 | **Vue 3** (Composition API) | 模板语法天然适合 Dashboard 类数据展示页面 |
| 组件库 | **Naive UI** | 树摇优化、暗色主题一流；DataTable / Tag / Card / Drawer 等组件开箱即用 |
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
  │  JSON {                        │  + project.Load() + 关联
  │    projects: [...],            │
  │    sessions: [...]             │
  │  }                             │
  │ <───────────────────────────── │
  │                                │
  │  PUT /api/v1/projects/:id      │
  │  {"priority": "primary"}       │
  │ ─────────────────────────────> │
  │                                │  strategy.SetPrimary("proj-1")
  │  200 OK                        │
  │ <───────────────────────────── │
```

- **当前方案**：前端轮询 `/api/v1/dashboard`，间隔可配置
- **后续升级**：WebSocket 实时推送

### 5.4 Web 终端集成（ttyd + tmux + Claude 关联）✅ 已实现

**定位**：辅助功能，非主要交互路径。用户的主要操作在自有终端/VSCode 中进行，Web 终端仅作为便捷备选。

**核心思路**：通过 Claude 的 `/statusline` 功能，在终端状态行显示 session ID 的前 8 个字符作为前缀。pflow 管理的 tmux session 可以通过 `tmux capture-pane` 解析出这个前缀，从而建立 **tmux session ↔ Claude session** 的关联。

**关联流程**：

```
pflow claude 启动流程:
  1. 检查并配置 Claude statusline（~/.claude/settings.json）
  2. 创建 tmux session + 启动 Claude
  3. 异步 wait + tmux capture-pane 提取 8-char session 前缀
  4. 保存映射到 ~/.pflow/mappings.json

Dashboard 可选操作:
  1. GET /api/v1/terminal/lookup?session_id=<prefix>
  2. 后端查 mappings.json → 匹配 tmux session
  3. 返回 ttyd URL → 前端可选连接 Web 终端
```

**组件实现**：

| 组件 | 角色 | 文件 |
|------|------|------|
| Session Manager | tmux + ttyd 进程管理器：创建/销毁会话、分配端口、追踪进程状态 | `internal/session/manager.go` |
| Claude Session | Claude statusline 配置、Claude 进程启动、capture-pane 前缀解析 | `internal/session/claude.go` |
| Mapping | tmux↔Claude session 映射持久化（`~/.pflow/mappings.json`） | `internal/session/mapping.go` |
| CLI: `pflow claude` | 一键创建 tmux + Claude 托管会话 | `cmd/pflow/main.go:runClaudeCmd` |
| API: terminal/* | 终端启动/停止/列表/lookup 端点 | `internal/api/server.go` |

**外部依赖**：
- `tmux`（必须）— 终端多路复用
- `ttyd`（可选）— Web 终端网关
- `jq`（必须）— Claude statusline 命令中用于解析 JSON

**安全设计**：
- ttyd 绑定 `127.0.0.1` 仅监听本地，不暴露到公网
- 每个会话分配独立端口（默认从 10000 开始递增）
