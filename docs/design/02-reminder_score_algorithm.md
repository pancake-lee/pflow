# pflow 提醒分数算法设计文档

## 1. 概述

本文档定义 pflow 中用于**渐进式提醒**的核心算法。该算法根据用户当前的专注状态、任务的等待时长、今日累计投入时间以及任务优先级（主线/支线/普通），计算每个任务的“提醒分数”。提醒分数决定用户界面中任务卡片的视觉突出程度以及是否触发桌面通知。

**设计目标**：
- 保护主线任务的深度工作，初始 15 分钟内完全不打扰。
- 随着专注时间延长，逐渐提高支线任务的可见性，引导用户适时切换。
- 防止多任务同时高亮，通过非线性变换拉大分数差距。
- 当用户没有活跃任务时，只突出主线任务。
- 校准时间分配偏差（支线占用过多时间时，增加主线提醒分数）。

---

## 2. 输入变量

针对每个任务（项目） `i`，算法需要以下输入：

| 变量 | 类型 | 含义 | 获取方式 |
|------|------|------|----------|
| `waiting_i` | 浮点数（分钟） | 任务下所有会话中最长的 waiting 持续时间（无 waiting 则为 0） | 从 session 状态中的 `waitingSince` 计算当前时间差 |
| `streak_i` | 浮点数（分钟） | 用户在当前任务上的**连续活跃分钟数**。若最近一次活动距当前时间小于 5 分钟，则继续累加；否则重置为 0 | 监听 Dashboard 上的用户操作（点击、终端输入、切换 session 等），记录最近活动时间戳 |
| `total_i` | 浮点数（分钟） | 今日累计专注分钟数（自 0 点起，`streak_i` 增量的累加） | 每日重置，每次活跃增加时长 |
| `is_primary_i` | 布尔 | 是否为主线任务 | 用户设定 |
| `is_current` | 布尔 | 是否为当前活跃任务（用户最近操作的那个） | 根据 `streak_i > 0` 且最近活动时间戳最大者判定 |

全局常量（可配置）：

| 常量 | 默认值 | 含义 |
|------|--------|------|
| `PROTECT_MIN` | 15 | 专注保护期（分钟），此期间其他任务不产生提醒 |
| `W_WAIT` | 1.0 | 等待时间权重 |
| `W_STREAK` | 0.5 | 专注时间乘数系数（每 15 分钟） |
| `PRIMARY_BONUS` | 2.0 | 支线活跃时，主线的额外乘数 |
| `W_CORRECT` | 0.5 | 今日累计时间矫正系数 |
| `EXP_POWER` | 2.0 | 差异化指数（幂函数指数） |
| `REMINDER_THRESHOLDS` | [2, 5, 10] | 低/中/高提醒分数阈值 |

---

## 3. 计算步骤

### 3.1 确定当前活跃任务

找出所有 `streak_i > 0` 的任务中，最近活动时间戳最大的那一个，记为 `cur`。如果没有任何任务有 `streak_i > 0`，则 `cur = null`。

### 3.2 基础等待分

```
base_i = waiting_i * W_WAIT
```

### 3.3 专注干扰因子（仅当 `cur` 非空时）

对于 `cur` 任务本身：`factor_cur = 0`（自己不因专注而增加提醒）。

对于其他任务 `i != cur`：
- 若 `streak_cur < PROTECT_MIN`，则 `factor_i = 0`（保护期内，不产生任何提醒因子）。
- 否则 `factor_i = min( (streak_cur / PROTECT_MIN) * W_STREAK, 2.0 )`。
- 若 `cur` 是支线任务（`is_primary_cur == false`）且 `i` 是主线任务（`is_primary_i == true`），则 `factor_i = factor_i * PRIMARY_BONUS`。

### 3.4 当前活跃调整分

```
adjusted_i = base_i * factor_i
```

若 `cur == null`（无活跃任务），则：
- 主线任务：`adjusted_primary = base_primary`（因子为 1）
- 其他任务：`adjusted_i = 0`

### 3.5 今日累计时间矫正（仅针对主线任务）

如果主线任务的今日累计时间 `total_primary` 小于**所有支线任务的平均今日累计时间** `avg_secondary_total`，则增加一个修正分：

```
correction = max(0, (avg_secondary_total - total_primary) * W_CORRECT)
adjusted_primary = adjusted_primary + correction
```

### 3.6 差异化拉大差距（幂函数）

