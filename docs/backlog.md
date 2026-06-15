# backlog

> 全部需求池，按优先级排序。每个周期从池中挑选任务写入 `todo.md`。

## ✅ 已完成

- **阶段一：可行性验证** — 全部验证项通过（详见 [`prd.md`](./prd.md) 第 5 节）
- **阶段二：Web Dashboard** — Vue 3 + Naive UI 浏览器端可视化面板，支持 DataTable 会话列表、红绿灯状态、筛选/排序、自动刷新、侧边栏详情、Go embed 单文件部署
- **`pflow claude` CLI 子命令** — 一键创建 tmux + Claude 托管会话，自动配置 statusline、提取 session 前缀、保存关联映射。支持 `-name` / `-dir` / `-force` / `-no-attach` 参数
- **Web 终端集成（ttyd + tmux）** — 侧边栏通过 ttyd 嵌入 Web 终端，通过 Claude statusline 的 8 位 session ID 前缀关联 tmux↔Claude session，Dashboard 可自动 lookup 并打开终端交互
- **Session 管理与映射持久化** — `internal/session` 包：tmux + ttyd 进程生命周期管理、`~/.pflow/mappings.json` 映射持久化、statusline 自动配置、capture-pane 前缀解析

## P0 — 核心体验，不做产品不完整

- [X] `pflow status` — 状态仪表盘（CLI 文本表格 + Web Dashboard 双模式）
- [X] `pflow probe <id>` — 探测单个 session 详细状态
- [X] `pflow serve` — HTTP Dashboard API + Web Dashboard（单二进制部署）
- [X] `pflow claude` — 启动 Claude Code 托管会话（tmux + statusline）
- [X] **项目根标记** — `~/.pflow/project_roots.json` 存储被标记为项目根的路径列表 + 优先级（primary/secondary/normal）。路径即项目，不引入独立 ID/名称实体。
- [X] **Session 自动归类** — 最长前缀匹配算法：session 的 cwd 匹配到某个项目根路径则自动归入。根目录 `/` 禁止标记。未匹配的作为独立 session 展示。
- [X] **"识别为项目" ☐** — Dashboard 界面上每个 distinct 工作目录旁提供勾选框 + hover tooltip，一键标记/取消项目根
- [X] **主线/支线策略引擎** — 设定 1 主线 + 最多 2 支线，数量限制校验，优先级切换规则
- [X] **Dashboard 项目分组视图** — 前端从扁平表格改为按 root 分组的视图，按优先级分区展示（主线/支线/普通/未归类）

## P1 — 明显提效，用户高频受益

- [ ] （优先）**提醒分数算法** — 综合等待时长、专注持续、今日累计、项目优先级计算每个项目的提醒分数。设计文档：[`docs/design/02-reminder_score_algorithm.md`](./design/02-reminder_score_algorithm.md)。保护期 15min 不打扰，幂函数差异化防止多任务同时高亮。
- [ ] （优先）**注意力遮罩层** — 在项目卡片上叠加 `::before` 伪元素半透明遮罩，透明度随提醒分数动态变化。三级提醒（低/中/高），hover 降低遮罩。预留 CSS 变量接口供后续换肤系统。设计文档：[`docs/design/03-attention_mask.md`](./design/03-attention_mask.md)。
- [ ] **桌面通知 + 卡片动画** — 分数超阈值时触发浏览器 Notification API + 卡片呼吸灯/边框闪烁效果
- [ ] **项目折叠/展开** — 记忆用户对普通项目和归档项目的折叠状态
- [ ] Shell 补齐脚本（`pflow` 子命令 + session ID）
- [ ] （优先）tmux定期截图刷新sessionid，/clear和/resume或重启，导致session绑定更换，要同步到页面做重新绑定

## P2 — 锦上添花，有余力时做

- [ ] **Hermes Last Resp 提取**：接入 `~/.hermes/state.db` SQLite，查询 messages 表获取 assistant 回复内容，填充 `LastResp` 字段。当前仅从 request_dump body 提取了 `LastReq`。
- [ ] WebSocket 实时推送（替代 Dashboard 轮询）
- [ ] Session 状态变化时的浏览器通知（Notification API）
- [ ] （优先）军情哨分析建议（`pflow suggest`）
- [ ] （优先）多 Agent 类型启动（`pflow hermes`）
- [ ] TUI Dashboard（Bubble Tea 终端可视化战报）
- [ ] 暗色/亮色主题切换
- [ ] （远期预留）**双层换肤系统** — 内容层 + 遮罩层独立换肤，支持社区皮肤。设计文档已完成（[`docs/design/99-dual_layer_skinning_system.md`](./design/99-dual_layer_skinning_system.md)），短期开发预留 CSS 变量接口即可。
- [ ] （远期预留）**Web AI 平台状态监控** — 浏览器扩展监控 DeepSeek/Kimi/ChatGPT 等平台的 AI 对话状态。设计文档已完成（[`docs/design/99-ai-chat-web-attach.md`](./design/99-ai-chat-web-attach.md)），需浏览器扩展开发 + 后端 WebSocket。

## P3 — 远期/探索，条件成熟再做

- [ ] 军情哨主动推送（需后台守护进程）
- [ ] 统帅偏好学习（推送频率自适应）
- [ ] 战局图：任务依赖关系的可视化建立与阻塞检测
- [ ] 游戏化外壳（战场地图隐喻的视觉包装）
- [ ] VSCode 扩展
- [ ] 跨设备同步（手机/平板看状态、点批准）
- [ ] 多 Agent 类型支持（Cline、Codex CLI 等）
