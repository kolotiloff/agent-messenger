# Python client draft

This directory is kept, not connected to the product. It was started as a persistent client in Python; before tests and a commit, the decision changed: the persistent client and the future broker are written in Go. There are no plans to finish this draft. The product command is `cmd/messenger`.

The draft is not part of `go test` and is not stage acceptance. After moving to this directory, its own tests were run here once:

```sh
python3 -m unittest discover -s experiments/python_client_draft/tests -q
```

Rerun in this Linux environment: 31 tests, all passed. This is a check of the preserved draft, not product acceptance and not a replacement for the Mac reports. The product checks are the Go client tests.

What is useful in it as scenario reading, alongside the already verified test bench `experiments/session_safety`:

- `agent_messenger/behavior.py` — wording for pause, job stop, client shutdown and the list of limitations.
- `agent_messenger/identity.py` — the canonical environment directory and session key, not an arbitrary profile name.
- `agent_messenger/store.py` — queues, aliases, reply deduplication and refusal to release a lock without proof.
- `agent_messenger/adapters.py` — explicit resume and event checking for Claude, Codex and Cursor.
- `agent_messenger/process_info.py` — a process snapshot is not declared proof that all descendants have exited.

A divergence not carried over to Go: on the next start, the draft itself moves someone else's active attempt to `uncertain` and revokes its MCP. In the Go client, `shutdown` does not do this. Until the operator calls `stop-job`, an already running CLI may still finish writing a reply, while a new attempt for this session still does not start.
