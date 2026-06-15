# pflow 提醒分数算法设计文档（双维度版）

---

## 1. 概述

本文档定义 pflow 中用于**渐进式注意力引导**的双维度核心算法。算法为每个任务（项目）输出两个独立的分数：

- **提醒分数**（`reminder_score`）：表示任务需要用户**立即关注**的紧急程度。数值越高，视觉上越应该“主动吸引”用户（如发光、脉冲、缩放）。
- **迷雾分数**（`fog_score`）：表示任务应当被**视觉抑制**的程度。数值越高，遮罩层越浓（半透明、模糊、灰度），让任务显得“不重要”或“应暂时忽略”。

双维度分离使得系统可以**同时表达“拉住你”和“推开你”两种注意力导向**，比单一透明度更符合人类视觉注意机制。

---

## 2. 设计原则

1. **当前活跃项目**（用户正在操作的那个）永远：
   - `reminder_score = 0`（不需要提醒自己）
   - `fog_score = 0`（不需要遮住自己）

2. **保护期**（用户显式开启专注模式且专注时长未达到设定值）：
   - 非当前项目的 `fog_score` 被强制提高（主动“推开”），`reminder_score` 保持低值。
   - 保护期结束后，`fog_score` 逐渐降低，`reminder_score` 逐渐升高。

3. **无活跃任务时**：只对主线项目计算 `reminder_score`，其他项目 `reminder_score = 0` 且 `fog_score` 较高。

4. **支线任务长时间等待** → `reminder_score` 升高（拉你过去）。
5. **主线被支线过度占用时间** → 主线 `reminder_score` 升高（补偿机制）。

6. **两个分数正交**：提醒分数高 ≠ 迷雾分数低，但通常情况下呈负相关（因为需要清晰呈现）。保护期是打破负相关的特例：此时迷雾分数高而提醒分数低。

---

## 3. 输入变量

每个任务 `i` 的输入与单维度版相同：

| 变量 | 类型 | 含义 |
|------|------|------|
| `waiting_i` | float64 (min) | 最长 waiting 持续时间 |
| `streak_i` | float64 (min) | 当前连续活跃分钟数 |
| `total_i` | float64 (min) | 今日累计专注分钟数 |
| `is_primary_i` | bool | 是否为主线 |
| `last_active` | time.Time | 最后活动时间 |

全局状态：

| 变量 | 含义 |
|------|------|
| `curProject` | 当前活跃项目路径（`streak` 最大且 `last_active` 在 `CurWindow` 内） |
| `focusActive` | 是否显式开启专注模式 |
| `focusMinutes` | 专注模式设定的保护时长（分钟） |

常量（同单维度版）：

| 常量 | 默认值 | 含义 |
|------|--------|------|
| `CurWindow` | 60 min | cur 判定窗口 |
| `ProtectMin` | 5 min | 专注保护期参考阈值（用于因子计算） |
| `WWait` | 1.0 | 等待时间权重 |
| `WStreak` | 0.5 | streak 因子系数 |
| `PrimaryBonus` | 2.0 | 支线→主线战略乘数 |
| `WCorrect` | 0.5 | 今日累计校正系数 |
| `ExpPower` | 2.0 | 提醒分数幂指数 |
| `MinFactor` | 0.5 | streak 不可测量时的兜底因子 |
| `FogProtectMax` | 0.9 | 保护期最大迷雾分数 |
| `FogBaseNonProtect` | 0.3 | 非保护期基础迷雾分数（用于无提醒时） |

---

## 4. 提醒分数计算（`reminder_score`）

沿用单维度版算法，最终值记为 `reminder_raw`，再应用幂函数得到 `reminder_score`。

### 4.1 步骤摘要

1. 确定 `curProject`。
2. `base_i = waiting_i * WWait`。
3. 干扰因子 `factor_i`：
   - 若 `curProject == nil`：仅主线 `factor=1`，其他 `factor=0`。
   - 若 `i == curProject`：`factor=0`。
   - 其他：
     - 若 `focusActive && streak_cur < focusMinutes`：`factor=0`（保护期完全抑制提醒）。
     - 否则 `streak_ratio = min( (streak_cur / ProtectMin) * WStreak, 2.0 )`。
     - 若当前为支线且目标为主线，`streak_ratio *= PrimaryBonus`。
     - `factor = max(streak_ratio, MinFactor)`（当 `streak_cur` 测量为 0 时使用 `MinFactor`）。
