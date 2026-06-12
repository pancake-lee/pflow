# changelog

> 版本历史，面向用户描述每个版本的新增内容。

## v0.2.0 (当前)

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

## v0.1.0 (阶段一，已被 v0.2.0 取代)

阶段一验证通过后的首个版本：

- `pflow probe` — 探测 Claude Code 会话状态
- `pflow status` — CLI 文本表格显示多会话状态
- Dashboard API — HTTP 端点返回所有 session 的 JSON 状态快照
