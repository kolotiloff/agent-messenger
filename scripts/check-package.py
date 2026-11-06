#!/usr/bin/env python3
# Developer/CI test only: the installed product does not require Python.
import pathlib,tempfile,tarfile,hashlib,json,os,subprocess,time,sys,shutil,socket
if sys.platform!='darwin':raise SystemExit('Package installation acceptance requires macOS')
if len(sys.argv)!=4:raise SystemExit('Usage: check-package.py PACKAGE_DIR VERSION1 VERSION2')
base=pathlib.Path(sys.argv[1]).resolve();first_version,second_version=sys.argv[2:];root=pathlib.Path(tempfile.mkdtemp(prefix='am-install-',dir='/tmp'));home=root/'home';home.mkdir();env=dict(os.environ,HOME=str(home),XDG_DATA_HOME=str(home/'xdg'),APPDATA=str(home/'appdata'))
def extract(version):
 matches=list(base.glob('agent-messenger-'+version+'-darwin-*.tar.gz'));assert len(matches)==1,matches;archive=matches[0];record=pathlib.Path(str(archive)+'.sha256').read_text().split()[0];assert hashlib.sha256(archive.read_bytes()).hexdigest()==record
 dest=root/version;dest.mkdir()
 with tarfile.open(archive) as tf:
  for m in tf.getmembers():
   assert m.isfile() and not m.name.startswith('/') and '..' not in pathlib.PurePosixPath(m.name).parts
   target=dest/m.name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(tf.extractfile(m).read());target.chmod(m.mode)
 manifest=json.loads((dest/'manifest.json').read_text());assert hashlib.sha256((dest/'messenger').read_bytes()).hexdigest()==manifest['binary_sha256'];return dest/'messenger'
def run(binary,*args,ok=True):
 p=subprocess.run([str(binary),*args],env=env,capture_output=True,text=True,timeout=15)
 if ok:assert p.returncode==0,p.stderr
 else:assert p.returncode!=0
 return p
first=extract(first_version);second=extract(second_version);prefix=home/'prefix';data=home/'Library'/'Application Support'/'AgentMessenger'
assert json.loads(run(first,'version').stdout)['version']==first_version;assert not data.exists()
run(first,'install','--prefix',str(prefix));installed=prefix/'bin'/'messenger';old=installed.resolve();assert old.is_file()
def control(command,**fields):
 with socket.socket(socket.AF_UNIX,socket.SOCK_STREAM) as connection:
  connection.settimeout(10);connection.connect(str(data/'control.sock'))
  connection.sendall((json.dumps(dict(cmd=command,**fields))+'\n').encode())
  with connection.makefile('rb') as stream:response=json.loads(stream.readline(2*1024*1024))
  assert response.get('ok') is True,response
  return response
prepared_id='ui-00000000-0000-4000-8000-000000000001'
queued_id='ui-00000000-0000-4000-8000-000000000002'
client=subprocess.Popen([str(installed),'serve'],env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
try:
 for _ in range(100):
  try:run(installed,'status');break
  except Exception:time.sleep(.05)
 else:raise RuntimeError('installed client did not start')
 profile=root/'profile';workspace=root/'workspace';profile.mkdir();workspace.mkdir()
 run(installed,'register-session','--provider','claude','--environment',str(profile),'--conversation-id','test-only-install','--alias','test','--cli','/usr/bin/false','--workspace',str(workspace))
 run(installed,'pause');run(installed,'enqueue','--alias','test','--id','install-test-one','--from','test-user','--text','Queue must survive an update')
 run(second,'install','--prefix',str(prefix),ok=False);assert installed.resolve()==old
 # Preserve both an unsent durable envelope and a queued envelope whose browser
 # acknowledgment was lost. Neither is allowed to replay automatically on update.
 for identifier,text in [(prepared_id,'Prepared before update'),(queued_id,'Queued before update')]:
  control('ui-prepare',message_id=identifier,alias='test',text=text)
 assert control('ui-submit',message_id=queued_id)['created'] is True
 outbox_before=control('ui-outbox')['outbox']
 assert {item['state'] for item in outbox_before['items']}=={'prepared','queued'}
 before=json.loads(run(installed,'inspect').stdout)
 assert len(before['Requests'])==2 and not before['Attempts']
 run(installed,'shutdown');client.wait(timeout=10)
 run(second,'install','--prefix',str(prefix));new=installed.resolve();assert new!=old and old.is_file()
 assert json.loads(run(installed,'version').stdout)['version']==second_version
 run(second,'install','--prefix',str(prefix));assert installed.resolve()==new
 client=subprocess.Popen([str(installed),'serve'],env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 for _ in range(100):
  try:after=json.loads(run(installed,'inspect').stdout);break
  except Exception:time.sleep(.05)
 else:raise RuntimeError('updated client did not start')
 assert before==after,(before,after)
 assert control('ui-outbox')['outbox']==outbox_before
 assert control('ui-submit',message_id=queued_id)['created'] is False
 # Reading and idempotent submit have not turned the prepared envelope into work.
 final=json.loads(run(installed,'inspect').stdout)
 assert len(final['Requests'])==2 and not final['Attempts']
 assert all(request['id']!=prepared_id for request in final['Requests'])
 status=run(installed,'status').stdout;assert 'cli_launch=not-authorized' in status
 run(installed,'shutdown');client.wait(timeout=10)
 report=dict(archive_and_binary_checksums=True,version_without_state_creation=True,clean_install=True,running_client_update_refused=True,update_while_stopped=True,idempotent_install=True,old_binary_retained=True,queue_snapshot_unchanged=True,prepared_and_queued_outbox_preserved=True,no_automatic_submission=True,queued_id_deduplicated_after_update=True,new_models_started=0,test_processes_stopped=True)
 print(json.dumps(report))
finally:
 if client.poll() is None:
  try:run(installed,'shutdown');client.wait(timeout=10)
  except Exception:client.terminate();client.wait(timeout=5)

# All paths below root were created by this isolated check.
shutil.rmtree(root)
