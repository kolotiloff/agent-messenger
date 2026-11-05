"""Fail-closed resume commands and result checks for Claude, Codex, and Cursor.

Building a command does not launch it. The default test suite must not start
these CLIs. There is no --last, --continue, or "latest session" form.
"""
from __future__ import annotations

import json
import sys

from agent_messenger.errors import AdapterError
from agent_messenger.identity import REAL_PROVIDERS, check_conversation_id

EXPECTED_CLAUDE_TOOLS = (
    'mcp__messenger_bound__read_current',
    'mcp__messenger_bound__reply',
)
FORBIDDEN_ARGUMENTS = (
    '--last',
    '--continue',
    '--last-session',
    '--continue-session',
    '--fork-session',
)

CLAUDE_PROMPT = (
    'Read the assigned message using messenger_bound read_current. '
    'Call messenger_bound reply with text formed by concatenating the literal "ACK:" '
    'and the exact received text. Add NO space, newline or other character between them. '
    'For example, received "abc" must produce exactly "ACK:abc". '
    'Do not choose an address or message ID. After storage is confirmed output DONE and stop.'
)

CODEX_CURSOR_PROMPT = (
    'Authorized local Messenger test. Use ONLY messenger_bound MCP tools. '
    'First call read_current. Then call reply with text made by concatenating '
    'the literal "ACK:" and the exact received text, with NO space or newline '
    'between them. Example: "abc" becomes "ACK:abc". Do not select an address '
    'or message ID. After storage is confirmed output DONE and stop. '
    'Do not poll, execute shell commands or access files.'
)


def _require(condition, message):
    if not condition:
        raise AdapterError(message)


def reject_forbidden_argv(argv):
    for arg in argv:
        if not isinstance(arg, str):
            raise AdapterError('Command arguments must be strings')
        lowered = arg.lower()
        for item in FORBIDDEN_ARGUMENTS:
            if lowered == item or lowered.startswith(item + '='):
                raise AdapterError('Refusing launch flag ' + arg)
        if lowered in {'last', 'latest', 'continue'}:
            raise AdapterError('Refusing session selector ' + arg)
    return argv


def _mcp_invocation(db, token):
    return [sys.executable, '-m', 'agent_messenger.mcp_server', '--db', str(db), '--token', str(token)]


def cursor_preflight(home, environ):
    if (home / '.cursor' / 'mcp.json').exists():
        raise AdapterError('Global Cursor MCP config exists; inspect before launch')
    if environ.get('CURSOR_CONFIG_DIR'):
        raise AdapterError('Custom Cursor config directory needs inspection')


def _prepare_cursor_workspace(workspace, db, token):
    directory = workspace / '.cursor'
    directory.mkdir(parents=True, exist_ok=True)
    command = _mcp_invocation(db, token)
    directory.joinpath('mcp.json').write_text(json.dumps({'mcpServers': {'messenger_bound': {
        'command': command[0],
        'args': command[1:],
    }}}), encoding='utf-8')
    directory.joinpath('cli.json').write_text(json.dumps({'permissions': {
        'allow': ['Mcp(messenger_bound:read_current)', 'Mcp(messenger_bound:reply)'],
        'deny': ['Shell(*)', 'Read(*)', 'Write(*)', 'WebFetch(*)'],
    }}), encoding='utf-8')


def build_argv(provider, *, cli, db, token, workspace, conversation_id, home, environ):
    """Argv for an already-bound session. Does not create a conversation or spawn."""
    if provider not in REAL_PROVIDERS:
        raise AdapterError('No real CLI adapter for this provider')
    conversation_id = check_conversation_id(conversation_id)
    command = _mcp_invocation(db, token)
    if provider == 'claude':
        config = {'mcpServers': {'messenger_bound': {
            'command': command[0], 'args': command[1:]}}}
        argv = [str(cli), '-p', '--output-format', 'stream-json', '--verbose',
                '--tools', '', '--strict-mcp-config', '--mcp-config', json.dumps(config),
                '--setting-sources', '', '--settings', '{"disableAllHooks":true}',
                '--disable-slash-commands', '--no-chrome', '--permission-mode', 'dontAsk',
                '--allowedTools', ','.join(EXPECTED_CLAUDE_TOOLS),
                '--system-prompt',
                'You participate in an authorized local messaging test. '
                'Use only the supplied MCP tools and stop after the requested response.',
                '--resume', conversation_id, CLAUDE_PROMPT]
    elif provider == 'codex':
        argv = [str(cli), 'exec', '--ignore-user-config', '--sandbox', 'read-only',
                '--skip-git-repo-check', '--json', '-c', 'web_search="disabled"',
                '-c', 'model_reasoning_effort="low"']
        for feature in ('shell_tool', 'apps', 'plugins', 'hooks', 'multi_agent',
                        'browser_use', 'computer_use'):
            argv += ['--disable', feature]
        for key, value in (
            ('command', command[0]),
            ('args', command[1:]),
            ('required', True),
            ('enabled_tools', ['read_current', 'reply']),
            ('default_tools_approval_mode', 'approve'),
        ):
            argv += ['-c', 'mcp_servers.messenger_bound.' + key + '=' + json.dumps(value)]
        argv += ['resume', conversation_id, CODEX_CURSOR_PROMPT]
    elif provider == 'cursor':
        cursor_preflight(home, environ)
        _prepare_cursor_workspace(workspace, db, token)
        argv = [str(cli), '-p', '--resume=' + conversation_id, '--output-format', 'stream-json',
                '--sandbox', 'enabled', '--trust', '--approve-mcps',
                '--workspace', str(workspace), CODEX_CURSOR_PROMPT]
    else:
        raise AdapterError('No real CLI adapter for this provider')
    _require(_binds_exact_id(provider, argv, conversation_id),
             'Launch command lost the exact conversation id')
    if '--session-id' in argv:
        raise AdapterError('Launch command must resume the bound id and must not create a session')
    return reject_forbidden_argv(argv)


