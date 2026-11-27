"""Request the signed update over a private two-computer broker on a native POSIX runner."""
from contextlib import ExitStack, closing
import json
import os
from pathlib import Path
import select
import shutil
import socket
import subprocess
import sys
import time


class Node:
    def __init__(self, home, program, stack):
        home.mkdir()
        self.root = home / "db"
        self.env = dict(os.environ, HOME=str(home), CFFIXED_USER_HOME=str(home),
                        XDG_DATA_HOME=str(home / "xdg"), HTTPS_PROXY="http://127.0.0.1:1",
                        NO_PROXY="127.0.0.1,localhost")
        self.program = program
        self.stack = stack

    def start(self):
        # Opening once records the real installation path for the remote helper.
        with subprocess.Popen([self.program, "--root", self.root, "desktop"], env=self.env,
                              stdin=subprocess.PIPE, stdout=subprocess.PIPE) as window:
            self.stack.callback(self.stop)
            try:
                assert select.select([window.stdout], [], [], 40)[0], "fixture window timed out"
                assert window.stdout.readline().startswith(b"http://127.0.0.1:")
            finally:
                window.stdin.close()
                window.wait(timeout=15)
            assert window.returncode == 0

    def stop(self):
        subprocess.run([self.program, "--root", self.root, "shutdown"], env=self.env,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=20)

    def call(self, command, **fields):
        with closing(socket.socket(socket.AF_UNIX)) as conn:
            conn.settimeout(25)
            conn.connect(str(self.root / "control.sock"))
            conn.sendall((json.dumps(dict(cmd=command, **fields)) + "\n").encode())
            with conn.makefile() as incoming:
                reply = json.loads(incoming.readline())
            if not reply["ok"]:
                raise ValueError(reply.get("error", "fixture request was rejected"))
            return reply

    def server(self, action, **body):
        return self.call("server", action=action, text=json.dumps(body))["overview"]


def run_bridge(work, old, new, channel, previous, version):
    with ExitStack() as stack:
        host = Node(work / "host", new, stack)
        worker = Node(work / "worker", old, stack)
        if sys.platform.startswith("linux"):
            # The draft is intentionally not publicly discoverable. Use exactly the
            # verified archive and manifest the old updater would have downloaded.
            cached = Path(worker.env["XDG_DATA_HOME"]) / "agent-messenger/updates"
            shutil.copytree(channel, cached)
        host.start()
        worker.start()
        assert worker.call("status")["status"]["version"] == previous
        with closing(socket.socket()) as port:
            port.bind(("127.0.0.1", 0))
            number = port.getsockname()[1]
        broker = host.server("enable", scope="local", port=number)["host"]
        address, = broker["addresses"]
        assert "127.0.0.1" in address
        invitation = host.server("invite", address=address)["code"]
        member = worker.server("connect", code=invitation, name="Bridge fixture")["member"]
        device = member["device_id"]
        # A broker can see a new member a tick before the supervisor knows it.
        for _ in range(90):
            try:
                requested = host.server("update", device=device)
                break
            except ValueError as error:
                waiting = ("этот компьютер не в сети", "не на связи", "такого компьютера нет", "программа старше")
                if not any(reason in str(error) for reason in waiting):
                    raise
                time.sleep(1)
        else:
            raise RuntimeError("worker never advertised an updatable version")
        identifier, = [r["request_id"] for r in requested["updates"] if r["computer"] == device]
        for _ in range(180):
            rows = host.server("status").get("updates") or []
            result = next((r for r in rows if r["request_id"] == identifier), {})
            if result.get("state") in ("refused", "failed", "done", "cancelled"):
                break
            time.sleep(1)
        else:
            raise RuntimeError("no final reply to the remote request")
        if sys.platform == "darwin":
            assert result["state"] == "refused" and "Mac" in result["text"], result
            expected = previous
        else:
            assert result["state"] == "done" and result["version"] == version, result
            assert json.loads(subprocess.check_output([old, "version"], env=worker.env))["version"] == version
            expected = version
        assert worker.call("status")["status"]["version"] == expected
        again = worker.server("check")["member"]
        assert again["reachable"] and again["device_id"] == device and again["broker_id"] == broker["broker_id"]
        saved = host.server("status")["host"]
        assert saved["fingerprint"] == broker["fingerprint"]
        assert sum(d["id"] == device for d in saved["devices"]) == 1
        print("Remote request verified:", result["state"], "running version", expected, flush=True)


if __name__ == "__main__":
    run_bridge(*(Path(x) for x in sys.argv[1:5]), *sys.argv[5:])
