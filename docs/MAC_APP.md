# Agent Messenger.app

The native shell for Mac: AppKit, the system WKWebView and the embedded Go client. No browser, Python, Node, Go or terminal is needed. It is a technical preview: the app is signed ad-hoc and not notarized.

## Installation and opening

1. Download `AgentMessenger-Mac.zip` with the Mac button in the [README](../README.md) (always the newest release).
2. Safari unpacks it into `Agent Messenger.app`; otherwise double-click the zip.
3. Move `Agent Messenger.app` to `~/Applications` and open it. The first time macOS may ask to confirm the opening in System Settings → Privacy & Security.

Later versions are checked automatically at launch, every 30 minutes and when you return to the window (with an overall interval of no less than 30 minutes). **«Доступно обновление»** appears at the bottom left; the manual check is in the menu **Agent Messenger → Проверить обновления…**. No GitHub sign-in is needed.

The package has an ad-hoc signature: Developer ID and notarization are not done yet. Bypassing macOS protections is not required and not suggested. Apple Silicon and macOS 15 or later only.

On opening, the shell finds the previous database via the existing protective registry, or uses the normal user directory when there are no reservations. Several different directories, a corrupted registry or a missing saved database lead to a refusal. Reservations are not deleted or restored automatically. If the running client uses a different directory without recorded reservations, choose it via the menu «Файл → Выбрать каталог данных…»; there is no search for a "last session".

The application connects to an already running client. If there is none, it starts one background client, which runs tasks at once (D-029). The global lock protects this path too. The window itself does not invoke a CLI and does not sign in on the user's behalf.

The red button closes the window, but the application stays in the Dock; «Файл → Открыть окно» or reopening the application brings it back. Cmd+Q closes the shell and its local HTTP process. The background client and its tasks keep running. Its own UI process lives while the inherited stdin channel is open; if the shell crashes, EOF closes the HTTP server. There is no automatic restart of models.

## Interface boundaries

The embedded WebView receives only the random private loopback address of its UI process. It does not open external sites, new windows, file URLs or arbitrary navigations. WebView storage is non-persistent; after a full quit, unsent text may disappear.

## The window

The tabs are **«Задачи» → «Агенты» → «Проект» → «Сервер» → «Статистика»**; tasks open first. The same window runs on Windows and Linux; what each tab does is in [TASK_MODEL.md](TASK_MODEL.md).

- The left panel lists the project and the computers of the network with their agents. At its bottom are the version, the update notice, the gear «Настройки программы» (theme: light, dark or system; language: Русский or English) and the bug icon «Отзыв о программе» (a remark with a picture, saved on this computer; «Выгрузить все отзывы» writes them to one zip in Downloads).
- An icon at the top right shows or hides the task list.
- «Проект» holds the project folder and «Общие указания исполнителям», copied into every new task.
- **«Вид → Обновить задачи и статусы»** (⌘R) refreshes the window. It does not install a new version.

The theme set in the window also applies to the app's system windows and is kept across launches and updates.

## Build and checks

From verified sources on a Mac with stable Go and Apple Command Line Tools:

```sh
bash scripts/build-mac-app.sh /absolute/new/dir 0.0.0-preview.3 FULL_SHA
python3 scripts/check-desktop.py '/absolute/new/dir/Agent Messenger.app/Contents/Resources/messenger'
```

The builder refuses to overwrite an existing app/zip, builds Go with CGO_ENABLED=0, compiles Swift, creates the icon, signs its own bundle ad-hoc, verifies the signature, prints version and creates a zip with SHA-256. SOURCE_SHA is set by the caller; a release requires checking that the source tree is clean.

An automated test on a separate HOME verifies launching without models, two UIs connecting to one client, EOF/reopening without losing the queue, and the absence of model attempts. Go tests separately verify directory selection, that reservations stay unchanged, and refusal of model flags. CI builds the bundle and checks the bridge, but this is not presented as a visual test of the WebView.

The owner uses the app daily on a Mac; CI builds the bundle on every pull request. There are no real models in the tests.

Foundations of the shell: [NSApplication](https://developer.apple.com/documentation/appkit/nsapplication), [WKWebView](https://developer.apple.com/documentation/webkit/wkwebview). The Messenger core remains in Go; Swift is only the macOS platform shell.

## Updates from the window

[Signed updates via GitHub Releases](UPDATES.md) are installed by Sparkle on a button press. After an update the window restarts an older background client by itself (D-091). If tasks are running, it does not interrupt them on its own: it says «Обновление ждёт задачи» and offers «Дождаться и перезапустить» or «Прервать и перезапустить» (T-123).
