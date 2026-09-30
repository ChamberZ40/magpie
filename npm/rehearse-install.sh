#!/usr/bin/env bash
# Rehearse a release end to end without publishing anything: build this
# checkout for every platform, serve it the way a GitHub release would, run
# npm/publish.sh against a throwaway local registry (verdaccio), then install
# from that registry the way a user would. Your real global magpie, ~/.magpie,
# npm login and service are untouched.
#
#   npm/rehearse-install.sh [version]   # default: npm/package.json
#
# Checks, with npm 12 and npm 11: the install prints no install-script warning,
# the binary is already there when npm returns, and `magpie --help` lists init.
# Then without the platform package, which must fall back to downloading.
# Needs the network for npx (set https_proxy if you use one).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:-$(node -p "require('$ROOT/npm/package.json').version")}"
WORK="${REHEARSE_DIR:-$(mktemp -d -t magpie-rehearse)}"
MIRROR_PORT="${MIRROR_PORT:-8765}"
REGISTRY_PORT="${REGISTRY_PORT:-4873}"
REGISTRY="http://127.0.0.1:$REGISTRY_PORT/"

rm -rf "$WORK/mirror" "$WORK/registry" "$WORK/home"
mkdir -p "$WORK/mirror/v$VERSION" "$WORK/registry" "$WORK/home"
echo "rehearse: working in $WORK"
pids=()
trap 'kill "${pids[@]}" 2>/dev/null || true' EXIT

# 1. Release binaries for every platform, named as `make release-all` names them.
echo "rehearse: building $VERSION for every platform"
node -e 'for (const p of require(process.argv[1])) console.log(p.goos, p.goarch)' \
  "$ROOT/npm/platforms.json" | while read -r goos goarch; do
  ext=""; [ "$goos" = windows ] && ext=".exe"
  (cd "$ROOT" && GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build \
    -ldflags "-s -w -X main.version=v$VERSION -X main.commit=$(git rev-parse --short HEAD)" \
    -o "$WORK/mirror/v$VERSION/magpie-v$VERSION-$goos-$goarch$ext" ./cmd/magpie)
done
# Like the real release: archives and checksums.txt only, no raw binaries.
(cd "$WORK/mirror/v$VERSION" && for f in magpie-*; do
  case "$f" in
    *.exe) zip -q "${f%.exe}.zip" "$f" ;;
    *) tar czf "$f.tar.gz" "$f" ;;
  esac
  rm "$f"
done && shasum -a 256 magpie-* >checksums.txt)

# 2. A release mirror and an empty registry that accepts anonymous-ish publishes.
python3 -m http.server "$MIRROR_PORT" --bind 127.0.0.1 --directory "$WORK/mirror" \
  >"$WORK/mirror.log" 2>&1 &
pids+=($!)
cat >"$WORK/registry/config.yaml" <<EOF
storage: $WORK/registry/storage
max_body_size: 200mb
auth:
  htpasswd:
    file: $WORK/registry/htpasswd
    max_users: 10
uplinks: {}
packages:
  '**':
    access: \$all
    publish: \$authenticated
log: { type: stdout, format: pretty, level: warn }
EOF
npx -y verdaccio@6 --config "$WORK/registry/config.yaml" --listen "127.0.0.1:$REGISTRY_PORT" \
  >"$WORK/registry.log" 2>&1 &
pids+=($!)
for _ in $(seq 1 60); do curl -fs "${REGISTRY}-/ping" >/dev/null 2>&1 && break; sleep 1; done
token="$(curl -fs -X PUT "${REGISTRY}-/user/org.couchdb.user:rehearse" \
  -H 'content-type: application/json' -d '{"name":"rehearse","password":"rehearse-only"}' \
  | node -e 'process.stdin.on("data", (d) => console.log(JSON.parse(d).token))')"
printf '//127.0.0.1:%s/:_authToken=%s\n' "$REGISTRY_PORT" "$token" >"$WORK/npmrc"

