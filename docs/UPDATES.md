# Updates

Updates on Mac, Windows and Linux, and updates of another computer of the network. The programs receive signed builds from the public GitHub Releases of `kolotiloff/agent-messenger`. No GitHub key or login is needed to download. The private signing key stays in the developer's Keychain; the sources and the app contain only the public Ed25519 verification key.

## What gets updated

On Mac, Sparkle 2.10.1 replaces the app and the embedded interface, then reopens it. New clients are started from an immutable copy of the binary outside the `.app`. When the window then sees an older background client, it asks it to restart once, by itself (D-091); the database, protective reservations and `active-runs.json` stay as they are. A client with running tasks refuses that restart, because a restart interrupts them and they are never repeated on their own. The window then says «Обновление ждёт задачи» and offers «Дождаться и перезапустить» (restart once the tasks end) or «Прервать и перезапустить» (T-123). Rollback to an older version is not offered. Unsaved input text may be lost when the window restarts.

Immediately before it asks the older client to stop, the Mac window records one `messenger_updating` event for coordinators on that Mac. If contact with all peers drops during the next two minutes and recovers within that period, the normal loss/back pair is skipped. A prolonged outage reports the usual loss event.

## Delivery

The Mac app checks GitHub Releases in the background, up to once every 30 minutes, and on a manual check from the app menu. When it finds a newer compatible release, the update arrow appears next to this computer's version. Press it to download and install through the window's update dialog. The Mac feed uses `update-darwin-arm64.json`, its `.sig` file and an archive named by its SHA-256. A release with an incomplete or corrupted set is rejected. The Windows and Linux windows follow the same check-then-download pattern.

Only the release list uses the GitHub API (`api.github.com`), once per check. The updater orders that list by descending numeric preview build and requires the signed manifest version to match its release tag (D-194). The files are downloaded from the release's own download address (`github.com/kolotiloff/agent-messenger/releases/download/<tag>/<name>`, which redirects to GitHub's file host); an asset whose address is anything else is refused. These downloads do not count against GitHub's limit of 60 API requests an hour per address. Before 2026-11-09 each check made three API requests, and the owner's computers behind one address used the limit up, so a manual check failed (T-103). When GitHub still refuses for that limit, the check says so with the time to try again («GitHub временно ограничил проверки…»; `update-fetch` returns `state: rate_limited` with `retry_at`) instead of a download failure.

Builds up to 0.100 stop at the first compatible release in the public list. This prevented discovery of 0.101 and 0.102 when GitHub put 0.99 first. Version 1.0 is offered through the normal in-app update: the original 0.99 and 0.101 clients both found and downloaded it on macOS, and the public channel was checked on all five targets.

The release workflow checks the public list in the order used by 0.99 for all five platforms. Its first eligible signed package must lead to build 101 or higher, where numeric sorting takes over. If an older entry blocks this route, the optional recovery switch can rename only the blocking 0.98 and 0.99 platform manifests to `archived-update-*.json` after verifying the destination. Installers, archives and signatures keep their original URLs. Restore a manifest by renaming the same release asset to its original name. A failed post-publication check can be rerun independently of signing and publishing.

The command contacts only the fixed repository over HTTPS. Only the exact API and GitHub asset CDN domains are allowed; redirects are restricted. Requests carry no account tokens, cookies, provider keys or the window's local key. Responses are limited in size and time. On a network error the UI reports a failure and does not present it as "no updates". The command does not contact the supervisor, models or the session database.

The JSON signature is verified before the archive is downloaded; then the OS, architecture, API, size and SHA-256 are checked. A version lower than the installed/saved one is not installed; a different package with the same version is rejected. A single file lock forbids parallel writes. The archive is immutable, and the signed metadata is switched with one atomic `current.json`. A network or signature error leaves the previous package in place. A corrupted `current.json` does not permit rollback to an old pair of files. For the first transition, the old local `manifest.json` + `manifest.sig` format is supported.

A local HTTP bridge with a separate window Bearer key hands the verified package to Sparkle. Sparkle additionally verifies the archive's EdDSA before unpacking. The dynamic XML is generated locally and is not a remote trusted feed. Checking, downloading, waiting and installing do not call models.

## Windows and Linux

