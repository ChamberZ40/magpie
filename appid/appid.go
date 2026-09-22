// Package appid holds the project's on-disk identity: the directory names,
// service label, and environment-variable prefix that carry the product name.
//
// It exists to keep that name in one place. The project was called cc-connect
// before it grew past Claude Code; every trace of that name is gone, and
// nothing here honors it, because this project had no installs on the old name
// to keep working.
//
// This package is a leaf: it imports only the standard library, so core,
// config, daemon and cmd can all depend on it without coupling to each other.
package appid

const (
	// Name is the product name, and the binary name.
	Name = "magpie"

	// DirName is the dotted directory this project keeps state in, both under
	// the user's home and inside a workspace.
	DirName = "." + Name

	// ServiceLabel identifies the background service to launchd and systemd.
	ServiceLabel = "com." + Name + ".service"
)
