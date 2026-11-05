"""State-machine tests for the durable store. No supervisor and no model."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import unittest

SRC = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(SRC))

from agent_messenger.errors import RecoveryBlocked, Rejected
from agent_messenger.identity import canonical_directory, session_key
from agent_messenger.mcp_server import invoke
from agent_messenger.process_info import (
    capture_process,
    process_exit_proven,
    signal_process_group,
)
from agent_messenger.store import Store


def rpc(db, token, calls):
    requests = [{'jsonrpc': '2.0', 'id': 0, 'method': 'initialize', 'params': {
        'protocolVersion': '2025-06-18', 'capabilities': {},
        'clientInfo': {'name': 'synthetic-test', 'version': '1'}}}]
    requests += [{'jsonrpc': '2.0', 'id': index + 1, 'method': method, 'params': params}
                 for index, (method, params) in enumerate(calls)]
    env = os.environ.copy()
    env['PYTHONPATH'] = str(SRC)
    completed = subprocess.run(
        [sys.executable, '-m', 'agent_messenger.mcp_server', '--db', str(db), '--token', token],
        input=''.join(json.dumps(item) + '\n' for item in requests),
        capture_output=True, text=True, timeout=10, check=True, env=env)
    return [json.loads(line)['result'] for line in completed.stdout.splitlines()]


def tool(name, args):
    return ('tools/call', {'name': name, 'arguments': args})


class StoreTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='am-store-')
        self.root = Path(self.temp.name)
        self.db = self.root / 'queue.sqlite'
        self.store = Store(self.db)
        self.env = self.root / 'env'
        self.env.mkdir()
        self.workspace = self.root / 'workspace'
        self.workspace.mkdir()
        registered = self.store.register_session(
            provider='synthetic', environment=str(self.env), conversation_id='conversation-1',
            alias='worker', workspace=str(self.workspace))
        self.key = registered['session_key']

    def tearDown(self):
        try:
            self.store.close()
        except Exception:
            pass
        self.temp.cleanup()

    def enqueue(self, message_id='m1', alias='worker', text=None):
        return self.store.enqueue(message_id, alias, 'coordinator', text or ('request ' + message_id))

    def running(self):
        self.enqueue()
        token = self.store.claim(self.key)
        self.store.begin_launch(token)
        self.store.confirm_identity(token, self.key)
        return token

    def test_environment_must_be_a_real_directory(self):
        with self.assertRaises(Rejected):
            self.store.register_session(
                provider='synthetic', environment='work-label', conversation_id='conversation-1',
                alias='other', workspace=str(self.workspace))
        with self.assertRaises(Rejected):
            self.store.register_session(
                provider='synthetic', environment=str(self.root / 'missing'),
                conversation_id='conversation-9', alias='other', workspace=str(self.workspace))
        with self.assertRaises(Rejected):
            canonical_directory('relative/profile')

    def test_symlink_and_second_alias_share_one_lock(self):
        link = self.root / 'env-link'
        link.symlink_to(self.env, target_is_directory=True)
        again = self.store.register_session(
            provider='synthetic', environment=str(link), conversation_id='conversation-1',
            alias='alias-two', workspace=str(self.workspace))
        self.assertEqual(again['session_key'], self.key)
        self.assertFalse(again['created'])
        self.assertEqual(sorted(again['aliases']), ['alias-two', 'worker'])
        self.enqueue()
        self.enqueue('m2', alias='alias-two')
        self.assertIsNotNone(self.store.claim(self.key))
        self.assertIsNone(self.store.claim(self.key))
        self.assertEqual(len(self.store.snapshot()['sessions']), 1)

    def test_workspace_and_mail_label_do_not_split_or_rebind_the_lock(self):
        other_workspace = self.root / 'other-workspace'
        other_workspace.mkdir()
        with self.assertRaises(Rejected):
            self.store.register_session(
                provider='synthetic', environment=str(self.env), conversation_id='conversation-1',
                alias='worker', workspace=str(other_workspace))
        other = self.store.register_session(
            provider='synthetic', environment=str(self.env), conversation_id='conversation-2',
            alias='other', workspace=str(self.workspace))
        self.assertNotEqual(other['session_key'], self.key)
        self.enqueue()
        self.enqueue('m2', alias='other')
        self.assertIsNotNone(self.store.claim(self.key))
        self.assertIsNotNone(self.store.claim(other['session_key']))

    def test_exact_conversation_id_required(self):
        for bad in ('last', 'latest', 'continue', '--resume', '-p'):
            with self.assertRaises(Rejected):
                self.store.register_session(
                    provider='synthetic', environment=str(self.env), conversation_id=bad,
                    alias='bad-' + bad.strip('-'), workspace=str(self.workspace))

    def test_duplicate_enqueue_and_conflicting_route(self):
        self.assertTrue(self.enqueue())
        self.assertFalse(self.enqueue())
        with self.assertRaises(Rejected):
            self.store.enqueue('m1', 'worker', 'coordinator', 'other payload')
        with self.assertRaises(Rejected):
            self.store.enqueue('m1', 'worker', 'someone-else', 'request m1')

    def test_identity_gate_and_bound_reply(self):
        self.enqueue()
        token = self.store.claim(self.key)
        self.store.begin_launch(token)
        with self.assertRaises(Rejected):
            self.store.current_message(token)
        with self.assertRaises(Rejected):
            self.store.reply(token, 'early')
        with self.assertRaises(Rejected):
            self.store.confirm_identity(token, session_key('synthetic', str(self.env.resolve()), 'wrong-id'))
        self.store.confirm_identity(token, self.key)
        self.assertEqual(self.store.current_message(token)['text'], 'request m1')
        first = self.store.reply(token, 'ACK:request m1')
        again = self.store.reply(token, 'ACK:request m1')
        self.assertEqual(again, first)
        self.assertEqual((first['sender'], first['recipient'], first['reply_to']),
                         ('worker', 'coordinator', 'm1'))
        with self.assertRaises(Rejected):
            self.store.reply(token, 'different')
        self.assertEqual(len(self.store.snapshot()['replies']), 1)

    def test_mcp_stdio_rejects_send_and_closed_context(self):
        token = self.running()
        results = rpc(self.db, token, [
            ('tools/list', {}),
            tool('read_current', {}),
            tool('send_message', {'to': 'stranger', 'text': 'wrong'}),
            tool('reply', {'text': 'ACK:request m1', 'message_id': 'foreign'}),
            tool('reply', {'text': 'ACK:request m1'}),
        ])
        results = results[1:]
        self.assertEqual([item['name'] for item in results[0]['tools']], ['read_current', 'reply'])
        self.assertTrue(results[2]['isError'])
        self.assertTrue(results[3]['isError'])
        self.assertFalse(results[4]['isError'])
        self.store.finish_after_wait(token, 0)
        denied = rpc(self.db, token, [tool('read_current', {}), tool('reply', {'text': 'late'})])[1:]
        self.assertTrue(all(item['isError'] for item in denied))

    def test_completion_requires_supervised_exit_and_reply(self):
        token = self.running()
        self.assertEqual(self.store.finish_after_wait(token, 0), 'uncertain')
        self.enqueue('m2')
        self.assertIsNone(self.store.claim(self.key))
        self.store.close()
        reopened = Store(self.db)
        try:
            self.assertEqual(reopened.snapshot()['requests'][0]['state'], 'uncertain')
            self.assertIsNone(reopened.claim(self.key))
        finally:
            reopened.close()

    def test_nonzero_exit_keeps_reply_and_slot(self):
        token = self.running()
        self.store.reply(token, 'stored')
        self.assertEqual(self.store.finish_after_wait(token, 1), 'uncertain')
        self.assertEqual(len(self.store.snapshot()['replies']), 1)
        self.assertIsNone(self.store.claim(self.key))

    def test_pause_blocks_claim_without_clearing_the_queue(self):
        self.enqueue()
        self.store.set_paused(True)
        self.assertIsNone(self.store.claim(self.key))
        self.assertEqual(self.store.snapshot()['requests'][0]['state'], 'queued')
        self.store.set_paused(False)
        self.assertIsNotNone(self.store.claim(self.key))

    def test_six_connections_one_claim(self):
        self.enqueue()
        self.store.close()
        barrier = threading.Barrier(6)
        results = []

        def race():
            store = Store(self.db)
            try:
                barrier.wait(timeout=5)
                results.append(store.claim(self.key))
            finally:
                store.close()

        threads = [threading.Thread(target=race) for _ in range(6)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(timeout=5)
        self.store = Store(self.db)
        self.assertEqual(sum(item is not None for item in results), 1)

    def test_stop_revokes_mcp_and_resolution_does_not_unlock(self):
        token = self.running()
        self.store.reply(token, 'ACK:request m1')
        stopped = self.store.revoke_current(self.key)
        self.assertEqual(stopped['state'], 'uncertain')
        self.assertFalse(stopped['launch_allowed'])
        with self.assertRaises(Rejected):
            invoke(self.store, token, 'read_current', {})
        with self.assertRaises(Rejected):
            invoke(self.store, token, 'reply', {'text': 'later'})
        with self.assertRaises(Rejected):
            self.store.accept_verified('worker', 'ok')
        with self.assertRaises(RecoveryBlocked):
            self.store.accept_verified('worker', 'compared the reply with the task log')
        with self.assertRaises(RecoveryBlocked):
            self.store.authorize_attempt('worker', 'operator asked', 'actions-may-repeat')
        self.assertEqual(len(self.store.snapshot()['attempts']), 1)
        self.assertEqual(self.store.attempt_state(token), 'uncertain')
        closed = self.store.fail_closed('worker', 'do not run this message again')
        self.assertFalse(closed['retried'])
        self.assertFalse(closed['launch_allowed'])
        self.enqueue('m2')
        self.assertIsNone(self.store.claim(self.key))
        self.assertEqual(self.store.snapshot()['requests'][0]['state'], 'failed')
        actions = [row['action'] for row in self.store.snapshot()['audit']]
        self.assertIn('accept_refused', actions)
        self.assertIn('authorize_refused', actions)
        self.assertIn('fail_closed', actions)

    def test_reconcile_blocks_unsupervised_attempt_without_retry(self):
        token = self.running()
        self.store.close()
        revived = Store(self.db)
        try:
            changed = revived.reconcile_unsupervised()
            self.assertEqual(changed, [self.key])
            self.assertEqual(revived.attempt_state(token), 'uncertain')
            with self.assertRaises(Rejected):
                revived.current_message(token)
            self.enqueue_on = revived.enqueue('m2', 'worker', 'coordinator', 'next')
            self.assertIsNone(revived.claim(self.key))
            again = revived.reconcile_unsupervised()
            self.assertEqual(again, [])
            self.assertEqual(revived.attempt_state(token), 'uncertain')
        finally:
            revived.close()
        self.store = Store(self.db)

    def test_process_record_is_not_exit_proof(self):
        self.assertFalse(process_exit_proven(None))
        self.assertFalse(process_exit_proven({}))
        self.assertFalse(process_exit_proven({'pid': None, 'pgid': None}))
        self.assertFalse(process_exit_proven({'pid': 1, 'pgid': 1, 'boot_id': 'other-boot'}))
        own = capture_process(os.getpid())
        self.assertEqual(own['pid'], os.getpid())
        self.assertTrue(own['pgid'])
        self.assertFalse(own['exit_proof'])
        self.assertIn(own['start_time_source'], ('supervisor_clock', 'linux_proc_stat'))
        self.assertFalse(process_exit_proven(own))
        self.assertEqual(signal_process_group(os.getpgrp()), 'not_signaled_own_group')
        child = subprocess.Popen(
            [sys.executable, '-c', 'import time; time.sleep(30)'],
            stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            start_new_session=True)
        try:
            recorded = capture_process(child.pid)
            self.assertEqual(signal_process_group(recorded['pgid']), 'sigterm_sent_not_proof')
            self.assertFalse(process_exit_proven(recorded))
            self.assertEqual(child.wait(timeout=5), -15)
        finally:
            if child.poll() is None:
                child.kill()
                child.wait(timeout=5)

    def test_queue_file_survives_reopen(self):
        self.enqueue()
        self.store.close()
        revived = Store(self.db)
        try:
            self.assertEqual(revived.snapshot()['requests'][0]['text'], 'request m1')
            self.assertEqual(revived.snapshot()['requests'][0]['state'], 'queued')
        finally:
            revived.close()
        self.store = Store(self.db)


if __name__ == '__main__':
    unittest.main()
