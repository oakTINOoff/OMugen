# Source this in an MSYS2 MINGW64 shell before running go/make commands:
#   source /d/OMugen/build/omugen_env.sh
# Smart App Control blocks MSYS2's unsigned git.exe, so use the signed Git for Windows.
if [[ -x "/c/Program Files/Git/cmd/git.exe" ]]; then
	export PATH="/c/Program Files/Git/cmd:$PATH"
fi
# The repository this script belongs to (D:\OMugen here, elsewhere in CI).
_omugen_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$_omugen_root" || return

export CGO_ENABLED=1
export GOEXPERIMENT=arenas

# cgo flags for `go test ./src` / `go vet ./src` (build.sh sets its own).
# FFmpeg is the minimal copy build.sh compiled into build/ffmpeg.
export PKG_CONFIG_PATH="$_omugen_root/build/ffmpeg/lib/pkgconfig:/mingw64/lib/pkgconfig:${PKG_CONFIG_PATH:-}"
_omugen_pc="libavformat libavcodec libavutil libswscale libswresample libavfilter libxmp sdl2"
if pkg-config --exists $_omugen_pc 2>/dev/null; then
	export CGO_CFLAGS="$(pkg-config --cflags $_omugen_pc)"
	export CGO_LDFLAGS="$(pkg-config --libs $_omugen_pc)"
fi
# Test binaries load FFmpeg/SDL2 DLLs from lib/ (and on Linux, from the
# FFmpeg build itself, as lib/ fills only during a build).
export PATH="$_omugen_root/lib:$PATH"
export LD_LIBRARY_PATH="$_omugen_root/lib:$_omugen_root/build/ffmpeg/lib:${LD_LIBRARY_PATH:-}"
unset _omugen_pc _omugen_root
