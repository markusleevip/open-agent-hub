# Personal Instructions Spec

## Status

Draft for review.

## Background

Open Agent Hub already has several context channels:

- `rules` table for workspace/project/agent rules.
- `output_preferences` table for per-user output preferences.
- `hub.get_output_preferences` MCP tool.
- `hub.get_project_context`, which includes output preferences.
- `hub.sync_project`, which renders `.openagent/local/profile.md`.

There is currently a product gap: the console does not provide a first-class "personal instructions" page similar to Codex personalization settings. The existing `Output Preferences` page uses the generic `RuleManager` and writes `rules.type = output_preference`, while the REST endpoint `/api/output-preferences` and MCP tool `hub.get_output_preferences` read the separate `output_preferences` table. This means console edits and agent-consumed preferences are not aligned.

## Goals

1. Add a first-class user-scoped Personal Instructions page.
2. Store personal instructions in the data path consumed by REST, MCP, and sync output.
3. Keep workspace/team rules separate from personal preferences.
4. Preserve a clear priority order so personal instructions cannot override safety or workspace policy.
5. Keep `.openagent/` as generated service-sync output and ignored by Git.

## Non-Goals

- Do not replace workspace/project/global rules.
- Do not make personal instructions shared across users by default.
- Do not allow personal instructions to bypass tool policy or security controls.
- Do not build collaborative editing or version history in P0.

## Priority Model

Effective instruction order must be:

1. Platform/system safety policy.
2. Workspace policy and tool policy.
3. Global rules.
4. Project rules.
5. Agent-specific rules.
6. Personal instructions and output preferences.
7. Memories and skills.

If personal instructions conflict with higher-priority policy, higher-priority policy wins.

## Data Model

P0 should reuse `output_preferences`.

Current model:

```go
type OutputPreference struct {
    BaseModel
    UserID      string
    WorkspaceID string
    Key         string
    Value       string
}
```

Required logical keys:

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `language` | string | `zh-CN` | Default response language. |
| `verbosity` | string | `normal` | Output detail level: `concise`, `normal`, `detailed`. |
| `personality` | string | `pragmatic` | Default assistant style/personality. |
| `custom_instructions` | text | empty | Free-form personal instructions. |
| `memory_enabled` | bool-string | `true` | Whether agent memory proposal is enabled. |
| `skip_tool_memory` | bool-string | `true` | Whether tool/web-assisted conversations should be skipped for memory generation. |
| `response_style` | string | `direct` | High-level answer style. |

Recommended DB constraint:

```text
unique(workspace_id, user_id, key)
```

If adding this constraint is risky for existing SQLite/Postgres data, P0 can enforce uniqueness in service code first and add a migration after deduplicating rows.

## REST API

Add a dedicated handler instead of using `RuleManager`.

### `GET /api/personal-instructions`

Returns merged defaults plus stored values for current `workspace_id + user_id`.

Response:

```json
{
  "language": "zh-CN",
  "verbosity": "normal",
  "personality": "pragmatic",
  "response_style": "direct",
  "custom_instructions": "",
  "memory": {
    "enabled": true,
    "skip_tool_context": true
  },
  "updated_at": "2026-06-17T23:00:00Z"
}
```

### `PUT /api/personal-instructions`

Validates and upserts all supported keys.

Request:

```json
{
  "language": "zh-CN",
  "verbosity": "normal",
  "personality": "pragmatic",
  "response_style": "direct",
  "custom_instructions": "默认中文回复，代码和命令保持原文。",
  "memory": {
    "enabled": true,
    "skip_tool_context": true
  }
}
```

Validation:

- `language`: `zh-CN`, `en-US`, `auto`.
- `verbosity`: `concise`, `normal`, `detailed`.
- `personality`: `pragmatic`, `concise`, `rigorous`, `friendly`, `custom`.
- `response_style`: `direct`, `explanatory`, `checklist`.
- `custom_instructions`: max 20,000 characters in P0.
- booleans in `memory` required when present.

Audit:

- action: `personal_instructions.update`
- target type: `output_preference`
- payload should include changed keys, but not full custom instruction text if it may contain sensitive data. Store content length/hash instead.

## MCP Behavior

### `hub.get_output_preferences`

