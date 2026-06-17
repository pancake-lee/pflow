# Backlog

> 全部技术需求池，按序号排列。
> 状态标记：Done、Pending、WIP、Approved、Abandoned、Rejected

| 状态     | 序号 | 类别       | 任务                                  | 简述                                                                                                                                                     |
| -------- | ---- | ---------- | ------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Done     | 0    | 核心架构   | 阶段一：可行性验证                    | 验证 cc-connect 通信能力、tmux 会话托管、Web 终端嵌入等核心技术可行性                                                                                    |
| Done     | 1    | Web 前端   | 阶段二：Web Dashboard                 | Vue 3 + Naive UI 浏览器端可视化面板，DataTable 会话列表、红绿灯状态、筛选/排序、自动刷新、侧边栏详情、Go embed 单文件部署                                |
| Done     | 2    | CLI 工具   | `pflow claude` CLI 子命令           | 一键创建 tmux + Claude 托管会话，自动配置 statusline、提取 session 前缀、保存关联映射。支持 `-name` / `-dir` / `-force` / `-no-attach`           |
| Done     | 3    | 会话管理   | Web 终端集成（ttyd + tmux）           | 侧边栏通过 ttyd 嵌入 Web 终端，通过 Claude statusline 的 8 位 session ID 前缀关联 tmux↔Claude session，Dashboard 可自动 lookup 并打开终端交互           |
| Done     | 4    | 会话管理   | Session 管理与映射持久化              | tmux + ttyd 进程生命周期管理、`~/.pflow/mappings.json` 映射持久化、statusline 自动配置、capture-pane 前缀解析                                          |
| Done     | 5    | CLI 工具   | `pflow status`                      | 状态仪表盘，CLI 文本表格 + Web Dashboard 双模式                                                                                                          |
| Done     | 6    | CLI 工具   | `pflow probe <id>`                  | 探测单个 session 详细状态                                                                                                                                |
| Done     | 7    | 核心架构   | `pflow serve`                       | HTTP Dashboard API + Web Dashboard，单二进制部署                                                                                                         |
| Done     | 8    | 核心架构   | 项目根标记                            | `~/.pflow/project_roots.json` 存储被标记为项目根的路径列表 + 优先级（primary/secondary/normal）。路径即项目，不引入独立 ID/名称实体                    |
| Done     | 9    | 会话管理   | Session 自动归类                      | 最长前缀匹配算法：session cwd 匹配到某个项目根路径则自动归入。根目录 `/` 禁止标记。未匹配的作为独立 session 展示                                       |
| Done     | 10   | Web 前端   | "识别为项目" UI                       | Dashboard 界面上每个 distinct 工作目录旁提供勾选框 + hover tooltip，一键标记/取消项目根                                                                  |
| Done     | 11   | 核心架构   | 主线/支线策略引擎                     | 设定 1 主线 + 最多 2 支线，数量限制校验，优先级切换规则                                                                                                  |
| Done     | 12   | Web 前端   | Dashboard 项目分组视图                | 前端从扁平表格改为按 root 分组的视图，按优先级分区展示（主线/支线/普通/未归类）                                                                          |
| Done     | 13   | 注意力     | 提醒分数算法                          | 综合等待时长、专注持续、今日累计、项目优先级计算每个项目的提醒分数。设计文档：[`02-reminder_score_algorithm.md`](./design/02-reminder_score_algorithm.md) |
| Done     | 14   | 注意力     | 注意力遮罩层                          | 双维度设计：雾化遮罩（Fog）+ 高亮跑马灯（Marquee）。专注模式统一遮罩覆盖非关注区域。设计文档：[`03-attention_mask.md`](./design/03-attention_mask.md)     |
| Approved | 15   | 交互体验   | 项目折叠/展开                         | 记忆用户对普通项目和归档项目的折叠状态                                                                                                                   |
| Approved | 16   | 会话管理   | tmux 会话绑定同步                     | tmux 定期截图刷新 sessionId，`/clear` 和 `/resume` 或重启导致 session 绑定更换，同步到页面做重新绑定                                                 |
| Approved | 17   | 智能分析   | 军情哨分析建议（`pflow suggest`）   | 基于会话状态和历史数据，主动给出分析建议                                                                                                                 |
| Done     | 18   | Agent 管理 | 多 Agent 类型启动（`pflow hermes`） | 支持启动不同类型的 AI Agent（Claude Code 以外的其他 Agent）                                                                                              |
| Pending  | 19   | 通知系统   | 桌面通知                              | 分数超阈值时触发浏览器 Notification API                                                                                                                  |
| Pending  | 20   | 智能分析   | 军情哨主动推送                        | 后台守护进程主动推送提醒（需守护进程支持）                                                                                                               |
| Pending  | 21   | 智能分析   | 统帅偏好学习                          | 推送频率自适应，根据用户行为学习偏好                                                                                                                     |
| Done     | 22   | 扩展能力   | Hermes 会话扫描系统                   | 通过 `hermes sessions export` CLI 获取全量会话数据（含 LastReq/LastResp/时间戳/CWD），支持 source 过滤（默认排除 cron）、时间窗口过滤、项目匹配       |
| Pending  | 23   | 扩展能力   | 双层换肤系统                          | 内容层 + 遮罩层独立换肤，支持社区皮肤。设计文档已完成（[`99-dual_layer_skinning_system.md`](./design/99-dual_layer_skinning_system.md)）                  |
| Pending  | 24   | 扩展能力   | Web AI 平台状态监控                   | 浏览器扩展监控 DeepSeek/Kimi/ChatGPT 等平台的 AI 对话状态。设计文档已完成（[`99-ai-chat-web-attach.md`](./design/99-ai-chat-web-attach.md)）              |
| Pending  | 25   | 扩展能力   | 跨设备同步                            | 手机/平板看状态、点批准                                                                                                                                  |
| Pending  | 26   | Agent 管理 | 多 Agent 类型支持                     | 支持 Cline、Codex CLI 等其他 Agent 类型（远期预留）                                                                                                      |

