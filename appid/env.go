package appid

import "os"

// EnvPrefix is the prefix on every environment variable this project reads.
const EnvPrefix = "MAGPIE_"

// LegacyEnvPrefix is the pre-rename prefix. Writes still emit it alongside
// EnvPrefix; see EnvPair.
const LegacyEnvPrefix = "CC_"

// legacyEnvPrefixes are the pre-rename prefixes, tried in order when the
// MAGPIE_ name is unset. Two of them, because the old naming was not
// consistent: most variables were CC_SOMETHING, a few CC_CONNECT_SOMETHING.
//
// CC_ is also exactly the prefix the rename was meant to get rid of — it reads
// as "Claude Code", which this has not been specific to for a long time.
var legacyEnvPrefixes = []string{LegacyEnvPrefix, "CC_CONNECT_"}

// LookupEnv reads the variable named by suffix, preferring MAGPIE_<suffix> and
// falling back to the pre-rename names, so an existing plist or hook script
// keeps working.
//
// A MAGPIE_ variable that is set but empty wins over a non-empty legacy one:
// explicitly clearing the current name must not silently resurrect an old
// value from a plist the user has forgotten about.
func LookupEnv(suffix string) (string, bool) {
	if v, ok := os.LookupEnv(EnvPrefix + suffix); ok {
		return v, true
	}
	for _, prefix := range legacyEnvPrefixes {
		if v, ok := os.LookupEnv(prefix + suffix); ok {
			return v, true
		}
	}
	return "", false
}

// Getenv is LookupEnv without the found flag, mirroring os.Getenv.
func Getenv(suffix string) string {
	v, _ := LookupEnv(suffix)
	return v
}

// EnvName returns the current name for suffix. Use it in messages that tell a
// user which variable to set — they should be pointed at the current name even
// when the value was picked up from a legacy one.
func EnvName(suffix string) string { return EnvPrefix + suffix }

// EnvPair returns the "NAME=value" assignments for suffix under both the
// current and the pre-rename prefix, for passing to a child process.
//
// Both, because the consumer lives outside this repository and cannot be
// updated in the same commit: a user's hook script, or an older binary of this
// project still on PATH. Emitting the legacy name costs one extra entry and
// keeps those working; emitting the current one lets them migrate.
func EnvPair(suffix, value string) []string {
	return []string{
		EnvPrefix + suffix + "=" + value,
		LegacyEnvPrefix + suffix + "=" + value,
	}
}
