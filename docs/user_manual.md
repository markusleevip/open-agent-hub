# Open Agent Hub — 用户使用手册

> **版本：** v0.2  
> **更新时间：** 2026-06-08  
> **适用对象：** 开发者、技术团队主管、AgentOps 运维人员

---

## 1. 产品概述与定位

### 1.1 问题背景
随着 AI 编码工具（如 Cursor、Claude Code、Windsurf、VSCode Copilot 等）在日常开发中的普及，技术团队和个人开发者往往面临以下痛点：
* **配置零散**：每个工具都有独立的 `.cursorrules`、`claudecode.json` 等配置文件，多处维护不仅繁琐而且容易版本冲突。
* **记忆孤岛**：Agent 在 Cursor 中学到的项目偏好、特殊约定或调试经验，在 Claude Code 中完全无法复用。
* **工具重复配置**：每个客户端都需要单独接入 GitHub、Notion、DB 等外部 MCP Server，且多客户端的配置文件包含敏感 Token，难以进行统一凭证管理。
* **安全性与合规性缺失**：企业团队无法对 Agent 行为进行统一管控，例如：限制 Agent 写入生产数据库、删除文件、或在未经确认的情况下自动执行危险命令。

### 1.2 产品定位
**Open Agent Hub** 是一个基于 **MCP (Model Context Protocol)** 协议的 **AgentOps SaaS 平台**。它作为一个中心化的智能中枢，为不同 AI Agent 提供统一的**规则管理、跨会话记忆同步、团队级 Skill 分发、工具路由与安全策略拦截**能力。

```
┌────────────────────────────────────────────────────────┐
│             AI Agents (Cursor, Claude Code, etc.)      │
└───────────────────────────┬────────────────────────────┘
                            │
                            ▼ (HTTPS / Streamable HTTP)
┌────────────────────────────────────────────────────────┐
│                     MCP Gateway                        │
│             (Auth, Tenant, Tool Routing, Policies)     │
└──────────────┬──────────────┬──────────────┬───────────┘
               │              │              │
        Context Service  Memory Service  Tool Proxy
               │              │              │
        ┌──────▼──────┐┌──────▼──────┐┌──────▼──────┐
        │  Rules &    ││  pgvector   ││  External   │
        │ Preferences ││  Memory DB  ││ MCP Servers │
        └─────────────┘└─────────────┘└─────────────┘
```

---

## 2. 快速接入指南

### 2.1 术语说明
在接入前，需要理解以下四层多租户结构概念：
* **Organization (组织)**：最高隔离边界，通常代表一家公司或独立团队，绑定计费账单。
* **Workspace (工作区)**：隔离的核心单元。配置、记忆、Token、外部 MCP 路由均在工作区内完全隔离。
* **Project (项目)**：对应具体代码仓库（如 `api-service`），可以绑定项目级的专属规则（Project Rules）。
* **User (用户)**：通过工作区成员关联的注册用户。

---

### 2.2 获取 MCP Token
AI 客户端接入 Hub 必须使用工作区级别的 **MCP Token** 进行身份校验：
1. 登录 Open Agent Hub SaaS 控制台。
2. 导航至左侧菜单 **「MCP Tokens」** 页面。
3. 点击右上角 **「生成新 Token」** 按钮。
4. 输入 Token 名称（例如 `My Mac Studio`），选择 Scopes（推荐选 `read` 和 `write`），设置有效期（天，`0` 表示永久）。
5. 点击确认，系统将一次性展示生成的明文 Token（格式为 `pat_xxxxxx...`）。
6. **请立即复制并安全保存该 Token**。出于安全考虑，后端仅存储 Hash，网页刷新后将无法再次获取明文。

---

### 2.3 各主流客户端接入步骤

根据后端的部署方式，SaaS 模式下统一的接入口如下：
* **接口 URL**：`http://localhost:18085/mcp` （本地开发环境端口，生产环境下为 `https://mcp.openagenthub.com/mcp`）
* **鉴权方式**：`Authorization` 请求头携带 `Bearer <Your_MCP_Token>`

#### 2.3.1 Cursor 接入
1. 打开 Cursor，进入 **Settings** -> **Features**。
2. 滚动找到 **MCP** 区域，点击 **「+ Add New MCP Server」**。
3. 配置参数：
   * **Name**: `open-agent-hub`
   * **Type**: `sse` (或使用 `command` 通信，推荐 SSE / Streamable HTTP)
   * **URL**: `http://localhost:18085/mcp`
