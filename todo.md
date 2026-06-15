# todo

> 当前周期：阶段四 提醒分数算法 + 注意力遮罩层 MVP

## 目标

实现 PRD 阶段四（智能调度）的核心基础设施：

1. **提醒分数算法**：根据用户专注状态、等待时长、今日累计时间、项目优先级，为每个项目计算提醒分数（详见 [`docs/design/02-reminder_score_algorithm.md`](./docs/design/02-reminder_score_algorithm.md)）
2. **注意力遮罩层**：在 Dashboard 项目卡片上叠加 `::before` 半透明遮罩，透明度随提醒分数动态变化（详见 [`docs/design/03-attention_mask.md`](./docs/design/03-attention_mask.md)）

MVP 范围：后端算法 + 前端遮罩。桌面通知、卡片动画、声音提示等高级提醒形式留待后续。

## P0 — Activity Tracker（数据追踪基础设施）

### P0-1 `internal/attention/` 包骨架

- [ ] 创建 `internal/attention/` 目录
- [ ] `config.go`：常量定义（PROTECT_MIN=15, W_WAIT=1.0, W_STREAK=0.5, PRIMARY_BONUS=2.0, W_CORRECT=0.5, EXP_POWER=2.0, REMINDER_THRESHOLDS=[2,5,10]）
- [ ] `activity.go`：`ProjectActivity` 结构体（streak 连续活跃分钟 / total 今日累计分钟 / lastActiveTime 最近活动时间戳）
- [ ] `score.go`：`ReminderInput` 和 `ReminderOutput` 结构体

### P0-2 Activity 追踪逻辑

- [ ] 以 session 状态作为用户活跃的代理指标：项目下有 busy/running session → 用户正在该项目的"前线"
- [ ] `streak` 计算：最近活动距当前 < 5min 则继续累加，否则重置为 0
- [ ] `total` 计算：当日 streak 增量的累加（每日 0 点重置）
- [ ] `lastActiveTime` 记录：每次活跃时更新时间戳
- [ ] `is_current` 判定：`streak > 0` 且 `lastActiveTime` 最大者为当前活跃项目
- [ ] 由于 session 状态来自文件扫描（无实时事件），MVP 阶段在每次 Dashboard API 请求时重新计算

### P0-3 Waiting 时长提取

- [ ] 从现有 session 数据中获取 waiting 状态的持续时间
- [ ] 每个项目取最长 waiting session 的时间作为 `waiting_i`
- [ ] 无 waiting session 的项目 `waiting_i = 0`

### P0-4 配置支持

- [ ] 默认常量硬编码在 `config.go`
- [ ] 可选：从 `~/.pflow/config.json` 的 `attention` 段读取用户自定义参数（与设计文档第 5 节对齐）

## P1 — Score Calculator（提醒分数计算引擎）

### P1-1 核心算法实现

- [ ] 实现 `CalculateScores(projects, sessions, activities) -> map[projectPath]ScoreResult`
- [ ] 按设计文档第 3 节逐步实现：
  - [ ] 3.1 确定当前活跃任务 `cur`
  - [ ] 3.2 基础等待分 `base_i = waiting_i * W_WAIT`
  - [ ] 3.3 专注干扰因子（cur 非空时）
    - `streak_cur < PROTECT_MIN` → `factor_i = 0`
    - 否则 `factor_i = min((streak_cur / PROTECT_MIN) * W_STREAK, 2.0)`
    - 支线活跃 + 目标是主线 → `factor_i *= PRIMARY_BONUS`
  - [ ] 3.4 当前活跃调整
    - `cur == null` → 仅主线 `adjusted_i = base_i`，其余为 0
  - [ ] 3.5 今日累计矫正（主线 total < 支线平均 → 增加修正分）
  - [ ] 3.6 幂函数差异化 `final = raw ^ EXP_POWER`（拉大差距）
  - [ ] 3.7 映射提醒等级（无/低/中/高）

### P1-2 单元测试

- [ ] 测试用例 1：主线专注 25min + 支线等待（复现设计文档第 6 节场景 1）
- [ ] 测试用例 2：无活跃任务 + 主线等待（复现场景 2）
- [ ] 测试用例 3：保护期内不产生提醒（streak_cur < 15min）
- [ ] 测试用例 4：支线活跃时主线 bonus
- [ ] 测试用例 5：幂函数差异化效果验证
- [ ] 测试用例 6：今日累计矫正（支线超时 → 主线加分）
- [ ] 测试用例 7：空项目列表 / 无 session 项目等边界情况

