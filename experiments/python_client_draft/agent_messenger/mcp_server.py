"""Reply-only MCP for one execution token. It cannot claim, launch, or finish."""
from __future__ import annotations

import argparse
import json
import sys

from agent_messenger.errors import Rejected
from agent_messenger.store import Store


def invoke(store, token, name, args):
    if name == 'read_current' and args == {}:
        return store.current_message(token)
    if name == 'reply' and isinstance(args, dict) and set(args) == {'text'}:
        return store.reply(token, args['text'])
    raise Rejected('Only read_current() and reply(text) are allowed')


def serve(db_path, token):
    store = Store(db_path)
    tools = [
        {'name': 'read_current',
         'description': 'Read the message assigned to this execution.',
         'inputSchema': {'type': 'object', 'properties': {}, 'additionalProperties': False},
         'annotations': {'readOnlyHint': True, 'openWorldHint': False}},
        {'name': 'reply',
         'description': 'Reply to the assigned message. Routing is automatic.',
         'inputSchema': {'type': 'object', 'properties': {'text': {
             'type': 'string', 'minLength': 1, 'maxLength': 2000}},
             'required': ['text'], 'additionalProperties': False},
         'annotations': {'readOnlyHint': False, 'destructiveHint': False,
                         'idempotentHint': True, 'openWorldHint': False}},
    ]
    try:
        for line in sys.stdin:
            request = json.loads(line)
            if 'id' not in request:
                continue
            method = request.get('method')
            if method == 'initialize':
                result = {'protocolVersion': '2025-06-18', 'capabilities': {'tools': {}},
                          'serverInfo': {'name': 'agent-messenger', 'version': '0.1.0'}}
            elif method == 'ping':
                result = {}
            elif method == 'tools/list':
                result = {'tools': tools}
            elif method == 'tools/call':
                try:
                    params = request.get('params', {})
                    value = invoke(store, token, params.get('name'), params.get('arguments', {}))
                    result = {'content': [{'type': 'text', 'text': json.dumps(value)}], 'isError': False}
                except Rejected as error:
                    result = {'content': [{'type': 'text', 'text': str(error)}], 'isError': True}
            else:
                print(json.dumps({'jsonrpc': '2.0', 'id': request['id'],
                                  'error': {'code': -32601, 'message': 'Method not found'}}), flush=True)
                continue
            print(json.dumps({'jsonrpc': '2.0', 'id': request['id'], 'result': result}), flush=True)
    finally:
        store.close()


def main(argv=None):
    parser = argparse.ArgumentParser(description='Bound reply MCP for one execution token.')
    parser.add_argument('--db', required=True)
    parser.add_argument('--token', required=True)
    args = parser.parse_args(argv)
    serve(args.db, args.token)


if __name__ == '__main__':
    main()