4. 为请求注入 HTTP Header（如 Cursor UI 支持输入 Headers，请加入）：
   * Key: `Authorization`
   * Value: `Bearer pat_YourTokenHere`
5. 保存后等待连接指示灯变为 **绿色 (Active)**。

#### 2.3.2 Claude Code 接入
Claude Code 在本地终端通过配置文件管理 MCP，编辑或创建 `~/.claude.json`：
```json
{
  "mcpServers": {
    "open-agent-hub": {
      "command": "curl",
      "args": [
        "-s",
        "-X",
        "POST",
        "-H",
        "Content-Type: application/json",
        "-H",
        "Authorization: Bearer pat_YourTokenHere",
        "-d",
        "{{input}}",
        "http://localhost:18085/mcp"
      ]
    }
  }
}
```
保存后，在 Claude Code 中输入 `/mcp` 即可查看到 `open-agent-hub` 所暴露的 14 个核心工具。

#### 2.3.3 Windsurf 接入
编辑 `~/.codeium/windsurf/mcp_config.json`，添加以下内容：
```json
{
  "mcpServers": {
    "open-agent-hub": {
      "command": "curl",
      "args": [
        "-s",
        "-X",
        "POST",
        "-H",
        "Content-Type: application/json",
        "-H",
        "Authorization: Bearer pat_YourTokenHere",
        "-d",
        "{{input}}",
        "http://localhost:18085/mcp"
      ]
    }
  }
}
```

### 2.4 项目绑定与本地快照（`.openagent/`）

项目级数据（Project Rules、`scope=project` 的记忆）需要把 Agent 的**当前工作目录**绑定到一个
Project（事实源是项目的 `repo_path`）。两种绑定方式：

**方式 A — `openagent` CLI（推荐给开发者）**：在项目根目录执行：

```bash
openagent init --server http://localhost:18085 --token pat_xxx   # 首次：注册+绑定+落盘快照
openagent sync                                                    # 日常：按 ETag 增量刷新
openagent status                                                  # 查看本地状态并检查服务端是否有新版本
openagent status --local                                          # 仅查看本地状态，不访问服务端
```

**方式 B — 纯 MCP（适用于 Cline 等无法运行 CLI 的场景）**：让 Agent 在任务开始时调用一次：

```
hub.sync_project(project_path=<当前工作目录绝对路径>,
                 register_project=true,
                 project_name=<由 LLM 根据项目内容起的语义名称，不要用路径>)
```

路径匹配不到项目时会自动创建并绑定；解析出的绑定会写入当前 MCP 会话，**同一会话内后续的
`hub.save_memory` / `hub.propose_memory`（scope=project）等调用自动继承**，无需再传参。也可以在
单次调用里显式传 `project_path` 覆盖。若 `scope=project` 但没有任何绑定，工具会直接报错并提示
绑定方法（不会再静默存为无项目记忆）。

同步会在项目根目录生成只读快照 `.openagent/`（本地生成目录，建议整体加入项目 `.gitignore`；
个人数据在 `local/` 子目录），并向 `CLAUDE.md`/`AGENTS.md` 注入托管块。快照内容请勿手工编辑，
统一在控制台修改后重新同步。

多人协作时，服务端规则、项目配置、公共 Skill 或记忆由任意成员更新后，其他成员在自己的项目目录执行
`openagent sync` 即可按 ETag 拉取最新快照；如果只是想确认是否落后服务端，执行 `openagent status`。
个性化指令属于当前用户，会同步到 `.openagent/local/profile.md`，不会作为团队共享配置提交。

---

## 3. SaaS 控制台功能详解

控制台提供了可视化管理后台，用于灵活操控和审查所有接入 Agent 的表现。

### 3.1 工作区与成员管理
* **工作区列表**：可以通过控制台右上角或 **「工作区管理」** 页面快速创建多个工作区，在组织内部划分「开发」、「测试」、「生产」或「团队A/B」环境。各工作区间配置完全物理隔离。
* **成员管理**：Admin 以上权限用户可以通过输入邮箱邀请已注册的用户加入当前工作区，并分配 `owner` / `admin` / `member` / `viewer` 角色。
  * `owner` 拥有全部管理及结算权限。
  * `admin` 可以配置 Rules、Connected Servers，但不能更改账单。
  * `member` 拥有读写记忆、规则等基础开发权限。
  * `viewer` 仅有只读浏览权限。

