package services

import (
	"time"

	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/models"
)

// policy implements spec Appendix E.3 P0-2: Tool Policy full-field validation + quota enforcement + confirmation.

const (
	DecisionAllow   = "allow"
	DecisionDeny    = "deny"
	DecisionConfirm = "confirm"
)

// PolicyVerdict is the policy verdict result for a single tool call.
type PolicyVerdict struct {
	Decision string `json:"decision"` // allow | deny | confirm
	Code     string `json:"code"`     // Semantic code for mapping to MCP error codes
	Reason   string `json:"reason"`
}

// ToolCallEval holds all inputs needed to evaluate a tool call (assembled by the caller from DB,
// enabling pure-function unit tests).
type ToolCallEval struct {
	IsHubTool           bool
	WorkspaceQuotaDaily int                // 0 = unlimited
	WorkspaceDailyUsed  int64              // today's total tool_call usage
	Policy              *models.ToolPolicy // nil = no policy configured
	ToolDailyUsed       int64
	UserToolDailyUsed   int64
	ConfirmEnabled      bool // global ENABLE_CONFIRMATION
	Confirmed           bool // whether the client has already confirmed
}

// EvaluateToolCall is the pure-function policy evaluation (no DB dependency).
//
// Validation order: workspace daily quota (applies to all tools including hub.*) -> hub.* allow
// -> connected tool's Allowed / daily limit / per-user limit / confirmation.
func EvaluateToolCall(e ToolCallEval) PolicyVerdict {
	// 1) Workspace daily invocation quota (applies to all tools, including built-in hub.*)
	if e.WorkspaceQuotaDaily > 0 && e.WorkspaceDailyUsed >= int64(e.WorkspaceQuotaDaily) {
		return PolicyVerdict{DecisionDeny, "quota_exceeded", "workspace daily tool invocation limit reached"}
	}

	// 2) Built-in hub.* tools are not subject to external connected tool policy
	if e.IsHubTool {
		return PolicyVerdict{DecisionAllow, "", ""}
	}

	// 3) Connected tool policy full-field validation
	if e.Policy != nil {
		if !e.Policy.Allowed {
			return PolicyVerdict{DecisionDeny, "tool_forbidden", "this tool is forbidden by policy"}
		}
		if e.Policy.MaxCallsPerDay > 0 && e.ToolDailyUsed >= int64(e.Policy.MaxCallsPerDay) {
			return PolicyVerdict{DecisionDeny, "tool_quota_exceeded", "this tool has reached its daily invocation limit"}
		}
		if e.Policy.MaxCallsPerUser > 0 && e.UserToolDailyUsed >= int64(e.Policy.MaxCallsPerUser) {
			return PolicyVerdict{DecisionDeny, "tool_user_quota_exceeded", "this tool has reached its per-user daily invocation limit"}
		}
		if e.ConfirmEnabled && e.Policy.RequiresConfirmation && !e.Confirmed {
			return PolicyVerdict{DecisionConfirm, "needs_confirmation",
				"high-risk operation requires confirmation (retry with __confirm: true in arguments)"}
		}
	}

	return PolicyVerdict{DecisionAllow, "", ""}
}

// CheckToolCall assembles DB inputs and evaluates a tool call.
func CheckToolCall(workspaceID, userID, toolName string, isHub, confirmEnabled, confirmed bool) PolicyVerdict {
	var ws models.Workspace
	database.DB.Select("quota_tool_call_daily").First(&ws, "id = ?", workspaceID)

	eval := ToolCallEval{
		IsHubTool:           isHub,
		WorkspaceQuotaDaily: ws.QuotaToolCallDaily,
		WorkspaceDailyUsed:  WorkspaceDailyToolCalls(workspaceID),
		ConfirmEnabled:      confirmEnabled,
		Confirmed:           confirmed,
	}

	if !isHub {
		eval.Policy = GetToolPolicy(workspaceID, toolName)
		if eval.Policy != nil {
			if eval.Policy.MaxCallsPerDay > 0 {
				eval.ToolDailyUsed = ToolDailyCount(workspaceID, toolName)
			}
			if eval.Policy.MaxCallsPerUser > 0 {
				eval.UserToolDailyUsed = UserToolDailyCount(workspaceID, userID, toolName)
			}
		}
	}

	return EvaluateToolCall(eval)
}

// GetToolPolicy reads the policy for a connected tool; returns nil if not configured.
func GetToolPolicy(workspaceID, toolName string) *models.ToolPolicy {
	var p models.ToolPolicy
	if err := database.DB.Where("workspace_id = ? AND tool_name = ?", workspaceID, toolName).First(&p).Error; err != nil {
		return nil
	}
	return &p
}

// WorkspaceDailyToolCalls returns today's total tool_call usage for the workspace (from UsageRecord).
func WorkspaceDailyToolCalls(workspaceID string) int64 {
	var n int64
	database.DB.Model(&models.UsageRecord{}).
		Where("workspace_id = ? AND metric = ? AND period = ?", workspaceID, "tool_call", todayPeriod()).
		Select("COALESCE(SUM(quantity),0)").Scan(&n)
	return n
}

// ToolDailyCount returns today's successful invocation count for a tool (from ToolInvocationLog).
func ToolDailyCount(workspaceID, toolName string) int64 {
	var n int64
	database.DB.Model(&models.ToolInvocationLog{}).
		Where("workspace_id = ? AND tool_name = ? AND status = ? AND invoked_at >= ?",
			workspaceID, toolName, "ok", startOfToday()).Count(&n)
	return n
}

// UserToolDailyCount returns today's successful invocation count for a user on a specific tool.
func UserToolDailyCount(workspaceID, userID, toolName string) int64 {
	var n int64
	database.DB.Model(&models.ToolInvocationLog{}).
		Where("workspace_id = ? AND user_id = ? AND tool_name = ? AND status = ? AND invoked_at >= ?",
			workspaceID, userID, toolName, "ok", startOfToday()).Count(&n)
	return n
}

// MemoryQuotaExceeded checks whether the workspace has reached its active memory limit
// (QuotaMemoryCount; 0 = unlimited).
func MemoryQuotaExceeded(workspaceID string) bool {
	var ws models.Workspace
	if err := database.DB.Select("quota_memory_count").First(&ws, "id = ?", workspaceID).Error; err != nil {
		return false
	}
	if ws.QuotaMemoryCount <= 0 {
		return false
	}
	var n int64
	database.DB.Model(&models.Memory{}).
		Where("workspace_id = ? AND state = ?", workspaceID, "active").Count(&n)
	return n >= int64(ws.QuotaMemoryCount)
}

func todayPeriod() string { return time.Now().Format("2006-01-02") }

func startOfToday() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}
