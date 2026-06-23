# Open Agent Hub

[English](README.md) | 简体中文

Open Agent Hub 是一个基于 **MCP (Model Context Protocol)** 协议的 **AgentOps 平台**。它作为 AI 编码工具（Cursor、Claude Code、Cline、Windsurf、OpenCode 等）的中心化智能中枢，提供统一的**规则管理、跨会话记忆同步、团队级 Skill 分发、外部工具路由与安全策略拦截**能力。

```
┌────────────────────────────────────────────────────────┐
│             AI Agents (Cursor, Claude Code, etc.)      │
└───────────────────────────┬────────────────────────────┘
                            │  HTTPS / Streamable HTTP
                            ▼
┌────────────────────────────────────────────────────────┐
│                     MCP Gateway                        │
│        (鉴权、租户隔离、工具路由、策略拦截)              │
└──────────────┬──────────────┬──────────────┬───────────┘
               │              │              │
        Context Service  Memory Service  Tool Proxy
        (规则与偏好)      (团队记忆)      (外部 MCP)
```

所有数据都按四层租户模型隔离：**Organization（组织）→ Workspace（工作区）→ Project（项目）→ User（用户）**，其中 Workspace 是核心隔离单元。

## 功能特性

- **统一规则管理** — 全局、项目、Agent 三级规则覆盖，一处维护，所有接入的 Agent 自动获取正确的上下文，无需为每个客户端单独配置。
- **跨会话记忆** — Agent 在工作中积累的经验和发现持久化存储，切换客户端或开启新会话后依然可用。从 Cursor 切到 Claude Code，团队知识库仍在那里。
- **团队 Skill 分发** — 将可复用的操作流程（调试指南、部署步骤、编码规范）打包为 Skill，一键安装后自动同步到所有成员的 Agent。
- **外部工具路由** — 在控制台统一接入外部 MCP Server（GitHub、PostgreSQL、Notion 等），所有 Agent 通过 Hub 网关访问，免去每台开发机重复配置 API Key。
- **安全策略拦截** — 控制 Agent 可使用的工具范围，对高危操作（如删除数据库记录、部署生产环境）要求人工二次确认，并可设置每日调用配额。
- **审计与可观测** — 所有 Agent 操作记录在不可变审计日志中。控制台仪表盘提供实时会话数、工具调用趋势和 Top 工具分析。

深入阅读：[用户使用手册](docs/user_manual.md) · [完整产品规格](docs/spec.md) · [Agent 工程指南](AGENTS.md)

## 环境要求

- **Go 1.26+** — SQLite 驱动依赖 CGO，需要本机有 C 编译工具链
- **Node.js 18+** 及 npm（前端）
- **Docker**（可选，容器化部署用）

## 快速开始（本地开发）

### 1. 启动后端

后端是单个 Go 二进制，同时运行两个 HTTP 服务：**Console REST API**（默认 `:8084`）和 **MCP Gateway**（默认 `:8085`）。

```bash
# 可选：自定义配置（所有变量均有默认值）
cp backend/.env.example backend/.env

# 启动（自动加载 backend/.env，若存在）
./scripts/start-backend.sh

# 等价的手动方式：
# cd backend && go run ./cmd/server/
```

脚本还支持 `./scripts/start-backend.sh build`（先编译再运行）和 `./scripts/start-backend.sh test`（跑测试套件）。

首次启动会自动建库（默认 SQLite，位于 `backend/data/openagenthub.db`）并初始化默认管理员账号。

### 2. 启动前端

```bash
cd frontend
npm install
npm run dev
```

打开 http://localhost:13000，用默认管理员登录：

- **用户名：** `admin`
- **密码：** `admin123`

Vite 开发服务器已内置代理：`/api`、`/health` → `:8084`，`/mcp` → `:8085`，无需额外配置。

## 接入 AI 客户端

### 获取 MCP Token

1. 在控制台进入 **「MCP Tokens」** 页面，点击 **「生成新 Token」**。
2. 输入名称（如 `My Mac Studio`），选择 Scopes（推荐选 `read` + `write`），设置有效期（`0` 表示永久）。
3. 立即复制生成的 Token（格式 `pat_...`）——出于安全考虑，该 Token 仅展示一次。

### 配置客户端

在你的 AI 编码工具中添加 Hub 作为 MCP Server。以下是主流客户端的现成配置：

#### Cursor

编辑 `~/.cursor/mcp.json`：

```json
{
  "mcpServers": {
    "open-agent-hub": {
      "url": "http://localhost:8085/mcp",
      "headers": {
        "Authorization": "Bearer pat_<your-token>"
      }
    }
  }
}
```

#### Claude Code

在项目根目录创建或编辑 `.mcp.json`：

```json
{
  "mcpServers": {
    "open-agent-hub": {
      "type": "http",
      "url": "http://localhost:8085/mcp",
      "headers": {
        "Authorization": "Bearer pat_<your-token>"
      }
    }
  }
}
```

