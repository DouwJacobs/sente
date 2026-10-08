#!/usr/bin/env python3
"""Production-image acceptance using disposable Docker volumes and invented data only."""
import argparse
import hashlib
import http.cookiejar
import json
import re
from pathlib import Path
import socket
import sqlite3
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[1]

def docker(*args, capture=True):
    return subprocess.check_output(['docker', *map(str, args)], text=True).strip() if capture else subprocess.run(['docker', *map(str, args)], check=True)

class Smoke:
    def __init__(self, image, work):
        self.image, self.work = image, work
        self.prefix = 'sente-release-' + uuid.uuid4().hex[:12]
        self.data, self.backups = self.prefix+'-data', self.prefix+'-backups'
        self.name = self.prefix+'-server'
        with socket.socket() as sock:
            sock.bind(('127.0.0.1',0))
            self.port = sock.getsockname()[1]
        self.url = f'http://127.0.0.1:{self.port}'
        self.cookies = http.cookiejar.CookieJar()
        self.http = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.cookies))
        self.csrf = ''
        for volume in (self.data, self.backups):
            docker('volume','create',volume)

    def options(self):
        return ['--read-only','--cap-drop=ALL','--security-opt=no-new-privileges',
                '--tmpfs','/tmp:size=128m,mode=1777','-v',self.data+':/app/data',
                '-v',self.backups+':/app/backups','-e','PUBLIC_URL='+self.url]

    def start(self):
        docker('run','-d','--name',self.name,*self.options(),'-p',f'127.0.0.1:{self.port}:8080',self.image)
        self.wait()

    def wait(self):
        for _ in range(120):
            try:
                if self.request('/api/health')[0] == 200:
                    return
            except (OSError, urllib.error.URLError):
                pass
            time.sleep(0.25)
        raise RuntimeError('Image did not become healthy within 30 seconds')

    def request(self,path,body=None,headers=None,opener=None):
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(self.url+path,data=data,headers={
            'Content-Type':'application/json','Origin':self.url,'X-CSRF-Token':self.csrf,**(headers or {})})
        try:
            response = (opener or self.http).open(req,timeout=10)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw=response.read()
            try:
                value=json.loads(raw)
            except ValueError:
                value=raw.decode()
            return response.status,value,response.headers

    def stop(self):
        docker('stop','--time','15',self.name)

    def restart(self):
        docker('start',self.name)
        self.wait()

    def copy_db(self, path):
        docker('cp',self.name+':/app/data/finance.sqlite',path)

    def install_db(self,path):
        docker('cp',path,self.name+':/app/data/finance.sqlite')
        # Docker cp uses the host UID. Only this disposable volume is chowned.
        docker('run','--rm','--user','0:0','--entrypoint','chown',*self.options(),'--cap-add=CHOWN',self.image,
               '10001:10001','/app/data/finance.sqlite')

    def offline(self,*command):
        return docker('run','--rm',*self.options(),self.image,*command)

    def cleanup(self):
        subprocess.run(['docker','rm','-f',self.name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        for volume in (self.data,self.backups):
            subprocess.run(['docker','volume','rm',volume],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)

MCP_TOKEN = 'synthetic-release-agent-token'
TABLES = ['users','accounts','grants','transactions','allocations','audit','mcp_tokens','mcp_user_context']

def snapshot(path):
    with sqlite3.connect(path) as db:
        return {table:db.execute('SELECT * FROM '+table+' ORDER BY 1').fetchall() for table in TABLES}

def seed_credentials(db):
    expires=int(time.time())+86400
    token_hash=hashlib.sha256(MCP_TOKEN.encode()).hexdigest()
    db.execute("INSERT INTO mcp_tokens(id,user_id,name,token_hash,permissions,expires_at) VALUES(99,1,'Synthetic release agent',?,?,?)",
               (token_hash,json.dumps({'schema_version':1,'preset':'review','read_context':True}),expires))
    db.execute("INSERT INTO mcp_access_tokens VALUES(?,?,?,?)",(hashlib.sha256(b'synthetic-release-access').hexdigest(),99,'http://example.invalid/api/mcp',expires))
    db.execute("INSERT INTO mcp_refresh_tokens VALUES(?,?,?,?,?,0)",(hashlib.sha256(b'synthetic-release-refresh').hexdigest(),99,'synthetic-client','http://example.invalid/api/mcp',expires))

def initialize(smoke):
    status,value,_=smoke.request('/api/mcp',{'jsonrpc':'2.0','id':1,'method':'initialize',
        'params':{'protocolVersion':'2025-03-26','capabilities':{},'clientInfo':{'name':'synthetic-release','version':'1'}}},
        headers={'Authorization':'Bearer '+MCP_TOKEN,'Accept':'application/json, text/event-stream'})
    if isinstance(value,str) and 'data: ' in value:
        value=json.loads(next(line[6:] for line in value.splitlines() if line.startswith('data: ')))
    return status,value

def fresh(smoke,version,commit):
    smoke.start()
    status,setup,_=smoke.request('/api/setup')
    assert status==200 and setup['required']
    smoke.csrf=setup['csrf']
    status,result,_=smoke.request('/api/setup',{'username':'release-demo','password':'synthetic-release-password'})
    assert status==200, result
    smoke.csrf=result['csrf']
    status,info,headers=smoke.request('/api/build')
    assert status==200 and info['version']==version and info['commit']==commit, info
    assert 'no-store' in headers['Cache-Control']
    assert smoke.request('/')[0]==200
    assert smoke.request('/manifest.webmanifest')[0]==200
    smoke.stop()
    path=smoke.work/'fresh.sqlite'
    smoke.copy_db(path)
    with sqlite3.connect(path) as db:
        assert db.execute('SELECT count(*) FROM categories').fetchone()[0]==0
        assert db.execute('SELECT count(*) FROM sessions').fetchone()[0]==1
        seed_credentials(db)
        db.execute("INSERT INTO mcp_user_context(user_id,content) VALUES(1,'Synthetic context')")
    db.execute('PRAGMA wal_checkpoint(TRUNCATE)')
    db.close()
    smoke.install_db(path)
    smoke.restart()
    assert smoke.request('/api/me')[0]==200, 'Session/persistence lost on normal restart'
    status,value=initialize(smoke)
    assert status==200 and value['result']['serverInfo']['version']==version, value
    # Backup after credential creation, so restore proves both browser and agent invalidation.
    smoke.stop()
    backup=smoke.offline('backup').splitlines()[-1]
    assert backup.startswith('/app/backups/finance-')
    smoke.offline('restore',backup)
    smoke.restart()
    assert smoke.request('/api/me')[0]==401
    assert initialize(smoke)[0]==401
    status,result,_=smoke.request('/api/login',{'username':'release-demo','password':'synthetic-release-password'})
    assert status==200, result
    smoke.stop()
    smoke.copy_db(path)
    with sqlite3.connect(path) as db:
        for table in ('mcp_tokens','mcp_access_tokens','mcp_refresh_tokens'):
            assert db.execute('SELECT count(*) FROM '+table).fetchone()[0]==0
        assert db.execute('SELECT content FROM mcp_user_context').fetchone()[0]=='Synthetic context'
        assert db.execute('PRAGMA integrity_check').fetchone()[0]=='ok'
    print('PASS fresh image: hardened onboarding, metadata/MCP, restart persistence, backup/restore, browser/agent invalidation')

def upgrade(smoke):
    # Frozen real v22 schema; no downgrade of a current schema is used.
    path=smoke.work/'v22.sqlite'
    with sqlite3.connect(path) as db:
        db.executescript((ROOT/'internal/app/testdata/migrations/v22.sql').read_text())
        db.executescript("""
INSERT INTO users(id,username,password,admin,budget_member) VALUES(1,'historical-demo','unused',1,1);
INSERT INTO accounts(id,name,bank_id,household) VALUES(1,'Synthetic private','release-private',0);
INSERT INTO grants VALUES(1,1,'editor');
INSERT INTO categories(id,name,group_name,kind) VALUES(1,'Synthetic expense','Historical','expense');
INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,source_key,provenance,review_state)
VALUES(1,1,'2026-09-01',-12345,'Synthetic purchase','2026-09-01',-12345,'Synthetic original','synthetic-source-key','{"synthetic":true}','approved');
INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(1,1,-12345);
INSERT INTO audit(user_id,account_id,entity,entity_id,action,details) VALUES(1,1,'transaction',1,'synthetic-fixture','{"synthetic":true}');
INSERT INTO mcp_user_context(user_id,content) VALUES(1,'Historical synthetic context');
""")
        seed_credentials(db)
    db.close()
    before=snapshot(path)
    # Create a stopped container to copy the historical database into its new volume.
    docker('create','--name',smoke.name,*smoke.options(),'-p',f'127.0.0.1:{smoke.port}:8080',smoke.image)
    smoke.install_db(path)
    smoke.restart()
    smoke.stop()
    current=smoke.work/'upgraded.sqlite'
    smoke.copy_db(current)
    assert snapshot(current)==before, 'Upgrade changed financial/source/audit/access/consent data'
    with sqlite3.connect(current) as db:
        history=db.execute('SELECT version FROM migrations ORDER BY version').fetchall()
        target=int(re.search(r'const schemaVersion = ([0-9]+)',(ROOT/'internal/app/migrate.go').read_text()).group(1))
        assert (22,) in history
        assert [v for (v,) in history if v>=23]==list(range(23,target+1)), history
        assert db.execute('PRAGMA integrity_check').fetchone()[0]=='ok'
        assert not db.execute('PRAGMA foreign_key_check').fetchall()
    smoke.restart()
    smoke.stop()
    smoke.copy_db(current)
    with sqlite3.connect(current) as db:
        assert db.execute('SELECT version FROM migrations ORDER BY version').fetchall()==history
    assert snapshot(current)==before
    print('PASS production-image v22 upgrade: money/source/audit/grants/MCP consent preserved, integrity and once-only restart')

def main():
    p=argparse.ArgumentParser()
    p.add_argument('image')
    p.add_argument('--version',required=True)
    p.add_argument('--commit',required=True)
    args=p.parse_args()
    info=json.loads(docker('image','inspect',args.image))[0]
    labels=info['Config']['Labels']
    assert info['Architecture']=='amd64' and info['Os']=='linux'
    assert info['Config']['User']=='finance'
    assert labels['org.opencontainers.image.version']==args.version
    assert labels['org.opencontainers.image.revision']==args.commit
    assert labels['org.opencontainers.image.licenses']=='GPL-3.0-only'
    docker('run','--rm','--read-only','--cap-drop=ALL','--security-opt=no-new-privileges',
           '--entrypoint','sh',args.image,'-c',
           'test "$(id -u)" = 10001 && test -s /usr/share/licenses/sente/LICENSE && test -s /usr/share/licenses/sente/NOTICE && ! touch /app/root-write-test')
    with tempfile.TemporaryDirectory(prefix='sente-image-smoke-') as temp:
        work=Path(temp)
        for check in (lambda s:fresh(s,args.version,args.commit),upgrade):
            smoke=Smoke(args.image,work)
            try:
                check(smoke)
            except Exception:
                subprocess.run(['docker','logs',smoke.name],check=False)
                raise
            finally:
                smoke.cleanup()
    print('PASS production image acceptance (linux/amd64; disposable synthetic volumes only)')

if __name__=='__main__':
    main()
