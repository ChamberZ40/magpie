package feishu

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ChamberZ40/magpie/core"
)

// TestBuildRichCard_HeaderIsStableAcrossBuilds guards the root cause of the old
// header flicker: pickThinkingVerb() indexed a 61-verb list by
// time.Now().Unix(), and the engine rebuilds this card on every streaming patch,
// so one turn's header drifted "Pondering… → Simmering… → Whirring…" about once
// a second. The header must now depend only on (status, lang).
func TestBuildRichCard_HeaderIsStableAcrossBuilds(t *testing.T) {
	steps := []core.ToolStep{
		{Kind: core.ToolStepKindThinking, Summary: "weighing options"},
		{Kind: core.ToolStepKindTool, Name: "Bash", Summary: `{"command":"ls"}`},
	}

	first := buildRichCard(core.CardStatusWorking, "", steps, "partial", true, "")
	second := buildRichCard(core.CardStatusWorking, "", steps, "partial", true, "")
	if first != second {
		t.Fatalf("rich card is not deterministic across builds:\nfirst:  %s\nsecond: %s", first, second)
	}
	if got := cardHeaderTitle(t, first); got != "● Thinking…" {
		t.Fatalf("header title = %q, want ● Thinking…", got)
	}
}

func TestBuildRichCard_HeaderLocalization(t *testing.T) {
	// Thinking and Working intentionally share one word and one template: both
	// mean "the turn is still running" to a reader. Done shows no title at all —
	// the green header bar plus the finished answer below it are the signal.
	for _, tc := range []struct {
		lang     string
		status   core.CardStatus
		title    string
		template string
	}{
		{"", core.CardStatusWorking, "● Thinking…", "blue"},
		{"en", core.CardStatusThinking, "● Thinking…", "blue"},
		{"en", core.CardStatusWorking, "● Thinking…", "blue"},
		{"en", core.CardStatusDone, "", "green"},
		{"en", core.CardStatusError, "✖ Error", "red"},
		{"zh", core.CardStatusWorking, "● 思考中…", "blue"},
		{"zh", core.CardStatusDone, "", "green"},
		{"zh", core.CardStatusError, "✖ 出错", "red"},
		{"ja", core.CardStatusWorking, "● 考え中…", "blue"},
		{"ja", core.CardStatusDone, "", "green"},
		// zh-TW has its own entries; es exercises a non-CJK translation.
		{"zh-TW", core.CardStatusError, "✖ 出錯", "red"},
		{"es", core.CardStatusWorking, "● Pensando…", "blue"},
		// Unknown tag falls back to English rather than rendering a raw key.
		{"de", core.CardStatusError, "✖ Error", "red"},
	} {
		cardJSON := buildRichCard(tc.status, tc.lang, nil, "body", false, "")
		if got := cardHeaderTitle(t, cardJSON); got != tc.title {
			t.Errorf("lang=%q status=%s title = %q, want %q", tc.lang, tc.status, got, tc.title)
		}
		if got := cardHeaderTemplate(t, cardJSON); got != tc.template {
			t.Errorf("lang=%q status=%s template = %q, want %q", tc.lang, tc.status, got, tc.template)
		}
	}
}

func TestBuildRichCard_PanelTitleLocalization(t *testing.T) {
	steps := []core.ToolStep{
		{Kind: core.ToolStepKindThinking, Summary: "thinking out loud"},
		{Kind: core.ToolStepKindTool, Name: "Bash", Summary: `{"command":"ls"}`},
	}
	for _, tc := range []struct {
		lang      string
		reasoning string
		tools     string
	}{
		{"", "🧠 Reasoning (1)", "🔧 Tools (1)"},
		{"zh", "🧠 推理 (1)", "🔧 工具 (1)"},
		{"ja", "🧠 推論 (1)", "🔧 ツール (1)"},
		{"es", "🧠 Razonamiento (1)", "🔧 Herramientas (1)"},
	} {
		panels := collectCardPanels(t, buildRichCard(core.CardStatusDone, tc.lang, steps, "body", false, ""))
		if len(panels) != 2 {
			t.Fatalf("lang=%q panel count = %d, want 2: %#v", tc.lang, len(panels), panels)
		}
		if got := cardPanelTitle(panels[0]); got != tc.reasoning {
			t.Errorf("lang=%q reasoning panel = %q, want %q", tc.lang, got, tc.reasoning)
		}
		if got := cardPanelTitle(panels[1]); got != tc.tools {
			t.Errorf("lang=%q tools panel = %q, want %q", tc.lang, got, tc.tools)
		}
		// The panel header's icon slot is the expand chevron. Filling it would
		// trade the open/close affordance for a decorative glyph, so the
		// semantic icon lives in the title text and this must stay unset.
		for i, panel := range panels {
			header, _ := panel["header"].(map[string]any)
			if _, exists := header["icon"]; exists {
				t.Errorf("lang=%q panel[%d] sets header.icon, which overrides the expand chevron: %#v", tc.lang, i, header)
			}
		}
	}
}

