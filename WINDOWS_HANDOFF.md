# BAR LAN — Windows development handoff

Updated 2026-09-12. Read this file and README.md before continuing. This is a curated project handoff, not an import of the previous chat history.

## Current objective

Continue development on the user's Windows PC, where BAR is installed. First validate and fix the actual native app, installed-content lists/previews, and a two-PC LAN match. Do not start HumanAI implementation yet.

Repository: https://github.com/torqie/bar-lan
Branch: main
Latest app release at handoff: v0.2.0
Release: https://github.com/torqie/bar-lan/releases/tag/v0.2.0
App source at that release: 4e105952c6085a676bda2a7dd7970f1d6a4a4afc
Passing CI: https://github.com/torqie/bar-lan/actions/runs/34718014787

The handoff itself is a later documentation-only commit. Pull main before working. The previous development checkout was clean and all application changes were pushed. Do not assume any previous machine's tools, paths, or local knowledge files exist here.

## User context and decisions

- Two Windows PCs on the same LAN, with BAR installed on both.
- Go is approved. The app must launch directly into a native desktop window, not a browser or local website.
- User confirmed that automatic BAR data-folder selection works on their machine.
- Choose the newest installed engine automatically. Display the engine version and make agreement between PCs clear.
- Ask each person only for their own player name. The guest's name arrives through room registration; the host must not type it.
- Games and maps must be dropdowns populated from installed content, not exact-name text inputs.
- Include map previews when available.
- Welcome window: Your name, Host Game, Join Game. Host and Join open separate native windows.
- Make basic LAN gameplay work without the public BAR lobby or an Internet connection after content is downloaded.
- Later HumanAI: optional local bridge, BYOK, Claude/Gemini first, provider-neutral game observations and strategy plans. Never request API keys in chat or commit secrets.

## Implemented in v0.2

- Standard Win32 controls called from Go; no browser, WebView, third-party GUI framework, or external Go modules.
- Automatic install detection and natural numeric ordering of engine-directory version/build numbers. An advanced Installation settings window permits an override.
- Player name/data folder preferences saved under the user's configuration directory in bar-lan/settings.json.
- Installed game/map enumeration, checksums, and RGB565 minimaps through the chosen engine's unitsync.dll. Calls run in a disposable helper process with a timeout; content operations are serialized within the app.
- Host opens a waiting room before starting Recoil. Guest discovers it and registers their own name. Engine SHA-256 and game/map checksums must match.
- Host's Start game together is enabled after guest registration. The generated host script contains both names. Both PCs launch Recoil after the host starts.
- UDP discovery plus directed-IP discovery fallback; room/registration/heartbeat/launch messages use UDP 8453. Recoil uses UDP 8452. No TCP/HTTP listener.
- Guest slot expires after ten seconds of missing heartbeats while waiting. Fixed two-player 1v1; Armada/Cortex and map-defined starting positions.
- CLI retained as bar-lan-cli.exe for diagnostics. Its original direct host/join flow still requires explicit names/content; use the GUI for v0.2 rooms.

## What is actually verified

Passed on the local development machine and/or the linked CI run:

- Go tests, including race checks; Go static checks; Windows and Linux builds.
- Real loopback UDP discovery and room exchange; guest names, duplicate slots, heartbeat/leave/expiry, compatibility checks, and start gating.
- Fake-engine launch, temporary-script cleanup, process cancellation and duplicate-launch prevention.
- Natural engine ordering and RGB565-to-BMP pixel conversion.
- On a real GitHub Windows runner: opening the welcome window, entering a name, navigating Host/Join, finding the dropdown/list/preview controls, returning, and closing.

**Not yet verified:** unitsync against the user's actual BAR installation; real dropdown contents; minimap rendering; visual layout/high-DPI behavior; successful real Recoil hosting/joining; a complete two-PC BAR match. Control-level CI checks do not establish any of these. Do not claim a match worked until it has been played.

