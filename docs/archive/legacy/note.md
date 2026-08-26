# note

> 参考笔记：记录不随每次上下文加载、但偶尔需要翻阅的规则、知识、历史决定等。
> 详细的开发日志和实现记录已归档到 [`../cycles/`](../cycles/)。

## 核心设计理念

### "最好的设计应该是无感的"

pflow 是信息层，不替代用户已有的工作软件（终端、VSCode 等）。用户看 Dashboard 获取信息后，自己切换到终端/VSCode 操作。"亲赴前线"是用户的自然行为——不需要 pflow 提供按钮或内嵌终端来完成。

详见 [`prd.md §3.3`](../../prd.md)。

### "路径即项目"

不引入独立的项目 ID/名称实体。session 元数据中已有的 working directory 就是天然的归属标识。用户只需标记"哪些路径是项目根"，子目录 session 自动按最长前缀匹配归入。目录名即"项目名"。

详见 [`prd.md §4.1`](../../prd.md), [`tech.md §2.3`](../../tech.md)。

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
| `~/.pflow/project_roots.json` | 项目根标记 + Slot 映射（v2 格式） | `internal/project/` |
| `~/.pflow/mappings.json` | tmux ↔ Agent session 映射（含 agentName/status/PID） | `internal/session/` |
| `~/.pflow/config.json` | 用户自定义配置（提醒参数等，规划中） | `internal/attention/config.go` |
| `~/.pflow/focus.log` | tmux focus 事件日志（用于活跃时间估算 Tier 1） | `internal/timetrack/focus.go` |
| `~/.claude/settings.json` | Claude statusline 配置 | `internal/session/claude.go` |
| `~/.claude/sessions/<pid>.json` | Claude session 元数据（DirScan 模式读取） | `internal/session/claude_scan.go` |
| `~/.claude/projects/<project>/<session>.jsonl` | Claude transcript | `internal/claude/activity.go` |
| `~/.hermes/sessions/sessions.json` | Hermes gateway 管理的活跃 session（状态富化用） | `internal/hermes/activity.go` |
| `~/.hermes/sessions/request_dump_*.json` | Hermes API 请求快照（含 cwd，fallback 数据源） | `internal/hermes/activity.go` |
| `~/.hermes/.pflow_cache_export.jsonl` | Hermes sessions export 临时缓存 | `internal/hermes/activity.go` |

## 已知注意事项

### Hermes 会话扫描

**主数据源**：`hermes sessions export` 输出的 JSONL（含 `id`/`source`/`title`/`last_active`/`messages[]`/`system_prompt`），通过临时缓存文件 `~/.hermes/.pflow_cache_export.jsonl` 读取。此数据源提供最完整的 LastReq/LastResp/时间戳/CWD 信息。

**会话 ID 处理**：hermes ID 格式为 `YYYYMMDD_HHMMSS_suffix`，前缀 8 位是日期（同日会话会重复），**改取后缀 8/16 位**作为 ShortID（与 `hermes sessions list` 行为一致）。`SuffixID()` 实现此逻辑。

**CWD 提取优先级**：1) request_dump 文件（fallback 循环中）→ 2) export 中的 system_prompt `Current working directory:` 行 → 3) 回退到 source 名称（如 "cli"）。CLI session → 真实目录，weixin/cron session → `/`（无意义，回退到 platform 名称）。

**Source 过滤**：`ScanOptions.SourceFilter` 支持按来源类型过滤（cli/weixin/cron），默认 `"cli,weixin"` 排除 cron。三个数据源循环（export → gateway fallback → dump fallback）均应用过滤。

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

阶段四 MVP 中，用户活跃追踪使用 session 状态作为代理指标（非鼠标/键盘监听）。每次 Dashboard API 请求时无状态计算，无需持久化。遮罩层预留 CSS 变量接口但不实现完整换肤。详见 [`cycles/05-phase4-kickoff.md`](../cycles/05-phase4-kickoff.md)。

## Claude 会话扫描模式

当前支持两种 Claude session 关联模式，通过 `SetClaudeCaptureMode()` 切换：

