#!/usr/bin/env bash
# Run the cross-browser harness with the no-root browser-libs on LD_LIBRARY_PATH.
# Chromium and Firefox are runnable in this environment; WebKit runs under
# Xvfb (see docs/cross-browser-results.md, "WebKit green (2026-10-28)"):
#   xvfb-run -a ./scripts/run.sh --project=webkit
# Bare MiniBrowser without a display server aborts with "Could not create EGL
# display" — that is the environment, not the harness.
#
# The vendored library tree is a build input, not a dependency (the same rule
# as fixtures/): it is not committed. Override SYS to point at your own tree;
# on a machine where the browsers' system libraries are installed normally,
# SYS may be left absent and this script falls back to the system loader.
#
# Fresh machine, root or sudo available (the CI path):
#   cd e2e && npm ci && npx playwright install --with-deps chromium firefox
# Fresh machine, no root:
#   extract the runtime packages into $SYS/root, e.g.
#     mkdir -p "$SYS" && cd "$SYS"
#     apt-get download libnspr4 libnss3 libasound2t64 libgbm1 libxkbcommon0 \
#       libatk1.0-0t64 libatk-bridge2.0-0t64 libatspi2.0-0t64 libcups2t64 \
#       libx11-6 libxcb1 libxcomposite1 libxdamage1 libxfixes3 libxrandr2 \
#       libpango-1.0-0 libcairo2 libglib2.0-0t64 libdbus-1-3 libexpat1 \
#       libxkbcommon0 libepoxy0 libgtk-3-0t64
#     for f in ./*.deb; do dpkg-deb -x "$f" root/; done
#   then point a fontconfig file at that tree and set SYS to it. The package
#   set is the one the phase-0 resolve.sh found by iterating `ldd` over
#   Chromium's headless shell; it is Debian/Ubuntu-specific.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# The vendored no-root browser libs. Override with SYS=/path/to/syslibs.
# Falls back gracefully to system libraries if the path is absent.
SYS="${SYS:-/tmp/opencode/shadowdom-proto/syslibs}"
# Normalize empty values to fall back to the default gracefully.
if [ -z "${SYS}" ]; then
  SYS="/tmp/opencode/shadowdom-proto/syslibs"
fi

# Firefox is linked against the NSS libraries shipped next to its binary on
# some installs; keep that directory on the path when it exists.
FIREFOX_DIR="${FIREFOX_DIR:-$(ls -d "${HOME}"/.cache/ms-playwright/firefox-*/firefox 2>/dev/null | tail -1 || true)}"

# Compose LD_LIBRARY_PATH from directories that actually exist. A missing
# vendored tree must degrade to the system libraries, not inject empty entries
# (and must not abort: a developer with system deps needs no SYS at all).
ld_paths=()
if [ -n "${FIREFOX_DIR}" ] && [ -d "${FIREFOX_DIR}" ]; then
  ld_paths+=("${FIREFOX_DIR}")
fi
if [ -d "${SYS}/root/usr/lib/x86_64-linux-gnu" ]; then
  ld_paths+=("${SYS}/root/usr/lib/x86_64-linux-gnu")
fi
if [ -d "${SYS}/root/lib/x86_64-linux-gnu" ]; then
  ld_paths+=("${SYS}/root/lib/x86_64-linux-gnu")
fi
if [ ${#ld_paths[@]} -gt 0 ]; then
  joined="$(IFS=:; printf '%s' "${ld_paths[*]}")"
  export LD_LIBRARY_PATH="${joined}${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}"
fi

if [ ! -d "${SYS}" ]; then
  echo "run.sh: SYS=${SYS} is absent; using the system library path." >&2
  echo "run.sh: if a browser fails to load a shared library, install its deps with" >&2
  echo "run.sh:   cd e2e && npx playwright install --with-deps chromium firefox" >&2
  echo "run.sh: or point SYS= at a vendored tree (see docs/cross-browser-results.md)." >&2
fi

# The vendored fontconfig file names absolute paths inside its own tree; use it
# only when it is present, otherwise let the system fontconfig answer.
if [ -f "${SYS}/fonts.conf" ]; then
  export FONTCONFIG_FILE="${SYS}/fonts.conf"
fi

# Skip Playwright's host-requirement probe: on the no-root path the libraries
# are present but the distribution check cannot see them.
export PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1

cd "$ROOT"
exec npx playwright test "$@"
