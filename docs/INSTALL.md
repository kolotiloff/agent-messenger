# Installing Agent Messenger

For regular use of the Mac app, start with [downloading and installing Agent Messenger.app](MAC_APP.md). A technical preview is available for macOS 15+ on Apple Silicon. Windows, Linux and Intel Mac will get separate packages after installation and updates have been verified; there are no ready downloads for them yet.

Below is a separate archive of the command-line client for developers and testing. It is a different package format; the commands on this page are not needed to install the `.app`.

## From a verified archive

1. Download the archive and the checksum to compare against from a trusted place. A checksum attached by an unknown sender to an unknown file does not confirm provenance.
2. Verify the archive's SHA-256 and unpack it into a separate directory. In it, run `shasum -a 256 -c SHA256SUMS` and `./messenger version`. Metadata is also in `manifest.json`.
3. Run `./messenger install`. By default the version is placed in `~/.local/lib/agent-messenger/`, and `~/.local/bin/messenger` points to it. You can set `--prefix /absolute/user/directory`.
4. Add the `bin` directory to PATH yourself if needed. The installer does not change shell settings, does not start the client/model, does not sign in and does not install a service.
5. Next steps — [local client](CLIENT.md), [background run](BACKGROUND.md), [window](LOCAL_UI.md). Without model authorization, launching does not invoke a CLI. For a first try you need a separate test directory and test sessions.

No sudo is needed. The package has a local ad-hoc signature, without Developer ID and notarization. Running it after a browser download with Gatekeeper checks is not yet accepted; the package does not suggest any commands to remove protection or quarantine. This is a limitation before public distribution, not a passed check of installation by a new user.

## Updating

A new version is installed with the same command from a new verified archive. Binaries are kept in separate directories per version and SHA-256; the previous one is not overwritten. The link is switched by an atomic rename. When reinstalling the same build, the content of the existing file is verified.

Switching is protected by the shared client and CLI-launch locks. A running client, an unfinished launch, any persisted CLI reservation, or an empty/corrupted registry blocks installation. The installer **does not stop processes and does not clear reservations**. Follow the existing safe shutdown/recovery procedure. In the current prototype this may restrict updating until the owner lifecycle is resolved; no bypass is added.

The queue, outbox, history and profiles are stored separately from the program. The installer does not open or migrate SQLite. Launching the next version may perform its documented schema changes. The old binary version is kept, but running it is not a database rollback; compatibility of an arbitrary downgrade is not promised. Someone else's `bin/messenger` file or a link to another product is not replaced.

## Building the package from source

From the root of a verified state of the repository:

```sh
go run ./cmd/package-preview \
  --version 0.0.0-preview.1 \
  --revision FULL_SOURCE_SHA \
  --out /absolute/builds/directory
```

The `--revision` parameter is the commit SHA declared by the builder; the program does not certify that arbitrarily modified local files match this SHA. Before a release, the clean state of the sources must be verified separately. Do not use the placeholder SHA of a test package as the number of a real release.

The builder invokes Go without a shell, embeds the version/revision, uses `-trimpath -buildvcs=false`, signs the native binary ad-hoc, checks that the `version` command runs, and creates a tar.gz, a manifest, the binary's SHA-256 and a separate archive checksum. The package includes instructions and documentation. An existing archive with the same name is not overwritten. Uploading or publishing to GitHub is not done automatically.

For the Go 1.22.2/macOS combination verified here, `--external-linker` and Apple's installed build tool were used. In other builds `CGO_ENABLED=0` is the default. You can set `--go /exact/path/to/go`. The Go/Apple tools are needed by the builder; the user of a ready archive does not need them. CI keeps test archives only after the installation and update checks; the builder and the source SHA are recorded in the metadata. Bit-for-bit reproducibility, separate provenance attestation and a developer signature are not yet accepted. For new builds use a supported stable Go version; 1.22.2 is kept in CI as a source compatibility check.

Installation and update verification — [Mac report](reports/2026-11-06-preview-install-mac.md).
