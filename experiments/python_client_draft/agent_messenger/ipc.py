"""Local command socket. Unix datagrams are not used; one JSON line per connection.

The socket lives beside the canonical store. It is not a network broker.
"""
from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import socket

from agent_messenger.errors import RecoveryBlocked, Rejected, SupervisorUnavailable


def socket_path(data_dir):
    direct = Path(data_dir) / 'client.sock'
    encoded = os.fsencode(direct)
    # macOS sun_path is about 104 bytes including the trailing NUL.
    if len(encoded) <= 100:
        return direct
    digest = hashlib.sha256(os.fsencode(str(Path(data_dir).resolve()))).hexdigest()[:20]
    return Path('/tmp') / ('am-' + digest + '.sock')


def call(data_dir, command, args=None, timeout=30):
    directory = Path(data_dir)
    token_path = directory / 'ipc.token'
    path_file = directory / 'ipc.path'
    if not token_path.is_file() or not path_file.is_file():
        raise SupervisorUnavailable(
            'Client is not running. This command does not start a second supervisor.')
    token = token_path.read_text(encoding='utf-8').strip()
    target = path_file.read_text(encoding='utf-8').strip()
    if not token or not target:
        raise SupervisorUnavailable(
            'Client is not running. This command does not start a second supervisor.')
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    try:
        sock.settimeout(timeout)
        try:
            sock.connect(target)
        except OSError as exc:
            raise SupervisorUnavailable(
                'Client is not running. This command does not start a second supervisor.'
            ) from exc
        payload = json.dumps({'token': token, 'command': command, 'args': args or {}}) + '\n'
        sock.sendall(payload.encode('utf-8'))
        data = b''
        while b'\n' not in data:
            chunk = sock.recv(65536)
            if not chunk:
                break
            if len(data) + len(chunk) > 2_000_000:
                raise SupervisorUnavailable('Client response was too large')
            data += chunk
    finally:
        sock.close()
    if not data:
        raise SupervisorUnavailable(
            'Client is not running. This command does not start a second supervisor.')
    return json.loads(data.decode('utf-8'))


class CommandServer:
    def __init__(self, path, token, handler):
        self.path = Path(path)
        if self.path.exists():
            self.path.unlink()
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.bind(os.fspath(self.path))
        if os.name == 'posix':
            os.chmod(self.path, 0o600)
        self.sock.listen(16)
        self.sock.setblocking(False)
        self.token = token
        self.handler = handler

    def fileno(self):
        return self.sock.fileno()

    def accept_pending(self):
        while True:
            try:
                conn, _addr = self.sock.accept()
            except BlockingIOError:
                return
            try:
                self._handle(conn)
            finally:
                conn.close()

    def _handle(self, conn):
        conn.settimeout(5)
        data = b''
        try:
            while b'\n' not in data and len(data) < 1_000_000:
                chunk = conn.recv(65536)
                if not chunk:
                    break
                data += chunk
            response = self._dispatch(data)
            conn.sendall((json.dumps(response) + '\n').encode('utf-8'))
        except OSError:
            return

    def _dispatch(self, data):
        try:
            payload = json.loads(data.decode('utf-8').split('\n', 1)[0] or '{}')
        except (UnicodeDecodeError, json.JSONDecodeError):
            return {'ok': False, 'error': {'code': 'rejected', 'message': 'Command is not JSON'}}
        if not isinstance(payload, dict) or payload.get('token') != self.token:
            return {'ok': False, 'error': {'code': 'unauthorized', 'message': 'Bad client token'}}
        command = payload.get('command')
        args = payload.get('args') or {}
        try:
            result = self.handler(command, args)
        except RecoveryBlocked as exc:
            return {'ok': False, 'error': {'code': 'recovery_blocked', 'message': str(exc)}}
        except Rejected as exc:
            return {'ok': False, 'error': {'code': 'rejected', 'message': str(exc)}}
        except Exception as exc:  # noqa: BLE001 - one client call must not kill the supervisor
            return {'ok': False, 'error': {
                'code': 'error', 'message': '%s: %s' % (exc.__class__.__name__, exc)}}
        return {'ok': True, 'result': result}

    def close(self):
        try:
            self.sock.close()
        finally:
            if self.path.exists():
                try:
                    self.path.unlink()
                except OSError:
                    pass
