import http from './http'
import type {
  LoginResponse,
  MeResponse,
  User,
  Workspace,
  WorkspaceMember,
  Invitation,
  Rule,
  Memory,
  APIKey,
  APIKeyWithToken,
  AgentClient,
  ConnectedMCPServer,
  ToolPolicy,
  ToolInvocationLog,
  AuditLog,
  Project,
  UsageDashboard,
  PublicSkillTemplate,
  SkillInstall,
  SyncRecordsResult,
  PersonalInstructions
} from '@/types'

// ============ Auth ============
export const authApi = {
  login: (username: string, password: string) =>
    http.post<unknown, LoginResponse>('/auth/login', { username, password }),
  register: (data: { username: string; password: string; display_name?: string }) =>
    http.post<unknown, LoginResponse>('/auth/register', data),
  me: () => http.get<unknown, MeResponse>('/auth/me'),
  switchWorkspace: (workspace_id: string) =>
    http.post<unknown, LoginResponse>('/auth/switch-workspace', { workspace_id })
}

// ============ Workspaces ============
export const workspaceApi = {
  list: () => http.get<unknown, Workspace[]>('/workspaces'),
  get: (id: string) => http.get<unknown, Workspace>(`/workspaces/${id}`),
  create: (data: { name: string; slug?: string }) =>
    http.post<unknown, LoginResponse>('/workspaces', data),
  update: (id: string, data: Partial<Workspace>) =>
    http.put<unknown, Workspace>(`/workspaces/${id}`, data),
  delete: (id: string) => http.delete<unknown, null>(`/workspaces/${id}`),
  leave: (id: string) => http.post<unknown, null>(`/workspaces/${id}/leave`, {})
}

// ============ Members ============
export const memberApi = {
  list: () => http.get<unknown, WorkspaceMember[]>('/members'),
  invite: (data: { username: string; role: string }) =>
    http.post<unknown, WorkspaceMember>('/members', data),
  updateRole: (id: string, role: string) =>
    http.put<unknown, WorkspaceMember>(`/members/${id}`, { role }),
  remove: (id: string) => http.delete<unknown, null>(`/members/${id}`),
  myInvitations: () => http.get<unknown, Invitation[]>('/my-invitations'),
  acceptInvitation: (id: string) => http.post<unknown, null>(`/my-invitations/${id}/accept`),
  rejectInvitation: (id: string) => http.post<unknown, null>(`/my-invitations/${id}/reject`)
}

// ============ Projects ============
export const projectApi = {
  list: () => http.get<unknown, Project[]>('/projects'),
  get: (id: string) => http.get<unknown, Project>(`/projects/${id}`),
  create: (data: { name: string; slug: string; description?: string; stack?: string; structure?: string }) =>
    http.post<unknown, Project>('/projects', data),
  update: (id: string, data: Partial<Project>) =>
    http.put<unknown, Project>(`/projects/${id}`, data),
  delete: (id: string) => http.delete<unknown, null>(`/projects/${id}`),
  syncRecords: (id: string) =>
    http.get<unknown, SyncRecordsResult>(`/projects/${id}/sync-records`)
}

// ============ Rules ============
export const ruleApi = {
  list: (params?: { scope?: string; type?: string; project_id?: string }) =>
    http.get<unknown, Rule[]>('/rules', { params }),
  get: (id: string) => http.get<unknown, Rule>(`/rules/${id}`),
  create: (data: Partial<Rule>) => http.post<unknown, Rule>('/rules', data),
  update: (id: string, data: Partial<Rule>) => http.put<unknown, Rule>(`/rules/${id}`, data),
  delete: (id: string) => http.delete<unknown, null>(`/rules/${id}`),
  globalRules: () => http.get<unknown, Rule[]>('/rules/global'),
  projectRules: (project_id: string) =>
    http.get<unknown, Rule[]>('/rules/project', { params: { project_id } }),
  workspacePolicy: () => http.get<unknown, unknown>('/workspace-policy'),
  outputPreferences: () => http.get<unknown, Record<string, string>>('/output-preferences')
}

export const personalInstructionsApi = {
  get: () => http.get<unknown, PersonalInstructions>('/personal-instructions'),
  update: (data: PersonalInstructions) =>
    http.put<unknown, PersonalInstructions>('/personal-instructions', data)
}

// ============ Memories ============
export const memoryApi = {
  list: (params?: {
    scope?: string
    type?: string
    state?: string
    category?: string
    project_id?: string
    keyword?: string
    limit?: number
  }) => http.get<unknown, Memory[]>('/memories', { params }),
  get: (id: string) => http.get<unknown, Memory>(`/memories/${id}`),
  create: (data: Partial<Memory>) => http.post<unknown, Memory>('/memories', data),
  update: (id: string, data: Partial<Memory>) => http.put<unknown, Memory>(`/memories/${id}`, data),
  archive: (id: string) => http.post<unknown, Memory>(`/memories/${id}/archive`),
  delete: (id: string) => http.delete<unknown, null>(`/memories/${id}`),
  search: (data: { query: string; top_k?: number; scope?: string; type?: string }) =>
    http.post<unknown, Memory[]>('/memories/search', data),
  stats: () => http.get<unknown, Record<string, number>>('/memories/stats')
}

