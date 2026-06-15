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
│   │   ├── store.go                # ~/.pflow/project_roots.json 读写
│   │   └── strategy.go             # 主线/支线策略校验与切换
│   ├── attention/                  # 注意力管理（阶段四新增）
│   │   ├── activity.go             # 用户活跃追踪（streak / total / lastActiveTime）
│   │   ├── score.go                # 提醒分数计算引擎
│   │   └── config.go               # 可配置常量（PROTECT_MIN, W_WAIT, ...）
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
│  注意力层（internal/attention）                    │  ← 提醒分数计算、活跃追踪
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
                    │  project_roots.json  │  ← 项目定义 + 优先级策略
                    └──────┬───────────────┘
                           │ priority + matched_root
                    ┌──────▼───────────────┐
Claude transcript ──┤                      │
Hermes sessions ─── │  SessionSummary      │──→ attention.Score() ──→ reminder_score
Mappings ────────── │                      │
                    └──────────────────────┘
                           │
                    ┌──────▼───────────────┐
                    │  Dashboard API (JSON)│
                    │  sessions + scores   │
                    └──────┬───────────────┘
                           │
                    ┌──────▼───────────────┐
                    │  Web Dashboard       │
                    │  项目卡片 → session  │
                    │  遮罩层 ← score      │
                    │  主线/支线/普通分区   │
                    └──────────────────────┘
```

### 2.3 项目数据模型

**路径即项目**。不引入独立的项目 ID/名称实体。session 元数据中已有的 working directory 就是天然的归属标识。

```json
// ~/.pflow/project_roots.json
{
  "version": 1,
  "roots": [
    { "path": "/home/user/code/pflow", "priority": "primary" },
    { "path": "/home/user/code/hermes", "priority": "secondary" },
    { "path": "/home/user/code/pancake", "priority": "normal" }
  ]
}
```

- `path` 是唯一键。用户通过 Dashboard 界面的 ☐ "识别为项目" 勾选框标记/取消。
- 不需要 `id`、`name`、`created_at` 等字段——路径本身就是标识，目录名就是天然的"项目名"。

**Session 自动归类**：最长前缀匹配。

```
Session cwd: /home/user/code/pflow/internal/api
Roots:
  /home/user/code/pflow         → 匹配 ✓
  /home/user/code/pflow/internal → 如果存在则匹配（更具体），否则回退到 /home/user/code/pflow

规则：
1. 遍历 roots，取所有 path 为 session.cwd 前缀的匹配
2. 选 path 最长者（最具体的匹配）
3. 无匹配 → 作为"未归类 session"展示
4. / 不能被标记为 root（API 层拒绝）
```

**优先级语义**：

| priority | 含义 | 数量限制 | Dashboard 展示 |
|----------|------|---------|---------------|
| `primary` | 今日主线 | 1 | ⭐ 独立区域，始终展开 |
| `secondary` | 支线 | 最多 3 | 🚩 独立区域，始终展开 |
| `normal` | 普通关注 | 不限 | 📁 可折叠区域 |

**优先级切换规则**：
- 设为主线：原主线降为 normal，目标升为 primary
- 设为支线：若支线数 < 3 则直接加入；已满则拒绝并提示
- 设为普通：从 primary/secondary 移除
- 取消标记（DELETE）：从 roots 中移除，匹配该 root 的 session 重新归类

### 2.4 注意力模块（`internal/attention/`）

详见 [`docs/design/02-reminder_score_algorithm.md`](./design/02-reminder_score_algorithm.md) 和 [`docs/design/03-attention_mask.md`](./design/03-attention_mask.md)。

**职责**：
- **活跃追踪**（`activity.go`）：追踪每个项目的用户活跃状态（streak 连续活跃分钟数、total 今日累计分钟数、lastActiveTime 最近活动时间戳）
- **提醒分数计算**（`score.go`）：综合 waiting 时长、streak、今日累计、优先级等因素，计算每个项目的提醒分数
- **可配置常量**（`config.go`）：PROTECT_MIN、各类权重、阈值等

**数据流**：
```
Session 状态（waiting/busy/idle）
    + 项目优先级（primary/secondary/normal）
    + 用户活跃追踪（streak/total）
        ↓
    attention.Score() → reminder_score
        ↓
    Dashboard API → 前端遮罩层 opacity
```

**MVP 简化**：初始版本用 session 状态变化作为用户活跃的代理指标（项目下有 busy session → 用户在该项目活跃），后续可接入更精确的操作监听（终端输入等）。

## 3. 实施阶段

### 3.1 阶段一：可行性验证 ✅ 已完成

构建最小技术验证链路。产出：`pflow status`、`pflow probe`、`pflow serve`、双 Agent 支持。

### 3.2 阶段二：Web Dashboard ✅ 已完成

Vue 3 + Naive UI 浏览器端面板。产出：`pflow serve` 单二进制部署、`pflow claude` tmux 托管、Web 终端集成。

### 3.3 阶段三：项目策略管理 ✅ 已完成

**目标**：从"扁平 session 列表"升级为按项目路径分组的视图，支持主线/支线策略。

| 步骤 | 内容 | 产出 |
|------|------|------|
| 1. 数据层 | `internal/project/` 包：`~/.pflow/project_roots.json` 读写、优先级校验 | Go 包 |
| 2. 归类逻辑 | 最长前缀匹配算法；根目录保护（拒绝 `/`）；向后兼容（无 roots 时全部视为未归类） | 归类函数 |
| 3. API 层 | Dashboard API 返回 `matched_root` 字段；新增 `PUT/DELETE/GET /api/v1/project-roots` | REST API |
| 4. 前端标记交互 | 每个 distinct 工作目录旁的 ☐ "识别为项目" 勾选框 + hover tooltip | Vue 组件 |
| 5. 前端分组视图 | 替换扁平表格为按 root 分组的项目视图，按优先级分区展示 | Vue 组件 |

### 3.4 阶段四：智能调度 ← 当前阶段

**目标**：实现提醒分数算法 + 注意力遮罩层，为智能调度打下基础。

| 步骤 | 内容 | 产出 |
|------|------|------|
| 1. Activity Tracker | `internal/attention/activity.go`：streak / total / lastActiveTime 追踪，每日重置 | Go 包 |
| 2. Score Calculator | `internal/attention/score.go`：提醒分数计算引擎 + 单元测试 | Go 函数 |
| 3. API 层 | Dashboard API 返回 `reminder_score` 字段 | REST API |
| 4. 前端遮罩层 | PrimaryCard/SecondaryCard/GroupCard 添加 `::before` 遮罩伪元素，opacity 关联分数 | Vue/CSS |
| 5. 提醒等级 | 低/中/高三级视觉区分（不同背景色/透明度） | Vue/CSS |

### 3.5 后续阶段

| 阶段 | 内容 |
|------|------|
| 阶段四后续 | 桌面通知、卡片动画、军情哨主动推送、偏好学习 |
| 阶段五：体验层 | TUI Dashboard、双层换肤系统、浏览器扩展监控、游戏化外壳、跨设备同步 |

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
  │  JSON {                        │  + project.MatchRoots()
  │    project_roots: [...],       │
  │    sessions: [                 │
  │      { ..., matched_root: ".." }│
  │    ]                           │
  │  }                             │
  │ <───────────────────────────── │
  │                                │
  │  PUT /api/v1/project-roots     │
  │  {"path":"/code/pflow", "priority":"primary"}
  │ ─────────────────────────────> │
  │                                │  project.SetPriority()
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
