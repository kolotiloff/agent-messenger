"""Check the signed candidate on a native runner, using the previous release's updater."""
import hashlib
import json
import os
from pathlib import Path
import platform
from queue import Empty, Queue
import re
import shutil
import subprocess
import sys
import tempfile
from threading import Thread
import time
import zipfile


def run(*args, **kwargs):
    return subprocess.check_output(args, encoding="utf-8", timeout=180, **kwargs).strip()


def probe_coordinator(program, root, folder, version, env):
    """Initialize the real stdio relay and read the installed project's agents."""
    env = {key: value for key, value in env.items() if key != "AGENT_MESSENGER_WORKER_RUN"}
    incoming = Queue()
    with subprocess.Popen([program, "--root", root, "coordinator", "--project", "bridge"],
                          env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                          encoding="utf-8") as process:
        def collect_responses():
            for line in process.stdout:
                incoming.put(line)
            incoming.put(None)

        reader = Thread(target=collect_responses, daemon=True)
        reader.start()

        def send(method, params, request_id=None):
            message = dict(jsonrpc="2.0", method=method, params=params)
            if request_id is not None:
                message["id"] = request_id
            process.stdin.write(json.dumps(message) + "\n")
            process.stdin.flush()

        def request(request_id, method, params):
            send(method, params, request_id)
            deadline = time.monotonic() + 20
            while time.monotonic() < deadline:
                try:
                    line = incoming.get(timeout=max(0, deadline - time.monotonic()))
                except Empty:
                    break
                assert line is not None, f"coordinator closed stdout during {method}"
                reply = json.loads(line)
                if reply.get("id") == request_id:
                    assert "error" not in reply, f"coordinator rejected {method}: {reply['error']}"
                    return reply["result"]
            raise RuntimeError(f"coordinator timed out during {method}")

        try:
            initialized = request(1, "initialize", dict(protocolVersion="2025-06-18",
                                                         capabilities={},
                                                         clientInfo=dict(name="bridge-test", version="1")))
            assert initialized["protocolVersion"] == "2025-06-18"
            send("notifications/initialized", {})
            tools = [tool["name"] for tool in request(2, "tools/list", {})["tools"]]
            assert len(tools) == len(set(tools)) and "list_agents" in tools
            result = request(3, "tools/call", dict(name="list_agents", arguments={}))
            assert not result.get("isError"), result
            roster = json.loads(result["content"][0]["text"])["computers"]
            local, = (computer for computer in roster if computer.get("this"))
            assert Path(local["folder"]).resolve() == folder.resolve()
            assert local["messenger_version"] == version
            return initialized["serverInfo"]["name"], local["computer"]
        finally:
            process.stdin.close()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=10)
                raise
            finally:
                reader.join(timeout=5)
            assert process.returncode == 0, "coordinator did not exit cleanly"


repo = Path.cwd()
target = os.environ["BRIDGE_TARGET"]
version = os.environ["VERSION"]
previous_tag = os.environ["PREVIOUS_TAG"]
previous = previous_tag.removeprefix("v")
key = (repo / "desktop/macos/update-public-key.txt").read_text().strip()
system, machine = target.split("-")
ext = ".exe" if system == "windows" else ""
assert run("go", "env", "GOOS") == system and run("go", "env", "GOARCH") == machine
base = Path.home() / "AgentMessengerTest"
base.mkdir(exist_ok=True)
work = Path(tempfile.mkdtemp(prefix="bridge-", suffix=".noindex" if system == "darwin" else "", dir=base))
verify = work / ("verify" + ext)
run("go", "build", "-o", verify, "./scripts/verify-update-signature")


def package(tag):
    folder = work / tag
    folder.mkdir()
    name = "update-" + target + ".json"

    def download(asset):
        run("gh", "release", "download", tag, "--repo", os.environ["GITHUB_REPOSITORY"],
            "--pattern", asset, "--dir", folder)
        return folder / asset

    manifest = download(name)
    signature = download(name + ".sig")
    run(verify, key, manifest, signature.read_text().strip())
    data = json.loads(manifest.read_bytes())
    assert data["version"] == tag[1:] and data["os"] == system and data["arch"] == machine
    assert re.fullmatch(r"[0-9a-f]{64}", data["sha256"])
    archive = download(data["sha256"] + ".zip")
    assert archive.stat().st_size == data["length"]
    assert hashlib.sha256(archive.read_bytes()).hexdigest() == data["sha256"]
    run(verify, key, archive, data["signature"])
    manifest.rename(folder / "manifest.json")
    signature.rename(folder / "manifest.sig")
    return folder, archive, data


