# Session ID 与 Tmux 管理逻辑

> 本文档梳理 pflow 中 Claude Session ID、Tmux Session、以及二者映射关系的完整生命周期。

---

## 1. 三套标识符

pflow 中存在三套不同层级的"会话标识符"：

| 层级 | 示例 | 来源 | 存储位置 |
|------|------|------|----------|
| **Claude Session ID** | `3ca06c7d-xxxx-xxxx-xxxx-xxxxxxxxxxxx` | Claude Code 内部生成 | `~/.claude/sessions/<pid>.json` 的 `sessionId` 字段 |
| **Claude Prefix (8 字符)** | `3ca06c7d` | 从 Claude Session ID 截取前 8 位 | 在 Claude statusline 中显示；存入 `~/.pflow/mappings.json` |
| **Tmux Session Name** | `pflow-myproject` | pflow 从 workDir 的目录名派生 | tmux 自身管理 (`tmux ls` 可见)；在 pflow 内存中 |

三者的关系：

```
Claude Session ID (全量)
    │
    ├──[取前 8 字符]──▶ Claude Prefix (3ca06c7d)
    │                        │
    │                        └──[mappings.json 持久化]──▶ Tmux Session Name (pflow-myproject)
    │
    └──[claude.Scan() 聚合]──▶ 仪表盘展示 (截断为 8 字符)
```

---

## 2. Tmux Session Name 的命名规则

### 2.1 命名逻辑（`internal/session/manager.go`）

```
用户指定的 name（或 workDir 的 base name）
    │
    ▼
sanitizeName():
    - 转小写
    - 只保留 [a-z0-9._-]，其余替换为 `-`
    - 去掉首尾的 `-`
    - 如果结果为空 → 使用 "pflow"
    - 前缀添加 "pflow-"
    │
    ▼
uniqueName():
    - 如果 "pflow-foo" 已被占用 → "pflow-foo-1"
    - 仍然被占用 → "pflow-foo-2" 以此类推
```

### 2.2 设计意图

- **Tmux 名字以目录为标识，不以 Claude Session ID 为标识**。因为 tmux session 是持久化的终端环境，用户可能 attach/detach 多次，Claude 可以重启（产生新的 Session ID），但终端环境不变。
- `pflow-` 前缀用于命名空间隔离，避免与用户手动创建的 tmux session 冲突。
- 唯一性后缀（`-1`, `-2`）仅在同一个 Manager 实例的内存中检查，不跨进程。因此如果两个 pflow server 进程分别对同一目录调用 `Start()`，可能产生同名冲突——实际场景中不会发生，因为只有一个 server 进程。

---

## 3. Claude Prefix (8 字符) —— 桥接层

### 3.1 为什么是 8 字符

- Claude 的 statusline 中天然展示 `sid8`（前 8 位 session ID），这是 Claude Code 已有的 UI 惯例。
- 8 字符 hex 提供 32-bit 空间（约 43 亿种可能），在同一台机器上发生冲突的概率可忽略。
- 无需引入额外的短 ID 生成机制——直接从 Claude 的数据中提取。

### 3.2 Prefix 的捕获（`internal/session/claude.go:captureClaudePrefix`）

```
1. 等 Claude 启动后，用 tmux capture-pane -t <session>:0.0 -p 抓取终端内容
2. 用正则 `^\s*([a-f0-9]{8})\s*[| ]` 匹配 statusline 第一列
3. 每 500ms 重试，最长等 10 秒
4. 成功 → 返回 prefix；超时 → 返回空字符串
```

### 3.3 Statusline 的配置（`internal/session/claude.go:setupStatusline`）

- pflow 会修改 `~/.claude/settings.json`，写入一个 `statusLine.type = "command"` 的配置
- 该 command 从 Claude 提供的 stdin JSON 中提取 `session_id`，截取前 8 位显示
- 配置 statusline **必须先于 Claude 启动**——Claude 在启动时读取 settings.json，启动后修改不生效
- 如果已有不同的 statusline 配置，默认报错拒绝覆盖；`-force` 标志可强制覆盖
- pflow简单判断了 command 包含`sid8`和`session_id`则认为配置正确了
  - 因为用户可能有自己的配置，所以只能让用户自己保证
  - 如果无法获取 statusline 的`sid8`，后果是网页上无法直接打开终端进行操作

