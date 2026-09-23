package main

import (
	"fmt"
	"log/slog"

	"github.com/ChamberZ40/magpie/config"
)

// clientPrintPacingOption is the per-platform option that tells the client how
// fast to reveal characters it has already received. It is a render ceiling,
// not a source of frames: if we push a frame every 200ms, asking the client to
// print every 50ms changes nothing on screen.
const clientPrintPacingOption = "card_print_frequency_ms"

// clientPrintPacingWarning reports the contradiction between our push interval
// and a platform's client-side print pacing, or "" when there is none.
//
// The key is looked up across every platform rather than against a known
// platform type: core and the assembly layer stay free of platform names, and
// any platform adopting the same option gets the check for free.
func clientPrintPacingWarning(proj *config.ProjectConfig, streaming config.StreamingDisplay) string {
	if proj == nil {
		return ""
	}
	for _, p := range proj.Platforms {
		printMS, ok := optionAsInt(p.Options[clientPrintPacingOption])
		if !ok || printMS <= 0 || streaming.ThrottleMS <= printMS {
			continue
		}
		return fmt.Sprintf(
			"project %q platform %q: %s=%d has no effect because the server only pushes every %dms; "+
				"lower display.streaming.throttle_ms to %d or below, or raise %s",
			proj.Name, p.Type, clientPrintPacingOption, printMS,
			streaming.ThrottleMS, printMS, clientPrintPacingOption)
	}
	return ""
}

// optionAsInt reads a platform option that should hold a whole number. TOML
// decodes integers as int64, but options survive other round-trips too, so the
// common numeric shapes are accepted. A malformed value is not reported here —
// the platform owning the option validates it and gives the better message.
func optionAsInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func warnIfClientPrintsFasterThanWePush(proj *config.ProjectConfig, streaming config.StreamingDisplay) {
	if msg := clientPrintPacingWarning(proj, streaming); msg != "" {
		slog.Warn(msg)
	}
}
