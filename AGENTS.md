# AGENTS.md — pflow AI 工作规则

> 本文件与 [`CLAUDE.md`](./CLAUDE.md) 内容必须同步。完整 Harness 入口见 [`docs/harness.md`](./docs/harness.md)。

## 项目概况

pflow 是一个 Go 项目，目标是在 cc-connect 的“与 Claude Code CLI 通信”能力之上，构建多 Agent 会话的注意力管理与调度工具。详见 [`README.md`](./README.md)。

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

## 必须遵守

- 本项目是独立 MIT 开源项目，不是 cc-connect 的 Fork。
- 引用 cc-connect 代码时，在文件头标注来源、版权和修改说明。
- 不修改或提交 `.local/` 个人工作文档。
- Go 修改后运行 `gofmt`；优先设置 `GOTOOLCHAIN=local`。
- JSON 配置使用临时文件 + rename 原子写入；不执行破坏性清理。

## 文档与 Harness

- `README.md` 面向用户；`docs/prd.md` 写产品承诺；`docs/tech.md` 写架构；`docs/design/` 写当前方案；`docs/backlog.md` 是规划与实现的唯一交接点。
- 当前规则、编码规范、评估和文档治理见 `docs/handbook/`；历史材料见 `docs/archive/`。
- 每次对话开始读 [`todo.md`](./todo.md)，完成任务后同步更新进度。
- `评估` 只记录证据和问题现象；`规划` 把方案写入 backlog；`实现` 按 backlog 执行并运行验证。
- 修改 `AGENTS.md` 时必须同步修改 `CLAUDE.md`，两者内容保持一致。

