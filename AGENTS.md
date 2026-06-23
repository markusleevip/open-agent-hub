# AGENTS.md

This file provides guidance to Qoder (qoder.com) when working with code in this repository.

Open Agent Hub is an **MCP-based AgentOps SaaS platform**: it gives AI agents (Cursor, Claude Code, Cline, etc.) centralized rule management, cross-session memory sync, team Skill distribution, external-tool routing, and policy enforcement. Further reading: [README.md](README.md), [CLAUDE.md](CLAUDE.md), [docs/spec.md](docs/spec.md), [docs/user_manual.md](docs/user_manual.md).

## Commands

### Backend (Go, run from `backend/`)

The Go module is `github.com/openagenthub/backend`. Go **1.26+** is required — the SQLite driver (`mattn/go-sqlite3`) needs CGO, so a C toolchain must be present (`CGO_ENABLED=1`, which is the default on supported platforms).

```bash
go run ./cmd/server/                       # start both servers (Console + MCP Gateway), foreground
go build -o openagent-bin ./cmd/server/    # build the server binary
go build ./cmd/openagent/                  # build the project-onboarding CLI (init / sync / status)
go test ./...                              # all tests
go test ./tests/ -run TestLogin -v         # single integration test (tests/api_test.go, tests/sync_test.go)
go vet ./...                               # lint
gofmt -w .                                 # format
```

`./scripts/start-backend.sh` is a wrapper with extra modes (auto-loads `backend/.env` if present): `run` (foreground, default for dev) / `build` (compile then run) / `test` / `start`|`stop`|`restart`|`status`|`logs` (background via PID file).

### Frontend (React/Vite, run from `frontend/`)

```bash
npm install
npm run dev        # Vite dev server on :13000 (host 0.0.0.0), proxies /api,/health→:8084 and /mcp→:8085
npm run build      # tsc -b && vite build → frontend/dist/
```

### End-to-end (requires a running backend on ports **18084/18085**)

```bash
# terminal 1 (from backend/)
CONSOLE_PORT=18084 MCP_PORT=18085 go run ./cmd/server/
# terminal 2 (from repo root)
./scripts/e2e-test.sh
```

### CI

`.github/workflows/ci.yml` runs on push/PR to `main`/`dev`: backend does `go vet`, `go build`, then `go test ./... -race` with `CGO_ENABLED=1`; frontend does `npm ci && npm run build`.

## Architecture

### Single Go binary, two HTTP servers

`backend/cmd/server/main.go` starts two Gin servers in one process:
- **Console REST API** (default `:8084`) — the SaaS admin console backend consumed by the React frontend. Routes are built in `buildConsoleRouter`. Auth via JWT (`middleware.AuthRequired`).
- **MCP Gateway** (default `:8085`) — the agent-facing endpoint (`buildMCPRouter`, handlers in `internal/mcp/gateway.go`). Speaks JSON-RPC 2.0 over: `POST /mcp` (Streamable HTTP), `GET /mcp` (SSE), and legacy `GET /sse` + `POST /message` for older clients (Cursor, Claude Desktop). Auth via `Authorization: Bearer …`.

### Second binary — `openagent` CLI (`backend/cmd/openagent/main.go`)

Binds a code repo to a project (`init`), pulls the local config snapshot (`sync`, ETag-incremental), and reports state (`status`). Writes `.openagent/config.json` (committable project identity), `.openagent/local/state.json` (machine-local sync state, gitignored), injects managed blocks (`<!-- openagenthub:begin/end -->`) into the repo's CLAUDE.md/AGENTS.md, and generates `.mcp.json`. Credentials live in `~/.openagent/credentials.json`, never in the project. The snapshot is rendered server-side by `internal/services/syncbundle.go` (shared by the CLI and the `hub.sync_project` tool): one-way server→local, deterministic content (no timestamps) so ETags stay stable; personal data (profile/memories) under `local/`.

### Service layer (`internal/services/`)

Holds logic shared between REST handlers and MCP tools:
- `config_resolver.go` — merges workspace/project/agent-scoped rules into effective rules + computes a rule ETag.
- `policy.go` — tool-policy evaluation, daily quotas, the `__confirm` two-step handshake for high-risk tools.
- `audit.go` — audit + usage recording.
- `textsearch.go` — memory search scoring (token overlap with CJK handling; **not** vector embeddings).
- `syncbundle.go` — renders the project sync snapshot bundle (used by the `hub.sync_project` tool and the CLI).
- `output_preferences.go` — user output style preferences (scoped per `user_id`, decoupled from workspace).

