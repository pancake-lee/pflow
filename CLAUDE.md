# CLAUDE.md — pflow AI 工作规则

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

- `README.md` 面向人，写项目是什么、怎么用
- `CLAUDE.md` 面向 AI，写怎么开发、规则和禁忌
- `docs/prd.md` 面向产品功能，不写技术实现
- `docs/tech.md` 面向技术架构，不写代码级细节
- `docs/backlog.md` 维护完整需求池
- `docs/note.md` 参考笔记——不随上下文加载，偶尔需要翻阅的规则/知识/历史决策。如设计理念原因、已知坑点、关键数据路径速查
- `docs/cycles/` 按功能/周期归档的开发日志——note.md 中过长的实现记录应迁移到此
- `docs/design/` 重要功能的设计文档，按编号命名（`02-*` 当前开发、`99-*` 远期预留）
- 代码是实现的唯一真相源，文档只描述架构层面的设计

### 工作流程

1. 每次对话开始时读 [`todo.md`](./todo.md) 了解当前任务
2. 完成任务后更新 `todo.md` 的进度
3. 开发过程中值得记录的技术细节写入 `docs/cycles/` 对应的周期文件（按功能/日期命名）
4. 发现新的需求/想法补充到 [`docs/backlog.md`](./docs/backlog.md)
5. 需要长期参考的规则/知识/决策更新到 [`docs/note.md`](./docs/note.md)
6. 周期结束后将 todo.md 完成项回写到 backlog.md，清理 todo.md 开始新周期
