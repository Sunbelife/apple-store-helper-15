#!/usr/bin/env python3
"""Public Apple inventory observations; deliberately stores no client metadata."""
import json
import html
import time
import threading
import auth
import os
import sqlite3
from datetime import datetime, timezone, timedelta
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlsplit

DB = os.environ.get('STOCK_DB', '/var/lib/iphone-stock/events.sqlite3')
AUTH_CONFIG = None
FAILURES = []
AUTH_LOCK = threading.Lock()

FIELDS = ('area', 'province', 'city', 'store_number', 'store_name', 'model', 'capacity', 'color', 'sku', 'detected_at')

def connect():
    db = sqlite3.connect(DB, timeout=10)
    db.row_factory = sqlite3.Row
    return db

def initialize():
    Path(DB).parent.mkdir(parents=True, exist_ok=True)
    with connect() as db:
        db.execute('PRAGMA journal_mode=WAL')
        db.execute('''CREATE TABLE IF NOT EXISTS events (
            id INTEGER PRIMARY KEY, area TEXT NOT NULL, province TEXT NOT NULL,
            city TEXT NOT NULL, store_number TEXT NOT NULL, store_name TEXT NOT NULL,
            model TEXT NOT NULL, capacity TEXT NOT NULL, color TEXT NOT NULL,
            sku TEXT NOT NULL, detected_at TEXT NOT NULL, received_at TEXT NOT NULL,
            bucket INTEGER NOT NULL, UNIQUE(area, store_number, sku, bucket))''')
        db.execute('CREATE INDEX IF NOT EXISTS received_idx ON events(received_at DESC)')

def validate(payload):
    if not isinstance(payload, dict) or set(payload) != set(FIELDS):
        raise ValueError('unexpected fields')
    for key, value in payload.items():
        if not isinstance(value, str) or len(value) > 160 or any(ord(c) < 32 for c in value):
            raise ValueError('invalid field')
        if key != 'province' and not value.strip():
            raise ValueError('missing field')
    if payload['area'] not in ('cn', 'hk', 'jp', 'sg', 'us', 'uk', 'au'):
        raise ValueError('invalid area')
    dt = datetime.fromisoformat(payload['detected_at'].replace('Z', '+00:00'))
    now = datetime.now(timezone.utc)
    if dt.tzinfo is None or abs((now-dt).total_seconds()) > 86400:
        raise ValueError('invalid time')
    payload = dict(payload)
    payload['detected_at'] = dt.astimezone(timezone.utc).isoformat(timespec='seconds')
    return payload

