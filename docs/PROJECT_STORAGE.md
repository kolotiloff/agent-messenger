# Project-local storage (T-258)

## Current implementation

The development build selects a verified split layout during normal client startup. `queue.sqlite` retains the original data and the active generation selector. The selected generation holds `computer.sqlite` for shared settings and one database for each project. A single-project export works from the active layout or the legacy queue, including when the project was removed from the window (D-066).

```sh
messenger --root DIR project-snapshot --name NAME --output PATH.sqlite
```

The destination must be a new file in an existing directory that supports hard links. The command reads the live queue without beginning a writer transaction. It copies the selected project's records, worker runs, questions, notes, events, action items, output, report images and feedback. The file keeps task and run IDs, event sequences, existing columns and indexes. SQLite checks the staged copy before it becomes visible at the requested path. Network membership, unsent messages, coordinator claims, agents and the reservation registry do not go into this history copy. The client refuses to use the snapshot as a live queue. No data is removed from the source.

## Staging and verification

```sh
messenger --root DIR project-storage-stage --output-dir NEW_DIR
messenger project-storage-verify --dir NEW_DIR
```

Staging takes one SQLite backup so all files agree on a single point in time. It creates:

- `legacy-backup.sqlite` with the source database intact.
- `computer.sqlite` with the shared settings and network state, the existing receipt-deduplication ledger, a project catalog and a task routing index. Its task-history tables are removed.
- One database per project with its history, existing event cursor and pending task messages that have an unambiguous project route. Replies to a remote start are matched by the sender and remote task ID. Machine messages and unresolved legacy replies stay in the computer database. Existing message IDs, order, parked state and bytes are copied.
- `manifest.json` with project files, task and pending-message counts, cursors and SHA-256 digests. It is published last. The verify command rejects a missing manifest or a changed or missing database.

No worker is launched and the live database keeps all history and pending messages. Archive and staged computer files are refused if opened as an ordinary working queue. An interrupted stage can be run again. If a process stopped while publishing files and left an output directory without the manifest, choose a new output directory for the next attempt. The existing output is never reused or silently overwritten.

An offline migration check used a synthetic two-project queue created by the 0.94 release source. Format 5 verification passed. The completed project's report, transcript output and usage stayed together. The other project's unsent assignment retained its sequence, operation ID and exact bytes. Both routes were in the computer database. The pending continuation receipts and prior task states added to the fixture went to their respective project files. The same fixture was activated on a separate copy. The ordinary queue opener selected the split files after restart, and the pending assignment kept its exact bytes.

## Delivery across separate files

The receiver and sender components use format 5 of the prepared layout. The verify command can still check formats 1 through 4. Inbound responses for already assigned tasks use the verified project route. Inbound answers, notes, continuations and cancellations additionally require the stored sender to match the task's origin. The task change and its operation receipt commit in the project database. The computer's broker cursor advances only after that commit. If the process restarts between commits, the replay finds the project receipt and advances the cursor without repeating the task change. An unresolved route stops the advance. The existing single-database receiver uses the same handlers and deduplication.

The staged outbox keeps the old queue order and starts new local sequences beyond the previous high-water mark. The split sender reads the computer and project queues and acknowledges a message by its source database, sequence and operation ID. Coincident sequence numbers in different projects do not cause deletion or duplication.

A known computer that reconnects with a new broker or peer keeps its project routes and pending replies. The project updates its task and outbox first, then the computer catalog updates its route and any pending start acceptance. Redelivery repairs a catalog rollback from the validated computer ID. It never creates a second assignment.

Machine-wide changes, including an update result or loss of network contact, queue notices in the same computer transaction. A coordinator reads notices for its project through the ordinary task-event cursor. The project's receipt prevents duplicates if shared acknowledgement fails. A network-loss notice counts only the open tasks of the receiving project.

The integration tests deliberately fail the cursor write after saving a report or note, reopen the files and verify one copy and eventual cursor advancement. The worker receives the answer and pending note in the next run. The test also checks the machine's paused-agent policy on a continuation and cancels a second continuation before it can launch. The other project remains independent. The sender now passes the selected message with its database identity when accepting or rejecting a delivery.

## Local task admission across project databases

Local task admission reads the project folder and rules, agent setup and pending updates from the computer database. It writes the task and its first run intent together in the project database before registering the route in the computer database. A missing route prevents launch. Repeating the same explicit operation after a failed route write registers the already committed task. It creates no second task or run.

Resource locks are optional and domain-neutral. Messenger supplies no resource name by default. Tasks that explicitly name the same resource on the same computer share its queue across projects. Tasks without that shared name are independent. The common admission transaction serializes checks across separate database connections. The project run state remains authoritative after it is recorded: a launch with an uncertain outcome still holds the resource when the files reopen. The existing process registry remains required before starting a worker.

Tests exercise two project files, a failed route registration, explicit replay, concurrent admission through independent connections and a verified refusal before exec. The legacy single-database task entry points reuse the preparation and launch-state helpers. The normal client now uses this dispatch path after selecting the split layout.

## Remote task admission

An incoming assignment uses the exact project name and the same admission policy as the existing client. The project database commits the mirror, its original sender identity and its first launch intent. The computer database commits the confirmed route, the start reply and the broker cursor together. This keeps the acceptance reply invisible until the route exists. A delivery replay after a failed computer commit finds the same mirror and publishes its route without a second run. An existing remote task identity cannot create another mirror in a different project.

