// Backend type definitions, aligned with Go GORM models
// All IDs are UUID strings

export interface BaseModel {
  id: string
  created_at: string
  updated_at: string
}

export interface Organization extends BaseModel {
  name: string
  slug: string
  plan: string
  status: string
}

export interface Workspace extends BaseModel {
  org_id: string
  name: string
  slug: string
  type: 'personal' | 'team' | string
  quota_memory_count: number
  quota_tool_call_daily: number
  status: string
}

export interface User extends BaseModel {
  username: string
  display_name: string
  avatar_url: string
  status: string
  last_login_at: string | null
}

export interface WorkspaceMember extends BaseModel {
  workspace_id: string
  user_id: string
  role: 'owner' | 'admin' | 'member' | 'viewer' | string
  status: 'active' | 'pending' | 'rejected' | string
  invited_by?: string
  invited_at: string
  joined_at: string | null
  user?: User
}

export interface Invitation extends WorkspaceMember {
  workspace?: Workspace
}

export interface Project extends BaseModel {
  org_id: string
  workspace_id: string
  name: string
  slug: string
  description: string
  stack: string
  structure: string
  status: string
  repo_path: string
  git_remote: string
  repo_name: string
}

export interface SyncRecord extends BaseModel {
  org_id: string
  workspace_id: string
  project_id: string
  user_id: string
  agent_client_id: string
  client: string
  client_name: string
  repo_path: string
  etag: string
  sync_count: number
  synced_at: string
  user_display_name: string
  username: string
  stale: boolean
}

export interface SyncRecordsResult {
  records: SyncRecord[]
  current_etag: string
}

export interface Rule extends BaseModel {
  org_id: string
  workspace_id: string
  project_id: string | null
  agent_name: string | null
  name: string
  description: string
  value: string
  type: string // workspace_policy | output_preference | global_rule | project_rule | ...
  tags: string // JSON string
  scope: 'workspace' | 'project' | string
  version: number
}

export interface PersonalInstructions {
  language: 'zh-CN' | 'en-US' | 'auto' | string
  verbosity: 'concise' | 'normal' | 'detailed' | string
  code_style: 'google' | 'standard' | 'project' | 'custom' | string
  personality: 'pragmatic' | 'concise' | 'rigorous' | 'friendly' | 'custom' | string
  response_style: 'direct' | 'explanatory' | 'checklist' | string
  custom_instructions: string
  memory: {
    enabled: boolean
    skip_tool_context: boolean
  }
  updated_at?: string
}

export interface Memory extends BaseModel {
  org_id: string
  workspace_id: string
  project_id: string | null
  user_id: string
  content: string
  type: string // fact | preference | lesson | progress | context | note ...
  category: 'workspace' | 'project' | string
  tags: string // JSON string
  scope: 'workspace' | 'project' | string
  provenance: 'human_curated' | 'agent_proposed' | 'imported' | string
  importance: number
  pinned: boolean
  state: 'active' | 'archived' | 'pending_review' | 'rejected' | string
  access_count: number
  last_access_at: string | null
  char_count: number
  version: number
  similarity?: number
}

export interface PublicSkillTemplate extends BaseModel {
  slug: string
  name: string
  description: string
  content: string
  category: string
  tags: string
  version: number
  risk_level: 'low' | 'medium' | 'high' | string
  visibility: string
  source: string
  status: 'active' | 'draft' | 'archived' | string
  installs?: SkillInstall[]
  installed?: boolean
  installed_version?: number
}

export interface SkillInstall extends BaseModel {
  workspace_id: string
  project_id: string | null
  template_id: string
  installed_version: number
  state: 'active' | 'disabled' | 'archived' | string
  pinned: boolean
  override_content?: string
  installed_by: string
  installed_at: string
  upgraded_at: string | null
  template?: PublicSkillTemplate
  project?: Project
}

export interface APIKey extends BaseModel {
  workspace_id: string
  name: string
  prefix: string
  scopes: string // JSON string
  last_used_at: string | null
  expires_at: string | null
  created_by: string
  revoked_at: string | null
}

export interface APIKeyWithToken extends APIKey {
  token: string // only returned on creation
}

export interface AgentClient extends BaseModel {
  workspace_id: string
  user_id: string
  client_type: string // cursor | claude_code | windsurf | vscode | custom
  client_name: string
  client_version: string
  project_id: string | null
  install_path: string
  status: 'active' | 'inactive' | string
  first_seen_at: string
  last_seen_at: string | null
}

export interface ConnectedMCPServer extends BaseModel {
  workspace_id: string
  name: string
  display_name: string
  endpoint: string
  transport: 'streamable_http' | 'sse' | 'stdio' | string
  auth_type: 'none' | 'bearer' | 'api_key' | 'oauth' | string
  tools_json: string
  policy_json: string
  status: 'pending' | 'active' | 'error' | string
  last_health_check_at: string | null
}

export interface ToolPolicy extends BaseModel {
  workspace_id: string
  connected_server_id: string
  tool_name: string
  allowed: boolean
  requires_confirmation: boolean
  max_calls_per_day: number
  max_calls_per_user: number
  risk_level: 'low' | 'medium' | 'high' | string
}

export interface ToolInvocationLog extends BaseModel {
  workspace_id: string
  user_id: string
  agent_client_id: string
  mcp_session_id: string
  tool_name: string
  connected_server_id: string | null
  input_json: string
  output_summary: string
  status: 'success' | 'error' | string
  error_code: string
  error_message: string
  latency_ms: number
  confirmed: boolean
  invoked_at: string
}

export interface AuditLog extends BaseModel {
  workspace_id: string
  actor: string
  actor_type: 'user' | 'api_key' | 'agent' | 'system' | string
  action: string
  target: string
  target_type: string
  payload: string
  client_ip: string
}

export interface UsageRecord extends BaseModel {
  workspace_id: string
  user_id: string
  metric: string
  quantity: number
  period: string
  recorded_at: string
}

export interface UsageDashboard {
  today_calls: number
  month_calls: number
  active_sessions: number
  memory_writes: number
  memory_total: number
  trend_7d: { date: string; count: number }[]
  top_tools: { tool: string; count: number }[]
}

export interface LoginResponse {
  token: string
  expires_at: string
  user: User
  workspace: Workspace | null
  org: Organization | null
  role: string
  workspaces: Workspace[]
}

export interface MeResponse {
  user: User
  workspace: Workspace | null
  org: Organization | null
  role: string
  workspaces: Workspace[]
}

export interface APIResp<T> {
  code: number
  message: string
  data: T
}
