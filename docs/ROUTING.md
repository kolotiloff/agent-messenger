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
- **No direct sending by agents.** Agents cannot start a new conversation themselves. Only `read_current` and `reply` exist while a message is processed. A separate coordinator mode for sending new messages is not implemented.
- **Local only.** Routing works between sessions of one client. Network delivery between computers is a later stage.
