package feishu

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ChamberZ40/magpie/core"
)

// Feishu derives the chat-list preview from the first text-bearing element when
// a card carries no config.summary. This card puts the collapsible panels
// before the markdown body, so every finished turn previewed as "🧠 推理 (2)"
// instead of the answer the user actually got.
func TestBuildRichCard_SummaryIsTheReplyNotThePanelTitle(t *testing.T) {
	steps := []core.ToolStep{
		{Kind: core.ToolStepKindThinking, Summary: "weighing options"},
		{Kind: core.ToolStepKindTool, Name: "Bash", Summary: `{"command":"ls"}`},
	}
	card := buildRichCard(core.CardStatusDone, "zh", steps,
		"## 结论\n\n**已经**修好了，见 [PR](https://example.com)。", cardStreaming{}, "")

	if got, want := cardSummary(t, card), "结论 已经修好了，见 PR。"; got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

// The card is created before the agent has produced any text, and the creation
// payload is the only one the element-streaming path ever sends config on. The
// header is the honest stand-in until the first full-card update carries the
// real reply.
func TestBuildRichCard_SummaryFallsBackToHeaderWhileTheReplyIsEmpty(t *testing.T) {
	steps := []core.ToolStep{{Kind: core.ToolStepKindThinking, Summary: "weighing options"}}
	card := buildRichCard(core.CardStatusWorking, "zh", steps, "", cardStreaming{enabled: true}, "")

	if got, want := cardSummary(t, card), "● 思考中…"; got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

// A done card has no header title, and a turn whose whole output lived in the
// panels has no body either. Writing an empty summary would blank the chat-list
// row, so the field is omitted and Feishu's own fallback stays in charge.
func TestBuildRichCard_NoSummaryWhenThereIsNothingToPreview(t *testing.T) {
	steps := []core.ToolStep{{Kind: core.ToolStepKindTool, Name: "Bash", Summary: `{"command":"ls"}`}}
	card := buildRichCard(core.CardStatusDone, "zh", steps, "", cardStreaming{}, "")

	var decoded map[string]any
	if err := json.Unmarshal([]byte(card), &decoded); err != nil {
		t.Fatalf("card JSON is invalid: %v\n%s", err, card)
	}
	config, ok := decoded["config"].(map[string]any)
	if !ok {
		t.Fatalf("card JSON missing config: %#v", decoded)
	}
	if _, exists := config["summary"]; exists {
		t.Fatalf("empty turn set config.summary, which blanks the chat-list row: %#v", config)
	}
}

func TestRichCardSummaryTextStripsMarkdown(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"heading", "# Title", "Title"},
		{"bold and italic", "**bold** and _italic_", "bold and italic"},
		{"inline code", "run `npm test` now", "run npm test now"},
		{"link", "see [the docs](https://example.com)", "see the docs"},
		{"image", "![a cat](https://example.com/c.png)", "a cat"},
		{"bullet list", "- one\n- two", "one two"},
		{"ordered list", "1. one\n2. two", "one two"},
		{"quote", "> quoted", "quoted"},
		{"fence keeps the code", "```sh\nls -l\n```", "ls -l"},
		{"blank lines collapse", "a\n\n\nb", "a b"},
		{"whitespace only is empty", "   \n\n  ", ""},
		{"negative number is not a bullet", "-5 degrees", "-5 degrees"},
	} {
		if got := richCardSummaryText(tc.in); got != tc.want {
			t.Errorf("%s: richCardSummaryText(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// Truncating by bytes would cut a CJK reply mid-rune and send Feishu invalid
// UTF-8, which is the common case here rather than the exotic one.
func TestRichCardSummaryTextTruncatesByRunes(t *testing.T) {
	got := richCardSummaryText(strings.Repeat("中", richCardSummaryMaxRunes+50))

	if n := len([]rune(got)); n != richCardSummaryMaxRunes+1 {
		t.Fatalf("summary is %d runes, want %d plus the ellipsis", n, richCardSummaryMaxRunes)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("truncated summary %q does not end in an ellipsis", got)
	}
}

func cardSummary(t *testing.T, cardJSON string) string {
	t.Helper()
	var card map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		t.Fatalf("card JSON is invalid: %v\n%s", err, cardJSON)
	}
	config, ok := card["config"].(map[string]any)
	if !ok {
		t.Fatalf("card JSON missing config: %#v", card)
	}
	summary, ok := config["summary"].(map[string]any)
	if !ok {
		t.Fatalf("card config has no summary, so Feishu previews the panel title instead: %#v", config)
	}
	content, _ := summary["content"].(string)
	return content
}
