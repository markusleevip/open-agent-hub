package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel is the base model
type BaseModel struct {
	ID        string         `gorm:"primarykey;type:text" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate GORM hook - auto-generates UUID
func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	return nil
}

// ============================================================
// Tenant models
// ============================================================

// Organization represents an organization
type Organization struct {
	BaseModel
	Name   string `gorm:"type:varchar(128);not null" json:"name"`
	Slug   string `gorm:"type:varchar(64);uniqueIndex" json:"slug"`
	Plan   string `gorm:"type:varchar(32);not null;default:'free'" json:"plan"`
	Status string `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
}

func (Organization) TableName() string { return "organizations" }

// Workspace represents a workspace
type Workspace struct {
	BaseModel
	OrgID              string `gorm:"type:text;not null;index" json:"org_id"`
	Name               string `gorm:"type:varchar(128);not null" json:"name"`
	Slug               string `gorm:"type:varchar(64);not null" json:"slug"`
	Type               string `gorm:"type:varchar(16);not null;default:'team'" json:"type"`
	QuotaMemoryCount   int    `gorm:"not null;default:10000" json:"quota_memory_count"`
	QuotaToolCallDaily int    `gorm:"not null;default:5000" json:"quota_tool_call_daily"`
	Status             string `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
}

func (Workspace) TableName() string { return "workspaces" }

// User represents a user
type User struct {
	BaseModel
	Username     string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"username"`
	PasswordHash string     `gorm:"type:varchar(255);not null" json:"-"`
	DisplayName  string     `gorm:"type:varchar(128)" json:"display_name"`
	AvatarURL    string     `gorm:"type:varchar(512)" json:"avatar_url"`
	Status       string     `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

func (User) TableName() string { return "users" }

// WorkspaceMember represents a workspace member
type WorkspaceMember struct {
	BaseModel
	WorkspaceID string     `gorm:"type:text;not null;index" json:"workspace_id"`
	UserID      string     `gorm:"type:text;not null;index" json:"user_id"`
	Role        string     `gorm:"type:varchar(16);not null;default:'member'" json:"role"`
	Status      string     `gorm:"type:varchar(16);not null;default:'active';index" json:"status"`
	InvitedBy   string     `gorm:"type:text" json:"invited_by"`
	InvitedAt   time.Time  `json:"invited_at"`
	JoinedAt    *time.Time `json:"joined_at"`
}

func (WorkspaceMember) TableName() string { return "workspace_members" }

// ============================================================
// Rules & Configuration
// ============================================================

// Rule represents a rule
type Rule struct {
	BaseModel
	OrgID       string  `gorm:"type:text;not null;index" json:"org_id"`
	WorkspaceID string  `gorm:"type:text;not null;index" json:"workspace_id"`
	ProjectID   *string `gorm:"type:text;index" json:"project_id"`
	AgentName   *string `gorm:"type:varchar(64);index" json:"agent_name"`
	Name        string  `gorm:"type:varchar(128);not null" json:"name"`
	Description string  `gorm:"type:varchar(512)" json:"description"`
	Value       string  `gorm:"type:text;not null" json:"value"`
	Type        string  `gorm:"type:varchar(32);not null;index" json:"type"`
	Tags        string  `gorm:"type:text;default:'[]'" json:"tags"`
	Scope       string  `gorm:"type:varchar(16);not null;default:'workspace'" json:"scope"`
	Version     int     `gorm:"not null;default:1" json:"version"`
}

func (Rule) TableName() string { return "rules" }

// OutputPreference represents output preferences
type OutputPreference struct {
	BaseModel
	UserID      string `gorm:"type:text;not null;index" json:"user_id"`
	WorkspaceID string `gorm:"type:text;not null;index" json:"workspace_id"`
	Key         string `gorm:"type:varchar(64);not null" json:"key"`
	Value       string `gorm:"type:text;not null" json:"value"`
}

func (OutputPreference) TableName() string { return "output_preferences" }

// ============================================================
// Memory System
// ============================================================

// Memory represents a memory entry
type Memory struct {
	BaseModel
	OrgID        string     `gorm:"type:text;not null;index" json:"org_id"`
	WorkspaceID  string     `gorm:"type:text;not null;index" json:"workspace_id"`
	ProjectID    *string    `gorm:"type:text;index" json:"project_id"`
	UserID       string     `gorm:"type:text;not null" json:"user_id"`
	Content      string     `gorm:"type:text;not null" json:"content"`
	Type         string     `gorm:"type:varchar(32);not null;index" json:"type"`
	Category     string     `gorm:"type:varchar(16);not null" json:"category"`
	Tags         string     `gorm:"type:text;default:'[]'" json:"tags"`
	Scope        string     `gorm:"type:varchar(16);not null;default:'workspace'" json:"scope"`
	Provenance   string     `gorm:"type:varchar(32);not null;default:'human_curated'" json:"provenance"`
	Importance   float64    `gorm:"not null;default:0.5" json:"importance"`
	Pinned       bool       `gorm:"not null;default:false;index" json:"pinned"`
	State        string     `gorm:"type:varchar(16);not null;default:'active'" json:"state"`
	AccessCount  int        `gorm:"not null;default:0" json:"access_count"`
	LastAccessAt *time.Time `json:"last_access_at"`
	CharCount    int        `gorm:"not null;default:0" json:"char_count"`
	Version      int        `gorm:"not null;default:1" json:"version"`
}

func (Memory) TableName() string { return "memories" }

// MemorySnapshot represents a memory snapshot
type MemorySnapshot struct {
	BaseModel
	WorkspaceID string    `gorm:"type:text;not null;index" json:"workspace_id"`
	SessionID   string    `gorm:"type:text;not null;uniqueIndex" json:"session_id"`
	Content     string    `gorm:"type:text;not null" json:"content"`
	CharCount   int       `gorm:"not null" json:"char_count"`
	MemoryIDs   string    `gorm:"type:text;default:'[]'" json:"memory_ids"`
	Version     string    `gorm:"type:varchar(32);not null" json:"version"`
	FrozenAt    time.Time `json:"frozen_at"`
}

func (MemorySnapshot) TableName() string { return "memory_snapshots" }

// MemoryAccessLog represents memory access logs
type MemoryAccessLog struct {
	BaseModel
	MemoryID    string    `gorm:"type:text;not null;index" json:"memory_id"`
	WorkspaceID string    `gorm:"type:text;not null" json:"workspace_id"`
	UserID      string    `gorm:"type:text;not null" json:"user_id"`
	QueryType   string    `gorm:"type:varchar(32)" json:"query_type"`
	Relevance   float64   `gorm:"type:double precision" json:"relevance"`
	AccessedAt  time.Time `gorm:"index" json:"accessed_at"`
}

func (MemoryAccessLog) TableName() string { return "memory_access_log" }

// MemoryValidity represents memory validity (dual time axis)
type MemoryValidity struct {
	BaseModel
	MemoryID     string     `gorm:"type:text;not null;index" json:"memory_id"`
	WorkspaceID  string     `gorm:"type:text;not null;index" json:"workspace_id"`
	ValidFrom    time.Time  `gorm:"not null" json:"valid_from"`
	ValidUntil   *time.Time `json:"valid_until"`
	RecordedAt   time.Time  `gorm:"not null" json:"recorded_at"`
	SupersededBy *string    `gorm:"type:text" json:"superseded_by"`
}

func (MemoryValidity) TableName() string { return "memory_validity" }

// MemoryMapping represents memory mappings (Local Bridge compatible)
type MemoryMapping struct {
	BaseModel
	MemoryID    string     `gorm:"type:text;not null;index" json:"memory_id"`
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

// SkillCurationLog represents skill curation logs
type SkillCurationLog struct {
	BaseModel
	SkillID   string    `gorm:"type:text;not null;index" json:"skill_id"`
	OldState  string    `gorm:"type:varchar(16);not null" json:"old_state"`
	NewState  string    `gorm:"type:varchar(16);not null" json:"new_state"`
	Reason    string    `gorm:"type:text;not null" json:"reason"`
	CuratedAt time.Time `json:"curated_at"`
}

func (SkillCurationLog) TableName() string { return "skill_curation_log" }

// PublicSkillTemplate represents platform public Skill templates.
type PublicSkillTemplate struct {
	BaseModel
	Slug        string `gorm:"type:varchar(96);not null;uniqueIndex" json:"slug"`
	Name        string `gorm:"type:varchar(128);not null" json:"name"`
	Description string `gorm:"type:varchar(512)" json:"description"`
	Content     string `gorm:"type:text;not null" json:"content"`
	Category    string `gorm:"type:varchar(32);not null;index" json:"category"`
	Tags        string `gorm:"type:text;default:'[]'" json:"tags"`
	Version     int    `gorm:"not null;default:1" json:"version"`
	RiskLevel   string `gorm:"type:varchar(16);not null;default:'low'" json:"risk_level"`
	Visibility  string `gorm:"type:varchar(16);not null;default:'public'" json:"visibility"`
	Source      string `gorm:"type:varchar(32);not null;default:'platform'" json:"source"`
	Status      string `gorm:"type:varchar(16);not null;default:'active';index" json:"status"`
}

func (PublicSkillTemplate) TableName() string { return "public_skill_templates" }

// SkillInstall represents the installation of a public Skill template in a workspace/project.
type SkillInstall struct {
	BaseModel
	WorkspaceID      string     `gorm:"type:text;not null;index" json:"workspace_id"`
	ProjectID        *string    `gorm:"type:text;index" json:"project_id"`
	TemplateID       string     `gorm:"type:text;not null;index" json:"template_id"`
	InstalledVersion int        `gorm:"not null;default:1" json:"installed_version"`
	State            string     `gorm:"type:varchar(16);not null;default:'active';index" json:"state"`
	Pinned           bool       `gorm:"not null;default:false;index" json:"pinned"`
	OverrideContent  string     `gorm:"type:text" json:"override_content"`
	InstalledBy      string     `gorm:"type:text" json:"installed_by"`
	InstalledAt      time.Time  `json:"installed_at"`
	UpgradedAt       *time.Time `json:"upgraded_at"`
}

func (SkillInstall) TableName() string { return "skill_installs" }

// ============================================================
// Agent & Session
// ============================================================

// AgentClient represents an agent client
type AgentClient struct {
	BaseModel
	WorkspaceID   string     `gorm:"type:text;not null;index" json:"workspace_id"`
	UserID        string     `gorm:"type:text;not null" json:"user_id"`
	ClientType    string     `gorm:"type:varchar(32);not null;index" json:"client_type"`
	ClientName    string     `gorm:"type:varchar(128)" json:"client_name"`
	ClientVersion string     `gorm:"type:varchar(32)" json:"client_version"`
	ProjectID     *string    `gorm:"type:text;index" json:"project_id"`
	InstallPath   string     `gorm:"type:varchar(512)" json:"install_path"`
	Status        string     `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
	FirstSeenAt   time.Time  `json:"first_seen_at"`
	LastSeenAt    *time.Time `json:"last_seen_at"`
}