为防止多个任务同时拥有相近的提醒分数，对正数分数进行指数放大：

```
raw_i = adjusted_i
if raw_i > 0:
    final_score_i = raw_i ^ EXP_POWER
else:
    final_score_i = 0
```

指数 `EXP_POWER > 1` 会使分数差距扩大，高分更高，低分更低。

### 3.7 提醒等级

根据 `final_score_i` 对照阈值确定提醒强度：

| 等级 | 条件 | 界面表现 |
|------|------|----------|
| 无提醒 | `final_score_i < REMINDER_THRESHOLDS[0]` | 仅更新 Dashboard 角标（若有） |
| 低提醒 | `REMINDER_THRESHOLDS[0] ≤ final_score_i < REMINDER_THRESHOLDS[1]` | 卡片背景色变化/边框闪烁 |
| 中提醒 | `REMINDER_THRESHOLDS[1] ≤ final_score_i < REMINDER_THRESHOLDS[2]` | 桌面通知 + 卡片呼吸灯效果 |
| 高提醒 | `final_score_i ≥ REMINDER_THRESHOLDS[2]` | 声音 + 居中弹窗，建议切换任务 |

**同一时间只对 `final_score_i` 最高的一个任务发出中/高提醒**，避免信息过载。

---

## 4. 算法特性与符合性说明

| 用户期望 | 算法体现 |
|----------|----------|
| 刚切换到主线任务，至少 15 分钟不被打扰 | `factor_i = 0` 当 `streak_cur < PROTECT_MIN` |
| 长时间专注主线 → 支线提醒分数增加 | `factor_i` 随 `streak_cur` 线性增长 |
| 长时间专注支线 → 主线和另一支线分数增加，主线额外乘 `PRIMARY_BONUS` | 支线活跃时主线 `factor_i` 额外乘以 `PRIMARY_BONUS` |
| 无任何任务活跃 → 只提醒主线 | `cur == null` 时仅主线 `adjusted` 非零 |
| 支线占用时间超过主线 → 主线提醒分数增加 | 今日累计矫正，`correction` 为正 |
| 避免多任务同时强提醒 | 幂函数拉大差距，且实际提醒只取最高分任务 |

---

## 5. 配置示例（`~/.pflow/config.json`）

```json
{
  "attention": {
    "protect_min": 15,
    "w_wait": 1.0,
    "w_streak": 0.5,
    "primary_bonus": 2.0,
    "w_correct": 0.5,
    "exp_power": 2.0,
    "reminder_thresholds": [2, 5, 10]
  }
}
```

用户可自行调整这些参数以适应个人工作节奏。例如，希望更敏感可降低 `protect_min` 或增大 `w_wait`。

---

## 6. 计算示例

**场景**：主线 P 专注中，`streak_P = 25` 分钟；支线 S1 等待 12 分钟，S2 等待 3 分钟；今日累计：P=40，S1=60，S2=20。常量采用默认值。

- `base_P = 0`, `base_S1 = 12`, `base_S2 = 3`。
- `streak_P = 25 ≥ 15`，`factor_S1 = (25/15)*0.5 = 0.833`，`factor_S2` 相同。主线因子为 0。
- `adjusted_P = 0`, `adjusted_S1 = 12*0.833 = 10`, `adjusted_S2 = 3*0.833 = 2.5`。
- 支线平均 `(60+20)/2 = 40`，主线累计 40，差值 0，无矫正。
- `final_S1 = 10^2 = 100`（高提醒），`final_S2 = 2.5^2 = 6.25`（中提醒），`final_P = 0`。
- 结果：S1 触发高提醒，S2 中提醒，主线无提醒。

**另一种场景**：无活跃任务，主线等待 5 分钟，支线等待 10 分钟。
- `base_P = 5`, `base_S1 = 10`。
- `cur = null` → `adjusted_P = 5`, `adjusted_S1 = 0`。
- `final_P = 25`（中高提醒），S1 为 0。
- 仅主线获得提醒。

---

## 7. 相关心理模型

本算法借鉴了以下心理学理论：
- **耶克斯-多德森定律**：中等唤醒水平表现最佳；渐进提醒逐步提升唤醒。
- **注意恢复理论**：长时间专注后需要切换；算法在专注超时后提高支线可见度。
- **记忆提取-放弃模型**：通过逐渐增强提醒帮助用户维持对挂起任务的元认知。

详见《pflow 设计参考理论知识》文档。

---

*本文档对应 pflow 版本 v0.2+*