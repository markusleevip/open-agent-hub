package services

import "testing"

func TestPersonalInstructionsFromRowsDefaults(t *testing.T) {
	got := personalInstructionsFromRows(nil)
	if got.Language != "zh-CN" || got.Verbosity != "normal" || got.CodeStyle != "google" {
		t.Fatalf("unexpected defaults: %+v", got)
	}
	if !got.Memory.Enabled || !got.Memory.SkipToolContext {
		t.Fatalf("unexpected memory defaults: %+v", got.Memory)
	}
}

func TestNormalizePersonalInstructionsRejectsInvalidValues(t *testing.T) {
	_, err := NormalizePersonalInstructions(PersonalInstructions{
		Language:      "jp-JP",
		Verbosity:     "normal",
		CodeStyle:     "google",
		Personality:   "pragmatic",
		ResponseStyle: "direct",
		Memory: PersonalInstructionMemory{
			Enabled:         true,
			SkipToolContext: true,
		},
	})
	if err == nil {
		t.Fatal("expected invalid language error")
	}
}

func TestPersonalInstructionsToRows(t *testing.T) {
	rows := personalInstructionsToRows(PersonalInstructions{
		Language:           "zh-CN",
		Verbosity:          "detailed",
		CodeStyle:          "project",
		Personality:        "rigorous",
		ResponseStyle:      "checklist",
		CustomInstructions: "默认中文回复。",
		Memory: PersonalInstructionMemory{
			Enabled:         false,
			SkipToolContext: true,
		},
	})
	if rows[OutputPrefMemoryEnabled] != "false" {
		t.Fatalf("memory_enabled = %q", rows[OutputPrefMemoryEnabled])
	}
	if rows[OutputPrefSkipToolMemory] != "true" || rows["skip_tool_context"] != "true" {
		t.Fatalf("skip tool flags not mirrored: %+v", rows)
	}
	if rows[OutputPrefCustomInstructions] != "默认中文回复。" {
		t.Fatalf("custom instructions mismatch: %+v", rows)
	}
}
