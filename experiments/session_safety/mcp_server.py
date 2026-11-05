"""Reply-only MCP for ONE trusted controller-selected execution context."""
import argparse
import json
import sys
from runtime import Store, Rejected


def serve(db_path, token):
    store = Store(db_path)
    tools = [
        {'name': 'read_current', 'description': 'Read the message assigned to this execution.',
         'inputSchema': {'type': 'object', 'properties': {}, 'additionalProperties': False},
         'annotations': {'readOnlyHint': True, 'openWorldHint': False}},
        {'name': 'reply', 'description': 'Reply to the assigned message. Routing is automatic.',
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
                          'serverInfo': {'name': 'bound-reply-experiment', 'version': '0.1.0'}}
            elif method == 'ping':
                result = {}
            elif method == 'tools/list':
                result = {'tools': tools}
            elif method == 'tools/call':
                try:
                    params = request.get('params', {})
                    name, args = params.get('name'), params.get('arguments', {})
                    if name == 'read_current' and args == {}:
                        value = store.current_message(token)
                    elif name == 'reply' and isinstance(args, dict) and set(args) == {'text'}:
                        value = store.reply(token, args['text'])
                    else:
                        raise Rejected('Only read_current() and reply(text) are allowed')
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


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--db', required=True)
    parser.add_argument('--token', required=True)
    args = parser.parse_args()
    serve(args.db, args.token)
