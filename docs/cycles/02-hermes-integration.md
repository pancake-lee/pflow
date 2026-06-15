# Hermes Agent 集成调研

> 周期：2026-06-11 | 状态：✅ 已完成调研，Last Resp 待实现

## 集成可行性

**结论：可行。** Hermes 支持 ACP 协议，且本地有丰富的可读数据。

**ACP 协议支持**：
- `hermes acp --check` 通过（v0.14.0，与 Claude Code 使用相同的 Agent Communication Protocol）
- `hermes acp` 以 stdio JSON-RPC 模式启动，供编辑器集成（VS Code、Zed、JetBrains）
- cc-connect 的 `agent/acp/` 包已实现完整的 ACP 适配器（session 管理、权限处理、RPC 通信）

**本地数据源**：
| 数据源 | 路径 | 内容 |
|--------|------|------|
| sessions.json | `~/.hermes/sessions/sessions.json` | 当前活跃 session（session_key → 元数据） |
| state.db | `~/.hermes/state.db` | SQLite：90 sessions、3203 messages、token 统计 |
| gateway_state.json | `~/.hermes/gateway_state.json` | Gateway 平台连接状态（weixin 等） |
| request_dump | `~/.hermes/sessions/request_dump_*.json` | 原始 API request/response JSONL |

**实现策略**：阶段一采用文件系统扫描（与 Claude Code 方案一致），直接读取 `sessions.json` + request_dump 文件。ACP 协议对接留作后续——当需要管理 Hermes session（启动/停止/attach）时再接入。

## 元数据字段参考

### sessions.json（gateway 管理，实时）

| 字段 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `session_key` | string | 全局唯一键，编码 agent/channel/chat | `agent:main:weixin:dm:o9cq...@im.wechat` |
| `session_id` | string | 会话短 ID | `20260611_144133_0d5a2c1e` |
| `created_at` | time (local, no tz) | 创建时间 | `2026-06-11T14:41:33.639638` |
| `updated_at` | time (local, no tz) | 最后活跃时间 | `2026-06-11T15:11:17.276941` |
| `display_name` | string/null | 显示名称（可手动设置） | `null` |
| `platform` | string | 平台 | `weixin` / `cli` |
| `chat_type` | string | 聊天类型 | `dm` / `group` |
| `origin.platform` | string | 消息来源平台 | `weixin` |
| `origin.chat_name` | string/null | 渠道名称 | `null` |
| `origin.user_name` | string | 用户标识 | `o9cq803...@im.wechat` |
| `origin.chat_topic` | string/null | 群聊主题 | `null` |
| `input_tokens` | int | 累计输入 token | `0`（gateway session 不在此累积） |
| `output_tokens` | int | 累计输出 token | `0` |
| `total_tokens` | int | 累计总 token | `0` |
| `last_prompt_tokens` | int | 最近一次 prompt token 数 | `44169` |
| `estimated_cost_usd` | float | 预估费用 | `0.0` |
| `suspended` | bool | 是否挂起 | `false` |
| `resume_pending` | bool | 是否等待恢复 | `false` |

### request_dump（事后快照，每次 API 调用一个文件）

| 字段 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `timestamp` | time | 快照时间 | `2026-06-11T14:41:42` |
| `session_id` | string | 所属 session | `20260611_144133_0d5a2c1e` |
| `reason` | string | 快照原因 | `max_retries_exhausted` / `non_retryable_client_error` |
| `request.method` | string | HTTP 方法 | `POST` |
| `request.url` | string | API 端点 | `https://api.kimi.com/coding/chat/completions` |
| `request.body.model` | string | 使用的模型 | `kimi-for-coding` |
| `request.body.system` | string | **系统 prompt（含 cwd）** | `Current working directory: /root/code/pancake` |
| `request.body.messages` | array | 对话消息 | 最后一条为用户消息 |
| `error` | object/null | 错误信息 | `{"type":"invalid_request_error",...}` |

### gateway_state.json（gateway 进程状态）

| 字段 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `pid` | int | gateway 进程 PID | `14` |
| `gateway_state` | string | gateway 状态 | `running` / `stopped` |
| `active_agents` | int | 活跃 agent 数 | `0` |
| `platforms.<name>.state` | string | 各平台连接状态 | `connected` / `paused` |
| `platforms.<name>.error_message` | string/null | 平台错误信息 | `null` |

### cron jobs（`cron/jobs.json`）

| 字段 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `id` | string | cron job ID | `3910e8758c9b` |
| `name` | string | 任务名称 | `knowledge-nudge-daily` |
| `schedule.kind` | string | 调度类型 | `cron` |
| `schedule.expr` | string | cron 表达式 | `0 9 * * *` |
| `enabled` | bool | 是否启用 | `true` |
| `workdir` | string/null | **工作目录（可配置）** | `null`（当前未设置） |
| `origin.platform` | string | 投递平台 | `weixin` |
| `last_run_at` | time | 上次运行时间 | `2026-06-11T09:00:55+08:00` |
| `last_status` | string | 上次状态 | `error` / `success` |

### 关键结论

- Hermes **没有**原生的 cwd/project 字段（sessions.json 不含、无 `.hermes/config` project 设置）
- **cwd 的唯一可靠来源**：request_dump 文件内 system prompt 中的 `Current working directory: <path>` 行
  - CLI session → 真实项目目录（如 `/root/code/pancake`）✅ 已实现提取
  - weixin/cron session → `/`（无意义，回退 platform 名）✅ 已实现
- cron job 配置中 `workdir` 字段为 `null`（当前未使用），后续若 cron 配置了 workdir 可优先使用
- assistant 回复内容在 `state.db` SQLite 中（`messages` 表），Last Resp 暂未接入（见 backlog P2）