The programs for Windows and Linux use the same channel and the same public key, built into the program (D-048). Each release has `update-windows-amd64.json`, `update-windows-arm64.json`, `update-linux-amd64.json`, `update-linux-arm64.json`, their `.sig` files, and a `<sha256>.zip` for each with only `messenger.exe` or `messenger` inside. `scripts/package-others.sh` builds the archives; the release job signs them on macOS with the same key and Sparkle's `sign_update` (`scripts/sign-other-updates.py`) and checks each signature against the public key before uploading.

The window (`messenger open`) checks when it opens and then every 30 minutes, in the background, without the Mac's `update-fetch` helper. «Обновиться» in the window:

1. checks the downloaded archive's size, SHA-256 and Ed25519 signature again;
2. unpacks the one program file and writes it next to the running one (`.<name>.update`);
3. runs it with `version` and requires the exact version, revision, system and architecture from the signed metadata;
4. replaces the program: on Linux one rename; on Windows the running file is renamed to `<name>.old` (or `.old-1`… if an older one is still running) and the new one takes its name; the leftovers are removed at a later start. A program installed with `messenger install` is installed as a new version folder next to the running one instead, while the background client keeps running from its own folder (T-107); a manual `messenger install` still needs the client stopped;
5. closes the page and starts the new program with the same arguments, which opens the window again.

Any failure before step 4 leaves the program untouched and removes the staged file. The background client and its tasks are not touched by the replacement; the new window then restarts an older client as on the Mac (D-091, T-123). The program must sit in a folder the user can write to.

### From another computer of the network (T-078, D-100, T-155, D-133)