func (AgentClient) TableName() string { return "agent_clients" }

// MCPSession represents an MCP session
type MCPSession struct {
	BaseModel
	WorkspaceID   string `gorm:"type:text;not null;index" json:"workspace_id"`
	UserID        string `gorm:"type:text;not null" json:"user_id"`
	AgentClientID string `gorm:"type:text;not null;index" json:"agent_client_id"`
	// ProjectID session-level project binding: written after hub.sync_project resolves/registers a project;
	// subsequent tool calls in the same session automatically inherit it.
	ProjectID       string     `gorm:"type:text;index" json:"project_id"`
	AccessTokenHash string     `gorm:"type:varchar(128);not null;index" json:"-"`
	Scopes          string     `gorm:"type:text;default:'[]'" json:"scopes"`
	Status          string     `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
	StartedAt       time.Time  `gorm:"index" json:"started_at"`
	LastActivityAt  *time.Time `json:"last_activity_at"`
	EndedAt         *time.Time `json:"ended_at"`
	ClientIP        string     `gorm:"type:varchar(64)" json:"client_ip"`
	UserAgent       string     `gorm:"type:varchar(512)" json:"user_agent"`
}

func (MCPSession) TableName() string { return "mcp_sessions" }

// ============================================================
// Tool Routing & Policies
// ============================================================

// ConnectedMCPServer represents an external MCP Server
type ConnectedMCPServer struct {
	BaseModel
	WorkspaceID       string     `gorm:"type:text;not null;index" json:"workspace_id"`
	Name              string     `gorm:"type:varchar(64);not null" json:"name"`
	DisplayName       string     `gorm:"type:varchar(128)" json:"display_name"`
	Endpoint          string     `gorm:"type:varchar(512);not null" json:"endpoint"`
	Transport         string     `gorm:"type:varchar(16);not null;default:'streamable_http'" json:"transport"`
	AuthType          string     `gorm:"type:varchar(16);not null;default:'none'" json:"auth_type"`
	AuthConfig        string     `gorm:"type:text" json:"-"`
	ToolsJSON         string     `gorm:"type:text;not null;default:'[]'" json:"tools_json"`
	PolicyJSON        string     `gorm:"type:text;default:'{}'" json:"policy_json"`
	Status            string     `gorm:"type:varchar(16);not null;default:'pending'" json:"status"`
	LastHealthCheckAt *time.Time `json:"last_health_check_at"`
}

func (ConnectedMCPServer) TableName() string { return "connected_mcp_servers" }

// ToolPolicy represents a tool policy
type ToolPolicy struct {
	BaseModel
	WorkspaceID          string `gorm:"type:text;not null;index" json:"workspace_id"`
	ConnectedServerID    string `gorm:"type:text;not null;index" json:"connected_server_id"`
	ToolName             string `gorm:"type:varchar(128);not null" json:"tool_name"`
	Allowed              bool   `gorm:"not null;default:true" json:"allowed"`
	RequiresConfirmation bool   `gorm:"not null;default:false" json:"requires_confirmation"`
	MaxCallsPerDay       int    `gorm:"not null;default:0" json:"max_calls_per_day"`
	MaxCallsPerUser      int    `gorm:"not null;default:0" json:"max_calls_per_user"`
	RiskLevel            string `gorm:"type:varchar(16);not null;default:'low'" json:"risk_level"`
}

func (ToolPolicy) TableName() string { return "tool_policies" }

// ToolInvocationLog represents tool invocation logs
type ToolInvocationLog struct {
	BaseModel
	WorkspaceID       string    `gorm:"type:text;not null;index" json:"workspace_id"`
	UserID            string    `gorm:"type:text;not null" json:"user_id"`
	AgentClientID     string    `gorm:"type:text" json:"agent_client_id"`
	MCPSessionID      string    `gorm:"type:text;index" json:"mcp_session_id"`
	ToolName          string    `gorm:"type:varchar(128);not null;index" json:"tool_name"`
	ConnectedServerID *string   `gorm:"type:text" json:"connected_server_id"`
	InputJSON         string    `gorm:"type:text" json:"input_json"`
	OutputSummary     string    `gorm:"type:text" json:"output_summary"`
	Status            string    `gorm:"type:varchar(16);not null;index" json:"status"`
	ErrorCode         string    `gorm:"type:varchar(64)" json:"error_code"`
	ErrorMessage      string    `gorm:"type:text" json:"error_message"`
	LatencyMs         int       `gorm:"not null;default:0" json:"latency_ms"`
	Confirmed         bool      `gorm:"not null;default:false" json:"confirmed"`
	InvokedAt         time.Time `gorm:"index" json:"invoked_at"`
}

func (ToolInvocationLog) TableName() string { return "tool_invocation_logs" }

// ============================================================
// Auth & Billing
// ============================================================

// APIKey represents an API key
type APIKey struct {
	BaseModel
	WorkspaceID string     `gorm:"type:text;not null;index" json:"workspace_id"`
	Name        string     `gorm:"type:varchar(128);not null" json:"name"`
	Prefix      string     `gorm:"type:varchar(16);not null;index" json:"prefix"`
	Hash        string     `gorm:"type:varchar(256);not null" json:"-"`
	Scopes      string     `gorm:"type:text;default:'[]'" json:"scopes"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedBy   string     `gorm:"type:text" json:"created_by"`
	RevokedAt   *time.Time `json:"revoked_at"`
}

