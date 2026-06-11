# note

> 活跃技术备忘：当前在用的实现细节、调试技巧、临时方案、已知问题。

## 当前周期（阶段一 可行性验证）

### Hermes Agent 集成可行性（2026-06-11）

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

**当前活跃 session**（2026-06-11）：
- `20260611_144133_0d5a2c1e` — weixin 渠道，status: running
- `20260611_135406_b0a840` — cli 渠道，status: running

**实现策略**：阶段一采用文件系统扫描（与 Claude Code 方案一致），直接读取 `sessions.json` + SQLite。ACP 协议对接留作后续——当需要管理 Hermes session（启动/停止/attach）时再接入。
