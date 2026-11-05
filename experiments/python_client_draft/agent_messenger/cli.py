"""Command line for the single local client. Commands other than serve only connect."""
from __future__ import annotations

import argparse
import json
import sys

from agent_messenger.behavior import PAUSE, SHUTDOWN, STOP_CURRENT, UNPAUSE
from agent_messenger.config import ConfigError
from agent_messenger.errors import Rejected, SupervisorBusy, SupervisorUnavailable
from agent_messenger.ipc import call
from agent_messenger.paths import default_data_dir
from agent_messenger.supervisor import Supervisor


def build_parser():
    parser = argparse.ArgumentParser(
        prog='agent-messenger',
        description=(
            'Local Agent Messenger client. One supervisor owns the queue and '
            'CLI launches. Status and idle waiting do not call a model. '
            'A network broker is not started.'
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='\n\n'.join((
            'pause: ' + PAUSE,
            'unpause: ' + UNPAUSE,
            'stop-current: ' + STOP_CURRENT,
            'shutdown: ' + SHUTDOWN,
        )),
    )
    parser.add_argument(
        '--data-dir',
        help='Canonical store directory. Default is the per-user data directory, not a per-run database.',
    )
    commands = parser.add_subparsers(dest='command', required=True)

    serve = commands.add_parser('serve', help='Run the single supervisor until shutdown.')
    serve.add_argument(
        '--allow-synthetic', action='store_true',
        help='Allow the built-in synthetic worker. This does not allow Claude, Codex, or Cursor.',
    )
    serve.add_argument(
        '--allow-model-calls', action='store_true',
        help=(
            'Allow real Claude, Codex, and Cursor launches. Also requires '
            'allow_model_calls true in the store config. This spends tokens. '
            'The default test suite does not pass this flag.'
        ),
    )

    commands.add_parser('status', help='Show queue and active attempts. Does not launch a process or call a model.')
    commands.add_parser('inspect', help='Show local sessions, queue, attempts, and the audit tail. Does not call a model.')
    commands.add_parser('pause', help=PAUSE)
    commands.add_parser('unpause', help=UNPAUSE)
    commands.add_parser('stop-current', help=STOP_CURRENT).add_argument('--alias')
    commands.add_parser('shutdown', help=SHUTDOWN)

    enqueue = commands.add_parser(
        'enqueue',
        help='Append one message through the running client. Does not call a model. '
             'The same id and content is kept once; the same id with different content is rejected.',
    )
    enqueue.add_argument('--message-id', required=True)
    enqueue.add_argument('--alias', required=True)
    enqueue.add_argument('--sender', required=True)
    enqueue.add_argument('--text', required=True)

    register = commands.add_parser(
        'register-session',
        help='Bind a mail alias to a real CLI environment directory and an exact conversation id. '
             'A second alias for the same environment and id shares one lock.',
    )
    register.add_argument('--provider', required=True, choices=('synthetic', 'claude', 'codex', 'cursor'))
    register.add_argument('--environment', required=True)
    register.add_argument('--conversation-id', required=True)
    register.add_argument('--alias', required=True)
    register.add_argument('--workspace')
    register.add_argument('--cli')

    accept = commands.add_parser(
        'accept-verified',
        help='Ask to accept a verified result. A stored reply is not verification. '
             'This version refuses: process-tree exit proof is not implemented, and nothing is unlocked.',
    )
    accept.add_argument('--alias', required=True)
    accept.add_argument('--verification', required=True)

    failed = commands.add_parser(
        'fail-closed',
        help='Close an uncertain attempt without retry and keep history. '
             'Does not kill a process, does not declare it stopped, and does not release the session.',
    )
    failed.add_argument('--alias', required=True)
    failed.add_argument('--reason', required=True)

    retry = commands.add_parser(
        'authorize-attempt',
        help='Ask for a separate new attempt. Actions may be repeated. '
             'This version refuses because process-tree exit proof is not implemented. '
             'Pass --acknowledgement actions-may-repeat. No attempt is created.',
    )
    retry.add_argument('--alias', required=True)
    retry.add_argument('--reason', required=True)
    retry.add_argument('--acknowledgement', required=True)
    return parser


def _data_dir(value):
    if value:
        return value
    return default_data_dir()


def _payload(args):
    command = args.command
    if command == 'stop-current':
        payload = {}
        if args.alias:
            payload['alias'] = args.alias
        return payload
    if command == 'enqueue':
        return {
            'message_id': args.message_id,
            'alias': args.alias,
            'sender': args.sender,
            'text': args.text,
        }
    if command == 'register-session':
        payload = {
            'provider': args.provider,
            'environment': args.environment,
            'conversation_id': args.conversation_id,
            'alias': args.alias,
        }
        if args.workspace:
            payload['workspace'] = args.workspace
        if args.cli:
            payload['cli'] = args.cli
        return payload
    if command == 'accept-verified':
        return {'alias': args.alias, 'verification': args.verification}
    if command == 'fail-closed':
        return {'alias': args.alias, 'reason': args.reason}
    if command == 'authorize-attempt':
        return {
            'alias': args.alias,
            'reason': args.reason,
            'acknowledgement': args.acknowledgement,
        }
    return {}


def emit(payload, ok=True):
    json.dump(payload, sys.stdout, indent=2, sort_keys=True, ensure_ascii=False)
    sys.stdout.write('\n')
    sys.stdout.flush()
    return 0 if ok else 2


def main(argv=None):
    args = build_parser().parse_args(argv)
    try:
        data = _data_dir(args.data_dir)
    except Rejected as exc:
        sys.exit(emit({'ok': False, 'error': {'code': 'rejected', 'message': str(exc)}}, ok=False))
    if args.command == 'serve':
        try:
            Supervisor(
                data,
                allow_synthetic=args.allow_synthetic,
                allow_model_calls=args.allow_model_calls,
            ).serve()
        except SupervisorBusy as exc:
            sys.exit(emit({'ok': False, 'error': {'code': 'supervisor_busy', 'message': str(exc)}}, ok=False))
        except ConfigError as exc:
            sys.exit(emit({'ok': False, 'error': {'code': 'config', 'message': str(exc)}}, ok=False))
        return
    try:
        response = call(data, args.command, _payload(args))
    except SupervisorUnavailable as exc:
        sys.exit(emit({'ok': False, 'error': {'code': 'not_running', 'message': str(exc)}}, ok=False))
    sys.exit(emit(response, ok=bool(response.get('ok'))))


if __name__ == '__main__':
    main()
