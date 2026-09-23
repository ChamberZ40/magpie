package claudecode

import (
	"context"
	"strings"
	"testing"

	"github.com/ChamberZ40/magpie/core"
)

// streamEventDelta builds the `stream_event` envelope Claude Code emits for one
// text token delta when --include-partial-messages is set. Shape taken from a
// live CLI run (v2.1.278).
func streamEventDelta(blockIdx int, text string) map[string]any {
	return map[string]any{
		"type": "stream_event",
		"event": map[string]any{
			"type":  "content_block_delta",
			"index": float64(blockIdx),
			"delta": map[string]any{
				"type": "text_delta",
				"text": text,
			},
		},
	}
}

func assistantTextMessage(blocks ...any) map[string]any {
	return map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": blocks,
		},
	}
}

func textBlock(text string) any {
	return map[string]any{"type": "text", "text": text}
}

// drainText collects the Content of every EventText currently buffered.
func drainText(t *testing.T, cs *claudeSession) []string {
	t.Helper()
	var got []string
	for len(cs.events) > 0 {
		evt := <-cs.events
		if evt.Type == core.EventText {
			got = append(got, evt.Content)
		}
	}
	return got
}

func newStreamTestSession(t *testing.T) (*claudeSession, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cs := &claudeSession{
		events: make(chan core.Event, 64),
		ctx:    ctx,
	}
	cs.alive.Store(true)
	return cs, cancel
}

