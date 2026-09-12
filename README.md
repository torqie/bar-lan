# BAR LAN prototype

A small, dependency-free Go companion for **two Windows PCs with BAR already installed**. It launches the installed Recoil engine directly, without logging into or contacting the public BAR lobby. It does not download or bundle BAR, its assets, or Recoil.

**Status:** initial prototype. Script generation and LAN discovery have automated tests; a real Windows-to-Windows BAR match remains the acceptance gate. This is a fixed two-player 1v1 (Armada vs Cortex, map-defined starting positions), not a full lobby. Host reserves the guest name before launching. No HumanAI/provider calls are implemented.

## Build

Install Go 1.24 or newer, then in PowerShell:

```powershell
git clone git@github.com:torqie/bar-lan.git
cd bar-lan
go test ./...
go build -ldflags "-H windowsgui" -o dist/bar-lan.exe ./cmd/bar-lan
go build -o dist/bar-lan-cli.exe ./cmd/bar-lan
```

Copy `dist/bar-lan.exe` to both PCs and double-click it to open the native Windows app. Copy `dist/bar-lan-cli.exe` too if you want command-line diagnostics. No Go installation is needed to run the executable. GitHub Actions also builds Windows artifacts when enabled for this repository.

## Desktop app

Double-click **bar-lan.exe**. This is a native Windows window built with standard Windows controls; it needs no browser, local web server, WebView, or additional GUI runtime.

1. Choose the detected BAR data folder, or paste your custom data path and click **Detect / refresh**. Select the engine executable from the dropdown (or paste its full path). Multiple engines require an explicit choice.
2. On PC A, fill in the host/guest names and exact installed game/map values described below, then click **Host game**.
3. On PC B, choose its local install/engine, click **Find games**, select PC A and click **Join selected game**. The companion checks the engine hash and uses the reserved guest name.
4. If discovery fails, enter PC A's IP as the discovery target, or enter its IP and the reserved name under **Join by IP**. Manual joining uses the game port field and skips engine hash verification.
5. Launch errors and engine output appear in the log. Readiness and match start happen in Recoil. Exit Recoil normally, or click **Stop game** to terminate it. Close BAR LAN after the game stops.

The window remains responsive during discovery and gameplay. Current limits: fixed desktop layout, no saved preferences, no content picker/download, and no visual Windows QA performed yet. Keyboard Tab navigation uses standard controls. The command examples below use the separate CLI build.

## Prepare both PCs

1. Use BAR normally once to download **the same engine build, exact game version, and map** on both PCs. Launch that map in a local skirmish on each PC to confirm content is present. Then exit BAR and its launcher. BAR being installed alone does not guarantee map/game content is cached.
2. Identify the BAR **data** folder and engine executable:

   ```powershell
   .\bar-lan-cli.exe detect
   .\bar-lan-cli.exe detect --data 'D:\Games\Beyond-All-Reason\data'
   ```

   Common installs are under `%LOCALAPPDATA%\Programs\Beyond-All-Reason\data` or `%ProgramFiles%\Beyond-All-Reason\data`. Portable/custom installs need `--data`. `BAR_DATA_DIR` is also supported. Detection lists engine candidates; if several exist, select one explicitly rather than assuming the newest folder is correct.
3. Get the **exact `GameType` and `MapName`** from the `[GAME]` section of the start script produced by that working BAR skirmish (commonly `script.txt` in the data directory; launch logs may identify a different script path). Copy only those values, without their trailing semicolons. Keep any private information in the original script local. Use a pinned game name/version or archive name, not a moving rapid tag. Map names include `.smf`. Both PCs must have matching content; this prototype does not inventory archives or calculate game/map checksums. Recoil performs the actual content/sync checks.
4. Set PowerShell variables on each PC (replace these example paths):

   ```powershell
   $BarData = 'C:\Users\YOU\AppData\Local\Programs\Beyond-All-Reason\data'
   $BarEngine = Join-Path $BarData 'engine\YOUR-ENGINE-BUILD\spring.exe'
   ```

## Host on PC A

Replace game/map placeholders with the values from the working local skirmish:

```powershell
.\bar-lan-cli.exe host --data $BarData --engine $BarEngine --name Alice --guest Bob --game 'EXACT GAME NAME AND VERSION' --map 'EXACT MAP NAME.smf' --dry-run
.\bar-lan-cli.exe host --data $BarData --engine $BarEngine --name Alice --guest Bob --game 'EXACT GAME NAME AND VERSION' --map 'EXACT MAP NAME.smf'
```

The first command previews the start script without launching or opening a listener. The second starts Recoil and discovery. Keep the terminal open. Recoil waits for the reserved player `Bob`; follow the in-game ready/start controls after both players connect. Avoid forcing a start before the guest connects.

## Discover and join on PC B

```powershell
.\bar-lan-cli.exe discover
.\bar-lan-cli.exe join --data $BarData --engine $BarEngine
```

With exactly one discovered host, `join` uses the reserved guest name and compares SHA-256 hashes of the engine executables. It prints the host's game and map requirements. No executable paths or scripts received from the network are run. Hash matching checks the executable only, not accompanying DLLs/content.

If there are multiple hosts, or broadcast fails, find PC A's IPv4 address with `ipconfig` and use directed discovery:

```powershell
.\bar-lan-cli.exe discover --target 192.168.1.20
.\bar-lan-cli.exe join --target 192.168.1.20 --data $BarData --engine $BarEngine
```

