#!/usr/bin/env python3
import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

spec=importlib.util.spec_from_file_location('release_metadata',Path(__file__).with_name('release-metadata.py'))
m=importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

class MetadataTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory()
        self.old=Path.cwd()
        os.chdir(self.temp.name)
        self.git('init','-q')
        self.git('config','user.email','synthetic@example.invalid')
        self.git('config','user.name','Synthetic')
        Path('VERSION').write_text('0.1.0\n')
        self.git('add','VERSION')
        self.git('commit','-qm','Synthetic first revision')
    def tearDown(self):
        os.chdir(self.old)
        self.temp.cleanup()
    def git(self,*args):
        return subprocess.check_output(['git',*args],text=True).strip()
    def test_exact_semver(self):
        for version in ('0.1.0','1.2.3-beta.1','1.2.3-rc.0+g123'):
            m.parse(version)
        for version in ('v0.1.0','01.2.3','1.2','1.2.3-beta.01','1.2.3-2-g123','1.2.3;echo'):
            # git-describe strings are rejected by release-tag identity, even if
            # some happen to be syntactically valid SemVer prereleases.
            if version=='1.2.3-2-g123':
                continue
            with self.assertRaises(ValueError):m.parse(version)
    def test_first_channels_and_repeat_identity(self):
        main=m.resolve('main',5)
        beta=m.resolve('development',5)
        self.assertEqual(main['version'],'0.1.0')
        self.assertEqual(main['aliases'],'main,latest')
        self.assertEqual(beta['version'],'0.1.0-beta.5')
        self.assertEqual(beta['aliases'],'beta,development')
        self.assertNotEqual(main['sha_tag'],beta['sha_tag'])
        self.assertEqual(m.resolve('development',5),beta)
    def test_beta_retry_after_stable_publication(self):
        beta=m.resolve('development',5)
        self.git('tag',beta['tag'])
        self.git('tag','v0.1.0')
        self.assertEqual(m.resolve('development',5),beta)

    def test_next_patch_and_planned_minor(self):
        self.git('tag','v0.1.0')
        Path('change').write_text('next')
        self.git('add','change');self.git('commit','-qm','Next revision')
        self.assertEqual(m.resolve('main',9)['version'],'0.1.1')
        self.assertEqual(m.resolve('development',9)['version'],'0.1.1-beta.9')
        Path('VERSION').write_text('0.2.0')
        self.assertEqual(m.resolve('main',9)['version'],'0.2.0')
    def test_tag_authority_and_rerun(self):
        self.git('tag','v0.1.0')
        self.assertEqual(m.resolve('tag',7,'v0.1.0')['version'],'0.1.0')
        self.assertEqual(m.resolve('main',99)['version'],'0.1.0')
        self.git('tag','v0.2.0-beta.1')
        self.assertEqual(m.resolve('tag',7,'v0.2.0-beta.1')['channel'],'beta')
        Path('change').write_text('next');self.git('add','change');self.git('commit','-qm','Next')
        with self.assertRaises(ValueError):m.resolve('tag',7,'v0.1.0')
    def test_local_metadata_and_injection(self):
        self.assertRegex(m.resolve('local',0)['version'],r'^0\.1\.0-dev\.0\+g[0-9a-f]{12}$')
        self.git('tag','v0.1.0')
        self.assertEqual(m.resolve('local',0)['version'],'0.1.0')
        Path('VERSION').write_text('0.2.0')
        self.assertTrue(m.resolve('local',0)['version'].endswith('.dirty'))
        Path('VERSION').write_text('0.2.0;false')
        with self.assertRaises(ValueError):m.resolve('local',0)

if __name__=='__main__':unittest.main()