| 模式 | 数据源 | 更新频率 | 优缺点 |
|------|--------|----------|--------|
| `DirScan`（默认） | 扫描 `~/.claude/sessions/<pid>.json` + 匹配 `name` 字段 | 每 5s | 更快更可靠，需要 `claude -n <name>` |
| `Statusline`（旧方案） | `tmux capture-pane` 解析 statusline 中的 8-char 前缀 | 每 15s | 向后兼容，有 subprocess 开销 |

详见 `internal/session/claude_scan.go` 和 `internal/session/claude.go`。

## Slot 映射优先级系统

从阶段三的 `priority` 字段升级为阶段四的固定 slot 映射：

- **旧方案**：`RootsFile.Roots[].Priority` 直接存 `primary/secondary/normal`
- **新方案**：`RootsFile.Slots` map 独立存储 slot → path 映射，`Root.Slot` 字段记录所属 slot
  - `primary`: 主线
  - `secondary_1`, `secondary_2`: 两个支线 slot
  - 设置 slot 时自动腾退旧路径为 normal
- **v1→v2 迁移**：`migrateV1ToV2()` 在 Load 时自动转换
- **前端**：PrimaryCard 和 SecondaryCard 通过 slot API（`PUT/DELETE /api/v1/project-roots/slot`）操作

## 活跃时间估算降级链

`internal/timetrack/` 实现三级降级链（详见 `docs/design/09-active-time-calculation.md`）：

1. **Tier 1（最高精度）**：tmux focus 事件日志 → `focus.go` 精确到秒的窗口计时
2. **Tier 2（中精度）**：session 消息数估算 → `SessionTodayMinutes()` 每条消息 ≈ 3 分钟
3. **Tier 3（最低精度）**：wall-clock 回退 → 活跃窗口 × 0.3 系数

前端展示时优先使用 Tier 1 数据（通过 `/api/v1/dashboard` 的 project-level focus override）。

## 知识锚点与军情哨

- **军情哨**（`internal/suggest/`）：~20 个分析场景，从 S1（紧急等待）到 S20（主线超 4h 提醒）
- **知识锚点**（Knowledge Anchor）：12 条认知科学理论卡片，提供军情建议背后的理论依据
  - 前端组件：`web/src/components/KnowledgeAnchor.vue`
  - 数据：`internal/suggest/suggest.go` 中的 `allKnowledgeTips`
  - 设计文档：`docs/design/10-tips.md`

## 周期归档索引

| 文件 | 周期 | 内容 |
|------|------|------|
| [`cycles/01-phase1-phase2-foundation.md`](../cycles/01-phase1-phase2-foundation.md) | 2026-06-11 | 可行性验证 + Web Dashboard + CLI + Subprocess |
| [`cycles/02-hermes-integration.md`](../cycles/02-hermes-integration.md) | 2026-06-11 | Hermes Agent 集成调研 + 元数据字段参考 |
| [`cycles/03-tmux-ttyd.md`](../cycles/03-tmux-ttyd.md) | 2026-06-12 | tmux + ttyd Web 终端集成 |
| [`cycles/04-phase3-design-and-ui.md`](../cycles/04-phase3-design-and-ui.md) | 2026-06-13~14 | 阶段三设计决策 + Dashboard UI 重构 |
| [`cycles/05-phase4-kickoff.md`](../cycles/05-phase4-kickoff.md) | 2026-06-15 | 阶段四启动：提醒分数 + 遮罩层 MVP 规划 |
| [`cycles/06-phase4-reminder-attention-mask.md`](../cycles/06-phase4-reminder-attention-mask.md) | 2026-06-16 | 阶段四实现：提醒分数引擎 + 注意力遮罩层 |
| [`design/07-terminal-session-mapping.md`](../../design/07-terminal-session-mapping.md) | 2026-06-17 | 终端会话映射方案重构 |
| [`cycles/08-knowledge-anchor.md`](../cycles/08-knowledge-anchor.md) | 2026-06-25 | 知识锚点 + 军情哨提示系统 |
