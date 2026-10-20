import unittest,sqlite3,uuid
from datetime import datetime,timezone,timedelta
import analytics
class UsageTests(unittest.TestCase):
 def setUp(self):
  self.db=sqlite3.connect(':memory:');analytics.initialize(self.db);self.uid=str(uuid.uuid4());self.day=datetime.now(timezone(timedelta(hours=8))).date().isoformat()
 def event(self,event='page_stats',uid=None):return dict(installation=uid or self.uid,event_id=str(uuid.uuid4()),event=event,version='2.0.36',platform='darwin')
 def test_uv_pv_dedup_and_erasure(self):
  p=self.event();self.assertTrue(analytics.record(self.db,p));self.assertFalse(analytics.record(self.db,p));analytics.record(self.db,self.event());analytics.record(self.db,self.event('monitor_start'));analytics.record(self.db,self.event('stock_report'));analytics.record(self.db,self.event(uid=str(uuid.uuid4())))
  s=analytics.summary(self.db,self.day);self.assertEqual((s['uv'],s['pv'],s['monitor_uv'],s['contributor_uv'],s['reports']),(2,3,1,1,1))
  self.assertEqual(analytics.erase(self.db,{'installation':self.uid}),4);self.assertEqual(analytics.summary(self.db,self.day)['uv'],1)
  self.assertNotIn(self.uid,str(self.db.execute('select * from usage_events').fetchall()));self.assertNotIn('installation',s)
 def test_rejects_unknown_and_sensitive_fields(self):
  for p in [dict(self.event(),ip='127.0.0.1'),dict(self.event(),event='password'),dict(self.event(),installation='serial123'),dict(self.event(),version='bad')]:
   with self.assertRaises(ValueError):analytics.record(self.db,p)
 def test_retention(self):
  p=self.event();analytics.record(self.db,p);self.db.execute('update usage_events set received=0');analytics.record(self.db,self.event());self.assertEqual(self.db.execute('select count(*) from usage_events').fetchone()[0],1)
