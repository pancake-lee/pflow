# todo

> 当前周期：阶段二 Web Dashboard

## 目标

构建基于 **Vue 3 + Naive UI + TypeScript + Vite** 的浏览器端可视化面板，替代终端文本表格，提供真正意义上的"军帐战报"。

## P0 — 核心面板，必须完成

### P0-1 项目初始化

- [x] 用 Vite 创建 Vue 3 + TypeScript 项目（`web/` 目录）
- [x] 安装 Naive UI、配置暗色主题
- [x] 配置与 Go 后端的开发代理（Vite proxy → `localhost:8080`）
- [x] 建立目录结构：`components/`、`composables/`、`types/`、`views/`

### P0-2 Dashboard 主页面

- [x] 会话列表表格（Naive UI DataTable）：
  - 列：Agent 图标 / Session ID / Project / Status（红绿灯） / Name / Last Active / Last Req / Last Resp
  - 红绿灯渲染：🟢 busy/running、🟡 waiting/suspended、⚪ idle、⚫ unknown/completed
  - 相对时间显示（"3m ago"、"1h ago"）
  - 文本截断 + hover 展开 tooltip
- [x] 控制栏（筛选参数）：
  - Time window 选择器（1h / 3h / 6h / 1d / 3d / 7d）
  - Max inactive per project 输入
  - Agent type 过滤（All / Claude / Hermes）
- [x] 自动刷新：可配置间隔（off / 10s / 30s / 60s），轮询 `/api/v1/dashboard`
- [x] 统计摘要栏：活跃数 / 等待数 / 空闲数 / 总计

### P0-3 会话详情

- [x] 点击会话行 → 侧边抽屉显示详情：
  - 完整 Session ID
  - Agent 类型、Platform（Hermes）
  - Status + TrafficLight
  - IsActive 状态
  - 完整 Last Req / Last Resp 文本

## P1 — 体验完善

- [x] Go embed：`//go:embed web/dist/*` 将前端打包进 Go binary，`pflow serve` 单文件部署
- [x] 空状态设计：无 session 时的引导提示
- [x] 错误状态：API 不可用时的 NAlert 提示
- [x] Loading 状态：NSpin 包裹
- [x] Go embed：`//go:embed web/dist/*` 将前端打包进 Go binary，`pflow serve` 单文件部署
- [x] `max_inactive` 排序稳定性：inactive session 按 LastActive 降序排列后再截断，确保输出稳定
- [x] Hermes 红绿灯修复：API `status` 字段使用纯文本（不再含 emoji 前缀），前端统一拼接 `traffic_light + status`
- [x] 状态统一：Claude `unknown` + Hermes `suspended`/`completed` → 统一为 `inactive`（⚫）
- [x] 侧边栏可调宽度：拖拽手柄调整，min 1/4 屏幕、max 3/4 屏幕
- [x] 侧边栏完整内容：新增 `last_req_full`/`last_resp_full` 字段贯穿全链路，抽屉展示完整文本；表格保持 15 字截断
- [x] `max_inactive` 默认值改为 1（CLI + Web 同步）
- [x] `CLAUDE.md` 规则：禁止直接 `go build`，必须通过 `make build`
- [ ] 响应式布局：桌面端为主，平板可用（暂未严格测试）

## P2 — 锦上添花

- [ ] WebSocket 实时推送（替代轮询）
- [ ] Session 状态变化时的浏览器通知（Notification API）
- [ ] 会话时间线可视化（甘特图式的时间分布）
- [ ] 暗色/亮色主题切换

## 不包含（本周期）

- Agent 启动/停止/attach（后端能力，留待后续）
- 军情哨推送（留待阶段三）
- 游戏化外壳（留待阶段四）
- TUI Dashboard（Bubble Tea 方案暂时搁置，Web 面板先行）

## 验证目标

| 指标 | 目标 |
|------|------|
| Dashboard 页面首屏加载 | < 2s |
| 自动刷新延迟（轮询） | 与设定间隔一致 |
| 多 session 展示 | 支持 50+ 条无卡顿 |
| Go embed 后二进制增量 | < 5MB |
| 暗色主题视觉一致性 | Naive UI 暗色主题通过 |
