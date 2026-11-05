"""Durable queue and session lock. The supervisor is the only launcher.

SQLite ``BEGIN IMMEDIATE`` is extra protection on top of the OS supervisor
lock. MCP can read and reply only while its own attempt is ``running``.
"""
from __future__ import annotations

from contextlib import contextmanager
import json
import os
import sqlite3
import time
import uuid

from agent_messenger.errors import RecoveryBlocked, Rejected
from agent_messenger.identity import (
    canonical_directory,
    canonical_executable,
    check_alias,
    check_body,
    check_conversation_id,
    check_provider,
    check_sender,
    session_key,
)
from agent_messenger.process_info import process_exit_proven

_LAUNCH_STATES = ('reserved', 'launching', 'running')


def _now():
    return time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())


class Store:
    def __init__(self, path):
        self.path = str(path)
        self.db = sqlite3.connect(self.path, timeout=5, isolation_level=None)
        self.db.row_factory = sqlite3.Row
        self.db.execute('PRAGMA foreign_keys=ON')
        self.db.execute('PRAGMA busy_timeout=5000')
        self.db.execute('PRAGMA journal_mode=WAL')
        self.db.execute('PRAGMA synchronous=FULL')
        self.db.executescript('''
            CREATE TABLE IF NOT EXISTS meta (
                key TEXT PRIMARY KEY,
                value TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS sessions (
                key TEXT PRIMARY KEY,
                provider TEXT NOT NULL,
                environment_id TEXT NOT NULL,
                conversation_id TEXT NOT NULL,
                peer TEXT NOT NULL,
                cli_path TEXT,
                workspace TEXT NOT NULL,
                active TEXT,
                hold_reason TEXT,
                attention TEXT,
                UNIQUE(provider, environment_id, conversation_id));
            CREATE TABLE IF NOT EXISTS aliases (
                alias TEXT PRIMARY KEY,
                session_key TEXT NOT NULL REFERENCES sessions(key));
            CREATE TABLE IF NOT EXISTS requests (
                id TEXT PRIMARY KEY,
                session_key TEXT NOT NULL REFERENCES sessions(key),
                sender TEXT NOT NULL,
                text TEXT NOT NULL,
                state TEXT NOT NULL,
                created_at TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS attempts (
                token TEXT PRIMARY KEY,
                session_key TEXT NOT NULL REFERENCES sessions(key),
                request_id TEXT NOT NULL REFERENCES requests(id),
                state TEXT NOT NULL,
                generation INTEGER NOT NULL,
                process_json TEXT,
                stop_requested INTEGER NOT NULL DEFAULT 0,
                created_at TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS replies (
                id TEXT PRIMARY KEY,
                reply_to TEXT NOT NULL UNIQUE REFERENCES requests(id),
                attempt_token TEXT NOT NULL,
                sender TEXT NOT NULL,
                recipient TEXT NOT NULL,
                text TEXT NOT NULL,
                created_at TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS audit (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                at TEXT NOT NULL,
                action TEXT NOT NULL,
                session_key TEXT,
                attempt_token TEXT,
                detail_json TEXT NOT NULL);
        ''')
        if os.name == 'posix':
            os.chmod(self.path, 0o600)

    def close(self):
        self.db.close()

    @contextmanager
    def transaction(self):
        self.db.execute('BEGIN IMMEDIATE')
        try:
            yield
            self.db.execute('COMMIT')
        except BaseException:
            self.db.execute('ROLLBACK')
            raise

    def _meta_get(self, key, default=None):
        row = self.db.execute('SELECT value FROM meta WHERE key=?', (key,)).fetchone()
        return row['value'] if row else default

    def _meta_set(self, key, value):
        self.db.execute('INSERT OR REPLACE INTO meta(key, value) VALUES (?, ?)', (key, value))

    def _audit(self, action, session=None, attempt=None, detail=None):
        self.db.execute(
            'INSERT INTO audit(at, action, session_key, attempt_token, detail_json) VALUES (?,?,?,?,?)',
            (_now(), action, session, attempt, json.dumps(detail or {}, sort_keys=True)))

    def is_paused(self):
        return self._meta_get('intake_paused', '0') == '1'

    def set_paused(self, paused):
        with self.transaction():
            self._meta_set('intake_paused', '1' if paused else '0')
            self._audit('pause' if paused else 'unpause', detail={'paused': bool(paused)})
        return self.is_paused()

    def launch_counts(self):
        return {
            'synthetic_launches': int(self._meta_get('synthetic_launches', '0')),
            'model_launches': int(self._meta_get('model_launches', '0')),
        }

    def add_launch(self, provider):
        with self.transaction():
            key = 'synthetic_launches' if provider == 'synthetic' else 'model_launches'
            value = int(self._meta_get(key, '0')) + 1
            self._meta_set(key, str(value))
            self._audit('launch_counted', detail={'kind': key, 'count': value})
            return value

    def register_session(self, *, provider, environment, conversation_id, alias,
                         workspace, cli_path=None):
        """Bind a mail alias to one real environment. A second alias is not a second lock."""
        provider = check_provider(provider)
        environment_id = canonical_directory(environment)
        conversation_id = check_conversation_id(conversation_id)
        alias = check_alias(alias)
        workspace_id = canonical_directory(workspace)
        if provider == 'synthetic':
            if cli_path:
                raise Rejected('Synthetic sessions do not take a CLI path')
            cli = None
        else:
            if not cli_path:
                raise Rejected('A real CLI path is required')
            cli = canonical_executable(cli_path)
        key = session_key(provider, environment_id, conversation_id)
        with self.transaction():
            other = self.db.execute('SELECT session_key FROM aliases WHERE alias=?', (alias,)).fetchone()
            if other and other['session_key'] != key:
                raise Rejected('Alias already names a different session')
            existing = self.db.execute('SELECT * FROM sessions WHERE key=?', (key,)).fetchone()
            created = existing is None
            if existing:
                if (existing['cli_path'] or None) != cli or existing['workspace'] != workspace_id:
                    raise Rejected('Session identity cannot be rebound')
                if not other:
                    self.db.execute('INSERT INTO aliases(alias, session_key) VALUES (?, ?)', (alias, key))
                    self._audit('add_alias', key, detail={'alias': alias})
            else:
                self.db.execute(
                    '''INSERT INTO sessions(key, provider, environment_id, conversation_id, peer,
                       cli_path, workspace, active, hold_reason, attention)
                       VALUES (?,?,?,?,?,?,?,NULL,NULL,NULL)''',
                    (key, provider, environment_id, conversation_id, alias, cli, workspace_id))
                self.db.execute('INSERT INTO aliases(alias, session_key) VALUES (?, ?)', (alias, key))
                self._audit('register_session', key, detail={
                    'provider': provider, 'alias': alias, 'environment_id': environment_id})
            aliases = [row['alias'] for row in self.db.execute(
                'SELECT alias FROM aliases WHERE session_key=? ORDER BY alias', (key,))]
        return {
            'session_key': key,
            'provider': provider,
            'environment_id': environment_id,
            'conversation_id': conversation_id,
            'alias': alias,
            'aliases': aliases,
            'created': created,
        }

    def _session_for_alias(self, alias):
        alias = check_alias(alias)
        row = self.db.execute(
            '''SELECT s.* FROM aliases a JOIN sessions s ON s.key=a.session_key
               WHERE a.alias=?''', (alias,)).fetchone()
        if not row:
            raise Rejected('Unknown alias')
        return row

    def session_for_alias(self, alias):
        return self._session_for_alias(alias)

    def enqueue(self, message_id, alias, sender, text):
        if not isinstance(message_id, str) or not message_id.strip() or message_id != message_id.strip():
            raise Rejected('Invalid envelope')
        if len(message_id) > 200 or any(char in message_id for char in '\r\n\x00'):
            raise Rejected('Invalid envelope')
        sender = check_sender(sender)
        text = check_body(text, 8000, 'Message text')
        with self.transaction():
            session = self._session_for_alias(alias)
            key = session['key']
            old = self.db.execute('SELECT * FROM requests WHERE id=?', (message_id,)).fetchone()
            if old:
                if (old['session_key'], old['sender'], old['text']) != (key, sender, text):
                    raise Rejected('Message ID reused with different content or route')
                self._audit('enqueue_duplicate', key, detail={'message_id': message_id})
                return False
            self.db.execute(
                'INSERT INTO requests(id, session_key, sender, text, state, created_at) VALUES (?,?,?,?,?,?)',
                (message_id, key, sender, text, 'queued', _now()))
            self._audit('enqueue', key, detail={'message_id': message_id})
            return True

    def queued_sessions(self):
        return self.db.execute('''
            SELECT s.* FROM sessions s
            WHERE s.active IS NULL AND s.hold_reason IS NULL
              AND EXISTS (
                SELECT 1 FROM requests r WHERE r.session_key=s.key AND r.state='queued')
            ORDER BY s.rowid''').fetchall()

    def active_sessions(self):
        return self.db.execute(
            'SELECT * FROM sessions WHERE active IS NOT NULL ORDER BY rowid').fetchall()

    def claim(self, key):
        """Reserve before spawning. Pause, a hold, or an active token yields nothing."""
        with self.transaction():
            if self._meta_get('intake_paused', '0') == '1':
                return None
            session = self.db.execute('SELECT * FROM sessions WHERE key=?', (key,)).fetchone()
            if not session:
                raise Rejected('Unknown session')
            if session['active'] is not None or session['hold_reason']:
                return None
            request = self.db.execute(
                '''SELECT id FROM requests WHERE session_key=? AND state='queued'
                   ORDER BY rowid LIMIT 1''', (key,)).fetchone()
            if not request:
                return None
            token = uuid.uuid4().hex
            generation = self.db.execute(
                'SELECT COUNT(*) AS n FROM attempts WHERE request_id=?',
                (request['id'],)).fetchone()['n'] + 1
            self.db.execute(
                '''INSERT INTO attempts(token, session_key, request_id, state, generation,
                   process_json, stop_requested, created_at) VALUES (?,?,?,?,?,?,0,?)''',
                (token, key, request['id'], 'reserved', generation, None, _now()))
            updated = self.db.execute(
                'UPDATE sessions SET active=?, attention=NULL WHERE key=? AND active IS NULL AND hold_reason IS NULL',
                (token, key)).rowcount
            if updated != 1:
                raise Rejected('Session claim lost')
            self.db.execute("UPDATE requests SET state='reserved' WHERE id=?", (request['id'],))
            self._audit('claim', key, token, {'request_id': request['id'], 'generation': generation})
            return token

    def _row(self, token):
        return self.db.execute('''
            SELECT a.*, s.peer, s.provider, s.environment_id, s.conversation_id,
                   s.active, s.hold_reason, s.workspace, s.cli_path,
                   r.sender, r.text, r.state AS request_state
            FROM attempts a
            JOIN sessions s ON s.key=a.session_key
            JOIN requests r ON r.id=a.request_id
            WHERE a.token=?''', (token,)).fetchone()

    def _owned(self, token, states):
        row = self._row(token)
        if not row or row['active'] != token or row['state'] not in states:
            raise Rejected('Inactive, quarantined or unknown execution context')
        return row

    def begin_launch(self, token):
        """Commit launch intent before process creation. MCP stays closed."""
        with self.transaction():
            row = self._owned(token, ('reserved',))
            self.db.execute("UPDATE attempts SET state='launching' WHERE token=?", (token,))
            self.db.execute("UPDATE requests SET state='launching' WHERE id=?", (row['request_id'],))
            self._audit('begin_launch', row['session_key'], token, {})

    def confirm_identity(self, token, actual_session_key):
        with self.transaction():
            row = self._owned(token, ('launching',))
            if actual_session_key != row['session_key']:
                raise Rejected('CLI started a different session')
            self.db.execute("UPDATE attempts SET state='running' WHERE token=?", (token,))
            self.db.execute("UPDATE requests SET state='running' WHERE id=?", (row['request_id'],))
            self._audit('confirm_identity', row['session_key'], token, {})

    def record_process(self, token, record):
        with self.transaction():
            row = self._owned(token, ('launching', 'running'))
            self.db.execute(
                'UPDATE attempts SET process_json=? WHERE token=?',
                (json.dumps(record, sort_keys=True), token))
            self._audit('record_process', row['session_key'], token, {
                'pid': record.get('pid'),
                'pgid': record.get('pgid'),
                'start_time_source': record.get('start_time_source'),
                'exit_proof': False,
            })

    def current_message(self, token):
        with self.transaction():
            row = self._owned(token, ('running',))
            return {'id': row['request_id'], 'from': row['sender'], 'text': row['text']}

    def reply(self, token, text):
        """Route and idempotency key belong to the client, not the caller."""
        text = check_body(text, 2000, 'Reply text')
        with self.transaction():
            row = self._owned(token, ('running',))
            old = self.db.execute(
                'SELECT * FROM replies WHERE reply_to=?', (row['request_id'],)).fetchone()
            if old:
                if old['text'] != text:
                    raise Rejected('A different reply is already stored; no overwrite')
                return _public_reply(old)
            reply = {
                'id': uuid.uuid4().hex,
                'reply_to': row['request_id'],
                'attempt_token': token,
                'sender': row['peer'],
                'recipient': row['sender'],
                'text': text,
                'created_at': _now(),
            }
            self.db.execute(
                '''INSERT INTO replies(id, reply_to, attempt_token, sender, recipient, text, created_at)
                   VALUES (?,?,?,?,?,?,?)''',
                (reply['id'], reply['reply_to'], reply['attempt_token'], reply['sender'],
                 reply['recipient'], reply['text'], reply['created_at']))
            self._audit('reply', row['session_key'], token, {'reply_id': reply['id']})
            return _public_reply(reply)

    def finish_after_wait(self, token, returncode):
        """Trusted supervisor only, after wait() on the child it spawned.

        Exit 0 without a stored reply, or a non-zero exit with a reply, stays
        uncertain and keeps the session. This is not used for crash recovery.
        """
        with self.transaction():
            row = self._owned(token, ('running',))
            if row['stop_requested']:
                raise Rejected('Stop was requested; completion is not automatic')
            reply = self.db.execute(
                'SELECT id FROM replies WHERE reply_to=?', (row['request_id'],)).fetchone()
            if returncode == 0 and reply is not None:
                self.db.execute("UPDATE attempts SET state='completed' WHERE token=?", (token,))
                self.db.execute("UPDATE requests SET state='completed' WHERE id=?", (row['request_id'],))
                self.db.execute(
                    'UPDATE sessions SET active=NULL, hold_reason=NULL WHERE key=? AND active=?',
                    (row['session_key'], token))
                self._audit('finish_completed', row['session_key'], token, {'returncode': returncode})
                return 'completed'
            reason = 'exit_without_reply' if reply is None else 'nonzero_exit'
            self.db.execute("UPDATE attempts SET state='uncertain' WHERE token=?", (token,))
            self.db.execute("UPDATE requests SET state='uncertain' WHERE id=?", (row['request_id'],))
            self.db.execute(
                'UPDATE sessions SET hold_reason=? WHERE key=?', (reason, row['session_key']))
            self._audit('finish_uncertain', row['session_key'], token, {
                'returncode': returncode, 'reply_present': reply is not None, 'reason': reason})
            return 'uncertain'

    def mark_uncertain(self, key, reason):
        """Revoke MCP and keep the slot. Does not enqueue a retry."""
        with self.transaction():
            return self._mark_uncertain(key, reason)

    def _mark_uncertain(self, key, reason):
        session = self.db.execute('SELECT * FROM sessions WHERE key=?', (key,)).fetchone()
        if not session or not session['active']:
            return False
        attempt = self.db.execute(
            'SELECT * FROM attempts WHERE token=?', (session['active'],)).fetchone()
        if not attempt or attempt['state'] in ('completed', 'failed'):
            return False
        if attempt['state'] != 'uncertain':
            self.db.execute("UPDATE attempts SET state='uncertain' WHERE token=?", (attempt['token'],))
            self.db.execute(
                '''UPDATE requests SET state='uncertain'
                   WHERE id=? AND state IN ('reserved', 'launching', 'running')''',
                (attempt['request_id'],))
        if not session['hold_reason']:
            self.db.execute(
                'UPDATE sessions SET hold_reason=? WHERE key=?', (reason, key))
        self._audit('mark_uncertain', key, attempt['token'], {'reason': reason})
        return True

    def reconcile_unsupervised(self):
        """A start that did not spawn these attempts cannot know they are alive.

        Mark them uncertain, keep the slot, and do not retry. This is not a
        PID check and it does not adopt the old process.
        """
        changed = []
        with self.transaction():
            rows = self.db.execute(
                'SELECT key, active FROM sessions WHERE active IS NOT NULL').fetchall()
            for row in rows:
                attempt = self.db.execute(
                    'SELECT state FROM attempts WHERE token=?', (row['active'],)).fetchone()
                if attempt and attempt['state'] in _LAUNCH_STATES:
                    self._mark_uncertain(row['key'], 'unsupervised_on_start')
                    changed.append(row['key'])
        return changed

    def revoke_current(self, key):
        """Stop path: uncertain immediately. Signaling is the supervisor's job."""
        with self.transaction():
            session = self.db.execute('SELECT * FROM sessions WHERE key=?', (key,)).fetchone()
            if not session or not session['active']:
                raise Rejected('No current job')
            attempt = self.db.execute(
                'SELECT * FROM attempts WHERE token=?', (session['active'],)).fetchone()
            if not attempt or attempt['state'] in ('completed', 'failed'):
                raise Rejected('No current job')
            self.db.execute(
                "UPDATE attempts SET state='uncertain', stop_requested=1 WHERE token=?",
                (attempt['token'],))
            self.db.execute(
                '''UPDATE requests SET state='uncertain'
                   WHERE id=? AND state IN ('reserved', 'launching', 'running', 'uncertain')''',
                (attempt['request_id'],))
            if not session['hold_reason']:
                self.db.execute(
                    "UPDATE sessions SET hold_reason='stop_current' WHERE key=?", (key,))
            process = json.loads(attempt['process_json']) if attempt['process_json'] else None
            self._audit('stop_current', key, attempt['token'], {
                'mcp_revoked': True, 'retried': False, 'process_exit_proof': False})
            return {
                'token': attempt['token'],
                'session_key': key,
                'state': 'uncertain',
                'process': process,
                'retried': False,
                'launch_allowed': False,
                'process_exit_proof': False,
            }

    def note_signal(self, key, token, result):
        with self.transaction():
            self._audit('stop_signal', key, token, {
                'result': result, 'process_exit_proof': False})

    def set_attention(self, key, note):
        text = str(note)[:500]
        with self.transaction():
            self.db.execute('UPDATE sessions SET attention=? WHERE key=?', (text, key))
            self._audit('attention', key, detail={'note': text})

    def audit_shutdown(self):
        with self.transaction():
            active = self.db.execute(
                'SELECT COUNT(*) AS n FROM sessions WHERE active IS NOT NULL').fetchone()['n']
            self._audit('shutdown', detail={
                'queue': 'kept',
                'active_processes': 'left_running',
                'marked_completed': False,
                'mcp_revoked_by_this_command': False,
                'active_sessions': active,
            })

    def _require_uncertain(self, alias):
        session = self._session_for_alias(alias)
        if not session['active']:
            raise Rejected('No blocked attempt on this session')
        attempt = self.db.execute(
            'SELECT * FROM attempts WHERE token=?', (session['active'],)).fetchone()
        if not attempt or attempt['state'] != 'uncertain':
            raise Rejected(
                'The attempt is not uncertain. fail/accept/authorize do not kill a running process')
        reply = self.db.execute(
            'SELECT * FROM replies WHERE reply_to=?', (attempt['request_id'],)).fetchone()
        process = json.loads(attempt['process_json']) if attempt['process_json'] else None
        return session, attempt, reply, process

    def accept_verified(self, alias, verification):
        """Record a refusal unless process-tree proof exists. It does not.

        A verification sentence is required so a bare reply cannot be presented
        as acceptance. Even with that sentence, acceptance stays blocked.
        """
        if not isinstance(verification, str) or len(verification.strip()) < 8:
            raise Rejected(
                'Verification must describe the check that was performed. '
                'A stored reply is not verification')
        text = verification.strip()
        if text.lower() in {'reply', 'reply_exists', 'stored reply', 'ok', 'yes', 'accepted'}:
            raise Rejected(
                'Verification must describe the check that was performed. '
                'A stored reply is not verification')
        with self.transaction():
            session, attempt, reply, process = self._require_uncertain(alias)
            if reply is not None and text == reply['text']:
                raise Rejected('Verification must be distinct from the reply text')
            proven = process_exit_proven(process)
            self._audit('accept_refused', session['key'], attempt['token'], {
                'verification_present': True,
                'reply_present': reply is not None,
                'process_exit_proof': bool(proven),
                'state_changed': False,
            })
        if proven:
            raise RecoveryBlocked(
                'Process-exit proof was reported, but this client has no accept transition. '
                'The result was not accepted and the session was not released.')
        raise RecoveryBlocked(
            'Accepting a result is blocked: reliable proof that the old process '
            'tree has exited is not implemented. A stored reply, a missing PID, '
            'or this verification text does not accept the result and does not '
            'allow a new launch.')

    def fail_closed(self, alias, reason):
        """Close this attempt without retry. Does not claim the process stopped."""
        if not isinstance(reason, str) or not reason.strip():
            raise Rejected('A reason is required')
        with self.transaction():
            session, attempt, _reply, _process = self._require_uncertain(alias)
            self.db.execute("UPDATE attempts SET state='failed' WHERE token=?", (attempt['token'],))
            self.db.execute("UPDATE requests SET state='failed' WHERE id=?", (attempt['request_id'],))
            self.db.execute(
                "UPDATE sessions SET hold_reason='failed_closed' WHERE key=?", (session['key'],))
            self._audit('fail_closed', session['key'], attempt['token'], {
                'reason': reason.strip()[:500],
                'retried': False,
                'launch_allowed': False,
                'process_declared_stopped': False,
            })
            return {
                'state': 'failed',
                'retried': False,
                'launch_allowed': False,
                'process_exit_proof': False,
                'history_kept': True,
            }

    def authorize_attempt(self, alias, reason, acknowledgement):
        """A repeat would be a new attempt. Proof is unimplemented, so this refuses.

        The warning that actions may be repeated is part of the refusal. No
        attempt row is created and the slot stays occupied.
        """
        if acknowledgement != 'actions-may-repeat':
            raise Rejected(
                'A repeat must be explicitly acknowledged with actions-may-repeat. '
                'Actions may be repeated. No attempt was created.')
        if not isinstance(reason, str) or not reason.strip():
            raise Rejected('A reason is required')
        with self.transaction():
            session, attempt, _reply, process = self._require_uncertain(alias)
            proven = process_exit_proven(process)
            before = self.db.execute(
                'SELECT COUNT(*) AS n FROM attempts WHERE request_id=?',
                (attempt['request_id'],)).fetchone()['n']
            self._audit('authorize_refused', session['key'], attempt['token'], {
                'reason': reason.strip()[:500],
                'acknowledgement': acknowledgement,
                'warning': 'actions may be repeated',
                'process_exit_proof': bool(proven),
                'attempt_created': False,
                'attempts_for_request': before,
            })
        if proven:
            raise RecoveryBlocked(
                'Process-exit proof was reported, but this client will not create another '
                'attempt. Actions may be repeated. No attempt was created.')
        raise RecoveryBlocked(
            'A new attempt is forbidden until reliable proof the old process '
            'tree has exited. That proof is not implemented. A missing PID is '
            'not proof. Actions may be repeated if a later version authorizes '
            'another attempt. No attempt was created.')

    def attempt_state(self, token):
        row = self.db.execute('SELECT state FROM attempts WHERE token=?', (token,)).fetchone()
        return row['state'] if row else None

    def snapshot(self):
        tables = ('sessions', 'aliases', 'requests', 'attempts', 'replies', 'audit', 'meta')
        return {table: [dict(row) for row in self.db.execute('SELECT * FROM ' + table + ' ORDER BY rowid')]
                for table in tables}

    def status_data(self):
        from agent_messenger.behavior import LIMITATIONS, PAUSE, SHUTDOWN, STOP_CURRENT, UNPAUSE
        counts = self.launch_counts()
        active = []
        for session in self.active_sessions():
            attempt = self.db.execute(
                'SELECT state, process_json FROM attempts WHERE token=?',
                (session['active'],)).fetchone()
            process = json.loads(attempt['process_json']) if attempt and attempt['process_json'] else None
            active.append({
                'provider': session['provider'],
                'alias_peer': session['peer'],
                'conversation_id': session['conversation_id'],
                'state': attempt['state'] if attempt else None,
                'hold_reason': session['hold_reason'],
                'pid': None if not process else process.get('pid'),
                'process_exit_proof': False,
            })
        queued = self.db.execute(
            "SELECT COUNT(*) AS n FROM requests WHERE state='queued'").fetchone()['n']
        return {
            'paused': self.is_paused(),
            'queued': queued,
            'active': active,
            'sessions': self.db.execute('SELECT COUNT(*) AS n FROM sessions').fetchone()['n'],
            'broker_mode': 'local_only',
            'broker_started': False,
            'synthetic_launches': counts['synthetic_launches'],
            'model_launches': counts['model_launches'],
            'limitations': list(LIMITATIONS),
            'commands': {
                'pause': PAUSE,
                'unpause': UNPAUSE,
                'stop-current': STOP_CURRENT,
                'shutdown': SHUTDOWN,
            },
        }

    def inspect_data(self):
        def rows(sql, args=()):
            return [dict(row) for row in self.db.execute(sql, args)]

        attempts = []
        for row in rows('SELECT * FROM attempts ORDER BY rowid'):
            process = json.loads(row['process_json']) if row['process_json'] else None
            if process:
                process = {
                    'pid': process.get('pid'),
                    'pgid': process.get('pgid'),
                    'start_time_source': process.get('start_time_source'),
                    'supervisor_observed_at': process.get('supervisor_observed_at'),
                    'linux_start_unix': process.get('linux_start_unix'),
                    'boot_id_recorded': bool(process.get('boot_id')),
                    'run_dir': process.get('run_dir'),
                    'exit_proof': False,
                }
            attempts.append({
                'token': row['token'],
                'session_key': row['session_key'],
                'request_id': row['request_id'],
                'state': row['state'],
                'generation': row['generation'],
                'stop_requested': bool(row['stop_requested']),
                'process': process,
            })
        aliases = {}
        for row in rows('SELECT alias, session_key FROM aliases ORDER BY alias'):
            aliases.setdefault(row['session_key'], []).append(row['alias'])
        sessions = []
        for row in rows('SELECT * FROM sessions ORDER BY rowid'):
            sessions.append({
                'session_key': row['key'],
                'provider': row['provider'],
                'environment_id': row['environment_id'],
                'conversation_id': row['conversation_id'],
                'peer': row['peer'],
                'aliases': aliases.get(row['key'], []),
                'workspace': row['workspace'],
                'cli_path': row['cli_path'],
                'active': row['active'],
                'hold_reason': row['hold_reason'],
                'attention': row['attention'],
            })
        audit = rows('SELECT id, at, action, session_key, attempt_token, detail_json FROM audit ORDER BY id DESC LIMIT 50')
        for item in audit:
            item['detail'] = json.loads(item.pop('detail_json'))
        return {
            'status': self.status_data(),
            'sessions': sessions,
            'requests': rows(
                'SELECT id, session_key, sender, text, state, created_at FROM requests ORDER BY rowid'),
            'attempts': attempts,
            'replies': rows(
                'SELECT id, reply_to, sender, recipient, text, created_at FROM replies ORDER BY rowid'),
            'audit': list(reversed(audit)),
        }


def _public_reply(row):
    if not isinstance(row, dict):
        row = dict(row)
    return {
        'id': row['id'],
        'reply_to': row['reply_to'],
        'sender': row['sender'],
        'recipient': row['recipient'],
        'text': row['text'],
    }
