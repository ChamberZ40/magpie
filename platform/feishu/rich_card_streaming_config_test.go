package feishu

import (
	"encoding/json"
	"testing"

	lark "github.com/larksuite/oapi-sdk-go/v3"

	"github.com/ChamberZ40/magpie/core"
)

// Feishu's streaming_mode only says "this card will be updated in place". How
// the client reveals the text it already holds is a separate knob, and without
// it the typing rhythm is whatever our push interval happens to be — visibly
// steppy. streaming_config carries that pacing.

func cardConfig(t *testing.T, cardJSON string) map[string]any {
	t.Helper()
	var card struct {
		Config map[string]any `json:"config"`
	}
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		t.Fatalf("unmarshal card: %v", err)
	}
	return card.Config
}

func TestRichCardStreamingConfig(t *testing.T) {
	t.Run("pacing is sent while streaming", func(t *testing.T) {
		cfg := cardConfig(t, buildRichCard(core.CardStatusWorking, "", nil, "body",
			cardStreaming{enabled: true, printFreqMs: 50, printStep: 2}, ""))

		if cfg["streaming_mode"] != true {
			t.Errorf("streaming_mode = %v, want true", cfg["streaming_mode"])
		}
		sc, ok := cfg["streaming_config"].(map[string]any)
		if !ok {
			t.Fatalf("streaming_config = %#v, want a map", cfg["streaming_config"])
		}
		// Feishu wants per-client overrides under a "default" key, not a bare number.
		for _, tc := range []struct {
			key  string
			want float64
		}{
			{"print_frequency_ms", 50},
			{"print_step", 2},
		} {
			field, ok := sc[tc.key].(map[string]any)
			if !ok {
				t.Errorf("%s = %#v, want a map with a default", tc.key, sc[tc.key])
				continue
			}
			if field["default"] != tc.want {
				t.Errorf("%s default = %v, want %v", tc.key, field["default"], tc.want)
			}
		}
	})

	t.Run("a finished card carries no pacing", func(t *testing.T) {
		cfg := cardConfig(t, buildRichCard(core.CardStatusDone, "", nil, "body", cardStreaming{}, ""))

		if cfg["streaming_mode"] != false {
			t.Errorf("streaming_mode = %v, want false", cfg["streaming_mode"])
		}
		if _, present := cfg["streaming_config"]; present {
			t.Error("streaming_config should be absent when the card is not streaming")
		}
	})

	// An unset knob must not become a literal zero: print_frequency_ms=0 would
	// tell the client to dump the whole buffer at once, which is exactly the
	// look we are trying to fix.
	t.Run("unset pacing is omitted rather than sent as zero", func(t *testing.T) {
		cfg := cardConfig(t, buildRichCard(core.CardStatusWorking, "", nil, "body",
			cardStreaming{enabled: true}, ""))

		if _, present := cfg["streaming_config"]; present {
			t.Errorf("streaming_config = %#v, want it omitted", cfg["streaming_config"])
		}
	})
}

// The parsed options and the renderer are each covered above; this is the seam
// between them — the interface method core actually calls.
func TestBuildRichCardCarriesTheConfiguredPacing(t *testing.T) {
	p := &Platform{cardPrintFreqMs: 30, cardPrintStep: 4}

	sc, ok := cardConfig(t, p.BuildRichCard(core.CardStatusWorking, "", nil, "body", true, ""))["streaming_config"].(map[string]any)
	if !ok {
		t.Fatal("streaming_config missing — the platform's pacing never reached the card")
	}
	for _, tc := range []struct {
		key  string
		want float64
	}{
		{"print_frequency_ms", 30},
		{"print_step", 4},
	} {
		field, _ := sc[tc.key].(map[string]any)
		if field["default"] != tc.want {
			t.Errorf("%s default = %v, want %v", tc.key, field["default"], tc.want)
		}
	}
}

func TestNewPlatformCardPrintPacing(t *testing.T) {
	newFeishu := func(t *testing.T, extra map[string]any) (*Platform, error) {
		t.Helper()
		opts := map[string]any{"app_id": "cli_test", "app_secret": "secret"}
		for k, v := range extra {
			opts[k] = v
		}
		p, err := newPlatform("feishu", lark.FeishuBaseUrl, opts)
		if err != nil {
			return nil, err
		}
		return extractBasePlatform(p), nil
	}

	t.Run("defaults", func(t *testing.T) {
		fp, err := newFeishu(t, nil)
		if err != nil {
			t.Fatalf("newPlatform: %v", err)
		}
		if fp.cardPrintFreqMs != defaultCardPrintFreqMs || fp.cardPrintStep != defaultCardPrintStep {
			t.Errorf("got (%d, %d), want the defaults (%d, %d)",
				fp.cardPrintFreqMs, fp.cardPrintStep, defaultCardPrintFreqMs, defaultCardPrintStep)
		}
	})

	// int64 mirrors how TOML decodes integers.
	t.Run("custom values", func(t *testing.T) {
		fp, err := newFeishu(t, map[string]any{
			"card_print_frequency_ms": int64(30),
			"card_print_step":         int64(3),
		})
		if err != nil {
			t.Fatalf("newPlatform: %v", err)
		}
		if fp.cardPrintFreqMs != 30 || fp.cardPrintStep != 3 {
			t.Errorf("got (%d, %d), want (30, 3)", fp.cardPrintFreqMs, fp.cardPrintStep)
		}
	})

	for _, tc := range []struct {
		name string
		opts map[string]any
	}{
		{"negative frequency", map[string]any{"card_print_frequency_ms": -1}},
		{"non-numeric frequency", map[string]any{"card_print_frequency_ms": "fast"}},
		{"zero step would print nothing", map[string]any{"card_print_step": 0}},
		{"negative step", map[string]any{"card_print_step": -2}},
		{"non-numeric step", map[string]any{"card_print_step": "one"}},
	} {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			if _, err := newFeishu(t, tc.opts); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}
