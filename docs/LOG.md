# Work log

One entry per work session, newest first. Write what was done, which tool did it, the PR, and anything the next session must know. Keep entries to a few lines; details belong in PRs and reports.

## 2026-11-25 — Codex (T-296, T-297)

- Reproduced the success notice appearing alongside the old installation spinner. Reconciled the update reply and shared watch by request ID so the notice, version and control advance together. The WebKit and Chromium fixture covers out-of-order success and failure, a distinct queued request, and cancellation before the installer starts.
- Show the full reason as a tooltip only when the notification actually clips it. The three-notice limit and the existing footer layout still apply.

## 2026-11-25 — Codex (1.0.2 release, T-290–T-295)

- Merged [PR #198](https://github.com/kolotiloff/agent-messenger/pull/198) as `41dc97c` after all required checks passed. All eight [client jobs](https://github.com/kolotiloff/agent-messenger/actions/runs/45519001699) completed successfully. The release also incorporates the verified 1.0.1 record from #197.
- Published [1.0.2, build 105](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.2-preview.105) with 22 assets. All five signed public update targets passed independent verification. The [post-publication job](https://github.com/kolotiloff/agent-messenger/actions/runs/45521441937/attempts/2) passed on its own retry after a GitHub API rate limit.

## 2026-11-25 — Codex (T-292)

- Implemented the approved red offline cross. The cross and online dot share an 8 by 8 px footprint and position. The gap to the computer name follows the approved mockup. [PR #198](https://github.com/kolotiloff/agent-messenger/pull/198).

## 2026-11-25 — Codex (T-295)

- Reviewed window tooltip wording. Gave the cancel-update control a short action label and exposed its status separately. Normalized update, server and task hover text in both dictionaries. [PR #198](https://github.com/kolotiloff/agent-messenger/pull/198).

## 2026-11-25 — Codex (T-290, T-291)

- Added Codex command observations and a project-scoped advisory for unexplained silence. Kept the existing completed-turn exit warning. Added Stop the task to eligible rows with the composer stop guard.
- Used the download arrow with tray in the local and remote update controls. [PR #198](https://github.com/kolotiloff/agent-messenger/pull/198).

## 2026-11-25 — Codex (1.0.1 release)

- Published [1.0.1, build 104](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.1-preview.104) from `f2f6f4c` after all eight [main checks](https://github.com/kolotiloff/agent-messenger/actions/runs/45497895256/attempts/2) passed. The first Linux run with Go 1.22 timed out waiting for the command socket in four fixture tests. The targeted retry passed on the same revision. All four [release jobs](https://github.com/kolotiloff/agent-messenger/actions/runs/45504161598) passed, including signed upgrade verification for all five targets. Verified 22 uploaded release assets.

## 2026-11-25 — Codex (T-291)

- Recorded a future task-list context menu action for tasks whose existing chat stop action is available. The action targets the clicked row and uses the shared pending-stop guard.

## 2026-11-25 — Codex (T-290)

- Recorded the worker diagnostics follow-up after a report of delayed exit on Windows. Confirmed the existing `worker_still_running` advisory requires a completed CLI turn and a verified live worker process. The reported run's CLI events remain unverified.

## 2026-11-25 — Codex (T-289)

- Integrated PRs #192–195 for the next release. Fixed three update state regressions involving equal display versions, an absent requester, and successive requests for one computer. See the [release audit](reports/2026-11-25-t289-release-audit.md) for validation and live acceptance status.

## 2026-11-25 — Codex (T-292, T-293, T-294)

- Refined computer cards in both themes and changed the Mac menu bar click behavior. Verified the cards in Chromium and WebKit and built the native Mac application. Live menu bar clicks remain unverified.
- Updated [PR #198](https://github.com/kolotiloff/agent-messenger/pull/198) with a larger green dot at the lower right of the monitor, a wider gap before the name, and muted names with crossed monitors for offline computers.
- Added a pending update cancellation indicator with redraw, reopening and lost response coverage. A three-computer fixture checks repeated submission and receiver confirmation. Branch `codex/t292-computer-card-headers`.

## 2026-11-25 — Codex (T-285)

- Added Planned to the task sidebar for tasks awaiting their first run. Remote executors report queuing on acceptance and confirm the first observed worker process. Shared task snapshots preserve the phase and an available resource name.
- Checked the transition with two fixture computers in single and split storage. Chromium and WebKit exercise selecting a task while queued and moving it to Active when it starts.

## 2026-11-24 — Codex (T-287)

- Bounded the receiver's pre-installation wait and added broker notifications for sidebar update controls. Verified the five-minute boundary, installer protection and cancellation by another observer on a three-computer loopback network.
- Added a browser check for the download arrow, the rotating circular arrow and red cancel cross, the reduced-motion setting, the installation indicator, and the yellow coordinator-wait status. Captured fictional sidebar and task cards in WebKit. These changes are for the next release.

## 2026-11-24 — Codex (T-283, T-284, T-286)

- Published [1.0](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.0-preview.103) with 22 assets. All four jobs in the [release workflow](https://github.com/kolotiloff/agent-messenger/actions/runs/45459724676) passed. The shipping 0.99 and 0.101 Mac programs each found and downloaded signed build 103 from the public list. The owner’s Mac staged it without an installation or restart.
- After the release, marked the local computer with a thin neutral stripe independent of selection and online state. WebKit computer switching and control checks passed, including light and dark at 600 and 1100 pixels. Added T-285 for the planned task section.

## 2026-11-24 — Codex (T-288)

- The task window sends one stop request and keeps its button disabled while waiting. The task view distinguishes a stop request still queued locally from one accepted by the broker. The receiver’s confirmed outcome determines the final status.
- The network fixture covers queuing, broker acceptance and final cancellation. The split-storage fixture covers reopening with a pending stop and notifying the correct project after delivery. The Chromium and WebKit controls checks exercise repeated Escape presses during the wait.

## 2026-11-24 — Codex (T-284)

- Prepared public version 1.0 after 0.102 reproduced the legacy discovery issue. The package and tag retain the signed format understood by 0.99 and the ordered updater in 0.101.

## 2026-11-24 — Codex (T-283)

- Confirmed the 0.99 check still returned current in an isolated home. The corrected finder staged signed build 101 on the owner’s Mac. Added independent public release checks for all five platform routes and an optional, reversible archive step for blocking discovery manifests.

## 2026-11-24 — Codex (T-281)

- Computer cards show the agent name and its status while the worker has yet to report a model. Once the reported model arrives, it appears beside the name. A comma precedes the status in both cases. Verified in Chromium and WebKit with fictional local and remote tasks.

## 2026-11-24 — Codex (T-282)

- Reproduced the false current-version result from the installed 0.99 binary. The GitHub response listed builds 99, 98, and 100 in that order. Select by numeric preview build and require a signed version matching its tag. The updater race tests passed, and the patched 0.99 test binary found the real signed 0.100 in a separate home directory. Also verified the signature on the direct Mac download. The published 0.100 binaries predate this updater fix.
- Published [0.101](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.101) from `e84576b`. GitHub cancelled the signing job before it received an ARM runner; the [second attempt](https://github.com/kolotiloff/agent-messenger/actions/runs/45437843015/attempts/2) succeeded. Verified 22 uploaded assets and all five update sets (signatures, hashes, sizes and source revision). The live response put builds 99 and 98 before 101. The original 0.99 check still reported itself current, and the patched test binary found 0.101. No live upgrade was performed.

## 2026-11-24 — Codex (T-280)

- Confirmed the affected computer still reported 0.99 and a machine-wide queued update. Added safe version reconciliation and a coordinator cancellation tied to the queued request ID. Verified the two-computer and split-store scenarios with fixture data.
- `scripts/check.sh` and cross-build checks passed locally. Recorded T-281 for a future release, covering every provider in the computer sidebar.
- Published [0.100](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.100) from `8ca1dfe` in [workflow 45428605429](https://github.com/kolotiloff/agent-messenger/actions/runs/45428605429). Verified the Latest designation, all 22 uploaded assets, and all five signed update sets (signatures, revision, hashes, sizes). No live update was performed.

## 2026-11-24 — Claude Code (T-278, T-279)

- T-278 (D-192): the VPS card listed finished tasks sent to ПК-2 in the earlier network, because «host» named ПК-2 then and names the VPS now. Tasks whose broker is not the current network no longer match any current computer in the window, its filter and cleanup, the shared list and the usage page. T-279: choosing a computer waits for the program's list instead of flashing an empty one; new `scripts/check-window-computer-switch.cjs`. Not released.

## 2026-11-24 — Claude Code (0.98 release)

- Published [0.98](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.98) from `1fac081` with T-268 to T-277. Verified 22 assets and all five signed update sets (signature, version, source SHA, archive hash and length). Before the merge, the WebKit window checks on PR #183 failed four times on test-environment issues, fixed in the test scripts only: the picture fixture now serves PNG and lets `blob:` loads through, the settings read is awaited from before the click, the action group is reopened after a lost click, and cancelled requests while the switch check leaves the page are expected. No installed-app or VPS upgrade was performed.

## 2026-11-24 — Claude Code (T-277)

- Automatic updates: a switch in the program settings of each computer (D-191). Windows and Linux clients look for a release every 30 minutes and queue their own update once per release. Mac and the update marks are open in Q-031. Go tests and the Chromium window check pass.

## 2026-11-24 — Claude Code (T-272 to T-276)

- T-272: the update's work count now uses state indexes; a query-plan test fails with the old query. T-273: found why the computer order was lost after updates (a refused report drops the lasting id for a while) and kept a peer-to-computer map in the order file. T-274: readable model names in the statistics filter. T-275: a hidden window no longer redraws timers or polls statistics, and polls the network once a minute. T-276: replaced «·» separators. Window checks pass in Chromium; WebKit runs in CI.

## 2026-11-24 — Claude Code (T-268 to T-271)

- Finished the picture viewer from the draft kept after 0.97 and added the English texts it lacked. On the owner's screenshots: statistics filters now share one height and start at the left, the project page is split into cards, and the computer chip is removed because the highlighted card already shows the choice (D-189). New `scripts/check-window-pictures.cjs` covers the viewer and a chosen computer that leaves the list.

## 2026-11-23 — Claude Code (0.97 release)

- Published [0.97](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.97) from `5de88fc` with T-264 to T-267. Verified 22 assets and all five signed update sets (signature, version, source SHA, archive hash and length). The owner moved the picture viewer (T-268) to the next release. No installed-app or VPS upgrade was performed.

## 2026-11-23 — Claude Code (T-264, CI of PR #182)

- On PR #182 the window-controls job was cancelled at its 10-minute limit while apt installed the browser system packages (9.5 minutes). Raised that job's limit to 20 minutes. The macOS / Go 1.22.2 job in the same run never got a runner and was cancelled after 15 minutes of waiting; the push reruns it. Nothing in the product changed.

## 2026-11-23 — Claude Code (T-267)

- Reviewed PR #176 (0.95). Fixed an inbox stall on a network message without a task route, a resource held by a task whose start failed half-way, a client that would not start after a failed storage activation, and a launch reported as failed after it was recorded (D-188). Three tests that asserted the stall now assert the skip; new tests cover the rest and fail on the old code.

## 2026-11-23 — Claude Code (T-266)

- Measured the jumps with `scripts/check-window-stability.cjs`: statistics collapsed from 1198 to 548 px on every computer, period or project choice, and the task list collapsed on a project switch. Both now keep their data until the new data arrives (D-187). The check runs in CI.

## 2026-11-23 — Claude Code (T-265)

- The welcome flashed on a project switch because the window drew the project before choosing its top task. It now chooses first and shows a delayed spinner while loading. The welcome is limited to the first run (D-186). Added `scripts/check-window-switch.cjs` to CI.

## 2026-11-23 — Claude Code (T-264)

- Reviewed the 0.96 release and the code of T-262 and T-263; no defects found in the product changes. Found why the 0.96 checks on `main` and PR #180 failed: the `core` race suite exceeds the 10-minute limit, and the Mac package check raced project storage preparation. Raised the limits and made the check wait for readiness. `TestTaskOutputIsReadToTheEnd` remains a known `SQLITE_BUSY` flake.

## 2026-11-23 — Codex (0.96 release)

- Published [0.96](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.96) from `b28a5cb` with T-262 and T-263. Verified all five signed update sets, 22 assets, the release tag and the VPS installer syntax. The Linux test timeout and signing runner queue required two separate job retries. No installed-app or VPS upgrade was performed.

## 2026-11-23 — Codex (T-263)

- Closed the window revision cancellation path that can abandon a read and added recovery for SQLite 517. Confirmed the existing project and task remain visible when migration is deferred. Chromium and WebKit exercised a failed first view and recovery. The production Mac still runs the released build.

## 2026-11-23 — Codex (T-262)

- Reproduced `SQLITE_BUSY_SNAPSHOT` and missing mailbox entries under concurrent sends on the 0.95 broker. Acquired the writer lock at the start of broker transactions. The regression covers concurrent sends and duplicate operation IDs. Full macOS checks passed.
- The owner restored the VPS with the released installer. Diagnosed persistent local 517 errors on Mac and verified the original projects, task history and network computers returned after a normal restart. T-263 tracks this separate client problem.

## 2026-11-23 — Codex (T-258, first stage)

- Added checked, project-scoped SQLite history snapshots. The read-only source stays usable, and a destination is published only after the copy succeeds. Documented the migration and delivery contracts that have to land before live project files replace the shared queue.

## 2026-11-22 — Codex (0.94 release)

- Published [0.94](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.94) from `d0e3a4e` and verified the five signed update sets. Corrected the browser regression test to read computed padding and color inside the page before returning simple values. Both browsers passed.

## 2026-11-22 — Codex (0.94 candidate)

- Combined the window update handoff, remote duration estimates, coordinator attribution and Codex exit-delay notice for release from main.

## 2026-11-22 — Codex (T-249)

- Added a coordinator event for the observed delay between Codex completing a turn and its process exiting. Checked that a waiting coordinator receives the event, a remote worker forwards it, and a subsequently verified exit still publishes its report. No live Windows or model test.

## 2026-11-22 — Codex (T-248)

- Record each coordinator's provider and optional model on follow-ups, answers and notes, including remote delivery. Show remote continuation text in the task history. Test coordinator changes, delivery to both computers and the label for older messages.

## 2026-11-22 — Codex (T-247)

- Added a marked estimate to statistics records with received remote progress and a terminal outcome but no exact worker duration. Date and duration views now include timed tasks without token usage. The regression fixture exercises a three-hour remote task and the precedence of an exact duration.

## 2026-11-22 — Codex (T-240)

- Added a local window handoff after remote Windows/Linux updates. The window waits for the background client and checks the installed file against the signed update before reopening. Automated checks use a fixture. Native desktop acceptance is still pending.
- Updated the Windows restart and installed-window launch flags to suppress the console window while allowing the desktop window to open.

## 2026-11-22 — Codex (T-246)

- Added inset space to the statistics sort buttons and kept the labels in their original positions. The browser check covers symmetric insets and hover feedback for all three columns, and verifies both sort directions.

## 2026-11-22 — Codex (T-245 follow-up)

- Captured new English screenshots for the public README and download guide. Both themes show Claude Code, Codex and Cursor with fictional tasks. Also captured the English statistics table and welcome screen.

## 2026-11-22 — Codex (T-241 to T-245)

- Published [0.93](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.93) from `efd0e83` and verified the five manifest signatures and archive digests.

## 2026-11-22 — Codex (T-245)

- Combined T-241 to T-244 for 0.93 and refreshed the public README and four screenshots. The screenshot review caught an existing outgoing-avatar style that hid the assigning agent icon; removed it and checked that the icon is visible.

## 2026-11-22 — Codex (T-243 and T-244)

- Record the assigning chat's agent and optional model with the original task, including its remote mirror. Reuse the worker agent icon in the coordinator header. Use the generic coordinator label when no author information is recorded.
- Added drag-and-drop and keyboard reordering for computer cards, saved in the local window.

## 2026-11-22 — Codex (T-242)

- Confirmed that the affected run had a saved report and an overload diagnostic. Updated the Codex worker parser and the window. Fake workers cover reconnect followed by success and exit zero without confirmed completion. The browser fixture checks the message for old saved failures.

## 2026-11-22 — Codex (T-241)

- Added duration and sortable date, duration and token headers to statistics. The store ranks the selected period before limiting the displayed list. A remote task uses the executing computer's elapsed time.
- Regression checks cover all six orders, time-only tasks, the worker's duration reaching the assigning computer and visible sorting controls at desktop and compact widths.

## 2026-11-21 — Codex (T-237 to T-239)

- Published [0.92](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.92) from `644d7b8` and verified the five manifest signatures and archive digests.

## 2026-11-20 — Codex (T-239)

- Added the local computer's coordinator label for the selected project. The label follows the live chat serving the role, including after a relay replacement.

## 2026-11-20 — Codex (T-238)

- Matched computer names and task titles in the list and statistics. Reused the section heading for confirmations and system text for project instructions.

## 2026-11-20 — Codex (T-237)

- Used the same inline placement for the sidebar waiting label and running duration. Applied the waiting color to the entire waiting agent row and removed the waiting dot in the task list.

## 2026-11-20 — Codex (T-235 and T-236)

- Published [0.91](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.91) from `e49c54f` and verified all five signed update manifests and archives.

## 2026-11-20 — Codex (T-236)

- Kept running work blue when another task for the agent waits, counted both tasks, and delivered worker model reports to the assigning computer.

## 2026-11-20 — Codex (T-235)

- Matched the sidebar activity label to the task list's blue and removed its animation.

## 2026-11-20 — Codex (T-234)

- Made the sidebar agent rows and task card text the same size. Put the muted running duration after a comma, with no background.
- Published [0.90](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.90) from `cd92102` and verified all five signed update manifests and archives.

## 2026-11-20 — Codex (T-229 to T-233)

- Published [0.89](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.89) from `8ff9acc` and verified all five signed update manifests and archives.

## 2026-11-20 — Codex (T-233)

- Reduced the chat body and input to 14 px regular text and kept a 21 px line height. Checked both at wide and narrow window widths.

## 2026-11-20 — Codex (T-232)

- Stopped older command-group timers when a newer worker action arrives. Made command-group headings muted and used a neutral sweep on the live label. The first 0.89 build was cancelled before publishing.

## 2026-11-20 — Codex (T-231)

- Aligned the task header and known sidebar activity with the list, and updated running durations even with no new messages. The Chromium fixture passed with an older conversation response and stale remote presence. Included in [PR #147](https://github.com/kolotiloff/agent-messenger/pull/147).

## 2026-11-20 — Codex (T-230)

- Reconciled the computer card and coordinator roster with the confirmed update reply. The two-computer fixture checks that the update notice and card agree while the broker still reports the earlier version.

## 2026-11-20 — Codex (T-229)

- Added a blue sweep to running agent labels in the computer list. The elapsed time uses a soft blue background and sits beside the label. The window fixture covers running, waiting, idle and reduced-motion states.

## 2026-11-20 — Codex (T-226 to T-228)

- Combined the command group status, on-demand agent versions and separate amber update arrow for 0.88.
- Published [0.88](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.88) from `d81bbc4` and verified all five signed manifests.

## 2026-11-20 — Codex (T-223 to T-225)

- Combined session timestamps, queued owner notes for remote workers and readable action labels for 0.87. Unified the window fixture so it exercises all three changes together.
- Published [0.87](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.87) from `7fd6c19` and verified all five signed manifests.

## 2026-11-19 — Codex (T-222)

- Refreshed the README and About, platform download and first-open steps, coordinator and network setup, and the update guide. Captured current light, dark and first-launch screenshots from fictional browser fixtures using the 0.86 window assets.

## 2026-11-19 — Codex (T-215–T-221 release)

- Published [0.86](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.86) from `55e31e2b54d01cad088c2675786b1f1879e81b82` in [run 44499053023](https://github.com/kolotiloff/agent-messenger/actions/runs/44499053023). GitHub uploaded all 22 assets. Verified the five update manifests against the app’s public key and matched their revisions and archive sizes.

## 2026-11-19 — Codex (T-220, T-221)

- Stopped the 0.86 run before publication to fix clipped computer names. The chat and task header now reuse the task list model formatter. Both regressions are covered by fictional browser fixtures.

## 2026-11-19 — Codex (T-219)

- Increased the window typography with existing font families. Used Chromium to check window controls and capture both themes on fictional task data.

## 2026-11-19 — Codex (T-218)

- Moved the connected server card action beside the connection details. Added it to [PR #132](https://github.com/kolotiloff/agent-messenger/pull/132) for the 0.86 preview requested by the owner.

## 2026-11-19 — Codex (T-215–T-217)

- Implemented the two statistics formats as one, the finished task wording and the approved B1 layout for local and remote computers. Shared tasks provide optional model and start metadata with fallback for brokers that explicitly reject the new fields. Verified the UI using fictional data in Chromium. Ready for review; no release.

## 2026-11-19 — Claude Code (T-215)

- Owner asked for the open tasks and the last release: 0.85 is published (PR #130). Recorded T-215 from the owner's screenshot: the statistics task rows get one format in both tabs; T-216: «Работал» instead of «Готово за» in finished task rows; T-217: model and busy time in the left panel (owner chose «В1» from mockups: model grey, time at the right edge). No code changed.

## 2026-11-18 — Claude Code (T-198)

- Owner answered Q-030 (b): D-147, T-205. The cause was `Wait` waiting for the error output a leftover child inherited; now limited after the exit. Also T-212, T-209, and parts of T-211 and T-208. Owner: release when done, don't wait.
- T-207: connecting an agent stored the file its command link pointed at, so the Mac kept the old Codex after `codex update`; the link is now stored, and old rows switch to a known link that still points at the stored file. T-214: statistics default to the latest tasks; no repeated heading. Q-030 asked again (needed for T-205).
- T-204, T-203 and T-210 fixed on the branch (stale update flag on the coordinator; stall limit instead of a total limit for update downloads; long op id of a window answer over the network). T-213 done on the branch: `update-find`; the Mac only looks in the background and downloads on the button.
- Read the Mac feedback export of 11-18 (50 remarks; new since 11-16: nine). Recorded T-204 to T-212; the worker Claude percent remark joins T-191. The Mac could not fetch 0.84 on a 33 KB/s link: T-203.
- Released [0.84](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.84) from `5d04fd6` via PR #129 (release run 44349253768, third attempt: the first two got no Mac runner); 22 assets, five update feeds and the Linux x64 checksum checked.
- T-202: task rows show the model after the status; the computer name stays (owner's choice).
- T-201: statistics by agent and model; the model counts only tasks known here, since other computers report per agent.
- T-200: the owner's Mac listed its paused agents in the left panel; this computer's card now leaves them out like the other computers do.
- Released [0.83](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.83) from `1ab2415` via PR #127 (release run 44332389459); 22 assets, five update feeds and the Linux x64 checksum checked.
- Owner asked to do the nearest tasks too: T-197 statistics sort, T-184 tray (Windows, Linux, Mac), T-199 Mac update dialog (Sparkle user driver). Linux tray tried under Xvfb with stalonetray.
- Checked the 0.82 release (run, 22 assets). Owner asked for one layout of the computer cards: dot before the name, version top right, update bottom right, also for this computer. Done on `claude/eager-volta-6sbjsa`; checked in Chromium with fictional data.

## 2026-11-18 — Codex (T-192, T-193, T-195, T-196)

- Prepared the next preview: GPT-6.2 Codex starter models including Sol Low, one model list per provider on the coordinator computer, project menu stability and early worker presence.
- The owner chose central lists instead of a remote settings command. Released [0.82](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.82) from `f1f5c6132a4b24f4792791b3c8a5969d6ee1d420`, [run](https://github.com/kolotiloff/agent-messenger/actions/runs/44295419912). All 22 published assets checked including five signed updates. Full local, browser and [PR CI](https://github.com/kolotiloff/agent-messenger/actions/runs/44293474608) checks passed.

## 2026-11-17 — Claude Code (T-194)

- Logo at the top of the README; `docs/assets/social-preview.png` for the repository's social preview (the owner uploads it in Settings, the API cannot set it).
- Released 0.81 from `main` 204b319 via PR #122 (release run 44040632883), merged without waiting for the PR checks at the owner's request (icon and docs only; the release runs its own checks). The Mac still showed the old icon after the update; icon cache suspected.
- Drew six bright icon variants; the owner chose А (yellow, dark bubble with eyes). Replaced the icon in the SVG, the Windows resources (regenerated, pixels compared with the SVG) and the Mac icon script (not run here).

## 2026-11-16 — Claude Code (0.80)

- Released 0.80 from `main` 9c36dd3 via PR #121 (release run 43917733249) with T-189, T-190, T-191. First CI run caught two platform-only test issues in the new T-191 tests (macOS /private/tmp path, Windows absolute path); fixed in the tests. Recorded T-192 and T-193 from the owner's remarks.

## 2026-11-15 — Claude Code (T-181, T-158, T-159)

- T-191 (D-143): Codex usage limits read through `codex app-server` `account/rateLimits/read` every 10 minutes, without a model call; both windows stored, carried in presence (older hosts get the plain limit), coordinator told to spread work by what is left. Unit tests with a fake app-server; no real Codex call made here.

- Read the Mac feedback export of 11-16 (38 remarks, 2 new). T-190 (D-142): no false takeover note after a restart. T-191 recorded (usage percent). T-189 (D-141): export only new remarks, «Очистить журнал» and `clear_feedback`.
- Released 0.79 from `main` e7f22f0 via PR #120 (release run 43756475974); Linux x64 asset checked (checksum, `version`). First Windows test attempt hit an unrelated SQLITE_BUSY flake in `TestTaskOutputIsReadToTheEnd`; the rerun passed.
- T-187 (D-140): update found in the background, downloaded and installed by one button in a dialog. T-188: task list toggle icon. Owner updated the VPS to 0.78 over SSH.
- Released 0.78 from `main` 875b8ce via PR #119 (release run 43747998585); Linux x64 asset downloaded from releases/latest, checksum and `version` checked.
- T-186: shared tasks carry where they run; «Агент на ПК — от ДАЛ» labels; agent removal in a terminal; «Убрать все» for finished tasks. Window check extended (frameless block, clear-done).
- T-185: the computer list no longer shows a disabled agent through its old coordinator chat.
- T-183 (D-139): remote update of the VPS service, `server rename`, 44 px caption row on Windows. Remote update tried end to end in a container against the real signed 0.77.
- Owner installed 0.77's server on a new Debian VPS with one command; all 5 computers joined over the internet.
- Released 0.77 from `main` 6709058 via PR #118 (release run 43724921881).
- T-036 (D-138): `install-server.sh` release asset and `messenger server run/enable/invite/status/disable/revoke`; «сервер» label in the computer list and `server: true` in `list_agents`. Tried in a container (commands, installer with a stand-in systemd); not on a real VPS.
- T-182 (D-137): «Интернет» scope for the hosted broker. Public peers are let in with their own limits (32 slots in all, 8 per source); the owner's external address is the first invitation address; six-digit pairing answers only local sources. Broker tests and the window check; released in 0.76 (PR #117, `main` a10535f, release run 43704189375, 21 assets), not tried between networks.
- Released 0.75 from `main` fd34136 via PR #116 (release run 43694772945, 21 assets). The window check now ignores WebKit's report of a background request cut off by a reload.
- T-158 (D-136): `list_agents` gives `newest_release` with notes from this computer's update channel and `update_available` per computer. T-159 (D-135): `continue_task` for an interrupted task of another computer, closed there only when its worker is certainly gone. Tests in `mcp` and `core`; released in 0.75, not tried between real computers. T-160 confirmed by the owner.
- T-181: the chat footer had its own height cap and scroll, so several update notices scrolled inside the chat and the scrollbars overlapped them. Removed it; at most 3 notices, a new one pushes out the oldest; long text cut to 3 lines. Browser fixture check added to `check-window-controls.cjs`. Released in 0.75.

## 2026-11-14 — Claude Code (merged #108–#113; T-178 Mac connecting screen)

- Released 0.74 from `main` 825fb76 via PR #115 (release run 43648585334): T-179, T-180. Recorded T-181.
- T-179: reviewed another agent's 0.73 work (T-155, T-173/T-174, update ring, MCP-plugin label, grey line). Fixed: mail to an asleep but listed computer was retired after 15 min (a waiting task failed); the update icon now fills a ring from `download.progress` instead of spinning in another colour; refusals give their reason in the journal; the Windows task-list toggle sits beside the window buttons. T-180: the window says project names must match exactly (another network's tasks were refused for a different name). Not released; real downloads not tried.
- Released 0.73 from `main` 8bb64e2 via PR #114 (release run 43641508174, 21 assets). CI on #114 caught a data race in the T-172 test; fixed before merge. Not installed on the owner's computers.

- T-175 (D-134): worker pictures with reports and questions, window thumbnails, coordinator `task_images` (MCP images), sent between computers (large ones shrunk to JPEG ≤ 180 KB). Core, MCP and browser-fixture checks; no real worker yet.
- T-177: `network_lost` / `network_back` coordinator events (once per change, per project, per network). Core test; not tried between real computers.
- T-172: runs record last output, the end of the CLI's turn and, while the process stays, its group's processes by name (Linux /proc, macOS ps, Windows job list). Shown in the task header and `tasks`; journal line once. No closing (Q-030). Verified with a fake worker; real Windows/macOS paths unverified.
- Merged PRs #108–#113 in order, resolving doc conflicts and two code conflicts (update icon uses the ring and T-155's text; the `update_pending` fallback runs before model names are stripped, like T-173).
- T-178: the Mac connecting screen at every start showed «Выбрать каталог данных…» and a retry button. Now a spinner only; reason, retry and note only on a failure; the data-directory choice stays in «Файл». Not compiled here (no Swift on Linux); Mac CI and the owner's Mac verify it. Not released.

## 2026-11-14 — Cursor (T-173, T-174)

- Ноут-Linux on 0.72: the model lists were in its settings and a task with an effort started, but `list_agents` elsewhere showed them empty. The journal also repeated that a mailbox to a computer not in the project was full. Presence no longer drops the model names because another field was refused, and mail to a computer that has left is retired after the reconnect grace. A listed computer's full mailbox still waits. Not checked on the owner's computers.

## 2026-11-14 — Cursor (coordinator journal)

- Read the issue list and the Mac feedback export of 2026-11-14 (35 remarks). Did not copy the journals into the repo. Already shipped and left as they are: T-171, T-152 (export button), T-138, T-150, T-149, T-148, T-161 (D-124), Q-027. Recorded what is still asked: T-172 (Q-030), T-173, T-174, T-175, T-176, T-177. T-157, T-158 and T-159 were already in TASKS. Server failover stays deferred.

## 2026-11-14 — Cursor (closing line of a task)

- «Отчёт отправлен координатору, сессия закрыта» is ordinary grey text, the same size as the message, not the small italic line. Not released. Branch `cursor/report-closed-text-bfd1`.

## 2026-11-14 — Cursor (update icon while downloading)

- The update arrow stays while a release is ready. While this computer downloads or installs, and while another computer's update is under way, the same circle shows a spinning ring. A remote computer does not report progress. Not released. Branch `cursor/update-download-spinner-bfd1`.

## 2026-11-14 — Cursor (project page heading)

- On «Проект», the «Координатор?» mark is a section: «MCP-плагин», with the hint «Подключение к графическому редактору», then Claude Code, Codex and Cursor as before. Not released. Branch `cursor/mcp-plugin-heading-bfd1`.

## 2026-11-14 — Cursor (T-155 update_messenger)

- T-155 (D-133) on `cursor/t155-update-messenger-bfd1`: the coordinator tool `update_messenger` queues a Windows/Linux Messenger update until live work ends; the computer takes no new tasks and shows «ждёт обновления». A Mac refuses with the app-button text. Window remote-update uses the same queue. Result as `messenger_update` and a notice; nothing retried.
- Verified: core, supervisor (queued remote path and local queue), MCP, broker presence fallback, `check.sh`. Not verified: real install between computers, Mac app path, the window on the Mac. Not released.

## 2026-11-14 — Codex (T-168–T-171 release 0.72)

- Released [0.72](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.72), source `f3bd2d45e9f18890419e09996bb7a97bb3b009bd`, [run](https://github.com/kolotiloff/agent-messenger/actions/runs/43600787465). PR #105 and PR #106 checks passed; exact checked #106 head `baf18d23021164c4babc4b96f381c1a15a5f6543`.
- Verified all 21 public assets and five signed manifests/archives, revision, version/build, sizes and checksums. No installation on the owner's computers, real model, installer or account login was run.
- Includes explicit install recovery, Cursor sidebar names, first-login profile validation and current worker model/effort choices. Custom lists and recorded tasks stay unchanged. Windows account acceptance remains with the owner.

## 2026-11-14 — Codex (T-171 model selections, preparing 0.72)

- Researched current official Codex/Claude/Cursor documentation and Cursor's metadata-only model list. Added exact model/effort choices, native argv mapping, task guidance and migration preserving custom settings.
- Owner expanded the ongoing 0.72 release scope. No real model calls. Final full macOS race checks, focused migration/argv/presence tests and Chromium/WebKit passed. PR CI and publication pending.

## 2026-11-14 — Codex (T-170 first login, preparing 0.72)

- Owner requested the new version and clarified the Cursor login issue is on Windows. The remembered message matches the frontend's generic HTTP-400 hint.
- Reproduced a local blocker before terminal launch: discovery passes a default profile path that does not exist until first login, but command construction rejected it. Allow that missing default path, retain strict custom-profile validation, and return specific path errors (D-131). No real account login or model run.
- PR #105 (T-168/T-169) merged after all CI jobs passed. T-170 is on `codex/t170-first-login`; browser checks passed, full checks/PR/release pending. Target combined release 0.72.

## 2026-11-14 — Codex (T-168/T-169 setup recovery and Cursor sidebar)

- Owner reported a closed Codex installation terminal on another Mac leaving 0.71 waiting. Implemented honest unknown status, an explicit Find action and durable recovery after confirmed CLI registration; preserve evidence and prevent late output from registering again. POSIX wrappers record hangup/interruption (D-130).
- Owner also reported a temporary Cursor/VS Code sidebar row. The coordinator name classifier recognized only Claude/Codex; Cursor now merges into the existing Cursor row (D-064). Raw unknown clients have a chat tooltip.
- Work on `codex/t168-setup-recovery`, not released. Full local race checks, final localui tests and Chromium/WebKit fixtures passed. PR CI/publication next; no real installer, model or working chat was run.

## 2026-11-14 — Codex (T-153/T-163–T-167 release 0.71)

- Owner explicitly requested the combined release. Merged [PR #103](https://github.com/kolotiloff/agent-messenger/pull/103), exact checked head `14212a16d4e5a47ccfc7f0680eb6c8aa46f0bf25`, at `daae5642bf80fc6193a05fa143678ecf0bcca1cc` after mandatory checks passed.
- [Release 0.71](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.71) published through [run 43591866974](https://github.com/kolotiloff/agent-messenger/actions/runs/43591866974); all 21 public assets, five signed update feeds/archives and checksums verified. Local full race/cross-build/browser checks and PR CI passed. Real clean-machine installation, login and two-Windows-machine LAN acceptance remain outstanding.
- Inspected the reported Cursor connection read-only: the project's messenger source was Disabled in Cursor, although its configuration and executable existed. Showed the owner the Enable switch; no working-chat test, permission change or real model call. Release record on `codex/t153-t167-release-71`.

## 2026-11-14 — Codex (T-153/T-165/T-166/T-167 combined follow-up)

- Owner added installation/find, CLI folder trust, silent sign-in and failed six-digit pairing between two Windows computers, then explicitly asked to finish and release together with T-163/T-164.
- Implemented terminal installation with durable results and one registration attempt; own-CLI trust confirmation; independent Windows interactive console, Linux login-shell PATH and visible launcher uncertainty; repeated public discovery requests and explicit unavailable-UDP fallback. The frontend was also dropping structured terminal errors; it now displays them with a copyable command. Exact acquaintance-machine failures remain unconfirmed.
- Fake installer, launch/registration, trust and browser interaction checks pass; local full race/cross-build/browser checks pass; PR CI and release pending. See [report](reports/2026-11-14-t153-t165-t167.md).

## 2026-11-14 — Codex (T-163 theme selection, T-164 first-run welcome)

- Reproduced the reported 0.70 settings bug using actual clicks: a popover outside the modal dialog was visible but inert. Theme and language menus now remain inside their owning dialog. The earlier T-154 visual check changed the theme programmatically and did not test option selection.
- Removed the project gate that showed only «Создайте проект» on first launch. The existing welcome page now includes project creation before coordinator setup; the empty task pane is hidden until there is a project (D-126).
- Added repeatable Chromium/WebKit checks and a CI job for real pointer/keyboard selection, system theme, persistence, bridge messages and first-project setup, using only fictional HTTP data. Screenshots at 1100/600 px in both themes checked. Native installed-app acceptance not performed. Local full repository checks passed. Not released.

## 2026-11-14 — Codex (T-154/T-161/T-162 release 0.70)

- Owner explicitly requested publication. Merged PR #101 at `4b7c687eda006e5556536fd18ddbb35d9bcd2670` after required CI checks passed (the informational Windows suite was still running).
- Published [0.70](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.70) through [release run 43584251128](https://github.com/kolotiloff/agent-messenger/actions/runs/43584251128). All 21 public assets, five signed update feeds/archive signatures, source revision and checksums verified. Full PR CI and another local `scripts/check.sh` passed. No owner installation or real-model acceptance; T-161/T-162 retain those checks. Release record branch: `codex/t154-t161-t162-release-70`.

## 2026-11-14 — Codex (T-154 cards, T-161 waiting, T-162 Cursor coordinator)

- T-154: fixed row/action geometry, retained values during diagnostics, per-provider DOM preservation and discovery/check on return from installation. Browser behavior and layout checked with fictional data in light/dark at 1100/600 px. No models or working Messenger state used.
- T-161/D-124: inspected the owner's reported coordinator chat read-only. Events were delivered, but a finished background shell watch did not resume Codex after its turn ended. Corrected client-specific waiting instructions; active wait for Codex/unknown clients, background task notifications for Claude Code only. No new idle-chat wake mechanism; real Codex acceptance still needed.
- T-162/D-125: added Cursor project connection and window rows, preserving existing config and forwarding the worker guard. Fake protocol client received a question while waiting and a report after its answer; real Cursor acceptance pending.
- Verified: macOS `scripts/check.sh`, all four cross-compilation targets (Windows completed after a dependency download failure), and browser fixture scenarios.
- Owner confirmed 0.69 released. Changes on `codex/t154-stable-agent-cards` await review and a later functional release; [verification report](reports/2026-11-14-t154-t161.md).

## 2026-11-14 — Cursor (T-160 Codex coordinator)

- Codex can be the coordinator (D-123, PR #100, `4c8af3e`). The window writes the messenger table into the project folder's `.codex/config.toml` and leaves every other line, a foreign entry, a turned-off entry and an unreadable file alone. Claude's connection is unchanged. Verified: `scripts/check.sh`. Not verified with a real Codex chat. Not released.

## 2026-11-14 — Claude Code (docs check, T-152 window, 0.68)

- Docs checked against the program (T-151). T-152 reworked with the owner over several rounds: welcome page, the coordinator as one row of «Проект» (this computer only), dialogs close on an outside click, settings and sidebar tidy-ups, feedback export without a jump. T-158 part: `messenger_version` in `list_agents`. Feedback export of 11-14 read; new T-157, T-158, T-159. Owner asked to release before going to sleep.

## 2026-11-13 — Claude Code (0.66, T-148 note texts, feedback review)

- 0.66 released (PR #97): T-139, T-146. GitHub cancelled the publish job once (no Mac runner in 15 min); one re-run published it. All 21 files checked; Linux feed build 66. The CLI watch workflow's first manual run passed.
- T-150 and T-149 (D-122) done on `claude/ui-tweaks` on the owner's word; tests and a window check with fictional data; not released.
- Read the coordinator's feedback export (15 remarks). T-148: note texts say a worker reads a note only when its run ends and that `cancel` stops it now. New tasks T-147 (phone observer, not a priority), T-149, T-150.

## 2026-11-12 — Claude Code (0.65, T-139 CLI watch, T-146 replies kept)

- 0.65 released (PR #96): T-145 (D-120). All 21 release files checked; the Linux feed says build 65.
- T-139: the agent card marks a CLI newer than the checked one «не проверена»; a daily workflow checks the flags and Codex features Messenger passes to the latest CLIs. Checked versions: Claude Code 2.1.322, Codex 0.167.2; Cursor not recorded yet.
- T-146 (D-121): a worker's reply to a computer the server does not know is parked, not deleted, and goes when that computer is back (same or new id).
- Owner: a worker restarting Unity is the project's matter; dropped from the open list.

## 2026-11-12 — Claude Code (0.64, T-145 stuck tasks close where they ran)

- 0.64 merged (PR #95) and released on the owner's word: T-140 to T-144.
- ПК-1 (0.62) refused an update with «идут задачи (3)» after the owner closed those tasks on the Mac: they were interrupted or blocked on ПК-1, which refuses a cancel for them (D-021). T-145 (D-120): an explicit cancel closes such a task where it ran once its worker is certainly gone, or reports `cancel_refused`; the window offers «Закрыть» for this computer's stuck tasks; a refused update names the tasks. ПК-1 must first get this version from its own window.

## 2026-11-12 — Claude Code (T-140 fold arrow)

- 0.63 merged (PR #94) and released on the owner's word; all release files and the five signed update feeds checked in place.
- T-140: the fold arrow of grouped actions moved after the text (variant Б of three shown to the owner). On `claude/ui-tweaks`, waits for the next functional release.
- T-139 (CLI update tracking) recorded as the next task.
- T-141 CLI error output in failures, T-142 run and network journal for `get_log`, T-143 model typo hint.
- T-144: section «Требуют решения» with «Закрыть все» (asks first); choosing a computer opens its newest task. Screenshots with fictional data sent to the owner.
- Coordinator feedback 2026-11-12 (journal of 12 entries): Cursor on ПК-1 failed with no text: its Opus limit was spent until 12/10 and Cursor said so only on its error stream (T-141). Lost report of 11-10 (task-3871…): events from ПК-1 stopped after 16:44:52, the cause is unknown without a journal (T-142). Model-list typo hint (T-143). The rest of the journal was fixed in 0.63 or stays open (server failover). A worker restarting Unity is the project's own matter, not Messenger's (owner, 2026-11-12).

## 2026-11-12 — Claude Code (T-137 lost replies from Ноут-Linux)

- Owner asked to release: 0.63 with T-137 and T-138.
- T-138: a completed run clears a stale «exhausted» limit; `idle_computers` in `list_agents`; the remote update confirmation warns about the server computer.
- The window rebuilt the sidebar every 2 s and the agent cards and task list on every task change, even when nothing on screen changed; that reset hover, tooltips and animations (the owner saw flicker). Unchanged parts now keep their nodes.
- Linux: the menu entry pointed at one version's file of a `messenger install` installation, so after an update from the Mac or by hand the menu still started the old version. It now starts the installation's `bin/messenger` link; the new client fixes an existing entry when it starts.
- Linux: a downloaded newer program updates the installation and opens from it, while the client runs; `install` still needs a stopped client and now says so in plain words instead of «client is already running». A remote task that never accepted, or never answered a cancel, closes at once; the window offers «Закрыть» for it.

- Read the coordinator's feedback journal from the Mac (11 remarks) and its issue list (row 34). Found why Ноут-Linux's replies never reached the Mac: its outbox was stuck behind 50+ messages for a full mailbox (D-119). Fixed, with a supervisor test that fails on the old code.
- The window says «Нет ответа от «X»» for an unaccepted task; `not_accepted` says when the task may be running there.
- Other remarks of the journal not handled yet: lost report from ПК-1 (11-10 18:51, may be the same cause), server failover and a coordinator event when every computer is lost, workers restarting GUI apps.

## 2026-11-10 — Claude Code (T-136 firewall after updates; 0.62)

- README rewritten for 0.61 with window screenshots (PR #93).
- The owner's remote update of ПК-2 (the server, Windows) cut the network: the firewall asked again for the new version's folder. T-136 (D-118): the Windows client now runs from one path for every version. ПК-2 itself still needs someone to press «Разрешить доступ» there.
- PR #93 merged and released as 0.62 (`v0.0.0-preview.62`); every download and signed update file is in place.

## 2026-11-10 — Claude Code (0.61 released)

- PR #92 merged and 0.61 released (`v0.0.0-preview.61`); every download and signed update file is in place. The first Windows CI run failed in the feedback export test (home folder comes from USERPROFILE there); fixed in the test.

## 2026-11-10 — Claude Code (T-133 scroll, T-135 feedback; 0.61 ready)

- T-133: the task chat keeps its place while the owner reads; down arrow, «Новые сообщения».
- T-135 (D-117): feedback journal: bug icon in the window, `report_feedback` / `export_feedback` for the coordinator, zip export.
- Coordinator list file updated with versions and given to the owner. Owner asked to release 0.61.
- Owner's UI tweaks: sidebar agents without circles, a busy agent reads «Claude работает» / «Codex ждёт ответа»; the feedback icon sits next to the version at the bottom of the sidebar; the feedback dialog keeps one height for the picture row and shows the export as folder + file name with a copy icon. Language and theme moved from the sidebar into «Настройки программы» (gear next to the feedback icon); later app settings go there too.

## 2026-11-10 — Claude Code (coordinator list rows 24, 26, 30, 31, WIP)

- T-134 (D-116): a refused answer opens the question again; `not_accepted` after 5 minutes; `network_note` when the server has not been heard from; `idle_computers` in `wait`; watch command in `--help`.
- Owner looked at moving «Отключить» next to «Подключено» and chose to keep it where it is.
- Next: T-133 scroll arrow (screenshots first), then 0.61 when the owner says so.

## 2026-11-10 — Claude Code (T-132, coordinator list rows 28 and 32, WIP)

- T-132 (D-115): tasks follow a computer into a new network by a lasting computer id; starts are never re-sent.
- T-134: watch command first in the coordinator instructions and in every `wait`/`tasks` result; `get_log` gives a reason for an empty log; Codex and Cursor failures carry the CLI's error text.
- A flaky supervisor test fixed: the background coordinator repair finishes before the store closes.
- Still to do for 0.61: rows 24, 26, 30, 31 of the coordinator list, T-133 (scroll arrow, screenshots first).

## 2026-11-10 — Claude Code (T-131 stuck tasks, WIP)

- T-131 (D-114): lost-contact status for other computers' tasks; own task out of reach shows «Закрыть» instead of input and stop; right-click «Удалить из списка». Dialogs close with a cross. Screenshots shown to the owner.
- The owner's Mac/ПК-2 mismatch comes from moving the server (new network, new ids): recorded as T-132.
- The coordinator's list (rows 24–31) still needs to be gone through.

## 2026-11-10 — Claude Code (0.60 released, busy update icon)

- 0.60 released from PR #91.
- T-128 (owner): the yellow «Обновление запрошено…» line is replaced by the update icon, dimmed and pulsing, with the state on hover. On `claude/ui-tweaks`, not released.

## 2026-11-10 — Claude Code (night: 0.60 changes T-128, T-126, T-115, T-129, T-114, T-113)

- T-128: notices above the reply box; update icon with a confirm on older computers' cards. Restart icon and «update after the tasks» deferred.
- T-126: a task on a computer out of reach says «Связь потеряна, исход неизвестен»; the coordinator sees `contact_lost`.
- T-115, T-129: the task header and `tasks` say whether the worker had «Любые команды»; a client started inside an agent sandbox is reported in the window.
- T-114: coordinator tool `note_task`. T-113 (D-113): `continue_task` for another computer's finished task.
- All on `claude/ui-tweaks`, checked with `scripts/check.sh` and fake stores only; not released (the owner releases 0.60).

## 2026-11-10 — Claude Code (release 0.59, T-130)

- 0.59 released from PR #90 (T-111, T-112, T-117–T-125, T-127).
- T-130 (D-112): the coordinator may answer a question to the owner; the window shows it as the coordinator's answer. On `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (T-121 task time label, T-122 one place for «Любые команды», T-123 restart waits for tasks)

- The update button at the bottom left shows a down arrow with a bar (owner); the page menu with «Перезагрузить» (it only reloads the page) is gone outside text. T-126, T-128, T-129 recorded for 0.60.
- T-127 (D-111): own server or joining another one, not both; «Отключить» leaves the server; a computer that joined again is listed once; the network error clears on a change.
- T-124: stop and send in the reply box are drawn icons on one grid, level and alike, with the same hover.
- T-125 (D-110): the owner's message to a worker waiting for the coordinator answers its question at once.
- T-123: the restart warning lists the running tasks and by default waits for them, then restarts by itself (owner's variant 1).
- T-122 (D-109): the switch for other computers' tasks is on «Агенты» under «Любые команды» and on by default (owner's decision).
- The owner chose variant 2 of three mockups: the task row shows «сегодня 21:49», «вчера 18:05» or the date; full times are in the tooltip. On `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (T-117, T-119, T-120, T-112, T-118)

- Coordinator's chat had six tools: its entry in Claude started the client's copy of an old version. T-117: the entry starts the installed program, our stale entry is replaced by itself, and `messenger coordinator` relays to the updated program (D-107). A chat open before this version needs one reconnect or a new chat.
- T-120: the coordinator watches in the background (`coordinator-watch`) and ends its turn, so the owner's messages reach it; wait waits at most 120 s.
- T-112: `task_messages` reads the last messages of any task. T-119: own tasks name this computer. T-118 is blocked by Q-029 (continuing an interrupted run would relax D-021).
- All on `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (T-111 open other computers' tasks)

- Owner on 0.58: some tasks in the list would not open. They were other computers' tasks, now mixed with own ones (T-110) but still not clickable. They open as a read-only card now. On `claude/ui-tweaks`, not released.
- The same owner restarted ПК-1's client during tasks after an update; the window asks first («Перезапустить всё равно») and the interrupted runs wait for the owner with an unknown result, as designed (no automatic retry).

## 2026-11-09 — Claude Code (release 0.58)

- PR #89 merged and released as 0.58 (`0.0.0-preview.58`): T-097 to T-110 and a ten-minute test budget per package in `scripts/check.sh` (the supervisor package timed out on CI at five).
- Linux computers installed with `install` cannot update themselves from 0.57 (T-107): install 0.58 there by hand once. Other computers' tasks from coordinators on 0.57 show which computer gave them, not where they run (T-110).
- Tasks from other computers run with permission checks unless «Любые команды для задач других компьютеров» is on, on the computer that runs them (D-097); the owner hit this on ПК-1 and Linux.

## 2026-11-09 — Claude Code (T-097 hints in the chat)

- The answer hint moved from under the reply box into the chat, in italics under the owner's answer (owner's choice). On `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (release preview.57)

- Merged #88 (T-096 agent filter, T-078 remote update, reply box without a needless scrollbar) and released preview.57; all files checked (Mac, Windows x64/ARM64, Linux x64/ARM64, checksums, 5 signed update feeds and archives).
- Remote update works only between computers on preview.57 or newer: the first install of preview.57 on each computer is by hand.

## 2026-11-09 — Claude Code (T-078 remote update)

- T-078 (D-100, Q-025 a): «Обновить программу» in another computer's menu; that computer installs its newest signed update and restarts its client only when no task runs; a Mac refuses. Computers report their version, shown on the cards. On `claude/ui-tweaks`, not released. Real Windows/Linux installs between computers are left to CI and the owner.

## 2026-11-09 — Claude Code (T-096 agent filter)

- The owner did not see a running task of ПК-1: the agent filter was «Cursor». T-096: choosing a computer resets the filter; a hidden active task is announced with «Показать всех агентов». On `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (release preview.56)

- Merged #87 (T-083 to T-095) and released preview.56; all files checked (Mac, Windows x64/ARM64, Linux x64/ARM64, checksums, 5 signed update feeds and archives).
- The owner works from another flat with all four computers through his own VPN; it works because the VPN gives private addresses. BROKER.md now says how a VPN works with "local network" and that 100.64.0.0/10 is refused.

## 2026-11-09 — Claude Code (coordinator's list done, security fixes)

- T-087 full last worker message kept and read with `task_output` (D-098); T-088 tokens in every report; T-090 the newest coordinator chat takes over (D-099); T-092 computers stay listed while reconnecting; T-093 `list_agents` explains empty models, limits and folders. T-089 waits for Q-027.
- T-094 security fixes (D-097, Q-028): network tasks without full access by default, outbox can no longer be stopped, broker hardening, log redaction, CI permissions and smaller items. Two helper agents did the broker and small fixes in separate worktrees; merged by cherry-pick.
- All on `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (coordinator's list, T-091)

- The coordinator's list of Messenger issues became T-087..T-093 and Q-027 (a worker that cannot commit next to another session). T-091: a failed run says why in plain words (not signed in, error exit, crashed, ended without a report) before the raw evidence; the window shows it in Russian. On `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (T-086 fresh coordinator chat)

- The owner moves all work to Messenger and wants a fresh coordinator chat each day and night. T-086: the coordinator's instructions tell a new chat to start from `tasks` and its cursor; `wait` from 0 would replay the whole history. On `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (T-085 write to a working worker)

- T-085 (D-096, Q-026 a): the owner writes to a working worker; the note waits and the same conversation continues with it when the current run ends; a report written meanwhile is held. On `claude/ui-tweaks`, not released. Security review: all four reports are in; fixes follow.

## 2026-11-09 — Claude Code (T-084, security review started)

- T-084: the reply box is one line, grows with its text and shrinks after sending; buttons at its bottom. T-085 (write to a working worker) waits for Q-026. A full security review was started at the owner's request.

## 2026-11-09 — Claude Code (T-083)

- T-083: another computer's agent card shows «Задачи: N идёт, N ждёт ответа» instead of «Задачи на этом компьютере». On `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (release preview.55)

- Merged #86 (T-079 to T-082) and released preview.55; all files checked (Mac, Windows x64/ARM64, Linux x64/ARM64, checksums, 5 signed update feeds and archives).

## 2026-11-09 — Claude Code (T-082 no title bar on Mac and Linux)

- T-082 (D-095): the page runs to the top edge on Mac (transparent title bar, system buttons at the top left) and Linux (empty GTK title bar, page buttons). Verified: Linux window under Xvfb, page in Chromium with a stubbed Mac bridge. The Swift app is checked by CI and the owner. On `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (T-081 limit advice)

- T-081 (D-094): `start_task` warns the coordinator (`limit_note`) when the chosen agent was last close to or at its limit; `list_agents` marks a passed reset. Advice only. On `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (T-080 limits)

- The owner sees no limits on the Mac and no percent elsewhere. Causes: limits come only from worker tasks on that computer; Claude sends a state, not a share we can read; Codex prints none in `exec` mode. T-080 (D-093): cards name the window and reset, clearer empty text, and the agent's own limit fields are kept for the coordinator (`limit.reported`). On `claude/ui-tweaks`, not released; not verified with real agents.

## 2026-11-09 — Claude Code (T-077, release)

- T-077 (D-091): the window restarts the old background client by itself after an update when no task runs; asks only otherwise. The owner's wish to update another computer remotely is T-078, Q-025.
- Released preview.54 (PR #85) and checked its files. T-079 (D-092): a start screen instead of the restart popup, chosen by the owner from mockups; on `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (T-076, UI tweak)

- T-076 (D-090): statistics of the whole network and filtered by the computer chosen in the sidebar; another computer's agents with their limits, view only. Usage travels with presence only when it changes. The owner approved the look from screenshots with test data. Collected on `claude/ui-tweaks`, not released.
- Language and theme lists have the same width in both languages.

## 2026-11-09 — Claude Code (T-074, T-075, UI tweak)

- Owner's first working days with the program: the coordinator stops reading the owner while it waits and forgets workers' events after an interruption (T-074, D-088); workers that could not commit finished with a cut report (T-075, D-089). Both changed on `claude/ui-tweaks` with tests; not released, the owner asked to collect changes first.
- The empty «Самые затратные задачи» card: «Пока пусто» aligned with the rows.

## 2026-11-09 — Claude Code (releases 50–53, backlog tidy)

- Merged #81 (T-071), #82 (T-070), #83 (T-072 + T-073 decision); released preview.51, .52, .53 and checked every release has all its files. preview.50 (T-057) checked too.
- T-030 and T-019 moved to Done: both were already done (T-029, T-059); Q-001 marked answered.

## 2026-11-08 — Claude Code (T-072 resource locks)

- `start_task` takes an optional `lock`; a later unfinished task with the same lock on the same computer waits to start (D-086). The window says «Ждёт ресурс «…»», the coordinator sees `waiting_for_lock`.
- Verified: core and MCP tests, `check.sh`. Not verified with real workers.

## 2026-11-08 — Claude Code (T-070 readiness)

- Each computer's presence report says whether it takes other computers' tasks and which projects have a folder; `list_agents` shows `accepts_tasks` and `has_project` (D-085). An older host refuses the field; the report then goes without it for 10 minutes.
- Verified: broker, core, supervisor tests, `check.sh`. Not verified between real computers.

## 2026-11-08 — Claude Code (T-071 task activity, T-049 split)

- T-049 split into T-070…T-073 and Q-024. T-071: the coordinator's `tasks` shows an open task's last worker action with its time and every task's tokens (D-084).
- Verified: core and MCP tests, `check.sh`. Not verified with a real coordinator.

## 2026-11-08 — Claude Code (T-057 remote log)

- Coordinator tool `get_log(computer, lines)`: request and reply go through the mailbox; the other computer answers only with «Отдавать журнал по сети» on (off by default) and removes ids, codes and keys (D-083). Screenshot: docs/mockups/log-share-built.png (stubbed).
- Verified: core, supervisor (two computers, real broker), MCP tests, `check.sh`. Not verified between real computers.

## 2026-11-08 — Claude Code (T-053 Windows installation)

- A double click installs the program for the user (D-082): programs folder, Start menu, «Приложения» with «Удалить»; data always kept (Q-023). `install` and `uninstall` commands on Windows.
- Verified: plan tests, `check.sh`, `check-builds.sh`, a new CI step that installs and removes on the Windows runner. Not verified by hand on Windows.

## 2026-11-08 — Claude Code (T-052 short invitation)

- 6-digit code next to the long one (D-081): found on the local network over UDP 47100, proved with SPAKE2, 3 tries, 10 minutes. New dependency `filippo.io/edwards25519` v1.1.0. Screenshot: docs/mockups/short-invite-built.png (stubbed).
- Verified: pairing tests on this computer (loopback), `check.sh`. Not verified: two real computers, firewalls, broadcast on Wi-Fi.

## 2026-11-08 — Claude Code (T-047 shared task list)

- Each computer shares the tasks it started (7 days, up to 50, start of the report) through the broker's `POST /v1/tasks`; the task list shows the other computers' tasks of the project read-only (D-080). Screenshot: docs/mockups/shared-tasks-built.png (stubbed data).
- Verified: broker test (limits, revoked, replacement), store test (what is shared, duplicates hidden, dropped on leaving), two-computer supervisor test, `check.sh`, headless Chromium. Not verified between real computers.

## 2026-11-08 — Claude Code (T-059 window language)

- «Язык» in the sidebar: Russian (default) or English (D-079), on top of T-069. Client messages, CLI and Mac menus stay Russian.
- Verified: translation coverage test, `check.sh`, headless Chromium in English and Russian. Not verified: native windows.

## 2026-11-08 — Claude Code (T-069 Linux window)

- Linux window with the system's WebKitGTK through `purego` (D-078), on top of T-068. Browser path kept as fallback.
- Verified in this container under Xvfb: page shown, Ctrl+C closes, no display falls back. Not verified on a real Linux desktop.

## 2026-11-08 — Claude Code (T-068 token statistics)

- «Статистика» tab (D-077): today / 7 days / 30 days, total, bar per day with a tooltip, by agent, by computer, ten most expensive tasks (click opens the task). Backend: `Store.UsageStats`, control `usage-stats`, `GET /api/usage-stats?period=`.
- Verified: core and route tests, `check.sh`, headless Chromium with stubs in both themes. Not verified: native windows, real usage.

## 2026-11-08 — Claude Code (T-062 Windows window without title bar)

- Windows window without the system title bar (D-076): own minimise, maximise, close; drag by the tab row and the sidebar top; double click maximises. Built on `claude/t061-agent-cards`, released together with T-061.
- Verified: Windows cross-build with vet (x64, ARM64), page controls in headless Chromium with a stubbed bridge. Not verified on a real Windows computer.

## 2026-11-08 — Claude Code (T-061 agent cards)

- Agent cards and the settings dialog built as approved (D-070, D-075): check, gear and on/off switch per card; four steps; one «Задачи» button. The switch keeps the agent's settings and hides it from the coordinator and other computers. Claude lists `haiku, sonnet, opus` from the start; models are one comma-separated field. Screenshot: docs/mockups/agent-cards-built.png (stubbed data).
- Cautious choices (Q-022): «Удалить» opens the official page; «CLI неисправен» not shown yet.
- Verified: core, window route, protocol and diagnostics tests, `check.sh`, headless Chromium with stubs in both themes. Not verified: native windows, real agents.

## 2026-11-08 — Claude Code (before preview.42: owner's answers)

- Owner's answers: no dollars, tokens only (D-074); server status «Подключено / Нет связи / Не подключено» (Q-021); project rename stays local (Q-019); «v0.42» display with unchanged tags (Q-020).
- Owner's order of work recorded in TASKS: design (T-061, T-062), then statistics (T-068), Linux window (T-069), language, shared list, short invitation, Windows installation, remote log, coordinator wishlist. Owner: release an update after each step.
- Verified: `check.sh`; task rows in headless Chromium with stubs show tokens only.

## 2026-11-08 — Claude Code (T-058 what a task spent)

- The task list shows each task's cost (Claude's own estimate) or tokens (Codex, Cursor); the task header shows input, cached and output tokens (D-068). Remote tasks report their total back.
- Verified: adapter and core tests, `check.sh`, mocked screenshot. Not verified: real CLI output (no real models), Cursor's usage format, anything on the owner's computers.
- Statuses shortened, with commas («Готово за 3 мин, $0.42»; the owner does not want dots between words). T-060: Claude on Ноут-Linux was blue because a chat had once used Messenger's tools there and that connection never expires; blue now means a running task only (D-069). T-059 (Russian/English window) recorded.

## 2026-11-08 — Claude Code (T-056 one switch for commands, login and logout on cards)

- The owner turned «Любые команды» on everywhere but it did not help. Cause found in code: an answer or a follow-up ran with the setting its task started with. Now such runs take the agent's current setting. Logs on the owner's computers were not read: they are not reachable from this session.
- One «Любые команды» switch for every agent of the computer on the Agents tab (D-067); the per-agent switch is gone from the dialog.
- Cards: «Войти» when no login is confirmed, «Выйти» (asks first) after a confirmed login; an agent with the standard profile is checked too, which used to leave «Не проверен» without a button. «Проверить» moved out of the card: the window checks on open and when the owner returns after signing in or out. Card actions are one row: «Задачи», «Настроить».
- preview.40 was released earlier today (T-055).
- Later the same day: «Настройки» on cards; dialog statuses stay on their button's line; click a computer in the sidebar to filter tasks; agent icon in the name line so chat text starts at the edge; a run without a report keeps the worker's last message.
- Ноут-Linux Codex: the owner's screenshot shows the cause, `~/.local/bin/codex-code-mode-host` is missing (Codex installed incompletely), so the Messenger tools can not run. Not a Messenger fault. Advised to reinstall Codex.
- Owner's idea of fetching logs from any computer recorded as T-057 / Q-018.
- Then: a slider for «Любые команды», new worker agents start with it on, the agent dialog saves by itself, the update is one «Обновить» button. Not done yet: the Server tab's on/off buttons are not sliders yet.
- Verified: tests, `check.sh`, mocked screenshots. Not verified: `claude auth logout` and the other sign-out commands on real CLIs, anything on the owner's computers.

## 2026-11-08 — Claude Code (T-055 confirm popup; project settings in the selector)

- Removing a project now asks in a popup «Удалить проект «X»?» with «Отмена» and «Удалить» instead of a second click on the same button (owner).
- The project selector has «Настроить проект…»: it opens the project dialog with the folder and rules. The folder bar on the Agents tab is gone; the current folder is shown in that dialog.
- «Любые команды» is a two-button switch «Не разрешены / Разрешены» instead of a checkbox.
- Verified: `node --check`, `check.sh`, a static screenshot of both dialogs in headless Chromium (no running client). Not verified: the flow against a live client, on Mac, Windows or Linux.

## 2026-11-08 — Claude Code (T-054 full access for workers, T-055 remove a project)

- preview.38 released from PR #68. The owner's coordinator found that only Cursor on PC-1 could commit: Claude workers wait for command approval, Codex's sandbox has no network. «Разрешить любые команды» per agent (D-065). The owner's missing task on the Mac was another project selected in the window, not a bug.
- «Удалить проект» in the project dialog (D-066): refused while a task of the project is unfinished; history kept.
- Verified here: adapter, core and window route tests, `check.sh`. Not verified: real CLIs with the switch; the dialogs in a browser.

## 2026-11-08 — Claude Code (sidebar without «координатор»; T-052, T-053 planned)

- PRs #66 and #67 merged. The sidebar no longer marks agents as «координатор» (D-064). The owner asked for a 6-digit invitation (T-052, Q-017) and a normal Windows installation with removal from Settings (T-053); both planned on `claude/t052-t053-plan`.
- Verified here: JS syntax, `check.sh`. Not verified: the sidebar on Mac and Windows.

## 2026-11-08 — Claude Code (T-051 continue a task; agent card lines)

- `continue_task` resumes a finished task's conversation with a follow-up (D-063), kept simple: no recovery after a restart; unclear or remote tasks refused. Agent cards: «Выполняет задачи» removed, each state is one short line in one font (owner's feedback on preview.37).
- Verified here: core and coordinator tests, `check.sh`, a headless Chromium screenshot of the cards. Not verified: real CLIs resuming a finished conversation.

## 2026-11-08 — Claude Code (T-050 worker rules, short reports)

- preview.37 release started from `main` da18160 (PR #65). The owner's coordinator proposed ways to save tokens; points 1 and 2 are done (D-062): per-project worker rules copied into each task (also remote), a built-in short-report rule, and `report` fields `changes` / `tests` / `files` stored as labelled lines. Continuing a finished task is T-051, kept simple at the owner's request.
- Verified here: core tests (rules stored, copied, kept, sent to the other computer, its own rules not used), report composition, MCP argument test, window route test, `check.sh`. Not verified: real CLIs filling the new fields; the dialog in a browser.

## 2026-11-08 — Claude Code (T-048 computer names)

- The owner's Linux laptop joined as «Ноут-Linux», but its own window showed the system name, and a rename in «Имя в сети» never reached the others (D-039 kept the joining name). Now each report carries the window's name and the broker renames the device (D-061); a computer that joined earlier takes its joining name. The coordinator's wishlist from the owner's project is recorded as T-049; the shared task list is T-047.
- Verified here: broker test (rename reaches the host's roster and device list, the connection file keeps it), core test of taking the joining name, `check.sh`. Not verified on the owner's computers.

## 2026-11-08 — Claude Code (T-046 owner's UI feedback)

- After updating to preview.36 the owner found: the restart button on the left and «Перезапускаем…» written in it; agent statuses empty until the setup dialog's check; stretched dialog buttons; «Подключить исполнителем» unclear; on Linux the page opened in the browser (no Chromium-based browser installed, Q-016). Changed as in D-060.
- Verified here: core test of disconnecting, window route test, `check.sh`, headless Chromium screenshots of the restart dialog, the cards (three automatic checks) and the setup dialog. Not verified: a real Windows, Mac or Linux window.

## 2026-11-08 — Claude Code (T-045 icons, Windows update restart)

- The owner's Linux agent said there is no menu entry: our doc was stale (the first `messenger open` adds one since preview.35), but the entry had no icon. Now Linux gets the icon (SVG in the user's icon folder) and the page has a favicon. Windows had no icon at all: `scripts/gen-icon` draws the Mac icon with the standard library and writes `cmd/messenger/rsrc_windows_{amd64,arm64}.syso` (a hand-written COFF resource; an external resource tool was not allowed to run here); the window loads it.
- The owner pressed «Обновиться» on Windows: «Обновлено. Окно откроется заново.», then nothing. The window was closed by `Terminate` from another goroutine; it posts WM_QUIT to the calling thread, so it was lost. Now it runs on the window's thread.
- Verified here: the PE files built for amd64 and arm64 contain the icon resources at relocated addresses (read back with `debug/pe`); tests of the generator and the Linux entry; `check.sh`. Not verified: the icon and the restart on a real Windows; the icon in a Linux desktop.

## 2026-11-08 — Claude Code (T-044 limits per agent)

- Each agent's last reported usage limit is kept and given to the coordinator as a `limit` field of `list_agents`, shown on the agent card («Лимит») and sent to the other computers (D-059). The coordinator asked for a field, not text in the report.
- Verified here: `check.sh`, Windows vet, parser tests (Claude and Codex shapes), core and a full supervisor run with a fake CLI that hits its limit. Not verified: what the real CLIs print (Q-015); a real network.

## 2026-11-08 — Claude Code (T-043 models per agent)

- PR #63 merged (722c51f); preview.36 release started. The owner's coordinator asked for exact model names for Codex and Cursor: it may not guess them. Now the owner lists them on the agent card, `list_agents` returns them, and Codex and Cursor refuse an unlisted name (D-058). `list_agents` also uses the computer's own name now.
- The owner also wants each agent's remaining limit known to the coordinator, as a separate field (T-044, not started).
- Verified here: `check.sh`, Windows vet, new core, broker, window-route tests. Not verified: real CLIs with listed names; a mixed network of preview.36 and newer computers.

## 2026-11-08 — Claude Code (T-042 owner's two-computer test, follow-ups)

- The owner's test passed: the Claude coordinator on the Mac gave Codex on Windows an audit and got the report. Follow-ups done: tool actions in the feed are grouped into one collapsible line; the computer has its own name on the «Сервер» tab, the host included; tasks of other computers are accepted by default (D-056); Codex trust is read from all `config.toml` forms; «Войти в аккаунт» and «Установить …» buttons (D-057); on Windows no inner frame and a title bar in the theme's colour.
- Verified here: `check.sh`, `GOOS=windows go vet`, new tests (trust forms, names, login commands per OS, routes), headless Chromium screenshots of the grouping and of the Windows layout. Not verified: real Windows (title bar, terminal login, install page), the real Codex config, the Codex and Cursor install page addresses (not reachable from here).

## 2026-11-08 — Claude Code (T-042 progress between computers)

- The owner ran preview.35 on a Mac and a Windows computer: Windows hosted the server, the Mac joined and gave Codex on Windows a task. It was first refused because «Задачи других компьютеров» was off (Q-014, by design); the refusal showed as an English note in the middle of the feed. After turning it on, Codex worked on Windows, but the Mac showed nothing until the end.
- Now the Mac shows «Принято на компьютере …» and the worker's progress lines (D-055), and known refusals are explained in Russian. Tested with core tests (two stores, messages carried by hand); the window change is checked for syntax only. Not tested on two real computers.

## 2026-11-08 — Claude Code (T-041 agents on Windows)

- The owner ran preview.34 on Windows: the window opened in the browser (as designed, D-045). Claude Code was found but «Проверить версию и вход» showed nothing: the check ran only on macOS and asked for login only for one exact version. Fixed (D-051). Codex was not found: newer npm packages keep `codex.exe` in `vendor/<target>/bin`, not `vendor/<target>/codex`; both are searched now. Cursor is `%LOCALAPPDATA%\cursor-agent\agent.cmd`, a script that runs `versions\<newest>\node.exe index.js` through PowerShell; Messenger now starts that pair directly (D-052). Verified with tests on Linux; not on a real Windows computer.
- The owner did not want the browser tab and console on Windows: the window is now the program's own (WebView2) on Windows and an app-mode window on Linux (D-054). Linux was run here with a fake browser (window file, page answered, cleanup on close); WebView2 is compiled only, not run.
- The owner asked for a button to choose paths instead of typing them: «Выбрать…» opens the system's dialog (D-053). Tested with a fake dialog; the real dialogs (Windows, macOS, Linux) are not checked here.
- PR #62 merged and preview.35 released (48b883e). CI found two problems on the way: GitHub's download server returned 504 for Sparkle (the download now retries, up to 15 min on `claude/t041-sparkle-retry`), and a supervisor test caught a race: `Wait` closed `StdoutPipe` before the last lines were read, so a finished task could be marked unverified. Now a plain pipe is read to the end, and output still open 5 s after the exit is not trusted.

## 2026-11-07 — Claude Code (owner's Mac results)

- The owner's Mac: the Claude coordinator, connected by the window itself in preview.34 (T-039, no Terminal), gave «Посчитать 5 + 5» to the real Cursor CLI; Cursor read the task, reported «5 + 5 = 10» and its model, and the session closed. Earlier Codex and Claude Code tasks in the same project also finished. This is the first check of T-039 with the real Claude Code.
- Noticed: Cursor's progress lines in English («I'll start by reading…») are shown in the report. Possible follow-up: show only the final report.

## 2026-11-07 — Claude Code (T-040 faster release)

- The owner found a release too slow (about 15 minutes). The Mac checks ran only after the Windows and Linux job. Branch `claude/t040-faster-release`: a separate «Mac checks» job runs next to it, and «Build, sign and publish» waits for both. The checks themselves are kept. Expected: about 10 minutes. Not verified until the next release run.
- PR #59 (T-039) merged; preview.34 released (run 42396560953, 21 assets). Checked here: the Linux x64 download, its SHA-256 and `version`.

## 2026-11-07 — Claude Code (T-039 coordinator without Terminal)

- The owner asked not to type the `claude mcp add` command. Branch `claude/t039-connect-coordinator`: the window connects the coordinator by itself after a project is created and on «Подключить к Claude» (D-050). Every worker run is marked so the coordinator server gives it no tools.
- Verified on Linux with a fake Claude CLI: unit tests (connect, already connected, another entry kept, failure shown once and not retried, no Claude agent), and an end-to-end run of the built program in a browser: a new project was connected, the entry in `~/.claude.json` started the coordinator, which answered `list_agents`; an error from Claude was shown with its message. Not verified: the real Claude Code, Windows, Mac.

## 2026-11-07 — Claude Code (T-012 Cursor names)

- The owner saw «cursor» in lower case under a computer in the sidebar. Fixed: the sidebar's short names, the task list's agent filter (it had no Cursor, so the Cursor card's «Задачи» link selected nothing), the `agent-set` help, and the `check-cli` line, which now prints «Claude Code», «Codex», «Cursor». A new test requires a proper name for every task agent in each of the window's name lists and in the filter.

## 2026-11-07 — Claude Code (release preview.33, window grant fix)

- PR #55 (Cursor) merged into PR #54; PR #54 squash-merged to `main` (56ee506); PR #53 closed (its commit was already in #54).
- The first preview.33 release run (42386214790) stopped in `check.sh`: `TestWindowAuthorizationIsScopedIdempotentAndNotRestored` saw a model start for a refused session. Cause: the window's grant (Claude test session and Codex setup) opened model calls before the store refused the alias, and withdrew them after; the 30 ms loop could claim the refused session's queued message in between. Now the store is checked first (`CheckAuthorizeSetup`, `AliasInUse`) and only then the grant is made. A new test repeats refused grants with the loop running and failed every time before the fix.
- PR #56 merged (3ae2058); preview.33 released (run 42388079985, 21 assets). Checked here: downloads via the README links, SHA-256, `version`, and the Linux amd64 update signatures against the app's key.

## 2026-11-07 — Claude Code (T-012 Cursor tasks)

- The owner said there is no rule against Cursor. Branch `claude/t012-cursor-tasks`: Cursor as a task worker (D-049), tested with a fake Cursor CLI only; no real Cursor or model calls.

## 2026-11-07 — Claude Code (D-047 downloads, T-038 updates on Windows and Linux)

- Branch `claude/t016-download-buttons`: plain downloads behind README buttons (D-047), and signed in-window updates on Windows and Linux (D-048): `internal/selfupdate`, `/api/self-update`, the window's button, relaunch, signing in the release (`scripts/sign-other-updates.py`).
- Verified on Linux here with a test key (preview.900 → preview.901 through the window); Windows-only replace tests run on the Windows CI runner. No real model calls; Cursor not run.

## 2026-11-07 — Claude Code (T-016 Windows and Linux)

- Branch `claude/t016-windows-linux`, PR #52: Windows platform layer (job objects, LockFileEx, process snapshot, boot identity, background start), `.exe`-only agents, `messenger open` for a browser window, archives for Windows and Linux in CI and in the release. D-043 to D-046; Q-014 settled (off by default).
- Windows CI found that a job's name disappears when its last handle closes; the client now keeps the handle for the run and treats an unreachable job as not proven. SQLite path on Windows fixed (drive letter).
- Windows CI also found that a rename over `active-runs.json` fails while a reader has it open; the replace and the read now retry for up to 2 s (a refused rename changes nothing).
- PR #52 merged (squash, 11d938c) after two green CI runs on 1ce5622; preview.32 released (run 42380342750).
- No real model calls; Cursor not run; no real Windows desktop used.

## 2026-11-07 — Claude Code (T-037 tasks between computers)

- PR #49 merged, preview.30 released (run 42370126728). PR #50 merged after green CI on Linux and macOS, preview.31 released from 019f30a (run 42371886488).
- Branch `claude/t037-network-tasks`: broker mailbox, remote and mirror tasks in the store (schema 3), the client's 3-second exchange, `list_agents`/`start_task` across computers, the accept switch on «Сервер» (off by default, Q-014). D-041, D-042.
- No real model calls; Cursor not run; Mac app not run. Two clients on one Linux machine exchanged a task through the window API and the coordinator.

## 2026-11-07 — Claude Code (T-015 «Сервер» tab polish)

- Branch `claude/t015-status-style`: one status style, one-row server controls with locked fields, tooltips instead of hint lines, joined devices in the sidebar with a right-click «Отозвать», icon copy in the invitation, quiet update block, font-measured text centring. The owner approved it on a live preview; released as preview.29.
- No real model calls; Mac app not run.

## 2026-11-07 — Claude Code (T-035 computers and agents in the sidebar)

- preview.29 released from df7ade8 (PR #48). Owner asked to continue without them.
- T-035: presence reports (`POST /v1/presence`), the roster in the server status, sidebar cards for other computers. VPS optional (D-040). No real model calls; a helper program played the second computer.

## 2026-11-07 — Claude Code (preview.28 released)

- PR #46 (T-033) merged after green CI on Linux and macOS; released as preview.28 from 98d346e (run 42360015163). Assets: the app zip, `update-darwin-arm64.json` and its `.sig`. Not yet checked on the Mac.

## 2026-11-07 — Claude Code (T-031–T-034)

- PR #45 merged: T-034 failure reasons and reset time, T-031 bounded `wait`, T-032 lean `tasks` (`if_version` instead of `if_cursor`, because starting or cancelling a task changes the list without an event), window tweaks.
- T-033 on `claude/t033-resume-brief`: resumed `read_task` without the task text. Fake-CLI resume test checks both the short and the `include_task` answer.
- No real model calls; Mac app not run.

## 2026-11-07 — Claude Code (no references to other products)

- At the owner's request the docs no longer name or link other products: the earlier comparison document was removed, its accepted directions moved to RELEASE_PLAN.md "Planned directions" without sources. Keep it that way: no product names, links or comparisons in code, docs, commits or PRs.

## 2026-11-07 — Claude Code (token and reliability review)

- Owner asked to change only what saves tokens, simplifies work or adds reliability. Written up in [TOKEN_REVIEW.md](TOKEN_REVIEW.md), D-038; new tasks T-031–T-034 in Next.
- Measured our fixed prompt cost (~1.2k tokens for the coordinator, ~130 for a worker); the cost is in `wait`, `tasks` and resumed `read_task` answers. No code changed.

## 2026-11-07 — Claude Code (preview.27 released)

- PR #42 merged into `main` (5881844) after green CI on Linux and macOS; released as preview.27 (run 42353801652). Assets: the app zip, `update-darwin-arm64.json` and its `.sig`.
- This update itself may still need the old background client restarted by hand (the copy-command button in the error chip): the blocking restart dialog arrives with this version and works from the next update on. Not yet checked on the Mac.

## 2026-11-07 — Claude Code (restart the background client from the window)

- Owner: after an update the window said «В фоне старая версия клиента, перезапустите его» and asked for a terminal. Now the client reports its version. While it is older than the window, a modal dialog blocks the window with «Перезапустить» or «Выйти» (the owner asked for a blocking dialog instead of a banner, since the window cannot work with an old client). It asks first if tasks are running (D-037).
- Checked: Go tests with fake clients (order of stop and start, kept pause, refused or lingering shutdown, lock wait, no downgrade, confirmation, one restart at a time). On Linux: an old client built from `main` was replaced from a preview.27 window by the button in headless Chromium; the PID changed and the client was not paused. In the browser the dialog stays open on Esc and covers the tabs, the confirmation step shows (with a stubbed answer), «Выйти» sends the quit message to the app and leaves the client running. Not checked on the Mac (the Swift quit handler is new).
- Owner: keep to the Claude app style, the buttons became too big. Controls are back to 32 px (compact 28), the dialog follows Claude's alerts (16 px padding, leaving choice on the left, main action on the right), and a destructive button has red text on a soft red fill. Measured in the browser: only 32 and 28 px controls.
- Owner: the text in buttons sat high. Measured by pixels: 9 px above the capitals, 13 below. Buttons, tabs, select fields and inputs now get a 4 px top padding (`--am-text-nudge`), which gives 11 and 11. Checked with the Linux font (Inter); the Mac font (SF) has the same kind of offset in the owner's screenshot, but it was not measured on the Mac.

## 2026-11-07 — Claude Code (window style rules)

- Owner: gaps between buttons differ between the sidebar and the agent cards; the style is scattered. The stylesheet was rewritten on one set of tokens (D-036, [UI_STYLE.md](UI_STYLE.md)); dead rules from the old chat screen were removed; agent cards and server sections share `.am-card`.
- Checked by screenshots of every tab, a dialog and a menu in both themes at 1100 and 600 px, and by measuring control heights (36 and 32 only). Not checked on the Mac.
- Owner follow-ups: the server note moved to a right column; the server is «Запустить»/«Остановить» (not «Включить»/«Выключить») in the window and in error texts; dropdown menus now look like the Claude app (4 px inset, 32 px options, background only on hover or keyboard focus, thin accent check on the chosen option). Checked in headless Chromium, both themes, mouse and keyboard.

## 2026-11-07 — Claude Code (third night: preview.25, server setup)

- Released preview.25 (PR #40: error chip, outdated-client message, task list fix).
- T-015 first part on `claude/t015-server-setup`: `internal/broker` (hosted broker, pinned TLS, one-time invitations, device keys, revoke), the control command `server`, `/api/server*`, the «Сервер» tab, `status` broker state. D-033–D-035, Q-013, [BROKER.md](BROKER.md).
- Audit by two independent reviewers; all findings fixed (connection cap, LAN peers, restart/lock race, shutdown order, settings rollback, joins, durable writes, rate limit, names, timeouts, and window state issues). T-030 recorded for an older timeout issue. PR #41, release preview.26.
- Verified on Linux with tests and headless Chromium; not with two computers, a real LAN or on the Mac.

## 2026-11-07 — Claude Code with the owner (T-027 closed)

- The owner tested preview.24 on the Mac and said everything works. T-027 moved to Done on that report. Next: Q-012, then T-028.

## 2026-11-07 — Claude Code (second night: preview.24)

- From the owner's T-027 feedback: stop button like a chat (square, Esc, D-031), model choice by the coordinator with guidance in its instructions and skill (D-030), window fixes listed in HANDOFF; the window's client now allows tasks and resumes itself (D-029); Claude workers accept edits (D-028).
- Found while testing: a relative `--root` failed with a SQLite error in the coordinator; it is now made absolute.
- Cleanup (D-032): removed code nothing calls (one was an unused helper that released a reservation by PID) and the `experiments/` folder; links now point to commit `41e0e6e`. Persistent paths kept: Q-012 asks the owner about T-028.
- Verified on Linux with fake CLIs and headless Chromium; CI on Linux and macOS. Not verified on the Mac or with real models.

## 2026-11-07 — Claude Code with the owner (first T-027 run, preview.23)

- After the update, the old client from preview.22 was gone and no client ran in the default data directory; `start --claude-stream --allow-tasks` and the window's "Продолжить" fixed it. The app lives in `~/Applications`, not `/Applications`.
- With real models on the Mac: the Claude-app coordinator started a Codex task; Codex created the file, the run was verified, and the coordinator got the report through `wait`. A Claude worker found the Messenger tools through ToolSearch (Q-010 answered: no change needed), asked the owner twice, and the same session resumed after each answer from the window and reported.
- Found: a Claude worker cannot edit without an allow rule (Q-011); the window showed "Работает" for an unlaunched task, two Claude lines in the computer card, a long title overflowing the task list, and answers needed Cmd+Enter. Fixed on `claude/t027-window-fixes`. Still open: code blocks in worker text are shown raw; "Команда" rows need their command in the title; duration should count from launch; the window says "client unavailable" for any error.

## 2026-11-07 — Claude Code (release for T-027)

- The owner asked for a build to test and chose to merge into `main` and release preview.23, so the installed app can update itself.
- macOS CI was red on PR #36. Two causes were in tests: `/tmp` is a link on macOS, and a reservation check ran a moment too early. One was a product bug in the persistent Claude path: the loop could claim a second stream before the first had taken the exclusive startup lease, so the second message became uncertain. Reproduced on Linux under CPU load; the loop now starts one stream process at a time.

## 2026-11-06/07 night — Claude Code (T-022, T-024–T-026, T-029)

- The owner asked for autonomous work overnight; PR #35 (T-009) was cancelled and left as is.
- T-022: replies and sends are held until the run is verified.
- T-024: task core (projects, folders, tasks, runs, worker tools, verification, events). T-025: coordinator MCP with `wait`. T-026: worker profile and trust check.
- Real Codex 0.155.1 and Claude Code 2.1.319 ran the worker path against local fake model APIs. Found that Claude 2.1.319 defers MCP tools behind ToolSearch (Q-010).
- T-029: the new window from the approved mockup, checked in headless Chromium. Q-009 (how the window allows task launches) and D-027 recorded.
- All on `claude/task-core-night`, [PR #36](https://github.com/kolotiloff/agent-messenger/pull/36). Next: T-027 on the Mac.

## 2026-11-06 — Claude Code with the owner (window direction)

- Agreed the future window with the owner on a mockup built from the real window files: an observation panel with a "Tasks" tab and coordinator status. Owner questions show as a yellow dot and inside the task's conversation, with an answer field only while the worker waits; the coordinator does not read that exchange. Recorded as D-024; added T-029.

## 2026-11-06 — Claude Code with the owner (Q-008 decided)

- The owner accepted the task model: D-021 (one task, one session; Claude-app coordinator with `wait`; clarifications resume the same task session; best-effort cleanup), D-022 (planning rules and parallelism live in each project's docs and the coordinator's judgement; no approval or worker limit in Messenger), D-023 (workers use their project's settings). Codex as coordinator deferred.
- D-004 partly, D-005 and D-016 superseded for new work; the first rule in `AGENTS.md` updated accordingly. Tasks T-024–T-028 added; T-009 superseded; T-021 and T-023 done.
- Next: T-022, then T-024.

## 2026-11-06 — Claude Code with the owner (T-023 on the Mac, Claude app)

- The owner ran the wait pilot in a local Claude Code-tab session. The coordinator waited 5 min in the foreground, woke by itself on each event, kept waiting after the owner wrote in between (the app moved the call to the background and it still woke the chat), lost nothing, and had no empty wake-ups. [Report](reports/2026-11-06-wait-pilot-claude-mac.md).
- Found: the Code tab did not load the project `.mcp.json`; `claude mcp add --scope local` in the test folder worked. README updated. Go was downloaded into `~/AgentMessengerTest/go-sdk` to build the probe.
- Next: Codex (app and CLI), then the owner decides Q-008.

## 2026-11-06 — Claude Code (T-023 wait pilot)

- Read the Codex review of RETHINK version 3 ([RETHINK_REVIEW_V3.md](RETHINK_REVIEW_V3.md)): better direction, pilot first. The owner agreed to start the pilot.
- Added `experiments/wait_probe`: a stdio MCP server with `wait` and `status`, a `send` command standing in for worker reports, a call log, progress notifications during long waits, and `setup.sh`, which prepares `~/AgentMessengerTest/pilot-coordinator` without touching global Claude or Codex settings.
- Verified on Linux without models (race tests, manual run). Codex config keys come from search results; the Codex docs were not reachable. Next: the owner runs the scenarios in the README on the Mac.

## 2026-11-06 — Codex (T-021 version 3 review)

- Reviewed `RETHINK.md` at `7d02b62` against the unchanged product code and official Claude Code/Codex documentation. Saved findings and all seven answers in [RETHINK_REVIEW_V3.md](RETHINK_REVIEW_V3.md), including convenience, illustrative token costs and effectiveness.
- Recommend a bounded task pilot, server-enforced approved plans, idempotent operations and replayable events. Within-task ask/answer still needs an explicit resume policy; actual desktop wait behavior is an acceptance gate. Q-008 remains pending.
- Native Mac arm64 `scripts/check.sh` passed on identical product code with Go 1.27.3, including all race tests; an initial restricted run could not access test sockets/process observations. Documentation only; no real models, product changes or releases.

## 2026-11-06 — Claude Code (RETHINK version 3)

- With the owner, replaced the proposal with version 3: the coordinator is the owner's own Claude/Codex chat with Messenger as MCP and a skill; one task = one new worker session; the coordinator waits with a blocking `wait` tool, which spends no tokens while open.
- Checked the Claude Code docs for waking an idle chat: cross-session messages, `asyncRewake` hooks and channels (research preview, terminal CLI) exist; none is needed for the first version.
- Q-008 rewritten with four owner decisions. Next: Codex review of version 3. No code changed.

## 2026-11-06 — Claude Code (RETHINK version 2)

- Checked the Codex review against the code: R2 (reply routed before verification), R3 (one request per attempt) and R4 (strict tool-sequence validators) are confirmed.
- Rewrote [RETHINK.md](RETHINK.md) as version 2: weaker ownership guarantee named plainly, provisional reports, one message per run, profile-aware validators, migration before removal, owner-only budget reset. Section 9 maps each finding to its change.
- Q-008 now lists the three decisions for the owner. Added T-022 (publish reply after verification), which is useful whatever Q-008 decides. No code changed.

## 2026-11-06 — Codex (T-021 review)

- Reviewed `RETHINK.md` against `59d58eb` on `claude/vigilant-darwin-34d88b`; findings and all five section 9 answers are in [RETHINK_REVIEW.md](RETHINK_REVIEW.md).
- Recommendation: accept direction with changes. Main gaps: escaped-descendant ownership, reports routed before verification, batching, worker validators and migration of existing owners. Q-008 remains pending; existing safety rules are unchanged.
- Mac arm64 `scripts/check.sh` passed with Go 1.27.3, including all race tests. Documentation only; no product changes, real-model runs or release.

## 2026-11-06 — Claude Code (rethink)

- Discussed the concept with the owner. Wrote [RETHINK.md](RETHINK.md): one-shot runs that resume a saved conversation instead of persistent processes, real tools for workers, questions to the owner. No code changed.
- Found while reading the code: the one-shot path already exists but keeps every session reserved after its first run; and every launch path strips the agent to `read_current`/`reply`, so no path can yet run a worker that edits files.
- Added Q-008 and T-021. Next: Codex review, then the owner's decision.

## 2026-11-06 — Claude Code (T-004 on the Mac)

- Released preview.21 from CI. The owner updated, logged in to a separate Codex profile and created a Codex session from the window. Real Codex answered both messages, with both turns verified.
- Found and fixed: the Codex setup from the window did not resume the paused queue. [PR #33](https://github.com/kolotiloff/agent-messenger/pull/33).

## 2026-11-06 — Claude Code (releases from CI, T-006)

- The owner added the `SPARKLE_ED_PRIVATE_KEY` secret. Merged [PR #31](https://github.com/kolotiloff/agent-messenger/pull/31) and started the first CI release, preview.20.
- T-006: a Codex session can be created from the window in one confirmed step. [PR #32](https://github.com/kolotiloff/agent-messenger/pull/32).

## 2026-11-06 — Claude Code (incident: app would not open after reboot)

- preview.19 window failed with "environment must be an existing directory". The reservations pointed to `/private/tmp/am-claude-nee2aviw`, which macOS had deleted on reboot. The owner verified the evidence and moved the registry aside; the window works again with empty history.
- Added `recover-missing-root`, the window's refusal of temporary data directories, and a rule against `/tmp` data directories in acceptance runs. Fixed a CI race in the test helper that waited for the control socket. All of this is in [PR #31](https://github.com/kolotiloff/agent-messenger/pull/31).

## 2026-11-06 — Claude Code (T-008)

- Persistent Claude streams now run in their own goroutines; the main loop no longer blocks on one. `current` became a job map, `stop-job` and shutdown stop every stream, and the limit is 4 concurrent streams. Test: two sessions, two processes, two turns each. [PR #31](https://github.com/kolotiloff/agent-messenger/pull/31).

## 2026-11-06 — Claude Code (T-003)

- Found on real Codex 0.155.1 that `config/read` returns config keys in a different order on every call. Fingerprints are now taken over canonical JSON. Without this, every re-check would have refused the turn.
- The config is re-checked before every turn; a mismatch returns the unsent message to the queue. Added a schema version with refusal of newer databases. [PR #30](https://github.com/kolotiloff/agent-messenger/pull/30).

## 2026-11-06 — Claude Code (T-005)

- Merged [PR #29](https://github.com/kolotiloff/agent-messenger/pull/29) (T-002, T-007) after green CI.
- Reply routing between local agents with `MaxAutoReplyHops` = 3. The end-to-end test with fake Claude and fake Codex stops at the limit, with both owners still alive. [PR #30](https://github.com/kolotiloff/agent-messenger/pull/30). Several Claude streams were split off as T-008.

## 2026-11-06 — Claude Code (T-007)

- `recover-codex`: two-phase retirement of an uncertain Codex setup after a proven reboot (same rule as `recover-stream`). Tests cover refusal on the same boot, with a live client, with a wrong confirmation, after a finished recovery, and resuming a half-done recovery. [PR #29](https://github.com/kolotiloff/agent-messenger/pull/29).

## 2026-11-06 — Claude Code (T-002)

- Codex message delivery: owner controller, `codex_turns`, `RunTurn` with an early-event buffer and server-request handling, bound tools through SQLite. Tests with a fake app-server cover two turns, pause, and six failure modes. No model was called. [PR #29](https://github.com/kolotiloff/agent-messenger/pull/29).
- Codex sessions are stored without a CLI path, so older clients cannot launch them.
- The owner reported a Mac freeze and reboot; no Mac action is needed for this work.

## 2026-11-06 — Claude Code (T-001)

- Installed Codex 0.155.1 from npm in the cloud session. Generated the app-server schema. Confirmed on the real binary that `thread/start` declares dynamic tools, needs no login and leaves no rollout for an empty thread.
- Implemented Codex session setup: an RPC client, the `codex_setups` table and `CreateCodexSession`, with tests. Fixed the windows test build and a socket-path limit in the test harness. `check.sh` and `check-builds.sh` pass. [PR #28](https://github.com/kolotiloff/agent-messenger/pull/28).
- Next: T-002.

## 2026-11-06 — Claude Code

- Took over from Codex. Read the project; `scripts/check.sh` passes on Linux.
- Translated all documentation to English; added `AGENTS.md` (shared rules) and `CLAUDE.md` (imports it). [PR #26](https://github.com/kolotiloff/agent-messenger/pull/26), merged.
- Closed draft PRs #2–#18 and gave the Russian ones English titles; their content is in `main`.
- Added the working documents: `docs/README.md`, `HANDOFF.md` (was `HANDOFF_CLAUDE.md`), `TASKS.md`, `DECISIONS.md`, `QUESTIONS.md`, this log.
- Next: T-001.

## 2026-11-05 – 2026-11-06 — Codex (summary)

Built the product from the first Go client to preview.19 in about 50 commits:

- persistent Claude with pause/resume, exact resume and recovery after a reboot;
- the background client;
- diagnostics (`doctor`, `check-cli`);
- the overview and timeline read APIs;
- the local UI with sending and a durable outbox;
- Mac preview packaging and install;
- CI on Mac and Linux;
- the native Mac shell and Sparkle updates from GitHub Releases;
- session setup and scoped launch from the window;
- Codex app-server preflight and turn guards.

Reports for each step are in [reports/](reports/). The handoff is in [HANDOFF.md](HANDOFF.md).

## 2026-11-06 — clickable window mockup

- Published a clickable mockup of the D-024 window (computers, task list with agent filter, task feed, answer field for an owner question). The owner approved it.
- Owner's note: the mockup is a layout reference only. T-029 builds on the program's existing styles and code, not on the mockup's code. Recorded in T-029 and D-024.

## 2026-11-06 — window mockup approved

- The owner approved the clickable window mockup after a series of changes: one working folder per computer (D-025) shown as a compact bar on the Agents tab with per-agent trust; sidebar agents with status dots and a "координатор" word; task rows in three lines (agent logo and name, title, coloured status with duration); agent filter as a dropdown; role tags next to names in the feed; no "·" separators; capitalised statuses; spacing from stylesheet tokens.
- The mockup is saved in [docs/mockups/2026-11-06-task-window.html](mockups/2026-11-06-task-window.html) so any agent can open it. It is a layout reference; T-029 builds on the program's own styles and code.

## 2026-11-06 — several projects (D-026)

- The owner wants to be able to run two games at once with two coordinator chats. Recorded as D-026: projects exist from the start (name, coordinator connection, folder per computer, tasks); agents are shared. The idea T-030 is folded into T-024, T-025 and T-029.
- The approved mockup now has a project switcher at the top of the sidebar ([docs/mockups/2026-11-06-task-window.html](mockups/2026-11-06-task-window.html)).

## 2026-11-08 — design drafts saved

- Saved the draft mockups that are still being agreed into `docs/mockups/` (settings dialog, per-card switch, Windows window without the system title bar) and wrote the points already agreed into T-061; added T-062 for the title bar. Nothing built; waiting for the owner's approval.

## 2026-11-08 — T-063 widths and empty-click

- The sidebar and the task list can be resized by dragging their edge (double click restores, width remembered). A click on empty sidebar space shows all computers' tasks again. Checked in headless Chromium against the page assets only; not in the native windows.

## 2026-11-08 — T-064 tab «Проект»

- The project's settings moved from the project list to a new tab «Проект» (D-071). The project list now holds only projects and «Новый проект». Ellipses dropped from all button labels. Checked in Chromium with a stubbed window API; not against a live client or in the native windows.

## 2026-11-08 — T-064 auto-save, T-065 project rename

- The «Проект» tab saves by itself and shows only errors; «Удалить проект» stands on the right; the folder label is «Папка проекта».
- A project can be renamed on the tab. Only the shown name changes (D-072); Q-019 asks whether a rename should reach other computers and the coordinator. Core and route tests added; the page checked in Chromium with a stubbed API, not against a live client.
- Owner idea recorded as T-066: the update offer as a blue button at the top right.

## 2026-11-08 — «Любые команды» text

- The explanation under «Любые команды» on the Agents tab is normal-size muted text, not a small hint (owner). Checked in Chromium.

## 2026-11-08 — T-067 Server tab, variant Б

- The owner chose variant Б: three cards in one column, the accept switch inside «Этот компьютер», the name saves by itself, no «Сохранить», «Проверить связь» or side note, one computer name. The owner also approved the agent cards with the per-card switch and the settings dialog (T-061, not built yet). Checked in Chromium with a stubbed API only.

## 2026-11-08 — T-066 update icon, version name

- An available update is a blue round arrow next to the version at the bottom of the sidebar (owner's variant А); tooltip names the version. The window shows «v0.41»; tags stay 0.0.0-preview.N (D-073, Q-020). Checked in Chromium with a stubbed status only.
