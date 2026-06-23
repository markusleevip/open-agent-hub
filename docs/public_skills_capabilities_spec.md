# Public Skills and Capability Packs Spec

> Status: draft for review
> Scope: P0 implementation plan for public Skill catalog, workspace/project installs, and related MCP capability surfaces.

## 1. Background

The current Skills page stores skills as `memories` rows with `type = 'skill'`. This works for custom workspace skills, but it does not support a reusable public catalog:

- no canonical `name`, `description`, `version`, `source`, or `visibility`
- no install/uninstall relationship between a public template and a workspace/project
- no upgrade path or version lock
- no separation between platform-curated content and tenant-private memories
- no way to group Rules, Skills, Prompts, and Tool Policies into a reusable capability pack

P0 should add a public catalog and installation layer without breaking the existing custom Skill behavior.

## 2. Goals

P0 goals:

- Add a public Skill catalog that can be seeded by the platform.
- Allow a workspace to install public Skills.
- Allow an installed public Skill to optionally target one project.
- Keep existing custom skills based on `memories.type = 'skill'` working.
- Include installed public Skills in `hub.sync_project` output under `.openagent/skills/`.
- Add Console APIs and UI for browsing public Skills and installing/uninstalling them.
- Add read-only MCP tools for listing and searching available/installed Skills.
- Avoid Agent self-install by default.

Non-goals for P0:

- No external marketplace publishing flow.
- No community submission/review workflow.
- No automatic upgrades.
- No paid catalog or billing integration.
- No vector embeddings for Skills.
- No Prompt/Resource implementation beyond leaving extension points.

## 3. Data Model

### 3.1 Public Skill Template

New table: `public_skill_templates`

Fields:

| Field | Type | Notes |
|---|---|---|
| `id` | string | UUID, `BaseModel` |
| `slug` | string | stable public identifier, unique |
| `name` | string | display name |
| `description` | string | short summary |
| `content` | text | full `SKILL.md` body content |
| `category` | string | e.g. `coding`, `review`, `debugging`, `frontend`, `backend`, `deployment` |
| `tags` | text | JSON array string, same style as current models |
| `version` | int | monotonically increasing template version |
| `risk_level` | string | `low`, `medium`, `high` |
| `visibility` | string | P0 only `public`; reserved for `private`, `org` |
| `source` | string | `platform`, `imported`, `manual` |
| `status` | string | `active`, `draft`, `archived` |

Indexes:

- unique index on `slug`
- index on `status`
- index on `category`

### 3.2 Skill Install

New table: `skill_installs`

Fields:

| Field | Type | Notes |
|---|---|---|
| `id` | string | UUID, `BaseModel` |
| `workspace_id` | string | required tenant boundary |
| `project_id` | nullable string | nil means workspace-wide install |
| `template_id` | string | references `public_skill_templates.id` |
| `installed_version` | int | version locked at install/upgrade time |
| `state` | string | `active`, `disabled`, `archived` |
| `pinned` | bool | sorting and sync priority |
| `override_content` | text | installed content snapshot; P0 uses it to keep the installed version stable |
| `installed_by` | string | user id |
| `installed_at` | time | install timestamp |
| `upgraded_at` | nullable time | last upgrade timestamp |

Indexes and constraints:

- unique logical key: `(workspace_id, project_id, template_id)` where possible
- index on `(workspace_id, state)`
- index on `(workspace_id, project_id, state)`

SQLite note:

- SQLite partial unique constraints are awkward through GORM tags. P0 can enforce uniqueness in handlers before insert and add a normal non-unique index.

## 4. Compatibility With Existing Custom Skills

Existing custom skills remain stored as `memories` rows:

- `type = 'skill'`
- `category = 'procedural'`
- `scope = 'workspace'` or `scope = 'project'`

The existing Skills page remains the custom Skill management page. Public catalog installs are additive.

In `hub.sync_project`, generated skill files come from two sources:

1. custom active skills from `memories`
2. installed active public skills from `skill_installs` joined with `public_skill_templates`

File path convention:

