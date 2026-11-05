"""No-model checks of untrusted CLI event parsing and completion boundaries."""
import copy
import unittest
from claude_probe import validate_claude
from resume_probe import Codex, Cursor


class AdapterTests(unittest.TestCase):
    def test_codex_rejects_failed_foreign_and_unexpected_actions(self):
        events = [{'type': 'thread.started', 'thread_id': 'test-id'}]
        for name in ('read_current', 'reply'):
            events.append({'type': 'item.completed', 'item': {'type': 'mcp_tool_call',
                'server': 'messenger_bound', 'tool': name, 'status': 'completed',
                'result': {'content': []}, 'error': None}})
        events.append({'type': 'turn.completed'})
        adapter = Codex()
        adapter.validate(events, 'test-id')
        mutations = []
        bad = copy.deepcopy(events); bad[0]['thread_id'] = 'wrong'; mutations.append(bad)
        bad = copy.deepcopy(events); bad[1]['item']['error'] = 'denied'; mutations.append(bad)
        bad = copy.deepcopy(events); bad[1]['item']['server'] = 'foreign'; mutations.append(bad)
        bad = copy.deepcopy(events); bad[1]['item'] = {'type': 'command_execution'}; mutations.append(bad)
        mutations.extend([events+[{'type': 'turn.failed'}], events+[{'type': 'turn.completed'}], events[:-1]])
        for bad in mutations:
            with self.subTest(events=bad), self.assertRaises(RuntimeError):
                adapter.validate(bad, 'test-id')

    def test_cursor_rejects_failed_foreign_and_unexpected_actions(self):
        events = [{'type': 'system', 'subtype': 'init', 'session_id': 'test-id'}]
        events.append({'type': 'tool_call', 'subtype': 'completed', 'tool_call': {'getMcpToolsToolCall': {}}})
        for name in ('read_current', 'reply'):
            events.append({'type': 'tool_call', 'subtype': 'completed', 'tool_call': {'mcpToolCall': {
                'args': {'providerIdentifier': 'messenger_bound', 'toolName': name},
                'result': {'success': {'isError': False}}}}})
        events.append({'type': 'result', 'subtype': 'success', 'is_error': False, 'session_id': 'test-id'})
        adapter = Cursor()
        adapter.validate(events, 'test-id')
        mutations = []
        bad = copy.deepcopy(events); bad[-1]['session_id'] = 'wrong'; mutations.append(bad)
        bad = copy.deepcopy(events); bad[-1]['is_error'] = True; mutations.append(bad)
        bad = copy.deepcopy(events); bad[2]['tool_call']['mcpToolCall']['result'] = {'error': 'denied'}; mutations.append(bad)
        bad = copy.deepcopy(events); bad[2]['tool_call'] = {'shellToolCall': {}}; mutations.append(bad)
        bad = copy.deepcopy(events); bad[2]['tool_call']['mcpToolCall']['args']['providerIdentifier'] = 'foreign'; mutations.append(bad)
        mutations.extend([events+[events[-1]], events[:-1]])
        for bad in mutations:
            with self.subTest(events=bad), self.assertRaises(RuntimeError):
                adapter.validate(bad, 'test-id')

    def test_claude_completion_validation_preserved(self):
        events = [{'type': 'assistant', 'message': {'content': [
            {'type': 'tool_use', 'name': 'mcp__messenger_bound__read_current'},
            {'type': 'tool_use', 'name': 'mcp__messenger_bound__reply'}]}},
            {'type': 'result', 'subtype': 'success', 'is_error': False, 'session_id': 'test-id'}]
        validate_claude(events, 'test-id')
        bad = copy.deepcopy(events); bad[-1]['permission_denials'] = ['denied']
        with self.assertRaises(RuntimeError):
            validate_claude(bad, 'test-id')
        with self.assertRaises(RuntimeError):
            validate_claude(events, 'wrong-id')

    def test_startup_missing_ids_fail_closed(self):
        with self.assertRaises(RuntimeError):
            Codex().startup_id({'type': 'thread.started'})
        with self.assertRaises(RuntimeError):
            Cursor().startup_id({'type': 'system', 'subtype': 'init'})


if __name__ == '__main__':
    unittest.main()