// TestBuildRichCard_FooterIsOneNotationBlock covers the rich-card footer, which
// had no coverage at all before (every existing test passed statusFooter="").
func TestBuildRichCard_FooterIsOneNotationBlock(t *testing.T) {
	footer := "⏱ 2.4s\nclaude-opus-5 · ctx 4%\n~/code/magpie"
	elements := cardBodyElements(t, buildRichCard(core.CardStatusDone, "en", nil, "answer", false, footer))

	if len(elements) < 2 {
		t.Fatalf("want at least body + hr + footer elements, got %#v", elements)
	}
	hr := elements[len(elements)-2]
	if hr["tag"] != "hr" {
		t.Errorf("element before footer = %#v, want an hr separator", hr)
	}
	last := elements[len(elements)-1]
	if last["tag"] != "markdown" {
		t.Fatalf("footer element tag = %v, want markdown", last["tag"])
	}
	if last["text_size"] != "notation" {
		t.Errorf("footer text_size = %v, want notation", last["text_size"])
	}
	content, _ := last["content"].(string)
	for _, want := range []string{"⏱ 2.4s", "claude-opus-5 · ctx 4%", "~/code/magpie"} {
		if !strings.Contains(content, want) {
			t.Errorf("footer content %q missing %q", content, want)
		}
	}
	// One block, not three: count how many notation markdown elements exist.
	notation := 0
	for _, elem := range elements {
		if elem["tag"] == "markdown" && elem["text_size"] == "notation" {
			notation++
		}
	}
	if notation != 1 {
		t.Errorf("notation markdown block count = %d, want exactly 1", notation)
	}

	// Empty footer renders neither the separator nor a footer block.
	for _, elem := range cardBodyElements(t, buildRichCard(core.CardStatusDone, "en", nil, "answer", false, "")) {
		if elem["tag"] == "hr" {
			t.Errorf("empty footer should not emit an hr: %#v", elem)
		}
		if elem["text_size"] == "notation" {
			t.Errorf("empty footer should not emit a notation block: %#v", elem)
		}
	}
}

func TestProgressCardPanelTitlesAreLocalized(t *testing.T) {
	items := []core.ProgressCardEntry{
		{Kind: core.ProgressEntryThinking, Text: "considering"},
		{Kind: core.ProgressEntryToolUse, Tool: "Bash", Text: "ls"},
		{Kind: core.ProgressEntryInfo, Text: "note"},
	}
	// ja and es used to fall back to English because the old progressPanelTitle
	// only special-cased zh.
	for _, tc := range []struct{ lang, reasoning, tools, updates string }{
		{"en", "🧠 Reasoning (1)", "🔧 Tools (1)", "📋 Updates (1)"},
		{"zh", "🧠 推理 (1)", "🔧 工具 (1)", "📋 更新 (1)"},
		{"ja", "🧠 推論 (1)", "🔧 ツール (1)", "📋 更新 (1)"},
		{"es", "🧠 Razonamiento (1)", "🔧 Herramientas (1)", "📋 Actualizaciones (1)"},
	} {
		elements := appendProgressGroupedElements(nil, items, tc.lang, true)
		var titles []string
		for _, elem := range elements {
			if elem["tag"] == "collapsible_panel" {
				titles = append(titles, cardPanelTitle(elem))
			}
		}
		want := []string{tc.reasoning, tc.tools, tc.updates}
		if len(titles) != len(want) {
			t.Fatalf("lang=%q panel titles = %v, want %v", tc.lang, titles, want)
		}
		for i := range want {
			if titles[i] != want[i] {
				t.Errorf("lang=%q panel[%d] = %q, want %q", tc.lang, i, titles[i], want[i])
			}
		}
	}
}

