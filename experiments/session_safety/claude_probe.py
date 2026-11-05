"""Explicit opt-in integration test: TWO real Claude turns; macOS/POSIX only.

Uses existing local Claude authorization. Never retries an ambiguous attempt.
Run --help for required arguments. Raw logs and state must stay outside Git.
"""
import argparse
import json
import os
from pathlib import Path
import selectors
import signal
import subprocess
import sys
import time
import uuid
from runtime import Store, session_key


SERVER = Path(__file__).with_name('mcp_server.py').resolve()
EXPECTED_TOOLS = {'mcp__messenger_bound__read_current', 'mcp__messenger_bound__reply'}


def command(cli, db, token, conversation_id, resume):
    config = {'mcpServers': {'messenger_bound': {
        'command': sys.executable, 'args': [str(SERVER), '--db', str(db), '--token', token]}}}
    return [str(cli), '-p', '--output-format', 'stream-json', '--verbose',
            '--tools', '', '--strict-mcp-config', '--mcp-config', json.dumps(config),
            '--setting-sources', '', '--settings', '{"disableAllHooks":true}',
            '--disable-slash-commands', '--no-chrome', '--permission-mode', 'dontAsk',
            '--allowedTools', ','.join(sorted(EXPECTED_TOOLS)),
            '--system-prompt', 'You participate in an authorized local messaging test. '
            'Use only the supplied MCP tools and stop after the requested response.',
            '--resume' if resume else '--session-id', conversation_id,
            'Read the assigned message using messenger_bound read_current. '
            'Call messenger_bound reply with text formed by concatenating the literal "ACK:" '
            'and the exact received text. Add NO space, newline or other character between them. '
            'For example, received "abc" must produce exactly "ACK:abc". '
            'Do not choose an address or message ID. After storage is confirmed output DONE and stop.']