def _binds_exact_id(provider, argv, conversation_id):
    if provider == 'claude' and '--resume' in argv:
        return argv[argv.index('--resume') + 1] == conversation_id
    if provider == 'codex' and 'resume' in argv:
        return argv[argv.index('resume') + 1] == conversation_id
    if provider == 'cursor':
        return '--resume=' + conversation_id in argv
    return False


def observe_startup(provider, event):
    """Return a session id when ``event`` is that provider's startup event.

    Other events return None. A startup event without an id fails closed.
    """
    if not isinstance(event, dict):
        raise AdapterError('CLI event is not an object')
    if provider == 'synthetic':
        if event.get('type') == 'system' and event.get('subtype') == 'init':
            _require(isinstance(event.get('session_id'), str) and event.get('session_id'),
                     'Missing synthetic session id')
            return event['session_id']
        return None
    if provider == 'claude':
        if event.get('type') == 'system' and event.get('subtype') == 'init':
            _require(isinstance(event.get('session_id'), str) and event.get('session_id'),
                     'Missing Claude session id')
            _require(set(event.get('tools', [])) == set(EXPECTED_CLAUDE_TOOLS),
                     'Unexpected available tools')
            return event['session_id']
        return None
    if provider == 'codex':
        if event.get('type') == 'thread.started':
            _require(isinstance(event.get('thread_id'), str) and event.get('thread_id'),
                     'Missing Codex ID')
            return event['thread_id']
        return None
    if provider == 'cursor':
        if event.get('type') == 'system' and event.get('subtype') == 'init':
            _require(isinstance(event.get('session_id'), str) and event.get('session_id'),
                     'Missing Cursor ID')
            return event['session_id']
        return None
    raise AdapterError('No adapter for this provider')


def validate_result(provider, events, conversation_id):
    conversation_id = check_conversation_id(conversation_id)
    if provider == 'claude':
        _validate_claude(events, conversation_id)
    elif provider == 'codex':
        _validate_codex(events, conversation_id)
    elif provider == 'cursor':
        _validate_cursor(events, conversation_id)
    else:
        raise AdapterError('No result validator for this provider')


def _validate_claude(events, conversation_id):
    results = [event for event in events if event.get('type') == 'result']
    _require(len(results) == 1, 'CLI did not finish with one verified result')
    result = results[0]
    _require(result.get('session_id') == conversation_id and result.get('subtype') == 'success'
             and not result.get('is_error') and not result.get('permission_denials'),
             'CLI result or session identity mismatch')
    calls = [block for event in events if event.get('type') == 'assistant'
             for block in event.get('message', {}).get('content', [])
             if block.get('type') == 'tool_use']
    _require([block.get('name') for block in calls] == list(EXPECTED_CLAUDE_TOOLS),
             'Unexpected tool sequence')


def _validate_codex(events, conversation_id):
    _require([event.get('thread_id') for event in events if event.get('type') == 'thread.started']
             == [conversation_id], 'Codex ID mismatch')
    _require(sum(event.get('type') == 'turn.completed' for event in events) == 1,
             'Missing/duplicate completion')
    _require(not any(event.get('type') in ('error', 'turn.failed') for event in events),
             'Codex error event')
    calls = []
    for event in events:
        if event.get('type') != 'item.completed':
            continue
        item = event.get('item', {})
        kind = item.get('type')
        if kind == 'mcp_tool_call':
            _require(item.get('server') == 'messenger_bound' and item.get('status') == 'completed'
                     and not item.get('error'), 'Failed or foreign MCP call')
            result = item.get('result') or {}
            _require(result and not result.get('isError') and not result.get('is_error'),
                     'MCP returned error')
            calls.append(item.get('tool'))
        else:
            _require(kind in ('agent_message', 'reasoning', 'plan_update'),
                     'Unexpected Codex tool/action')
    _require(calls == ['read_current', 'reply'], 'Codex tool sequence mismatch')


def _validate_cursor(events, conversation_id):
    starts = [observe_startup('cursor', event) for event in events
              if event.get('type') == 'system' and event.get('subtype') == 'init']
    _require(starts == [conversation_id], 'Cursor ID mismatch')
    _require(all(event.get('session_id', conversation_id) == conversation_id for event in events),
             'Cursor event changed session')
    results = [event for event in events if event.get('type') == 'result']
    _require(len(results) == 1 and results[0].get('subtype') == 'success'
             and results[0].get('is_error') is False, 'Cursor result failed')
    _require(not any(event.get('type') == 'error' for event in events), 'Cursor error event')
    calls = []
    for event in events:
        if event.get('type') != 'tool_call' or event.get('subtype') != 'completed':
            continue
        call = event.get('tool_call', {})
        if 'getMcpToolsToolCall' in call:
            continue
        _require('mcpToolCall' in call, 'Unexpected Cursor tool/action')
        mcp = call['mcpToolCall']
        args = mcp.get('args', {})
        success = mcp.get('result', {}).get('success')
        _require(args.get('providerIdentifier') == 'messenger_bound', 'Foreign Cursor MCP')
        _require(isinstance(success, dict) and not success.get('isError'), 'Cursor MCP error')
        calls.append(args.get('toolName'))
    _require(calls == ['read_current', 'reply'], 'Cursor tool sequence mismatch')
