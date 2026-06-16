# 主 Session 判定：Star 加成机制

> 本文档定义 pflow 仪表盘中每个项目卡片的"主 Session"判定规则。核心机制：**⭐ 不是"指定主 Session"，而是给指定 session 一个时间加成，让它在竞逐主 Session 时拥有 30 分钟的"宽容窗口"。**

---

## 1. 核心概念：Star ≠ Main Session

旧设计（已废弃）中，⭐ 按钮直接将某个 session 设为主 session（硬覆盖）。新设计中：

| 概念                                 | 含义                          | 持久化          | 作用方式                                  |
| ------------------------------------ | ----------------------------- | --------------- | ----------------------------------------- |
| **Star（星标）**               | 用户对某个 session 的偏好标记 | ❌ 前端内存 ref | 在 `last_active` 对比时提供 30 分钟加成 |
| **Main Session（主 Session）** | 卡片头部突出展示的 session    | N/A（每次计算） | 取 effective_last_active 最大的 session   |

**关键区别**：Star 不等于主 Session。Star 只是给 session 一个"竞争优势"，如果被星标的 session 闲置太久（超过加成时长），最近活跃的其他 session 仍然会成为主 Session。

---

## 2. 判定规则：effective_last_active 比大小

### 2.1 算法

```
对于 group 中的每个 session：
  effective_time = last_active + (is_starred ? STAR_BONUS_MINUTES : 0)

返回 effective_time 最大的 session
```

伪代码：

```typescript
const STAR_BONUS_MINUTES = 30  // 可配置，见 §2.3

function getMainSession(group: SessionGroup | null): DashboardEntry | null {
  if (!group || group.sessions.length === 0) return null

  const now = Date.now()
  let best: DashboardEntry | null = null
  let bestTime = -Infinity

  for (const s of group.sessions) {
    const base = new Date(s.last_active).getTime()
    const bonus = s.session_id === starredSessionIds.value[group.key]
      ? STAR_BONUS_MINUTES * 60_000
      : 0
    const effective = base + bonus
    if (effective > bestTime) {
      bestTime = effective
      best = s
    }
  }

  return best
}
```

### 2.2 直觉示例

假设当前时间是 14:00，项目下有 A、B 两个 session：

| 场景              | Session A (⭐)          | Session B (无星标)      | 胜出 | 原因                          |
| ----------------- | ----------------------- | ----------------------- | ---- | ----------------------------- |
| 刚切换            | `last_active = 13:55` | `last_active = 13:58` | A    | 13:55 + 30min = 14:25 > 13:58 |
| 闲置略久          | `last_active = 13:20` | `last_active = 13:35` | A    | 13:20 + 30min = 13:50 > 13:35 |
| 闲置太久          | `last_active = 12:00` | `last_active = 13:35` | B    | 12:00 + 30min = 12:30 < 13:35 |
| ⭐ session 已退出 | 不存在于 group 中       | `last_active = 13:45` | B    | star 标记失效，只剩 B         |

**设计意图**：⭐ 提供了一个 30 分钟的"宽容窗口"——用户去接杯水、回个消息再回来，被星标的 session 仍是主 Session。但如果用户已经切换到另一个 session 工作很久，star 自动失效。

### 2.3 加成参数定义

```typescript
// web/src/config/attention.ts

/**
 * 主 Session 星标加成（分钟）。
 *
 * 被 ⭐ 标记的 session 在竞争主 Session 时，其 last_active 会
 * 获得此分钟数的虚拟偏移。只要该 session 的闲置时间不比其他
 * session 长超过此值，它就会保持为主 Session。
 *
 * 设为 0 则星标无效果（等效于纯按 last_active 排序）。
 */
export const STAR_BONUS_MINUTES = 30
```

参数集中放在 `web/src/config/attention.ts`，与马赛克 / 遮罩等可调参数在一起，方便微调。

---

## 3. 状态管理

### 3.1 starredSessionIds

```typescript
// web/src/views/DashboardView.vue

// 每个 project group 最多一个星标 session
const starredSessionIds = ref<Record<string, string>>({})

function handleStarSession(groupKey: string, sessionId: string) {
  // 同一个 project 内互斥：新 star 自动取消旧 star
  starredSessionIds.value = { ...starredSessionIds.value, [groupKey]: sessionId }
}
```

**约束**：

- 每个 project group 最多 1 个 star
- 点击新的 ⭐ 自动替换旧的（group 内互斥）
- 不持久化（下次刷新后 star 丢失，用户需重新标记）

### 3.2 与旧设计的对照

|                     | 旧设计 (`mainSessionIds`) | 新设计 (`starredSessionIds`) |
| ------------------- | --------------------------- | ------------------------------ |
| 语义                | "指定主 Session"（硬覆盖）  | "给 Session 加成"（软偏好）    |
| 影响方式            | 直接返回指定 session        | 影响 effective_time 排序       |
| 被标记但闲置过久    | 仍是主 Session              | 自动让位给更活跃的 session     |
| 标记的 session 退出 | 降级到 is_active            | 降级到纯 last_active 排序      |
| group 内互斥        | ✅                          | ✅                             |

---

## 4. UI 交互

### 4.1 ⭐ 按钮的 hover 提示

```
┌─────────────────────────────────────────────────────────────┐
│ Other sessions:                                             │
│   🖥  def456  🟡 waiting  15m  │ "Refactor API"   │ ⭐ 🖥   │
│                                           ┌─────────────────┐
│                                           │ 星标后只要闲置时间 │
│                                           │ 比其他对话不长于   │
│                                           │ 30min 即可保持    │
│                                           │ 主要对话          │
│                                           └─────────────────┘
```

