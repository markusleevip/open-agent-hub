package services

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/models"
)

const (
	OutputPrefLanguage           = "language"
	OutputPrefVerbosity          = "verbosity"
	OutputPrefCodeStyle          = "code_style"
	OutputPrefPersonality        = "personality"
	OutputPrefResponseStyle      = "response_style"
	OutputPrefCustomInstructions = "custom_instructions"
	OutputPrefMemoryEnabled      = "memory_enabled"
	OutputPrefSkipToolMemory     = "skip_tool_memory"
)

const maxCustomInstructionsLen = 20000

var outputPreferenceKeyOrder = []string{
	OutputPrefLanguage,
	OutputPrefVerbosity,
	OutputPrefCodeStyle,
	OutputPrefPersonality,
	OutputPrefResponseStyle,
	OutputPrefCustomInstructions,
	OutputPrefMemoryEnabled,
	OutputPrefSkipToolMemory,
}

// PersonalInstructions is the console-facing shape for user-scoped output preferences.
type PersonalInstructions struct {
	Language           string                    `json:"language"`
	Verbosity          string                    `json:"verbosity"`
	CodeStyle          string                    `json:"code_style"`
	Personality        string                    `json:"personality"`
	ResponseStyle      string                    `json:"response_style"`
	CustomInstructions string                    `json:"custom_instructions"`
	Memory             PersonalInstructionMemory `json:"memory"`
	UpdatedAt          *time.Time                `json:"updated_at,omitempty"`
}

type PersonalInstructionMemory struct {
	Enabled         bool `json:"enabled"`
	SkipToolContext bool `json:"skip_tool_context"`
}

func DefaultPersonalInstructions() PersonalInstructions {
	return PersonalInstructions{
		Language:      "zh-CN",
		Verbosity:     "normal",
		CodeStyle:     "google",
		Personality:   "pragmatic",
		ResponseStyle: "direct",
		Memory: PersonalInstructionMemory{
			Enabled:         true,
			SkipToolContext: true,
		},
	}
}

func GetPersonalInstructions(userID string) (PersonalInstructions, error) {
	var prefs []models.OutputPreference
	if err := database.DB.Where("user_id = ?", userID).
		Order("key asc, created_at asc").Find(&prefs).Error; err != nil {
		return PersonalInstructions{}, err
	}
	return personalInstructionsFromRows(prefs), nil
}

func SavePersonalInstructions(userID string, input PersonalInstructions) (PersonalInstructions, []string, error) {
	normalized, err := NormalizePersonalInstructions(input)
	if err != nil {
		return PersonalInstructions{}, nil, err
	}
	current, err := GetOutputPreferencesMap(userID)
	if err != nil {
		return PersonalInstructions{}, nil, err
	}

	values := personalInstructionsToRows(normalized)
	changed := make([]string, 0, len(values))
	for _, key := range outputPreferenceKeyOrder {
		next := values[key]
		if current[key] != next {
			changed = append(changed, key)
		}
		if err := upsertOutputPreference(userID, key, next); err != nil {
			return PersonalInstructions{}, nil, err
		}
	}

	saved, err := GetPersonalInstructions(userID)
	if err != nil {
		return PersonalInstructions{}, nil, err
	}
	return saved, changed, nil
}

func NormalizePersonalInstructions(input PersonalInstructions) (PersonalInstructions, error) {
	out := input
	out.Language = strings.TrimSpace(out.Language)
	out.Verbosity = strings.TrimSpace(out.Verbosity)
	out.CodeStyle = strings.TrimSpace(out.CodeStyle)
	out.Personality = strings.TrimSpace(out.Personality)
	out.ResponseStyle = strings.TrimSpace(out.ResponseStyle)

	if out.Language == "" {
		out.Language = "zh-CN"
	}
	if out.Verbosity == "" {
		out.Verbosity = "normal"
	}
	if out.CodeStyle == "" {
		out.CodeStyle = "google"
	}
	if out.Personality == "" {
		out.Personality = "pragmatic"
	}
	if out.ResponseStyle == "" {
		out.ResponseStyle = "direct"
	}

	if !oneOf(out.Language, "zh-CN", "en-US", "auto") {
		return PersonalInstructions{}, fmt.Errorf("invalid language: %s", out.Language)
	}
	if !oneOf(out.Verbosity, "concise", "normal", "detailed") {
		return PersonalInstructions{}, fmt.Errorf("invalid verbosity: %s", out.Verbosity)
	}
	if !oneOf(out.CodeStyle, "google", "standard", "project", "custom") {
		return PersonalInstructions{}, fmt.Errorf("invalid code_style: %s", out.CodeStyle)
	}
	if !oneOf(out.Personality, "pragmatic", "concise", "rigorous", "friendly", "custom") {
		return PersonalInstructions{}, fmt.Errorf("invalid personality: %s", out.Personality)
	}
	if !oneOf(out.ResponseStyle, "direct", "explanatory", "checklist") {
		return PersonalInstructions{}, fmt.Errorf("invalid response_style: %s", out.ResponseStyle)
	}
	if len([]rune(out.CustomInstructions)) > maxCustomInstructionsLen {
		return PersonalInstructions{}, errors.New("custom_instructions exceeds 20000 characters")
	}
	return out, nil
}

