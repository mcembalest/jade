import io,json,os,tempfile,unittest,uuid,urllib.request,urllib.error
from pathlib import Path
import bridge,cloud
URL=os.environ.get('JADE_TEST_SYNC_URL')
@unittest.skipUnless(URL,'Set JADE_TEST_SYNC_URL to the isolated local Worker')
class CloudTests(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory();self.root=Path(self.temp.name)/'project';self.root.mkdir()
  self.old=bridge.SUPPORT;bridge.SUPPORT=Path(self.temp.name)/'support'
  self.id='p-'+str(uuid.uuid4());self.c={'endpoint':URL,'agentToken':'agent-test-secret','roots':[{'id':self.id,'name':'Test','path':str(self.root),'cloud':True}]}
  self.client=cloud.ProjectSync(support=bridge.SUPPORT)
 def tearDown(self):bridge.SUPPORT=self.old;self.temp.cleanup()
 def phone(self,suffix='',body=None):
  r=urllib.request.Request(URL+'/v1/projects/'+self.id+suffix,data=None if body is None else json.dumps(body).encode(),headers={'Authorization':'Bearer local-test-key-not-for-production-123456','Content-Type':'application/json'})
  with urllib.request.urlopen(r) as response:return json.load(response)
 def edit(self,path,content,revision):return self.phone('/file',{'path':path,'content':content,'baseRevision':revision,'mutationId':str(uuid.uuid4())})
 def test_mac_off_delivery_restart_and_local_deletion(self):
  f=self.root/'code.py';f.write_text('original')
  self.client.cycle(self.c);r=self.phone('/file?path=code.py')['file'];self.assertEqual(r['macRevision'],r['revision'])
  accepted=self.edit('code.py','phone while Mac off',r['revision'])
  self.assertEqual(f.read_text(),'original');self.assertNotEqual(accepted['file']['macRevision'],accepted['acceptedRevision'])
  restarted=cloud.ProjectSync(support=bridge.SUPPORT);restarted.cycle(self.c)
  self.assertEqual(f.read_text(),'phone while Mac off')
  r=self.phone('/file?path=code.py')['file'];self.assertEqual(r['macRevision'],r['revision'])
  f.unlink();restarted.cycle(self.c);self.assertFalse(f.exists());self.assertIn('Removed',self.phone('/file?path=code.py')['file']['macIssue'])
 def test_independent_edits_preserve_both(self):
  f=self.root/'code.py';f.write_text('original');self.client.cycle(self.c)
  r=self.phone('/file?path=code.py')['file'];f.write_text('Mac independently changed')
  self.edit('code.py','phone independently changed',r['revision']);self.client.cycle(self.c)
  self.assertEqual(f.read_text(),'Mac independently changed');r=self.phone('/file?path=code.py')['file']
  self.assertEqual(r['content'],'phone independently changed');self.assertIn('Conflict',r['macIssue']);self.assertNotEqual(r['macRevision'],r['revision'])
 def test_lost_upload_reply_replays_same_id(self):
  f=self.root/'code.py';f.write_text('first');self.client.cycle(self.c);f.write_text('second')
  lost=[False]
  def transport(c,path,body=None):
   result=bridge.api(c,path,body)
   if path.endswith('/file') and body and not lost[0]:lost[0]=True;raise OSError('Lost reply after acceptance')
   return result
  cloud.ProjectSync(support=bridge.SUPPORT,transport=transport).cycle(self.c)
  state=json.loads((bridge.SUPPORT/'cloud-state'/(self.id+'.json')).read_text());self.assertIn('pending',state['code.py'])
  self.client.cycle(self.c);state=json.loads((bridge.SUPPORT/'cloud-state'/(self.id+'.json')).read_text());self.assertNotIn('pending',state['code.py'])
  self.assertEqual(len(self.phone('/history?path=code.py')['revisions']),2)
 def test_new_nested_file_and_corrupt_state(self):
  self.client.cycle(self.c);self.edit('src/new.py','phone file','');self.client.cycle(self.c)
  self.assertEqual((self.root/'src/new.py').read_text(),'phone file')
  state=bridge.SUPPORT/'cloud-state'/(self.id+'.json');state.write_text('broken')
  result=self.client.cycle(self.c);self.assertIn('Paused',result[self.id]);self.assertEqual(state.read_text(),'broken')
 def test_symlink_and_pause(self):
  outside=Path(self.temp.name)/'outside';outside.write_text('outside unchanged');(self.root/'link').symlink_to(outside)
  self.client.cycle(self.c);self.edit('link','malicious replacement','');self.client.cycle(self.c)
  self.assertEqual(outside.read_text(),'outside unchanged');self.assertTrue((self.root/'link').is_symlink())
  self.c['roots'][0]['cloud']=False;self.client.cycle(self.c)
  with self.assertRaises(urllib.error.HTTPError) as e:self.edit('new.py','paused','')
  self.assertEqual(e.exception.code,403)