**状态检查逻辑：**

| 状态 | 含义 | 行为 |
|------|------|------|
| `StatuslineOK` | 已配好，包含 sid8 | 无需操作 |
| `StatuslineNotSet` | 没有 statusline 配置 | 直接写入 |
| `StatuslineDifferent` | 有 statusline 但不是 pflow 的 | 报错（除非 `-force`） |

---

## 4. 完整的生命周期

### 4.1 创建：`pflow claude`

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. 解析参数：-name, -dir, -force, -no-attach                     │
│ 2. sanitizeName(workDir) → "pflow-myproject"                    │
│ 3. uniqueName() → 确保不重名                                      │
│ 4. 检查是否已有同名 tmux session：                                 │
│    - 有 client 连接 → 拒绝（提示用 -name 或手动 attach）            │
│    - 无 client（孤儿 session）→ 复用，不重启 Claude              │
│ 5. setupStatusline(force) → 配置 ~/.claude/settings.json         │
│ 6. tmux new-session -d -s <name> -c <workDir>                   │
│ 7. tmux send-keys "cd <workDir> && claude --permission-mode     │
│    acceptEdits" C-m                                              │
│ 8. [异步 goroutine] captureClaudePrefix(10s) → 成功后保存 mapping │
│ 9. 返回 Session 结构（Name + WorkDir）                            │
│10. tmux attach -t <name>（除非 -no-attach）                       │
└─────────────────────────────────────────────────────────────────┘
```

**关键时序：** 步骤 8 是异步的，步骤 9-10 不等待 prefix 捕获完成。这意味着：
- 用户立即可以 attach 到 tmux 开始使用 Claude
- 但前端"打开终端"按钮可能在最初几秒内不可用（mapping 尚未写入）

### 4.2 发现：`pflow status` / Dashboard API

```
┌─────────────────────────────────────────────────────────────────┐
│ 数据源：                                                          │
│   ~/.claude/sessions/<pid>.json   → SessionMeta (sessionId,      │
│                                      cwd, status, pid, etc.)     │
│   ~/.claude/history.jsonl         → HistoryEntry (display,       │
│                                      timestamp, project,         │
│                                      sessionId)                  │
│   ~/.claude/projects/<proj>/<sid>.jsonl → 最后一条 user/assistant│
│                                           消息文本               │
│                                                                  │
│ 聚合逻辑（claude.Scan() → aggregate()）：                         │
│   按 sessionId 全量聚合以上三个数据源                              │
│   状态推断：meta.status (busy/waiting/idle) 直接来自 Claude 的     │
│   会话元数据文件                                                  │
│                                                                  │
│ API 层展示：                                                      │
│   - SessionID 截断为 8 字符 (truncate8)                          │
│   - 交叉引用 mappings.json，标注 has_terminal / terminal_tmux_name│
└─────────────────────────────────────────────────────────────────┘
```

### 4.3 终端查找：Web UI 的"打开终端"

```
用户点击某个 Session → 前端调用 GET /api/v1/terminal/lookup?session_id=<full-id>