### P1-3 Dashboard API 集成

- [ ] 在 `GET /api/v1/dashboard` 响应中为每个 matched root 增加 `reminder_score` 和 `reminder_level` 字段
- [ ] `reminder_level` 取值：`"none"` / `"low"` / `"medium"` / `"high"`
- [ ] 初始阶段继续使用轮询刷新（每次请求重新计算分数）

## P2 — Attention Mask（前端注意力遮罩层）

### P2-1 TypeScript 类型 + Composable

- [ ] `web/src/types/dashboard.ts`：增加 `reminder_score: number` 和 `reminder_level: string` 字段
- [ ] `web/src/composables/useReminderScores.ts`：从 Dashboard 响应中提取分数，计算归一化 intensity（`min(1, score / 10)`）
- [ ] 在 `DashboardView.vue` 中通过 props 传递给 PrimaryCard / SecondaryCard / GroupCard

### P2-2 遮罩层 CSS 基础

- [ ] PrimaryCard / SecondaryCard / GroupCard 已设置 `position: relative`（确认或添加）
- [ ] 添加 `::before` 伪元素：
  ```css
  .project-card::before {
    content: '';
    position: absolute; top: 0; left: 0;
    width: 100%; height: 100%;
    pointer-events: none;
    transition: opacity 0.2s ease;
    z-index: 1;
  }
  ```
- [ ] 使用 CSS 变量控制遮罩 `background` 和 `opacity`

### P2-3 三级提醒视觉

- [ ] 低提醒 (`reminder_level = "low"`)：`rgba(0,0,0,0.3)`
- [ ] 中提醒 (`reminder_level = "medium"`)：`rgba(0,0,0,0.6)`
- [ ] 高提醒 (`reminder_level = "high"`)：`rgba(0,0,0,0.9)`
- [ ] 或使用动态 `--mask-opacity` 变量：`opacityFromScore(score) = min(0.8, score / 12)`
- [ ] 无提醒时遮罩完全透明（`opacity: 0`）

### P2-4 交互细节

- [ ] 卡片 `:hover` 时降低遮罩透明度（或完全清晰），方便查看内容
- [ ] 确保遮罩层点击穿透（`pointer-events: none` 已在伪元素中设置）
- [ ] `transition` 平滑过渡（已在基础 CSS 中设置 `0.2s ease`）

### P2-5 预留皮肤扩展接口

- [ ] 在 `:root` 或组件中定义 CSS 变量：
  ```css
  --mask-bg: rgba(0, 0, 0, 0.5);
  --mask-blend: normal;
  ```
- [ ] 遮罩层背景引用变量：`background: var(--mask-bg); mix-blend-mode: var(--mask-blend);`
- [ ] 不实现具体皮肤切换 UI，仅预留变量接口（为 [`docs/design/99-dual_layer_skinning_system.md`](./docs/design/99-dual_layer_skinning_system.md) 做铺垫）

## P3 — 联调与收尾

### P3-1 端到端验证

- [ ] 模拟场景验证：不同项目状态组合 → 检查 API 返回的 reminder_score 正确性
- [ ] 前端验证：确认遮罩层 opacity 随分数正确变化
- [ ] 三级提醒视觉确认（低/中/高/无各状态均正确）
- [ ] Hover 交互确认（降低遮罩后内容可读）

### P3-2 边界情况

- [ ] 无项目根标记时（全部未归类）——系统正常运行，遮罩仅对主线有效
- [ ] 所有 session 处于同一状态时
- [ ] API 返回错误时前端优雅降级（遮罩保持透明）

### P3-3 文档收尾

- [ ] 更新 `docs/note.md` 记录实现过程中的关键决策
- [ ] 将完成的任务从 todo.md 回写到 `docs/backlog.md`

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
- [`docs/design/99-dual_layer_skinning_system.md`](./docs/design/99-dual_layer_skinning_system.md) — 远期换肤系统
- [`docs/design/99-ai-chat-web-attach.md`](./docs/design/99-ai-chat-web-attach.md) — 远期浏览器扩展
- [`docs/reference.md`](./docs/reference.md) — 设计参考理论知识
