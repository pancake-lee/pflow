# pflow 评估指南

## 评估原则

评估器只判断产出是否满足承诺，记录证据和问题现象，不提出修复方案。评估前先读取对应 backlog、PRD、design、编码规范和基线。

## 评分维度

### 代码质量

- 正确性：实现是否符合设计和边界行为。
- 健壮性：错误、空值、进程退出、文件损坏和并发路径是否可控。
- 可维护性：模块边界、命名、重复、注释和规范一致性。
- 简洁性：是否存在死代码、无必要抽象或重复实现。

### 功能效果

- 准确性：session 状态、项目归类、提醒和建议是否正确。
- 完整性：核心场景、Agent 类型、异常路径和空状态是否覆盖。
- 一致性：CLI、API、Dashboard 和持久化状态是否一致。

### 用户价值

- 可用性：用户能否据此判断下一步行动。
- 交互体验：加载、等待、错误反馈和 Dashboard 扫读是否顺畅。
- 惊喜度与 AI 增量：仅对确实面向用户的智能建议启用，不强行给基础设施打分。

## 分数锚点

- 9–10：完整可靠，边界处理充分。
- 7–8：核心可用，仅有局部改进点。
- 5–6：可运行但有明显缺口或技术债。
- 3–4：重要场景失败，需要返工。
- 1–2：核心能力不可交付或存在严重风险。

## 验证工具箱

```bash
GOTOOLCHAIN=local make vet
GOTOOLCHAIN=local make test
GOTOOLCHAIN=local make build
cd web && npx vue-tsc --noEmit
```

按功能补充：`go test ./internal/<module>/...`、`go test -race`、API `httptest`、CLI 手动命令、tmux/真实 Agent 验收。报告应记录实际执行的命令和结果，不把未执行的检查写成通过。

## 报告格式

报告放在 `docs/eval/reports/YYYY-MM-DD-<topic>.md`，至少包含：目标、环境、验收项 Pass/Fail、分维度得分、证据、未通过问题和 backlog 链接。若要程序化追踪，增加同名 JSON，仅存结构化字段。

