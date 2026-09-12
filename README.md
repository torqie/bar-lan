# BAR LAN

A native Windows companion for playing Beyond All Reason together on a local network. No public lobby login, browser, or API key is required. BAR and its content must already be installed.

## Get the app

Download the Windows ZIP from [Releases](https://github.com/torqie/bar-lan/releases), extract it on **both PCs**, and double-click **bar-lan.exe**. Use the same BAR LAN release on both PCs; v0.2 rooms are incompatible with the original v0.1 desktop workflow.

## Play together

1. Enter **your name** on the welcome screen. The app remembers it and your BAR data folder.
2. The newest installed engine build is selected automatically. Its version is displayed. **Installation settings** lets you change the detected folder or choose another installed engine if needed.
3. On PC A, choose **Host Game**. In the separate host window, choose an installed **game** and **map** from dropdowns. The latest BAR game is the default. A minimap preview appears when the map supports it.
4. Click **Open room**. The host waits without launching Recoil yet.
5. On PC B, enter that person's own name, choose **Join Game**, and select the discovered room. The app shows the host, engine build, game, and map, including a preview when locally available. Click **Join selected room**.
6. BAR LAN compares the engine executable and game/map checksums. Both windows show that versions match; the host sees the guest's chosen name. Only then does **Start game together** become available on PC A.
7. The host starts the game. Both PCs launch Recoil automatically. Follow its in-game ready/start controls. Exit Recoil normally, or use **Leave room / stop game** to terminate it.

Both players need distinct names. A guest who leaves or loses contact releases the waiting-room slot after at most ten seconds. The host chooses a fixed two-player 1v1 with map-defined starting positions; team customization and AI are future work.

### Finding content

Games, maps, checksums, and previews are read from **unitsync.dll supplied with the selected Recoil engine**. This understands BAR's installed archives and rapid packages; archive filenames do not need to be entered manually. Scanning runs in a separate process, with a timeout, so a DLL failure does not crash the window. Content operations are serialized within the app.

If lists are empty or content cannot be read, open BAR once and download/play a local skirmish with the desired game and map, then close BAR and return to BAR LAN. Check that `unitsync.dll` is beside the selected engine executable. Use a matching 64-bit Windows engine. Content is never downloaded by this app. A missing preview does not prevent hosting an otherwise valid map.

The default engine is selected by natural comparison of its installation directory's version/build numbers (for example build 100 comes after 99), not file modification time. If two PCs have different newest builds, update BAR on both or choose the same installed build under Installation settings. A matching folder label is only preliminary; joining verifies the actual executable hash and content checksums.

## Network and firewall

Use a trusted home LAN and a **Private** Windows network profile. No router port forwarding is needed.

| Traffic on host PC | Port | Purpose |
| --- | --- | --- |
| BAR LAN | UDP 8453 | Discovery, guest registration, heartbeat and launch signal |
| Recoil engine | UDP 8452 | Actual game traffic |

Allow BAR LAN and Recoil when Windows asks about Private-network access. Both PCs require outbound UDP and replies to their OS-selected source ports. Discovery and room control use the same port; there is no HTTP server or TCP listener.

If discovery fails, replace the discovery target in Join Game with PC A's IPv4 address from `ipconfig`, then click **Find games**. A subnet broadcast can also be used. VPNs, guest Wi-Fi/client isolation and separate VLANs can block LAN communication.

Optional host-side rules, from an elevated PowerShell with your actual executable paths:

```powershell
$BarCompanion = 'C:\Tools\bar-lan.exe'
$BarEngine = 'C:\YOUR-BAR-DATA\engine\YOUR-BUILD\spring.exe'
New-NetFirewallRule -DisplayName 'BAR LAN room' -Direction Inbound -Action Allow -Protocol UDP -LocalPort 8453 -Program $BarCompanion -Profile Private -RemoteAddress LocalSubnet
New-NetFirewallRule -DisplayName 'BAR LAN game' -Direction Inbound -Action Allow -Protocol UDP -LocalPort 8452 -Program $BarEngine -Profile Private -RemoteAddress LocalSubnet
```

Update the engine rule when its path changes. Remove these rules with `Remove-NetFirewallRule -DisplayName 'BAR LAN room'` and `Remove-NetFirewallRule -DisplayName 'BAR LAN game'`. The waiting-room token prevents accidental slot reuse; this remains an unencrypted trusted-LAN protocol, not Internet authentication.

## Build and diagnostics

Go 1.24 or newer, on Windows:

```powershell
go test -race ./...
go vet ./...
go build -ldflags "-H windowsgui" -o dist/bar-lan.exe ./cmd/bar-lan
go build -o dist/bar-lan-cli.exe ./cmd/bar-lan
```

The desktop app has no additional GUI runtime dependencies. Source builds need no third-party Go packages. CI tests Linux and Windows and checks the native welcome/host/join windows and their controls on Windows. The optional CLI is useful for diagnostics:

```powershell
.\bar-lan-cli.exe detect
.\bar-lan-cli.exe discover
.\bar-lan-cli.exe discover --target 192.168.1.20
```

Custom/portable data folders can be supplied with `BAR_DATA_DIR` or Installation settings. Settings are stored in the user's application configuration directory under `bar-lan/settings.json` and contain only player name and data folder. The app does not handle API keys yet.

The original direct-launch CLI remains available for troubleshooting. It requires the guest name to be reserved in advance and uses exact game/map values; the **desktop waiting-room workflow removes those requirements from the user interface**:

```powershell
.\bar-lan-cli.exe host --data 'D:\BAR\data' --name Alice --guest Bob --game 'EXACT GAME NAME' --map 'EXACT MAP NAME' --dry-run
.\bar-lan-cli.exe join --data 'D:\BAR\data' --host 192.168.1.20 --name Bob
```

Remove `--dry-run` from the host command to launch. `--engine` can override automatic latest selection. Manual CLI joining skips the GUI's compatibility checks. Use the desktop to join v0.2 waiting rooms.

## Acceptance test and limitations

The two-PC BAR match is still the acceptance gate. Automated checks validate protocol behavior, process handling and native controls; they do not prove the installed BAR build can complete a match.

1. Download the same app release on both PCs and confirm matching installed BAR content.
2. Check automatic engine choice, game/map lists, and previews on the actual install.
3. Open a room, join with another name, confirm both players and compatibility status, and start together.
4. Play five minutes with both commanders responding and no desync.
5. Repeat with directed discovery and with WAN disconnected while retaining LAN connectivity.
6. Test mismatched engine/content rejection, a guest leaving, host restart, and a map without a preview.

Record outcomes in [docs/TESTING.md](docs/TESTING.md). Recoil's `infolog.txt` is in the BAR data folder; preserve relevant excerpts before another launch overwrites it. Existing BAR settings/logs/replays use that same data folder. Close the BAR launcher while using this app. The GUI uses a fixed desktop layout; small-screen/high-DPI visual testing remains pending.

## Architecture / HumanAI

`internal/room` owns pre-game registration and launch coordination; `internal/lan` owns versioned discovery; `internal/content` isolates unitsync; `internal/install` resolves engine builds; `internal/recoil` generates scripts. The native desktop and CLI share process handling. [Milestones](docs/MILESTONES.md) describe the remaining work.

HumanAI remains provider-neutral observation/strategy-plan and bridge interfaces only. Claude/Gemini BYOK adapters, local credential storage, team-visible observations and validated game commands are later milestones. Pure LAN play remains independent of AI or Internet access.

## Primary references

- [Recoil start scripts](https://github.com/beyond-all-reason/RecoilEngine/blob/BAR105/doc/StartScriptFormat.txt)
- [Unitsync API](https://github.com/beyond-all-reason/RecoilEngine/blob/master/tools/unitsync/unitsync_api.h): installed names, checksums and RGB565 minimaps.
- [Recoil data-directory selection](https://github.com/beyond-all-reason/RecoilEngine/blob/master/rts/System/FileSystem/DataDirLocater.cpp): isolated helper environment.
- [BAR launcher paths](https://github.com/beyond-all-reason/spring-launcher/blob/master/src/write_path.js)

Independent companion prototype. No BAR or Recoil code/assets are bundled.
