package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"

	"github.com/ChamberZ40/magpie/config"
	"github.com/ChamberZ40/magpie/core"
)

// The streaming defaults exist twice: config owns the user-facing ones, and
// core carries its own copy because CLAUDE.md keeps core on stdlib alone, so
// it cannot import config to read them. Two copies drift. cmd/magpie imports
// both packages and is the only place that can notice.
//
// If this fails, the binary's real behaviour is core's number and the
// documented behaviour is config's.
func TestStreamingDefaultsAgreeAcrossPackages(t *testing.T) {
	for _, tc := range []struct {
		name     string
		inConfig int
		inCore   time.Duration
	}{
		{"throttle", config.DefaultStreamThrottleMS, core.DefaultStreamThrottle},
		{"fallback throttle", config.DefaultStreamFallbackThrottleMS, core.DefaultStreamFallbackThrottle},
	} {
		if want := time.Duration(tc.inConfig) * time.Millisecond; tc.inCore != want {
			t.Errorf("%s: core has %v, config has %v", tc.name, tc.inCore, want)
		}
	}

	if config.DefaultStreamThrottleChars != core.DefaultStreamThrottleChars {
		t.Errorf("throttle chars: core has %d, config has %d",
			core.DefaultStreamThrottleChars, config.DefaultStreamThrottleChars)
	}
	if config.DefaultStreamFallbackThrottleChars != core.DefaultStreamFallbackChars {
		t.Errorf("fallback chars: core has %d, config has %d",
			core.DefaultStreamFallbackChars, config.DefaultStreamFallbackThrottleChars)
	}
}

// DisplayCfg is assembled in two places — once at startup and once on reload —
// and an omitted Streaming field is not a compile error, it silently resolves
// back to the built-in defaults. reloadConfig forgot it, so editing
// [display.streaming] and reloading put the old pacing back with no warning.
//
// Checking the source rather than the behaviour is deliberate: reloadConfig
// needs a live engine with platforms attached, while the mistake it guards
// against is visible in the literal itself, and the guard keeps covering a
// third assembly site added later.
func TestEveryDisplayCfgLiteralWiresStreaming(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing main.go: %v", err)
	}

	found := 0
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "DisplayCfg" {
			return true
		}
		found++
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Streaming" {
				return true
			}
		}
		t.Errorf("%s: core.DisplayCfg literal does not set Streaming, so the configured pacing is dropped",
			fset.Position(lit.Pos()))
		return true
	})

	if found == 0 {
		t.Fatal("no core.DisplayCfg literals found in main.go; the wiring moved and this guard is now blind")
	}
}
