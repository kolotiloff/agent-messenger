"""Two guarded real turns in an EXISTING, idle TEST session of Codex or Cursor.

Explicit opt-in, existing local login, new private run directory. POSIX prototype.
Never use a working conversation or an ID currently open in another CLI/GUI.
"""
import argparse
import json
import os
from pathlib import Path
import sys
import uuid
from claude_probe import SERVER, run_turn
from runtime import Store, session_key

PROMPT = ('Authorized local Messenger test. Use ONLY messenger_bound MCP tools. '
          'First call read_current. Then call reply with text made by concatenating '
          'the literal "ACK:" and the exact received text, with NO space or newline '
          'between them. Example: "abc" becomes "ACK:abc". Do not select an address '
          'or message ID. After storage is confirmed output DONE and stop. '
          'Do not poll, execute shell commands or access files.')


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


class Codex:
    provider = 'codex'

    def command(self, cli, db, token, root, sid):
        argv = [str(cli), 'exec', '--ignore-user-config', '--sandbox', 'read-only',
                '--skip-git-repo-check', '--json', '-c', 'web_search="disabled"',
                '-c', 'model_reasoning_effort="low"']
        for feature in ['shell_tool', 'apps', 'plugins', 'hooks', 'multi_agent', 'browser_use', 'computer_use']:
            argv += ['--disable', feature]
        for key, value in {'command': sys.executable, 'args': [str(SERVER), '--db', str(db), '--token', token],
                           'required': True, 'enabled_tools': ['read_current', 'reply'],
                           'default_tools_approval_mode': 'approve'}.items():
            argv += ['-c', 'mcp_servers.messenger_bound.'+key+'='+json.dumps(value)]
        return argv+['resume', sid, PROMPT]

    def startup_id(self, event):
        if event.get('type') == 'thread.started':
            require(isinstance(event.get('thread_id'), str), 'Missing Codex ID')
            return event['thread_id']

    def validate(self, events, sid):
        require([e.get('thread_id') for e in events if e.get('type') == 'thread.started'] == [sid], 'Codex ID mismatch')
        require(sum(e.get('type') == 'turn.completed' for e in events) == 1, 'Missing/duplicate completion')
        require(not any(e.get('type') in ('error', 'turn.failed') for e in events), 'Codex error event')
        calls = []
        for event in events:
            if event.get('type') != 'item.completed':
                continue
            item = event.get('item', {})
            kind = item.get('type')
            if kind == 'mcp_tool_call':
                require(item.get('server') == 'messenger_bound' and item.get('status') == 'completed'
                        and not item.get('error'), 'Failed or foreign MCP call')
                result = item.get('result') or {}
                require(result and not result.get('isError') and not result.get('is_error'), 'MCP returned error')
                calls.append(item.get('tool'))
            else:
                require(kind in ('agent_message', 'reasoning', 'plan_update'), 'Unexpected Codex tool/action')
        require(calls == ['read_current', 'reply'], 'Codex tool sequence mismatch')