---

## 详细说明

### 13. 提醒分数算法

综合等待时长、专注持续、今日累计、项目优先级计算每个项目的提醒分数。设计文档：[`docs/design/02-reminder_score_algorithm.md`](./design/02-reminder_score_algorithm.md)。保护期 15min 不打扰，幂函数差异化防止多任务同时高亮。

### 14. 注意力遮罩层

双维度设计：雾化遮罩（Fog，透明度随提醒分数变化）+ 高亮跑马灯（Marquee，边框动画）。专注模式统一遮罩覆盖非关注区域，颜色与页面背景一致。CSS 变量接口预留供后续换肤系统。设计文档：[`docs/design/03-attention_mask.md`](./design/03-attention_mask.md)。

### 22. Hermes 会话扫描系统

**已实现。** 通过 `hermes sessions export` CLI 获取全量会话数据（替代原计划的 SQLite 直读方案），以 JSONL 格式导出含 messages、system_prompt、last_active、source 等完整信息。支持：
- **会话 ID 去重**：hermes ID 前缀 8 位为日期（重复），改取后缀 8/16 位作为唯一标识（与 `hermes sessions list` 行为一致）
- **Source 过滤**：`--source` 参数按来源类型过滤（cli/weixin/cron），默认排除 cron
- **时间窗口过滤**：`-window` 参数限制最近活跃的会话
- **项目匹配**：从 system_prompt 提取 CWD，匹配到对应项目卡片
- **LastReq/LastResp 提取**：从 export 的 messages 数组提取最后一条 user/assistant 消息
- **Gateway 状态富化**：结合 sessions.json 补充 suspended/running 状态和 token 统计

### 23. 双层换肤系统

内容层 + 遮罩层独立换肤，支持社区皮肤。设计文档已完成（[`docs/design/99-dual_layer_skinning_system.md`](./design/99-dual_layer_skinning_system.md)），短期开发预留 CSS 变量接口即可。

### 24. Web AI 平台状态监控

浏览器扩展监控 DeepSeek/Kimi/ChatGPT 等平台的 AI 对话状态。设计文档已完成（[`docs/design/99-ai-chat-web-attach.md`](./design/99-ai-chat-web-attach.md)），需浏览器扩展开发 + 后端 WebSocket。
