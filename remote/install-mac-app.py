from pathlib import Path
import os,plistlib,subprocess,sys,time
repo=Path(__file__).resolve().parent.parent
app=Path.home()/'Applications/JaDE Mac Connection.app'
contents=app/'Contents';binary=contents/'MacOS/JaDEMacConnection'
binary.parent.mkdir(parents=True,exist_ok=True)
subprocess.run(['swiftc',str(repo/'remote/MacConnection.swift'),'-o',str(binary)],check=True)
(contents/'Info.plist').write_bytes(plistlib.dumps({'CFBundleIdentifier':'com.mcembalest.jade.macconnection','CFBundleExecutable':'JaDEMacConnection','CFBundleName':'JaDE Mac Connection','CFBundlePackageType':'APPL','CFBundleVersion':'1','LSUIElement':True,'NSDocumentsFolderUsageDescription':'JaDE reads and edits the writing folders you enable for your iPhone.','NSDesktopFolderUsageDescription':'JaDE reads and edits the repos you enable for your iPhone.','JaDEPython':sys.executable}))
subprocess.run(['codesign','--force','--sign','-',str(app)],check=True,capture_output=True)
domain='gui/'+str(os.getuid());label='com.mcembalest.jade.remote'
subprocess.run(['launchctl','bootout',domain+'/'+label],capture_output=True)
for _ in range(40):
 if subprocess.run(['launchctl','print',domain+'/'+label],capture_output=True).returncode:break
 time.sleep(.1)
agent=Path.home()/'Library/LaunchAgents'/f'{label}.plist'
agent.write_bytes(plistlib.dumps({'Label':label,'ProgramArguments':['/usr/bin/open','-g',str(app)],'RunAtLoad':True}))
subprocess.run(['launchctl','bootstrap',domain,str(agent)],check=True)
launcher=Path.home()/'Library/Application Support/JaDE/Choose JaDE Mac folders.command'
import shlex
launcher.write_text('#!/bin/sh\nopen '+shlex.quote(str(app))+' --args --choose\n');launcher.chmod(0o700)
subprocess.run(['open',str(app),'--args','--choose'],check=True)
print('JaDE Mac Connection installed and folder chooser opened.')
