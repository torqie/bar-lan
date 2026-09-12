# Validation record

## Automated/local checks — 2026-09-12

Passed locally: `go test -race ./...`, `go vet ./...`, and Windows amd64 cross-builds for native desktop and CLI. Windows runtime and visual checks remain pending. Tests cover host/client script structure, unsafe TDF values, duplicate names, invalid host addresses, ambiguous engine selection, engine hashing, a real loopback UDP discovery exchange, fake-engine process launch with spaced paths, temporary-script cleanup, and GUI stop/duplicate-launch behavior. CI is configured for Windows and Linux; workflow configuration alone is not evidence of a passing hosted run.

## Windows two-PC test — pending

No Windows PCs or installed BAR engine were available in the development environment. A cross-built Windows binary is not evidence that a BAR match launched successfully.

Record after following the README:

| Check | Result |
| --- | --- |
| Native window, controls, keyboard navigation and DPI layout | Pending |
| GUI discovery, launch errors, stop and close behavior | Pending |
| Windows versions and LAN topology | Pending |
| Engine build on both PCs | Pending |
| Game version / map | Pending |
| Local skirmish on each PC | Pending |
| Broadcast discovery / directed discovery | Pending |
| Discovered join and five-minute match | Pending |
| Manual IP join | Pending |
| Offline WAN test (LAN stays up) | Pending |
| Restart and discovery cleanup | Pending |
| Wrong name / mismatched engine / missing map errors | Pending |

Attach redacted relevant infolog excerpts for failures. Do not attach API keys or full private lobby scripts.