func GetOutputPreferencesMap(userID string) (map[string]string, error) {
	instructions, err := GetPersonalInstructions(userID)
	if err != nil {
		return nil, err
	}
	return personalInstructionsToRows(instructions), nil
}

func RenderPersonalProfile(userID string) (string, error) {
	instructions, err := GetPersonalInstructions(userID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("## Output Preferences\n\n")
	b.WriteString("- language: " + instructions.Language + "\n")
	b.WriteString("- verbosity: " + instructions.Verbosity + "\n")
	b.WriteString("- code_style: " + instructions.CodeStyle + "\n")
	b.WriteString("- personality: " + instructions.Personality + "\n")
	b.WriteString("- response_style: " + instructions.ResponseStyle + "\n")
	b.WriteString("\n## Memory Preferences\n\n")
	b.WriteString("- memory_enabled: " + strconv.FormatBool(instructions.Memory.Enabled) + "\n")
	b.WriteString("- skip_tool_memory: " + strconv.FormatBool(instructions.Memory.SkipToolContext) + "\n")
	if strings.TrimSpace(instructions.CustomInstructions) != "" {
		b.WriteString("\n## Custom Instructions\n\n")
		b.WriteString(strings.TrimRight(instructions.CustomInstructions, "\n") + "\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

func personalInstructionsFromRows(prefs []models.OutputPreference) PersonalInstructions {
	out := DefaultPersonalInstructions()
	var latest *time.Time
	for _, p := range prefs {
		switch p.Key {
		case OutputPrefLanguage:
			out.Language = p.Value
		case OutputPrefVerbosity:
			out.Verbosity = p.Value
		case OutputPrefCodeStyle:
			out.CodeStyle = p.Value
		case OutputPrefPersonality:
			out.Personality = p.Value
		case OutputPrefResponseStyle:
			out.ResponseStyle = p.Value
		case OutputPrefCustomInstructions:
			out.CustomInstructions = p.Value
		case OutputPrefMemoryEnabled:
			out.Memory.Enabled = parseBoolDefault(p.Value, true)
		case OutputPrefSkipToolMemory:
			out.Memory.SkipToolContext = parseBoolDefault(p.Value, true)
		}
		if latest == nil || p.UpdatedAt.After(*latest) {
			t := p.UpdatedAt
			latest = &t
		}
	}
	out.UpdatedAt = latest
	normalized, err := NormalizePersonalInstructions(out)
	if err != nil {
		return DefaultPersonalInstructions()
	}
	normalized.UpdatedAt = latest
	return normalized
}

func personalInstructionsToRows(instructions PersonalInstructions) map[string]string {
	return map[string]string{
		OutputPrefLanguage:           instructions.Language,
		OutputPrefVerbosity:          instructions.Verbosity,
		OutputPrefCodeStyle:          instructions.CodeStyle,
		OutputPrefPersonality:        instructions.Personality,
		OutputPrefResponseStyle:      instructions.ResponseStyle,
		OutputPrefCustomInstructions: instructions.CustomInstructions,
		OutputPrefMemoryEnabled:      strconv.FormatBool(instructions.Memory.Enabled),
		OutputPrefSkipToolMemory:     strconv.FormatBool(instructions.Memory.SkipToolContext),
		"skip_tool_context":          strconv.FormatBool(instructions.Memory.SkipToolContext),
	}
}

func upsertOutputPreference(userID, key, value string) error {
	var rows []models.OutputPreference
	if err := database.DB.Where("user_id = ? AND key = ?", userID, key).
		Order("created_at asc, id asc").Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return database.DB.Create(&models.OutputPreference{
			WorkspaceID: "",
			UserID:      userID,
			Key:         key,
			Value:       value,
		}).Error
	}
	if err := database.DB.Model(&rows[0]).Update("value", value).Error; err != nil {
		return err
	}
	for i := 1; i < len(rows); i++ {
		if err := database.DB.Delete(&rows[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func parseBoolDefault(value string, fallback bool) bool {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
