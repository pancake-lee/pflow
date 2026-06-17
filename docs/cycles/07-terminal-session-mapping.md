# pflow 会话容器方案（基于 tmux）

> 本文档描述 pflow 如何利用 tmux 作为会话容器，实现 Agent 进程持久化、Session ID 映射、Web 终端集成和自动销毁等核心功能。经过对 abduco、dtach、shpool 等替代工具的全面评估，确认 tmux 是唯一能够同时满足 pflow 所有技术需求的方案。

## 一、背景与需求

pflow 需要一个会话容器来托管 CLI Agent（Claude Code、Hermes 等），需满足以下需求：

| 需求                 | 说明                                                    |
| -------------------- | ------------------------------------------------------- |
| **持久化**     | 用户断开 SSH 后，Agent 进程继续在后台运行               |
| **多端同步**   | CLI 和 Web Dashboard 可同时附着到同一会话，操作实时同步 |
| **屏幕恢复**   | 新客户端附着时能看到完整的当前屏幕内容（包括历史输出）  |
| **程序化控制** | pflow 后端能向容器发送命令（如 `/status`）并读取输出  |
| **Web 终端**   | 在浏览器中嵌入终端界面，支持查看和输入                  |
| **自动销毁**   | Agent 退出后容器自动销毁，不留残留 shell                |
| **无感体验**   | 用户不需要学习 tmux 的复杂操作                          |

## 二、替代工具评估结论

在最终确定方案前，对候选工具进行了全面评估：

| 工具             | 屏幕恢复          | 程序化截屏         | 外部输入        | 多客户端    | 持久化 | 综合结论                     |
| ---------------- | ----------------- | ------------------ | --------------- | ----------- | ------ | ---------------------------- |
| **tmux**   | ✅                | ✅`capture-pane` | ✅`send-keys` | ✅          | ✅     | **唯一可行**           |
| **abduco** | ❌ 新客户端空白   | ❌                 | ❌              | ✅          | ✅     | 无法满足屏幕恢复和程序化控制 |
| **dtach**  | ❌ 新客户端空白   | ❌                 | ❌              | ⚠️ 不稳定 | ✅     | 同上                         |
| **shpool** | ❌ 不支持多客户端 | ❌                 | ❌              | ❌          | ✅     | 多客户端不支持               |

**关键发现**：

- abduco/dtach 不维护屏幕缓冲区，新客户端附着后看到空白屏幕，用户无法获知当前 Agent 状态（如权限提示），导致"盲操作"，认知成本远高于 tmux 的复制粘贴问题。
- shpool 明确不支持多客户端同时附着，无法实现 CLI 和 Web 同步。
- 因此，**tmux 是 pflow 的唯一可行选择**。后续所有方案均基于 tmux 构建。

## 三、整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│  pflow CLI / Dashboard                                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐   │
│  │ pflow claude │  │ pflow hermes│  │ pflow session list │   │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘   │
│         │                │                     │               │
└─────────┼────────────────┼─────────────────────┼───────────────┘
          │                │                     │
          ▼                ▼                     ▼
┌─────────────────────────────────────────────────────────────────┐
│  pflow 核心层                                                   │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  TmuxBackend (统一封装)                                  │  │
│  │  - CreateSession(name, cmd)                             │  │
│  │  - SendKeys(session, keys)                             │  │
│  │  - CapturePane(session, lines) → string                │  │
│  │  - DestroySession(session)                             │  │
│  │  - SessionExists(session) → bool                       │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Agent 适配层                                            │  │
│  │  - ClaudeAdapter: 扫描 JSON 目录 + 监控变化              │  │
│  │  - HermesAdapter: 发送 /status + 截屏解析                │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  映射存储 (mappings.json)                                │  │
│  │  - containerName ↔ agentType + agentSessionId + name    │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────────┐
│  tmux 会话层                                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐   │
│  │ pflow-xxx   │  │ pflow-yyy   │  │ ...                 │   │
│  │ (运行claude) │  │ (运行hermes)│  │                     │   │
│  └─────────────┘  └─────────────┘  └─────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## 四、Session ID 与名称绑定机制

### 4.1 Claude Code

**原理**：

- Claude 支持 `-n <name>` 参数，设置一个稳定名称，该名称写入 `~/.claude/sessions/<pid>.json` 的 `name` 字段。
- 同一会话中，执行 `clear` 或 `resume` 后，`sessionId` 会变化，但 `name` 保持不变。
- 通过扫描 `~/.claude/sessions/` 目录下所有 JSON 文件，匹配 `name` 字段，即可获取最新的 `sessionId`。

**启动流程**：

1. 用户执行 `pflow claude start --name my-project --dir /path`
2. pflow 创建 tmux 会话，会话名为 `pflow-claude-my-project`
3. 在 tmux 中执行：`claude -n my-project`
4. 轮询扫描 `~/.claude/sessions/` 目录下的所有 JSON 文件（每秒一次，最多 10 秒）：
   - 解析每个文件的 `name` 字段
   - 若找到 `name == "my-project"`，读取对应的 `sessionId`