### 3.2 项目管理
当一个工作区内有多个不同的代码库时，可以通过 **「项目管理」** 登记对应的项目（例如：`frontend-app` 和 `backend-api`）。
* **自动识别**：在 Agent 发起 MCP 调用时，可通过请求头传入 `X-Project-Path` 或者 `project_id` 参数。
* **规则继承**：当 Agent 绑定了特定项目后，调用 `hub.get_project_rules` 会自动合并 **全局规则** 和 **项目专用规则** 交付给 Agent。

### 3.3 Context Hub (上下文配置中心)
Context Hub 负责向 Agent 灌输“行为准则和偏好设定”：
* **全局规则 (Global Rules)**：适用于整个工作区，例如指定使用的后端语言版本、企业代码安全规范、测试覆盖率指标等。
* **项目规则 (Project Rules)**：绑定至特定项目的规则，如特定前端组件库的使用要求、项目文件树目录约定等。
* **输出偏好 (Output Preferences)**：可以在控制台或通过 Tool 配置用户的输出倾向。默认提供了：
  * `language`: 返回语言（如 `zh-CN`）
  * `verbosity`: 详细程度（`concise` / `normal` / `verbose`）
  * `code_style`: 编码风格（如 `google` / `airbnb`）

### 3.4 Memory Hub (智能记忆中心)
这是 Open Agent Hub 最核心的能力之一。它将 Agent 本地无法保存的长期经验向量化，并在每次交互时进行语义检索召回。
* **记忆浏览**：可以查看工作区中生效的全部记忆，包括重要度、访问次数、最近活跃时间和置顶（Pin）状态。
* **智能记忆提议机制 (`propose_memory`)**：
  * AI Agent 在检测到新的长期事实或用户偏好时，会在后台静默调用 `hub.propose_memory` 提交提议。
  * **评分过滤**：系统会自动评估该条记忆的置信度和内容质量。对于包含 `TODO`、`temp` 等临时内容，或者相似度已经与现有记忆达到 92% 以上的语义重复内容，系统会自动将其过滤（Reject/Pending）。
  * **人工审核流**：非高置信度但有价值的记忆会进入 **「待审核」** Tab 页，用户在控制台手动点击「接受」后，方可正式生效成为 Agent 的长期记忆。
* **技能管理 (Skills)**：用于存放“程序性记忆”（操作步骤、脚本流），例如特定的 Debug 步骤或编译发布指南。状态可划分为 `active` / `stale` / `archived`，辅助 Agent 在接收复杂任务时快速查询最佳实践。

### 3.5 Tool Hub (工具治理与代理)
用于统一管理 Agent 能够使用的所有外部扩展工具：
* **连接外部服务器 (Connected MCP Servers)**：
  在控制台配置上游外部 MCP Server，例如 GitHub MCP、PostgreSQL MCP 等。Hub 将扮演反向代理的角色，统一对外发布工具列表，免去了在每个 AI 客户端重复配置 API Key 的烦恼。
* **安全拦截策略 (Tool Policies)**：
  对所有工具设定准入规则。
  * **Allowed (启用/禁用)**：一键禁用某个工具。
  * **Requires Confirmation (二次确认)**：高危工具（如直接删除数据库记录、部署线上服务、发送邮件、修改全局配置文件）必须设置为 `TRUE`。当 Agent 尝试调用时，Hub 会拦截并下发交互确认请求，确保安全合规。
  * **Quota (调用配额)**：可以设置每个工具每天或每个用户的调用频次上限，防止调用失控。

---

## 4. MCP Tools 规格速查

接入 Open Agent Hub 后，客户端 Agent 默认被赋予了以下 **14 个内置核心工具**（前缀为 `hub.`）。它们帮助 Agent 自主获取上下文。

