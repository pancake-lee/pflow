# todo

> 当前周期：阶段四 提醒分数算法 + 注意力遮罩层 MVP — **已完成** ✅
>
> 下一周期：P1 交互增强（通知/动画/折叠）或 P2 锦上添花项

## 目标

实现 PRD 阶段四（智能调度）的核心基础设施：

1. ✅ **提醒分数算法**：根据等待时长、专注持续、今日累计、项目优先级，为每个项目计算提醒分数（详见 [`docs/design/02-reminder_score_algorithm.md`](./docs/design/02-reminder_score_algorithm.md)）
2. ✅ **注意力遮罩层**：双维度设计——雾化遮罩（Fog，透明度随提醒分数变化）+ 高亮跑马灯（Marquee，边框动画随提醒分数变化）。专注模式统一遮罩覆盖标题/筛选/分组/非专注卡片，颜色与页面背景一致。

MVP 范围：后端算法 + 前端遮罩。桌面通知、卡片动画、声音提示等高级提醒形式留待后续。

## P0 — Activity Tracker（数据追踪基础设施）

### P0-1 `internal/attention/` 包骨架

- [x] 创建 `internal/attention/` 目录
- [x] `config.go`：常量定义（CurWindow=60, ProtectMin=5（MVP 降为 5 方便验证）, WWait=1.0, WStreak=0.5, PrimaryBonus=2.0, ExpPower=2.0）
- [x] `types.go`：`ReminderInput` / `ReminderOutput` 结构体，双维度输出（highlight + fog_pct）
- [x] `score.go`：`CalculateScores()` — 核心算法实现
- [x] `focus.go`：`FocusState` — 专注模式状态管理（全局单例，支持 extend/stop/snapshot）

### P0-2 Activity 追踪逻辑

- [x] 以 session 状态作为用户活跃的代理指标：项目下有 busy/running session → 用户正在该项目
- [x] `streak` 计算：从 busy session 的连续活跃估算
- [x] `total` 计算：当日累计活跃时长
- [x] `lastActiveTime` 记录
- [x] `is_current` 判定：最近活跃且 streak > 0 者为当前活跃项目
- [x] 每次 Dashboard API 请求时重新计算（无实时事件）

### P0-3 Waiting 时长提取

- [x] 从 session 数据中获取 waiting 状态的持续时间
- [x] 每个项目取等待 session timestamps 计算 waiting 时长

### P0-4 配置支持

- [x] 默认常量硬编码在 `config.go`
- [ ] 可选：从 `~/.pflow/config.json` 读取（TODO 后续）

## P1 — Score Calculator（提醒分数计算引擎）

### P1-1 核心算法实现

- [x] 实现 `CalculateScores(projects, sessions, activities, focusState) -> map[projectPath]ReminderOutput`
- [x] 按设计文档逐步实现：
  - [x] 确定当前活跃任务 `cur`
  - [x] 基础等待分 `base_i = waiting_i * WWait`
  - [x] 专注干扰因子：聚焦时 `streak_cur < focusMinutes` → factor_i=0；否则 `factor_i = min((streak_cur / ProtectMin) * WStreak, 2.0)`
  - [x] 支线活跃 + 目标是主线 → `factor_i *= PrimaryBonus`
  - [x] 无活跃任务 → 仅主线 adjusted = base，其余为 0
  - [x] 今日累计矫正
  - [x] 幂函数差异化 `final = raw ^ ExpPower`
  - [x] 映射提醒等级（none/low/medium/high）
- [x] **超出设计**：双维度输出——highlight（高亮分数 0-100）+ fog_pct（雾化分数 0-100），替代原单一提醒分级

### P1-2 单元测试

- [x] 7 个测试用例覆盖核心场景（`score_test.go`）

### P1-3 Dashboard API 集成

- [x] `GET /api/v1/dashboard` 响应增加 `reminder_scores: map[string]ReminderOutput`
- [x] `focus?: FocusSnapshot` 字段（专注模式状态）
- [x] `POST /api/v1/focus/extend` — 激活/延长专注
- [x] `POST /api/v1/focus/stop` — 退出专注

