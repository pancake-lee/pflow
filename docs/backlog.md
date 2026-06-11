# backlog

> 全部需求池，按优先级排序。每个周期从池中挑选任务写入 `todo.md`。

## P0 — 核心体验，不做产品不完整

- ✅ 阶段一全部验证项（详见 [`prd.md`](./prd.md) 第 5 节）— 已完成
- **→ Web Dashboard（浏览器访问，适合挂副屏）** ← 当前阶段，从 P3 提前
- Agent 会话启动：`pflow start --project X --task "..."` 在后台启动 Claude Code
- 状态仪表盘：`pflow status` 红绿灯战况表
- 亲赴前线：`pflow attach <session>` 唤起终端进入 Agent 会话
- 会话持久化：断开终端后 Agent 继续后台工作

## P1 — 明显提效，用户高频受益

- `pflow suggest` 手动触发军情哨分析建议
- `pflow focus --main A --side B,C` 设定主攻/侧翼
- Agent 沉默超阈值时终端通知
- Shell 补齐脚本

## P2 — 锦上添花，有余力时做

- **Hermes Last Resp 提取**：接入 `~/.hermes/state.db` SQLite（纯 Go 驱动如 `modernc.org/sqlite`），查询 messages 表获取 assistant 回复内容，填充 `LastResp` 字段。当前仅从 request_dump body 提取了 `LastReq`。
- 军情哨主动推送（定时 + 事件触发，需后台守护进程）
- 统帅偏好学习（推送频率自适应）
- 战局图：任务依赖关系的可视化建立与阻塞检测
- TUI Dashboard（Bubble Tea 终端可视化战报）

## P3 — 远期/探索，条件成熟再做

- 游戏化外壳（战场地图隐喻的视觉包装）
- VSCode 扩展
- 跨设备同步（手机/平板看状态、点批准）
- 多 Agent 类型支持（Cline、Codex CLI 等）
- 多项目战局图（跨项目任务依赖与资源调度）
