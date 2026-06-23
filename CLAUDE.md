# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Open Agent Hub is an **MCP-based AgentOps SaaS platform**: it gives AI agents (Cursor, Claude Code, Cline, etc.) centralized rule management, cross-session memory sync, team Skill distribution, external-tool routing, and policy enforcement. The detailed engineering guide lives in [AGENTS.md](AGENTS.md) and the full spec in [docs/spec.md](docs/spec.md) (appendix E there tracks spec-vs-code gaps); this file covers the essentials plus corrections.

## Commands

Backend (Go **1.26+**, run from `backend/`). The SQLite driver (`mattn/go-sqlite3`) needs CGO, so a C toolchain must be present (`CGO_ENABLED=1`, the default on supported platforms):
```bash
go run ./cmd/server/          # start both servers (Console + MCP Gateway)
go build -o openagent-bin ./cmd/server/   # server binary ("openagent" is the CLI, see below)
go build ./cmd/openagent/     # project-onboarding CLI (init / sync / status)
go test ./...                 # all tests (CI runs with -race)
go test ./tests/ -run TestLogin -v   # single integration test (tests/api_test.go, tests/sync_test.go)
go vet ./...                  # lint
gofmt -w .                    # format
```
`./scripts/start-backend.sh` does the same as `go run` but auto-loads `backend/.env` (see `backend/.env.example`); modes: `run` (default) / `build` / `test`.

Frontend (React/Vite, run from `frontend/`):
```bash
npm install
npm run dev        # Vite dev server on :13000 (host 0.0.0.0); proxies /api,/health→:8084 and /mcp→:8085
npm run build      # tsc -b && vite build
```

End-to-end (requires a running backend; the script targets ports **18084/18085**):
```bash
CONSOLE_PORT=18084 MCP_PORT=18085 go run ./cmd/server/   # in backend/
./scripts/e2e-test.sh                                    # from repo root
```

CI (`.github/workflows/ci.yml`, runs on push/PR to `main`/`dev`): backend does `go vet`, `go build`, then `go test ./... -race` with `CGO_ENABLED=1`; frontend does `npm ci && npm run build`.

## Architecture

**Single Go binary, two HTTP servers** (`backend/cmd/server/main.go`):
- **Console REST API** — the SaaS admin console backend, consumed by the React frontend. Routes built in `buildConsoleRouter`.
- **MCP Gateway** — the agent-facing endpoint (`buildMCPRouter`, handlers in `internal/mcp/gateway.go`). Speaks JSON-RPC 2.0 over: `POST /mcp` (Streamable HTTP), `GET /mcp` (SSE), and legacy `GET /sse` + `POST /message` for older clients (Cursor, Claude Desktop).

**Second binary — `openagent` CLI** (`backend/cmd/openagent/main.go`): binds a code repo to a project (`init`), pulls the local config snapshot (`sync`, ETag-incremental), and reports state (`status`). It writes `.openagent/config.json` (committable project identity), `.openagent/local/state.json` (machine-local sync state, gitignored), injects managed blocks (`<!-- openagenthub:begin/end -->`) into the repo's CLAUDE.md/AGENTS.md, and generates `.mcp.json`. Credentials live in `~/.openagent/credentials.json`, never in the project. The snapshot itself is rendered server-side by `internal/services/syncbundle.go` (shared by the CLI and the `hub.sync_project` tool): one-way server→local, deterministic content (no timestamps) so ETags stay stable, personal data (profile/memories) under `local/`.

**Service layer** (`internal/services/`) holds logic shared between REST handlers and MCP tools: `config_resolver.go` (merges workspace/project/agent-scoped rules into effective rules + rule ETag), `policy.go` (tool-policy evaluation, daily quotas, the `__confirm` handshake), `audit.go` (audit + usage recording), `textsearch.go` (memory search scoring — token overlap with CJK handling, not vectors), `syncbundle.go`, `output_preferences.go` (user output-style prefs).

**Two auth paths converge on `Authorization: Bearer …`:**
- Web users → **JWT** (issued by `internal/auth`, validated in `middleware.AuthRequired`). Login/register is by **`username`** (not email — `User.Email` was renamed to `User.Username`; the JWT `Username` claim and `auth.ValidateUsername` format rule follow). Login only matches workspace members with `status = "active"`.
- Agents → personal access tokens prefixed **`pat_`**. The gateway (`AuthenticateAndContext`) tries JWT first, then falls back to `pat_` API-key lookup.

**Four-level tenant model** — every data model and request is scoped by it:
```
Organization → Workspace → Project → User
```
Workspace is the core isolation unit (rules, memory, tokens are workspace-isolated). Models carry `org_id` / `workspace_id` / `user_id`. Tenant resolution is in `internal/middleware/tenant.go`. Workspaces have a `Type` field: `personal` (one per user, auto-created on register, cannot be deleted) or `team` (invited members). **Workspace membership** carries a `Status` (`active` / pending) and `InvitedBy`: members are invited and must accept — see `MemberHandler.ListMyInvitations` / `AcceptInvitation` / `RejectInvitation` in `internal/handlers/workspace.go`. **Project binding** resolves per tool call with priority `project_id` arg > project identity (`git_remote`/`project_path` args) > session binding (`mcp_sessions.project_id`, set by `hub.sync_project` and inherited for the rest of the session) > `X-Project-Path` header (legacy fallback) — see `resolveProjectID` in `internal/mcp/tools.go`. Identity matching (`services.FindProjectByIdentity`) is **cross-machine-aware**: normalized `git_remote` (most reliable) > exact `repo_path` (same machine) > unique `repo_name` basename fallback. `RepoPath` is per-machine "last seen"; the durable cross-machine keys are `git_remote` and `repo_name`, backfilled on each sync. Project-scoped operations error loudly when nothing resolves; `hub.sync_project` with `register_project=true` auto-creates the project (LLM-supplied semantic `project_name`, deduplicated slug).

