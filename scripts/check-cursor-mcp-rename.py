#!/usr/bin/env python3
"""Check approved server names in an isolated Cursor CLI profile without a model.

Run with --cursor /path/to/cursor-agent --kotel /path/to/development/kotel.
Pass --previous-kotel to exercise an approved legacy path across a package upgrade.
The fixture is disposable and no user profile or chat is loaded.
"""

import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


WORKER_MARKER = "AGENT_MESSENGER_WORKER_RUN"


def workspace(fixture):
    user_home = fixture / "home"
    profile = user_home / ".cursor"
    project = fixture / "project"
    config = project / ".cursor" / "mcp.json"
    profile.mkdir(parents=True)
    config.parent.mkdir(parents=True)
    env = {
        "HOME": str(user_home),
        "USERPROFILE": str(user_home),
        "CFFIXED_USER_HOME": str(user_home),
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
    return project, config, env


def check(cursor, kotel, fixture, previous=None):
    project, config, env = workspace(fixture)
    command = kotel
    if previous:
        command = fixture / ("messenger.exe" if os.name == "nt" else "messenger")
        shutil.copy2(previous, command)

    def version():
        result = subprocess.check_output([str(command), "version"], env=env, text=True, timeout=15)
        return json.loads(result)["version"]

    def registration(program, root):
        return {
            "type": "stdio",
            "command": str(program),
            "args": ["--root", str(root), "coordinator", "--project", "Fixture"],
            "env": {WORKER_MARKER: "${env:" + WORKER_MARKER + "}"},
        }

    entry = registration(command, fixture / "kotel-data")
    servers = dict.fromkeys(("messenger", "fixture-other", "fixture-disabled", "fixture-unapproved"), entry)

    def write_config():
        config.write_text(json.dumps({"mcpServers": servers}), encoding="utf-8")

    def run(action, server, expected=None, *, error=False, worker=False, scope=(project, env)):
        folder, base_env = scope
        command_env = base_env | ({WORKER_MARKER: "fixture-task"} if worker else {})
        result = subprocess.run(
            [str(cursor), "mcp", action, server],
            cwd=folder,
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

    if previous:
        before = version()
        project_settings = config.read_bytes()
        staged = command.with_suffix(".staged")
        shutil.copy2(kotel, staged)
        staged.replace(command)
        after = version()
        if before == after:
            raise RuntimeError("The prior and current binaries report the same version.")
        run("list-tools", "messenger", "- list_agents (")
        if config.read_bytes() != project_settings:
            raise RuntimeError("The existing MCP project registration changed during the upgrade.")
        print(f"The approved legacy registration still exposed tools after {before} -> {after}.")

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

    fresh_project, fresh_config, fresh_env = workspace(fixture / "fresh")
    fresh_entry = registration(kotel, fixture / "fresh" / "kotel-data")
    fresh_config.write_text(json.dumps({"mcpServers": {"kotel": fresh_entry}}), encoding="utf-8")
    fresh_scope = (fresh_project, fresh_env)
    run("list-tools", "kotel", "has not been approved", error=True, scope=fresh_scope)
    run("enable", "kotel", scope=fresh_scope)
    run("list-tools", "kotel", "- list_agents (", scope=fresh_scope)
    run("list-tools", "messenger", "not found in config", error=True, scope=fresh_scope)
    print("A fresh Cursor profile exposed tools after approval of kotel only.")
    print("Authenticated chat resume and per-tool permissions require separate acceptance.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cursor", type=Path, required=True)
    parser.add_argument("--kotel", type=Path, required=True)
    parser.add_argument("--previous-kotel", type=Path)
    args = parser.parse_args()
    cursor, kotel = args.cursor.resolve(), args.kotel.resolve()
    previous = args.previous_kotel.resolve() if args.previous_kotel else None
    for binary in (cursor, kotel, previous):
        if binary and not binary.is_file():
            parser.error(f"executable does not exist: {binary}")
    fixtures = Path.home() / "AgentMessengerTest"
    fixtures.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="cursor-mcp-rename-", dir=fixtures) as directory:
        check(cursor, kotel, Path(directory), previous)


if __name__ == "__main__":
    main()
