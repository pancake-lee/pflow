# 阶段四启动：提醒分数算法 + 注意力遮罩层 MVP

> 周期：2026-06-15 | 状态：📋 规划中

## 周期切换

阶段三（项目策略管理）已全部完成。从"静态策略面板"进化为"动态注意力引导系统"。

## 新文档融合

| 文档 | 定位 | 融入位置 |
|------|------|----------|
| `docs/design/02-reminder_score_algorithm.md` | 提醒分数算法设计 | prd.md §4.1, tech.md §2.4/§3.4, backlog.md P1 |
| `docs/design/03-attention_mask.md` | 注意力遮罩层技术方案 | prd.md §4.2, tech.md §3.4, backlog.md P1 |
| `docs/design/99-dual_layer_skinning_system.md` | 双层换肤系统（远期预留） | prd.md §5, backlog.md P2（低权重） |
| `docs/design/99-ai-chat-web-attach.md` | Web AI 平台监控（远期预留） | prd.md §5, backlog.md P2（低权重） |

## MVP 策略

- **后端**：`internal/attention/` 包，session 状态作为用户活跃代理指标（不监听真实鼠标/键盘）
- **前端**：`::before` 伪元素半透明遮罩，动态 opacity 映射提醒分数
- **不包含**：桌面通知、声音、弹窗、换肤系统完整实现

## 关键设计决策

- **Activity tracking 使用 session 状态（busy/running）作为用户活跃的代理指标**，而非监听鼠标/键盘——MVP 简化。后续可升级为真实操作监听。
- **遮罩层使用 `::before` + CSS 变量预留皮肤接口**——不阻塞后续双层换肤系统（[`99-dual_layer_skinning_system.md`](../design/99-dual_layer_skinning_system.md)），但 MVP 只做半透明黑。
- **提醒分数在每次 Dashboard API 请求时重新计算**——无状态设计，无需持久化。后续引入 WebSocket 推送时再考虑增量计算。

## 开发计划

详见 [`todo.md`](../../todo.md)。
