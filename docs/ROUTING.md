# Replies between agents

A reply always goes to the sender of the message it answers. The code sets the recipient; the model cannot choose it.

When that sender is the alias of another open local session, the reply is also queued for that session as a new message. This happens in the same SQLite transaction that stores the reply, so a stored reply is never left undelivered. This is how Claude and Codex talk to each other on one computer: a message to Codex from the alias `worker`, a Claude session, is answered by Codex. The answer is queued for `worker`, and Claude's answer to it goes back to Codex.

The delivered message:

- has the ID `reply-<reply id>`, so it exists exactly once;
- has the replying alias as its sender and the reply text as its text;
- records `in_reply_to`, the message it answers, and `hops`, its place in the chain.

## Limit

A chain starts with a message that is not itself a delivered reply (`hops` 0). At most `MaxAutoReplyHops` = 3 replies in one chain are delivered automatically.

The reply that would be the fourth delivery is stored and shown, but it is not queued, and an `auto_reply_limit` event is recorded. Two agents therefore cannot answer each other forever.

## Replies that are not routed

A reply is not queued for anyone in these cases:

- **Unregistered sender.** The message came from a sender that is not a local alias, for example the user in the window.
- **Same session.** The reply would go back to the session that wrote it.
- **Closed session.** The recipient session is closed. A `reply_not_routed` event is recorded.

## Who processes a delivered message

A delivered message is an ordinary queued message, so the usual rules apply:

- It is processed only while model calls are allowed for the recipient and the queue is not paused.
- A Claude session needs `serve --claude-stream`. A Codex session needs its live owner from `create-codex-session`.
- An uncertain session keeps it queued.

## Limits of this version

- **Concurrent owners.** Up to 4 persistent Claude sessions and any number of Codex owners run side by side.
- **Sending is Claude-only for now.** Codex threads declare their tools when they are created, so existing Codex sessions only have `read_current` and `reply`.
- **Local only.** Routing works between sessions of one client. Network delivery between computers is a later stage.

## Agents starting a message (`send`)

A persistent Claude session has a third tool, `send(to, text, receipt)`. It lets a coordinator hand a task to another agent.

- **Who it can write to.** `read_current` also returns `from` and `agents`, the aliases of the other open local sessions.
- **Receipt.** `send` requires the turn's current receipt, like `reply`. The turn still has to end with `reply`.
- **The new message.** It is queued for the recipient with the ID `send-<id>`, from the sending alias. It has `in_reply_to` set to the current message, and `hops` is one more than the current message's.
  - It counts toward the same chain limit, so a send past `MaxAutoReplyHops` is refused.
  - At most `MaxSendsPerTurn` = 3 sends are allowed per turn.
  - The recipient must be another open session, never the sender itself.
- **The answer.** It comes back to the sender through ordinary reply routing.
- **Checking the turn.** The Claude turn check (`ValidateClaudeStream`) accepts `read_current`, then up to 3 `send` calls, then `reply`.

The window can authorize up to 4 test sessions in one client run, for example one Claude and one Codex session that talk to each other.

