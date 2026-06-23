package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openagenthub/backend/internal/auth"
	"github.com/openagenthub/backend/internal/config"
	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/models"
	"github.com/openagenthub/backend/internal/response"
)

const (
	CtxUserID      = "user_id"
	CtxUsername    = "username"
	CtxOrgID       = "org_id"
	CtxWorkspaceID = "workspace_id"
	CtxRole        = "role"
	CtxScopes      = "scopes"
	CtxClaims      = "claims"
)

// AuthRequired requires authentication
func AuthRequired(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			response.Unauthorized(c, "missing bearer token")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := auth.ParseJWT(cfg, tokenString)
		if err != nil {
			response.Unauthorized(c, "invalid or expired token: "+err.Error())
			return
		}

		// Inject context
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUsername, claims.Username)
		c.Set(CtxOrgID, claims.OrgID)
		c.Set(CtxWorkspaceID, claims.WorkspaceID)
		c.Set(CtxRole, claims.Role)
		c.Set(CtxScopes, claims.Scopes)
		c.Set(CtxClaims, claims)
		c.Next()
	}
}

// GetUserID gets UserID from context
func GetUserID(c *gin.Context) string {
	if v, ok := c.Get(CtxUserID); ok {
		return v.(string)
	}
	return ""
}

func GetUsername(c *gin.Context) string {
	if v, ok := c.Get(CtxUsername); ok {
		return v.(string)
	}
	return ""
}

func GetOrgID(c *gin.Context) string {
	if v, ok := c.Get(CtxOrgID); ok {
		return v.(string)
	}
	return ""
}

func GetWorkspaceID(c *gin.Context) string {
	if v, ok := c.Get(CtxWorkspaceID); ok {
		return v.(string)
	}
	return ""
}

func GetRole(c *gin.Context) string {
	if v, ok := c.Get(CtxRole); ok {
		return v.(string)
	}
	return ""
}

func GetScopes(c *gin.Context) []string {
	if v, ok := c.Get(CtxScopes); ok {
		if scopes, ok := v.([]string); ok {
			return scopes
		}
	}
	return nil
}

func GetClaims(c *gin.Context) *auth.Claims {
	if v, ok := c.Get(CtxClaims); ok {
		if claims, ok := v.(*auth.Claims); ok {
			return claims
		}
	}
	return nil
}

// RequireRole requires a specific role (owner / admin / member / viewer)
func RequireRole(roles ...string) gin.HandlerFunc {
	roleSet := make(map[string]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}
	return func(c *gin.Context) {
		role := GetRole(c)
		if !roleSet[role] {
			response.Forbidden(c, "insufficient role: "+role)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireScope requires a specific scope
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopes := GetScopes(c)
		for _, s := range scopes {
			if s == scope || s == "admin" {
				c.Next()
				return
			}
		}
		response.Forbidden(c, "missing required scope: "+scope)
		c.Abort()
	}
}

// EnforceQuota is the quota enforcement middleware
func EnforceQuota(metric string) gin.HandlerFunc {
	return func(c *gin.Context) {
		workspaceID := GetWorkspaceID(c)
		if workspaceID == "" {
			c.Next()
			return
		}

		var ws models.Workspace
		if err := database.DB.First(&ws, "id = ?", workspaceID).Error; err != nil {
			c.Next()
			return
		}

		switch metric {
		case "tool_call":
			if ws.QuotaToolCallDaily <= 0 {
				c.Next()
				return
			}
			var count int64
			today := strings.TrimSpace("")
			database.DB.Model(&models.UsageRecord{}).
				Where("workspace_id = ? AND metric = ? AND period >= ?", workspaceID, "tool_call", today).
				Select("COALESCE(SUM(quantity), 0)").Scan(&count)
			if count >= int64(ws.QuotaToolCallDaily) {
				response.Error(c, 429, "tool call daily quota exceeded")
				c.Abort()
				return
			}
		case "memory":
			if ws.QuotaMemoryCount <= 0 {
				c.Next()
				return
			}
			var count int64
			database.DB.Model(&models.Memory{}).Where("workspace_id = ? AND state = 'active'", workspaceID).Count(&count)
			if count >= int64(ws.QuotaMemoryCount) {
				response.Error(c, 429, "memory count quota exceeded")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
