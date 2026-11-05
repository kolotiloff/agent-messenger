"""Adapter checks copied from the opt-in probes. They never launch a CLI."""
import copy
from pathlib import Path
import tempfile
import unittest
import sys

SRC = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(SRC))

from agent_messenger.adapters import (
    build_argv,
    observe_startup,
    reject_forbidden_argv,
    validate_result,
)
from agent_messenger.errors import AdapterError


class AdapterTests(unittest.TestCase):
    def test_commands_resume_exact_id_and_refuse_selectors(self):
        with tempfile.TemporaryDirectory(prefix='am-argv-') as temporary:
            root = Path(temporary)
            workspace = root / 'workspace'
            workspace.mkdir()
            home = root / 'home'
            home.mkdir()
            for provider in ('claude', 'codex', 'cursor'):
                argv = build_argv(
                    provider, cli='/tmp/messenger-test-cli', db=root / 'queue.sqlite',
                    token='token-test', workspace=workspace, conversation_id='test-id',
                    home=home, environ={})
                self.assertNotIn('--last', argv)
                self.assertNotIn('--continue', argv)
                self.assertNotIn('--session-id', argv)
                self.assertFalse(any(arg.startswith('--last') or arg.startswith('--continue') for arg in argv))
                if provider == 'cursor':
                    self.assertIn('--resume=test-id', argv)
                elif provider == 'codex':
                    self.assertEqual(argv[argv.index('resume') + 1], 'test-id')
                else:
                    self.assertEqual(argv[argv.index('--resume') + 1], 'test-id')
            with self.assertRaises(AdapterError):
                reject_forbidden_argv(['claude', '--last'])
            with self.assertRaises(AdapterError):
                reject_forbidden_argv(['claude', '--continue=abc'])
            occupied = home / '.cursor'
            occupied.mkdir()
            occupied.joinpath('mcp.json').write_text('{}', encoding='utf-8')
            with self.assertRaises(AdapterError):
                build_argv(
                    'cursor', cli='/tmp/messenger-test-cli', db=root / 'queue.sqlite',
                    token='token-test', workspace=workspace, conversation_id='test-id',
                    home=home, environ={})
            with self.assertRaises(AdapterError):
                build_argv(
                    'cursor', cli='/tmp/messenger-test-cli', db=root / 'queue.sqlite',
                    token='token-test', workspace=workspace, conversation_id='test-id',
                    home=root / 'empty-home', environ={'CURSOR_CONFIG_DIR': '/tmp/custom'})

    def test_codex_rejects_failed_foreign_and_unexpected_actions(self):
        events = [{'type': 'thread.started', 'thread_id': 'test-id'}]
        for name in ('read_current', 'reply'):
            events.append({'type': 'item.completed', 'item': {
                'type': 'mcp_tool_call', 'server': 'messenger_bound', 'tool': name,
                'status': 'completed', 'result': {'content': []}, 'error': None}})
        events.append({'type': 'turn.completed'})
        validate_result('codex', events, 'test-id')
        mutations = []
        bad = copy.deepcopy(events)
        bad[0]['thread_id'] = 'wrong'
        mutations.append(bad)
        bad = copy.deepcopy(events)
        bad[1]['item']['error'] = 'denied'
        mutations.append(bad)
        bad = copy.deepcopy(events)
        bad[1]['item']['server'] = 'foreign'
        mutations.append(bad)
        bad = copy.deepcopy(events)
        bad[1]['item'] = {'type': 'command_execution'}
        mutations.append(bad)
        mutations.extend([events + [{'type': 'turn.failed'}], events + [{'type': 'turn.completed'}], events[:-1]])
        for bad in mutations:
            with self.subTest(events=bad), self.assertRaises(AdapterError):
                validate_result('codex', bad, 'test-id')

    def test_cursor_rejects_failed_foreign_and_unexpected_actions(self):
        events = [{'type': 'system', 'subtype': 'init', 'session_id': 'test-id'}]
        events.append({'type': 'tool_call', 'subtype': 'completed',
                       'tool_call': {'getMcpToolsToolCall': {}}})
        for name in ('read_current', 'reply'):
            events.append({'type': 'tool_call', 'subtype': 'completed', 'tool_call': {'mcpToolCall': {
                'args': {'providerIdentifier': 'messenger_bound', 'toolName': name},
                'result': {'success': {'isError': False}}}}})
        events.append({'type': 'result', 'subtype': 'success', 'is_error': False, 'session_id': 'test-id'})
        validate_result('cursor', events, 'test-id')
        mutations = []
        bad = copy.deepcopy(events)
        bad[-1]['session_id'] = 'wrong'
        mutations.append(bad)
        bad = copy.deepcopy(events)
        bad[-1]['is_error'] = True
        mutations.append(bad)
        bad = copy.deepcopy(events)
        bad[2]['tool_call']['mcpToolCall']['result'] = {'error': 'denied'}
        mutations.append(bad)
        bad = copy.deepcopy(events)
        bad[2]['tool_call'] = {'shellToolCall': {}}
        mutations.append(bad)
        bad = copy.deepcopy(events)
        bad[2]['tool_call']['mcpToolCall']['args']['providerIdentifier'] = 'foreign'
        mutations.append(bad)
        mutations.extend([events + [events[-1]], events[:-1]])
        for bad in mutations:
            with self.subTest(events=bad), self.assertRaises(AdapterError):
                validate_result('cursor', bad, 'test-id')

    def test_claude_completion_validation_preserved(self):
        events = [{'type': 'assistant', 'message': {'content': [
            {'type': 'tool_use', 'name': 'mcp__messenger_bound__read_current'},
            {'type': 'tool_use', 'name': 'mcp__messenger_bound__reply'}]}},
            {'type': 'result', 'subtype': 'success', 'is_error': False, 'session_id': 'test-id'}]
        validate_result('claude', events, 'test-id')
        bad = copy.deepcopy(events)
        bad[-1]['permission_denials'] = ['denied']
        with self.assertRaises(AdapterError):
            validate_result('claude', bad, 'test-id')
        with self.assertRaises(AdapterError):
            validate_result('claude', events, 'wrong-id')

    def test_startup_missing_ids_fail_closed(self):
        with self.assertRaises(AdapterError):
            observe_startup('codex', {'type': 'thread.started'})
        with self.assertRaises(AdapterError):
            observe_startup('cursor', {'type': 'system', 'subtype': 'init'})
        with self.assertRaises(AdapterError):
            observe_startup('claude', {'type': 'system', 'subtype': 'init', 'session_id': 'test-id',
                                       'tools': ['Bash']})
        self.assertIsNone(observe_startup('codex', {'type': 'other'}))


if __name__ == '__main__':
    unittest.main()
