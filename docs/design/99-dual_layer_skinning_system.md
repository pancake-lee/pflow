# pflow 双层换肤系统设计文档

## 1. 概述

本文档定义 pflow 的**双层换肤系统**。该系统将 Dashboard UI 逻辑拆分为两个独立层：

- **底层（内容层）**：负责展示项目卡片、会话列表、状态信息、按钮等**功能性内容**。这一层的视觉风格可以整体替换，例如从“工作面板”风格变为“商铺摊位”风格、RPG 风格、极简风格等。
- **上层（注意力遮罩层）**：独立于内容层，用于表达任务的“提醒分数”或专注状态。这一层也可以单独换肤，例如从“半透明玻璃”变为“战争迷雾”、“灯光渐暗”、“门帘拉下”等隐喻效果。

**核心目标**：
- 两层之间**完全解耦**：更换任意一层的皮肤不影响另一层的逻辑。
- 允许组合：用户可以选择 A 内容皮肤 + B 遮罩皮肤，创造个性化体验。
- 为未来游戏化、趣味化交互预留空间。

---

## 2. 设计理念

### 2.1 为什么要拆分为两层？

| 需求 | 单层实现 | 双层实现 |
|------|----------|----------|
| 改变卡片信息的视觉风格（如变成店铺） | 需要同时调整内容样式和提醒效果，逻辑混杂 | 只改底层，上层保持独立 |
| 改变提醒的表现形式（如用雾气取代透明度） | 需要重写整个卡片的提醒逻辑 | 只改上层，内容层不受影响 |
| 用户混搭（例如“工作内容 + 战争迷雾”） | 不支持 | 支持 |
| 社区贡献皮肤 | 容易冲突，需约定大量 CSS 类名 | 两层独立贡献，互不干扰 |

### 2.2 层级定义

```
┌─────────────────────────────────┐
│         浏览器窗口               │
│  ┌─────────────────────────────┐│
│  │    Dashboard 容器            ││
│  │  ┌───────────────────────┐  ││
│  │  │  上层：注意力遮罩层     │  ││
│  │  │  (伪元素/独立层)       │  ││
│  │  └───────────────────────┘  ││
│  │  ┌───────────────────────┐  ││
│  │  │  底层：内容层          │  ││
│  │  │  (卡片、文字、按钮)    │  ││
│  │  └───────────────────────┘  ││
│  └─────────────────────────────┘│
└─────────────────────────────────┘
```

实际渲染中，上层通过 CSS 伪元素（`::before`）或绝对定位的 `<div>` 覆盖在底层之上，且 `pointer-events: none`。

---

## 3. 技术实现

### 3.1 CSS 变量驱动的换肤

整个系统的换肤基于 CSS 自定义属性（变量）。每个皮肤定义一组变量值，切换时只需修改根元素（`<html>` 或 `<body>`）的类名。

**底层变量**（内容层）：

```css
/* 默认工作面板皮肤 */
:root {
  --card-bg: #1e293b;
  --card-border: 1px solid #334155;
  --card-shadow: 0 4px 6px -1px rgba(0,0,0,0.1);
  --text-primary: #f1f5f9;
  --text-secondary: #94a3b8;
  --button-bg: #3b82f6;
  --button-hover: #2563eb;
  /* 其他内容样式变量 */
}

/* 商铺摊位皮肤 */
.skin-shop {
  --card-bg: url('/assets/wood_plate.png') no-repeat center/cover;
  --card-border: 2px solid #b45309;
  --card-shadow: 0 8px 12px rgba(0,0,0,0.3);
  --text-primary: #fef3c7;
  --text-secondary: #fde68a;
  --button-bg: #d97706;
  --button-hover: #b45309;
}
```

**上层变量**（遮罩层）：

```css
/* 默认半透明黑遮罩 */
:root {
  --mask-bg: rgba(0, 0, 0, 0.5);
  --mask-blend: normal;
  --mask-transition: opacity 0.2s;
}

/* 战争迷雾皮肤 */
.skin-fog {
  --mask-bg: radial-gradient(circle at 30% 40%, rgba(0,0,0,0.2), rgba(0,0,0,0.9));
  --mask-blend: multiply;
}

/* 灯光渐暗皮肤 */
.skin-dimming {
  --mask-bg: linear-gradient(135deg, rgba(0,0,0,0.1), rgba(0,0,0,0.8));
  --mask-blend: darken;
}

/* 门帘皮肤（可结合动画） */
.skin-door {
  --mask-bg: url('/assets/door_half.png') no-repeat center/contain;
  --mask-blend: normal;
}
```

