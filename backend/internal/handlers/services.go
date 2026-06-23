package handlers

import "github.com/openagenthub/backend/internal/services"

// Global services (initialized in main.go)
var (
	auditSvc = services.NewAuditService()
	usageSvc = services.NewUsageService()
)