5. 建立映射：`containerName → { agentType: "claude", agentName: "my-project", agentSessionId: "xxxx" }`
6. 映射持久化到 `~/.pflow/mappings.json`

**变化更新**：

- 轮询（每 5 秒）监控 `~/.claude/sessions/` 目录的文件变化，扫描更新时间。
- 当有新文件生成或现有文件更新时，重新读取 JSON 文件。
- 若 `name` 匹配且 `sessionId` 与当前记录不同，则自动更新映射。
- `name` 保持不变，用户无感。
- JSON 文件是以PID命名，而PID和tmux是稳定的映射，所以也可以记录到mappings.json
  - PID可以参与映射变更判断，以防claude有未摸清楚的name和session_id行为，或者更新后行为改变
  - 则 tmux -> pid -> name 这个关系变化时，我们认为是意外情况，输出错误日志

**保留原方案****：**

    原方案使用statusline输出session_id前8位为前缀，然后通过tmux截屏解析出来，再建立映射。

    这份代码我希望保留，通过一个代码开关来切换，实现新方案后，开关指向新方案即可。

### 4.2 Hermes

**原理**：

- Hermes 没有类似 `-n` 的参数，无法启动时就设置好名字
- 虽然交互界面中，`/title`可以修改名字，最后将出现在 `hermes sessions list` 的 Title 列
  - 但是 `hermes sessions list` 在用户发送第一个消息前，不会列出这个session，所以走不通
- 但在交互界面中，`/status` 命令会输出当前会话的 `Session ID: xxxxxxxx-xxxx-...`。
- 通过 `tmux send-keys` 发送 `/status`，然后 `tmux capture-pane` 截屏解析。

**启动流程**：

1. 用户执行 `pflow hermes start --name my-session --dir /path`
2. pflow 创建 tmux 会话，会话名为 `pflow-hermes-my-session`
3. 在 tmux 中执行：`hermes`
4. 等待 3 秒，确保 Hermes 初始化完成
5. 发送 `/status`：`tmux send-keys -t pflow-hermes-my-session "/status" Enter`
6. 等待 1 秒，截屏最后 30 行：`tmux capture-pane -t pflow-hermes-my-session -p -S -30`
7. 用正则提取 `Session ID: ([a-f0-9-]+)`
8. 建立映射：`containerName → { agentType: "hermes", agentName: "my-session", agentSessionId: "xxxx" }`
9. 映射持久化

**名称更新**：

- 用户可通过 `/title new-name` 修改 Hermes 会话名称，不需要处理这种情况。
- pflow 可定期（如每 60 秒）或由用户主动触发（`pflow session refresh --name my-session`）重新执行上述流程，更新 `agentName` 字段。

**交互界面内 `/status`命令输出参考：**

```txt
⚙️  /status
Hermes CLI Status

Session ID: 20260617_112541_649f4d
Path: ~/.hermes
Model: deepseek-v4-flash (deepseek)
Created: 2026-06-17 11:25
Last Activity: 2026-06-17 11:25
Tokens: 0
Agent Running: No

Session recap — 20260617
  (nothing to recap — no messages yet)
```

### 4.3 映射数据结构

```json
// ~/.pflow/mappings.json
{
  "sessions": [
    {
      "containerName": "pflow-claude-my-project",
      "agentType": "claude",
      "agentName": "my-project",
      "agentSessionId": "cfe9d4d9-cc15-4d4a-b646-314262c3cc33",
      "workDir": "/root/code/pflow",
      "createdAt": "2025-06-17T10:00:00Z",
      "lastUpdated": "2025-06-17T12:30:00Z",
      "status": "active"
    },
    {
      "containerName": "pflow-hermes-my-session",
      "agentType": "hermes",
      "agentName": "my-session",
      "agentSessionId": "abc12345-...",
      "workDir": "/root/code/pflow",
      "createdAt": "2025-06-17T11:00:00Z",
      "lastUpdated": "2025-06-17T11:30:00Z",
      "status": "active"
    }
  ]
}
```

## 五、Web 终端集成

### 5.1 实现方式

Web 终端通过 `ttyd` 实现：

```bash
ttyd -p <port> tmux attach -t <containerName>
```

用户访问 `http://<host>:<port>` 即可看到完整的 tmux 会话界面。

### 5.2 多客户端同步

tmux 原生支持多客户端附着：

- CLI 用户：`tmux attach -t <containerName>`
- Web 用户：通过 ttyd 附着到同一会话
- 所有客户端共享同一屏幕视图，输入同步

### 5.3 屏幕恢复

tmux 维护独立的屏幕缓冲区，新客户端附着时自动显示当前屏幕内容（包括历史输出），用户可立即了解 Agent 状态（如权限提示、等待输入等）。这确保了 CLI 和 Web 体验的一致性。

