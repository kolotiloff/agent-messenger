"""One local supervisor, synthetic processes only. Does not call a model."""
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest

SRC = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(SRC))

from agent_messenger.behavior import PAUSE, SHUTDOWN, STOP_CURRENT
from agent_messenger.errors import Rejected
from agent_messenger.store import Store


def wait_until(predicate, timeout=8):
    end = time.monotonic() + timeout
    last = None
    while time.monotonic() < end:
        last = predicate()
        if last:
            return last
        time.sleep(0.02)
    raise AssertionError('timed out: %r' % (last,))


class Client:
    def __init__(self, data, allow_synthetic=True, extra_env=None, extra_args=()):
        self.data = Path(data)
        self.env = os.environ.copy()
        self.env['PYTHONPATH'] = str(SRC)
        if extra_env:
            self.env.update(extra_env)
        self.allow_synthetic = allow_synthetic
        self.extra_args = tuple(extra_args)
        self.proc = None
        self.stderr_path = self.data / 'supervisor.stderr'

    def start(self):
        self.data.mkdir(parents=True, exist_ok=True)
        command = [sys.executable, '-m', 'agent_messenger', '--data-dir', str(self.data), 'serve']
        if self.allow_synthetic:
            command.append('--allow-synthetic')
        command.extend(self.extra_args)
        stderr = open(self.stderr_path, 'w', encoding='utf-8')
        self.proc = subprocess.Popen(
            command, env=self.env, stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE, stderr=stderr, text=True)
        line = self._read_line()
        try:
            payload = json.loads(line)
        except json.JSONDecodeError as exc:
            raise RuntimeError('%s / %s' % (line, self.stderr())) from exc
        if not payload.get('ok'):
            raise RuntimeError(json.dumps(payload) + self.stderr())
        return payload

    def _read_line(self):
        end = time.monotonic() + 10
        while time.monotonic() < end:
            if self.proc.poll() is not None:
                raise RuntimeError('supervisor exited %s %s' % (self.proc.returncode, self.stderr()))
            line = self.proc.stdout.readline()
            if line:
                return line
            time.sleep(0.01)
        raise TimeoutError(self.stderr())

    def stderr(self):
        try:
            return self.stderr_path.read_text(encoding='utf-8')
        except OSError:
            return ''

    def cmd(self, *args):
        completed = subprocess.run(
            [sys.executable, '-m', 'agent_messenger', '--data-dir', str(self.data), *args],
            env=self.env, capture_output=True, text=True, timeout=15)
        try:
            body = json.loads(completed.stdout)
        except json.JSONDecodeError:
            body = {'_stdout': completed.stdout, '_stderr': completed.stderr}
        return completed.returncode, body, completed.stderr

    def shutdown(self):
        if self.proc is None or self.proc.poll() is not None:
            return
        self.cmd('shutdown')
        try:
            self.proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self.proc.kill()
            self.proc.wait(timeout=5)

    def stop_workers(self):
        runs = self.data / 'runs'
        if not runs.exists():
            return
        for pid_file in runs.glob('*/worker.pid'):
            try:
                pid = int(pid_file.read_text(encoding='utf-8').strip())
            except ValueError:
                continue
            try:
                os.kill(pid, signal.SIGTERM)
            except OSError:
                pass


class ClientTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='amc', dir='/tmp')
        self.data = Path(self.temp.name)
        self.client = Client(self.data)
        self.client.start()
        self.env = self.data / 'cli-env'
        self.env.mkdir()

    def tearDown(self):
        self.client.shutdown()
        self.client.stop_workers()
        if self.client.proc is not None and self.client.proc.poll() is None:
            self.client.proc.kill()
            self.client.proc.wait(timeout=5)
        self.temp.cleanup()

    def register(self, alias='worker', conversation='conv-1', environment=None):
        code, body, err = self.client.cmd(
            'register-session', '--provider', 'synthetic',
            '--environment', str(environment or self.env),
            '--conversation-id', conversation, '--alias', alias)
        self.assertEqual(code, 0, (body, err))
        return body['result']

    def enqueue(self, message_id, text, alias='worker'):
        code, body, err = self.client.cmd(
            'enqueue', '--message-id', message_id, '--alias', alias,
            '--sender', 'coordinator', '--text', text)
        self.assertEqual(code, 0, (body, err))
        return body['result']

    def inspect(self):
        code, body, err = self.client.cmd('inspect')
        self.assertEqual(code, 0, (body, err))
        return body['result']

    def status(self):
        code, body, err = self.client.cmd('status')
        self.assertEqual(code, 0, (body, err))
        return body['result']

    def attempt_dirs(self):
        return [Path(item['process']['run_dir']) for item in self.inspect()['attempts']
                if item.get('process') and item['process'].get('run_dir')]

    def test_second_supervisor_is_refused_and_commands_do_not_launch(self):
        other = subprocess.run(
            [sys.executable, '-m', 'agent_messenger', '--data-dir', str(self.data),
             'serve', '--allow-synthetic'],
            env=self.client.env, capture_output=True, text=True, timeout=10)
        self.assertNotEqual(other.returncode, 0)
        self.assertIn('lock', other.stdout.lower())
        self.assertEqual(self.status()['supervisor'], True)
        self.assertEqual(self.status()['model_launches'], 0)
        idle = Client(self.data / 'idle-store')
        try:
            idle.start()
            self.register()
            # The idle client is a different store; this client's queue is empty.
            time.sleep(0.3)
            self.assertEqual(self.status()['synthetic_launches'], 0)
            self.assertEqual(self.status()['children'], 0)
            self.assertEqual(self.status()['model_launches'], 0)
            self.assertFalse(self.status()['broker_started'])
            self.assertEqual(self.status()['broker_mode'], 'local_only')
        finally:
            idle.shutdown()

    def test_help_states_queue_and_process_behavior(self):
        completed = subprocess.run(
            [sys.executable, '-m', 'agent_messenger', '--help'],
            env=self.client.env, capture_output=True, text=True, timeout=10)
        self.assertEqual(completed.returncode, 0)
        for text in (PAUSE, STOP_CURRENT, SHUTDOWN):
            self.assertIn(text, completed.stdout)
        down = subprocess.run(
            [sys.executable, '-m', 'agent_messenger', '--data-dir', str(self.data / 'absent'), 'enqueue',
             '--message-id', 'm1', '--alias', 'worker', '--sender', 'coordinator', '--text', 'hello'],
            env=self.client.env, capture_output=True, text=True, timeout=10)
        self.assertNotEqual(down.returncode, 0)
        self.assertIn('does not start a second supervisor', down.stdout)

    def test_two_messages_run_in_order_without_a_model(self):
        self.register()
        self.enqueue('m1', 'alpha')
        directory = wait_until(lambda: next((path for path in self.attempt_dirs()
                                             if (path / 'replied').exists()), None))
        self.assertTrue((directory / 'heartbeat').exists() or (directory / 'replied').exists())
        self.enqueue('m2', 'beta')
        time.sleep(0.3)
        snapshot = self.inspect()
        self.assertEqual([item['state'] for item in snapshot['requests']], ['running', 'queued'])
        self.assertEqual(snapshot['status']['synthetic_launches'], 1)
        self.assertEqual(snapshot['status']['model_launches'], 0)
        self.assertEqual(snapshot['replies'][0]['text'], 'ACK:alpha')
        self.assertEqual(snapshot['replies'][0]['reply_to'], 'm1')
        (directory / 'release').write_text('1', encoding='utf-8')
        wait_until(lambda: self.inspect()['requests'][0]['state'] == 'completed')
        second = wait_until(lambda: next((path for path in self.attempt_dirs()
                                          if path != directory and (path / 'replied').exists()), None))
        self.assertEqual(self.status()['synthetic_launches'], 2)
        self.assertEqual(self.status()['model_launches'], 0)
        (second / 'release').write_text('1', encoding='utf-8')
        wait_until(lambda: [item['state'] for item in self.inspect()['requests']] == ['completed', 'completed'])

    def test_pause_keeps_the_process_and_blocks_the_next_claim(self):
        self.register()
        self.enqueue('m1', 'alpha')
        directory = wait_until(lambda: next((path for path in self.attempt_dirs()
                                             if (path / 'heartbeat').exists()), None))
        pid = int((directory / 'worker.pid').read_text(encoding='utf-8'))
        first = (directory / 'heartbeat').read_text(encoding='utf-8')
        code, body, err = self.client.cmd('pause')
        self.assertEqual(code, 0, (body, err))
        self.assertTrue(body['result']['paused'])
        self.enqueue('m2', 'beta')
        time.sleep(0.25)
        self.assertNotEqual((directory / 'heartbeat').read_text(encoding='utf-8'), first)
        os.kill(pid, 0)
        self.assertEqual([item['state'] for item in self.inspect()['requests']], ['running', 'queued'])
        self.assertEqual(self.status()['synthetic_launches'], 1)
        (directory / 'release').write_text('1', encoding='utf-8')
        wait_until(lambda: self.inspect()['requests'][0]['state'] == 'completed')
        time.sleep(0.3)
        self.assertEqual(self.inspect()['requests'][1]['state'], 'queued')
        self.assertEqual(self.status()['synthetic_launches'], 1)
        code, body, err = self.client.cmd('unpause')
        self.assertEqual(code, 0, (body, err))
        wait_until(lambda: self.status()['synthetic_launches'] == 2)

    def test_stop_revokes_mcp_and_does_not_retry(self):
        self.register()
        self.enqueue('m1', 'alpha')
        directory = wait_until(lambda: next((path for path in self.attempt_dirs()
                                             if (path / 'replied').exists()), None))
        pid = int((directory / 'worker.pid').read_text(encoding='utf-8'))
        token = self.inspect()['attempts'][0]['token']
        code, body, err = self.client.cmd('stop-current')
        self.assertEqual(code, 0, (body, err))
        self.assertEqual(body['result']['state'], 'uncertain')
        self.assertFalse(body['result']['launch_allowed'])
        self.assertFalse(body['result']['process_exit_proof'])
        self.assertIn('not_proof', body['result']['signal'])
        store = Store(self.data / 'queue.sqlite')
        try:
            with self.assertRaises(Rejected):
                store.current_message(token)
        finally:
            store.close()
        self.enqueue('m2', 'beta')
        time.sleep(0.4)
        snapshot = self.inspect()
        self.assertEqual(snapshot['requests'][0]['state'], 'uncertain')
        self.assertEqual(snapshot['requests'][1]['state'], 'queued')
        self.assertEqual(snapshot['status']['synthetic_launches'], 1)
        code, body, err = self.client.cmd(
            'authorize-attempt', '--alias', 'worker', '--reason', 'checked the log',
            '--acknowledgement', 'actions-may-repeat')
        self.assertNotEqual(code, 0)
        self.assertEqual(body['error']['code'], 'recovery_blocked')
        self.assertIn('Actions may be repeated', body['error']['message'])
        self.assertEqual(len(self.inspect()['attempts']), 1)
        code, body, err = self.client.cmd(
            'accept-verified', '--alias', 'worker',
            '--verification', 'compared the stored reply with the assigned task')
        self.assertNotEqual(code, 0)
        self.assertEqual(body['error']['code'], 'recovery_blocked')
        self.assertEqual(self.inspect()['requests'][0]['state'], 'uncertain')
        code, body, err = self.client.cmd(
            'fail-closed', '--alias', 'worker', '--reason', 'do not retry this message')
        self.assertEqual(code, 0, (body, err))
        self.assertEqual(body['result']['state'], 'failed')
        self.assertFalse(body['result']['retried'])
        code, body, err = self.client.cmd('unpause')
        self.assertEqual(code, 0, (body, err))
        time.sleep(0.3)
        self.assertEqual(self.inspect()['requests'][1]['state'], 'queued')
        self.assertEqual(self.status()['synthetic_launches'], 1)
        wait_until(lambda: not _alive(pid))

    def test_shutdown_leaves_the_process_and_restart_blocks_it(self):
        self.register()
        self.enqueue('m1', 'alpha')
        self.enqueue('m2', 'beta')
        directory = wait_until(lambda: next((path for path in self.attempt_dirs()
                                             if (path / 'heartbeat').exists()), None))
        pid = int((directory / 'worker.pid').read_text(encoding='utf-8'))
        token = self.inspect()['attempts'][0]['token']
        code, body, err = self.client.cmd('shutdown')
        self.assertEqual(code, 0, (body, err))
        self.assertEqual(body['result']['active_processes'], 'left_running')
        self.assertFalse(body['result']['marked_completed'])
        self.assertFalse(body['result']['mcp_revoked_by_this_command'])
        self.client.proc.wait(timeout=5)
        store = Store(self.data / 'queue.sqlite')
        try:
            self.assertEqual(store.current_message(token)['id'], 'm1')
            self.assertEqual(store.snapshot()['requests'][1]['state'], 'queued')
        finally:
            store.close()
        time.sleep(0.2)
        self.assertNotEqual((directory / 'heartbeat').read_text(encoding='utf-8'), '')
        os.kill(pid, 0)
        restarted = Client(self.data)
        restarted.proc = None
        try:
            restarted.start()
            self.client = restarted
            snapshot = self.inspect()
            self.assertEqual(snapshot['requests'][0]['state'], 'uncertain')
            self.assertEqual(snapshot['requests'][1]['state'], 'queued')
            self.assertEqual(snapshot['status']['synthetic_launches'], 1)
            store = Store(self.data / 'queue.sqlite')
            try:
                with self.assertRaises(Rejected):
                    store.current_message(token)
            finally:
                store.close()
            time.sleep(0.2)
            os.kill(pid, 0)
            self.assertEqual(self.status()['synthetic_launches'], 1)
        finally:
            pass

    def test_killed_supervisor_does_not_retry_on_restart(self):
        self.register()
        self.enqueue('m1', 'alpha')
        self.enqueue('m2', 'beta')
        directory = wait_until(lambda: next((path for path in self.attempt_dirs()
                                             if (path / 'heartbeat').exists()), None))
        pid = int((directory / 'worker.pid').read_text(encoding='utf-8'))
        self.client.proc.send_signal(signal.SIGKILL)
        self.client.proc.wait(timeout=5)
        time.sleep(0.2)
        os.kill(pid, 0)
        restarted = Client(self.data)
        restarted.start()
        self.client = restarted
        self.assertEqual(self.inspect()['requests'][0]['state'], 'uncertain')
        time.sleep(0.3)
        self.assertEqual(self.inspect()['requests'][1]['state'], 'queued')
        self.assertEqual(self.status()['synthetic_launches'], 1)
        os.kill(pid, signal.SIGTERM)
        wait_until(lambda: not _alive(pid))
        time.sleep(0.3)
        self.assertEqual(self.inspect()['requests'][0]['state'], 'uncertain')
        self.assertEqual(self.status()['synthetic_launches'], 1)
        self.assertEqual(self.status()['model_launches'], 0)

    def test_wrong_advertised_id_never_enables_a_reply(self):
        self.client.shutdown()
        self.client = Client(self.data, extra_env={
            'AM_TEST_SEAM': '1',
            'AM_ADVERTISE_SESSION_ID': 'wrong-id',
        })
        self.client.start()
        self.register()
        self.enqueue('m1', 'alpha')
        wait_until(lambda: self.inspect()['requests'] and self.inspect()['requests'][0]['state'] == 'uncertain')
        self.assertEqual(self.inspect()['replies'], [])
        self.assertEqual(self.status()['model_launches'], 0)
        self.enqueue('m2', 'beta')
        time.sleep(0.3)
        self.assertEqual(self.inspect()['requests'][1]['state'], 'queued')

    def test_same_profile_aliases_cannot_launch_twice(self):
        link = self.data / 'env-link'
        link.symlink_to(self.env, target_is_directory=True)
        first = self.register(alias='one')
        second = self.register(alias='two', environment=link)
        self.assertEqual(first['session_key'], second['session_key'])
        self.enqueue('m1', 'alpha', alias='one')
        self.enqueue('m2', 'beta', alias='two')
        wait_until(lambda: any((path / 'replied').exists() for path in self.attempt_dirs()))
        time.sleep(0.25)
        self.assertEqual(self.status()['synthetic_launches'], 1)
        self.assertEqual(self.status()['children'], 1)

    def test_duplicate_message_and_real_cli_gate(self):
        self.register()
        self.assertTrue(self.enqueue('m1', 'alpha')['enqueued'])
        self.assertFalse(self.enqueue('m1', 'alpha')['enqueued'])
        code, body, err = self.client.cmd(
            'enqueue', '--message-id', 'm1', '--alias', 'worker',
            '--sender', 'coordinator', '--text', 'changed')
        self.assertNotEqual(code, 0, err)
        self.assertEqual(body['error']['code'], 'rejected')
        marker = self.data / 'model-launched'
        script = self.data / 'fake_cli.py'
        script.write_text(
            '#!%s\nfrom pathlib import Path\nPath(%r).write_text("launched")\n' % (sys.executable, str(marker)),
            encoding='utf-8')
        script.chmod(0o755)
        code, body, err = self.client.cmd(
            'register-session', '--provider', 'claude', '--environment', str(self.env),
            '--conversation-id', 'claude-test-id', '--alias', 'claude-box',
            '--workspace', str(self.data / 'claude-work'), '--cli', str(script))
        # Workspace must exist first.
        self.assertNotEqual(code, 0)
        (self.data / 'claude-work').mkdir()
        code, body, err = self.client.cmd(
            'register-session', '--provider', 'claude', '--environment', str(self.env / 'claude-home'),
            '--conversation-id', 'claude-test-id', '--alias', 'claude-box',
            '--workspace', str(self.data / 'claude-work'), '--cli', str(script))
        self.assertNotEqual(code, 0)
        (self.env / 'claude-home').mkdir()
        code, body, err = self.client.cmd(
            'register-session', '--provider', 'claude', '--environment', str(self.env / 'claude-home'),
            '--conversation-id', 'claude-test-id', '--alias', 'claude-box',
            '--workspace', str(self.data / 'claude-work'), '--cli', str(script))
        self.assertEqual(code, 0, (body, err))
        code, body, err = self.client.cmd(
            'enqueue', '--message-id', 'c1', '--alias', 'claude-box',
            '--sender', 'coordinator', '--text', 'do not launch')
        self.assertEqual(code, 0, (body, err))
        time.sleep(0.4)
        self.assertFalse(marker.exists())
        self.assertEqual(self.status()['model_launches'], 0)
        held = [item for item in self.inspect()['requests'] if item['id'] == 'c1']
        self.assertEqual(held[0]['state'], 'queued')

    def test_broker_mode_is_refused_before_launch(self):
        self.client.shutdown()
        config = {
            'version': 1,
            'broker': {'mode': 'connect', 'connect_url': 'http://127.0.0.1:9', 'broker_id': 'remote'},
            'allow_model_calls': True,
        }
        (self.data / 'config.json').write_text(json.dumps(config), encoding='utf-8')
        refused = subprocess.run(
            [sys.executable, '-m', 'agent_messenger', '--data-dir', str(self.data),
             'serve', '--allow-synthetic', '--allow-model-calls'],
            env=self.client.env, capture_output=True, text=True, timeout=10)
        self.assertNotEqual(refused.returncode, 0)
        self.assertIn('not implemented', refused.stdout)
        self.assertNotIn('synthetic_worker', refused.stdout)
        (self.data / 'config.json').write_text(json.dumps({
            'version': 1,
            'broker': {'mode': 'local_only', 'connect_url': 'http://127.0.0.1:9', 'broker_id': None},
            'allow_model_calls': False,
        }), encoding='utf-8')
        self.client = Client(self.data)
        self.client.start()
        self.assertEqual(self.status()['broker_mode'], 'local_only')
        self.assertFalse(self.status()['broker_started'])
        self.assertFalse(self.status()['allow_model_calls'])


def _alive(pid):
    try:
        os.kill(pid, 0)
    except OSError:
        return False
    return True


if __name__ == '__main__':
    unittest.main()
