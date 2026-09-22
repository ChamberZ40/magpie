package appid

import "testing"

// The fallback exists so a launchd plist or hook script written before the
// rename keeps working. These pin the precedence that makes that safe.

func TestLookupEnv_PrefersTheCurrentName(t *testing.T) {
	t.Setenv("MAGPIE_LOG_FILE", "/new.log")
	t.Setenv("CC_LOG_FILE", "/old.log")

	got, ok := LookupEnv("LOG_FILE")
	if !ok || got != "/new.log" {
		t.Errorf("LookupEnv = %q, %v; want \"/new.log\", true", got, ok)
	}
}

func TestLookupEnv_FallsBackToLegacyNames(t *testing.T) {
	for _, tt := range []struct{ name, envName, suffix string }{
		{"CC_ prefix", "CC_LOG_FILE", "LOG_FILE"},
		{"CC_CONNECT_ prefix", "CC_CONNECT_PERMISSION_HOOK_SKIP", "PERMISSION_HOOK_SKIP"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envName, "legacy")
			if got, ok := LookupEnv(tt.suffix); !ok || got != "legacy" {
				t.Errorf("LookupEnv(%q) = %q, %v; want \"legacy\", true", tt.suffix, got, ok)
			}
		})
	}
}

// Clearing the current name must not resurrect a value from a plist the user
// has long forgotten about.
func TestLookupEnv_EmptyCurrentNameBeatsSetLegacyName(t *testing.T) {
	t.Setenv("MAGPIE_LOG_FILE", "")
	t.Setenv("CC_LOG_FILE", "/old.log")

	got, ok := LookupEnv("LOG_FILE")
	if !ok {
		t.Fatal("LookupEnv reported unset; an explicitly-empty variable is set")
	}
	if got != "" {
		t.Errorf("LookupEnv = %q, want the empty current value to win", got)
	}
}

func TestLookupEnv_UnsetEverywhere(t *testing.T) {
	if got, ok := LookupEnv("DEFINITELY_NOT_SET_ANYWHERE"); ok || got != "" {
		t.Errorf("LookupEnv = %q, %v; want \"\", false", got, ok)
	}
}

// Messages that tell a user what to set must name the current variable, even
// when the value in hand came from a legacy one.
func TestEnvName_UsesTheCurrentPrefix(t *testing.T) {
	if got, want := EnvName("LOG_FILE"), "MAGPIE_LOG_FILE"; got != want {
		t.Errorf("EnvName = %q, want %q", got, want)
	}
}
