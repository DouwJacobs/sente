#!/usr/bin/env python3
"""Load a candidate for smoke tests and prove its filesystem and runtime settings match the OCI platform."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile


def candidate_config(archive):
    with tarfile.open(archive) as tar:
        def blob(digest):
            algorithm,value=digest.split(':',1)
            if algorithm!='sha256' or len(value)!=64 or any(c not in '0123456789abcdef' for c in value):
                raise ValueError('Unexpected OCI digest')
            raw=tar.extractfile('blobs/sha256/'+value).read()
            if hashlib.sha256(raw).hexdigest()!=value:
                raise ValueError('OCI blob content does not match its digest')
            return json.loads(raw)
        root=json.load(tar.extractfile('index.json'))
        # Buildx archives wrap the attested image index in an OCI-layout index.
        index=blob(root['manifests'][0]['digest'])
        if 'manifests' in index:
            selected=[m for m in index['manifests'] if m.get('platform')=={'architecture':'amd64','os':'linux'}]
            if len(selected)!=1:
                raise ValueError('Expected exactly one linux/amd64 platform')
            manifest=blob(selected[0]['digest'])
        else:
            manifest=index
        digest=manifest['config']['digest']
        return digest,blob(digest)

def main():
    p=argparse.ArgumentParser()
    p.add_argument('archive',type=Path)
    p.add_argument('image')
    a=p.parse_args()
    digest,candidate=candidate_config(a.archive)
    with tempfile.TemporaryDirectory(prefix='sente-image-load-') as temp:
        archive=Path(temp)/'docker.tar'
        subprocess.run(['skopeo','copy','--format','v2s2','oci-archive:'+str(a.archive),
                        'docker-archive:'+str(archive)+':'+a.image],check=True)
        subprocess.run(['docker','load','-i',str(archive)],check=True)
    loaded=json.loads(subprocess.check_output(['docker','image','inspect',a.image]))[0]
    if loaded['RootFS']['Layers']!=candidate['rootfs']['diff_ids']:
        raise RuntimeError('Loaded filesystem differs from the candidate platform')
    if any(loaded['Config'].get(key)!=value for key,value in candidate['config'].items()):
        raise RuntimeError('Loaded runtime settings differ from the candidate platform')
    if loaded['Architecture']!=candidate['architecture'] or loaded['Os']!=candidate['os']:
        raise RuntimeError('Loaded architecture differs from the candidate platform')
    # Docker may normalize config JSON and change its hash during Docker-archive
    # import. The uncompressed filesystem hashes and every runtime field must match.
    print('Loaded candidate filesystem/settings verified against OCI config '+digest)

if __name__=='__main__':main()
