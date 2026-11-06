# Registry and data directory audit: Mac, November 6, 2026

An additional check of installation preparation found that the shared `loadRuns` successfully decoded JSON `null` into a nil slice, which was then treated as having no reservations. With such file contents, this allowed `reconcileRuns` to let a different database through. The installer already had its own strict array check, but the shared startup and recovery must be protected the same way.

A regression test was added and first run on Mac before the fix: it failed with `null registry accepted as empty: <nil>`. After the fix, a nil slice from an existing file returns `ErrRunsCorrupt`. A correctly written empty array `[]` is allowed; a missing file on the very first start also remains allowed.

The checks cover `null` with whitespace, rejection of a different database, refusal of rememberRun and recover-failed, refusal of a full Supervisor.Run with zero launches of the test launcher, and the file staying unchanged. These checks run in isolated temporary user directories. No real models or live reservations were used; Cursor CLI was not run.

This is not proof that any real process has exited, and not a mechanism for releasing reservations. The registry value is not manually fixed or deleted.

Additionally, a bypass via a missing/relative Root was found and reproduced: `[{}]` could match the current directory `.` after filepath.Clean; the same name `data` in different working directories does not identify one database. The second test failed before the fix with `unidentified root matched current directory: [{}]: <nil>`.

Existing records with a missing or relative root are now treated as corrupt. A new serve with a relative --root is allowed, but before doing any work it resolves the directory to an absolute physical path; that path is what gets stored in new reservations. Comparing the stored root with the current directory uses the physical path. An inaccessible directory does not allow startup. Old ambiguous records are not rewritten and not interpreted relative to the new working directory.

A separate runtime test starts a test persistent process from a relative directory, checks the absolute physical Root in the registry, and checks that a link to the same database is allowed. This is a fake process from the test binary, not Claude/Codex/Cursor. The original record and a symbolic link to the same database are not treated as two different directories.

Path normalization ran into the macOS Unix socket path length limit. The database identity uses the physical path, while the local connection uses the shorter original/absolute spelling without resolving symbolic links. The server and client share SocketPath. This keeps the working short /tmp and shortens long relative paths with many .. components. The first run revealed bind/connect errors in these cases; after the fix, a separate runtime test of a relative launch and shutdown passed.

The full final suite `go test -race -count=1 -ldflags=-linkmode=external ./...` passed on Mac. The earlier failed runs are kept as evidence of detection and fixing; the outcome is not based only on successful compilation.
