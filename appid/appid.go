// Package appid holds the project's on-disk identity: the directory names,
// service label, and environment-variable prefix that carry the product name.
//
// It exists because that name changed. The project was called cc-connect back
// when it only drove Claude Code; it is now magpie. Renaming the source is
// free, but the name is also baked into state that lives outside the
// repository — a user's ~/.cc-connect, the .cc-connect directory this writes
// into their workspaces, the CC_* variables in their launchd plist and hook
// scripts. Every name here therefore has a legacy counterpart that is still
// honored on read, so an install predating the rename keeps working without
// anyone moving files or editing a plist.
//
// This package is a leaf: it imports only the standard library, so core,
// config, daemon and cmd can all depend on it without coupling to each other.
package appid

const (
	// Name is the product name, and the binary name.
	Name = "magpie"
	// LegacyName is what the project was called before the rename.
	LegacyName = "cc-connect"

	// DirName is the dotted directory this project keeps state in, both under
	// the user's home and inside a workspace.
	DirName = "." + Name
	// LegacyDirName is the pre-rename equivalent of DirName.
	LegacyDirName = "." + LegacyName

	// ServiceLabel identifies the background service to launchd and systemd.
	ServiceLabel = "com." + Name + ".service"
	// LegacyServiceLabel is the pre-rename equivalent of ServiceLabel. An
	// installed service keeps running under it until the user reinstalls, so
	// uninstall and status have to recognize both.
	LegacyServiceLabel = "com." + LegacyName + ".service"
)
