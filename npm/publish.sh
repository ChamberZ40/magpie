#!/usr/bin/env bash
# Publish a release to npm: the six platform packages, then the wrapper that
# depends on them. Run it after CI has published the GitHub release.
#
#   npm/publish.sh <version>          # e.g. npm/publish.sh 1.0.5
#
# The binaries come from the release's archives (checked against its
# checksums.txt), so npm ships exactly what GitHub does. Platform packages go
# first: once the wrapper is public, every install resolves its pinned
# optionalDependencies, and a missing one silently drops users onto the
# slower download fallback. A version already on the registry is skipped, so
# a publish that failed halfway can simply be rerun.
#
# MAGPIE_DOWNLOAD_BASE (a mirror laid out like releases/download) and
# NPM_CONFIG_REGISTRY redirect both ends, for rehearsing against a local
# registry.
set -euo pipefail

NPM_DIR="$(cd "$(dirname "$0")" && pwd)"
VERSION="${1:-}"
VERSION="${VERSION#v}"
if [ -z "$VERSION" ]; then
  echo "usage: npm/publish.sh <version>" >&2
  exit 2
fi

pkg_version="$(node -p "require('$NPM_DIR/package.json').version")"
if [ "$pkg_version" != "$VERSION" ]; then
  echo "publish: npm/package.json is $pkg_version, not $VERSION — run npm/set-version.js first" >&2
  exit 1
fi

base="${MAGPIE_DOWNLOAD_BASE:-https://github.com/ChamberZ40/magpie/releases/download}"
base="${base%/}/v$VERSION"
work="$(mktemp -d -t magpie-publish)"
trap 'rm -rf "$work"' EXIT

echo "publish: fetching release binaries from $base"
curl -fsSL -o "$work/checksums.txt" "$base/checksums.txt"
# The release carries archives only (plus checksums.txt covering them), so
# fetch each platform's archive, verify it, and unpack the binary.
archives="$(node -e '
  const { PLATFORMS, NAME } = require(process.argv[1] + "/platforms");
  for (const p of PLATFORMS) {
    console.log(`${NAME}-v${process.argv[2]}-${p.goos}-${p.goarch}${p.goos === "windows" ? ".zip" : ".tar.gz"}`);
  }' "$NPM_DIR" "$VERSION")"
mkdir -p "$work/archives" "$work/bin"
for name in $archives; do
  curl -fsSL -o "$work/archives/$name" "$base/$name" \
    || { echo "publish: could not download $base/$name" >&2; exit 1; }
  grep "  $name\$" "$work/checksums.txt" >>"$work/wanted.txt" \
    || { echo "publish: $name is not in checksums.txt" >&2; exit 1; }
done
(cd "$work/archives" && shasum -a 256 -c "$work/wanted.txt" >/dev/null) \
  || { echo "publish: checksum mismatch, refusing to publish" >&2; exit 1; }
for name in $archives; do
  case "$name" in
    *.zip) unzip -q -o "$work/archives/$name" -d "$work/bin" ;;
    *) tar xzf "$work/archives/$name" -C "$work/bin" ;;
  esac
done

node "$NPM_DIR/stage-platforms.js" "$VERSION" "$work/bin" "$work/pkgs" >/dev/null

publish_dir() { # publish_dir <dir>
  local name
  name="$(node -p "require('$1/package.json').name")"
  if npm view "$name@$VERSION" version >/dev/null 2>&1; then
    echo "publish: $name@$VERSION already published, skipping"
    return
  fi
  (cd "$1" && npm publish --access public)
}

for dir in "$work"/pkgs/*/; do
  publish_dir "${dir%/}"
done
publish_dir "$NPM_DIR"

echo "publish: done — npm install -g @z40/magpie@$VERSION"
