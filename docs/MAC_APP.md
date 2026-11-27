# Kotel.app

The native shell for Mac: AppKit, the system WKWebView and the embedded Go client. No browser, Python, Node, Go or terminal is needed. It is a technical preview: the app is signed ad-hoc and not notarized.

## Installation and opening

Follow the [Mac download and first-open steps](DOWNLOAD.md#mac) on an Apple Silicon Mac running macOS 15 or later. The app is ad-hoc signed and awaits Developer ID signing and notarization.

Later versions are checked in the background. When a release is available, the update arrow appears next to this computer's version. Press it to open the Kotel update dialog. The manual check is in **Kotel → Проверить обновления…**.

On opening, the shell finds the previous database via the existing protective registry, or uses the normal user directory when there are no reservations. Several different directories, a corrupted registry or a missing saved database lead to a refusal. Reservations are not deleted or restored automatically. If the running client uses a different directory without recorded reservations, choose it via the menu «Файл → Выбрать каталог данных…»; there is no search for a "last session".

While the window connects to its client (at every start and after an update) it shows only a spinner. The reason, «Повторить подключение» and a short note appear only when connecting failed (T-178); the data-directory choice is no longer on that screen, only in the «Файл» menu.

The application connects to an already running client. If there is none, it starts one background client, which runs tasks at once (D-029). The global lock protects this path too. The window itself does not invoke a CLI and does not sign in on the user's behalf.

The red button hides the window in the macOS menu bar. Reopen it from the Kotel icon or by opening the app again. Cmd+Q closes the shell and its local HTTP process. The background client and its tasks keep running. The UI process lives while the inherited stdin channel is open; if the shell crashes, EOF closes its HTTP server.

## Interface boundaries

The embedded WebView receives only the random private loopback address of its UI process. It does not open external sites, new windows, file URLs or arbitrary navigations. WebView storage is non-persistent; after a full quit, unsent text may disappear.

## The window

The window opens to the current task. The left sidebar shows active work and the Archive opens on the right. The same window runs on Windows and Linux. See the [task model](TASK_MODEL.md).

- The left sidebar groups active tasks by computer or project and can show a flat list. Busy agents show the model and elapsed time when available. The bottom-left toolbar opens Settings, feedback and Statistics and shows this computer’s version and update arrow. The feedback form saves remarks and optional screenshots on this computer.
- The button at the top right toggles the Archive.
- Settings has General, Project, Agents and Network sections. Project holds the local project folder and shared worker instructions. Network shows connection settings and the computers’ versions and update controls.
- **«Вид → Обновить задачи и статусы»** (⌘R) refreshes the window. It does not install a new version.

The theme set in the window also applies to the app's system windows and is kept across launches and updates.

## Build and checks

From verified sources on a Mac with stable Go and Apple Command Line Tools:

```sh
bash scripts/build-mac-app.sh /absolute/new/dir 0.0.0-preview.3 FULL_SHA
python3 scripts/check-desktop.py '/absolute/new/dir/Kotel.app/Contents/Resources/kotel'
```

The builder refuses to overwrite an existing app/zip, builds Go with CGO_ENABLED=0, compiles Swift, creates the icon, signs its own bundle ad-hoc, verifies the signature, prints version and creates a zip with SHA-256. SOURCE_SHA is set by the caller; a release requires checking that the source tree is clean.

An automated test on a separate HOME verifies launching without models, two UIs connecting to one client, EOF/reopening without losing the queue, and the absence of model attempts. Go tests separately verify directory selection, that reservations stay unchanged, and refusal of model flags. CI builds the bundle and checks the bridge, but this is not presented as a visual test of the WebView.

The owner uses the app daily on a Mac; CI builds the bundle on every pull request. There are no real models in the tests.

Foundations of the shell: [NSApplication](https://developer.apple.com/documentation/appkit/nsapplication), [WKWebView](https://developer.apple.com/documentation/webkit/wkwebview). The Kotel core remains in Go; Swift is only the macOS platform shell.

## Updates from the window

[Signed updates via GitHub Releases](UPDATES.md) are installed by Sparkle on a button press. After an update the window restarts an older background client by itself (D-091). If tasks are running, it does not interrupt them on its own: it says «Обновление ждёт задачи» and offers «Дождаться и перезапустить» or «Прервать и перезапустить» (T-123).
