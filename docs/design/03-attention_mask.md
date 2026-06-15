# pflow 注意力遮罩层技术方案（MVP 版）

## 1. 概述

本文档描述 pflow Dashboard 中用于实现**注意力引导**的视觉遮罩层技术方案。该层独立于内容层（项目卡片、会话列表等），通过叠加半透明遮罩并动态调整其透明度（或其他视觉属性），直观反映任务的“提醒分数”。

**核心原则**：
- 遮罩层使用 CSS 伪元素（`::before` / `::after`）实现，不污染 DOM 结构。
- 遮罩层设置 `pointer-events: none`，确保点击事件穿透到下方卡片。
- 遮罩层的透明度（或背景图像）与提醒分数关联，随分数线性或非线性变化。
- 预留皮肤扩展接口，通过 CSS 变量支持未来更换背景图片、混合模式等。

---

## 2. 技术实现

### 2.1 基础 HTML/CSS 结构（Vue 3 组件）

每个项目卡片组件结构如下：

```vue
<template>
  <div 
    class="project-card" 
    :class="maskClass"
    :style="maskStyle"
    @click="onCardClick"
  >
    <!-- 卡片原有内容：项目名称、会话列表、状态等 -->
    <div class="card-content">
      <h3>{{ project.name }}</h3>
      <!-- ... -->
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue';

const props = defineProps(['project', 'reminderScore']);

// 提醒分数归一化到 0~1，用于透明度（或其它效果）
const intensity = computed(() => Math.min(1, props.reminderScore / 10));

// 动态样式：透明度或背景
const maskStyle = computed(() => ({
  '--mask-opacity': intensity.value,
  // 可扩展其他 CSS 变量
}));

// 也可根据分数区间添加不同 class（用于不同表现）
const maskClass = computed(() => {
  if (props.reminderScore >= 10) return 'mask-high';
  if (props.reminderScore >= 5) return 'mask-medium';
  if (props.reminderScore >= 2) return 'mask-low';
  return '';
});
</script>

<style scoped>
.project-card {
  position: relative;   /* 为伪元素提供定位上下文 */
  overflow: hidden;
  /* 卡片原有样式（背景、边框、圆角等） */
  background: var(--card-bg, #1e293b);
  border-radius: 0.75rem;
  /* ... */
}

/* 注意力遮罩层（伪元素） */
.project-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;   /* 让点击穿透到卡片内容 */
  transition: opacity 0.2s ease;
  /* 默认透明，后续通过 class 或 style 覆盖 */
  background: rgba(0, 0, 0, 0);
  z-index: 1;
}

/* 根据提醒分数设置透明度（简单半透明黑） */
.project-card.mask-low::before {
  background: rgba(0, 0, 0, 0.3);
}
.project-card.mask-medium::before {
  background: rgba(0, 0, 0, 0.6);
}
.project-card.mask-high::before {
  background: rgba(0, 0, 0, 0.9);
}

/* 或使用 CSS 变量动态控制（更灵活） */
.project-card {
  --mask-opacity: 0;
}
.project-card::before {
  background: rgba(0, 0, 0, var(--mask-opacity));
}
</style>
```

### 2.2 动态关联提醒分数

在父组件（Dashboard）中，每个项目拥有实时计算的 `reminderScore`，通过 props 传递给卡片组件。卡片内部根据分数设置 `maskStyle` 中的 CSS 变量 `--mask-opacity`，伪元素自动更新。

**更高效的归一化函数**（可根据需要调整曲线）：

```javascript
function opacityFromScore(score) {
  // 使用 sigmoid 或简单线性，使 0~10 分映射到 0~0.8
  return Math.min(0.8, score / 12);
}
```

### 2.3 预留皮肤扩展

通过 CSS 变量，后续可以轻松更换遮罩层的表现，而不修改组件逻辑。

```css
/* 默认皮肤：半透明黑色遮罩 */
:root {
  --mask-bg: rgba(0, 0, 0, 0.5);
  --mask-blend: normal;
}

/* 磨砂玻璃皮肤 */
.theme-glass {
  --mask-bg: rgba(255, 255, 255, 0.2);
  --mask-blend: overlay;
  backdrop-filter: blur(4px); /* 可叠加模糊效果 */
}

/* 云雾皮肤 */
.theme-fog {
  --mask-bg: radial-gradient(circle at center, rgba(200,200,200,0.4), rgba(100,100,100,0.8));
  --mask-blend: multiply;
}

/* 门/光效皮肤（未来可替换为背景图） */
.theme-door {
  --mask-bg: url('/images/door-closed.png') no-repeat center/contain;
  --mask-blend: normal;
}

/* 应用皮肤到遮罩层 */
.project-card::before {
  background: var(--mask-bg);
  mix-blend-mode: var(--mask-blend);
  /* backdrop-filter 可单独设置 */
}
```

用户切换皮肤时，只需更改根元素的类（如 `document.documentElement.classList.add('theme-glass')`），所有卡片遮罩层自动适配。

---

## 3. 性能优化

- **使用 `transform: translateZ(0)` 提升合成层**（可选）：当卡片很多时，可以开启硬件加速。
- **避免使用 `backdrop-filter`** 在低端设备上可能卡顿，MVP 阶段仅使用半透明背景。
- **限制卡片数量**：Dashboard 默认显示主线 + 支线 + 最近活跃普通项目（最多 10 个），其余折叠或分页。

---

## 4. 交互细节

- **鼠标悬停**：当用户将鼠标移到卡片上时，可以临时降低遮罩透明度（或完全清晰），方便查看内容。这通过添加 `:hover` 样式覆盖即可。
- **点击卡片**：由于遮罩层 `pointer-events: none`，点击直接传递给卡片，可正常切换焦点或打开终端。
- **动画过渡**：遮罩透明度变化添加 `transition: all 0.2s`，避免突变。

示例：

```css
.project-card:hover::before {
  opacity: 0.1;  /* 或者 background: rgba(0,0,0,0) */
}
```

---

## 5. 与提醒分数算法的集成

后端（或前端 store）定期计算每个项目的 `reminderScore`，通过 WebSocket 或轮询推送到 Dashboard 组件。卡片组件接收后动态调整遮罩透明度。

**数据流**：
```
提醒分数算法 → 更新 store 中 projects[].reminderScore → 卡片组件 props 变化 → 重新计算 intensity → CSS 变量更新 → 遮罩透明度变化
```

---

## 6. 总结

本方案使用纯 CSS 伪元素实现独立的注意力遮罩层，具有以下优点：
- **低侵入**：不修改现有卡片内容结构。
- **高性能**：仅使用 `background` 和 `opacity`，无额外 DOM 元素。
- **易扩展**：通过 CSS 变量支持多种皮肤（半透明、磨砂玻璃、云雾、图片等）。
- **与算法解耦**：遮罩层只关心归一化的强度值，提醒分数计算独立演进。

MVP 阶段可仅实现半透明黑色遮罩 + 透明度映射，后续逐步丰富皮肤系统。