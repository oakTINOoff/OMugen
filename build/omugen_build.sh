#!/usr/bin/env bash
# OMugen build wrapper for MSYS2 MINGW64 on Windows.
#
# Usage (from PowerShell):
#   $env:MSYSTEM="MINGW64"; C:\msys64\usr\bin\bash.exe -lc "/d/OMugen/build/omugen_build.sh"
#
# Produces a drop-in OMugen.exe: the engine's own files (scripts, shaders,
# fonts, engine data, runtime DLLs) are embedded, so the exe alone runs from
# an unmodified MUGEN folder. OMUGEN_EMBED=0 builds without the embedded pack
# (faster; the exe then needs the repo's external/, data/, font/ and lib/).
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
TARGET="${1:-Win64}"
echo "==> OMugen build started $(date '+%F %T')" | tee "$LOG"
echo "==> git: $(command -v git)" | tee -a "$LOG"

run_build() {
	./build/build.sh "$TARGET" 2>&1 | tee -a "$LOG"
	return "${PIPESTATUS[0]}"
}

if [[ "${OMUGEN_EMBED:-1}" == "0" ]]; then
	run_build
	status=$?
else
	# Engine assets that Ikemen GO releases ship from a separate repo.
	if [[ ! -d build/screenpack/data/gofx ]]; then
		echo "==> Cloning Ikemen-GO-Screenpack (engine assets)" | tee -a "$LOG"
		git clone --depth=1 https://github.com/ikemen-engine/Ikemen-GO-Screenpack.git build/screenpack 2>&1 | tee -a "$LOG"
	fi
	# The runtime DLLs land in lib/ after a regular build.
	status=0
	if ! compgen -G "lib/SDL2*.dll" >/dev/null; then
		echo "==> First build: producing runtime DLLs in lib/" | tee -a "$LOG"
		run_build
		status=$?
	fi
	# The OMugen screenpack's sprites are generated (tools/screenpackgen).
	if [[ $status -eq 0 ]] && { [[ ! -f screenpack/omugen/system.sff ]] || [[ -n "$(find tools/screenpackgen tools/internal -newer screenpack/omugen/system.sff -name '*.go')" ]]; }; then
		echo "==> Generating the OMugen screenpack sprites" | tee -a "$LOG"
		go run ./tools/screenpackgen 2>&1 | tee -a "$LOG"
		status=${PIPESTATUS[0]}
	fi
	if [[ $status -eq 0 ]]; then
		echo "==> Packing engine files" | tee -a "$LOG"
		go run ./tools/packengine 2>&1 | tee -a "$LOG"
		status=${PIPESTATUS[0]}
	fi
	if [[ $status -eq 0 ]]; then
		export GOFLAGS="${GOFLAGS:-} -tags=omugen_engine"
		run_build
		status=$?
	fi
fi
echo "==> OMugen build finished with exit code $status" | tee -a "$LOG"
exit "$status"
