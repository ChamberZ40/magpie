package main

import "github.com/ChamberZ40/magpie/config"

// projectPinsToolDetail reports whether this project sets tool_detail in its own
// [projects.display]. /verbose persists to the global [display] section, which
// the project value outranks on reload, so the engine warns when both exist.
func projectPinsToolDetail(proj *config.ProjectConfig) bool {
	return proj != nil && proj.Display != nil && proj.Display.ToolDetail != nil
}
