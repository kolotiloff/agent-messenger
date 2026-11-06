# Card overview: verification on Mac

November 5, 2026, macOS arm64, Go 1.22.2. Continuation of the diagnostics work after PR #7.

The full `go test -race -count=1 -ldflags=-linkmode=external ./...` passed on Mac. External linking is needed for this old Go on the current macOS; the linker warns about LC_DYSYMTAB, and the test programs exit successfully. `CGO_ENABLED=0` builds for linux/amd64 and windows/amd64 passed; nothing was run on those OSes.

The new checks cover merging aliases of a single binding, excluding CLI identifiers/tokens/paths/message texts from the overview, the database snapshot being unchanged after reading, distinguishing owner hold, idle and uncertain, explicit list truncation and an empty JSON array. API errors do not return raw database error contents.

The integration check starts the local client with a stub launcher, requests the overview three times through the real local socket and confirms: the message stays in the queue, there are no attempts or replies, and the launcher is not called. Neither the user profile nor real models are used by this check.

An interactive concept of the app was prepared in the chat. In the browser, the following were checked: agent cards, navigation between sections, adding a demo message with a queue update, VPS selection and the layout on a narrow screen (320 px, no horizontal overflow of the root). The concept is not connected to the client and is not acceptance of the window. The server, network, coordinator and remote agents in it are demo states.

The next step is a bounded API for the message log and linked replies, then the first window with real local data. The limitations on process termination, persistent Codex/Cursor and other OSes have not been lifted.
