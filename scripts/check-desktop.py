#!/usr/bin/env python3
"""Native Go bridge test. No WebView, providers, real profiles or model calls."""
import json,os,pathlib,select,socket,subprocess,sys,tempfile,time,urllib.request
binary=str(pathlib.Path(sys.argv[1]).resolve())
with tempfile.TemporaryDirectory(prefix='am-app-',dir='/tmp') as temp:
 home=pathlib.Path(temp);root=home/'data'
 env=dict(os.environ,HOME=str(home),XDG_DATA_HOME=str(home/'xdg'),APPDATA=str(home/'appdata'))
 def call(*args):
  return subprocess.run([binary,'--root',str(root),*args],env=env,capture_output=True,text=True,check=True,timeout=15)
 def launch():
  p=subprocess.Popen([binary,'--root',str(root),'desktop'],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
  try:
   ready,_,_=select.select([p.stdout],[],[],20);assert ready,'no readiness'
   url=p.stdout.readline().strip();assert url.startswith('http://127.0.0.1:'),p.stderr.read() if p.poll() is not None else url
   address,token=url.split('#')
   with urllib.request.urlopen(urllib.request.Request(address+'api/status',headers={'Authorization':'Bearer '+token}),timeout=5) as response: status=json.load(response)
   return p,status
  except BaseException:
   p.stdin.close();p.wait(timeout=15);raise
 def close(p):p.stdin.close();p.wait(timeout=15)
 processes=[]
 try:
  first,status=launch();processes.append(first)
  # D-029: a client the window starts runs task workers at once, without
  # authorizing any session's model calls.
  assert not status['paused'] and status['cli_launch']=='not-authorized' and status.get('tasks_enabled'),status
  original=call('status').stdout
  profile=home/'profile';profile.mkdir();workspace=home/'workspace';workspace.mkdir()
  call('register-session','--provider','claude','--environment',str(profile),'--conversation-id','synthetic-desktop-test','--alias','test','--cli','/usr/bin/false','--workspace',str(workspace))
  call('enqueue','--alias','test','--id','desktop-one','--from','test','--text','Keep across window closure')
  before=json.loads(call('inspect').stdout)
  second,attached=launch();processes.append(second)
  assert attached['supervisor_pid']==status['supervisor_pid'],'duplicate supervisor'
  close(first);processes.remove(first)
  close(second);processes.remove(second)
  assert json.loads(call('inspect').stdout)==before,'closing UI changed queue'
  third,reopened=launch();processes.append(third)
  assert reopened['supervisor_pid']==status['supervisor_pid']
  assert json.loads(call('inspect').stdout)==before
  close(third);processes.remove(third)
  assert not before['Attempts']
  print(json.dumps(dict(start_without_models=True,tasks_enabled=True,reuse_client=True,ui_eof_keeps_client=True,reopen_keeps_queue=True,attempts=0)))
 finally:
  for p in processes:
   if p.poll() is None:close(p)
  try:call('shutdown')
  except (subprocess.CalledProcessError,subprocess.TimeoutExpired):pass
  # Wait for the supervisor socket to close before removing its isolated data.
  for _ in range(100):
   try:call('status')
   except (subprocess.CalledProcessError,subprocess.TimeoutExpired):break
   time.sleep(.05)
