# Project-local storage (T-258)

## Current implementation

Messenger still runs tasks from one `queue.sqlite` per computer. Removal from the window preserves the project and its history there (D-066). The first step towards project-local storage is a consistent, independently inspectable SQLite copy of a single project. It works for a visible project or one removed from the window.

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

An offline migration check used a synthetic two-project queue created by the 0.94 release source. Format 3 verification passed. The completed project's report, transcript output and usage stayed together. The other project's unsent assignment retained its sequence, operation ID and exact bytes. Both routes were in the computer database. This exercise covered preparation and verification of the files. Live activation still requires a separate acceptance run.

## Delivery across separate files

The receiver and sender components use format 3 of the prepared layout. The verify command can still check earlier format 1 and 2 layouts. Inbound responses for already assigned tasks use the verified project route. The response and its operation receipt commit in the project database. The computer's broker cursor advances only after that commit. If the process restarts between commits, the replay finds the project receipt and advances the cursor without repeating the result. An unresolved route stops the advance. The existing single-database receiver uses the same report and deduplication code.

The staged outbox keeps the old queue order and starts new local sequences beyond the previous high-water mark. The split sender reads the computer and project queues and acknowledges a message by its source database, sequence and operation ID. Coincident sequence numbers in different projects do not cause deletion or duplication.

The integration tests use two real SQLite files and deliberately fail the cursor write after saving a worker's report, then close and reopen the files. They verify one report, no change to the other project, and eventual advancement of the broker cursor. Outbox tests verify exact legacy order and acknowledgement of only the selected file. These split receiver and sender components have not been connected to the running supervisor.

## Local task admission across project databases

Local task admission reads the project folder and rules, agent setup and pending updates from the computer database. It writes the task and its first run intent together in the project database before registering the route in the computer database. A missing route prevents launch. Repeating the same explicit operation after a failed route write registers the already committed task. It creates no second task or run.

Resource locks are optional and domain-neutral. Messenger supplies no resource name by default. Tasks that explicitly name the same resource on the same computer share its queue across projects. Tasks without that shared name are independent. The common admission transaction serializes checks across separate database connections. The project run state remains authoritative after it is recorded: a launch with an uncertain outcome still holds the resource when the files reopen. The existing process registry remains required before starting a worker.

Tests exercise two project files, a failed route registration, explicit replay, concurrent admission through independent connections and a verified refusal before exec. The legacy single-database task entry points reuse the preparation and launch-state helpers. Split dispatch and process execution have not been activated.

## Remote task admission

An incoming assignment uses the exact project name and the same admission policy as the existing client. The project database commits the mirror, its original sender identity and its first launch intent. The computer database commits the confirmed route, the start reply and the broker cursor together. This keeps the acceptance reply invisible until the route exists. A delivery replay after a failed computer commit finds the same mirror and publishes its route without a second run. An existing remote task identity cannot create another mirror in a different project.

The two-project test injects a cursor write failure, checks that the unregistered run cannot launch, reopens both files and replays the assignment. It also delivers a separate assignment for the other project and a request for a missing project. The remote supervisor, worker process and later messages still use the legacy dispatch path in the running app.

## Outgoing assignments and worker completion

The assigning computer records the task and its start message in the same project transaction. The sender only selects a start that has a confirmed computer route for the matching project, broker and peer. An interruption between commits leaves the single committed task and envelope intact. Repeating its explicit operation repairs the route without generating another start. An ineligible start cannot exhaust the send batch or hold back another project. The test sends two such assignments to a legacy executor and applies both acceptances to their own project files.

The project worker store checks the run, its computer and its registered project route before opening the existing database. It provides the computer's immutable sender ID on each SQLite connection and keeps persistent reads and writes in the project file. The existing process-observation, identity and receipt checks still govern access. A worker completion saves its result, usage, full output and delivery frames in one project transaction. An injected outbox failure rolls back all of those changes. The end-to-end storage test forwards a verified report to a legacy origin after recovery. It simulates process evidence and does not start a worker or alter the shared process registry.

## Window revisions

The computer database tracks machine and coordinator changes. Project databases track their own task and transcript revisions. Staging regenerates these derived counters, while preserving the history and event cursors they describe. The reader checks the computer catalog and reads the selected project's revisions. It cannot use a task's revision through another project. The worker test verifies that a failed completion emits no changed revision and a committed completion changes the task and window revisions for exactly its own project. The running window still needs to use this reader at cutover.

## Live layout

- Keep settings, machine identity, network membership and shared resource ownership in the computer database. The project catalog owns a stable local storage ID and maps the existing network project name to it. The display title can change independently.
- Put tasks, their runs, events, questions, notes, outputs, images and task usage in a database for each project. Keep the durable outbound task messages and inbound task-operation records in the same database as the task. A worker result and its outbound report must be committed together.
- Route incoming task messages by the existing project name for a new assignment and by the stored task identity for subsequent messages. Advance the computer's broker cursor only after the project's transaction commits. Replaying an already committed delivery must be harmless even if the cursor update failed. Existing peers can continue sending frames containing only the old task identity.
- Keep physical resource ownership and live-process recovery machine-wide. Claiming a resource and recording a launch intent must remain recoverable between the two databases. Any ambiguity retains the resource and the reservation until observed evidence resolves it.

SQLite WAL can commit an individual database atomically. Attaching multiple files does not make a commit crash-atomic across them. The design must preserve the delivery and reservation invariants with explicit durable steps. [SQLite ATTACH documentation](https://www.sqlite.org/lang_attach.html)

## Migration and archive behaviour

The staging command already preserves task IDs, mirror and origin links, old operation deduplication, unacknowledged messages and event cursors. The running client still needs activation, worker-run and process-registry routing, and storage dispatch in the supervisor. Verify project row counts, SQLite integrity, pending delivery routing and that each task still opens with its history. Activate the new storage only after that verification and while old writers are stopped. Keep a recoverable copy of the old data directory. A failed staging pass must leave the old layout usable and a second pass must not duplicate a task or report.

Keep ordinary removal reversible by archiving the project history after its tasks and pending deliveries settle. Recover the archive on reopen. Permanent deletion of the archive is a separate owner action. Live supervisor routing, peer rebinding across files, migration cutover and restore, and the window's archive workflow are pending. The next release requires active separate databases, verified migration from 0.94, and two projects working at the same time. Acceptance covers a restart during migration, a mixed-version two-computer delivery and an update waiting for work from more than one project.