## Start here on Windows

1. Read README.md and inspect git status. Preserve any new local changes.
2. Detect available Git/Go tools and the BAR data/engine location. Do not ask the user to retype paths the machine can supply. Go 1.24+ is required; CI uses 1.26.1.
3. Build the current checkout, then launch the native app. Close the BAR launcher before testing against its data folder.
4. Confirm the selected engine, enumerate actual games/maps, and inspect a selected map preview. Investigate the first actual failure before redesigning the app again.
5. Work with the user to open a room on PC A and join from PC B. Verify own-name registration, matching status, start coordination, both commanders, and five minutes without desync.
6. Record exact engine/game/map versions and actual results in docs/TESTING.md. Repeat directed discovery and WAN-off/LAN-on tests when the basic match works.

Build from the repository in PowerShell:

```powershell
go test ./...
go vet ./...
go build -ldflags "-H windowsgui" -o dist/bar-lan.exe ./cmd/bar-lan
go build -o dist/bar-lan-cli.exe ./cmd/bar-lan
.\dist\bar-lan.exe
```

Run `go test -race ./...` when the required Windows C compiler is available; a missing race-toolchain dependency is not a passing test. The repository's scripts/windows-ui-smoke.ps1 is intended for GitHub CI and expects RUNNER_TEMP; use it there or adapt its temporary-directory setup explicitly for local use.

## Troubleshooting and known limits

- Native helper: unitsync.dll must be beside the selected 64-bit engine. The helper working directory is the engine directory; SPRING_ISOLATED, SPRING_WRITEDIR and SPRING_DATADIR point to BAR data. Missing/incomplete game/map archives or DLL dependencies remain possible real-install issues.
- Install/content scanning and hashing use the selected local engine, not executable paths supplied by network peers.
- Actual launch coordination currently uses an explicit engine-process-start event and a two-second guest delay. This is not a verified engine-socket readiness handshake. Investigate loading/connect timing if the first join fails.
- The GUI is a fixed desktop layout. Small screens, scaling, keyboard interaction and visual polish need local testing.
- No automatic content downloads, team customization, game-state bridge or AI providers yet. Missing content should be downloaded using BAR.
- Do not run BAR's launcher concurrently against the same data folder. Existing settings/logs/replays live there; preserve relevant infolog.txt excerpts before another launch overwrites them.
- Windows firewall: allow companion UDP 8453 and engine UDP 8452 on Private networks. No router port forwarding. If broadcast fails, enter the host IPv4 in Join Game and Find games again. Guest Wi-Fi isolation and VPNs can interfere.
- Native startup trace is optional: set BAR_LAN_UI_TRACE to a local file path before launching. It records startup stages, not screenshots.
- CI's original window-lookup failure was a test interop bug, already fixed: passing PowerShell `$null` to a C# string parameter did not produce the desired native null class pointer. FindWindow now takes IntPtr.Zero. The app itself opened correctly.

## Code map

- cmd/bar-lan/gui_windows.go — native windows, controls, previews, preferences.
- cmd/bar-lan/lobby.go — desktop room lifecycle and launch coordination.
- cmd/bar-lan/gui_session.go — shared cancellable game process state.
- cmd/bar-lan/main.go — CLI, helper dispatch, engine process launch.
- internal/install — data detection, engine selection/version ordering, hash.
- internal/content — unitsync helper and BMP conversion.
- internal/room — pre-game UDP protocol and guest lease.
- internal/lan — discovery protocol (v1 CLI and v2 rooms).
- internal/recoil — validated TDF start-script generation.
- internal/humanai — skeleton interfaces only.
- scripts/windows-ui-smoke.ps1 and .github/workflows/build.yml — Windows native control checks and builds.

Continue with concrete Windows verification and targeted fixes. Preserve the user-friendly native flow. Publish the next tested Windows ZIP to GitHub Releases when a useful fix is ready, including a clear statement of what was actually verified.
