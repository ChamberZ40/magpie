package core

import (
	"strings"
	"testing"
)

// The context indicator has two jobs that pull against each other: stay out of
// the way at normal usage, and be impossible to miss once the window is nearly
// full. These tests pin the boundary between those two states, since that is the
// only part a reader cannot verify by eye.

func TestRichFooterCtxBar_FillTracksPercent(t *testing.T) {
	tests := []struct {
		pct  int
		want string
	}{
		{0, "──────────"},
		{1, "──────────"},  // rounds down: 0.1 cells
		{5, "━─────────"},  // rounds up: 0.5 cells
		{29, "━━━───────"}, // 2.9 -> 3
		{50, "━━━━━─────"},
		{68, "━━━━━━━───"}, // 6.8 -> 7
		{91, "━━━━━━━━━─"}, // 9.1 -> 9, deliberately not full
		{95, "━━━━━━━━━━"}, // 9.5 -> 10
		{100, "━━━━━━━━━━"},
	}
	for _, tt := range tests {
		got := richFooterCtxBar(tt.pct)
		if got != tt.want {
			t.Errorf("richFooterCtxBar(%d) = %q, want %q", tt.pct, got, tt.want)
		}
		// Width must be stable so the footer does not reflow as usage climbs.
		if n := len([]rune(got)); n != richFooterCtxBarCells {
			t.Errorf("richFooterCtxBar(%d) width = %d runes, want %d", tt.pct, n, richFooterCtxBarCells)
		}
	}
}

func TestRichFooterCtxBar_ClampsOutOfRange(t *testing.T) {
	for _, pct := range []int{-50, -1, 101, 1000} {
		got := richFooterCtxBar(pct)
		if n := len([]rune(got)); n != richFooterCtxBarCells {
			t.Errorf("richFooterCtxBar(%d) width = %d runes, want %d", pct, n, richFooterCtxBarCells)
		}
	}
}

func TestRichFooterContext_EscalatesOnlyAtAlertThreshold(t *testing.T) {
	const window = 200_000
	tests := []struct {
		name      string
		used      int
		wantAlert bool
	}{
		{"well below", 58_700, false},
		{"mid", 136_000, false},
		{"climbing but not critical", 182_000, false},
		{"one below threshold", (richFooterCtxAlertPct - 1) * window / 100, false},
		{"exactly at threshold", richFooterCtxAlertPct * window / 100, true},
		{"nearly full", 192_000, true},
		{"full", window, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := richFooterContext(&ContextUsage{UsedTokens: tt.used, ContextWindow: window}, LangChinese)
			hasDot := strings.Contains(got, "🔴")
			hasLeft := strings.Contains(got, "剩 ")
			if hasDot != tt.wantAlert || hasLeft != tt.wantAlert {
				t.Errorf("richFooterContext(used=%d) = %q; alert dot=%v left=%v, want both %v",
					tt.used, got, hasDot, hasLeft, tt.wantAlert)
			}
			// The bar and a percentage are present in both states.
			if !strings.Contains(got, "上下文 ") || !strings.Contains(got, "%") {
				t.Errorf("richFooterContext(used=%d) = %q, want label, bar and percent", tt.used, got)
			}
		})
	}
}

// The baseline is fixed system overhead the user cannot reclaim, so it is
// subtracted from both the used total and the window. That cancels out of
// `remaining` — only the percentage moves, and with it the alert boundary.
func TestRichFooterContext_ExcludesBaselineFromPercent(t *testing.T) {
	// 190k of a 200k window is 95% raw, which would alert. Excluding the 20k
	// baseline it is 170k of 180k = 94%, which must not.
	got := richFooterContext(&ContextUsage{
		UsedTokens:     190_000,
		BaselineTokens: 20_000,
		ContextWindow:  200_000,
	}, LangChinese)
	if !strings.Contains(got, "94%") {
		t.Errorf("richFooterContext = %q, want 94%% (baseline-excluded)", got)
	}
	if strings.Contains(got, "🔴") {
		t.Errorf("richFooterContext = %q, want no alert below %d%%", got, richFooterCtxAlertPct)
	}

	// Past the threshold the remaining budget appears, also baseline-excluded:
	// 176k of 180k = 98%, 4k left.
	got = richFooterContext(&ContextUsage{
		UsedTokens:     196_000,
		BaselineTokens: 20_000,
		ContextWindow:  200_000,
	}, LangChinese)
	if !strings.Contains(got, "98%") {
		t.Errorf("richFooterContext = %q, want 98%% (baseline-excluded)", got)
	}
	if !strings.Contains(got, "剩 4.0k") {
		t.Errorf("richFooterContext = %q, want remaining 4.0k", got)
	}
}

func TestRichFooterContext_EmptyWithoutUsableUsage(t *testing.T) {
	tests := []struct {
		name  string
		usage *ContextUsage
	}{
		{"nil", nil},
		{"no window", &ContextUsage{UsedTokens: 100}},
		{"no usage data", &ContextUsage{ContextWindow: 200_000}},
		{"baseline swallows window", &ContextUsage{UsedTokens: 100, BaselineTokens: 200_000, ContextWindow: 200_000}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := richFooterContext(tt.usage, LangChinese); got != "" {
				t.Errorf("richFooterContext = %q, want empty", got)
			}
		})
	}
}

// Falling back to TotalTokens / Input+Output keeps the indicator alive for
// agents that do not report UsedTokens directly.
func TestContextBudget_UsedTokenFallbacks(t *testing.T) {
	tests := []struct {
		name     string
		usage    *ContextUsage
		wantUsed int
	}{
		{"prefers UsedTokens", &ContextUsage{UsedTokens: 50, TotalTokens: 99, ContextWindow: 100}, 50},
		{"falls back to TotalTokens", &ContextUsage{TotalTokens: 60, ContextWindow: 100}, 60},
		{"falls back to in+out", &ContextUsage{InputTokens: 30, OutputTokens: 40, ContextWindow: 100}, 70},
		{"clamps overflow to window", &ContextUsage{UsedTokens: 500, ContextWindow: 100}, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			used, remaining, window, ok := contextBudget(tt.usage)
			if !ok {
				t.Fatalf("contextBudget ok = false, want true")
			}
			if used != tt.wantUsed {
				t.Errorf("used = %d, want %d", used, tt.wantUsed)
			}
			if used+remaining != window {
				t.Errorf("used(%d) + remaining(%d) != window(%d)", used, remaining, window)
			}
		})
	}
}

func TestRichFooterContext_LocalizedAlertSuffix(t *testing.T) {
	usage := &ContextUsage{UsedTokens: 190_000, ContextWindow: 200_000}
	tests := []struct {
		lang Language
		want string
	}{
		{LangEnglish, "left"},
		{LangChinese, "剩"},
		{LangTraditionalChinese, "剩"},
		{LangJapanese, "残り"},
		{LangSpanish, "quedan"},
	}
	for _, tt := range tests {
		got := richFooterContext(usage, tt.lang)
		if !strings.Contains(got, tt.want) {
			t.Errorf("richFooterContext(%s) = %q, want it to contain %q", tt.lang, got, tt.want)
		}
		if !strings.Contains(got, "🔴") {
			t.Errorf("richFooterContext(%s) = %q, want alert dot", tt.lang, got)
		}
	}
}
