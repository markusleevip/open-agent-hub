package services

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
	"strconv"

	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/models"
)

// config_resolver implements spec Appendix E.3 P0-1: three-level configuration override merge engine.
//
// Priority (specificity, higher overrides lower):
//
//	workspace global (project_id=nil, agent_name=nil)  -> 0
//	project level    (project_id match)                -> +1
//	agent level      (agent_name match)                -> +2
//
// i.e. agent > project > workspace. The override key is (Type, Name): within a given (project, agent)
// context, a more specific rule with the same (Type, Name) overrides a broader one, rather than
// returning multiple results simultaneously.

// EffectiveRule is the surviving rule after merging, with its source level information attached.
type EffectiveRule struct {
	models.Rule
	Specificity int    `json:"specificity"` // 0=workspace, 1=project, 2=agent, 3=project+agent
	OriginScope string `json:"origin_scope"`
}

// ruleKey is the override key: the same (Type, Name) is treated as the same logical configuration.
func ruleKey(r models.Rule) string {
	return r.Type + "\x00" + r.Name
}

// applicable determines whether a rule applies to the given (project, agent) context and returns
// its specificity.
//
//   - project dimension: rule project_id is nil -> applies to all projects; otherwise must equal
//     the context projectID.
//   - agent dimension: rule agent_name is nil -> applies to all agents; otherwise must equal
//     the context agentName.
func applicable(r models.Rule, projectID *string, agentName *string) (int, bool) {
	spec := 0

	if r.ProjectID != nil {
		if projectID == nil || *r.ProjectID != *projectID {
			return 0, false
		}
		spec += 1
	}

	if r.AgentName != nil {
		if agentName == nil || *r.AgentName != *agentName {
			return 0, false
		}
		spec += 2
	}

	return spec, true
}

// better decides the winning rule when override keys conflict: specificity > version > updated_at > id (ensures determinism).
func better(cand EffectiveRule, cur EffectiveRule) bool {
	if cand.Specificity != cur.Specificity {
		return cand.Specificity > cur.Specificity
	}
	if cand.Version != cur.Version {
		return cand.Version > cur.Version
	}
	if !cand.UpdatedAt.Equal(cur.UpdatedAt) {
		return cand.UpdatedAt.After(cur.UpdatedAt)
	}
	return cand.ID > cur.ID
}

func originScope(spec int) string {
	switch spec {
	case 3:
		return "project+agent"
	case 2:
		return "agent"
	case 1:
		return "project"
	default:
		return "workspace"
	}
}

// MergeRules is the pure-function merge engine (no DB dependency, easy to unit test).
// Given a set of candidate rules and a target (project, agent) context, it returns the deduplicated
// effective rules after override resolution, sorted by (Type, Name).
func MergeRules(rules []models.Rule, projectID *string, agentName *string) []EffectiveRule {
	winners := make(map[string]EffectiveRule, len(rules))

	for _, r := range rules {
		spec, ok := applicable(r, projectID, agentName)
		if !ok {
			continue
		}
		cand := EffectiveRule{Rule: r, Specificity: spec, OriginScope: originScope(spec)}
		key := ruleKey(r)
		if cur, exists := winners[key]; !exists || better(cand, cur) {
			winners[key] = cand
		}
	}

	out := make([]EffectiveRule, 0, len(winners))
	for _, w := range winners {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// ResolveEffectiveRules queries all rules in the workspace and merges them into the effective
// configuration for the given (project, agent) context.
//
// A nil projectID means "workspace global" resolution (only rules with nil project_id are included);
// a nil agentName only includes rules with nil agent_name (agent-agnostic rules).
func ResolveEffectiveRules(workspaceID string, projectID *string, agentName *string) []EffectiveRule {
	var rules []models.Rule
	database.DB.Where("workspace_id = ?", workspaceID).Find(&rules)
	return MergeRules(rules, projectID, agentName)
}

// RuleValues extracts the value list of effective rules (in sorted order).
func RuleValues(rules []EffectiveRule) []string {
	values := make([]string, 0, len(rules))
	for _, r := range rules {
		values = append(values, r.Value)
	}
	return values
}

// RuleETag computes a stable ETag based on the (id, version) of effective rules.
func RuleETag(rules []EffectiveRule) string {
	h := md5.New()
	for _, r := range rules {
		h.Write([]byte(r.ID + ":" + strconv.Itoa(r.Version) + ";"))
	}
	return hex.EncodeToString(h.Sum(nil))
}