#### Cline

编辑 Cline 的 MCP 设置（VS Code 中的 `cline_mcp_settings.json`）：

```json
{
  "mcpServers": {
    "open-agent-hub": {
      "url": "http://localhost:8085/mcp",
      "headers": {
        "Authorization": "Bearer pat_<your-token>"
      }
    }
  }
}
```

#### Windsurf

编辑 `~/.codeium/windsurf/mcp_config.json`：

```json
{
  "mcpServers": {
    "open-agent-hub": {
      "serverUrl": "http://localhost:8085/mcp",
      "headers": {
        "Authorization": "Bearer pat_<your-token>"
      }
    }
  }
}
```

#### OpenCode

在项目根目录创建或编辑 `opencode.json`（全局配置路径为 `~/.config/opencode/opencode.json`）：

```json
{
  "mcp": {
    "open-agent-hub": {
      "type": "remote",
      "url": "http://localhost:8085/mcp",
      "headers": {
        "Authorization": "Bearer pat_<your-token>"
      },
      "enabled": true
    }
  }
}
```

#### 其他 MCP 客户端

任何支持 **Streamable HTTP** 的客户端均可接入：`POST http://localhost:8085/mcp`，请求头携带 `Authorization: Bearer pat_…`。旧版客户端也可使用 `GET /sse` + `POST /message` 传输。

### 验证连接

```bash
curl -X POST http://localhost:8085/mcp \
  -H "Authorization: Bearer pat_<your-token>" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

成功响应会返回所有可用 `hub.*` 工具的 JSON 数组。

### 绑定项目

绑定项目后，Hub 可以将 Agent 调用关联到正确的项目级规则和记忆。

**方式 A — `openagent` CLI（推荐）：**

```bash
cd backend && go build ./cmd/openagent/   # 编译 CLI

# 在你的项目仓库内：
openagent init --server http://localhost:8085 --token pat_xxx
openagent sync     # 增量刷新本地快照（按 ETag）
openagent status   # 查看本地状态并检查服务端是否有新版本
```

`init` 会落盘 `.openagent/` 快照目录、向 `CLAUDE.md`/`AGENTS.md` 注入托管块，并生成 `.mcp.json` 供 MCP 客户端使用。凭据保存在 `~/.openagent/credentials.json`，绝不写入项目目录。`.openagent/` 是本地生成快照，建议加入项目 `.gitignore`。

**方式 B — 纯 MCP（适用于无法运行 CLI 的客户端）：**

让 Agent 在任务开始时调用一次 `hub.sync_project`：

```
hub.sync_project(
  project_path=<当前工作目录绝对路径>,
  register_project=true,
  project_name=<语义化项目名，如 "支付服务 API">
)
```

项目不存在时会自动创建，绑定在当前 MCP 会话内持续生效。

## 接入后能做什么

**跨团队管理规则。** 定义全局规则（编码标准、安全策略），再叠加项目级规则（特定仓库的约定）。在控制台更新规则后，所有接入的 Agent 在下次同步时自动获取——不用再到处发 `.cursorrules` 文件了。

**共建团队记忆。** 当 Agent 发现了值得记住的内容（棘手的调试技巧、项目特有的坑），它会调用 `hub.propose_memory`。高质量记忆自动采纳；不确定的进入控制台审核队列，人工确认后生效。一旦通过，团队所有成员的 Agent 立即可用。

**分发可复用 Skill。** 将操作流程（部署检查清单、故障响应手册、Code Review 指南）打包为 Skill，成员一键安装后同步到 `.openagent/skills/`，支持离线访问。

**通过 Hub 代理外部工具。** 在控制台统一接入 GitHub MCP、PostgreSQL MCP 或任何自定义 MCP Server。所有 Agent 通过 Hub 网关访问，统一鉴权、策略管控和熔断保护。再也不用在每台开发机上重复配置 API Key。

**执行安全策略。** 将高危工具（数据库删除、生产部署）标记为需要人工确认。设置每日调用配额防止 Agent 失控。每次工具调用都有完整的审计日志可追溯。

## MCP 工具速查

接入后，你的 Agent 可使用 **25 个内置工具**（前缀 `hub.`）：

| 类别 | 工具 | 说明 |
|---|---|---|
| **记忆** | `hub.search_memory` | 语义搜索工作区/项目记忆 |
| | `hub.get_relevant_memory` | 根据当前上下文自动召回相关记忆 |
| | `hub.propose_memory` | 提议一条新记忆（经写入纪律评分） |
| | `hub.save_memory` | 直接保存记忆（策略允许时） |
| | `hub.update_memory` | 更新已有记忆（递增版本号） |
| | `hub.archive_memory` | 归档记忆（从搜索结果中排除） |
| **规则与策略** | `hub.get_global_rules` | 获取工作区全局规则 |
| | `hub.get_project_rules` | 获取合并后的全局 + 项目规则 |
| | `hub.get_project_context` | 完整上下文包（规则 + 偏好 + 记忆 + Skill） |
| | `hub.get_workspace_policy` | 获取工具路由策略和配额 |
| | `hub.get_tool_policy` | 获取特定工具的策略 |
| | `hub.get_usage_policy` | 获取使用配额和当前消耗量 |
| | `hub.get_remaining_quota` | 获取今日剩余工具调用额度 |
| | `hub.get_output_preferences` | 获取用户输出偏好（语言、详细度、风格） |
| **Agent 与集成** | `hub.get_agent_profile` | 获取当前 Agent 信息和套餐 |
| | `hub.list_connected_tools` | 列出已连接外部 MCP Server 的工具 |
| | `hub.invoke_connected_tool` | 调用外部 MCP Server 上的工具（代理转发） |
| **Skill** | `hub.list_skills` | 列出可用的团队 Skill |
| | `hub.search_skills` | 按关键词搜索 Skill |
| | `hub.get_skill` | 按 ID 获取单个 Skill |
| **项目上下文** | `hub.get_project_stack` | 获取已绑定项目的技术栈 |
| | `hub.get_project_structure` | 获取已绑定项目的目录结构 |
| | `hub.update_project_context` | 更新项目描述、技术栈或目录结构 |
| **同步** | `hub.sync_project` | 绑定工作目录到项目并获取快照 |
| **审计** | `hub.report_action` | 上报操作记录用于审计 |

## 配置

全部配置通过环境变量加载（完整注释清单见 [backend/.env.example](backend/.env.example)）。关键项：

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `CONSOLE_PORT` | `8084` | Console REST API |
| `MCP_PORT` | `8085` | MCP Gateway |
| `DB_TYPE` | `sqlite` | `sqlite`（开发）/ `postgres`（生产） |
| `DB_DSN` | `data/openagenthub.db` | SQLite 文件路径或 Postgres DSN |
| `JWT_SECRET` | 开发占位值 | **生产必须覆盖** |
| `ENCRYPTION_KEY` | 开发占位值 | 32 字节密钥，加密连接器凭据——**生产必须覆盖** |
| `BOOTSTRAP_USERNAME` / `BOOTSTRAP_PASSWORD` | `admin` / `admin123` | 初始管理员——**生产必须改密码** |

## 部署

### 后端（Docker）

```bash
cd backend
docker build -t open-agent-hub .

