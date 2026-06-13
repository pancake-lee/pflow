# todo

> 当前周期：阶段三 项目策略管理

## 目标

让 Dashboard 从"扁平 session 列表"进化为以**项目路径**为维度的两级视图，支持用户设定 1 个主线 + 最多 2 个支线的注意力策略。

核心原则：
- **路径即项目**：不引入独立的项目 ID/名称实体。路径天然唯一，且 session 元数据中已有 working directory。
- **零手动归类**：用户不需要手动创建项目、命名、或把 session 拖到某个项目下。只需标记"哪些路径是项目根"，子目录 session 自动归入。
- **最好的设计应该是无感的**：pflow 不替代用户的终端/VSCode，只负责信息的聚合和呈现。

已完成：`pflow status` / `probe` / `serve` / `claude`、Web Dashboard、Web 终端（ttyd）。详见 [`docs/prd.md`](./docs/prd.md) 阶段三。

## P0 — 数据层 + 自动归类 ✅

### P0-1 项目根存储

- [x] 新增 `internal/project/` 包
- [x] `~/.pflow/project_roots.json` 文件结构（version + roots 列表，path 为唯一键）
- [x] 读/写函数 + 原子化写入（tmp 文件 + rename）

### P0-2 Session 自动归类逻辑

- [x] 每个 session 已有 `Project` 字段（working directory，来自 Claude/Hermes scan）
- [x] 最长前缀匹配算法（`MatchRootFromList`），含路径边界检测
- [x] 根目录保护：`/` 不能被标记为项目根（API 层拒绝）

### P0-3 策略引擎

- [x] `SetPriority(path, priority)` / `RemoveRoot(path)` / `Validate()`
- [x] API 端点：`PUT` / `DELETE` / `GET` `/api/v1/project-roots`

### P0-4 Dashboard API 升级

- [x] `GET /api/v1/dashboard` 返回 `project_roots` + `matched_root` 字段
- [x] 未归类 session 通过 `matched_root` 为空区分（非独立的 `unmatched_sessions` 字段）

## P1 — 前端视图重构 ✅

### P1-1 项目根标记交互

- [x] GroupCard 头部 ☐ "识别为项目" checkbox + hover tooltip
- [x] 勾选 → `PUT /api/v1/project-roots` (priority=normal)，取消勾选 → `DELETE`

### P1-2 Dashboard 项目分组视图（第一版）

- [x] 按 `matched_root` 分组 → `GroupCard.vue` 组件
- [x] 四区布局：⭐主线 / 🚩支线 / 📁普通 / 📂未归类

### P1-3 优先级管理（第一版）

- [x] 优先级下拉切换，主线自动降级，支线限额校验，Toast 反馈

## P2 — UI 优化 ✅ (2026-06-14 完成)

### P2-1 统计区域移至标题栏

- [x] Total / Active / Waiting / Idle 四列统计内嵌到顶部 Header 中
- [x] 带颜色标识的数值 + 标签，紧凑排版

### P2-2 主线项目卡片重构

- [x] 全宽 `PrimaryCard.vue` 组件
- [x] 主 session 区域：左侧纵向字段（agent / session ID / status / name / time），右侧左右分栏展示 last req / last resp
- [x] 其他 session 列表（紧凑表格）
- [x] 操作列（⭐设为主session + 🖥终端图标）

### P2-3 支线项目卡片重构

- [x] 2 个 `SecondaryCard.vue` 组件，左右并排（CSS grid 1fr 1fr）
- [x] 主 session 区域：req/resp 上下堆叠（宽度受限场景）
- [x] 其他 session 列表不展示 req/resp 列

### P2-4 优先级分配交互重构

- [x] 优先级选择器移至 PrimaryCard / SecondaryCard 标题栏右侧 `NSelect` 下拉
- [x] 交互语义："将某个项目分配到主线/支线槽位"
- [x] 选择 → `PUT /api/v1/project-roots` 设定对应 priority
- [x] 清除 → 降级为 normal（保留为项目根）
- [x] GroupCard 中移除优先级下拉（仅保留 ☐ 识别为项目 checkbox）

### P2-5 主线/支线占位

- [x] 即使无对应优先级的项目，卡片也不消失，保持占位区域
- [x] 空状态显示 "Assign a project to this slot..." 提示

### P2-6 后端配合

- [x] `MaxSecondary` 从 3 改为 2（`internal/project/store.go`）

### P2-7 视觉与布局精修（2026-06-14）

- [x] 主线区域绿色背景（`zone-section--primary`）、支线区域黄色背景（`zone-section--secondary`）
- [x] 主线标题栏重构：`⭐ 主线项目 [basename] [fullPath] --- [project ▼]` 内联布局，下拉框移至 zone header
- [x] 主线主 session 元数据重构：第一行 ⭐+agent+sessionid+TTY，第二行 状态+时间，Name 3行截断
- [x] 支线主 session 增加 TTY 图标（可连接时显示在 sessionid 后）
- [x] 主线表格保留 last req / last resp 列（与普通/未归类表格列一致）
- [x] 所有表格（主线/支线/普通/未归类）空数据时展示表头 + 一条 "-" 占位行
- [x] Header 和 Footer 粘性定位（`position: sticky`），滚动页面不消失

### P2-8 继续精修（2026-06-14）

- [x] 移除所有主 session 区域和表格中的 Last Resp 列（API 字段和详情侧拉栏保留）
- [x] 筛选栏紧凑化：`padding` 10px→6px，`margin-bottom` 16px→12px
- [x] 主线/支线标题栏：用项目下拉框 NSelect 替代项目名+路径的文字展示（下拉框既展示又选择，省空间）
- [x] 去掉"🚩支线项目...1/2"共享分割线，支线两个卡片各自显示标题 `🚩 支线项目1` / `🚩 支线项目2`（通过 `:index` prop 传入）
- [x] 主 session 的 Last Req 区域固定 3 行高度（`min-height: 4.5em; max-height: 4.5em`），移除 `-webkit-line-clamp`

## 已实现文件清单

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

## 不包含（本周期）

- `pflow attach` / `suggest` / `focus` CLI 子命令（设计原则：pflow 不替代用户工作软件）
- 沉默提醒 / 军情哨主动推送（留待阶段四）
- TUI Dashboard / 游戏化外壳（留待阶段五）
- 深色路径显示 / 折叠状态 localStorage 记忆（留待后续优化）

1. 减少筛选控件上下空隙，紧凑一点
2. ⭐/🚩 主线项目/支线项目 项目下拉框 agent图标/名字 session tty图标 status lastActive
3. 去掉“🚩支线项目.....1/2”这个“分割线”
4. 支线项目分左右区域，标题展示“🚩 支线项目1...”和“🚩 支线项目2...”