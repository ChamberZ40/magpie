package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The release tag, the Makefile's VERSION and npm/package.json's version have
// to name the same release. install.js builds its download URL as
// releases/download/v${package.version}/..., so a package published ahead of
// its tag 404s on every postinstall — the failure lands on the user, not on us,
// and a published version cannot be taken back.
//
// The Makefile has carried a comment asking for this since the fork. A comment
// is not a check: 1.0.0 shipped to npm from a tree three commits behind the
// tag. This is the check.
func TestNpmPackageVersionMatchesMakefile(t *testing.T) {
	makefile := readRepoFile(t, "Makefile")
	pkg := readRepoFile(t, "npm", "package.json")

	match := regexp.MustCompile(`(?m)^VERSION\s*:?=\s*(\S+)`).FindStringSubmatch(makefile)
	if match == nil {
		t.Fatal("Makefile has no VERSION assignment")
	}
	makeVersion := strings.TrimPrefix(match[1], "v")

	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(pkg), &manifest); err != nil {
		t.Fatalf("parse npm/package.json: %v", err)
	}

	if manifest.Version != makeVersion {
		t.Errorf("npm/package.json is %q but the Makefile builds %q; "+
			"publishing this package would download a release tag that does not exist",
			manifest.Version, makeVersion)
	}
}

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", ".."}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
