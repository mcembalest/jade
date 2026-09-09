import io,json,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import bridge

class CompanionTests(unittest.TestCase):
    def test_fixed_destination_and_shared_state(self):
        state={'enabled':True,'messages':[{'id':'shared','role':'assistant','text':'Same conversation'}],'pending':[]}
        with patch('bridge.urllib.request.urlopen',return_value=io.BytesIO(json.dumps(state).encode())) as open_url:
            result=bridge.companion_request({'companion':{'action':'chat','message':'Hello'}})
            request=open_url.call_args.args[0]
            self.assertEqual(request.full_url,'http://127.0.0.1:7339/companion')
            self.assertEqual(json.loads(request.data),{'action':'chat','message':'Hello'})
            self.assertEqual(result['companion'],state)
    def test_reject_arbitrary_commands_urls_and_invalid_arguments(self):
        for payload in [{'action':'shell'},{'action':'chat','message':'x','url':'https://bad.test'}, {'action':'enabled','enabled':'yes'}, {'action':'chat','message':'x'*8001}]:
            with self.assertRaises(ValueError):bridge.companion_request({'companion':payload})
    def test_receipt_reserved_before_call_and_result_persisted(self):
        with tempfile.TemporaryDirectory() as folder:
            receipt=Path(folder)/'receipt'
            def respond(_):
                self.assertIn('interrupted',json.loads(receipt.read_text())['error'])
                return {'companion':{'enabled':True,'messages':[]}}
            with patch('bridge.companion_request',side_effect=respond):result=bridge.run_companion({},receipt)
            self.assertEqual(json.loads(receipt.read_text()),result)
    def test_failure_retains_readable_receipt(self):
        with tempfile.TemporaryDirectory() as folder:
            receipt=Path(folder)/'receipt'
            with patch('bridge.companion_request',side_effect=OSError('offline')):
                bridge.run_companion({},receipt)
            self.assertIn('error',json.loads(receipt.read_text()))
