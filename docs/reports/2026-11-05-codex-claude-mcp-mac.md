# Codex ↔ Claude: MCP and continuation of both sessions on a Mac

Date: 2026-11-05. **Exchange and continuation of both sessions — PASS. Full scenario — PARTIAL:** the last confirmation was delivered as a new message, not linked to the previous reply.

## Verified scenario

CLI: Codex 0.155.1, Claude Code 2.1.318. A local experimental MCP server was used, with a shared SQLite database, separate inboxes and a fixed identity for each process.

1. Codex sent Claude a random control string via `send_message` and exited.
2. Claude received only an instruction to check its inbox — without the control string in the prompt. It read the message via `read_messages` and sent `ACK:` with the received string via `reply`. The link to the original message was preserved.
3. Codex continued the original session by exact ID, read the reply and sent a `RECEIVED:` confirmation with its text.
4. Claude continued its original session by exact ID, read the confirmation and returned its exact text.

All four CLI processes exited with code 0 and successful provider results. For both agents, the IDs before and after continuation matched. The database has three messages; contents and recipients are correct. Message contents were not carried between prompts by hand.

Actual MCP log:

```text
codex  send_message
claude read_messages
claude reply
codex  read_messages
codex  send_message
claude read_messages
```

The server log, the database and the tool call events of both CLIs agree. Claude received no permission denials in this scenario.

## Reply-linking problem found

At the third step, the prompt asked to reply to the received message but did not specify the exact tool name or the `message_id` argument. Codex chose `send_message` instead of `reply`. The confirmation reached Claude, but its `reply_to` is empty.

The strict message-link check stopped the test controller after the third step. The message had already been stored, so the send was not run again. After reading the log, only the independent check of Claude resuming was continued. The initial check failure is kept in the results; the later check separately distinguishes successful delivery from a missing link.

This is an observation from one run, confirmed by logs, not proof of consistent Codex behavior. Even a more precise instruction does not by itself replace a protocol check.

**Requirement for the future client:** when processing a message that requires a reply, set the original message context programmatically and verify the result's link before marking it "done". A new standalone message and a reply must be distinguished. A mismatch must not trigger a blind resend or a second model launch. This requirement is not yet implemented in the production client.

## Settings and limits of the check

Claude was launched in a separate folder with `--strict-mcp-config`, an explicit test server, and built-in tools, hooks and slash commands disabled. Only `read_messages` and `reply` were allowed without prompting; `dontAsk` mode. All three server tools were visible in Claude's catalog, including `send_message`, which was not in the allowed list. Refusal when trying to call it was not checked separately. Full permission bypass was not used.

Codex used a separate configuration with the test MCP and with shell, apps, plugins, hooks and subagents disabled. Authorization settings were not copied. Raw logs, conversation IDs, control strings and account data stay outside the repository.

One more part of H5 was confirmed, as well as Claude continuing with MCP after the previous process exited. Together with previous experience, the Codex–Cursor and Codex–Claude pairs have been verified. A direct Claude–Cursor exchange and all three working simultaneously were not verified.

This is a local test bench on a Mac: no VPS, network broker, background client, production authorization or protection against two session owners. Runs were sequential. H6/H7/H8/H9 and Windows/Linux remain open. Token savings were not measured; model polling while waiting was not used. The extra confirmation was part of the test, not a proposal to have the model do technical ACKs in a production system.

## Next steps

Pin down reply linking at the client protocol level. Then verify session ownership, a second concurrent launch and an uncertain result after a failure. These checks can mostly be done without models, using controlled test processes.

Parameter sources: [Claude CLI](https://code.claude.com/docs/en/cli-reference), [Claude permissions](https://code.claude.com/docs/en/permissions).
