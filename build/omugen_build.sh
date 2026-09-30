#!/usr/bin/env bash
# OMugen build wrapper for MSYS2 MINGW64 on Windows.
#
# Usage (from PowerShell):
#   $env:MSYSTEM="MINGW64"; C:\msys64\usr\bin\bash.exe -lc "/d/OMugen/build/omugen_build.sh"
#
# Why this wrapper exists:
# - Smart App Control blocks MSYS2's unsigned /usr/bin/git.exe, so we put the
#   signed Git for Windows first on PATH.
# - Smart App Control also blocks gcc's liblto_plugin.dll; a default gcc
#   "specs" file in the MSYS2 toolchain disables the LTO linker plugin.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
source "$REPO_ROOT/build/omugen_env.sh"
cd "$REPO_ROOT"

LOG="$REPO_ROOT/build/omugen_build.log"
echo "==> OMugen build started $(date '+%F %T')" | tee "$LOG"
echo "==> git: $(command -v git)" | tee -a "$LOG"

./build/build.sh "${1:-Win64}" 2>&1 | tee -a "$LOG"
status=${PIPESTATUS[0]}
echo "==> OMugen build finished with exit code $status" | tee -a "$LOG"
exit "$status"