## 六、自动销毁机制

### 6.1 核心命令

```bash
tmux new-session -d -s <containerName> \
  "trap '' TSTP; <agent_cmd>; tmux kill-session -t <containerName>"
```

- `trap '' TSTP`：阻止用户通过 `Ctrl+Z` **挂起** Agent 回到 shell
- Agent 正常退出后，执行 `tmux kill-session` 销毁容器
- 无残留 shell，用户无法在 Agent 退出后执行其他命令

### 6.2 异常处理

| 场景                 | 处理                        |
| -------------------- | --------------------------- |
| Agent 启动失败       | 容器立即销毁，返回错误      |
| 用户 `Ctrl+C`      | Agent 退出 → 容器销毁      |
| 用户 `Ctrl+Z`      | 被 `trap` 阻止，无法挂起  |
| 网络断开（SSH 中断） | tmux 会话独立运行，不受影响 |
| pflow 服务重启       | 映射文件保留，可重新读取    |

### 6.3 清理孤儿会话

pflow 启动时扫描所有 tmux 会话，如果会话名匹配 `pflow-*` 前缀但在映射文件中找不到对应记录，或对应进程已不存在，则自动销毁。

## 七、用户交互

### 7.1 启动 Agent

```bash
# 启动 Claude
pflow claude start --name my-project --dir /path

# 启动 Hermes
pflow hermes start --name my-session --dir /path
```

注意，`pflow claude`和 `pflow hermes`拥有默认值，提供快速进入的命令

### 7.2 查看会话

```bash
pflow session list
# 输出：
#   Container        Agent    Name         Session ID                        Status
#   pflow-claude-..  claude   my-project   cfe9d4d9-cc15-...                 active
#   pflow-hermes-..  hermes   my-session   abc12345-...                      active
```

### 7.3 Web Dashboard

- 显示所有会话，按项目分组
- 每个会话卡片显示 Agent 类型、名称、Session ID（截断）、状态、最后活跃时间
- 点击"Open Terminal"打开 Web 终端（ttyd 嵌入）

  以上内容其实都有了，而claude的name字段可能需要优化传递的过程

### 7.4 销毁会话

```bash
pflow session destroy <containerName>
```

## 八、优势与局限

### 优势

| 特性                 | 说明                                           |
| -------------------- | ---------------------------------------------- |
| **屏幕恢复**   | Web 端附着后立即看到当前状态，无需猜测         |
| **多端同步**   | CLI 和 Web 操作实时同步，体验一致              |
| **程序化控制** | 支持 capture-pane 和 send-keys，实现自动化映射 |
| **自动销毁**   | Agent 退出后容器自动清理，无残留               |
| **无感封装**   | 用户通过 pflow 命令操作，无需了解 tmux         |

### 已知局限

| 局限                           | 说明                            | 缓解措施                                     |
| ------------------------------ | ------------------------------- | -------------------------------------------- |
| **Windows SSH 复制粘贴** | tmux 下右键粘贴不稳定           | 提供配置指南（`Shift` 绕过法），或辅助命令 |
| **依赖 tmux**            | 用户需安装 tmux                 | 大多数 Linux 发行版已预装，提供安装指南      |
| **Hermes 映射依赖截屏**  | 需要稳定的 `/status` 输出格式 | 若格式变化，可更新解析正则                   |

## 九、决策记录

### 9.1 为什么选择 tmux 而非替代工具

| 替代工具         | 失败原因                                                   |
| ---------------- | ---------------------------------------------------------- |
| **abduco** | 不支持屏幕恢复（新客户端空白），不支持程序化截屏和外部输入 |
| **dtach**  | 同上，且多客户端附着不稳定                                 |
| **shpool** | 不支持多客户端同时附着                                     |

**核心结论**：唯一能够同时满足"屏幕恢复 + 程序化控制 + 多客户端同步"的工具是 tmux。

### 9.2 关于复制粘贴问题的决策

不回避 tmux 在 Windows SSH 下的复制粘贴问题，而是通过以下方式应对：

1. 提供详细的配置指南（`~/.tmux.conf`、Windows Terminal 设置）
2. 文档强调 `Shift` 键绕过 tmux 的方法
3. 在 pflow 中封装辅助命令

**决定**：保留 tmux，优化体验，而非替换工具。

## 十、附录

### A. tmux 推荐配置（针对 Windows SSH）

```bash
# ~/.tmux.conf
set -g mouse off
set -g default-terminal "screen-256color"
set -g status-keys vi
set -g mode-keys vi
set -g history-limit 10000
```

### B. Windows Terminal 配置建议

```json
// settings.json
{
  "profiles": {
    "defaults": {
      "supportsApplicationEscapeSequences": false
    }
  },
  "interaction": {
    "mouse": {
      "pasteOnRightClick": true
    }
  }
}
```
