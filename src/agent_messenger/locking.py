"""One supervisor per canonical store, using an OS file lock in addition to SQLite."""
from __future__ import annotations

import os

from agent_messenger.errors import SupervisorBusy


class SupervisorLock:
    """Exclusive non-blocking lock. A second supervisor fails before it launches."""

    def __init__(self, path):
        self.path = path
        self._file = None
        self._fd = os.open(os.fspath(path), os.O_CREAT | os.O_RDWR, 0o600)
        try:
            if os.name == 'posix':
                os.chmod(path, 0o600)
            self._acquire()
        except BaseException:
            os.close(self._fd)
            self._fd = None
            raise

    def _acquire(self):
        try:
            if os.name == 'nt':
                import msvcrt
                self._file = os.fdopen(self._fd, 'r+')
                self._file.seek(0)
                msvcrt.locking(self._file.fileno(), msvcrt.LK_NBLCK, 1)
            else:
                import fcntl
                fcntl.flock(self._fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError as exc:
            raise SupervisorBusy(
                'Another supervisor holds the canonical store lock. '
                'Commands must talk to that client; a second supervisor will not start.'
            ) from exc

    def release(self):
        fd = self._fd
        if fd is None:
            return
        self._fd = None
        try:
            if os.name == 'nt':
                import msvcrt
                if self._file is not None:
                    self._file.seek(0)
                    try:
                        msvcrt.locking(self._file.fileno(), msvcrt.LK_UNLCK, 1)
                    except OSError:
                        pass
                    self._file.close()
                    self._file = None
            else:
                import fcntl
                fcntl.flock(fd, fcntl.LOCK_UN)
                os.close(fd)
        except OSError:
            pass
