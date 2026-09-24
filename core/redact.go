package core

import (
	"regexp"
	"strings"
)

var (
	// secretAssignRe matches KEY=value where the key name reads as a credential.
	//
	// The prefix before the credential word is optional. It used to be
	// mandatory ([A-Za-z_][A-Za-z0-9_]*), which quietly excluded every bare
	// key — API_KEY=, TOKEN=, SECRET= all went through unmasked while
	// MY_API_KEY= was caught. The bare form is the common one.
	//
	// The value alternatives are ordered longest-first so a quoted value is
	// consumed whole: a secret containing a space would otherwise match only up
	// to the space and leak its tail.
	secretAssignRe = regexp.MustCompile(`(?i)\b((?:[A-Za-z_][A-Za-z0-9_]*)?(?:token|secret|password|api[_-]?key|authorization|cookie|credential|bearer|session[_-]?id|client[_-]?secret|access[_-]?key)[A-Za-z0-9_]*)=("[^"]*"|'[^']*'|[^\s"'` + "`" + `]+)`)
	authHeaderRe   = regexp.MustCompile(`(?i)(Authorization\s*:\s*(?:Bearer|Basic|Token)\s+)([^\s'"` + "`" + `]+)`)
)

// RedactInlineSecrets masks credentials that appear inline in free text —
// KEY=value assignments and Authorization headers.
//
// It lives in core rather than in a platform because every platform and every
// display path needs the same answer. It used to sit in the Feishu renderer,
// which meant the other platforms had no redaction at all and, within Feishu,
// only the paths that happened to call it were covered.
func RedactInlineSecrets(value string) string {
	value = secretAssignRe.ReplaceAllString(value, "$1=[redacted]")
	value = authHeaderRe.ReplaceAllString(value, "$1[redacted]")
	return value
}

// sanitizeToolResultForDisplay prepares a tool's raw output for any surface
// that shows it: redact, then cut to maxLen.
//
// The order is the point. Truncating first can slice a secret out of the shape
// the patterns recognize — cut inside a quoted value and the closing quote is
// gone, so neither the quoted nor the unquoted alternative matches and the
// surviving prefix ships in the clear. Redacting first leaves nothing to slice.
func sanitizeToolResultForDisplay(result string, maxLen int) string {
	return truncateIf(RedactInlineSecrets(result), maxLen)
}

// RedactEnv returns a copy of env with values of sensitive keys masked.
// Only env vars whose key contains a sensitive substring are redacted.
func RedactEnv(env []string) []string {
	sensitiveKeys := []string{
		"KEY", "TOKEN", "SECRET", "PASSWORD", "CREDENTIAL",
	}
	out := make([]string, len(env))
	for i, e := range env {
		idx := strings.IndexByte(e, '=')
		if idx < 0 {
			out[i] = e
			continue
		}
		key := strings.ToUpper(e[:idx])
		redact := false
		for _, s := range sensitiveKeys {
			if strings.Contains(key, s) {
				redact = true
				break
			}
		}
		if redact {
			out[i] = e[:idx+1] + "***"
		} else {
			out[i] = e
		}
	}
	return out
}

// RedactArgs returns a copy of args with values after sensitive flag names masked.
// Sensitive flags: --api-key, --api_key, --token, --secret, -k, etc.
func RedactArgs(args []string) []string {
	out := make([]string, len(args))
	copy(out, args)

	sensitiveFlags := []string{
		"--api-key", "--api_key", "--apikey",
		"--token", "--secret", "--password",
		"-k",
	}

	for i := 0; i < len(out); i++ {
		arg := strings.ToLower(out[i])

		// --flag=value format
		for _, f := range sensitiveFlags {
			if strings.HasPrefix(arg, f+"=") {
				out[i] = out[i][:strings.Index(out[i], "=")+1] + "***"
				break
			}
		}

		// --flag value format
		for _, f := range sensitiveFlags {
			if arg == f && i+1 < len(out) {
				out[i+1] = "***"
				i++
				break
			}
		}
	}
	return out
}