class Cursor:
    provider = 'cursor'

    def command(self, cli, db, token, root, sid):
        # --approve-mcps is used only in a controlled workspace with no global MCP.
        require(not (Path.home()/'.cursor/mcp.json').exists(), 'Global Cursor MCP config exists; inspect before test')
        require(not os.environ.get('CURSOR_CONFIG_DIR'), 'Custom Cursor config directory needs inspection')
        directory = root/'workspace/.cursor'
        directory.mkdir(exist_ok=True)
        directory.joinpath('mcp.json').write_text(json.dumps({'mcpServers': {'messenger_bound': {
            'command': sys.executable, 'args': [str(SERVER), '--db', str(db), '--token', token]}}}))
        directory.joinpath('cli.json').write_text(json.dumps({'permissions': {
            'allow': ['Mcp(messenger_bound:read_current)', 'Mcp(messenger_bound:reply)'],
            'deny': ['Shell(*)', 'Read(*)', 'Write(*)', 'WebFetch(*)']}}))
        return [str(cli), '-p', '--resume='+sid, '--output-format', 'stream-json',
                '--sandbox', 'enabled', '--trust', '--approve-mcps',
                '--workspace', str(root/'workspace'), PROMPT]

    def startup_id(self, event):
        if event.get('type') == 'system' and event.get('subtype') == 'init':
            require(isinstance(event.get('session_id'), str), 'Missing Cursor ID')
            return event['session_id']

    def validate(self, events, sid):
        starts = [self.startup_id(e) for e in events if e.get('type') == 'system' and e.get('subtype') == 'init']
        require(starts == [sid], 'Cursor ID mismatch')
        require(all(e.get('session_id', sid) == sid for e in events), 'Cursor event changed session')
        results = [e for e in events if e.get('type') == 'result']
        require(len(results) == 1 and results[0].get('subtype') == 'success'
                and results[0].get('is_error') is False, 'Cursor result failed')
        require(not any(e.get('type') == 'error' for e in events), 'Cursor error event')
        calls = []
        for event in events:
            if event.get('type') != 'tool_call' or event.get('subtype') != 'completed':
                continue
            call = event.get('tool_call', {})
            if 'getMcpToolsToolCall' in call:
                continue
            require('mcpToolCall' in call, 'Unexpected Cursor tool/action')
            mcp = call['mcpToolCall']
            args = mcp.get('args', {})
            success = mcp.get('result', {}).get('success')
            require(args.get('providerIdentifier') == 'messenger_bound', 'Foreign Cursor MCP')
            require(isinstance(success, dict) and not success.get('isError'), 'Cursor MCP error')
            calls.append(args.get('toolName'))
        require(calls == ['read_current', 'reply'], 'Cursor tool sequence mismatch')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--provider', choices=['codex', 'cursor'], required=True)
    parser.add_argument('--cli', type=Path, required=True)
    parser.add_argument('--session-id', required=True)
    parser.add_argument('--run-dir', type=Path, required=True)
    parser.add_argument('--allow-model-calls', action='store_true')
    args = parser.parse_args()
    if not args.allow_model_calls or os.name != 'posix':
        parser.error('Requires POSIX and explicit --allow-model-calls')
    uuid.UUID(args.session_id)
    root = args.run_dir.resolve()
    root.mkdir(parents=True, mode=0o700, exist_ok=False)
    root.joinpath('workspace').mkdir()
    cli = args.cli.resolve(strict=True)
    adapter = Codex() if args.provider == 'codex' else Cursor()
    key = session_key(args.provider, 'local-default-test', args.session_id)
    db = root/'queue.sqlite'
    store = Store(db)
    try:
        store.register(key, args.provider+'-test')
        for i in (1, 2):
            store.enqueue('message-'+str(i), key, 'test-coordinator', 'PAYLOAD-'+uuid.uuid4().hex)
        root.joinpath('private-binding.json').write_text(json.dumps({'provider': args.provider, 'session_id': args.session_id}))
        turns = []
        for index in (1, 2):
            if index == 2:
                store.close()
                store = Store(db)
            turns.append(run_turn(store, db, key, cli, root, args.session_id, index, adapter=adapter))
            print(json.dumps(turns[-1]), flush=True)
        snapshot = store.snapshot()
        require([r['state'] for r in snapshot['requests']] == ['completed', 'completed'], 'Incomplete queue')
        require(len(snapshot['attempts']) == 2 and len(snapshot['replies']) == 2, 'Unexpected row count')
        require(snapshot['sessions'][0]['active'] is None, 'Session still active')
        summary = {'provider': args.provider, 'passed': True, 'same_session': True,
                   'real_cli_turns': 2, 'linked_replies': 2, 'automatic_retries': 0, 'turns': turns}
        root.joinpath('summary.json').write_text(json.dumps(summary, indent=2))
        print(json.dumps(summary), flush=True)
    finally:
        store.close()


if __name__ == '__main__':
    main()
