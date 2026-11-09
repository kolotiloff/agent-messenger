# Automated pre-release checks

The `.github/workflows/checks.yml` workflow runs on pull requests, on changes to main/master and manually. It needs no working CLIs or provider accounts. It checks the code with stand-in processes; it is not acceptance of real Claude/Codex/Cursor.

## Matrix

| Job | What is checked |
|---|---|
| Ubuntu 24.04, Go 1.22.2 | Formatting, module integrity, vet, the full suite with race; compiling the product and test binaries for macOS arm64/amd64, Linux amd64 and Windows amd64 |
| macOS 15, Go 1.22.2 | Native checks of compatibility with the baseline Go version |
| macOS 15, current stable Go | Native checks; two preview archives, SHA-256, clean install, refusal to update a live client, update of a stopped client with a non-empty queue and outgoing prepared/queued messages; no automatic sending and no duplicate after the update |
| Ubuntu 24.04, current stable Go | Compatibility of code and tests with the current stable Go version; the actual version is recorded in the log |

A Windows build is not presented as running on Windows. GitHub checks on Mac/Linux also do not replace experience on the four user computers, the availability of their CLIs, logins, sleep/wake and the operation of a network broker. Confirming that a specific SHA is ready requires all jobs to succeed for exactly that SHA.

## Access and cost limits

Workflow permissions are read-only access to repository contents. Actions are pinned to exact SHAs, and checkout does not persist credentials in git. Not used: pull_request_target, user secrets, agent logins, auto-merge or release publication. The build job creates only technical archives with an ad-hoc signature; there is no Developer ID/notarization.

Each of the four jobs is limited to 20 minutes. A new run for the same PR cancels the previous one. There is no schedule or nightly polling. Artifacts are kept for 7 days. In a private repository GitHub uses the account's available Actions minutes; the workflow does not change billing or limit settings. [GitHub runner types and terms](https://docs.github.com/en/actions/reference/runners/github-hosted-runners).

Pull request sources run in the ordinary restricted PR context, without being moved into a privileged workflow. [GitHub Actions events](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows).

## The same checks locally

From the repository root:

```sh
bash scripts/check.sh
bash scripts/check-builds.sh
```

You can set `GO_BIN=/absolute/path/to/go`. The first script does not change formatting but reports differences; it checks dependencies and runs native race tests. On Mac it uses the external linker for compatibility with the already verified Go 1.22.2. A short temporary directory is needed because of Unix socket path length limits; only the directory the script itself created is deleted.

The second script compiles every test package but does not run it under a foreign OS. It does not substitute a fake successful run result.

Checking two Mac archives:

```sh
python3 scripts/check-package.py /archives/dir VERSION_1 VERSION_2
```

This is a developer/CI tool using the Python standard library. The installed product does not need Python. The check creates a separate HOME, program directory and data, connects a fake agent program (`/usr/bin/false`) and pauses the client, refuses an update while the client runs, updates while it is stopped, and compares the agents and the pause before and after. It does not change the working installation and does not call real models (T-028 replaced its old session and queue steps).

After a successful Mac job, the archives are available in that run's Artifacts. They contain the revision of the checked-out code. For a PR this may be GitHub's temporary merge commit; it is not automatically equal to the branch head. Actions archives remain preview even if all checks are green.

## Builder version

CI archives are built with the current stable Go on Mac; Go 1.22.2 is kept only for source compatibility. The actual version is in the log and in `messenger version`. For an accepted release, the Go version must be recorded together with the source SHA and checksums. The old minimum `go` in go.mod is not a recommendation to ship binaries with an old runtime. [Go release policy](https://go.dev/doc/devel/release#policy): the two most recent major versions are supported.
