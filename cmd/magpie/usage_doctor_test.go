package main

import (
	"regexp"
	"strings"
	"testing"
)

// The --help text is hand-written while the dispatchers are code, so the two
// drift silently: `magpie doctor runas` was advertised in both the command
// list and the examples for a subcommand that never existed, and running it
// exited 2 with "unknown doctor subcommand". These tests pin every doctor
// subcommand the help text names to one the dispatcher actually accepts.

func doctorSubcommandsInUsage(t *testing.T) []string {
	t.Helper()

	var names []string
	seen := make(map[string]bool)
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}

	lines := strings.Split(usageText(), "\n")
	inDoctorSection := false
	for _, line := range lines {
		// "  doctor             Diagnose the local setup", followed by its
		// subcommands at one deeper indent until the next blank line.
		if strings.HasPrefix(line, "  doctor ") {
			inDoctorSection = true
			continue
		}
		if inDoctorSection {
			if !strings.HasPrefix(line, "    ") {
				inDoctorSection = false
			} else {
				add(strings.Fields(line)[0])
			}
		}
	}

	// "  magpie doctor user-isolation    Check ..." in the Examples block.
	example := regexp.MustCompile(`magpie doctor (\S+)`)
	for _, m := range example.FindAllStringSubmatch(usageText(), -1) {
		add(m[1])
	}

	if len(names) == 0 {
		t.Fatal("no doctor subcommands found in usage text; the parser or the help layout changed")
	}
	return names
}

func TestUsage_DoctorSubcommandsAreDispatchable(t *testing.T) {
	accepted := make(map[string]bool, len(doctorSubcommands))
	for _, name := range doctorSubcommands {
		accepted[name] = true
	}

	for _, name := range doctorSubcommandsInUsage(t) {
		if !accepted[name] {
			t.Errorf("--help advertises %q, which `magpie doctor` rejects; accepted: %v",
				name, doctorSubcommands)
		}
	}
}
