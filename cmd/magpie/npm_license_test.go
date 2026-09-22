package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The npm tarball is a redistribution, and MIT conditions the grant on the
// copyright notice travelling with every copy. npm auto-includes a LICENSE
// only from the package directory, and ours lives at the repo root — one level
// above npm/ — so without a copy inside npm/ the published package carries the
// "license": "MIT" claim and none of the notice it refers to. That is the exact
// gap upstream has, and the reason this fork's LICENSE has a provenance note.
//
// A copy can drift from its original, so this asserts they are identical
// rather than merely that the file exists.
func TestNpmPackageShipsTheLicense(t *testing.T) {
	root, err := os.ReadFile(filepath.Join("..", "..", "LICENSE"))
	if err != nil {
		t.Fatalf("read root LICENSE: %v", err)
	}
	packaged, err := os.ReadFile(filepath.Join("..", "..", "npm", "LICENSE"))
	if err != nil {
		t.Fatalf("read npm/LICENSE: %v (the published package would ship no license notice)", err)
	}
	if string(packaged) != string(root) {
		t.Error("npm/LICENSE has drifted from the root LICENSE; copy the root file over it")
	}
}
