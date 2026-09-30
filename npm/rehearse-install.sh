#!/usr/bin/env bash
# Rehearse `npm install -g @z40/magpie` end to end without publishing anything:
# build this checkout for the host, serve it the way a GitHub release would,
# and install the packed npm wrapper from that local mirror into a throwaway
# prefix and HOME. Your real global magpie, ~/.magpie and service are untouched.
#
#   npm/rehearse-install.sh [version]   # default: npm/package.json
#
# Leaves everything under $REHEARSE_DIR (default: a fresh temp dir) and prints
# how to run the installed magpie from there.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:-$(node -p "require('$ROOT/npm/package.json').version")}"
WORK="${REHEARSE_DIR:-$(mktemp -d -t magpie-rehearse)}"
PORT="${PORT:-8765}"
GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"

if [ "$GOOS" = "windows" ]; then
  echo "rehearse: run this on macOS or Linux" >&2
  exit 1
fi

mkdir -p "$WORK/mirror/v$VERSION" "$WORK/home"
echo "rehearse: working in $WORK"

# 1. The release archive, named exactly as npm/install.js expects.
bin_name="magpie-v$VERSION-$GOOS-$GOARCH"
(cd "$ROOT" && CGO_ENABLED=0 go build \
  -ldflags "-s -w -X main.version=v$VERSION -X main.commit=$(git rev-parse --short HEAD)" \
  -o "$WORK/mirror/v$VERSION/$bin_name" ./cmd/magpie)
(cd "$WORK/mirror/v$VERSION" && tar czf "$bin_name.tar.gz" "$bin_name" && rm "$bin_name")

# 2. The npm package as `npm publish` would upload it, at the rehearsed version.
rm -rf "$WORK/pkg"
cp -R "$ROOT/npm" "$WORK/pkg"
rm -rf "$WORK/pkg/bin"
(cd "$WORK/pkg" && npm pkg set version="$VERSION" >/dev/null \
  && npm pack --silent --pack-destination "$WORK" >/dev/null)
tarball="$WORK/z40-magpie-$VERSION.tgz"

# 3. Serve the mirror and install from it.
python3 -m http.server "$PORT" --bind 127.0.0.1 --directory "$WORK/mirror" \
  >"$WORK/mirror.log" 2>&1 &
server=$!
trap 'kill "$server" 2>/dev/null || true' EXIT
sleep 1

# npm 12 blocks package install scripts by default, so a plain install leaves
# the download (and the "magpie init" hint) to the wrapper's first run; npm 11
# and older still run postinstall. Rehearse both. The npm 11 run is fetched
# with npx, so it needs the network (set https_proxy if you use one).
mirror_env=(HOME="$WORK/home" NO_PROXY=127.0.0.1 no_proxy=127.0.0.1
  MAGPIE_DOWNLOAD_BASE="http://127.0.0.1:$PORT")
fail=0
check() { # check <label> <log with the hint> <prefix>
  local version help
  version="$(env "${mirror_env[@]}" "$3/bin/magpie" --version 2>&1 || true)"
  help="$(env "${mirror_env[@]}" "$3/bin/magpie" --help 2>&1 || true)"
  grep -q "magpie init" "$2" \
    || { echo "rehearse: FAIL [$1] no magpie init hint in $2" >&2; fail=1; }
  [[ "$version" == *"$VERSION"* ]] \
    || { echo "rehearse: FAIL [$1] installed binary is not $VERSION" >&2; fail=1; }
  [[ "$help" == *" init"* ]] \
    || { echo "rehearse: FAIL [$1] --help does not list init" >&2; fail=1; }
  [ "$fail" -ne 0 ] || echo "rehearse: ok [$1]"
}

rm -rf "$WORK/prefix" "$WORK/prefix-npm11"
env "${mirror_env[@]}" npm install -g --prefix "$WORK/prefix" "$tarball" \
  >"$WORK/install-default.log" 2>&1
env "${mirror_env[@]}" "$WORK/prefix/bin/magpie" --version >"$WORK/first-run.log" 2>&1
echo "--- npm $(npm --version) install, then first run:"; cat "$WORK/first-run.log"
check "npm $(npm --version)" "$WORK/first-run.log" "$WORK/prefix"

if env "${mirror_env[@]}" npx -y npm@11 install -g --foreground-scripts \
  --prefix "$WORK/prefix-npm11" "$tarball" >"$WORK/install-npm11.log" 2>&1; then
  echo "--- npm 11 install:"; grep '^\[magpie\]' "$WORK/install-npm11.log" || true
  check "npm 11" "$WORK/install-npm11.log" "$WORK/prefix-npm11"
else
  echo "rehearse: skip [npm 11] could not run npx npm@11 — see $WORK/install-npm11.log" >&2
fi

[ "$fail" -eq 0 ] || exit 1
magpie="$WORK/prefix/bin/magpie"

cat <<EOF

rehearse: OK — installed $VERSION into $WORK/prefix

Try it as a brand-new user (sandboxed HOME, nothing of yours is read):
  export PATH="$WORK/prefix/bin:\$PATH"
  HOME="$WORK/home" magpie init

To get a real reply you need your real agent login, so use your real HOME but
a separate config, and stop your running service first so two magpies do not
answer the same bot:
  magpie daemon stop
  $magpie init --config "$WORK/config.toml"     # choose "run in this terminal"
  magpie daemon start                           # afterwards
EOF
