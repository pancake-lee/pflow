# 知识锚点（Knowledge Anchor）开发记录

**日期**: 2026-06-25  
**关联设计**: [`docs/design/10-tips.md`](../design/10-tips.md)  
**关联需求**: Backlog #36

## 实现概述

实现了 Dashboard 右下角的知识锚点卡片功能，展示军情建议的认知科学理论依据。

## 关键决策

### Scenario ID 代码化

设计文档中的 `scenario_001` ~ `scenario_020` 原本只存在于文档层面。本次实现将其落地为 `Suggestion.ScenarioID` 字段，每个 check 函数返回时填充对应的 scenario_id。

### 映射方案选择

采用方案 A（后端驱动）：Go suggest 引擎输出 scenario_id，API 层通过映射表注入 knowledge_tip，前端直接从 API 响应中获取。

### checkPrimaryIdle 参数变更

`checkPrimaryIdle` 被 S3 和 S4 共用，原来通过 `icon` 参数区分。本次新增 `scenarioID string` 参数来区分 scenario_003 和 scenario_004。

## 改动文件

| 文件 | 改动 |
|------|------|
| `internal/suggest/suggest.go` | Suggestion 加 ScenarioID；20 个 check 函数各加 ID；新增 KnowledgeTip 类型、12 条数据、init 索引、LookupTip 函数 |
| `internal/api/server.go` | SuggestionJSON 加 scenario_id + knowledge_tip；generateSuggestions 注入 knowledge tip |
| `web/src/types/dashboard.ts` | 新增 KnowledgeTip 接口；Suggestion 加 scenario_id + knowledge_tip 字段 |
| `web/src/config/knowledge-tips.ts` | 新建，12 条知识内容 + GENERIC_TIPS 常量 |
| `web/src/components/KnowledgeAnchor.vue` | 新建，右下角毛玻璃卡片组件 |
| `web/src/views/DashboardView.vue` | 集成 KnowledgeAnchor 组件 |

## 组件状态逻辑

- **关联态**：API 返回的 suggestion 带 knowledge_tip → 显示对应 tip，不轮播
- **默认态（轮播）**：无 knowledge_tip → 从 GENERIC_TIPS 中每 15 秒轮播
- **悬停态**：暂停轮播，显示上下翻页按钮
- **手动翻阅**：暂停自动切换 30 秒后恢复
