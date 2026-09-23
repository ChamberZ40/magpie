package feishu

import (
	"regexp"
	"strings"
)

// richCardSummaryMaxRunes caps config.summary.content. The chat list shows far
// less than this, but the field rides along on every full-card update, so a
// whole reply pasted into it is pure payload waste.
const richCardSummaryMaxRunes = 100

var (
	// Only the fence markers go; the code between them is often the entire
	// answer to a "run this" turn.
	richSummaryFenceRe = regexp.MustCompile("(?m)^[ \t]*```.*$")
	// Leading block markers: quotes, ATX headings, bullets, ordered items.
	// The bullet and ordered forms require trailing whitespace so "-5 degrees"
	// and "1.5x" survive intact.
	richSummaryLeadRe     = regexp.MustCompile(`^[ \t]*(?:[>#]+[ \t]*|[-*+][ \t]+|\d+[.)][ \t]+)+`)
	richSummaryImageRe    = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	richSummaryLinkRe     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	richSummaryEmphasisRe = regexp.MustCompile("(?:\\*\\*|__|\\*|_|`)")
	richSummarySpaceRe    = regexp.MustCompile(`\s+`)
)

// richCardSummaryText renders markdown as the single line Feishu shows in the
// chat list (config.summary.content).
//
// Without a summary Feishu derives the preview from the first text-bearing
// element, and this card leads with the collapsible panels — so a finished turn
// previewed as "🧠 推理 (2)" where the reply belonged. The stripping is
// deliberately shallow: this is a preview, not a renderer, and anything it
// leaves behind is a cosmetic wart in one chat-list row.
//
// Returns "" when there is nothing to preview; the caller decides what to do
// with that rather than writing an empty summary, which would blank the row.
func richCardSummaryText(markdown string) string {
	if strings.TrimSpace(markdown) == "" {
		return ""
	}

	text := richSummaryFenceRe.ReplaceAllString(markdown, "")
	// Images before links: the image form is a superset of the link form, and
	// the link pattern would otherwise leave a stray "!" behind.
	text = richSummaryImageRe.ReplaceAllString(text, "$1")
	text = richSummaryLinkRe.ReplaceAllString(text, "$1")

	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = richSummaryLeadRe.ReplaceAllString(line, "")
		line = richSummaryEmphasisRe.ReplaceAllString(line, "")
		if line = strings.TrimSpace(line); line != "" {
			cleaned = append(cleaned, line)
		}
	}

	joined := strings.TrimSpace(richSummarySpaceRe.ReplaceAllString(strings.Join(cleaned, " "), " "))
	if runes := []rune(joined); len(runes) > richCardSummaryMaxRunes {
		return strings.TrimSpace(string(runes[:richCardSummaryMaxRunes])) + "…"
	}
	return joined
}
