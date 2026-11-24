# T-289 — Release readiness audit

The candidate integrates PRs [#192](https://github.com/kolotiloff/agent-messenger/pull/192), [#193](https://github.com/kolotiloff/agent-messenger/pull/193), [#194](https://github.com/kolotiloff/agent-messenger/pull/194) and [#195](https://github.com/kolotiloff/agent-messenger/pull/195), on top of the local computer marker already in `main` (#191). The installed release baseline is `v1.0.0-preview.103`.

## Findings fixed

| Failure | Fix and regression check |
| --- | --- |
| The sidebar treated two numbered builds with the same display version as equal. A computer on build 103 could lose its update button when build 104 was available and both displayed as 1.0. | Compare numeric build numbers for numbered versions and retain the display name for the label. `check-window-update-watch.cjs` covers both the offered update and the new version arriving ahead of presence. |
| A queued update remained on the server's shared status board when the requesting member was revoked before the result could be delivered. | A target's empty queue clears the mark once it has reported that exact request as pending. Update the presence and the mark in one transaction, then wake the observers. `TestUpdateWatchClearsWhenTheExecutorReportsTheQueueEmpty` covers expiry without a requester and a delayed report of the same request. |
| The result of one update could clear the flag for a later request to the same peer, opening task admission during the later update. | Preserve a pending request with a different ID while reconciling the reported version. `TestNetUpdateResultClearsStalePendingFlag` checks the new request and worker admission. |

The browser fixture for switching projects now serves an empty same-origin page with a successful response. Chromium had rejected its previous 404 navigation before the fixture could clear storage and test the first-run welcome.

## Coverage

| Area | Scenarios |
| --- | --- |
| Project data and statistics | Legacy data partitioning, failed migration and retry, independent project histories and time ranges, durable delivery and cursor recovery, pending work across projects, and explicit resource locks. |
| Agents and task lifecycle | Coordinator wakeups and project isolation, a first run waiting for a resource, actual model reporting, question/answer/report delivery, delayed exit and provider capacity after a durable report, and quarantine of runs whose exit is uncertain. |
| Remote task controls and updates | Single stop submission, durable delivery, final task state from the executor, shared update status, exact-request cancellation, the five-minute boundary for an unstarted install, and a running installer protected from cancellation. |
| Window | Both themes, 600 px and 1100 px widths, first-run setup, project and computer switching, status and model labels, reduced motion, picture viewer, and quiet behavior when hidden. |
| Distribution | Cross-compilation and local macOS ARM64 application build. Test packages check clean install, refusal to overwrite a running client, restart/reopen, preservation of the queue and outbox, and deduplication after installing a newer binary. |

The three regression checks failed on the integrated branch before their corresponding fixes and passed afterward. The integrated source passed the local native Go race suite on macOS ARM64 before the audit fixes. The affected Go packages were exercised again with focused race tests. The public channel checker also verified the signed manifests and matching archives of existing build 103 for all five targets.

The release and any fleet rollout are separate from this source audit. The signed updater against a new public release and mixed-version computers on the owner's network have no live acceptance result here. No real model, working database, or installed client was used as a fixture.
