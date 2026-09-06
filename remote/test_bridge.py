import tempfile, unittest, uuid
from pathlib import Path
import bridge
class BridgeTests(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory();self.root=Path(self.temp.name)/'root';self.root.mkdir();bridge.SUPPORT=Path(self.temp.name)/'support'
  self.c={'roots':[{'id':'test','name':'Test','path':str(self.root)}]}
 def tearDown(self):self.temp.cleanup()
 def call(self,action,path='',**kw):return bridge.perform(self.c,dict(id=str(uuid.uuid4()),action=action,root='test',path=path,**kw))
 def test_read_write_conflict_and_backup(self):
  p=self.root/'code.py';p.write_text('print(1)\n');r=self.call('read','code.py')
  self.assertTrue(self.call('write','code.py',content='print(2)\n',revision=r['revision'])['saved'])
  self.assertTrue(self.call('write','code.py',content='print(3)\n',revision=r['revision'])['conflict'])
  self.assertEqual(p.read_text(),'print(2)\n');self.assertEqual(next((bridge.SUPPORT/'remote-backups').iterdir()).read_text(),'print(1)\n')
  self.assertTrue(self.call('write','code.py',content='print(2)\n',revision=r['revision'])['saved'])
 def test_create_and_revoke(self):
  self.assertTrue(self.call('write','new.ts',content='const a=1;',revision='new')['saved'])
  self.assertTrue(self.call('write','new.ts',content='different',revision='new')['conflict'])
  self.c['roots']=[]
  with self.assertRaises(ValueError):self.call('read','new.ts')
 def test_boundaries(self):
  (self.root/'link').symlink_to(self.root.parent,target_is_directory=True)
  for path in ['../escape','.git/config','/etc/passwd','link/outside','node_modules/x']:
   with self.assertRaises((ValueError,OSError)):self.call('read',path)
  self.assertEqual(self.call('list')['entries'],[])
 def test_binary_and_size(self):
  (self.root/'bad.bin').write_bytes(b'\0data')
  (self.root/'big.txt').write_bytes(b'a'*(bridge.MAX+1))
  for path in ['bad.bin','big.txt']:
   with self.assertRaises(ValueError):self.call('read',path)
if __name__=='__main__':unittest.main()
