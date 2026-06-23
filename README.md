# Open Agent Hub

English | [简体中文](README_zh.md)

Open Agent Hub is an **MCP-based AgentOps platform**. It acts as a central hub for AI coding agents (Cursor, Claude Code, Cline, Windsurf, OpenCode, etc.), providing unified rule management, cross-session memory sync, team-level Skill distribution, external-tool routing, and security policy enforcement — all over the [Model Context Protocol](https://modelcontextprotocol.io/).

```
┌────────────────────────────────────────────────────────┐
│             AI Agents (Cursor, Claude Code, etc.)      │
└───────────────────────────┬────────────────────────────┘
                            │  HTTPS / Streamable HTTP
                            ▼
┌────────────────────────────────────────────────────────┐
│                     MCP Gateway                        │
│        (Auth, Tenant, Tool Routing, Policies)          │
└──────────────┬──────────────┬──────────────┬───────────┘
               │              │              │
        Context Service  Memory Service  Tool Proxy
        (Rules & Prefs)  (Team Memory)   (External MCP)
```

Everything is scoped by a four-level tenant model: **Organization → Workspace → Project → User**, with Workspace as the core isolation unit.

## Features

- **Unified Rule Management** — Define rules once at the global, project, or agent level. All connected agents automatically receive the right context without per-client config files.
- **Cross-Session Memory** — Agent insights and discoveries persist across sessions and clients. Switch from Cursor to Claude Code and your team's knowledge base is still there.
- **Team Skill Distribution** — Package reusable skills (debugging workflows, deploy procedures, coding standards) and distribute them to every team member's agent via a single install.
- **External Tool Routing** — Connect external MCP servers (GitHub, PostgreSQL, Notion, etc.) once in the console. All agents access them through the Hub — no duplicate API-key configuration per client.
- **Security Policy Enforcement** — Control which tools agents can use, require human-in-the-loop confirmation for high-risk operations, and set daily call quotas.
- **Audit & Observability** — Every agent action is logged in an immutable audit trail. The dashboard shows live sessions, tool-call trends, and top-tool analytics.

Further reading: [User Manual](docs/user_manual.md) · [Full Spec](docs/spec.md) · [Agent Engineering Guide](AGENTS.md)

## Prerequisites

- **Go 1.26+** — the SQLite driver requires CGO, so a C toolchain must be available
- **Node.js 18+** with npm (frontend)
- **Docker** (optional, for containerized deployment)

## Quick Start (Local Development)

### 1. Start the backend

The backend is a single Go binary running two HTTP servers: the **Console REST API** (default `:8084`) and the **MCP Gateway** (default `:8085`).

```bash
# Optional: customize config (all variables have sane defaults)
cp backend/.env.example backend/.env

# Start (auto-loads backend/.env if present)
./scripts/start-backend.sh

# Equivalent manual form:
# cd backend && go run ./cmd/server/
```

The script also supports `./scripts/start-backend.sh build` (compile then run) and `./scripts/start-backend.sh test` (run the test suite).

On first start the database is created automatically (SQLite at `backend/data/openagenthub.db` by default) and a default admin account is seeded.

### 2. Start the frontend

```bash
cd frontend
npm install
npm run dev
```

Open http://localhost:13000 and log in with the default admin:

- **Username:** `admin`
- **Password:** `admin123`

The Vite dev server already proxies `/api`, `/health` → `:8084` and `/mcp` → `:8085`, so no extra configuration is needed.

## Connect Your AI Agent

### Get an MCP Token

1. In the console, go to **MCP Tokens** and click **Generate New Token**.
2. Enter a name (e.g. `My Mac Studio`), select scopes (`read` + `write` recommended), and set an expiry (0 = never).
3. Copy the token immediately (format `pat_...`) — it is shown only once.

### Configure Your Client

Add the Hub as an MCP server in your AI coding tool. Below are ready-to-use configs for popular clients:

#### Cursor

Edit `~/.cursor/mcp.json`:

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

Create or edit `.mcp.json` in your project root:

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

Edit your Cline MCP settings (`cline_mcp_settings.json` in VS Code):

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

Edit `~/.codeium/windsurf/mcp_config.json`:

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

Create or edit `opencode.json` in your project root (or `~/.config/opencode/opencode.json` for global config):

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

#### Other MCP Clients

Any client that supports **Streamable HTTP** can connect with `POST http://localhost:8085/mcp` and an `Authorization: Bearer pat_…` header. Legacy `GET /sse` + `POST /message` is also supported for older clients.

### Verify the Connection

```bash
curl -X POST http://localhost:8085/mcp \
  -H "Authorization: Bearer pat_<your-token>" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

A successful response returns a JSON array of all available `hub.*` tools.

### Bind Your Project

Binding a project lets the Hub associate agent calls with the right project-level rules and memories.

**Option A — `openagent` CLI (recommended):**

```bash
cd backend && go build ./cmd/openagent/   # build the CLI

# In your project repository:
openagent init --server http://localhost:8085 --token pat_xxx
openagent sync     # refresh the local snapshot (ETag-incremental)
openagent status   # show local state and check whether the server has a newer snapshot
```

`init` writes the `.openagent/` snapshot directory, injects managed blocks into `CLAUDE.md`/`AGENTS.md`, and generates `.mcp.json` for MCP clients. Credentials are stored in `~/.openagent/credentials.json`, never inside the project. `.openagent/` is a local generated snapshot and should usually be ignored by Git.

**Option B — Pure MCP (for clients that can't run the CLI):**

Have the agent call `hub.sync_project` once at task start:

```
hub.sync_project(
  project_path=<absolute path of working directory>,
  register_project=true,
  project_name=<semantic project name, e.g. "Payment Service API">
)
```

The project is auto-created if it doesn't exist, and the binding persists for the rest of the MCP session.

## What You Can Do After Connecting

**Manage rules across your team.** Define Global Rules (coding standards, security policies) that apply to every project, then layer Project Rules for repo-specific conventions. When you update a rule in the console, every connected agent picks it up on the next sync — no more emailing `.cursorrules` files around.

**Build a shared team memory.** When an agent discovers something worth remembering (a tricky debugging technique, a project-specific gotcha), it calls `hub.propose_memory`. High-quality memories are auto-accepted; borderline ones go to a review queue in the console. Once approved, the knowledge is instantly available to every team member's agent.

**Distribute reusable Skills.** Package operational procedures (deploy checklists, incident response playbooks, code-review guidelines) as Skills in the console. Team members install them with one click, and they sync to `.openagent/skills/` for offline access.

**Proxy external tools through the Hub.** Connect your GitHub MCP, PostgreSQL MCP, or any custom MCP server in the console. All agents access these tools through the Hub's gateway — with unified authentication, tool policies, and circuit-breaker protection. No more duplicate API-key configs across every developer's machine.

**Enforce security policies.** Mark high-risk tools (database deletes, production deploys) as requiring human confirmation. Set daily call quotas to prevent runaway agents. Every tool invocation is audit-logged with full traceability.

## MCP Tools Quick Reference

After connecting, your agent has access to **21 built-in tools** (prefix `hub.`):

| Category | Tool | Description |
|---|---|---|
| **Memory** | `hub.search_memory` | Semantic search across workspace/project memories |
| | `hub.get_relevant_memory` | Auto-recall memories relevant to current context |
| | `hub.propose_memory` | Propose a new memory (write-discipline scoring) |
| | `hub.save_memory` | Direct-save a memory (when policy allows) |
| | `hub.update_memory` | Update an existing memory (increments version) |
| | `hub.archive_memory` | Archive a memory (excluded from search) |
| **Rules & Policy** | `hub.get_global_rules` | Fetch workspace-wide rules |
| | `hub.get_project_rules` | Fetch merged global + project rules |
| | `hub.get_project_context` | Full context bundle (rules + prefs + memories + skills) |
| | `hub.get_workspace_policy` | Fetch tool routing policies and quotas |
| | `hub.get_tool_policy` | Fetch policy for a specific tool |
| | `hub.get_usage_policy` | Fetch usage quotas and current consumption |
| | `hub.get_remaining_quota` | Get today's remaining tool-call budget |
| | `hub.get_output_preferences` | Fetch user output preferences (language, verbosity, style) |
| **Agent & Integration** | `hub.get_agent_profile` | Get current agent info and plan |
| | `hub.list_connected_tools` | List tools from connected external MCP servers |
| | `hub.invoke_connected_tool` | Invoke a tool on an external MCP server (proxied) |
| **Skills** | `hub.list_skills` | List available team skills |
| | `hub.search_skills` | Search skills by keyword |
| **Sync** | `hub.sync_project` | Bind working directory to a project and fetch snapshot |
| **Audit** | `hub.report_action` | Report an action for audit logging |

## Configuration

All configuration is via environment variables (see [backend/.env.example](backend/.env.example) for the full annotated list). Key items:

| Variable | Default | Notes |
|---|---|---|
| `CONSOLE_PORT` | `8084` | Console REST API |
| `MCP_PORT` | `8085` | MCP Gateway |
| `DB_TYPE` | `sqlite` | `sqlite` (dev) / `postgres` (prod) |
| `DB_DSN` | `data/openagenthub.db` | SQLite path or Postgres DSN |
| `JWT_SECRET` | dev placeholder | **must override in production** |
| `ENCRYPTION_KEY` | dev placeholder | 32-byte key, encrypts connector credentials — **must override in production** |
| `BOOTSTRAP_USERNAME` / `BOOTSTRAP_PASSWORD` | `admin` / `admin123` | seeded admin — **change the password in production** |

## Deployment

### Backend (Docker)

```bash
cd backend
docker build -t open-agent-hub .

docker run -d --name open-agent-hub \
  -p 8084:8084 -p 8085:8085 \
  -v openagent-data:/app/data \
  -e JWT_SECRET=<random-secret> \
  -e ENCRYPTION_KEY=<random-32-byte-key> \
  -e BOOTSTRAP_PASSWORD=<strong-password> \
  open-agent-hub
```

Notes:

- The three `-e` overrides above are required for production; the server prints security warnings if they are left at defaults.
- With SQLite (default), persist `/app/data` via a volume as shown. For PostgreSQL, drop the volume and set `-e DB_TYPE=postgres -e DB_DSN='host=... user=... password=... dbname=...'`.
- Schema migrations run automatically at startup (GORM AutoMigrate + a versioned data-migration runner); no manual migration step is needed.

### Frontend (static hosting)

The backend does **not** serve the frontend build — deploy `frontend/dist/` behind any static server and reverse-proxy the API paths:

```bash
cd frontend
npm install
npm run build      # outputs to frontend/dist/
```

Example nginx config:

```nginx
server {
    listen 80;

    root /var/www/open-agent-hub;   # frontend/dist/
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;   # SPA fallback
    }

    location /api/    { proxy_pass http://127.0.0.1:8084; }
    location /health  { proxy_pass http://127.0.0.1:8084; }
    location /mcp     { proxy_pass http://127.0.0.1:8085; }
}
```

## Testing

```bash
# Unit & integration tests
cd backend && go test ./...

# End-to-end test (expects the backend on ports 18084/18085)
cd backend && CONSOLE_PORT=18084 MCP_PORT=18085 go run ./cmd/server/   # terminal 1
./scripts/e2e-test.sh                                                 # terminal 2, from repo root
```

## Project Layout

```
backend/    Go backend — Console REST API + MCP Gateway (cmd/server),
            project-onboarding CLI (cmd/openagent)
frontend/   React + TypeScript + Ant Design console (Vite)
docs/       User manual & full product spec
scripts/    start-backend.sh, e2e-test.sh
```

## Documentation

- [User Manual](docs/user_manual.md) — Console walkthrough, client onboarding guide, and FAQ
- [Product Spec](docs/spec.md) — Full technical specification
- [Agent Engineering Guide](AGENTS.md) — Guide for AI agents working on this codebase

## License

See [LICENSE](LICENSE).
# Open Agent Hub

English | [简体中文](README_zh.md)

Open Agent Hub is an **MCP-based AgentOps platform**. It acts as a central hub for AI coding agents (Cursor, Claude Code, Cline, etc.), providing unified rule management, cross-session memory sync, team-level Skill distribution, external-tool routing, and security policy enforcement — all over the [Model Context Protocol](https://modelcontextprotocol.io/).

```
┌────────────────────────────────────────────────────────┐
│             AI Agents (Cursor, Claude Code, etc.)      │
└───────────────────────────┬────────────────────────────┘
                            │  HTTPS / Streamable HTTP
                            ▼
┌────────────────────────────────────────────────────────┐
│                     MCP Gateway                        │
│        (Auth, Tenant, Tool Routing, Policies)          │
└──────────────┬──────────────┬──────────────┬───────────┘
               │              │              │
        Context Service  Memory Service  Tool Proxy
        (Rules & Prefs)  (Team Memory)   (External MCP)
```

Everything is scoped by a four-level tenant model: **Organization → Workspace → Project → User**, with Workspace as the core isolation unit.

Further reading (in Chinese): [User Manual](docs/user_manual.md) · [Full Spec](docs/spec.md) · [Agent Engineering Guide](AGENTS.md)

## Prerequisites

- **Go 1.26+** — the SQLite driver requires CGO, so a C toolchain must be available
- **Node.js 18+** with npm (frontend)
- **Docker** (optional, for containerized deployment)

## Quick Start (Local Development)

### 1. Start the backend

The backend is a single Go binary running two HTTP servers: the **Console REST API** (default `:8084`) and the **MCP Gateway** (default `:8085`).

```bash
# Optional: customize config (all variables have sane defaults)
cp backend/.env.example backend/.env

# Start (auto-loads backend/.env if present)
./scripts/start-backend.sh

# Equivalent manual form:
# cd backend && go run ./cmd/server/
```

The script also supports `./scripts/start-backend.sh build` (compile then run) and `./scripts/start-backend.sh test` (run the test suite).

On first start the database is created automatically (SQLite at `backend/data/openagenthub.db` by default) and a default admin account is seeded.

### 2. Start the frontend

```bash
cd frontend
npm install
npm run dev
```

Open http://localhost:13000 and log in with the default admin:

- **Email:** `admin@open-agent-hub.dev`
- **Password:** `admin123`

The Vite dev server already proxies `/api`, `/health` → `:8084` and `/mcp` → `:8085`, so no extra configuration is needed.

### 3. Connect an AI client

1. In the console, go to **MCP Tokens** and generate a token (shown once, format `pat_...`).
2. Bind your code repository with the `openagent` CLI:

   ```bash
   cd backend && go build ./cmd/openagent/   # build the CLI

   # In your project repository:
   openagent init --server http://localhost:8085 --token pat_xxx
   openagent sync     # refresh the local snapshot (ETag-incremental)
   openagent status   # show local state and check whether the server has a newer snapshot
   ```

   `init` writes the `.openagent/` snapshot, injects managed blocks into `CLAUDE.md`/`AGENTS.md`, and generates `.mcp.json` for MCP clients. Credentials are stored in `~/.openagent/credentials.json`, never inside the project. `.openagent/` is a local generated snapshot and should usually be ignored by Git; when another teammate updates shared server-side config, run `openagent sync` locally to pull the latest version.

3. Alternatively, point any MCP client directly at the gateway: `POST http://localhost:8085/mcp` with `Authorization: Bearer pat_xxx` (legacy `GET /sse` + `POST /message` is also supported for older clients). For clients that can't run the CLI, have the agent call `hub.sync_project` once at task start with `project_path=<its working directory>` and `register_project=true` — the project is auto-created (the agent supplies a semantic name) and the binding persists for the rest of the session.

See the [User Manual](docs/user_manual.md) (Chinese) for the full onboarding guide per client.

## Configuration

All configuration is via environment variables (see [backend/.env.example](backend/.env.example) for the full annotated list). Key items:

| Variable | Default | Notes |
|---|---|---|
| `CONSOLE_PORT` | `8084` | Console REST API |
| `MCP_PORT` | `8085` | MCP Gateway |
| `DB_TYPE` | `sqlite` | `sqlite` (dev) / `postgres` (prod) |
| `DB_DSN` | `data/openagenthub.db` | SQLite path or Postgres DSN |
| `JWT_SECRET` | dev placeholder | **must override in production** |
| `ENCRYPTION_KEY` | dev placeholder | 32-byte key, encrypts connector credentials — **must override in production** |
| `BOOTSTRAP_EMAIL` / `BOOTSTRAP_PASSWORD` | `admin@open-agent-hub.dev` / `admin123` | seeded admin — **change the password in production** |

## Deployment

### Backend (Docker)

```bash
cd backend
docker build -t open-agent-hub .

docker run -d --name open-agent-hub \
  -p 8084:8084 -p 8085:8085 \
  -v openagent-data:/app/data \
  -e JWT_SECRET=<random-secret> \
  -e ENCRYPTION_KEY=<random-32-byte-key> \
  -e BOOTSTRAP_PASSWORD=<strong-password> \
  open-agent-hub
```

Notes:

- The three `-e` overrides above are required for production; the server prints security warnings if they are left at defaults.
- With SQLite (default), persist `/app/data` via a volume as shown. For PostgreSQL, drop the volume and set `-e DB_TYPE=postgres -e DB_DSN='host=... user=... password=... dbname=...'`.
- Schema migrations run automatically at startup (GORM AutoMigrate + a versioned data-migration runner); no manual migration step is needed.

### Frontend (static hosting)

The backend does **not** serve the frontend build — deploy `frontend/dist/` behind any static server and reverse-proxy the API paths:

```bash
cd frontend
npm install
npm run build      # outputs to frontend/dist/
```

Example nginx config:

```nginx
server {
    listen 80;

    root /var/www/open-agent-hub;   # frontend/dist/
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;   # SPA fallback
    }

    location /api/    { proxy_pass http://127.0.0.1:8084; }
    location /health  { proxy_pass http://127.0.0.1:8084; }
    location /mcp     { proxy_pass http://127.0.0.1:8085; }
}
```

## Testing

```bash
# Unit & integration tests
cd backend && go test ./...

# End-to-end test (expects the backend on ports 18084/18085)
cd backend && CONSOLE_PORT=18084 MCP_PORT=18085 go run ./cmd/server/   # terminal 1
./scripts/e2e-test.sh                                                 # terminal 2, from repo root
```

## Project Layout

```
backend/    Go backend — Console REST API + MCP Gateway (cmd/server),
            project-onboarding CLI (cmd/openagent)
frontend/   React + TypeScript + Ant Design console (Vite)
docs/       User manual & full product spec (Chinese)
scripts/    start-backend.sh, e2e-test.sh
```

## License

See [LICENSE](LICENSE).
