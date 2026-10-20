"""Opt-in installation analytics. No IP, hardware ID or user content is stored."""
import hashlib,re,sqlite3
from datetime import datetime,timezone,timedelta
EVENTS={'app_open','page_monitor','page_stats','monitor_start','stock_report'}
ID=re.compile(r'^[0-9a-f-]{36}$')
def identity(value):
    if not isinstance(value,str) or not ID.fullmatch(value):raise ValueError('invalid identifier')
    return hashlib.sha256(value.encode()).hexdigest()
def initialize(db):
    db.execute('CREATE TABLE IF NOT EXISTS usage_events (event_id TEXT PRIMARY KEY, installation TEXT NOT NULL, event TEXT NOT NULL, day TEXT NOT NULL, version TEXT NOT NULL, platform TEXT NOT NULL, received INTEGER NOT NULL)')
    db.execute('CREATE INDEX IF NOT EXISTS usage_day ON usage_events(day,installation)')
def record(db,p):
    if not isinstance(p,dict) or set(p)!={'installation','event_id','event','version','platform'}:raise ValueError('invalid fields')
    uid,eid=identity(p['installation']),identity(p['event_id'])
    if p['event'] not in EVENTS or p['platform'] not in ('darwin','windows','linux') or not re.fullmatch(r'\d+\.\d+\.\d+',str(p['version'])):raise ValueError('invalid event')
    now=datetime.now(timezone.utc);day=now.astimezone(timezone(timedelta(hours=8))).date().isoformat()
    db.execute('DELETE FROM usage_events WHERE received < ?',(int(now.timestamp())-30*86400,))
    if db.execute('SELECT count(*) FROM usage_events WHERE installation=? AND day=?',(uid,day)).fetchone()[0]>=500:raise ValueError('daily event limit')
    inserted=db.execute('INSERT OR IGNORE INTO usage_events VALUES(?,?,?,?,?,?,?)',(eid,uid,p['event'],day,p['version'],p['platform'],int(now.timestamp()))).rowcount==1
    db.execute('DELETE FROM usage_events WHERE event_id IN (SELECT event_id FROM usage_events ORDER BY received DESC LIMIT -1 OFFSET 100000)')
    return inserted
def erase(db,p):
    if not isinstance(p,dict) or set(p)!={'installation'}:raise ValueError('invalid fields')
    return db.execute('DELETE FROM usage_events WHERE installation=?',(identity(p['installation']),)).rowcount
def summary(db,day):
    datetime.strptime(day,'%Y-%m-%d')
    rows=db.execute('SELECT installation,event,version,platform FROM usage_events WHERE day=? AND received>=?',(day,int(datetime.now(timezone.utc).timestamp())-30*86400)).fetchall()
    users={r[0] for r in rows};active={r[0] for r in rows if r[1]=='app_open'}
    return dict(date=day,uv=len(users),launch_uv=len(active),pv=sum(r[1] in ('page_monitor','page_stats') for r in rows),monitor_uv=len({r[0] for r in rows if r[1]=='monitor_start'}),contributor_uv=len({r[0] for r in rows if r[1]=='stock_report'}),reports=sum(r[1]=='stock_report' for r in rows),retention_days=30)