// ============ Skills ============
export const skillApi = {
  list: (params?: { state?: string }) =>
    http.get<unknown, Memory[]>('/skills', { params }),
  create: (data: { content: string; tags?: string; importance?: number; pinned?: boolean }) =>
    http.post<unknown, Memory>('/skills', data),
  changeState: (id: string, state: 'active' | 'stale' | 'archived', reason?: string) =>
    http.put<unknown, Memory>(`/skills/${id}/state`, { state, reason })
}

// ============ Public Skills ============
export type PublicSkillPayload = {
  slug: string
  name: string
  description?: string
  content: string
  category: string
  tags?: string[]
  risk_level: 'low' | 'medium' | 'high'
  status: 'draft' | 'active' | 'archived'
}

export const publicSkillApi = {
  list: (params?: { category?: string; keyword?: string; status?: string; installed?: boolean }) =>
    http.get<unknown, PublicSkillTemplate[]>('/public-skills', { params }),
  get: (id: string) => http.get<unknown, PublicSkillTemplate>(`/public-skills/${id}`),
  create: (data: PublicSkillPayload) =>
    http.post<unknown, PublicSkillTemplate>('/public-skills', data),
  update: (id: string, data: PublicSkillPayload) =>
    http.put<unknown, PublicSkillTemplate>(`/public-skills/${id}`, data),
  changeStatus: (id: string, status: 'draft' | 'active' | 'archived') =>
    http.put<unknown, PublicSkillTemplate>(`/public-skills/${id}/status`, { status })
}

// ============ Public Skill Installs ============
export const skillInstallApi = {
  list: (params?: { project_id?: string; state?: string }) =>
    http.get<unknown, SkillInstall[]>('/skill-installs', { params }),
  create: (data: { template_id: string; project_id?: string; pinned?: boolean }) =>
    http.post<unknown, SkillInstall>('/skill-installs', data),
  changeState: (id: string, state: 'active' | 'disabled' | 'archived') =>
    http.put<unknown, SkillInstall>(`/skill-installs/${id}/state`, { state }),
  upgrade: (id: string) => http.post<unknown, SkillInstall>(`/skill-installs/${id}/upgrade`)
}

// ============ Tokens ============
export const tokenApi = {
  list: () => http.get<unknown, APIKey[]>('/tokens'),
  create: (data: { name: string; scopes?: string[]; expires_in_days?: number }) =>
    http.post<unknown, APIKeyWithToken>('/tokens', data),
  revoke: (id: string) => http.post<unknown, APIKey>(`/tokens/${id}/revoke`),
  delete: (id: string) => http.delete<unknown, null>(`/tokens/${id}`)
}

// ============ Agent Clients ============
export const agentClientApi = {
  list: () => http.get<unknown, AgentClient[]>('/agent-clients'),
  get: (id: string) => http.get<unknown, AgentClient>(`/agent-clients/${id}`),
  delete: (id: string) => http.delete<unknown, null>(`/agent-clients/${id}`)
}

// ============ Connected MCP Servers ============
export const connectedServerApi = {
  list: () => http.get<unknown, ConnectedMCPServer[]>('/connected-servers'),
  get: (id: string) => http.get<unknown, ConnectedMCPServer>(`/connected-servers/${id}`),
  create: (data: Partial<ConnectedMCPServer>) =>
    http.post<unknown, ConnectedMCPServer>('/connected-servers', data),
  update: (id: string, data: Partial<ConnectedMCPServer>) =>
    http.put<unknown, ConnectedMCPServer>(`/connected-servers/${id}`, data),
  delete: (id: string) => http.delete<unknown, null>(`/connected-servers/${id}`)
}

// ============ Tool Policies ============
export const toolPolicyApi = {
  list: (params?: { connected_server_id?: string; tool_name?: string }) =>
    http.get<unknown, ToolPolicy[]>('/tool-policies', { params }),
  update: (id: string, data: Partial<ToolPolicy>) =>
    http.put<unknown, ToolPolicy>(`/tool-policies/${id}`, data)
}

// ============ Tool Invocation Logs ============
export const toolInvocationApi = {
  list: (params?: { tool_name?: string; status?: string; limit?: number; agent_client_id?: string }) =>
    http.get<unknown, ToolInvocationLog[]>('/tool-invocation-logs', { params })
}

// ============ Audit ============
export const auditApi = {
  list: (params?: { action?: string; actor_type?: string; limit?: number }) =>
    http.get<unknown, AuditLog[]>('/audit-logs', { params })
}

// ============ Usage ============
export const usageApi = {
  dashboard: () => http.get<unknown, UsageDashboard>('/usage/dashboard')
}

// ============ Direct MCP call (for inspector) ============
export const mcpApi = {
  call: (token: string | null, body: { jsonrpc: '2.0'; id: number | string; method: string; params?: unknown }) => {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    if (token) {
      headers.Authorization = `Bearer ${token}`
    }
    return fetch('/mcp', {
      method: 'POST',
      headers,
      body: JSON.stringify(body)
    }).then((r) => r.json())
  }
}

// Re-export
export type {
  LoginResponse,
  MeResponse,
  User,
  Workspace,
  WorkspaceMember,
  Invitation,
  Rule,
  Memory,
  APIKey,
  APIKeyWithToken,
  AgentClient,
  ConnectedMCPServer,
  ToolPolicy,
  ToolInvocationLog,
  AuditLog,
  Project,
  UsageDashboard,
  PublicSkillTemplate,
  SkillInstall,
  SyncRecordsResult
}
