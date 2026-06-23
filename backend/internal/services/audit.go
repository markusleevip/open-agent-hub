package services

import (
	"encoding/json"
	"time"

	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/models"
)

// AuditService is the audit log service
type AuditService struct{}

func NewAuditService() *AuditService {
	return &AuditService{}
}

// Log records an audit log entry
func (s *AuditService) Log(workspaceID, actor, actorType, action, target, targetType string, payload interface{}, clientIP string) {
	var payloadStr string
	if payload != nil {
		if b, err := json.Marshal(payload); err == nil {
			payloadStr = string(b)
		}
	}
	log := models.AuditLog{
		WorkspaceID: workspaceID,
		Actor:       actor,
		ActorType:   actorType,
		Action:      action,
		Target:      target,
		TargetType:  targetType,
		Payload:     payloadStr,
		ClientIP:    clientIP,
	}
	// Async write
	go func() {
		database.DB.Create(&log)
	}()
}

// LogSync records a sync log entry (for certain key operations)
func (s *AuditService) LogSync(workspaceID, actor, actorType, action, target, targetType string, payload interface{}, clientIP string) error {
	var payloadStr string
	if payload != nil {
		if b, err := json.Marshal(payload); err == nil {
			payloadStr = string(b)
		}
	}
	log := models.AuditLog{
		WorkspaceID: workspaceID,
		Actor:       actor,
		ActorType:   actorType,
		Action:      action,
		Target:      target,
		TargetType:  targetType,
		Payload:     payloadStr,
		ClientIP:    clientIP,
	}
	return database.DB.Create(&log).Error
}

// UsageService is the usage statistics service
type UsageService struct{}

func NewUsageService() *UsageService {
	return &UsageService{}
}

// Record records a usage entry
func (s *UsageService) Record(workspaceID, userID, metric string, quantity int) {
	now := time.Now()
	period := now.Format("2006-01-02") // daily
	record := models.UsageRecord{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Metric:      metric,
		Quantity:    quantity,
		Period:      period,
		RecordedAt:  now,
	}
	go func() {
		database.DB.Create(&record)
	}()
}
