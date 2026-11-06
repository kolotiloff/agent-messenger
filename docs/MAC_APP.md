# Agent Messenger.app

The first native shell for Mac: AppKit, the system WKWebView and an embedded Go client. A separate browser, Python, Node, Go or a terminal is not needed for normal opening. The chosen design/v1 styling is preserved. This is a local preview, not a finished network release.

## Installation and opening

1. Download **[Mac preview.19 · Apple Silicon](https://github.com/kolotiloff/agent-messenger/releases/download/v0.0.0-preview.19/4584caecdbb280d861d2d06c812c19c08cde385781b3d632ab7f863339242afa.zip)**. [Release notes and files](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.19).
2. Unpack the ZIP by double-clicking it. The archive's long name is its checksum; inside is `Agent Messenger.app`.
3. Move `Agent Messenger.app` to the user folder `~/Applications` («Программы», "Applications"). Open it by double-clicking or via Spotlight/Dock.

Later versions are checked automatically at launch, every 5 minutes and when you return to the window (with an overall interval of no less than 5 minutes). **«Доступно обновление»** ("Update available") appears at the bottom left; the manual check is in the menu **Agent Messenger → Проверить обновления…** ("Check for Updates…"). No GitHub sign-in is needed.

 The package currently has an ad-hoc signature: clean distribution through Gatekeeper, Developer ID and notarization is not accepted. Bypassing macOS protections is not required and not suggested. Verified locally on Apple Silicon; the minimum version for the shell is macOS 15, Intel is not accepted separately.

On opening, the shell finds the previous database via the existing protective registry, or uses the normal user directory when there are no reservations. Several different directories, a corrupted registry or a missing saved database lead to a refusal. Reservations are not deleted or restored automatically. If the running client uses a different directory without recorded reservations, choose it via the menu «Файл → Выбрать каталог данных…» ("File → Choose Data Directory…"); there is no search for a "last session".

The application connects to an already running client. If there is no client, it starts one background instance, paused and without model permissions. The global lock protects this path too. Opening an already running client does not change its pause or previously granted permissions. The window itself does not invoke a CLI and does not sign in on the user's behalf.

The red button closes the window, but the application stays in the Dock; «Файл → Открыть окно» ("File → Open Window") or reopening the application brings it back. Cmd+Q closes the shell and its local HTTP process. The background client, the queue and the CLI owners remain. The application does not call shutdown on close. Its own UI process lives while the inherited stdin channel is open; if the shell crashes, EOF closes the HTTP server. There is no automatic restart of models.

## Interface boundaries

The embedded WebView receives only the random private loopback address of its UI process. It does not open external sites, new windows, file URLs or arbitrary navigations. WebView storage is non-persistent; after a full quit, unsent text may disappear. Outgoing messages already written are restored from SQLite under the existing rules. Additional saving of unsent drafts is not enabled yet; updates are installed on a button press.

Existing window functions are messages, history, cards, sending, pause, CLI check and creating a separate test Claude session. In preview.19 it can be explicitly launched from the session list; the permission is limited to one conversation. A background client of version 19 is required. CLI login, the broker, four computers, safe release of old owners and Developer ID/notarization remain separate stages.

## Build and checks

From verified sources on a Mac with stable Go and Apple Command Line Tools:

```sh
bash scripts/build-mac-app.sh /absolute/new/dir 0.0.0-preview.3 FULL_SHA
python3 scripts/check-desktop.py '/absolute/new/dir/Agent Messenger.app/Contents/Resources/messenger'
```

The builder refuses to overwrite an existing app/zip, builds Go with CGO_ENABLED=0, compiles Swift, creates the icon, signs its own bundle ad-hoc, verifies the signature, prints version and creates a zip with SHA-256. SOURCE_SHA is set by the caller; a release requires checking that the source tree is clean.

An automated test on a separate HOME verifies launching without models, two UIs connecting to one client, EOF/reopening without losing the queue, and the absence of model attempts. Go tests separately verify directory selection, that reservations stay unchanged, and refusal of model flags. CI builds the bundle and checks the bridge, but this is not presented as a visual test of the WebView.

On the user's Mac the window opened natively: history and cards are available; closing the window and returning via the menu were verified. The full Go suite with race passed. There are no real models in the tests.

Foundations of the shell: [NSApplication](https://developer.apple.com/documentation/appkit/nsapplication), [WKWebView](https://developer.apple.com/documentation/webkit/wkwebview). The Messenger core remains in Go; Swift is only the macOS platform shell.

## Updates from the window

[Signed updates via GitHub Releases](UPDATES.md) are implemented, with a local channel for development. A new version is shown at the bottom of the left panel; installation and restart are done by Sparkle on a button press. An already running background client stays on its version; its processes, database and sessions are not restarted. Compatibility, check boundaries, publishing and acceptance boundaries are described separately.

## Interface refinements of November 6

Section order: **«Переписка» ("Conversations") → «Агенты» ("Agents") → «Сервер» ("Server")**. Conversations opens first. History scrolls within its own area; the input field stays at the bottom. The bottom info bar, the outer frame of the browser mockup and the duplicate header in the native window were removed; the input field is compact, with a soft background. Agents are grouped by provider: Claude Code, Codex, Cursor. The CLI discovery status is in the card, without a duplicate list below. The «Сессии» ("Sessions") button opens a separate window with its own scrolling. [Setup and a test launch of Claude](SESSION_SETUP.md) are available in the current client; existing working conversations are not rebound.

The theme is chosen at the bottom of the left panel: light, dark or system. The native window setting is kept across launches/updates and also applies to the application's system windows.

By default the message queue is shown to the right of the conversation; an icon at the top hides and restores the panel. There is no separate queue tab. Currently the panel shows the number of pending messages and sessions requiring attention, based on client data. The queue panel, the message history and the list of computers scroll independently; the theme, current version and update are pinned at the bottom left.

Manual refresh of messages, queue and statuses is in the menu **«Вид → Обновить сообщения и статусы»** ("View → Refresh Messages and Statuses") (⌘R). It does not install a new version of the program. The top bar of the conversation keeps the toggle for the right queue column.