Return all personal instruction keys, not only `language`, `verbosity`, and `code_style`.

Example:

```json
{
  "language": "zh-CN",
  "verbosity": "normal",
  "code_style": "google",
  "personality": "pragmatic",
  "response_style": "direct",
  "custom_instructions": "默认中文回复，代码和命令保持原文。",
  "memory_enabled": "true",
  "skip_tool_memory": "true"
}
```

### `hub.get_project_context`

Continue including `output_preferences`, but make sure it uses the same defaults and keys as `hub.get_output_preferences`.

Recommended implementation:

- Add a shared service helper, for example `services.GetOutputPreferences(workspaceID, userID)`.
- Use the helper in REST, MCP, and sync rendering to avoid drift.

## Sync Behavior

`.openagent/` is generated output from the service and must be ignored by Git.

Root `.gitignore` must include:

```gitignore
.openagent/
```

`hub.sync_project` should render personal instructions into:

```text
.openagent/local/profile.md
```

Suggested output:

```md
<!-- Generated by Open Agent Hub. Do not edit; manage in the console and refresh via `openagent sync` or hub.sync_project. -->

# Personal Profile

## Output Preferences

- language: zh-CN
- verbosity: normal
- personality: pragmatic
- response_style: direct

## Memory Preferences

- memory_enabled: true
- skip_tool_memory: true

## Custom Instructions

默认中文回复，代码和命令保持原文。
```

`local/profile.md` stays under `.openagent/local/`; it is personal and should not be committed.

## Frontend UX

Add a first-class page:

```text
Context Hub -> Personal Instructions
```

Suggested route:

```text
/context/personal-instructions
```

Fields:

- Personality select.
- Language select.
- Verbosity select.
- Response style select.
- Custom instructions textarea.
- Memory enabled switch.
- Skip tool-assisted memory switch.
- Save button.

UX rules:

- Explain that these settings are personal and user-scoped.
- Show that workspace/project rules still take priority.
- Disable save while unchanged.
- Show validation for overly long custom instructions.
- Use a full-page form, not the generic rules table.

## Migration Plan

P0 migration can be additive:

1. Keep existing `output_preferences` table.
2. Add service-level upsert by `workspace_id + user_id + key`.
3. Optionally backfill existing `rules.type = output_preference` rows into `output_preferences`.
4. After verifying duplicates are resolved, add DB unique constraint in a later migration.

Backfill rule:

- `Rule.Name` maps to `OutputPreference.Key`.
- `Rule.Value` maps to `OutputPreference.Value`.
- Only workspace-scoped rows with `type = output_preference`.
- If a key already exists in `output_preferences`, do not overwrite in P0; report conflict count in logs.

## Testing

Backend:

- `GET /api/personal-instructions` returns defaults for a new user.
- `PUT /api/personal-instructions` upserts values.
- Repeated `PUT` updates existing keys instead of creating duplicates.
- `hub.get_output_preferences` returns saved custom instructions.
- `hub.get_project_context` includes the same values.
- `hub.sync_project` renders `.openagent/local/profile.md`.

Frontend:

- Page loads defaults.
- Save updates values.
- Save button disables while unchanged.
- Long custom instruction shows validation.
- Build passes.

Manual QA:

- Configure custom instructions in console.
- Call MCP Inspector `hub.get_output_preferences`.
- Run `hub.sync_project` and inspect returned `.openagent/local/profile.md`.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Personal instructions conflict with policy | Agent behavior drift | Enforce priority order and explain it in UI. |
| Sensitive content in audit logs | Privacy issue | Audit changed keys and length/hash, not full text. |
| Existing OutputPreferences page writes wrong table | Agent does not consume settings | Replace or redirect the page to new Personal Instructions implementation. |
| Duplicate keys in `output_preferences` | Undefined preference values | Use service-level upsert and later DB constraint. |
| `.openagent/` accidentally committed | Personal/team sync output leaks | Keep `.openagent/` in root `.gitignore`. |

## Rollout

1. Review and approve this spec.
2. Implement backend service helper and REST handler.
3. Update MCP tools and sync profile rendering.
4. Add frontend page and menu entry.
5. Keep existing `Output Preferences` route as redirect or compatibility wrapper.
6. Add tests and run `go test ./...`, `npm run build`.