func (APIKey) TableName() string { return "api_keys" }

// UsageRecord represents usage records
type UsageRecord struct {
	BaseModel
	WorkspaceID string    `gorm:"type:text;not null;index" json:"workspace_id"`
	UserID      string    `gorm:"type:text" json:"user_id"`
	Metric      string    `gorm:"type:varchar(32);not null;index" json:"metric"`
	Quantity    int       `gorm:"not null" json:"quantity"`
	Period      string    `gorm:"type:varchar(16);not null;index" json:"period"`
	RecordedAt  time.Time `gorm:"index" json:"recorded_at"`
}

func (UsageRecord) TableName() string { return "usage_records" }

// AuditLog represents audit logs
type AuditLog struct {
	BaseModel
	WorkspaceID string `gorm:"type:text;not null;index" json:"workspace_id"`
	Actor       string `gorm:"type:varchar(128);not null" json:"actor"`
	ActorType   string `gorm:"type:varchar(16);not null" json:"actor_type"`
	Action      string `gorm:"type:varchar(64);not null;index" json:"action"`
	Target      string `gorm:"type:varchar(128)" json:"target"`
	TargetType  string `gorm:"type:varchar(32)" json:"target_type"`
	Payload     string `gorm:"type:text" json:"payload"`
	ClientIP    string `gorm:"type:varchar(64)" json:"client_ip"`
}

