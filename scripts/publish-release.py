#!/usr/bin/env python3
"""Promote a tested OCI archive, preserving digests and immutable source identities."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

REPO='DouwJacobs/sente'
IMAGE='docker.io/douwjacobs/sente'

def run(*args, **kwargs):
    return subprocess.check_output(args,text=True,**kwargs).strip()

def inspect(reference, raw=False, public=False):
    result=subprocess.run(['skopeo','inspect',*(['--raw'] if raw else []),*(['--no-creds'] if public else []),reference],text=True,capture_output=True)
    if result.returncode:
        # Authentication, rate limits and transport errors must not be treated as absence.
        if 'manifest unknown' in result.stderr.lower() or 'name unknown' in result.stderr.lower():
            return None
        raise RuntimeError('Registry inspection failed; publication stopped')
    return result.stdout if raw else json.loads(result.stdout)

def checked_identity(info,version,commit):
    labels=info.get('Labels') or {}
    if labels.get('org.opencontainers.image.version')!=version or labels.get('org.opencontainers.image.revision')!=commit:
        raise RuntimeError('Immutable image tag already belongs to a different version/revision')

spec=importlib.util.spec_from_file_location('metadata',Path(__file__).with_name('release-metadata.py'))
metadata=importlib.util.module_from_spec(spec)
spec.loader.exec_module(metadata)

def precedence(version):
    parsed=metadata.parse(version)
    pre=parsed.group(4)
    return (*map(int,parsed.groups()[:3]), 1 if pre is None else 0,
            tuple((0,int(v)) if v.isdigit() else (1,v) for v in pre.split('.')) if pre else ())

def main():
    version=os.environ['SENTE_VERSION'];metadata.parse(version)
    commit=os.environ['SENTE_COMMIT']
    if run('git','rev-parse','HEAD')!=commit or run('git','status','--porcelain','--untracked-files=no'):
        raise RuntimeError('Publication requires a clean exact-source checkout')
    tag=os.environ['RELEASE_TAG']
    if tag!='v'+version:
        raise RuntimeError('Version/tag mismatch')
    channel=os.environ['RELEASE_CHANNEL']
    if channel not in ('stable','beta'):
        raise RuntimeError('Unknown publication channel')
    aliases=os.environ['IMAGE_ALIASES'].split(',')
    expected=['main','latest'] if channel=='stable' else ['beta','development']
    if aliases!=expected:
        raise RuntimeError('Channel aliases do not match the release channel')
    image_tag=os.environ['IMAGE_TAG']
    sha_tag=os.environ['SHA_TAG']
    if image_tag!=version or sha_tag!='sha-'+commit+('-beta' if channel=='beta' else ''):
        raise RuntimeError('Immutable image identity mismatch')
    if run('gh','api',f'repos/{REPO}','--jq','.private')!='false':
        raise RuntimeError('Public image publication requires public corresponding source; repository is still private')
    username=os.environ.get('DOCKERHUB_USERNAME','')
    token=os.environ.get('DOCKERHUB_TOKEN','')
    if not username or not token:
        raise RuntimeError('Configure DOCKERHUB_USERNAME and DOCKERHUB_TOKEN repository secrets')
    # Temporary auth file never enters source, logs, build args, artifacts or provenance.
    with tempfile.TemporaryDirectory(prefix='sente-registry-auth-') as temp:
        os.environ['REGISTRY_AUTH_FILE']=str(Path(temp)/'auth.json')
        subprocess.run(['skopeo','login','--username',username,'--password-stdin','docker.io'],
                       input=token+'\n',text=True,stdout=subprocess.DEVNULL,check=True)
        archive='oci-archive:'+sys.argv[1]
        destination='docker://'+IMAGE+':'+image_tag
        existing=inspect(destination)
        # Check every destination before uploading any immutable tag.
        for alias in aliases:
            current=inspect('docker://'+IMAGE+':'+alias)
            if current:
                old=(current.get('Labels') or {}).get('org.opencontainers.image.version')
                if old and metadata.SEMVER.fullmatch(old) and precedence(old)>precedence(version):
                    raise RuntimeError('Refusing to move a channel to an older version')
        prior_sha=inspect('docker://'+IMAGE+':'+sha_tag)
        if prior_sha:
            checked_identity(prior_sha,version,commit)
        if existing:
            checked_identity(existing,version,commit)
            source='docker://'+IMAGE+'@'+existing['Digest']
            # A rebuild may have different provenance/base layers. Reruns reuse and
            # re-test the original published digest instead of overwriting its version.
            run('docker','pull',IMAGE+'@'+existing['Digest'])
            subprocess.run(['python3','scripts/test-release-image.py',IMAGE+'@'+existing['Digest'],
                            '--version',version,'--commit',commit],check=True)
        else:
            subprocess.run(['skopeo','copy','--all','--preserve-digests',archive,destination],check=True)
            raw=inspect(archive,raw=True)
            expected_digest='sha256:'+hashlib.sha256(raw.encode()).hexdigest()
            published=inspect(destination)
            if published['Digest']!=expected_digest:
                raise RuntimeError('Published digest differs from the verified OCI archive')
            source='docker://'+IMAGE+'@'+published['Digest']
        info=inspect(source)
        digest=info['Digest']
        # Public distribution must work without the publisher's registry credentials.
        anonymous=inspect(source,public=True)
        if not anonymous or anonymous['Digest']!=digest:
            raise RuntimeError('The verified image must be publicly readable before completing publication')
        if prior_sha and prior_sha['Digest']!=digest:
            raise RuntimeError('Immutable SHA alias has a different digest')
        if not prior_sha:
            subprocess.run(['skopeo','copy','--all','--preserve-digests',source,'docker://'+IMAGE+':'+sha_tag],check=True)
        # Git tags never move. The tag is created only after image acceptance/upload.
        ref=subprocess.run(['gh','api',f'repos/{REPO}/git/ref/tags/{tag}'],text=True,capture_output=True)
        if ref.returncode==0:
            object=json.loads(ref.stdout)['object']
            # Annotated tags from a manual push resolve through the checked-out Git tag.
            if run('git','rev-parse',tag+'^{}')!=commit:
                raise RuntimeError('Release tag already identifies a different revision')
        elif '404' in ref.stderr:
            run('gh','api',f'repos/{REPO}/git/refs','-f','ref=refs/tags/'+tag,'-f','sha='+commit)
        else:
            raise RuntimeError('Cannot verify/create the immutable release tag')
        output=Path('work/release');output.mkdir(parents=True,exist_ok=True)
        archive_path=output/(tag+'-source.tar.gz')
        subprocess.run(['git','archive','--format=tar.gz','--prefix=sente-'+version+'/',
                        '-o',str(archive_path),commit],check=True)
        (output/'SHA256SUMS').write_text(hashlib.sha256(archive_path.read_bytes()).hexdigest()+'  '+archive_path.name+'\n')
        evidence={'version':version,'commit':commit,'image':IMAGE+'@'+digest,'digest':digest,
                  'platforms':['linux/amd64'],'channel':channel,'source':archive_path.name,
                  'verification_run':os.environ.get('GITHUB_RUN_ID')}
        (output/'image.json').write_text(json.dumps(evidence,indent=2)+'\n')
        generated=json.loads(run('gh','api',f'repos/{REPO}/releases/generate-notes','-f','tag_name='+tag,'-f','target_commitish='+commit))['body']
        notes=f"""{generated}

