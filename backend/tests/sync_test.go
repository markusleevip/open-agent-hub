package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/mcp"
	"github.com/openagenthub/backend/internal/models"
)

// callTool calls a tool via the gateway tools/call and decodes the embedded JSON result.
func callTool(t *testing.T, name string, args map[string]interface{}, headers map[string]string) map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	gw := mcp.NewGateway(testCfg)
	r.POST("/mcp", gw.HandleHTTP)

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]interface{}{"name": name, "arguments": args},
	})
	req, _ := http.NewRequest("POST", "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+getTestToken(t))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Error  map[string]interface{} `json:"error"`
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != nil {
		t.Fatalf("rpc error: %v", resp.Error)
	}
	if len(resp.Result.Content) == 0 {
		t.Fatalf("empty content: %s", w.Body.String())
	}
	if resp.Result.IsError {
		t.Fatalf("tool error: %s", resp.Result.Content[0].Text)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Result.Content[0].Text), &out); err != nil {
		t.Fatalf("invalid tool payload: %v\n%s", err, resp.Result.Content[0].Text)
	}
	return out
}

func TestSyncProjectRegisterAndETag(t *testing.T) {
	projectPath := "/tmp/oah-sync-test-project"

	// First time: register project and return full bundle
	res := callTool(t, "hub.sync_project", map[string]interface{}{
		"project_path":     projectPath,
		"register_project": true,
		"project_name":     "Sync Test",
	}, nil)

	if res["changed"] != true {
		t.Fatalf("expected changed=true, got %v", res["changed"])
	}
	etag, _ := res["etag"].(string)
	if etag == "" {
		t.Fatal("expected non-empty etag")
	}
	project, _ := res["project"].(map[string]interface{})
	if project == nil || project["name"] != "Sync Test" {
		t.Fatalf("expected registered project, got %v", res["project"])
	}

	files, _ := res["files"].([]interface{})
	paths := map[string]bool{}
	for _, f := range files {
		fm := f.(map[string]interface{})
		paths[fm["path"].(string)] = true
		if fm["content"].(string) == "" {
			t.Errorf("file %v has empty content", fm["path"])
		}
	}
	for _, want := range []string{
		".openagent/rules.md",
		".openagent/project.md",
		".openagent/local/profile.md",
		".openagent/local/memories.md",
		".openagent/skills/index.json",
		".openagent/.gitignore",
	} {
		if !paths[want] {
			t.Errorf("bundle missing file %s (got %v)", want, paths)
		}
	}
	block, _ := res["managed_block"].(string)
	if block == "" {
		t.Fatal("expected managed_block")
	}
	instr, _ := res["instructions"].(string)
	if instr == "" {
		t.Fatal("expected instructions when changed=true")
	}
	if !strings.Contains(instr, "managed_block") || !strings.Contains(instr, "CLAUDE.md") {
		t.Errorf("instructions missing key guidance:\n%s", instr)
	}

	// Second time: provide etag, expect changed=false and no files
	res2 := callTool(t, "hub.sync_project", map[string]interface{}{
		"project_path": projectPath,
		"etag":         etag,
	}, nil)
	if res2["changed"] != false {
		t.Fatalf("expected changed=false with matching etag, got %v", res2["changed"])
	}
	if _, hasFiles := res2["files"]; hasFiles {
		t.Fatal("unchanged response should not carry files")
	}
	if _, hasInstr := res2["instructions"]; hasInstr {
		t.Fatal("unchanged response should not carry instructions")
	}

	// Third time: after adding a new skill, etag should change and bundle should include skills/<slug>/SKILL.md
	var proj models.Project
	if err := database.DB.First(&proj, "id = ?", project["id"]).Error; err != nil {
		t.Fatalf("load project: %v", err)
	}
	skill := models.Memory{
		OrgID:       proj.OrgID,
		WorkspaceID: proj.WorkspaceID,
		UserID:      "test-user",
		Content:     "# Deploy Checklist\n\nSteps to deploy safely.",
		Type:        "skill",
		Category:    "procedural",
		Scope:       "workspace",
		Provenance:  "human_curated",
		Importance:  0.7,
		State:       "active",
	}
	if err := database.DB.Create(&skill).Error; err != nil {
		t.Fatalf("create skill: %v", err)
	}
	t.Cleanup(func() { database.DB.Delete(&skill) })

	res3 := callTool(t, "hub.sync_project", map[string]interface{}{
		"project_path": projectPath,
		"etag":         etag,
	}, nil)
	if res3["changed"] != true {
		t.Fatal("expected changed=true after adding a skill")
	}
	skillPath := ".openagent/skills/custom/deploy-checklist-" + skill.ID[:8] + "/SKILL.md"
	found := false
	for _, f := range res3["files"].([]interface{}) {
		fm := f.(map[string]interface{})
		if fm["path"] == skillPath {
			found = true
			content := fm["content"].(string)
			// frontmatter name must be a valid skill identifier (= directory basename, kebab-case),
			// not a natural-language title; description remains the human-readable summary.
			wantName := "deploy-checklist-" + skill.ID[:8]
			if !strings.HasPrefix(content, "---\nname: \""+wantName+"\"\ndescription: \"Steps to deploy safely.\"\n---\n") {
				t.Errorf("unexpected SKILL.md frontmatter:\n%s", content)
			}
		}
		if fm["path"] == ".openagent/skills/index.json" {
			if !strings.Contains(fm["content"].(string), skill.ID) {
				t.Error("skills/index.json missing the new skill")
			}
		}
	}
	if !found {
		t.Fatalf("bundle missing %s", skillPath)
	}

	var tpl models.PublicSkillTemplate
	if err := database.DB.First(&tpl, "slug = ?", "go-service-review").Error; err != nil {
		t.Fatalf("load public skill template: %v", err)
	}
	install := models.SkillInstall{
		WorkspaceID:      proj.WorkspaceID,
		TemplateID:       tpl.ID,
		InstalledVersion: tpl.Version,
		State:            "active",
		Pinned:           true,
		OverrideContent:  tpl.Content,
		InstalledBy:      "test-user",
		InstalledAt:      time.Now(),
	}
	if err := database.DB.Create(&install).Error; err != nil {
		t.Fatalf("install public skill: %v", err)
	}
	t.Cleanup(func() { database.DB.Delete(&install) })

	res4 := callTool(t, "hub.sync_project", map[string]interface{}{
		"project_path": projectPath,
	}, nil)
	publicPath := ".openagent/skills/public/go-service-review-v1-" + install.ID[:8] + "/SKILL.md"
	foundPublic := false
	for _, f := range res4["files"].([]interface{}) {
		fm := f.(map[string]interface{})
		if fm["path"] == publicPath {
			foundPublic = strings.Contains(fm["content"].(string), "Go Service Review")
		}
		if fm["path"] == ".openagent/skills/index.json" {
			index := fm["content"].(string)
			if !strings.Contains(index, install.ID) || !strings.Contains(index, tpl.ID) {
				t.Error("skills/index.json missing public skill install metadata")
			}
		}
	}
	if !foundPublic {
		t.Fatalf("bundle missing %s", publicPath)
	}

	listed := callTool(t, "hub.list_skills", map[string]interface{}{
		"project_path": projectPath,
		"source":       "public",
	}, nil)
	listSkills, _ := listed["skills"].([]interface{})
	if len(listSkills) == 0 {
		t.Fatal("hub.list_skills should include installed public skills")
	}
	searched := callTool(t, "hub.search_skills", map[string]interface{}{
		"project_path": projectPath,
		"query":        "Go review",
		"source":       "public",
	}, nil)
	searchSkills, _ := searched["skills"].([]interface{})
	if len(searchSkills) == 0 {
		t.Fatal("hub.search_skills should match installed public skills")
	}

	// Fourth time: X-Project-Path header drives get_project_rules (no project_id passed)
	rules := callTool(t, "hub.get_project_rules", map[string]interface{}{},
		map[string]string{"X-Project-Path": projectPath})
	if _, ok := rules["effective_rules"]; !ok {
		t.Fatalf("expected effective_rules via X-Project-Path binding, got %v", rules)
	}
}

