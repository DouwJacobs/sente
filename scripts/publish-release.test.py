#!/usr/bin/env python3
"""Publication boundary checks; no live registry or GitHub writes."""
import importlib.util
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

spec=importlib.util.spec_from_file_location('publisher',Path(__file__).with_name('publish-release.py'))
p=importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)

class PublicationTests(unittest.TestCase):
    def test_missing_manifest_is_distinct_from_registry_failure(self):
        for error in ('unauthorized: incorrect credentials','TLS handshake timeout','toomanyrequests'):
            with patch.object(subprocess,'run',return_value=subprocess.CompletedProcess([],1,'',error)):
                with self.assertRaises(RuntimeError):p.inspect('docker://example.invalid/image:v1')
        with patch.object(subprocess,'run',return_value=subprocess.CompletedProcess([],1,'','manifest unknown')):
            self.assertIsNone(p.inspect('docker://example.invalid/image:v1'))
    def test_immutable_identity_requires_both_version_and_full_commit(self):
        info={'Labels':{'org.opencontainers.image.version':'0.1.0','org.opencontainers.image.revision':'abc'}}
        p.checked_identity(info,'0.1.0','abc')
        for version,commit in [('0.1.1','abc'),('0.1.0','different')]:
            with self.assertRaises(RuntimeError):p.checked_identity(info,version,commit)
    def test_semver_channel_order_handles_numeric_prereleases(self):
        versions=['0.1.0-beta.2','0.1.0-beta.12','0.1.0-rc.1','0.1.0','0.1.1-beta.1','0.1.1']
        self.assertEqual(sorted(reversed(versions),key=p.precedence),versions)
    def test_public_access_explicitly_bypasses_publisher_credentials(self):
        with patch.object(subprocess,'run',return_value=subprocess.CompletedProcess([],0,'{}','')) as call:
            p.inspect('docker://example.invalid/image@sha256:abc',public=True)
            self.assertIn('--no-creds',call.call_args.args[0])

    def test_raw_manifest_is_not_reformatted_before_digest(self):
        raw='{"schemaVersion":2,"manifests":[]} '
        with patch.object(subprocess,'run',return_value=subprocess.CompletedProcess([],0,raw,'')):
            self.assertEqual(p.inspect('oci-archive:/tmp/synthetic.tar',raw=True),raw)

if __name__=='__main__':unittest.main()
