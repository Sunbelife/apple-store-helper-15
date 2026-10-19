import json
import tempfile
import unittest
from datetime import datetime, timezone
from pathlib import Path
import server

class StockLogTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        server.DB = str(Path(self.tmp.name)/'events.sqlite3')
        server.initialize()
        self.event = dict(zip(server.FIELDS, ['cn','广东','深圳','R001','深圳-益田假日广场','iPhone Duo','512GB','星光白色','MK2P4CH/A',datetime.now(timezone.utc).isoformat()]))
    def tearDown(self):
        self.tmp.cleanup()
    def test_deduplicates_same_store_sku_but_keeps_other_store(self):
        self.assertTrue(server.record(self.event))
        self.assertFalse(server.record(self.event))
        self.assertTrue(server.record(dict(self.event,store_number='R002')))
        with server.connect() as db:
            self.assertEqual(db.execute('SELECT count(*) FROM events').fetchone()[0],2)
            columns = [r[1] for r in db.execute('PRAGMA table_info(events)')]
            self.assertNotIn('ip',columns)
    def test_rejects_extra_fields_invalid_time_and_empty_city(self):
        for change in ({'device_id':'no'},{'detected_at':'2020-01-01T00:00:00Z'},{'city':''},{'area':'unknown'}):
            with self.assertRaises(ValueError):server.record(dict(self.event,**change))

    def test_statistics_timezone_filters_and_public_fields(self):
        server.record(self.event)
        server.record(dict(self.event,store_number='R002'))
        day = datetime.now(timezone.utc).astimezone(timezone(__import__('datetime').timedelta(hours=8))).date().isoformat()
        result = server.statistics({'date':[day]})
        self.assertEqual(result['total'],2)
        self.assertEqual(sum(result['hourly']),2)
        self.assertEqual(result['cities'][0]['stores'],2)
        self.assertNotIn('events',result)
        self.assertEqual(server.statistics({'date':[day],'color':['missing']})['total'],0)
        with self.assertRaises(ValueError):server.statistics({'date':["bad"]})


class PasswordSessionTests(unittest.TestCase):
    def test_settlement_password_verifier_and_session_expiry(self):
        import auth, base64, hashlib, time
        salt=b'test-only-salt'
        config={'salt':base64.b64encode(salt).decode(),'password_hash':base64.b64encode(hashlib.pbkdf2_hmac('sha256',b'test-password',salt,1000)).decode(),'pbkdf2_rounds':1000,'cookie_secret':base64.b64encode(b'test-only-session-secret').decode()}
        self.assertTrue(auth.check_password(config,'test-password'))
        self.assertFalse(auth.check_password(config,'wrong-password'))
        value=auth.cookie_value(config,int(time.time()))
        self.assertTrue(auth.valid_session(config,auth.COOKIE_NAME+'='+value))
        self.assertFalse(auth.valid_session(config,auth.COOKIE_NAME+'='+value+'x'))
        expired=auth.cookie_value(config,int(time.time())-auth.SESSION_SECONDS-1)
        self.assertFalse(auth.valid_session(config,auth.COOKIE_NAME+'='+expired))

if __name__ == '__main__': unittest.main()
