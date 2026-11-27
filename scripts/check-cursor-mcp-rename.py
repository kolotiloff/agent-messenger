#!/usr/bin/env python3
"""Check approved server names in an isolated Cursor CLI profile without a model.

Run with --cursor /path/to/cursor-agent --kotel /path/to/development/kotel.
The fixture is disposable and no user profile or chat is loaded.
"""

import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile


WORKER_MARKER = "AGENT_MESSENGER_WORKER_RUN"


def check(cursor, kotel, fixture):
    user_home = fixture / "home"
    profile = user_home / ".cursor"
    project = fixture / "project"
    config = project / ".cursor" / "mcp.json"
    profile.mkdir(parents=True)
    config.parent.mkdir(parents=True)
    env = {
        "HOME": str(user_home),
        "USERPROFILE": str(user_home),
        "APPDATA": str(user_home / "AppData" / "Roaming"),
        "LOCALAPPDATA": str(user_home / "AppData" / "Local"),
        "CURSOR_CONFIG_DIR": str(profile),
        "XDG_CACHE_HOME": str(user_home / ".cache"),
        "XDG_CONFIG_HOME": str(user_home / ".config"),
        "XDG_DATA_HOME": str(user_home / ".local" / "share"),
        "PATH": os.defpath,
    }
    if os.name == "nt":
        env["SYSTEMROOT"] = os.environ["SYSTEMROOT"]

    entry = {
        "type": "stdio",
        "command": str(kotel),
        "args": ["--root", str(fixture / "kotel-data"), "coordinator", "--project", "Fixture"],
        "env": {WORKER_MARKER: "${env:" + WORKER_MARKER + "}"},
    }
    servers = dict.fromkeys(("messenger", "fixture-other", "fixture-disabled", "fixture-unapproved"), entry)

    def write_config():
        config.write_text(json.dumps({"mcpServers": servers}), encoding="utf-8")

    def run(action, server, expected=None, *, error=False, worker=False):
        command_env = env | ({WORKER_MARKER: "fixture-task"} if worker else {})
        result = subprocess.run(
            [str(cursor), "mcp", action, server],
            cwd=project,
            env=command_env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            timeout=30,
        )
        if (result.returncode != 0) != error or (expected and expected not in result.stdout):
            raise RuntimeError(f"{action} {server}: unexpected result: {result.stdout[-450:]}")

    write_config()
    run("enable", "messenger")
    run("enable", "fixture-other")
    run("disable", "fixture-disabled")
    run("list-tools", "messenger", "- list_agents (")
    print("The legacy server was approved and exposed coordinator tools.")

    # Change just the name. The transport, command, arguments and work folder
    # remain identical, so a rejection identifies a name-scoped approval.
    servers["kotel"] = servers.pop("messenger")
    write_config()
    run("list-tools", "kotel", "has not been approved", error=True)
    run("enable", "kotel")
    run("list-tools", "kotel", "- list_agents (")
    run("list-tools", "messenger", "not found in config", error=True)
    run("list-tools", "fixture-other", "- list_agents (")
    run("list-tools", "fixture-disabled", "is disabled", error=True)
    run("list-tools", "fixture-unapproved", "has not been approved", error=True)
    print("Scoped reapproval enabled only the renamed server and preserved other decisions.")

    # The worker marker can also affect Cursor's approval digest. Approve that
    # variant in this disposable profile to inspect the server's own guard.
    run("enable", "kotel", worker=True)
    run("list-tools", "kotel", "No tools available", worker=True)
    run("list-tools", "kotel", "- list_agents (")
    print("The approved worker transport exposed no coordinator tools.")
    print("Authenticated chat resume and per-tool permissions require separate acceptance.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cursor", type=Path, required=True)
    parser.add_argument("--kotel", type=Path, required=True)
    args = parser.parse_args()
    cursor, kotel = args.cursor.resolve(), args.kotel.resolve()
    for binary in (cursor, kotel):
        if not binary.is_file():
            parser.error(f"executable does not exist: {binary}")
    fixtures = Path.home() / "AgentMessengerTest"
    fixtures.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="cursor-mcp-rename-", dir=fixtures) as directory:
        check(cursor, kotel, Path(directory))


if __name__ == "__main__":
    main()
