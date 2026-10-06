<p align="center">
  <img src="external/icons/omugen_logo.png" width="600" alt="OMugen"/>
</p>

**OMugen is a 64-bit fighting game engine that plays M.U.G.E.N 1.1 games exactly as they
are.** Drop `OMugen.exe` into any MUGEN folder and play: same characters, stages,
screenpack and controls, nothing to convert or edit, plus online/LAN play, a built-in
screenpack for any roster size, and tools to find what's broken in a game.

## What OMugen adds

| | Stock Ikemen GO | OMugen |
|---|---|---|
| Running a MUGEN game | Copy its engine files in, set up a new config | Drop in one `OMugen.exe`; engine files are inside it |
| Controls | Its own `config.ini` | `data/mugen.cfg`, with MUGEN 1.1's exact keyboard and joystick codes |
| Changing controls / video in Options | Written to its own config | Written back to `mugen.cfg`, like MUGEN |
| MUGEN screenpacks | Font art, grids and glyphs can render wrong | MUGEN 1.1 behavior (per-glyph palettes, `cell.spacing`, old `.fnt` banks) |
| Loading a 1,580-character roster | Minutes before the menu (6 min 28 s measured) | Menu in about 2.5 s; every portrait ready 1.6 s later (from the second start, thanks to a cache of the sprite-file headers in `save/cache`). Right after a reboot or an antivirus update, the roster's files are read in parallel (8x faster than one by one) |
| A game's screenpack is broken or missing | The game closes | Falls back to the OMugen screenpack and says so |
| Rosters of any size | Fixed grid from the screenpack | OMugen screenpack sizes and scrolls its grid to the roster |
| Checking a whole game | — | `OMugen.exe -doctor`: loads every character and stage, writes a report |
| Content MUGEN tolerates | Some characters refused (bad statedef types, extra values, Windows-1252 file names) | Loaded with a warning, as in MUGEN |
| High refresh rate monitors | Frame rate only in the config file; above 60 fps the camera moves faster and can change gameplay | Options > Video > Frame Rate (up to 240, or the monitor's rate): the game still runs at 60 ticks per second and plays identically (checked tick by tick at 60, 144 and 240 fps), with the characters, camera and stage scrolling drawn smoothly between ticks |
| Effects | — | Optional glow (bloom) on additive effects |
| Making your own game | Start from an empty folder | `OMugen.exe -newgame "My Game"`: a playable game with a template fighter and stage, all generated from code |

## Using it
Copy `OMugen.exe` into a MUGEN 1.1 folder (next to `data`, `chars`, `stages`) and run it.
The engine's own files (scripts, shaders, fonts, engine data, runtime DLLs and the OMugen
screenpack) are inside the exe and unpack once to `%LOCALAPPDATA%\OMugen\engine\<version>\`,
never into the game folder. OMugen only adds a `save` folder (settings, logs, replays,
reports). Files in the game folder always take priority over the engine's copies.

- **Controls** come straight from `mugen.cfg`: keyboard keys (SDL 1.2 key codes) and
  joystick codes decoded from MUGEN 1.1 itself, read through the same Windows API MUGEN
  uses, so every pad button matches. Options > Input Settings shows and edits them in
  MUGEN's own notation (`A-17`...). The pad's Start/Options button opens the pause menu.
- **Controllers that just work:** controllers are read through SDL (DualShock 4 and DualSense,
  also over Bluetooth, Xbox pads, DS4Windows), laid out the way MUGEN numbers them so
  `mugen.cfg` codes keep their meaning (`OMUGEN_JOY=winmm` reads WinMM like MUGEN).
  Players get the controllers that actually send input,
  in the order they are first pressed (up to 4); a player keeps the controller their own
  codes name when it sends input. Controllers Windows lists but that send nothing, and
  copies that only mirror another pad (DS4Windows), are skipped. Players 2-4 without
  joystick codes of their own (a fresh `mugen.cfg` has none for P2, and MUGEN has no P3-P4)
  play with player 1's layout. Options > Input Settings > Reassign Controllers hands them
  out again.
- **Controller indicator:** in menus, P1-P4 tags in the corner show who has a controller
  (lit while pressing), a notice appears when a controller joins, and with several players
  the highlighted menu item and the tags show who is selecting.
- **Screenpack, resolution, window size and fullscreen** come from `mugen.cfg`; changes
  made in Options are saved back to it (a backup goes to `save/backups` first).
- **OMugen screenpack:** Options > Video Settings > OMugen Screenpack (or `-omugenui`).
  A 1920x1080 interface drawn entirely by OMugen's own generator. It uses the game's
  characters, stages, lifebars, sounds and music, sizes the select grid to the roster
  (scrolling when needed), and fits portraits of any size. It also takes over
  automatically when a game's own screenpack or lifebars fail to load.
- **OMugen Doctor:** `OMugen.exe -doctor` checks every character and stage in
  `select.def` the way a match loads them and writes `save/doctor/report.html`
  (errors, crashes, missing sprites and warnings, grouped by how common they are).
  Options: `-doctor chars|stages`, `-doctorlimit N`, `-doctorfilter name1,name2`.
- **Glow:** Options > Video Settings > Glow Effects (off by default, like MUGEN).
- **Frame rate:** Options > Video Settings > Frame Rate: 60 (default, like MUGEN), 120, 144, 165, 240
  or Monitor. Game logic always runs at 60 ticks per second; `OMUGEN_TICKLOG=file` logs the
  game state of every tick to check that a match plays the same at any frame rate.

## Making your own game
`OMugen.exe -newgame "My Game"` creates a folder with everything a new fighting game needs,
ready to play and to change:

- **Proto**, a complete template fighter: 6 palettes, normals on the ground and in the air,
  a projectile (down, down-forward, forward + punch), a super beam (the motion twice +
  punch) and every standard animation the common states use, with collision boxes.
- **Proto Arena**, a parallax night-dojo stage in 1280x720.
- An editable copy of the OMugen screenpack, `data/select.def`, and the game's name as
  its window title.
- `GUIDE.txt`: how the files fit together and how to make characters, stages and menus.

Everything uses the MUGEN 1.1 formats, so MUGEN tools and tutorials apply. Proto and the
stage are drawn by `tools/templategen` (no third-party art); the template itself is in
`templates/newgame`.

## Tools
- `tools/screenpackgen`: generates the OMugen screenpack's art from code.
- `tools/templategen`: draws Proto and Proto Arena and writes their `.sff` and `.air`.
- `tools/logogen`: draws the OMugen logo and icons.
- `tools/mugenjoytest`: shows the MUGEN joystick code and config-screen label of every
  button you press. Use `-raw` to see what Windows reports.
- `tools/make_testgame.ps1`: builds a zero-copy test folder from a MUGEN game
  (junctions and hard links, never writes to the game).
- `-inputscript file`: plays scripted key presses and screenshots, for automated
  screen tests.

## Building
Windows (MSYS2 MINGW64): see [BUILDING.md](./BUILDING.md), then run
`build/omugen_build.sh Win64`. It builds the runtime DLLs into `lib/`, generates the
screenpack art, packs the engine files with `tools/packengine` into
`src/omugen_engine.zip`, and builds the drop-in `OMugen.exe` with `-tags=omugen_engine`.
`OMUGEN_EMBED=0` makes a quicker development build that uses the repository's
`external/`, `data/`, `font/` and `lib/` instead. `build/omugen_env.sh` sets up the
environment for `go test` / `go vet`.

Linux: `build/omugen_build.sh Linux` builds `OMugen_Linux`, with the engine files
embedded as on Windows. Its libraries stay in `lib/` next to it, because Linux loads
them before OMugen starts.

GitHub Actions (`.github/workflows/builds.yml`) builds both. Pushing a version tag
(`git tag v0.1.0 && git push origin v0.1.0`) makes a Release with
`OMugen-<version>-windows.zip` and `OMugen-<version>-linux.tar.gz`. Pushing the `ci`
branch, or starting the workflow by hand, gives the same builds as downloads on the
run's page, kept for a week.

## Credits
OMugen's engine core comes from [Ikemen GO](https://github.com/ikemen-engine/Ikemen-GO)
by the Ikemen GO team and contributors (itself a rewrite of Suehiro's Ikemen); OMugen
builds its MUGEN drop-in compatibility, tools and interface on top of it.
M.U.G.E.N is by Elecbyte; OMugen is not affiliated with Elecbyte and contains none of
its code.

## License
MIT License; see [LICENCE.txt](LICENCE.txt), which keeps the Ikemen GO copyright notices
as the license requires. Bundled assets keep their own licenses: Open Sans (SIL OFL) and
the fallback lifebars from the Ikemen GO screenpack (CC BY 3.0). The OMugen screenpack's
art is generated by OMugen itself.

This program dynamically links FFmpeg (LGPL v2.1). You may rebuild it against a modified
FFmpeg; `build/build.sh` builds the exact FFmpeg configuration from source.
