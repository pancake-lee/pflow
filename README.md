# pflow

[English](./README.en.md) | 中文

多 Agent 会话的注意力管理与调度工具

## 你是否也遇到过这些问题？

- Claude 在处理代码，Hermes 在写文档。你在等谁先完成，该去哪个窗口看？
- 切到其他窗口忙了一会儿，回来忘了哪个 Agent 正在等你授权，只能逐屏翻找。
- 多个项目的 AI 会话散落在不同终端、不同 tmux 中，没有一个统一的视图能看到“全盘战况”。

pflow 把这些 AI 会话的状态汇聚到一张仪表盘上，让你像将军看沙盘一样，一眼看清哪路兵在打、哪路兵在等、该去哪路支援。

知识卡候选包含：当前全部行动建议关联的知识（去重）、随机 2 条未重复的内置知识、随机 2 条自定义知识，总数不限。每类不足 2 条时取全部可用内容。候选选好后统一打乱一次，后续轮播和翻页保持该顺序。打开设置中的「自定义知识点」，填写标题、知识正文和可选的应用说明，点击「保存设置」，即可将内容加入自定义知识池，独立抽选，不受关联知识数量影响。支持编辑和删除，内容保存在服务端 `~/.pflow/settings.json`。

## Dashboard 预览

![Dashboard 截图](./docs/screenshot.png)

## 快速开始

```bash
# 打开 Web Dashboard（浏览器访问 http://localhost:10002）
pflow serve

# 在当前项目启动 Claude 托管会话（tmux 自动配置）
pflow claude -dir .

# 在当前项目启动 Codex 托管会话
pflow codex -dir .

# 查看今日军情建议（专注/切换/授权提醒）
pflow suggest
```

pflow 托管会话会在运行时配置鼠标滚动：应用自行处理鼠标时透传滚轮，否则使用 tmux 历史滚动。无需修改 `~/.tmux.conf`，其他 tmux 会话沿用原有设置。旧托管会话可先按 `Ctrl+B`、`D` 分离，再用新版 pflow 的原启动命令接入，无需退出 Agent。

## Windows 桌面版

桌面版在原生 Windows 上运行，通过 HTTP/HTTPS 连接 Linux 上的 `pflow serve`。Agent 扫描、项目、日程与偏好在 Linux 端处理，exe 仅保存连接地址和日志。本轮支持单个服务，本地 Agent 与多个远端融合展示的后续方向见 [Future #55](./docs/design/99_future.md)。

在 Linux 构建环境安装系统 Go、pnpm 与 MinGW-w64 交叉编译器（`x86_64-w64-mingw32-gcc`），安装项目依赖后执行：

```bash
make build-windows
```

产物为 `bin/pflow.exe`，复制到 Windows 后双击，首次填写服务地址并点击「保存并连接」。Windows 需要 WebView2 Runtime，无需 Go、Node.js 或项目源码。连接地址保存于 `%USERPROFILE%\.pflow\desktop.json`，日志位于 `%USERPROFILE%\.pflow\logs\desktop.log`；关闭窗口即退出。

- 本机 Docker：在 Agent 所在容器运行 `pflow serve`，创建容器时将服务端口映射至宿主机，例如 `-p 127.0.0.1:10002:10002`，桌面填写 `http://127.0.0.1:10002`。pflow 与 Agent 同容器运行可正确读取数据、查询 PID 和访问 tmux。
- 本机 WSL：在 Agent 所在发行版运行 `pflow serve`，桌面通常填写 `http://127.0.0.1:10002`。
- 远程 Linux：填写可达的受保护服务地址，或先在 Windows 终端建立已有 SSH 隧道，再填写转发后的地址。例如 `ssh -N -L 127.0.0.1:11002:127.0.0.1:10002 user@host`，桌面填写 `http://127.0.0.1:11002`。此处远端 `10002` 必须是 SSH 主机可访问的服务端口；Ctrl+C 停止隧道，桌面不管理 SSH 进程。

通过窗口顶部「连接设置」切换服务；网络不可达时 Dashboard 显示错误，仍可修改地址。当前 API 不提供认证，应使用本机映射、已有受保护网络或 SSH 隧道。桌面版暂不提供 Web 终端，请通过已有终端操作 Agent，Linux 浏览器版保留终端功能。

## 核心体验

| 场景 | pflow 怎么做 |
|------|-------------|
| 启动一个 Agent 会话 | `pflow claude -dir .` → tmux + Agent 自动配置，Dashboard 立即出现 |
| 多会话状态一目了然 | Dashboard 红绿灯：🟢交战中 / 🟡待命 / ⚪休整 |
| Agent 需要授权 | 点一下侧边栏，Web 终端直接 attach 到同一 tmux 会话，按 1 确认 |
| 今天该专注什么 | 主线/支线策略 + 提醒分数算法 → `pflow suggest` 告诉你下一步 |

## 为什么用 pflow

- **不是又一个 Agent**：pflow 不做对话，只做调度。它让你同时用好多个 Agent，而不是在它们之间消耗精力。
- **无感工作流**：在终端里正常用 Claude Code、Hermes，它们自动出现在 Dashboard 上——不需要改变你的使用习惯。
- **一键直达**：从“看到状态”到“操作会话”只需要一次点击，不用复制粘贴命令或手动 tmux attach。

## 已实现的核心能力

- **多 Agent 监控**：Claude Code、Hermes、Codex CLI 会话统一展示
- **项目策略管理**：主线/支线任务映射 + 提醒分数算法 + 注意力遮罩
- **Web 终端集成**：Dashboard 侧边栏一键 attach 到 Agent 会话（基于 ttyd + tmux）
- **注意力引导**：专注模式保护期 + 行动建议引擎（~20 个静态场景）
- **完整文档**：PRD、技术设计、需求池、测试策略齐全

## 技术栈

| 层级 | 技术 |
|------|------|
| CLI 框架 | Go 标准库（手写子命令分发） |
| Web Dashboard | Vue 3 + Naive UI（Go embed 单二进制部署） |
| 后端 | Go（tmux 管理、状态扫描、session 映射） |
| Agent 集成 | Claude Code（statusline + JSON 扫描）、Hermes（export 解析） |

## 路线图

- [x] 双 Agent 状态监控 + Web Dashboard
- [x] 项目策略管理 + 提醒分数算法 + 注意力遮罩
- [x] Web 终端集成 + 专注模式
- [x] 行动建议引擎（静态场景）
- [ ] 桌面通知（浏览器 Notification API）
- [ ] 行动建议主动推送 + 偏好学习
- [ ] 双层换肤系统

## 文档

| 文档 | 用途 |
|------|------|
| [`docs/prd.md`](./docs/prd.md) | 产品需求文档 |
| [`docs/tech.md`](./docs/tech.md) | 技术设计方案 |
| [`docs/backlog.md`](./docs/backlog.md) | 全部需求池 |
| [`docs/harness.md`](./docs/harness.md) | AI 辅助开发工作流与文档入口 |
| [`docs/screenshot.png`](./docs/screenshot.png) | 最新截图 |

## License

MIT