┌─────────────────────────────────────────────────────────────────┐
│ 1. 取 session_id 的前 8 字符作为 prefix                            │
│ 2. 查询 mappings.json 中 claude_prefix 匹配的记录                  │
│ 3. 无匹配 → 返回 found=false + hint                              │
│ 4. 有匹配 → 检查 tmux session 是否还活着                           │
│ 5. 尝试 live verify：capture-pane 抓取当前 statusline 的 prefix     │
│    - 匹配成功 → Verified=true                                    │
│    - 匹配失败（如 Claude 正在 auth 模式）→ Verified=false + Warning │
│ 6. 返回 tmux 连接信息（名称、目录、ttyd URL 等）                     │
└─────────────────────────────────────────────────────────────────┘
```

### 4.4 清理

- **mapping 清理**：`cleanStale()` 在 `LoadMappings()` 时自动执行，移除 tmux session 已不存在的映射
- **tmux session 清理**：`Manager.Stop()` 只杀 ttyd 进程，**不杀** tmux session（用户可能想手动 reattach）
- **孤儿 session 复用**：`StartClaudeSession` 检测到无 client 的 tmux session 时会复用，而不是创建新的

---

## 5. Mappings 持久化

### 5.1 存储格式（`~/.pflow/mappings.json`）

```json
{
  "version": 1,
  "mappings": [
    {
      "tmux_name": "pflow-myproject",
      "work_dir": "/home/user/code/myproject",
      "claude_prefix": "3ca06c7d",
      "created_at": "2026-06-16T10:30:00Z"
    }
  ]
}
```

### 5.2 查找方向

| 查找方式 | 用途 | 实现 |
|----------|------|------|
| `findByTmuxName(name)` | 检查某个 tmux 是否已有映射 | 遍历 + 精确匹配 |
| `findByClaudePrefix(prefix)` | 通过 Claude 8 位 prefix 反查 tmux | 遍历 + 精确匹配（可能返回多条） |
| `findByWorkDir(dir)` | 检查某个目录是否已有映射 | 遍历 + 精确匹配 |

### 5.3 UPSERT 语义

`addMapping()` 对同一 `tmux_name` 执行 upsert：
- 如果已存在同名记录 → 删除旧记录，写入新记录
- 同一 tmux session 的 Claude 重启（产生新 session ID）→ prefix 自动更新

---

## 6. 边界情况与容错

### 6.1 Claude 重启

- Claude 进程退出后，tmux session 中的 shell 仍在运行
- 用户在 tmux 中重新运行 `claude` 命令，产生新的 Session ID
- 如果 statusline 正确配置，新的 8-char prefix 会出现在终端中
- **但**：`captureClaudePrefix` 只在 `pflow claude` 创建时异步执行一次，不会持续监控
- 因此 restart 后的新 prefix **不会**自动更新到 mappings.json
- 前端 lookup 的 live verify 会发现 prefix 不匹配，返回 `Verified=false`

### 6.2 Claude 处于 Auth 模式

- 首次运行 Claude 时需要浏览器认证
- 此时 statusline 可能不渲染（Claude 尚未进入正常的 REPL 循环）
- `captureClaudePrefix` 在 10 秒内捕获不到 prefix → 返回空字符串
- mapping **不会**被保存
- 用户完成认证后，Claude 进入正常模式，statusline 开始渲染
- 但此时已错过捕获窗口，mapping 永久缺失
- **解决方案（TODO）**：可考虑在 lookup 时做一次"迟到捕获"

### 6.3 孤儿 Tmux Session

- 场景：pflow claude 启动后，用户 detach 然后 tmux session 因某种原因变成无 client 状态
- `StartClaudeSession` 检测到同名 tmux 存在 + 无 client → 复用该 session，不重启 Claude
- 如果有 client（用户或其他程序 attached）→ 拒绝操作

### 6.4 同名目录冲突

- `uniqueName()` 基于内存中的 `m.sessions` map 去重
- 如果两个不同的目录恰好 sanitize 后同名，第二个会得到 `-1` 后缀
- 示例：`/home/user/code/my-project` 和 `/home/user/code/my_project` → `pflow-my-project` 和 `pflow-my-project-1`

---

## 7. 关键代码路径速查

| 功能 | 文件 | 关键函数 |
|------|------|----------|
| Tmux session 创建与管理 | `internal/session/manager.go` | `Start()`, `StartExisting()`, `Stop()`, `ensureTmux()` |
| Tmux 命名规则 | `internal/session/manager.go` | `sanitizeName()`, `uniqueName()` |
| Claude session 启动（含 statusline） | `internal/session/claude.go` | `StartClaudeSession()` |
| Statusline 配置与检查 | `internal/session/claude.go` | `checkStatusline()`, `setupStatusline()` |
| Prefix 捕获（capture-pane） | `internal/session/claude.go` | `captureClaudePrefix()` |
| Prefix → Tmux 查找 | `internal/session/claude.go` | `LookupByClaudeSessionID()` |
| Mapping 持久化 | `internal/session/mapping.go` | `addMapping()`, `findByClaudePrefix()`, `LoadMappings()` |
| Claude 会话数据扫描 | `internal/claude/activity.go` | `Scan()`, `aggregate()`, `readSessionMetas()` |
| Claude 子进程管理 | `internal/claude/subprocess.go` | `Start()`, `Client` |
| Dashboard API（展示 + 终端查找） | `internal/api/server.go` | `handleDashboard()`, `handleTerminalLookup()` |
| CLI 入口 | `cmd/pflow/main.go` | `runClaudeCmd()`, `runStatusCmd()`, `runServeCmd()` |

---

## 8. 与"Path is Project"哲学的关系

pflow 的设计理念是"路径即项目"——不使用额外的项目 ID 或名称实体。Session ID 管理也遵循这一原则：

- **Tmux session name 从目录名派生**，不是从 Claude session ID 派生
- **mappings.json 中存储 `work_dir`** 而不是 project name
- **仪表盘通过 `work_dir` 匹配 project roots**（`project.MatchRootFromList`）
- 一个目录可以有多个历史 Claude session（每个有不同 session ID），但都映射到同一个 tmux 环境

---

## 9. 后台映射同步

### 9.1 动机

`/clear`、`/resume` 或 Claude 重启会导致 Claude Session ID 变更，但 tmux session 保持不变。`SyncMappings()` 定期从每个 tmux pane 重新捕获 statusline 中的 8-char prefix，检测到变更时自动更新 `mappings.json`，使 Web UI 的终端绑定保持正确。

### 9.2 同步流程

```
┌─────────────────────────────────────────────────────────────────┐
│ serve 启动后 5s → 启动后台 goroutine                              │
│                                                                  │
│ 每 15s 一次循环：                                                  │
│   1. 加载 mappings.json 中所有映射                                  │
│   2. 对每个活着的 tmux session：                                    │
│      capture-pane (3s 超时) → 提取当前 prefix                      │
│      如果 prefix 非空且与保存的不同 → 更新 mapping                  │
│   3. 如果有更新 → 保存到 mappings.json                              │
│   4. 清理已死 tmux 的过期映射                                        │
└─────────────────────────────────────────────────────────────────┘
```

### 9.3 关键参数

| 参数 | 值 | 理由 |
|------|-----|------|
| 同步间隔 | 15s | 平衡及时性和资源消耗 |
| 捕获超时 | 3s | Claude 运行时 statusline 立即可见；超时表示 Claude 未运行 |
| 启动延迟 | 5s | 等待 server 完全就绪 |

---

## 10. 手动测试用例

> 以下用例依赖真实的 tmux 和 Claude Code 环境，需手动执行。

### 前置条件

```bash
# 1. 构建并启动 pflow server
make build
./bin/pflow serve -port 8080

