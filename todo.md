# todo

> 当前周期：阶段三 CLI 能力扩展

## 目标

在已打通的"Dashboard 看到状态 → Web 终端交互"闭环基础上，补齐 CLI 端的独立操作能力，让用户无需浏览器也能完成"赴前线/获取建议/设定焦点"等核心操作。

已完成：`pflow claude`、Web 终端集成、Tmux↔Claude session 映射。详见 [`docs/prd.md`](./docs/prd.md) 阶段三。

## P0 — 核心闭环，必须完成

### P0-1 `pflow attach` — 独立的终端唤醒命令

- [ ] 按 session ID（前缀匹配）查找并 attach 到对应 tmux session
- [ ] 按项目名/路径查找并 attach
- [ ] 如果 session 没有关联的 tmux（非 pflow 托管），给出明确提示
- [ ] 支持 `--list` / `--choose` 交互选择模式（多个匹配时）
- [ ] `pflow attach` 无参数时列出所有当前可 attach 的会话供选择

### P0-2 `pflow focus` — 主攻/侧翼配置

- [ ] `pflow focus --main <session-id>` 设定主攻方向
- [ ] `pflow focus --side <session-id,...>` 设定侧翼战场（逗号分隔）
- [ ] `pflow focus --show` 展示当前焦点配置
- [ ] `pflow focus --clear` 清除所有焦点配置
- [ ] 配置持久化（`~/.pflow/focus.json` 或 TOML）
- [ ] Dashboard API 返回焦点信息，前端高亮显示主攻/侧翼 session

### P0-3 沉默提醒

- [ ] 后台 goroutine 定期扫描活跃 session 的最后活动时间
- [ ] 主攻 session 沉默超过可配置阈值（默认 5min）→ 终端输出通知
- [ ] 通知内容：哪个 session、沉默了多久、当前状态
- [ ] 可配置提醒间隔（避免重复骚扰）
- [ ] `pflow serve` 启动时自动开启（可通过 `--no-watch` 禁用）

## P1 — 体验提升

### P1-1 `pflow suggest` — 军情哨手动触发

- [ ] 分析当前所有会话状态（busy/waiting/idle 分布、等待时间、项目关联）
- [ ] 输出一条引导建议：应该关注哪个 session、为什么
- [ ] 建议维度：等待最久的、主攻方向有更新的、侧翼有新进展的
- [ ] 纯 CLI 文本输出，不依赖 LLM（基于规则引擎）
- [ ] 后续可接入 LLM 做更智能的分析

### P1-2 Shell 补齐脚本

- [ ] Bash 补齐：`pflow <subcommand>` 子命令名补齐
- [ ] `pflow attach <session-id-prefix>` 补齐可用的 session ID
- [ ] `pflow claude -dir <path>` 补齐目录路径
- [ ] Zsh 补齐（可选，优先 Bash）
- [ ] 安装方式：`make install` 或 `pflow completion bash > /etc/bash_completion.d/pflow`

### P1-3 多 Agent 类型启动

- [ ] `pflow start --agent hermes --project X` 启动 Hermes 托管会话
- [ ] 统一 `pflow start` 作为通用启动入口（`pflow claude` 保留为快捷方式）
- [ ] 每种 agent 类型有自己的 tmux + 进程管理逻辑

## P2 — 锦上添花（可延后）

- [ ] **Hermes Last Resp 提取**：接入 `~/.hermes/state.db` SQLite，提取 assistant 回复内容填充 `LastResp`
- [ ] **WebSocket 实时推送**：`GET /api/v1/dashboard/ws`，状态变化时服务端主动推送
- [ ] **浏览器通知**：Session 状态变化时触发 Notification API
- [ ] **会话时间线**：甘特图式的时间分布可视化

## 不包含（本周期）

- 军情哨主动推送（需后台守护进程，留待阶段四）
- 统帅偏好学习（留待阶段四）
- 战局图（留待阶段四）
- TUI Dashboard（留待阶段五）
- 游戏化外壳（留待阶段五）