**MCP tools** (25) are all registered in `internal/mcp/tools.go` via `RegisterP0Tools(registry)` (called from `NewGateway`). They use the `hub.` prefix and group into: memory (`hub.search_memory`, `hub.get_relevant_memory`, `hub.propose_memory`, `hub.save_memory`, `hub.update_memory`, `hub.archive_memory`), rules/policy (`hub.get_global_rules`, `hub.get_project_rules`, `hub.get_project_context`, `hub.get_workspace_policy`, `hub.get_tool_policy`, `hub.get_usage_policy`, `hub.get_remaining_quota`, `hub.get_output_preferences`), agent/integration (`hub.get_agent_profile`, `hub.list_connected_tools`, `hub.invoke_connected_tool`), skills (`hub.list_skills`, `hub.search_skills`, `hub.get_skill`), project context (`hub.get_project_stack`, `hub.get_project_structure`, `hub.update_project_context`), audit (`hub.report_action`), and sync (`hub.sync_project` — returns the same bundle the CLI writes). External MCP servers are proxied through `hub.invoke_connected_tool` (upstream calls in `internal/mcp/utils.go`, guarded by a per-server circuit breaker in `internal/mcp/breaker.go`). See AGENTS.md for per-tool notes. The full registry is the source of truth — grep `r.Register(Tool{` if the count looks off.

**Request flow inside the gateway:** `handleMethod` dispatches by JSON-RPC method → `initialize` / `tools/list` / `tools/call`. `tools/call` runs `checkPolicy` (tool-policy enforcement) before invocation and `logInvocation` after (audit trail). Sessions are tracked via the `Mcp-Session-Id` header.

## Conventions

- **Tools** (`tools.go`): register with `r.Register(Tool{Name, Description, InputSchema}, handler)`. Handlers take `(ctx *Context, args map[string]interface{})`, do not write HTTP responses directly, and return `(interface{}, error)` — the gateway wraps errors into MCP error responses. Helper arg extractors: `strArg`, `intArg`, `floatArg`, `boolArg`.
- **REST handlers** (`internal/handlers/`): `NewXHandler()` constructor + methods like `List`/`Get`/`Create`; register routes in `buildConsoleRouter` in `main.go`. REST responses use the unified `internal/response` format: `{code, message, data}` — code `0` = success; non-zero = error (e.g. `40000` bad request, `40100` unauthorized, `50000` internal).
- **Models** (`internal/models/models.go`): embed `BaseModel` (UUID id via `BeforeCreate` hook + timestamps + soft delete), add an explicit `TableName()`, and register in `database.AutoMigrate` (`internal/database/database.go`). Schema changes (new tables/columns) go through AutoMigrate; **data migrations** (backfills, transforms) go in the append-only `migrations` slice in `internal/database/migrate.go` — IDs are `YYYYMMDD_NN_description`, applied in order, recorded in `schema_migrations`, and must be idempotent. Never edit a shipped migration.
- DB is SQLite in dev, PostgreSQL in prod (both GORM drivers vendored) — migrations and raw SQL must work on both.
- **Frontend i18n**: all UI strings go through react-i18next keys; add every new key to **both** `frontend/src/locales/en.json` and `zh.json` (fallback language is `zh`). No hardcoded display text in components. State management uses Zustand; UI components use Ant Design.
- **Frontend data flow**: `frontend/src/api/http.ts` is an Axios instance that auto-unwraps the `{code: 0, data: ...}` envelope. Auth state (token, user, workspace, role) persisted via Zustand + `zustand/persist` in `frontend/src/store/auth.ts`. Routes use three guard levels: `ProtectedRoute` (has token), `WorkspaceRoute` (has token + active workspace, else redirect to `/onboarding`), `AdminRoute` (role is `owner` or `admin`).

## Config

Loaded in `internal/config/config.go` from env vars (defaults shown — **these differ from older docs**):

| Env var | Default | Notes |
|---|---|---|
| `CONSOLE_PORT` | `8084` | tests & `e2e-test.sh` assume `18084` |
| `MCP_PORT` | `8085` | tests & `e2e-test.sh` assume `18085` |
| `DB_TYPE` | `sqlite` | `sqlite` (dev) / `postgres` (prod) |
| `DB_DSN` | `data/openagenthub.db` | SQLite file path or Postgres DSN |
| `JWT_SECRET` | dev placeholder | change in production |
| `JWT_EXPIRE_HOURS` | `168` (7d) | |
| `BOOTSTRAP_USERNAME` | `admin` | seeded admin (was `BOOTSTRAP_EMAIL`) |
| `BOOTSTRAP_PASSWORD` | `admin123` | seeded admin |
| `ENCRYPTION_KEY` | dev placeholder | 32-byte key; encrypts connected-server `auth_config` at rest |
| `REDIS_ADDR` | empty | optional, rate-limit/cache (not yet wired) |
| `ENABLE_CONFIRMATION` | `false` | gates the high-risk tool two-step `__confirm` handshake |

On startup, `SecurityWarnings()` logs warnings for any sensitive config left at dev defaults.

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