docker run -d --name open-agent-hub \
  -p 8084:8084 -p 8085:8085 \
  -v openagent-data:/app/data \
  -e JWT_SECRET=<随机密钥> \
  -e ENCRYPTION_KEY=<随机32字节密钥> \
  -e BOOTSTRAP_PASSWORD=<强密码> \
  open-agent-hub
```

说明：

- 上面三个 `-e` 在生产环境必须覆盖，否则服务启动时会打印安全警告。
- 使用 SQLite（默认）时按上例挂载 `/app/data` 卷持久化；使用 PostgreSQL 时去掉卷挂载，改设 `-e DB_TYPE=postgres -e DB_DSN='host=... user=... password=... dbname=...'`。
- 数据库迁移在启动时自动执行（GORM AutoMigrate + 版本化数据迁移），无需手动操作。

### 前端（静态托管）

后端**不托管**前端构建产物——需将 `frontend/dist/` 部署到任意静态服务器，并反向代理 API 路径：

```bash
cd frontend
npm install
npm run build      # 产物输出到 frontend/dist/
```

nginx 配置示例：

```nginx
server {
    listen 80;

    root /var/www/open-agent-hub;   # 即 frontend/dist/
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;   # SPA 回退
    }

    location /api/    { proxy_pass http://127.0.0.1:8084; }
    location /health  { proxy_pass http://127.0.0.1:8084; }
    location /mcp     { proxy_pass http://127.0.0.1:8085; }
}
```

## 测试

```bash
# 单元测试与集成测试
cd backend && go test ./...

# 端到端测试（要求后端运行在 18084/18085 端口）
cd backend && CONSOLE_PORT=18084 MCP_PORT=18085 go run ./cmd/server/   # 终端 1
./scripts/e2e-test.sh                                                 # 终端 2，在仓库根目录执行
```

## 目录结构

```
backend/    Go 后端 — Console REST API + MCP Gateway (cmd/server)、
            项目接入 CLI (cmd/openagent)
frontend/   React + TypeScript + Ant Design 控制台（Vite）
docs/       用户手册与完整产品规格
scripts/    start-backend.sh、e2e-test.sh
```

## 文档

- [用户使用手册](docs/user_manual.md) — 控制台功能详解、客户端接入指南与 FAQ
- [产品规格文档](docs/spec.md) — 完整技术规格
- [Agent 工程指南](AGENTS.md) — 面向 AI Agent 的开发指南

## 许可证

见 [LICENSE](LICENSE)。
