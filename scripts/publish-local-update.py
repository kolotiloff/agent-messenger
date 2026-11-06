#!/usr/bin/env python3
"""Publish one signed preview to this user's local development channel.
The private key stays in Keychain. No upload, provider call or client shutdown.
"""
import argparse,base64,hashlib,json,os,pathlib,plistlib,re,shutil,subprocess,tempfile
p=argparse.ArgumentParser()
p.add_argument('bundle_directory',type=pathlib.Path)
p.add_argument('--sparkle',required=True,type=pathlib.Path)
p.add_argument('--notes',required=True,type=pathlib.Path)
p.add_argument('--directory',type=pathlib.Path,default=pathlib.Path.home()/'Library/Application Support/AgentMessenger/updates')
a=p.parse_args();source=a.bundle_directory.resolve();target=a.directory.resolve()
app=source/'Agent Messenger.app';archive=source/'Agent-Messenger-mac.zip'
with (app/'Contents/Info.plist').open('rb') as f: info=plistlib.load(f)
version=info['CFBundleShortVersionString'];build=int(info['CFBundleVersion'])
if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+-preview\.[1-9][0-9]{0,8}',version) or int(version.rsplit('.',1)[1])!=build: raise SystemExit('A numbered preview version is required')
if info.get('AMClientAPI')!=1: raise SystemExit('Unsupported client API')
subprocess.run(['/usr/bin/codesign','--verify','--deep','--strict',str(app)],check=True)
meta=json.loads(subprocess.check_output([str(app/'Contents/Resources/messenger'),'version']))
if meta['revision']=='0'*40 or meta['version']!=version or meta['os']!='darwin' or not re.fullmatch('[0-9a-f]{40}',meta['revision']): raise SystemExit('Bundle metadata mismatch')
public=subprocess.check_output([str(a.sparkle/'bin/generate_keys'),'--account','agent-messenger-preview','-p'],text=True).strip()
if public!=info['SUPublicEDKey']: raise SystemExit('Signing key does not match application')
notes=a.notes.read_text();raw=archive.read_bytes()
if len(raw)>256*1024*1024 or not raw or len(notes.encode())>16000: raise SystemExit('Update exceeds limits')
digest=hashlib.sha256(raw).hexdigest()
# Verify that the exact ZIP being signed contains the supplied application.
with tempfile.TemporaryDirectory(prefix='am-publish-') as d:
 subprocess.run(['/usr/bin/ditto','-x','-k',str(archive),d],check=True)
 unpacked=pathlib.Path(d)/'Agent Messenger.app'
 subprocess.run(['/usr/bin/codesign','--verify','--deep','--strict',str(unpacked)],check=True)
 def files(root):return {str(x.relative_to(root)):('link',os.readlink(x)) if x.is_symlink() else ('file',x.stat().st_mode&0o777,hashlib.sha256(x.read_bytes()).hexdigest()) for x in root.rglob('*') if x.is_file() or x.is_symlink()}
 if files(app)!=files(unpacked):raise SystemExit('ZIP and bundle differ; refusing to sign')
signer=[str(a.sparkle/'bin/sign_update'),'--account','agent-messenger-preview','-p']
signature=subprocess.check_output(signer+[str(archive)],text=True).strip()
manifest=dict(schema=1,build=build,version=version,revision=meta['revision'],os=meta['os'],arch=meta['arch'],client_api=1,sha256=digest,length=len(raw),signature=signature,notes=notes)
target.mkdir(parents=True,exist_ok=True,mode=0o700)
previous=target/'manifest.json'
if previous.exists():
 old=json.loads(previous.read_text())
 if int(old['build'])>build or (int(old['build'])==build and old['sha256']!=digest):raise SystemExit('Refusing an older or conflicting build')
with tempfile.TemporaryDirectory(prefix='.publish-',dir=target) as d:
 temp=pathlib.Path(d);m=temp/'manifest.json'
 m.write_text(json.dumps(manifest,ensure_ascii=False,separators=(',',':'))+'\n')
 signed=subprocess.check_output(signer+[str(m)],text=True).strip()
 if len(base64.b64decode(signed,validate=True))!=64:raise SystemExit('Unexpected signature')
 dest=target/(digest+'.zip')
 if dest.exists():
  if dest.is_symlink() or hashlib.sha256(dest.read_bytes()).hexdigest()!=digest:raise SystemExit('Archive collision')
 else:
  copy=temp/'archive.zip';shutil.copyfile(archive,copy);os.chmod(copy,0o600);os.link(copy,dest)
 s=temp/'manifest.sig';s.write_text(signed+'\n')
 os.chmod(m,0o600);os.chmod(s,0o600)
 # A check between these replacements fails signature validation safely.
 os.replace(s,target/'manifest.sig');os.replace(m,target/'manifest.json')
print(json.dumps(dict(version=version,build=build,channel=str(target),signed=True)))
