"""No live model calls or personal credentials. Run with python3 -m unittest."""
import importlib.util,json,os,tempfile,unittest
from pathlib import Path
from unittest.mock import patch

class RuntimeTests(unittest.TestCase):
    def setUp(self):
        self.home=tempfile.TemporaryDirectory()
        self.env=patch.dict(os.environ,{'HOME':self.home.name});self.env.start()
        spec=importlib.util.spec_from_file_location('runtime_server',Path(__file__).with_name('server.py'))
        self.server=importlib.util.module_from_spec(spec);spec.loader.exec_module(self.server)
    def tearDown(self):
        self.env.stop();self.home.cleanup()
    def test_refreshed_credentials_are_not_replaced_by_old_object_copy(self):
        self.server.restore_auth({'refresh_token':'new-test-token'})
        self.server.restore_auth({'refresh_token':'stale-test-token'})
        self.assertEqual(self.server.auth_read()['refresh_token'],'new-test-token')
        self.assertEqual((self.server.AUTH/'auth.json').stat().st_mode & 0o777,0o600)
    def test_full_history_is_readable_and_memory_changes_are_returned(self):
        def run(command,**options):
            root=Path(options['cwd'])
            self.assertEqual(json.loads((root/'history/700.json').read_text())['document']['text'],'Old finding')
            self.assertEqual((root/'INSTRUCTIONS.md').read_text(),'Search astronomy')
            self.assertIn('workspace-write',command)
            self.assertIn('sandbox_workspace_write.network_access=false',command)
            (root/'MEMORY.md').write_text('New working memory')
            (root/'result.json').write_text(json.dumps({'report':'Report','findings':[]}))
            return type('Result',(),{'returncode':0})()
        with patch.object(self.server,'sandbox_ready',return_value=True),patch.object(self.server.subprocess,'run',side_effect=run):
            result=self.server.run_research({'date':'2026-09-16','notebook':{'name':'Ada','profile':'My own identity','instructions':'Search astronomy','memory':'Prior memory'},'history':[{'seq':700,'document':{'text':'Old finding'}}]})
        self.assertEqual(result['memory'],'New working memory')
    def test_probe_uses_current_cli_and_preserves_restrictions(self):
        with patch.object(self.server.subprocess,'run',return_value=type('Result',(),{'returncode':0})()) as run:
            self.assertTrue(self.server.sandbox_ready())
            args=run.call_args.args[0]
            self.assertEqual(args[:2],['codex','sandbox'])
            self.assertNotIn('linux',args)
            self.assertIn('sandbox_workspace_write.network_access=false',args)

if __name__=='__main__':unittest.main()
