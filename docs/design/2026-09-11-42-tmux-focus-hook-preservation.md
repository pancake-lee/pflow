# #42 tmux 焦点 hook 保留修复

> 本文档是 [#42 项目计时](../backlog.md#42-基于-tmux-焦点事件的项目计时) 首轮评估后的修复方案。

## 目标

让 pflow 注册和退出时只管理自己的 `client-focus-in/out` hook，不替换、删除或重复用户已有 hook。

## 方案

- 注册前读取全局 hook 列表，以 pflow 脚本绝对路径识别已有 pflow hook；存在时不重复追加。
- 通过 tmux 的 append 语义向 hook 数组增加 pflow hook，使同名用户 hook 继续保留。
- 清理时仅解析并删除包含 pflow 脚本路径的数组索引，不使用未带索引的删除命令。
- 以纯文本解析函数覆盖：无 hook、用户 hook、已有 pflow hook、混合 hook 和多个数组索引。
- 聚合测试补上 A→B 切换和超时同项目恢复，继续以消息估算作为无有效数据时的回退。

## 验收

- 注册和清理前后，用户的同名 hook 命令逐字保留。
- 重启 `serve` 后每种 pflow focus hook 恰有一条。
- 跨项目切换立即断开，5 分钟内仅同项目恢复才合并。
