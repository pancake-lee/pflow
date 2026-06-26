# pflow 代码质量审查

> 2026-06-27 整理回顾，面向维护者。每项含问题和建议，待审批后执行。

## 一、项目概况

| 指标 | 数值 |
|------|------|
| Go 源文件 | 29 个 |
| 内部包 | 10 个（claude, hermes, project, attention, timetrack, suggest, session, api, config） |
| 前端文件 | 15 个（Vue + TS + CSS） |
| 测试覆盖 | 3/10 包有测试（attention, project, session） |
| Go 版本 | 1.24.4（与 go.mod 一致） |
| 代码行数 | ~3700 行（cmd/main.go + server.go + DashboardView.vue = 三个最大文件） |

## 二、需要立即修复的问题

### 2.1 🔴 缩进错误（`internal/api/server.go`）

第 218 行和第 250 行存在对齐错误：

```go
// 当前（错误）:
Name:         s.Name,
    FirstActive:  s.FirstActive,  // 多了一层缩进
LastActive:   s.LastActive,

// 应为:
Name:         s.Name,
FirstActive:  s.FirstActive,
LastActive:   s.LastActive,
```

**影响**：仅美观问题，不影响语义。但会误导后续维护者。

**建议**：直接修复缩进。

### 2.2 🟡 代码重复：`main.go` ↔ `server.go`

以下函数在两个文件中实现完全相同（或近乎相同）：

| 函数 | main.go | server.go |
|------|---------|-----------|
| `hermesStatusToSuggest` | ✓ | ✓ |
| `detectCurrentProject` / `detectCurrentProjectFromAPI` | ✓ | ✓ |
| `processAlive` | ✓ | ✓ |
| `loadFocusLog` / `loadAndResolveFocusLog` + `readResolvedFocusLog` | ✓ | ✓ |

**影响**：
- 修改 bug 时需要两处同步修改，容易遗漏。
- `detectCurrentProject` 和 `detectCurrentProjectFromAPI` 的唯一区别是后者在 `rootsFile == nil ||

\`mappings empty` 时提前返回。可以合并。

**建议**：
- **方案 A（推荐）**：将共享函数提取到 `internal/` 下的新文件（如 `internal/shared/util.go`），两处 import 同一实现。
- **方案 B**：将 CLI 命令（status, probe, suggest）和 API 服务放在同一 binary 的不同入口，共享一个 `internal/` 逻辑层。当前 main.go 中大量业务逻辑可以直接调用 `internal/` 包的导出函数。
- **方案 C**：暂不处理，仅加注释标注"修改时需同步"。

> 💡 推荐方案 A，改动最小，且立即消除重复隐患。

## 三、架构与包结构

### 3.1 🟡 `cmd/pflow/main.go` 过于庞大（1021 行）

当前 `main.go` 承担了：
- CLI 入口与命令路由
- 全局标志解析
- 每个子命令的业务逻辑（status, probe, serve, claude, hermes, session list/destroy, suggest）
- 格式化辅助函数（shortID, truncate, escapeNewlines, formatMinutes, formatSince, formatTime, formatMinutesBrief, shortPath）
- Tmux 客户端检测（detectCurrentProject）
- 进程存活检查（processAlive）
- Hermes 状态映射（hermesStatusToSuggest）
- Focus log 加载（loadFocusLog）
- 每日摘要打印（printDailySummary）

**建议**：按命令拆分到 `internal/cli/` 包下，每个子命令一个文件。格式化辅助函数移到 `internal/cli/format.go`。

### 3.2 🟡 `internal/api/server.go` 同样庞大（1188 行）

除了 HTTP handler 和路由注册，还包含了：
- `computeReminderScores` — 提醒分数聚合逻辑（~150 行）
- `generateSuggestions` — 军情分析生成逻辑（~110 行）
- `detectCurrentProjectFromAPI` — tmux 客户端检测（重复）
- `hermesStatusToSuggest` — 状态映射（重复）
- `processAlive` — 进程检测（重复）
- `loadAndResolveFocusLog` / `readResolvedFocusLog` — focus log 加载（重复）

**建议**：
- 将 `computeReminderScores` 移到 `internal/attention/`（或新文件 `attention/compute.go`）
- 将 `generateSuggestions` 移到 `internal/suggest/`（或新文件 `suggest/generate.go`）
- 共享工具函数移到统一位置（参见 §2.2）

### 3.3 🟢 包分层清晰

当前包依赖关系合理：
- `attention` / `timetrack` → 不依赖其他 internal 包
- `suggest` → 依赖 `timetrack`
- `project` → 独立
- `api` → 依赖所有 internal 包（作为聚合层）
- `cmd/pflow` → 依赖所有 internal 包（作为 CLI 聚合层）

`api` 和 `cmd/pflow` 同为聚合层且存在代码重复，符合预期——它们都是"最外层"。问题在于两个聚合层之间有大量重复的胶水代码。

### 3.4 🟢 `go vet` 无问题

```bash
$ go vet ./internal/...
# 无输出 = 全部通过
```

## 四、代码风格

### 4.1 🟢 命名约定一致

