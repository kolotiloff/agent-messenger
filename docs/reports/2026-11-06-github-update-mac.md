# Public Mac update: preview.12 → preview.13

Checked on November 6, 2026 on the user's Mac, Apple Silicon. Released code: `0f3790c3f650aa568cc883c62923a8c849dbf658`.

## Actual result

- [Preview.13](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.13) was published with three assets: signed metadata, its signature and the archive.
- Before the app check, the main local channel contained build 12. Build 13 was prepared in a separate staging directory and was not copied locally into this channel.
- The check was started from the installed app. A new window process and the installed version `0.0.0-preview.13` with the exact release revision were then observed; the main channel received build 13 from GitHub. The checksum of the downloaded archive matched the signed metadata and the GitHub asset digest.
- All 13 requests, 10 attempts and 9 replies matched before and after the transition. The running background client kept the same PID. There were no new model attempts; model run permission remained off.
- In the installed version, the following were checked: light/dark themes, buttons visible without hovering, provider icons, absence of a duplicate header, and placement of the queue controls inside its tab. The window hides the update button when there is no new version.

## Code checks

[CI for the exact source commit](https://github.com/kolotiloff/agent-messenger/actions/runs/42178423429): all four jobs succeeded — macOS/Linux, Go 1.22.2 and stable. This includes race checks, compilation for target platforms and Mac package tests per the CI matrix. JavaScript passed a syntax check; Swift passed type checking and the build. The transport separately tests corruption, signatures, size/address limits, locks and preservation of the previous channel.

## Boundaries

A real transition through public GitHub followed by installation was verified. The hourly timer was not separately verified here with a full one-hour wait. The user's exact clicks inside the installer window were not logged; the result was accepted based on the installed app's version and the preservation of state.

The background client stayed on the previous version. Its update, migrations and full rollback have not yet been accepted. Windows/Linux GUI, Intel Mac and a clean installation through Gatekeeper on another Mac were not verified. The new right/left message layout and a general re-check of the palette were requested after build 13 and are not included in this release. Private identifiers, databases, keys and session logs are not published.
