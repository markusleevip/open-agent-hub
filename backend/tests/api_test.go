package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openagenthub/backend/internal/config"
	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/handlers"
	"github.com/openagenthub/backend/internal/mcp"
	"github.com/openagenthub/backend/internal/middleware"
	"github.com/openagenthub/backend/internal/models"
)

var testCfg *config.Config

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	tmpFile := "/tmp/oah_test_" + time.Now().Format("20060102150405") + ".db"
	testCfg = &config.Config{
		ConsolePort:       18084,
		MCPPort:           18085,
		DBType:            "sqlite",
		DBDSN:             tmpFile,
		JWTSecret:         "test-secret-for-testing",
		JWTExpire:         24,
		BootstrapUsername: "admin",
		BootstrapPassword: "test123",
		EncryptionKey:     "test-encryption-key-32-bytes!!",
	}
	if err := database.Init(testCfg); err != nil {
		panic("db init: " + err.Error())
	}
	if err := database.DB.Use(middleware.NewTenantPlugin(middleware.TenantContextKey)); err != nil {
		panic("tenant plugin: " + err.Error())
	}
	code := m.Run()
	os.Remove(tmpFile)
	os.Exit(code)
}

func setupRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	authH := handlers.NewAuthHandler(testCfg)
	api.POST("/auth/login", authH.Login)
	api.POST("/auth/register", authH.Register)

	authed := api.Group("")
	authed.Use(middleware.AuthRequired(testCfg))
	{
		authed.GET("/auth/me", authH.Me)
		authed.GET("/workspaces", handlers.NewWorkspaceHandler(testCfg).List)
		authed.GET("/rules", handlers.NewRuleHandler().List)
		authed.GET("/personal-instructions", handlers.NewRuleHandler().GetPersonalInstructions)
		authed.PUT("/personal-instructions", handlers.NewRuleHandler().UpdatePersonalInstructions)
		authed.GET("/output-preferences", handlers.NewRuleHandler().GetOutputPreferences)
		authed.GET("/memories", handlers.NewMemoryHandler().List)
		authed.GET("/skills", handlers.NewSkillHandler().List)
		authed.GET("/public-skills", handlers.NewPublicSkillHandler().List)
		publicSkillH := handlers.NewPublicSkillHandler()
		publicSkillAdmin := authed.Group("/public-skills")
		publicSkillAdmin.Use(middleware.RequireRole("owner", "admin"))
		{
			publicSkillAdmin.POST("", publicSkillH.Create)
			publicSkillAdmin.PUT("/:id", publicSkillH.Update)
			publicSkillAdmin.PUT("/:id/status", publicSkillH.ChangeStatus)
		}
		skillInstallH := handlers.NewSkillInstallHandler()
		authed.POST("/skill-installs", skillInstallH.Create)
		authed.PUT("/skill-installs/:id/state", skillInstallH.ChangeState)
		authed.GET("/tokens", handlers.NewTokenHandler(testCfg).List)
		authed.GET("/agent-clients", handlers.NewAgentClientHandler().List)
		authed.GET("/connected-servers", handlers.NewConnectedServerHandler().List)
		authed.GET("/tool-policies", handlers.NewToolPolicyHandler().List)
		authed.GET("/audit-logs", handlers.NewAuditHandler().List)
		authed.GET("/usage/dashboard", handlers.NewUsageHandler().Dashboard)
	}
	return r
}

func getTestToken(t *testing.T) string {
	t.Helper()
	r := setupRouter()
	body, _ := json.Marshal(map[string]string{
		"username": testCfg.BootstrapUsername,
		"password": testCfg.BootstrapPassword,
	})
	req, _ := http.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp["data"].(map[string]interface{})
	token, _ := data["token"].(string)
	return token
}

