## pflow 设计背后的参考理论知识

本文档汇总了 pflow 在功能设计、交互模式、任务管理策略中所借鉴的心理学、管理学、认知科学等领域的研究成果。每个条目对应一个具体的功能或设计决策，并说明其理论依据。

---

### 一、注意力管理与认知负担

| 功能/设计 | 参考理论 | 核心观点 | 来源 |
|-----------|----------|----------|------|
| `docs/backlog.md` 作为需求与计划的唯一交接点 | **情境认知（Situated Cognition）** + **外部认知卸载（External Cognitive Offloading）** | 将待办事项和实施上下文写入稳定的项目文档，释放工作记忆容量并支持跨会话协作 | Hutchins, 1995; Risko & Gilbert, 2016 |
| 当前工作范围从 backlog 中明确挑选 | **工作记忆容量限制** | 聚焦有限数量的候选任务，避免同时激活过多目标 | Miller, 1956; Cowan, 2001 |
| 周期完成后归档周期记录，backlog 保留未完成事项 | **情境折旧（Situational Decay）** | 及时移除过时的工作提示，保持当前交接信息有效 | 基于情境认知的延伸 |
| Dashboard 同时展示多个项目状态，但只高亮主线 | **注意力片段（Attention Fragments）** + **残留注意力（Attention Residue）** | 任务切换后仍有部分认知资源留在原任务；减少高亮项目可降低残留干扰 | Leroy, 2009; Mark et al., 2012 |

---

### 二、任务优先级与工作策略

| 功能/设计 | 参考理论 | 核心观点 | 来源 |
|-----------|----------|----------|------|
| 每天设定 1 个主线 + 最多 2 个支线 | **1-2 主线/支线策略** | 工作记忆容量有限（4±1），1 主线 + 2 支线保留合理多任务空间，避免分散 | 认知科学 |
| 主线项目拥有“免打扰”保护期 | **深度工作（Deep Work）** | 无干扰的专注状态才能产生高价值产出 | Newport, 2016 |
| 支线任务阻塞时渐进提醒 | **渐进提醒模型（Gradual Reminding）** | 提醒强度应随等待时间逐步升级，避免过度打断和任务遗忘 | 综合耶克斯-多德森定律、注意恢复理论、记忆提取-放弃模型 |
| 用户可延长专注时间（+15 分钟） | **番茄工作法（Pomodoro Technique）** | 以 25 分钟为工作单元，允许自定义调节 | Cirillo, 2006 |
| 全提醒等于没提醒，只对最高分任务发强提醒 | **信息过载（Information Overload）** + **决策疲劳（Decision Fatigue）** | 同时多个高强度提醒会导致用户忽略所有提醒 | 行为经济学 |

---

### 三、授权与信任

| 功能/设计 | 参考理论 | 核心观点 | 来源 |
|-----------|----------|----------|------|
| Claude Code 的 `acceptEdits` 模式 | **信任粒度（Granular Trust）** + **例外管理（Management by Exception）** | 预设低风险操作的自动批准，仅对高风险或异常请求人工确认 | 管理学；Drucker, 1954 |
| 批量授权（累积多个请求后一次性确认） | **任务批处理（Task Batching）** | 将多个相似操作合并为一次决策，减少上下文切换次数 | 时间管理/敏捷开发 |
| 用户在 `settings.json` 自定义允许/拒绝规则 | **白名单/黑名单策略** | 明确授权边界，AI 在边界内自主执行 | 计算机安全/权限管理 |

---

### 四、工作流与团队协作映射

| 功能/设计 | 参考理论 | 核心观点 | 来源 |
|-----------|----------|----------|------|
| 用户委派 AI 执行子任务，异步审核结果 | **授权与监督（Delegation & Oversight）** | 领导设定目标，下属执行，领导只在例外时介入 | 管理学 |
| 多个 AI 会话视为“下属”，用户作为“领导” | **多代理协作（Multi-Agent Coordination）** | 一个协调者管理多个并行工作的智能体 | 分布式人工智能 |
| 批量审阅 AI 输出结果 | **冲刺评审（Sprint Review）** | 固定时间窗口集中验收成果，而非实时盯梢 | 敏捷开发 (Scrum) |
| 任务分解为最小可执行单元 | **原子化任务（Atomic Task）** | 拆分复杂任务为 3-5 分钟可完成的小块，便于异步执行和碎片时间利用 | GTD (Getting Things Done) |

---

### 五、用户界面与体验

