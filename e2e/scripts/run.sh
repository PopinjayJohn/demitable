#!/usr/bin/env bash
# Run the cross-browser harness with the no-root browser-libs on LD_LIBRARY_PATH.
# Chromium and Firefox are runnable in this environment; WebKit is not (see
# docs/cross-browser-results.md) because the headless MiniBrowser cannot create
# an EGL display here.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SYS=/tmp/opencode/shadowdom-proto/syslibs
export LD_LIBRARY_PATH="/home/johan/.cache/ms-playwright/firefox-1555/firefox:${SYS}/root/usr/lib/x86_64-linux-gnu:${SYS}/root/lib/x86_64-linux-gnu:${LD_LIBRARY_PATH:-}"
export FONTCONFIG_FILE="${SYS}/fonts.conf"
export PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1

cd "$ROOT"
exec npx playwright test "$@"
