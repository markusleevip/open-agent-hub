package services

import (
	"testing"

	"github.com/openagenthub/backend/internal/models"
)

func pol(allowed bool, perDay, perUser int, requiresConfirm bool) *models.ToolPolicy {
	return &models.ToolPolicy{
		Allowed:              allowed,
		MaxCallsPerDay:       perDay,
		MaxCallsPerUser:      perUser,
		RequiresConfirmation: requiresConfirm,
	}
}

func TestEvaluateToolCall_WorkspaceQuota(t *testing.T) {
	// Quota exhausted: even hub.* should be denied
	v := EvaluateToolCall(ToolCallEval{
		IsHubTool: true, WorkspaceQuotaDaily: 100, WorkspaceDailyUsed: 100,
	})
	if v.Decision != DecisionDeny || v.Code != "quota_exceeded" {
		t.Fatalf("exhausted quota should deny/quota_exceeded, got %+v", v)
	}
	// Not exhausted: allow
	v = EvaluateToolCall(ToolCallEval{
		IsHubTool: true, WorkspaceQuotaDaily: 100, WorkspaceDailyUsed: 99,
	})
	if v.Decision != DecisionAllow {
		t.Fatalf("available quota should allow, got %+v", v)
	}
	// Quota=0 means unlimited
	v = EvaluateToolCall(ToolCallEval{
		IsHubTool: true, WorkspaceQuotaDaily: 0, WorkspaceDailyUsed: 999999,
	})
	if v.Decision != DecisionAllow {
		t.Fatalf("quota=0 should be unlimited, got %+v", v)
	}
}

func TestEvaluateToolCall_HubBypassesToolPolicy(t *testing.T) {
	// hub.* is not affected by connected tool policy (even when given a disallowed policy)
	v := EvaluateToolCall(ToolCallEval{
		IsHubTool: true, Policy: pol(false, 0, 0, false),
	})
	if v.Decision != DecisionAllow {
		t.Fatalf("hub.* should bypass tool policy, got %+v", v)
	}
}

func TestEvaluateToolCall_ConnectedToolPolicy(t *testing.T) {
	// No policy configured -> allow by default
	if v := EvaluateToolCall(ToolCallEval{IsHubTool: false, Policy: nil}); v.Decision != DecisionAllow {
		t.Fatalf("no policy should allow, got %+v", v)
	}
	// Allowed=false -> disabled
	if v := EvaluateToolCall(ToolCallEval{Policy: pol(false, 0, 0, false)}); v.Decision != DecisionDeny || v.Code != "tool_forbidden" {
		t.Fatalf("disabled tool should deny/tool_forbidden, got %+v", v)
	}
	// Daily limit
	if v := EvaluateToolCall(ToolCallEval{Policy: pol(true, 10, 0, false), ToolDailyUsed: 10}); v.Code != "tool_quota_exceeded" {
		t.Fatalf("should trigger tool_quota_exceeded, got %+v", v)
	}
	// Per-user limit
	if v := EvaluateToolCall(ToolCallEval{Policy: pol(true, 0, 3, false), UserToolDailyUsed: 3}); v.Code != "tool_user_quota_exceeded" {
		t.Fatalf("should trigger tool_user_quota_exceeded, got %+v", v)
	}
}

func TestEvaluateToolCall_Confirmation(t *testing.T) {
	// Requires confirmation but global switch is off -> allow directly
	if v := EvaluateToolCall(ToolCallEval{Policy: pol(true, 0, 0, true), ConfirmEnabled: false}); v.Decision != DecisionAllow {
		t.Fatalf("confirm switch off should allow, got %+v", v)
	}
	// Requires confirmation + switch on + not confirmed -> confirm
	if v := EvaluateToolCall(ToolCallEval{Policy: pol(true, 0, 0, true), ConfirmEnabled: true, Confirmed: false}); v.Decision != DecisionConfirm || v.Code != "needs_confirmation" {
		t.Fatalf("should require confirmation, got %+v", v)
	}
	// Requires confirmation + switch on + confirmed -> allow
	if v := EvaluateToolCall(ToolCallEval{Policy: pol(true, 0, 0, true), ConfirmEnabled: true, Confirmed: true}); v.Decision != DecisionAllow {
		t.Fatalf("confirmed should allow, got %+v", v)
	}
}

func TestEvaluateToolCall_PrecedenceQuotaBeforePolicy(t *testing.T) {
	// Workspace quota takes precedence over tool policy: even if the tool is disabled, quota_exceeded is reported first
	v := EvaluateToolCall(ToolCallEval{
		WorkspaceQuotaDaily: 5, WorkspaceDailyUsed: 5,
		Policy: pol(false, 0, 0, false),
	})
	if v.Code != "quota_exceeded" {
		t.Fatalf("workspace quota should take precedence, got %+v", v)
	}
}
