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
- [ ] **项目数据模型** — `~/.pflow/projects.json`：项目定义（id/name/path/priority），CRUD API
- [ ] **Session ↔ Project 关联** — session 增加 `projectId` 字段，支持按工作目录自动归类；未关联的归入"未分类"项目
- [ ] **主线/支线策略引擎** — 设定 1 主线 + 最多 3 支线，数量限制校验，优先级切换规则
- [ ] **Dashboard 项目视图重构** — 前端从扁平表格改为"项目卡片 → session 列表"两级结构，按优先级分区展示（主线/支线/普通/归档）
- [ ] **历史数据迁移** — 自动将现有 session 按工作目录归类到同名项目

## P1 — 明显提效，用户高频受益

- [ ] **沉默提醒** — 主线项目 Agent 沉默超阈值时终端通知（可配置阈值 + 提醒间隔）
- [ ] **项目手动排序** — 拖拽或按钮调整支线/普通项目的显示顺序
- [ ] **项目折叠/展开** — 记忆用户对普通项目和归档项目的折叠状态
- [ ] Shell 补齐脚本（`pflow` 子命令 + session ID）

## P2 — 锦上添花，有余力时做

- [ ] **Hermes Last Resp 提取**：接入 `~/.hermes/state.db` SQLite，查询 messages 表获取 assistant 回复内容，填充 `LastResp` 字段。当前仅从 request_dump body 提取了 `LastReq`。
- [ ] WebSocket 实时推送（替代 Dashboard 轮询）
- [ ] Session 状态变化时的浏览器通知（Notification API）
- [ ] 军情哨分析建议（`pflow suggest`）
- [ ] 多 Agent 类型启动（`pflow start --agent hermes`）
- [ ] TUI Dashboard（Bubble Tea 终端可视化战报）
- [ ] 暗色/亮色主题切换

## P3 — 远期/探索，条件成熟再做

- [ ] 军情哨主动推送（需后台守护进程）
- [ ] 统帅偏好学习（推送频率自适应）
- [ ] 战局图：任务依赖关系的可视化建立与阻塞检测
- [ ] 游戏化外壳（战场地图隐喻的视觉包装）
- [ ] VSCode 扩展
- [ ] 跨设备同步（手机/平板看状态、点批准）
- [ ] 多 Agent 类型支持（Cline、Codex CLI 等）
