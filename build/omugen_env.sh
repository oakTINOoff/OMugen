# Source this in an MSYS2 MINGW64 shell before running go/make commands:
#   source /d/OMugen/build/omugen_env.sh
# Smart App Control blocks MSYS2's unsigned git.exe, so use the signed Git for Windows.
if [[ -x "/c/Program Files/Git/cmd/git.exe" ]]; then
	export PATH="/c/Program Files/Git/cmd:$PATH"
fi
cd /d/OMugen || return

export CGO_ENABLED=1
export GOEXPERIMENT=arenas

# cgo flags for `go test ./src` / `go vet ./src` (build.sh sets its own).
# FFmpeg is the minimal copy build.sh compiled into build/ffmpeg.
export PKG_CONFIG_PATH="/d/OMugen/build/ffmpeg/lib/pkgconfig:/mingw64/lib/pkgconfig:${PKG_CONFIG_PATH:-}"
_omugen_pc="libavformat libavcodec libavutil libswscale libswresample libavfilter libxmp sdl2"
export CGO_CFLAGS="$(pkg-config --cflags $_omugen_pc)"
export CGO_LDFLAGS="$(pkg-config --libs $_omugen_pc)"
unset _omugen_pc
# Test binaries load FFmpeg/SDL2 DLLs from lib/.
export PATH="/d/OMugen/lib:$PATH"