func (AuditLog) TableName() string { return "audit_logs" }

// OAuthToken represents an OAuth token
type OAuthToken struct {
	BaseModel
	UserID       string     `gorm:"type:text;not null;index" json:"user_id"`
	Provider     string     `gorm:"type:varchar(32);not null" json:"provider"`
	AccessToken  string     `gorm:"type:text;not null" json:"-"`
	RefreshToken string     `gorm:"type:text" json:"-"`
	TokenType    string     `gorm:"type:varchar(16);default:'Bearer'" json:"token_type"`
	Scope        string     `gorm:"type:text" json:"scope"`
	ExpiresAt    *time.Time `json:"expires_at"`
}

func (OAuthToken) TableName() string { return "oauth_tokens" }

// Project represents a project
type Project struct {
	BaseModel
	OrgID       string `gorm:"type:text;not null;index" json:"org_id"`
	WorkspaceID string `gorm:"type:text;not null;index" json:"workspace_id"`
	Name        string `gorm:"type:varchar(128);not null" json:"name"`
	Slug        string `gorm:"type:varchar(64);not null" json:"slug"`
	Description string `gorm:"type:text" json:"description"`
	Stack       string `gorm:"type:text" json:"stack"`     // JSON
	Structure   string `gorm:"type:text" json:"structure"` // JSON
	Status      string `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
	// RepoPath is the most recently bound local repo absolute path; varies per machine;
	// used only for same-machine exact matching and display, not as a cross-machine primary key.
	RepoPath string `gorm:"type:varchar(512);index" json:"repo_path"`
	// GitRemote is the normalized git remote URL (stripped of protocol/credentials/.git suffix);
	// the primary identifier for recognizing the same project across machines.
	GitRemote string `gorm:"type:varchar(512);index" json:"git_remote"`
	// RepoName is the repository directory name (filepath.Base); used as a cross-machine fallback
	// match key when git_remote is missing (only effective on unique hits).
	RepoName string `gorm:"type:varchar(128);index" json:"repo_name"`
}

func (Project) TableName() string { return "projects" }

// SyncRecord records the most recent snapshot sync per (user x client x machine) for a given project,
// enabling multi-user multi-device sync observability.
// The one-way server->local sync itself is not persisted; this table is a sidecar audit:
// answers "who, with which client, on which machine, synced to which etag, and when".
// Upserted by (project_id, user_id, agent_client_id, repo_path); repeated syncs only update
// etag/synced_at and increment sync_count without creating new rows.
type SyncRecord struct {
	BaseModel
	OrgID         string    `gorm:"type:text;not null;index" json:"org_id"`
	WorkspaceID   string    `gorm:"type:text;not null;index" json:"workspace_id"`
	ProjectID     string    `gorm:"type:text;not null;index" json:"project_id"`
	UserID        string    `gorm:"type:text;not null;index" json:"user_id"`
	AgentClientID string    `gorm:"type:text;index" json:"agent_client_id"`
	Client        string    `gorm:"type:varchar(64)" json:"client"`           // Client type, e.g. cursor / claude-code / unknown
	ClientName    string    `gorm:"type:varchar(128)" json:"client_name"`     // Client display name
	RepoPath      string    `gorm:"type:varchar(512)" json:"repo_path"`       // Local repo path, used as the "machine" differentiation key
	ETag          string    `gorm:"column:etag;type:varchar(64)" json:"etag"` // Bundle etag synced to by this client (explicit column name to avoid GORM splitting into e_tag)
	SyncCount     int       `gorm:"not null;default:1" json:"sync_count"`     // Cumulative sync count
	SyncedAt      time.Time `gorm:"index" json:"synced_at"`
}

func (SyncRecord) TableName() string { return "sync_records" }
