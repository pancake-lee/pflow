# CLAUDE.md — pflow AI 工作规则

> 本文件与 [`AGENTS.md`](./AGENTS.md) 内容必须同步。完整 Harness 入口见 [`docs/harness.md`](./docs/harness.md)。

## 项目概况

pflow 是一个 Go 项目，目标是在 cc-connect 的“与 Claude Code CLI 通信”能力之上，构建多 Agent 会话的注意力管理与调度工具。详见 [`README.md`](./README.md)。

技术栈：Go（Cobra、Bubble Tea、tmux/ttyd 集成）+ Vue 3（Naive UI、TypeScript、Vite）。开发模式：AI 辅助开发，用户作为“总指挥”。

---

## 全局行为约束

以下规则适用于所有工作模式：

- **减少 Markdown 表格**：优先用列表（`-`）组织内容。仅当每行内容可控制在 80 字符以内时才使用表格。宽表格在编辑器中阅读体验差。
- **Mermaid 流程图方向**：默认 `flowchart TD`（向下）。当图中某一级存在超过 6 个平级节点时改用 `flowchart LR`（向右）。
- **方案选择**：存在多个可行方案时，先列出方案（含核心利弊）让用户选择，不要自行决定。
- **单任务串行**：一个对话回合只沿一条线索推进一个任务。不要同时诊断两个 bug、同时提两个方案、同时问两个不相关的问题。
- **规划任务落点**：方案、子任务和实施步骤必须写入 `docs/backlog.md` 对应条目，作为 Plan → Generate 的唯一交接载体；不创建独立任务计划文档或 exec plan。
- **避免上下文爆炸**：一次只聚焦一个具体问题，先读取最小必要代码，分析并解决后再继续。
- **不要滥用 try catch** 处理代码问题，要真正解决代码错误的根源。
- **根因优先**：本项目体量较小，遇到问题优先找根因并直接修复，而不是叠加容错/降级逻辑。
- **配置文件读取**：调试配置问题时，可以读取 `.local/` 下的用户本地配置，但绝不能将 API Key、密码等隐私写入会被提交到仓库的文件。
- **不要做 Git 暂存/提交/推送**等修改操作，用户自行管理版本控制。
- **禁止遗留后台进程**：启动进程前先检查残留，禁止 `&` 或 `nohup` 后直接结束对话，对话结束前清理所有启动的进程。
- **WEB 开发**：优先使用 pnpm；无需启动 Dev Server 并抓取页面验证，改代码后由用户自行启动。
- **代码提交信息**：产出代码后给出简短中文 commit 信息，一行/一句话，符合 commitlint 规范；不代替用户提交。
- **Go 工具链**：始终设置 `GOTOOLCHAIN=local`，不使用 Go 自动下载 toolchain。
- **主动沟通**：发现阻塞性问题时，积极说明情况并给出选择，不要把问题默默记入 backlog 等用户发现。
- **协作规则双文件同步**：`CLAUDE.md` 与 `AGENTS.md` 是 Claude 和 Codex 工作流的共同规则入口，内容必须始终保持一致。任一文件发生修改时，必须在同一轮同步修改另一文件。
- **专题中枢文档**：同一需求及其衍生需求跨越多个提交、多次修改、多个文档记录时，应编写 `docs/design/YYYY-MM-DD-<序号>-<topic>-hub.md`，集中串联关联产物、时间线、完成度、下一轮建议。关联文档应在顶部反向链接到中枢文档。

---

## 工作模式

AI 根据触发词自动切换工作模式。触发后，读取 `docs/handbook/work-modes.md` 中对应模式的完整流程执行。

- **评估模式** — 触发：`评估`、`评估一下`、`打分`、`检查质量`
  - 读：相关 design、评估基线和目标代码
  - 产出：评估报告（维度评分 + 得分点/失分点）+ backlog 新条目（仅描述问题，不写方案）
- **规划模式** — 触发：`规划一下`、`帮我设计`、`出个方案`、`怎么做`、`我发现问题`、`这里不够好`、`效果不对`、`有个 bug`
  - 读：backlog、prd、tech、相关 design 和 handbook；涉及配置问题时读 `.local/` 实际配置
  - 产出：backlog 条目方案；中大型任务补充 design 文档和明确任务列表
- **生成模式** — 触发：`按方案执行`、`开始开发`、`实现这个`、`完成开发`
  - 读：已规划 backlog 条目、关联 design、tech 和 coding-conventions
  - 产出：代码变更、测试结果、backlog 和受影响文档更新
