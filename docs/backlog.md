# backlog

> 全部需求池，按优先级排序。每个周期从池中挑选任务写入 `todo.md`。

## ✅ 已完成

- **阶段一：可行性验证** — 全部验证项通过（详见 [`prd.md`](./prd.md) 第 5 节）
- **阶段二：Web Dashboard** — Vue 3 + Naive UI 浏览器端可视化面板，支持 DataTable 会话列表、红绿灯状态、筛选/排序、自动刷新、侧边栏详情、Go embed 单文件部署
- **`pflow claude` CLI 子命令** — 一键创建 tmux + Claude 托管会话，自动配置 statusline、提取 session 前缀、保存关联映射。支持 `-name` / `-dir` / `-force` / `-no-attach` 参数
- **Web 终端集成（ttyd + tmux）** — 侧边栏通过 ttyd 嵌入 Web 终端，通过 Claude statusline 的 8 位 session ID 前缀关联 tmux↔Claude session，Dashboard 可自动 lookup 并打开终端交互
- **Session 管理与映射持久化** — `internal/session` 包：tmux + ttyd 进程生命周期管理、`~/.pflow/mappings.json` 映射持久化、statusline 自动配置、capture-pane 前缀解析

## P0 — 核心体验，不做产品不完整

- [x] `pflow status` — 状态仪表盘（CLI 文本表格 + Web Dashboard 双模式）
- [x] `pflow probe <id>` — 探测单个 session 详细状态
- [x] `pflow serve` — HTTP Dashboard API + Web Dashboard（单二进制部署）
- [x] `pflow claude` — 启动 Claude Code 托管会话（tmux + statusline）
- [ ] 亲赴前线：`pflow attach <session>` 独立子命令（当前通过 `pflow claude` 自带 attach + Web 终端 ttyd 实现，但缺少独立的 tmux attach 查找子命令）
- [ ] 多 Agent 类型启动：`pflow start --agent hermes --project X`（当前仅支持 Claude）

## P1 — 明显提效，用户高频受益

- [ ] `pflow suggest` — 手动触发军情哨分析建议
- [ ] `pflow focus --main A --side B,C` — 设定主攻/侧翼
- [ ] Agent 沉默超阈值时终端通知
- [ ] Shell 补齐脚本

## P2 — 锦上添花，有余力时做

- [ ] **Hermes Last Resp 提取**：接入 `~/.hermes/state.db` SQLite（纯 Go 驱动如 `modernc.org/sqlite`），查询 messages 表获取 assistant 回复内容，填充 `LastResp` 字段。当前仅从 request_dump body 提取了 `LastReq`。
- [ ] **军情哨主动推送**（定时 + 事件触发，需后台守护进程）
- [ ] **统帅偏好学习**（推送频率自适应）
- [ ] **战局图**：任务依赖关系的可视化建立与阻塞检测
- [ ] TUI Dashboard（Bubble Tea 终端可视化战报）
- [ ] WebSocket 实时推送（替代 Dashboard 轮询）
- [ ] Session 状态变化时的浏览器通知（Notification API）
- [ ] 会话时间线可视化（甘特图式的时间分布）
- [ ] 暗色/亮色主题切换

## P3 — 远期/探索，条件成熟再做

- [ ] 游戏化外壳（战场地图隐喻的视觉包装）
- [ ] VSCode 扩展
- [ ] 跨设备同步（手机/平板看状态、点批准）
- [ ] 多 Agent 类型支持（Cline、Codex CLI 等）
- [ ] 多项目战局图（跨项目任务依赖与资源调度）
