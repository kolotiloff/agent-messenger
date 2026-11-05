"""Experimental local queue. No provider CLI, credentials or network access."""
from contextlib import contextmanager
import hashlib
import json
import sqlite3
import uuid


class Rejected(ValueError):
    pass


def session_key(provider, profile, conversation_id):
    """Caller supplies canonical local profile and exact persisted CLI ID."""
    values = [provider, profile, conversation_id]
    if any(not isinstance(v, str) or not v for v in values):
        raise Rejected('Canonical session identity is required')
    return hashlib.sha256(json.dumps(values).encode()).hexdigest()


class Store:
    def __init__(self, path):
        self.db = sqlite3.connect(str(path), timeout=5, isolation_level=None)
        self.db.row_factory = sqlite3.Row
        self.db.execute('PRAGMA foreign_keys=ON')
        self.db.execute('PRAGMA synchronous=FULL')
        self.db.executescript('''
            CREATE TABLE IF NOT EXISTS sessions (
                key TEXT PRIMARY KEY, peer TEXT NOT NULL, active TEXT);
            CREATE TABLE IF NOT EXISTS requests (
                id TEXT PRIMARY KEY, session TEXT NOT NULL REFERENCES sessions(key),
                sender TEXT NOT NULL, text TEXT NOT NULL, state TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS attempts (
                token TEXT PRIMARY KEY, session TEXT NOT NULL REFERENCES sessions(key),
                request TEXT NOT NULL UNIQUE REFERENCES requests(id),
                state TEXT NOT NULL, child_pid INTEGER);
            CREATE TABLE IF NOT EXISTS replies (
                id TEXT PRIMARY KEY, reply_to TEXT NOT NULL UNIQUE REFERENCES requests(id),
                sender TEXT NOT NULL, recipient TEXT NOT NULL, text TEXT NOT NULL);
        ''')

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

    def register(self, key, peer):
        with self.transaction():
            old = self.db.execute('SELECT peer FROM sessions WHERE key=?', (key,)).fetchone()
            if old and old['peer'] != peer:
                raise Rejected('Session identity cannot be rebound')
            self.db.execute('INSERT OR IGNORE INTO sessions VALUES (?,?,NULL)', (key, peer))

    def enqueue(self, message_id, key, sender, text):
        if not all(isinstance(v, str) and v for v in (message_id, key, sender, text)):
            raise Rejected('Invalid envelope')
        with self.transaction():
            old = self.db.execute('SELECT * FROM requests WHERE id=?', (message_id,)).fetchone()
            if old:
                if (old['session'], old['sender'], old['text']) != (key, sender, text):
                    raise Rejected('Message ID reused with different content or route')
                return False
            self.db.execute('INSERT INTO requests VALUES (?,?,?,?,?)',
                            (message_id, key, sender, text, 'queued'))
            return True

    def claim(self, key):
        """Reserve before spawning. No time-based stealing or automatic retry."""
        with self.transaction():
            session = self.db.execute('SELECT * FROM sessions WHERE key=?', (key,)).fetchone()
            if not session:
                raise Rejected('Unknown session')
            if session['active'] is not None:
                return None
            request = self.db.execute(
                "SELECT id FROM requests WHERE session=? AND state='queued' ORDER BY rowid LIMIT 1",
                (key,)).fetchone()
            if not request:
                return None
            token = uuid.uuid4().hex
            self.db.execute('INSERT INTO attempts VALUES (?,?,?,?,NULL)',
                            (token, key, request['id'], 'reserved'))
            self.db.execute('UPDATE sessions SET active=? WHERE key=?', (token, key))
            self.db.execute("UPDATE requests SET state='reserved' WHERE id=?", (request['id'],))
            return token

    def _owned(self, token, states):
        row = self.db.execute('''SELECT a.*, s.peer, r.sender, r.text
            FROM attempts a JOIN sessions s ON a.session=s.key
            JOIN requests r ON r.id=a.request
            WHERE a.token=? AND s.active=a.token''', (token,)).fetchone()
        if not row or row['state'] not in states:
            raise Rejected('Inactive, quarantined or unknown execution context')
        return row

    def begin_launch(self, token, require_identity=False):
        # Commit intent BEFORE Popen: the child may exist even if no PID was saved.
        with self.transaction():
            row = self._owned(token, ('reserved',))
            state = 'launching' if require_identity else 'running'
            self.db.execute('UPDATE attempts SET state=? WHERE token=?', (state, token))
            self.db.execute('UPDATE requests SET state=? WHERE id=?', (state, row['request']))

    def confirm_identity(self, token, actual_session_key):
        """Trusted CLI adapter verifies startup identity before enabling MCP calls."""
        with self.transaction():
            row = self._owned(token, ('launching',))
            if row['session'] != actual_session_key:
                raise Rejected('CLI started a different session')
            self.db.execute("UPDATE attempts SET state='running' WHERE token=?", (token,))
            self.db.execute("UPDATE requests SET state='running' WHERE id=?", (row['request'],))

    def record_pid(self, token, pid):
        with self.transaction():
            self._owned(token, ('launching', 'running'))
            self.db.execute('UPDATE attempts SET child_pid=? WHERE token=?', (pid, token))

    def current_message(self, token):
        with self.transaction():
            row = self._owned(token, ('running',))
            return {'id': row['request'], 'from': row['sender'], 'text': row['text']}

    def reply(self, token, text):
        """No model-selected address, reply ID, sender or idempotency key."""
        if not isinstance(text, str) or not 1 <= len(text) <= 2000:
            raise Rejected('Reply text must contain 1..2000 characters')
        with self.transaction():
            row = self._owned(token, ('running',))
            old = self.db.execute('SELECT * FROM replies WHERE reply_to=?', (row['request'],)).fetchone()
            if old:
                if old['text'] != text:
                    raise Rejected('A different reply is already stored; no overwrite')
                return dict(old)
            reply = {'id': uuid.uuid4().hex, 'reply_to': row['request'],
                     'sender': row['peer'], 'recipient': row['sender'], 'text': text}
            self.db.execute('INSERT INTO replies VALUES (?,?,?,?,?)', tuple(reply.values()))
            return reply

    def finish_after_wait(self, token, returncode):
        """Trusted supervisor only; call after waiting for its own child to exit."""
        with self.transaction():
            row = self._owned(token, ('running',))
            reply = self.db.execute('SELECT id FROM replies WHERE reply_to=?', (row['request'],)).fetchone()
            completed = returncode == 0 and reply is not None
            state = 'completed' if completed else 'uncertain'
            self.db.execute('UPDATE attempts SET state=? WHERE token=?', (state, token))
            self.db.execute('UPDATE requests SET state=? WHERE id=?', (state, row['request']))
            if completed:
                self.db.execute('UPDATE sessions SET active=NULL WHERE key=?', (row['session'],))
            return state

    def quarantine(self, key):
        """Explicit recovery action: freeze, never clear or reuse an active slot."""
        with self.transaction():
            session = self.db.execute('SELECT active FROM sessions WHERE key=?', (key,)).fetchone()
            if not session or not session['active']:
                return
            token = session['active']
            row = self._owned(token, ('reserved', 'launching', 'running', 'uncertain'))
            self.db.execute("UPDATE attempts SET state='uncertain' WHERE token=?", (token,))
            self.db.execute("UPDATE requests SET state='uncertain' WHERE id=?", (row['request'],))

    def snapshot(self):
        # Test diagnostics only. Not exposed to agents through MCP.
        return {table: [dict(r) for r in self.db.execute('SELECT * FROM '+table+' ORDER BY rowid')]
                for table in ('sessions', 'requests', 'attempts', 'replies')}
