# Mac app updates

The app receives signed builds from the public GitHub Releases of `kolotiloff/agent-messenger`. No GitHub key or login is needed to download. The private signing key stays in the developer's Keychain; the sources and the app contain only the public Ed25519 verification key.

## What gets updated

Sparkle 2.10.1 replaces the app and the embedded interface, then reopens it. A running background client stays on its previous version: its processes, database, queue, model permission and protective reservations do not change. New clients are started from an immutable copy of the binary outside the `.app`. Full background client update, migrations and rollback have not yet been accepted. Unsaved input text may be lost when the window restarts.

## Delivery

When the window opens, then every 5 minutes, and when the app is activated (with an overall limit of one automatic request per 5 minutes), if automatic checking is enabled, the built-in `update-fetch` command checks the 20 most recent GitHub Releases. An available version is indicated by the «Доступно обновление» ("Update available") button at the bottom left. A manual check is in the app menu. When there is no update, there is no permanent large button. Drafts are skipped; preview releases are allowed. The first published release with metadata for the current OS and architecture defines the channel. File names: `update-darwin-arm64.json`, `update-darwin-arm64.json.sig` and `<sha256>.zip` (for Intel, `darwin-amd64`). A release with an incomplete or corrupted set is rejected, not masked by an older release.

The command contacts only the fixed repository over HTTPS. Only the exact API and GitHub asset CDN domains are allowed; redirects are restricted. Requests carry no account tokens, cookies, provider keys or the window's local key. Responses are limited in size and time. On a network error the UI reports a failure and does not present it as "no updates". The command does not contact the supervisor, models or the session database.

The JSON signature is verified before the archive is downloaded; then the OS, architecture, API, size and SHA-256 are checked. A version lower than the installed/saved one is not installed; a different package with the same version is rejected. A single file lock forbids parallel writes. The archive is immutable, and the signed metadata is switched with one atomic `current.json`. A network or signature error leaves the previous package in place. A corrupted `current.json` does not permit rollback to an old pair of files. For the first transition, the old local `manifest.json` + `manifest.sig` format is supported.

A local HTTP bridge with a separate window Bearer key hands the verified package to Sparkle. Sparkle additionally verifies the archive's EdDSA before unpacking. The dynamic XML is generated locally and is not a remote trusted feed. Checking, downloading, waiting and installing do not call models.

## Preparing a release

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

Local transitions between preview builds via Sparkle have been verified on the user's Mac. The public preview.12 → preview.13 transition on Mac has been accepted: [actual report](reports/2026-11-06-github-update-mac.md). Remaining before the full stage: background client update, backup and rollback, Gatekeeper/notarization on a clean Mac, and runtime acceptance on Windows/Linux.

API contracts: [GitHub Releases](https://docs.github.com/en/rest/releases/releases), [downloading assets](https://docs.github.com/en/rest/releases/assets). Preparing a future public release does not mean all Messenger features are ready.

The preview.13 → preview.14 transition has also been [verified on Mac](reports/2026-11-06-github-update14-mac.md), with the queue and the previous background client preserved.

Starting with preview.15, the timer allows a deviation of up to 15 seconds. Automatic requests are limited to one run per 5 minutes; no parallel download is created. With automatic checking disabled, the manual menu command remains. This is a periodic check, not push: computer sleep, the network and an active installer may delay detection.

The preview.14 → preview.15 transition and a repeated automatic check without restart have been [accepted on Mac](reports/2026-11-06-github-update15-mac.md).
# Reopening the window during a download

The downloader may finish after the window has been closed. A new window does not take over its lock and does not treat a busy download as GitHub being unavailable: `update-fetch` returns `state: busy`, the window shows «Загружается обновление…» ("Downloading update…") and repeats the check after 5 seconds, for at most 5 minutes in a row. While the lock is held, the retry makes no network request. Once it is released, the downloader checks the signed channel in the usual way. A long wait offers an explicit retry. A verified available package remains available for installation. Download and signature verification errors are not masked as no internet connection.
