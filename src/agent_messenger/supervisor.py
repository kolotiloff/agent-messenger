"""The one local supervisor. It holds the OS lock, the queue, and every launch.

Idle waiting uses ``select`` on the command socket. It does not call a model.
A second process that cannot take the lock exits before it can launch.
"""
from __future__ import annotations

import json
import os
from pathlib import Path
import secrets
import select
import signal
import subprocess
import sys
import threading

from agent_messenger.adapters import (
    build_argv,
    cursor_preflight,
    observe_startup,
    reject_forbidden_argv,
    validate_result,
)
from agent_messenger.config import load_config
from agent_messenger.errors import AdapterError, ConfigError, Rejected
from agent_messenger.identity import (
    REAL_PROVIDERS,
    canonical_directory,
    check_alias,
    check_conversation_id,
    check_provider,
    session_key,
)
from agent_messenger.ipc import CommandServer, socket_path
from agent_messenger.locking import SupervisorLock
from agent_messenger.paths import prepare_data_dir
from agent_messenger.process_info import capture_process, signal_process_group
from agent_messenger.store import Store


def _child_env():
    env = os.environ.copy()
    root = str(Path(__file__).resolve().parents[1])
    previous = env.get('PYTHONPATH')
    env['PYTHONPATH'] = root if not previous else root + os.pathsep + previous
    env['PYTHONUNBUFFERED'] = '1'
    return env


def _popen_kwargs():
    if os.name == 'nt':
        return {'creationflags': subprocess.CREATE_NEW_PROCESS_GROUP}
    return {'start_new_session': True}