// The whole point of forwarding deltas: each one reaches the engine on its own
// so the IM preview has something to render mid-generation.
func TestHandleStreamEventForwardsTextDeltas(t *testing.T) {
	cs, cancel := newStreamTestSession(t)
	defer cancel()

	for _, chunk := range []string{"流式", "输出", "已生效"} {
		cs.handleStreamEvent(streamEventDelta(0, chunk))
	}

	got := drainText(t, cs)
	want := []string{"流式", "输出", "已生效"}
	if len(got) != len(want) {
		t.Fatalf("got %d text events %q, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("delta %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// Regression guard for the duplication this feature could easily introduce:
// --include-partial-messages does not suppress the completed `assistant`
// event, so the block that was already streamed must not be emitted twice.
func TestHandleAssistantSuppressesFullyStreamedBlock(t *testing.T) {
	cs, cancel := newStreamTestSession(t)
	defer cancel()

	cs.handleStreamEvent(streamEventDelta(0, "流式"))
	cs.handleStreamEvent(streamEventDelta(0, "输出"))
	streamed := drainText(t, cs)

	cs.handleAssistant(assistantTextMessage(textBlock("流式输出")))

	if after := drainText(t, cs); len(after) != 0 {
		t.Errorf("assistant event re-emitted already-streamed text %q", after)
	}
	if joined := strings.Join(streamed, ""); joined != "流式输出" {
		t.Errorf("deltas reassemble to %q, want %q", joined, "流式输出")
	}
}

// If the finished block is longer than what the deltas delivered (a delta lost
// to a full channel, say), only the missing tail may be emitted.
func TestHandleAssistantEmitsOnlyUnstreamedTail(t *testing.T) {
	cs, cancel := newStreamTestSession(t)
	defer cancel()

	cs.handleStreamEvent(streamEventDelta(0, "流式"))
	drainText(t, cs)

	cs.handleAssistant(assistantTextMessage(textBlock("流式输出已生效")))

	got := drainText(t, cs)
	if len(got) != 1 || got[0] != "输出已生效" {
		t.Errorf("got %q, want [\"输出已生效\"]", got)
	}
}

// When deltas and the finished block disagree outright, the full block wins:
// duplicating text is recoverable, dropping the reply is not.
func TestHandleAssistantEmitsFullBlockOnPrefixMismatch(t *testing.T) {
	cs, cancel := newStreamTestSession(t)
	defer cancel()

	cs.handleStreamEvent(streamEventDelta(0, "完全不同的内容"))
	drainText(t, cs)

	cs.handleAssistant(assistantTextMessage(textBlock("流式输出")))

	got := drainText(t, cs)
	if len(got) != 1 || got[0] != "流式输出" {
		t.Errorf("got %q, want [\"流式输出\"]", got)
	}
}

// Block indexes restart at 0 for every message in a tool-using turn, so the
// previous message's bookkeeping must not suppress the next message's text.
func TestMessageStartResetsStreamedText(t *testing.T) {
	cs, cancel := newStreamTestSession(t)
	defer cancel()

	cs.handleStreamEvent(streamEventDelta(0, "第一条"))
	drainText(t, cs)

	cs.handleStreamEvent(map[string]any{
		"type":  "stream_event",
		"event": map[string]any{"type": "message_start"},
	})
	cs.handleAssistant(assistantTextMessage(textBlock("第一条")))

	got := drainText(t, cs)
	if len(got) != 1 || got[0] != "第一条" {
		t.Errorf("after message_start the block should be emitted in full, got %q", got)
	}
}

// Only text_delta drives the preview; thinking deltas keep arriving through
// the completed assistant event and must not leak in as reply text.
func TestHandleStreamEventIgnoresNonTextDeltas(t *testing.T) {
	cs, cancel := newStreamTestSession(t)
	defer cancel()

	cs.handleStreamEvent(map[string]any{
		"type": "stream_event",
		"event": map[string]any{
			"type":  "content_block_delta",
			"index": float64(0),
			"delta": map[string]any{"type": "thinking_delta", "thinking": "推理中"},
		},
	})
	cs.handleStreamEvent(map[string]any{
		"type": "stream_event",
		"event": map[string]any{
			"type":  "content_block_delta",
			"index": float64(1),
			"delta": map[string]any{"type": "input_json_delta", "partial_json": `{"a":`},
		},
	})

	if got := drainText(t, cs); len(got) != 0 {
		t.Errorf("non-text deltas emitted text events %q", got)
	}
}

// Multi-block messages must be tracked per index: suppressing block 0 may not
// suppress a second text block that was never streamed.
func TestStreamedTextIsTrackedPerBlockIndex(t *testing.T) {
	cs, cancel := newStreamTestSession(t)
	defer cancel()

	cs.handleStreamEvent(streamEventDelta(0, "已流式"))
	drainText(t, cs)

	cs.handleAssistant(assistantTextMessage(
		textBlock("已流式"),
		map[string]any{"type": "tool_use", "name": "Read", "input": map[string]any{}},
		textBlock("未流式"),
	))

	got := drainText(t, cs)
	if len(got) != 1 || got[0] != "未流式" {
		t.Errorf("got %q, want [\"未流式\"]", got)
	}
}

// Malformed envelopes must not panic the read loop.
func TestHandleStreamEventToleratesMalformedPayloads(t *testing.T) {
	cs, cancel := newStreamTestSession(t)
	defer cancel()

	payloads := []map[string]any{
		{"type": "stream_event"},
		{"type": "stream_event", "event": "not-an-object"},
		{"type": "stream_event", "event": map[string]any{"type": "content_block_delta"}},
		{"type": "stream_event", "event": map[string]any{
			"type":  "content_block_delta",
			"delta": map[string]any{"type": "text_delta"},
		}},
		{"type": "stream_event", "event": map[string]any{
			"type":  "content_block_delta",
			"delta": map[string]any{"type": "text_delta", "text": ""},
		}},
	}
	for _, p := range payloads {
		cs.handleStreamEvent(p)
	}

	if got := drainText(t, cs); len(got) != 0 {
		t.Errorf("malformed payloads produced text events %q", got)
	}
}

// A delta with no index field must default to block 0 rather than panicking or
// silently landing on a bogus key.
func TestHandleStreamEventDefaultsMissingIndexToZero(t *testing.T) {
	cs, cancel := newStreamTestSession(t)
	defer cancel()

	cs.handleStreamEvent(map[string]any{
		"type": "stream_event",
		"event": map[string]any{
			"type":  "content_block_delta",
			"delta": map[string]any{"type": "text_delta", "text": "无索引"},
		},
	})
	drainText(t, cs)

	cs.handleAssistant(assistantTextMessage(textBlock("无索引")))

	if got := drainText(t, cs); len(got) != 0 {
		t.Errorf("index-less delta did not register against block 0, got %q", got)
	}
}
