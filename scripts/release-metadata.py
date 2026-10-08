#!/usr/bin/env python3
"""Resolve reproducible SemVer and image identities from the checked-out Git revision."""
import argparse
import json
import os
import re
import subprocess
from pathlib import Path

NUMBER = r"(?:0|[1-9][0-9]*)"
IDENT = r"(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)"
SEMVER = re.compile(rf"^({NUMBER})\.({NUMBER})\.({NUMBER})(?:-({IDENT}(?:\.{IDENT})*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$")

def parse(version):
    match = SEMVER.fullmatch(version)
    if not match:
        raise ValueError(f"Invalid SemVer: {version}")
    return match

def stable_key(version):
    return tuple(map(int, parse(version).groups()[:3]))

def git(*args):
    return subprocess.check_output(['git', *args], text=True).strip()

def resolve(mode, run_number, ref=''):
    commit = git('rev-parse', 'HEAD')
    tags = git('tag', '--list').splitlines()
    stable = [t[1:] for t in tags if t.startswith('v') and SEMVER.fullmatch(t[1:])
              and not parse(t[1:]).group(4) and not parse(t[1:]).group(5)]
    target = Path('VERSION').read_text().strip()
    match = parse(target)
    if match.group(4) or match.group(5):
        raise ValueError('VERSION must contain a stable SemVer target')
    latest = max(stable, key=stable_key, default=None)
    # A declared minor/major target takes precedence; otherwise advance the patch.
    if latest and stable_key(target) <= stable_key(latest):
        major, minor, patch = stable_key(latest)
        target = f'{major}.{minor}.{patch + 1}'
    if mode == 'tag':
        if not ref.startswith('v'):
            raise ValueError('Release tags must start with v')
        version = ref[1:]
        parsed = parse(version)
        if parsed.group(5):
            raise ValueError('Release tags may not contain build metadata; use prerelease identifiers')
        if git('rev-parse', ref + '^{}') != commit:
            raise ValueError('Tag does not identify the checked-out revision')
        channel = 'beta' if parsed.group(4) else 'stable'
        # Prevent an older manual tag from rolling mutable channels backward.
        if latest and stable_key(version) < stable_key(latest):
            raise ValueError('Release would downgrade the stable line')
    elif mode == 'main':
        exact = [t[1:] for t in git('tag', '--points-at', 'HEAD').splitlines()
                 if t.startswith('v') and t[1:] in stable]
        if len(exact) > 1:
            raise ValueError('Multiple stable versions identify this revision')
        version = exact[0] if exact else target
        if latest and stable_key(version) < stable_key(latest):
            raise ValueError('Cannot republish an older main revision')
        channel = 'stable'
    elif mode == 'development':
        # A later stable publication must not change an interrupted beta run's identity.
        exact = [t[1:] for t in git('tag', '--points-at', 'HEAD').splitlines()
                 if t.startswith('v') and SEMVER.fullmatch(t[1:])
                 and parse(t[1:]).group(4) == f'beta.{run_number}'
                 and not parse(t[1:]).group(5)]
        if len(exact) > 1:
            raise ValueError('Multiple beta versions identify this publication run')
        version = exact[0] if exact else f'{target}-beta.{run_number}'
        channel = 'beta'
    else:
        exact = [t[1:] for t in git('tag', '--points-at', 'HEAD').splitlines()
                 if t.startswith('v') and SEMVER.fullmatch(t[1:])]
        dirty = bool(git('status', '--porcelain', '--untracked-files=no'))
        version = sorted(exact)[-1] if exact and not dirty else f'{target}-dev.{run_number}+g{commit[:12]}'
        if dirty:
            version += '.dirty'
        channel = 'local'
    parse(version)
    aliases = ['main', 'latest'] if channel == 'stable' else ['beta', 'development'] if channel == 'beta' else []
    sha_tag = 'sha-' + commit + ('-beta' if channel == 'beta' else '')
    return {'version': version, 'commit': commit, 'built_at': git('show', '-s', '--format=%cI', 'HEAD'),
            'tag': 'v' + version, 'channel': channel, 'docker_tag': version.replace('+', '_'),
            'sha_tag': sha_tag, 'aliases': ','.join(aliases)}

def main():
    p = argparse.ArgumentParser()
    p.add_argument('--mode', choices=['main', 'development', 'tag', 'local'], default='local')
    p.add_argument('--run-number', type=int, default=0)
    p.add_argument('--ref', default='')
    p.add_argument('--output', choices=['json', 'github', 'env'], default='json')
    args = p.parse_args()
    if args.run_number < 0:
        p.error('Run number must be nonnegative')
    data = resolve(args.mode, args.run_number, args.ref)
    if args.output == 'github':
        with open(os.environ['GITHUB_OUTPUT'], 'a') as out:
            for key, value in data.items():
                out.write(f'{key}={value}\n')
    elif args.output == 'env':
        for key in ('version', 'commit', 'built_at'):
            name = {'version':'SENTE_VERSION', 'commit':'SENTE_COMMIT', 'built_at':'SENTE_BUILD_TIME'}[key]
            print(f'{name}={data[key]}')
    else:
        print(json.dumps(data, indent=2))

if __name__ == '__main__':
    main()
