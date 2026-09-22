package appid

import "os"

// EnvPrefix is the prefix on every environment variable this project reads.
const EnvPrefix = "MAGPIE_"

// LookupEnv reads the variable named by suffix.
//
// Only the current name: the pre-rename CC_ and CC_CONNECT_ prefixes were
// consulted as a fallback until this project's only deployment had migrated,
// and CC_ is a prefix other tools use too, so keeping it meant a variable
// meant for something else could quietly configure this one.
func LookupEnv(suffix string) (string, bool) {
	return os.LookupEnv(EnvPrefix + suffix)
}

// Getenv is LookupEnv without the found flag, mirroring os.Getenv.
func Getenv(suffix string) string {
	v, _ := LookupEnv(suffix)
	return v
}

// EnvName returns the name of the variable holding suffix. Use it in messages
// that tell a user which variable to set.
func EnvName(suffix string) string { return EnvPrefix + suffix }

// EnvPair returns the "NAME=value" assignment for suffix, as a slice because
// callers append it to a child process's environment.
func EnvPair(suffix, value string) []string {
	return []string{EnvPrefix + suffix + "=" + value}
}
