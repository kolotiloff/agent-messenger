# Agentgram: comparison and ideas for Messenger

Checked on November 6, 2026. The [dfradehubs/agentgram](https://github.com/dfradehubs/agentgram) repository was examined at `f5bb5f0eb2674592fb82ff9f7d6ce7b753ed423e`: documentation, agent model, context routing and the group coordinator. Agentgram was not run here; this is a source analysis, not a full operational or security acceptance. Below, facts about Agentgram are kept separate from decisions for our plan.

## What we have in common and where we differ

Both projects give a person a shared interface for several agents, history, and connection via MCP. Agentgram already offers a web interface, streaming events, group conversations, user permissions, integrations and server deployment. It also has a local mode; it must not be described as requiring Kubernetes. It is an adjacent product and a partial competitor. [Project description](https://github.com/dfradehubs/agentgram).

Agentgram connects agents mainly over network protocols: REST/SSE, A2A, Google ADK. The Claude Code/Cursor it mentions also act as clients of its MCP gateway: they call tools of connected server-side agents. This does not mean owning their local CLI processes or safely resuming an existing conversation. In the model, routing and documentation reviewed, no ready equivalent of our management of local CLI sessions was found. [MCP](https://github.com/dfradehubs/agentgram/blob/f5bb5f0eb2674592fb82ff9f7d6ce7b753ed423e/docs/MCP.md), [agent model](https://github.com/dfradehubs/agentgram/blob/f5bb5f0eb2674592fb82ff9f7d6ce7b753ed423e/api/internal/models/agent.go).

| Approach | Advantages | Cost and limitations |
|---|---|---|
| Messenger: adapter for an existing CLI session | The working folder, tools, login and context of the chosen provider are preserved. Suits agents on personal computers. | Depends on the behavior of each CLI and OS; completion, resume and protection against parallel runs are complex. An ambiguous result blocks further work. |
| Agentgram: shared gateway for network agents | More universal at the protocol level; a richer ready-made web interface and access control. | The agent must expose a supported API or get a wrapper. An existing local CLI session does not automatically become a network agent. |
| Mailboxes and MCP only | Simple core, weak dependence on the provider. | Someone still has to wake the agent, deliver the message and confirm processing. Having an MCP tool does not by itself start a turn in an idle GUI chat. |

Our current implementation is less mature in breadth of features: persistent Claude on Mac has been accepted; the four-computer network, persistent Codex/Cursor and the other OSes have not yet been accepted. Fitting our scenario does not prove that we are more reliable than Agentgram.

## Decision for our architecture

Keep the CLI adapters, but separate network delivery from execution. The network core addresses a logical agent on a device and stores the envelope and acknowledgements. The local binding determines which adapter and which specific session receive the message. Later, an API agent or an external MCP client may sit behind the same address.

Currently the SQLite queue references `session_key` directly; delivery independent of the session is not yet implemented. Before the network stage, the binding version and migration must be defined. An already accepted message must not be silently redirected to a new session after the binding changes. Locking of the exact CLI ID and the uncertain-result rules remain in the adapter.

The coordinator is an assignable role of one of the agents. We do not introduce a mandatory extra model inside the broker. Handing over the role does not transfer internal history between providers.

## What we take into the plan

| Priority and stage | Idea and source | Readiness condition |
|---|---|---|
| Stage 3: connection | A setup wizard modeled on Agentgram's first run: step-by-step choice of agent, connection and checks. [Changelog](https://github.com/dfradehubs/agentgram/blob/f5bb5f0eb2674592fb82ff9f7d6ce7b753ed423e/CHANGELOG.md). | CLI installation, login, compatibility and session readiness are shown separately. A status check does not call the model. No false "connected". |
| Stages 2–3: observation | A live stream of available events: reply, tool start/result, waiting, error. Reconnecting the interface restores the view from an event cursor. Agentgram documents buffering and returning to a running job. [Changelog](https://github.com/dfradehubs/agentgram/blob/f5bb5f0eb2674592fb82ff9f7d6ce7b753ed423e/CHANGELOG.md). | A window drop and reconnection do not create a new message or a model run. We do not promise hidden reasoning that the provider does not expose. |
| Stage 5: network | Separation of transport and execution, logical addresses, binding version, idempotent IDs and acknowledgement after write. [Agentgram architecture](https://github.com/dfradehubs/agentgram/blob/f5bb5f0eb2674592fb82ff9f7d6ce7b753ed423e/docs/ARCHITECTURE.md). | Disconnecting/reconnecting a device does not lose a message and does not repeat a model run already started. Changing the binding does not change the recipient of an accepted message. |
| Before automatic group chains | Pass only new relevant context; limit size, number of hops and budget. Agentgram assembles a bounded context delta between agents. [context.go](https://github.com/dfradehubs/agentgram/blob/f5bb5f0eb2674592fb82ff9f7d6ce7b753ed423e/api/internal/proxy/context.go). | Context is tracked by confirmed IDs/cursors; truncation is explicit. Input/output tokens and the number of calls are measured. A loop stops at the limit without losing history. |
| Stage 5: permissions | Restricted MCP tools and allowed recipients, a log of control actions. Agentgram has permissions on tools and agents. [MCP](https://github.com/dfradehubs/agentgram/blob/f5bb5f0eb2674592fb82ff9f7d6ce7b753ed423e/docs/MCP.md). | An agent does not get the right to change settings/keys merely because it can send a message. The log contains no secrets. |
| After basic exchange between four devices | A separate REST/A2A adapter, if a real scenario appears. | One API agent passes the same contract for IDs, delivery, errors and limits, without bypassing the CLI guarantees. |

The connection wizard, the event stream and view recovery are the nearest product ideas. They do not replace the current priority of updates. REST/A2A protocols, corporate SSO/RBAC, Slack, public links and a marketplace do not become mandatory tasks for the first release.

## Token savings

In Agentgram's group mode, the model chooses the next participant; an additional summary step may also call the model. This applies to the group coordinator, not to all single messages. [orchestrator.go](https://github.com/dfradehubs/agentgram/blob/f5bb5f0eb2674592fb82ff9f7d6ce7b753ed423e/api/internal/orchestrator/orchestrator.go).

In our case, routing, ACK, waiting, window refresh and connectivity checks are done by ordinary code. A coordinator agent still spends tokens when it plans or replies. A saved CLI session does not make its context free; zero spend while idle is not unique to Messenger. A percentage of savings cannot be promised without an identical scenario and measurements. Useful measures are batching small messages, context deltas, loop protection and no LLM for service delivery.

## Borrowing code

So far only ideas have been recorded; no Agentgram code has been carried over. The repository uses [MIT](https://github.com/dfradehubs/agentgram/blob/f5bb5f0eb2674592fb82ff9f7d6ce7b753ed423e/LICENSE); if code is carried over in the future, keep the notices the license requires. Do not carry over the whole stack just for one feature.
