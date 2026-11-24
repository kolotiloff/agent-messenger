"""Check the public route used by 0.99, including each platform's signed manifest."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import tempfile
import time
from urllib.request import Request, urlopen

REPO = "kolotiloff/agent-messenger"
API = f"https://api.github.com/repos/{REPO}/releases?per_page=20"
TARGETS = ("darwin-arm64", "windows-amd64", "windows-arm64", "linux-amd64", "linux-arm64")
RETIRABLE = {"v0.0.0-preview.98", "v0.0.0-preview.99"}


def read(url, limit):
    headers = {"User-Agent": "Agent-Messenger-Updater", "X-GitHub-Api-Version": "2026-03-10",
               "Accept": "application/vnd.github+json" if url == API else "application/octet-stream"}
    with urlopen(Request(url, headers=headers), timeout=45) as response:
        data = response.read(limit + 1)
    if len(data) > limit:
        raise ValueError(f"Oversized response for {url}")
    return data


def asset(release, name):
    matches = [a for a in release["assets"] if a["name"] == name]
    if not matches:
        return None
    base = f"https://github.com/{REPO}/releases/download/{release['tag_name']}/"
    if len(matches) != 1 or matches[0]["browser_download_url"] != base + name:
        raise ValueError(f"Unexpected asset address for {name}")
    item = matches[0]
    if item["state"] != "uploaded" or item["size"] <= 0:
        raise ValueError(f"Incomplete asset {name}")
    return item


def verify(release, target, verifier, key, directory):
    name = f"update-{target}.json"
    manifest = directory / name
    manifest.write_bytes(read(asset(release, name)["browser_download_url"], 32768))
    signature = read(asset(release, name + ".sig")["browser_download_url"], 256)
    subprocess.run([verifier, key, manifest, signature.decode().strip()], check=True)
    data = json.loads(manifest.read_bytes())
    version = f"0.0.0-preview.{data['build']}"
    if (data["build"] < 101 or version != data["version"] or "v" + version != release["tag_name"]
            or target != f"{data['os']}-{data['arch']}" or not re.fullmatch("[a-f0-9]{64}", data["sha256"])):
        raise ValueError(f"Invalid signed upgrade destination for {target}")
    archive = asset(release, data["sha256"] + ".zip")
    if archive is None or archive["size"] != data["length"] or not 0 < data["length"] <= 256 << 20:
        raise ValueError(f"Archive size mismatch for {target}")

    location = directory / archive["name"]
    if not location.exists():
        package = read(archive["browser_download_url"], data["length"])
        if len(package) != data["length"] or hashlib.sha256(package).hexdigest() != data["sha256"]:
            raise ValueError(f"Archive checksum mismatch for {target}")
        location.write_bytes(package)
    subprocess.run([verifier, key, location, data["signature"]], check=True)


def inspect(version, verifier, key, directory):
    releases = [r for r in json.loads(read(API, 2 << 20)) if not r["draft"]]
    expected = next((r for r in releases if r["tag_name"] == "v" + version), None)
    if not expected:
        raise ValueError(f"The public updater listing does not contain {version}")
    blocked = []
    for target in TARGETS:
        name = f"update-{target}.json"
        verify(expected, target, verifier, key, directory)
        for release in releases:
            item = asset(release, name)
            if item is None:
                continue
            if release["tag_name"] in RETIRABLE:
                blocked.append((release["tag_name"], item))
                continue
            if release != expected:
                verify(release, target, verifier, key, directory)
            print(f"{target}: verified route to {release['tag_name']}", flush=True)
            break
        else:
            raise ValueError(f"No upgrade destination for {target}")
    return blocked


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("version")
    parser.add_argument("--retire-legacy-manifests", action="store_true")
    args = parser.parse_args()
    key = Path("desktop/macos/update-public-key.txt").read_text().strip()
    with tempfile.TemporaryDirectory() as tmp:
        directory = Path(tmp)
        verifier = directory / "verify"
        subprocess.run(["go", "build", "-o", verifier, "./scripts/verify-update-signature"], check=True)
        blocked = inspect(args.version, verifier, key, directory)
        if blocked and not args.retire_legacy_manifests:
            raise SystemExit("Older manifests precede the signed upgrade: " +
                             ", ".join(f"{tag}/{a['name']}" for tag, a in blocked))
        # 0.98 before 0.99, so users still on 0.99 never see a downgrade.
        for tag, item in sorted(blocked, key=lambda entry: entry[0]):
            name = item["name"]
            print(f"Retiring {tag}/{name} (asset {item['id']})", flush=True)
            subprocess.run(["gh", "api", "--method", "PATCH",
                            f"repos/{REPO}/releases/assets/{item['id']}",
                            "-f", f"name=archived-{name}", "--silent"], check=True)
        if blocked:
            for delay in (5, 15, 30, 60, 60):
                time.sleep(delay)
                if not inspect(args.version, verifier, key, directory):
                    return
            raise SystemExit("Public listing still serves old manifest names; recheck after cache expiry.")


if __name__ == "__main__":
    main()
