"""Synthetic processes used ONLY by test_safety.py. Never launches an agent."""
import json
import os
from pathlib import Path
import subprocess
import sys
import time
from runtime import Store


def wait_file(path, timeout=15):
    until = time.monotonic() + timeout
    while not path.exists():
        if time.monotonic() > until:
            raise TimeoutError(str(path))
        time.sleep(0.01)


def rpc(db, token, calls):
    requests = [{'jsonrpc': '2.0', 'id': 0, 'method': 'initialize', 'params': {
        'protocolVersion': '2025-06-18', 'capabilities': {},
        'clientInfo': {'name': 'synthetic-test', 'version': '1'}}},
        {'jsonrpc': '2.0', 'method': 'notifications/initialized'}]
    requests += [{'jsonrpc': '2.0', 'id': i+1, 'method': method, 'params': params}
                 for i, (method, params) in enumerate(calls)]
    process = subprocess.run([sys.executable, str(Path(__file__).with_name('mcp_server.py')),
                              '--db', str(db), '--token', token],
                             input=''.join(json.dumps(r)+'\n' for r in requests),
                             capture_output=True, text=True, timeout=10, check=True)
    return [json.loads(line)['result'] for line in process.stdout.splitlines()][1:]


def tool(name, args):
    return ('tools/call', {'name': name, 'arguments': args})


def main():
    mode, db, key, directory = sys.argv[1:5]
    directory = Path(directory)
    if mode == 'worker':
        token = key
        directory.joinpath('child-started').write_text(str(os.getpid()))
        results = rpc(db, token, [tool('read_current', {}), tool('reply', {'text': 'synthetic reply'})])
        if any(r.get('isError') for r in results):
            raise RuntimeError('Worker MCP failed')
        directory.joinpath('replied').touch()
        end = time.monotonic() + 15
        beat = 0
        while not directory.joinpath('release').exists():
            if time.monotonic() > end:
                raise TimeoutError('Synthetic child safety timeout')
            beat += 1
            directory.joinpath('heartbeat').write_text(str(beat))
            time.sleep(0.02)
        directory.joinpath('child-exited').touch()
        return

    store = Store(db)
    try:
        if mode == 'claim':
            directory.joinpath('ready').touch()
            wait_file(directory.parent/'barrier')
            directory.joinpath('result.json').write_text(json.dumps(store.claim(key)))
            return
        token = store.claim(key)
        if token is None:
            directory.joinpath('blocked').touch()
            return
        directory.joinpath('token').write_text(token)
        if mode == 'crash-before-spawn':
            directory.joinpath('paused').touch()
            wait_file(directory/'release')
            return
        store.begin_launch(token)
        if mode == 'crash-after-intent':
            directory.joinpath('paused').touch()
            wait_file(directory/'release')
            return
        process = subprocess.Popen([sys.executable, __file__, 'worker', db, token, str(directory)],
                                   stdin=subprocess.DEVNULL)
        if mode == 'crash-before-pid':
            wait_file(directory/'replied')
            directory.joinpath('paused').touch()
            wait_file(directory/'release')
        store.record_pid(token, process.pid)
        code = process.wait(timeout=20)
        if mode == 'crash-before-finish':
            directory.joinpath('paused').touch()
            wait_file(directory/'finish-release')
        state = store.finish_after_wait(token, code)
        directory.joinpath('finished').write_text(state)
    finally:
        store.close()


if __name__ == '__main__':
    main()
