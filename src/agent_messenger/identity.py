"""Session identity is a real CLI environment, not a mail label."""
from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import re

from agent_messenger.errors import Rejected

PROVIDERS = ('synthetic', 'claude', 'codex', 'cursor')
REAL_PROVIDERS = ('claude', 'codex', 'cursor')
FORBIDDEN_CONVERSATION_IDS = frozenset({
    'last', 'latest', 'continue', 'new', 'current',
    '--last', '--continue', '--resume', '--last-session',
})
_ALIAS = re.compile(r'^[A-Za-z0-9][A-Za-z0-9._+/-]{0,127}$')


def check_provider(provider):
    if not isinstance(provider, str):
        raise Rejected('Provider is required')
    provider = provider.strip().lower()
    if provider not in PROVIDERS:
        raise Rejected('Provider must be synthetic, claude, codex, or cursor')
    return provider


def check_conversation_id(conversation_id):
    """Only an exact persisted id. Never last, continue, or latest."""
    if not isinstance(conversation_id, str) or not conversation_id or conversation_id.strip() != conversation_id:
        raise Rejected('Exact conversation id is required')
    if conversation_id.lower() in FORBIDDEN_CONVERSATION_IDS or conversation_id.startswith('-'):
        raise Rejected('Exact conversation id is required; last/continue selectors are refused')
    if any(char in conversation_id for char in '\r\n\x00'):
        raise Rejected('Exact conversation id is required')
    if len(conversation_id) > 200:
        raise Rejected('Conversation id is too long')
    return conversation_id


def check_alias(alias):
    if not isinstance(alias, str) or not _ALIAS.fullmatch(alias):
        raise Rejected('Mail alias must be a short label, not an environment path')
    return alias


def check_sender(sender):
    if not isinstance(sender, str) or not sender.strip() or len(sender) > 200:
        raise Rejected('Sender is required')
    if any(char in sender for char in '\r\n\x00'):
        raise Rejected('Sender is required')
    return sender


def check_body(text, limit, label):
    if not isinstance(text, str) or not 1 <= len(text) <= limit or '\x00' in text:
        raise Rejected('%s must contain 1..%d characters' % (label, limit))
    return text


def _stored_name(directory, part):
    """Prefer the directory entry's own spelling so two labels share one path."""
    exact = None
    folded = None
    try:
        with os.scandir(directory) as entries:
            for entry in entries:
                if entry.name == part:
                    exact = entry.name
                    break
                if folded is None and entry.name.lower() == part.lower():
                    folded = entry.name
    except OSError:
        return part
    return exact or folded or part


def normalize_case(path):
    parts = Path(path).parts
    if not parts:
        return Path(path)
    current = Path(parts[0])
    for part in parts[1:]:
        current = current / _stored_name(current, part)
    return current


def canonical_directory(path):
    """Resolved existing directory. Symlinks and spelling variants collapse."""
    if not isinstance(path, str) or not path.strip():
        raise Rejected('A real environment directory is required')
    expanded = os.path.expanduser(path.strip())
    if not os.path.isabs(expanded):
        raise Rejected('Environment path must be an existing absolute directory')
    if not os.path.isdir(expanded):
        raise Rejected('Environment path must be an existing absolute directory')
    resolved = os.path.realpath(expanded)
    if not os.path.isdir(resolved):
        raise Rejected('Environment path must be an existing absolute directory')
    return str(normalize_case(resolved))


def canonical_executable(path):
    if not isinstance(path, str) or not path.strip():
        raise Rejected('CLI path must be an existing executable file')
    expanded = os.path.expanduser(path.strip())
    if not os.path.isabs(expanded):
        raise Rejected('CLI path must be an existing executable file')
    if not os.path.isfile(expanded):
        raise Rejected('CLI path must be an existing executable file')
    resolved = os.path.realpath(expanded)
    if not os.path.isfile(resolved):
        raise Rejected('CLI path must be an existing executable file')
    if os.name == 'posix' and not os.access(resolved, os.X_OK):
        raise Rejected('CLI path must be an existing executable file')
    return str(normalize_case(resolved))


def session_key(provider, environment_id, conversation_id):
    """Key of (provider, canonical environment, exact conversation id).

    ``environment_id`` must already be the canonical directory. Mail aliases
    are not part of the key.
    """
    provider = check_provider(provider)
    conversation_id = check_conversation_id(conversation_id)
    if not isinstance(environment_id, str) or not environment_id or not os.path.isabs(environment_id):
        raise Rejected('Canonical environment is required')
    payload = json.dumps([provider, environment_id, conversation_id], separators=(',', ':'))
    return hashlib.sha256(payload.encode('utf-8')).hexdigest()