// TestBuildCardJSONWithStatus_KeepsLocalizedHeader is a regression test for a
// real reported symptom: "思考中" never appeared during a live turn. buildRichCard
// set the header correctly, but SetPreviewStatus patches the whole card through
// buildCardJSONWithStatus on every status change mid-turn — and that builder
// hardcoded an empty header title, blanking it again immediately.
func TestBuildCardJSONWithStatus_KeepsLocalizedHeader(t *testing.T) {
	for _, tc := range []struct {
		lang     string
		status   core.CardStatus
		title    string
		template string
	}{
		{"zh", core.CardStatusWorking, "● 思考中…", "blue"},
		{"zh", core.CardStatusThinking, "● 思考中…", "blue"},
		{"en", core.CardStatusWorking, "● Thinking…", "blue"},
		{"zh", core.CardStatusDone, "", "green"},
		{"zh", core.CardStatusError, "✖ 出错", "red"},
		// No status at all keeps the old neutral header.
		{"zh", core.CardStatus(""), "", "grey"},
	} {
		cardJSON := buildCardJSONWithStatus("body", tc.status, tc.lang)
		if got := cardHeaderTitle(t, cardJSON); got != tc.title {
			t.Errorf("lang=%q status=%q title = %q, want %q", tc.lang, tc.status, got, tc.title)
		}
		if got := cardHeaderTemplate(t, cardJSON); got != tc.template {
			t.Errorf("lang=%q status=%q template = %q, want %q", tc.lang, tc.status, got, tc.template)
		}
	}
}

// The status card and the full rich card must agree on the header, otherwise a
// mid-turn preview patch visibly changes the header text.
func TestStatusCardHeaderMatchesRichCard(t *testing.T) {
	for _, status := range []core.CardStatus{
		core.CardStatusWorking, core.CardStatusThinking, core.CardStatusDone, core.CardStatusError,
	} {
		rich := cardHeaderTitle(t, buildRichCard(status, "zh", nil, "body", false, ""))
		simple := cardHeaderTitle(t, buildCardJSONWithStatus("body", status, "zh"))
		if rich != simple {
			t.Errorf("status=%q header differs: rich=%q status-card=%q", status, rich, simple)
		}
	}
}

// --- helpers ---

func cardBodyElements(t *testing.T, cardJSON string) []map[string]any {
	t.Helper()
	var card map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		t.Fatalf("card JSON is invalid: %v\n%s", err, cardJSON)
	}
	body, ok := card["body"].(map[string]any)
	if !ok {
		t.Fatalf("card JSON missing body: %#v", card)
	}
	raw, ok := body["elements"].([]any)
	if !ok {
		t.Fatalf("card body elements have unexpected type: %#v", body["elements"])
	}
	elements := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if elem, ok := item.(map[string]any); ok {
			elements = append(elements, elem)
		}
	}
	return elements
}

func cardHeader(t *testing.T, cardJSON string) map[string]any {
	t.Helper()
	var card map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		t.Fatalf("card JSON is invalid: %v\n%s", err, cardJSON)
	}
	header, ok := card["header"].(map[string]any)
	if !ok {
		t.Fatalf("card JSON missing header: %#v", card)
	}
	return header
}

func cardHeaderTitle(t *testing.T, cardJSON string) string {
	t.Helper()
	title, _ := cardHeader(t, cardJSON)["title"].(map[string]any)
	content, _ := title["content"].(string)
	return content
}

func cardHeaderTemplate(t *testing.T, cardJSON string) string {
	t.Helper()
	template, _ := cardHeader(t, cardJSON)["template"].(string)
	return template
}
