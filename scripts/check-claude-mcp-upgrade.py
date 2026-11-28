#!/usr/bin/env python3
"""Check Claude's supported MCP registration commands without loading a chat.

Use an unpacked previous release and the candidate Kotel executable. Only the
CLI's MCP subcommands run, with separate synthetic homes and projects.
"""

import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


def workspace(root):
    home = root / "home"
    config = home / ".claude"
    project = root / "project"
    config.mkdir(parents=True)
    project.mkdir()
    env = {
        "HOME": str(home),
        "USERPROFILE": str(home),
        "CFFIXED_USER_HOME": str(home),
        "APPDATA": str(home / "AppData" / "Roaming"),
        "LOCALAPPDATA": str(home / "AppData" / "Local"),
        "CLAUDE_CONFIG_DIR": str(config),
        "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
        "XDG_CACHE_HOME": str(home / ".cache"),
        "XDG_CONFIG_HOME": str(home / ".config"),
        "XDG_DATA_HOME": str(home / ".local" / "share"),
        "PATH": os.defpath,
        "NO_COLOR": "1",
    }
    if os.name == "nt":
        env["SYSTEMROOT"] = os.environ["SYSTEMROOT"]
    return config, project, env


def run(argv, project, env):
    result = subprocess.run(argv, cwd=project, env=env, text=True, timeout=30,
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    if result.returncode != 0:
        raise RuntimeError(f"Unexpected exit from {Path(argv[0]).name}: {result.stdout[-450:]}")
    return result.stdout


def add(claude, project, env, name, program, root):
    run([claude, "mcp", "add", "--scope", "local", name, "--", program,
         "--root", root, "coordinator", "--project", "Fixture"], project, env)


def servers(config, project):
    settings_file = config / ".claude.json"
    settings = json.loads(settings_file.read_text(encoding="utf-8"))
    return settings["projects"][str(project)]["mcpServers"]


def connected(claude, project, env, *names):
    listing = run([claude, "mcp", "list"], project, env)
    for name in names:
        if not any(line.startswith(name + ":") and line.rstrip().endswith("Connected")
                   for line in listing.splitlines()):
            raise RuntimeError(f"Claude did not connect to {name}: {listing[-450:]}")


def check(claude, previous, candidate, fixture):
    legacy = fixture / "legacy"
    config, project, env = workspace(legacy)
    command = legacy / ("messenger.exe" if os.name == "nt" else "messenger")
    shutil.copy2(previous, command)
    add(claude, project, env, "messenger", command, legacy / "data")
    add(claude, project, env, "fixture_other", command, legacy / "data")

    # Rules exist in the disposable workspace only. Checking their bytes does
    # not claim that a model turn applied them.
    permissions_file = project / ".claude" / "settings.local.json"
    permissions_file.parent.mkdir()
    permissions_file.write_text(json.dumps({"permissions": {
        "allow": ["mcp__messenger__list_agents"],
        "deny": ["mcp__fixture_other__list_agents"],
    }}), encoding="utf-8")
    original_permissions = permissions_file.read_bytes()
    original_servers = servers(config, project)
    connected(claude, project, env, "messenger", "fixture_other")
    before = json.loads(run([command, "version"], project, env))["version"]

    staged = command.with_suffix(".staged")
    shutil.copy2(candidate, staged)
    staged.replace(command)
    after = json.loads(run([command, "version"], project, env))["version"]
    if before == after:
        raise RuntimeError("The prior and current binaries report the same version.")
    connected(claude, project, env, "messenger", "fixture_other")
    if servers(config, project) != original_servers or permissions_file.read_bytes() != original_permissions:
        raise RuntimeError("A legacy registration or its fixture permission settings changed.")
    print(f"Claude's legacy registration connected after {before} -> {after}. Settings were unchanged.")

    fresh = fixture / "fresh"
    config, project, env = workspace(fresh)
    add(claude, project, env, "kotel", candidate, fresh / "data")
    connected(claude, project, env, "kotel")
    if set(servers(config, project)) != {"kotel"}:
        raise RuntimeError("A fresh profile did not contain exactly the kotel registration.")
    print("A fresh Claude profile connected to kotel only.")
    print("Authenticated chat resume and runtime tool permissions require separate acceptance.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--claude", type=Path, required=True)
    parser.add_argument("--previous-kotel", type=Path, required=True)
    parser.add_argument("--kotel", type=Path, required=True)
    args = parser.parse_args()
    binaries = [path.resolve() for path in (args.claude, args.previous_kotel, args.kotel)]
    for binary in binaries:
        if not binary.is_file():
            parser.error(f"Executable does not exist: {binary}")
    fixtures = Path.home() / "AgentMessengerTest"
    fixtures.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="claude-mcp-upgrade-", dir=fixtures) as directory:
        check(*binaries, Path(directory).resolve())


if __name__ == "__main__":
    main()
