# Building OMugen

OMugen is written in Go with cgo. It links **SDL2** (window, input, controllers),
**FFmpeg** (video backgrounds) and **libxmp** (module music). `build/build.sh` compiles a
small, decoder-only FFmpeg (with libvpx, for WebM alpha) into `build/ffmpeg` the first time,
so you don't need FFmpeg development packages.

`build/omugen_build.sh` is the one command to run. It:

1. builds the runtime libraries into `lib/` (Windows, first build only),
2. generates the art and sounds from code (`tools/screenpackgen`, `tools/templategen`,
   `tools/lifebarsgen`) when their sources changed,
3. packs the engine files into `src/omugen_engine.zip` (`tools/packengine`),
4. builds the drop-in executable with the pack embedded (`-tags=omugen_engine`).

The generated art (`screenpack/omugen/system.sff`, `screenpack/lifebars`,
`screenpack/engine`, Proto's `.sff`/`.air`) and the pack are not in git; the build makes
them.

## Windows (MSYS2)

Install [MSYS2](https://www.msys2.org), open **MSYS2 MINGW64** and install the tools:

```bash
pacman -Syu --noconfirm
pacman -S --noconfirm git make diffutils \
  mingw-w64-x86_64-go mingw-w64-x86_64-toolchain mingw-w64-x86_64-pkg-config \
  mingw-w64-x86_64-nasm mingw-w64-x86_64-tools-git \
  mingw-w64-x86_64-SDL2 mingw-w64-x86_64-libxmp
```

Then, from the repository:

```bash
bash build/omugen_build.sh
```

This writes `OMugen.exe` (about 37 MB, everything inside) and `build/omugen_build.log`.
Copy the exe into a MUGEN folder to try it, or use `tools/make_testgame.ps1` to make a
test folder that links to a game without ever writing to it.

From PowerShell:

```powershell
$env:MSYSTEM="MINGW64"; C:\msys64\usr\bin\bash.exe -lc "bash /c/path/to/OMugen/build/omugen_build.sh"
```

- `OMUGEN_EMBED=0 bash build/omugen_build.sh` makes a quicker development build without
  the pack; it then needs the repository's `external/`, `data/`, `font/` and `lib/` next
  to it.
- `DEBUG_BUILD=1` keeps a console window for logs and panics.
- The repository path must not contain spaces or other characters autotools dislike.

## Linux

Debian/Ubuntu packages (other distributions: the same libraries):

```bash
sudo apt-get install -y build-essential pkg-config yasm nasm \
  libasound2-dev libgl1-mesa-dev xorg-dev libgtk-3-dev libxmp-dev libsdl2-dev
```

and Go 1.27 from <https://go.dev/dl> (the version CI uses; distribution packages are often
older). Then:

```bash
./build/omugen_build.sh Linux
```

This writes `OMugen_Linux` with the engine files embedded. Its libraries stay in `lib/`
next to it (Linux loads them before OMugen starts), so ship both together.

## Tests

```bash
source build/omugen_env.sh     # cgo flags and library paths
go test -vet=off -count=1 ./src ./src/mugenjoy
go vet ./tools/...
gofmt -l src tools             # must print nothing
```

`-vet=off` because some upstream engine code doesn't pass vet. Tests that need a real
game (like `TestLoadRealJumpForceConfig`) skip themselves when it isn't there.

For screens and menus, run the game with `-inputscript file`: it plays scripted key
presses and saves screenshots to `save/shots` (see `src/omugen_inputscript.go`).

## GitHub Actions

`.github/workflows/builds.yml` builds Windows, Linux and Android and runs the tests on
Linux.

- Push a version tag to publish a Release (notes from `docs/releases/<tag>.md`) with
  `OMugen-<version>-windows.zip`, `OMugen-<version>-linux.tar.gz`,
  `OMugen-<version>-android.apk` and checksums: `git tag v0.1.0 && git push origin v0.1.0`.
  The Release waits for all three builds: a failed one publishes nothing.
- Push the `ci` branch (`git push -f origin main:ci`), or start the workflow by hand
  (Actions > builds > Run workflow), for test builds kept a week on the run's page.
- Each build also pushes its log and step results to
  `refs/ci-logs/<windows|linux|android|release>`, readable without a browser:
  `git fetch origin "+refs/ci-logs/*:refs/ci-logs/*"` then
  `git show ci-logs/linux:STATUS` (also `omugen_build.log`, `omugen_test.log`).
- Gradle fetches its own parts from Maven Central and Google; when one of them is down the
  Android build fails with "Could not find" for well-known packages. Run it again later.

It runs on every push to `main` and on pull requests too (those get no secrets: their APK
is signed with the test key).

## Android

`./build/omugen_build.sh Android` builds the engine as `libmain.so` (arm64, Android 8+) with
its files embedded, the libraries it needs (SDL2, FFmpeg, libvpx, libxmp) and the app in
`android/`, and writes `bin/OMugen.apk`. It runs on a Linux or macOS host (on Windows: WSL)
and needs the Android NDK (`ANDROID_NDK_HOME`), the Android SDK (`ANDROID_SDK_ROOT`),
JDK 17 and Gradle 8.7 or newer on the PATH (CI uses 8.10.2). GitHub Actions has all of
these, so the simplest way is the `android` job of `builds.yml` (the APK is in its
downloads). `BUILD_ANDROID_APK=0` stops before the APK; `./build/build.sh AndroidApk` then
makes just the APK (CI does that in a step of its own, the only one that gets the signing
key).

- `android/` is OMugen's app: a game picker (MUGEN folders in Internal storage/OMugen, on USB
  drives and SD cards, or in added folders: `GameLibrary`), the
  settings (on-screen controller or Bluetooth controllers, frame rate) and the on-screen
  DualShock 4 (`TouchPadView`). SDL's own Java files are copied in from SDL's source at
  build time.
- `android/omugen-test.p12` signs the APK, always with the same key, so a new APK installs
  over the old one. With the repository secrets `OMUGEN_KEYSTORE_BASE64` (a PKCS12 key
  store in base64) and `OMUGEN_KEYSTORE_PASSWORD` (optional: `OMUGEN_KEY_ALIAS`,
  `OMUGEN_KEY_PASSWORD`; the key's alias defaults to `omugen`), CI signs with that release
  key instead. Switching keys means uninstalling the old app once (Android refuses an
  update signed with another key), which also clears the app's settings.
- Errors: the engine shows its error messages as Android dialogs and writes this run's log
  (with Go's own crash reports) to `last-run.log` in the app's storage. When a game closed
  without being quit from its menu, the app shows what went wrong on its next start
  (from that log and the app's lines of Android's log), saves it as
  `OMugen/OMugen-error-report.txt` and can share it. Go only sees the environment the
  process started with, so the app passes its folders to `nativeOnSDLReady` instead of
  setting environment variables.

## Other platforms

`build/build.sh` still has the macOS target it inherited (`make OMugen_MacOSARM`), but
OMugen doesn't build or test it yet.
