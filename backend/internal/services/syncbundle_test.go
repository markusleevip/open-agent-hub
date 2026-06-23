package services

import (
	"strings"
	"testing"

	"github.com/openagenthub/backend/internal/models"
)

func TestUpsertManagedBlock_InsertIntoEmptyDoc(t *testing.T) {
	block := RenderManagedBlock(nil)
	out := UpsertManagedBlock("", block)
	if !strings.Contains(out, ManagedBlockBegin) || !strings.Contains(out, ManagedBlockEnd) {
		t.Fatalf("markers missing in output:\n%s", out)
	}
}

func TestUpsertManagedBlock_AppendPreservesUserContent(t *testing.T) {
	doc := "# My Project\n\nuser notes here\n"
	block := RenderManagedBlock(nil)
	out := UpsertManagedBlock(doc, block)
	if !strings.HasPrefix(out, "# My Project") {
		t.Fatalf("user content not preserved at top:\n%s", out)
	}
	if !strings.Contains(out, "user notes here") {
		t.Fatal("user content lost")
	}
	if !strings.Contains(out, ManagedBlockBegin) {
		t.Fatal("managed block not appended")
	}
}

func TestUpsertManagedBlock_ReplaceIsIdempotent(t *testing.T) {
	doc := "before\n\n" + ManagedBlockBegin + "\nold content\n" + ManagedBlockEnd + "\n\nafter\n"
	block := RenderManagedBlock(nil)

	out1 := UpsertManagedBlock(doc, block)
	out2 := UpsertManagedBlock(out1, block)
	if out1 != out2 {
		t.Fatalf("upsert not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out1, out2)
	}
	if strings.Contains(out1, "old content") {
		t.Fatal("old block content not replaced")
	}
	if !strings.Contains(out1, "before") || !strings.Contains(out1, "after") {
		t.Fatal("content outside the block was modified")
	}
	if strings.Count(out1, ManagedBlockBegin) != 1 {
		t.Fatalf("expected exactly one managed block, got %d", strings.Count(out1, ManagedBlockBegin))
	}
}

func TestRenderManagedBlock_MentionsCoreInstructions(t *testing.T) {
	block := RenderManagedBlock(nil)
	for _, want := range []string{
		".openagent/rules.md",
		".openagent/skills/",
		"hub.propose_memory",
		"hub.sync_project",
		"hub.get_tool_policy",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("managed block missing %q", want)
		}
	}
}

func TestSkillNameAndDescription(t *testing.T) {
	name, desc := skillNameAndDescription("# Deploy Checklist\n\nSteps to deploy safely.\n\nmore text")
	if name != "Deploy Checklist" {
		t.Errorf("name = %q", name)
	}
	if desc != "Steps to deploy safely." {
		t.Errorf("description = %q", desc)
	}

	name, desc = skillNameAndDescription("plain first line\nsecond line")
	if name != "plain first line" || desc != "second line" {
		t.Errorf("got %q / %q", name, desc)
	}

	name, desc = skillNameAndDescription("")
	if name != "Untitled Skill" || desc != "Untitled Skill" {
		t.Errorf("empty content fallback: %q / %q", name, desc)
	}

	// Long name truncation should add ellipsis
	longName := strings.Repeat("a", 100)
	name, desc = skillNameAndDescription(longName)
	if !strings.HasSuffix(name, "...") {
		t.Errorf("truncated name should end with '...': got %q (len=%d)", name, len(name))
	}
	if len([]rune(name)) != 80 {
		t.Errorf("truncated name should be 80 runes: got %d", len([]rune(name)))
	}
	// When truncated, description falls back to full name (before truncation)
	if desc != longName {
		t.Errorf("description should be full name before truncation: got %q", desc)
	}
}