### Two auth paths converge on `Authorization: Bearer …`

- Web users → **JWT** (issued by `internal/auth`, validated in `middleware.AuthRequired`). Login/register is by **`username`** (not email — `User.Email` was renamed to `User.Username`; the JWT `Username` claim and `auth.ValidateUsername` format rule follow). Login only matches workspace members with `status = "active"`.
- Agents → personal access tokens prefixed **`pat_`**. The gateway (`AuthenticateAndContext`) tries JWT first, then falls back to `pat_` API-key lookup.

### Four-level tenant model

```
Organization → Workspace → Project → User
```

Workspace is the core isolation unit (rules, memory, tokens are workspace-isolated). Models carry `org_id` / `workspace_id` / `user_id`. Tenant resolution is in `internal/middleware/tenant.go`. Workspaces have a `Type` field: `personal` (one per user, auto-created on register, cannot be deleted) or `team` (invited members). **Workspace membership** carries a `Status` (`active` / pending) and `InvitedBy`: members are invited and must accept — see `MemberHandler.ListMyInvitations` / `AcceptInvitation` / `RejectInvitation` in `internal/handlers/workspace.go`.

**Project binding** resolves per tool call with priority: `project_id` arg > project identity (`git_remote`/`project_path` args) > session binding (`mcp_sessions.project_id`, set by `hub.sync_project` and inherited for the rest of the session) > `X-Project-Path` header (legacy fallback). See `resolveProjectID` in `internal/mcp/tools.go`. Identity matching (`services.FindProjectByIdentity`) is cross-machine-aware: normalized `git_remote` (most reliable) > exact `repo_path` (same machine) > unique `repo_name` basename fallback. `hub.sync_project` with `register_project=true` auto-creates the project (LLM-supplied semantic `project_name`, deduplicated slug).

### MCP Gateway request flow

`handleMethod` dispatches by JSON-RPC method → `initialize` / `tools/list` / `tools/call`. `tools/call` runs `checkPolicy` (tool-policy enforcement) before invocation and `logInvocation` after (audit trail). Sessions are tracked via the `Mcp-Session-Id` header.

### MCP tools

All registered in `internal/mcp/tools.go` via `RegisterP0Tools(registry)` (called from `NewGateway`). They use the `hub.` prefix. There are **25 tools**:

- **Memory (6):** `hub.search_memory`, `hub.get_relevant_memory`, `hub.propose_memory` (the required write entry point — write-discipline scoring may auto-accept or queue for review), `hub.save_memory` (direct save, only when policy allows auto-accept), `hub.update_memory` (increments version), `hub.archive_memory`.
- **Rules & policy (8):** `hub.get_global_rules`, `hub.get_project_rules`, `hub.get_project_context` (rules + prefs + memories + skills), `hub.get_workspace_policy`, `hub.get_tool_policy`, `hub.get_usage_policy`, `hub.get_remaining_quota`, `hub.get_output_preferences`.
- **Agent & integration (3):** `hub.get_agent_profile`, `hub.list_connected_tools`, `hub.invoke_connected_tool` (proxies to external MCP servers; upstream calls in `internal/mcp/utils.go`, guarded by a per-server circuit breaker in `internal/mcp/breaker.go`).
- **Skills (3):** `hub.list_skills`, `hub.search_skills`, `hub.get_skill` (single skill by ID).
- **Project context (3):** `hub.get_project_stack` (technology stack), `hub.get_project_structure` (directory structure), `hub.update_project_context` (update description/stack/structure via MCP).
- **Sync (1):** `hub.sync_project` (binds the working directory to a project and returns a snapshot bundle of team rules, project info, skills, and key memories).
- **Audit (1):** `hub.report_action`.

External MCP servers are proxied through `hub.invoke_connected_tool` with per-server circuit breakers (only transport-layer errors / 5xx trip the breaker; 4xx and JSON-RPC errors reset it).

## Conventions

### MCP tools (`internal/mcp/tools.go`)

Register with `r.Register(Tool{Name, Description, InputSchema}, handler)`. Handlers take `(ctx *Context, args map[string]interface{})`, do **not** write HTTP responses directly, and return `(interface{}, error)` — the gateway wraps errors into MCP error responses. Helper arg extractors: `strArg`, `intArg`, `floatArg`, `boolArg`.

### REST handlers (`internal/handlers/`)

