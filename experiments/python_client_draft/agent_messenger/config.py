"""Local configuration. Broker modes are reserved and refused."""
from __future__ import annotations

import json
import os
from pathlib import Path

from agent_messenger.errors import ConfigError

DEFAULT_CONFIG = {
    'version': 1,
    'broker': {
        'mode': 'local_only',
        'connect_url': None,
        'broker_id': None,
    },
    'allow_model_calls': False,
}


def load_config(path):
    """Return local_only config. Never opens a network connection."""
    file = Path(path)
    if not file.exists():
        file.write_text(json.dumps(DEFAULT_CONFIG, indent=2) + '\n', encoding='utf-8')
        if os.name == 'posix':
            os.chmod(file, 0o600)
    try:
        loaded = json.loads(file.read_text(encoding='utf-8'))
    except (OSError, json.JSONDecodeError) as exc:
        raise ConfigError('Client config is not readable JSON') from exc
    if not isinstance(loaded, dict) or not isinstance(loaded.get('broker'), dict):
        raise ConfigError('Client config must contain a broker object')
    mode = loaded['broker'].get('mode')
    if mode != 'local_only':
        raise ConfigError(
            'Broker mode %r is not implemented. The client will not start a broker '
            'or connect to one. The only accepted mode is local_only.' % (mode,))
    allow = loaded.get('allow_model_calls', False)
    if not isinstance(allow, bool):
        raise ConfigError('allow_model_calls must be a boolean')
    return {
        'version': 1,
        'broker': {
            'mode': 'local_only',
            'connect_url': loaded['broker'].get('connect_url'),
            'broker_id': loaded['broker'].get('broker_id'),
        },
        'allow_model_calls': allow,
    }
