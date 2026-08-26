# 阶段三：项目策略管理 — 设计与 UI 重构

> 周期：2026-06-13 ~ 2026-06-14 | 状态：✅ 已完成

## 设计方向调整（2026-06-13）

核心理念明确：**最好的设计应该是无感的**。

- pflow 是信息层，不替代用户已有的工作软件（终端、VSCode 等）
- "亲赴前线"是用户的自然行为——用户看 Dashboard 获取信息后，自己切换到终端/VSCode 操作
- Dashboard 的价值是**呈现信息、辅助决策**，而非在工具内嵌入另一个工具
- Web 终端（ttyd）保留为辅助备选，不作为主要交互路径

阶段三重定向：从 CLI 独立操作命令 → 项目维度的策略管理。

## 项目模型设计决策

**路径即项目**，不引入独立实体：

- 不创建项目 ID、名称等独立实体。路径天然唯一，目录名即"项目名"。
- 不要求用户手动创建项目、命名、或把 session 拖到某个项目下。
- 使用 session 元数据中已有的 working directory（`Project` 字段）作为归属标识。
- 用户只需通过 ☐ "识别为项目" 勾选来标记哪些路径是项目根。
- 子目录 session 按最长前缀匹配自动归入。

存储：`~/.pflow/project_roots.json`，仅含 `[{path, priority}]` 的简单列表。

## Dashboard UI 全面重构（2026-06-14）

### 主/支线卡片重构

- 统计区域（Total/Active/Waiting/Idle）从内容区移至顶部 Header 栏
- `PrimaryCard.vue`：全宽主线卡片
  - 主 session 区域：左侧纵向排列 agent/sessionID/status/name/time，右侧左右分栏展示 last req / last resp
  - 其他 session 紧凑表格，操作列含 ⭐设为主session + 🖥终端图标
  - 标题栏含项目分配下拉框（`NSelect`），替代原来的优先级菜单
- `SecondaryCard.vue`：半宽支线卡片（2 列 grid 并排）
  - 主 session req/resp 上下堆叠（宽度受限场景）
  - 其他 session 表格不含 req/resp 列
- `GroupCard.vue`：移除优先级下拉（仅保留 ☐ checkbox），供普通/未归类使用
- 优先级切换交互变更：从卡片内下拉 → 主线/支线标题栏项目分配槽位
  - 选择一个项目 → `PUT /api/v1/project-roots` 设定对应优先级
  - 清除选择 → 降级为 `normal`
- `MaxSecondary` 从 3 改为 2（`internal/project/store.go`）
- 主线/支线卡片始终可见，无对应项目时显示占位状态

### 主 session 概念

- 每个主线/支线项目有一个"主 session"（首个活跃 session 或第一个）
- 非主 session 表格中提供 ⭐ 按钮将其提升为主 session（前端 in-memory 状态）
- 主 session 和列表行均支持点击弹出侧边栏详情

### P2-7 视觉与布局精修

- 主线区域绿色背景（`zone-section--primary`）、支线区域黄色背景（`zone-section--secondary`）
- 主线标题栏重构：`⭐ 主线项目 [basename] [fullPath] --- [project ▼]` 内联布局，下拉框移至 zone header
- 主线主 session 元数据重构：第一行 ⭐+agent+sessionid+TTY，第二行 状态+时间，Name 3行截断
- 支线主 session 增加 TTY 图标（可连接时显示在 sessionid 后）
- 主线表格保留 last req / last resp 列（与普通/未归类表格列一致）
- 所有表格（主线/支线/普通/未归类）空数据时展示表头 + 一条 "-" 占位行
- Header 和 Footer 粘性定位（`position: sticky`），滚动页面不消失

### P2-8 继续精修

- 移除所有主 session 区域和表格中的 Last Resp 列（API 字段和详情侧拉栏保留）
- 筛选栏紧凑化：`padding` 10px→6px，`margin-bottom` 16px→12px
- 主线/支线标题栏：用项目下拉框 NSelect 替代项目名+路径的文字展示（下拉框既展示又选择，省空间）
- 去掉"🚩支线项目...1/2"共享分割线，支线两个卡片各自显示标题 `🚩 支线项目1` / `🚩 支线项目2`（通过 `:index` prop 传入）
- 主 session 的 Last Req 区域固定 3 行高度（`min-height: 4.5em; max-height: 4.5em`），移除 `-webkit-line-clamp`

### 已实现文件清单

| 文件 | 说明 |
|------|------|
| `internal/project/store.go` | `Manager` 结构体，项目根 JSON 文件的原子读写 |
| `internal/project/strategy.go` | `SetPriority` / `RemoveRoot` / `Validate` / `MatchRootFromList` |
| `internal/project/strategy_test.go` | 11 个测试用例 |
| `internal/api/server.go` | 3 个 project-roots API + Dashboard 响应扩展 `matched_root` |
| `web/src/types/dashboard.ts` | `ProjectRoot` / `matched_root` / `project_roots` 类型 |
| `web/src/components/GroupCard.vue` | 普通/未归类分组卡片（checkbox，无优先级下拉） |
| `web/src/components/PrimaryCard.vue` | ⭐主线项目卡片（主 session 展示 + 列表 + 项目分配下拉） |
| `web/src/components/SecondaryCard.vue` | 🚩支线项目卡片（上下 req/resp + 列表 + 项目分配下拉） |
| `web/src/views/DashboardView.vue` | 重构后的 Dashboard：Header 统计 + PrimaryCard + 2×SecondaryCard + 普通/未归类 |