`NewXHandler()` constructor + methods like `List`/`Get`/`Create`; register routes in `buildConsoleRouter` in `main.go`. REST responses use the unified `internal/response` format. Public-skill admin routes require `middleware.RequireRole("owner", "admin")`.

### Models (`internal/models/models.go`)

Embed `BaseModel` (UUID id via `BeforeCreate` hook + timestamps + soft delete), add an explicit `TableName()`, and register in `database.AutoMigrate` (`internal/database/database.go`).

### Database migrations

- **Schema changes** (new tables/columns) → GORM `AutoMigrate` in `internal/database/database.go`.
- **Data migrations** (backfills, transforms) → append-only `migrations` slice in `internal/database/migrate.go`. IDs are `YYYYMMDD_NN_description`, applied in order, recorded in `schema_migrations`, and must be idempotent. **Never edit a shipped migration.**
- DB is SQLite in dev, PostgreSQL in prod (both GORM drivers vendored) — migrations and raw SQL must work on both.

### Frontend i18n

All UI strings go through react-i18next keys; add every new key to **both** `frontend/src/locales/en.json` and `zh.json` (fallback language is `zh`). No hardcoded display text in components. State management uses Zustand; UI components use Ant Design.

### Error handling

- MCP tools return `error`; the gateway wraps into MCP error responses.
- REST API uses the unified `internal/response.Response` format: `{code, message, data}`. Code `0` = success; non-zero = error (e.g. `40000` bad request, `40100` unauthorized, `50000` internal).
- Tools/handlers do not write HTTP responses directly.

### Frontend data flow

- `frontend/src/api/http.ts` is an Axios instance that auto-unwraps the `{code: 0, data: ...}` envelope and auto-extracts `items` from paginated responses.
- Auth state (token, user, workspace, role) persisted via Zustand + `zustand/persist` in `frontend/src/store/auth.ts` (localStorage key `oah-auth`).
- Routes use three guard levels: `ProtectedRoute` (has token), `WorkspaceRoute` (has token + active workspace, else redirect to `/onboarding`), `AdminRoute` (role is `owner` or `admin`).
- All pages are lazy-loaded via `React.lazy` + `Suspense`.

## Config

Loaded in `internal/config/config.go` from env vars (see `backend/.env.example` for the annotated list). Key items:

| Env var | Default | Notes |
|---|---|---|
| `CONSOLE_PORT` | `8084` | Console REST API; tests & `e2e-test.sh` assume `18084` |
| `MCP_PORT` | `8085` | MCP Gateway; tests & `e2e-test.sh` assume `18085` |
| `DB_TYPE` | `sqlite` | `sqlite` (dev) / `postgres` (prod) |
| `DB_DSN` | `data/openagenthub.db` | SQLite file path or Postgres DSN |
| `JWT_SECRET` | dev placeholder | **must override in production** |
| `JWT_EXPIRE_HOURS` | `168` (7d) | |
| `BOOTSTRAP_USERNAME` | `admin` | seeded admin |
| `BOOTSTRAP_PASSWORD` | `admin123` | seeded admin — **change in production** |
| `ENCRYPTION_KEY` | dev placeholder | 32-byte key; encrypts connected-server `auth_config` at rest — **must override in production** |
| `REDIS_ADDR` | empty | optional, rate-limit/cache (not yet wired) |
| `ENABLE_CONFIRMATION` | `false` | gates the high-risk tool two-step `__confirm` handshake |

On startup, `SecurityWarnings()` logs warnings for any sensitive config left at dev defaults. First start auto-creates the SQLite DB and seeds a default admin account.

<!-- openagenthub:begin -->
## Open Agent Hub

This project is connected to Open Agent Hub (MCP server `hub`).

- At session start, check freshness first: if MCP is available, call `hub.sync_project` with this repository's absolute `project_path` and the local `.openagent/local/state.json` ETag when available. If the tool reports changes, refresh the returned files or ask the user to run `openagent sync`.
- After the freshness check, read `.openagent/rules.md` (team rules); also read `.openagent/local/profile.md` and `.openagent/local/memories.md` if present (personal profile, preferences, and key memories).
- Project context lives in `.openagent/project.md`.
- Team skills live under `.openagent/skills/` (one directory per skill, see `skills/index.json`).
- Search team memory with `hub.search_memory`; persist new knowledge via `hub.propose_memory` (never edit the snapshot files directly).
- Before invoking external tools, check `hub.get_tool_policy`.
<!-- openagenthub:end -->
