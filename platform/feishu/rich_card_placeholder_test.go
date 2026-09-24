package feishu

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ChamberZ40/magpie/core"
)

// A turn with no tool calls and no thinking events used to synthesize an empty
// Reasoning panel carrying a hardcoded English "Thinking..." placeholder, and
// later got no panel at all. Neither told a reader what they wanted to know at
// a glance: that the turn ran no tools. The Tools panel now always renders,
// and with nothing in it says so in the reader's language.

func TestBuildRichCard_ToolsPanelShowsWhenNoToolRan(t *testing.T) {
	for _, streaming := range []cardStreaming{{enabled: true}, {}} {
		cardJSON := buildRichCard(core.CardStatusThinking, "zh", nil, "答案是 42", streaming, "")
		panels := collectCardPanels(t, cardJSON)
		if len(panels) != 1 {
			t.Fatalf("streaming=%v: panel count = %d, want only the Tools panel: %#v",
				streaming.enabled, len(panels), panels)
		}
		panel, _ := json.Marshal(panels[0])
		for _, want := range []string{"🔧 工具", "本轮未调用工具"} {
			if !strings.Contains(string(panel), want) {
				t.Errorf("streaming=%v: tools panel = %s, want it to contain %q", streaming.enabled, panel, want)
			}
		}
		if strings.Contains(cardJSON, "Thinking...") {
			t.Errorf("streaming=%v: card still carries the hardcoded placeholder: %s", streaming.enabled, cardJSON)
		}
		if !strings.Contains(cardJSON, "42") {
			t.Errorf("streaming=%v: card lost its body: %s", streaming.enabled, cardJSON)
		}
	}
}

func TestBuildRichCard_NoToolsPlaceholderIsLocalized(t *testing.T) {
	for lang, want := range map[string]string{
		"en": "No tools called", "zh": "本轮未调用工具", "zh-TW": "本輪未呼叫工具",
		"ja": "ツールの呼び出しなし", "es": "No se llamó a ninguna herramienta",
	} {
		if cardJSON := buildRichCard(core.CardStatusDone, lang, nil, "ok", cardStreaming{}, ""); !strings.Contains(cardJSON, want) {
			t.Errorf("lang=%q: card should contain %q: %s", lang, want, cardJSON)
		}
	}
}

// The one placeholder a reader can actually reach — the summary standing in for
// steps trimmed out of an over-long panel — has to speak their language.
func TestBuildRichCard_HiddenStepSummaryIsLocalized(t *testing.T) {
	var steps []core.ToolStep
	for i := 0; i < 15; i++ {
		steps = append(steps, core.ToolStep{Kind: core.ToolStepKindTool, Name: "Bash", Summary: "cmd", Done: true})
	}

	tests := []struct {
		lang string
		want string
	}{
		{"en", "5 earlier steps hidden"},
		{"zh", "已隐藏 5 个更早的步骤"},
		{"zh-TW", "已隱藏 5 個更早的步驟"},
		{"ja", "以前のステップ 5 件を非表示"},
		{"es", "5 pasos anteriores ocultos"},
	}
	for _, tt := range tests {
		cardJSON := buildRichCard(core.CardStatusWorking, tt.lang, steps, "", cardStreaming{enabled: true}, "")
		if !strings.Contains(cardJSON, tt.want) {
			t.Errorf("lang=%q: card should contain %q: %s", tt.lang, tt.want, cardJSON)
		}
	}
}
