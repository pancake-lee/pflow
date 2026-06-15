# note

> 参考笔记：记录不随每次上下文加载、但偶尔需要翻阅的规则、知识、历史决定等。
> 详细的开发日志和实现记录已归档到 [`docs/cycles/`](./cycles/)。

## 核心设计理念

### "最好的设计应该是无感的"

pflow 是信息层，不替代用户已有的工作软件（终端、VSCode 等）。用户看 Dashboard 获取信息后，自己切换到终端/VSCode 操作。"亲赴前线"是用户的自然行为——不需要 pflow 提供按钮或内嵌终端来完成。

详见 [`prd.md §3.3`](./prd.md)。

### "路径即项目"

不引入独立的项目 ID/名称实体。session 元数据中已有的 working directory 就是天然的归属标识。用户只需标记"哪些路径是项目根"，子目录 session 自动按最长前缀匹配归入。目录名即"项目名"。

详见 [`prd.md §4.1`](./prd.md), [`tech.md §2.3`](./tech.md)。

### 信息聚合优于功能堆叠

Dashboard 的核心价值是让用户快速判断"现在该关注哪个项目"，而非在 pflow 中完成所有操作。Web 终端（ttyd）保留为辅助备选，不作为主要交互路径。

## 工程约定

### 原子化写入

所有 JSON 配置文件写入采用 **tmp 文件 + rename** 模式：

```go
// 1. 写入临时文件
f, _ := os.CreateTemp(dir, ".tmp-*")
json.NewEncoder(f).Encode(data)
f.Close()
// 2. 原子 rename
os.Rename(f.Name(), targetPath)
```

涉及文件：`project_roots.json`, `mappings.json`, `settings.json`。

### Statusline 配置时序

Claude 在启动时读取 `~/.claude/settings.json`，因此 statusline 配置**必须先于 `tmux new-session`** 完成。`StartClaudeSession` 中配置先写入，prefix 捕获是异步的（Claude 启动后 statusline 才渲染）。

### `make build` 必须用于编译

直接 `go build` 会嵌入过时的前端资源。`make build` 先执行 `npm run build` 生成 `web/dist/`，再 `go build`。

## 关键数据路径

| 路径 | 内容 | 管理包 |
|------|------|--------|
| `~/.pflow/project_roots.json` | 项目根标记 + 优先级 | `internal/project/` |
| `~/.pflow/mappings.json` | tmux ↔ Claude session 映射 | `internal/session/` |
| `~/.pflow/config.json` | 用户自定义配置（提醒参数等） | `internal/attention/config.go`（规划中） |
| `~/.claude/settings.json` | Claude statusline 配置 | `internal/session/claude.go` |
| `~/.claude/projects/<project>/<session>.jsonl` | Claude transcript | `internal/claude/activity.go` |
| `~/.hermes/sessions/sessions.json` | Hermes session 列表 | `internal/hermes/activity.go` |
| `~/.hermes/sessions/request_dump_*.json` | Hermes API 请求快照（含 cwd） | `internal/hermes/activity.go` |
| `~/.hermes/state.db` | Hermes SQLite（messages 表含 Last Resp） | 待接入（backlog P2） |

## 已知注意事项

### Hermes cwd 提取

Hermes 没有原生的 cwd/project 字段。cwd 的唯一可靠来源是 request_dump 文件内 system prompt 中的 `Current working directory: <path>` 行。CLI session → 真实目录，weixin/cron session → `/`（无意义，回退到 platform 名称）。详见 [`cycles/02-hermes-integration.md`](./cycles/02-hermes-integration.md)。

### Claude session ID 前缀

8 字符前缀来自 Claude statusline 输出，通过 `tmux capture-pane` 解析。`/clear` 或 `/resume` 会导致 session 绑定更换，需要重新 capture 并同步到前端（backlog P1 待处理）。

### 状态与红绿灯映射

| Agent | 状态 | 红绿灯 | IsActive |
|-------|------|--------|----------|
| Claude | `busy` | 🟢 | true |
| Claude | `waiting` | 🟡 | true |
| Claude | `idle` | ⚪ | true |
| Claude | `inactive` | ⚫ | false |
| Hermes | running | 🟢 | true |
| Hermes | inactive | ⚫ | false |

busy 状态时清除 Last Resp（避免展示不匹配的 req/resp 对）。文本列中的 `\n` `\r` `\t` 转义为 `\n` `\r` `\t`，防止破坏表格布局。

### 提醒分数 MVP 简化

阶段四 MVP 中，用户活跃追踪使用 session 状态作为代理指标（非鼠标/键盘监听）。每次 Dashboard API 请求时无状态计算，无需持久化。遮罩层预留 CSS 变量接口但不实现完整换肤。详见 [`cycles/05-phase4-kickoff.md`](./cycles/05-phase4-kickoff.md)。

## 周期归档索引

| 文件 | 周期 | 内容 |
|------|------|------|
| [`cycles/01-phase1-phase2-foundation.md`](./cycles/01-phase1-phase2-foundation.md) | 2026-06-11 | 可行性验证 + Web Dashboard + CLI + Subprocess |
| [`cycles/02-hermes-integration.md`](./cycles/02-hermes-integration.md) | 2026-06-11 | Hermes Agent 集成调研 + 元数据字段参考 |
| [`cycles/03-tmux-ttyd.md`](./cycles/03-tmux-ttyd.md) | 2026-06-12 | tmux + ttyd Web 终端集成 |
| [`cycles/04-phase3-design-and-ui.md`](./cycles/04-phase3-design-and-ui.md) | 2026-06-13~14 | 阶段三设计决策 + Dashboard UI 重构 |
| [`cycles/05-phase4-kickoff.md`](./cycles/05-phase4-kickoff.md) | 2026-06-15 | 阶段四启动：提醒分数 + 遮罩层 MVP 规划 |