4. `adjusted_i = base_i * factor_i`。
5. 今日累计校正（仅主线）：`correction = max(0, median(secondary_total) - primary_total) * WCorrect`，加至 `adjusted_primary`。
6. `reminder_score_i = adjusted_i ^ ExpPower`（若 `adjusted_i > 0`，否则 0）。

**关键改动**：保护期内 `factor=0` → `reminder_score=0`。此时所有注意力应被迷雾抑制，而非提醒。

---

## 5. 迷雾分数计算（`fog_score`）

`fog_score` 是一个 `[0, 1]` 区间的值，0 表示完全清晰（无遮罩），1 表示完全被迷雾覆盖。

### 5.1 基础规则

- **当前项目**：`fog_score = 0`（永远清晰）。
- **无当前项目时**：主线 `fog_score = 0`，其他项目 `fog_score = 0.7`（基本遮蔽）。
- **保护期**（`focusActive && streak_cur < focusMinutes`）：
  - 非当前项目的 `fog_score` 按保护期剩余比例计算，取值范围 `[FogProtectMin, FogProtectMax]`。
  - 剩余因子 `remain_ratio = 1 - (streak_cur / focusMinutes)`（保护期越长，剩余比例越小？streak_cur 是已专注时间，剩余时间 = focusMinutes - streak_cur，注意保护期定义是“streak_cur < focusMinutes”，剩余比例 = (focusMinutes - streak_cur) / focusMinutes）。
  - `fog_score = FogProtectMax * remain_ratio + FogProtectMin * (1 - remain_ratio)`。
  - 例如 `FogProtectMax=0.9, FogProtectMin=0.5`，刚开始 streak_cur=0 → remain=1 → fog=0.9；快结束时 streak_cur≈focusMinutes → fog≈0.5。

- **非保护期**（包括未开启专注模式或已过保护期）：
  - 首先计算“无关紧要度” `unimportance_i`，基于等待时间和提醒分数排名。
  - 若 `reminder_score_i` 较大，则迷雾应低。定义 `unimportance_i = 1 - clamp(reminder_score_i / max_reminder, 0, 1)`，其中 `max_reminder` 是当前所有非当前项目的 `reminder_score` 最大值（若为 0 则用 1 避免除零）。
  - 但还需要考虑等待时间极短且无提醒的项目：它们应该被适度遮蔽。因此基础迷雾值为 `FogBaseNonProtect`（例如 0.3），然后根据提醒分数比例进一步降低：
    `fog_score = FogBaseNonProtect * (1 - unimportance_i)`。
  - 也可以更直接：`fog_score = max(0, FogBaseNonProtect - reminder_score_i / (max_reminder + 1))`，但这样提醒分数最大的项目 fog 可能为负，需 clamp。
  - 简化版：`fog_score = clamp(FogBaseNonProtect * (1 - (reminder_score_i / (max_reminder + 1))), 0, 1)`。

- **等待时间为 0 且 streak=0 且非当前**：这些项目可能是闲置会话，给予较高迷雾（0.6）。

### 5.2 推荐公式（MVP 简洁版）

```
if i == curProject:
    fog = 0
elif curProject == nil:
    fog = 0 if is_primary else 0.7
elif focusActive and streak_cur < focusMinutes:
    remain = (focusMinutes - streak_cur) / focusMinutes
    fog = FogProtectMax * remain + 0.5 * (1 - remain)   // FogProtectMin=0.5
else:
    maxRem = max(reminder_score[j] for j != curProject) or 1
    if maxRem == 0:
        fog = FogBaseNonProtect   // 0.3
    else:
        // reminder_score 越高，fog 越低，线性插值范围 [0, FogBaseNonProtect]
        fog = FogBaseNonProtect * (1 - min(1, reminder_score_i / maxRem))
```

