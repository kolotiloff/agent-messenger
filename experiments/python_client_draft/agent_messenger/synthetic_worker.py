"""Synthetic execution used by tests and by --allow-synthetic. Never a model."""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import sys
import time

from agent_messenger.errors import Rejected
from agent_messenger.mcp_server import invoke
from agent_messenger.store import Store


def run(db, token, session_id, run_dir):
    run_dir = Path(run_dir)
    run_dir.mkdir(parents=True, exist_ok=True)
    run_dir.joinpath('worker.pid').write_text(str(os.getpid()), encoding='utf-8')
    advertised = session_id
    if os.environ.get('AM_TEST_SEAM') == '1' and os.environ.get('AM_ADVERTISE_SESSION_ID'):
        advertised = os.environ['AM_ADVERTISE_SESSION_ID']
    print(json.dumps({'type': 'system', 'subtype': 'init', 'session_id': advertised}), flush=True)
    store = Store(db)
    try:
        deadline = time.monotonic() + 20
        while True:
            try:
                message = invoke(store, token, 'read_current', {})
                break
            except Rejected:
                if time.monotonic() > deadline:
                    return 2
                time.sleep(0.02)
        try:
            invoke(store, token, 'reply', {'text': 'ACK:' + message['text']})
        except Rejected:
            return 2
        run_dir.joinpath('replied').write_text('1', encoding='utf-8')
        end = time.monotonic() + 120
        while not run_dir.joinpath('release').exists():
            if time.monotonic() > end:
                return 3
            run_dir.joinpath('heartbeat').write_text(str(time.time()), encoding='utf-8')
            time.sleep(0.05)
        run_dir.joinpath('exited').write_text('1', encoding='utf-8')
        return 0
    finally:
        store.close()


def main(argv=None):
    parser = argparse.ArgumentParser(description='Synthetic messenger worker. Does not call a model.')
    parser.add_argument('--db', required=True)
    parser.add_argument('--token', required=True)
    parser.add_argument('--session-id', required=True)
    parser.add_argument('--run-dir', required=True)
    args = parser.parse_args(argv)
    sys.exit(run(args.db, args.token, args.session_id, args.run_dir))


if __name__ == '__main__':
    main()
