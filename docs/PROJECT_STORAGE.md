# Project-local storage (T-258)

## Current implementation

Messenger still runs tasks from one `queue.sqlite` per computer. Removal from the window preserves the project and its history there (D-066). The first step towards project-local storage is a consistent, independently inspectable SQLite copy of a single project. It works for a visible project or one removed from the window.

```sh
messenger --root DIR project-snapshot --name NAME --output PATH.sqlite
```

The destination must be a new file in an existing directory that supports hard links. The command reads the live queue without beginning a writer transaction. It copies the selected project's records, worker runs, questions, notes, events, action items, output, report images and feedback. The file keeps task and run IDs, event sequences and existing columns. SQLite checks the staged copy before it becomes visible at the requested path. Network membership, unsent messages, coordinator claims, agents and the reservation registry do not go into this history copy. The client refuses to use the snapshot as a live queue. No data is removed from the source.

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

## Live layout

- Keep settings, machine identity, network membership and shared resource ownership in the computer database. The project catalog owns a stable local storage ID and maps the existing network project name to it. The display title can change independently.
- Put tasks, their runs, events, questions, notes, outputs, images and task usage in a database for each project. Keep the durable outbound task messages and inbound task-operation records in the same database as the task. A worker result and its outbound report must be committed together.
- Route incoming task messages by the existing project name for a new assignment and by the stored task identity for subsequent messages. Advance the computer's broker cursor only after the project's transaction commits. Replaying an already committed delivery must be harmless even if the cursor update failed. Existing peers can continue sending frames containing only the old task identity.
- Keep physical resource ownership and live-process recovery machine-wide. Claiming a resource and recording a launch intent must remain recoverable between the two databases. Any ambiguity retains the resource and the reservation until observed evidence resolves it.

SQLite WAL can commit an individual database atomically. Attaching multiple files does not make a commit crash-atomic across them. The design must preserve the delivery and reservation invariants with explicit durable steps. [SQLite ATTACH documentation](https://www.sqlite.org/lang_attach.html)

## Migration and archive behaviour

The staging command already preserves task IDs, mirror and origin links, old operation deduplication, unacknowledged messages and event cursors. The running client still needs storage dispatch and recovery that use those prepared files. Verify project row counts, SQLite integrity, pending delivery routing and that each task still opens with its history. Activate the new storage only after that verification and while old writers are stopped. Keep a recoverable copy of the old data directory. A failed staging pass must leave the old layout usable and a second pass must not duplicate a task or report.

Keep ordinary removal reversible by archiving the project history after its tasks and pending deliveries settle. Recover the archive on reopen. Permanent deletion of the archive is a separate owner action. Live routing, the migration cutover and restore, and the window's archive workflow are pending. Verify a restart during migration, a mixed-version two-computer delivery and an update waiting for work from more than one project.