**说明**：保护期内 `reminder_score_i` 均为 0，所以上述非保护期分支不会进入，保护期独立控制。

---

## 6. 视觉映射规则

前端根据两个分数分别应用视觉效果：

| 分数 | 取值范围 | 视觉表现 | 实现方式 |
|------|----------|----------|----------|
| `reminder_score` | 0 ~ ∞（通常 >10 即很高） | **高亮强度**：边框发光、饱和色提升、脉冲动画频率/幅度 | CSS `box-shadow` 强度线性；`animation` 时长反比于分数；`filter: brightness()` 增量 |
| `fog_score` | 0 ~ 1 | **遮罩浓度**：半透明黑色层不透明度、模糊程度、灰度程度 | 伪元素 `background-color` 的 alpha = `fog_score`；可选 `backdrop-filter: blur(fog_score * 4px)` |

**叠加规则**：
- 迷雾遮罩位于卡片上方（伪元素），高亮效果通常作用于卡片边框或内容（独立于遮罩）。两者可以共存，但为了高亮清晰，当 `reminder_score` 很高时，可以临时降低 `fog_score`（例如强制 fog=0）。不过由于提醒分数高时迷雾分数已经很低，自然不冲突。
- 当前项目（`isCurrent`）不应用任何高亮或迷雾（保持默认样式）。

---

## 7. 配置常量扩展

在 `~/.pflow/config.json` 中增加：

```json
{
  "attention": {
    // ... 原有常量 ...
    "fog": {
      "protect_max": 0.9,
      "protect_min": 0.5,
      "base_non_protect": 0.3,
      "no_current_other": 0.7
    }
  }
}
```

---

## 8. 计算示例（双维度）

### 示例 A：主线专注 20 分钟，支线 waiting 30 分钟（无 focus 模式）

- `cur = P`, `streak_cur = 20`, `focusActive=false`
- 提醒分数：按原算法，支线 `reminder_score = 3600`（超高）
- 迷雾分数：`maxRem = 3600`，支线 `fog = 0.3 * (1 - min(1, 3600/3600)) = 0`（完全清晰）
- 主线 `fog = 0`，`reminder = 0`。
- 视觉效果：支线高亮（强烈脉冲），主线无特效。用户自然被支线吸引。

### 示例 B：显式专注模式，刚启动 3 分钟

- `focusActive=true, focusMinutes=15, streak_cur=3`
- 提醒分数：支线 `reminder_score = 0`（保护期内 factor=0）
- 迷雾分数：`remain = (15-3)/15 = 0.8`，`fog = 0.9*0.8 + 0.5*0.2 = 0.72+0.1=0.82`
- 主线 `fog=0`。
- 视觉效果：主线清晰，支线被浓雾遮蔽。用户继续专注主线。

### 示例 C：无当前任务，主线 waiting 5 分钟

- `cur = nil`
- 提醒分数：主线 `reminder_score = 25`，其他支线 0。
- 迷雾分数：主线 `fog=0`，支线 `fog=0.7`。
- 视觉效果：主线清晰且高亮（25 分对应中高提醒），支线灰暗。引导用户回到主线。

---

## 9. 与单维度版的对比优势

- **保护期内不再有“分数低导致迷雾轻”的矛盾**：保护期强制高迷雾、低提醒，清晰表达“别看其他任务”。
- **高提醒项目获得主动吸引**：不再是“仅仅不被遮住”，而是主动发光/脉冲。
- **支持更丰富的注意力梯度**：例如一个项目提醒分数中等（微亮），另一个项目迷雾中等（半透明），用户可以自然排序。

---

## 10. 实现注意事项

- 后端 `CalculateScores` 函数应返回结构体 `{ReminderScore float64, FogScore float64}` 的 map。
- 前端需维护两个独立的响应式变量，分别绑定到卡片的样式（`--reminder-intensity` 和 `--fog-opacity`）。
- 高亮动画应使用 CSS `will-change` 和 `transform` 以保证性能。
- 迷雾层使用 `pointer-events: none` 确保点击穿透。

---