The two-project test injects a cursor write failure, checks that the unregistered run cannot launch, reopens both files and replays the assignment. It also delivers a separate assignment for the other project and a request for a missing project. The new scoped store supports task admission, candidate selection and recording a launch, including remote follow-ups. An installed build with this change uses the selected generation after migration.

## Outgoing assignments and worker completion

The assigning computer records the task and its start message in the same project transaction. The sender only selects a start that has a confirmed computer route for the matching project, broker and peer. An interruption between commits leaves the single committed task and envelope intact. Repeating its explicit operation repairs the route without generating another start. An ineligible start cannot exhaust the send batch or hold back another project. The test sends two such assignments to a legacy executor and applies both acceptances to their own project files.

The supervisor passes the selected database to the usual task MCP entry point. The project worker store checks the run, its computer and its registered project route before opening the existing file. The sender ID copied at staging must match the computer database. The existing process-observation, identity and receipt checks still govern access. A worker completion saves its result, usage, full output and delivery frames in one project transaction. An injected outbox failure rolls back all of those changes. The tests exercise task MCP with the correct and incorrect project or receipt, then simulate process evidence to verify reporting. They do not start a real worker or alter the shared process registry.

## Window revisions

The computer database tracks machine and coordinator changes. Project databases track their own task and transcript revisions. Staging regenerates the derived counters while preserving the history and event cursors they describe. The scoped store builds window snapshots, paged history, feeds, revisions and usage statistics from the selected project. The worker test verifies that a failed completion emits no changed revision and a committed completion changes the task and window revisions for exactly its own project. Tests cover seven- and thirty-day usage, ten completed cards per page and no cross-project activity.

## Live layout

- Keep settings, machine identity, network membership and shared resource ownership in the computer database. The project catalog owns a stable local storage ID and maps the existing network project name to it. The display title can change independently.
- Put tasks, their runs, events, questions, notes, outputs, images and task usage in a database for each project. Keep the durable outbound task messages and inbound task-operation records in the same database as the task. A worker result and its outbound report must be committed together.
- Route incoming task messages by the existing project name for a new assignment and by the stored task identity for subsequent messages. Advance the computer's broker cursor only after the project's transaction commits. Replaying an already committed delivery must be harmless even if the cursor update failed. Existing peers can continue sending frames containing only the old task identity.
- Keep physical resource ownership and live-process recovery machine-wide. Claiming a resource and recording a launch intent must remain recoverable between the two databases. Any ambiguity retains the resource and the reservation until observed evidence resolves it.

SQLite WAL can commit an individual database atomically. Attaching multiple files does not make a commit crash-atomic across them. The design must preserve the delivery and reservation invariants with explicit durable steps. [SQLite ATTACH documentation](https://www.sqlite.org/lang_attach.html)

## Migration and archive behaviour

The activation component holds the legacy SQLite writer transaction during the backup, verifies and syncs the staged generation, and commits its selector with a fence against older writers. A launch or conversation still owned by the old store stops the switch. The previous file and a backup remain available for recovery. A failed selector commit leaves the previous queue usable. If the commit result is uncertain, the candidate generation stays available for the next ordinary open to resolve from the committed selector.

Creating a new project writes its own file before adding the catalog entry. A failed catalog commit can reuse the same file on explicit replay. Ordinary removal hides the project and keeps its separate history for restoration under the same name. It is refused while the project has open work. The update gate counts work from all projects without sending their titles to another project's coordinator. Normal desktop, CLI and service startup invoke activation while holding the existing supervisor lease. If the shared process registry still contains a reservation, the queue owns a run, or an observed coordinator process is still using the legacy file, startup keeps the legacy layout and tries the switch at the next normal restart. `status` reports `preparing`, `pending` or `active`. During preparation the control socket accepts status, pause, resume and shutdown. Task and network processing start only after selecting the queue. The existing readiness handshake is compatible with the 0.94 update helper. The old file and backup stay on disk. The application feedback journal stays in `computer.sqlite`, and project history exports include its remarks for that project.

## Verification

- The macOS development package was built and the desktop bridge passed its start, attach and reopen checks. The window’s regular restart action replaced a running 0.94 client with the candidate on a synthetic 188 MB queue in 2.69 seconds. The window returned separate task lists and seven- and thirty-day statistics with a computer filter. The migrated queue reopened with the same generation. It kept the 300 completed tasks, their output and usage, and the other project's pending network message, including its exact body and operation ID.
- In the same synthetic archive under one window request per 500 ms, the median window response fell from 57.09 ms for 300 completed rows in 0.94 to 1.65 ms for 10 rows in the development build. The background process's mean CPU over the sampled intervals fell from 18.63% to 0.56%. These measurements are specific to the test and do not measure a live VPS.
- Code built from the 0.94 release source exchanged assignments and reports with the split client for two projects in both directions. Replaying the same deliveries produced one report per task. The exchange test used the existing wire frames and simulated process evidence, without a live multi-host installation.
- The normal supervisor fixture ran two fake workers concurrently on the migrated layout and recorded each report in its project. An open coordinator and a retained launch prevent migration. Coordinator event reads no longer take the writer lock, and the background watch keeps the same cursor on transient contention.
- Both Mac targets plus Linux and Windows amd64 compile. Signed distribution through the public channel and mixed-version deployment on separate computers still require acceptance before release.