- **项目管理** — 触发：`版本归档`、`归档`、`里程碑`、`版本规划`、`迭代计划`
  - 读：backlog、archive/*
  - 产出：archive/*、清理后的 backlog、入口和决策历史更新
- **全流程模式** — 触发：`完整走一遍`、`全流程`、`一站式`、`从头到尾`、`整个工作流`、`规划生成评估全流程`
  - 串联规划→生成→评估，适合小体量单会话闭环
  - 读：backlog、目标代码和相关规范
  - 产出：代码变更 + backlog 条目更新 + 轻量自检

**模式间交接**：通过 backlog 条目结构化字段（状态 / 背景 / 方案 / 分析 / 验收）传递上下文。用户使用 `/clear` 清空上下文后，AI 读取 backlog 和相关文档即可继续工作。

**详细流程、路由规则、handoff 和设计原则**：见 [`docs/handbook/work-modes.md`](docs/handbook/work-modes.md)。

---

## 文档层级

冲突时，优先以高层文档为准；修改时间更新者优先。

- **L1** `AGENTS.md` / `CLAUDE.md`：全局协作规则、文档索引、工作模式触发。禁区：产品需求、技术细节、任务进度。
- **L1.5** `docs/handbook/work-modes.md`：工作模式完整流程、路由规则、handoff 协议。
- **L1.5** `docs/handbook/eval-guide.md`：评估模式操作指南。
- **L1.5** `docs/handbook/coding-conventions.md`：Go、Web、Markdown 编码规范。
- **L1.5** `docs/handbook/doc-review.md`：文档审阅规范。
- **L2** `README.md` / `README.en.md`：项目简介、核心价值、快速开始、文档索引。禁区：详细技术方案、任务拆解。
- **L3** `docs/prd.md`：产品做什么、用户故事和验收标准。禁区：API 设计、数据模型、部署命令。
- **L4** `docs/tech.md`：架构、API 契约、数据模型和技术选型。禁区：任务拆分、估时。
- **L4** `docs/harness.md`：Harness 高层架构索引。禁区：实现细节。
- **L5** `docs/design/*-hub.md`：专题中枢，串联同一需求的设计、评估、backlog 和提交记录。
- **L6** `docs/reference.md`：设计参考理论和背景知识。
- **L6** `docs/eval/baseline.md`：量化评估基线。
- **L7** `docs/backlog.md`：需求池、版本范围和 Plan → Generate 交接。归档后只保留未完成条目。
- **L8** `docs/archive/`：已完成版本、周期记录、一次性审查和历史产物。
- **L9** 代码实现：最终事实来源。

---

## 编译与测试

```bash
make              # 构建前端 + 后端，输出到 bin/pflow
make env          # 安装全部依赖（Go + Node.js）
make vet          # Go 静态分析
make test         # Go 测试
make clean        # 清理构建产物
make dev          # 启动 API server
make run          # 构建并启动完整服务
```

- 前端代码在 `web/`，后端代码在 `cmd/` + `internal/`。
- `//go:embed` 在项目根 `embed.go`；`make build` 必须先生成 `web/dist/` 再构建 Go。
- 编译产物只输出到 `bin/`，禁止直接 `go build`，避免嵌入过时前端资源。
- Go 修改后运行 `gofmt`。
- JSON 配置使用临时文件 + rename 原子写入；不执行破坏性清理。

---

## 文档索引

- [`docs/prd.md`](docs/prd.md) — 产品需求、用户故事、验收标准
- [`docs/tech.md`](docs/tech.md) — 架构、API 契约、数据模型
- [`docs/backlog.md`](docs/backlog.md) — 需求池、版本范围、交接协议
- [`docs/harness.md`](docs/harness.md) — Harness 工程架构索引
- [`docs/handbook/work-modes.md`](docs/handbook/work-modes.md) — 工作模式和 handoff
- [`docs/handbook/eval-guide.md`](docs/handbook/eval-guide.md) — 评估操作指南
- [`docs/handbook/coding-conventions.md`](docs/handbook/coding-conventions.md) — 编码与验证规范
- [`docs/handbook/doc-review.md`](docs/handbook/doc-review.md) — 文档审阅规范
- [`docs/eval/baseline.md`](docs/eval/baseline.md) — 评估基线
- [`docs/design/`](docs/design/) — 功能设计文档
- [`docs/archive/`](docs/archive/) — 已完成版本的归档
