# Backlog

> 当前未完成的技术需求池，按序号排列。
> 状态标记：Pending、WIP、Approved、Abandoned、Rejected。
> 已完成的历史条目已迁移至版本归档，不在当前 backlog 重复维护。

本版本已完成的 31 个条目（`#0–#18`、`#22`、`#27–#37`）已归档至 [`archive/cycles/09-v0.0.8-dynamic-attention-guidance.md`](archive/cycles/09-v0.0.8-dynamic-attention-guidance.md)。

## 任务总览

| 状态 | 序号 | 类别 | 任务 | 简述 |
| ---- | ---- | ---- | ---- | ---- |
| Pending | 19 | 通知系统 | 桌面通知 | 分数超阈值时触发浏览器 Notification API |
| Pending | 20 | 智能分析 | 军情哨主动推送 | 后台守护进程主动推送提醒（需守护进程支持） |
| Pending | 21 | 智能分析 | 统帅偏好学习 | 推送频率自适应，根据用户行为学习偏好 |
| Pending | 23 | 扩展能力 | 双层换肤系统 | 内容层 + 遮罩层独立换肤，支持社区皮肤 |
| Pending | 24 | 扩展能力 | Web AI 平台状态监控 | 浏览器扩展监控 DeepSeek/Kimi/ChatGPT 等平台的 AI 对话状态 |
| Pending | 25 | 扩展能力 | 跨设备同步 | 手机/平板看状态、点批准 |
| Pending | 26 | Agent 管理 | 多 Agent 类型支持 | 支持 Cline、Codex CLI 等其他 Agent 类型（远期预留） |

## 待规划条目

### #19 桌面通知

- **状态**：Pending
- **目标**：当项目提醒分数超过阈值时，通过浏览器 Notification API 通知用户。
- **验收**：通知权限、阈值触发、重复通知抑制和权限拒绝均有明确行为。

### #20 军情哨主动推送

- **状态**：Pending
- **目标**：由后台守护进程主动推送军情哨提醒，而不是等待 Dashboard 请求触发。
- **依赖**：需要后台守护进程和可靠的推送通道。
- **验收**：服务在无页面主动刷新时仍能按规则推送，并能查看推送原因和时间。

### #21 统帅偏好学习

- **状态**：Pending
- **目标**：根据用户对提醒的响应和忽略行为，自适应调整提醒频率。
- **验收**：偏好数据可解释、可重置，学习结果不会突破用户设定的通知边界。

### #23 双层换肤系统

- **状态**：Pending
- **目标**：内容层与注意力遮罩层独立换肤，支持可扩展的社区皮肤。
- **设计依据**：[`design/99-dual_layer_skinning_system.md`](design/99-dual_layer_skinning_system.md)。
- **验收**：皮肤可独立配置内容层和遮罩层，切换后不破坏提醒状态和可读性。

### #24 Web AI 平台状态监控

- **状态**：Pending
- **目标**：通过浏览器扩展监控 DeepSeek、Kimi、ChatGPT 等 Web AI 平台的对话状态。
- **设计依据**：[`design/99-ai-chat-web-attach.md`](design/99-ai-chat-web-attach.md)。
- **验收**：扩展按最小权限读取状态，Dashboard 能区分平台、会话和状态变化。

### #25 跨设备同步

- **状态**：Pending
- **目标**：支持在手机或平板查看会话状态并执行批准操作。
- **验收**：跨设备身份、状态同步、授权安全和离线/断线行为有明确方案。

### #26 更多 Agent 类型支持

- **状态**：Pending
- **目标**：在 Claude Code 和 Hermes 之外支持 Cline、Codex CLI 等 Agent。
- **验收**：Agent 适配边界、会话标识、状态采集和 Dashboard 展示方式统一。

## 规划约束

- 新需求先进入本文件，再由规划模式补充背景、方案、任务拆分和验收。
- 方案、子任务和实施步骤统一写入对应 backlog 条目，不创建独立任务计划文档。
- 完成后的条目在版本归档时迁移到 `docs/archive/`，并从当前 backlog 的总览和详情中删除。
