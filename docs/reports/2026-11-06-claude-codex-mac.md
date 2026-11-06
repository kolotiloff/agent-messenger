# First real Claude ↔ Codex exchange (Mac, preview.22)

2026-11-06, the owner's Mac (Apple Silicon), Agent Messenger preview.22. The preview was released from CI.

## Setup

- The owner updated to preview.22 and restarted the background client. As designed, the previous Codex session became uncertain.
- In the window, the owner created new Codex test sessions in the logged-in managed profile `~/AgentMessengerTest/codex-profile`.
- The owner created new Claude test sessions and launched them.
- The window listed four sessions with model calls allowed: two Codex and two Claude.

## Run

The owner wrote to `claude-test-4a8255c9`: «Спроси у codex-test-xxxx, какой сегодня день недели, и перескажи ответ» ("Ask codex-test-xxxx what day of the week it is today, and tell me the answer"). `codex-test-xxxx` was the alias the owner gave the Codex session.

1. **Claude sends.** Claude read the message, called `send` to `codex-test-xxxx`, and replied to the owner that the question was sent and the answer would come separately. In the meantime it gave its own date: 2026-11-06, Friday. Turn verified.
2. **Codex answers.** Codex received the message from `claude-test-4a8255c9` and answered «Сегодня пятница.» ("Today is Friday."). Turn verified.
3. **The answer comes back.** The answer was routed back to Claude. Claude replied «codex-test-xxxx ответил: «Сегодня пятница». Это совпадает с моей датой (2026-11-06, пятница).» ("codex-test-xxxx answered: 'Today is Friday.' That matches my date (2026-11-06, Friday)."). Turn verified.
4. **The extra turn.** Under the current routing rule, Claude's last reply was addressed to Codex, so it was queued for Codex once more. Codex confirmed with «Да, всё верно: 6 ноября 2026 года — пятница.» ("Yes, that's right: 6 November 2026 is a Friday."), turn verified. That reply was the fourth step, so the automatic-step limit stopped it and it was not routed.

## Findings

- **The first real agent-to-agent exchange works.** It covered the `send` tool, routing back to the sender, several window grants and verified turns on both sides.
- **Extra turn.** Step 4 is an extra Codex turn that only acknowledges. Proposal: do not route a reply whose message was itself a routed reply. A conversation then continues only through an explicit `send`.

## Not verified

- A run longer than one question.
- Three or more agents.
