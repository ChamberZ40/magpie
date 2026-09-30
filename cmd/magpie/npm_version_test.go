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
// releases/download/v${package.version}/..., and publish.sh fetches the
// platform binaries from that tag, so a package published ahead of its tag
// cannot be built or installed — the failure lands on the user, not on us,
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

// The wrapper finds its binary in one platform package per OS/CPU, pinned in
// optionalDependencies. A platform missing from that list, pinned to another
// release, or not built by the Makefile leaves its users on the slow download
// fallback — or, for a release asset that does not exist, on none at all.
func TestNpmPlatformPackagesMatchRelease(t *testing.T) {
	var platforms []struct {
		Node   string `json:"node"`
		GOOS   string `json:"goos"`
		GOARCH string `json:"goarch"`
	}
	if err := json.Unmarshal([]byte(readRepoFile(t, "npm", "platforms.json")), &platforms); err != nil {
		t.Fatalf("parse npm/platforms.json: %v", err)
	}
	var manifest struct {
		Version              string            `json:"version"`
		Scripts              map[string]string `json:"scripts"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if err := json.Unmarshal([]byte(readRepoFile(t, "npm", "package.json")), &manifest); err != nil {
		t.Fatalf("parse npm/package.json: %v", err)
	}
	makefile := readRepoFile(t, "Makefile")

	want := make(map[string]string, len(platforms))
	for _, p := range platforms {
		want["@z40/magpie-"+p.Node] = manifest.Version
		if !strings.Contains(makefile, p.GOOS+"/"+p.GOARCH) {
			t.Errorf("platforms.json lists %s/%s but the Makefile's PLATFORMS does not build it", p.GOOS, p.GOARCH)
		}
	}
	for name, version := range want {
		if got, ok := manifest.OptionalDependencies[name]; !ok {
			t.Errorf("optionalDependencies is missing %s", name)
		} else if got != version {
			t.Errorf("optionalDependencies pins %s to %q, want exactly %q (run node npm/set-version.js)", name, got, version)
		}
	}
	for name := range manifest.OptionalDependencies {
		if _, ok := want[name]; !ok {
			t.Errorf("optionalDependencies has %s, which platforms.json does not list", name)
		}
	}
	// npm 12 blocks install scripts by default and warns about each one; the
	// platform packages exist so that installing needs none.
	if len(manifest.Scripts) > 0 {
		t.Errorf("npm/package.json has scripts %v; installing must not need any", manifest.Scripts)
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
