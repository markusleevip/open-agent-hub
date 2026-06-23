# Open Agent Hub — 基于 MCP 的 AgentOps SaaS 平台 技术规格文档

> v0.2 架构升级版（2026-06-07）。原 v0.1「本地配置与记忆同步中心」能力降级为可选 Local Bridge 附录保留。

## 文档目录

### Part I — 产品与架构

- 第 1 章 [项目概述](#1-项目概述)
- 第 2 章 [系统架构](#2-系统架构-control-plane--data-plane)
- 第 3 章 [核心概念与术语](#3-核心概念与术语)
- 第 4 章 [多租户与权限模型](#4-多租户与权限模型)

### Part II — MCP Gateway

- 第 5 章 [MCP 协议接入设计](#5-mcp-协议接入设计)
- 第 6 章 [MCP Tools 规格](#6-mcp-tools-规格)
- 第 7 章 [MCP Resources & Prompts](#7-mcp-resources--prompts)

### Part III — Control Plane 与核心服务

- 第 8 章 [核心数据模型（多租户版）](#8-核心数据模型多租户版)
- 第 9 章 [数据库设计](#9-数据库设计)
- 第 10 章 [记忆系统（向量化升级版）](#10-记忆系统向量化升级版)
- 第 11 章 [MCP Gateway 服务](#11-mcp-gateway-服务)
- 第 12 章 [Context Service](#12-context-service)
- 第 13 章 [Tool Routing 与外部 MCP 聚合](#13-tool-routing-与外部-mcp-聚合)
- 第 14 章 [性能与可扩展性](#14-性能与可扩展性)

### Part IV — 工程实现与运维

- 第 15 章 [前端设计（SaaS Console）](#15-前端设计saas-console)
- 第 16 章 [后端项目结构](#16-后端项目结构)
- 第 17 章 [技术选型](#17-技术选型)
- 第 18 章 [部署方案](#18-部署方案)
- 第 19 章 [开发阶段规划（p0p1p2）](#19-开发阶段规划p0p1p2)
- 第 20 章 [安全考虑](#20-安全考虑)
- 第 21 章 [错误处理与测试](#21-错误处理与测试)

### 附录

- 附录 A [Local Bridge Adapter 设计](#附录-a-local-bridge-adapter-设计)（P2 可选）
- 附录 B [Local Bridge 同步引擎](#附录-b-local-bridge-同步引擎)（P2 可选）
- 附录 C [旧版 ToC 工具迁移指南](#附录-c-旧版-toc-工具迁移指南)
- 附录 D [术语对照表](#附录-d-术语对照表)

---

## 1. 项目概述

### 1.1 问题背景

当前 AI Agent 工具生态呈现严重碎片化：

- **配置分散**：Cursor、Claude Code、OpenCode、GitHub Copilot、Windsurf 等工具有独立的配置文件系统，分散在不同位置，格式各异。同一条规则需要在多处手动维护。
- **记忆孤岛**：Agent 在不同工具中产生的用户偏好、项目上下文、Skill 无法跨工具复用——Cursor 学到的项目约定，Claude Code 完全看不到。
- **工具重复配置**：每个 Agent 工具都要分别接入 GitHub、Notion、PostgreSQL、Figma、Slack 等外部 MCP Server，配置维护成本随 Agent 数量线性增长。
- **权限治理缺失**：企业团队无法对 Agent 的行为（能调用哪些工具、能否访问敏感记忆、能否写入数据库）做统一管控。
- **无法跨会话复用上下文**：每次新会话 Agent 都从零开始，无法有效利用历史决策与项目经验。

### 1.2 产品定位

> **Open Agent Hub 是一个基于 MCP（Model Context Protocol）协议的 AgentOps SaaS 平台，为 Cursor、Claude Code、Windsurf、OpenCode 等 AI Agent 提供统一的规则、记忆、Skill、工具路由、权限控制和上下文分发能力。**

用户只需要在自己的 Agent 工具中接入 Open Agent Hub 提供的 **MCP Server**，即可让不同 Agent 共享统一的规则、记忆、项目上下文和工具能力。

### 1.3 五大核心卖点

1. **一个 MCP 接入口，连接所有 Agent**：用户只需在每个 Agent 工具中配置一次 `https://mcp.openagenthub.com/{workspace_id}`，即可让所有 Agent 接入。
2. **一个后台，管理所有 Agent 行为**：规则、记忆、Skill、工具权限统一在 SaaS 控制台管理。
3. **跨 Agent 共享记忆**：Cursor 学到的项目偏好，Claude Code 也能用；同一用户在不同 Agent 间无缝迁移上下文。
4. **外部工具统一代理**：不用在每个 Agent 里重复配置 GitHub / Notion / DB / Figma MCP——所有外部 MCP Server 由 Hub 统一聚合。
5. **团队级 Agent 治理**：团队可统一规定 Agent 怎么写代码、怎么审查、能调用哪些工具、能否访问敏感记忆。

### 1.4 部署形态

- **SaaS 托管**（主）：用户使用 `mcp.openagenthub.com` 公共服务，开箱即用
- **自托管开源**（次）：企业可下载自部署版本，私有化部署
- **Local Bridge**（可选组件，P2）：本地配置文件同步增强能力，仅在需要写本地文件时启用

### 1.5 v0.1 → v0.2 关键变化

| 维度 | v0.1（旧）| v0.2（新）|
|------|----------|----------|
| 产品形态 | 本地单进程工具 | SaaS 多租户平台 |
| 客户接入 | 下载二进制 + 文件同步 | 配置 MCP Server URL |
| 核心组件 | SyncHub + Adapter + FileWatcher | MCP Gateway + Context/Memory/Tool Service |
| 数据库 | SQLite/MySQL | PostgreSQL + pgvector（生产）/ SQLite（dev）|
| 租户模型 | 单 user_id | Organization → Workspace → Project → User 四层 |
| 认证 | JWT + API Key | OAuth 2.1 + MCP Token + Workspace Token |
| 同步能力 | 本地文件 Push/Pull | 运行时 MCP 调用 + 可选 Local Bridge |

---

## 2. 系统架构 (Control Plane + Data Plane)

### 2.1 双层架构总览

Open Agent Hub 采用 **Control Plane（控制面）+ Data Plane（数据面）** 的 SaaS 双层架构：

```
┌──────────────────────────────────────────────────────┐
│            Control Plane（SaaS 后台）                 │
│                                                       │
│  ┌──────────┐ ┌──────────┐ ┌──────────────────┐    │
│  │ React    │ │ Workspace│ │ Org / Members    │    │
│  │ Console  │ │ Switcher │ │ / Billing / Audit│    │
│  └──────────┘ └──────────┘ └──────────────────┘    │
│  ┌──────────────────────────────────────────────┐   │
│  │  4 Hub: Agent / Context / Memory / Tool       │   │
│  └──────────────────────────────────────────────┘   │
│              │ REST API (端口 8084)                    │
└──────────────┼───────────────────────────────────────┘
               │
┌──────────────┼───────────────────────────────────────┐
│            Data Plane（MCP Gateway）                 │
│              │ MCP over Streamable HTTP (端口 8085)   │
│  ┌───────────▼──────────────────────────────────┐  │
│  │           MCP Gateway                         │  │
│  │  Auth / Tenant / Policy / Tool Router         │  │
│  └────┬─────────┬─────────┬─────────┬─────────┘  │
│        │         │         │         │              │
│  ┌─────▼──┐ ┌────▼───┐ ┌──▼────┐ ┌──▼─────────┐ │
│  │Context │ │Memory  │ │ Tool  │ │   Skill    │ │
│  │Service │ │Service │ │Routing│ │  Service   │ │
│  └────┬───┘ └────┬───┘ └──┬────┘ └──┬─────────┘ │
│        │         │        │          │              │
└────────┼─────────┼────────┼──────────┼──────────────┘
         │         │        │          │
┌────────▼─────────▼────────▼──────────▼──────────────┐
│                 SaaS Core Backend                    │
│  Workspace / User / Billing / Audit / Policy         │
└────────┬────────────────────────────┬───────────────┘
         │                            │
┌────────▼────────┐         ┌─────────▼──────────────┐
│  PostgreSQL 14+ │         │    Vector Database     │
│  + pgvector     │         │  (pgvector 同库)       │
│  Config / Users │         │  Memory Retrieval      │
└─────────────────┘         └────────────────────────┘
```

**4 Hub 的定位**：Agent Hub / Context Hub / Memory Hub / Tool Hub 是**前端产品分类**与**后端服务包名**的统称，**不**意味着 4 个独立的微服务。后端起步是单进程多 Service（package-level），流量增长后再按 Service 拆分。

### 2.2 组件清单

| 组件 | 部署位置 | 职责 |
|------|---------|------|
| **React Console** | Control Plane | 4 Hub 后台管理界面 |
| **MCP Gateway** | Data Plane | MCP 协议接入、Tool 路由、租户上下文注入 |
| **Context Service** | Data Plane | Global/Project Rules 与 Output Preferences 聚合 |
| **Memory Service** | Data Plane | 记忆读写、写入纪律、Skill 治理、向量召回 |
| **Tool Routing Service** | Data Plane | 外部 MCP Server 代理、Tool Policy 校验 |
| **Skill Service** | Data Plane | Skill 分发与执行计划（run_skill_plan）|
| **PostgreSQL + pgvector** | Storage | 关系数据 + 向量检索 |
| **Redis** | Storage | 缓存、限流、会话粘性 |
| **Local Bridge**（可选）| 用户本机 | 本地配置文件同步增强 |

### 2.3 数据流（MCP 调用）

```
1. Agent (Cursor/Claude Code 等) 启动
   ↓
2. 从 workspace 配置中读取 MCP Server URL:
   https://mcp.openagenthub.com/{workspace_id}
   + 携带 OAuth/PAT Token
   ↓
3. 发起 Streamable HTTP 连接
   ↓
4. MCP Gateway 接收请求
   ├─ 认证中间件验证 Token → 解析 workspace_id、user_id
   ├─ 多租户中间件注入 workspace_id 到 context
   ├─ 限流中间件按 workspace+tool 维度检查配额
   ↓
5. Tool Router 匹配目标 Tool
   ├─ 本平台 Tool → 转发到对应 Service
   └─ 外部 MCP Tool → 代理到 connected_mcp_servers
   ↓
6. Service 执行业务逻辑
   ├─ Context Service → 读取 Rules、聚合返回
   ├─ Memory Service → 向量检索 / 写入纪律评分
   └─ Tool Routing → 转发到上游 MCP Server
   ↓
7. 结果按 MCP 协议返回 Agent
   ↓
8. 记录 ToolInvocationLog + AuditLog（异步）
```

### 2.4 v0.1 Push/Pull 同步的迁移路径

v0.1 中的本地文件 Push/Pull 同步不再作为主路径。其能力被两种新方式替代：

- **运行时分发**（主）：Agent 通过 MCP 协议在运行时获取配置、记忆、Skill
- **Local Bridge**（可选，P2）：独立 daemon 仍提供本地文件同步能力，见附录 A/B

---

## 3. 核心概念与术语

为避免后续章节歧义，本章统一定义 SaaS 化后必备术语：

| 术语 | 定义 |
|------|------|
| **Tenant** | 租户，即整个 Open Agent Hub 平台的一个独立计费与隔离单元 |
| **Organization** | 组织，Tenant 下的最高层级，代表一个公司/团队 |
| **Workspace** | 工作区，Organization 下的项目协作单元，配置与记忆的主要隔离边界 |
| **Project** | 项目，Workspace 下的具体工程目录，可绑定仓库与项目级 Rule |
| **User** | 用户，Organization 成员，通过 WorkspaceMember 关联到 Workspace |
| **Agent Client** | 接入 Hub 的 Agent 客户端实例（如用户的 Cursor、Claude Code）|
| **MCP Session** | Agent Client 与 MCP Gateway 之间的一次连接会话 |
| **MCP Token** | Workspace 级别的 MCP 接入凭证（OAuth/PAT/Workspace Token）|
| **Tool** | MCP 协议暴露的可调用能力，命名形如 `hub.get_global_rules` |
| **Resource** | MCP 协议暴露的只读数据，URI 形如 `hub://workspace/{ws_id}/rules/global` |
| **Prompt** | MCP 协议暴露的预置提示模板 |
| **Tool Policy** | 工具调用前的策略校验规则（允许/拒绝/二次确认/配额）|
| **Rule** | 规则，v0.1 GlobalConfig 的升级，scope 区分 workspace/project/agent |
| **Local Bridge** | v0.1 本地同步能力的延续，作为可选 P2 组件 |

---

## 4. 多租户与权限模型

### 4.1 四层租户结构

```
Tenant (Open Agent Hub SaaS)
└── Organization (公司/团队)
    └── Workspace (项目协作单元)  ← 配置与记忆的主要隔离边界
        └── Project (具体工程)
            └── User (成员, 通过 WorkspaceMember 关联)
```

每个请求必须携带 `workspace_id` 上下文，所有数据查询都受 workspace 隔离约束。

### 4.2 RBAC 权限对象与动作

**权限对象**：

| 对象 | 范围 |
|------|------|
| workspace | 工作区元数据、成员、配额 |
| project | 项目上下文、规则 |
| memory | 记忆条目 |
| skill | Skill 内容与状态 |
| rule | 规则条目 |
| connected_tool | 外部 MCP 工具接入 |
| mcp_server | MCP Server 接入 |
| api_key | API Key 管理 |
| billing | 账单与订阅 |
| audit | 审计日志（只读）|

**动作**：`read` / `write` / `invoke` / `sync` / `admin` / `delete` / `export`

**角色**：

| 角色 | 权限 |
|------|------|
| Owner | 所有权限（含 billing 与 admin）|
| Admin | 除 billing 外所有权限 |
| Member | 工作区内 read/write/invoke，不能管理成员 |
| Viewer | 仅 read |

### 4.3 Tool Policy 数据结构

```go
type ToolPolicy struct {
    ID                    string `json:"id"`
    WorkspaceID           string `json:"workspace_id"`
    ConnectedServerID     string `json:"connected_server_id"`  // 空表示平台 Tool
    ToolName              string `json:"tool_name"`            // 如 github.create_pull_request
    Allowed               bool   `json:"allowed"`
    RequiresConfirmation  bool   `json:"requires_confirmation"`
    AllowedRepositories   []string `json:"allowed_repositories,omitempty"`
    MaxCallsPerDay        int    `json:"max_calls_per_day"`
    MaxCallsPerMinute     int    `json:"max_calls_per_minute"`
    CreatedAt             time.Time `json:"created_at"`
    UpdatedAt             time.Time `json:"updated_at"`
}
```

**Tool Policy 示例**：

```json
{
  "tool": "github.create_pull_request",
  "policy": {
    "allowed": true,
    "requires_confirmation": true,
    "allowed_repositories": ["open-agent-hub/core"],
    "max_calls_per_day": 50
  }
}
```

### 4.4 高风险操作二次确认

以下高风险操作**必须**通过 Tool Policy 设置 `requires_confirmation: true`：

```
- 删除文件
- 写数据库
- 发邮件
- 发消息
- 创建 PR
- 部署服务
- 调用付费 API
- 修改生产环境配置
```

Agent Client 收到此类 Tool 调用时，必须在 UI 层弹出确认对话框，用户手动确认后才能执行。

---

## 5. MCP 协议接入设计

### 5.1 为什么选 MCP

MCP（Model Context Protocol）是官方定义的标准协议，让 AI 应用以统一方式连接外部数据源和工具。MCP Server 可暴露 **tools** / **resources** / **prompts**，HTTP 传输下也定义授权机制。

选 MCP 作为唯一外部协议的原因：

- **跨 Agent 复用**：MCP 是 Anthropic 主导的行业标准，Cursor/Claude Code/Windsurf/OpenCode 全部原生支持
- **统一入口**：用户只需配置一个 MCP URL，即可让所有 Agent 接入
- **生态丰富**：外部工具（GitHub/Notion/DB/Figma 等）已有大量 MCP 实现，可被 Hub 代理

### 5.2 传输层选择

| 传输 | 用途 | 说明 |
|------|------|------|
| **Streamable HTTP** | SaaS 公共接入（强制）| 兼容 HTTP/SSE，单连接支持双向流 |
| SSE | 向后兼容旧 Agent | 仅 Server→Client 单向流 |
| stdio | Local Bridge 本地模式 | Local Bridge 走 stdio 与本机 Agent 通信 |

**SaaS 公共接入**统一使用 Streamable HTTP，端点：

```
POST https://mcp.openagenthub.com/mcp
```

### 5.3 Server URL 规范

```
单 workspace:        https://mcp.openagenthub.com/mcp
多 workspace 路由:    https://mcp.openagenthub.com/ws/{workspace_id}/mcp
```

请求头必须携带：

| Header | 必填 | 说明 |
|--------|------|------|
| `Authorization` | 是 | `Bearer <mcp_token>`，支持 OAuth/PAT/Workspace Token |
| `X-Project-Path` | 否 | 当前项目路径，项目绑定的回退来源之一（见 5.3.1，优先级最低） |
| `Mcp-Session-Id` | 否 | 客户端生成的 session UUID，用于会话连续性与会话级项目绑定 |

#### 5.3.1 项目绑定（Project Binding）

项目级数据（Project Rules、scope=project 的记忆）必须解析到一个具体 Project。绑定的事实源是
`projects.repo_path`（agent 工作目录的绝对路径，服务端做归一化：清理冗余分隔符、去尾部斜杠）。
工具调用按以下优先级解析项目，全部落空且操作要求项目时**显式报错**（不允许静默落空）：

```
1. 工具入参 project_id          （最高优先级，直接指定）
2. 工具入参 project_path        （按 repo_path 归一化精确匹配）
3. 会话级绑定                    （mcp_sessions.project_id，见下）
4. X-Project-Path 请求头        （静态配置的回退，兼容用途）
```

**会话级绑定**：`hub.sync_project` 解析（或注册）项目成功后，将 `project_id` 写入当前
`mcp_sessions` 记录；同一会话的后续调用自动继承，agent 无需逐次传参。推荐的 agent 流程：

```
任务开始 → hub.sync_project(project_path=<cwd>, register_project=true,
                            project_name=<LLM 根据项目内容起的语义名称>)
        → 之后的 hub.propose_memory / hub.save_memory(scope=project) 等自动挂到该项目
```

**自动注册**：`project_path` 匹配不到项目且 `register_project=true` 时自动创建项目并绑定该路径。
`project_name` 应由 agent 的 LLM 依据项目内容（README、代码结构等）给出**语义名称，禁止使用文件
系统路径**；缺省回退为目录 basename。slug 在 workspace 内自动去重（冲突追加 `-2`、`-3`…）。

#### 5.3.2 本地快照（`.openagent/` 目录）

`hub.sync_project` / `openagent` CLI 将服务端配置单向渲染为项目根目录下的只读快照（事实源始终在
Console；内容确定性渲染、不含时间戳，按 ETag 增量同步）：

```
.openagent/
├── config.json          # 项目身份（server_url/project_id），可提交、跨设备共享
├── rules.md             # 生效规则（global→project→agent 合并结果）
├── project.md           # 项目信息
├── skills/              # 团队 Skill（index.json + <slug>/SKILL.md）
├── .gitignore           # 排除 local/
└── local/               # 个人数据，不可提交
    ├── state.json       # 本机同步状态（etag、文件 hash）
    ├── profile.md       # 个人 profile
    └── memories.md      # 关键记忆摘要
```

同时向仓库的 `CLAUDE.md` / `AGENTS.md` 注入托管块（`<!-- openagenthub:begin/end -->`），并生成
`.mcp.json`。凭据存放于 `~/.openagent/credentials.json`（按 server URL 索引），绝不写入项目目录。

### 5.4 OAuth 2.1 握手流程

```
1. Agent 启动时读取 workspace 配置:
   {
     "mcpServers": {
       "open-agent-hub": {
         "url": "https://mcp.openagenthub.com/mcp",
         "auth": "oauth"
       }
     }
   }

2. Agent 发现需要认证（HTTP 401），发起 OAuth 2.1 + PKCE 流程

3. 重定向到 Open Agent Hub 授权端点:
   https://mcp.openagenthub.com/oauth/authorize?
     client_id=...&
     redirect_uri=...&
     code_challenge=...&
     state=...&
     scope=workspace:{workspace_id}

4. 用户在 Hub 登录 + 授权

5. 回调返回 authorization code

6. Agent 用 code 换取 access_token + refresh_token

7. 后续请求 Bearer access_token

8. MCP Gateway 中间件验证 token 签名、有效期、scope
```

**Token 类型**：

| 类型 | 用途 | 有效期 |
|------|------|--------|
| **OAuth access_token** | Web UI 登录 + MCP 接入 | 1 小时 |
| **OAuth refresh_token** | 刷新 access_token | 30 天 |
| **PAT（Personal Access Token）** | CLI/脚本程序化调用 `sk-xxx` | 用户自定义 |
| **Workspace MCP Token** | 团队级共享 MCP 凭证 | 用户自定义 |
| **Short-lived Session Token** | 单次会话临时凭证 | 5 分钟 |

### 5.5 会话生命周期（MCPSession）

```go
type MCPSessionStatus string

const (
    MCPSessionStatusInitializing MCPSessionStatus = "initializing"
    MCPSessionStatusActive       MCPSessionStatus = "active"
    MCPSessionStatusIdle         MCPSessionStatus = "idle"      // 无活动但未关闭
    MCPSessionStatusClosed       MCPSessionStatus = "closed"
    MCPSessionStatusError        MCPSessionStatus = "error"
)
```

会话状态机：

```
initializing → active ⇄ idle → closed
                  ↘
                   error → closed
```

会话超时：

- 空闲超时：30 分钟无 Tool 调用转入 idle
- 强制断开：refresh_token 过期后强制 closed
- 异常断开：网络错误累计 3 次自动 closed

### 5.6 MCP 错误规范映射

MCP 协议错误码 → HTTP 状态码 + 业务错误码：

| MCP 错误 | HTTP | 业务码 | 含义 |
|---------|------|--------|------|
| `ParseError` | 400 | 40001 | JSON 解析失败 |
| `InvalidRequest` | 400 | 40002 | 请求格式错误 |
| `MethodNotFound` | 404 | 40401 | Tool/Resource 不存在 |
| `InvalidParams` | 422 | 42201 | 参数校验失败 |
| `InternalError` | 500 | 50000 | 内部错误 |
| `Unauthorized` | 401 | 40100 | Token 无效/过期 |
| `Forbidden` | 403 | 40300 | 无权限/超出配额 |
| `RateLimited` | 429 | 42900 | 触发限流 |
| `ToolRequiresConfirmation` | 409 | 40901 | Tool 需要用户确认 |
| `UpstreamUnavailable` | 502 | 50200 | 上游 MCP Server 不可用 |
| `WorkspaceNotFound` | 404 | 40402 | workspace_id 不存在 |

---

## 6. MCP Tools 规格

本章定义 Open Agent Hub 通过 MCP 协议暴露的所有 Tool 规格。共 6 大类、26 个核心 Tool。

### 6.1 Tool 通用规范

所有 Tool 请求遵循统一 JSON-RPC 2.0 格式：

```json
{
  "jsonrpc": "2.0",
  "id": "req-uuid",
  "method": "tools/call",
  "params": {
    "name": "hub.get_global_rules",
    "arguments": { ... }
  }
}
```

所有 Tool 响应：

```json
{
  "jsonrpc": "2.0",
  "id": "req-uuid",
  "result": {
    "content": [
      { "type": "text", "text": "..." }
    ],
    "isError": false
  }
}
```

通用字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `workspace_id` | string (UUID) | 自动从 token 注入，调用方无需传 |
| `project_id` | string (UUID, optional) | 项目级 Tool 必传 |
| `request_id` | string (UUID) | 链路追踪 ID |

### 6.2 配置类 Tools（P0 必做）

#### `hub.get_agent_profile`

获取当前 Agent Client 接入信息。

**输入**：无

**输出**：

```json
{
  "agent_client_id": "uuid",
  "client_type": "cursor",
  "client_name": "My Cursor",
  "workspace_id": "uuid",
  "user_id": "uuid",
  "plan": "pro"
}
```

**错误码**：`Unauthorized` / `WorkspaceNotFound`

#### `hub.get_global_rules`

获取 Workspace 全局规则。

**输入**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `format` | string | 否 | `markdown` (默认) / `json` |

**输出**：

```json
{
  "rules": [
    "后端使用 Go",
    "前端使用 React",
    "所有接口必须有错误处理"
  ],
  "version": "2026-06-07T10:00:00Z",
  "etag": "abc123"
}
```

**SLO**：P99 < 50ms

#### `hub.get_project_rules`

获取当前 Project 的规则。

**输入**：

| 字段 | 类型 | 必填 |
|------|------|------|
| `project_id` | string | 是 |

**输出**：与 `get_global_rules` 类似，包含 `project_rules` + 合并后的 `effective_rules`

#### `hub.get_workspace_policy`

获取 Workspace 整体策略（Tool Policy 集合 + 配额）。

**输入**：无

**输出**：

```json
{
  "tool_policies": [...],
  "quotas": {
    "memory_count_max": 10000,
    "tool_call_monthly_max": 100000
  }
}
```

#### `hub.get_output_preferences`

获取用户输出风格偏好（语言、详细程度、代码风格）。

**输入**：无

**输出**：

```json
{
  "language": "zh-CN",
  "verbosity": "concise",
  "code_style": "google"
}
```

### 6.3 记忆类 Tools（P0 必做）

#### `hub.search_memory`

语义检索记忆。

**输入**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | 是 | 查询文本 |
| `scope` | string | 否 | `workspace` (默认) / `project` / `agent` |
| `project_id` | string | scope=project 时必填 | |
| `limit` | int | 否 | 默认 10，最大 50 |
| `min_relevance` | float | 否 | 默认 0.6 |

**输出**：

```json
{
  "memories": [
    {
      "id": "uuid",
      "content": "...",
      "importance": 0.92,
      "scope": "project",
      "type": "semantic",
      "relevance": 0.88,
      "created_at": "2026-06-07T..."
    }
  ]
}
```

**SLO**：P99 < 200ms

#### `hub.get_relevant_memory`

基于当前会话上下文自动检索相关记忆（无需 query）。

**输入**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `context_summary` | string | 是 | 会话摘要 |
| `limit` | int | 否 | 默认 5 |

**输出**：与 `search_memory` 相同

#### `hub.propose_memory`

Agent 提交记忆候选（必经入口）。记忆不会立即写入，会经过写入纪律评分 + 可能的用户审核。

**输入**：

```json
{
  "content": "用户偏好使用 Go + React 开发 SaaS 平台",
  "type": "user_preference",
  "scope": "workspace",
  "source_agent": "cursor",
  "confidence": 0.9,
  "tags": ["tech_stack"]
}
```

**输出**：

```json
{
  "decision": "accepted",  // accepted / pending_review / rejected
  "memory_id": "uuid",
  "reason": "长期偏好，复用价值高"
}
```

**决策流程**：`propose_memory` → Memory Scoring → Policy Check → Auto Accept 或 Pending Review → Save

#### `hub.save_memory`

直接保存（仅在 Policy 允许 auto-accept 时使用）。

**输入**：与 `propose_memory` 相同

**输出**：`memory_id`

#### `hub.update_memory`

更新已有记忆。

**输入**：

| 字段 | 类型 | 必填 |
|------|------|------|
| `memory_id` | string | 是 |
| `content` | string | 是 |

#### `hub.archive_memory`

归档记忆（不删除，标记为 superseded）。

**输入**：`memory_id`

### 6.4 Skill 类 Tools（P1）

#### `hub.list_skills`

列出当前 Workspace 可用 Skill。

#### `hub.search_skills`

按关键词/标签检索 Skill。

#### `hub.get_skill`

获取 Skill 详情。

**输入**：`skill_id` 或 `skill_name`

**输出**：

```json
{
  "id": "uuid",
  "name": "VibeCoding 项目初始化",
  "description": "...",
  "content": "...",
  "state": "active",
  "tags": ["init", "vibecoding"]
}
```

#### `hub.run_skill_plan`

执行 Skill 工作流计划（调用 LLM 配合 Skill 模板执行多步骤任务）。

**输入**：

```json
{
  "skill_id": "uuid",
  "inputs": { ... }
}
```

### 6.5 MCP 聚合类 Tools（P1）

#### `hub.list_connected_tools`

列出 Workspace 已接入的外部 MCP Server 及可调用的 Tool。

#### `hub.invoke_connected_tool`

调用外部 MCP Server 的 Tool（命名空间前缀：`{server_name}.{tool_name}`）。

**输入**：

```json
{
  "tool": "github.create_pull_request",
  "arguments": { ... }
}
```

**前置校验**：Tool Policy 检查（allowed / requires_confirmation / quota）

**SLO**：P99 < 2s（含上游）

#### `hub.get_tool_policy`

获取 Tool 的策略配置。

### 6.6 项目上下文类 Tools（P1）

| Tool | 说明 |
|------|------|
| `hub.get_project_context` | 完整项目上下文聚合返回 |
| `hub.get_project_stack` | 技术栈识别（语言/框架/依赖）|
| `hub.get_project_structure` | 目录结构摘要 |
| `hub.get_project_rules` | 项目级规则（与 `get_project_rules` 同义）|
| `hub.update_project_context` | 更新项目元信息（需 write 权限）|

### 6.7 审计类 Tools（P0）

#### `hub.report_action`

Agent 上报自身行为（用于审计与可观测性）。

**输入**：

```json
{
  "action": "code_edit",
  "target": "src/main.go",
  "summary": "添加错误处理"
}
```

#### `hub.get_usage_policy`

获取 Workspace 使用策略与配额。

#### `hub.get_remaining_quota`

获取剩余配额（Tool 调用次数、记忆条数等）。

**输出**：

```json
{
  "tool_calls_remaining": 85000,
  "memory_count_remaining": 8500,
  "reset_at": "2026-07-01T00:00:00Z"
}
```

### 6.8 Tool 优先级汇总

| Tool | 优先级 | 阶段 |
|------|--------|------|
| `hub.get_agent_profile` | P0 | 阶段一 |
| `hub.get_global_rules` | P0 | 阶段一 |
| `hub.get_project_rules` | P0 | 阶段一 |
| `hub.get_workspace_policy` | P0 | 阶段一 |
| `hub.search_memory` | P0 | 阶段一 |
| `hub.get_relevant_memory` | P0 | 阶段一 |
| `hub.propose_memory` | P0 | 阶段一 |
| `hub.save_memory` | P0 | 阶段一 |
| `hub.update_memory` | P0 | 阶段一 |
| `hub.archive_memory` | P0 | 阶段一 |
| `hub.report_action` | P0 | 阶段一 |
| `hub.get_usage_policy` | P0 | 阶段一 |
| `hub.get_remaining_quota` | P0 | 阶段一 |
| `hub.get_output_preferences` | P1 | 阶段二 |
| `hub.list_skills` | P1 | 阶段二 |
| `hub.search_skills` | P1 | 阶段二 |
| `hub.get_skill` | P1 | 阶段二 |
| `hub.run_skill_plan` | P1 | 阶段二 |
| `hub.list_connected_tools` | P1 | 阶段二 |
| `hub.invoke_connected_tool` | P1 | 阶段二 |
| `hub.get_tool_policy` | P1 | 阶段二 |
| `hub.get_project_context` | P1 | 阶段二 |
| `hub.get_project_stack` | P1 | 阶段二 |
| `hub.get_project_structure` | P1 | 阶段二 |
| `hub.update_project_context` | P1 | 阶段二 |

P0 共 14 个 Tool，构成本文档阶段一 MVP；P1 共 12 个 Tool 在阶段二交付。

---

## 7. MCP Resources & Prompts

### 7.1 Resource URI 设计

MCP Resource 适用于**稳定、只读、可缓存**的数据。Hub 暴露的 Resource URI 规范：

```
hub://workspace/{workspace_id}/rules/global
hub://workspace/{workspace_id}/rules/global?v=2026-06-07
hub://workspace/{workspace_id}/skills
hub://workspace/{workspace_id}/memory/snapshot/latest
hub://workspace/{workspace_id}/tool-policies
hub://project/{project_id}/context
hub://project/{project_id}/context/snapshot/latest
hub://project/{project_id}/rules
hub://project/{project_id}/skills
hub://user/{user_id}/preferences
```

**资源类型 → 推荐实现**：

| 数据类型 | 推荐实现 | 原因 |
|---------|---------|------|
| 全局规则 | Resource（带版本）| 高频读、变化少、可缓存 |
| 记忆快照 | Resource（Snapshot）| 稳定视图，避免 prompt 抖动 |
| 项目上下文 | Resource（带 ETag）| 读多写少 |
| 技能清单 | Resource + 列表子 Resource | 全量+详情分层 |
| 记忆搜索结果 | Tool（不缓存）| 动态 query 结果 |

### 7.2 版本化策略

每个 Resource 响应必须带：

- **ETag 头**：内容指纹
- **Cache-Control 头**：`max-age=300, must-revalidate`
- **Last-Modified 头**：

Agent 收到 304 Not Modified 时直接复用本地缓存，**避免重新注入 LLM 上下文**，从而保护 Prompt Cache。

```http
HTTP/1.1 200 OK
ETag: "abc123def456"
Cache-Control: max-age=300, must-revalidate
Last-Modified: Fri, 07 Jun 2026 10:00:00 GMT
Content-Type: application/json

{
  "rules": [...],
  "version": "2026-06-07T10:00:00Z"
}
```

### 7.3 预置 Prompts

Hub 暴露的预置 Prompt 模板（命名空间：`open_agent_hub_*`）：

| Prompt 名称 | 用途 |
|-------------|------|
| `open_agent_hub_project_bootstrap` | 项目初始化：读取规则/偏好/记忆后开始工作 |
| `open_agent_hub_code_review` | 代码审查：调用 Skill + 规则做 PR 审查 |
| `open_agent_hub_memory_review` | 记忆审查：让用户审视 Agent 提议的新记忆 |
| `open_agent_hub_vibecoding_plan` | VibeCoding 计划：把自然语言需求转技术任务 |
| `open_agent_hub_refactor_plan` | 重构计划：分析代码后给出重构建议 |
| `open_agent_hub_skill_run` | 通用 Skill 执行入口 |

### 7.4 Prompt 示例：project_bootstrap

```markdown
你是一个遵守 Open Agent Hub Workspace 规则的开发 Agent。

请按以下步骤开始：

1. 调用 hub.get_global_rules 获取当前 Workspace 的全局规则
2. 调用 hub.get_project_rules 获取当前项目的规则
3. 调用 hub.get_output_preferences 获取用户的输出偏好
4. 调用 hub.search_memory 查询与当前任务相关的记忆
5. 加载相关 Skill（如有）
6. 根据以上上下文开始执行用户任务

如果在交互中发现新的长期偏好或事实，请调用 hub.propose_memory，
而不是直接保存到本地。
```

### 7.5 Snapshot 资源模式

为保护 LLM Prompt Cache，Hub 为记忆与项目上下文提供**不可变快照**资源：

- URI 末尾固定 `?v={snapshot_version}` 标识快照版本
- 同一会话内 snapshot 不变
- 新记忆写入不破坏当前 snapshot，下一会话再生成新 snapshot
- 这是 Prompt Cache 协调的核心机制

具体实现参见第 10 章「记忆系统」的冻结快照机制。

---

## 8. 核心数据模型（多租户版）

> 本章为多租户 SaaS 化的数据模型设计。**v0.1 中的所有模型保留并通过新增 `OrgID` / `WorkspaceID` / `ProjectID` 三级租户字段升级**；同时新增 12 张 SaaS 多租户实体表。
>
> 命名变更：`GlobalConfig` → `Rule`（scope 区分 `workspace` / `project` / `agent` 三级），`AgentInstance` → `AgentClient`，`user_id` → `workspace_id`（租户隔离边界）。
>
> 完整字段变更对照见 [附录 D 术语对照表](#附录-d-术语对照表)。

### 8.1 租户模型

#### Organization（组织）

```go
type Organization struct {
    ID        string    `gorm:"primarykey;type:uuid" json:"id"`
    Name      string    `gorm:"type:varchar(128);not null" json:"name"`
    Slug      string    `gorm:"type:varchar(64);uniqueIndex" json:"slug"`
    Plan      string    `gorm:"type:varchar(32);not null;default:'free'" json:"plan"` // free / pro / enterprise
    Status    string    `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

func (Organization) TableName() string { return "organizations" }

type OrgPlan string

const (
    OrgPlanFree       OrgPlan = "free"
    OrgPlanPro        OrgPlan = "pro"
    OrgPlanEnterprise OrgPlan = "enterprise"
)
```

#### Workspace（工作空间）

```go
type Workspace struct {
    ID                 string     `gorm:"primarykey;type:uuid" json:"id"`
    OrgID              string     `gorm:"type:uuid;not null;index:idx_workspace_org" json:"org_id"`
    Name               string     `gorm:"type:varchar(128);not null" json:"name"`
    Slug               string     `gorm:"type:varchar(64);not null" json:"slug"`
    QuotaMemoryCount   int        `gorm:"not null;default:10000" json:"quota_memory_count"`
    QuotaToolCallDaily int        `gorm:"not null;default:5000" json:"quota_tool_call_daily"`
    QuotaVectorMB      int        `gorm:"not null;default:512" json:"quota_vector_mb"`
    Status             string     `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
    CreatedAt          time.Time  `json:"created_at"`
    UpdatedAt          time.Time  `json:"updated_at"`
}

func (Workspace) TableName() string { return "workspaces" }
```

#### WorkspaceMember（工作空间成员）

```go
type WorkspaceMember struct {
    ID          string     `gorm:"primarykey;type:uuid" json:"id"`
    WorkspaceID string     `gorm:"type:uuid;not null;index:idx_member_workspace" json:"workspace_id"`
    UserID      string     `gorm:"type:uuid;not null;index:idx_member_user" json:"user_id"`
    Role        string     `gorm:"type:varchar(16);not null;default:'member'" json:"role"`
    InvitedAt   time.Time  `json:"invited_at"`
    JoinedAt    *time.Time `json:"joined_at"`
}

func (WorkspaceMember) TableName() string { return "workspace_members" }

type WorkspaceRole string

const (
    WorkspaceRoleOwner  WorkspaceRole = "owner"  // 工作空间所有者
    WorkspaceRoleAdmin  WorkspaceRole = "admin"  // 管理员
    WorkspaceRoleMember WorkspaceRole = "member" // 普通成员
    WorkspaceRoleViewer WorkspaceRole = "viewer" // 只读访客
)
```

#### User（用户）

```go
type User struct {
    ID          string     `gorm:"primarykey;type:uuid" json:"id"`
    Email       string     `gorm:"type:varchar(255);uniqueIndex" json:"email"`
    DisplayName string     `gorm:"type:varchar(128)" json:"display_name"`
    AvatarURL   string     `gorm:"type:varchar(512)" json:"avatar_url"`
    Status      string     `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
    LastLoginAt *time.Time `json:"last_login_at"`
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
}

func (User) TableName() string { return "users" }
```

### 8.2 规则与配置

> v0.1 的 `GlobalConfig` 在 v0.2 重命名为 `Rule`，并升级为支持 `workspace` / `project` / `agent` 三级 scope。

```go
type Rule struct {
    ID          string  `gorm:"primarykey;type:uuid" json:"id"`
    OrgID       string  `gorm:"type:uuid;not null;index:idx_rule_org" json:"org_id"`
    WorkspaceID string  `gorm:"type:uuid;not null;index:idx_rule_workspace" json:"workspace_id"`
    ProjectID   *string `gorm:"type:uuid;index:idx_rule_project" json:"project_id"`     // nullable: 跨项目规则为空
    AgentName   *string `gorm:"type:varchar(64);index:idx_rule_agent" json:"agent_name"` // nullable: 跨 Agent 规则为空
    Name        string  `gorm:"type:varchar(128);not null" json:"name"`
    Description string  `gorm:"type:varchar(512)" json:"description"`
    Value       string  `gorm:"type:text;not null" json:"value"`
    Type        string  `gorm:"type:varchar(32);not null;index:idx_rule_type" json:"type"`
    Tags        string  `gorm:"type:text;default:'[]'" json:"tags"`
    Scope       string  `gorm:"type:varchar(16);not null;default:'workspace'" json:"scope"`
    Version     int     `gorm:"not null;default:1" json:"version"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

func (Rule) TableName() string { return "rules" }

type RuleType string

const (
    RuleTypeSystemPrompt    RuleType = "system_prompt"
    RuleTypeStyleGuide      RuleType = "style_guide"
    RuleTypeOutputFormat    RuleType = "output_format"
    RuleTypeCustomMCPServer RuleType = "custom_mcp_server"
    RuleTypePermission      RuleType = "permission"
    RuleTypeCustom          RuleType = "custom"
)

type RuleScope string

const (
    RuleScopeWorkspace RuleScope = "workspace" // 工作空间级
    RuleScopeProject   RuleScope = "project"   // 项目级
    RuleScopeAgent     RuleScope = "agent"     // Agent 级
)
```

#### OutputPreference（输出偏好）

> 用户级偏好（语言、风格、详细程度等），与 `Rule` 解耦单独存储。

```go
type OutputPreference struct {
    ID          string    `gorm:"primarykey;type:uuid" json:"id"`
    UserID      string    `gorm:"type:uuid;not null;index:idx_pref_user" json:"user_id"`
    WorkspaceID string    `gorm:"type:uuid;not null;index:idx_pref_workspace" json:"workspace_id"`
    Key         string    `gorm:"type:varchar(64);not null" json:"key"`
    Value       string    `gorm:"type:text;not null" json:"value"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

func (OutputPreference) TableName() string { return "output_preferences" }
```

### 8.3 记忆系统模型

> v0.1 记忆模型完整保留，**新增 `WorkspaceID` + `ProjectID` + `Embedding` 向量字段**。算法核心（写入纪律 / 双时间轴 / 冻结快照 / Skill 三状态机）详见 [第 10 章 记忆系统](#10-记忆系统向量化升级版)。

```go
type Memory struct {
    ID          string     `gorm:"primarykey;type:uuid" json:"id"`
    OrgID       string     `gorm:"type:uuid;not null;index:idx_memory_org" json:"org_id"`
    WorkspaceID string     `gorm:"type:uuid;not null;index:idx_memory_workspace" json:"workspace_id"`
    ProjectID   *string    `gorm:"type:uuid;index:idx_memory_project" json:"project_id"`
    UserID      string     `gorm:"type:uuid;not null;index:idx_memory_user_type" json:"user_id"`
    Content     string     `gorm:"type:text;not null" json:"content"`
    Type        string     `gorm:"type:varchar(32);not null" json:"type"`
    Category    string     `gorm:"type:varchar(16);not null" json:"category"`
    Tags        string     `gorm:"type:text;default:'[]'" json:"tags"`
    Scope       string     `gorm:"type:varchar(16);not null;default:'workspace'" json:"scope"`
    Provenance  string     `gorm:"type:varchar(32);not null;default:'human_curated'" json:"provenance"`
    Importance  float64    `gorm:"not null;default:0.5" json:"importance"`
    Pinned      bool       `gorm:"not null;default:false;index:idx_memory_pinned" json:"pinned"`
    State       string     `gorm:"type:varchar(16);not null;default:'active'" json:"state"`
    AccessCount int        `gorm:"not null;default:0" json:"access_count"`
    LastAccessAt *time.Time `json:"last_access_at"`
    CharCount    int        `gorm:"not null;default:0" json:"char_count"`

    // 向量检索字段（v0.2 新增）
    Embedding      []byte  `gorm:"type:vector(1536)" json:"-"` // pgvector 存储（bin16 表示的 1536 维 float32）
    EmbeddingModel string  `gorm:"type:varchar(64)" json:"embedding_model"`
    EmbeddedAt     *time.Time `json:"embedded_at"`

    Version   int       `gorm:"not null;default:1" json:"version"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

func (Memory) TableName() string { return "memories" }
```

#### MemoryValidity（双时间轴）

```go
type MemoryValidity struct {
    ID           string     `gorm:"primarykey;type:uuid" json:"id"`
    MemoryID     string     `gorm:"type:uuid;not null;index:idx_validity_memory" json:"memory_id"`
    WorkspaceID  string     `gorm:"type:uuid;not null;index:idx_validity_workspace" json:"workspace_id"`
    ValidFrom    time.Time  `gorm:"not null" json:"valid_from"`
    ValidUntil   *time.Time `json:"valid_until"`
    RecordedAt   time.Time  `gorm:"not null" json:"recorded_at"`
    SupersededBy *string    `gorm:"type:uuid" json:"superseded_by"`
}

func (MemoryValidity) TableName() string { return "memory_validity" }
```

#### MemorySnapshot（冻结快照）

```go
type MemorySnapshot struct {
    ID          string    `gorm:"primarykey;type:uuid" json:"id"`
    WorkspaceID string    `gorm:"type:uuid;not null;index:idx_snapshot_workspace" json:"workspace_id"`
    SessionID   string    `gorm:"type:uuid;not null;uniqueIndex" json:"session_id"`
    Content     string    `gorm:"type:text;not null" json:"content"`
    CharCount   int       `gorm:"not null" json:"char_count"`
    MemoryIDs   string    `gorm:"type:text;default:'[]'" json:"memory_ids"`
    Version     string    `gorm:"type:varchar(32);not null" json:"version"` // snapshot URI 版本号
    FrozenAt    time.Time `json:"frozen_at"`
}

func (MemorySnapshot) TableName() string { return "memory_snapshots" }
```

#### MemoryAccessLog（访问日志）

```go
type MemoryAccessLog struct {
    ID          string    `gorm:"primarykey;type:uuid" json:"id"`
    MemoryID    string    `gorm:"type:uuid;not null;index:idx_access_memory" json:"memory_id"`
    WorkspaceID string    `gorm:"type:uuid;not null" json:"workspace_id"`
    UserID      string    `gorm:"type:uuid;not null" json:"user_id"`
    QueryType   string    `gorm:"type:varchar(32)" json:"query_type"`
    Relevance   float64   `gorm:"type:double precision" json:"relevance"`
    AccessedAt  time.Time `gorm:"index:idx_access_time" json:"accessed_at"`
}

func (MemoryAccessLog) TableName() string { return "memory_access_log" }
```

#### MemoryMapping（记忆映射 - Local Bridge 兼容保留）

```go
type MemoryMapping struct {
    ID          string     `gorm:"primarykey;type:uuid" json:"id"`
    MemoryID    string     `gorm:"type:uuid;not null;index:idx_memmap_memory" json:"memory_id"`
    AgentName   string     `gorm:"type:varchar(64);not null" json:"agent_name"`
    TargetPath  string     `gorm:"type:varchar(512);not null" json:"target_path"`
    FieldPath   string     `gorm:"type:varchar(256)" json:"field_path"`
    Transform   string     `gorm:"type:varchar(64);default:'identity'" json:"transform"`
    Enabled     bool       `gorm:"not null;default:true" json:"enabled"`
    Frozen      bool       `gorm:"not null;default:false" json:"frozen"`
    LastSyncAt  *time.Time `json:"last_sync_at"`
    LastSyncMD5 string     `gorm:"type:varchar(64)" json:"last_sync_md5"`
}

func (MemoryMapping) TableName() string { return "memory_mappings" }
```

#### SkillCurationLog（Skill 治理日志）

```go
type SkillCurationLog struct {
    ID        string    `gorm:"primarykey;type:uuid" json:"id"`
    SkillID   string    `gorm:"type:uuid;not null;index:idx_curation_skill" json:"skill_id"`
    OldState  string    `gorm:"type:varchar(16);not null" json:"old_state"`
    NewState  string    `gorm:"type:varchar(16);not null" json:"new_state"`
    Reason    string    `gorm:"type:text;not null" json:"reason"`
    CuratedAt time.Time `json:"curated_at"`
}

func (SkillCurationLog) TableName() string { return "skill_curation_log" }
```

### 8.4 Agent 与 Session 模型

#### AgentClient（Agent 客户端）

> v0.1 的 `AgentInstance` 在 v0.2 重命名为 `AgentClient`，字段基本兼容并新增 `ClientType` 枚举。

```go
type AgentClient struct {
    ID            string     `gorm:"primarykey;type:uuid" json:"id"`
    WorkspaceID   string     `gorm:"type:uuid;not null;index:idx_client_workspace" json:"workspace_id"`
    UserID        string     `gorm:"type:uuid;not null" json:"user_id"`
    ClientType    string     `gorm:"type:varchar(32);not null" json:"client_type"` // cursor / claude-code / opencode / windsurf / copilot
    ClientName    string     `gorm:"type:varchar(128)" json:"client_name"`
    ClientVersion string     `gorm:"type:varchar(32)" json:"client_version"`
    ProjectID     *string    `gorm:"type:uuid;index:idx_client_project" json:"project_id"`
    InstallPath   string     `gorm:"type:varchar(512)" json:"install_path"`
    Status        string     `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
    FirstSeenAt   time.Time  `json:"first_seen_at"`
    LastSeenAt    *time.Time `json:"last_seen_at"`
    CreatedAt     time.Time  `json:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at"`
}

func (AgentClient) TableName() string { return "agent_clients" }

type AgentClientType string

const (
    AgentClientTypeCursor     AgentClientType = "cursor"
    AgentClientTypeClaudeCode AgentClientType = "claude-code"
    AgentClientTypeOpenCode   AgentClientType = "opencode"
    AgentClientTypeWindsurf   AgentClientType = "windsurf"
    AgentClientTypeCopilot    AgentClientType = "copilot"
    AgentClientTypeUnknown    AgentClientType = "unknown"
)
```

#### MCPSession（MCP 会话）

```go
type MCPSession struct {
    ID              string     `gorm:"primarykey;type:uuid" json:"id"`
    WorkspaceID     string     `gorm:"type:uuid;not null;index:idx_session_workspace" json:"workspace_id"`
    UserID          string     `gorm:"type:uuid;not null" json:"user_id"`
    AgentClientID   string     `gorm:"type:uuid;not null;index:idx_session_client" json:"agent_client_id"`
    AccessTokenHash string     `gorm:"type:varchar(64);not null;index:idx_session_token" json:"-"`
    Scopes          string     `gorm:"type:text;default:'[]'" json:"scopes"`
    Status          string     `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
    StartedAt       time.Time  `gorm:"index:idx_session_start" json:"started_at"`
    LastActivityAt  *time.Time `json:"last_activity_at"`
    EndedAt         *time.Time `json:"ended_at"`
    ClientIP        string     `gorm:"type:varchar(64)" json:"client_ip"`
    UserAgent       string     `gorm:"type:varchar(512)" json:"user_agent"`
}

func (MCPSession) TableName() string { return "mcp_sessions" }

type MCPSessionStatus string

const (
    MCPSessionStatusActive  MCPSessionStatus = "active"
    MCPSessionStatusIdle    MCPSessionStatus = "idle"
    MCPSessionStatusClosed  MCPSessionStatus = "closed"
    MCPSessionStatusExpired MCPSessionStatus = "expired"
)
```

### 8.5 Tool 路由与策略模型

#### ConnectedMCPServer（外部 MCP Server 注册）

```go
type ConnectedMCPServer struct {
    ID                string     `gorm:"primarykey;type:uuid" json:"id"`
    WorkspaceID       string     `gorm:"type:uuid;not null;index:idx_server_workspace" json:"workspace_id"`
    Name              string     `gorm:"type:varchar(64);not null" json:"name"`  // 命名空间前缀
    DisplayName       string     `gorm:"type:varchar(128)" json:"display_name"`
    Endpoint          string     `gorm:"type:varchar(512);not null" json:"endpoint"`
    Transport         string     `gorm:"type:varchar(16);not null;default:'streamable_http'" json:"transport"`
    AuthType          string     `gorm:"type:varchar(16);not null;default:'none'" json:"auth_type"`
    AuthConfig        string     `gorm:"type:text" json:"-"` // 加密存储
    ToolsJSON         string     `gorm:"type:text;not null" json:"tools_json"`   // 注册时缓存的工具列表
    PolicyJSON        string     `gorm:"type:text;default:'{}'" json:"policy_json"` // 默认策略
    Status            string     `gorm:"type:varchar(16);not null;default:'pending'" json:"status"`
    LastHealthCheckAt *time.Time `json:"last_health_check_at"`
    CreatedAt         time.Time  `json:"created_at"`
    UpdatedAt         time.Time  `json:"updated_at"`
}

func (ConnectedMCPServer) TableName() string { return "connected_mcp_servers" }
```

#### ToolPolicy（工具策略）

```go
type ToolPolicy struct {
    ID                  string    `gorm:"primarykey;type:uuid" json:"id"`
    WorkspaceID         string    `gorm:"type:uuid;not null;index:idx_policy_workspace" json:"workspace_id"`
    ConnectedServerID   string    `gorm:"type:uuid;not null;index:idx_policy_server" json:"connected_server_id"`
    ToolName            string    `gorm:"type:varchar(128);not null" json:"tool_name"`
    Allowed             bool      `gorm:"not null;default:true" json:"allowed"`
    RequiresConfirmation bool     `gorm:"not null;default:false" json:"requires_confirmation"`
    MaxCallsPerDay      int       `gorm:"not null;default:0" json:"max_calls_per_day"` // 0 = 无限制
    MaxCallsPerUser     int       `gorm:"not null;default:0" json:"max_calls_per_user"`
    RiskLevel           string    `gorm:"type:varchar(16);not null;default:'low'" json:"risk_level"`
    CreatedAt           time.Time `json:"created_at"`
    UpdatedAt           time.Time `json:"updated_at"`
}

func (ToolPolicy) TableName() string { return "tool_policies" }

type ToolRiskLevel string

const (
    ToolRiskLow      ToolRiskLevel = "low"
    ToolRiskMedium   ToolRiskLevel = "medium"
    ToolRiskHigh     ToolRiskLevel = "high"
    ToolRiskCritical ToolRiskLevel = "critical"
)
```

#### ToolInvocationLog（工具调用日志）

```go
type ToolInvocationLog struct {
    ID                string    `gorm:"primarykey;type:uuid" json:"id"`
    WorkspaceID       string    `gorm:"type:uuid;not null" json:"workspace_id"`
    UserID            string    `gorm:"type:uuid;not null" json:"user_id"`
    AgentClientID     string    `gorm:"type:uuid" json:"agent_client_id"`
    MCPSessionID      string    `gorm:"type:uuid;index:idx_log_session" json:"mcp_session_id"`
    ToolName          string    `gorm:"type:varchar(128);not null;index:idx_log_tool_time" json:"tool_name"`
    ConnectedServerID *string   `gorm:"type:uuid" json:"connected_server_id"`
    InputJSON         string    `gorm:"type:text" json:"input_json"`
    OutputSummary     string    `gorm:"type:text" json:"output_summary"`
    Status            string    `gorm:"type:varchar(16);not null;index:idx_log_status" json:"status"`
    ErrorCode         string    `gorm:"type:varchar(64)" json:"error_code"`
    ErrorMessage      string    `gorm:"type:text" json:"error_message"`
    LatencyMs         int       `gorm:"not null;default:0" json:"latency_ms"`
    Confirmed         bool      `gorm:"not null;default:false" json:"confirmed"`
    InvokedAt         time.Time `gorm:"index:idx_log_invoked_at" json:"invoked_at"`
}

func (ToolInvocationLog) TableName() string { return "tool_invocation_logs" }
```

### 8.6 鉴权与计费模型

#### APIKey（API 密钥）

```go
type APIKey struct {
    ID          string     `gorm:"primarykey;type:uuid" json:"id"`
    WorkspaceID string     `gorm:"type:uuid;not null;index:idx_apikey_workspace" json:"workspace_id"`
    Name        string     `gorm:"type:varchar(128);not null" json:"name"`
    Prefix      string     `gorm:"type:varchar(16);not null;index:idx_apikey_prefix" json:"prefix"` // 用于显示的前缀
    Hash        string     `gorm:"type:varchar(128);not null" json:"-"` // Argon2 hash
    Scopes      string     `gorm:"type:text;default:'[]'" json:"scopes"`
    LastUsedAt  *time.Time `json:"last_used_at"`
    ExpiresAt   *time.Time `json:"expires_at"`
    CreatedBy   string     `gorm:"type:uuid" json:"created_by"`
    CreatedAt   time.Time  `json:"created_at"`
    RevokedAt   *time.Time `json:"revoked_at"`
}

func (APIKey) TableName() string { return "api_keys" }
```

#### OAuthToken（OAuth 令牌）

```go
type OAuthToken struct {
    ID           string     `gorm:"primarykey;type:uuid" json:"id"`
    UserID       string     `gorm:"type:uuid;not null;index:idx_oauth_user" json:"user_id"`
    Provider     string     `gorm:"type:varchar(32);not null" json:"provider"` // google / github / gitlab
    AccessToken  string     `gorm:"type:text;not null" json:"-"`              // 加密存储
    RefreshToken string     `gorm:"type:text" json:"-"`
    TokenType    string     `gorm:"type:varchar(16);default:'Bearer'" json:"token_type"`
    Scope        string     `gorm:"type:text" json:"scope"`
    ExpiresAt    *time.Time `json:"expires_at"`
    CreatedAt    time.Time  `json:"created_at"`
    UpdatedAt    time.Time  `json:"updated_at"`
}

func (OAuthToken) TableName() string { return "oauth_tokens" }
```

#### UsageRecord（用量记录）

```go
type UsageRecord struct {
    ID          string    `gorm:"primarykey;type:uuid" json:"id"`
    WorkspaceID string    `gorm:"type:uuid;not null;index:idx_usage_workspace" json:"workspace_id"`
    UserID      string    `gorm:"type:uuid" json:"user_id"`
    Metric      string    `gorm:"type:varchar(32);not null;index:idx_usage_metric" json:"metric"` // tool_call / memory_write / vector_query
    Quantity    int       `gorm:"not null" json:"quantity"`
    Period      string    `gorm:"type:varchar(16);not null;index:idx_usage_period" json:"period"` // YYYY-MM-DD / YYYY-MM
    RecordedAt  time.Time `gorm:"index:idx_usage_time" json:"recorded_at"`
}

func (UsageRecord) TableName() string { return "usage_records" }
```

#### AuditLog（审计日志，append-only）

```go
type AuditLog struct {
    ID          string    `gorm:"primarykey;type:uuid" json:"id"`
    WorkspaceID string    `gorm:"type:uuid;not null;index:idx_audit_workspace" json:"workspace_id"`
    Actor       string    `gorm:"type:varchar(128);not null" json:"actor"`      // user_id / api_key_id / system
    ActorType   string    `gorm:"type:varchar(16);not null" json:"actor_type"` // user / api_key / system
    Action      string    `gorm:"type:varchar(64);not null;index:idx_audit_action" json:"action"` // memory.delete / rule.update / tool.invoke
    Target      string    `gorm:"type:varchar(128)" json:"target"`
    TargetType  string    `gorm:"type:varchar(32)" json:"target_type"`
    Payload     string    `gorm:"type:text" json:"payload"` // JSON 详情
    ClientIP    string    `gorm:"type:varchar(64)" json:"client_ip"`
    CreatedAt   time.Time `gorm:"index:idx_audit_time" json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }
```

> **审计日志不可变约束**：所有 `audit_logs` 行禁止 `UPDATE` / `DELETE`。PostgreSQL 通过 `REVOKE UPDATE, DELETE ON audit_logs FROM app_user` + 触发器 `RAISE EXCEPTION` 兜底。

### 8.7 实体关系概览

```
Organization (1) ──< Workspace (n)
Workspace    (1) ──< WorkspaceMember (n) >── User
Workspace    (1) ──< Rule
Workspace    (1) ──< Memory ──< MemoryValidity
Workspace    (1) ──< MemorySnapshot
Workspace    (1) ──< MemoryAccessLog
Workspace    (1) ──< AgentClient ──< MCPSession
Workspace    (1) ──< ConnectedMCPServer ──< ToolPolicy
Workspace    (1) ──< ToolInvocationLog
Workspace    (1) ──< APIKey
Workspace    (1) ──< UsageRecord
Workspace    (1) ──< AuditLog
Workspace    (1) ──< OutputPreference >── User
```

### 8.8 字段级租户隔离约束

所有 `Workspace` 隔离的实体必须在表内保留 `workspace_id` 字段，并通过三层防御保证租户隔离：

1. **应用层 GORM Plugin** 自动注入 `WHERE workspace_id = ?`（详见 [第 9.6 节 GORM Workspace 插件](#96-gorm-workspace-插件)）
2. **数据库层 RLS 兜底**（详见 [第 9.5 节 行级安全（RLS）](#95-行级安全rls)）
3. **GORM Tag 复合索引** `(workspace_id, ...)` 保证查询效率

> **强制约束**：所有 v0.1 旧表（`memories` / `memory_snapshots` / `memory_access_log` / `skill_curation_log` / `memory_mappings`）必须在 migration 阶段加 `workspace_id` 字段，并通过默认赋值 `personal_workspace` 兼容旧数据。

---

## 附录 A. Local Bridge Adapter 设计（P2 可选）

> 本附录描述 **Local Bridge 插件** 的 Adapter 设计。Local Bridge 是 v0.2 的可选组件（P2 交付），不阻塞 P0/P1 进度。
>
> **使用场景**：当用户希望 Open Agent Hub 把配置 / 记忆 同步到本地 Agent 配置文件（如 Cursor `.cursor/mcp.json`）时启动。SaaS 模式下不需要。

### A.1 Adapter 接口

```go
type AgentAdapter interface {
    Name() string
    DisplayName() string
    Description() string
    DiscoverConfigs(ctx context.Context) ([]ConfigLocation, error)
    ReadConfig(ctx context.Context, path string) (*ConfigContent, error)
    WriteConfig(ctx context.Context, path string, content *ConfigContent) error
    ConfigSchema() *AdapterSchema
    ValidateConfig(content *ConfigContent) error
    DetectInstallation(ctx context.Context) (*InstallInfo, error)
}

type ConfigLocation struct {
    Path        string     `json:"path"`
    Type        ConfigType `json:"type"`
    Description string     `json:"description"`
    Format      string     `json:"format"`
    Scope       Scope      `json:"scope"`
    Writable    bool       `json:"writable"`
}

type ConfigContent struct {
    Raw         []byte       `json:"raw"`
    Format      string       `json:"format"`
    Parsed      interface{}  `json:"parsed"`
    ContentType string       `json:"content_type"`
}

type AdapterSchema struct {
    Fields     []SchemaField   `json:"fields"`
    Formats    []string        `json:"formats"`
    Transforms []TransformRule `json:"transforms"`
}

type SchemaField struct {
    Path        string `json:"path"`
    Type        string `json:"type"`
    Description string `json:"description"`
    Required    bool   `json:"required"`
}

type TransformRule struct {
    Name     string     `json:"name"`
    FromType ConfigType `json:"from_type"`
    Template string     `json:"template"`
}

type InstallInfo struct {
    Installed bool   `json:"installed"`
    Path      string `json:"path"`
    Version   string `json:"version"`
}
```

### A.2 预置 Adapter 规格

#### A.2.1 Cursor Adapter

| 配置项 | 文件路径 | 格式 | 类型 |
|--------|----------|------|------|
| 项目规则 | `.cursor/rules/*.mdc` | Markdown + Frontmatter | rule |
| 全局规则 | `~/.cursor/rules/*.mdc` | Markdown + Frontmatter | rule |
| 旧格式规则 | `.cursorrules` | Markdown | rule |
| MCP 配置 | `.cursor/mcp.json` | JSON | mcp_server |

**MCP 配置格式：**
```json
{
  "mcpServers": {
    "server-name": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-xxx"],
      "env": { "KEY": "value" }
    }
  }
}
```

#### A.2.2 Claude Code Adapter

| 配置项 | 文件路径 | 格式 | 类型 |
|--------|----------|------|------|
| 项目指令 | `CLAUDE.md` | Markdown | system_prompt / rule |
| 用户指令 | `~/.claude/CLAUDE.md` | Markdown | system_prompt / rule |
| MCP 配置 | `.claude/mcp.json` | JSON | mcp_server |
| 权限配置 | `.claude/settings.json` | JSON | permission |

**MCP 配置格式：**
```json
{
  "mcpServers": {
    "server-name": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-xxx"],
      "env": { "KEY": "value" }
    }
  }
}
```

#### A.2.3 OpenCode Adapter

| 配置项 | 文件路径 | 格式 | 类型 |
|--------|----------|------|------|
| 主配置 | `opencode.json` | JSON | rule / environment |
| Agent 配置 | `.opencode/agents/` | JSON/YAML | rule |
| Skill 配置 | `.opencode/skills/` | Markdown | rule |
| MCP 配置 | `opencode.json` → `mcp` 字段 | JSON | mcp_server |

#### A.2.4 GitHub Copilot Adapter

| 配置项 | 文件路径 | 格式 | 类型 |
|--------|----------|------|------|
| 项目指令 | `.github/copilot-instructions.md` | Markdown | rule |
| MCP 配置 | `.github/copilot-mcp.json` | JSON | mcp_server |

#### A.2.5 Windsurf Adapter

| 配置项 | 文件路径 | 格式 | 类型 |
|--------|----------|------|------|
| 规则 | `.windsurfrules` | Markdown | rule |
| MCP 配置 | `.windsurf/mcp.json` | JSON | mcp_server |

### A.3 Adapter 注册表

```go
type AdapterRegistry struct {
    adapters map[string]AgentAdapter
    mu       sync.RWMutex
}

func (r *AdapterRegistry) Register(adapter AgentAdapter)
func (r *AdapterRegistry) Get(name string) (AgentAdapter, bool)
func (r *AdapterRegistry) List() []AgentAdapter
func (r *AdapterRegistry) AutoDiscover(ctx context.Context) ([]*AgentInstance, error)
```

## 附录 B. Local Bridge 同步引擎（P2 可选）

> 本附录描述 Local Bridge 同步引擎设计。Local Bridge 作为独立 daemon，通过 Workspace MCP Token 与 SaaS 后端通信，实现本地文件与云端的双向同步。

### B.1 同步引擎核心

```go
type SyncHub struct {
    registry    *AdapterRegistry
    watchers    map[string]*FileWatcher
    transformer *Transformer
    resolver    *ConflictResolver
    eventCh     chan SyncEvent
    mu          sync.RWMutex
}

func (h *SyncHub) PushSync(ctx context.Context, configID string) ([]SyncResult, error)
func (h *SyncHub) PullSync(ctx context.Context, agentName, configPath string) (*SyncResult, error)
func (h *SyncHub) FullSync(ctx context.Context) ([]SyncResult, error)
func (h *SyncHub) StartWatching(ctx context.Context) error
func (h *SyncHub) StopWatching(ctx context.Context) error
```

### B.2 Push 同步流程（全局 → Agent）

```
1. 读取 GlobalConfig 及其 Mappings
2. 遍历每个 AgentMapping:
   a. 获取对应 Adapter
   b. 读取当前 Agent 配置文件内容 (用于冲突检测)
   c. 计算 MD5，与 LastSyncMD5 对比:
      - 相同: 无外部变更，直接写入
      - 不同: 检测冲突
   d. 使用 Transformer 将全局配置值转换为目标格式
   e. 如果检测到冲突:
      - 记录 Conflict 到 DB
      - 通过 WebSocket 通知前端
      - 跳过本次写入
   f. 无冲突: 调用 Adapter.WriteConfig() 写入
   g. 更新 Mapping.LastSyncMD5 和 LastSyncAt
   h. 记录 SyncRecord
3. 返回同步结果
```

### B.3 Pull 同步流程（Agent → 全局）

```
1. File Watcher 检测到 Agent 配置文件变更
2. 获取对应 Adapter
3. 读取变更后的配置文件内容
4. 查找关联的 GlobalConfig 和 Mapping
5. 使用 Transformer 反向转换回全局格式
6. 与当前全局配置值对比:
   - 无差异: 跳过
   - 有差异且无冲突: 更新全局配置
   - 有冲突: 记录 Conflict，通知前端
7. 记录 SyncRecord
```

### B.4 Transformer 转换规则

```go
type Transformer struct {
    rules map[string]TransformFunc
}

type TransformFunc func(value interface{}, mapping AgentMapping) (interface{}, error)
```

预置转换规则：

| 转换标识 | 源格式 | 目标格式 | 说明 |
|----------|--------|----------|------|
| `mcp_to_cursor` | 统一 MCP | Cursor MCP JSON | 嵌入 `mcpServers` |
| `mcp_to_claude` | 统一 MCP | Claude MCP JSON | 嵌入 `mcpServers` |
| `mcp_to_opencode` | 统一 MCP | OpenCode JSON | 写入 `mcp` 字段 |
| `rule_to_mdc` | Markdown | Cursor .mdc 格式 | 添加 Frontmatter |
| `rule_to_md` | Markdown | 纯 Markdown | 直接写入 |
| `identity` | 任意 | 相同 | 无转换，直接写入 |

#### B.4.1 MCP 多配置合并规则

当多个 `Type=mcp_server` 的 GlobalConfig 映射到同一 Agent 的同一配置文件时（如 Cursor 的 `.cursor/mcp.json`），需要合并写入：

```
1. 读取目标配置文件的现有 mcpServers 对象
2. 遍历所有映射到该文件的 mcp_server 类型全局配置
3. 每个全局配置的 Value 为一个独立的 MCP Server 定义 JSON：
   { "server-name": { "command": "...", "args": [...], "env": {...} } }
4. 将所有 MCP Server 定义合并到目标文件的 mcpServers 字段中
5. 写入时保留目标文件中未被任何映射管理的 mcpServers 条目（用户手动添加的）
6. 删除映射中已不存在的旧条目（对比 LastSyncMD5）
```

### B.5 文件监听

```go
type FileWatcher struct {
    watcher  *fsnotify.Watcher
    paths    map[string]WatchTarget
    debounce time.Duration
}

type WatchTarget struct {
    AgentName  string
    ConfigPath string
    ConfigType ConfigType
}
```

## 9. 数据库设计（PostgreSQL + pgvector）

> 本章为多租户 SaaS 化后的数据库设计。生产环境强制使用 **PostgreSQL 14+** + **pgvector 扩展**；保留 SQLite 作为开发环境；引入 **golang-migrate** 替代 AutoMigrate 作为生产迁移工具。

### 9.1 数据库选型

| 环境 | 选型 | 启用条件 | 说明 |
|------|------|---------|------|
| 生产 | PostgreSQL 14+ | 默认 | 支多并发、行级锁、JSONB、分区表 |
| 向量检索 | pgvector 0.5+ | Memory 功能启用 | HNSW 索引，1536 维 float32 |
| 共享缓存 | Redis 7+ | 部署启动 | Session / 限流 / 缓存 / 广播 |
| 开发 | SQLite (glebarez/sqlite) | `db-type=sqlite` | 单文件，与生产 schema 同构（不支持分区表与 RLS） |

**双引擎架构**：

```go
// config.yaml
db:
  type: postgres                    # postgres | sqlite
  dsn: postgres://app:secret@db:5432/openagenthub?sslmode=require
  max_open_conns: 50
  max_idle_conns: 10
  conn_max_lifetime: 30m

vector:
  enabled: true                     # false 时退化为关键词检索
  extension: pgvector
  embedding_dim: 1536
  hnsw_m: 16
  hnsw_ef_construction: 64
```

### 9.2 表结构总览

所有 GORM 模型定义见 [第 8 章 核心数据模型](#8-核心数据模型多租户版)。本节列出所有表与关键表选项：

| 表名 | 实体 | 分区 | RLS | 重要索引 |
|------|------|------|-----|----------|
| `organizations` | Organization | ✗ | ✗ | slug (uniq) |
| `workspaces` | Workspace | ✗ | ✗ | (org_id) |
| `users` | User | ✗ | ✗ | email (uniq) |
| `workspace_members` | WorkspaceMember | ✗ | ✗ | (workspace_id, user_id) |
| `rules` | Rule | ✗ | ✔ | (workspace_id, type), (workspace_id, scope) |
| `output_preferences` | OutputPreference | ✗ | ✔ | (workspace_id, user_id, key) |
| `memories` | Memory | ✗ | ✔ | (workspace_id, type), (workspace_id, pinned), vector HNSW |
| `memory_validity` | MemoryValidity | ✗ | ✔ | (memory_id) |
| `memory_snapshots` | MemorySnapshot | ✗ | ✔ | (workspace_id, session_id) |
| `memory_access_log` | MemoryAccessLog | 按月分区 | ✔ | (memory_id, accessed_at) |
| `memory_mappings` | MemoryMapping | ✗ | ✗¹ | (memory_id) |
| `skill_curation_log` | SkillCurationLog | ✗ | ✗¹ | (skill_id) |
| `agent_clients` | AgentClient | ✗ | ✗¹ | (workspace_id, client_type) |
| `mcp_sessions` | MCPSession | ✗ | ✗¹ | (workspace_id, status) |
| `connected_mcp_servers` | ConnectedMCPServer | ✗ | ✔ | (workspace_id, name) |
| `tool_policies` | ToolPolicy | ✗ | ✔ | (workspace_id, tool_name) |
| `tool_invocation_logs` | ToolInvocationLog | 按月分区 | ✔ | (workspace_id, tool_name, invoked_at) |
| `api_keys` | APIKey | ✗ | ✗¹ | prefix, (workspace_id) |
| `oauth_tokens` | OAuthToken | ✗ | ✗² | (user_id, provider) |
| `usage_records` | UsageRecord | 按月分区 | ✗¹ | (workspace_id, metric, period) |
| `audit_logs` | AuditLog | 按月分区 | ✗¹ | (workspace_id, action, created_at) |

> ¹ 不启用 RLS但 仍需 `workspace_id` 过滤（应用层控制即可）  
> ² `oauth_tokens` 以 user 为主，不设 RLS

### 9.3 GORM 模型注册

```go
// initialize/gorm_biz.go
func RegisterTables() {
    db := global.GLB_DB
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    err := db.WithContext(ctx).AutoMigrate(
        // 租户模型
        model.Organization{},
        model.Workspace{},
        model.User{},
        model.WorkspaceMember{},
        // 规则与配置
        model.Rule{},
        model.OutputPreference{},
        // 记忆系统
        model.Memory{},
        model.MemoryValidity{},
        model.MemoryMapping{},
        model.MemorySnapshot{},
        model.MemoryAccessLog{},
        model.SkillCurationLog{},
        // Agent 与 Session
        model.AgentClient{},
        model.MCPSession{},
        // Tool 路由与策略
        model.ConnectedMCPServer{},
        model.ToolPolicy{},
        model.ToolInvocationLog{},
        // 鉴权与计费
        model.APIKey{},
        model.OAuthToken{},
        model.UsageRecord{},
        model.AuditLog{},
    )
    if err != nil {
        global.GLB_LOG.Error("register table failed", zap.Error(err))
    }
}
```

> **AutoMigrate 仅用于开发环境**。生产环境必须使用 [第 9.7 节 迁移策略](#97-迁移策略) 描述的 `golang-migrate`。

### 9.4 索引策略

**复合索引模板**：

```go
// 列表查询：按 workspace 过滤后排序
type Memory struct {
    ...
    WorkspaceID string `gorm:"type:uuid;not null;index:idx_memory_workspace_type,priority:1;index:idx_memory_workspace_pinned,priority:1"`
    Type        string `gorm:"type:varchar(32);index:idx_memory_workspace_type,priority:2"`
    Pinned      bool   `gorm:"index:idx_memory_workspace_pinned,priority:2"`
}
```

生成的索引：

```sql
CREATE INDEX idx_memory_workspace_type ON memories (workspace_id, type);
CREATE INDEX idx_memory_workspace_pinned ON memories (workspace_id, pinned);
```

**向量索引**（HNSW）：

```sql
CREATE INDEX idx_memories_embedding ON memories
USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
```

### 9.5 行级安全（RLS）

**所有需要租户隔离的业务表** 必须启用 RLS 兑底，示例：

```sql
-- 启用 RLS
ALTER TABLE memories ENABLE ROW LEVEL SECURITY;
ALTER TABLE memories FORCE ROW LEVEL SECURITY;

-- 策略：只允许访问当前 workspace 的记录
CREATE POLICY memories_tenant_isolation ON memories
    USING (workspace_id = current_setting('app.current_workspace_id', true)::uuid);

-- 业务角色仅授于必要权限
REVOKE ALL ON memories FROM app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON memories TO app_user;
```

**GORM 连接初始化注入**：

```go
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    // 每个连接 session 变量由业务中间件设置
})

// 在业务中间件中：
func TenantContextMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        workspaceID := getWorkspaceID(c)
        // gorm session 设置
        db := global.GLB_DB.Session(&gorm.Session{
            Context: context.WithValue(c.Request.Context(), "workspace_id", workspaceID),
        })
        c.Set("tenant_db", db)

        // raw sql 设置 session 变量，使 RLS 生效
        db.Exec(fmt.Sprintf("SET app.current_workspace_id = '%s'", workspaceID))
        c.Next()
    }
}
```

### 9.6 GORM Workspace 插件

为避免每次查询手动加 `WHERE workspace_id = ?`，实现一个 GORM Plugin 自动注入：

```go
// plugin/tenant/plugin.go
type TenantPlugin struct {
    workspaceKey string
}

func (p *TenantPlugin) Name() string { return "tenant-plugin" }

func (p *TenantPlugin) Initialize(db *gorm.DB) error {
    // 拦截 query / update / delete 调用
    return db.Callback().Query().Before("gorm:query").Register("tenant:query", p.beforeQuery)
}

// 仅对需要隔离的表生效（黑名单中以跳过）
var tenantSkippedTables = map[string]bool{
    "organizations": true,
    "users":         true,
    "oauth_tokens":  true,
}

func (p *TenantPlugin) beforeQuery(db *gorm.DB) {
    if db.Statement.Schema == nil {
        return
    }
    tableName := db.Statement.Schema.Table
    if tenantSkippedTables[tableName] {
        return
    }
    workspaceID, ok := db.Statement.Context.Value(p.workspaceKey).(string)
    if !ok || workspaceID == "" {
        return
    }
    db.Statement.AddClause(clause.Where{
        Exprs: []clause.Expression{
            clause.Eq{Column: clause.Column{Name: "workspace_id"}, Value: workspaceID},
        },
    })
}
```

**插件使用**：

```go
import "github.com/openagenthub/plugin/tenant"

db.Use(&tenant.TenantPlugin{workspaceKey: "workspace_id"})
```

### 9.7 迁移策略

**生产使用 `golang-migrate`**（与 GORM AutoMigrate 不同时启用）。

迁移文件结构：

```
migrations/
├── 0001_create_tenant_tables.up.sql
├── 0001_create_tenant_tables.down.sql
├── 0002_create_rule_and_memory.up.sql
├── 0002_create_rule_and_memory.down.sql
├── 0003_add_vector_embedding.up.sql          -- v0.2 新增
├── 0003_add_vector_embedding.down.sql
├── 0004_backfill_workspace_id.up.sql        -- 旧数据迁移
├── 0004_backfill_workspace_id.down.sql
├── 0005_enable_rls.up.sql                   -- v0.2 RLS 启用
├── 0005_enable_rls.down.sql
├── 0006_create_partition_tables.up.sql      -- tool_invocation_logs / memory_access_log
└── 0006_create_partition_tables.down.sql
```

**启动时检查**：

```go
// initialize/migrate.go
func RunMigrations() {
    m, err := migrate.New(
        "file://migrations",
        global.GLB_CONFIG.DB.DSN,
    )
    if err != nil { log.Fatal(err) }
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        log.Fatal(err)
    }
}
```

**schema 变更流程**：

1. 修改 GORM model 定义
2. 编写 `migrations/00NN_*.up.sql` / `.down.sql`（必须可逆向）
3. 在 CI 中运行 `migrate up && migrate down` 验证双向幂等
4. 更新 `initialize/gorm_biz.go` AutoMigrate 列表（仅供 dev 环境）
5. 部署后启动时自动应用未执行的迁移

**重大变更需额外处理**：

- 加列 / 索引：零停机
- 修改列类型 / 加约束：使用 `ALTER TABLE ... ALGORITHM=INPLACE, LOCK=NONE` 避免表锁
- 启用 RLS：需同时修改应用层查询路径，可能需要双写期

### 9.8 分区表策略

**`tool_invocation_logs` 按月分区**：

```sql
CREATE TABLE tool_invocation_logs (
    id              UUID NOT NULL,
    workspace_id    UUID NOT NULL,
    -- 其他字段
    invoked_at      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (id, invoked_at)
) PARTITION BY RANGE (invoked_at);

-- 创建分区
CREATE TABLE tool_invocation_logs_2026_06 PARTITION OF tool_invocation_logs
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

-- 预创建未来 3 个月
-- 自动任务：每月 25 日检查并预创建下月分区
```

同样为 `memory_access_log` / `usage_records` / `audit_logs` 启用按月分区。

### 9.9 Vector 索引与混合检索

**HNSW 索引参数选型**：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `m` | 16 | 每节点邻居数，16-32 平衡精度与内存 |
| `ef_construction` | 64 | 构建时召回深度，越高越准越慢 |
| `ef_search` | 40 (查询时设置) | 查询召回深度 |

**混合检索（BM25 + 向量）**：

```sql
-- 向量召回
SELECT id, content, 1 - (embedding <=> $1) AS score
FROM memories
WHERE workspace_id = $2
ORDER BY embedding <=> $1
LIMIT 50;

-- BM25 关键词召回
SELECT id, content, ts_rank_cd(content_tsv, query) AS score
FROM memories, to_tsquery('simple', $1) query
WHERE workspace_id = $2
  AND content_tsv @@ query
ORDER BY score DESC
LIMIT 50;

-- 融合：使用 Reciprocal Rank Fusion (RRF)
WITH vec AS (...向量召回...),
     kw AS (...BM25 召回...)
SELECT id, content,
       (1.0 / (60 + vec_rank)) + (1.0 / (60 + kw_rank)) AS rrf_score
FROM ...
ORDER BY rrf_score DESC
LIMIT 10;
```

**降级策略**：向量索引不可用时退化为纯关键词检索（需在 `Memory.Embedding` 为 NULL 时自动跳过 HNSW 路径）。

### 9.10 旧数据迁移策略

**Step 1：创建 default workspace**

```sql
-- 为旧 v0.1 数据提供默认 workspace
INSERT INTO workspaces (id, org_id, name, slug, status, created_at, updated_at)
SELECT
    gen_random_uuid(),
    (SELECT id FROM organizations WHERE slug = 'legacy'),
    'Personal Workspace (Legacy)',
    'personal-legacy',
    'active',
    NOW(),
    NOW()
FROM organizations LIMIT 1;

-- 默认 workspace ID 变量
\set personal_workspace_id `SELECT id FROM workspaces WHERE slug = 'personal-legacy' LIMIT 1`
```

**Step 2：回填 workspace_id**

```sql
-- 旧表回填
UPDATE memories SET workspace_id = :'personal_workspace_id' WHERE workspace_id IS NULL;
UPDATE memory_snapshots SET workspace_id = :'personal_workspace_id' WHERE workspace_id IS NULL;
UPDATE memory_access_log SET workspace_id = :'personal_workspace_id' WHERE workspace_id IS NULL;
UPDATE skill_curation_log SET workspace_id = :'personal_workspace_id' WHERE workspace_id IS NULL;
```

**Step 3：保留 legacy_user_id 兼容字段**

```sql
-- 保留旧 user_id 供兼容 API 使用
ALTER TABLE memories ADD COLUMN legacy_user_id VARCHAR(64);
UPDATE memories SET legacy_user_id = user_id;
CREATE INDEX idx_memories_legacy_user ON memories (legacy_user_id);
```

---

## 10. 记忆系统（向量化升级版）

> 本章是 **Open Agent Hub 记忆系统** 的完整设计。**v0.1 的 14.2 分类体系 / 14.5 写入纪律 / 14.6 双时间轴 / 14.7 冻结快照 / 14.9 Skill 三状态机 全部以 95% 以上代码保留**。v0.2 升级点：
> 1. **多租户改造**：`user_id` 升级为 `workspace_id` + `project_id` 二级隔离
> 2. **向量化**：新增 `Embedding vector(1536)` 字段与 `EmbeddingModel` 记录
> 3. **召回引擎**：从纯文本检索升级为 BM25 + 向量混合检索（详见 [第 10.10 节](#1010-向量召回引擎新增)）
> 4. **删除 PullSync**：在 SaaS 化架构下，Local Bridge 仅作为可选 P2 组件，本章描述的是纯 SaaS 记忆流程

### 10.1 设计背景与动机

当前 spec 覆盖了「配置同步」（规则、MCP Server、权限等静态配置的分发），但 Agent 在使用过程中产生的**动态记忆**——用户偏好、交互事实、工作流经验——同样是需要跨 Agent 共享的关键数据。

配置与记忆的区别：

| 维度 | 配置 (Config) | 记忆 (Memory) |
|------|---------------|---------------|
| 来源 | 用户手动编写 | Agent 从交互中自动提取 |
| 变化频率 | 低（按需修改） | 高（每次对话可能产生新记忆）|
| 格式 | 结构化 (JSON/YAML/MD) | 混合（结构化事实 + 非结构化文本）|
| 同步策略 | 即时写入 | 需要写入纪律（不是所有内容都值得持久化）|
| 失效方式 | 用户主动删除 | 需要自动失效机制（过时、冲突、低价值）|
| 与 Prompt Cache 的关系 | 低频变更，Cache 影响小 | 高频写入，与 Cache 前缀稳定性天然冲突 |

参考 AWS 博客《存之有序，治之有矩——Agent 记忆系统的工程实践与演进》中的工程实践，本节将 Memory 系统纳入 Open Agent Hub 的统一同步框架。

### 10.2 记忆分类体系

```
Agent Memory
├── 陈述性记忆 (Declarative)          — "我记得什么事实"
│   ├── 语义记忆 (Semantic)           — 客观事实与知识点
│   ├── 用户偏好 (User Preference)    — 用户的风格/习惯选择
│   └── 情节记忆 (Episodic)          — 完整交互经历（可复盘）
│
└── 程序性记忆 (Procedural)           — "我记得怎么做"
    ├── 人工策展 Skill                — 开发者/用户编写的 Skill
    └── Agent 自产 Skill              — Agent 从成功工作流中蒸馏的 Skill
```

### 10.3 记忆数据模型（多租户版）

完整定义见 [第 8.3 节 记忆系统模型](#83-记忆系统模型)。核心变化：

- **多租户字段**：`OrgID` + `WorkspaceID` (+ `ProjectID` 可选) 保证租户隔离
- **向量字段**：`Embedding vector(1536)` + `EmbeddingModel` + `EmbeddedAt` 支持语义检索
- **索引调整**：`idx_memory_workspace_type` 复合索引替换原 `idx_memory_user_type`

```go
// 节选关键字段（其他字段同 v0.1）
type Memory struct {
    ID          string     `gorm:"primarykey;type:uuid" json:"id"`
    OrgID       string     `gorm:"type:uuid;not null;index:idx_memory_org" json:"org_id"`
    WorkspaceID string     `gorm:"type:uuid;not null;index:idx_memory_workspace" json:"workspace_id"`
    ProjectID   *string    `gorm:"type:uuid;index:idx_memory_project" json:"project_id"`
    UserID      string     `gorm:"type:uuid;not null" json:"user_id"`
    // ... 业务字段同上 ...
    Embedding      []byte     `gorm:"type:vector(1536)" json:"-"`
    EmbeddingModel string     `gorm:"type:varchar(64)" json:"embedding_model"`
    EmbeddedAt     *time.Time `json:"embedded_at"`
}

type MemoryType string

const (
    MemoryTypeSemantic       MemoryType = "semantic"
    MemoryTypeUserPreference MemoryType = "user_preference"
    MemoryTypeEpisodic       MemoryType = "episodic"
    MemoryTypeSkill          MemoryType = "skill"
)

type MemoryCategory string

const (
    MemoryCategoryDeclarative MemoryCategory = "declarative"
    MemoryCategoryProcedural  MemoryCategory = "procedural"
)

type MemoryProvenance string

const (
    ProvenanceHumanCurated   MemoryProvenance = "human_curated"
    ProvenanceAgentExtracted MemoryProvenance = "agent_extracted"
    ProvenanceAgentAuthored  MemoryProvenance = "agent_authored"
)

type SkillState string

const (
    SkillStateActive   SkillState = "active"
    SkillStateStale    SkillState = "stale"
    SkillStateArchived SkillState = "archived"
)
```

> `State` 字段仅对 `Type=skill` 的记忆有效，用于 Skill 三状态机（详见 [第 10.8 节](#108-skill-三状态机)）。其他类型记忆默认 `active` 且不可变更。

### 10.4 写入纪律

#### 10.4.1 写入策略选择

**策略一：LLM 判官（高准确率，高成本）**

```
每次写入触发双 LLM 架构:
  1. 信息提取 LLM: 从对话中提取候选记忆
  2. 决策 LLM: 对候选判断 ADD / UPDATE / DELETE / NONE

适用场景: 高价值记忆（用户关键背景、过敏信息），低频写入
成本: 每次写入消耗 2 次 LLM 调用
```

**策略二：公式打分（零推理成本，确定性）**

```
后台周期性任务 (默认每天凌晨 3 点):
  Light 阶段 → REM 阶段 → Deep 阶段

Deep 阶段六维打分:
  ┌────────────────────┬──────┬──────────────────────────────┐
  │ 维度                │ 权重 │ 描述                          │
  ├────────────────────┼──────┼──────────────────────────────┤
  │ Frequency (频次)    │ 0.24 │ 被引用次数                    │
  │ Relevance (相关度)  │ 0.30 │ 召回后的相关性评分均值        │
  │ QueryDiversity      │ 0.15 │ 被多少种不同问题触发          │
  │ Recency (新鲜度)    │ 0.15 │ 最近是否被用过                │
  │ Consolidation       │ 0.10 │ 是否跨日复现                  │
  │ ConceptualRichness  │ 0.06 │ 概念标签密度                  │
  └────────────────────┴──────┴──────────────────────────────┘

三重硬门槛 (同时满足才晋升长期记忆):
  - minScore: 综合分 >= 阈值 (默认 0.6)
  - minRecallCount: 至少被召回 N 次 (默认 3)
  - minUniqueQueries: 至少被 N 种不同查询触发 (默认 2)
```

**策略三：托管策略（零运维，内置兏底）**

```
预置四种策略模板:
  - SemanticMemoryStrategy: 提取客观事实与知识点
  - SummaryMemoryStrategy: 压缩长对话为短摘要
  - UserPreferenceMemoryStrategy: 提取用户偏好/风格
  - EpisodicMemoryStrategy: 保留完整交互经历

每种策略底层: Extraction → Consolidation
可覆写各阶段的 system prompt 指令
```

#### 10.4.2 写入纪律引擎

```go
type WriteDisciplineEngine struct {
    strategy WriteStrategy
    scorer   *MemoryScorer
    embedder EmbeddingClient  // v0.2 新增
}

type WriteStrategy interface {
    Evaluate(ctx context.Context, candidate *MemoryCandidate) (*WriteDecision, error)
}

type WriteDecision struct {
    Action   WriteAction `json:"action"`
    TargetID *string     `json:"target_id"`
    Score    float64     `json:"score"`
    Reason   string      `json:"reason"`
}

type WriteAction string

const (
    WriteActionAdd    WriteAction = "add"
    WriteActionUpdate WriteAction = "update"
    WriteActionDelete WriteAction = "delete"
    WriteActionNone   WriteAction = "none"
)
```

#### 10.4.3 写入后嵌入（v0.2 新增）

> v0.2 中所有新写入或更新的记忆 **必须同步生成 Embedding 并写入 vector(1536) 字段**，供后续语义检索使用。

```go
func (e *WriteDisciplineEngine) embed(ctx context.Context, m *Memory) error {
    vec, err := e.embedder.Embed(ctx, m.Content)
    if err != nil {
        return err
    }
    // pgvector 存储为 bytea (float32 little-endian 1536 维)
    buf := new(bytes.Buffer)
    for _, v := range vec {
        binary.Write(buf, binary.LittleEndian, v)
    }
    m.Embedding = buf.Bytes()
    m.EmbeddingModel = e.embedder.ModelName()
    now := time.Now()
    m.EmbeddedAt = &now
    return nil
}
```

**嵌入服务抽象**：

```go
type EmbeddingClient interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    ModelName() string
}

// 生产实现：OpenAI text-embedding-3-small
type OpenAIEmbedder struct { client *openai.Client }
```

### 10.5 失效机制与双时间轴

| 场景 | 问题 | 解决方案 |
|------|------|----------|
| 低频但重要 | 用户关键背景被访问频率淘汰误伤 | 设置 `pinned: true` 标记，豁免淘汰 |
| 时间新但语义旧 | 用户复述三年前状态的记录，时间戳新不等于内容新 | 语义去重：写入前用向量相似度检测语义重复 |
| 并存而非冲突 | 三年前保守 vs 今年激进，两条都真实 | 双时间轴 (bi-temporal)：保留历史，查询时按有效时间窗口过滤 |

#### 10.5.1 双时间轴模型

完整定义见 [第 8.3 节 MemoryValidity](#83-记忆系统模型)。

查询逻辑：

```
查询"当前有效的记忆":  ValidUntil IS NULL AND SupersededBy IS NULL
查询"2024年的记忆":   ValidFrom <= '2024-12-31' AND (ValidUntil IS NULL OR ValidUntil > '2024-01-01')
```

旧记忆不删除，只标记为 superseded，支持历史回溯。

#### 10.5.2 语义去重（v0.2 新增）

写入前使用向量检索检测是否已有语义重复的记忆：

```go
func (e *WriteDisciplineEngine) detectSemanticDuplication(ctx context.Context, candidate *Memory) (*Memory, float64, error) {
    if candidate.Embedding == nil {
        if err := e.embed(ctx, candidate); err != nil {
            return nil, 0, err
        }
    }
    // 查 top-1 最相似
    neighbors, err := e.vectorIndex.Search(ctx, candidate.Embedding, 1)
    if err != nil { return nil, 0, err }
    if len(neighbors) == 0 { return nil, 0, nil }
    top := neighbors[0]
    if top.Score >= 0.92 {  // 阈值：余弦相似度 >= 0.92
        return top.Memory, top.Score, nil
    }
    return nil, top.Score, nil
}
```

### 10.6 冻结快照与 Prompt Cache 协调

```
会话启动时:
  1. 从 DB 加载当前有效记忆
  2. 格式化为 System Prompt 片段
  3. 生成不可变快照 (_snapshot)
  4. 在 System Prompt 中注入快照内容
  5. 在快照后放置 cachePoint

会话期间:
  - 新记忆写入 DB，但不修改当前会话的 System Prompt
  - 快照在会话期间保持不变

下次会话:
  - 重新加载最新记忆，生成新快照
```

System Prompt 结构分层：

```
┌───────────────────────────────┐
│ 基础 System Prompt             │  ← 缓存
├───────────────────────────────┤
│ 冻结记忆快照 (frozen snapshot) │  ← 缓存
├───────────────────────────────┤
│ cachePoint 边界                │
├───────────────────────────────┤
│ 动态对话内容                    │  ← 不缓存
└───────────────────────────────┘
```

完整 `MemorySnapshot` 模型见 [第 8.3 节](#83-记忆系统模型)，唯一变动是新增 `Version` 字段用于 [第 7.2 节](#72-资源版本化与-etag) 描述的 Resource URI 版本化。

**Snapshot 作为 MCP Resource**：

> v0.2 中，冻结快照可以通过 MCP Resource URI 访问，以实现 Prompt Cache 协调。

```
hub://workspace/{ws_id}/memory/snapshot?v=2026-06-07T03:15:42Z
```

不同会话拉取同一时间点的快照，拼接 System Prompt 的前缓部分保持一致 → Prompt Cache 命中率提升。

代价：本会话内新写入的记忆对当前会话不可见，需等下次会话才生效。

### 10.7 跨模型的容量上限：字符级约束

```go
type MemoryCapacityConfig struct {
    GlobalCharLimit    int `json:"global_char_limit"`
    PerMemoryCharLimit int `json:"per_memory_char_limit"`
    SkillCharLimit     int `json:"skill_char_limit"`
}

const (
    DefaultGlobalCharLimit    = 2200
    DefaultPerMemoryCharLimit = 500
    DefaultSkillCharLimit     = 1375
)
```

**v0.2 多租户配置**：每个 Workspace 可独立设置 `MemoryCapacityConfig`，覆盖到 `workspaces.memory_capacity_json` 字段（v0.2 migration 补齐）。

写入时超限处理：超限 → 拒写 → 返回当前用量提示 → 引导用户/Agent 调用 replace 或 remove

### 10.8 Skill 三状态机

**原则一：写入与治理分离** — Agent 即时写入 Skill，质量判断不在写入时做，由独立的后台机制 (Curator) 定期完成。

**原则二：Provenance 作为一等公民** — Curator 只治理 Agent 自产的 Skill，不碰人类策展的。

**原则三：治理动作可逆 + 可审查** — 从不自动删除，只 archive（可恢复），每轮治理留审计记录。

#### 10.8.1 状态转换图

```
  ┌──────────┐  7天未使用   ┌──────────┐  再7天未使用  ┌──────────┐
  │  active  │ ──────────→ │  stale   │ ────────────→ │ archived │
  └──────────┘             └──────────┘                └──────────┘
       ↑                        │                           │
       │     被再次使用          │                           │ 恢复
       └────────────────────────┘                           │
       ↑                                                    │
       └────────────────────────────────────────────────────┘
```

```go
type SkillCurator struct {
    idlePeriod time.Duration
    auditLog   AuditLogger
}

type SkillCurationLog struct {
    ID        string    `gorm:"primarykey;type:uuid" json:"id"`
    SkillID   string    `gorm:"type:uuid;not null;index:idx_curation_skill" json:"skill_id"`
    OldState  string    `gorm:"type:varchar(16);not null" json:"old_state"`
    NewState  string    `gorm:"type:varchar(16);not null" json:"new_state"`
    Reason    string    `gorm:"type:text;not null" json:"reason"`
    CuratedAt time.Time `json:"curated_at"`
}
```

**v0.2 改造**：Curator 同样在多租户上下文中运行，需要 filter `workspace_id`。

### 10.9 记忆格式转换规则（Local Bridge 兼容保留）

> v0.2 中记忆主要通过 MCP Tool 读取，不再以同步文件为主。但 **Local Bridge 插件** (P2) 仍会使用这些转换规则将记忆写入 Agent 本地配置文件。

| 转换标识 | 源格式 | 目标格式 | 说明 |
|----------|--------|----------|------|
| `memory_to_mdc` | Markdown | Cursor .mdc 格式 | 包装为规则文件 + Frontmatter |
| `memory_to_claude_md` | Markdown | CLAUDE.md 段落 | 追加 `## Memory` section |
| `memory_to_md_file` | Markdown | 独立 .md 文件 | 直接写入 MEMORY.md |
| `memory_to_windsurf` | Markdown | .windsurfrules 段落 | 追加 `## Memory` section |
| `skill_to_mdc` | Skill Markdown | Cursor .mdc 规则 | Skill 转为可执行规则 |
| `skill_to_claude_md` | Skill Markdown | CLAUDE.md 段落 | 追加 `## Skills` section |

Skill 转换与 Memory 转换的关系：

- `memory_to_*` 规则适用于 Type 为 `semantic` / `user_preference` / `episodic` 的记忆
- `skill_to_*` 规则专用于 `Type = skill` 的记忆
- 两者独立，因为 Skill 需要特殊处理：
  a. Skill 的 Frontmatter 需要附加 `alwaysApply: true` 或 `globs` 字段
  b. Skill 在 CLAUDE.md 中归入 `## Skills` 段落而非 `## Memory` 段落
  c. Skill 写入时需检查 State，仅 `active` 和 `stale` 状态的 Skill 参与同步，`archived` 的跳过

### 10.10 向量召回引擎（v0.2 新增）

> 本节是 v0.2 的核心升级。从纯关键词检索升级为 **BM25 + 向量混合检索**。

#### 10.10.1 检索器接口

```go
type MemoryRetriever interface {
    Search(ctx context.Context, query *SearchQuery) ([]*SearchResult, error)
}

type SearchQuery struct {
    WorkspaceID     string
    ProjectID       *string
    UserID          string
    Text            string
    TopK            int       // 默认 20
    MinScore        float64   // 默认 0.5
    Types           []string  // 可限定 Type 过滤
    Categories      []string
    IncludeArchived bool      // 默认 false
}

type SearchResult struct {
    Memory *Memory
    Score  float64
    Source string  // "vector" | "bm25" | "hybrid"
    Rank   int
}
```

#### 10.10.2 实现：混合检索（BM25 + 向量）

**步骤 1：Embedding 查询**：

```go
func (r *HybridRetriever) embedQuery(ctx context.Context, q string) ([]float32, error) {
    return r.embedder.Embed(ctx, q)
}
```

**步骤 2：并行召回**：

```go
type rankedItem struct {
    Memory   *Memory
    VecRank  int     // 0 = 未召回
    VecScore float64
    BMRank   int
    BMScore  float64
}

func (r *HybridRetriever) parallelRecall(ctx context.Context, q *SearchQuery) ([]*rankedItem, error) {
    var wg sync.WaitGroup
    var vecResults, bmResults []*SearchResult
    var vecErr, bmErr error

    wg.Add(2)
    go func() {
        defer wg.Done()
        vecResults, vecErr = r.vectorRecall(ctx, q)
    }()
    go func() {
        defer wg.Done()
        bmResults, bmErr = r.bm25Recall(ctx, q)
    }()
    wg.Wait()
    // ... merge logic ...
    return nil, nil
}
```

**步骤 3：RRF 融合**：

```go
func (r *HybridRetriever) fuseRRF(vec, bm []*SearchResult, k int) []*SearchResult {
    m := map[string]*rankedItem{}
    for rank, res := range vec {
        key := res.Memory.ID
        if _, ok := m[key]; !ok { m[key] = &rankedItem{Memory: res.Memory} }
        m[key].VecRank = rank + 1
        m[key].VecScore = res.Score
    }
    for rank, res := range bm {
        key := res.Memory.ID
        if _, ok := m[key]; !ok { m[key] = &rankedItem{Memory: res.Memory} }
        m[key].BMRank = rank + 1
        m[key].BMScore = res.Score
    }
    out := make([]*SearchResult, 0, len(m))
    const rrfK = 60
    for _, it := range m {
        rrfScore := 0.0
        if it.VecRank > 0 { rrfScore += 1.0 / (rrfK + float64(it.VecRank)) }
        if it.BMRank > 0  { rrfScore += 1.0 / (rrfK + float64(it.BMRank)) }
        out = append(out, &SearchResult{
            Memory: it.Memory,
            Score:  rrfScore,
            Source: source(it),
        })
    }
    sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
    if len(out) > k { out = out[:k] }
    return out
}
```

#### 10.10.3 MCP Tool 接入

上述检索能力通过 [第 6.3 节 记忆类 Tools](#63-记忆类-tools-p0) 的 `hub.search_memory` / `hub.get_relevant_memory` 暴露给 Agent。

#### 10.10.4 降级策略

| 场景 | 降级路径 |
|------|----------|
| 向量服务不可用 | 退化为纯 BM25 检索 |
| pgvector 索引未创建 | 退化为全表向量扫描 + LIMIT |
| Embedding 为 NULL 的记忆 | 跳过向量召回，仅参与 BM25 |
| 召回结果数 < TopK | 补足到 TopK，不足不报错 |

### 10.11 记忆压缩（Compaction）

> v0.2 保留 v0.1 的压缩逻辑不变。压缩仅作用于声明性记忆 (Declarative)，Skill 不压缩。

```
Compaction 触发条件:
  - 手动触发: 调用 MCP Tool `hub.compact_memories`
  - 自动触发: 当有效记忆总字符数超过 GlobalCharLimit 的 90% 时

压缩策略:
  1. 收集同类记忆（相同 Type + Category）
  2. 按语义相似度聚类（使用嵌入向量余弦相似度，阈值 0.85）
  3. 同簇内多条记忆合并为一条:
     a. 保留 Importance 最高的记忆作为基础
     b. 将同簇其他记忆内容作为补充信息追加
     c. 合并后 CharCount 不超过 PerMemoryCharLimit
     d. 被合并的记忆标记为 SupersededBy = 新记忆 ID
  4. 更新 MemoryValidity 记录
  5. 记录 SyncRecord

不参与压缩的记忆:
  - pinned: true 的记忆豁免
  - Provenance = human_curated 的记忆豁免
```

### 10.12 记忆映射（Local Bridge 兼容保留）

完整 `MemoryMapping` 模型见 [第 8.3 节](#83-记忆系统模型)。

| Agent | 记忆文件路径 | 格式 | 说明 |
|-------|-------------|------|------|
| Cursor | `.cursor/rules/memory.mdc` | Markdown + Frontmatter | 规则文件承载记忆 |
| Claude Code | `CLAUDE.md` → `## Memory` 段落 | Markdown | 追加到指令文件 |
| OpenCode | `.opencode/memory.md` | Markdown | 独立记忆文件 |
| Windsurf | `.windsurfrules` → `## Memory` 段落 | Markdown | 追加到规则文件 |
| 通用 | `MEMORY.md` | Markdown | 项目根目录，兼容 OpenClaw/Hermes |

> **v0.2 设计决策**：P0 阶段所有 `MemoryMapping` API 保留为 **仅供 Local Bridge 插件使用**，主 SaaS 流程不依赖文件同步。详见 [附录 A Local Bridge Adapter 设计](#附录-a-local-bridge-adapter-设计) 与 [附录 B Local Bridge 同步引擎](#附录-b-local-bridge-同步引擎)。

### 10.13 前端 Memory 页面（待 Stage 6 改造）

> 页面主体保留 v0.1 设计。Stage 6 会在多租户上下文上重写为 SaaS Console 页面，包括 Workspace Switcher + Org 导航。

#### Dashboard 中的记忆区域

- 记忆容量使用情况 (已用字符 / 总上限)
- 按类型统计 (Semantic / Preference / Episodic / Skill)
- 最近写入的记忆列表
- 冻结快照状态指示

#### Memory Explorer

- 左侧：记忆列表，按类型分组
  - 筛选：类型 / Provenance / Pinned / 有效状态
  - 搜索：关键词 + 语义搜索
- 右侧：记忆详情
  - 内容编辑器
  - 元信息（类型、来源、重要性评分、访问次数）
  - 有效时间窗口设置
  - 映射到哪些 Agent（仅 Local Bridge 启用时显示）
  - 访问历史

#### Skill Manager

- Skill 卡片列表
  - 状态徽标 (active / stale / archived)
  - 来源标记 (human / agent)
  - 使用频次
  - 最近使用时间
- Skill 详情
  - 内容预览与编辑
  - 治理审计记录
  - 归档 / 恢复操作

#### Memory Timeline

- 时间线视图，展示记忆的演化
  - 新增、更新、失效、取代事件
  - 支持按时间范围筛选
  - 点击查看任意时间点的记忆快照

#### Memory Settings

- 写入策略选择 (LLM 判官 / 公式打分 / 托管策略)
- 字符上限配置
- 冻结快照开关
- Skill 治理周期配置
- 嵌入模型选择与 API Key 配置
- 记忆映射管理（仅 Local Bridge 启用时可见）

---

## 15. 前端设计（SaaS Console）

> Open Agent Hub 的 **SaaS Console** 前端，面向 Workspace 管理员与开发者。顶部导航以 4 个 Hub 划分业务领域，后端是单进程多 Service（不是微服务）。

### 15.1 技术栈

> 参考 opentoken-vip 前端已验证的技术栈，保持团队技术一致性。

| 技术 | 选型 | 说明 |
|------|------|------|
| 构建工具 | Vite | 快速开发与构建，团队已使用 |
| 框架 | React 19 | 团队已使用 |
| 语言 | TypeScript | 类型安全 |
| UI 组件 | Ant Design 5.x + 中文 locale | 团队已使用 |
| 状态管理 | React hooks + React Query | 服务端状态与缓存 |
| 路由 | React Router v6 (createBrowserRouter) | 团队已使用，data router 模式 |
| API 客户端 | 原生 fetch + `request<T>()` 封装 | 与 opentoken-vip 一致 |
| 编辑器 | Monaco Editor | JSON/Markdown 配置 / 记忆内容编辑 |
| 图标 | react-icons + @ant-design/icons | 团队已使用 |
| Markdown | react-markdown + react-syntax-highlighter | 团队已使用 |
| 表格 | AG Grid Community | 大表格（Audit / Tool Invocation / Usage） |
| 图表 | ECharts | Usage Dashboard |
| 样式方案 | CSS Modules + Ant Design props | 与 opentoken-vip 一致 |
| 嵌入 | go:embed 静态文件 | 单二进制分发 |

### 15.2 顶部 4 Hub 导航架构

> **4 Hub 是前端产品分类，不是后端微服务边界**。后端在 P0 阶段是单进程多 Service（详见 [第 16 章 后端项目结构](#16-go-后端项目结构待-stage-6-改造)）。

```
┌──────────────────────────────────────────────────────────────────┐
│ [Org Switcher] [Workspace Switcher]                              │
├──────────────────────────────────────────────────────────────────┤
│   Agent Hub    Context Hub    Memory Hub    Tool Hub              │
└──────────────────────────────────────────────────────────────────┘
```

**各 Hub 覆盖页面**：

| Hub | 覆盖页面 |
|-----|----------|
| **Agent Hub** | Organization / Workspace / Members / MCP Tokens / Agent Clients / Local Bridge / Audit Logs |
| **Context Hub** | Global Rules / Project Rules / Workspace Policy / Output Preferences / Snapshot Versions |
| **Memory Hub** | Memory Explorer / Memory Timeline / Skill Manager / Memory Settings / Vector Index Status |
| **Tool Hub** | Connected MCP Servers / Tool Policies / Tool Catalog / Usage Dashboard / Quota / Billing |

### 15.3 Agent Hub 页面

#### 15.3.1 Organization Settings

- 组织基础信息（名称、Slug、Logo）
- Plan 与计费状态
- 账单信息 + Stripe Customer Portal
- 迁组织/合并组织（Future）

#### 15.3.2 Workspace Management

- Workspace 列表（创建、切换、重命名、删除）
- Workspace 详情：成员 / 限额 / Quota 使用 / Plan
- Workspace 级别：默认规则 / 默认 Skill 模板

#### 15.3.3 Members

- 成员列表（邮箱、角色、加入时间、最后活跃）
- 邀请成员（Email / Link）
- 角色调整：Owner / Admin / Member / Viewer
- 移除成员

#### 15.3.4 MCP Tokens

- Workspace Token 列表
  - 名称、创建者、创建时间、最后使用、过期时间
  - 显示前缀 + `****` 脱敏
  - 生成时一次性返回明文，后续只显示 hash
- 生成 / 轮换 / 撤销
- Scope 选择（read / write / admin）

#### 15.3.5 Agent Clients

- 连接的 Agent Client 列表
  - ClientType / ClientName / ClientVersion / Project / LastSeen
- 点击查看详情
  - 连接历史
  - 调用统计
  - 项目推断结果
- 手动断开 / 重连

#### 15.3.6 Audit Logs

- 审计日志列表（AG Grid）
  - Actor / Action / Target / 时间 / IP
  - 高级筛选：时间范围 / Actor / Action 类型
- 详情弹窗：完整 Payload JSON

#### 15.3.7 Local Bridge (P2)

- Bridge 状态（在线 / 离线 / 同步中 / 错误）
- Bridge 配置：Token / 同步路径 / 同步方向（双向 / 仅推 / 仅拉）
- 历史同步记录（类同步引擎设计）
- 安装包下载（macOS / Windows / Linux）

### 15.4 Context Hub 页面

#### 15.4.1 Global Rules

- 规则列表（按 Type / Scope / Pinned 筛选）
- 规则编辑器
  - Monaco Editor（JSON / Markdown）
  - 名称 / 描述 / 类型 / Scope / Tags
  - 版本历史
- 批量操作：启用 / 停用 / 导出

#### 15.4.2 Project Rules

- 项目选择器（仅显示当前 Workspace 下的项目）
- 项目内规则列表（同上 Global Rules）

#### 15.4.3 Workspace Policy

- Workspace 级策略配置
  - 默认 Tool Policy
  - Quota 限额（可超出 Plan 限额则计费）
  - 允许的 ClientType 白名单
  - 二次确认策略

#### 15.4.4 Output Preferences

- 用户级偏好列表
  - language / code_style / detail_level / use_typescript / 等
- 可视化设置 / 导入 / 导出

#### 15.4.5 Snapshot Versions

- 冻结快照历史
- 任意时间点可重建 Snapshot
- 预览 Snapshot 内容

### 15.5 Memory Hub 页面

#### 15.5.1 Memory Explorer

- 左侧：记忆列表，按类型分组
  - 筛选：Type / Category / Provenance / Pinned / 有效状态
  - 搜索：关键词 + 语义搜索（调用 `hub.search_memory`）
  - 批量操作：Pinned / Archive / Export
- 右侧：记忆详情
  - 内容编辑器
  - 元信息（Type / Provenance / Importance / AccessCount / LastAccess）
  - 有效时间窗口（ValidFrom / ValidUntil）
  - 映射到哪些 Agent（仅 Local Bridge 启用时显示）
  - 访问历史（MemoryAccessLog）
  - Embedding 状态（是否已嵌入 / 模型 / 嵌入时间）

#### 15.5.2 Memory Timeline

- 时间线视图，展示记忆的演化
  - 新增 / 更新 / 失效 / 取代 事件
  - 支持按时间范围筛选
  - 点击查看任意时间点的记忆快照

#### 15.5.3 Skill Manager

- Skill 卡片列表
  - 状态徽标（active / stale / archived）
  - 来源标记（human / agent）
  - 使用频次 / 最近使用时间
- Skill 详情
  - 内容预览与编辑
  - 治理审计记录
  - 归档 / 恢复操作

#### 15.5.4 Memory Settings

- 写入策略选择（LLM 判官 / 公式打分 / 托管策略）
- 字符上限配置（Global / PerMemory / Skill）
- 冻结快照开关
- Skill 治理周期配置
- 嵌入模型选择与 API Key 配置
- 记忆映射管理（仅 Local Bridge 启用时可见）

#### 15.5.5 Vector Index Status

- pgvector 索引状态
  - 索引大小 / 召回质量
  - 重建索引操作
- Embedding 任务队列
  - 待嵌入 / 已嵌入 / 错误
  - 重试失败的 Embedding 任务

### 15.6 Tool Hub 页面

#### 15.6.1 Connected MCP Servers

- 上游 MCP Server 列表
  - Name / Endpoint / Transport / AuthType / Status
- 详情页
  - Tools 列表（从 ToolsJSON 解析）
  - 默认 Tool Policy 模板
  - 健康检查状态
  - 凭据管理（OAuth / API Key 轮换）
- 注册新 Server（Endpoint + Auth）

#### 15.6.2 Tool Policies

- 工具策略列表（按 Server 分组）
  - ToolName / Allowed / RequiresConfirmation / MaxCallsPerDay / RiskLevel
- 批量编辑策略
- 风险等级可视化（Low / Medium / High / Critical）

#### 15.6.3 Tool Catalog

- 所有可用工具的只读视图（含上游 Server 提供的）
- 按命名空间前缀分组
- 工具详情：Schema / 风险 / 使用频次

#### 15.6.4 Usage Dashboard

- 指标卡（KPI）
  - 今日 / 本周 / 本月 Tool 调用数
  - 活跃 MCP Session 数
  - 记忆写入 / 读取次数
  - Vector 查询次数
- 趋势图（ECharts）
  - 按 Tool / 按 Agent / 按 Workspace
- 限额使用率（Quota）

#### 15.6.5 Quota & Billing

- Plan 当前用量
- 限额警告（接近上限时高亮）
- 升级 Plan 入口（Stripe Checkout）
- 账单历史 / 发票下载

### 15.7 前端目录结构

```
web/
├── index.html
├── package.json
├── vite.config.ts                # proxy: /api → localhost:8084, /mcp → localhost:8085
├── tsconfig.json
├── tsconfig.node.json
└── src/
    ├── main.tsx                  # ReactDOM.createRoot 入口
    ├── App.tsx                   # ConfigProvider + RouterProvider
    ├── router/
    │   ├── index.tsx             # createBrowserRouter
    │   └── guards.tsx            # AuthGuard / WorkspaceGuard / RoleGuard
    ├── api/
    │   ├── client.ts             # request<T>() + auth header injection
    │   ├── org.ts
    │   ├── workspace.ts
    │   ├── members.ts
    │   ├── mcp-tokens.ts
    │   ├── rules.ts
    │   ├── memories.ts
    │   ├── skills.ts
    │   ├── connected-servers.ts
    │   ├── tool-policies.ts
    │   ├── tool-invocation-logs.ts
    │   ├── usage.ts
    │   ├── audit-logs.ts
    │   └── agent-clients.ts
    ├── hooks/
    │   ├── useAuth.ts            # 当前 user / org / workspace
    │   ├── useWorkspace.ts       # Workspace Switcher
    │   └── usePermissions.ts     # RBAC 检查
    ├── components/
    │   ├── Layout/
    │   │   ├── TopNav.tsx        # 4 Hub + Org/Workspace Switcher
    │   │   └── SideNav.tsx
    │   ├── MonacoEditor.tsx
    │   ├── CapacityBar.tsx
    │   ├── DiffViewer.tsx
    │   ├── QuotaGauge.tsx
    │   ├── RiskBadge.tsx
    │   └── ...
    ├── pages/
    │   ├── agent-hub/
    │   │   ├── OrganizationSettings.tsx
    │   │   ├── WorkspaceManagement.tsx
    │   │   ├── Members.tsx
    │   │   ├── MCPTokens.tsx
    │   │   ├── AgentClients.tsx
    │   │   ├── AuditLogs.tsx
    │   │   └── LocalBridge.tsx
    │   ├── context-hub/
    │   │   ├── GlobalRules.tsx
    │   │   ├── ProjectRules.tsx
    │   │   ├── WorkspacePolicy.tsx
    │   │   ├── OutputPreferences.tsx
    │   │   └── SnapshotVersions.tsx
    │   ├── memory-hub/
    │   │   ├── MemoryExplorer.tsx
    │   │   ├── MemoryTimeline.tsx
    │   │   ├── SkillManager.tsx
    │   │   ├── MemorySettings.tsx
    │   │   └── VectorIndexStatus.tsx
    │   ├── tool-hub/
    │   │   ├── ConnectedMCPServers.tsx
    │   │   ├── ToolPolicies.tsx
    │   │   ├── ToolCatalog.tsx
    │   │   ├── UsageDashboard.tsx
    │   │   └── QuotaBilling.tsx
    │   └── shared/
    │       ├── Login.tsx
    │       ├── NotFound.tsx
    │       └── ErrorBoundary.tsx
    └── stores/                    # 全局 store（轻量）
        └── auth.ts                # Zustand
```

### 15.8 前端路由表

```typescript
// router/index.tsx
const router = createBrowserRouter([
  {
    path: "/",
    element: <Layout />,
    children: [
      { index: true, element: <Navigate to="/agent-hub" replace /> },

      // Agent Hub
      { path: "agent-hub", element: <AgentHub />, children: [
        { index: true, element: <WorkspaceManagement /> },
        { path: "organization", element: <OrganizationSettings /> },
        { path: "members", element: <Members /> },
        { path: "mcp-tokens", element: <MCPTokens /> },
        { path: "agent-clients", element: <AgentClients /> },
        { path: "audit-logs", element: <AuditLogs /> },
        { path: "local-bridge", element: <LocalBridge /> },
      ]},

      // Context Hub
      { path: "context-hub", element: <ContextHub />, children: [
        { index: true, element: <GlobalRules /> },
        { path: "global-rules", element: <GlobalRules /> },
        { path: "project-rules", element: <ProjectRules /> },
        { path: "workspace-policy", element: <WorkspacePolicy /> },
        { path: "output-preferences", element: <OutputPreferences /> },
        { path: "snapshots", element: <SnapshotVersions /> },
      ]},

      // Memory Hub
      { path: "memory-hub", element: <MemoryHub />, children: [
        { index: true, element: <MemoryExplorer /> },
        { path: "explorer", element: <MemoryExplorer /> },
        { path: "timeline", element: <MemoryTimeline /> },
        { path: "skills", element: <SkillManager /> },
        { path: "settings", element: <MemorySettings /> },
        { path: "vector-status", element: <VectorIndexStatus /> },
      ]},

      // Tool Hub
      { path: "tool-hub", element: <ToolHub />, children: [
        { index: true, element: <ConnectedMCPServers /> },
        { path: "connected-servers", element: <ConnectedMCPServers /> },
        { path: "policies", element: <ToolPolicies /> },
        { path: "catalog", element: <ToolCatalog /> },
        { path: "usage", element: <UsageDashboard /> },
        { path: "quota", element: <QuotaBilling /> },
      ]},
    ],
  },
]);
```

---

## 11. MCP Gateway 服务

> MCP Gateway 是 Open Agent Hub **数据面** 的入口。所有 Agent 客户端通过 MCP 协议连接 Gateway，由 Gateway 统一处理认证、路由、上下文注入、限流、错误映射。

### 11.1 Gateway 在系统中的位置与边界

**位置**：

```
[Agent Client]
   │  MCP over Streamable HTTP
   ▼
┌─────────────────────────┐
│   MCP Gateway (8085)    │ ← 业务路由 + MCP 协议转换
└──────┬───────────┬──────┘
       │           │
       ▼           ▼
   Context     Memory     Tool
   Service     Service    Routing
       │           │           │
       └───────────┴───────────┘
                   │
                   ▼
            PostgreSQL + Redis
```

**核心职责**：

1. **MCP 协议解析**：处理 `initialize` / `tools/list` / `tools/call` / `resources/list` / `resources/read` / `prompts/list` / `prompts/get` 六个核心 method
2. **认证与授权**：从 token 解析 workspace_id，注入多租户上下文
3. **路由与转发**：根据 tool 名称路由到 Context Service / Memory Service / Tool Routing
4. **限流与熔断**：按 workspace + tool 维度限流，上游熔断保护
5. **错误规范映射**：业务错误 → MCP 标准错误码（详见 [第 5.6 节](#56-mcp-错误规范映射)）
6. **会话管理**：维护 MCPSession 生命周期（详见 [第 5.5 节](#55-会话生命周期)）

**不在 Gateway 中的职责**：

- 业务逻辑执行（由各 Service 实现）
- 持久化（由 Service 操作 DB）
- 认证 token 本身签发（由 SaaS 控制面签发）

### 11.2 五大角色实现

#### 11.2.1 配置分发（Configuration Distribution）

**职责**：Agent 启动时拉取 workspace 的全局规则、项目规则、输出偏好。

实现路径：

```
Agent 调 hub.get_global_rules
  → Gateway 路由到 Context Service.GetGlobalRules(workspace_id)
  → 从 Rules 表查询（带 ETag 比较）
  → 返回 { content, etag, version }
```

数据走 MCP Resource（适合高频读 + 缓存），详见 [第 7.1 节](#71-resource-uri-设计)。

#### 11.2.2 记忆检索（Memory Retrieval）

**职责**：Agent 在对话中检索相关历史记忆。

```
Agent 调 hub.search_memory({ text, top_k })
  → Gateway 路由到 Memory Service.Search(workspace_id, query)
  → 并行执行: 向量检索 + BM25 检索
  → RRF 融合 → 返回 top_k 条记忆
```

详见 [第 10.10 节](#1010-向量召回引擎新增)。

#### 11.2.3 记忆写入（Memory Persistence）

**职责**：Agent 提议记忆 → 写入纪律引擎过滤 → 成功则持久化。

```
Agent 调 hub.propose_memory / hub.save_memory
  → Memory Service.Persist(workspace_id, candidate)
  → 写入纪律引擎评估（LLM 判官 / 公式打分 / 托管策略）
  → 语义去重检测
  → 成功后：生成 Embedding → 写入 DB → 返回新记忆
  → 失败：返回决策与原因
```

详见 [第 10.4 节](#104-写入纪律)。

#### 11.2.4 Skill 分发（Skill Distribution）

**职责**：Agent 查询可用的 Skill 列表与详情。

```
Agent 调 hub.list_skills / hub.get_skill
  → Gateway 路由到 Memory Service.ListSkills(workspace_id, filter)
  → 过滤 state='active' 或 'stale'，排除 archived
  → 返回 Skill 列表与详情
```

#### 11.2.5 外部 MCP 聚合（External MCP Aggregation）

**职责**：Agent 通过 Hub 调用上游 MCP Server 提供的工具（GitHub / Notion / Figma / DB 等）。

```
Agent 调 hub.invoke_connected_tool({ tool_name, input })
  → Gateway 路由到 Tool Routing.Invoke(workspace_id, tool_name, input)
  → 检查 ToolPolicy（需走二次确认则返回 requires_confirmation=true）
  → 上游 MCP Server 调用
  → 记录 ToolInvocationLog
  → 返回结果
```

详见 [第 13 章 Tool Routing](#13-tool-routing-与外部-mcp-聚合)。

### 11.3 多租户上下文注入

所有 Tool 调用都需在多租户上下文中执行。中间件设计：

```go
// middleware/tenant.go
func TenantContextMiddleware(secret []byte) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. 解析 Authorization Header
        authHeader := c.GetHeader("Authorization")
        if !strings.HasPrefix(authHeader, "Bearer ") {
            c.AbortWithStatusJSON(401, mcpErr(MCPErrorUnauthorized, "missing bearer token"))
            return
        }
        rawToken := strings.TrimPrefix(authHeader, "Bearer ")

        // 2. 验证 token 并提取 claims
        claims, err := verifyMCPToken(rawToken, secret)
        if err != nil {
            c.AbortWithStatusJSON(401, mcpErr(MCPErrorUnauthorized, "invalid token"))
            return
        }

        // 3. 注入 workspace_id 到 context
        ctx := c.Request.Context()
        ctx = context.WithValue(ctx, "workspace_id", claims.WorkspaceID)
        ctx = context.WithValue(ctx, "user_id", claims.UserID)
        ctx = context.WithValue(ctx, "scopes", claims.Scopes)
        c.Request = c.Request.WithContext(ctx)

        // 4. 设置 PostgreSQL session 变量使 RLS 生效
        db := global.GLB_DB.WithContext(ctx)
        db.Exec(fmt.Sprintf("SET app.current_workspace_id = '%s'", claims.WorkspaceID))

        // 5. 维护 MCPSession
        sessionID := ensureSession(ctx, db, claims, c)
        ctx = context.WithValue(ctx, "mcp_session_id", sessionID)
        c.Request = c.Request.WithContext(ctx)

        c.Next()
    }
}
```

**Token 类型区分**：

| Token 类型 | 适用场景 | 生命周期 | 续期 |
|-----------|---------|---------|------|
| OAuth Access Token | Web / SaaS Console 登录 | 1 小时 | Refresh Token |
| PAT (Personal Access Token) | CLI / Local Bridge | 不限（可设过期） | 不自动续期 |
| Workspace MCP Token | Agent 连 Hub | 长期 + 可轮换 | Admin 手动 |
| Short-lived Session Token | SSE 连接 | 5 分钟 | 心跳续期 |

### 11.4 限流与熔断

#### 11.4.1 限流维度

| 维度 | 算法 | 键 | 默认限额 |
|------|------|-----|----------|
| Workspace Tool 总调用 | 令牌桶 | `ratelimit:ws:{ws_id}:tools` | 5000 次/天 |
| 单 Tool 调用 | 令牌桶 | `ratelimit:ws:{ws_id}:tool:{name}` | 1000 次/天 |
| Agent Client | 令牌桶 | `ratelimit:client:{client_id}` | 500 次/小时 |
| 向量检索 | 滑动窗口 | `ratelimit:ws:{ws_id}:vector` | 1000 次/小时 |
| 记忆写入 | 令牌桶 | `ratelimit:ws:{ws_id}:write` | 500 次/小时 |

#### 11.4.2 限流实现（Redis Lua 令牌桶）

```lua
-- KEYS[1]: 限流键
-- ARGV[1]: 容量, ARGV[2]: 速率（token/s）, ARGV[3]: 当前时间（毫秒）
local bucket = redis.call('HMGET', KEYS[1], 'tokens', 'last_refill')
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local tokens = tonumber(bucket[1]) or capacity
local last_refill = tonumber(bucket[2]) or now

-- 补充 token
local elapsed = math.max(0, now - last_refill) / 1000.0
tokens = math.min(capacity, tokens + elapsed * rate)

if tokens >= 1 then
    tokens = tokens - 1
    redis.call('HMSET', KEYS[1], 'tokens', tokens, 'last_refill', now)
    redis.call('EXPIRE', KEYS[1], 3600)
    return { 1, tokens }  -- allowed
else
    return { 0, 0 }  -- denied
end
```

**Go 侧封装**：

```go
func AllowRateLimit(ctx context.Context, key string, capacity, rate int) (bool, int, error) {
    res, err := global.GLB_REDIS.EvalSha(ctx, ratelimitScript, []string{key}, capacity, rate, time.Now().UnixMilli()).Result()
    if err != nil {
        return false, 0, err
    }
    arr := res.([]interface{})
    allowed := arr[0].(int64) == 1
    remaining := int(arr[1].(int64))
    return allowed, remaining, nil
}
```

#### 11.4.3 熔断器

使用 `github.com/sony/gobreaker` 保护上游 MCP Server 调用：

```go
var upstreamBreaker = gobreaker.NewCircuitBreaker(gobreaker.Settings{
    Name:        "upstream-mcp",
    MaxRequests: 3,
    Interval:    60 * time.Second,
    Timeout:     30 * time.Second,
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        return counts.ConsecutiveFailures > 5
    },
})

func InvokeUpstream(ctx context.Context, server *ConnectedMCPServer, method string, params json.RawMessage) (*jsonrpc.Response, error) {
    result, err := upstreamBreaker.Execute(func() (interface{}, error) {
        return doUpstreamCall(ctx, server, method, params)
    })
    if err != nil {
        return nil, err
    }
    return result.(*jsonrpc.Response), nil
}
```

#### 11.4.4 限流响应

超限响应标准（符合 MCP 错误规范）：

```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "error": {
        "code": -32003,
        "message": "rate limit exceeded",
        "data": {
            "retry_after_seconds": 60,
            "limit": 5000,
            "current": 5000
        }
    }
}
```

同时设置 HTTP Header：

```
HTTP/1.1 429 Too Many Requests
Retry-After: 60
X-RateLimit-Limit: 5000
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1717740000
```

### 11.5 MCP 错误规范映射

业务错误 → MCP 标准错误码的映射表：

| 业务错误 | HTTP 状态 | MCP 错误码 | 错误名 |
|----------|----------|------------|--------|
| 认证失败 | 401 | -32001 | unauthorized |
| 权限不足 | 403 | -32002 | forbidden |
| 限流超限 | 429 | -32003 | rate_limit_exceeded |
| 资源不存在 | 404 | -32004 | not_found |
| 资源冲突 | 409 | -32005 | conflict |
| 输入校验失败 | 400 | -32602 | invalid_params |
| 内部错误 | 500 | -32603 | internal_error |
| 上游 MCP 超时 | 504 | -32010 | upstream_timeout |
| 上游 MCP 错误 | 502 | -32011 | upstream_error |
| 工具未授权 | 403 | -32012 | tool_not_authorized |
| 工具需确认 | 409 | -32013 | tool_requires_confirmation |
| 记忆超过容量 | 413 | -32014 | memory_capacity_exceeded |

错误响应包装器：

```go
func mcpErr(code int, msg string, data ...interface{}) map[string]interface{} {
    errObj := map[string]interface{}{
        "code":    code,
        "message": msg,
    }
    if len(data) > 0 {
        errObj["data"] = data[0]
    }
    return map[string]interface{}{
        "jsonrpc": "2.0",
        "error":   errObj,
    }
}
```

### 11.6 Gateway 配置示例

```yaml
# config.yaml
mcp_gateway:
  enabled: true
  listen_addr: ":8085"
  transport: streamable_http  # 仅 Streamable HTTP

  auth:
    jwt_secret: ${MCP_JWT_SECRET}
    access_token_ttl: 1h
    session_token_ttl: 5m
    allow_pat: true           # 是否接受 PAT

  rate_limit:
    enabled: true
    redis_addr: redis:6379
    workspace_tool_daily: 5000
    agent_client_hourly: 500

  cors:
    allowed_origins:
      - "https://console.openagenthub.com"
    allowed_methods: ["GET", "POST", "OPTIONS"]

  sse:
    heartbeat_interval: 30s
    max_idle_time: 5m

  upstream:
    timeout: 30s
    max_retries: 2
    circuit_breaker:
      max_failures: 5
      timeout: 30s
```

---

## 12. Context Service

> Context Service 是 Open Agent Hub 的 **配置中心**，负责管理 Global Rules、Project Rules、Output Preferences 等 Agent 启动时加载的上下文数据。

### 12.1 服务边界

```
[Agent Client]
   │  MCP Tool: hub.get_global_rules / hub.get_project_rules / hub.get_output_preferences
   ▼
[Gateway] → [Context Service]
                │
                ├── Rules 表 (CRUD)
                ├── OutputPreferences 表
                ├── Redis 缓存
                └── MCP Resources (hub://workspace/.../rules/global)
```

### 12.2 Global / Project Rules CRUD

#### 12.2.1 内部接口

```go
// service/context/rule_service.go
type RuleService interface {
    // MCP Tool 接口
    GetGlobalRules(ctx context.Context, workspaceID string, etag string) (*RulesResult, error)
    GetProjectRules(ctx context.Context, workspaceID, projectID string, etag string) (*RulesResult, error)
    GetAgentRules(ctx context.Context, workspaceID, agentName string) ([]*Rule, error)
    GetWorkspacePolicy(ctx context.Context, workspaceID string) (*WorkspacePolicy, error)
    GetOutputPreferences(ctx context.Context, workspaceID, userID string) (map[string]string, error)

    // 管理面 REST API
    CreateRule(ctx context.Context, r *Rule) error
    UpdateRule(ctx context.Context, r *Rule) error
    DeleteRule(ctx context.Context, workspaceID, ruleID string) error
    ListRules(ctx context.Context, workspaceID string, filter *RuleFilter) ([]*Rule, error)
}

type RulesResult struct {
    Rules   []*Rule `json:"rules"`
    ETag    string  `json:"etag"`
    Version string  `json:"version"`
}
```

#### 12.2.2 ETag 机制

```go
func computeETag(rules []*Rule) string {
    // 按 Rule.ID 排序后取 max(UpdatedAt) 作为版本
    var maxUpdated time.Time
    for _, r := range rules {
        if r.UpdatedAt.After(maxUpdated) {
            maxUpdated = r.UpdatedAt
        }
    }
    return fmt.Sprintf(`W/"rules-%d-%s"`, len(rules), maxUpdated.UTC().Format(time.RFC3339Nano))
}
```

Agent 拉取时带 `If-None-Match` Header，无变化时返回 304 + 空体。

### 12.3 Output Preferences

用户级偏好（语言、风格、详细程度等）单独存储：

```go
type OutputPreference struct {
    ID          string
    UserID      string
    WorkspaceID string
    Key         string  // e.g. "language", "code_style", "detail_level"
    Value       string  // e.g. "zh-CN", "concise", "high"
}
```

通过 `hub.get_output_preferences` Tool 聚合返回：

```json
{
    "language": "zh-CN",
    "code_style": "concise",
    "detail_level": "medium",
    "use_typescript": "true",
    "prefer_functional": "false"
}
```

### 12.4 按 Tool 调用上下文聚合返回

为减少 Agent 多次调用 Context Tool，设计聚合 Tool `hub.get_agent_profile`：

```json
// 输入
{ "agent_name": "cursor", "project_id": "uuid" }

// 输出
{
    "agent_name": "cursor",
    "global_rules": [...],
    "project_rules": [...],
    "agent_rules": [...],
    "workspace_policy": {...},
    "output_preferences": {...},
    "etag": "W/.../",
    "version": "2026-06-07T03:15:42Z"
}
```

**优先级规则**（Agent 合并时）：

```
最终规则 = Global Rules
       + Project Rules (覆盖 Global 重名项)
       + Agent Rules (覆盖 Project 重名项)
       + Output Preferences (风格/格式偏好)
```

### 12.5 与 MCP Resources 的关联

按 [第 7.1 节](#71-resource-uri-设计) 设计，**稳定规则走 Resource，动态查询走 Tool**：

| 数据 | 走 Resource | 走 Tool | 理由 |
|------|------------|---------|------|
| Global Rules | ✔ `hub://workspace/{ws_id}/rules/global` | ✔ | 资源可被 Agent 预加载做 Cache |
| Project Rules | ✔ `hub://workspace/{ws_id}/project/{pid}/rules` | ✔ | 同上 |
| Agent Rules | ✗ | ✔ `hub.get_agent_rules` | 动态性高 |
| Output Preferences | ✗ | ✔ | 用户级，不适合 Resource 缓存 |
| Workspace Policy | ✔ `hub://workspace/{ws_id}/policy` | ✔ | 跨 Agent 共享 |

### 12.6 Context Service 配置

```yaml
context_service:
  cache:
    rules_ttl: 60s         # L1 进程内缓存
    redis_ttl: 300s        # L2 Redis 缓存
  aggregation:
    enabled: true
    max_concurrent_per_call: 5
  hot_reload:
    enabled: true          # 规则变更后主动失效缓存
```

---

## 13. Tool Routing 与外部 MCP 聚合

> Tool Routing 是 Open Agent Hub 的 **工具代理层**，负责将 Agent 的 tool 调用路由到上游 MCP Server（GitHub / Notion / DB / Figma / Slack 等），并执行 Tool Policy 校验、熔断保护、调用日志记录。

### 13.1 上游 MCP Server 注册表

上游 MCP Server 通过 `connected_mcp_servers` 表管理（详见 [第 8.5 节](#85-tool-路由与策略模型)）：

| 字段 | 说明 |
|------|------|
| Name | 命名空间前缀（如 `github`），最终工具名 = `{name}.{tool_name}` |
| Endpoint | 上游 MCP Server URL |
| Transport | `streamable_http` / `sse` / `stdio`（stdio 仅 Local Bridge） |
| AuthType | `none` / `bearer` / `oauth` / `api_key` |
| ToolsJSON | 注册时缓存的工具列表 |
| PolicyJSON | 默认 Tool Policy 模板 |

**注册流程**：

```go
func RegisterUpstreamServer(ctx context.Context, s *ConnectedMCPServer) error {
    // 1. 健康检查
    if err := healthCheck(ctx, s); err != nil {
        return fmt.Errorf("upstream health check failed: %w", err)
    }

    // 2. 拉取工具列表
    tools, err := fetchUpstreamTools(ctx, s)
    if err != nil {
        return fmt.Errorf("fetch tools failed: %w", err)
    }

    s.ToolsJSON = mustMarshal(tools)
    s.Status = "active"

    // 3. 持久化
    return global.GLB_DB.Create(s).Error
}
```

### 13.2 动态路由表

`tool_name` 命名规则：`{namespace}.{original_tool_name}`

例：上游 GitHub MCP Server 注册为 namespace=`github`，提供 `create_pull_request` 工具，则 Agent 看到 `github.create_pull_request`。

**路由表数据结构**：

```go
type RouteEntry struct {
    Namespace   string                 // github
    ToolName    string                 // create_pull_request
    FullName    string                 // github.create_pull_request
    Server      *ConnectedMCPServer
    LocalPolicy *ToolPolicy
}

type RouteTable struct {
    mu     sync.RWMutex
    routes map[string]*RouteEntry  // key: FullName
}

func (rt *RouteTable) Resolve(toolName string) (*RouteEntry, error) {
    rt.mu.RLock()
    defer rt.mu.RUnlock()
    entry, ok := rt.routes[toolName]
    if !ok {
        return nil, ErrToolNotFound
    }
    return entry, nil
}
```

**路由表刷新**：

- 启动时全量加载
- ConnectedMCPServer 变更时增量更新（订阅 DB 通知或定时轮询）
- 默认 60s TTL，过期重载

### 13.3 上游连接池

```go
type UpstreamClient struct {
    endpoint   string
    httpClient *http.Client
    transport  *http.Transport
    auth       AuthProvider
}

var clientPool = sync.Pool{
    New: func() interface{} {
        return &UpstreamClient{
            httpClient: &http.Client{
                Timeout: 30 * time.Second,
                Transport: &http.Transport{
                    MaxIdleConns:        100,
                    MaxIdleConnsPerHost: 10,
                    IdleConnTimeout:     90 * time.Second,
                },
            },
        }
    },
}
```

**连接复用**：`http.Transport` 自动管理 keep-alive；SSE 走长连接，HTTP 走短连接池。

### 13.4 并行扇出执行 + 熔断器

某些聚合 Tool（如 `hub.get_agent_profile`）需并行调用多个上游，使用 `errgroup` + 熔断：

```go
func (r *Router) ParallelInvoke(ctx context.Context, calls []*ToolCall) ([]*ToolResult, error) {
    g, ctx := errgroup.WithContext(ctx)
    results := make([]*ToolResult, len(calls))

    for i, call := range calls {
        i, call := i, call
        g.Go(func() error {
            res, err := r.InvokeWithBreaker(ctx, call)
            results[i] = res
            if err != nil {
                return err  // 一个失败不影响其他
            }
            return nil
        })
    }

    // 不阻塞：部分成功即可
    _ = g.Wait()
    return results, nil
}
```

熔断器配置见 [第 11.4.3 节](#1143-熔断器)。

### 13.5 Tool Policy 校验链路

每次 Tool 调用前必须过 Policy 校验：

```go
func (r *Router) checkPolicy(ctx context.Context, entry *RouteEntry, user *User) error {
    policy := entry.LocalPolicy
    if policy == nil {
        policy = getDefaultPolicy(entry.Server)  // 取 Server 默认策略
    }

    // 1. 是否允许
    if !policy.Allowed {
        return mcpErr(MCPErrorToolNotAuthorized, "tool blocked by policy", ...)
    }

    // 2. 配额检查
    if policy.MaxCallsPerDay > 0 {
        count := getTodayCallCount(entry.Server.WorkspaceID, entry.FullName)
        if count >= policy.MaxCallsPerDay {
            return mcpErr(MCPErrorRateLimited, "tool daily limit exceeded", ...)
        }
    }

    // 3. 二次确认
    if policy.RequiresConfirmation {
        return mcpErr(MCPErrorRequiresConfirmation, "user confirmation required", map[string]any{
            "tool":    entry.FullName,
            "input":   call.Input,
            "policy":  policy,
        })
    }

    return nil
}
```

**二次确认流程**：

```
Agent 调 hub.invoke_connected_tool
  → 首次调用：Policy.RequiresConfirmation = true
  → 返回 requires_confirmation 错误（code=-32013）
  → Agent UI 弹窗询问用户
  → 用户确认后，Agent 再调 hub.confirm_and_invoke(confirm_token, tool, input)
  → 校验 confirm_token 有效期（5 分钟）后真正执行
  → 记录 ToolInvocationLog.Confirmed = true
```

### 13.6 OAuth 凭据托管与轮换

上游 MCP Server 的 OAuth 凭据（access_token / refresh_token）由 Hub 统一托管：

```go
type OAuthCredentials struct {
    ServerID      string
    AccessToken   string  // 加密存储
    RefreshToken  string
    ExpiresAt     time.Time
    Scopes        []string
}

// 轮换流程
func (c *OAuthCredentials) Refresh(ctx context.Context) error {
    if time.Until(c.ExpiresAt) > 5*time.Minute {
        return nil  // 仍未过期
    }
    newTokens, err := exchangeRefreshToken(ctx, c.RefreshToken)
    if err != nil {
        return err
    }
    c.AccessToken = newTokens.AccessToken
    c.RefreshToken = newTokens.RefreshToken
    c.ExpiresAt = newTokens.ExpiresAt
    return updateInDB(c)
}
```

**自动 401 重试**：调用时若返回 401，触发 `Refresh()` 后重试 1 次。

### 13.7 工具调用审计

每次 Tool 调用都写入 `tool_invocation_logs` 表（详见 [第 8.5 节](#85-tool-路由与策略模型)）：

```go
func (r *Router) log(ctx context.Context, entry *RouteEntry, call *ToolCall, result *ToolResult, latency time.Duration, confirmed bool) {
    log := &ToolInvocationLog{
        WorkspaceID:       getWorkspaceID(ctx),
        UserID:            getUserID(ctx),
        AgentClientID:     getAgentClientID(ctx),
        MCPSessionID:      getMCPSessionID(ctx),
        ToolName:          entry.FullName,
        ConnectedServerID: &entry.Server.ID,
        InputJSON:         marshalInput(call.Input),
        OutputSummary:     summarize(result.Output),
        Status:            result.Status,
        LatencyMs:         int(latency.Milliseconds()),
        Confirmed:         confirmed,
        InvokedAt:         time.Now(),
    }
    global.GLB_DB.Create(log)
}
```

### 13.8 Tool Routing 配置

```yaml
tool_routing:
  enabled: true
  upstream:
    pool_size: 100
    request_timeout: 30s
    max_retries: 2
    circuit_breaker:
      max_failures: 5
      timeout: 30s
  policy:
    enforce_for_all: true
    default_daily_limit: 1000
  audit:
    log_input_max_bytes: 4096     # 超过则截断
    log_output_max_bytes: 4096
    retention_days: 90            # 分区表保留期
```

---

## 14. 性能与可扩展性

> 本章定义 Open Agent Hub 的性能目标、扩展策略与多租户隔离下的稳定性保障。

### 14.1 水平扩展策略

#### 14.1.1 Gateway 无状态化

```
目标：Gateway 实例不持有会话状态，可以任意水平扩缩

实现：
  - MCPSession 状态外置到 Redis（Hash 结构）
  - SSE 长连接仍需粘性会话 → 通过一致性 hash by session_id 路由
  - 工具调用上下文通过 token 每次重新构建
```

```go
// Redis 中的 Session 存储
type SessionState struct {
    SessionID    string
    WorkspaceID  string
    UserID       string
    Scopes       []string
    LastActivity time.Time
    StreamState  json.RawMessage  // SSE 增量状态
}

// Gateway 重启或迁移时，从 Redis 恢复
func restoreSession(ctx context.Context, sessionID string) (*SessionState, error) {
    key := fmt.Sprintf("session:%s", sessionID)
    data, err := global.GLB_REDIS.HGetAll(ctx, key).Result()
    if err != nil { return nil, err }
    return parseSessionState(data)
}
```

#### 14.1.2 SSE 会话粘性

```yaml
# Kubernetes Ingress / Nginx upstream consistent hash
upstream mcp_gateway {
    hash $cookie_session_id consistent;
    server mcp-gateway-1:8085;
    server mcp-gateway-2:8085;
    server mcp-gateway-3:8085;
}
```

#### 14.1.3 跨节点广播

WebSocket 事件推送走 Redis Pub/Sub：

```go
// 发布
global.GLB_REDIS.Publish(ctx, "ws:broadcast:ws_123", payload)

// 订阅
pubsub := global.GLB_REDIS.Subscribe(ctx, "ws:broadcast:*")
defer pubsub.Close()
ch := pubsub.Channel()
for msg := range ch {
    // 解析 workspace_id → 找本节点上的 WSClient → 推送
    deliverToLocalClients(msg)
}
```

#### 14.1.4 数据库读写分离 + 分片路线图

**P0-P1 阶段**：单 Primary + 1 Read Replica，按查询路由。

**P2 阶段**：按 `workspace_id` hash 分片到多个 Primary，引入 Vitess 或自研 shard router。

```go
// Shard router 接口预留
type ShardRouter interface {
    GetDB(workspaceID string) *gorm.DB
}

type HashShardRouter struct {
    shards []*gorm.DB
}

func (h *HashShardRouter) GetDB(workspaceID string) *gorm.DB {
    hash := fnv.New32a()
    hash.Write([]byte(workspaceID))
    idx := hash.Sum32() % uint32(len(h.shards))
    return h.shards[idx]
}
```

### 14.2 多级缓存策略

| 层级 | 介质 | 适用数据 | TTL | 失效策略 |
|------|------|----------|-----|----------|
| L1 进程内 | `sync.Map` + TTL | Rules / Tool Policy | 60s | TTL 过期 + DB 通知主动失效 |
| L2 Redis | 共享 | Memory Snapshot / Workspace Quota / OAuth Token | 5min | TTL 过期 + 主动 DEL |
| L3 CDN | CloudFront | 公开 Prompt 模板 | 1h | 版本号前缀 |

#### 14.2.1 Prompt Cache 协调

通过 **Resource URI 版本化 + ETag** 实现：

```
Agent 请求: hub://workspace/ws_123/rules/global
Server 响应: 
  - Body: { "content": "..." }
  - ETag: W/"v-2026-06-07T03:15:42Z"
  - Cache-Control: max-age=300

Agent 下次请求: If-None-Match: W/"v-2026-06-07T03:15:42Z"
Server 响应（无变化）: 304 Not Modified
```

**规则更新主动失效**：

```go
func (s *RuleService) UpdateRule(ctx context.Context, r *Rule) error {
    // 1. 写 DB
    if err := global.GLB_DB.Save(r).Error; err != nil {
        return err
    }
    // 2. 失效 L1 缓存
    s.localCache.Delete(cacheKey(r.WorkspaceID, r.Scope))
    // 3. 失效 L2 Redis
    global.GLB_REDIS.Del(ctx, fmt.Sprintf("rules:ws:%s:%s", r.WorkspaceID, r.Scope))
    // 4. 提升版本号（让所有 Agent 下次拉取拿到新内容）
    s.bumpVersion(r.WorkspaceID, r.Scope)
    return nil
}
```

### 14.3 配额与限流

#### 14.3.1 配额维度

| 维度 | 限制（Pro 计划） | 限制（Free 计划） |
|------|----------------|------------------|
| Memory Count | 100,000 | 5,000 |
| Vector Storage | 2 GB | 100 MB |
| Tool Calls / Day | 50,000 | 1,000 |
| Skill Count | 5,000 | 100 |
| API Keys | 20 | 2 |
| MCP Sessions 并发 | 100 | 5 |

#### 14.3.2 配额检查

```go
func (s *QuotaService) CheckAndIncrement(ctx context.Context, workspaceID, metric string, qty int) error {
    key := fmt.Sprintf("quota:%s:%s", workspaceID, metric)
    used, err := global.GLB_REDIS.IncrBy(ctx, key, int64(qty)).Result()
    if err != nil { return err }

    // 设置过期（按日 / 按月）
    if strings.HasSuffix(metric, "_daily") {
        global.GLB_REDIS.Expire(ctx, key, 24*time.Hour)
    } else {
        global.GLB_REDIS.Expire(ctx, key, 30*24*time.Hour)
    }

    limit := getLimit(workspaceID, metric)
    if used > int64(limit) {
        return mcpErr(MCPErrorQuotaExceeded, "quota exceeded", map[string]any{
            "metric": metric,
            "limit":  limit,
            "used":   used,
        })
    }
    return nil
}
```

#### 14.3.3 三层限流

详见 [第 11.4 节](#114-限流与熔断)：按 Tool / Agent Client / Workspace 三层。

### 14.4 多租户隔离与稳定性

#### 14.4.1 三层防御

| 层级 | 机制 | 失效时影响 |
|------|------|------------|
| 应用层 | GORM Plugin 自动注入 `workspace_id` | 业务代码漏改 |
| GORM Session | `SET app.current_workspace_id = ?` | 中间件漏调用 |
| 数据库层 | RLS Policy | 仅 DBA 可改，最可靠 |

#### 14.4.2 噪声邻居防护

```go
// 单租户慢查询检测
func (d *DBGuard) CheckSlowQuery(ctx context.Context, wsID string, duration time.Duration) {
    if duration > 5*time.Second {
        log.Warn("slow query detected", zap.String("workspace", wsID), zap.Duration("duration", duration))
        // 自动增加该 workspace 的限流权重
        d.adaptiveRateLimit(wsID, factor: 0.5)
    }
}
```

#### 14.4.3 单租户 CPU 配额（容器化）

```yaml
# Kubernetes ResourceQuota per workspace
apiVersion: v1
kind: ResourceQuota
metadata:
  name: workspace-ws-123
spec:
  hard:
    requests.cpu: "2"
    requests.memory: 4Gi
    limits.cpu: "4"
    limits.memory: 8Gi
```

P0-P1 阶段通过 namespace quota 实施；P2 阶段通过 per-tenant 容器池化实施。

### 14.5 性能 SLO 目标

| API | P50 | P95 | P99 | 错误率 |
|-----|-----|-----|-----|--------|
| `hub.get_global_rules` | 10ms | 30ms | 50ms | < 0.1% |
| `hub.get_agent_profile` | 30ms | 100ms | 200ms | < 0.1% |
| `hub.search_memory` (TopK=10) | 80ms | 150ms | 200ms | < 0.5% |
| `hub.propose_memory` | 50ms | 150ms | 300ms | < 0.5% |
| `hub.save_memory` | 30ms | 100ms | 200ms | < 0.5% |
| `hub.invoke_connected_tool` (GitHub) | 500ms | 1.5s | 2s | < 1% |
| `hub.list_skills` | 20ms | 80ms | 150ms | < 0.1% |
| Memory 召回准确率 | - | - | - | > 85% (人工评估) |

**SLO 监控**：

- OpenTelemetry Trace 上报到 Jaeger / Tempo
- Prometheus 指标：`mcp_request_duration_seconds` (histogram)、`mcp_request_total` (counter)
- Grafana Dashboard：SLO 概览 + 各 Tool 维度
- AlertManager：P99 超阈值告警

### 14.6 容量规划

#### 14.6.1 单实例承载

| 配置 | 承载能力 |
|------|----------|
| 2C4G × 1 实例 | 50 并发 MCP 会话，200 QPS |
| 4C8G × 1 实例 | 200 并发 MCP 会话，800 QPS |
| 8C16G × 1 实例 | 500 并发 MCP 会话，2000 QPS |

P0 起步：2 实例 4C8G 满足 1000 Workspace × 50 Session 平均负载。

#### 14.6.2 存储容量

| 数据 | 1 万 Workspace | 10 万 Workspace |
|------|---------------|----------------|
| Rules | 1 GB | 10 GB |
| Memory | 50 GB | 500 GB |
| Vector Embedding | 100 GB（1536 维 float32 × 100K/WS）| 1 TB |
| Tool Invocation Logs（90d）| 30 GB | 300 GB |
| Audit Logs（永久）| 10 GB | 100 GB |

P0 起步：500 GB SSD 单机 PG 即可。

---

## 16. Go 后端项目结构（SaaS 版）

> 本章描述 Open Agent Hub SaaS 后端的代码组织。**P0 阶段是单进程多 Service**，4 Hub（Agent / Context / Memory / Tool）只是**前端产品分类**（详见 Ch 15.2），并非后端微服务边界。微服务拆分是 P2 阶段才考虑的扩展。

### 16.1 顶层架构与设计原则

| 设计原则 | 体现 |
|---------|------|
| **单进程多 Service** | `main.go` 启动 8084（管理面 REST）+ 8085（MCP Gateway）；P0 共享进程，P2 视流量拆服务 |
| **扁平化按职责分包** | 顶层包按业务职责划分，避免过度分层 |
| **Handler / Service / Model 三层** | Handler 薄（参数校验 + 调用 Service），Service 厚（业务规则），Model 纯（ORM 映射） |
| **全局单例管理依赖** | `global/` 包管理 DB / Redis / Logger / Vector Client；Service 通过依赖注入获取 |
| **依赖显式注入** | Service 不直接读 `global.GLB_*`，通过构造函数注入，便于测试 mock |
| **P2 兼容路径** | `local-bridge/` 作为独立子模块（甚至可独立仓库），通过 Workspace MCP Token 与 SaaS 通信 |

### 16.2 顶层包结构

```text
open-agent-hub/
├── main.go                          # 程序入口（双端口监听）
├── config.yaml                      # 配置文件（详见 Ch 17）
├── go.mod
│
├── cmd/                             # 多入口
│   ├── server/
│   │   └── main.go                  # SaaS 主进程（8084 + 8085）
│   └── migrate/
│       └── main.go                  # 数据库迁移 CLI（包装 golang-migrate）
│
├── api/                             # 管理面 REST（端口 8084）
│   ├── routes.go                    # 路由注册
│   └── v1/
│       ├── org/                     # Organization 管理
│       ├── workspace/               # Workspace 管理
│       ├── member/                  # 成员与角色
│       ├── mcp_token/               # MCP Token 签发与撤销
│       ├── agent_client/            # Agent Client 识别
│       ├── rule/                    # Rule CRUD（Global / Project / Agent）
│       ├── output_preference/       # 输出偏好
│       ├── memory/                  # Memory 管理（CRUD + 治理）
│       ├── skill/                   # Skill 管理
│       ├── connected_server/        # 外部 MCP Server 注册
│       ├── tool_policy/             # Tool Policy 配置
│       ├── usage/                   # 用量与配额
│       ├── billing/                 # 计费与套餐
│       ├── audit/                   # 审计日志查询
│       └── local_bridge/            # Local Bridge 设备管理
│
├── mcp/                             # MCP Server 协议实现（端口 8085）
│   ├── server.go                    # Streamable HTTP 入口
│   ├── session/                     # MCPSession 状态机
│   │   ├── session.go
│   │   ├── manager.go               # 分布式 Session（Redis 存储）
│   │   └── lock.go                  # 一致性 hash 粘性
│   ├── transport/                   # 传输层
│   │   ├── streamable_http.go       # 强制支持的传输
│   │   ├── sse.go                   # 旧版本兼容
│   │   └── stdio.go                 # 仅 Local Bridge 内部测试
│   ├── protocol/                    # JSON-RPC 2.0 帧
│   │   ├── request.go
│   │   ├── response.go
│   │   └── error.go
│   ├── tools/                       # 26 个 hub.* Tools 实现
│   │   ├── rules.go                 # 配置类 5 个
│   │   ├── memory.go                # 记忆类 6 个
│   │   ├── skills.go                # Skill 类 4 个
│   │   ├── tool_routing.go          # MCP 聚合类 3 个
│   │   ├── project.go               # 项目上下文类 5 个
│   │   └── audit.go                 # 审计类 3 个
│   ├── resources/                   # MCP Resources
│   │   ├── rules_resource.go
│   │   ├── snapshot_resource.go
│   │   └── etag.go                  # ETag + 304 协商
│   └── prompts/                     # 预置 Prompts
│       ├── bootstrap.go             # open_agent_hub_project_bootstrap
│       ├── code_review.go
│       ├── memory_review.go
│       ├── vibecoding_plan.go
│       └── refactor_plan.go
│
├── gateway/                         # MCP Gateway 核心
│   ├── gateway.go                   # 5 大角色分发（配置/记忆读/记忆写/Skill/外部 MCP）
│   ├── middleware/
│   │   ├── tenant.go                # 多租户上下文注入
│   │   ├── auth.go                  # Token 校验 + workspace 归属
│   │   ├── ratelimit.go             # 三层限流（Tool / Client / Workspace）
│   │   ├── audit.go                 # 调用日志写入 audit_logs
│   │   ├── recovery.go              # panic 恢复
│   │   └── timeout.go               # 单 Tool 超时
│   ├── ratelimit/                   # 分布式令牌桶
│   │   ├── token_bucket.go          # Redis Lua 实现
│   │   └── quota.go                 # 配额查询
│   ├── circuit/                     # 熔断器
│   │   └── breaker.go               # 包装 gobreaker
│   └── errcode/                     # MCP 错误码映射
│       ├── code.go                  # -32700 ~ -32099 业务扩展码
│       └── mapping.go               # 内部错误 → MCP 错误
│
├── tenant/                          # 多租户上下文
│   ├── context.go                   # WorkspaceContext{OrgID, WorkspaceID, ProjectID, UserID, AgentClientID}
│   ├── resolver.go                  # 从 Token 解析 + 注入 ctx
│   ├── guard.go                     # workspace_id 强制校验（每个 Handler 入口）
│   └── rls.go                       # GORM Plugin（自动注入 workspace_id WHERE）
│
├── tool/                            # 外部 MCP 聚合与 Tool Policy
│   ├── router.go                    # 路由表 hub.github.create_pull_request
│   ├── registry.go                  # ConnectedMCPServer 注册表
│   ├── pool/                        # 上游连接池
│   │   ├── http_pool.go             # http.Transport 复用
│   │   └── stream_pool.go           # SSE / Streamable HTTP 长连接
│   ├── proxy/                       # 上游 MCP Server 调用
│   │   ├── client.go
│   │   ├── oauth_client.go          # 上游 OAuth 凭据
│   │   └── parallel.go              # 并行扇出
│   ├── policy/                      # Tool Policy 校验链路
│   │   ├── engine.go                # Allowed / RequiresConfirmation / MaxCalls
│   │   └── confirmation.go          # 高危操作二次确认
│   └── catalog/                     # Tool 目录
│       └── catalog.go               # 工具发现与缓存
│
├── auth/                            # OAuth + Token
│   ├── oauth/                       # OAuth 2.1 + PKCE
│   │   ├── server.go                # /authorize, /token 端点
│   │   ├── pkce.go                  # code_verifier / code_challenge
│   │   └── client.go                # 第三方登录
│   ├── token/                       # 多种 Token 类型
│   │   ├── workspace_mcp_token.go   # Workspace 级长期 Token
│   │   ├── pat.go                   # Personal Access Token
│   │   ├── session_token.go         # 短期会话 Token（24h）
│   │   └── jwks.go                  # JWT 签发与 JWKS 暴露
│   ├── rbac/                        # 权限引擎
│   │   ├── casbin.go                # casbin Model + Policy
│   │   └── policy.yaml              # RBAC 策略文件
│   └── identity/                    # 用户身份
│       ├── user.go
│       └── session.go
│
├── billing/                         # 计费
│   ├── quota/                       # 配额管理
│   │   ├── manager.go
│   │   └── tracker.go               # Redis 实时计数
│   ├── usage/                       # 用量统计
│   │   ├── aggregator.go            # 按 workspace + period 聚合
│   │   └── reporter.go              # 每日 / 每月报表
│   └── stripe/                      # Stripe 集成（P2）
│       ├── client.go
│       └── webhook.go
│
├── audit/                           # 审计日志
│   ├── logger.go                    # append-only 写入
│   ├── retention.go                 # 保留与归档
│   └── query.go                     # 查询 API（带 RLS）
│
├── memory/                          # 记忆系统（继承 Ch 14 算法）
│   ├── discipline/                  # 写入纪律
│   │   ├── engine.go
│   │   ├── llm_judge.go
│   │   ├── formula_scorer.go
│   │   ├── managed_strategy.go
│   │   └── scorer.go
│   ├── validity/                    # 双时间轴
│   │   ├── manager.go
│   │   └── bitemporal.go
│   ├── snapshot/                    # 冻结快照
│   │   ├── manager.go
│   │   └── cache_point.go
│   ├── retrieval/                   # 向量召回（新增）
│   │   ├── vector_engine.go         # pgvector HNSW 查询
│   │   ├── hybrid.go                # BM25 + 向量混合
│   │   ├── embedder.go              # Embedding 调用 OpenAI/Cohere
│   │   └── tenant_filter.go         # 强制 workspace_id 过滤
│   ├── compactor/
│   │   └── compactor.go
│   └── curator/
│       ├── curator.go
│       └── state_machine.go
│
├── model/                           # GORM 模型（详见 Ch 8）
│   ├── organization.go
│   ├── workspace.go
│   ├── workspace_member.go
│   ├── user.go
│   ├── agent_client.go
│   ├── mcp_session.go
│   ├── rule.go
│   ├── output_preference.go
│   ├── memory.go
│   ├── memory_validity.go
│   ├── memory_snapshot.go
│   ├── memory_access_log.go
│   ├── memory_mapping.go
│   ├── skill_curation_log.go
│   ├── connected_mcp_server.go
│   ├── tool_policy.go
│   ├── tool_invocation_log.go
│   ├── api_key.go
│   ├── oauth_token.go
│   ├── usage_record.go
│   └── audit_log.go
│
├── config/                          # 配置结构（YAML 绑定）
│   ├── config.go                    # Server 顶层
│   ├── system.go                    # host, ports, env
│   ├── postgres.go                  # pgx 配置
│   ├── redis.go                     # 缓存 + Session
│   ├── vector.go                    # pgvector 维度 / 距离度量
│   ├── mcp.go                       # MCP 传输 / 协议版本
│   ├── oauth.go                     # OAuth client_id/secret
│   ├── ratelimit.go                 # 限流阈值
│   ├── zap.go                       # 日志
│   └── cors.go
│
├── global/                          # 全局单例（精简）
│   ├── global.go                    # GLB_DB, GLB_REDIS, GLB_LOG, GLB_VECTOR
│   └── version.go
│
├── initialize/                      # 启动初始化
│   ├── postgres.go                  # pgx pool
│   ├── redis.go                     # go-redis client
│   ├── vector.go                    # pgvector client
│   ├── gorm_biz.go                  # GORM 模型注册（仅 dev）
│   ├── mcp_server.go                # MCP Server 启动
│   ├── gateway.go                   # Gateway Middleware 装配
│   └── frontend_static.go           # go:embed web/dist
│
├── migrate/                         # golang-migrate SQL 文件
│   ├── 000001_init_tenants.up.sql
│   ├── 000001_init_tenants.down.sql
│   ├── 000002_add_workspace_id.up.sql
│   ├── 000002_add_workspace_id.down.sql
│   ├── 000003_memory_embedding.up.sql        # pgvector 扩展
│   ├── 000004_partition_tool_logs.up.sql     # 按月分区
│   └── 000005_rls_policies.up.sql            # 行级安全策略
│
├── local-bridge/                    # Local Bridge（P2，可拆独立仓库）
│   ├── cmd/
│   │   └── bridge/main.go           # 独立二进制
│   ├── adapter/
│   │   ├── adapter.go
│   │   ├── cursor/
│   │   ├── claude/
│   │   ├── opencode/
│   │   ├── copilot/
│   │   └── windsurf/
│   ├── sync/
│   │   ├── transformer.go
│   │   ├── watcher.go               # fsnotify
│   │   └── conflict_resolver.go
│   ├── client/                      # 与 SaaS 通信
│   │   └── http_client.go
│   └── README.md
│
├── utils/
│   ├── uid.go                       # UUID / ULID
│   ├── crypto.go                    # SHA-256 / AES
│   ├── jwt.go                       # JWT 签发与验证
│   ├── httpclient.go                # 复用 http.Client
│   ├── jsonutil.go                  # gjson / sjson
│   └── fileutil.go
│
├── web/                             # SaaS Console 前端（go:embed）
│
└── deploy/                          # 部署产物
    ├── docker-compose.yml
    ├── docker-compose.dev.yml
    ├── helm/
    │   ├── Chart.yaml
    │   ├── values.yaml
    │   └── templates/
    ├── local-bridge-installer/
    │   ├── macos.pkg
    │   └── windows.msi
    └── k8s/                         # 裸 K8s 清单（Helm 之外的备选）
```

### 16.3 关键包说明

#### 16.3.1 `mcp/` — MCP Server 协议层

**职责**：实现 MCP 规范的 Server 端，对外暴露 `/mcp` 端点。**不感知业务**，只做协议适配。

```go
// mcp/server.go 核心入口
type Server struct {
    sessionMgr  *session.Manager
    tools       map[string]ToolHandler
    resources   map[string]ResourceHandler
    prompts     map[string]PromptHandler
    transport   transport.Transport
}

func (s *Server) HandleHTTP(w http.ResponseWriter, r *http.Request) {
    sess, err := s.sessionMgr.AcceptOrResume(r)
    if err != nil { /* OAuth challenge */ }
    defer s.sessionMgr.Save(sess)

    msg := s.protocol.Decode(r.Body)
    switch msg.Method {
    case "tools/call":
        s.dispatchTool(sess, msg)
    case "resources/read":
        s.dispatchResource(sess, msg)
    case "prompts/get":
        s.dispatchPrompt(sess, msg)
    }
}
```

**关键约束**：
- 不直接访问 DB / Redis；通过 Gateway 注入的 `WorkspaceContext` 调用 Service
- Session 状态外置到 Redis（多副本时一致）
- 所有 Tool 调用的实际执行委托给 `gateway/`

#### 16.3.2 `gateway/` — 5 大角色分发

```go
// gateway/gateway.go
type Gateway struct {
    ruleService      *rule.Service
    memoryService    *memory.Service
    skillService     *skill.Service
    toolRouter       *tool.Router
    auditLogger      *audit.Logger
    middlewares      []Middleware
}

func (g *Gateway) Dispatch(ctx context.Context, req *MCPRequest) *MCPResponse {
    // 1. Middleware 链
    for _, mw := range g.middlewares {
        ctx, err = mw(ctx, req)
        if err != nil { return errResponse(err) }
    }

    // 2. 按 Tool 前缀路由到 5 大角色
    switch {
    case strings.HasPrefix(req.Tool, "hub.get_") && isRuleTool(req.Tool):
        return g.ruleService.Handle(ctx, req)
    case strings.HasPrefix(req.Tool, "hub.search_memory") || strings.HasPrefix(req.Tool, "hub.propose_memory"):
        return g.memoryService.HandleRead(ctx, req)
    case strings.HasPrefix(req.Tool, "hub.save_memory"):
        return g.memoryService.HandleWrite(ctx, req)
    case strings.HasPrefix(req.Tool, "hub.list_skills") || strings.HasPrefix(req.Tool, "hub.search_skills"):
        return g.skillService.Handle(ctx, req)
    case strings.HasPrefix(req.Tool, "hub.invoke_connected_tool") || strings.HasPrefix(req.Tool, "hub.list_connected_tools"):
        return g.toolRouter.Handle(ctx, req)
    default:
        return errResponse(ErrMethodNotFound)
    }
}
```

#### 16.3.3 `tenant/` — 多租户上下文

**WorkspaceContext** 是所有业务 Service 的隐式输入：

```go
// tenant/context.go
type WorkspaceContext struct {
    OrgID         string  // 组织 ID
    WorkspaceID   string  // 工作区 ID
    ProjectID     string  // 项目 ID（可空）
    UserID        string  // 操作用户
    AgentClientID string  // 调用的 Agent Client
    Roles         []string // RBAC 角色
}

type ctxKey struct{}

func WithWorkspace(parent context.Context, ws *WorkspaceContext) context.Context {
    return context.WithValue(parent, ctxKey{}, ws)
}

func FromContext(ctx context.Context) *WorkspaceContext {
    ws, _ := ctx.Value(ctxKey{}).(*WorkspaceContext)
    return ws
}
```

**GORM Plugin 自动注入**：

```go
// tenant/rls.go
type WorkspaceFilter struct{}

func (p *WorkspaceFilter) Name() string { return "workspace_filter" }

func (p *WorkspaceFilter) BeforeQuery(db *gorm.DB) {
    ws := FromContext(db.Statement.Context)
    if ws == nil { return }
    if db.Statement.Schema != nil && hasWorkspaceIDField(db.Statement.Schema) {
        db.Statement.AddClause(clause.Where{
            Exprs: []clause.Expression{
                clause.Eq{Column: clause.Column{Table: db.Statement.Table, Name: "workspace_id"}, Value: ws.WorkspaceID},
            },
        })
    }
}
```

#### 16.3.4 `tool/` — 外部 MCP 聚合

**路由表**（前缀命名空间避免冲突）：

```go
// tool/router.go
type Route struct {
    Prefix     string                 // "github", "slack", "jira"
    ServerID   string                 // connected_mcp_servers.id
    ToolPolicy *ToolPolicy            // 限频 / 二次确认
}

var routes = []Route{
    {Prefix: "github", ServerID: "srv_github", ToolPolicy: &ToolPolicy{Allowed: true, MaxCallsPerDay: 1000}},
    {Prefix: "slack",  ServerID: "srv_slack",  ToolPolicy: &ToolPolicy{RequiresConfirmation: true}},
}
```

调用时 Tool 名称转换为 `hub.github.create_pull_request` 形式。

#### 16.3.5 `auth/` — Token 与权限

**4 种 Token**（按生命周期与作用域）：

| Token 类型 | 生命周期 | 作用域 | 存储 |
|----------|---------|--------|------|
| `Workspace MCP Token` | 长期（可吊销）| 单 Workspace 全部 | DB hash |
| `Personal Access Token (PAT)` | 长期（可吊销）| 单 User 全部 | DB hash |
| `Session Token` | 24h | 浏览器会话 | Redis |
| `OAuth Access Token` | 1h | 第三方资源 | DB hash |

#### 16.3.6 `memory/retrieval/` — 向量召回（新增）

```go
// memory/retrieval/hybrid.go
type HybridSearch struct {
    bm25       *BM25Index
    vector     *pgxvector.Client
    embedder   Embedder
    topK       int
    alpha      float64  // 向量权重 0.7
}

func (h *HybridSearch) Search(ctx context.Context, wsCtx *tenant.WorkspaceContext, q string) ([]Memory, error) {
    // 1. Embedding
    vec, _ := h.embedder.Embed(ctx, q)
    // 2. 强制 workspace_id 过滤
    filter := fmt.Sprintf("workspace_id = '%s'", wsCtx.WorkspaceID)
    // 3. 向量召回
    vecHits := h.vector.Search(ctx, vec, h.topK, filter)
    // 4. BM25 召回
    bmHits := h.bm25.Search(ctx, q, h.topK, filter)
    // 5. RRF 融合
    return reciprocalRankFusion(vecHits, bmHits, h.alpha), nil
}
```

### 16.4 启动入口（main.go）

```go
// main.go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"
    "go.uber.org/zap"

    "open-agent-hub/api"
    "open-agent-hub/config"
    "open-agent-hub/global"
    "open-agent-hub/gateway"
    "open-agent-hub/initialize"
    "open-agent-hub/mcp"
    "open-agent-hub/tenant"
)

func main() {
    cfg, err := config.Load(global.GLB_VP)
    if err != nil { log.Fatalf("config load: %v", err) }

    // 1. 初始化基础设施
    initialize.Postgres(cfg)
    initialize.Redis(cfg)
    initialize.Vector(cfg)
    initialize.Logger(cfg)

    // 2. 装配 Gateway 与 MCP Server
    gw := gateway.New(cfg)
    mcpServer := mcp.NewServer(cfg, gw)

    // 3. 启动双端口
    restSrv := &http.Server{Addr: cfg.System.RestAddr, Handler: api.NewRouter(gw)}
    mcpSrv  := &http.Server{Addr: cfg.System.MCPAddr,  Handler: mcpServer.Handler()}

    go func() { _ = restSrv.ListenAndServe() }()
    go func() { _ = mcpSrv.ListenAndServe() }()

    // 4. 优雅关闭
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    _ = restSrv.Shutdown(ctx)
    _ = mcpSrv.Shutdown(ctx)

    zap.L().Info("shutdown complete")
}
```

### 16.5 配置结构（config/）

完整 `config.yaml` 见 Ch 17.4，配置结构定义（YAML → Go struct 绑定）如下：

```go
// config/config.go
type Server struct {
    System    SystemConfig    `yaml:"system"`
    Postgres  PostgresConfig  `yaml:"postgres"`
    Redis     RedisConfig     `yaml:"redis"`
    Vector    VectorConfig    `yaml:"vector"`
    MCP       MCPConfig       `yaml:"mcp"`
    OAuth     OAuthConfig     `yaml:"oauth"`
    RateLimit RateLimitConfig `yaml:"ratelimit"`
    Zap       ZapConfig       `yaml:"zap"`
    CORS      CORSConfig      `yaml:"cors"`
}
```

### 16.6 与 Local Bridge 的边界

- **SaaS 端**（`open-agent-hub/`）：所有 `mcp/` `gateway/` `tenant/` `tool/` `auth/` `billing/` `audit/` `memory/` 都是 SaaS 服务端能力。
- **Local Bridge**（`local-bridge/`）：独立二进制，**不依赖** SaaS 的任何业务包，仅通过 HTTP 调用 SaaS 的 REST 与 MCP 端点（用 Workspace MCP Token 鉴权）。
- **共用**：`utils/` 与 `config/` 的部分类型定义通过 Go module 引用（submodule 方式）。

### 16.7 演进路线

| 阶段 | 包结构变化 |
|------|-----------|
| **P0** | 单仓库单进程，`local-bridge/` 作为子目录；`mcp/` 与 `api/` 共享进程 |
| **P1** | 流量上升后，将 `mcp/` 拆为独立二进制（独立部署），共享 `model/` `tenant/` `memory/` |
| **P2** | `local-bridge/` 拆为独立仓库（独立发版）；SaaS 内部按 `gateway/` `tool/` `memory/` 拆微服务 |



## 17. 技术选型（SaaS 版）

> 本章定义 Open Agent Hub SaaS 后端使用的技术栈、依赖、配置文件与启动命令。

### 17.1 后端技术栈总览

| 决策项 | 选型 | 理由 / 说明 |
|--------|------|-------------|
| **语言** | Go 1.22+ | 团队已有生产经验，静态二进制利于容器部署 |
| **HTTP 框架（REST）** | Gin | 团队已有生产使用，中间件生态成熟 |
| **MCP 协议** | `github.com/modelcontextprotocol/go-sdk`（官方） | 随上游演进，避免自研 |
| **ORM** | GORM + `pgx/v5` 直连 | GORM 处理 model，pgx 处理向量 / RLS 原生 SQL |
| **数据库（生产）** | PostgreSQL 14+ | 需 pgvector 扩展 + RLS + 分区表 |
| **数据库（开发）** | SQLite（`glebarez/sqlite`） | `db-type` 配置切换，零依赖启动 |
| **缓存 / Session / Pub/Sub** | Redis 7+ | 多副本 Session 一致、限流计数器、广播 |
| **向量检索** | pgvector 0.5+ | 与 PG 同库，P0–P1 够用，P2 可拆 Milvus |
| **配置管理** | Viper + `config.yaml` | 团队已使用，热重载 + 多 profile |
| **日志** | Zap + 自定义 Cutter | 结构化 JSON + 按日切割 |
| **错误追踪** | OpenTelemetry → Jaeger / Tempo | MCP 协议跨度追踪 |
| **指标** | Prometheus | `mcp_request_duration_seconds` 等 |
| **JSON 动态读写** | gjson / sjson | 记忆 / Rule JSONB 字段处理 |
| **数据库迁移** | golang-migrate | 版本化 .up.sql / .down.sql（生产） |
| **OAuth 2.1** | `golang.org/x/oauth2` + 自研 PKCE | 服务端 + 客户端 |
| **RBAC** | `github.com/casbin/casbin/v2` | 声明式 Policy 文件 |
| **熔断** | `github.com/sony/gobreaker` | 上游 MCP Server 调用熔断 |
| **限流** | Redis Lua 脚本（令牌桶） | 分布式原子 |
| **后台任务** | `github.com/hibiken/asynq` | 记忆 snapshot、Skill curator cron |
| **JWT** | `github.com/golang-jwt/jwt/v5` | Session Token 签发 |
| **Embedding** | `github.com/sashabaranov/go-openai` | 统一封装 OpenAI / Azure / Cohere |
| **MCP 客户端（上游）** | `mcp-go`（代理模式） | 发起 Streamable HTTP 请求 |
| **单二进制嵌入** | `go:embed web/dist` | 同一个二进制含前端 |
| **Local Bridge（P2）** | 独立仓库 + 独立二进制 | 与 SaaS 解耦，单独发版 |
| **WebSocket（内部）** | `gorilla/websocket` | 审计日志实时推送（保留兼容） |
| **文件监听（Local Bridge）** | fsnotify | Viper 已依赖 |

### 17.2 调整与删除

| 旧依赖 | 处理 | 原因 |
|--------|------|------|
| `github.com/glebarez/sqlite` | **保留**（仅 dev profile） | 本地零依赖体验 |
| `github.com/go-sql-driver/mysql` | **删除** | SaaS 不再支持 MySQL |
| `GORM AutoMigrate` | **删除** | 改用 golang-migrate 避免隐式迁移 |
| `core/sync_hub.go`（同步引擎） | **删除** | Local Bridge 独立仓库 |
| `core/ws_hub.go` | **保留** | 内部审计实时推送仍需 |
| `core/adapter_registry.go` | **删除** | Adapter 移到 Local Bridge |
| `middleware/auth.go`（本地 JWT） | **重写** | 改为 OAuth 2.1 + casbin |
| `response.Response`（`{code,data,msg}`） | **保留** | REST 管理面继续使用 |

### 17.3 关键依赖（go.mod 补充）

```go
// go.mod 补充 SaaS 关键依赖
module github.com/open-agent-hub/open-agent-hub

go 1.22

require (
    // Web
    github.com/gin-gonic/gin v1.10.0

    // MCP
    github.com/modelcontextprotocol/go-sdk v0.1.0   // 官方 SDK

    // 数据库
    gorm.io/gorm v1.25.10
    gorm.io/driver/postgres v1.5.7
    github.com/jackc/pgx/v5 v5.6.0                  // 直连 + 向量
    github.com/pgvector/pgvector-go v0.1.0          // 向量类型

    // 缓存
    github.com/redis/go-redis/v9 v9.6.1

    // 配置 / 日志
    github.com/spf13/viper v1.19.0
    go.uber.org/zap v1.27.0
    github.com/fsnotify/fsnotify v1.7.0             // Local Bridge 仍需

    // 鉴权
    golang.org/x/oauth2 v0.21.0
    github.com/golang-jwt/jwt/v5 v5.2.1
    github.com/casbin/casbin/v2 v2.77.2

    // 可靠性
    github.com/sony/gobreaker v1.0.0

    // 任务
    github.com/hibiken/asynq v0.24.1

    // 观测
    go.opentelemetry.io/otel v1.28.0
    github.com/prometheus/client_golang v1.20.0

    // JSON / 工具
    github.com/tidwall/gjson v1.17.0
    github.com/tidwall/sjson v1.10.0
    github.com/sashabaranov/go-openai v0.20.1       // Embedding

    // 迁移
    github.com/golang-migrate/migrate/v4 v4.17.1

    // WebSocket（内部推送，保留）
    github.com/gorilla/websocket v1.5.3
)
```

### 17.4 完整 config.yaml（SaaS 版）

```yaml
# ==========================================
# Open Agent Hub SaaS 主配置
# ==========================================
system:
  env: local                    # local | dev | staging | prod
  rest_addr: ":8084"            # 管理面 REST + SaaS Console 前端
  mcp_addr: ":8085"             # MCP Gateway（Agent 调用）
  public_base_url: "https://mcp.openagenthub.com"
  session_ttl: 24h              # MCP Session 有效期
  max_sessions_per_workspace: 50

# PostgreSQL
postgres:
  dsn: "postgres://hub:hub@localhost:5432/open_agent_hub?sslmode=disable"
  max_open_conns: 100
  max_idle_conns: 20
  conn_max_lifetime: 1h
  log_level: "warn"             # silent | error | warn | info

# pgvector
vector:
  dim: 1536                     # text-embedding-3-small
  distance: "cosine"            # cosine | l2 | ip
  hnsw_m: 16
  hnsw_ef_construction: 64
  hnsw_ef_search: 40

# Redis
redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  pool_size: 50

# MCP 协议
mcp:
  spec_version: "2025-03-26"    # 固定协议版本
  server_name: "open-agent-hub"
  server_version: "0.1.0"
  transport: "streamable_http"  # 强制 streamable_http
  session:
    store: "redis"
    ttl: 24h
  prompts_cache:
    enabled: true
    etag_strategy: "content_hash"

# OAuth 2.1
oauth:
  issuer: "https://mcp.openagenthub.com"
  authorization_code_ttl: 10m
  access_token_ttl: 1h
  refresh_token_ttl: 30d
  pkce_required: true
  providers:
    - name: "google"
      client_id: "${OAUTH_GOOGLE_CLIENT_ID}"
      client_secret: "${OAUTH_GOOGLE_SECRET}"
      scopes: ["openid", "email", "profile"]
    - name: "github"
      client_id: "${OAUTH_GITHUB_CLIENT_ID}"
      client_secret: "${OAUTH_GITHUB_SECRET}"
      scopes: ["user:email", "read:org"]

# 限流
ratelimit:
  per_tool_per_minute: 60       # 单一 Tool 单分钟调用上限
  per_client_per_minute: 120
  per_workspace_per_minute: 600
  per_workspace_per_day_tool_calls: 10000
  on_exceed: "429_with_retry_after"

# 配额（按 Plan 调整）
quota:
  free:
    workspaces_per_org: 1
    members_per_workspace: 3
    tool_calls_per_month: 5000
    memory_count: 1000
    vector_mb: 100
  pro:
    workspaces_per_org: 10
    members_per_workspace: 20
    tool_calls_per_month: 100000
    memory_count: 50000
    vector_mb: 5000

# 记忆系统
memory:
  write_strategy: "formula_scorer"   # llm_judge | formula_scorer | managed
  global_char_limit: 2200
  per_memory_char_limit: 500
  skill_char_limit: 1375
  curator_idle_days: 7
  snapshot:
    enabled: true
    cron: "0 3 * * *"
  retrieval:
    embedder: "openai"               # openai | azure | cohere | local
    model: "text-embedding-3-small"
    top_k: 10
    hybrid_alpha: 0.7                # 向量权重

# 嵌入模型凭据
embedder:
  openai:
    api_key: "${OPENAI_API_KEY}"
    base_url: "https://api.openai.com/v1"
  azure:
    endpoint: "${AZURE_ENDPOINT}"
    api_key: "${AZURE_API_KEY}"
    deployment: "text-embedding-3-small"

# 审计
audit:
  retention_days: 0                  # 0 = 永久保留
  archive_after_days: 180            # 180 天后归档到冷存储
  append_only: true

# Stripe 计费（P2）
billing:
  enabled: false
  stripe_secret_key: "${STRIPE_SECRET_KEY}"
  webhook_secret: "${STRIPE_WEBHOOK_SECRET}"
  plans: ["free", "pro", "team", "enterprise"]

# Zap 日志
zap:
  level: "info"
  format: "json"                     # console | json
  prefix: "[AGENT-HUB]"
  director: "log"
  show_line: true
  retention_day: 30

# CORS
cors:
  allowed_origins:
    - "https://console.openagenthub.com"
  allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
  allowed_headers: ["Content-Type", "Authorization", "Mcp-Session-Id"]
  allow_credentials: true
```

### 17.5 CLI 启动命令

```bash
# 使用默认 config.yaml
./open-agent-hub

# 指定配置文件
./open-agent-hub -c /etc/open-agent-hub/config.yaml

# 本地开发（优先使用 config.local.yaml）
./open-agent-hub

# 数据库迁移子命令
./open-agent-hub migrate up
./open-agent-hub migrate down 1
./open-agent-hub migrate force 20260607

# 健康检查
curl http://localhost:8084/healthz
curl http://localhost:8085/healthz
```

Viper 配置文件查找顺序：`-c` 标志 → `config.local.yaml` → `config.yaml` → 环境变量覆盖。

### 17.6 统一响应格式（管理面 REST）

管理面 REST 沿用 `{code, data, msg}` 响应包络，便于前端 `request<T>()` 统一处理：

```go
// domain/response/response.go
type Response struct {
    Code int         `json:"code"`
    Data interface{} `json:"data"`
    Msg  string      `json:"msg"`
}

const (
    SUCCESS        = 200
    ERROR          = 500
    ERROR_BAD_REQ  = 400
    ERROR_NO_AUTH  = 401
    ERROR_FORBID   = 403
    ERROR_NOT_FOUND = 404
    ERROR_QUOTA    = 429
)

func OkWithData(data interface{}, c *gin.Context)
func OkWithDetailed(data interface{}, msg string, c *gin.Context)
func FailWithMessage(msg string, c *gin.Context)
func FailWithDetailed(data interface{}, msg string, c *gin.Context)
func NoAuth(msg string, c *gin.Context)
```

**前端对应**：

```typescript
// web/src/api/request.ts
interface ApiResponse<T> {
  code: number;
  data: T;
  msg: string;
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${url}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
  const result: ApiResponse<T> = await response.json();
  if (result.code !== 200) {
    if (result.code === 401) { /* refresh token */ }
    throw new Error(result.msg);
  }
  return result.data;
}
```

### 17.7 全局单例（精简版）

> 仅基础设施（DB / Redis / Vector）使用全局单例；业务 Service 通过构造函数注入依赖。

```go
// global/global.go
var (
    GLB_CONFIG  config.Server
    GLB_DB      *gorm.DB
    GLB_PG      *pgxpool.Pool          // 直连（向量 / RLS）
    GLB_REDIS   *redis.Client
    GLB_VECTOR  *pgvector.Client
    GLB_VP      *viper.Viper
    GLB_LOG     *zap.Logger
    GLB_ASYNQ   *asynq.Client
)
```

### 17.8 前端技术栈

前端技术栈见 Ch 15.1。

### 17.9 Local Bridge 技术栈（P2 独立仓库）

Local Bridge 作为独立 Go 模块，技术栈尽量复用 SaaS 端以减少维护成本：

| 项 | 选型 |
|----|------|
| HTTP 客户端 | `net/http` + `golang.org/x/oauth2`（调用 SaaS） |
| MCP 客户端 | `mcp-go`（连接 SaaS Gateway） |
| 文件监听 | fsnotify |
| 配置 | 本地 YAML |
| 日志 | zap（精简） |
| 安装包 | macOS `.pkg` / Windows `.msi` / Linux `.deb` |
| 守护进程 | macOS launchd / Windows service / Linux systemd |

## 18. 部署方案（SaaS 版）

> Open Agent Hub SaaS 采用 **双进程统一构建 + 多环境差异化部署** 策略：P0 阶段 SaaS 服务端为单进程多 Service（REST 8084 + MCP 8085），P1 阶段拆为两个独立二进制独立部署，P2 阶段支持多区域。

### 18.1 双端口架构

```
┌──────────────────────────────────────────────────────────┐
│               open-agent-hub 进程 (P0)                    │
│                                                          │
│   :8084 REST  ─→ Gin Router                              │
│                 ├─ /api/v1/orgs/...        (SaaS Console) │
│                 ├─ /api/v1/workspaces/...                │
│                 ├─ /api/v1/rules/...                     │
│                 ├─ /api/v1/memories/...                  │
│                 ├─ /api/v1/billing/...                   │
│                 └─ /* (SPA fallback → web/dist)          │
│                                                          │
│   :8085 MCP    ─→ mcp.Server (Streamable HTTP)          │
│                 ├─ POST /mcp      (Initialize + Call)    │
│                 ├─ GET  /mcp      (SSE stream)           │
│                 └─ DELETE /mcp    (session close)        │
└──────────────────────────────────────────────────────────┘
```

P1 拆分为 2 个独立二进制 / Pod：`open-agent-hub-rest`（8084）+ `open-agent-hub-mcp`（8085），共率 `model/` `tenant/` `memory/` 等业务包。

### 18.2 本地开发（Docker Compose）

```yaml
# deploy/docker-compose.dev.yml
version: '3.9'

services:
  postgres:
    image: pgvector/pgvector:pg16
    environment:
      POSTGRES_USER: hub
      POSTGRES_PASSWORD: hub
      POSTGRES_DB: open_agent_hub
    ports: ["5432:5432"]
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./initdb:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U hub"]
      interval: 5s

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s

  migrate:
    build: .
    command: ["migrate", "up"]
    depends_on:
      postgres: { condition: service_healthy }
    environment:
      DATABASE_DSN: "postgres://hub:hub@postgres:5432/open_agent_hub?sslmode=disable"

  backend:
    build:
      context: ..
      dockerfile: deploy/Dockerfile
    command: ["-c", "/etc/open-agent-hub/config.local.yaml"]
    ports:
      - "8084:8084"
      - "8085:8085"
    volumes:
      - ./config.local.yaml:/etc/open-agent-hub/config.local.yaml
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
      migrate:  { condition: service_completed_successfully }

  frontend:
    image: node:20-alpine
    working_dir: /app
    volumes: ["../web:/app"]
    command: sh -c "npm ci && npm run dev -- --host 0.0.0.0"
    ports: ["3000:3000"]
    depends_on: [backend]

volumes:
  pgdata:
```

启动顺序：

```bash
# 1. 启基础设施
docker compose -f deploy/docker-compose.dev.yml up -d postgres redis

# 2. 迁移
docker compose -f deploy/docker-compose.dev.yml run --rm migrate

# 3. 启后端 + 前端 dev server
docker compose -f deploy/docker-compose.dev.yml up backend frontend
```

### 18.3 单二进制部署（P0 推荐）

P0 默认单二进制含前端（go:embed）：

```bash
# 构建前端
cd web && npm ci && npm run build            # 产物到 web/dist/

# 构建后端（嵌入 web/dist）
cd .. && go build -trimpath -ldflags="-s -w" -o bin/open-agent-hub .

# 运行
./bin/open-agent-hub
./bin/open-agent-hub -c /etc/open-agent-hub/config.yaml
```

### 18.4 Dockerfile

```dockerfile
# deploy/Dockerfile
# ---- 阶段 1：构建前端 ----
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- 阶段 2：构建后端 ----
FROM golang:1.22-alpine AS backend
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.version=$(git describe --tags)" \
    -o /out/open-agent-hub .

# ---- 阶段 3：运行时 ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /out/open-agent-hub /usr/local/bin/open-agent-hub
COPY deploy/config.yaml /etc/open-agent-hub/config.yaml
EXPOSE 8084 8085
USER nonroot:nonroot
ENTRYPOINT ["open-agent-hub"]
```

### 18.5 Kubernetes 部署（Helm Chart）

P1+ 生产推荐使用 Helm：

```yaml
# deploy/helm/values.yaml
replicaCount: 3

image:
  repository: openagenthub/open-agent-hub
  tag: "0.1.0"
  pullPolicy: IfNotPresent

service:
  rest:
    type: LoadBalancer
    port: 8084
  mcp:
    type: ClusterIP    # Cluster 内可访问，Agent 在 VPC 内
    port: 8085

ingress:
  enabled: true
  hosts:
    - host: mcp.openagenthub.com
      paths: ["/mcp"]
    - host: console.openagenthub.com
      paths: ["/"]
  tls:
    - hosts: [mcp.openagenthub.com, console.openagenthub.com]
      secretName: open-agent-hub-tls

postgresql:
  enabled: true
  auth:
    username: hub
    database: open_agent_hub
  primary:
    persistence:
      size: 500Gi
    resources:
      requests: { cpu: "2", memory: 8Gi }
      limits:   { cpu: "8", memory: 32Gi }
  # 启动时需手动执行 pgvector 扩展
  initdbScripts:
    00-extensions.sql: |
      CREATE EXTENSION IF NOT EXISTS vector;
      CREATE EXTENSION IF NOT EXISTS pgcrypto;

redis:
  enabled: true
  architecture: replication
  master:
    persistence: { size: 50Gi }
  replica:
    replicaCount: 2

resources:
  requests: { cpu: "1", memory: 2Gi }
  limits:   { cpu: "4", memory: 8Gi }

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 20
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 75

podDisruptionBudget:
  enabled: true
  minAvailable: 2

nodeSelector: {}
tolerations: []
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels: { app.kubernetes.io/name: open-agent-hub }
          topologyKey: kubernetes.io/hostname
```

部署命令：

```bash
helm repo add openagenthub https://charts.openagenthub.com
helm install hub openagenthub/open-agent-hub \
  -n open-agent-hub --create-namespace \
  -f deploy/helm/values.prod.yaml
```

### 18.6 环境配置矩阵

| 环境 | 部署形态 | 数据库 | Redis | MCP 地址 |
|------|---------|--------|-------|----------|
| **local** | docker-compose.dev | pgvector:pg16 (容器) | redis:7-alpine (容器) | `http://localhost:8085/mcp` |
| **dev** | k8s dev cluster | PG 单实例 + pgvector | Redis 单实例 | `https://mcp-dev.openagenthub.com/mcp` |
| **staging** | k8s staging | PG 主从 | Redis 主从 | `https://mcp-staging.openagenthub.com/mcp` |
| **prod** | k8s prod (Helm) | PG 主从 + 备份 + 监控 | Redis Sentinel / Cluster | `https://mcp.openagenthub.com/mcp` |

### 18.7 Local Bridge 安装与升级（P2）

Local Bridge 独立打包为 3 个平台安装包：

| 平台 | 包格式 | 自启动 | 升级机制 |
|------|-------|--------|---------|
| macOS | `.pkg` | launchd plist | 内置 updater 检查新版本 |
| Windows | `.msi` | Windows Service | Squirrel.Windows 增量更新 |
| Linux | `.deb` / `.rpm` | systemd unit | apt / yum repo |

```bash
# macOS
sudo installer -pkg open-agent-bridge-0.1.0.pkg -target /

# Windows
msiexec /i open-agent-bridge-0.1.0.msi

# Linux
sudo dpkg -i open-agent-bridge_0.1.0_amd64.deb
sudo systemctl enable --now open-agent-bridge
```

Local Bridge 首次启动后需用户登录 SaaS 账号并选择 Workspace 进行绑定（与 Workspace MCP Token 配对）。

### 18.8 多区域部署占位（P2）

| 区域 | 主要客户 | 数据主权 | 备注 |
|------|---------|---------|------|
| `us-east-1` | 北美 | 美国 | 主区域 |
| `eu-west-1` | 欧洲 | 欧盟 (GDPR) | 独立 PG 集群 |
| `ap-southeast-1` | 亚太 | 新加坡 | 后期启用 |

区域间通过 **只读 Event Bus**（如 Kafka MirrorMaker）同步审计日志与匿名指标，不同步业务数据。

### 18.9 监控与告警

| 指标 | 工具 | 告警阈值 |
|------|------|---------|
| 业务 SLO | Prometheus + Grafana | P99 错误率 > 1% 持续 5min |
| MCP 请求耗时 | OpenTelemetry → Jaeger | P99 > SLO × 1.5 |
| 数据库连接池 | pgx stats | 使用率 > 80% |
| 向量检索耗时 | prom histogram | P99 > 500ms |
| 配额使用 | usage_records | 月用量 > 80% 触发用户通知 |
| 多租户隔离 | RLS policy violation | 任何 violation 立即 page |

### 18.10 备份与恢复

- **PG 物理备份**：每日全量 + 每 15min WAL 归档（pgBackRest）
- **审计日志归档**：180 天后归档到 S3 / OSS 冷存储
- **Embedding 模型版本**：保留 2 个旧版本以便降级回滚
- **RPO**：15 分钟；**RTO**：1 小时

## 19. 开发阶段规划（重写）

> Open Agent Hub 采用 **18 周（4.5 月）** 分为三阶段交付：P0 闭环 / P1 增强 / P2 商业化。P0 不交付 Local Bridge，P1 交付能力补齐，P2 交付企业级与商业化能力。阶段交付后进入“质量打磨 + 招客户”阶段。

### 19.0 总体时间表

```
Week  1  2  3  4  5  6  7  8  9 10 11 12 13 14 15 16 17 18
      ├──P0 MVP 闭环──┤├─────P1 增强能力──────┤├────P2 商业化+集成────┤
      
关键里程碑：
  W6   内测客户 10 个 Workspace 启动
  W12  GA（公开可用）
  W18  商业化上线（Stripe + 多区域）
```

### 19.1 P0：SaaS + MCP 核心闭环（Week 1–6）

**目标**：让一个 Workspace 能用 MCP 与 Agent 互联，手动管理 Rules + Memory，调用外部 Tool 有记录。

**交付清单**：

| # | 交付项 | 验收标准 |
|---|--------|---------|
| 1 | **用户注册 / 登录**（Email + OAuth Google/GitHub） | 完成后进入 Organization，默认创建首个 Workspace |
| 2 | **Organization / Workspace / Member 管理** | Owner 可邀请成员、分配角色（owner / admin / member / viewer） |
| 3 | **MCP Token 签发与撤销** | 页面生成 Token + 一次性显示完整值；可轮换 |
| 4 | **MCP Server 接入** | Cursor / Claude Code 能 `claude mcp add ...` 接入不报错 |
| 5 | **Global Rules CRUD** | Rule 按 `scope=workspace` 创建、编辑、版本化（ETag） |
| 6 | **Project Rules CRUD** | Rule 按 `scope=project` 覆盖 workspace 规则 |
| 7 | **Memory 基础** | Semantic / Preference / Episodic / Skill 4 类 CRUD + 写入纪律（公式打分默认） |
| 8 | **5 个核心 Tools** | `hub.get_global_rules` / `hub.get_project_context` / `hub.search_memory` / `hub.propose_memory` / `hub.save_memory` |
| 9 | **Tool 调用日志** | `tool_invocation_logs` 表 + `/api/v1/usage/logs` 查询页 |
| 10 | **基础权限控制** | casbin RBAC + workspace_id 强制过滤 |
| 11 | **SaaS Console 骨架** | 4 Hub 布局、登录、Organization / Workspace 设置页 |
| 12 | **本地 Docker Compose 开发环境** | `docker compose up` 启 PG + Redis + 后端 |

**关键验收**：
- 端到端测试：Cursor 调用 `hub.search_memory` 返回真实记忆
- 多租户隔离：Workspace A 的 token 不能读 Workspace B 的任何数据
- P99：`hub.get_global_rules` < 50ms

### 19.2 P1：增强能力（Week 7–12）

**目标**：补齐 Skill、外部 MCP 聚合、向量检索、计量计费能力，达到 **GA 质量**。

**交付清单**：

| # | 交付项 | 验收标准 |
|---|--------|---------|
| 1 | **Skill 管理 + Prompt 模板** | Skill 三状态机（active / stale / archived）、5 个预置 Prompts |
| 2 | **外部 MCP Server 聚合** | Owner 可注册 GitHub / Slack / Jira 等上游 MCP Server；Tool 名称空间化为 `hub.<server>.<tool>` |
| 3 | **Tool Policy 校验** | `requires_confirmation` / `max_calls_per_day` 生效；高危 Tool 走二次确认 |
| 4 | **向量检索 + 混合召回** | pgvector HNSW + BM25；`hub.search_memory` 走混合召回，准确率 > 85% |
| 5 | **Embedding 管道** | `propose_memory` 自动 embed、存储到 `vector(1536)` |
| 6 | **用量统计 + 配额限流** | `usage_records` + Redis 令牌桶；超额返回 429 + Retry-After |
| 7 | **Prompt Cache Snapshot** | `hub://workspace/{ws}/snapshot` Resource + ETag 协商 |
| 8 | **Agent Client 识别** | MCP `clientInfo` 解析为 `AgentClient` 实体；“匿名”调用转 `signed-in` |
| 9 | **Project 自动识别** | 从 `clientInfo.workspace` 或文件路径后缀推断 Project |
| 10 | **Audit Logs 查询页** | 关键动作可追溯、可导出 |
| 11 | **性能 SLO 监控** | Prometheus + Grafana Dashboard + 告警规则 |
| 12 | **多环境部署** | dev / staging / prod Helm Chart |
| 13 | **完整 SaaS Console 页面** | Memory Explorer / Skill Manager / Memory Timeline / Snapshot Versions / Vector Index Status |

**关键验收**：
- 1 万 Workspace × 50 Session 平均负载下 P99 SLO 达标
- 压测报告（k6/vegeta）记录到 `docs/perf/p1-baseline.md`
- 10 个内测客户完成迁移并签署反馈表

### 19.3 P2：本地集成 + 商业化（Week 13–18）

**目标**：交付 Local Bridge（高级能力）、计费、多区域，支持独立 SaaS 商业化运营。

**交付清单**：

| # | 交付项 | 验收标准 |
|---|--------|---------|
| 1 | **Local Bridge 独立二进制** | Cursor / Claude Code / OpenCode / Copilot / Windsurf 5 个 Adapter 全部可用 |
| 2 | **双向同步 + 高级冲突解决** | Push + Pull + 三路合并策略；冲突 Resolver UI |
| 3 | **团队配置模板** | Workspace 可一键套用模板（前端项目 / 后端项目 / ML 项目 等） |
| 4 | **Quota 仪表盘 + 预警** | 达到 80% 用量邮件 + 站内信提醒 |
| 5 | **Stripe 计费集成** | Free / Pro / Team / Enterprise 4 个 Plan；Webhook 同步订阅状态 |
| 6 | **计费页 + 发票** | SaaS Console Billing 页面，可下载发票（PDF） |
| 7 | **多区域部署** | `us-east-1` + `eu-west-1` 两个区域；区域间事件同步 |
| 8 | **审计日志查询扩展** | 合规导出（CSV / JSONL）、SIEM 集成 |
| 9 | **企业 SSO** | SAML / OIDC（SAML 2.0 为主流企业客户需） |
| 10 | **公开 OpenAPI 文档** | 管理面 REST API 公开文档（Swagger UI / Redoc） |
| 11 | **官方网站 + 文档站** | 营销页 + 集成指南 + 最佳实践 |
| 12 | **公开 MCP 资源 / Prompts 仓库** | Community 可贡献预置 Prompt / Resource 模板 |

**关键验收**：
- 商业化收入 0→1 验证：3 个付费客户 / $10k MRR
- 4 区域全量可用
- 1个公开 sample 项目接入示例

### 19.4 里程碑与入场 / 出场标准

| 阶段 | 入场标准 | 出场标准 |
|------|---------|---------|
| **P0** | 架构、权限、协议确定 | 10 个内测 Workspace 启动，且 1 周内未反馈阻断性 Bug |
| **P1** | P0 闭环 | SLO 达标 + 10 个内测客户反馈为 “可上线” + Helm GA |
| **P2** | P1 达标 | $10k MRR + 4 区域全量 + 企业 SSO 可用 |

### 19.5 资源需求估算

| 阶段 | 后端 | 前端 | DevOps / SRE | 产品 / 设计 |
|------|------|------|-------------|------------|
| P0 | 2 | 1 | 0.5 | 0.5 |
| P1 | 2 | 1 | 1 | 0.5 |
| P2 | 2 | 1 | 1 | 1 |

**总计**：3.5 名前后端 + 1.5 名 DevOps / 产品，共 5 人 × 18 周。

### 19.6 关键风险与提前准备

| 风险 | 提前准备动作 |
|------|------------|
| MCP 协议官方 SDK 未达 P0 预期 | P0 第 1 周评估；同时维护 TypeScript 代理层以备退路 |
| pgvector 在 1000 万记忆下性能不达标 | P1 第 8 周压测；P2 预留 Milvus 切换点 |
| 5 个 Adapter 在 Cursor 版本升级后变动 | Local Bridge Adapter 隔离仓；L1-L3 退出 48h 热修复 |
| 企业客户要求本地化部署 | P2 预留 Helm on-premise 安装包 + License server |

## 23. WebSocket Hub 实现（兼容保留）

> 参照 opentoken-server 的 NodeHub 模式，实现 WebSocket 事件推送。

### 15.1 WSHub 核心结构

```go
type WSHub struct {
    clients    map[*WSClient]bool
    broadcast  chan []byte
    register   chan *WSClient
    unregister chan *WSClient
    mu         sync.RWMutex
}

type WSClient struct {
    hub  *WSHub
    conn *websocket.Conn
    send chan []byte
}

func NewWSHub() *WSHub
func (h *WSHub) Run()
func (h *WSHub) BroadcastEvent(eventType string, data interface{})
```

### 15.2 事件推送

```go
type WSEvent struct {
    Type string      `json:"type"`
    Data interface{} `json:"data"`
    Time time.Time   `json:"time"`
}

func (h *WSHub) BroadcastEvent(eventType string, data interface{}) {
    event := WSEvent{
        Type: eventType,
        Data: data,
        Time: time.Now(),
    }
    payload, _ := json.Marshal(event)
    h.broadcast <- payload
}
```

### 15.3 WebSocket Handler

```go
var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin:     func(r *http.Request) bool { return true },
}

func EventsWebSocket(c *gin.Context) {
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }
    client := &WSClient{
        hub:  global.GLB_WS_HUB,
        conn: conn,
        send: make(chan []byte, 256),
    }
    client.hub.register <- client

    go client.writePump()
    go client.readPump()
}
```

### 15.4 在同步引擎中推送事件

```go
// Push 同步完成后
global.GLB_WS_HUB.BroadcastEvent("sync:complete", map[string]interface{}{
    "config_id": configID,
    "results":   results,
})

// 检测到冲突时
global.GLB_WS_HUB.BroadcastEvent("conflict:detected", conflict)

// 记忆写入后
global.GLB_WS_HUB.BroadcastEvent("memory:created", memory)
```

## 20. 安全考虑

> 本章覆盖 SaaS 多租户、OAuth 2.1、MCP Tool 调用路径上的所有安全考虑。设计原则：**默认拒绝、最小权限、可审计**。

### 20.1 多租户隔离（强制）

**每次 MCP Tool 调用必须携带 WorkspaceContext**（OrgID + WorkspaceID + ProjectID + UserID + AgentClientID），从 `mcpsession` Token 中解析后注入 `gin.Context` / 业务 Context。

```go
// mcp/middleware/tenant.go
func TenantContext(required bool) gin.HandlerFunc {
    return func(c *gin.Context) {
        wsID := c.GetString("workspace_id")
        if wsID == "" {
            if required {
                response.FailWithMessage("missing workspace context", c)
                c.Abort()
                return
            }
        }
        c.Set("workspace_id", wsID)
        c.Set("project_id", c.GetString("project_id"))
        c.Set("user_id", c.GetString("user_id"))
        c.Next()
    }
}
```

**三重防护**：

1. **GORM 插件自动注入 `WHERE workspace_id = ?`**（见 9.6）
2. **PostgreSQL RLS 兏底**（见 9.5）：`USING (workspace_id = current_setting('app.workspace_id')::uuid)`
3. **仓库层 `WithWorkspace(wsID)` 依赖传递**：业务代码必须显式调用，禁止裸 `db.Find()`

违反任何一层返回 `403 workspace_mismatch`。

### 20.2 Token 与凭据安全

| 凭据类型 | 存储 | 生效期 | 轮换 | 传输 |
|----------|------|--------|------|------|
| OAuth Access Token | 仅存 Hash（SHA-256） | 1 小时 | 静默 Refresh | TLS 1.3 |
| OAuth Refresh Token | 仅存 Hash | 30 天 | 用户主动重新授权 | TLS 1.3 |
| MCP Server Token | Hash 存 `api_keys` | 可设 | Owner 主动轮换 | TLS 1.3 |
| 上游 MCP Server OAuth | 加密存 `connected_mcp_servers.auth` | 跟随上游 | 后台异步轮换 | 内部 RPC |

**明文 Token 一次性返回**：创建 Token API 返回完整明文，后续不可再获取。

```go
// token/hash.go
func HashToken(plain string) string {
    sum := sha256.Sum256([]byte(plain))
    return hex.EncodeToString(sum[:])
}

func CompareToken(plain, hash string) bool {
    return subtle.ConstantTimeCompare([]byte(HashToken(plain)), []byte(hash)) == 1
}
```

### 20.3 Prompt Injection 防御

**Tool 分级制度**（与第 4.4 节联动）：

| 级别 | Tool 示例 | 限制 |
|------|----------|------|
| L0 读类 | `hub.search_memory` / `hub.get_global_rules` | 无需确认 |
| L1 写类 | `hub.save_memory` / `hub.update_rule` | 限流 |
| L2 调用外部 MCP | `hub.invoke_connected_tool` | Tool Policy 检查 + 限流 |
| L3 高危（发邮件、付费、删库） | `github.create_pull_request` / `stripe.create_charge` | `requires_confirmation: true` 二次确认 + 输出脱敏 |

**二次确认流程**：

```
Agent 调用高危 Tool
  ↓
Gateway 检测 requires_confirmation
  ↓
返回 CallToolResult{Status: "pending_confirmation", ConfirmationID: xxx}
  ↓
Agent 必须在 Tool 输入里重复加 `confirmation_id` 重调
  ↓
Gateway 验证后执行
```

**输出脱敏**：所有 Tool 返回内容走 `output_sanitizer.go`，自动抩截 `sk-` / `ghp_` / `Bearer ` / AWS Access Key 等模式，默认替换为 `***`。

### 20.4 记忆污染防御（三段式）

```
propose_memory → 自动 Embedding + 初步打分（公式）→ review_queue → 人工 / 高质量 Agent 复核 → save_memory
```

- **不允许 Agent 直接调 `save_memory`**；必须经 `propose_memory` 走审核
- **公式打分维度**：与已有记忆的语义距离、嵌入向量、关键词重叠、Pinned 状态、Provenance
- **重复阈值**：余弦相似度 > 0.95 → 拒绝写入并提示 superseded
- **低质量打分**（< 0.3）→ 入 review_queue，等后台 Curator 复审

### 20.5 审计日志不可篡改

```go
// audit/log.go
type AuditLog struct {
    ID          string    `gorm:"primarykey"`
    WorkspaceID string    `gorm:"not null;index"`
    Actor       string    `gorm:"not null"`     // user_id / agent_client_id / system
    Action      string    `gorm:"not null"`     // hub.save_memory / config.update / billing.refund
    Target      string    `gorm:"not null"`
    Payload     string    `gorm:"type:jsonb"`   // 上下文与请求参数
    IP          string
    UserAgent   string
    CreatedAt   time.Time `gorm:"index"`
    PrevHash    string    `gorm:"type:varchar(64)"`  // 上一条 hash
    Hash        string    `gorm:"type:varchar(64);not null"`  // SHA-256(本条内容 + PrevHash)
}
```

- **仅 append**：无 UPDATE / DELETE 权限，DB 账号层限制
- **链式 Hash**：`Hash = SHA-256(WorkspaceID + Actor + Action + Target + Payload + CreatedAt + PrevHash)`；每条记录包含上一条 hash，形成不可篡改链表
- **每日 hash 快照**写入独立审计库，供合规对账
- **导出为 CSV / JSONL**（SIEM 集成）

### 20.6 上下游身份验证

| 调用方向 | 验证方式 | 实现位置 |
|----------|----------|----------|
| Agent → Hub | OAuth 2.1 + PKCE | `gateway/auth/oauth.go` |
| Hub → 上游 MCP Server | 上游要求的 OAuth / API Key | `tool/connectors/oauth_proxy.go` |
| Web Console → REST | Session Cookie + CSRF Token | `initialize.RegisterFrontendStaticRoutes` |
| Local Bridge → Hub | Workspace MCP Token | `local-bridge/auth/bridge_token.go` |

### 20.7 速率限制与告警

- **限流维度**：按 workspace + tool + agent_client 三级令牌桶
- **异常调用检测**：同一 IP 1 分钟内 > 100 次失败 → 临时封禁 10 分钟
- **告警通知**：关键告警推送到 SaaS Console 顶部 Alert Bar + 邮件

---

## 21. 错误处理与测试

### 21.1 上游 MCP 熔断器（Sony gobreaker）

```go
// tool/connectors/breaker.go
var breakerSettings = gobreaker.Settings{
    Name:        "upstream-mcp",
    MaxRequests: 5,
    Interval:    60 * time.Second,
    Timeout:     30 * time.Second,
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        return counts.ConsecutiveFailures > 5
    },
    OnStateChange: func(name string, from, to gobreaker.State) {
        log.Warn("circuit breaker state change",
            zap.String("name", name),
            zap.String("from", from.String()),
            zap.String("to", to.String()))
    },
}
```

- 熔断后返回 `MCP_ERROR -32009`（自定义：upstream_unavailable）
- 状态轮转：Closed → Open（连续 5 次失败）→ Half-Open（30s 后探活 1 次）→ Closed

### 21.2 向量检索降级

```go
// memory/recall/engine.go
func (e *Engine) Search(ctx context.Context, q string, k int) ([]Memory, error) {
    ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
    defer cancel()

    type result struct {
        mems []Memory
        err  error
    }
    ch := make(chan result, 1)
    go func() {
        mems, err := e.vectorSearch(ctx, q, k)
        ch <- result{mems, err}
    }()

    select {
    case r := <-ch:
        return r.mems, r.err
    case <-ctx.Done():
        log.Warn("vector search timeout, falling back to BM25")
        return e.bm25Search(ctx, q, k)  // 关键词检索
    }
}
```

- **三级降级**：向量检索 → BM25 关键词 → 提示用户改写查询
- **降级命中记入 SLO**：`hub.search_memory.fallback.count` Prometheus 指标

### 21.3 多租户隔离测试（pgTAP RLS）

```sql
-- migrations/0003_rls_test.sql
BEGIN;
SELECT plan(5);

-- 准备两个 workspace
INSERT INTO workspaces (id, name, org_id) VALUES
    ('11111111-1111-1111-1111-111111111111', 'WS-A', 'org-1'),
    ('22222222-2222-2222-2222-222222222222', 'WS-B', 'org-1');

SET app.workspace_id = '11111111-1111-1111-1111-111111111111';
SELECT is(
    (SELECT count(*) FROM memories WHERE workspace_id = '22222222-2222-2222-2222-222222222222'),
    0::bigint,
    'RLS blocks cross-workspace read'
);

SELECT is(
    (SELECT count(*) FROM memories),
    0::bigint,
    'RLS shows only own workspace data'
);

SELECT * FROM finish();
ROLLBACK;
```

**CI 必跑**：合并主干前必须全绿。

### 21.4 MCP 协议契约测试

```go
// tests/contract/mcp_test.go
func TestMCPContract(t *testing.T) {
    schema := jsonschema.MustCompile("schemas/mcp-tools.schema.json")
    for _, tool := range AllTools() {
        t.Run(tool.Name, func(t *testing.T) {
            // 必填字段校验
            require.NotEmpty(t, tool.Name)
            require.NotEmpty(t, tool.Description)
            require.NotNil(t, tool.InputSchema)
            require.NotNil(t, tool.OutputSchema)

            // JSON Schema 语法
            require.NoError(t, schema.Validate(tool.InputSchema))

            // 幂等性标注
            require.Contains(t, []string{"idempotent", "non_idempotent"}, tool.Idempotency)

            // 错误码是标准 JSON-RPC 之一
            for _, code := range tool.ErrorCodes {
                require.True(t, isStandardJSONRPCCode(code))
            }
        })
    }
}
```

- 所有 Tool 定义走 `pkg/jsonschema` 静态校验
- 变更 Tool 必须同步更新 `schemas/mcp-tools.schema.json` 与 `docs/spec.md` 第 6 章
- PR 检查：tool diff  →  spec diff  →  schema diff，三者必须同时存在

### 21.5 性能压测（k6 / vegeta）

**目标场景**：

| 场景 | 并发 | 持续 | 验收 |
|------|------|------|------|
| P0 Smoke | 100 MCP Session | 5 min | `hub.get_global_rules` P99 < 50ms、错误率 < 0.1% |
| P1 Baseline | 1k MCP Session | 30 min | `hub.search_memory` P99 < 200ms、QPS > 500 |
| P2 Stress | 10k MCP Session | 2 hour | `hub.invoke_connected_tool` P99 < 2s、错误率 < 1% |
| 噪声邻居 | 1 个 “吵闹” tenant + 99 个安静 tenant | 30 min | 安静 tenant P99 不被拖彽 > 20% |

**压测脚本示例**：

```js
// tests/perf/search-memory.js
import http from 'k6/http';
import { check } from 'k6';

export const options = {
    stages: [
        { duration: '5m', target: 100 },
        { duration: '30m', target: 1000 },
        { duration: '5m', target: 0 },
    ],
    thresholds: {
        http_req_duration: ['p(99)<200'],
        http_req_failed: ['rate<0.001'],
    },
};

export default function () {
    const res = http.post(`${__ENV.BASE_URL}/mcp`, JSON.stringify({
        jsonrpc: '2.0',
        method: 'tools/call',
        params: {
            name: 'hub.search_memory',
            arguments: { query: 'authentication', k: 10 },
        },
        id: 1,
    }), { headers: { 'Authorization': `Bearer ${__ENV.MCP_TOKEN}` } });

    check(res, {
        'status is 200': (r) => r.status === 200,
        'has results': (r) => JSON.parse(r.body).result !== null,
    });
}
```

### 21.6 CI / CD 门禁

| 检查 | 必跳项 | 实现 |
|------|--------|------|
| `go test ./...` | 0 fail | GitHub Actions |
| `pgTAP RLS 测试` | 0 fail | Docker 启动 PG → 运行 SQL |
| `golangci-lint run` | 0 issue | 本地 + CI |
| `go vet ./...` | 0 warning | CI |
| `MCP 契约测试` | 0 fail | 自研 runner |
| `OpenAPI schema diff` | 无 breaking change | `oasdiff` |
| `k6 P0 smoke` | P99 SLO | 预发环境 |

**合并主干门禁**：上述任一失败，PR 不可合。

### 21.7 错误响应统一规范

**MCP 调用错误码**（与第 5.6 节同步）：

| Code | 名称 | 含义 |
|------|------|------|
| -32700 | Parse error | JSON 解析失败 |
| -32600 | Invalid Request | 请求格式不合法 |
| -32601 | Method not found | Tool 不存在 |
| -32602 | Invalid params | 参数校验失败 |
| -32603 | Internal error | 内部错误 |
| -32001 | Unauthorized | 未认证 / Token 无效 |
| -32002 | Forbidden | 权限不足 / workspace_mismatch |
| -32003 | Not Found | 资源不存在 |
| -32004 | Rate Limited | 限流；返回 Retry-After |
| -32005 | Quota Exceeded | 配额超限；提示升级 Plan |
| -32006 | Tool Policy Denied | 违反 Tool Policy |
| -32007 | Confirmation Required | 需二次确认 |
| -32009 | Upstream Unavailable | 上游 MCP 熔断 |
| -32010 | Upstream Timeout | 上游 MCP 超时 |

**REST 管理面错误**（与 Ch 17 响应格式同步）：`{ code: 0, message: "", data: any }`；非 0 表示业务错误，附 `error_code` 供客户端路由。

## 附录 C. 旧版 ToC 工具迁移指南

> 本附录帮助 v0.1 本地单进程工具用户迁移到 v0.2 SaaS 平台。

### C.1 SaaS MCP vs Local Bridge 能力对比

| 能力 | v0.1（本地）| v0.2 SaaS MCP | v0.2 Local Bridge |
|------|------------|---------------|------------------|
| 配置集中管理 | 本地 JSON/SQLite | SaaS Console | SaaS Console 镜像 |
| 跨 Agent 记忆共享 | ✗ | ✔ MCP Tool | ✔ 文件同步 |
| 跨设备同步 | ✗ | ✔ | ✔ |
| 权限治理 | 本地 API Key | Workspace RBAC | 沿用 SaaS 权限 |
| 用量监控 | ✗ | ✔ Dashboard | ✔ |
| 外部工具统一代理 | ✗ | ✔ 上游 MCP | 取决于 SaaS 连接 |
| 离线工作 | ✔ | ✗ | ✔ |
| 安装复杂度 | 需下载二进制 | 配置 MCP URL | 安装 Bridge + 配 URL |

### C.2 旧版本地数据如何迁移到 SaaS

```
1. 在 SaaS Console 创建 Workspace（个人 / 团队）
2. 在本地运行 open-agent-hub migrate export
   → 生成 open-agent-hub-migration-2026-06-07.zip
     - global_configs.json
     - memories.json
     - agent_mappings.json
3. 在 SaaS Console 选择「Import from Local」
   → 上传 zip → 预览映射 → 确认导入
4. 转换规则:
   - user_id  → default Workspace 中的 legacy_user_id
   - GlobalConfig → Rule (scope='workspace')
   - Memory → Memory (新增 Embedding 字段，后台异步生成)
   - AgentInstance → AgentClient
   - AgentMapping → MemoryMapping（仅 Local Bridge 启用后需要）
```

### C.3 旧 Adapter 用户升级到 Local Bridge 的步骤

**情况 A：仅使用 SaaS 模式**（推荐大多数用户）

1. 在 Cursor / Claude Code / Windsurf 中配置 MCP Server URL：`https://mcp.openagenthub.com/{workspace_id}`
2. 卸载本地 open-agent-hub 二进制
3. 完成

**情况 B：需要继续同步本地文件**（高级用户）

1. 在 SaaS Console 启用 Local Bridge（设置 → 集成 → Local Bridge）
2. 下载 Local Bridge 二进制：macOS / Windows / Linux
3. 运行：`open-agent-hub-bridge init --workspace <ws_id> --token <mcp_token>`
4. Bridge 作为系统服务后台运行，自动同步文件
5. SaaS Console 显示 Bridge 状态（在线 / 离线 / 同步中）

### C.4 保留兼容性 API 列表

> v0.2 保留以下 v0.1 兼容 API，仅供 Local Bridge 插件使用。不建议新代码调用。

| 兼容 API | 路径 | 状态 | 替代 |
|----------|------|------|------|
| ListConfigs | `GET /api/v1/configs` | 可用 | hub.get_global_rules（Tool） |
| GetConfig | `GET /api/v1/configs/:id` | 可用 | hub.get_global_rules（Tool） |
| ListMemories | `GET /api/v1/memories` | 可用 | hub.search_memory（Tool） |
| GetMemory | `GET /api/v1/memories/:id` | 可用 | hub.get_memory（Tool） |
| ListSkills | `GET /api/v1/skills` | 可用 | hub.list_skills（Tool） |
| PushSync | `POST /api/v1/sync/push` | **废弃** | 直接走 MCP Gateway |
| PullSync | `POST /api/v1/sync/pull` | **废弃** | 直接走 MCP Gateway |
| DiscoverAgents | `POST /api/v1/agents/discover` | **废弃** | Agent Client 自动注册 |

> 废弃原因：SaaS 模式下 Agent 发现与同步由 MCP 连接自动处理，不需要手动 trigger。

---

## 附录 D. 术语对照表

> 本附录列出 v0.1 → v0.2 关键术语映射，供阅读历史文档、迁移旧数据时参考。

### D.1 概念层术语

| v0.1 旧术语 | v0.2 新术语 | 变化说明 |
|-------------|------------|----------|
| 单机部署 | SaaS 多租户 | 从本地进程到云端服务 |
| user_id | workspace_id | 租户隔离边界从用户变为工作空间 |
| AgentInstance | AgentClient | 强调「客户端」而非「实例」，并增加 `ClientType` 枚举 |
| GlobalConfig | Rule | 升级为支持 workspace/project/agent 三级 scope |
| 配置同步（SyncHub） | MCP Gateway | 从文件同步转为协议服务 |
| 本地数据库（SQLite/MySQL） | PostgreSQL + pgvector | 增加向量检索能力 |
| 单 user API Key | Workspace MCP Token | token 与 workspace 绑定 |

### D.2 数据模型层术语

| v0.1 字段 / 表 | v0.2 字段 / 表 | 变化说明 |
|----------------|----------------|----------|
| `GlobalConfig.Scope` = `global` | `Rule.Scope` = `workspace` | 默认 scope 升级 |
| `GlobalConfig.UserID` | `Rule.OrgID` + `Rule.WorkspaceID` + `Rule.ProjectID` | 二级租户隔离 |
| `AgentInstance.Status` | `AgentClient.Status` | 重命名 |
| `Memory.UserID` | `Memory.OrgID` + `Memory.WorkspaceID` + `Memory.ProjectID` | 二级租户隔离 |
| `Memory.Type=preference` | `Memory.Type=user_preference` | 重命名 |
| 无 | `Memory.Embedding vector(1536)` | v0.2 新增 |
| `SyncRecord` | `ToolInvocationLog` | 重命名并扩字段 |
| `Conflict` | `ToolInvocationLog.ErrorCode` / `AuditLog` | 冲突单独表被废弃 |
| `SyncRecord.Direction` = `pull` | **删除** | SaaS 化后无本地 pull 场景 |
| `MemoryMapping` | **保留**（仅 Local Bridge 使用）| 主 SaaS 流程不依赖 |

### D.3 协议层术语

| v0.1 概念 | v0.2 概念 | 变化说明 |
|----------|----------|----------|
| REST API（管理面）| REST API（管理面）+ MCP（数据面）| 双端口：8084 + 8085 |
| WebSocket（推送）| MCP Streamable HTTP + WebSocket | MCP 成为主流 |
| JWT Token | OAuth 2.1 + PKCE | 升级为业界标准 |
| API Key（单 user）| APIKey / OAuthToken / PAT / Workspace MCP Token | 四种 Token 类型 |

### D.4 角色层术语

| v0.1 角色 | v0.2 角色 | 变化说明 |
|----------|----------|----------|
| 终端用户 | Org Owner / Workspace Admin / Member / Viewer | RBAC 4 角色 |
| 同步引擎 | MCP Gateway | 协议层不同 |
| Adapter | Local Bridge Adapter | 仍保留但降级为附录 |

### D.5 模块包路径映射

| v0.1 包路径 | v0.2 包路径 | 变化 |
|------------|------------|------|
| `core/sync_hub.go` | `gateway/gateway.go` + `mcp/server.go` | 重命名 + 拆分 |
| `adapter/cursor/` | `local-bridge/adapter/cursor/` | 移到子模块 |
| `sync/transformer.go` | `local-bridge/sync/transformer.go` | 移到子模块 |
| `memory/` | `memory/`（保留）| 升级为多租户 + 向量 |
| `core/ws_hub.go` | `gateway/sse.go` + `realtime/ws_hub.go` | 拆分 |
| `model/global_config.go` | `model/rule.go` | 重命名 |
| `model/agent_instance.go` | `model/agent_client.go` | 重命名 |
| `model/sync_record.go` | `model/tool_invocation_log.go` | 重命名 |
| 无 | `model/organization.go` | v0.2 新增 |
| 无 | `model/workspace.go` | v0.2 新增 |
| 无 | `model/workspace_member.go` | v0.2 新增 |
| 无 | `model/mcp_session.go` | v0.2 新增 |
| 无 | `model/connected_mcp_server.go` | v0.2 新增 |
| 无 | `model/tool_policy.go` | v0.2 新增 |
| 无 | `model/api_key.go` | v0.2 新增 |
| 无 | `model/oauth_token.go` | v0.2 新增 |
| 无 | `model/usage_record.go` | v0.2 新增 |
| 无 | `model/audit_log.go` | v0.2 新增 |

---

## 附录 E. 实现现状与差距分析（Gap Analysis）

> **审计日期：** 2026-06-10　**审计范围：** `backend/` 当前代码 vs 本规格文档
> **结论：** 数据模型层（§8）与 MCP Tool 注册（§6）骨架基本到位，但本规格承诺的多项**控制/安全/检索能力只定义了字段或模型、缺少执行逻辑**。下表按"是否兑现 spec"分级。
>
> 状态图例：✅ 已实现　🟡 部分实现 / 名不副实　❌ 定义了但未接线（dead spec）

### E.1 差距总览

| # | 能力（对应 spec 章节） | 状态 | 现状与差距 |
|---|----------------------|:---:|------------|
| 1 | **配置解析/合并引擎**（§6.2、§12.2） | ✅ | **已于 2026-06-10 实现**（`internal/services/config_resolver.go`）：`MergeRules`/`ResolveEffectiveRules` 实现 `global→project→agent` 三级覆盖（specificity：agent=2 > project=1 > workspace=0），按 (Type,Name) 去重，胜出规则 tie-break 为 version→updated_at→id。`get_global_rules`/`get_project_rules`/`get_project_context` 及 REST `GetGlobalRules`/`GetProjectRules` 均已接入，新增 `agent_name` 维度（缺省回退 client_type）。覆盖纯函数单测 9 例。 |
| 2 | **向量召回引擎**（§9.9、§10.10、§9 标题"pgvector"） | 🟡 | **2026-06-10 修复致命 bug**：旧分词只保留 `a-z0-9`、中文相似度恒为 0；现引入 CJK bigram 分词器（`tokenize`），检索改用非对称覆盖率打分 `relevance`、去重保留对称 `similarity`（`internal/mcp/tools.go` + `similarity_test.go`），**中英文检索均可用**。**仍为词法检索**：`Memory` 无 embedding 字段、未接向量库；真正的 embedding/pgvector 语义召回留作可选增强（见 P1-1）。 |
| 3 | **Tool Policy 校验链**（§4.3、§13.5） | ✅ | **已于 2026-06-10 实现**（`internal/services/policy.go` `EvaluateToolCall`/`CheckToolCall`）：`Allowed` / `MaxCallsPerDay` / `MaxCallsPerUser` / `RequiresConfirmation` 全字段在 `handleToolsCall` 执行，配额类拒绝映射 `ErrCodeRateLimited`、禁用映射 `ErrCodeForbidden`。 |
| 4 | **高风险操作二次确认**（§4.4） | ✅ | **已于 2026-06-10 实现**：新增 config `EnableConfirmation`（env `ENABLE_CONFIRMATION`）；需确认且未确认时返回 `ErrCodeToolRequiresConfirm`，客户端带 `__confirm: true` 重试放行（该参数不透传 handler/上游）。 |
| 5 | **配额与限流**（§14.3、§14.4） | 🟡 | **每日工具调用配额（`QuotaToolCallDaily`）+ 记忆数量配额（`QuotaMemoryCount`）已于 2026-06-10 实现并拦截**（`save_memory`/`propose_memory` + Gateway）；`RedisAddr` 分布式限流仍未做（留待 P2）。 |
| 6 | **凭据加密托管**（§13.6、§20.2） | ✅ | **已于 2026-06-10 实现**。`SetEncryptionKey` 本就在 `database.Init` 中初始化；本次将 `EncryptAES`/`DecryptAES` 接入：连接器 Create/Update 时 `AuthConfig` 加密落库，`invokeUpstreamMCP` 调用前解密（对历史明文行向后兼容）；并对 `audit_logs` 中的凭据脱敏（`redactedMark`）。新增加解密 round-trip / 明文兼容 / 无密钥 no-op 单测。 |
| 7 | **写入纪律评分**（§10.4） | 🟡 | **2026-06-10：pending_review 候选已改为落库**（`state='pending_review'`，不再丢弃）+ 新增审核入口 `POST /memories/:id/review`（approve→active 并建有效期/批准时校验配额；reject→rejected），端到端测试覆盖。**剩余**：评分规则仍较简单（长度/置信度/TODO/0.92 去重），可后续增强。 |
| 8 | **行级安全 / 租户隔离**（§9.5 RLS、§8.8） | 🟡 | 隔离靠每条查询手写 `WHERE workspace_id = ?`，无 GORM 全局 Scope 插件（§9.6）兜底，**易因漏写条件造成越权**。 |
| 9 | **熔断 / 并行扇出**（§13.4、§21.1） | 🟡 | **2026-06-10：熔断已实现**（`internal/mcp/breaker.go`，每 server 一个 closed/open/half-open 状态机，仅传输层错误/5xx 计失败、4xx 与上游业务错误不计；`invokeUpstreamMCP` 已接入）。单测覆盖状态机 + 5xx 触发/4xx 不触发的端到端。**剩余**：并行扇出/连接池暂不适用（当前无多上游聚合调用点，待该功能出现时再补）。 |
| 10 | **迁移策略**（§9.7） | ✅ | **已于 2026-06-10 实现**：新增轻量版本化迁移运行器 `internal/database/migrate.go`（`schema_migrations` 表 + 有序迁移注册表 + 每条独立事务 + 幂等），`Init` 中 `autoMigrate`（基础表结构）后调用 `RunMigrations`（增量/数据迁移）。首条迁移回填 `memories.char_count`。单测覆盖应用/幂等/回填，并实测启动只应用一次。无第三方依赖。 |
| 11 | **容器化 / CI**（§18.4、§21.6） | 🟡 | **2026-06-10**：新增 `backend/Dockerfile`（多阶段，正确处理 sqlite 的 CGO 依赖）+ `.dockerignore` + `.github/workflows/ci.yml`（backend vet/build/test -race + frontend build）；核心逻辑单测已补齐（config/database/services/mcp 共 ~33 例）。**注**：Dockerfile 未在本地 docker 构建验证（沙箱无 docker），但构建命令已本地编译通过。 |
| 12 | **配置/文档漂移** | 🟡 | **2026-06-10**：新增 `backend/.env.example`（如实记录全部 env）+ 启动安全自检 `Config.SecurityWarnings()`（默认 JWT/加密密钥/口令时打印告警，已实测）。**剩余**：config.go 默认端口 `8084/8085` 与 agent.md/e2e/测试用的 `18084/18085` 不一致（已在 CLAUDE.md 标注，未强行改默认以免影响既有脚本）。 |
| 13 | **项目维度配置聚合视图**（§15.3/§15.4 前端、§8.1 Project） | ✅ | **2026-06-19 提出并实现。** 此前项目的 AI 配置散落在 6 个独立页面、各自靠页内"项目下拉"切换，`Projects.tsx` 只是身份列表，缺少"以项目为中心"的聚合入口（用户困惑来源）。**已实现**：新增 `frontend/src/pages/ProjectDetail.tsx`（路由 `projects/:id`），Tab = 概览（身份与绑定）/ 项目规则（内嵌 `RuleManager` 锁定本项目）/ 同步状态；`Projects.tsx` 列表名称可点击进入。浏览器端到端验证通过。 |
| 14 | **`project.md` 字段管理入口**（§8.1 Project、`syncbundle.renderProjectFile`） | ✅ | **2026-06-19 复核：实为已实现，非缺口。** `Projects.tsx` 表单已暴露 `description`/`stack`/`structure`/`git_remote`/`repo_name`，后端 `ProjectHandler.Create/Update` 也已接收这些字段（最初基于不完整 grep 误判为缺失）。`project.md` 的 `## Tech Stack` / `## Structure` 段可正常产出。详情页概览 Tab 额外提供只读展示。 |
| 15 | **同步可观测性 / SyncRecord**（§2.4、§7.5 Snapshot） | ✅ | **2026-06-19 提出并实现。** 新增 `models.SyncRecord`（`sync_records` 表，注册进 AutoMigrate）；`hub.sync_project` handler 旁路 `recordSyncProject()` 按 (project_id,user_id,agent_client_id,repo_path) upsert（changed 与否都记一次，累加 sync_count），覆盖 MCP agent 与 `openagent` CLI（CLI 亦走 `/mcp`）；REST `GET /projects/:id/sync-records` 返回各端记录 + 用户名 + 对比当前 bundle etag 的 `stale` 标记；详情页"同步状态"Tab 展示。集成测试 `TestSyncProjectRecordsSync`（建行/upsert 累加）+ 浏览器端到端验证通过。**注**：`ETag` 字段显式 `column:etag`，避开 GORM 默认 `e_tag` 拆分。 |
| 16 | **Agent(端)维度规则 UI**（§6.2、§8.2 `Rule.AgentName`） | ✅ | **2026-06-19 复核：实为已实现，非缺口。** `components/RuleManager.tsx` 已含 `agent_name` 表单项（"留空适用所有 Agent"）与表格 Agent 列，配合 gap #1 的 `ResolveEffectiveRules` agent 维度覆盖即可端到端工作（最初误判为缺失）。 |
| 17 | **多端绑定的可见与可纠正**（§2.3 `FindProjectByIdentity`） | ✅ | **2026-06-19 提出并实现（展示部分）。** 详情页概览 Tab 展示当前 `git_remote`/`repo_name`/最近 `repo_path`/`project_id` 及绑定说明；纠正路径复用既有 Projects 编辑表单（`git_remote`/`repo_name` 可改）。`repo_path` 为本机 last-seen，按设计只读。**剩余可选**：在详情页内直接重绑的快捷入口（当前需回列表编辑）。 |

> **2026-06-19 增补并落地：** #13–#17 为"多人多端同步 Agent 配置"主题。底层数据流本就正确（DB 为事实源、`.openagent/` 为单向只读快照）。经实现/复核：**#13（项目详情页）、#15（SyncRecord 同步可观测性）已新建完成**；**#14（project.md 字段）、#16（agent 维度规则 UI）复核发现实为既已实现**（最初基于不完整 grep 误判）；**#17（绑定可见）展示部分完成**，详情页内重绑快捷入口列为可选剩余。详见 E.3 的 P1-3 起。

### E.2 已正确实现（对照确认）

- ✅ 双服务架构（Console REST + MCP Gateway，§2.1、§18.1）
- ✅ 双鉴权路径：Web 用户 JWT + Agent `pat_` Token（§5、§20.2 的 Token 部分）
- ✅ MCP 传输：`POST/GET /mcp` + Legacy `/sse`+`/message`（§5.2）
- ✅ 工具调用审计落库 `ToolInvocationLog`（§13.7、§6.7）
- ✅ 30+ 数据模型与 `BaseModel`/UUID/多租户字段（§8）

### E.3 整改优先级（与产品目标"Agent 全局配置"对齐）

| 顺序 | 整改项 | 对应差距 | 验收标准 |
|:---:|--------|:--------:|----------|
| ~~P0-1~~ ✅ | ~~抽出 `ResolveEffectiveConfig`，实现带优先级的覆盖合并（agent > project > workspace），改造 `get_global_rules`/`get_project_rules` 支持 `agent_name` 维度 + 去重 override~~ **已完成 2026-06-10**，见 `services/config_resolver.go` + `config_resolver_test.go` | #1 | ✅ 同 `type/name` 的 project/agent 规则覆盖 global；单元测试覆盖三级合并 |
| ~~P0-2~~ ✅ | ~~在 `handleToolsCall` 接入 Policy 全字段校验 + 配额拦截 + 二次确认流程；config 补 `ENABLE_CONFIRMATION`~~ **已完成 2026-06-10**，见 `services/policy.go` + `policy_test.go`（Redis 分布式限流除外） | #3 #4 #5 | ✅ 超配额返回 `ErrCodeRateLimited`；`requires_confirmation` 工具走 `__confirm` 握手 |
| ~~P0-3~~ ✅ | ~~`AuthConfig` 写入加密 / 读取解密~~ **已完成 2026-06-10**（`SetEncryptionKey` 原已在 `Init` 中调用）；额外补充 audit 脱敏 | #6 | ✅ DB 内 `auth_config` 为密文；上游调用仍可正确鉴权 |
| 🟡 P1-1 | 记忆检索：~~修正中文分词~~ **已完成 2026-06-10**（CJK bigram + `relevance`/`similarity`，中文可召回）；**剩余可选**：接入真正 embedding（dev 暴力 cosine / prod pgvector）做语义召回 | #2 | ✅ 中文 query 能召回中文记忆；语义召回待定 |
| 🟡 P1-2 | ~~`pending_review` 落库 + 审核队列~~ **已完成 2026-06-10**（`memories.Review` + `propose_memory` 落库 + e2e 测试）；**剩余**：GORM Workspace Scope 插件兜底隔离（#8，需请求级 DB 会话改造，风险较高，建议单独立项） | #7 #8 | ✅ 待审记忆可在控制台复核；⏳ 漏写 `workspace_id` 兜底待做 |
| 🟡 P2 | ~~Dockerfile + CI 门禁 + 核心逻辑单测 + `.env.example` + 密钥告警 + 版本化迁移（#10）~~ **已完成 2026-06-10**；**剩余**：统一端口默认值（8084/8085 vs 18084/18085） | #10 #11 #12 | ✅ CI 配置就绪、单测齐备、版本化迁移落地；⏳ 端口默认值待统一 |
| ~~P1-3~~ ✅ | ~~**项目详情页**：以项目为中心的 Tab 聚合视图，作为该项目全部 agent 配置的单一入口；`Projects.tsx` 列表行点击进入~~ **已完成 2026-06-19**，见 `frontend/src/pages/ProjectDetail.tsx`（概览 / 项目规则内嵌 / 同步状态） | #13 | ✅ 点击项目进入详情页，一处查看绑定/规则/同步，无需反复切"项目下拉" |
| ~~P1-4~~ ✅ | ~~补全 `project.md` 字段编辑~~ **复核为既已实现**：`Projects.tsx` 表单与后端 Create/Update 早已支持 `description`/`stack`/`structure`（误判），详情页另加只读展示 | #14 | ✅ Console 填写技术栈/结构后 `.openagent/project.md` 含 `## Tech Stack` / `## Structure` |
| ~~P1-5~~ ✅ | ~~**SyncRecord 同步可观测性**：新增模型 + AutoMigrate；`hub.sync_project` 落 upsert；详情页"同步状态"展示各端 etag 与落后标记~~ **已完成 2026-06-19**（`models.SyncRecord` + `recordSyncProject` + `GET /projects/:id/sync-records` + `TestSyncProjectRecordsSync`） | #15 #17 | ✅ 控制台可见"谁/哪个端/哪台机器/同步到哪个 etag/何时"；落后端有视觉提示 |
| ~~P1-6~~ ✅ | ~~Agent 维度规则 UI~~ **复核为既已实现**：`RuleManager.tsx` 已含 `agent_name` 表单项 + 表格 Agent 列（误判） | #16 | ✅ 可为指定 client 创建规则，sync 后仅该端 `rules.md` 体现覆盖 |
| 🟡 P2-2 | **多端绑定可纠正**：~~详情页展示 `git_remote`/`repo_name`/`repo_path`~~ **展示已完成 2026-06-19**（详情页概览 Tab）；纠正复用 Projects 编辑表单。**剩余可选**：详情页内直接重绑的快捷入口 | #17 | 🟡 绑定信息可见、可经编辑表单修正；详情页内重绑快捷入口待做 |

> 本附录为活文档：每完成一项整改，请将对应行状态更新为 ✅ 并在 commit 中引用 `附录 E.<编号>`。

---