def record(payload):
    payload = validate(payload)
    now = datetime.now(timezone.utc)
    with connect() as db:
        cursor = db.execute('INSERT OR IGNORE INTO events (' + ','.join(FIELDS) + ',received_at,bucket) VALUES (' + ','.join('?' for _ in range(12)) + ')',
                            [payload[k] for k in FIELDS] + [now.isoformat(timespec='seconds'), int(now.timestamp()) // 60])
        inserted = cursor.rowcount == 1
        db.execute("DELETE FROM events WHERE received_at < strftime('%Y-%m-%dT%H:%M:%S', 'now', '-180 days')")
        db.execute('DELETE FROM events WHERE id IN (SELECT id FROM events ORDER BY id DESC LIMIT -1 OFFSET 100000)')
    return inserted

def statistics(params):
    tz = timezone(timedelta(hours=8))
    now = datetime.now(tz)
    day = params.get('date', [now.date().isoformat()])[0]
    start = datetime.strptime(day, '%Y-%m-%d').replace(tzinfo=tz)
    if start.date() > now.date() or (now.date()-start.date()).days > 180:
        raise ValueError('date out of range')
    end = start + timedelta(days=1)
    clauses, values = ['detected_at >= ?', 'detected_at < ?'], [start.astimezone(timezone.utc).isoformat(timespec='seconds'), end.astimezone(timezone.utc).isoformat(timespec='seconds')]
    for key in ('area','model','capacity','color','city'):
        value = params.get(key, [''])[0]
        if len(value) > 160: raise ValueError('invalid filter')
        if value:
            clauses.append(key+' = ?'); values.append(value)
    if params.get('recent', [''])[0] == '1':
        clauses.append('detected_at >= ?'); values.append((now-timedelta(hours=1)).astimezone(timezone.utc).isoformat(timespec='seconds'))
    with connect() as db:
        rows = db.execute('SELECT area,city,store_number,detected_at FROM events WHERE '+' AND '.join(clauses),values).fetchall()
        options = {k:[r[0] for r in db.execute('SELECT DISTINCT '+k+' FROM events ORDER BY '+k) if r[0]] for k in ('area','model','capacity','color')}
        latest = db.execute('SELECT max(detected_at) FROM events').fetchone()[0]
    hourly = [0]*24
    cities = {}
    for r in rows:
        detected = datetime.fromisoformat(r['detected_at']).astimezone(tz)
        hourly[detected.hour] += 1
        key = (r['area'],r['city'])
        city = cities.setdefault(key,dict(area=r['area'],city=r['city'],count=0,stores=set(),last_at=''))
        city['count'] += 1; city['stores'].add(r['store_number']); city['last_at'] = max(city['last_at'],r['detected_at'])
    ranking = []
    for city in cities.values():
        city['stores'] = len(city['stores']); ranking.append(city)
    ranking.sort(key=lambda c:(-c['count'],c['city']))
    return dict(date=day, timezone='Asia/Shanghai', total=len(rows), city_count=len(ranking), hourly=hourly, cities=ranking, options=options, latest_at=latest, updated_at=now.isoformat(timespec='seconds'))

class Handler(BaseHTTPRequestHandler):
    server_version = 'StockLog'
    def log_message(self, *args):
        pass  # Never log IP, request headers, URLs or request bodies.
    def reply(self, status, body, content_type='application/json; charset=utf-8'):
        raw = body if isinstance(body, bytes) else json.dumps(body, ensure_ascii=False).encode()
        self.send_response(status)
        self.send_header('Content-Type', content_type)
        self.send_header('Content-Length', str(len(raw)))
        self.send_header('Cache-Control', 'no-store')
        self.send_header('X-Robots-Tag', 'noindex, nofollow')
        self.end_headers()
        self.wfile.write(raw)
    def render_login(self, error='', status=200):
        message = '<div class="error">'+html.escape(error)+'</div>' if error else ''
        body = Path(__file__).with_name('login.html').read_text().replace('{error}', message).encode()
        self.reply(status, body, 'text/html; charset=utf-8')
    def do_POST(self):
        if self.path in ('/iphone-stock/login', '/iphone-stock/logout'):
            origin = self.headers.get('Origin')
            if origin and origin != 'https://sunbelife.com':
                return self.reply(403, {'error':'invalid origin'})
            if self.path.endswith('/logout'):
                self.send_response(303)
                self.send_header('Location', '/iphone-stock')
                self.send_header('Set-Cookie', auth.COOKIE_NAME+'=; Path=/iphone-stock; Max-Age=0; HttpOnly; Secure; SameSite=Strict')
                self.end_headers()
                return
            try:
                length = int(self.headers.get('Content-Length','0'))
                if not 0 < length <= 4096: raise ValueError()
                form = parse_qs(self.rfile.read(length).decode('utf-8'))
                password = form.get('password',[''])[0]
            except (ValueError, UnicodeError):
                return self.render_login('请求无效，请重试。', 400)
            with AUTH_LOCK:
                now = time.time()
                FAILURES[:] = [stamp for stamp in FAILURES if now-stamp < 900]
                if len(FAILURES) >= 10:
                    return self.render_login('尝试次数过多，请稍后再试。',429)
                if not AUTH_CONFIG or not auth.check_password(AUTH_CONFIG,password):
                    FAILURES.append(now)
                    return self.render_login('密码不正确，请重新输入。',403)
                FAILURES.clear()
            self.send_response(303)
            self.send_header('Location','/iphone-stock')
            self.send_header('Cache-Control','private, no-store')
            self.send_header('Set-Cookie',auth.COOKIE_NAME+'='+auth.cookie_value(AUTH_CONFIG,int(now))+'; Path=/iphone-stock; Max-Age='+str(auth.SESSION_SECONDS)+'; HttpOnly; Secure; SameSite=Strict')
            self.end_headers()
            return
        if self.path != '/api/iphone-stock/events':
            return self.reply(404, {'error':'not found'})
        try:
            length = int(self.headers.get('Content-Length', '0'))
            if not 0 < length <= 4096:
                return self.reply(413, {'error':'body too large or empty'})
            if self.headers.get_content_type() != 'application/json':
                return self.reply(415, {'error':'JSON required'})
            inserted = record(json.loads(self.rfile.read(length)))
        except (ValueError, TypeError, OverflowError):
            return self.reply(400, {'error':'invalid observation'})
        except sqlite3.Error:
            return self.reply(503, {'error':'temporarily unavailable'})
        self.reply(201 if inserted else 200, {'ok':True, 'inserted':inserted})
    def do_GET(self):
        parts = urlsplit(self.path)
        if parts.path == '/iphone-stock/api/stats':
            try:
                return self.reply(200, statistics(parse_qs(parts.query)))
            except (ValueError, TypeError):
                return self.reply(400, {'error':'invalid filters'})
            except sqlite3.Error:
                return self.reply(503, {'error':'temporarily unavailable'})
        if parts.path in ('/iphone-stock', '/iphone-stock/') and not auth.valid_session(AUTH_CONFIG,self.headers.get('Cookie')):
            return self.render_login()
        if parts.path == '/iphone-stock/api/events' and not auth.valid_session(AUTH_CONFIG,self.headers.get('Cookie')):
            return self.reply(401, {'error':'login required'})
        if parts.path in ('/iphone-stock', '/iphone-stock/'):

            return self.reply(200, Path(__file__).with_name('index.html').read_bytes(), 'text/html; charset=utf-8')
        if parts.path != '/iphone-stock/api/events':
            return self.reply(404, {'error':'not found'})
        params = parse_qs(parts.query)
        try:
            page = max(1, min(int(params.get('page', ['1'])[0]), 2000))
        except ValueError:
            return self.reply(400, {'error':'invalid page'})
        query = params.get('q', [''])[0][:100]
        where, values = '', []
        if query:
            where = " WHERE instr(province || city || store_name || model || capacity || color || sku, ?) > 0"
            values = [query]
        with connect() as db:
            total = db.execute('SELECT count(*) FROM events'+where, values).fetchone()[0]
            rows = db.execute('SELECT '+','.join(FIELDS)+',received_at FROM events'+where+' ORDER BY id DESC LIMIT 50 OFFSET ?', values+[(page-1)*50]).fetchall()
        self.reply(200, {'total':total, 'page':page, 'events':[dict(row) for row in rows]})

if __name__ == '__main__':
    AUTH_CONFIG = auth.load_config(os.environ.get('STOCK_AUTH_CONFIG','/etc/iphone-stock-auth.json'))
    initialize()
    server = ThreadingHTTPServer(('127.0.0.1', int(os.environ.get('PORT','18774'))), Handler)
    server.timeout = 10
    server.serve_forever()