An update icon on another computer's card in the left panel, shown when that computer's version is older than this one, asks that computer, through the broker's mailbox, to update itself (T-128; it asks for confirmation first). The coordinator's `update_messenger` does the same, for this computer or another of the network. It is offered only for a computer on line that reports its version (older ones would not answer). Running tasks are never interrupted. The computer answers at once: `queued` while a task still runs or waits (it takes no new tasks and shows «ждёт обновления»), refused on a Mac (open Messenger there and press «Обновиться»), when its window never recorded the program file (`program.json` in the data folder, written each time `messenger open` starts) or while another update runs; otherwise «updating». A queued request starts the helper as soon as no task still runs; blocked and interrupted tasks do not hold it. Its client then starts the hidden `_remote-update` command from that program file, detached. The helper checks the live tasks again, runs the same check and steps 1–4 above, restarts the background client from the installed program (the client's own shutdown command, then a start from a runtime copy; a pause stays), and writes `remote-update.json` (0600). Whichever client runs sends that result and removes the file; one that never appears is answered as failed after 30 minutes. If there is no newer release, the answer is «уже последняя версия», or a restart when the running client is older than the program file. The result appears as a notice above the reply box (T-128) and as a `messenger_update` event for the coordinator. Nothing is retried on its own; `active-runs.json` and reservations are not touched.

Starting with 0.100, ask the coordinator to call `cancel_messenger_update` with the computer ID from `list_agents` to release a queued update. Wait for the `messenger_update` result before sending new work. The command refers to the exact observed request and leaves started installations and running tasks alone. The receiver must run 0.100 or newer. To cancel an automatic update or one requested by another computer, the broker must also be 0.100 or newer to carry the pending request ID. The original requester can use its saved ID on an older broker. An already-installed target or a signed check proving there is no newer release releases the pending update even while tasks still run.

With T-240, a Windows or Linux window checks locally for the newly installed version, even while in the tray. When the background client answers with that newer version and the recorded program matches the signed update, the window closes and opens the new program. This step leaves the background client running. A window from a release before T-240 needs to be exited once via the tray and reopened from the installed program to enable this behavior.

On Windows, the update and the installer's handoff to the installed program start the desktop window without creating a Command Prompt.

## Releasing from GitHub Actions

The **Release preview** workflow (`.github/workflows/release.yml`) builds, signs and publishes a preview from `main`. Run it under **Actions → Release preview → Run workflow** with the version (for example, `0.0.0-preview.86`) and release notes.

From 1.0, the public name comes from the semantic version before the signed build suffix. For example, `1.0.0-preview.103` displays as 1.0 and sorts as build 103. Earlier builds keep their original names.

People see the short version: the release title, the Mac app («О программе», `CFBundleShortVersionString`), the Mac update dialog and the window say «0.58» for `0.0.0-preview.58` (owner, 2026-11-09, D-105). The tag, the manifests and the program's own `version` keep the full form, because every installed program checks it before it accepts an update; changing it would need a release that accepts both forms first, and all computers updated to it.

1. **Checks.** It refuses any ref other than `main`, a malformed version, an existing tag and empty notes. Then it runs `scripts/check.sh`.
2. **Build.** It builds the app with `build-mac-app.sh` and runs the desktop lifecycle check.
3. **Signing.** It signs with `publish-local-update.py --key-file`.
   - The key comes from the repository secret `SPARKLE_ED_PRIVATE_KEY`.
   - The key is written to a `0600` temporary file and removed after signing.
   - Each signature (archive and manifest) is verified against `SUPublicEDKey` from the built app by `scripts/verify-update-signature` before anything is kept.
4. **Windows and Linux.** It signs the four update archives built on the Linux job (`scripts/sign-other-updates.py`, same key file, each signature checked).
5. **Publishing.** It uploads the Mac assets, the downloads and the signed Windows and Linux updates to a draft release, then publishes it.

The secret holds the Sparkle EdDSA key exported once from the developer's Keychain:

```sh
generate_keys --account agent-messenger-preview -x key.txt
```

Paste the contents of `key.txt` into Settings → Secrets and variables → Actions → `SPARKLE_ED_PRIVATE_KEY`, then delete the file. Anyone who can change workflows on `main` can use this secret, so keep write access to the repository limited.

## Preparing a release locally

```sh
SPARKLE_DIR=/absolute/path/to/sparkle \
  bash scripts/build-mac-app.sh /absolute/path/to/output 0.0.0-preview.NUMBER FULL_SOURCE_SHA
python3 scripts/publish-local-update.py /absolute/path/to/output \
  --sparkle /absolute/path/to/sparkle --notes /absolute/path/to/notes.txt \
  --directory /absolute/path/to/staging-channel
```

The script uses a separate Keychain account `agent-messenger-preview`, checks that the ZIP matches the bundle and creates `release-assets` next to the build. Only the three files from this folder are uploaded to the GitHub Release. Upload all files to a draft first, then publish. Do not upload the Keychain, source profiles, test databases, session logs or the internal `current.json`. Local publishing to the main channel remains a development method and a way to make the first transition from the old updater.

Sparkle is downloaded from the official release with a pinned checksum; its license is included. Python is needed only by the developer for publishing, not by the app. Ad-hoc signing of the Mac app does not replace Developer ID and notarization for distribution to other people's Macs.

## Tests and remaining stages

Transport tests cover a correct download, an allowed CDN redirect, a foreign domain, a wrong signature, corruption/truncation/size overrun, a repeated asset name, a missing signature, HTTP errors, rejection of version downgrade/conflict, preservation of the previous channel and blocking of a second writer. A shell test checks that the queue is preserved and that no models are started.

Local transitions between preview builds via Sparkle have been verified on the user's Mac. The public preview.12 → preview.13 transition on Mac has been accepted: [actual report](reports/2026-11-06-github-update-mac.md). Since then every release has been installed this way on the owner's Mac, Windows and Linux computers. Still missing: backup and rollback, and Gatekeeper/notarization on a clean Mac.

API contracts: [GitHub Releases](https://docs.github.com/en/rest/releases/releases), [downloading assets](https://docs.github.com/en/rest/releases/assets). Preparing a future public release does not mean all Messenger features are ready.

The preview.13 → preview.14 transition has also been [verified on Mac](reports/2026-11-06-github-update14-mac.md), with the queue and the previous background client preserved.

Starting with preview.15, the timer allows a deviation of up to 15 seconds. Automatic requests are limited to one run per 30 minutes (5 minutes before T-103); no parallel download is created. With automatic checking disabled, the manual menu command remains. This is a periodic check, not push: computer sleep, the network and an active installer may delay detection.

The preview.14 → preview.15 transition and a repeated automatic check without restart have been [accepted on Mac](reports/2026-11-06-github-update15-mac.md).
# Reopening the window during a download

The downloader may finish after the window has been closed. A new window does not take over its lock and does not treat a busy download as GitHub being unavailable: `update-fetch` returns `state: busy`, the window shows «Загружается обновление…» ("Downloading update…") and repeats the check after 5 seconds, for at most 5 minutes in a row. While the lock is held, the retry makes no network request. Once it is released, the downloader checks the signed channel in the usual way. A long wait offers an explicit retry. A verified available package remains available for installation. Download and signature verification errors are not masked as no internet connection.
