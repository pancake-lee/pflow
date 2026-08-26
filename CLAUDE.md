# CLAUDE.md — pflow AI 工作规则

> 本文件与 [`AGENTS.md`](./AGENTS.md) 内容必须同步。完整 Harness 入口见 [`docs/harness.md`](./docs/harness.md)。

## 项目概况

pflow 是一个 Go 项目，目标是在 cc-connect 的"与 Claude Code CLI 通信"能力之上，构建多 Agent 会话的注意力管理与调度工具。详见 [`README.md`](./README.md)。

## 编译与测试

```bash
make              # 构建前端 + 后端，输出到 bin/pflow
make env          # 安装全部依赖（Go + Node.js）
make vet          # Go 静态分析
make test         # Go 测试
make clean        # 清理构建产物
make dev          # 启动 API server（配合 cd web && npm run dev 前端开发）
make run          # 构建并启动完整服务
```

- 前端代码在 `web/` 目录，后端代码在 `cmd/` + `internal/`
- `//go:embed` 指令在项目根 `embed.go`，由 `cmd/pflow/main.go` 引用
- `make build` 会先 `npm run build` 生成 `web/dist/`，再 `go build` 嵌入二进制
- 编译产物只输出到 `bin/` 目录，不污染项目根
- **禁止直接使用 `go build` 编译**，必须通过 `make build`，因为前端构建是编译的前置步骤，直接 `go build` 会嵌入过时的前端资源

## 工作规则

### 必须遵守

- 本项目是独立 MIT 开源项目，不是 cc-connect 的 Fork
- 引用 cc-connect 代码时，必须在文件头部标注来源、版权和修改说明
- 不要修改 `.local/` 目录下的个人工作文档
- 不要提交 `.local/` 目录（已在 `.gitignore` 中）

### 代码风格

- 遵循 Go 标准代码风格
- 包名使用小写单词，避免下划线
- 导出函数和类型需要注释

### 文档原则

- `README.md` 面向用户；`CLAUDE.md` / `AGENTS.md` 面向 AI。
- `docs/prd.md` 写产品承诺，`docs/tech.md` 写架构，`docs/design/` 写当前功能方案，`docs/backlog.md` 写可执行方案与验收。
- 当前规则和项目速查见 `docs/handbook/`；历史周期、审查、版本记录见 `docs/archive/`。
- 代码是实现的唯一真相源；不要在文档中复制已经落地的函数、字段和 API 细节。

### Harness 工作流程

完整触发词和交接协议见 [`docs/handbook/work-modes.md`](./docs/handbook/work-modes.md)。核心约束如下：

1. 每次对话开始读 [`todo.md`](./todo.md)；按范围补读 backlog、PRD、tech、design 和 handbook。
2. `评估` 只收集证据、评分和问题现象，不修改代码、不提出修复方案。
3. `规划` 将背景、技术方案、任务拆分和验收写进 `docs/backlog.md`；中大型任务再建 design 文档。
4. `实现` 按已规划条目改代码，并运行相关测试；完成后同步 backlog、todo 和当前文档。
5. 新需求进入 backlog；已完成的周期性材料进入 `docs/archive/`，当前有效结论迁移到 handbook、tech 或 design。
6. 修改本文件时必须同步修改 `AGENTS.md`。
