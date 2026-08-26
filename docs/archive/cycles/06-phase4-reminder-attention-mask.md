# 阶段四完成：提醒分数算法 + 注意力遮罩层 MVP

> 周期：2026-06-15 ~ 2026-06-16 | 状态：✅ 已完成

## 交付成果

### 后端 `internal/attention/` 包

- `config.go` — 算法常量（CurWindow=60, ProtectMin=5, WWait=1.0, WStreak=0.5, PrimaryBonus=2.0, ExpPower=2.0）
- `types.go` — `ReminderInput` / `ReminderOutput` 双维度输出（highlight 0-100 + fog_pct 0-100）
- `score.go` — `CalculateScores()` 核心算法：当前活跃判定 → 等待基础分 → 专注干扰因子 → 今日累计矫正 → 幂函数差异化 → 雾化分计算
- `focus.go` — `FocusState` 专注模式状态管理（全局单例，extend/stop/snapshot）
- `score_test.go` — 7 个测试用例覆盖核心场景
- API：`reminder_scores` 字段集成到 `/api/v1/dashboard`；`POST /api/v1/focus/extend` / `POST /api/v1/focus/stop`

### 前端注意力可视层

- `web/src/types/dashboard.ts` — `ReminderScoreInfo`、`FocusState` TypeScript 类型
- `web/src/config/attention.ts` — 集中可调参数（MARQUEE 动画 / FOG 雾化 / FOCUS 专注模式 dimOpacity）
- `web/src/composables/useReminderScores.ts` — 双维度映射函数
- PrimaryCard / SecondaryCard：`::before` 雾化遮罩 + `::after` 高亮跑马灯动画（conic-gradient + mask）
- 专注模式：非聚焦区域统一遮罩，颜色与页面背景一致

### 设计文档

- [`docs/design/02-reminder_score_algorithm.md`](../../design/02-reminder_score_algorithm.md) — 算法设计
- [`docs/design/03-attention_mask.md`](../../design/03-attention_mask.md) — 遮罩层技术方案
- [`docs/design/04-test-cases.md`](../../design/04-test-cases.md) — 测试用例

## 关键决策回顾

- **Activity tracking 使用 session 状态（busy/running）作为用户活跃的代理指标**——MVP 简化
- **遮罩层使用 `::before`/`::after` + CSS 变量预留皮肤接口**——不阻塞后续换肤系统
- **提醒分数在每次 Dashboard API 请求时重新计算**——无状态设计
- **双维度输出替代原三级提醒分级**——高亮跑马灯 + 雾化遮罩，独立可调

## 未包含（留待后续）

- 桌面通知（Notification API）/ 声音提示 / 居中弹窗
- 用户操作监听（鼠标/键盘事件）——当前使用 session 状态代理
- WebSocket 实时推送
- 双层换肤系统完整实现