# 2. 在另一个终端中，用 pflow 启动一个 Claude session
./bin/pflow claude -dir /path/to/some/project

# 3. 打开浏览器访问 http://localhost:8080，确认 Dashboard 可见
#    展开该项目的 session detail，确认 Terminal 按钮可用
```

### TC-1: `/clear` 后映射自动更新

260616 测试通过

**目的**：验证 `/clear` 导致 session ID 变更后，后台 sync 自动更新映射，Dashboard 中新 session 的 Terminal 按钮可用。

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 在 tmux 中的 Claude 里记下当前 session ID（statusline 第一列，如 `3ca06c7d`） | — |
| 2 | 在 Claude 中输入 `/clear` 并回车 | Claude 开始新对话，statusline 显示新的 8-char prefix |
| 3 | 等待最多 15 秒 | 后台 sync 检测到 prefix 变更，日志输出：`sync: tmux=pflow-xxx prefix changed: <旧> → <新>` |
| 4 | 刷新 Dashboard 页面 | 旧 session（`/clear` 前）的 `has_terminal` 变为 `false`；新 session 的 `has_terminal` 变为 `true` |
| 5 | 点击新 session 的 Terminal 按钮 | 能正常打开 Web Terminal，连接到同一个 tmux session |

### TC-2: `/resume` 后映射自动更新

260616 测试通过

**目的**：验证 `/resume` 切换回历史 session 后，映射也能正确更新。

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 在 Claude 中执行 `/resume`，选择一个不同的历史 session | Claude 切换到该历史 session，statusline 显示对应的 8-char prefix |
| 2 | 等待最多 15 秒 | 后台 sync 检测到 prefix 变更并更新映射 |
| 3 | 刷新 Dashboard | 当前活跃 session 获得 `has_terminal: true`，之前的 session 失去绑定 |

### TC-3: Claude 退出后重启

**目的**：验证 Claude 进程退出后重新启动，新 session 能自动绑定到同一 tmux。

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 在 Claude 中按 `Ctrl+D`（或输入 `/exit`）退出 Claude | tmux session 仍存活（bash 还在），但 Claude 已退出 |
| 2 | 在 tmux 中重新运行 `claude` | 新的 Claude 进程启动，statusline 显示新的 prefix |
| 3 | 等待最多 15 秒 | 后台 sync 检测到 prefix 变更，更新映射 |
| 4 | 刷新 Dashboard | 新 session 的 Terminal 可用；如果旧 session 仍在 Dashboard 中（如窗口期内），其 `has_terminal` 为 `false` |

### TC-4: 无 Claude 运行时不误报

**目的**：验证当 tmux session 存活但 Claude 未运行时，sync 不会误删映射或报错。

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 退出 Claude（`Ctrl+D`），但保持 tmux session | tmux session 存在，Claude 不在运行 |
| 2 | 等待 15 秒，观察 pflow server 日志 | 无错误日志；sync 因 `captureClaudePrefix` 返回空而跳过该 session |
| 3 | 在 tmux 中重新启动 `claude` | Claude 运行，statusline 显示新的 prefix |
| 4 | 等待 15 秒 | 映射被更新为新 prefix，无异常 |

### TC-5: 多 tmux session 各自独立同步

**目的**：验证同时管理多个项目时，每个项目的映射独立更新，互不干扰。

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 用 `pflow claude -dir /path/to/project-a` 启动项目 A 的 Claude | 创建 tmux session `pflow-project-a`，prefix 为 A 的 session ID |
| 2 | 用 `pflow claude -dir /path/to/project-b` 启动项目 B 的 Claude | 创建 tmux session `pflow-project-b`，prefix 为 B 的 session ID |
| 3 | 在项目 A 的 tmux 中执行 `/clear` | 仅项目 A 的 Claude session ID 变更 |
| 4 | 等待 15 秒 | 仅 `pflow-project-a` 的映射被更新；`pflow-project-b` 的映射不变 |
| 5 | 刷新 Dashboard | 两个项目的 Terminal 按钮均正确指向各自的 tmux session |

### TC-6: 映射持久化跨 server 重启

**目的**：验证 pflow server 重启后，映射仍能正确恢复和同步。

| 步骤 | 操作 | 预期结果 |
|------|------|----------|
| 1 | 确认已有至少一个活跃的 pflow tmux session（Claude 在运行） | — |
| 2 | 停止 pflow server（`Ctrl+C`） | — |
| 3 | 重新启动 `./bin/pflow serve -port 8080` | server 启动，5s 后首次 sync |
| 4 | 等待首个 sync 周期完成 | 日志显示从 `mappings.json` 恢复了映射，且 live capture 确认 prefix 未变（无更新） |
| 5 | 打开 Dashboard | Terminal 按钮正常工作 |

### 日志观察要点

执行上述用例时，关注 pflow server 的日志输出（默认写入 `./logs/` 目录）：

```
sync: tmux=pflow-xxx prefix changed: 3ca06c7d → a1b2c3d4    # prefix 变更
sync: updated 1 mapping(s)                                    # 更新计数
sync: cleaned 0 stale mapping(s)                              # 过期清理计数
```

如果 sync 周期内无变更，则不输出日志（静默）。