## Verified distribution

- Version: `{version}`; source revision: `{commit}`.
- Image: `douwjacobs/sente@{digest}` (linux/amd64 only).
- Immutable tags: `{image_tag}`, `{sha_tag}`.
- GPL-3.0-only; dependency licenses remain applicable. Source archive and checksum attached.
- SBOM and build provenance are attached to the OCI image index.
- Source verification and disposable hardened image onboarding, restart, recovery,
  credential invalidation and schema-22 upgrade checks passed in this publication run.

Read [upgrade and recovery](https://github.com/{REPO}/blob/{commit}/docs/RELEASES.md)
before updating. Restore a pre-upgrade database when reverting a schema upgrade.
Live FNB/external MCP clients and physical devices are not verified by synthetic CI.
"""
        notes_path=output/'release-notes.md';notes_path.write_text(notes)
        release=subprocess.run(['gh','release','view',tag,'--repo',REPO],capture_output=True,text=True)
        if release.returncode:
            if 'release not found' not in release.stderr.lower():
                raise RuntimeError('Cannot inspect GitHub release')
            subprocess.run(['gh','release','create',tag,'--repo',REPO,'--verify-tag','--title','Sente '+version,
                '--notes-file',str(notes_path),*(['--prerelease','--latest=false'] if channel=='beta' else ['--latest']),
                str(archive_path),str(output/'SHA256SUMS'),str(output/'image.json')],check=True)
        else:
            # Recover an interrupted asset upload without editing historical release notes.
            assets=json.loads(run('gh','release','view',tag,'--repo',REPO,'--json','assets'))['assets']
            present={asset['name'] for asset in assets}
            missing=[str(file) for file in (archive_path,output/'SHA256SUMS',output/'image.json') if file.name not in present]
            if missing:
                subprocess.run(['gh','release','upload',tag,'--repo',REPO,*missing],check=True)
        for alias in aliases:
            subprocess.run(['skopeo','copy','--all','--preserve-digests',source,'docker://'+IMAGE+':'+alias],check=True)
            if inspect('docker://'+IMAGE+':'+alias)['Digest']!=digest:
                raise RuntimeError('Channel digest did not match the verified image')
        print('Published verified '+version+' at '+digest)

if __name__=='__main__':main()
