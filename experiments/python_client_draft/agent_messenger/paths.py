"""Canonical data directory. Tests pass an explicit directory instead."""
from __future__ import annotations

import os
from pathlib import Path
import sys

from agent_messenger.errors import Rejected


def default_data_dir():
    """One store per user. This is not a per-run database."""
    if sys.platform == 'darwin':
        return Path.home() / 'Library' / 'Application Support' / 'AgentMessenger'
    if os.name == 'nt':
        appdata = os.environ.get('APPDATA')
        if not appdata:
            raise Rejected('APPDATA is not set; pass --data-dir')
        return Path(appdata) / 'AgentMessenger'
    base = os.environ.get('XDG_DATA_HOME')
    if base:
        return Path(base).expanduser() / 'agent-messenger'
    return Path.home() / '.local' / 'share' / 'agent-messenger'


def prepare_data_dir(path):
    directory = Path(path).expanduser()
    directory.mkdir(parents=True, mode=0o700, exist_ok=True)
    if os.name == 'posix':
        os.chmod(directory, 0o700)
    return directory.resolve()
