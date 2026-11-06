#!/usr/bin/env python3
"""Summarize a Codex acceptance run from `messenger inspect` output.

Usage: messenger --root DIR inspect | python3 scripts/codex-acceptance-summary.py ALIAS [--resume]

With --resume it checks a resumed session (T-009): the same thread served by
a second app-server after recover-codex --resume and resume-codex-session.

Prints states and reply texts only. Thread IDs, session keys, tokens and
receipts are not printed, so the output can be shared.
"""

import json
import sys


def main():
    args = sys.argv[1:]
    resume = "--resume" in args
    args = [a for a in args if a != "--resume"]
    if len(args) != 1:
        sys.exit("usage: messenger --root DIR inspect | codex-acceptance-summary.py ALIAS [--resume]")
    alias = args[0]
    snap = json.load(sys.stdin)
    setups = [s for s in snap.get("CodexSetups") or [] if s.get("alias") == alias]
    if not setups:
        sys.exit(f"no Codex setup for alias {alias!r}")
    setup = setups[-1]
    print(f"setup: state={setup.get('state')} exit_observed={setup.get('exit_observed')} "
          f"thread_stored={bool(setup.get('thread_id'))} detail={setup.get('detail')!r}")

    keys = {s["key"] for s in snap.get("Sessions") or []
            if s.get("provider") == "codex" and s.get("start_mode") == "codex_owned"}
    requests = [r for r in snap.get("Requests") or [] if r.get("session_key") in keys]
    attempts = [a for a in snap.get("Attempts") or [] if a.get("session_key") in keys]
    tokens = {a["token"] for a in attempts}
    replies = {r["reply_to"]: r for r in snap.get("Replies") or []}
    verified = [v for v in snap.get("Verifications") or [] if v.get("attempt_token") in tokens]

    for r in requests:
        reply = replies.get(r["id"])
        text = reply["text"][:200] if reply else None
        print(f"message {r['id']}: state={r.get('state')} reply={text!r}")
    pids = {a.get("pid") for a in attempts}
    ok = sum(1 for v in verified if v.get("provider_ok") == 1 and v.get("exit_observed") == 0)
    print(f"attempts={len(attempts)} distinct_processes={len(pids)} verified_turns={ok}")
    for e in snap.get("Events") or []:
        if str(e.get("kind", "")).startswith(("codex_", "auto_reply", "reply_")):
            print(f"event {e['kind']}: {str(e.get('detail'))[:120]}")

    if resume:
        # Messages lost with the old owner may stay uncertain; nothing may be
        # left waiting, and turns must have completed in two processes.
        resumed = any(e.get("kind") == "codex_thread_resumed" for e in snap.get("Events") or [])
        done_pids = {a.get("pid") for a in attempts if a.get("state") == "turn_completed"}
        print(f"resumed={resumed} processes_with_completed_turns={len(done_pids)}")
        passed = (setup.get("state") == "created" and resumed and len(done_pids) >= 2
                  and all(r.get("state") in ("completed", "uncertain") for r in requests))
    else:
        passed = (setup.get("state") == "created" and len(requests) >= 2 and ok == len(requests)
                  and all(r.get("state") == "completed" for r in requests) and len(pids) == 1)
    print("RESULT:", "PASS" if passed else "CHECK THE LINES ABOVE")


if __name__ == "__main__":
    main()
