package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// SaveAgentWorkDir sets work_dir under a project's [projects.agent.options],
// preserving comments and layout.
func SaveAgentWorkDir(projectName, workDir string) error {
	configMu.Lock()
	defer configMu.Unlock()
	return patchProjectAgentOption(projectName, "work_dir", workDir)
}

// SaveAgentType sets type under a project's [projects.agent], preserving
// comments and layout.
func SaveAgentType(projectName, agentType string) error {
	configMu.Lock()
	defer configMu.Unlock()
	return patchProjectAgentField(projectName, "type", agentType)
}

// patchProjectAgentField does a surgical text-level update of a single key
// directly under [projects.agent] for the given project. The caller must hold
// configMu.
func patchProjectAgentField(projectName, key, value string) error {
	if ConfigPath == "" {
		return fmt.Errorf("config path not set")
	}
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	projectIdx := -1
	for i := range cfg.Projects {
		if cfg.Projects[i].Name == projectName {
			projectIdx = i
			break
		}
	}
	if projectIdx < 0 {
		return fmt.Errorf("project %q not found in config", projectName)
	}

	lines, hadTrailing := splitConfigLines(string(data))
	spans := buildRawProjectSpans(lines)
	if projectIdx >= len(spans) {
		return fmt.Errorf("project %q located in parsed config but not raw file", projectName)
	}
	span := spans[projectIdx]
	if span.agentStart < 0 {
		return fmt.Errorf("project %q has no [projects.agent] section", projectName)
	}

	lines = upsertTomlStringKey(lines, span.agentStart+1, span.agentEnd, key, value)
	return writeRawConfig(joinConfigLines(lines, hadTrailing))
}

// OptionValue is one key to write, with its value already TOML-encoded
// (`"card"`, `true`, `3`).
type OptionValue struct {
	Key string
	Raw string
}

// SaveSectionValue sets key under a top-level [section], creating the section
// when absent. raw must already be TOML-encoded. Comments and layout are
// preserved.
func SaveSectionValue(section, key, raw string) error {
	configMu.Lock()
	defer configMu.Unlock()
	return patchSectionField(section, key, raw)
}

// SavePlatformOptions writes values into [projects.platforms.options] of the
// project's first platform of platformType, creating the options table when
// absent. Comments and layout are preserved.
func SavePlatformOptions(projectName, platformType string, values []OptionValue) error {
	configMu.Lock()
	defer configMu.Unlock()

	if ConfigPath == "" {
		return fmt.Errorf("config path not set")
	}
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	projectIdx, platformIdx := findPlatform(cfg, projectName, platformType)
	if projectIdx < 0 {
		return fmt.Errorf("project %q not found in config", projectName)
	}
	if platformIdx < 0 {
		return fmt.Errorf("project %q has no %s platform", projectName, platformType)
	}

	lines, hadTrailing := splitConfigLines(string(data))
	span := func() (rawPlatformSpan, error) {
		spans := buildRawProjectSpans(lines)
		if projectIdx >= len(spans) || platformIdx >= len(spans[projectIdx].platforms) {
			return rawPlatformSpan{}, fmt.Errorf("project %q platform located in parsed config but not raw file", projectName)
		}
		return spans[projectIdx].platforms[platformIdx], nil
	}
	sp, err := span()
	if err != nil {
		return err
	}
	if sp.optionsStart < 0 {
		lines = insertLines(lines, sp.end+1, []string{"", "[projects.platforms.options]"})
		if sp, err = span(); err != nil {
			return err
		}
	}
	for _, v := range values {
		lines = upsertTomlRawKey(lines, sp.optionsStart+1, sp.optionsEnd, v.Key, v.Raw)
		if sp, err = span(); err != nil {
			return err
		}
	}
	return writeRawConfig(joinConfigLines(lines, hadTrailing))
}

func findPlatform(cfg *Config, projectName, platformType string) (projectIdx, platformIdx int) {
	for i := range cfg.Projects {
		if cfg.Projects[i].Name != projectName {
			continue
		}
		for j, p := range cfg.Projects[i].Platforms {
			if p.Type == platformType {
				return i, j
			}
		}
		return i, -1
	}
	return -1, -1
}
