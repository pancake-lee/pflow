# #42 tmux 焦点项目计时，第一轮评估

## 结论

- **总分：6.8 / 10，不通过**
- 已验证：`GOTOOLCHAIN=local make vet`、`GOTOOLCHAIN=local make test`、前端构建、运行中的 tmux `focus-events on`、Dashboard API 本机响应。

## 得分点

- `focus-events` 缺失时只追加一条配置，已有 on/off 和其他用户配置不被改写。
- 使用 `client-focus-in/out`，且无日志时现有消息估算链路仍可工作。
- 聚合已覆盖短暂同项目失焦合并、长间隔断开、未闭合段和异常 out 事件。

## 失分点

- `tmux set-hook -g` 会替换同名的用户全局 hook，违背“不覆盖用户其他配置”的承诺；关闭服务时还会移除该 hook，无法辨别自己的与用户的 hook。
- 聚合测试未构造跨项目焦点切换，不能证明 A 切至 B 时 A 一定在失焦刻截断。
- 缺少 tmux 不支持新版 hook 的可诊断判定测试，当前只依赖命令失败后的泛化错误。

## 待规划问题

- 为 pflow hook 使用 tmux hook 数组的独立索引或保留/恢复策略，确保注册和清理都不会修改用户 hook。
- 补充跨项目切换、同项目恢复、超时恢复和不支持版本的端到端边界测试。
