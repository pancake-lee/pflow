# Tmux + ttyd Web 终端集成

> 周期：2026-06-12 | 状态：✅ 已完成

## 概述

已实现完整的"pflow+tmux 启动 Claude → Dashboard 通过 ttyd+tmux 在 Web 中提供终端交互"链路。

## 核心链路

```
pflow claude -dir /path/to/project
  → 自动配置 Claude statusline（~/.claude/settings.json）
  → 创建 tmux session（pflow-<name>）
  → 在 tmux 中启动 Claude Code
  → 异步 capture-pane 提取 8-char session 前缀
  → 保存映射到 ~/.pflow/mappings.json

pflow serve
  → Dashboard API 返回 session 列表时标注 has_terminal
  → 前端点击打开详情 → lookup API 查找 tmux 关联
  → 找到 → 显示"连接终端"按钮 → iframe 嵌入 ttyd Web 终端
```

## 实现文件

| 文件 | 功能 |
|------|------|
| `internal/session/manager.go` | tmux + ttyd 进程管理器：`Start()`, `StartExisting()`, `Stop()`, `List()`, 端口分配、依赖检查 |
| `internal/session/claude.go` | Claude statusline 配置（`checkStatusline` / `setupStatusline`）、`StartClaudeSession()` 创建 tmux+Claude、`captureClaudePrefix()` capture-pane 前缀解析、`LookupByClaudeSessionID()` 按前缀查找 |
| `internal/session/mapping.go` | `Mapping` 结构体、`mappingManager` 持久化管理（原子写入、tmux 存活检查、stale 清理）、`LoadMappings()` 公开接口 |
| `cmd/pflow/main.go:runClaudeCmd` | `pflow claude` CLI 子命令，支持 `-name` / `-dir` / `-force` / `-no-attach` |
| `internal/api/server.go` 终端 API | `POST /terminal/start`、`POST /terminal/stop`、`GET /terminal/list`、`GET /terminal/lookup` |

## Statusline 格式

```
sid8 | model | ctx | tok | session
```

例如：`c50e1b2e | deepseek-v4-pro | ctx 45%/ 55% | in:12000 out:800 | total:50000/32000`

## 映射持久化

`~/.pflow/mappings.json`：

```json
{
  "version": 1,
  "mappings": [
    {
      "tmux_name": "pflow-pflow",
      "work_dir": "/root/code/pflow",
      "claude_prefix": "c50e1b2e",
      "created_at": "2026-06-12T14:00:00Z"
    }
  ]
}
```

## 设计决策

- **异步前缀捕获**：Claude 启动后 statusline 需要几秒才渲染，`StartClaudeSession` 立即返回，prefix 捕获在 goroutine 中后台完成，不阻塞用户 attach
- **Statusline 先于 Claude 配置**：Claude 在启动时读取 `settings.json`，所以必须在 `tmux new-session` 之前配置好
- **只处理 pflow 托管的会话**：没有通过 `pflow claude` 启动的 Claude 无法关联 tmux，Terminal 面板是可选的增强功能
- **原子化写入**：settings.json 和 mappings.json 均通过 tmp 文件 + rename 保证写入原子性
- **非强制依赖**：只有 tmux 是 `pflow claude` 的核心依赖；ttyd 和 jq 分别只在 Web 终端和 statusline 时需要

## API 返回的终端关联信息

Dashboard API 返回的每个 Claude session 包含：

```json
{
  "has_terminal": true,
  "terminal_tmux_name": "pflow-pflow"
}
```

前端据此决定是否显示"连接终端"按钮。
