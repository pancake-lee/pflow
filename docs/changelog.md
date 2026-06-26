# changelog

> 版本历史，面向用户描述每个版本的新增内容。

## v0.0.8 (当前)

阶段三 + 阶段四核心功能：

### 项目策略管理（军帐改革）

- **Slot 映射机制** — 1 主线 + 2 支线的固定 slot，替换旧的 priority 字段
  - 主线（primary）、支线 slot #1（secondary_1）、支线 slot #2（secondary_2）
  - 前端下拉选择器替换旧的配置交互
  - Slot 替换：新路径设入 slot 时自动腾退旧路径为 normal
- **"路径即项目"** — 用户标记路径为项目根，子目录 session 自动归入（最长前缀匹配）
- **根目录保护** — `/` 禁止标记为项目根
- **项目折叠记忆** — 记忆用户对普通/归档项目的折叠状态，刷新后保持

### 军情哨（Suggest Engine）

- **`pflow suggest`** — 基于会话状态和历史数据，主动给出分析建议
  - ~20 个分析场景（S1-S20），按优先级排序
  - 覆盖：紧急等待、主线空闲、注意力失衡、异常退出、积极反馈、时间提醒等
- **Dashboard 军情卡片** — Web 端展示 suggest 分析结果
- **知识锚点（Knowledge Anchor）** — Dashboard 右下角知识卡片
  - 12 条认知科学理论卡片（注意力残留、认知卸载、元认知瓶颈等）
  - 跟随军情自动切换，支持轮播和悬停翻阅

### 注意力管理

- **提醒分数算法** — 综合等待时长、专注持续、今日累计、项目优先级计算提醒分数
- **注意力遮罩层** — 双维度设计：雾化遮罩（Fog）+ 高亮跑马灯（Marquee）
- **专注模式** — 点击"专注"按钮进入保护期，非关注项目统一遮罩
  - 每点一次延长 15 分钟保护
  - 保护期内其他项目不推送提醒
- **活跃时间估算** — 三级降级链：tmux 焦点事件 → session 消息数估算 → wall-clock 回退

### 会话管理

- **`pflow hermes`** — 启动 Hermes Agent 托管会话
  - 支持 `-model` 指定模型、`-resume` 恢复已有会话、`-no-attach` 后台启动
- **`pflow session list`** — CLI 表格展示所有会话映射
- **`pflow session destroy`** — 销毁指定 tmux 会话并清理映射
- **Claude JSON 目录扫描** — 通过 `~/.claude/sessions/` 目录扫描替代 statusline 截屏（更快更可靠）
- **Hermes `/status` 截屏解析** — 替代 `hermes sessions export`（更快）
- **Tmux 自动销毁优化** — `trap '' TSTP` 阻止 Ctrl+Z、Agent 退出后自动清理
- **孤儿会话清理** — 启动时扫描并清理无映射记录的 `pflow-*` tmux 会话
- **后台轮询监控** — 每 5s 扫描 session 变化并自动更新映射

### 交互体验

- **主会话标记** — 每个项目下星标标记最主要会话（带时间补偿窗口）
- **Dashboard 终端映射优化** — 使用 Claude `-n` name 优化展示
- **前台页面展示今日累计专注时间**

### 设计文档

- 完整的设计文档体系（`docs/design/`），覆盖提醒算法、遮罩层、测试用例、会话管理、suggest 引擎、活跃时间计算、知识锚点等

---

## v0.0.5

阶段二 + 阶段三核心功能：

### Dashboard（军帐战报）

- **Web Dashboard** — 浏览器访问 `http://localhost:8080`，Vue 3 + Naive UI 暗色主题面板
  - DataTable 会话列表：Agent 图标、Session ID、Project、红绿灯状态、Name、Last Active、Last Req、Last Resp
  - 控制栏：时间窗口筛选（1h/3h/6h/1d/3d/7d）、max_inactive 控制、Agent 类型过滤
  - 自动刷新（off/10s/30s/60s 可配置）
  - 统计摘要栏：活跃/等待/空闲/总计
  - 会话详情抽屉（侧边栏可调宽度），展示完整 Last Req / Last Resp
- **CLI Dashboard** — `pflow status` 终端文本表格
- **会话探测** — `pflow probe <id>` 单个会话详细状态
- **双 Agent 支持** — Claude Code + Hermes 统一 Dashboard 展示

### 会话管理（亲赴前线）

- **`pflow claude`** — 一键创建 tmux + Claude 托管会话
  - 自动配置 Claude statusline（`~/.claude/settings.json`）
  - 创建 tmux session + 启动 Claude Code
  - 自动提取 8 位 session ID 前缀建立 tmux↔Claude 映射
  - 支持 `-name` / `-dir` / `-force` / `-no-attach` 参数
- **Web 终端集成** — Dashboard 侧边栏嵌入 ttyd Web 终端
  - 通过 statusline 前缀自动关联 tmux session
  - 可在浏览器中直接与 Claude 交互（包括权限确认）
  - Session 映射持久化到 `~/.pflow/mappings.json`

### 部署

- **单二进制部署** — `make build` 将 Vue SPA 通过 `//go:embed` 嵌入 Go binary，`pflow serve` 一个命令启动全部

---

## v0.0.1

阶段一验证通过后的首个版本：

- `pflow probe` — 探测 Claude Code 会话状态
- `pflow status` — CLI 文本表格显示多会话状态
- Dashboard API — HTTP 端点返回所有 session 的 JSON 状态快照
