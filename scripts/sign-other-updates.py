#!/usr/bin/env python3
"""Sign the Windows and Linux update archives of one release (D-048).

Each <os>-<arch>.zip from package-others.sh gets the same signed metadata the
Mac update has: update-<os>-<arch>.json with an Ed25519 signature of the
archive inside, update-<os>-<arch>.json.sig, and the archive as <sha256>.zip.
Every signature is checked against the app's public key before it is kept.
The program inside is run and checked by the window before it is installed;
it cannot be run here (another system).
"""
import argparse, base64, hashlib, json, pathlib, re, subprocess, zipfile

p = argparse.ArgumentParser()
p.add_argument('archives', type=pathlib.Path, help='folder with <os>-<arch>.zip files')
p.add_argument('--version', required=True)
p.add_argument('--revision', required=True)
p.add_argument('--notes', required=True, type=pathlib.Path)
p.add_argument('--sign', required=True, help='signing command; the file to sign is appended (prints a base64 signature)')
p.add_argument('--public-key', required=True)
p.add_argument('--out', required=True, type=pathlib.Path)
a = p.parse_args()

if not re.fullmatch(r'0\.0\.0-preview\.[1-9][0-9]{0,8}', a.version):
    raise SystemExit('A numbered preview version is required')
if not re.fullmatch(r'[0-9a-f]{40}', a.revision):
    raise SystemExit('A full source revision is required')
build = int(a.version.rsplit('.', 1)[1])
notes = a.notes.read_text()
if len(notes.encode()) > 16000:
    raise SystemExit('Notes exceed limits')
signer = a.sign.split()
expected = {'windows-amd64', 'windows-arm64', 'linux-amd64', 'linux-arm64'}
found = {x.stem for x in a.archives.glob('*.zip')}
if found != expected:
    raise SystemExit('Expected exactly the archives %s, found %s' % (sorted(expected), sorted(found)))


def sign(path):
    signature = subprocess.check_output(signer + [str(path)], text=True).strip()
    if len(base64.b64decode(signature, validate=True)) != 64:
        raise SystemExit('Unexpected signature')
    subprocess.run(['go', 'run', './scripts/verify-update-signature', a.public_key, str(path), signature], check=True)
    return signature


a.out.mkdir(parents=True, exist_ok=True)
for target in sorted(expected):
    os_name, arch = target.split('-')
    archive = a.archives / (target + '.zip')
    raw = archive.read_bytes()
    if not raw or len(raw) > 256 * 1024 * 1024:
        raise SystemExit('Archive exceeds limits')
    inner = 'messenger.exe' if os_name == 'windows' else 'messenger'
    with zipfile.ZipFile(archive) as z:
        if z.namelist() != [inner] or z.testzip() is not None:
            raise SystemExit('Unexpected archive content in ' + archive.name)
    digest = hashlib.sha256(raw).hexdigest()
    manifest = dict(schema=1, build=build, version=a.version, revision=a.revision, os=os_name, arch=arch,
                    client_api=1, sha256=digest, length=len(raw), signature=sign(archive), notes=notes)
    name = 'update-%s-%s.json' % (os_name, arch)
    m = a.out / name
    m.write_text(json.dumps(manifest, ensure_ascii=False, separators=(',', ':')) + '\n')
    (a.out / (name + '.sig')).write_text(sign(m) + '\n')
    (a.out / (digest + '.zip')).write_bytes(raw)
print(json.dumps(dict(version=a.version, build=build, signed=sorted(expected))))