def unpack(archive, dest):
    if system == "darwin":
        run("ditto", "-x", "-k", archive, dest)
        bundles = list(dest.glob("*.app"))
        assert len(bundles) == 1 and bundles[0].name in ("Agent Messenger.app", "Kotel.app")
        app = bundles[0]
        run("codesign", "--verify", "--deep", "--strict", app)
        return app / "Contents/Resources/messenger"
    exe = dest / ("messenger" + ext)
    with zipfile.ZipFile(archive) as z:
        assert z.namelist() == [exe.name]
        exe.write_bytes(z.read(exe.name))
    exe.chmod(0o755)
    return exe


_, old_zip, old_meta = package(previous_tag)
channel, new_zip, new_meta = package("v" + version)
assert new_meta["revision"] == os.environ["CANDIDATE_REVISION"]
assert new_meta["build"] > old_meta["build"]
# All network access is complete. The packaged programs receive no GitHub token.
os.environ.pop("GH_TOKEN", None)
source = work / "previous"
run("git", "worktree", "add", "--detach", source, old_meta["revision"])
driver = source / "scripts/bridge-check"
driver.mkdir()
shutil.copyfile(repo / "scripts/check-signed-install/main.go", driver / "main.go")
install = work / ("install-check" + ext)
run("go", "build", "-o", install, "./scripts/bridge-check", cwd=source)
dest = work / "installed"
dest.mkdir()
exe = unpack(old_zip, dest)
assert json.loads(run(exe, "version"))["version"] == previous

home = work / "home"
home.mkdir()
env = dict(os.environ, HOME=str(home), USERPROFILE=str(home), LOCALAPPDATA=str(home / "local"),
           APPDATA=str(home / "roaming"), XDG_DATA_HOME=str(home / "xdg"), CFFIXED_USER_HOME=str(home))
root = work / "db"


def client(*args):
    return run(exe, "--root", root, *args, env=env)


def wait_ready():
    for _ in range(150):
        try:
            return json.loads(client("inspect"))
        except (subprocess.CalledProcessError, ValueError):
            time.sleep(0.1)
    raise RuntimeError("isolated client did not become ready")


def stop():
    client("shutdown")
    for _ in range(150):
        if subprocess.run([exe, "--root", root, "status"], env=env, stdout=subprocess.DEVNULL,
                          stderr=subprocess.DEVNULL, timeout=10).returncode:
            return
        time.sleep(0.1)
    raise RuntimeError("isolated client did not stop")


client("project-add", "--name", "bridge", "--folder", work)
_, previous_computer = probe_coordinator(exe, root, work, previous, env)
client("start")
try:
    wait_ready()
    fake = Path(os.environ["SystemRoot"]) / "System32/where.exe" if system == "windows" else "/usr/bin/false"
    client("register-session", "--provider", "claude", "--environment", home, "--conversation-id", "synthetic-bridge",
           "--alias", "bridge", "--cli", fake, "--workspace", work)
    client("enqueue", "--alias", "bridge", "--id", "bridge-one", "--from", "test", "--text", "Retain this message")
    before = json.loads(client("inspect"))
    assert len(before["Requests"]) == 1 and not before["Attempts"]
finally:
    stop()
run(install, channel, key, exe, previous, version, env=env)
if system == "darwin":
    # Sparkle installs a whole bundle. Overlaying an archive would retain
    # resources removed in this version and invalidate the code signature.
    staged = work / "staged"
    staged.mkdir()
    candidate = unpack(new_zip, staged).parents[2]
    existing = exe.parents[2]
    existing.rename(work / "previous.app")
    candidate.rename(existing)
    run("codesign", "--verify", "--deep", "--strict", existing)
    assert (existing / "Contents/Resources/kotel").samefile(exe)
assert json.loads(run(exe, "version"))["revision"] == new_meta["revision"]
name, computer = probe_coordinator(exe, root, work, version, env)
assert name == "kotel" and computer == previous_computer
if system == "darwin":
    assert probe_coordinator(exe.with_name("kotel"), root, work, version, env) == (name, computer)
client("start")
try:
    after = wait_ready()
    # Starting a paused client records a new pause event. Every original
    # event and every row of the durable queue must still be present.
    old_events = before.pop("Events")
    new_events = after.pop("Events")
    assert new_events[:len(old_events)] == old_events
    assert all(row["kind"] == "pause" for row in new_events[len(old_events):])
    assert after == before
finally:
    stop()
print(f"{platform.system()} {target}: {previous} -> {version}, signed package, queue and coordinator verified")

if system in ("linux", "darwin"):
    remote = work / "net"
    remote.mkdir()
    installed = remote / "installed"
    installed.mkdir()
    old_program = unpack(old_zip, installed)
    subprocess.run([sys.executable, repo / "scripts/check-remote-bridge.py", remote, old_program,
                    exe, channel, previous, version], check=True, timeout=400)
    print(f"{target}: remote update reply and device identity verified")
