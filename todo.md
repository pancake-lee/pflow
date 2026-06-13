# todo

> 当前周期：阶段三 项目策略管理

## 目标

从"扁平 session 列表"升级为"项目 → session"两级结构，支持用户设定 **1 个主线 + 最多 3 个支线** 的策略，让 Dashboard 从"状态监控"进化为"注意力分配决策辅助"。

核心原则：**最好的设计应该是无感的**。pflow 不替代用户的终端/VSCode，只负责信息的聚合和呈现。

已完成：`pflow status` / `probe` / `serve` / `claude`、Web Dashboard、Web 终端（ttyd）。详见 [`docs/prd.md`](./docs/prd.md) 阶段三。

## P0 — 数据层 + 核心策略，必须完成

### P0-1 项目数据模型

- [ ] 新增 `internal/project/` 包
- [ ] `~/.pflow/projects.json` 文件结构定义和读写（`store.go`）
  - 字段：`id` / `name` / `path`（可选，本地项目路径）/ `priority`（primary | secondary | normal | archived）/ `created_at` / `sort_order`
  - 默认项目：首次启动时自动创建"未分类"项目（`is_default: true`，`priority: normal`）
- [ ] 项目 CRUD 函数：`Create` / `Get` / `List` / `Update` / `Delete`
- [ ] 原子化写入（tmp 文件 + rename，与 mappings.json 一致）

### P0-2 Session ↔ Project 关联

- [ ] 扩展现有 Claude/Hermes scan 流程，为每个 session 关联 `projectId`
- [ ] 按工作目录自动归类：对于有 `path` 字段的 session，匹配项目 `path` 前缀；匹配到的自动关联
- [ ] 未匹配的 session 归入"未分类"默认项目
- [ ] 历史数据迁移：无需手动脚本，首次启动时自动将现有 session 归类
- [ ] 前端 store 中实现 `groupSessionsByProject` 计算属性

### P0-3 主线/支线策略引擎

- [ ] `internal/project/strategy.go`：优先级切换规则
  - `SetPrimary(projectId)`：原主线降为 normal，目标升为 primary
  - `SetSecondary(projectId)`：支线数 < 3 则直接加入；已满则返回错误
  - `SetNormal(projectId)`：从 primary/secondary 移除
  - `Archive(projectId)`：移入 archived
- [ ] 所有切换操作前后端双重校验（防止并发修改导致的超限）
- [ ] API 端点：
  - `PUT /api/v1/projects/:id` — 更新项目（含优先级切换）
  - `POST /api/v1/projects` — 创建项目
  - `DELETE /api/v1/projects/:id` — 删除项目（session 重新归入"未分类"）

### P0-4 Dashboard API 升级

- [ ] `GET /api/v1/dashboard` 返回结构扩展：增加 `projects` 字段
  ```json
  {
    "now": "...",
    "window": "1d",
    "projects": [ { "id": "...", "name": "...", "priority": "...", ... } ],
    "sessions": [ { ..., "project_id": "..." } ],
    "errors": []
  }
  ```
- [ ] 保持向后兼容：`sessions` 数组结构不变，仅增加 `project_id` 字段
- [ ] 项目按 `sort_order` 排序，优先级分区逻辑在前端实现

## P1 — 前端视图重构，必须完成

### P1-1 Dashboard 项目视图

- [ ] 新建组件 `ProjectCard.vue`：展示项目名、优先级标记、内部 session 列表
  - Session 行复用现有样式（状态灯 + session ID + last active + last req/resp）
  - 每个项目卡片显示 session 数量和状态分布摘要
- [ ] 新建组件 `PrioritySelector.vue`：优先级切换按钮/菜单
  - 选项受当前优先级和数量限制约束（如支线已满时"设为支线"按钮禁用并提示）
- [ ] 重构 `DashboardView.vue` 主布局：
  - 替换扁平 DataTable 为分区布局（⭐ 主线 / 🚩 支线 / 📁 普通 / 📦 归档）
  - 每个分区独立滚动，主线/支线始终展开，普通/归档可折叠
  - 统计摘要栏保留（按分区统计）

### P1-2 交互与状态管理

- [ ] Pinia store 新增 `projects` 模块（或扩展现有 store）
- [ ] 优先级切换时前端乐观更新 + 后端校验失败时回滚
- [ ] 支线数量超限时 Toast 提示 + 按钮禁用
- [ ] 归档项目默认折叠，点击展开
- [ ] 用户折叠/展开状态记忆（localStorage）

## P2 — 优化与迁移

- [ ] 帮助提示（Tooltip）：主线/支线规则说明
- [ ] 空状态设计：无项目时的引导提示
- [ ] 历史 session 迁移提示：首次升级时告知用户归类结果，允许手动调整
- [ ] 项目卡片拖拽排序（可选，可用 `vuedraggable`）

## 不包含（本周期）

- `pflow attach` / `suggest` / `focus` CLI 子命令（设计原则：pflow 不替代用户工作软件）
- 沉默提醒 / 军情哨主动推送（留待阶段四）
- TUI Dashboard / 游戏化外壳（留待阶段五）
- 多 Agent 类型启动（留待 backlog P2）
