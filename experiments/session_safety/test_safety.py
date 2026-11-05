"""Run with python3 -m unittest discover -s experiments/session_safety -v."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from runtime import Rejected, Store, session_key
from process_fixture import rpc, tool, wait_file


class SafetyTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='messenger-safety-')
        self.root = Path(self.temp.name)
        self.db = self.root/'queue.sqlite'
        self.store = Store(self.db)
        self.key = session_key('synthetic', 'local-test', 'conversation-1')
        self.store.register(self.key, 'worker')
        self.processes = []

    def tearDown(self):
        # Release even orphaned fixtures; no long-running services are installed.
        for process, directory, log in self.processes:
            directory.joinpath('release').touch()
            directory.joinpath('finish-release').touch()
        for process, directory, log in self.processes:
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=5)
            if directory.joinpath('child-started').exists():
                wait_file(directory/'child-exited', timeout=5)
            log.close()
        self.store.close()
        self.temp.cleanup()

    def enqueue(self, mid='m1', key=None):
        return self.store.enqueue(mid, key or self.key, 'coordinator', 'request '+mid)

    def running(self):
        self.enqueue()
        token = self.store.claim(self.key)
        self.store.begin_launch(token)
        return token

    def start(self, mode, name):
        directory = self.root/name
        directory.mkdir()
        log = directory.joinpath('stderr').open('w')
        process = subprocess.Popen([sys.executable, str(Path(__file__).with_name('process_fixture.py')),
                                    mode, str(self.db), self.key, str(directory)],
                                   stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=log)
        self.processes.append((process, directory, log))
        return process, directory

    def assert_blocked(self, name):
        process, directory = self.start('normal', name)
        self.assertEqual(process.wait(timeout=5), 0)
        self.assertTrue(directory.joinpath('blocked').exists())
        self.assertFalse(directory.joinpath('child-started').exists())

    def test_reply_bound_by_server_and_duplicate_ack_loss(self):
        token = self.running()
        results = rpc(self.db, token, [
            ('tools/list', {}), tool('read_current', {}),
            tool('send_message', {'to': 'stranger', 'text': 'wrong'}),
            tool('reply', {'text': 'wrong', 'message_id': 'foreign'}),
            tool('reply', {'text': 'correct'})])
        self.assertEqual([t['name'] for t in results[0]['tools']], ['read_current', 'reply'])
        self.assertTrue(results[2]['isError'])
        self.assertTrue(results[3]['isError'])
        self.assertFalse(results[4]['isError'])
        first = json.loads(results[4]['content'][0]['text'])
        # A new MCP process retries after losing its response: same stored reply.
        repeated = rpc(self.db, token, [tool('reply', {'text': 'correct'}),
                                        tool('reply', {'text': 'different'})])
        self.assertEqual(json.loads(repeated[0]['content'][0]['text']), first)
        self.assertTrue(repeated[1]['isError'])
        self.assertEqual((first['reply_to'], first['sender'], first['recipient']),
                         ('m1', 'worker', 'coordinator'))
        self.assertEqual(len(self.store.snapshot()['replies']), 1)

    def test_foreign_or_completed_context_cannot_reply(self):
        token = self.running()
        self.store.reply(token, 'ok')
        self.assertEqual(self.store.finish_after_wait(token, 0), 'completed')
        self.enqueue('m2')
        next_token = self.store.claim(self.key)
        self.store.begin_launch(next_token)
        for bad in (token, 'invented-context'):
            results = rpc(self.db, bad, [tool('reply', {'text': 'late'})])
            self.assertTrue(results[0]['isError'])
        self.assertEqual(len(self.store.snapshot()['replies']), 1)

    def test_six_processes_only_one_claim(self):
        self.enqueue('m1')
        self.enqueue('m2')
        racers = [self.start('claim', 'racer-'+str(i)) for i in range(6)]
        for _, directory in racers:
            wait_file(directory/'ready')
        self.root.joinpath('barrier').touch()
        claims = []
        for process, directory in racers:
            self.assertEqual(process.wait(timeout=5), 0)
            claims.append(json.loads(directory.joinpath('result.json').read_text()))
        self.assertEqual(sum(c is not None for c in claims), 1)
        self.assertEqual([r['state'] for r in self.store.snapshot()['requests']], ['reserved', 'queued'])

    def test_second_worker_waits_until_first_exit(self):
        self.enqueue('m1')
        self.enqueue('m2')
        first, directory = self.start('normal', 'first')
        wait_file(directory/'replied')
        self.assertIsNone(first.poll())
        self.assert_blocked('second-while-first-alive')
        directory.joinpath('release').touch()
        self.assertEqual(first.wait(timeout=5), 0)
        self.assertEqual(directory.joinpath('finished').read_text(), 'completed')
        second, other = self.start('normal', 'second-after-exit')
        wait_file(other/'replied')
        other.joinpath('release').touch()
        self.assertEqual(second.wait(timeout=5), 0)
        snapshot = self.store.snapshot()
        self.assertEqual([r['state'] for r in snapshot['requests']], ['completed', 'completed'])
        self.assertEqual(len(snapshot['attempts']), 2)

    def test_duplicate_delivery_cannot_start_another_attempt(self):
        token = self.running()
        self.assertFalse(self.enqueue())
        with self.assertRaises(Rejected):
            self.store.enqueue('m1', self.key, 'coordinator', 'conflicting payload')
        self.store.reply(token, 'done')
        self.store.finish_after_wait(token, 0)
        self.assertFalse(self.enqueue())
        self.assertIsNone(self.store.claim(self.key))
        self.assertEqual(len(self.store.snapshot()['attempts']), 1)

    def test_independent_sessions_do_not_share_lock(self):
        other = session_key('synthetic', 'local-test', 'conversation-2')
        self.store.register(other, 'other-worker')
        self.enqueue()
        self.enqueue('m2', other)
        self.assertIsNotNone(self.store.claim(self.key))
        self.assertIsNotNone(self.store.claim(other))
        self.assertNotEqual(self.key, session_key('synthetic', 'other-profile', 'conversation-1'))

    def test_same_reservation_cannot_authorize_two_launches(self):
        token = self.running()
        with self.assertRaises(Rejected):
            self.store.begin_launch(token)
        self.assertEqual(len(self.store.snapshot()['attempts']), 1)

    def test_identity_required_before_mcp_can_read_or_reply(self):
        self.enqueue()
        token = self.store.claim(self.key)
        self.store.begin_launch(token, require_identity=True)
        results = rpc(self.db, token, [tool('read_current', {}), tool('reply', {'text': 'early'})])
        self.assertTrue(all(r['isError'] for r in results))
        self.store.confirm_identity(token, self.key)
        self.assertEqual(self.store.current_message(token)['id'], 'm1')
        self.assertEqual(self.store.reply(token, 'after verification')['reply_to'], 'm1')

    def test_mismatched_identity_never_enables_mcp(self):
        self.enqueue()
        token = self.store.claim(self.key)
        self.store.begin_launch(token, require_identity=True)
        with self.assertRaises(Rejected):
            self.store.confirm_identity(token, session_key('synthetic', 'local-test', 'wrong-id'))
        self.store.quarantine(self.key)
        with self.assertRaises(Rejected):
            self.store.confirm_identity(token, self.key)
        self.assertIsNone(self.store.claim(self.key))
        self.assertEqual(self.store.snapshot()['replies'], [])

    def test_clean_exit_without_reply_does_not_complete(self):
        token = self.running()
        self.assertEqual(self.store.finish_after_wait(token, 0), 'uncertain')
        self.enqueue('m2')
        self.assertIsNone(self.store.claim(self.key))

    def test_nonzero_exit_with_reply_keeps_slot(self):
        token = self.running()
        self.store.reply(token, 'already stored')
        self.assertEqual(self.store.finish_after_wait(token, 1), 'uncertain')
        self.enqueue('m2')
        self.assertIsNone(self.store.claim(self.key))
        self.assertEqual(len(self.store.snapshot()['replies']), 1)

    def test_crash_after_reservation_before_spawn(self):
        self.enqueue()
        self.enqueue('m2')
        process, directory = self.start('crash-before-spawn', 'owner')
        wait_file(directory/'paused')
        process.kill()
        process.wait(timeout=5)
        self.assertFalse(directory.joinpath('child-started').exists())
        self.assert_blocked('restart')
        self.store.quarantine(self.key)
        self.assertEqual(self.store.snapshot()['attempts'][0]['state'], 'uncertain')
        self.assert_blocked('restart-after-quarantine')

    def test_crash_with_live_child_before_pid_saved(self):
        self.enqueue()
        self.enqueue('m2')
        process, directory = self.start('crash-before-pid', 'owner')
        wait_file(directory/'paused')
        token = directory.joinpath('token').read_text()
        self.assertIsNone(self.store.snapshot()['attempts'][0]['child_pid'])
        process.kill()
        process.wait(timeout=5)
        wait_file(directory/'heartbeat')
        # Continued heartbeat is evidence the child survived its supervisor.
        before = directory.joinpath('heartbeat').read_text()
        end = time.monotonic()+3
        while directory.joinpath('heartbeat').read_text() in ('', before):
            self.assertLess(time.monotonic(), end)
            time.sleep(0.02)
        self.assert_blocked('restart-with-live-child')
        self.store.quarantine(self.key)
        self.assertTrue(rpc(self.db, token, [tool('reply', {'text': 'late'})])[0]['isError'])
        directory.joinpath('release').touch()
        wait_file(directory/'child-exited')
        self.assert_blocked('restart-after-child-exit')
        self.assertEqual(len(self.store.snapshot()['attempts']), 1)

    def test_crash_after_launch_intent_before_process_creation(self):
        self.enqueue()
        self.enqueue('m2')
        process, directory = self.start('crash-after-intent', 'owner')
        wait_file(directory/'paused')
        process.kill()
        process.wait(timeout=5)
        snapshot = self.store.snapshot()
        self.assertEqual(snapshot['attempts'][0]['state'], 'running')
        self.assertIsNone(snapshot['attempts'][0]['child_pid'])
        self.assertFalse(directory.joinpath('child-started').exists())
        self.assert_blocked('restart')
        self.store.quarantine(self.key)
        self.assert_blocked('restart-quarantined')

    def test_crash_after_child_exit_before_completion_commit(self):
        self.enqueue()
        self.enqueue('m2')
        process, directory = self.start('crash-before-finish', 'owner')
        wait_file(directory/'replied')
        directory.joinpath('release').touch()
        wait_file(directory/'paused')
        process.kill()
        process.wait(timeout=5)
        self.assertTrue(directory.joinpath('child-exited').exists())
        self.assert_blocked('restart')
        self.store.quarantine(self.key)
        snapshot = self.store.snapshot()
        self.assertEqual(snapshot['attempts'][0]['state'], 'uncertain')
        self.assertEqual(len(snapshot['replies']), 1)


if __name__ == '__main__':
    unittest.main(verbosity=2)