# npm 11 still runs install scripts; fetch it before npm is pointed at the
# local registry, which has nothing but magpie.
npm install --prefix "$WORK/npm11" npm@11 >"$WORK/npm11.log" 2>&1 \
  || { cat "$WORK/npm11.log"; echo "rehearse: FAIL could not fetch npm 11" >&2; exit 1; }

# Everything npm does from here sees only the sandbox.
export HOME="$WORK/home" NPM_CONFIG_USERCONFIG="$WORK/npmrc" NPM_CONFIG_REGISTRY="$REGISTRY"
export NO_PROXY=127.0.0.1 no_proxy=127.0.0.1
export MAGPIE_DOWNLOAD_BASE="http://127.0.0.1:$MIRROR_PORT"

# 3. The real publish script, pointed at the mirror and the local registry.
"$ROOT/npm/publish.sh" "$VERSION" >"$WORK/publish.log" 2>&1 \
  || { cat "$WORK/publish.log"; echo "rehearse: FAIL publish.sh" >&2; exit 1; }
grep '^publish:' "$WORK/publish.log"

# 4. Install the way a user would, and check what they get.
fail=0
check_install() { # check_install <label> <npm command...>
  local label="$1" prefix="$WORK/prefix-${1// /-}" log out before="$fail"
  shift
  rm -rf "$prefix"
  log="$WORK/install-${label// /-}.log"
  "$@" install -g --prefix "$prefix" "@z40/magpie@$VERSION" >"$log" 2>&1 \
    || { cat "$log"; echo "rehearse: FAIL [$label] npm install" >&2; fail=1; return; }
  if grep -qi "install-scripts\|allowScripts" "$log"; then
    cat "$log"; echo "rehearse: FAIL [$label] npm warned about install scripts" >&2; fail=1
  fi
  if ! ls "$prefix"/lib/node_modules/@z40/magpie/node_modules/@z40/magpie-*/bin/magpie* >/dev/null 2>&1; then
    echo "rehearse: FAIL [$label] no platform binary after npm install" >&2; fail=1
  fi
  out="$("$prefix/bin/magpie" --version 2>&1 || true)"
  if [[ "$out" == *"[magpie]"* ]]; then
    echo "$out"; echo "rehearse: FAIL [$label] first run still downloaded" >&2; fail=1
  fi
  [[ "$out" == *"$VERSION"* ]] || { echo "rehearse: FAIL [$label] got: $out" >&2; fail=1; }
  [[ "$("$prefix/bin/magpie" --help 2>&1)" == *" init"* ]] \
    || { echo "rehearse: FAIL [$label] --help does not list init" >&2; fail=1; }
  [ "$fail" -ne "$before" ] || echo "rehearse: ok [$label]"
}
check_install "npm $(npm --version)" npm
check_install "npm 11" "$WORK/npm11/node_modules/.bin/npm"

# Without its platform package (a platform with none, or one npm skipped) the
# wrapper must fall back to downloading the binary on first run.
prefix="$WORK/prefix-fallback"
rm -rf "$prefix"
npm install -g --prefix "$prefix" "@z40/magpie@$VERSION" >"$WORK/install-fallback.log" 2>&1
rm -rf "$prefix"/lib/node_modules/@z40/magpie/node_modules/@z40
out="$("$prefix/bin/magpie" --version 2>&1 || true)"
if [[ "$out" == *"Quick setup: run \`magpie init\`"* && "$out" == *"$VERSION"* ]]; then
  echo "rehearse: ok [no platform package: downloads on first run]"
else
  echo "$out"; echo "rehearse: FAIL [no platform package] fallback download" >&2; fail=1
fi

[ "$fail" -eq 0 ] || exit 1
cat <<EOF

rehearse: OK — $VERSION published to a local registry and installed cleanly.
Try the installed magpie as a brand-new user (sandboxed HOME):
  HOME="$WORK/home" "$WORK/prefix-npm-$(npm --version)/bin/magpie" init
EOF
