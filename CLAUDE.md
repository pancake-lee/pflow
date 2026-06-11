# CLAUDE.md — pflow AI 工作规则

## 项目概况

pflow 是一个 Go 项目，目标是在 cc-connect 的"与 Claude Code CLI 通信"能力之上，构建多 Agent 会话的注意力管理与调度工具。详见 [`README.md`](./README.md)。

## 编译与测试

```bash
go build ./...
go test ./...
go vet ./...
```

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
- `docs/note.md` 记录活跃技术备忘（调试技巧、临时方案等）
- `docs/backlog.md` 维护完整需求池
- 代码是实现的唯一真相源，文档只描述架构层面的设计

### 工作流程

1. 每次对话开始时读 [`todo.md`](./todo.md) 了解当前任务
2. 完成任务后更新 `todo.md` 的进度
3. 遇到需要记录的技术细节写到 [`docs/note.md`](./docs/note.md)
4. 发现新的需求/想法补充到 [`docs/backlog.md`](./docs/backlog.md)
5. 周期结束后按规范归档