class Supervisor:
    def __init__(self, data_dir, *, allow_synthetic=False, allow_model_calls=False):
        self.data_dir = Path(data_dir)
        self.allow_synthetic = bool(allow_synthetic)
        self._model_flag = bool(allow_model_calls)
        self.allow_model_calls = False
        self.store = None
        self.jobs = {}
        self._shutdown = False
        self._attention = {}
        self._signals_installed = False

    def serve(self):
        self.data_dir = prepare_data_dir(self.data_dir)
        lock = SupervisorLock(self.data_dir / 'supervisor.lock')
        server = None
        try:
            config = load_config(self.data_dir / 'config.json')
            if self._model_flag and not config['allow_model_calls']:
                raise ConfigError(
                    'Refusing --allow-model-calls because config allow_model_calls is false. '
                    'No model was called.')
            self.allow_model_calls = bool(self._model_flag and config['allow_model_calls'])
            self.store = Store(self.data_dir / 'queue.sqlite')
            reconciled = self.store.reconcile_unsupervised()
            token = secrets.token_hex(32)
            token_path = self.data_dir / 'ipc.token'
            token_path.write_text(token + '\n', encoding='utf-8')
            if os.name == 'posix':
                os.chmod(token_path, 0o600)
            chosen = socket_path(self.data_dir)
            (self.data_dir / 'ipc.path').write_text(os.fspath(chosen), encoding='utf-8')
            if os.name == 'posix':
                os.chmod(self.data_dir / 'ipc.path', 0o600)
            server = CommandServer(chosen, token, self.handle)
            self._install_signals()
            print(json.dumps({'ok': True, 'event': 'ready', 'reconciled': len(reconciled)}), flush=True)
            while not self._shutdown:
                self._poll_jobs()
                if self._shutdown:
                    break
                self._launch_ready()
                try:
                    readable, _, _ = select.select([server], [], [], 0.05)
                except (InterruptedError, OSError):
                    continue
                if readable and not self._shutdown:
                    server.accept_pending()
        finally:
            for job in list(self.jobs.values()):
                self._close_reader(job)
            self.jobs.clear()
            if server is not None:
                server.close()
            if self.store is not None:
                self.store.close()
                self.store = None
            lock.release()

    def _install_signals(self):
        if self._signals_installed or threading.current_thread() is not threading.main_thread():
            return
        signal.signal(signal.SIGTERM, self._on_signal)
        signal.signal(signal.SIGINT, self._on_signal)
        self._signals_installed = True

    def _on_signal(self, _signum, _frame):
        # Leave the active child alone. The loop performs a normal shutdown.
        self._shutdown = True

    def handle(self, command, args):
        if not isinstance(args, dict):
            raise Rejected('Arguments must be an object')
        if command == 'shutdown':
            return self._cmd_shutdown()
        if self._shutdown:
            raise Rejected('Client is shutting down')
        if command == 'status':
            self._exact(args, ())
            return self._status()
        if command == 'inspect':
            self._exact(args, ())
            data = self.store.inspect_data()
            data['children'] = len(self.jobs)
            data['allow_model_calls'] = self.allow_model_calls
            return data
        if command == 'pause':
            self._exact(args, ())
            self.store.set_paused(True)
            return self._status()
        if command == 'unpause':
            self._exact(args, ())
            self.store.set_paused(False)
            return self._status()
        if command == 'stop-current':
            self._exact(args, (), ('alias',))
            return self._cmd_stop(args.get('alias'))
        if command == 'enqueue':
            self._exact(args, ('message_id', 'alias', 'sender', 'text'))
            created = self.store.enqueue(args['message_id'], args['alias'], args['sender'], args['text'])
            return {'enqueued': created, 'message_id': args['message_id']}
        if command == 'register-session':
            self._exact(args, ('provider', 'environment', 'conversation_id', 'alias'), ('workspace', 'cli'))
            return self._cmd_register(args)
        if command == 'accept-verified':
            self._exact(args, ('alias', 'verification'))
            return self.store.accept_verified(args['alias'], args['verification'])
        if command == 'fail-closed':
            self._exact(args, ('alias', 'reason'))
            return self.store.fail_closed(args['alias'], args['reason'])
        if command == 'authorize-attempt':
            self._exact(args, ('alias', 'reason', 'acknowledgement'))
            return self.store.authorize_attempt(
                args['alias'], args['reason'], args['acknowledgement'])
        raise Rejected('Unknown command')

    def _exact(self, args, required, optional=()):
        unknown = set(args) - set(required) - set(optional)
        if unknown:
            raise Rejected('Unexpected arguments')
        for key in required:
            if key not in args:
                raise Rejected('Missing ' + key)

    def _status(self):
        data = self.store.status_data()
        data['supervisor'] = True
        data['accepting_commands'] = not self._shutdown
        data['children'] = len(self.jobs)
        data['allow_synthetic'] = self.allow_synthetic
        data['allow_model_calls'] = self.allow_model_calls
        return data

    def _cmd_shutdown(self):
        self._shutdown = True
        self.store.audit_shutdown()
        return {
            'shutting_down': True,
            'queue': 'kept',
            'active_processes': 'left_running',
            'marked_completed': False,
            'mcp_revoked_by_this_command': False,
            'active': self._status()['active'],
            'next_start': (
                'Unsupervised attempts become uncertain, their MCP context is revoked, '
                'and they are not launched again.'
            ),
        }

    def _cmd_stop(self, alias):
        if alias:
            check_alias(alias)
            session = self.store.session_for_alias(alias)
            if not session['active']:
                raise Rejected('That session has no current job')
            key = session['key']
        else:
            active = list(self.store.active_sessions())
            if len(active) != 1:
                raise Rejected('Pass alias when there is not exactly one current job')
            key = active[0]['key']
        info = self.store.revoke_current(key)
        signal_result = 'not_signaled_no_record'
        process = info.get('process') or {}
        pgid = process.get('pgid')
        if isinstance(pgid, int):
            signal_result = signal_process_group(pgid)
        self.store.note_signal(key, info['token'], signal_result)
        info['signal'] = signal_result
        info['mcp_revoked'] = True
        return info

    def _cmd_register(self, args):
        provider = check_provider(args['provider'])
        environment = canonical_directory(args['environment'])
        conversation = check_conversation_id(args['conversation_id'])
        workspace = args.get('workspace')
        if not workspace:
            if provider != 'synthetic':
                raise Rejected('A workspace directory is required for a real CLI session')
            workspace = str(self.data_dir / 'workspaces' / session_key(
                provider, environment, conversation)[:16])
            Path(workspace).mkdir(parents=True, mode=0o700, exist_ok=True)
        return self.store.register_session(
            provider=provider,
            environment=environment,
            conversation_id=conversation,
            alias=args['alias'],
            workspace=workspace,
            cli_path=args.get('cli'),
        )

    def _launch_ready(self):
        if self._shutdown or self.store.is_paused():
            return
        for session in self.store.queued_sessions():
            if not self._can_launch(session):
                continue
            try:
                self._start(session)
            except Rejected:
                continue

    def _can_launch(self, session):
        provider = session['provider']
        if provider == 'synthetic':
            return self.allow_synthetic
        if provider not in REAL_PROVIDERS or not self.allow_model_calls:
            return False
        if provider == 'cursor':
            try:
                cursor_preflight(Path.home(), os.environ)
            except AdapterError as exc:
                note = str(exc)
                if self._attention.get(session['key']) != note:
                    self._attention[session['key']] = note
                    self.store.set_attention(session['key'], note)
                return False
        return True

    def _start(self, session):
        key = session['key']
        token = self.store.claim(key)
        if not token:
            return
        run_dir = self.data_dir / 'runs' / token
        stdout = stderr = None
        try:
            run_dir.mkdir(parents=True, mode=0o700)
            if os.name == 'posix':
                os.chmod(run_dir, 0o700)
            self.store.begin_launch(token)
            argv = self._command(session, token, run_dir)
            reject_forbidden_argv(argv)
            stdout = open(run_dir / 'stdout.jsonl', 'ab', buffering=0)
            stderr = open(run_dir / 'stderr.log', 'ab', buffering=0)
            process = subprocess.Popen(
                argv,
                cwd=session['workspace'],
                stdin=subprocess.DEVNULL,
                stdout=stdout,
                stderr=stderr,
                close_fds=True,
                env=_child_env(),
                **_popen_kwargs(),
            )
        except Exception as exc:
            self.store.mark_uncertain(key, 'launch_failed')
            self.store.set_attention(key, 'launch_failed:' + exc.__class__.__name__)
            return
        finally:
            if stdout is not None:
                stdout.close()
            if stderr is not None:
                stderr.close()
        record = capture_process(process.pid)
        record['run_dir'] = str(run_dir)
        try:
            self.store.record_process(token, record)
            self.store.add_launch(session['provider'])
        except Rejected:
            self.store.mark_uncertain(key, 'launch_failed')
            signal_process_group(record.get('pgid'))
            return
        self.jobs[token] = {
            'process': process,
            'reader': open(run_dir / 'stdout.jsonl', 'rb', buffering=0),
            'pending': b'',
            'events': [],
            'confirmed': False,
            'identity_failed': False,
            'bad_output': False,
            'token': token,
            'session_key': key,
            'provider': session['provider'],
            'environment_id': session['environment_id'],
            'conversation_id': session['conversation_id'],
            'pgid': record.get('pgid'),
        }

    def _command(self, session, token, run_dir):
        if session['provider'] == 'synthetic':
            return [
                sys.executable, '-m', 'agent_messenger.synthetic_worker',
                '--db', self.store.path,
                '--token', token,
                '--session-id', session['conversation_id'],
                '--run-dir', str(run_dir),
            ]
        return build_argv(
            session['provider'],
            cli=session['cli_path'],
            db=self.store.path,
            token=token,
            workspace=Path(session['workspace']),
            conversation_id=session['conversation_id'],
            home=Path.home(),
            environ=os.environ,
        )

    def _poll_jobs(self):
        for token, job in list(self.jobs.items()):
            self._consume(job)
            code = job['process'].poll()
            if code is None:
                continue
            self._on_exit(job, code)
            self._close_reader(job)
            del self.jobs[token]

    def _consume(self, job):
        if job['identity_failed'] or job['bad_output']:
            return
        try:
            chunk = job['reader'].read(65536)
        except OSError:
            job['bad_output'] = True
            return
        if not chunk:
            return
        job['pending'] += chunk
        while b'\n' in job['pending']:
            if len(job['events']) > 20000:
                job['bad_output'] = True
                self.store.mark_uncertain(job['session_key'], 'cli_output_limit')
                return
            line, job['pending'] = job['pending'].split(b'\n', 1)
            if not line.strip():
                continue
            try:
                event = json.loads(line.decode('utf-8'))
            except (UnicodeDecodeError, json.JSONDecodeError):
                job['bad_output'] = True
                continue
            job['events'].append(event)
            if job['confirmed'] or job['identity_failed']:
                continue
            try:
                started = observe_startup(job['provider'], event)
            except AdapterError:
                self._fail_identity(job, 'startup_rejected')
                continue
            if started is None:
                continue
            try:
                actual = session_key(job['provider'], job['environment_id'], started)
                self.store.confirm_identity(job['token'], actual)
            except Rejected:
                self._fail_identity(job, 'identity_mismatch')
                continue
            job['confirmed'] = True

    def _fail_identity(self, job, reason):
        job['identity_failed'] = True
        self.store.mark_uncertain(job['session_key'], reason)
        signal_process_group(job.get('pgid'))

    def _on_exit(self, job, code):
        if job['identity_failed']:
            return
        state = self.store.attempt_state(job['token'])
        if state != 'running':
            if state == 'launching':
                self.store.mark_uncertain(job['session_key'], 'exit_before_identity')
            return
        if job['provider'] != 'synthetic':
            if job['bad_output']:
                self.store.mark_uncertain(job['session_key'], 'unreadable_cli_output')
                return
            try:
                validate_result(job['provider'], job['events'], job['conversation_id'])
            except AdapterError:
                self.store.mark_uncertain(job['session_key'], 'result_rejected')
                return
        try:
            self.store.finish_after_wait(job['token'], code)
        except Rejected:
            return

    def _close_reader(self, job):
        reader = job.get('reader')
        if reader is None:
            return
        try:
            reader.close()
        except OSError:
            pass
