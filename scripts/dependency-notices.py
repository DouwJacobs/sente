#!/usr/bin/env python3
"""Collect original license/notice files from installed build dependencies."""
import argparse
import json
from pathlib import Path
import shutil
import subprocess

p=argparse.ArgumentParser()
p.add_argument('kind', choices=['go','web'])
p.add_argument('output', type=Path)
a=p.parse_args()
a.output.mkdir(parents=True, exist_ok=True)
roots=[]
if a.kind == 'go':
    raw=subprocess.check_output(['go','list','-m','-json','all'], text=True)
    decoder=json.JSONDecoder()
    while raw.strip():
        value, end=decoder.raw_decode(raw.lstrip())
        raw=raw.lstrip()[end:]
        if value.get('Dir') and not value.get('Main'):
            roots.append((value['Path']+'@'+value['Version'], Path(value['Dir'])))
else:
    for package in Path('node_modules').rglob('package.json'):
        if package.parent.name == 'node_modules':
            continue
        roots.append((str(package.parent), package.parent))
count=0
for name,root in roots:
    for file in root.iterdir():
        if file.is_file() and file.name.lower().startswith(('license','licence','copying','notice','copyright')):
            target=a.output/name/file.name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(file,target)
            count+=1
if not count:
    raise RuntimeError('No dependency notices collected')
print(f'Collected {count} dependency notice files for {a.kind}')
