# Combined Kotel development build 118

The baseline is [public 1.0.14-preview.117](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.14-preview.117). The combined branch includes the approved [window changes](https://github.com/kolotiloff/agent-messenger/pull/247) and [MCP migration work](https://github.com/kolotiloff/agent-messenger/pull/242). Version 1.0.15-preview.118 identifies the local test builds. Public release signing and distribution are pending.

## Client checks

| Client | Observed | Open |
| --- | --- | --- |
| Codex | A stock 117 server answered `list_agents` from a new app-server thread. After replacing the binary and reconnecting the project, the same thread resumed and the tool call succeeded through `kotel`. The project remained trusted and a named per-tool approval rule survived. The check used the installed CLI and real MCP with local fixture conversation content. | Model-driven steering, permissions during a model turn and live context acceptance. |
| Claude | A fresh isolated CLI connected to the new coordinator registration. Local worker launch tests inspect its actual permission-mode argument. The stream parser uses the last main-model request, drops pre-compaction readings and ignores another model's capacity. | Named grants and a tool call from the same authenticated chat after upgrading. |
| Cursor | A fresh isolated installed CLI with an approved fixture listed coordinator tools on ordinary launch. A marked worker listed none. A separate rename-only experiment with identical transport settings lost the previous approval. Cursor retained the same approval digest but attached it to the server name. An unset marker can reach the server as an unexpanded literal. | Existing approvals under the new server name, a resumed authenticated chat and model-driven task access. The available task result provides token consumption without a measured context capacity. |

Tests exercise project settings collisions, custom commands, disabled entries, reconnecting a Codex profile, capped task access, stale peer software and context invalidation on the sender and executor. Window fixtures exercise remote update progress, matching request cancellation and confirmed installed versions. Both Chromium and WebKit check the grouped footer layout and route clicks from the footer and Network through the Mac update handler.

Two independent Mac clients running build 118 exchanged three direct tasks through a loopback broker. The receiving agent's local policy allowed full access. With remote full access off, a full request launched the Claude fixture in `acceptEdits`. With remote full access on, a standard request still launched in `acceptEdits`, and a full request launched in `bypassPermissions`. Each fixture reported through the real task MCP server and the reply appeared in its original conversation on the sender. These observations establish the arguments received by the launched process. Effective tool permissions inside an authenticated provider turn still require acceptance.

## Platform checks

The native Mac package at revision `2bcf498d9fbe3409ea7133a05652842599d5e007` was locally code-signed. Its bundle contains the Kotel executable and the sibling compatibility link. A stock 117 Mac node paired with the 118 host retained its identity and answered the update request with the native-window instruction. The stock release ZIP matched GitHub's SHA-256. Replacing a complete stock bundle in an isolated installation at its original path preserved the queued request, registered session and prior events. Both backend names opened 118. These checks used independent persistent data directories. The owner's installation and Dock were inspected without installing a development build over them.

The Windows and Linux binaries and tests cross-compile. Signed updates from stock 117, the Windows shortcut and taskbar, the Linux menu and tray, a completed mixed-version remote installation and final Mac installation/Dock continuity still require platform acceptance. The workflow can hold a signed draft for inspection. The original GitHub name still serves the public channel.

## Native Mac window

These captures are from the real AppKit window running local 118 with synthetic tasks. The Network capture shows a live local Mac and a disconnected paired 117 peer with its last-seen time.

[Dark task window](2026-11-27-kotel-window/dark-window.jpg) · [Archive](2026-11-27-kotel-window/archive.jpg) · [My Network](2026-11-27-kotel-window/network.jpg)

The [earlier native Statistics capture](2026-11-27-kotel-window/statistics-hover.jpg) shows six completed fixture tasks. Moving the pointer over the third task highlights that row across all four columns. A separate system WebKit pixel comparison confirmed that keyboard focus also stays inside the row. The current Statistics table also shares the Archive’s 4 px row spacing. Chromium and WebKit measured the gap and compared the rendered hover and focus colors with the Archive and sidebar computers in light and dark themes.

## Preliminary release scope and outstanding work

Under D-258 the owner requested a preliminary release of the completed functionality. Six private native captures of General, Project, Agents and the Network views were presented for interface review. The remaining provider work below carries forward. The signed platform checks still apply to this build.

Task workers still use one-shot CLI launches. Queuing an owner message for the next run works. Acknowledged delivery to the currently running turn under its original message ID (T-344) remains open. Claude advertises JSON streaming input and replayed user-message acknowledgments. Codex provides a turn-steer RPC in app-server mode. The installed Cursor print interface exposes neither streaming input nor user-message acknowledgments. The isolated provider profiles lack authenticated model access. A signed 117 → 118 platform matrix, desktop icon acceptance and the public 118 channel check remain outstanding. Keep the original repository, release keys and legacy entry points throughout the bridge.
