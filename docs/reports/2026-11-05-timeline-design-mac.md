# Message log and preserving the design

November 5, 2026, macOS arm64, Go 1.22.2. Changes on top of PR #8.

The full `go test -race -count=1 -ldflags=-linkmode=external ./...` passed on Mac. `CGO_ENABLED=0` builds for linux/amd64 and windows/amd64 passed. The program was not run on other OSes. The external linker's LC_DYSYMTAB warning remains a quirk of the Go version used; the test programs completed successfully.

Verified: history pages with identical timestamps and a new message arriving between reads, a shared log of two aliases, rejection of a foreign/nonexistent cursor, page boundaries and an empty array. Reading does not change the database snapshot. Verified: Unicode truncation of long text with a flag, and preservation of the link of a repeated message.

A reply before confirmation stays `not_verified`. A confirmed turn of the persistent process has `verified_turn` without an invented exit code; stopping the stream does not erase this result. Exit code 1 gives `process_failed`, even with reply text and a positive provider_ok. Spoofing the reply link or the sender/recipient, or missing confirmed identity or provider result, does not yield success. Execution tokens, profile paths and CLI IDs are not included in the result.

The integration check reads the log three times through a real local socket. The queue remains, no attempts or replies appear, and the stub launcher is not called. No real models or working sessions were used.

The user chose the style of the demo mockup. Preserved: the unmodified original fragment, the exact CSS, the color/size parameters, an HTML copy for the browser and SHA-256. Reconciliation confirms a match with the original shown in the chat and the absence of unsupported host calls other than optional view state. This preserves a previously verified mockup; it is not an implementation of the product window. The export shell's helper scripts use a CDN.

At the user's request, the official Claude Desktop and Remote Control documentation was studied, and a plan was added for verifying a graphical coordinator and dictation. Compatibility of these modes with the current stream-json adapter has not been verified; GUI connection, authorization and voice input were not run.
