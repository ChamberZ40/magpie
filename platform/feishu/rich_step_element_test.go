package feishu

import (
	"strings"
	"testing"

	"github.com/ChamberZ40/magpie/core"
)

// One call used to be one card element holding one string: title, target and
// output concatenated with two spaces and a newline. Feishu renders that as a
// single text node, so everything in it shares one size and one color, and the
// target reads with exactly as much weight as the raw output under it.
//
// Splitting the row into its own elements is what buys the target and the
// output their own indent and their own grey — a text node has one color, so
// there is no way to grey only part of one.

const detailIndent = "0px 0px 0px 22px"

func elementText(t *testing.T, elem map[string]any) map[string]any {
	t.Helper()
	text, ok := elem["text"].(map[string]any)
	if !ok {
		t.Fatalf("element = %#v, want a text node", elem)
	}
	return text
}

func elementContent(t *testing.T, elem map[string]any) string {
	t.Helper()
	content, ok := elementText(t, elem)["content"].(string)
	if !ok {
		t.Fatalf("element = %#v, want string content", elem)
	}
	return content
}

func TestRichStepElements_SplitsTitleDetailAndOutput(t *testing.T) {
	exit := 0
	success := true
	step := core.ToolStep{
		Kind: core.ToolStepKindTool, Name: "Bash", Summary: `{"command":"npm test"}`,
		Status: "completed", ExitCode: &exit, Success: &success,
		Result: "all 42 tests passed", Done: true,
	}

	elements := richStepElements(step, "en")
	if len(elements) != 3 {
		t.Fatalf("elements = %#v (%d), want title, detail and output", elements, len(elements))
	}

	// The title says what ran; the target belongs on its own row below it.
	if title := elementContent(t, elements[0]); title != "Run tests" {
		t.Errorf("title = %q, want the action alone", title)
	}
	if elements[0]["icon"] == nil {
		t.Error("title element carries no icon — the row loses its glyph")
	}

	for i, want := range []string{"npm test", "all 42 tests passed"} {
		elem := elements[i+1]
		if got := elementContent(t, elem); got != want {
			t.Errorf("elements[%d] content = %q, want %q", i+1, got, want)
		}
		// Indent and grey are the whole point of the split: they put the
		// detail visually under the title instead of beside it.
		if got := elem["margin"]; got != detailIndent {
			t.Errorf("elements[%d] margin = %v, want %q", i+1, got, detailIndent)
		}
		if got := elementText(t, elem)["text_color"]; got != "grey" {
			t.Errorf("elements[%d] text_color = %v, want grey", i+1, got)
		}
		// Only the title row gets the glyph; repeating it would read as a
		// second call.
		if elem["icon"] != nil {
			t.Errorf("elements[%d] carries an icon, want it on the title only", i+1)
		}
	}
}

// A call whose target adds nothing the title did not already say stays one
// element, so an indented row never appears empty or duplicated.
func TestRichStepElements_StaysOneElementWithNothingToIndent(t *testing.T) {
	step := core.ToolStep{Kind: core.ToolStepKindTool, Name: "Bash", Done: true}

	elements := richStepElements(step, "en")
	if len(elements) != 1 {
		t.Fatalf("elements = %#v (%d), want the title alone", elements, len(elements))
	}
	if elements[0]["margin"] != nil {
		t.Errorf("title element margin = %v, want none", elements[0]["margin"])
	}
}

// The verdict is about the call, not about its output, so it rides on the title
// row where the eye is already looking.
func TestRichStepElements_FailureMarkRidesOnTheTitleRow(t *testing.T) {
	exit := 2
	failed := false
	step := core.ToolStep{
		Kind: core.ToolStepKindTool, Name: "Bash", Summary: `{"command":"./deploy.sh"}`,
		Status: "failed", ExitCode: &exit, Success: &failed,
		Result: "ssh: connect to host db01 port 22: Connection refused", Done: true,
	}

	elements := richStepElements(step, "en")
	title := elementContent(t, elements[0])
	for _, want := range []string{"✗", "Failed", "exit 2"} {
		if !strings.Contains(title, want) {
			t.Errorf("title = %q, want it to contain %q", title, want)
		}
	}
	if got := elementContent(t, elements[len(elements)-1]); !strings.Contains(got, "Connection refused") {
		t.Errorf("last element = %q, want the output that explains the failure", got)
	}
}

// Thinking rows have no target and no output, and were already grey in full.
func TestRichStepElements_ThinkingStaysOneGreyElement(t *testing.T) {
	step := core.ToolStep{Kind: core.ToolStepKindThinking, Summary: "weighing options"}

	elements := richStepElements(step, "en")
	if len(elements) != 1 {
		t.Fatalf("elements = %#v (%d), want one", elements, len(elements))
	}
	if got := elementContent(t, elements[0]); got != "weighing options" {
		t.Errorf("content = %q, want the thinking text unchanged", got)
	}
	if got := elementText(t, elements[0])["text_color"]; got != "grey" {
		t.Errorf("text_color = %v, want grey", got)
	}
}

// The panel's ten-row cap counts calls, not elements: a split row must not cost
// two of a reader's ten.
func TestRichPanelElements_CapsCallsNotElements(t *testing.T) {
	exit := 0
	success := true
	steps := make([]core.ToolStep, 0, 12)
	for i := 0; i < 12; i++ {
		steps = append(steps, core.ToolStep{
			Kind: core.ToolStepKindTool, Name: "Bash", Summary: `{"command":"npm test"}`,
			Status: "completed", ExitCode: &exit, Success: &success, Done: true,
		})
	}

	elements := richPanelElements(steps, "en")
	// Ten calls at two elements each, plus the "N earlier steps" placeholder.
	if len(elements) != 21 {
		t.Fatalf("elements = %d, want 10 calls split in two plus the placeholder", len(elements))
	}
	if got := elementContent(t, elements[0]); !strings.Contains(got, "2") {
		t.Errorf("first element = %q, want the count of hidden steps", got)
	}
}

// richStepBody is still the flat form the card-too-big fallback prints, so it
// has to keep saying everything the elements say. Deriving both from one
// decomposition is what stops them from drifting apart.
func TestRichStepBodyMatchesTheElementSplit(t *testing.T) {
	exit := 2
	failed := false
	step := core.ToolStep{
		Kind: core.ToolStepKindTool, Name: "Bash", Summary: `{"command":"./deploy.sh"}`,
		Status: "failed", ExitCode: &exit, Success: &failed,
		Result: "Connection refused", Done: true,
	}

	parts := splitRichStep(step, "en")
	body := richStepBody(step, "en")
	elements := richStepElements(step, "en")

	for _, part := range []string{parts.Title, parts.Detail, parts.Mark, parts.Output} {
		if part == "" {
			t.Fatalf("parts = %#v, want this case to exercise all four", parts)
		}
		if !strings.Contains(body, part) {
			t.Errorf("body = %q, want it to contain %q", body, part)
		}
		found := false
		for _, elem := range elements {
			if strings.Contains(elementContent(t, elem), part) {
				found = true
			}
		}
		if !found {
			t.Errorf("no element carries %q", part)
		}
	}
}
