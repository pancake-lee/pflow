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
│       └── main.go              # CLI 入口
├── internal/
│   ├── claude/                  # Claude Code CLI 进程管理
│   │   ├── session.go           # Claude CLI 子进程管理、stream-json 解析、事件流
│   │   ├── proc_unix.go         # Unix 进程组管理
│   │   └── snapshot.go          # SessionSnapshot 状态快照
│   ├── manager/                 # Agent Session 管理器
│   │   ├── manager.go           # 多 Session 生命周期管理
│   │   ├── dashboard.go         # 状态聚合与 Dashboard 数据模型
│   │   └── eventloop.go         # 事件消费循环
│   ├── attention/               # 军情哨（注意力管理器）
│   │   ├── analyzer.go          # 状态分析引擎
│   │   └── advisor.go           # LLM 引导建议生成
│   ├── api/                     # HTTP API
│   │   ├── server.go            # Dashboard API server
│   │   └── handlers.go          # API 端点处理
│   └── config/                  # 配置管理
│       └── config.go            # TOML 配置解析
├── pkg/
│   └── tui/                     # TUI Dashboard（阶段二）
│       └── dashboard.go         # Bubble Tea 界面
├── docs/
│   ├── prd.md
│   └── tech-design.md
├── .local/                      # 个人工作文档（gitignored）
├── go.mod
├── go.sum
├── LICENSE                      # MIT
├── NOTICE                       # 第三方依赖版权声明
└── README.md
```

### 2.1 模块分层

```
┌─────────────────────────────────────────┐
│  CLI / TUI（cmd/pflow, pkg/tui）         │  ← 用户界面
├─────────────────────────────────────────┤
│  调度层（internal/manager, attention）    │  ← pflow 核心
├─────────────────────────────────────────┤
│  Agent 适配层（internal/claude）          │  ← Claude Code CLI 通信
├─────────────────────────────────────────┤
│  公共服务（internal/api, config）         │  ← 基础设施
└─────────────────────────────────────────┘
```

### 2.2 核心数据流

```
Claude Code 进程 → stream-json 事件流 → Event Loop 解析
                                            ↓
                                    SessionSnapshot（状态快照）
                                            ↓
                                    Manager 聚合多 Session
                                            ↓
                                    Dashboard API（JSON）
                                            ↓
                                    CLI / TUI 展示
```

`SessionSnapshot` 是 pflow 的核心数据模型——每个 Agent 会话对外暴露一个只读快照，包含：会话标识、当前状态（busy/waiting/idle）、最近操作摘要、上下文用量、进程存活状态。Dashboard API 层负责聚合所有 session 的快照并返回。

## 3. 阶段一实施计划

### 3.1 目标

构建最小技术验证链路：启动 Claude Code → 拿到事件流 → 导出状态快照 → CLI 显示。

### 3.2 步骤

| 步骤 | 内容 | 产出 |
|------|------|------|
| 1. 项目初始化 | `go mod init`，按需添加依赖 | 可编译的空项目 |
| 2. 实现 Claude 进程管理 | 实现 Claude CLI 子进程启动、stdin/stdout 管道通信、stream-json 事件解析 | 能启动 Claude Code 并拿到事件流 |
| 3. 实现状态快照 | 基于事件流推断状态（busy/waiting/idle），暴露 `Snapshot()` 方法 | `SessionSnapshot` 数据模型 |
| 4. 构建 Manager | 实现多 session 注册/注销，聚合生成 `Dashboard` | `GET /api/v1/dashboard` 返回 JSON |
| 5. CLI 原型 | `pflow probe` 和 `pflow status` 命令，输出文本表格 | 终端可见的状态仪表盘 |
| 6. 验证测试 | 同时启动 2-3 个 Claude Code 会话，验证状态准确性 | 验证报告 |

### 3.3 验证指标

| 指标 | 目标 | 测试方法 |
|------|------|---------|
| busy/waiting/idle 三态准确率 | > 80% | 人工标记 vs 系统判断 |
| 权限请求检测率 | > 70% | 触发工具调用，检测 permission request 事件 |
| 上下文用量准确度 | 与 `claude --version` 输出一致 | 对比系统取值与终端显示 |
| Dashboard API 延迟 | < 100ms | 单次请求耗时 |
| 多 session 并发 | 3 个 session 无串扰 | 同时运行，各自状态独立 |

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

| 选择 | 方案 | 理由 |
|------|------|------|
| 语言 | Go | CLI 工具首选，子进程管理和并发模型优秀 |
| TUI 框架 | Bubble Tea (charmbracelet) | 成熟稳定，社区活跃 |
| HTTP 路由 | 标准库 net/http（阶段一）/ chi（后续） | 阶段一仅 2-3 个端点，标准库足够 |
| 配置格式 | TOML | Go 生态主流，可读性好 |
| 数据持久化 | JSON 文件（阶段一）→ SQLite（后续） | 渐进式，先简单后扩展 |