| 功能/设计 | 参考理论 | 核心观点 | 来源 |
|-----------|----------|----------|------|
| 通过 ttyd 提供 Web 终端而非自建 PTY | **技术债务避免（Tech Debt Avoidance）** | 复用成熟开源工具，减少重复造轮子和维护成本 | 软件工程 |
| 浏览器插件监控 DOM 而非抓包 | **最小权限原则（Principle of Least Privilege）** | 只读取 UI 状态，不拦截网络请求，降低隐私风险和合规难度 | 计算机安全 |
| Dashboard 默认折叠普通项目 | **渐进式信息披露（Progressive Disclosure）** | 先展示最重要信息，次要信息按需展开 | 交互设计 (Nielsen) |
| 提醒强度用进度条/颜色/声音分级 | **多模态反馈（Multimodal Feedback）** | 视觉、听觉、触觉不同通道传递不同紧急程度 | 人机交互 |

---

### 六、认知偏差与干预

| 功能/设计 | 参考理论 | 核心观点 | 来源 |
|-----------|----------|----------|------|
| 用户可设置主线/支线，但系统会提醒分配时间 | **计划谬误（Planning Fallacy）** | 人们倾向于低估任务耗时、高估自己处理多任务的能力；系统辅助校准 | Kahneman & Tversky, 1979 |
| 提醒分数考虑“今日累计时间”矫正 | **累积偏差校正（Cumulative Bias Correction）** | 若支线任务实际耗时超过主线，系统主动引导回归主线 | 行为经济学 |
| 无活跃任务时只提醒主线 | **单一焦点原则（Single Focal Point）** | 当用户没有主动工作，系统应帮助明确最高优先级，避免决策瘫痪 | 心理学 |

---

### 七、被否决方案背后的理论

| 被否决方案 | 参考理论 | 否决原因 |
|------------|----------|----------|
| 网络层抓包监控 AI 网页状态 | **HTTPS 中间人攻击风险** | 需要安装 CA 证书，违反安全原则；平台可检测并封禁 |
| 解析 Claude TUI 提取权限请求 | **UI 的不稳定性** | 终端渲染依赖 ANSI 码和布局，版本更新容易破坏解析规则 |
| pflow 自建 WebSocket + PTY 桥接 | **成熟轮子原则** | ttyd 已稳定实现，自研成本高且易出边缘 bug |
| 总是同意所有授权（全自动模式） | **安全边际（Margin of Safety）** | AI 误操作风险不可接受，必须保留人工确认环节 |

---

### 八、总结

pflow 的设计并非凭空想象，而是植根于多学科理论基础之上。理解这些理论可以帮助用户更好地使用工具，并预判未来功能迭代的方向。如需深入阅读，推荐以下书籍：

- 《思考，快与慢》—— Daniel Kahneman
- 《深度工作》—— Cal Newport
- 《心流》—— Mihaly Csikszentmihalyi
- 《Getting Things Done》—— David Allen
- 《The Overflowing Brain》—— Torkel Klingberg

> 注：本文档将作为 pflow 内置帮助系统的理论附录，随版本更新补充。

---

## 九、Claude Code statusLine 技术参考