- custom skill: `.openagent/skills/custom/<slug>-<memory_id_prefix>/SKILL.md`
- public install: `.openagent/skills/public/<slug>-v<installed_version>-<install_id_prefix>/SKILL.md`

`skills/index.json` should include both types:

```json
[
  {
    "dir": "public/go-code-review-v1-0a1b2c3d",
    "name": "Go Code Review",
    "source": "public",
    "template_id": "uuid",
    "install_id": "uuid",
    "version": 1
  },
  {
    "dir": "custom/local-debug-flow-abc12345",
    "name": "Local Debug Flow",
    "source": "custom",
    "memory_id": "uuid",
    "version": 1
  }
]
```

## 5. Backend API

All APIs are under authenticated Console REST API and use the existing `{ code, message, data }` response wrapper.

### 5.1 Public Skills

`GET /api/public-skills`

Query:

- `category`
- `keyword`
- `status` default `active`
- `installed` optional `true|false`

Returns:

- list of templates with optional install metadata for current workspace.

`GET /api/public-skills/:id`

Returns:

- template detail
- current workspace install records

### 5.2 Skill Installs

`GET /api/skill-installs`

Query:

- `project_id`
- `state`

Returns:

- installed public skills for current workspace.

`POST /api/skill-installs`

Body:

```json
{
  "template_id": "uuid",
  "project_id": "optional uuid",
  "pinned": false
}
```

Behavior:

- verify template exists and is `active`
- verify `project_id` belongs to current workspace when present
- reject duplicate install for same `(workspace_id, project_id, template_id)`
- store `installed_version = template.version`
- store `override_content = template.content` as the installed content snapshot
- audit action `skill.install`

`PUT /api/skill-installs/:id/state`

Body:

```json
{ "state": "active" }
```

Allowed state values:

- `active`
- `disabled`
- `archived`

Behavior:

- only current workspace install can be changed
- audit action `skill.install_state_change`

`POST /api/skill-installs/:id/upgrade`

P0 optional. If implemented:

- set `installed_version = template.version`
- set `override_content = template.content`
- audit action `skill.install_upgrade`

## 6. MCP Tools

Add read-only tools:

### 6.1 `hub.list_skills`

Purpose:

- list custom active skills and installed public active skills available to current workspace/project.

Inputs:

```json
{
  "project_id": "optional",
  "project_path": "optional",
  "source": "all|custom|public",
  "limit": 50
}
```

Returns:

```json
{
  "skills": [
    {
      "source": "public",
      "name": "Go Code Review",
      "description": "...",
      "content": "...",
      "version": 1,
      "tags": ["go", "review"]
    }
  ]
}
```

### 6.2 `hub.search_skills`

Purpose:

- lexical search over available custom skills and installed public skills.

Inputs:

```json
{
  "query": "go review",
  "project_id": "optional",
  "project_path": "optional",
  "source": "all|custom|public",
  "limit": 10
}
```

Search implementation:

- reuse `services.Relevance` from `textsearch.go`
- sort by relevance desc, pinned desc, name asc

### 6.3 Agent Install Is Not P0

Do not add `hub.install_skill` in P0. Installation is a configuration/admin action and should be controlled through Console first.

## 7. Sync Behavior

Update `services.BuildSyncBundle` and `renderSkillFiles`:

- include active installed public skills for the workspace
- when project is bound, include:
  - workspace-wide installs (`project_id IS NULL`)
  - installs for the bound project
- when project is not bound, include only workspace-wide installs
- keep deterministic sorting:
  - `source` asc
  - `pinned` desc
  - `name` asc
  - `id` asc

ETag must include installed public skill content/version so local snapshots refresh after install, disable, archive, or upgrade.

## 8. Frontend

### 8.1 Navigation

Under Memory Hub:

- keep existing `Skills` page for custom skills
- add `Public Skills` page for catalog/install management

### 8.2 Public Skills Page

Primary controls:

- keyword search
- category filter
- installed filter
- table/list with name, description, category, tags, risk level, version, installed status
- install button
- disable/archive installed button

Install modal:

- select scope:
  - workspace-wide
  - project-specific
- project selector when project-specific
- pinned switch

### 8.3 API Client and Types

Add frontend types:

- `PublicSkillTemplate`
- `SkillInstall`
- `PublicSkillWithInstall`

Add APIs:

- `publicSkillApi.list`
- `publicSkillApi.get`
- `skillInstallApi.list`
- `skillInstallApi.create`
- `skillInstallApi.changeState`
- `skillInstallApi.upgrade` if P0 optional is implemented

## 9. Seed Data

P0 should seed a small platform catalog during bootstrap or a versioned migration.

Recommended initial templates:

1. `go-service-review`
   - category: `backend`
   - risk: `low`
   - content: checklist for Go service code review

2. `react-frontend-review`
   - category: `frontend`
   - risk: `low`
   - content: checklist for React/Vite UI review

3. `local-debug-evidence`
   - category: `debugging`
   - risk: `medium`
   - content: workflow requiring logs, config, DB, and command evidence before conclusion

4. `deployment-checklist`
   - category: `deployment`
   - risk: `medium`
   - content: deployment verification checklist

Seed behavior:

- use stable `slug`
- upsert by `slug`
- do not overwrite user-installed records
- if content changes, increment template `version`

## 10. Audit and Security

Audit actions:

- `skill.install`
- `skill.install_state_change`
- `skill.install_upgrade`
- `public_skill.create` reserved for future admin UI
- `public_skill.update` reserved for future admin UI

Security rules:

- public templates must not include tenant-private data
- public templates with `risk_level = high` should not be installable in P0 unless explicitly allowed later
- installed skills only appear inside the installing workspace
- project-specific installs must verify project ownership by `workspace_id`

## 11. Migration Plan

Add models:

- `PublicSkillTemplate`
- `SkillInstall`

Add `AutoMigrate` entries.

Add versioned migration:

- `20260617_01_seed_public_skill_templates`

The migration should:

- create/update template rows by slug
- leave installs unchanged
- be idempotent

No migration is needed for existing custom skills.

## 12. Tests

Backend unit/integration tests:

- list public skills only returns active templates by default
- install rejects missing/inactive/high-risk template
- install rejects duplicate in same workspace/project scope
- install rejects project from another workspace
- installed public skills appear in `BuildSyncBundle`
- disabled/archived installs do not appear in `BuildSyncBundle`
- `hub.list_skills` includes custom and public sources
- `hub.search_skills` ranks matching skills

Frontend validation:

- `npm run build`

Backend validation:

- `go test ./...`

## 13. Risks and Side Effects

| Risk | Impact | Mitigation |
|---|---|---|
| Context bloat | Too many installed skills increase local snapshot size and model context pressure | P0 list is install-based, not auto-enabled globally |
| Unsafe public instructions | Public templates can change Agent behavior | add `risk_level`, status, manual install, no auto-upgrade |
| Tenant leakage | Public catalog might accidentally include private content | separate public template table from workspace memories |
| Version drift | Users may unknowingly run stale Skills | show `installed_version` and available version in UI |
| Sync churn | Install changes affect `.openagent` ETag | deterministic render and ETag include version/hash |
| UX confusion | Existing Skills page and Public Skills page overlap | label existing page as custom skills; public page as install catalog |

## 14. P0 Implementation Checklist

1. Add models and migrations.
2. Add public skill seed migration.
3. Add REST handlers and routes.
4. Add sync rendering for installed public skills.
5. Add read-only MCP tools: `hub.list_skills`, `hub.search_skills`.
6. Add frontend types and API clients.
7. Add Public Skills page and navigation entry.
8. Add backend tests.
9. Run `go test ./...`.
10. Run `npm run build`.

## 15. Open Decisions

Before implementation, confirm:

1. P0 should keep existing custom Skills page unchanged and add a separate Public Skills page.
2. P0 should not allow Agent-side skill installation through MCP.
3. Public Skill seed templates can be English-first for `SKILL.md` content, with Chinese UI labels.
4. High-risk public skills should be visible but not installable in P0, or not seeded at all.