func TestSyncProjectUnboundPathGivesHint(t *testing.T) {
	res := callTool(t, "hub.sync_project", map[string]interface{}{
		"project_path": "/tmp/oah-nonexistent-path",
	}, nil)
	if res["project"] != nil {
		t.Fatalf("expected nil project, got %v", res["project"])
	}
	if hint, _ := res["hint"].(string); hint == "" {
		t.Fatal("expected hint for unbound path")
	}
	// Unbound project path should still return workspace-level files
	files, _ := res["files"].([]interface{})
	if len(files) == 0 {
		t.Fatal("expected workspace-level files even without a project")
	}
	if instr, _ := res["instructions"].(string); instr == "" {
		t.Fatal("expected instructions even for unbound path (files still returned)")
	}
}

// callToolErr calls a tool and expects a JSON-RPC error, returning the error message.
func callToolErr(t *testing.T, name string, args map[string]interface{}, headers map[string]string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	gw := mcp.NewGateway(testCfg)
	r.POST("/mcp", gw.HandleHTTP)

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]interface{}{"name": name, "arguments": args},
	})
	req, _ := http.NewRequest("POST", "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+getTestToken(t))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp struct {
		Error map[string]interface{} `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error == nil {
		t.Fatalf("expected rpc error, got: %s", w.Body.String())
	}
	msg, _ := resp.Error["message"].(string)
	return msg
}

func TestProjectBindingForMemories(t *testing.T) {
	base := "/tmp/oah-bind-test"

	// 0) When unbound, scope=project must error and guide to hub.sync_project
	errMsg := callToolErr(t, "hub.save_memory", map[string]interface{}{
		"content": "orphan project memory",
		"scope":   "project",
	}, map[string]string{"Mcp-Session-Id": "sess-bind-none"})
	if !strings.Contains(errMsg, "hub.sync_project") {
		t.Fatalf("expected guidance to hub.sync_project, got: %s", errMsg)
	}

	// 1) Register project (path with trailing slash; verify normalization) and establish session binding
	res := callTool(t, "hub.sync_project", map[string]interface{}{
		"project_path":     base + "/",
		"register_project": true,
		"project_name":     "Bind Test",
	}, map[string]string{"Mcp-Session-Id": "sess-bind-1"})
	project, _ := res["project"].(map[string]interface{})
	if project == nil {
		t.Fatalf("expected registered project, got %v", res)
	}
	projectID, _ := project["id"].(string)
	if project["repo_path"] != base {
		t.Errorf("expected normalized repo_path %q, got %v", base, project["repo_path"])
	}
	t.Cleanup(func() {
		database.DB.Unscoped().Where("id = ?", projectID).Delete(&models.Project{})
		database.DB.Unscoped().Where("project_id = ?", projectID).Delete(&models.Memory{})
		database.DB.Unscoped().Where("id IN ?", []string{"sess-bind-1", "sess-bind-none"}).Delete(&models.MCPSession{})
	})

	// 2) Explicit project_path (without trailing slash) saves project memory
	out := callTool(t, "hub.save_memory", map[string]interface{}{
		"content":      "explicit path project memory",
		"scope":        "project",
		"project_path": base,
	}, nil)
	var m models.Memory
	if err := database.DB.First(&m, "id = ?", out["memory_id"]).Error; err != nil {
		t.Fatalf("load memory: %v", err)
	}
	if m.ProjectID == nil || *m.ProjectID != projectID {
		t.Fatalf("expected memory bound to project %s, got %v", projectID, m.ProjectID)
	}

	// 3) Same session without project_path should inherit session binding
	out2 := callTool(t, "hub.save_memory", map[string]interface{}{
		"content": "session bound project memory",
		"scope":   "project",
	}, map[string]string{"Mcp-Session-Id": "sess-bind-1"})
	var m2 models.Memory
	if err := database.DB.First(&m2, "id = ?", out2["memory_id"]).Error; err != nil {
		t.Fatalf("load memory: %v", err)
	}
	if m2.ProjectID == nil || *m2.ProjectID != projectID {
		t.Fatalf("expected session-inherited binding to %s, got %v", projectID, m2.ProjectID)
	}

	// 4) scope=project retrieval filters by project
	sr := callTool(t, "hub.search_memory", map[string]interface{}{
		"query":        "project memory",
		"scope":        "project",
		"project_path": base,
	}, nil)
	if cnt, _ := sr["count"].(float64); cnt < 2 {
		t.Fatalf("expected >=2 project memories, got %v", sr["count"])
	}

	// 5) Same project name registered to another path should auto-deduplicate slug
	res2 := callTool(t, "hub.sync_project", map[string]interface{}{
		"project_path":     base + "-second",
		"register_project": true,
		"project_name":     "Bind Test",
	}, nil)
	p2, _ := res2["project"].(map[string]interface{})
	if p2 == nil {
		t.Fatalf("expected second project, got %v", res2)
	}
	t.Cleanup(func() {
		database.DB.Unscoped().Where("id = ?", p2["id"]).Delete(&models.Project{})
	})
	if p2["slug"] == project["slug"] {
		t.Fatalf("expected deduplicated slug, both are %v", p2["slug"])
	}
}

// TestSyncProjectRecordsSync verifies that hub.sync_project writes a SyncRecord as a side effect:
// first sync creates one row (sync_count=1), subsequent syncs by the same identity upsert (no new row),
// even when etag is unchanged (changed=false) it still records a "touch".
func TestSyncProjectRecordsSync(t *testing.T) {
	projectPath := "/tmp/oah-syncrecord-test"

	res := callTool(t, "hub.sync_project", map[string]interface{}{
		"project_path":     projectPath,
		"register_project": true,
		"project_name":     "SyncRecord Test",
	}, nil)
	project, _ := res["project"].(map[string]interface{})
	if project == nil {
		t.Fatalf("expected registered project, got %v", res["project"])
	}
	projectID, _ := project["id"].(string)
	t.Cleanup(func() {
		database.DB.Where("project_id = ?", projectID).Delete(&models.SyncRecord{})
		database.DB.Unscoped().Where("id = ?", projectID).Delete(&models.Project{})
	})

	var records []models.SyncRecord
	database.DB.Where("project_id = ?", projectID).Find(&records)
	if len(records) != 1 {
		t.Fatalf("expected exactly 1 sync record after first sync, got %d", len(records))
	}
	first := records[0]
	if first.SyncCount != 1 {
		t.Errorf("expected sync_count=1, got %d", first.SyncCount)
	}
	if first.RepoPath != projectPath {
		t.Errorf("expected repo_path=%q, got %q", projectPath, first.RepoPath)
	}
	if first.ETag == "" {
		t.Error("expected non-empty etag on sync record")
	}

	// Second sync (same identity, etag unchanged -> changed=false), should still record -> upsert increment, no new row
	res2 := callTool(t, "hub.sync_project", map[string]interface{}{
		"project_path": projectPath,
		"etag":         first.ETag,
	}, nil)
	if res2["changed"] != false {
		t.Fatalf("expected changed=false on re-sync, got %v", res2["changed"])
	}
	var after []models.SyncRecord
	database.DB.Where("project_id = ?", projectID).Find(&after)
	if len(after) != 1 {
		t.Fatalf("expected still 1 sync record after re-sync (upsert), got %d", len(after))
	}
	if after[0].SyncCount != 2 {
		t.Errorf("expected sync_count=2 after re-sync, got %d", after[0].SyncCount)
	}
}

// TestGetSkill verifies that hub.get_skill fetches custom and public skills by ID.
func TestGetSkill(t *testing.T) {
	// Setup: create a custom skill
	var ws models.Workspace
	database.DB.First(&ws)
	skill := models.Memory{
		OrgID:       ws.OrgID,
		WorkspaceID: ws.ID,
		UserID:      "test-user",
		Content:     "# Test Skill\n\nDo the thing.",
		Type:        "skill",
		Category:    "procedural",
		Scope:       "workspace",
		Provenance:  "human_curated",
		Importance:  0.7,
		State:       "active",
		Version:     1,
	}
	database.DB.Create(&skill)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&skill) })

	// Fetch custom skill by memory_id
	res := callTool(t, "hub.get_skill", map[string]interface{}{
		"skill_id": skill.ID,
	}, nil)
	if res["source"] != "custom" {
		t.Fatalf("expected source=custom, got %v", res["source"])
	}
	if res["name"] != "Test Skill" {
		t.Fatalf("expected name=Test Skill, got %v", res["name"])
	}
	if res["memory_id"] != skill.ID {
		t.Fatalf("expected memory_id=%s, got %v", skill.ID, res["memory_id"])
	}

	// Non-existent skill_id should return an error
	errMsg := callToolErr(t, "hub.get_skill", map[string]interface{}{
		"skill_id": "nonexistent-id",
	}, nil)
	if !strings.Contains(errMsg, "not found") {
		t.Fatalf("expected 'not found' error, got: %s", errMsg)
	}
}

// TestGetProjectStack verifies that hub.get_project_stack returns the project tech stack.
func TestGetProjectStack(t *testing.T) {
	projectPath := "/tmp/oah-stack-test"
	res := callTool(t, "hub.sync_project", map[string]interface{}{
		"project_path":     projectPath,
		"register_project": true,
		"project_name":     "Stack Test",
	}, nil)
	project, _ := res["project"].(map[string]interface{})
	projectID, _ := project["id"].(string)
	t.Cleanup(func() {
		database.DB.Unscoped().Where("id = ?", projectID).Delete(&models.Project{})
	})

	// Set stack
	database.DB.Model(&models.Project{}).Where("id = ?", projectID).Update("stack", `{"backend":"go","frontend":"react"}`)

	stackRes := callTool(t, "hub.get_project_stack", map[string]interface{}{
		"project_path": projectPath,
	}, nil)
	if stackRes["project_id"] != projectID {
		t.Fatalf("expected project_id=%s, got %v", projectID, stackRes["project_id"])
	}
	stack, _ := stackRes["stack"].(string)
	if !strings.Contains(stack, "go") {
		t.Fatalf("expected stack to contain 'go', got %v", stack)
	}
}

// TestGetProjectStructure verifies that hub.get_project_structure returns the project directory structure.
func TestGetProjectStructure(t *testing.T) {
	projectPath := "/tmp/oah-structure-test"
	res := callTool(t, "hub.sync_project", map[string]interface{}{
		"project_path":     projectPath,
		"register_project": true,
		"project_name":     "Structure Test",
	}, nil)
	project, _ := res["project"].(map[string]interface{})
	projectID, _ := project["id"].(string)
	t.Cleanup(func() {
		database.DB.Unscoped().Where("id = ?", projectID).Delete(&models.Project{})
	})

	database.DB.Model(&models.Project{}).Where("id = ?", projectID).Update("structure", `{"src":["pages","components"]}`)

	structRes := callTool(t, "hub.get_project_structure", map[string]interface{}{
		"project_path": projectPath,
	}, nil)
	if structRes["project_id"] != projectID {
		t.Fatalf("expected project_id=%s, got %v", projectID, structRes["project_id"])
	}
	structure, _ := structRes["structure"].(string)
	if !strings.Contains(structure, "pages") {
		t.Fatalf("expected structure to contain 'pages', got %v", structure)
	}
}

// TestUpdateProjectContext verifies that hub.update_project_context updates project metadata.
func TestUpdateProjectContext(t *testing.T) {
	projectPath := "/tmp/oah-update-ctx-test"
	res := callTool(t, "hub.sync_project", map[string]interface{}{
		"project_path":     projectPath,
		"register_project": true,
		"project_name":     "Update Ctx Test",
	}, nil)
	project, _ := res["project"].(map[string]interface{})
	projectID, _ := project["id"].(string)
	t.Cleanup(func() {
		database.DB.Unscoped().Where("id = ?", projectID).Delete(&models.Project{})
		database.DB.Where("target = ?", projectID).Delete(&models.AuditLog{})
	})

	// Update description + stack + structure
	upd := callTool(t, "hub.update_project_context", map[string]interface{}{
		"project_path": projectPath,
		"description":  "A test project for context updates",
		"stack":        `{"lang":"go"}`,
		"structure":    `{"cmd":["server"]}`,
	}, nil)
	if upd["updated"] != true {
		t.Fatalf("expected updated=true, got %v", upd["updated"])
	}
	if upd["description"] != "A test project for context updates" {
		t.Fatalf("expected updated description, got %v", upd["description"])
	}
	if upd["stack"] != `{"lang":"go"}` {
		t.Fatalf("expected updated stack, got %v", upd["stack"])
	}
	if upd["structure"] != `{"cmd":["server"]}` {
		t.Fatalf("expected updated structure, got %v", upd["structure"])
	}

	// Verify DB persistence
	var p models.Project
	database.DB.First(&p, "id = ?", projectID)
	if p.Description != "A test project for context updates" {
		t.Fatalf("DB description mismatch: %s", p.Description)
	}

	// Verify audit log
	var audit models.AuditLog
	if err := database.DB.Where("target = ? AND action = ?", projectID, "project.update_context").First(&audit).Error; err != nil {
		t.Fatalf("expected audit log, got: %v", err)
	}

	// Should error when no fields are provided
	errMsg := callToolErr(t, "hub.update_project_context", map[string]interface{}{
		"project_path": projectPath,
	}, nil)
	if !strings.Contains(errMsg, "at least one") {
		t.Fatalf("expected 'at least one' error, got: %s", errMsg)
	}
}