Tooltip 文案：**"星标后只要闲置时间比其他对话不长于 30min 即可保持主要对话"**

实现时从 `STAR_BONUS_MINUTES` 变量动态拼接文案，确保代码变量与 UI 提示一致。

### 4.2 星标状态的视觉反馈

- 🌟 Star Session的按钮，按钮本身就是标识
- ⭐ 不是Star Session的按钮
- Main Session标题栏中，sessionid和tty图标之间，加入Star Session的按钮

### 4.3 事件流

```
PrimaryCard ⭐ click
  → emit('starSession', row.session_id)
  → DashboardView.handleStarSession(group.key, sessionId)
  → starredSessionIds[group.key] = sessionId
  → 响应式更新 → getMainSession() 重新计算 effective_time
  → 卡片头部立即切换（如果星标 session 的 effective_time 胜出）
```

---

## 5. 无 Star 时的行为：纯 last_active 排序

当没有任何 session 被星标时（`starredSessionIds` 为空或对应 key 无记录），算法退化为：

```
effective_time = last_active + 0  // 无加成
→ 返回 last_active 最新的 session
```

这与旧设计中 `is_active` 优先的规则不同：

- **旧**：`is_active` 优先 → 当多个 session 都 active 时取第一个（有随机性）
- **新**：纯 `last_active` 排序 → 确定性更强，始终展示最近有活动的 session

---

## 6. 与后端注意力算法的关系

主 Session 的选择 **不影响** 后端的注意力分数计算。

`computeReminderScores()` 按项目聚合所有 session 的指标（waiting、streak、total），不关心哪个是"主 Session"。前端的主 Session 选择纯粹是一个展示层面的决策——决定卡片头部展示哪个 session 的元数据。

```
后端 computeReminderScores()        前端 getMainSession()
────────────────────────────        ────────────────────
输入: 该项目的所有 sessions          输入: 该项目的所有 sessions
聚合: waiting max, streak, total    比较: effective_last_active
输出: reminder_score, fog_score     输出: 一个 DashboardEntry
用途: 控制卡片的马赛克/遮罩效果       用途: 控制卡片头部展示的 session 信息
```

---

## 7. 边界情况

### 7.1 所有 session 的 last_active 都很久远

- 算法照常运行，选出 effective_time 最大的 session
- 即使所有 session 都已 inactive，仍有一个"主 Session"

### 7.2 项目 group 为空

- `getMainSession()` 返回 `null`
- 卡片显示空占位符（`PLACEHOLDER_ROW`）

### 7.3 星标 session 退出

- 被星标的 session 从扫描结果中消失
- `starredSessionIds` 中的记录不再匹配任何 session
- `base + bonus` 逻辑自然跳过（因为该 session 不在 group.sessions 中）
- 自动回退到纯 last_active 排序

### 7.4 页面刷新

- `starredSessionIds` 重置为 `{}`
- 所有 star 标记丢失
- 所有项目卡片按纯 last_active 排序
- 用户需重新标记

### 7.5 多个 session last_active 完全相同

- bonus 打破平局（被星标的胜出）
- 如果都没星标或都有星标（不可能，因为互斥），取数组中先出现的

### 7.6 STAR_BONUS_MINUTES = 0

- effective_time = last_active
- 行为等价于纯按 last_active 排序
- 星标按钮可保留但无实际效果（或 UI 层面隐藏）

---

## 8. 设计权衡

### 8.1 为什么用"时间窗口"而非"硬指定"

硬指定（旧设计）的问题：

- 用户早上标记了 session A，下午 session A 早已退出，但仍显示为"主 Session"
- 用户切换到 session B 工作一小时后，A 仍然是主 Session（除非手动再点 ⭐）

时间窗口方案：

- 用户偏好（⭐）被尊重，但有上限
- 自动适应实际使用模式——最近活跃的 session 最终会胜出
- 减少用户手动维护的负担

### 8.2 为什么是 30 分钟

- 足够覆盖短暂离开（接水、去洗手间、简短会议）
- 超出这个时长通常意味着用户确实切换了工作上下文
- 可以通过修改 `STAR_BONUS_MINUTES` 调整
- 参数暴露在 `web/src/config/attention.ts` 中，方便微调

### 8.3 为什么不持久化 star 标记

- 与当前架构一致（`mainSessionIds` 也不持久化）
- Session 生命周期短（进程退出即消失），持久化的价值有限
- 避免引入 localStorage 依赖（SSR 兼容性等）
- 未来可选：持久化到 `localStorage` 或 `~/.pflow/starred.json`

---

## 9. 关键代码路径（规划）

| 功能                      | 文件                                     | 关键标识                                     |
| ------------------------- | ---------------------------------------- | -------------------------------------------- |
| Star 加成常量             | `web/src/config/attention.ts`          | `STAR_BONUS_MINUTES`                       |
| 星标状态                  | `web/src/views/DashboardView.vue`      | `starredSessionIds` ref                    |
| 主 Session 计算           | `web/src/views/DashboardView.vue`      | `getMainSession()` — effective_time 排序  |
| 星标点击处理              | `web/src/views/DashboardView.vue`      | `handleStarSession()`                      |
| ⭐ 按钮 + tooltip         | `web/src/components/PrimaryCard.vue`   | Ops 列，`emit('starSession', ...)`         |
| ⭐ 按钮 + tooltip（支线） | `web/src/components/SecondaryCard.vue` | 同上                                         |
| Other sessions 分割       | `web/src/components/PrimaryCard.vue`   | `otherSessions` computed（排除主 Session） |
| 卡片头部展示              | `web/src/components/PrimaryCard.vue`   | `v-if="mainSession"` 模板块                |