def run_turn(store, db, key, cli, root, conversation_id, index, adapter=None):
    provider = adapter.provider if adapter else 'claude'
    token = store.claim(key)
    if token is None:
        raise RuntimeError('Session unavailable; no CLI launched')
    store.begin_launch(token, require_identity=True)
    process = None
    selector = selectors.DefaultSelector()
    events = []
    identity_verified = False
    start = time.monotonic()
    try:
        with (root/('turn-'+str(index)+'.jsonl')).open('wb') as raw, \
                (root/('turn-'+str(index)+'.stderr')).open('wb') as err:
            argv = (adapter.command(cli, db, token, root, conversation_id) if adapter
                    else command(cli, db, token, conversation_id, index > 1))
            process = subprocess.Popen(argv,
                cwd=root/'workspace', stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                stderr=err, start_new_session=True)
            store.record_pid(token, process.pid)
            # A separate process competes for the same durable slot while CLI is alive.
            if process.poll() is not None:
                raise RuntimeError('CLI exited before concurrency check')
            probe = subprocess.run([sys.executable, '-c',
                'import sys; from runtime import Store; s=Store(sys.argv[1]); '
                'result=s.claim(sys.argv[2]); s.close(); print("BLOCKED" if result is None else "BAD")',
                str(db), key], cwd=SERVER.parent, text=True, capture_output=True, timeout=10, check=True)
            if probe.stdout.strip() != 'BLOCKED' or process.poll() is not None:
                raise RuntimeError('Concurrent claim was not verified against a live CLI')
            selector.register(process.stdout, selectors.EVENT_READ)
            pending = b''
            while selector.get_map():
                if time.monotonic()-start > 120:
                    raise TimeoutError('CLI exceeded test deadline')
                for selected, _ in selector.select(timeout=0.1):
                    chunk = os.read(selected.fileobj.fileno(), 65536)
                    if not chunk:
                        selector.unregister(selected.fileobj)
                        break
                    raw.write(chunk)
                    raw.flush()
                    pending += chunk
                    while b'\n' in pending:
                        line, pending = pending.split(b'\n', 1)
                        if not line.strip():
                            continue
                        event = json.loads(line)
                        events.append(event)
                        sid = (adapter.startup_id(event) if adapter else
                               event.get('session_id') if event.get('type') == 'system'
                               and event.get('subtype') == 'init' else None)
                        if sid is not None:
                            actual = session_key(provider, 'local-default-test', sid)
                            if not adapter and set(event.get('tools', [])) != EXPECTED_TOOLS:
                                raise RuntimeError('Unexpected available tools')
                            store.confirm_identity(token, actual)
                            identity_verified = True
            if pending.strip():
                raise RuntimeError('Unterminated output event')
            code = process.wait(timeout=max(1, 120-(time.monotonic()-start)))
        if code != 0 or not identity_verified:
            raise RuntimeError('CLI did not exit successfully with verified identity')
        if adapter:
            adapter.validate(events, conversation_id)
        else:
            validate_claude(events, conversation_id)
        snapshot = store.snapshot()
        request = next(r for r in snapshot['requests'] if r['id'] == 'message-'+str(index))
        replies = [r for r in snapshot['replies'] if r['reply_to'] == request['id']]
        if len(replies) != 1 or replies[0]['text'] != 'ACK:'+request['text']:
            raise RuntimeError('Reply missing, unlinked or incorrect')
        if (replies[0]['sender'], replies[0]['recipient']) != (provider+'-test', 'test-coordinator'):
            raise RuntimeError('Reply route mismatch')
        if store.finish_after_wait(token, code) != 'completed':
            raise RuntimeError('Completion was not committed')
        return {'turn': index, 'passed': True, 'identity_checked_before_mcp': True,
                'concurrent_claim_blocked': True, 'seconds': round(time.monotonic()-start, 2)}
    except BaseException:
        # Freeze first. A killed child, lost stdout or validation failure is not retried.
        store.quarantine(key)
        if process is not None:
            try:
                os.killpg(process.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait(timeout=5)
        raise
    finally:
        selector.close()
        if process is not None and process.stdout:
            process.stdout.close()


def validate_claude(events, conversation_id):
    results = [e for e in events if e.get('type') == 'result']
    if len(results) != 1:
        raise RuntimeError('CLI did not finish with one verified result')
    result = results[0]
    if (result.get('session_id') != conversation_id or result.get('subtype') != 'success'
            or result.get('is_error') or result.get('permission_denials')):
        raise RuntimeError('CLI result or session identity mismatch')
    calls = [b for e in events if e.get('type') == 'assistant'
             for b in e.get('message', {}).get('content', []) if b.get('type') == 'tool_use']
    if [b['name'] for b in calls] != ['mcp__messenger_bound__read_current', 'mcp__messenger_bound__reply']:
        raise RuntimeError('Unexpected tool sequence')

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--claude', type=Path, required=True)
    parser.add_argument('--run-dir', type=Path, required=True, help='NEW private directory outside Git')
    parser.add_argument('--allow-model-calls', action='store_true', help='Explicitly authorize two real CLI turns')
    args = parser.parse_args()
    if not args.allow_model_calls:
        parser.error('This test spends tokens; pass --allow-model-calls explicitly')
    if os.name != 'posix':
        parser.error('This process-observation prototype supports POSIX only')
    cli = args.claude.resolve(strict=True)
    root = args.run_dir.resolve()
    root.mkdir(mode=0o700, parents=True, exist_ok=False)
    root.joinpath('workspace').mkdir()
    sid = str(uuid.uuid4())
    key = session_key('claude', 'local-default-test', sid)
    db = root/'queue.sqlite'
    store = Store(db)
    try:
        store.register(key, 'claude-test')
        for index in (1, 2):
            store.enqueue('message-'+str(index), key, 'test-coordinator', 'PAYLOAD-'+uuid.uuid4().hex)
        root.joinpath('private-binding.json').write_text(json.dumps({'session_id': sid, 'session_key': key}))
        turns = []
        for index in (1, 2):
            # Close/reopen the database between turns to avoid an in-memory queue assumption.
            if index == 2:
                store.close()
                store = Store(db)
            turns.append(run_turn(store, db, key, cli, root, sid, index))
            print(json.dumps(turns[-1]), flush=True)
        snapshot = store.snapshot()
        if [r['state'] for r in snapshot['requests']] != ['completed', 'completed']:
            raise RuntimeError('Queue did not complete exactly two requests')
        if len(snapshot['attempts']) != 2 or len(snapshot['replies']) != 2:
            raise RuntimeError('Unexpected number of executions or replies')
        if snapshot['sessions'][0]['active'] is not None:
            raise RuntimeError('Successful session slot still active')
        summary = {'passed': True, 'real_cli_turns': 2, 'same_session': True,
                   'linked_replies': 2, 'turns': turns, 'automatic_retries': 0}
        root.joinpath('summary.json').write_text(json.dumps(summary, indent=2))
        print(json.dumps(summary), flush=True)
    finally:
        store.close()


if __name__ == '__main__':
    main()