func TestYamlQuote(t *testing.T) {
	cases := map[string]string{
		"simple":                   `"simple"`,
		"has: colon space":         `"has: colon space"`,
		`has "quote"`:              `"has \"quote\""`,
		`back\slash`:               `"back\\slash"`,
		"":                         `""`,
	}
	for in, want := range cases {
		if got := yamlQuote(in); got != want {
			t.Errorf("yamlQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderSkillMarkdownYamlSafe(t *testing.T) {
	// name contains colon+space; should not break YAML frontmatter
	name := "VibeCoding: project bootstrap"
	desc := "Step 1: read rules. Step 2: load context."
	content := "# VibeCoding: project bootstrap\n\nInstructions here."
	out := renderSkillMarkdown(name, desc, content)

	// name/description in frontmatter must be double-quoted
	if !strings.Contains(out, "name: \""+name+"\""+"\n") {
		t.Errorf("name not properly quoted in frontmatter:\n%s", out)
	}
	if !strings.Contains(out, "description: \""+desc+"\""+"\n") {
		t.Errorf("description not properly quoted in frontmatter:\n%s", out)
	}
	// Original content should be preserved
	if !strings.Contains(out, "Instructions here.") {
		t.Errorf("skill content lost:\n%s", out)
	}
}

func TestSlugifySkillName(t *testing.T) {
	cases := map[string]string{
		"Deploy Checklist":  "deploy-checklist",
		"API v2 -- rollout": "api-v2-rollout",
		"部署清单":              "skill", // non-ASCII falls back to placeholder; dir name uniqueness ensured by id suffix
		"":                  "skill",
	}
	for in, want := range cases {
		if got := slugifySkillName(in); got != want {
			t.Errorf("slugifySkillName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeGitRemote(t *testing.T) {
	cases := map[string]string{
		"git@github.com:org/repo.git":                "github.com/org/repo",
		"https://github.com/org/repo.git":            "github.com/org/repo",
		"https://github.com/org/repo":                "github.com/org/repo",
		"https://user:token@github.com/org/repo.git": "github.com/org/repo",
		"ssh://git@github.com/org/repo.git":          "github.com/org/repo",
		"git@GitHub.com:Org/Repo.git":                "github.com/Org/Repo", // host lowercased, path case preserved
		"https://gitlab.example.com/group/sub/proj":  "gitlab.example.com/group/sub/proj",
		"  https://github.com/org/repo.git/  ":       "github.com/org/repo",
		"":                                           "",
		"   ":                                        "",
	}
	for in, want := range cases {
		if got := NormalizeGitRemote(in); got != want {
			t.Errorf("NormalizeGitRemote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRepoNameFromPath(t *testing.T) {
	cases := map[string]string{
		"/Users/mark/work/super-mario": "super-mario",
		"/home/bob/dev/super-mario/":   "super-mario",
		"/home/bob/dev/super-mario//":  "super-mario",
		"/":                            "",
		"":                             "",
	}
	for in, want := range cases {
		if got := RepoNameFromPath(in); got != want {
			t.Errorf("RepoNameFromPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderSyncInstructions(t *testing.T) {
	// No project (workspace-level)
	b1 := &SyncBundle{ETag: "abc123", ManagedFiles: []string{"CLAUDE.md", "AGENTS.md"}}
	out1 := RenderSyncInstructions(b1)
	if out1 == "" {
		t.Fatal("expected non-empty instructions")
	}
	// When unbound, should not mention project.md
	if strings.Contains(out1, "project.md") {
		t.Errorf("unbound instructions should not mention project.md:\n%s", out1)
	}
	// .gitignore step should appear regardless of project presence
	if !strings.Contains(out1, ".gitignore") {
		t.Errorf("instructions should mention .gitignore even without project")
	}

	// With project
	b2 := &SyncBundle{
		ETag:         "def456",
		Project:      &models.Project{Name: "x"},
		ManagedFiles: []string{"CLAUDE.md", "AGENTS.md"},
	}
	out2 := RenderSyncInstructions(b2)
	// Key elements assertion
	for _, want := range []string{
		"files",
		"managed_block",
		"managed_files",
		"CLAUDE.md",
		"AGENTS.md",
		ManagedBlockBegin,
		ManagedBlockEnd,
		".openagent/rules.md",
		".openagent/project.md", // should appear when project is present
		".openagent/local/state.json",
		"def456", // etag is embedded
		".gitignore",       // Step 5 should mention gitignore
		"git repository",   // Step 5 should mention git repo detection
	} {
		if !strings.Contains(out2, want) {
			t.Errorf("instructions missing %q", want)
		}
	}
	// Marker three-branch keywords should all appear
	for _, want := range []string{"replace", "missing or empty", "append"} {
		if !strings.Contains(out2, want) {
			t.Errorf("instructions missing marker-rule keyword %q", want)
		}
	}
}
