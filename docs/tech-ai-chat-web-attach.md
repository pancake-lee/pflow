## pflow Web AI 平台状态监控技术方案

### 1. 概述

pflow 已支持本地 CLI Agent（Claude Code、Hermes）的状态监控与终端集成。为扩展监控范围，本方案实现对**浏览器中运行的 AI 聊天平台**（如 DeepSeek、Kimi、ChatGPT 等）的状态监控，统一展示在 Dashboard 上。

**核心需求**：
- 监控用户在浏览器中与 AI 对话时的状态（`thinking` → `complete`）。
- 不读取聊天内容，仅检测页面上的 UI 状态指示器（如“正在输入”图标、停止生成按钮等）。
- 用户无感工作：无需改变浏览习惯，插件自动运行。
- 状态变化实时上报到 pflow 后端，Dashboard 统一展示。

### 2. 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│  用户浏览器 (Chrome/Edge)                                   │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  AI 聊天页面 (DeepSeek/Kimi/...)                     │  │
│  │  - 状态指示器 DOM 元素                                │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │ DOM 变化监听 (MutationObserver)   │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  pflow 浏览器扩展                                     │  │
│  │  - Content Script (状态检测)                         │  │
│  │  - Background Service Worker (状态聚合与上报)        │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                          │ WebSocket / HTTP
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  pflow 后端 (服务器)                                        │
│  - 接收状态更新，存储会话状态                               │
│  - 提供 Dashboard API                                       │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Dashboard (Web UI)                                         │
│  - 展示本地 Agent 会话 + 浏览器 AI 会话统一面板             │
└─────────────────────────────────────────────────────────────┘
```

### 3. 浏览器扩展设计

#### 3.1 扩展结构

采用 Manifest V3，包含以下核心文件：

```
extension/
├── manifest.json          # 扩展配置
├── background.js          # 后台 Service Worker
├── content-script.js      # 注入页面的脚本（状态检测）
├── options.html           # 选项页（用户自定义选择器）
└── icons/                 # 图标
```

#### 3.2 `manifest.json` 关键配置

```json
{
  "manifest_version": 3,
  "name": "pflow AI State Monitor",
  "version": "0.1.0",
  "permissions": ["storage", "tabs", "webRequest"],
  "host_permissions": [
    "https://chat.deepseek.com/*",
    "https://kimi.moonshot.cn/*",
    "https://chat.openai.com/*",
    "https://claude.ai/*"
  ],
  "background": {
    "service_worker": "background.js"
  },
  "content_scripts": [
    {
      "matches": [
        "https://chat.deepseek.com/*",
        "https://kimi.moonshot.cn/*",
        "https://chat.openai.com/*",
        "https://claude.ai/*"
      ],
      "js": ["content-script.js"],
      "run_at": "document_idle"
    }
  ],
  "options_ui": {
    "page": "options.html",
    "open_in_tab": false
  }
}
```

#### 3.3 状态检测逻辑 (`content-script.js`)

```javascript
// 平台配置：域名 -> 选择器映射
const platformConfig = {
  'chat.deepseek.com': {
    thinkingSelector: '.ds-message-typing-indicator',   // 思考中元素
    completeCheck: () => !document.querySelector('.ds-message-typing-indicator')
  },
  'kimi.moonshot.cn': {
    thinkingSelector: '.kimi-thinking',
    completeCheck: () => !document.querySelector('.kimi-thinking')
  },
  'chat.openai.com': {
    thinkingSelector: '.result-streaming, .icon-loading',
    completeCheck: () => !document.querySelector('.result-streaming')
  },
  'claude.ai': {
    thinkingSelector: '.message-streaming',
    completeCheck: () => !document.querySelector('.message-streaming')
  }
};

let currentStatus = 'idle';
let observer = null;

function detectStatus() {
  const host = window.location.hostname;
  const config = platformConfig[host];
  if (!config) return;

  const isThinking = document.querySelector(config.thinkingSelector) !== null;
  const newStatus = isThinking ? 'thinking' : 'complete';
  
  if (newStatus !== currentStatus) {
    currentStatus = newStatus;
    // 发送状态到后台
    chrome.runtime.sendMessage({
      type: 'STATUS_UPDATE',
      tabId: ???,        // 后台脚本可通过 sender.tab.id 获取
      status: currentStatus,
      url: window.location.href,
      timestamp: Date.now()
    });
  }
}

