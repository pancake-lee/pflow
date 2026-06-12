# testing

> 测试策略、测试分层、自动化方案。

## 策略

- **单元测试**：每个 `internal/` 包的纯逻辑需要覆盖（状态推断、事件解析、配置解析）
- **集成测试**：与真实 Claude Code CLI 的交互验证（需 `claude` 命令可用）
- **手动验证**：核心指标通过人工标记 vs 系统判断来评估

## 当前状态

### 已覆盖

- Go vet 静态分析（`make vet`）
- TypeScript 类型检查（`cd web && npx vue-tsc --noEmit`）
- `make build` 构建验证（前端 + 后端一体化）

### 待添加

- `go test` 单元测试
- 前端组件测试
- 阶段一验证清单（以下为历史记录）

## 阶段一验证清单（历史）

- [x] 启动 Claude Code 子进程，确认 `stream-json` 输出可解析
- [x] 发送 prompt，确认 text/tool_use/result 事件类型均可收到
- [x] 触发需要授权的工具调用（如执行命令），确认 permission_request 事件可检测
- [ ] 人工标记 10 个时刻的 Agent 真实状态，与系统推断对比，计算准确率
- [ ] 同时运行 3 个会话，各自执行不同任务，确认状态不串扰
- [ ] Dashboard API 响应时间 < 100ms
