package appid

import "testing"

func TestLookupEnv_ReadsTheCurrentName(t *testing.T) {
	t.Setenv("MAGPIE_LOG_FILE", "/new.log")

	got, ok := LookupEnv("LOG_FILE")
	if !ok || got != "/new.log" {
		t.Errorf("LookupEnv = %q, %v; want \"/new.log\", true", got, ok)
	}
}

// The pre-rename prefixes used to be consulted as a fallback. They are not
// anymore: this project had no users on the old name, so the fallback only
// bought a way for a stale CC_ variable — left behind by some other tool that
// happens to use the same prefix — to steer this one.
func TestLookupEnv_IgnoresPreRenameNames(t *testing.T) {
	for _, envName := range []string{"CC_LOG_FILE", "CC_CONNECT_LOG_FILE"} {
		t.Run(envName, func(t *testing.T) {
			t.Setenv(envName, "legacy")
			if got, ok := LookupEnv("LOG_FILE"); ok || got != "" {
				t.Errorf("LookupEnv = %q, %v; want \"\", false", got, ok)
			}
		})
	}
}

func TestLookupEnv_EmptyValueIsStillSet(t *testing.T) {
	t.Setenv("MAGPIE_LOG_FILE", "")

	got, ok := LookupEnv("LOG_FILE")
	if !ok {
		t.Fatal("LookupEnv reported unset; an explicitly-empty variable is set")
	}
	if got != "" {
		t.Errorf("LookupEnv = %q, want the empty value", got)
	}
}

func TestLookupEnv_UnsetEverywhere(t *testing.T) {
	if got, ok := LookupEnv("DEFINITELY_NOT_SET_ANYWHERE"); ok || got != "" {
		t.Errorf("LookupEnv = %q, %v; want \"\", false", got, ok)
	}
}

func TestEnvName_UsesTheCurrentPrefix(t *testing.T) {
	if got, want := EnvName("LOG_FILE"), "MAGPIE_LOG_FILE"; got != want {
		t.Errorf("EnvName = %q, want %q", got, want)
	}
}

// One name out, not two: a child process gets MAGPIE_ only.
func TestEnvPair_EmitsOnlyTheCurrentName(t *testing.T) {
	got := EnvPair("PROJECT", "demo")

	if len(got) != 1 || got[0] != "MAGPIE_PROJECT=demo" {
		t.Errorf("EnvPair = %v, want exactly [MAGPIE_PROJECT=demo]", got)
	}
}