| 工具名称 | 作用说明 | 核心输入参数 | 返回样例 |
| :--- | :--- | :--- | :--- |
| `hub.get_agent_profile` | 获取当前 Agent 客户端的信息及对应的套餐计费 | 无 | `{ "client_type": "cursor", "workspace_id": "uuid", "plan": "pro" }` |
| `hub.get_global_rules` | 获取当前工作区全局有效的 Markdown 规则 | `format` (默认为 `markdown` 或 `json`) | `{ "rules": ["规范1", "规范2"], "etag": "abc" }` |
| `hub.get_project_rules` | 获取合并了全局规则后的项目级规则 | `project_id` (必填) | `{ "effective_rules": [...] }` |
| `hub.get_workspace_policy` | 获取当前工作区的工具路由策略与用量配额 | 无 | `{ "tool_policies": [...], "quotas": {...} }` |
| `hub.get_output_preferences` | 获取用户偏好的语言、代码风格和详细程度说明 | 无 | `{ "language": "zh-CN", "verbosity": "concise" }` |
| `hub.search_memory` | 语义检索匹配工作区或项目下的长期记忆事实 | `query` (必填), `scope`, `limit` | `{ "memories": [{ "content": "偏好 Go 开发", "relevance": 0.88 }] }` |
| `hub.get_relevant_memory` | 根据当前对话上下文自动召回最相关的记忆 | `context_summary` (当前会话摘要，必填) | `{ "memories": [...] }` |
| `hub.propose_memory` | Agent 自动抓取并提议一条新记忆 | `content` (必填), `type`, `confidence` | `{ "decision": "accepted", "memory_id": "uuid" }` |
| `hub.save_memory` | 直接写入一条长期记忆（不受过滤器评分干预） | `content` (必填), `pinned` | `{ "memory_id": "uuid", "status": "active" }` |
| `hub.update_memory` | 修改一条已有记忆的内容并递增其版本号 | `memory_id` (必填), `content` (必填) | `{ "updated": true, "version": 2 }` |
| `hub.archive_memory` | 将某条记忆标记为归档状态（从搜索结果排除） | `memory_id` (必填) | `{ "archived": true }` |
| `hub.report_action` | Agent 上报关键操作（用于控制台审计与观测） | `action` (操作类型，必填), `target` | `{ "reported": true }` |
| `hub.get_usage_policy` | 获取当前工作区的配额以及今天/本月的总体用量 | 无 | `{ "usage": { "tool_call_today": 120 } }` |
| `hub.get_remaining_quota` | 快速获取今日剩余工具调用量 and 记忆数限制 | 无 | `{ "tool_calls_remaining": 4880 }` |

---

## 5. 安全性、用量与审计

### 5.1 审计日志 (Audit Logs)
* **不可变约束**：所有通过控制台操作（如重置 Token、删除项目、更新策略）或 Agent 调用核心工具的行为，均会实时、异步写入系统审计表 `audit_logs`。
* **安全审计**：管理员可在控制台 **「审计日志」** 界面中根据时间段、操作人员类型、具体 Action 进行追溯，确保发生异常代码变更或凭证泄露时有据可查。

### 5.2 监控仪表盘 (Usage Dashboard)
控制台主页提供了详细的健康用量看板：
* **实时会话数**：当前正与 Hub 网关保持活动连接的 MCP 会话总数。
* **趋势统计**：过去 7 天内各 Agent 发起的 Tool 调用频率趋势。
* **Top Tools 看板**：最频繁调用的前 5 个内置/外置工具，用于评估资源消耗。

---

## 6. 常见问题与排查 (FAQ)

### Q1：配置连接后，AI 客户端内连接状态报 `401 Unauthorized`？
1. 请进入控制台的 **「MCP Tokens」**，确认您在 Headers 传入的 Token 是否处于 `active` 状态（检查是否已被撤销 `revoked` 或已过期 `expired`）。
2. 请确认客户端配置的 HTTP 头字段是否拼写正确，要求：
   `Authorization: Bearer pat_xxxxxx`
3. 检查是否有本地缓存的旧凭证干扰。

### Q2：调用 MCP 接口时控制台报错 `404 Method Not Found` 或者是页面空白？
1. 请确认后端的监听端口是否配置正确。控制台 REST API 默认端口为 `18084`，而 **MCP Gateway 的服务端口是 `18085`**。
2. 在连接工具（如 Inspector 或 Cursor）时，配置的绝对路径必须是 `http://localhost:18085/mcp`，而不是控制台的接口地址。

### Q3：为什么 Agent 自动提交了 `hub.propose_memory` 后，在我的控制台「生效中」列表里找不到？
根据系统的**写入纪律机制**，如果 Agent 自动提取出的内容过于短小、逻辑杂乱，或者其包含像 `debug` 等时效性较强的临时变量，系统会自动将其决策置为 `pending_review`（待审核）。请进入 **「记忆浏览」 -> 点击「待审核」选项卡** 进行检查，手动点击接受即可将其激活为长期记忆。

### Q4：如何连接我们自己开发的本地 / 局域网 MCP Server？
在控制台 **「已连接的 MCP Server」** 中点击 **「新建 MCP Server」**，将您的自定义 MCP 服务的 IP/域名地址填入 `Endpoint`，Transport 选择 `streamable_http`。若该本地 Server 带有鉴权，可选择 `api_key` 并写入配置。完成后，配置对应的 Tool Policy 即可让团队内所有人的 Cursor 共享该局域网 Server 的所有工具！
