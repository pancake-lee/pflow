# todo

> 当前周期：阶段三 项目策略管理

## 目标

让 Dashboard 从"扁平 session 列表"进化为以**项目路径**为维度的两级视图，支持用户设定 1 个主线 + 最多 3 个支线的注意力策略。

核心原则：
- **路径即项目**：不引入独立的项目 ID/名称实体。路径天然唯一，且 session 元数据中已有 working directory。
- **零手动归类**：用户不需要手动创建项目、命名、或把 session 拖到某个项目下。只需标记"哪些路径是项目根"，子目录 session 自动归入。
- **最好的设计应该是无感的**：pflow 不替代用户的终端/VSCode，只负责信息的聚合和呈现。

已完成：`pflow status` / `probe` / `serve` / `claude`、Web Dashboard、Web 终端（ttyd）。详见 [`docs/prd.md`](./docs/prd.md) 阶段三。

## P0 — 数据层 + 自动归类，必须完成

### P0-1 项目根存储

- [ ] 新增 `internal/project/` 包
- [ ] `~/.pflow/project_roots.json` 文件结构：
  ```json
  {
    "version": 1,
    "roots": [
      { "path": "/home/user/code/pflow", "priority": "primary" },
      { "path": "/home/user/code/hermes", "priority": "secondary" },
      { "path": "/home/user/code/pancake", "priority": "normal" }
    ]
  }
  ```
- [ ] `roots` 是一个简单列表。`path` 是唯一键（同一个 path 不会出现两次）。
- [ ] 读/写函数 + 原子化写入（tmp 文件 + rename）

### P0-2 Session 自动归类逻辑

- [ ] 每个 session 已有 `Project` 字段（working directory，来自 Claude/Hermes scan）
- [ ] 归类算法：
  1. 加载 `project_roots.json` 中的 roots
  2. 对每个 session，遍历 roots，如果 session 的 `Project` 路径**以 root.path 开头**（即 session.cwd 是 root.path 本身或其子目录），则匹配成功
  3. 多个 root 可能匹配同一个 session（如 `/a` 和 `/a/b` 都是 root）→ 取最长前缀匹配（最具体的 root）
  4. 未匹配任何 root 的 session 作为"独立 session"展示（不归入任何项目分组）
- [ ] 根目录保护：`/` 不能被标记为项目根（API 层拒绝），防止所有 session 被错误归入同一个项目

### P0-3 策略引擎

- [ ] `internal/project/strategy.go`：
  - `SetPriority(path, priority)`：更新指定 path 的优先级
  - `ValidatePriorities()`：确保最多 1 个 primary、最多 3 个 secondary，超限时返回错误
  - `RemoveRoot(path)`：取消标记（不再视为项目根），匹配该 root 的 session 重新归入独立或更短的 root 匹配
- [ ] API 端点：
  - `PUT /api/v1/project-roots` — 标记/更新路径的优先级（body: `{ "path": "...", "priority": "primary" }`）
  - `DELETE /api/v1/project-roots?path=...` — 取消标记
  - `GET /api/v1/project-roots` — 返回当前所有 roots

### P0-4 Dashboard API 升级

- [ ] `GET /api/v1/dashboard` 返回结构扩展：
  ```json
  {
    "now": "...",
    "window": "1d",
    "project_roots": [
      { "path": "/home/user/code/pflow", "priority": "primary" }
    ],
    "sessions": [
      { ..., "project": "/home/user/code/pflow/internal/api", "matched_root": "/home/user/code/pflow" }
    ],
    "unmatched_sessions": [ ... ],
    "errors": []
  }
  ```
- [ ] `matched_root` 字段：标识该 session 被归入的 root，前端据此分组展示
- [ ] `unmatched_sessions`：没有匹配到任何 root 的 session，前端作为独立条目展示
- [ ] 保持向后兼容：现有字段不变，仅新增 `matched_root`

## P1 — 前端视图重构，必须完成

### P1-1 项目根标记交互

- [ ] 在 Dashboard 中，每个 session 行 / 每个 distinct 工作目录旁边增加一个勾选框 ☐ "识别为项目"
- [ ] Hover tooltip 文案："将该目录标记为项目根，其子目录下的所有 session 都将归类到此项目下"
- [ ] 勾选后调用 `PUT /api/v1/project-roots`，默认优先级 `normal`
- [ ] 已标记的路径显示 ☑ 已勾选状态，取消勾选调用 `DELETE`

### P1-2 Dashboard 项目分组视图

- [ ] 按 `matched_root` 将 session 分组展示（替代当前的扁平表格）
- [ ] 每个分组显示：
  - 路径（项目根）+ 优先级徽章（⭐主线 / 🚩支线 / 📁普通）
  - 优先级下拉切换（主线 / 支线 / 普通 / 取消标记），受数量限制控制
  - 组内 session 列表（复用现有行样式）
- [ ] 分区布局：
  - ⭐ 主线区域（0 或 1 个 root，始终展开）
  - 🚩 支线区域（最多 3 个 root，始终展开）
  - 📁 普通区域（不限，可折叠）
  - 未归类 session（`matched_root` 为空的，可折叠）
- [ ] 无 root 标记时的空状态引导：提示用户勾选目录旁的 ☐ 来创建第一个项目

### P1-3 优先级管理

- [ ] 每个项目分组的优先级下拉菜单：⭐ 主线 / 🚩 支线 / 📁 普通 / ✕ 取消标记
- [ ] 选"主线"时：原主线自动降为普通（前端乐观更新 + 后端校验）
- [ ] 选"支线"时：若已有 3 个支线，菜单项显示为禁用态 + tooltip 提示"支线项目已满（最多 3 个）"
- [ ] 操作反馈：Toast 提示成功/失败

## P2 — 优化

- [ ] 深色路径显示：分组标题中路径的展示方式（折叠 HOME、高亮项目名等）
- [ ] 用户对普通/未归类区域的折叠状态记忆（localStorage）
- [ ] 空状态设计：无 root 时的引导提示

## 不包含（本周期）

- `pflow attach` / `suggest` / `focus` CLI 子命令（设计原则：pflow 不替代用户工作软件）
- 沉默提醒 / 军情哨主动推送（留待阶段四）
- TUI Dashboard / 游戏化外壳（留待阶段五）
