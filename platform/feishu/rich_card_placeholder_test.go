package feishu

import (
	"strings"
	"testing"

	"github.com/ChamberZ40/magpie/core"
)

// A turn with no tool calls and no thinking events used to synthesize an empty
// Reasoning panel carrying a hardcoded English "Thinking..." placeholder. It
// claimed a section of the card to say nothing, and said it in the wrong
// language. The status header already reports that the turn is running.

func TestBuildRichCard_NoPanelWhenThereAreNoSteps(t *testing.T) {
	for _, streaming := range []bool{true, false} {
		cardJSON := buildRichCard(core.CardStatusThinking, "zh", nil, "答案是 42", streaming, "")
		if panels := collectCardPanels(t, cardJSON); len(panels) != 0 {
			t.Errorf("streaming=%v: panel count = %d, want 0 — an empty panel says nothing: %#v",
				streaming, len(panels), panels)
		}
		if strings.Contains(cardJSON, "Thinking...") {
			t.Errorf("streaming=%v: card still carries the hardcoded placeholder: %s", streaming, cardJSON)
		}
		// The body must survive the panel removal.
		if !strings.Contains(cardJSON, "42") {
			t.Errorf("streaming=%v: card lost its body: %s", streaming, cardJSON)
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
		cardJSON := buildRichCard(core.CardStatusWorking, tt.lang, steps, "", true, "")
		if !strings.Contains(cardJSON, tt.want) {
			t.Errorf("lang=%q: card should contain %q: %s", tt.lang, tt.want, cardJSON)
		}
	}
}
