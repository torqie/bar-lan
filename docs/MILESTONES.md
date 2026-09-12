# Milestone plan

## 1 — Direct two-PC LAN foundation (current)

Delivered: native Windows desktop GUI plus Go CLI, explicit/heuristic installation resolution, two-player host scripts, minimal client scripts, UDP discovery with directed/manual fallback, engine-executable hash comparison, firewall documentation, test/build workflow. No public lobby dependency in the companion.

Acceptance still required: two Windows PCs enter and play the same match for five minutes, repeat with manual IP and WAN disconnected, record engine/game/map versions and errors. Fixed roster, 1v1 only. Engine startup is not readiness. Content discovery and download are manual. These constraints keep the first proof focused.

## 2 — Friendly local session setup (v0.2 implemented; acceptance pending)

Implemented by user request: welcome screen with separate Host/Join windows, automatic newest installed engine, unitsync content dropdowns and minimap previews, guest-chosen names, compatibility fingerprints, pre-game registration/heartbeat, coordinated launch and saved name/data folder. Still pending: installed-BAR acceptance, small-screen/DPI polish, engine socket readiness rather than process-start plus delay, multi-NIC selection and team customization. Pin supported Recoil builds using acceptance evidence.

## 3 — HumanAI local bridge, no LLM yet

Validate the supported Recoil/BAR extension route first. Introduce a loopback-only, authenticated local bridge, observation versioning, team-visible state, and validated strategy intents. Test with a deterministic fake provider. Reject stale/wrong-match plans, unauthorized units and unsupported actions. Translate accepted plans into bounded legal game commands; never execute generated code. Keep nondeterministic external calls outside the synchronized simulation. Gameplay continues if the bridge disconnects.

## 4 — Local BYOK strategy providers

Implement Claude and Gemini adapters behind `humanai.Provider`, with separate local credential storage (Windows Credential Manager preferred), no keys in discovery/scripts/logs/repository, cancellation, timeouts, rate/budget caps and deterministic fallback behavior. Provider-specific prompts and API payloads must not become the game protocol. Add other providers only as needed. Never ask for keys in chat. AI remains optional, and pure LAN play remains independent of Internet connectivity.