### 3.2 皮肤应用机制

用户通过 Dashboard 上的“设置”面板选择：

- **内容皮肤**：下拉框选择“默认”、“商铺摊位”、“极简白”等。
- **遮罩皮肤**：下拉框选择“半透明黑”、“战争迷雾”、“灯光渐暗”等。

前端 JavaScript 动态切换根元素的类：

```javascript
function applySkin(contentSkin, maskSkin) {
  // 移除所有内容皮肤类
  document.documentElement.classList.remove('skin-shop', 'skin-minimal', ...);
  // 添加选中的内容皮肤类
  document.documentElement.classList.add(contentSkin);
  
  // 同理处理遮罩皮肤（遮罩皮肤类独立）
  document.documentElement.classList.remove('skin-fog', 'skin-dimming', 'skin-door');
  document.documentElement.classList.add(maskSkin);
}
```

### 3.3 遮罩层的强度动态变化

遮罩层需要根据提醒分数动态调整表现（如透明度、图像偏移等）。这通过 CSS 变量 `--mask-intensity` 实现，该变量由前端 computed 属性实时更新。

```css
/* 遮罩层基础样式 */
.project-card::before {
  background: var(--mask-bg);
  opacity: var(--mask-intensity, 0);
  mix-blend-mode: var(--mask-blend);
  transition: var(--mask-transition);
}

/* 战争迷雾皮肤下，强度影响径向渐变的大小和位置 */
.skin-fog .project-card::before {
  background: radial-gradient(circle at 30% 40%, 
    rgba(0,0,0,0.2), 
    rgba(0,0,0, calc(0.5 + var(--mask-intensity) * 0.5)));
}
```

这样，提醒分数越高，遮罩层的视觉抑制效果越强。

---

## 4. 皮肤开发规范

为了方便社区贡献皮肤，定义以下规范：

### 4.1 目录结构

```
pflow/
  web/
    src/
      skins/
        content/            # 内容皮肤
          default.css
          shop.css
          minimal.css
        mask/               # 遮罩皮肤
          default.css
          fog.css
          dimming.css
        index.js            # 皮肤注册与切换逻辑
```

### 4.2 皮肤文件内容

每个皮肤文件导出一组 CSS 变量和必要的动画关键帧。例如 `shop.css`：

```css
.skin-shop {
  --card-bg: url('/assets/wood_plate.png') no-repeat center/cover;
  --card-border: 2px solid #b45309;
  /* 其他变量 */
}
```

### 4.3 动态加载

为减小初始包体积，皮肤文件可异步加载。用户选择皮肤后，动态插入 `<link rel="stylesheet">` 或通过 `import()` 加载。

---

## 5. 示例：商铺摊位皮肤 + 战争迷雾遮罩

### 5.1 视觉描述

- **底层**：每个项目卡片变成一个摊位（木头台面、布招牌）。会话列表像是摊位上的商品，状态指示灯像是闪烁的招牌灯。按钮（如“终端”）做成拉绳或铃铛样式。
- **上层**：战争迷雾遮罩表现为从四周向中心的黑暗雾气，提醒分数越高，雾气越浓，摊位越难看清。当提醒分数极高时，迷雾中会隐约出现一个“？”图标，提示用户去处理。

### 5.2 实现要点

- 底层使用背景图片或 CSS 渐变模拟木头纹理。
- 遮罩层使用径向渐变，中心透明，边缘黑暗。`--mask-intensity` 控制中心透明区域的大小（强度越高，透明区域越小）。
- 迷雾中“？”图标可通过伪元素的 `content` 或额外绝对定位元素实现，根据 `--mask-intensity > 0.8` 显示。

---

## 6. 未来扩展

- **动画皮肤**：遮罩层可以根据提醒分数触发敲门动画、灯光闪烁等。
- **音效联动**：皮肤可附带音效（例如雾气涌动的环境音），通过 Web Audio API 实现，需用户授权。
- **社区市场**：允许用户分享皮肤包，一键导入。

---

## 7. 总结

双层换肤系统将 pflow 的视觉层解耦为**功能内容层**与**注意力引导层**，两者可独立定制和混合搭配。这种架构：
- 降低了皮肤开发的复杂度。
- 为用户提供了极高的个性化自由度。
- 为游戏化、趣味化交互打下了坚实基础。

MVP 阶段仅需实现默认皮肤和简单遮罩层（半透明黑），但代码结构上预留好 CSS 变量和类名切换机制，未来可平滑扩展。