// 使用 MutationObserver 监听 DOM 变化
observer = new MutationObserver(detectStatus);
observer.observe(document.body, { childList: true, subtree: true, attributes: true });
detectStatus(); // 立即检测一次
```

#### 3.4 后台服务 (`background.js`)

```javascript
let ws = null;
let serverUrl = 'ws://localhost:8080/ws';  // 可配置

function connectWebSocket() {
  ws = new WebSocket(serverUrl);
  ws.onopen = () => console.log('pflow: WebSocket connected');
  ws.onclose = () => setTimeout(connectWebSocket, 5000);
}

connectWebSocket();

// 接收来自 content script 的状态更新
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type === 'STATUS_UPDATE' && ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({
      sessionId: `web-${sender.tab.id}`,
      platform: new URL(message.url).hostname,
      status: message.status,
      timestamp: message.timestamp
    }));
  }
});
```

### 4. 后端接口与数据模型

#### 4.1 WebSocket 消息格式

```json
{
  "sessionId": "web-12345",
  "platform": "chat.deepseek.com",
  "status": "thinking",
  "timestamp": 1700000000000
}
```

后端将消息转换为内部会话对象，存储在内存或数据库中，并通过 Dashboard 的 WebSocket 广播给前端。

#### 4.2 Dashboard 展示

在 Dashboard 中，浏览器 AI 会话与本地 Agent 会话并列显示，用 `type` 字段区分：

```json
{
  "id": "web-12345",
  "type": "web",
  "name": "DeepSeek Chat",
  "status": "thinking",
  "lastActive": "2025-06-13T10:00:00Z",
  "url": "https://chat.deepseek.com"
}
```

前端可提供“打开标签页”按钮，方便用户快速跳转到该会话。

### 5. 用户自定义选择器（高级功能）

为解决平台改版或小众平台的适配问题，提供选项页让用户自定义选择器。

#### 5.1 选项页 UI

- 输入域名（如 `example.com`）
- 输入“思考中”选择器（CSS 选择器或 XPath）
- 提供测试按钮：高亮匹配元素
- 保存配置到 `chrome.storage.sync`

#### 5.2 自定义配置的加载

在 `content-script.js` 中，优先读取存储的自定义配置，若无则使用内置配置。

```javascript
chrome.storage.sync.get([host], (result) => {
  const config = result[host] || platformConfig[host];
  // 使用 config 进行检测
});
```

### 6. 隐私与安全设计

- **最小权限原则**：仅请求访问必要的 AI 平台域名。
- **数据保护**：不上传任何聊天内容，只传输状态（thinking/complete）和 URL 域名。
- **用户控制**：提供全局开关，用户可随时禁用监控（在扩展图标菜单中）。
- **开源透明**：所有代码开源，供用户审计。

### 7. 部署与维护

#### 7.1 扩展发布

- 打包 `.crx` 文件，发布到 Chrome 网上应用店（需支付一次性开发者注册费）。
- 也可提供 `.zip` 供用户手动加载（开发者模式）。

#### 7.2 平台适配维护

- 内置主流平台选择器，定期检查平台更新。
- 建立社区规则仓库，允许用户提交新平台配置。
- 插件自动更新（通过 Chrome 商店）可及时推送新适配。

### 8. 后续扩展方向

- **自动响应**：检测到权限请求或完成状态后，自动执行用户预设动作（如发送通知、复制结果）。
- **用量统计**：记录每个对话的响应时间、token 消耗（如果页面上有显示）。
- **跨设备同步**：通过 pflow 云端账号同步自定义选择器配置。

### 9. 总结

本方案利用浏览器扩展 + DOM 监听技术，实现了对 Web AI 平台状态的零干扰监控，与已有的本地 Agent 监控无缝整合。技术难度低，用户无感，隐私风险可控。该模块将作为 pflow 的重要组成部分，扩展产品对 AI 工作流的覆盖范围。