- 包名全部小写，无下划线 ✅
- 导出类型 PascalCase，导出函数 CamelCase ✅
- 私有函数 camelCase ✅
- JSON tag 使用 snake_case ✅

### 4.2 🟢 Godoc 注释总体良好

绝大多数导出类型和函数都有 godoc 注释。发现的缺失：
- `claude/subprocess.go:43` — `func Start(...)` 缺少文档注释
- 测试函数无 godoc（这是 Go 惯例，不算问题）

### 4.3 🟢 工程约定一致

- JSON 文件原子写入（tmp + rename）模式统一 ✅
- 错误处理使用 `fmt.Errorf("context: %w", err)` 包装 ✅
- 日志使用 `plogger`（来自 `github.com/pancake-lee/pgo/pkg/plogger`）✅

### 4.4 🟢 `make build` 约定正确执行

前端构建（`npm run build`）→ Go 编译的顺序正确。
无法直接 `go build` 编译（会嵌入过时前端资源），已在 `CLAUDE.md` 和 `note.md` 中明确记录。

## 五、测试

### 5.1 当前覆盖

| 包 | 测试文件 | 状态 |
|----|---------|------|
| `attention` | `score_test.go` | ✅ 通过 |
| `project` | `strategy_test.go` | ✅ 通过（17 个测试用例） |
| `session` | `claude_test.go` | ✅ 通过 |
| 其余 7 个包 | 无 | ❌ |

### 5.2 建议

- `internal/config/` 中的 `ParseWindow` 是纯函数，适合加 table-driven 单元测试
- `internal/suggest/` 的场景检查函数应加单元测试（输入 → 预期输出），当前 bug 只能靠手动测试发现
- `internal/timetrack/` 的时间估算函数应加边界条件测试
- `internal/api/` 的 HTTP handler 适合加集成测试（`httptest.NewServer`）
- `internal/hermes/` 的数据解析逻辑应加测试（`SuffixID`、source 过滤、CWD 提取）

**优先级建议**：先覆盖 `config` → `suggest` → `timetrack`，保证核心逻辑可回归。

## 六、前端

### 6.1 🟡 `DashboardView.vue` 过于庞大（1459 行）

单个 SFC 承载了：
- 模板渲染
- 筛选/排序/刷新状态管理
- 项目分组计算
- Slot 分配逻辑（assignSlot, clearSlot）
- 终端管理（lookupTerminal, startTerminal）
- 专注模式（focusExtend, focusStop, focusCountdown）
- 详情 Drawer
- 折叠状态持久化
- 主会话星标（starredSessionIds）

**建议**：将逻辑提取到 composables：
- `useSlotManagement` — slot 分配 / 清除
- `useTerminal` — 终端 lookup / start / modal 状态
- `useFocusMode` — 专注模式状态 + countdown
- `useMainSession` — 星标主会话逻辑

### 6.2 🟢 TypeScript 类型安全

定义了完整的 `DashboardEntry`, `ScanOptions`, `TerminalLookupResponse` 等类型，后端 JSON 契约有显式类型注解。✅

### 6.3 🟢 组件拆分合理

`PrimaryCard`, `SecondaryCard`, `GroupCard`, `SuggestCard`, `KnowledgeAnchor` 各司其职。✅

## 七、其他发现

### 7.1 🟢 Vue SPA 嵌入

`//go:embed web/dist/*` 在 `embed.go`，`cmd/pflow/main.go` 通过 `pflow.WebDist` 引用。SPA fallback 逻辑正确处理了 Vue Router 的 history mode。✅

### 7.2 🟡 `internal/attention/focus.go` 中的 `GetFocus` 使用包级别单例

```go
var globalFocus = &Focus{}
func GetFocus() *Focus { return globalFocus }
```

Go 中包级状态 + goroutine 并发访问需要互斥锁保护。检查 `focus.go` 是否有锁：
- 如果有 `sync.Mutex` 保护 → OK
- 如果没有 → 可能在并发请求中产生 data race

**建议**：确认 `Focus` 结构体内部有互斥保护，或改用 DI 方式传递（通过 `Server` 持有 `*Focus`）。

### 7.3 🟢 `go.mod` 依赖管理

直接依赖仅 `pancake-lee/pgo` 和 `go.uber.org/zap`，间接依赖（`pterm`, `kratos` 等）来自 pgo。依赖层次浅，可维护性高。✅

## 八、建议执行顺序

| 优先级 | 项目 | 预计改动量 |
|--------|------|-----------|
| 🔴 立即 | 修复 `server.go` 缩进错误 | 2 行 |
| 🟡 高 | 消除 `main.go` ↔ `server.go` 代码重复（提取到 shared 包） | ~100 行 |
| 🟡 中 | 拆分 `cmd/pflow/main.go` 的命令逻辑到 `internal/cli/` | ~500 行重排 |
| 🟡 中 | 将 `api/computeReminderScores` 移到 `attention/` | ~150 行 |
| 🟢 低 | 增加 `config`, `suggest`, `timetrack` 的单元测试 | 新增文件 |
| 🟢 低 | 提取 `DashboardView.vue` 逻辑到 composables | ~200 行移动 |
| 🟢 低 | 检查 `attention.Focus` 并发安全性 | 复核 |
