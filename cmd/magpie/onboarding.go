package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ChamberZ40/magpie/config"
	"github.com/ChamberZ40/magpie/core"
)

// placeholderWorkDir is the work_dir the starter config ships with. Setup
// commands look for it to remind the user that the agent has nowhere to run.
const placeholderWorkDir = "/path/to/your/project"

// platformSetupCommands maps a setup command to the registered platform types
// it can configure, in the order they are offered to a new user.
var platformSetupCommands = []struct {
	command   string
	platforms []string
}{
	{"magpie feishu setup", []string{"feishu", "lark"}},
	{"magpie weixin setup", []string{"weixin"}},
}

// setupCommands lists the QR-code setup commands this build can actually use.
// Deriving it from the registry keeps a selective build from advertising a
// platform it does not have.
func setupCommands() []string {
	registered := make(map[string]bool)
	for _, n := range core.ListRegisteredPlatforms() {
		registered[n] = true
	}
	var cmds []string
	for _, sc := range platformSetupCommands {
		for _, p := range sc.platforms {
			if registered[p] {
				cmds = append(cmds, sc.command)
				break
			}
		}
	}
	return cmds
}

func indentedSetupCommands(indent string) string {
	var b strings.Builder
	for _, cmd := range setupCommands() {
		b.WriteString(indent + cmd + "\n")
	}
	return b.String()
}

// firstRunGuide is printed after a bare `magpie` writes the starter config.
// It is the only onboarding text an npm user is sure to see: npm may skip the
// package's install script entirely.
func firstRunGuide(configPath string) string {
	return fmt.Sprintf(`Created a starter config at %s

Quickest way to finish, one question at a time:
  magpie init

Or by hand:
  1. Open that file and set work_dir to the directory the agent should work in.
  2. Connect a chat app. Each command shows a QR code to scan:
%s  3. Run magpie again, then message your bot.

Every option, annotated:
  magpie config example
`, configPath, indentedSetupCommands("       "))
}

// ensureConfigForSetup writes the starter config when none exists, so a setup
// command can be the very first thing a new user runs.
func ensureConfigForSetup(path string) (created bool, err error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("check config %s: %w", path, err)
	}
	if err := bootstrapConfig(path); err != nil {
		return false, fmt.Errorf("create config %s: %w", path, err)
	}
	return true, nil
}

// bootstrapConfigForSetup is ensureConfigForSetup for the setup commands: it
// reports what it did and exits on failure.
func bootstrapConfigForSetup(path string) {
	created, err := ensureConfigForSetup(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if created {
		fmt.Printf("Created a starter config at %s\n\n", path)
	}
}

// workDirReminder returns a note when the project's work_dir is still the
// starter placeholder, or "" when it looks set.
func workDirReminder(configPath, project string) string {
	cfg, err := config.LoadPermissive(configPath)
	if err != nil {
		return ""
	}
	for _, p := range cfg.Projects {
		if p.Name != project {
			continue
		}
		if dir, _ := p.Agent.Options["work_dir"].(string); dir == placeholderWorkDir {
			return fmt.Sprintf("Before starting magpie, set work_dir for project %q in %s\n"+
				"to the directory the agent should work in (it must already exist).\n", project, configPath)
		}
	}
	return ""
}

// printSetupNextSteps closes a successful setup command.
func printSetupNextSteps(configPath, project string) {
	if inInitWizard {
		return
	}
	if note := workDirReminder(configPath, project); note != "" {
		fmt.Println("⚠️  " + note)
	}
	fmt.Println("Next: run magpie (or restart the daemon), then message your bot.")
}
