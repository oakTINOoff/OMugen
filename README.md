<p align="center">
  <img src="external/icons/omugen_logo.png" width="600" alt="OMugen"/>
</p>

OMugen is a 64-bit 2D fighting game engine that runs M.U.G.E.N 1.1 games as they are.
Drop `OMugen.exe` into an existing MUGEN folder and play: same characters, stages,
screenpack and controls, without converting or editing a single file.

OMugen is built on the open source [Ikemen GO](https://github.com/ikemen-engine/Ikemen-GO)
engine (MIT) and keeps its extra features, including online and LAN netplay.

## Goals
- **Drop-in MUGEN 1.1 compatibility.** Read `data/mugen.cfg`, screenpacks and `select.def`
  exactly the way MUGEN 1.1 does, including its quirks, when a game was made for MUGEN.
- **Fast on huge rosters.** Menus in seconds, characters loaded in the background.
- **64-bit, modern renderers,** and no 2/4 GB memory ceiling.

## What works today
- Controls come straight from `mugen.cfg`: keyboard keys (SDL 1.2 key codes) and joystick
  codes decoded from MUGEN 1.1 itself, read through the same Windows API MUGEN uses, so
  every pad button matches. The pad's own Start/Options button opens the pause menu.
- Screenpack, game resolution and window size are read from `mugen.cfg`.
- MUGEN 1.1 screenpack behavior: per-glyph font palettes (menu art stored in fonts),
  single-value `cell.spacing`, old multi-bank `.fnt` fonts.

## Tools
- `tools/mugenjoytest`: shows the MUGEN joystick code and config-screen label of every
  button you press. Use `-raw` to see what Windows reports.
- `tools/make_testgame.ps1`: builds a zero-copy test folder from a MUGEN game
  (junctions and hard links, never writes to the game).
- `tools/logogen`: draws the OMugen logo and icons.

## Building
Windows (MSYS2 MINGW64): see [BUILDING.md](./BUILDING.md), then run
`build/omugen_build.sh Win64`, which produces `OMugen.exe` and its `lib/` folder.
`build/omugen_env.sh` sets up the environment for `go test` / `go vet`.

## Credits
OMugen is a fork of [Ikemen GO](https://github.com/ikemen-engine/Ikemen-GO) by the
Ikemen GO team and contributors, itself a rewrite of Suehiro's Ikemen. Thank you.
M.U.G.E.N is by Elecbyte; OMugen is not affiliated with Elecbyte.

## License
The engine is under the MIT License; see [LICENCE.txt](LICENCE.txt), which keeps the
Ikemen GO copyright notices as the license requires. Assets bundled from other projects
keep their own licenses (Creative Commons for the Ikemen GO screenpack assets).

This program dynamically links FFmpeg (LGPL v2.1). You may rebuild it against a modified
FFmpeg; `build/build.sh` builds the exact FFmpeg configuration from source.