func TestHealthCheck(t *testing.T) {
	r := setupRouter()
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestLogin(t *testing.T) {
	token := getTestToken(t)
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestMeEndpoint(t *testing.T) {
	r := setupRouter()
	token := getTestToken(t)
	req, _ := http.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnauthenticatedAccess(t *testing.T) {
	r := setupRouter()
	req, _ := http.NewRequest("GET", "/api/workspaces", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestWorkspacesList(t *testing.T) {
	r := setupRouter()
	token := getTestToken(t)
	req, _ := http.NewRequest("GET", "/api/workspaces", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"] != float64(0) {
		t.Fatalf("expected code 0, got %v", resp["code"])
	}
}

func TestPersonalInstructionsLifecycle(t *testing.T) {
	r := setupRouter()
	token := getTestToken(t)

	req, _ := http.NewRequest("GET", "/api/personal-instructions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("get personal instructions: HTTP %d %s", w.Code, w.Body.String())
	}
	var getResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &getResp)
	data, _ := getResp["data"].(map[string]interface{})
	if data["language"] != "zh-CN" || data["verbosity"] != "normal" {
		t.Fatalf("unexpected defaults: %+v", data)
	}

	custom := "默认中文回复，代码和命令保持原文。"
	body, _ := json.Marshal(map[string]interface{}{
		"language":            "zh-CN",
		"verbosity":           "detailed",
		"code_style":          "project",
		"personality":         "rigorous",
		"response_style":      "checklist",
		"custom_instructions": custom,
		"memory": map[string]interface{}{
			"enabled":           true,
			"skip_tool_context": false,
		},
	})
	for i := 0; i < 2; i++ {
		reqPut, _ := http.NewRequest("PUT", "/api/personal-instructions", bytes.NewReader(body))
		reqPut.Header.Set("Authorization", "Bearer "+token)
		reqPut.Header.Set("Content-Type", "application/json")
		wPut := httptest.NewRecorder()
		r.ServeHTTP(wPut, reqPut)
		if wPut.Code != 200 {
			t.Fatalf("put personal instructions #%d: HTTP %d %s", i+1, wPut.Code, wPut.Body.String())
		}
	}

	reqPrefs, _ := http.NewRequest("GET", "/api/output-preferences", nil)
	reqPrefs.Header.Set("Authorization", "Bearer "+token)
	wPrefs := httptest.NewRecorder()
	r.ServeHTTP(wPrefs, reqPrefs)
	if wPrefs.Code != 200 {
		t.Fatalf("get output preferences: HTTP %d %s", wPrefs.Code, wPrefs.Body.String())
	}
	var prefsResp map[string]interface{}
	json.Unmarshal(wPrefs.Body.Bytes(), &prefsResp)
	prefs, _ := prefsResp["data"].(map[string]interface{})
	if prefs["custom_instructions"] != custom || prefs["verbosity"] != "detailed" || prefs["skip_tool_memory"] != "false" {
		t.Fatalf("unexpected output preferences: %+v", prefs)
	}

	var duplicates int64
	database.DB.Model(&models.OutputPreference{}).
		Where("key = ? AND value = ?", "custom_instructions", custom).
		Count(&duplicates)
	if duplicates != 1 {
		t.Fatalf("expected one custom_instructions row after repeated PUT, got %d", duplicates)
	}
}

func TestPublicSkillCatalogAndInstall(t *testing.T) {
	r := setupRouter()
	token := getTestToken(t)

	req, _ := http.NewRequest("GET", "/api/public-skills", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("list public skills: HTTP %d %s", w.Code, w.Body.String())
	}
	var listResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &listResp)
	data, _ := listResp["data"].(map[string]interface{})
	items, _ := data["items"].([]interface{})
	if len(items) == 0 {
		t.Fatal("expected seeded public skills")
	}

	var tpl models.PublicSkillTemplate
	if err := database.DB.First(&tpl, "slug = ?", "react-frontend-review").Error; err != nil {
		t.Fatalf("load template: %v", err)
	}
	body, _ := json.Marshal(map[string]interface{}{
		"template_id": tpl.ID,
		"pinned":      true,
	})
	req2, _ := http.NewRequest("POST", "/api/skill-installs", bytes.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("install public skill: HTTP %d %s", w2.Code, w2.Body.String())
	}

	var install models.SkillInstall
	if err := database.DB.First(&install, "template_id = ?", tpl.ID).Error; err != nil {
		t.Fatalf("install not found: %v", err)
	}
	t.Cleanup(func() { database.DB.Delete(&install) })
	if install.OverrideContent == "" || install.InstalledVersion != tpl.Version {
		t.Fatalf("install should snapshot content and version, got version=%d content=%q", install.InstalledVersion, install.OverrideContent)
	}

	req3, _ := http.NewRequest("POST", "/api/skill-installs", bytes.NewReader(body))
	req3.Header.Set("Authorization", "Bearer "+token)
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != 409 {
		t.Fatalf("duplicate install should conflict, got HTTP %d %s", w3.Code, w3.Body.String())
	}

	stateBody, _ := json.Marshal(map[string]string{"state": "archived"})
	req4, _ := http.NewRequest("PUT", "/api/skill-installs/"+install.ID+"/state", bytes.NewReader(stateBody))
	req4.Header.Set("Authorization", "Bearer "+token)
	req4.Header.Set("Content-Type", "application/json")
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	if w4.Code != 200 {
		t.Fatalf("archive install: HTTP %d %s", w4.Code, w4.Body.String())
	}

	req5, _ := http.NewRequest("POST", "/api/skill-installs", bytes.NewReader(body))
	req5.Header.Set("Authorization", "Bearer "+token)
	req5.Header.Set("Content-Type", "application/json")
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, req5)
	if w5.Code != 200 {
		t.Fatalf("reinstall archived public skill: HTTP %d %s", w5.Code, w5.Body.String())
	}
	var reactivated models.SkillInstall
	if err := database.DB.First(&reactivated, "id = ?", install.ID).Error; err != nil {
		t.Fatalf("reactivated install not found: %v", err)
	}
	if reactivated.State != "active" || reactivated.InstalledVersion != tpl.Version {
		t.Fatalf("archived install should reactivate at current version, got state=%q version=%d", reactivated.State, reactivated.InstalledVersion)
	}
}

func TestPublicSkillCreateUpdateAndStatus(t *testing.T) {
	r := setupRouter()
	token := getTestToken(t)
	slug := "api-review-" + time.Now().Format("150405000")

	createBody, _ := json.Marshal(map[string]interface{}{
		"slug":        slug,
		"name":        "API Review",
		"description": "Review REST API compatibility.",
		"content":     "Review API routes, payloads, and compatibility notes.",
		"category":    "backend",
		"tags":        []string{"api", "review", "api"},
		"risk_level":  "low",
		"status":      "draft",
	})
	req, _ := http.NewRequest("POST", "/api/public-skills", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("create public skill: HTTP %d %s", w.Code, w.Body.String())
	}
	var createResp struct {
		Data models.PublicSkillTemplate `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	tpl := createResp.Data
	t.Cleanup(func() {
		database.DB.Unscoped().Delete(&models.SkillInstall{}, "template_id = ?", tpl.ID)
		database.DB.Unscoped().Delete(&models.PublicSkillTemplate{}, "id = ?", tpl.ID)
	})
	if tpl.Version != 1 || tpl.Status != "draft" || tpl.Source != "manual" {
		t.Fatalf("unexpected created template: version=%d status=%q source=%q", tpl.Version, tpl.Status, tpl.Source)
	}
	if tpl.Tags != `["api","review"]` {
		t.Fatalf("expected normalized tags, got %s", tpl.Tags)
	}

	dupReq, _ := http.NewRequest("POST", "/api/public-skills", bytes.NewReader(createBody))
	dupReq.Header.Set("Authorization", "Bearer "+token)
	dupReq.Header.Set("Content-Type", "application/json")
	dupW := httptest.NewRecorder()
	r.ServeHTTP(dupW, dupReq)
	if dupW.Code != 409 {
		t.Fatalf("duplicate slug should conflict, got HTTP %d %s", dupW.Code, dupW.Body.String())
	}

	statusBody, _ := json.Marshal(map[string]string{"status": "active"})
	statusReq, _ := http.NewRequest("PUT", "/api/public-skills/"+tpl.ID+"/status", bytes.NewReader(statusBody))
	statusReq.Header.Set("Authorization", "Bearer "+token)
	statusReq.Header.Set("Content-Type", "application/json")
	statusW := httptest.NewRecorder()
	r.ServeHTTP(statusW, statusReq)
	if statusW.Code != 200 {
		t.Fatalf("activate public skill: HTTP %d %s", statusW.Code, statusW.Body.String())
	}

	installBody, _ := json.Marshal(map[string]interface{}{"template_id": tpl.ID})
	installReq, _ := http.NewRequest("POST", "/api/skill-installs", bytes.NewReader(installBody))
	installReq.Header.Set("Authorization", "Bearer "+token)
	installReq.Header.Set("Content-Type", "application/json")
	installW := httptest.NewRecorder()
	r.ServeHTTP(installW, installReq)
	if installW.Code != 200 {
		t.Fatalf("install created public skill: HTTP %d %s", installW.Code, installW.Body.String())
	}

	updateBody, _ := json.Marshal(map[string]interface{}{
		"slug":        slug,
		"name":        "API Review Updated",
		"description": "Review REST API compatibility and docs.",
		"content":     "Review API routes, payloads, compatibility notes, and docs.",
		"category":    "backend",
		"tags":        []string{"api", "review", "docs"},
		"risk_level":  "medium",
		"status":      "active",
	})
	updateReq, _ := http.NewRequest("PUT", "/api/public-skills/"+tpl.ID, bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	r.ServeHTTP(updateW, updateReq)
	if updateW.Code != 200 {
		t.Fatalf("update public skill: HTTP %d %s", updateW.Code, updateW.Body.String())
	}
	var updateResp struct {
		Data models.PublicSkillTemplate `json:"data"`
	}
	if err := json.Unmarshal(updateW.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updateResp.Data.Version != 2 {
		t.Fatalf("content update should bump version to 2, got %d", updateResp.Data.Version)
	}

	var install models.SkillInstall
	if err := database.DB.First(&install, "template_id = ?", tpl.ID).Error; err != nil {
		t.Fatalf("load install: %v", err)
	}
	if install.InstalledVersion != 1 || install.OverrideContent != "Review API routes, payloads, and compatibility notes." {
		t.Fatalf("install snapshot should remain v1, got version=%d content=%q", install.InstalledVersion, install.OverrideContent)
	}
}

func TestRulesList(t *testing.T) {
	r := setupRouter()
	token := getTestToken(t)
	req, _ := http.NewRequest("GET", "/api/rules", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMemoriesList(t *testing.T) {
	r := setupRouter()
	token := getTestToken(t)
	req, _ := http.NewRequest("GET", "/api/memories", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMCPGatewayInitialize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	gw := mcp.NewGateway(testCfg)
	r.POST("/mcp", gw.HandleHTTP)

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]interface{}{},
	})
	req, _ := http.NewRequest("POST", "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+getTestToken(t))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result, _ := resp["result"].(map[string]interface{})
	if result["protocolVersion"] != "2025-11-25" {
		t.Fatalf("unexpected protocol version: %v", result["protocolVersion"])
	}
}

func TestMCPGatewayToolsList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	gw := mcp.NewGateway(testCfg)
	r.POST("/mcp", gw.HandleHTTP)

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	})
	req, _ := http.NewRequest("POST", "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+getTestToken(t))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	result, _ := resp["result"].(map[string]interface{})
	tools, _ := result["tools"].([]interface{})
	if len(tools) < 14 {
		t.Fatalf("expected at least 14 tools, got %d", len(tools))
	}
}

func TestEncryptionRoundTrip(t *testing.T) {
	database.SetEncryptionKey("test-encryption-key-32-bytes!!")
	plaintext := `{"token": "sk-secret-12345", "refresh": "rt-abc"}`
	encrypted := database.EncryptAES(plaintext)
	if encrypted == plaintext {
		t.Fatal("encryption should change the value")
	}
	decrypted := database.DecryptAES(encrypted)
	if decrypted != plaintext {
		t.Fatalf("expected %s, got %s", plaintext, decrypted)
	}
}

// mcpToolCall calls a tool through the MCP Gateway and returns the parsed tool result.
func mcpToolCall(t *testing.T, token, name string, args map[string]interface{}) map[string]interface{} {
	t.Helper()
	r := gin.New()
	gw := mcp.NewGateway(testCfg)
	r.POST("/mcp", gw.HandleHTTP)

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": 9, "method": "tools/call",
		"params": map[string]interface{}{"name": name, "arguments": args},
	})
	req, _ := http.NewRequest("POST", "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("mcp tool call %s: HTTP %d %s", name, w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != nil {
		t.Fatalf("mcp tool call %s error: %v", name, resp["error"])
	}
	result, _ := resp["result"].(map[string]interface{})
	content, _ := result["content"].([]interface{})
	if len(content) == 0 {
		t.Fatalf("mcp tool call %s: empty content", name)
	}
	text, _ := content[0].(map[string]interface{})["text"].(string)
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("mcp tool call %s: bad result json: %v", name, err)
	}
	return out
}

// End-to-end validation of P1-2: propose_memory with low confidence -> persisted as pending_review -> Review approve -> active
func TestMemoryProposeReviewFlow(t *testing.T) {
	token := getTestToken(t)

	// 1) Low-confidence proposal -> should be judged pending_review and persisted
	res := mcpToolCall(t, token, "hub.propose_memory", map[string]interface{}{
		"content":    "本项目灰度发布走 Argo Rollouts 渐进式流量切换",
		"confidence": 0.3,
	})
	if res["decision"] != "pending_review" {
		t.Fatalf("expected pending_review, got %v (reason=%v)", res["decision"], res["reason"])
	}
	if res["state"] != "pending_review" {
		t.Fatalf("expected state pending_review, got %v", res["state"])
	}
	memID, _ := res["memory_id"].(string)
	if memID == "" {
		t.Fatal("pending memory should be persisted with an id")
	}

	// 2) Verify DB state is indeed pending_review
	var m models.Memory
	if err := database.DB.First(&m, "id = ?", memID).Error; err != nil {
		t.Fatalf("pending memory not found in DB: %v", err)
	}
	if m.State != "pending_review" {
		t.Fatalf("expected DB state pending_review, got %s", m.State)
	}

	// 3) Review approve -> active
	rr := gin.New()
	rr.Use(gin.Recovery())
	authed := rr.Group("/api")
	authed.Use(middleware.AuthRequired(testCfg))
	authed.POST("/memories/:id/review", handlers.NewMemoryHandler().Review)

	body, _ := json.Marshal(map[string]string{"action": "approve"})
	req, _ := http.NewRequest("POST", "/api/memories/"+memID+"/review", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	rr.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("review approve: HTTP %d %s", w.Code, w.Body.String())
	}

	database.DB.First(&m, "id = ?", memID)
	if m.State != "active" {
		t.Fatalf("expected active after approve, got %s", m.State)
	}

	// 4) Re-reviewing a non-pending memory should be rejected
	req2, _ := http.NewRequest("POST", "/api/memories/"+memID+"/review", bytes.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	rr.ServeHTTP(w2, req2)
	if w2.Code == 200 {
		t.Fatal("re-reviewing a non-pending memory should fail")
	}
}
