package services

import (
	"testing"
	"time"

	"github.com/openagenthub/backend/internal/models"
)

func sp(s string) *string { return &s }

// rule builds a test rule. Passing "" for projectID / agentName means that dimension is empty (more permissive).
func rule(id, typ, name, value, projectID, agentName string, version int) models.Rule {
	r := models.Rule{
		Name:    name,
		Type:    typ,
		Value:   value,
		Version: version,
	}
	r.ID = id
	if projectID != "" {
		r.ProjectID = sp(projectID)
	}
	if agentName != "" {
		r.AgentName = sp(agentName)
	}
	return r
}

// valueOf returns the value of a (type,name) pair from the effective rules, or "" if not found.
func valueOf(rules []EffectiveRule, typ, name string) string {
	for _, r := range rules {
		if r.Type == typ && r.Name == name {
			return r.Value
		}
	}
	return ""
}

func TestMergeRules_OverridePrecedence(t *testing.T) {
	// Same (type=style, name=lang) across workspace / project / agent / project+agent — four layers
	rules := []models.Rule{
		rule("r-ws", "style", "lang", "ws", "", "", 1),
		rule("r-proj", "style", "lang", "proj", "P1", "", 1),
		rule("r-agent", "style", "lang", "agent", "", "cursor", 1),
		rule("r-both", "style", "lang", "both", "P1", "cursor", 1),
	}

	tests := []struct {
		name      string
		projectID *string
		agentName *string
		want      string
	}{
		{"workspace global resolution", nil, nil, "ws"},
		{"project context only", sp("P1"), nil, "proj"},
		{"agent context only", nil, sp("cursor"), "agent"},
		{"project+agent context picks most specific", sp("P1"), sp("cursor"), "both"},
		{"agent overrides project (when no both rule)", sp("P1"), sp("claude"), "proj"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MergeRules(rules, tc.projectID, tc.agentName)
			if v := valueOf(got, "style", "lang"); v != tc.want {
				t.Fatalf("want %q, got %q (rules=%+v)", tc.want, v, got)
			}
			if len(got) != 1 {
				t.Fatalf("same override key should yield exactly 1 rule, got %d", len(got))
			}
		})
	}
}

func TestMergeRules_NoLeakAcrossProjectsOrAgents(t *testing.T) {
	rules := []models.Rule{
		rule("r-p1", "lint", "x", "p1", "P1", "", 1),
		rule("r-p2", "lint", "x", "p2", "P2", "", 1),
		rule("r-a1", "lint", "y", "a1", "", "cursor", 1),
	}

	// Resolve P1: should not see P2's rules
	got := MergeRules(rules, sp("P1"), nil)
	if v := valueOf(got, "lint", "x"); v != "p1" {
		t.Fatalf("P1 should resolve to p1, got %q", v)
	}
	// agentName=nil in this context; agent-specific rule r-a1 should not appear
	if v := valueOf(got, "lint", "y"); v != "" {
		t.Fatalf("agent-specific rule should not leak into agent-less resolution, got %q", v)
	}

	// Workspace global resolution (project=nil): no project-specific rules should appear
	gw := MergeRules(rules, nil, nil)
	if len(gw) != 0 {
		t.Fatalf("global resolution should not include any project/agent-specific rules, got %+v", gw)
	}
}

func TestMergeRules_DistinctKeysCoexist(t *testing.T) {
	// Rules with different (type,name) should coexist and be sorted by (type,name)
	rules := []models.Rule{
		rule("b", "style", "indent", "tab", "", "", 1),
		rule("a", "lint", "max_len", "100", "", "", 1),
		rule("c", "style", "color", "off", "", "", 1),
	}
	got := MergeRules(rules, nil, nil)
	if len(got) != 3 {
		t.Fatalf("expected 3 effective rules, got %d", len(got))
	}
	wantOrder := []struct{ typ, name string }{
		{"lint", "max_len"}, {"style", "color"}, {"style", "indent"},
	}
	for i, w := range wantOrder {
		if got[i].Type != w.typ || got[i].Name != w.name {
			t.Fatalf("position %d should be (%s,%s), got (%s,%s)", i, w.typ, w.name, got[i].Type, got[i].Name)
		}
	}
}

func TestMergeRules_TieBreakByVersionThenUpdatedAt(t *testing.T) {
	now := time.Now()
	// Same specificity (both workspace-global); higher version wins
	r1 := rule("r1", "k", "n", "v1", "", "", 1)
	r2 := rule("r2", "k", "n", "v2", "", "", 3)
	r1.UpdatedAt = now
	r2.UpdatedAt = now.Add(-time.Hour) // even though older, higher version wins
	got := MergeRules([]models.Rule{r1, r2}, nil, nil)
	if v := valueOf(got, "k", "n"); v != "v2" {
		t.Fatalf("higher version should win, got %q", v)
	}

	// Same version; more recently updated wins
	r3 := rule("r3", "k", "m", "old", "", "", 2)
	r4 := rule("r4", "k", "m", "new", "", "", 2)
	r3.UpdatedAt = now.Add(-time.Hour)
	r4.UpdatedAt = now
	got2 := MergeRules([]models.Rule{r3, r4}, nil, nil)
	if v := valueOf(got2, "k", "m"); v != "new" {
		t.Fatalf("same version: newer UpdatedAt should win, got %q", v)
	}
}

func TestRuleValuesAndETag(t *testing.T) {
	rules := []models.Rule{
		rule("a", "lint", "x", "AAA", "", "", 1),
		rule("b", "style", "y", "BBB", "", "", 1),
	}
	eff := MergeRules(rules, nil, nil)
	vals := RuleValues(eff)
	if len(vals) != 2 || vals[0] != "AAA" || vals[1] != "BBB" {
		t.Fatalf("RuleValues order/content wrong: %+v", vals)
	}

	// ETag stability: same input -> same ETag
	if RuleETag(eff) != RuleETag(MergeRules(rules, nil, nil)) {
		t.Fatal("same effective rules should produce the same ETag")
	}
	// Version change -> ETag change
	rules[0].Version = 2
	if RuleETag(eff) == RuleETag(MergeRules(rules, nil, nil)) {
		t.Fatal("ETag should change after version bump")
	}
}
