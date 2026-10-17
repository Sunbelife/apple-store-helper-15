"""Password-only login using the settlement gateway's PBKDF2/session scheme."""
import base64
import hashlib
import hmac
import json
import time
from http import cookies
from pathlib import Path

COOKIE_NAME='iphone_stock_session'
SESSION_SECONDS=12*60*60

def load_config(path):
    config=json.loads(Path(path).read_text())
    for key in ('salt','password_hash','cookie_secret'):base64.b64decode(config[key],validate=True)
    return config

def cookie_value(config,stamp):
    signature=hmac.new(base64.b64decode(config['cookie_secret']),str(stamp).encode(),hashlib.sha256).digest()
    return str(stamp)+'.'+base64.urlsafe_b64encode(signature).decode().rstrip('=')

def valid_session(config,raw):
    if not config or not raw:return False
    try:
        jar=cookies.SimpleCookie();jar.load(raw)
        value=jar[COOKIE_NAME].value
        stamp=int(value.split('.',1)[0]);now=int(time.time())
        return 0<=now-stamp<SESSION_SECONDS and hmac.compare_digest(value,cookie_value(config,stamp))
    except (KeyError,ValueError,cookies.CookieError):return False

def check_password(config,password):
    result=hashlib.pbkdf2_hmac('sha256',password.encode(),base64.b64decode(config['salt']),int(config.get('pbkdf2_rounds',310000)))
    return hmac.compare_digest(result,base64.b64decode(config['password_hash']))
