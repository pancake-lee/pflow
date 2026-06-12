## 方案确认与分析

你已经通过 Claude 的 `/statusline` 功能，将 session ID 的前 8 个字符作为状态栏前缀显示在终端上。这意味着：**在 tmux 的任何时刻，只要 capture-pane 获取到当前屏幕内容，就能解析出这个前缀，从而唯一确定 Claude 内部 session 的标识符（至少前 8 位）**。

这是一个**极其聪明且实用的方案**，直接绕过了 pflow 需要主动获取 session ID 的难题。现在 pflow 只需做两件事：

1. **启动 Claude 时**：确保 Claude 的 status line 已配置为显示 session ID 前缀（一次性配置，用户可提前在 Claude 的设置中固化）。
2. **定期扫描**：通过 `tmux capture-pane` 获取每个托管会话的终端输出，用正则提取出类似 `^[a-f0-9]{8}` 这样的前缀，并与 pflow 记录的 `tmux session 名` 建立映射。

---

## 技术可行性分析

### 优点 ✅

- **无需解析复杂 JSON 或进程间通信**：只需简单的文本正则匹配。
- **实时性高**：capture-pane 可以在任意时刻获取状态，即使在权限请求等待期间，status line 也一直存在。
- **容错性强**：即使 Claude 内部 session ID 变化（实际上不会变），也可以定期更新映射。
- **用户透明**：用户只需配置一次 status line，后续所有会话自动包含该信息。
- **不依赖外部工具**：纯 tmux + 文本解析，无额外依赖。

### 注意事项 ⚠️

1. **Claude status line 的稳定性**：
   - `/statusline` 定制的内容是否会在 Claude 更新后改变格式？目前官方文档显示该功能是稳定的，但需留意版本升级。
   - 前缀是否始终显示在屏幕固定位置（通常是底部状态行）？capture-pane 需要捕获到包含该前缀的行。

2. **解析的可靠性**：
   - 需要设计健壮的正则表达式，匹配类似 `^[a-f0-9]{8}` 的模式，且能区分其他可能的数字串。
   - 最好匹配 `|` 前后的内容，因为 status line 典型格式：`c50e1b2e | deepseek-v4-pro | ctx ...`。可以提取第一个 `|` 之前的字符串。

3. **多 session 同时运行的区分**：
   - 每个 tmux session 独立捕获，映射关系是 `tmux session name` ↔ `claude prefix`。只要每个 tmux session 中运行的 Claude 有不同的 session ID，解析出的前缀就不同，可以区分。

4. **初始化时机**：
   - 当 pflow 启动 Claude 后，需要等待足够时间让 Claude 完全启动并显示 status line。建议启动后延迟 2-3 秒再进行首次捕获。
   - 可以轮询直到成功解析出前缀为止。

5. **Edge Cases**：
   - 如果用户通过 `tmux attach` 手动切换窗口，pflow 可能 capture 到其他 pane 的内容。应指定 pane：`tmux capture-pane -t <session>:0.0`。
   - 如果 Claude 崩溃或未正常启动，status line 不存在，需要超时处理。

---

## 实现方案建议

### 1. 配置 Claude status line（一次性）

程序自动帮用户设置以下配置到settings.json，
如果用户已经有了自己的statusline配置，那么需要提醒用户，给用户选择是否覆盖，
如果选择否，则告知用户需要在statusline最前面展示session的前8位。
否则dashboard无法打开对应的tmux，这也意味着dashboard打开tmux进行授权交互的功能，是可选的。
dashboard打开详情是，如果对tmux当前内容截取后无法解析到匹配的session(8)，则提示用户找不到即可，
这里也就过滤掉了原生终端启动claude，自建tmux启动claude，tmux的claude退出又重进等等各种意外路径，
我们只处理通过pflow+tmux+claude启动的并且用户配合设置好了statusline的情况即可。

```json
// ~/.claude/settings.json
"statusLine": {
    "type": "command",
    "command": "input=$(cat); sid=$(echo \"$input\" | jq -r '.session_id // \"-\"'); sid8=$(printf \"%.8s\" \"$sid\"); model=$(echo \"$input\" | jq -r '.model.display_name // empty'); used=$(echo \"$input\" | jq -r '.context_window.used_percentage // empty'); rem=$(echo \"$input\" | jq -r '.context_window.remaining_percentage // empty'); in_tok=$(echo \"$input\" | jq -r '.context_window.current_usage.input_tokens // 0'); out_tok=$(echo \"$input\" | jq -r '.context_window.current_usage.output_tokens // 0'); total_in=$(echo \"$input\" | jq -r '.context_window.total_input_tokens // 0'); total_out=$(echo \"$input\" | jq -r '.context_window.total_output_tokens // 0'); [ -n \"$used\" ] && [ -n \"$rem\" ] && ctx=\"ctx ${used}%/ ${rem}%\" || ctx=\"ctx --\"; tok=\"in:${in_tok} out:${out_tok}\"; session=\"total:${total_in}/${total_out}\"; [ -n \"$model\" ] && echo \"${sid8} | ${model} | ${ctx} | ${tok} | ${session}\" || echo \"${sid8} | ${ctx} | ${tok} | ${session}\""
}
```

### 2. pflow 启动新的托管会话

```go
func (m *SessionManager) StartSession(name, workDir string) (*ManagedSession, error) {
    // 1. 创建 tmux session，运行 claude
    tmuxName := name // 或生成唯一名
    cmd := exec.Command("tmux", "new-session", "-d", "-s", tmuxName, "-c", workDir, "claude")
    if err := cmd.Run(); err != nil {
        return nil, err
    }

    // 2. 等待 Claude 启动并解析 session 前缀
    var claudePrefix string
    for i := 0; i < 10; i++ { // 最多等待 10 秒
        time.Sleep(1 * time.Second)
        output, err := exec.Command("tmux", "capture-pane", "-t", tmuxName, "-p").Output()
        if err != nil {
            continue
        }
        prefix := parseClaudeSessionPrefix(string(output))
        if prefix != "" {
            claudePrefix = prefix
            break
        }
    }
    if claudePrefix == "" {
        // 可选：启动失败，清理 tmux
        exec.Command("tmux", "kill-session", "-t", tmuxName).Run()
        return nil, errors.New("failed to detect claude session prefix")
    }

    // 3. 保存映射
    sess := &ManagedSession{
        ID:            tmuxName,
        ClaudePrefix:  claudePrefix,
        WorkDir:       workDir,
        TmuxSession:   tmuxName,
    }
    // 存储到持久化
    return sess, nil
}

func parseClaudeSessionPrefix(output string) string {
    // 匹配行首为 8 位十六进制，后跟空格或竖线
    re := regexp.MustCompile(`(?m)^([a-f0-9]{8})\s*[| ]`)
    matches := re.FindStringSubmatch(output)
    if len(matches) >= 2 {
        return matches[1]
    }
    return ""
}
```

### 3. Dashboard 映射和唤起tmux

- 当用户对某个条目点击，打开详情侧边栏时，代码尝试查找对应的tmux，如果能匹配成功，则在元数据后面展示一个按钮，点击后再弹出一个终端交互界面

### 4. 关联到 Claude 内部 session ID

解析出的 8 位前缀并不是完整的 UUID，但足以在同一项目内的多个会话间区分。如果后续需要完整的 session ID（如通过 `claude --resume <full-id>`），pflow 可以在启动时额外从 `~/.claude/sessions/` 目录中查找匹配该前缀的最新文件，获得完整 ID。或者，用户可以通过 Web 终端手动执行 `claude sessions` 命令查看完整 ID。

---