> 本节保留 pflow 早期 `pflow claude` 的基础技术验证记录。当前实现改用
> `~/.claude/sessions/` 中的结构化元数据关联 tmux 与 Claude 会话，不读取或修改
> `~/.claude/settings.json`，也不依赖 statusLine。会话关联的当前设计见
> [`tech.md`](tech.md#54-web-终端集成ttyd--tmux--claude-关联已实现)。

Claude Code 会把当前会话的 JSON 数据通过 stdin 传给 `statusLine.command`，命令的 stdout 会显示在终端状态栏。该机制可用于显示模型、上下文使用率和会话 ID 前缀，也可作为调试 Claude 提供数据的入口。

### 配置位置与关键字段

- 配置位于 `~/.claude/settings.json` 的 `statusLine` 字段。
- 常用字段包括：`session_id`、`model.display_name`、`effort.level`，以及 `context_window.current_usage` 和 `context_window.used_percentage`。
- 会话原始记录还会提供 `transcript_path`、`cwd`、`session_name`、版本、成本和 `context_window_size` 等信息；字段可能在会话刚开始或尚未调用 API 时缺失。
- 调试时可先将 stdin 的实际内容落盘：

```bash
cat | tee /tmp/statusline-dump.json > /dev/null
```

之后使用 `jq` 或 `python3 -m json.tool` 查看该文件，再以同一份 JSON 单独测试 statusLine 命令。

以下是早期调试得到的代表性输入结构，具体字段会随 Claude Code 版本和会话状态变化：

```jsonc
{
  "session_id": "911b2385-ee8d-4956-a347-65d3dbecc16a",
  "transcript_path": "/root/.claude/projects/-root-code-pflow/<session>.jsonl",
  "cwd": "/root/code/pflow",
  "effort": { "level": "high" },
  "session_name": "Implement core todo list with mock data",
  "model": { "id": "deepseek-v4-pro", "display_name": "deepseek-v4-pro" },
  "workspace": {
    "current_dir": "/root/code/pflow",
    "project_dir": "/root/code/pflow",
    "added_dirs": [],
    "repo": { "host": "github.com", "owner": "pancake-lee", "name": "pflow" }
  },
  "version": "2.1.169",
  "output_style": { "name": "default" },
  "cost": {
    "total_cost_usd": 30.745574,
    "total_duration_ms": 138652104,
    "total_api_duration_ms": 3336229,
    "total_lines_added": 3384,
    "total_lines_removed": 1577
  },
  "context_window": {
    "total_input_tokens": 53185,
    "total_output_tokens": 328,
    "context_window_size": 200000,
    "current_usage": {
      "input_tokens": 65,
      "output_tokens": 328,
      "cache_creation_input_tokens": 0,
      "cache_read_input_tokens": 53120
    },
    "used_percentage": 27,
    "remaining_percentage": 73
  },
  "exceeds_200k_tokens": false,
  "fast_mode": false,
  "thinking": { "enabled": true }
}
```

### 累计 token 的计算

`context_window.total_input_tokens` 和 `total_output_tokens` 描述的是当前上下文窗口，不是整个会话的累计 API 消耗：历史压缩、缓存读取和新一轮对话都会影响它们，`total_output_tokens` 还可能在用户发新消息时归零。

若要显示累计输入/输出 token，应对每次 statusLine 调用的 `current_usage.input_tokens` 与 `current_usage.output_tokens` 增量累加，并按 session ID 保存上一次值与累计值。早期验证使用 `/tmp/claude_tokens_<session-id 前 8 位>` 作为临时状态文件；这种文件在重启或清理 `/tmp` 后会重新计数。

### 示例：显示会话、模型和累计 token

下面是早期验证使用的 `statusLine.command`。价格为当时的本地展示参数，不能视为当前模型价格或计费依据；如继续使用，应自行更新 `cus_input_price` 和 `cus_output_price`。

```json
{
  "statusLine": {
    "type": "command",
    "command": "input=$(cat); sid=$(echo \"$input\" | jq -r '.session_id // \"-\"'); sid8=$(printf \"%.8s\" \"$sid\"); model=$(echo \"$input\" | jq -r '.model.display_name // empty'); used=$(echo \"$input\" | jq -r '.context_window.used_percentage // empty'); effort=$(echo \"$input\" | jq -r '.effort.level // empty'); curr_in=$(echo \"$input\" | jq -r '.context_window.current_usage.input_tokens // 0'); curr_out=$(echo \"$input\" | jq -r '.context_window.current_usage.output_tokens // 0'); cus_input_price=3; cus_output_price=6; token_file=\"/tmp/claude_tokens_${sid8}\"; if [ ! -f \"$token_file\" ]; then echo \"0 0 0 0\" > \"$token_file\"; fi; read last_sum_in last_sum_out last_curr_in last_curr_out < \"$token_file\"; new_sum_in=$last_sum_in; new_sum_out=$last_sum_out; if [ \"$curr_in\" -gt \"$last_curr_in\" ]; then delta_in=$((curr_in - last_curr_in)); new_sum_in=$((last_sum_in + delta_in)); fi; if [ \"$curr_out\" -gt \"$last_curr_out\" ]; then delta_out=$((curr_out - last_curr_out)); new_sum_out=$((last_sum_out + delta_out)); fi; echo \"$new_sum_in $new_sum_out $curr_in $curr_out\" > \"$token_file\"; cost_in=$(awk 'BEGIN {printf \"%.4f\", '$new_sum_in' * '$cus_input_price' / 1000000}'); cost_out=$(awk 'BEGIN {printf \"%.4f\", '$new_sum_out' * '$cus_output_price' / 1000000}'); [ -n \"$used\" ] && ctx=\"$used%\" || ctx=\"--\"; [ -n \"$effort\" ] && effort_str=\"$effort\" || effort_str=\"--\"; [ -n \"$model\" ] && echo \"${sid8} | ${model} | ${effort_str} | ctx ${ctx} | total:${new_sum_in}/${new_sum_out} | cos:${cost_in}/${cost_out}\" || echo \"${sid8} | ${effort_str} | ctx ${ctx} | total:${new_sum_in}/${new_sum_out} | cos:${cost_in}/${cost_out}\""
  }
}
```

### 排查要点

- `jq` 不属于所有系统的预装工具。缺失时，解析命令会返回空值，状态栏常表现为 `ctx --` 和空的 token 字段。先安装 `jq` 或改用可用的 JSON 解析器。
- 调试阶段不要把 `jq` 的 stderr 重定向到 `/dev/null`，否则 `command not found` 等根因会被隐藏。
- 用 dump 的 JSON 单独验证命令时，可执行 `cat /tmp/statusline-dump.json | bash -c '你的statusLine命令'`。
- `settings.json` 内嵌 shell 命令的引号转义容易出错。若要程序化修改，使用 JSON 序列化写入并保持原子替换，避免手工拼接转义字符串。
- 对可能缺失的字段使用 `// empty` 或 `// 0` 等默认值，避免首次渲染或空会话导致整条状态栏失败。
