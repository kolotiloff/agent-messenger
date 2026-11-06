# Working local interface: Mac

November 6, 2026, macOS arm64, Go 1.22.2. Changes on top of PR #9.

The full `go test -race -count=1 -ldflags=-linkmode=external ./...` passed. The external linker's LC_DYSYMTAB warning persists with the Go version used; the test processes completed successfully. The JavaScript passed a syntax check. Builds with CGO_ENABLED=0 for linux/amd64 and windows/amd64 passed; the runtime on other OSes was not verified.

New server tests verify: refusal without a key, wrong Host/Origin and a cross-site request, no arbitrary proxying and a ban on data-modifying commands, page limits, the deadline of a request to the client, no raw errors/Detail, private response headers and the presence of embedded resources.

In the browser, the real local UI server and the Messenger client with SQLite in a separate temporary profile were tested. The data was prepared specifically for the test: three registrations, 55 messages, one synthetic linked reply. These are not results of real Claude/Codex/Cursor. The client had no authorization to launch models; the executable path of the test registrations is not a provider.

Verified: loading the last 50 messages and appending the remaining five without duplicates, an empty other session, cards, display of the confirmed test reply, HTML-like text without creating HTML elements. After reading, the snapshot of all tables matched the original. After the test client was stopped, the window showed a connection error, cleared the previous cards and did not present the unavailable queue as empty. Widths of 320 and 1024 px were checked; there is no horizontal overflow of the root on a narrow screen. Scrolling of the long-conversation area was added, showing the latest messages on open.

The test UIs/clients were stopped and the temporary tab was closed. No working user sessions were started. There is no installer, autostart, sending from the window or native shell yet. The browser preview is a check of reading and of the chosen design before the next UI commands.
