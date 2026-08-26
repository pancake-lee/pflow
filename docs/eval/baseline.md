# pflow 评估基线

> 基线只记录已经实际执行的结果；当前尚未建立完整的运行时效果基线。

## 工程基线

| 检查 | 命令 | 当前状态 |
| --- | --- | --- |
| Go 静态分析 | `GOTOOLCHAIN=local make vet` | 以最近一次实际运行结果为准 |
| Go 测试 | `GOTOOLCHAIN=local make test` | 以最近一次实际运行结果为准 |
| Web + Go 构建 | `GOTOOLCHAIN=local make build` | 以最近一次实际运行结果为准 |
| Web 类型检查 | `cd web && npx vue-tsc --noEmit` | 以最近一次实际运行结果为准 |

## 功能基线

- Session 映射：覆盖 Claude、Hermes、tmux 生命周期和持久化重启。
- 项目策略：覆盖最长前缀归类、主线/支线 slot、迁移和空输入。
- 注意力：覆盖提醒分数、专注保护期、时间估算降级链和建议场景。
- Dashboard：覆盖空状态、状态刷新、项目分组和终端入口。

新增可靠的测试或手动测量后，在本文件追加日期、提交、命令、结果和环境，不修改历史结果。