`--target` also accepts a subnet broadcast address such as `192.168.1.255` when appropriate for your subnet. VPNs and multiple network adapters can route the default broadcast incorrectly. The protocol is IPv4 UDP request/reply, with a three-second default discovery window (`--timeout 5s`).

Manual joining works even with discovery blocked:

```powershell
.\bar-lan-cli.exe join --host 192.168.1.20 --port 8452 --name Bob --data $BarData --engine $BarEngine
```

Manual joining skips the engine hash comparison and requires the exact guest name reserved by PC A. Verify versions yourself. Host and manual join must both use the same `--port` if changed. One advertised host per PC is supported (fixed discovery port 8453). Advertisements last until the Recoil process exits, including during a match; they do not mean a fresh player slot is available. Joining after match start is outside this milestone.

## Windows firewall and network

Use a trusted home LAN with the Windows network profile set to **Private**. No router port forwarding, Internet lobby account, or API key is required for this launch flow. Initial BAR content downloads may require Internet access.

| Traffic | Default | Requirement |
| --- | --- | --- |
| Recoil game | UDP 8452 on PC A | Allow inbound to the selected engine on host; PC B uses an OS-selected UDP source port |
| Discovery | UDP 8453 on PC A | Allow inbound to bar-lan.exe; it replies to the client's ephemeral port |
| Outbound | UDP | Allow companion/engine outbound and stateful replies on both PCs |

When Windows prompts, allow the applications on **Private networks**. If necessary, run these in an elevated PowerShell on PC A after defining `$BarEngine` there and replacing the companion path:

```powershell
New-NetFirewallRule -DisplayName 'BAR LAN discovery' -Direction Inbound -Action Allow -Protocol UDP -LocalPort 8453 -Program 'C:\Tools\bar-lan.exe' -Profile Private -RemoteAddress LocalSubnet
New-NetFirewallRule -DisplayName 'BAR LAN game' -Direction Inbound -Action Allow -Protocol UDP -LocalPort 8452 -Program $BarEngine -Profile Private -RemoteAddress LocalSubnet
```

Update the game rule if the engine path or game port changes. Remove these optional rules with `Remove-NetFirewallRule -DisplayName 'BAR LAN discovery'` and `Remove-NetFirewallRule -DisplayName 'BAR LAN game'`. Managed firewalls may also require explicit outbound rules. Guest Wi-Fi/client isolation or separate VLANs can prevent all direct connections. Discovery is unauthenticated and intended for a trusted LAN; player names are not access-control credentials.

## Two-PC acceptance test

1. Complete the local skirmish/content checks on both PCs. Record engine folder, game version, map, and the two LAN IPs.
2. Preview host/client scripts with `--dry-run`; ensure names, IP, port and content match.
3. Start PC A, discover it on PC B, then join before starting gameplay. Confirm both players reach the same map, have their own commanders, can issue orders, and see each other's movement. Play at least five minutes without a desync.
4. Repeat using manual `--host` joining. For an offline check, disconnect the WAN while preserving the LAN after all content has been downloaded, then repeat.
5. Exit Recoil and check that discovery stops. Reopen the host and repeat. Ctrl+C terminates the launched engine; exit through Recoil for normal cleanup.
6. Check wrong-name/manual join rejection, mismatched-engine discovery rejection, and a missing-map failure. Record actual results in `docs/TESTING.md`.

On failure, preserve the command (without secrets) and the end of `data\infolog.txt` before launching BAR again; it can be overwritten. A temporary start script is removed when the companion exits normally. Logs/settings/replays still go to your existing BAR data folder. Do not run the launcher and this companion concurrently against the same data directory.

**Troubleshooting:** no discovery → try directed discovery/manual IP and firewall checks; unauthorized player → use the reserved guest name exactly; missing archive/map or sync error → run the identical game/map locally on both PCs and align versions; multiple engine candidates → explicitly select `--engine`; UDP bind error → close another host process. Discovery starts when the engine process starts, not when it is fully loaded, so wait for PC A's loading screen to finish if an early join fails.

## Architecture and next milestones

`cmd/bar-lan` handles CLI/process lifetime; `internal/lan` owns versioned discovery; `internal/recoil` generates validated TDF scripts; `internal/install` resolves local paths. The only runtime dependency is the locally installed engine. Processes are launched with separate arguments, without a shell. There is no HTTP server, public lobby connection, or provider SDK.

See [milestones](docs/MILESTONES.md) and [test record](docs/TESTING.md). `internal/humanai` contains only a provider-neutral observation/strategy-plan contract and bridge/provider interfaces. Claude and Gemini are the first planned adapters, with local BYOK configuration later. There is no secret handling or AI game control yet.

## Upstream references

Implementation checked against these primary sources on 2026-09-12; actual compatibility must still be established against the users' installed build:

- [Recoil start-script format](https://github.com/beyond-all-reason/RecoilEngine/blob/BAR105/doc/StartScriptFormat.txt): host/client fields, reserved players, teams, and default port.
- [BAR launcher data-path resolution](https://github.com/beyond-all-reason/spring-launcher/blob/master/src/write_path.js) and [BAR direct engine launch example](https://github.com/beyond-all-reason/BYAR-Chobby/issues/643): data directory and isolation/write-dir arguments.
- [BAR faction definitions](https://github.com/beyond-all-reason/Beyond-All-Reason/blob/master/gamedata/sidedata.lua): Armada/Cortex side names.

This is an independent companion prototype, not an official BAR release. No upstream game assets are redistributed.
