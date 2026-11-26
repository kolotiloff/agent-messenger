# Handoff: current state

This file is the shared memory between tools (Claude Code, Cursor, Codex). Whoever stops working updates the **Current state** section; whoever starts reads it first. Tasks live in [TASKS.md](TASKS.md), decisions in [DECISIONS.md](DECISIONS.md), open questions in [QUESTIONS.md](QUESTIONS.md), the work history in [LOG.md](LOG.md).

## Owner's standing instruction for the night of 2026-11-07

The owner asked the agent to finish the program without them ("доделай бе меня полностью"), update the documentation on GitHub and clean out old code. Same rules as before: fake CLIs only, no real models, `active-runs.json` untouched, nothing retried on an unclear result, questions go to `QUESTIONS.md` with the most cautious option chosen. Removing the persistent paths (T-028) still needs the owner's explicit consent (Q-012).

## Current state

**T-320 uses a subtle tint to distinguish the right project in the split sidebar.** Hover brightens the half under the pointer from the shared header through the agent rows. The selected half retains its blue fill. Chromium and WebKit covered both hover positions, the blue selection, stable card sizes and the single-project mode. See D-225.

**[1.0.8, build 111](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.8-preview.111) is released** from `43535577935cab08cde7c07452b5ca111ff9731f`. [PR #220](https://github.com/kolotiloff/agent-messenger/pull/220) integrated T-319. All four [release jobs](https://github.com/kolotiloff/agent-messenger/actions/runs/45875009911) passed. The public channel serves five verified signed update packages, with 22 files in the release.

Both project columns show the same agents reported by each computer. Task activity stays scoped to its project. A missing project folder or an activity request in flight no longer replaces known agents, and a computer without agents shows the normal empty message in both columns. See D-224.

**[1.0.7, build 110](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.7-preview.110) is released** from `c71dffead7df31e42dcd28517dc59f2348a07f32`. It includes [T-317](https://github.com/kolotiloff/agent-messenger/pull/217) and [T-318](https://github.com/kolotiloff/agent-messenger/pull/218). All four [release jobs](https://github.com/kolotiloff/agent-messenger/actions/runs/45869548677) passed. The public channel serves five verified signed update packages, with 22 files in the release.

Selection fills the whole computer card in one-project mode and the chosen half, including the shared header, in the split sidebar. Clicking that half opens the tasks of its project and computer. Each project keeps its last computer choice for the current window session. Chromium and WebKit verified switching and equal card heights for equivalent agent rows. See D-222 and D-223.

**[1.0.6, build 109](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.6-preview.109) is released** from `829a0567fc5bee4c965c354d2104e8872de2f178`. [PR #215](https://github.com/kolotiloff/agent-messenger/pull/215) integrated T-314 through T-316. The [eight client checks](https://github.com/kolotiloff/agent-messenger/actions/runs/45841685798) and [four release jobs](https://github.com/kolotiloff/agent-messenger/actions/runs/45848162519) passed. The public channel serves five verified signed update packages, with 22 files in the release.

The sidebar can display two projects' agent columns on each computer with one active task conversation. Statistics and JSON export can cover both projects or either one. The picture viewer supports closing from the Mac title bar area and copying the selected picture as a full-resolution PNG. The computer card centers the update spinner during a Mac download. The task status and open question use a shared read-only snapshot during concurrent worker output. See D-219 through D-221.

On Apple Silicon the published 1.0.5 client found and downloaded 1.0.6 in an isolated home. The extracted app passed signature verification. The desktop bridge restarted the client and preserved two test projects and their history. Combined and separate statistics, JSON exports, reopening and SQLite integrity checks passed. The browser fixtures exercised the picture viewer with a simulated clipboard writer.

**[1.0.5, build 108](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.5-preview.108) is released** from `4c3c685df2d9e3aa2502deb858b6343c6452294e` as of 2026-11-26 (Europe/Moscow). [PR #210](https://github.com/kolotiloff/agent-messenger/pull/210) brings T-309 through T-313 together. All four [release jobs](https://github.com/kolotiloff/agent-messenger/actions/runs/45711532504) passed, including the public signed upgrade path from 0.99 on all five targets. The release contains 22 assets.

On Apple Silicon, the published 1.0.4 client found and downloaded 1.0.5 through the normal channel in an isolated home. The published 1.0.5 client restarted the old background client through the desktop bridge, retained two test projects' task histories and statistics, and exported each project separately. Both project databases passed SQLite integrity checks. The test used the extracted app package and left owner installations untouched.

**T-311 adds a filtered JSON export to Statistics.** It reads committed records for the selected project, period, computer, agent and model. It includes the full matching history known to this computer, with separate requested and last reported models and explicit missing-usage fields. Fixture databases cover legacy and split storage under a write transaction.

**T-312 and T-313 add optional assignment labels, review, retry lineage and model efficiency.** The coordinator records review through `review_task`. Finished launches save individual usage observations. The comparison runs on demand in Statistics and travels in the JSON export with labels, attempts and review history. Existing databases and older executors provide the data they already have. D-218 records the definitions and upgrade behavior.

T-313 groups model efficiency by task type and difficulty. The summary shows an inline task count and acceptance percentages with exact fractions on hover or keyboard focus. Thin rules separate the groups. A leading arrow opens a table with active runtime, sample sizes and the threshold for 90% of tasks, on the same rounded surface used for outgoing messages. The Statistics toolbar groups filters on the left and has chart and export buttons on the right. The window changes are included in [PR #209](https://github.com/kolotiloff/agent-messenger/pull/209).

**T-309 hides unavailable removal actions.** The task context menu shows Remove from list only when the selected task can be removed and opens only when there is an action or a stop request to show. See D-216.

**T-310 shows confirmed models in expanded computer rows.** Expanded running rows show the agent name and add the confirmed model beside it. Gray elapsed time follows a comma. [PR #207](https://github.com/kolotiloff/agent-messenger/pull/207), D-217.

**[1.0.4, build 107](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.4-preview.107) is released** from `4822cfd0e59934734468256dea541ab89060326e`. It includes [PR #202](https://github.com/kolotiloff/agent-messenger/pull/202) and [PR #204](https://github.com/kolotiloff/agent-messenger/pull/204). All four [release jobs](https://github.com/kolotiloff/agent-messenger/actions/runs/45626722653) passed. The public release contains 22 assets. The public upgrade route from 0.99 and signed archives were checked for macOS ARM64 and Windows and Linux on both architectures. No owner installation was updated as part of the release.

- T-304, T-306 and T-307 mute idle agents, show waiting tasks in yellow, keep computer names and role labels on one baseline and center the version and icons.
- T-308 separates computer contact from agent report freshness. Current worker evidence moves the matching task out of Planned while it waits for the launch event and its timestamp. Go fixtures cover delayed launch, stale worker evidence, and an active SQLite writer. Chromium and WebKit fixtures cover computer cards, the task list, and the open thread. The installation that reported the issue was not inspected. See D-215.

**T-305 refreshes the public GitHub page for 1.0.3.** The English screenshots use the released window assets and sample devices, projects and tasks. The landing page and its installation, network, task and update guides describe the released behavior.

**[1.0.3, build 106](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.3-preview.106) is released** from `bb9afce6f290b4c029669dc8c19efcb8ddb67975`. [PR #200](https://github.com/kolotiloff/agent-messenger/pull/200) and its eight [client checks](https://github.com/kolotiloff/agent-messenger/actions/runs/45570210886) passed. All four [release jobs](https://github.com/kolotiloff/agent-messenger/actions/runs/45572340945) passed, including discovery from 0.99. The public release has 22 uploaded assets. An independent check verified the public upgrade route, signatures, archive hashes and source revision for all five targets. Verification used fixture devices and did not install the update on the owner's computers.

- T-303 replaces the composer and task context menu actions with the stop request status. An explicit refusal restores the composer and its draft.
- T-301 lets coordinator `wait` return a ready report while SQLite has a concurrent writer. T-302 shows a compact count for parallel tasks with expandable model and timing details.
- T-296 synchronizes the confirmed update response, its notification and the computer card. T-298 keeps exact cancellation available for a queued update during a delayed roster refresh. T-297 shows notice tooltips only when the text is clipped. T-299 prunes update helper logs older than seven days while preserving current output and useful recent diagnostics.

**T-300 is closed.** The owner removed idle Codex wake-up from the plan after the probe confirmed that it needs access to the App Server for the same chat. The current desktop setup did not expose the default control socket. See D-208.

**[1.0.2, build 105](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.2-preview.105) is released** from `41dc97c4b35a943e576d0c362b1638ae1c581d81`. [PR #198](https://github.com/kolotiloff/agent-messenger/pull/198) is merged and its eight [client checks](https://github.com/kolotiloff/agent-messenger/actions/runs/45519001699) passed. The public release has 22 uploaded files. Both the local check and the [release workflow](https://github.com/kolotiloff/agent-messenger/actions/runs/45521441937/attempts/2) verified the signed upgrade route to 1.0.2 on all five targets. The workflow's channel job passed after a retry for an API rate limit.

This release adds T-290 through T-295. The coordinator gets a scoped advisory after prolonged unexplained Codex silence and can read a running command when the CLI reports it. The existing completed-turn exit warning is preserved. The task context menu stops the clicked eligible task with the same pending guard as the chat. Computer cards use equal-sized green dots and red crosses. The Mac menu bar uses primary click to open the window and secondary click for its menu. The update controls show a pending cancellation, use a download arrow with a tray and have consistent short tooltips.

Browser fixtures passed in WebKit and Chromium. The macOS desktop bridge passed the isolated start and reopen check. Live menu bar clicks, the original reported Windows run, and installation on the owner's fleet were not part of these checks.

**[1.0.1](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.1-preview.104) is released** from `f2f6f4ce92740eb6a9973d8b8730ea0462005c47`. All eight [client checks](https://github.com/kolotiloff/agent-messenger/actions/runs/45497895256/attempts/2) and all four [release jobs](https://github.com/kolotiloff/agent-messenger/actions/runs/45504161598) passed. The public release has 22 assets and the signed upgrade check passed for all five targets. The installed fleet has not been upgraded as part of this release task.

It includes Planned tasks, the remote stop delivery status, the synchronized update queue with five-minute expiry, the local computer stripe, and the confirmed model label. T-289 fixes three update state regressions. See [PR #196](https://github.com/kolotiloff/agent-messenger/pull/196) and the [audit report](reports/2026-11-25-t289-release-audit.md).

The earlier [1.0 release](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.0-preview.103) passed the [public upgrade check](https://github.com/kolotiloff/agent-messenger/actions/runs/45459724676) for all five platforms. The original macOS 0.99 and 0.101 binaries each downloaded the signed release in isolated homes. The owner’s installed 0.99 client also staged 1.0.

Updated 2026-11-24 by Codex (T-283). The original 0.99 binary still returned current after 0.101 shipped. Added a public-channel check to the release workflow and an opt-in recovery for legacy discovery. Fleet delivery requires verification after publication.

Updated 2026-11-24 by Codex (T-282).

- **0.101 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.101), [workflow](https://github.com/kolotiloff/agent-messenger/actions/runs/45437843015/attempts/2), [PR #187](https://github.com/kolotiloff/agent-messenger/pull/187), source `e84576b90cc5f1159836f8663ae0eb20ffdfd230`). The updater chooses signed packages by numeric build. The live GitHub response still placed 0.101 after 0.99 and 0.98. The unmodified 0.99 updater reports 0.99 as current. The patched test binary found signed 0.101 in a separate home directory. Affected installations need the official 0.101 installer once. No live computer was modified. See [UPDATES.md](UPDATES.md).

Updated 2026-11-24 by Codex (T-280).

- **0.100 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.100), [workflow](https://github.com/kolotiloff/agent-messenger/actions/runs/45428605429), [PR #185](https://github.com/kolotiloff/agent-messenger/pull/185), source `8ca1dfe91455fcd9d0a6b3fb561ce0d2ec739d5b`). The coordinator can cancel an exact queued update without stopping workers. A verified already-installed target clears its waiting row before checking live work. Confirmed 22 uploaded assets and verified the signatures, revision, sizes and hashes for all five update sets. PC-1 reported 0.99 and update pending when inspected before publication. No live computer or database was modified. See [UPDATES.md](UPDATES.md).
- After 1.0, T-281 shows model text in computer cards only once the worker has reported it. A comma separates the agent label from its elapsed time or waiting status. Browser checks cover the label before and after a model is reported.

Updated 2026-11-24 by Claude Code (T-278, T-279).

- On `claude/amazing-noether-apbflv`, not released: T-278 (D-192), an old-network task no longer shows as the task of the computer that now has its old peer id (the owner saw ПК-2's tasks on the VPS card); T-279, choosing a computer no longer flashes an empty list and pane. Checked with Go tests (`old_network_test.go`), `scripts/check.sh` and the window checks in Chromium; WebKit runs in CI; not seen in the installed app. The 0.98 release notes above are also only on this branch until a PR is opened.

Updated 2026-11-24 by Claude Code (0.98 release).

- **0.98 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.98), [workflow](https://github.com/kolotiloff/agent-messenger/actions/runs/45369041530), [PR #183](https://github.com/kolotiloff/agent-messenger/pull/183), source `1fac081985e74068b2f78c98cd72deca471478df`) with T-268 to T-277 below. Confirmed 22 assets, as in 0.97. Verified all five update manifests against the pinned key (a changed manifest fails), their version and source SHA, and that each archive hash and length names a published asset. The live Mac and VPS were not upgraded, and the automatic update (T-277) has not yet run against a real release on a real computer. The update icon and marks stay as they are for now (Q-031).

Updated 2026-11-24 by Claude Code (T-277).

- On `claude/amazing-noether-apbflv`, not released: «Автообновление» in «Настройки программы» (D-191). Windows, Linux and the server service queue their own update once per newer release and install it when no task runs. Checked with Go tests (`autoupdate_test.go`, with a fake release finder and helper) and the window check in Chromium. Not checked: a real release found on GitHub and installed this way on a real computer, WebKit, the Mac (no switch there; Q-031).

Updated 2026-11-24 by Claude Code (T-272 to T-276).

- On `claude/amazing-noether-apbflv`, not released, per D-190: the update's work count reads only current tasks through state indexes (T-272); the computer order survives a report without the lasting id (T-273); readable model names in the statistics filter (T-274); a hidden window stops redrawing and polling statistics (T-275); no «·» between words (T-276). Checked with Go tests and the window checks in Chromium; WebKit is not installed here and runs in CI; nothing seen on the Mac yet.
- Audit notes not acted on: the change watch reads two indexed rows every 250 ms while the window is open (cheap); the persistent Claude stream loop ticks every 50 ms; discovery every 60 s only looks up files. Each could be slowed further if the owner sees load.

Updated 2026-11-24 by Claude Code (T-268 to T-271).

- On `claude/amazing-noether-apbflv`, not released: the picture viewer (T-268), one-height statistics filters from the left (T-269), the project page as four cards (T-270), and no computer chip anywhere (T-271), per D-189. Checked in Chromium with fictional data, including the new `scripts/check-window-pictures.cjs`; WebKit runs in CI only; not tried in the installed app.

Updated 2026-11-23 by Claude Code (0.97 release).

- **0.97 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.97), [workflow](https://github.com/kolotiloff/agent-messenger/actions/runs/45269963629), [PR #182](https://github.com/kolotiloff/agent-messenger/pull/182), source `5de88fcc8d4c7c38e5a0bd395b790c727e5025f9`) with T-264 to T-267 below. Confirmed 22 assets, as in 0.96. Verified all five update manifests against the pinned key (a corrupted signature fails), their version and source SHA, and that each archive hash and length names a published asset. The live Mac and VPS were not upgraded. Next: T-268 (picture viewer); a draft patch was kept outside the repository and must be redone on a branch.

Updated 2026-11-23 by Claude Code (T-267).

- T-267 on `claude/amazing-noether-apbflv`, not released: review fixes for the 0.95 per-project storage (D-188). An unroutable network message no longer stops the inbox; a half-created task no longer blocks a resource and is closed at the next start; a failed activation keeps the single database. Go tests with fixture databases only.

Updated 2026-11-23 by Claude Code (T-266).

- T-266 on `claude/amazing-noether-apbflv`, not released: statistics and the task list no longer empty while new data loads, so blocks and scroll bars stop jumping. Checked in Chromium with fictional data only.

Updated 2026-11-23 by Claude Code (T-265).

- T-265 on `claude/amazing-noether-apbflv`, not released: switching projects shows a delayed spinner instead of flashing the welcome; the welcome appears only until the computer has seen a task or serves the network; an empty project says «Задач пока нет». Checked in Chromium with fictional data; not tried in the installed app.

Updated 2026-11-23 by Claude Code (T-264).

- Reviewed the 0.95 and 0.96 commits. The push checks on `main` for 0.96 ([run](https://github.com/kolotiloff/agent-messenger/actions/runs/45224688763)) failed in three jobs, and the PR #180 Mac job failed in the package install check. Neither is a product regression: the `core` race tests take 520-600 s against a 10-minute limit, and `scripts/check-package.py` did not wait for project storage preparation. T-264 fixes both in the scripts and workflow limits. The Mac package check runs only on macOS CI and was not run here. Open PRs #177 and #178 look superseded by #176 (0.95); the owner decides whether to close them.

Updated 2026-11-23 by Codex (0.96 release).

- **0.96 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.96), [workflow](https://github.com/kolotiloff/agent-messenger/actions/runs/45224882224/attempts/3), [PR #180](https://github.com/kolotiloff/agent-messenger/pull/180), source `b28a5cb8a3ef8f4447e0e4dc05d1bc27b6b3e31e`). T-262 serializes broker writes. T-263 keeps client database access usable after SQLite error 517 and distinguishes a failed project read from an empty list. The first Linux race-test run hit its suite timeout, and the first signing job did not acquire a Mac runner. GitHub successfully retried those jobs. Confirmed all 22 published assets, the release tag and Latest designation. Independently verified all five update manifests and archives against the pinned key, source SHA, lengths and SHA-256 hashes. The live Mac and VPS were not upgraded in this release run.

Updated 2026-11-23 by Codex (T-263).

- The client now finishes each indexed window revision read before reusing its SQLite connection. Cancellation can still stop a request while it waits for a connection. The store discards connections that return SQLite 517 and retries only if `BEGIN` failed. If the first view cannot be read, the project selector shows that projects are temporarily unavailable. The regression fixture retains a running task in a legacy queue while activation is pending. Verified the fixture and an actual SQLite 517 recovery, plus Chromium and WebKit. The installed application and release assets have not changed.

Updated 2026-11-23 by Codex (T-262).

- The owner recovered the VPS with the 0.95 installer. A separate persistent error 517 on the Mac prevented network messages and the window from reading local state. A normal restart restored both projects, the saved task history and the live computer list. Split activation remains pending behind an uncertain stream reservation. The cause of the persistent Mac snapshot error remains open. T-262 addresses a separately reproduced broker mailbox race and the macOS race suite passed.

Updated 2026-11-23 by Codex.

- **0.95 was released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.95), PR #176, source `aa9809b7c6410822b86eb2405183af04d2ba6385`). Normal startup selects verified per-project databases when safe and keeps the original queue. An isolated 188 MB migration and a mixed-version broker exchange passed. The release included T-250 through T-261. See [PROJECT_STORAGE.md](PROJECT_STORAGE.md).

Updated 2026-11-22 by Codex.

- **0.94 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.94), [PR #167](https://github.com/kolotiloff/agent-messenger/pull/167), source `d0e3a4ec658570e1a91387bb51a86e61ad2163e5`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/45067378364)). T-240 refreshes Windows and Linux windows after remote updates, T-246 balances statistics sort padding, T-247 estimates otherwise missing remote task durations, T-248 attributes coordinator follow-ups and T-249 alerts the coordinator when a completed Codex turn leaves its process running. Verified all 22 published assets and all five update manifests and archives against their signatures, revisions, sizes and SHA-256 digests. The separate [checks run](https://github.com/kolotiloff/agent-messenger/actions/runs/45065644594) failed the WebKit sort-padding test because the test tried to serialize a CSSStyleDeclaration directly. Reading the individual style values passed in Chromium and WebKit. Live desktop update acceptance is pending.

- T-245 follow-up: the public screenshots now use the app's English interface and fictional tasks for Claude Code, Codex and Cursor. Both themes, the statistics view and the first-launch screen were captured in Chromium from 0.93.

- **0.93 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.93), [PR #159](https://github.com/kolotiloff/agent-messenger/pull/159), source `efd0e835689c65f4936c3848f1f01e0c20cdf882`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/45026379107)). T-241 adds elapsed time and sortable statistics columns. T-242 distinguishes Codex reconnects from unconfirmed runs. T-243 shows the assigning agent, its icon and its model when supplied. T-244 saves the local computer-card order. T-245 refreshes the GitHub overview and screenshots. [CI](https://github.com/kolotiloff/agent-messenger/actions/runs/45025034516) passed. Confirmed 22 published assets and verified the five update manifest signatures, revisions, archive sizes and SHA-256 digests. No installed-app update or live-agent acceptance run.

- **0.92 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.92), [PR #154](https://github.com/kolotiloff/agent-messenger/pull/154), source `644d7b8b092855eff7705c4dd637f1e61b9892ac`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/44816188665)). T-237 places the amber waiting status inline and removes the extra dot in task cards. T-238 aligns title typography. T-239 labels this computer «координатор» for the selected project while its active coordinator chat is alive. Network routing still sends task events to the assigning computer. [CI](https://github.com/kolotiloff/agent-messenger/actions/runs/44813843278) passed. Confirmed all 22 assets were uploaded and verified the five update manifests against the app public key, release revision and the published archive sizes and SHA-256 digests. No installed-app update or live-agent acceptance run.
- **0.91 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.91), [PR #152](https://github.com/kolotiloff/agent-messenger/pull/152), source `e49c54f43267903345bb753a8ee98bf147f0f6fb`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/44800060649)). T-235 matches the sidebar activity color to the task list without animation. T-236 shows running work alongside waits, counts both, and forwards worker-reported models to the assigning computer. Checks: [release source](https://github.com/kolotiloff/agent-messenger/actions/runs/44799969990). Confirmed all 22 assets were uploaded and verified the five update manifests and archives against the app public key, release revision, sizes and SHA-256. No installed-app update or live-agent acceptance run.
- **0.90 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.90), [PR #150](https://github.com/kolotiloff/agent-messenger/pull/150), source `cd921027de8b3bc44b77c588e67023b709379806`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/44762588584)). T-234 uses 12 px in sidebar agent rows and task cards. The sidebar shows a muted duration after a comma beside the running model. The release source passed [CI](https://github.com/kolotiloff/agent-messenger/actions/runs/44762493265). Confirmed all 22 assets were uploaded. Verified the five update manifests and archives against the app public key, release revision, sizes and SHA-256. No installed-app update or live-agent acceptance run.
- **0.89 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.89), [PR #147](https://github.com/kolotiloff/agent-messenger/pull/147), [PR #148](https://github.com/kolotiloff/agent-messenger/pull/148), source `8ff9acc50274bec8b1502e9b126f1564e7c3da4c`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/44753461036)). T-229 highlights working agents with an elapsed label. T-230 updates the reported computer version after confirmation. T-231 synchronizes task status, model and timing across the window. T-232 keeps live command timing on the latest action group and uses muted headings with a neutral sweep on the running label. T-233 lightens conversation typography. The release source passed CI ([checks](https://github.com/kolotiloff/agent-messenger/actions/runs/44753388609)). Confirmed all 22 assets were uploaded. Verified the five update manifests and archives against the app public key, release revision, sizes and SHA-256. No installed desktop update or live multi-computer acceptance run.

- **0.88 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.88), [PR #145](https://github.com/kolotiloff/agent-messenger/pull/145), source `d81bbc4d2cd1cf28ea2bf076ca0852df09280e7f`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/44708744170)). T-226 adds command group status and a live timer. T-227 adds on-demand CLI versions. T-228 gives the update a separate amber arrow. The combined macOS race suite and Chromium window fixtures passed. Verified all 22 uploaded assets and the signatures, revisions and referenced archive sizes of all five update manifests. No installed desktop update or live-agent acceptance run.

- **0.87 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.87), [PR #140](https://github.com/kolotiloff/agent-messenger/pull/140), source `7fd6c198363545ba218510fe04ffd8ecff8b43e5`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/44647765968)). T-223 adds session event and task finish times. T-224 queues owner notes for remote workers. T-225 adds readable action captions, icons and grouped summaries. The combined macOS race suite and Chromium window fixtures passed. Verified all 22 uploaded assets and the signatures, revisions and referenced archive sizes of all five update manifests. No installed desktop update or live-agent acceptance run.

Updated 2026-11-19 by Codex.

- T-222 refreshes the public README, the platform install and update guides, and the light, dark and first-run screenshots from the 0.86 interface. Repository About now describes the current product. No program changes.
- **0.86 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.86), source `55e31e2b54d01cad088c2675786b1f1879e81b82`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/44499053023)). T-215–T-218 in [PR #132](https://github.com/kolotiloff/agent-messenger/pull/132), T-219 in [PR #133](https://github.com/kolotiloff/agent-messenger/pull/133) and T-220–T-221 in [PR #134](https://github.com/kolotiloff/agent-messenger/pull/134). Larger text, unified statistics and model labels, busy-agent model and elapsed time, and aligned update and disconnect controls. Computer names use the full line height. Verified 22 uploaded assets and all five manifest signatures, revisions and referenced archive sizes. Chromium preview uses fictional data; no installed-app update or multi-computer acceptance run.
- Released [0.85](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.85) from `5e49856` via PR #130 (release run 44432670621): T-203, T-204, T-205 (D-147), T-207, T-209, T-210, T-212, T-213, T-214, and parts of T-208 and T-211. 22 assets, five update feeds at build 85 and the Linux x64 checksum and `version` checked. CI "Window controls" was cancelled three times at the browser install (slow Ubuntu mirror, 10-minute limit); the same check passed locally in Chromium at the PR head. Not verified here: Swift (CI only), the Windows job code and the GTK window. On the Mac Codex may need connecting again once in its card; a 0.62 window process on Linux is closed by hand once; a Mac on 0.83 or 0.84 on a slow link installs 0.85 by hand. Open: T-206 (network drops, needs the server log), the rest of T-208 and T-211 (stale `running:1`).
- **0.84 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.84), [PR #129](https://github.com/kolotiloff/agent-messenger/pull/129), source `5d04fd698f0256108785f213deec6bf06792bccb`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/44349253768)) with T-200 (this computer's card leaves out paused agents), T-201 (statistics by agent and model) and T-202 (the model after the status in task rows). The publish job got no Mac runner twice (cancelled after 15 minutes in the queue, nothing published); the third attempt published. 22 assets; the five update feeds name build 84 and this revision; the Linux x64 program matches the checksum and reports the revision. All PR checks passed. The push run on `main` lost its `macos-15 / Go stable` job the same way (no runner). Not tried on the owner's computers; 0.83 → 0.84 is the first Mac update through the page's own dialog (T-199).
- **0.83 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.83), [PR #127](https://github.com/kolotiloff/agent-messenger/pull/127), source `1ab24150114f9556e516f582b41509efc2cf8eed`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/44332389459)) with T-198 (computer cards), T-197 (statistics sort), T-184 (tray, D-146) and T-199 (Mac update in the page's dialog, D-146). 22 assets published; the five update feeds name build 83 and this revision; the Linux x64 program matches the checksum and reports the revision. All PR checks passed, including the Mac native build. Not tried on the owner's computers: the Windows tray, the Mac menu bar icon; the new Mac update dialog first shows on 0.83 → 0.84.

Updated 2026-11-18 by Claude Code.

- Same branch, for 0.83 with T-198: T-197 (statistics «Затратные / Последние»), T-184 (tray on all three systems, one window per computer, D-146), T-199 (the Mac update in the page's dialog, no Sparkle windows, D-146). Windows code cross-built only; Linux tray run under Xvfb; Mac Swift compiled by CI only. After installing 0.83 on the Mac, the first update with the new dialog is 0.83 → 0.84.
- T-198 on `claude/eager-volta-6sbjsa`, not released: computer cards in the left panel have the dot before the name (centred on the capitals), the version top right and the update icon bottom right, the same for this computer; the version and update button left the bottom of the sidebar. Chromium check and screenshots with fictional data only. Goes into the next release with T-197.

Updated 2026-11-18 by Codex.

- **0.82 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.82), [PR #125](https://github.com/kolotiloff/agent-messenger/pull/125), source `f1f5c6132a4b24f4792791b3c8a5969d6ee1d420`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/44295419912)). Verified 22 published assets and five signed update feeds and archives. T-195 adds Sol 6.2 Low/Medium/High and Astra 6.2 Medium/High alongside Luna High. T-196 uses the coordinator's local model lists for updated workers. T-192 keeps the project menu open on refresh. T-193 sends worker presence when a run starts or ends. Final local race checks, Chromium/WebKit fixtures and [PR checks](https://github.com/kolotiloff/agent-messenger/actions/runs/44293474608) passed. No production model invocation or multi-computer acceptance run. Next: statistics sort by highest tokens or most recent, T-184 tray and the Mac update dialog.

- **0.81 is released** (PR #122, `main` 204b319, release run 44040632883) with T-194 (D-144), the yellow icon with an eyed bubble. The owner updated the Mac to 0.81 and still saw the old icon: likely the macOS icon cache (Sparkle replaced the bundle in place); told to `touch` the app, run `lsregister -f` and restart the Dock. Not yet confirmed. Next: T-192, T-193.
- **0.80 is released** (PR #121, `main` 9c36dd3, release run 43917733249) with T-189, T-190, T-191. Next, from the owner's remarks of 11-16: T-192 (project list closes itself on refresh), T-193 (agent states of other computers lag the task list). Then T-184 (tray), the Mac update dialog. After installing 0.80 on the Mac, check that `list_agents` shows Codex with `source: "check"`.
- T-189 (D-141), T-190 (D-142) on `claude/great-knuth-52qwv1`, released in 0.80: feedback export takes only new remarks and the journal can be cleared; a restarted coordinator chat is no longer told about another chat. T-191 (D-143): Codex limits read from the account every 10 minutes in percent (both windows), shown in `list_agents` and to the other computers; the coordinator is told to spread work by what is left. Not yet tried against a real Codex account: check on the Mac after 0.80 (journal line «codex limits not read» means it failed; `list_agents` shows `source: "check"` when it works). Claude/Cursor percent: no source found.
- 0.79 released (PR #120, `main` e7f22f0, run 43756475974) with T-187 (D-140) and T-188: one update button with a download/install/restart dialog (Windows/Linux; Mac shows the icon only when ready), and the task list toggle without a double background.
- 0.78 released (PR #119, `main` 875b8ce, run 43747998585) with T-183, T-185, T-186. The owner updated the VPS to 0.78 over SSH; the first real button update of the VPS is 0.78 → 0.79. Next: T-184 (tray); the update dialog for the Mac (Sparkle still shows its own window).
- T-186, in 0.78: shared tasks say which computer runs them (`runs_on_peer`, needs a 0.78 broker: the VPS must be updated once over SSH with the installer, 0.77 cannot update remotely); rows read «Cursor на ПК-1 — от Mac». Agent removal opens a terminal script (y/N, keeps sign-in); «Убрать все» clears finished tasks. The PowerShell removal script was not run here; check on Windows.
- T-185, in 0.78: a paused or removed agent is no longer listed for a computer just because its chat once used Messenger's tools as the coordinator (the likely cause of ПК-2 still showing a disabled agent).
- T-183 (D-139), in 0.78: the VPS server takes update requests from the window under systemd (helper installs, writes the result, then `systemctl restart --no-block`); `agent-messenger-server rename --name ИМЯ`; the Windows frameless caption row is 44 px so the tabs and the task-list toggle have a gap above them. The owner's VPS runs 0.77, which refuses remote updates: it must be updated to 0.78 once over SSH with the installer; after that the window's arrow works.
- **0.77 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.77), [PR #118](https://github.com/kolotiloff/agent-messenger/pull/118), source `67090584835801a57294204dbc24deb449f2a232`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/43724921881)). The installer downloads from `releases/latest` and the Linux program matched the release checksum.
- 2026-11-15, owner's acceptance of 0.77: `install-server.sh` installed the server on a new Debian VPS (systemd, internet scope, public address found by itself, invitation printed); TCP 47100 reachable from the Mac; all 5 computers (Мак, ПК-1, ПК-2, Ноут-Windows, Ноут-Linux) joined it over the internet and report in `agent-messenger-server status`. A task between computers through it was not yet confirmed.
- T-036 (D-138), released in 0.77: a server on a Linux VPS installs with one command, `curl -fsSL https://github.com/kolotiloff/agent-messenger/releases/latest/download/install-server.sh | sh` (systemd service, internet scope, invitation printed); `agent-messenger-server invite/status` afterwards; `messenger server …` commands for any data directory. The server computer is labelled «сервер» in the window's computer list and `server: true` in `list_agents`. The owner dropped the «server only» idea. Checked in a Linux container with a stand-in systemd only; the owner's VPS (Debian 13) still runs an older client started with `start` in its old data directory, which must be stopped with `shutdown` before installing. The earlier «Действие не подтверждено» on «Запустить сервер» through the SSH tunnel was not diagnosed (no output from the VPS).
- **0.76 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.76), [PR #117](https://github.com/kolotiloff/agent-messenger/pull/117), source `a10535f0cf810227e23ff387cdbe158db094ac27`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/43704189375), 21 assets) with T-182 (D-137): the server's «Интернет» scope (external address, public peers within limits, six-digit code local only). Not tried from another network; not installed on the owner's computers. The owner's server will be a home computer with a public address; the owner forwards the port.
- **0.75 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.75), [PR #116](https://github.com/kolotiloff/agent-messenger/pull/116), source `fd341362bd70f34c51dcc3c284eaabf13867de85`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/43694772945), 21 assets) with T-181, T-158 and T-159. Not installed on the owner's computers. CI on #116 first failed in WebKit on a reload-cut background request reported as a page error; the window check now ignores only that message.
- T-158 and T-159 released in 0.75: `list_agents` shows the newest downloaded release with its notes and `update_available` (D-136); `continue_task` continues an interrupted task of another computer once that computer confirms the worker is gone (D-135, needs 0.75 there). T-160 confirmed by the owner on real computers.
- T-181 released in 0.75: notices above the reply box have no scroll of their own; at most 3 are shown and a new one pushes out the oldest; long texts are cut to 3 lines. Goes into the next release.
- **0.74 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.74), [PR #115](https://github.com/kolotiloff/agent-messenger/pull/115), source `825fb7686eee8b009fad46b73efa55c304736487`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/43648585334)) with T-179 and T-180. Not installed on the owner's computers.
- Released in 0.74: T-179 (review of another agent's 0.73 work: asleep computer's mail no longer retired, update icon fills a ring from the real download progress, refusal reasons in the journal, Windows task-list toggle beside the window buttons) and T-180 (project names must match exactly: said in the window, the refusal and `list_agents`). Still to review: T-160 (Codex as coordinator).
- **0.73 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.73), [PR #114](https://github.com/kolotiloff/agent-messenger/pull/114), source `8bb64e236c2a308befb5eb9118de34180e9b38b5`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/43641508174)). It holds #108–#114: T-155, the MCP-plugin section, the update ring, grey closed-task line, T-173, T-174, T-172, T-175, T-177, T-178. 21 assets published; only the release run's own checks verified them. No installation on the owner's computers was performed; the Mac connecting screen and pictures in the window still need the owner's look. CI on #114 first caught a race in the T-172 test (fixed: the linger interval is atomic).
- Next: T-176 (planned tasks, large), T-157 (needs the owner's answer on how the giver role moves).
- T-175 (D-134) (released in 0.73): workers attach pictures to `report`/`ask`/`ask_owner` (`images`); the window shows them, the coordinator reads them with `task_images`; they travel to the computer that gave the task.
- T-177 (released in 0.73): one `network_lost` coordinator event per project when contact with every other computer is lost, `network_back` when it returns. Nothing else changes; server failover stays deferred.
- T-172 (released in 0.73): a run whose worker wrote its final message but did not end is described (time of the final message, processes of its group, quiet minutes) in the task header and in `tasks`; it is never closed on that (Q-030 still open).
- PRs #108–#113 merged into `main` (conflicts were only in HANDOFF/LOG, plus the update icon in `app.js` and the presence fallback order in `broker/service.go`; full `check.sh` passed on the #113 merge).
- T-178 (released in 0.73): the Mac connecting screen shows only a spinner; retry, reason and note only after a failure; «Выбрать каталог данных…» only in the «Файл» menu. Not compiled here; Mac CI and the owner's Mac check it. Goes into the next release with other tweaks. (Its first commit was titled T-172 before #112 gave that ID to another task.)

Updated 2026-11-14 by Cursor.

- T-173 and T-174 on `cursor/t173-t174-models-mailbox-bfd1`, not released. Ноут-Linux on 0.72 kept its model lists; the Mac's `list_agents` showed them empty, and the journal repeated that the mailbox of a computer not in the project was full. A presence report now keeps the model names when some other field is refused, and messages to a computer that has left are retired after the reconnect grace. A listed computer's full mailbox is unchanged. Not checked between the owner's computers.

- Read the coordinator's issue list and the Mac feedback export of 2026-11-14 (35 remarks, through 0.72). Already done and not reopened: effort on the model (T-171, 0.72); the export button no longer jumps (T-152); a stale «exhausted» limit clears (T-138); a late report and a delayed outbox are shown (T-150); a remote worker and notes to it (T-149); note text says a note waits for the run (T-148); Codex does not wake an idle chat (T-161, D-124); a separate folder per worker was declined (Q-027). Still open, now with task ids: T-172 a finished run that never ends, T-173 model lists cleared by the 0.72 update, T-174 the same full-mailbox line every few seconds, T-175 pictures in reports, T-176 planned tasks, T-177 one event when every computer is lost. Server failover stays deferred (owner, 2026-11-13). T-157, T-158 and T-159 were already open; the journal confirms T-159's lock. Q-030 asks before anyone closes a run that still has child processes.

- On `cursor/report-closed-text-bfd1`, not released: the line «Отчёт отправлен координатору, сессия закрыта» under a finished report is ordinary grey text, the same size as the message. The Mac window was not opened here.

- On `cursor/update-download-spinner-bfd1`, not released: the update icon stays an arrow while a release is ready to install. While this computer downloads or installs, and while another computer's update is under way, that circle shows a spinning ring. The other computer does not report how far the download is. The Mac app itself was not opened here.

- On `cursor/mcp-plugin-heading-bfd1`, not released: the project page section that linked Claude Code, Codex and Cursor no longer says «Координатор?». It is a section «MCP-плагин» with the hint «Подключение к графическому редактору». The three editors and their statuses are unchanged. T-155 (`update_messenger`) is a separate PR.

Updated 2026-11-14 by Cursor (T-155).

- T-155 (D-133) on `cursor/t155-update-messenger-bfd1`, not released. `update_messenger` asks a Windows or Linux computer (this one or another of the network) to install the newest Messenger, the program only. Running tasks are never interrupted: the computer answers queued, takes no new tasks, shows «ждёт обновления», and installs as soon as none runs. A Mac answers that its update needs the button in its app. Result: `messenger_update` event and a window notice. Nothing is retried. The window's update icon uses the same queue (the leftover of T-128). Verified: core, supervisor, MCP, broker presence fallback, `check.sh`. Not verified: a real install between computers, the Mac app-button path, the window on the Mac. Next after this PR: T-156.
- **0.72 is released** (see below). Next planned features were T-155 and T-156; T-155 is this branch.

Updated 2026-11-14 by Codex.

- **0.72 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.72), [PR #106](https://github.com/kolotiloff/agent-messenger/pull/106), source `f3bd2d45e9f18890419e09996bb7a97bb3b009bd`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/43600787465)). All 21 public assets and five signed update feeds/archives verified, including source revision, signatures, sizes and checksums. No installation on the owner's computers was performed.
- Released together: T-168 closed-install-terminal recovery and T-169 Cursor sidebar deduplication from [PR #105](https://github.com/kolotiloff/agent-messenger/pull/105); T-170 first login with a missing standard profile and actionable path errors; T-171 researched model/effort selections, native argv mapping and coordinator guidance. Exact checked PR #106 head `baf18d23021164c4babc4b96f381c1a15a5f6543`; all [CI jobs](https://github.com/kolotiloff/agent-messenger/actions/runs/43599668821) passed, including required Windows first-login/model/migration/presence tests and Mac native checks. Local full race and Chromium/WebKit tests passed.
- See [setup/sidebar report](reports/2026-11-14-t168-t169.md), [first-login report](reports/2026-11-14-t170.md), [models report](reports/2026-11-14-t171.md) and [model catalog/commands](MODELS.md). Upgrade the server, coordinator computer and workers to 0.72, then reconnect Messenger in the coordinator chat. Old servers retain presence but omit unsupported effort selections. Only unchanged previous starter lists migrate; custom/empty lists and recorded tasks are preserved.
- The acquaintance's exact Windows account flow is still unconfirmed. The reproduced missing-profile validation blocker is fixed; no real account login, installer, model or working chat was tested. If sign-in still fails after updating, obtain the detailed message and redacted Windows journal. Release record branch: `codex/t170-t171-release-72`.
- Owner asked how to send logs. Feedback export contains remarks/images only; terminal authentication output is not saved. The initial Mac-path guidance was for the wrong computer; Cursor authorization concerns Windows. Request only the relevant Messenger journal and a redacted terminal error, not credentials or a whole profile.

- **0.71 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.71), [PR #103](https://github.com/kolotiloff/agent-messenger/pull/103), source `daae5642bf80fc6193a05fa143678ecf0bcca1cc`, [release run](https://github.com/kolotiloff/agent-messenger/actions/runs/43591866974)). All 21 public assets and five signed update feeds/archives verified, including source revision, sizes and checksums. No installation on the owner's computers was performed.
- Released together: T-153 terminal installation/find for all three CLIs; T-163 theme/language selection; T-164 welcome before the first project; T-165 own-CLI folder-trust confirmation; T-166 interactive terminal launch and visible/copyable sign-in errors; T-167 repeated public discovery requests and explicit unavailable-UDP fallback for six-digit joining. See [setup/network report](reports/2026-11-14-t153-t165-t167.md) and [UI report](reports/2026-11-14-t163-t164.md).
- Local full race checks, cross-builds and Chromium/WebKit checks passed. [PR CI](https://github.com/kolotiloff/agent-messenger/actions/runs/43590990501) covers macOS/Linux, Windows runtime/setup/discovery/installation and native Mac build/lifecycle. No real installer, login, model or working-chat test was run. The acquaintance's exact sign-in and two-Windows-machine LAN failures remain unconfirmed; recheck them after upgrading both computers.
- Cursor follow-up: the owner reported missing coordinator tools. Read-only inspection in Cursor 3.30.14 found the selected project's `messenger` source explicitly Disabled, with its Enable switch off. The file and executable existed. The owner was shown Customize -> MCPs -> messenger -> Enable for the project. No approval setting or working chat was changed; `messenger_task` having no worker tools outside a task is expected. The owner reports the Codex coordinator works; real Cursor waiting/question/report acceptance remains separate.
- Next planned features: T-155 and T-156; continue the ordered backlog in TASKS.md. Release-record branch: `codex/t153-t167-release-71`.

### Previous release: 0.70

- **0.70 is released** ([release](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.70), [PR #101](https://github.com/kolotiloff/agent-messenger/pull/101), source `4b7c687eda006e5556536fd18ddbb35d9bcd2670`, [run 43584251128](https://github.com/kolotiloff/agent-messenger/actions/runs/43584251128)). All 21 public assets were downloaded and checked; all five update manifests name build 70 and the same source revision, with valid manifest/archive signatures and checksums. No installation on the owner's computers was performed.
- T-154: stable agent rows and action slots, previous results kept while checking, unchanged provider cards retained, discovery and a diagnostic after returning from installation. Browser checks with fictional data passed at 1100/600 px in both themes (56 px rows, 28 px actions); native visual behavior and real installation remain unverified.
- T-161 (D-124): worker events reached the Codex background watch but did not resume the idle chat. Codex and Cursor now get active-wait guidance; known Claude Code clients keep background notifications. No idle-chat wake mechanism was added. Real-model acceptance remains outstanding.
- T-162 (D-125): Cursor is offered as a coordinator on «Проект» and the welcome page. Connection writes project `.cursor/mcp.json`, preserves other settings and requires the owner's normal Cursor approval. Fake-client question/wait/answer/report tests passed; no real Cursor chat acceptance yet.
- Verified: full PR CI (macOS/Linux, Windows runtime and installation, native Mac build/lifecycle, cross-compilation), release workflow, local macOS `scripts/check.sh`, browser fixtures and the public release assets. See the [report](reports/2026-11-14-t154-t161.md).
- After updating, reconnect Messenger or start a new coordinator chat so the MCP server supplies the new instructions. Owner's next checks: Codex/Cursor keep waiting and receive a worker question and report without another owner prompt. At that release, the next planned feature was T-153; it is now released in 0.71.

Updated 2026-11-14 by Cursor.

- T-160 (D-123, PR #100, `4c8af3e`): Codex can be the project's coordinator. The «Проект» row connects Claude as before and writes only `mcp_servers.messenger` into the folder's `.codex/config.toml` for Codex. Not released. Not checked with a real Codex chat.
- 0.67 released. On `claude/docs-refresh` for 0.68 (the owner said release it): T-151 docs check, T-152 welcome page and the coordinator row on «Проект», window tweaks (dialogs close on an outside click, settings layout, sidebar icons, feedback export), T-158 part (`messenger_version` in `list_agents`).
- Coordinator feedback export of 2026-11-14 (19 remarks) read: new items recorded as T-157 (hand tasks to a coordinator on another computer), T-158 (versions and release notes for the coordinator), T-159 (a remote computer that died: needs 0.67 on the executor for `worker: none` and closing by cancel; `continue_task` for a remote interrupted task is missing). The 13:37 remark (remote progress silent for 2 h) is what T-149 in 0.67 shows as `worker_output_at`; the coordinator was still on 0.66.
- Next: T-154 cards without jumps, T-153 install in a terminal, T-155 `update_messenger`, T-156 full remote task view, T-157, T-159.

Updated 2026-11-13 by Claude Code.

- 0.66 released (PR #97, run 43317868228; the publish job was cancelled once by GitHub waiting for a Mac runner, re-run succeeded; 21 release files, the Linux feed says build 66): T-139 «не проверена» CLI mark and daily CLI watch (its first manual run passed), T-146 (D-121) worker replies parked, never dropped.
- On `claude/ui-tweaks` for the next release (0.67, not released; the owner releases): T-148 note texts say a worker reads a note only when its run ends; T-150 `late_report` and late outgoing messages shown; T-149 (D-122) notes to workers on other computers and whether a remote worker runs. Both new presence fields need the server computer on 0.67 to travel; until then clients fall back. Not checked between real computers. Recorded, not started: T-147 phone observer (not a priority). Owner said: server failover not now. Still open: server failover and an all-contact-lost event; `list_agents` lacks the «не проверена» mark; Cursor tested version not recorded.
- Coordinator feedback export of 2026-11-13 (15 remarks, 11-10 to 11-13) was read; all but the items above were already fixed in 0.61–0.66. Suspected: two Cursor workers on one computer stop each other (unconfirmed; T-141 now shows the CLI error text).

Updated 2026-11-12 by Claude Code.

- 0.65 released (PR #96): T-145 (D-120) an explicit cancel closes a stuck task where it ran. On `claude/ui-tweaks` for the next release: T-139 «не проверена» CLI mark and the daily CLI watch workflow (raise `TestedCLI` in `internal/diagnostics/tested.go` after checking a newer CLI); T-146 (D-121) a worker's reply is parked, never dropped. Open: server failover and an all-contact-lost event; `list_agents` does not show the «не проверена» mark yet. ПК-1 was on 0.62 and must be updated from its own window, then its old stuck tasks closed there.
- 0.64 released (PR #95): T-140 fold arrow, T-141 CLI error output in failures, T-142 run and network journal for `get_log`, T-143 model typo hint, T-144 «Требуют решения» and opening the newest task of a chosen computer. On `claude/ui-tweaks` for the next release: T-145 (D-120) an explicit cancel closes a stuck task where it ran when its worker is certainly gone; refused updates name the tasks. Then T-139.
- 0.63 released (PR #94, tag `v0.0.0-preview.63`; all downloads, the checksum list and the five signed update feeds checked in place). On `claude/ui-tweaks` for the next release: T-140 fold arrow, T-141 CLI error output in failures, T-142 run and network journal for `get_log`, T-143 model typo hint, T-144 «Требуют решения» section and opening the newest task of a chosen computer. Then T-139. Open from the coordinator's journal: server failover and a lost-contact event, an outcome when a worker ends with no delivered report. T-137 (D-119) a full mailbox no longer holds back a computer's other messages (Ноут-Linux's replies have not reached the Mac since 11-10); the window and `not_accepted` no longer call a possibly running task «Работает». T-138: a completed run clears a stale «exhausted» limit; `list_agents` gives `idle_computers`; the update confirmation warns about the server computer; the window no longer rebuilds unchanged parts (flicker); a remote task that never accepted or never answered a cancel closes at once; Linux: the menu entry starts the installation link, a downloaded newer file updates the installation while the client runs (`install` still needs a stopped client and says what to do).
- Owner's steps: update Ноут-Linux once the old way (window «Обновиться», or `messenger shutdown` then `install` from the download); its waiting replies then go out, and later updates work from the Mac card and by download. Delete a hand-made Linux menu entry that points at an old file. Tasks the coordinator already cancelled may get late acceptance or report lines.
- Open: lost report from ПК-1 on 11-10 (needs ПК-1's `background-*.log` from that evening); server failover (to discuss: a standby computer and a confirmed switch first); a new product name (Artel and Agent Mesh are taken); Windows update failure: waiting for the exact message.

Updated 2026-11-10 (late evening) by Claude Code.

- 0.62 released (PR #93, tag `v0.0.0-preview.62`, all downloads and signed update files in place): README for 0.61 with window screenshots; T-136 (D-118) the Windows client runs from `runtimes\messenger.exe` for every version, so the firewall permission survives updates.
- Owner's steps: on each Windows computer, after 0.62 starts, press «Разрешить доступ» once more (the new path); after that, updates do not ask again. Update the Windows server computer remotely only after it runs 0.62 and was allowed once. Not verified on a real Windows computer yet.
- ПК-2 (the server) is cut off after the remote update to 0.61: someone has to press «Разрешить доступ» in the firewall window there.

Updated 2026-11-10 (evening) by Claude Code.

- 0.61 released (PR #92, tag `v0.0.0-preview.61`, all downloads and signed update files in place): T-128 busy update icon, T-131 stuck tasks, T-132 tasks follow a computer into a new network, T-133 scroll arrow, T-134 coordinator list rows 18, 24, 26, 28, 30, 31, 32, T-135 feedback journal, plus window tweaks (agent status text in the sidebar, «Настройки программы» with language and theme).
- Owner's checks after 0.61 (every computer on 0.61): the coordinator records a remark with `report_feedback` and exports with `export_feedback`; the bug icon by the version saves a remark with a picture; tasks keep their computer after the server moves; a task not accepted in 5 minutes gives `not_accepted`. Not verified between real computers yet.
- Next after 0.61: feedback from other computers through the network; row 27 needs the owner's OK (D-021/D-108); row 29 (waking the coordinator) needs discussion; row 23 (images in reports); app settings may later hold updates and notifications.

Updated 2026-11-10 (day) by Claude Code.

- On `claude/ui-tweaks`, not released (all goes into 0.61, released after the whole coordinator list is done): T-128 busy update icon, T-131 stuck tasks (D-114), T-132 tasks follow a computer into a new network (D-115), T-134 coordinator list rows 24, 26, 28, 30, 31, 32 (D-116).
- Also on `claude/ui-tweaks`: T-133 scroll arrow, T-135 feedback journal (D-117). The owner asked to release all of it as 0.61.
- Next after 0.61: feedback from other computers through the network; row 27 (closing local interrupted tasks) needs the owner's OK because of D-021/D-108; row 29 (waking the coordinator) needs discussion; row 23 (images in reports).

Updated 2026-11-10 (morning) by Claude Code.

- Ready for 0.60 on `claude/ui-tweaks` (PR to `main` open, not released; the owner releases): T-130, T-128 (notices, update icon), T-126 (lost contact), T-115 (full access shown), T-129 (sandbox warning), T-114 (`note_task`), T-113 (continue another computer's task, D-113).
- Owner's checks after the release: update ПК-1 from the Mac card icon; turn ПК-1 off during a task and see «Связь потеряна, исход неизвестен»; ask the coordinator to write to a running worker and to continue a finished task of ПК-1 (both computers on 0.60). On the Mac, if Codex workers still have no network, quit Messenger fully and open it by hand; the window now warns when it was started from an agent sandbox.
- Next: T-128 restart icon, update that waits for tasks, continuing tasks of an earlier network (by computer name), interrupted runs on another computer.

Updated 2026-11-09 (evening) by Claude Code.

- Released in 0.59: T-111, T-112, T-117–T-125, T-127. After 0.59, on `claude/ui-tweaks`, not released: T-130 (the coordinator may answer a question to the owner, D-112). Next for 0.60: T-126 (offline and vanished computers' tasks), T-128 (notifications above the reply box, update and restart icons on computer cards), T-129 (client started inside an agent sandbox). Open: T-113, T-114, T-115.
- After 0.58: T-111 (other computers' tasks open as a read-only card) on `claude/ui-tweaks`, not released.
- 0.58 released (PR #89): model lists by default and filled once for earlier agents (T-098, T-109), questions to the owner visible to the coordinator (T-099), update checks within GitHub's limit (T-103), short version «0.58» (T-104), task list wider and wrapping (T-105), steady language/theme lists (T-106), Linux installation updates while the client runs (T-107, D-106), the worker's model next to its name (T-108), all active tasks of every computer together (T-110).
- Owner's steps: on a Linux computer installed with `install`, install 0.58 by hand once (`messenger shutdown`, then `./AgentMessenger-Linux-x64 install`, then `messenger open`).
- Open: where another computer's task runs is not shared (the 0.57 server refuses unknown fields, T-110); Linux title bar seen by the owner, waiting for a screenshot with the version; T-028 branch `claude/t028-retire-persistent` to merge and release as 0.59 with the README rewrite.

Updated 2026-11-09 (day) by Claude Code.

- T-077 (D-091): automatic restart of the old client after an update. All collected changes go to `main` and a preview release.
- preview.57 released (PR #88), with the reply box scrollbar fix: T-096 agent filter reset and hint; T-078 remote update (D-100): «Обновить программу» in another computer's menu, refused while tasks run and on a Mac, versions on the cards. Owner's test after the release: open the window once on a Windows or Linux computer (it records the program file), then update it from another computer and watch the card go «Обновляется…» → «Обновлено до …».
- preview.56 released (PR #87 merged) with T-083 to T-095 (T-089 closed without code): writing to a working worker (D-096), a new coordinator chat starts from `tasks`, full final message with `task_output` (D-098), tokens in every report, the newest coordinator chat takes over (D-099), plain failure reasons, computers listed while reconnecting, `list_agents` explanations, security fixes (D-097), task times on cards. Not verified with real agents between real computers yet: check on the Mac and ПК-1. Working from different places through a VPN is described in BROKER.md.
- preview.55 released (PR #86 merged) with: T-079 start screen (D-092), T-080 limit window and reset on the cards, the agent's raw limit fields in `list_agents` (D-093), T-081 limit warning in `start_task` (D-094), T-082 no system title bar on Mac and Linux (D-095; Mac: check dragging and double click by the top). After one real worker task of each agent, read `limit.reported` to learn whether a percent can be shown (Q-015).

- **Collected, not released** on `claude/ui-tweaks` (owner: collect changes, release later): «Пока пусто» alignment in statistics; T-074 coordinator catches up after an interrupted wait, wait 120 s by default (D-088); T-075 workers commit or ask and stay open (D-089). Language and theme lists of the same width; T-076 statistics of the whole network, filtered by the sidebar computer, another computer's agents view only, usage sent only when it changes (D-090). Not verified between real computers yet: check it on the Mac and ПК-1 after the release.

Updated 2026-11-09 (end of night) by Claude Code.

- **Released tonight:** preview.50 (T-057 log of another computer), preview.51 (T-071 last action and tokens per task, #81), preview.52 (T-070 readiness in `list_agents`, #82), preview.53 (T-072 resource locks, #83). T-073 decided without code (D-087). T-049 is finished except item 6 (Q-024).
- Backlog tidied: T-030 was already fixed with T-029 (per-route write deadline); T-019 is done by T-059 (Q-001 answered).
- **Owner's tests (not done here):** with two computers, ask the coordinator for `list_agents` (accepts tasks, project folder), start two tasks with the same `lock` on one computer and watch the second wait, and look at `tasks` for the last action and tokens. Everything was checked only with fake CLIs.
- **Waiting on the owner:** Q-024 (T-049 item 6), Q-023, Q-022, Q-018 default, Q-012 (T-028).
- **Next:** nothing in the owner's order is left. Open backlog items need the owner (T-013 signing, T-014 Intel Mac, T-028 consent) or are optional (T-017, T-036).

Updated 2026-11-08 (night, T-057) by Claude Code.

- **Released:** preview.50 (T-057 remote log), preview.51 (T-071 last action and tokens, #81), preview.52 (T-070 readiness, #82). T-072 resource locks goes out as preview.53.
- preview.48 is out (T-052 short invitation). **T-053** Windows installation merged (#79) after its CI step found and we fixed a folder that stayed on «Удалить»; goes out as preview.49.
- **T-057** log of another computer on `claude/t057-remote-log` (D-083): coordinator tool `get_log`, switch «Отдавать журнал по сети» on «Сервер», off by default. Owner's test: turn it on on one computer, ask the coordinator on another to read that computer's log.
- **T-049** split into T-070 (readiness in `list_agents`), T-071 (last action and tokens in `tasks`, done on `claude/t071-task-activity`, D-084), T-072 (resource locks), T-073 (results off the worker's computer) and Q-024 (item 6, unclear).
- **T-070** on `claude/t070-readiness` (D-085): `list_agents` shows `accepts_tasks` and `has_project` for another computer.
- **T-072** on `claude/t072-resource-locks` (D-086): `start_task` takes `lock`; the later task waits to start.
- **T-073** decided without code (D-087): reports already come back whole, files go through the project's version control.
- **Waiting on the owner:** Q-024 (T-049 item 6), Q-023, Q-018 default.

Updated 2026-11-08 (night, T-053) by Claude Code. The owner is asleep and asked to keep working alone, deciding by simplicity.

- **PR #73** (`claude/t061-agent-cards`): T-061 agent cards with the on/off switch, T-062 Windows window without the title bar; goes out as preview.43.
- **T-068** token statistics on `claude/t068-token-stats` (D-077, PR #74), goes out as preview.44.
- **T-069** Linux window on `claude/t069-linux-window` (D-078, built on T-068), goes out as preview.45. Owner's test on the Linux laptop: the window, its title bar in light and dark, the dock icon.
- **T-059** window language on `claude/t059-language` (D-079, built on T-069), goes out as preview.46.
- **T-047** shared task list on `claude/t047-shared-tasks` (D-080, built on T-059), goes out as preview.47. Owner's test: two computers in one network, tasks of the same project on both; each shows the other's under «На компьютере «…»» within about 15 seconds.
- **T-052** short invitation on `claude/t052-short-invite` (D-081, built on T-047), goes out as preview.48. Owner's test: invite on one computer, type the 6 digits on another in the same network. Windows may ask to allow Messenger in the firewall (UDP 47100); if the host is not found, the long code still works.
- **T-053** Windows installation on `claude/t053-windows-install` (D-082, built on T-052), goes out as preview.49. Owner's test on Windows: double-click the download; Agent Messenger appears in the Start menu and in «Приложения»; «Удалить» removes it and keeps `%LOCALAPPDATA%\AgentMessenger`.
- **Next, in the owner's order:** T-057 remote log, T-049.

Updated 2026-11-08 (before preview.42) by Claude Code.

- **PR #72** (`claude/t058-task-cost`) goes out as **preview.42** (owner asked to release everything done today): T-058 usage in statuses, T-060 blue dot only while running, T-063 resizable panes and empty-click, T-064 «Проект» tab with auto-save, T-065 project rename (shown name only, D-072, Q-019), T-066 update icon and «v0.41» (D-073, Q-020), T-067 Server tab variant Б, «Любые команды» text. All window changes were checked in Chromium with stubbed APIs only, not in the native windows.
- **Approved, not built yet:** T-061 agent cards (D-070), the settings dialog and the per-card switch; T-062 Windows window without the system title bar.
- **Owner's next asks:** Linux as a real program window (Q-016, now requested: WebKitGTK through cgo); status words for the server connection («Подключено / Нет связи / Не подключено» proposed, not answered); dollars in task statuses (not answered).

Updated 2026-11-08 (T-058) by Claude Code.

- **T-058** on `claude/t058-task-cost`: each task in the list shows what it spent (D-068). preview.41 carries T-056.
- Owner declined, no task: per-task worktrees, trigger automations, a file browser.

Updated 2026-11-08 (T-056, after preview.40) by Claude Code.

- Released today: preview.38 (#68), preview.39 (#69: T-054 full access, T-055 remove a project), preview.40 (#70: removal popup, «Настроить проект…», folder bar removed). T-050 and T-051 are on `main` (#66, #67).
- **T-056** on `claude/t056-access-all`: one «Любые команды» switch per computer, answers and follow-ups use the current setting, «Войти»/«Выйти» on cards (D-067). Next: PR, CI, release preview.41.
- **Waiting on the owner:** what the Linux update button does (it does not update); whether the coordinator should not block in `wait` for 10 minutes; Q-017 (invitation code).
- **Next owner requests:** a native Linux window (Q-016, WebKitGTK needs cgo); T-052 6-digit invitation; T-053 Windows installer; T-047 shared task list; T-049 coordinator wishlist.

Updated 2026-11-08 (T-048, before preview.37) by Claude Code.

- **PR #65** (`claude/t044-agent-limits`, same commits as `claude/t046-ui-feedback`): T-044 limits, T-045 icons and the Windows restart fix, T-046 agent cards, T-048 computer rename (D-061). T-043 (models) is already on `main` (#64). Next: CI, merge, release **preview.37**; every computer must update, the Mac (server) first: an older server refuses reports with models or limits and ignores renames.
- **Owner's requests not done yet, in order:** T-047 shared task list across computers (7-day retention, read-only on other computers); T-049 the coordinator's wishlist (computer readiness in `list_agents`, progress and last output, result files, resource locks such as the Unity editor, tokens per task); Q-016 a Linux window without Chrome/Chromium (today the browser opens when neither is installed).
- **Owner's Linux laptop:** a Codex worker failed because `~/.local/bin/codex-code-mode-host` is missing next to `codex`, i.e. Codex is installed incompletely; the owner was advised to reinstall Codex the official way. Not a Messenger fault; not verified here (no Codex in this container).
- **Answered, no change:** switching between computers like tabs existed only in the approved mockup, never in the program.

Updated 2026-11-08 (T-043) by Claude Code.

- **preview.36** released from `main` 722c51f (PR #63, T-041/T-042); check the assets.
- **T-043 on `claude/t043-agent-models`:** model names per agent, owner-listed, for the coordinator (D-058). Verified with tests only.
- **T-044 on `claude/t044-agent-limits` (on top of T-043):** each agent's last reported limit as a `limit` field (D-059). Real CLI fields not checked (Q-015).
- **T-045 on `claude/t045-linux-icon` (on top of T-044):** icons on Windows and Linux; the Windows window closes and reopens after «Обновиться».
- **T-046 on `claude/t046-ui-feedback` (on top of T-045):** agent cards check by themselves, simpler buttons, dialog actions on the right (D-060); Q-016 about the Linux window.
- **Next:** merge T-043 to T-046, release preview.37; the owner tests models and limits with real agents.

Updated 2026-11-08 (T-042) by Claude Code.

- **T-042 on `claude/t041-sparkle-retry` (PR #63):** after the owner's Mac → Windows test: progress and «Принято» on the starting computer (D-055), grouped tool actions, a computer name for every computer (host included), tasks of other computers accepted by default (D-056), Codex trust read from all `config.toml` forms, «Войти в аккаунт» / «Установить …» buttons, a frameless Windows window with a themed title bar (D-057).
- **Verified here:** `check.sh`, Windows vet, tests with fakes, headless Chromium screenshots. **Not verified:** real Windows, the real two-computer relay, the Codex/Cursor install page addresses.
- **Next:** CI, merge, preview.36; the owner tests it. Open idea from the owner: show every agent's usage limits (not started; see LOG).

Previous state:

Updated 2026-11-08 (preview.35) by Claude Code.

- **preview.35 released** from `main` 48b883e (PR #62, T-041; run 42470255420, the Mac build re-run once after GitHub's download server returned 504), 21 assets. Own window on Windows (WebView2) and Linux (app-mode browser window), the agent check on every OS, Codex and Cursor found on Windows, «Выбрать…» buttons (D-051–D-054). Also fixed: a worker's last output lines could be lost when it exited, so a finished task was marked unverified.
- **Checked here:** the Linux x64 program from the release matches the SHA-256 list and reports preview.35 at 48b883e.
- **Not checked:** anything on a real Windows computer or a Linux desktop (window, console, dialogs, Codex/Cursor discovery, the check); the Mac app.
- **Next:** the owner tests preview.35 on Windows. Branch `claude/t041-sparkle-retry` waits up to 15 min for the Sparkle download (GitHub 504s); merge it with this docs update. Open owner decisions: version 0.0.1 and Codex/Cursor as coordinator.

Previous state (2026-11-07, evening, later):

Updated 2026-11-07 (evening, later) by Claude Code.

- **T-039 / D-050 (branch `claude/t039-connect-coordinator`):** the window connects the coordinator chat to Claude by itself. It runs Claude's own `claude mcp add --scope local` in the project folder, right after the project is created and on «Подключить к Claude», and reads `~/.claude.json` to confirm. Worker runs are marked (`AGENT_MESSENGER_WORKER_RUN`), and the coordinator server gives them no tools.
- **Verified here (Linux):** fake Claude CLI tests; a browser run of the built program (connected, error shown, button); the written entry starts the coordinator, which answers `list_agents`. **Not verified:** the real Claude Code (where it writes local-scope servers, especially on Windows), Mac, Windows.
- **Next:** PR, CI, merge; ship in preview.34 together with the Cursor name fix (PR #58).

Updated 2026-11-07 (evening, preview.34) by Claude Code.

- **preview.34 released** from `main` 7617420 (PR #58 Cursor names, PR #59 T-039; run 42396560953), 21 assets. The window connects Claude to a project by itself (D-050).
- **Checked here:** the Linux x64 program from `/releases/latest/download/` matches the SHA-256 list and reports preview.34 at 7617420.
- **Not checked:** the real Claude Code connection (Mac, Windows, Linux); the Mac app; the in-window update preview.33 → preview.34 on Windows and Linux.
- **Open:** PR #60 (T-040) runs the release's Mac checks next to the Windows and Linux job; timing to confirm on preview.35.
- **Next:** the owner installs preview.34 (or updates from 33), creates a project, and checks that a new Claude chat in its folder sees the agents.

Updated 2026-11-07 (evening, release) by Claude Code.

- **preview.33 released** from `main` 3ae2058 (PR #54 with #55, and PR #56; run 42388079985). It is the latest release, with 21 assets: the Mac app zip and its update files; plain Windows and Linux programs (x64, ARM64) with a SHA-256 list; and signed update files for Windows and Linux (`update-<os>-<arch>.json`, `.sig`, `<sha256>.zip`).
- **PR #56:** the first release run stopped on a failing test. A refused window grant opened model calls for a moment before the store refused it. Now the store is checked first.
- **Checked here:** the README links (`/releases/latest/download/...`) download the Linux x64 and Windows x64 programs, and their SHA-256 values match the list. `version` of the Linux x64 program reports preview.33 at 3ae2058. The Linux amd64 update manifest and its archive are signed by the app's update key, and that key is built into the program.
- **Not checked:** the Mac app from this release; real Windows and Linux desktops; the real Cursor CLI; a real update from GitHub, possible only from preview.33 to preview.34.
- **Next:** the owner tests preview.33 on Mac, Windows and Linux, and Cursor with the real CLI. Then preview.34 tests the in-window update on Windows and Linux. Still waiting for the owner: Q-012/T-028, Q-013.

Updated 2026-11-07 (evening, later) by Claude Code.

- **The owner:** there is no rule against Cursor; set it up. The AGENTS.md line was removed, D-011 superseded, Q-004 answered.
- **T-012 (branch `claude/t012-cursor-tasks`, stacked on `claude/t016-download-buttons`):** Cursor runs tasks like Claude and Codex (D-049). Messenger keeps one `messenger_task` entry in `~/.cursor/mcp.json`; the run reaches its tools through `AGENT_MESSENGER_TASK` and `${env:...}`. The «Агенты» card and setup dialog work for Cursor; the coordinator can pick `cursor`.
- **Verified here (Linux):** a fake Cursor CLI that reads `~/.cursor/mcp.json` and fills `${env:...}` like Cursor: report, question and resumed run with `--resume=<chat>`, model, usage limit; a broken `mcp.json` is left alone and the run does not start; the window card and dialog in headless Chromium. **Not verified:** the real Cursor CLI (that `${env:...}` reaches the server, the current event format), Cursor on Windows (not supported: `.cmd`).
- **Next:** the owner's test with the real `cursor-agent` on Mac or Linux: connect Cursor in «Агенты», ask the coordinator to give Cursor a small task.

Updated 2026-11-07 (evening) by Claude Code.

- **Owner's requests:** plain «Download» buttons for Mac, Windows and Linux instead of archives, one heading above them; signed updates from the app on Windows and Linux.
- **D-047 (branch `claude/t016-download-buttons`):** each release publishes plain programs under fixed names and is marked latest; README buttons link to `/releases/latest/download/<name>`; [DOWNLOAD.md](DOWNLOAD.md) explains starting them.
- **T-038 / D-048 (same branch):** `internal/selfupdate` verifies and installs a signed update; the browser window checks every 5 minutes and shows «Доступно обновление» / «Обновиться»; after the install the window reopens from the new program. The release signs `update-<os>-<arch>` archives for Windows and Linux on the macOS job.
- **Verified here (Linux):** `check.sh`; the update package tests; an update preview.900 → preview.901 with a test key through the real window (the program replaced, the window reopened, the old client offered for restart); the signing script with a throwaway key. Windows CI runs the update package tests (including replacing a running program). **Not verified:** a real update from GitHub (only possible from preview.33 to preview.34), a real Windows desktop, the release signing step itself (it runs only in a release).
- **Next:** PR for this branch, green CI, merge, release preview.33; then the owner tests downloads on all three systems, and an update after preview.34.

Updated 2026-11-07 (afternoon) by Claude Code.

- **Owner's instruction:** "довести до релиза сейчас программу, все везде сбилдить, выложить на GitHub" — the owner tests on Windows, Linux and Mac later; decisions that need them are taken cautiously or postponed.
- **T-016 (PR #52, `claude/t016-windows-linux`):** Windows and Linux builds. Windows job objects as process groups (D-043; the client keeps each job's handle for the whole run, a job it cannot reach is not proven), `.exe` agents only (D-044), the window in the browser via `messenger open` (D-045), zip/tar.gz archives in every release, a Windows CI job. Guide: [WINDOWS_LINUX.md](WINDOWS_LINUX.md).
- **Verified:** see "What is verified" in WINDOWS_LINUX.md and the PR. **Not verified:** a real Windows 10/11 desktop, a browser opening on Windows, real Claude Code or Codex on Windows or Linux, ARM builds (compiled only).
- **preview.32 released** from `main` 11d938c (PR #52; run 42380342750): the Mac app zip, `update-darwin-arm64.json` and its `.sig`, and zip/tar.gz archives for Windows and Linux (amd64, arm64) with a SHA-256 list. Checked here: the checksums of the Linux amd64 and Windows amd64 archives, and `messenger version` from the Linux archive.
- **Next:** the owner tests preview.32 on Mac, Windows and Linux: start, the window in the browser, a task with a real agent, a task between two computers. Still waiting for the owner: Q-004 (Cursor), Q-012/T-028, Q-013.

Updated 2026-11-07 (night) by Claude Code.

- **preview.30 released** from `main` 598437c (PR #49, T-035: computers and their agents in the sidebar).
- **preview.31 released** from `main` 019f30a (PR #50, T-037; run 42371886488).
- **T-037 (PR #50, merged):** tasks between computers. The broker carries messages (`/v1/send`, `/v1/inbox`); a coordinator's `start_task` with another computer's ID records a remote task here and the other computer runs it as its own task, sending questions, the report and the outcome back. Answers and cancel go the other way. A computer runs other computers' tasks only when «Задачи других компьютеров» is on («Сервер» tab; off by default, Q-014). Schema version 3. Design: [TASK_MODEL.md](TASK_MODEL.md#tasks-between-computers-t-037), D-041, D-042.
- **Verified here (Linux):** `check.sh`; store tests for the whole exchange (refusal, question → owner answer → report, cancel both ways, a duplicate message, an undeliverable start or cancel); a supervisor test through a real broker over TLS; two separate clients on one machine joined by the window API: start from the coordinator, acceptance with the folder, cancel, refusal while not accepting; window screenshots in headless Chromium. **Not verified:** two real Macs, a real worker run started from another computer (the run itself is the same code as a local task), WebKit.
- **Next:** the owner tests preview.31 on two Macs: join, turn on «Задачи других компьютеров» on the second, ask the coordinator on the first to give a task to the second. Q-014 to confirm. Cursor (T-012) waits for Q-004.

Updated 2026-11-07 (late evening) by Claude Code.

- **T-035 (branch `claude/t035-network-roster`):** every computer reports its agents to the broker every 15 s; the sidebar shows all computers of the network with their agents and statuses, «Был на связи …» after 45 s without a report (D-039). VPS made optional (D-040, T-036 in the backlog).
- **Verified here (Linux):** `check.sh`; broker tests; headless Chromium with a helper program joining as a second computer. **Not verified:** two real Macs.
- **Next:** T-037 tasks between computers — design first in TASK_MODEL.md, with the cautious defaults recorded in QUESTIONS.md.

Updated 2026-11-07 (evening) by Claude Code.

- **«Сервер» tab and sidebar polish (T-015), approved by the owner on a live preview, going into preview.29:** statuses are a dot and plain text; Доступ, Порт and one start/stop button in one row, fields locked while the server runs, «Применить» removed; explanations moved into status tooltips so cards keep their height; computers that joined this server show in the sidebar «Компьютеры» (name, dot, last seen) with «Отозвать» in a right-click menu; invitation dialog copies with an icon; the sidebar shows only «Доступно обновление» and «Обновиться»; control text is centred by measuring the font at start.
- **Verified here (Linux):** `check.sh`; headless Chromium: start/stop, locked fields, focus, join a device to its own server, sidebar card, right-click menu, revoke. **Not verified:** the Mac window (WebKit, SF font centring), two real computers — the owner tests preview.29 on two Macs.
- **Next:** devices send their agents and statuses to the server so the sidebar cards show them (T-015 continued); Cursor adapter (T-012) waits for the owner to allow running Cursor CLI (Q-004).

Updated 2026-11-07 (day) by Claude Code.

- **Done and merged (PR #45):** T-034 failure reasons with the limit's reset time; T-031 bounded `wait`; T-032 lean `tasks` with `if_version`; window tweaks (menus stand apart from cards, green «Работает», the invitation in its own dialog).
- **T-033** (branch `claude/t033-resume-brief`): a resumed run's `read_task` leaves out the task text unless `include_task: true`.
- **Other products are not named anywhere** (owner's request): the comparison document was removed, the rule is in AGENTS.md. The owner deleted old merged branches (T-018).
- **Verified here (Linux):** `check.sh`, new tests with fake Claude and Codex CLIs, window screenshots in headless Chromium. **Not verified:** real usage limits (formats come from the CLIs' docs), the Mac app.
- **preview.28 released** from `main` at 98d346e (PR #45 and PR #46; run 42360015163): the app zip, `update-darwin-arm64.json` and its `.sig`. Not yet installed on the Mac.
- **Next:** owner updates to preview.28; then T-015 continued and Q-012 / T-028.

Updated 2026-11-07 (third night) by Claude Code.

- **Owner's instruction:** "доведи программу до релиза и сделай там настройку сервера", then an audit of the new work with all findings fixed.
- **preview.25 released** from `main` (PR #40): action errors as a chip above the input, a clear "old client in the background" message with the restart command, a fix for the task list status column.
- **T-015, first part** (branch `claude/t015-server-setup`): the background client can host a broker and a computer can join one by invitation. The window's «Сервер» tab enables it (local only by default, LAN on request), creates one-time invitations, lists and revokes devices, joins a broker, checks and drops the connection. `messenger status` shows the state. **Tasks do not travel between computers yet**; the tab says so. Details and file layout: [BROKER.md](BROKER.md); decisions D-033–D-035; Q-013 (device key in a 0600 file for now, not the Keychain).
- **Audit:** two independent reviews (security/protocol and window logic); every finding is fixed (list in [BROKER.md](BROKER.md) and [PR #41](https://github.com/kolotiloff/agent-messenger/pull/41)). One older issue outside this work went to the backlog as T-030.
- **Release:** PR #41 merged into `main` and released as preview.26 (see LOG for the outcome).
- **Verified here (Linux):** `check.sh`, `check-builds.sh`; tests in `internal/broker`, `internal/supervisor` (the client joins its own broker through the control command, revoke, check), `internal/localui`; the tab in headless Chromium in both themes, at 1100 and 600 px, against a real client.
- **preview.27 released** from PR #42: one set of style rules (D-036), the server note in a side column, «Запустить/Остановить сервер», Claude-style dropdowns, and a blocking «Перезапустить / Выйти» dialog for an older background client (D-037).
- **Not verified:** two real computers, a local network, the Mac app.
- **Next:**
  1. Owner: update, open «Сервер», turn the server on, invite a second computer when one is available.
  2. T-015 continued: tasks between computers through the broker, the VPS package and wizard, Keychain (Q-013).
  3. Q-012, then T-028 (T-027 is closed: the owner reported preview.24 works).

Previous update 2026-11-07 (second night) by Claude Code.

- **T-027 first run done with the owner on preview.23** (real models on the Mac): a Codex task and a Claude task with two owner questions, both verified and reported to the Claude-app coordinator through `wait`. Q-010 answered (ToolSearch works), Q-011 and Q-009 answered (D-028, D-029).
- **preview.24** (branch `claude/t027-window-fixes`, merged into `main`) contains everything found in that run:
  - Claude workers may edit files (`acceptEdits`, D-028);
  - the window's client allows tasks and resumes itself (D-029), so no terminal commands are needed;
  - a chat-like stop button (square button in the input field, Esc; D-031);
  - the coordinator may choose the worker's model (D-030), shown in the task header;
  - window fixes: Enter sends, no horizontal scroll, one line per program in the computer card, "Ждёт запуска" for unlaunched runs, duration from launch, code blocks and tool lines in worker text, specific error texts instead of "client unavailable";
  - a relative `--root` now works;
  - cleanup: unused code and `experiments/` removed (D-032).
- **Verified here (Linux):** `check.sh`, `check-builds.sh`, all tests with fake CLIs (including the model reaching both the first and the resumed run), the window in headless Chromium (stop by click and by Esc, light and dark), `check-desktop.py`. CI on Linux and macOS (includes the Mac app build).
- **Owner's Mac (2026-11-07 morning):** the owner tested preview.24 and reported that everything works. T-027 is closed on that report; the individual cases were not written up separately.
- **Next:** Q-012 (may the persistent Claude stream and Codex app-server be removed?), then T-028.

Previous update 2026-11-07 (first night) by Claude Code.

- **Built overnight** on `claude/task-core-night` ([PR #36](https://github.com/kolotiloff/agent-messenger/pull/36), into `claude/vigilant-darwin-34d88b`): T-022, T-024, T-025, T-026 and T-029. The task model now works end to end on one computer. Details are in [TASK_MODEL.md](TASK_MODEL.md).
  - projects and folders;
  - tasks with verified worker runs;
  - the coordinator MCP server with `wait`;
  - the worker profile;
  - the new window.
- **Verified on Linux:**
  - `check.sh` and `check-builds.sh`;
  - tests with fake CLIs;
  - real Codex 0.155.1 and Claude Code 2.1.319 as workers against local fake model APIs ([report](reports/2026-11-06-task-workers-linux.md));
  - the window in headless Chromium.
- **Not verified:**
  - real models;
  - the Mac app (WKWebView, the folder panel bridge, Swift build: CI only);
  - the Claude app as coordinator with this server (the `wait` contract was proven by the T-023 pilot).
- **Open questions for the owner:**
  - Q-009: how the window allows task launches. Today only `serve --allow-tasks`; the window's own client does not launch tasks.
  - Q-010: Claude Code hides MCP tools behind `ToolSearch`.
- **Release (2026-11-07):** the owner chose to merge PR #36 into `claude/vigilant-darwin-34d88b` and that into `main`, and to release preview.23 for T-027. Before that, macOS CI found a stream start race in the persistent Claude path (fixed: one stream process starts at a time) and two test-only path and timing issues.
- **Starting tasks for T-027:** the window's own client does not launch tasks (Q-009). Stop it with `messenger --root DATA shutdown`, start it with `messenger --root DATA start --claude-stream --allow-tasks`, then `messenger --root DATA resume`. `DATA` and the messenger path are shown in the window's coordinator command.
- **Next:**
  1. Review PR #36.
  2. Decide Q-009.
  3. Run T-027 on the Mac with real models: build a preview, create a project in the window, connect the Claude-app coordinator with the shown command, start the client with tasks allowed, then one Claude and one Codex task with a question round.
  4. Then T-028.
- **Persistent paths** (Claude stream, Codex app-server) are untouched and still work. Their replies are now published only after verification (T-022).
- **Schema version is 2.** Older clients refuse a data directory opened by this build.

Earlier state (kept for history):

Previous update 2026-11-06 by Claude Code (D-021–D-026 decided, window approved).

- **`main` ([PR #29](https://github.com/kolotiloff/agent-messenger/pull/29)):** T-001 and T-002 are done. `create-codex-session` creates a Codex thread owned by a live app-server, and queued messages to its alias are delivered one turn at a time in that same process. The turns use the bound `read_current`/`reply` tools. Details: [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md), [setup report](reports/2026-11-06-codex-setup-linux.md), [delivery report](reports/2026-11-06-codex-delivery-linux.md).
- **Verified:** only on Linux, with a fake app-server. No real Codex model turn has run yet.
- **`main` ([PR #29](https://github.com/kolotiloff/agent-messenger/pull/29)):** T-007 is done. `recover-codex` retires an uncertain Codex owner after a proven reboot. It is refused on Linux and Windows, which have no boot identity source yet.
- **`main` ([PR #30](https://github.com/kolotiloff/agent-messenger/pull/30)):** T-005 is done. Replies to another local agent are delivered as new messages, with a limit of 3 per chain ([ROUTING.md](ROUTING.md)). A persistent Claude session and a Codex owner exchange messages side by side; this was verified with fake CLIs.
- **`main` ([PR #30](https://github.com/kolotiloff/agent-messenger/pull/30)):** T-003 is done. The Codex config is checked again before every turn, and the database has a schema version.
- **`main` ([PR #31](https://github.com/kolotiloff/agent-messenger/pull/31)):** T-008 is done. Up to 4 persistent Claude sessions run side by side.
- **Releases:** Mac previews are now built, signed and published by the Release Mac preview workflow, using the `SPARKLE_ED_PRIVATE_KEY` secret ([UPDATES.md](UPDATES.md)). preview.20 was the first one released this way.
- **`main` ([PR #32](https://github.com/kolotiloff/agent-messenger/pull/32)):** T-006 is done. The window creates a Codex session and allows model calls only for it ([SESSION_SETUP.md](SESSION_SETUP.md)).
- **T-004 accepted on the Mac with preview.21:** real Codex answered two messages from the window, with both turns verified ([report](reports/2026-11-06-codex-window-mac.md)). A paused queue after setup was found and fixed in [PR #33](https://github.com/kolotiloff/agent-messenger/pull/33).
- **Direction decided (2026-11-06):** the product becomes "one task, one session" ([D-021–D-023](DECISIONS.md), [RETHINK.md](RETHINK.md)). The coordinator is the owner's Claude-app chat with Messenger over MCP and a blocking `wait`, confirmed by the [T-023 pilot](reports/2026-11-06-wait-pilot-claude-mac.md). Work so far is on branch `claude/vigilant-darwin-34d88b` (documentation and `experiments/wait_probe`); no product code changed yet.
- **Window and projects decided (2026-11-06):** one working folder per project per computer (D-025), several projects at once with one coordinator chat each (D-026), window approved as [docs/mockups/2026-11-06-task-window.html](mockups/2026-11-06-task-window.html) (D-024, details in T-029). PR #35 (T-009) is cancelled: do not merge or build on it.
- **Next:** T-022 (publish a reply only after verification), then T-024, T-025, T-026, T-029, then the Mac acceptance T-027 and T-028 in [TASKS.md](TASKS.md). Persistent paths stay until T-028. Claude ↔ Codex with real models happens in T-027.
- **Mac status:** after the freeze and reboot, preview.19 would not open. All reservations pointed to a test directory in `/private/tmp` that macOS had deleted. The owner moved `active-runs.json` aside after checking the evidence, and the window now works with the default data directory and empty history ([report](reports/2026-11-06-reboot-temp-root-mac.md)). The old Claude test session no longer exists. Fixes (`recover-missing-root`, refusing temporary data directories) are in PR #31 and ship with the next build.

Codex CLI for model-free checks in a cloud session: `npm install @openai/codex@0.155.1` in a scratch directory. Then use the native binary under `node_modules/@openai/codex-linux-x64/vendor/*/bin/codex` with `MESSENGER_CODEX_PROBE=...`.

The sections below are the original handoff from Codex (2026-11-06, base `fcec2d88cc000ca9afad5427e46bcf5701c7fb15`). They still describe the product state and the plan for the Codex adapter.

## Product and way of working

Agent Messenger is messaging between agents on macOS, Windows and Linux. Any supported agent can be the coordinator. The main client is written in Go: `cmd/messenger`, `internal`. The Python in `experiments` is research, not a direction for rewriting the product. The native Mac window lives in `desktop`; the interface lives in `internal/localui/assets`.

In the future the broker runs on one of the computers or on a VPS. The program is not tied to the user's projects, game engines, or the systems that store their files. Overall plan: [RELEASE_PLAN.md](RELEASE_PLAN.md).

Continue with the functional stages. Fold interface feedback into the same release; do not ship a separate build for every small item. The already chosen light/dark themes and layout are kept. Do not present preparatory components as a finished window feature. When a stage is finished, update the sources, the documentation and the verification results together.

## What actually works

- The installed user build is preview.19. The signed shell update to it through the updater has been verified.
- A separate new test session of real Claude Code 2.1.318 was created through the window. Two messages got `ACK:WINDOW-ONE` and `ACK:WINDOW-TWO` in one persistent process. The replies appeared automatically; three old messages of other sessions were not processed.
- After the check the queue was paused and the live Claude owner was kept. As of the handoff, the new developer must not stop it, release its reservation, or take over that conversation as a working development session. Check the actual state first with the standard tools, not by PIDs from old reports.
- Waiting and reading state do not send model requests. No timer-driven model prompts are needed.
- The background client and the shell update are separate processes. Automatic replacement of a running client and full rollback are not yet accepted.

Details: [preview.19 report](reports/2026-11-06-window-launch-mac.md), [scoped launch](SCOPED_LAUNCH.md), [updates](UPDATES.md).

## Current stage: persistent Codex

The installed CLI was studied at version 0.155.1. Persistent Codex is **not yet connected to serve or the window**. No real Codex model turns have been run with this new adapter.

Added and verified:

- `internal/adapters/codex_appserver.go`: arguments for a separate stdio app-server for the managed profile, and a check of the actual `config/read`. Missing fields and `null` are not treated as disabled capabilities.
- `internal/adapters/codex_turn.go`: checks of the exact thread/turn ID, the `read_current` → `reply` sequence, the receipt, and the reply write through handlers bound by the controller. A refusal, a repeat, an old receipt, `failed`/`interrupted`, and completion without a reply do not become success. This is not an RPC transport and not process management.
- Three checks of the real app-server without a model: a clean profile was accepted; an enabled inherited MCP was rejected; an explicitly disabled MCP was accepted. None of these checks call `thread/start` or `turn/start`.
- Full local Mac checks passed on Go 1.27.3 and 1.22.2. All four jobs of [CI 42206000660](https://github.com/kolotiloff/agent-messenger/actions/runs/42206000660) passed on Mac/Linux. Cross-compilation is not Windows acceptance.

Read [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md) and the [verification report](reports/2026-11-06-codex-preflight-mac.md).

Confirmed Codex specifics:

1. `app-server` does not accept `--ignore-user-config`; do not blindly carry over flags from one-shot `exec`.
2. `-c 'mcp_servers={}'` does not remove servers inherited from the file. Launch arguments alone are not enough to prove isolation.
3. `thread/start` issues the ID itself. In the investigation, an empty new conversation had no rollout for `thread/resume` after the process exited. Do not automatically replace a session that was not found with a new conversation.
4. Dynamic tools are experimental; check the format against the installed CLI and the [official App Server](https://learn.chatgpt.com/docs/app-server).

## Next task

Create and persist a Codex session before the first message, then connect the persistent transport to the client's existing guarantees. No changes for this were made in the last pass: only code reading and design were done.

Start by checking the current schema constraints:

- `SetupNewClaude` uses a UUID known in advance; Codex's server-issued ID needs a different creation order.
- `stream_runs.id` has a foreign key to `attempts(token)`. Currently a process owner cannot exist before the first message attempt. Do not create a fake message or a fake ID to work around this link.
- `ConfirmStreamIdentity` explicitly selects `provider='claude'`. You cannot simply drop the condition without checking the provider, the conversation and the specific turn.
- `Supervisor.current` and `runStream` are designed for a single persistent owner. Multiple agents require a separate controller change; one successful Codex reply does not yet prove Claude ↔ Codex.

Proposed implementation sequence, which may be refined with justification after reading the code:

1. Persist the creation intent before starting the process: a random setup ID, a reserved alias, the canonical managed profile, the verified CLI/workspace, and a configuration fingerprint. Separate creating the owner from a message attempt.
2. Write the shared reservation before launching the CLI. Check the actual configuration before creating the conversation; separately work out authentication for the managed profile without publishing or logging credentials.
3. Match the RPC of the creation response. Atomically persist the issued ID and the owner before the first message. A lost confirmation, a write error or a restart leaves an uncertain result; `thread/start` is not retried automatically.
4. Bind the confirmed turn ID to a specific attempt in SQLite. Buffer events that arrive before the RPC response, within a bounded buffer; tools are unavailable until identity is confirmed. Record the send intent before writing to stdin.
5. Connect `CodexTurnGuard` to the existing read/receipt/reply checks in the database. Success is only an exact `turn/completed` with status `completed` plus a persisted linked reply. EOF, timeout and stop do not release the owner and do not trigger a retry.
6. Test with a fake CLI: two consecutive processings, failures at every boundary, repeated RPC/call IDs, foreign turns, profile changes, restart, and no model calls while waiting. Check compatibility and that an old client refuses unsupported new state.
7. After that, two real messages in a separate new Codex test session, then an exact resume of the persisted session. Do not use working chats or GUI chats.
8. The next functional stage is independent owners and Claude ↔ Codex routing with a limit on automatic replies, then connection from the window.

## Constraints that must not be relaxed

Do not delete `active-runs.json`, do not empty it, and do not release a reservation based on a timer, a single PID, or an empty process group. Do not bypass the shared registry to launch from a different database. A signal to a process does not prove that all its descendants have exited. When the result is uncertain, do not repeat actions automatically.

No user session IDs, tokens, auth files, private SQLite dumps or full logs in GitHub. The repository is public. Do not restart the Mac automatically. Do not use working chats as test chats.

## Getting started and verification

Work in a regular clone of the [repository](https://github.com/kolotiloff/agent-messenger) from the current `main`, in a separate branch. The previous developer's local copy of the sources may have been a mirror without `.git`; do not automatically treat it as a full checkout.

```sh
bash scripts/check.sh
```

Go 1.22.2 or later is required. If needed, point to the installed Go with `GO_BIN`. The script itself selects the external linker for race tests on Mac. An additional model-free check of the installed Codex is described in the report; it is skipped by default. Real models are not run in the regular tests.

Deliverable of the next stage: a PR with the implementation, failure tests and updated documentation; list separately what actually passed and what remains. For audit, hand over the PR link and the exact SHA. Publish a new application package after the functional step is finished and verified, not for the sake of an intermediate library.