## P2 — Attention Mask（前端注意力遮罩层）

### P2-1 TypeScript 类型 + Composable

- [x] `web/src/types/dashboard.ts`：增加 `ReminderScoreInfo`、`FocusState` 接口
- [x] `web/src/composables/useReminderScores.ts`：双维度映射函数 `highlightToMarquee()` / `fogPctToOpacity()`，可调参数读取自 config
- [x] `web/src/config/attention.ts`：集中配置 MARQUEE / FOG / FOCUS 三段参数

### P2-2 遮罩层 CSS 基础

- [x] PrimaryCard / SecondaryCard 设置 `position: relative` + `overflow: hidden`
- [x] `::before` 伪元素：雾化遮罩（fog），`background: var(--n-color-target)` + `opacity: var(--fog-opacity)`
- [x] `::after` 伪元素：高亮跑马灯（marquee），conic-gradient + mask 实现边框动画
- [x] CSS 变量接口：`--fog-opacity`, `--fog-image`, `--hl-speed`, `--hl-width`, `--hl-opacity`

### P2-3 双维度注意力视觉

- [x] **替代原三级提醒方案**：采用双维度设计——雾化程度 + 高亮跑马灯
- [x] Highlight 0-100 → marquee speed/width/opacity 线性映射
- [x] Fog 0-100 → fog opacity 线性映射
- [x] Debug 开关：`debugHighlight` / `debugFogPct` 可强制覆盖分数

### P2-4 交互细节

- [x] 卡片 `:hover` 时降低雾化透明度（`opacity * 0.3`）
- [x] 遮罩层点击穿透（`pointer-events: none`）
- [x] CSS `transition` 平滑过渡

### P2-5 专注模式（超出原设计范围）

- [x] 专注模式焦点项目识别：PrimaryCard / SecondaryCard 内 `isFocusedProject` 判断
- [x] 非专注区域统一遮罩：header-stats / filter-bar / zone-collapse（普通+未归类分组）
- [x] 非专注卡片叠加 `focus-overlay`（PrimaryCard / SecondaryCard 未聚焦时）
- [x] 遮罩颜色与页面背景一致：`background: var(--n-color-target)` + 统一 `dimOpacity`
- [x] 配置集中在 `FOCUS.dimOpacity`（`attention.ts`）

## P3 — 联调与收尾（本周期完成项）

### P3-1 端到端验证

- [x] 模拟场景验证 API 返回 correct reminder_score
- [x] 前端验证遮罩层随分数正确变化
- [x] 专注模式视觉确认

### P3-2 边界情况

- [x] 无项目根标记时正常运行
- [x] 所有 session 同一状态
- [x] API 错误时前端优雅降级

### P3-3 文档收尾

- [x] 更新 todo.md 标记周期完成
- [x] 将完成的任务回写到 `docs/backlog.md`

## 不包含（本周期）

- 桌面通知（Notification API）/ 声音提示 / 居中弹窗 — 留待后续
- 双层换肤系统完整实现（仅预留 CSS 变量接口）
- Web AI 平台状态监控（浏览器扩展）
- WebSocket 实时推送（继续使用轮询刷新）
- 统帅偏好学习
- 用户操作监听（鼠标/键盘事件）— MVP 使用 session 状态作为活跃代理

## 设计文档

- [`docs/design/02-reminder_score_algorithm.md`](./docs/design/02-reminder_score_algorithm.md) — 算法设计
- [`docs/design/03-attention_mask.md`](./docs/design/03-attention_mask.md) — 遮罩层技术方案
- [`docs/design/04-test-cases.md`](./docs/design/04-test-cases.md) — 测试用例
- [`docs/design/99-dual_layer_skinning_system.md`](./docs/design/99-dual_layer_skinning_system.md) — 远期换肤系统
- [`docs/design/99-ai-chat-web-attach.md`](./docs/design/99-ai-chat-web-attach.md) — 远期浏览器扩展
- [`docs/reference.md`](./docs/reference.md) — 设计参考理论知识
