#!/usr/bin/env python3
"""Check app-server config merging without creating a thread or calling a model.

Uses only a temporary HOME/CODEX_HOME, synthetic MCP config and config/read.
Does not import credentials or connect to the user's app-server daemon.
"""

import argparse
import json
import os
from pathlib import Path
import queue
import subprocess
import tempfile
import threading
import time


DISABLED_FEATURES = (
    "shell_tool", "apps", "plugins", "hooks", "multi_agent", "browser_use",
    "computer_use", "goals",
)


def probe(cli, case, fixture, override):
    with tempfile.TemporaryDirectory(prefix="messenger-codex-config-") as tmp:
        root = Path(tmp)
        home = root / "home"
        profile = home / ".codex"
        workspace = root / "workspace"
        profile.mkdir(parents=True)
        workspace.mkdir()
        if fixture:
            # This executable must never be started: config/read only reads config.
            (profile / "config.toml").write_text(
                '[mcp_servers.fixture]\ncommand = "messenger-fixture-never-run"\n',
                encoding="utf-8",
            )
        env = {"HOME": str(home), "CODEX_HOME": str(profile),
               "PATH": os.defpath, "TMPDIR": str(root)}
        args = [str(cli), "app-server", "--listen", "stdio://",
                "-c", 'cli_auth_credentials_store="file"']
        for feature in DISABLED_FEATURES:
            args.extend(["--disable", feature])
        if override:
            args.extend(["-c", override])
        # Stderr is discarded: it may include platform paths. No logs are published.
        process = subprocess.Popen(args, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                   stderr=subprocess.DEVNULL, cwd=workspace, env=env,
                                   text=True, encoding="utf-8")
        received = queue.Queue()

        def reader():
            try:
                for line in process.stdout:
                    received.put(json.loads(line))
            except Exception as error:
                received.put(error)
            finally:
                received.put(EOFError("app-server closed stdout"))

        worker = threading.Thread(target=reader, daemon=True)
        worker.start()

        def send(payload):
            # Fail closed if a future edit accidentally tries to start a model turn.
            if payload.get("method") not in ("initialize", "initialized", "config/read"):
                raise RuntimeError("method is outside the model-free probe allowlist")
            process.stdin.write(json.dumps(payload) + "\n")
            process.stdin.flush()

        def request(request_id, method, params):
            send({"id": request_id, "method": method, "params": params})
            deadline = time.monotonic() + 15
            while True:
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    raise TimeoutError("app-server response deadline expired")
                item = received.get(timeout=remaining)
                if isinstance(item, Exception):
                    raise item
                if "id" not in item:
                    continue  # A notification is not a response.
                if "method" in item or item["id"] != request_id:
                    raise RuntimeError("unexpected server request or response ID")
                if "error" in item or "result" not in item:
                    raise RuntimeError("app-server refused a probe request")
                return item["result"]

        try:
            request(1, "initialize", {
                "clientInfo": {"name": "messenger_config_probe", "version": "0.1"},
                "capabilities": {"experimentalApi": True},
            })
            send({"method": "initialized"})
            result = request(2, "config/read", {"includeLayers": False, "cwd": str(workspace)})
            config = result["config"]
            servers = config.get("mcp_servers") or {}
            features = config.get("features") or {}
            if any(features.get(feature) is not False for feature in DISABLED_FEATURES):
                raise RuntimeError("a requested feature disable was not confirmed")
            active = sorted(name for name, server in servers.items()
                            if server.get("enabled", True) is not False)
            # Only fixture names and booleans leave this temporary environment.
            if set(servers) - {"fixture"}:
                raise RuntimeError("unexpected inherited server; isolation not established")
            expected_active = ["fixture"] if case == "empty-table" else []
            if active != expected_active:
                raise RuntimeError("config merge behavior changed; review adapter assumptions")
            return {"case": case, "fixture_present": "fixture" in servers,
                    "fixture_enabled": "fixture" in active,
                    "requested_features_disabled": True, "model_turns": 0}
        finally:
            process.stdin.close()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)
            worker.join(timeout=2)
            process.stdout.close()
            if process.returncode != 0:
                raise RuntimeError("probe app-server did not exit cleanly")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cli", required=True, type=Path, help="absolute path to Codex CLI")
    args = parser.parse_args()
    if not args.cli.is_absolute() or not args.cli.is_file():
        parser.error("--cli must be an existing absolute executable path")
    results = [probe(args.cli, "empty-table", True, "mcp_servers={}"),
               probe(args.cli, "explicit-disable", True, "mcp_servers.fixture.enabled=false"),
               probe(args.cli, "clean-profile", False, None)]
    print(json.dumps({"checks": results, "model_turns": 0}, indent=2))


if __name__ == "__main__":
    main()
