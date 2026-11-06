# Mac upgrade preview.13 → preview.14

November 6, 2026, the user's Apple Silicon Mac. Source commit `c16db2510b111b64248c424e9954642d57c97045`.

The [release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.14) contains an archive and signed metadata. The package was prepared in a separate staging directory; it was not copied into the live channel manually. In the running app, the badge for available version 14 was observed, followed by installed version 14 and the new interface elements. The sequence of clicks inside the installer was not fully logged, because the owner was also working with the window.

Checksum of the received archive: `e0e6e3f6c87de8da1f12461e6438f5db8c80d127b47808bae9ca4b226d193e8a`. Before and after, all 13 messages, 10 attempts and 9 replies matched. The background client PID did not change. Running models remained forbidden.

[All four CI jobs](https://github.com/kolotiloff/agent-messenger/actions/runs/42182147541) passed: Mac/Linux, Go 1.22.2/stable. Locally, the UI race tests and the Swift build passed. On demo data, both themes, queue collapsing, scrolling and the fixed input field were checked. The installed window shows the version, three tabs, the queue toggle and the «Вид» ("View") menu; the old data-refresh icon is hidden.

The release does not update an already running background client and does not accept the Windows/Linux GUI. After publishing, more frequent checks for new versions and an extra margin for the scrollbar were requested; these changes belong to the next release.
