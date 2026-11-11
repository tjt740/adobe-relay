from copy import deepcopy
import json
from pathlib import Path
import tempfile
from unittest import TestCase
from manual_gate import FILES, commands, digest, validate


class ManualGateTests(TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.sha = 'a' * 40
        workflow = Path(__file__).with_name('checked-workflow.yml').read_text()
        data = {name: '' for name in FILES}
        data.update({
            'workflow.yml': workflow,
            'backend-full-gate.sh': commands(workflow, 'Check deployment scripts and backend'),
            'frontend-full-gate.sh': commands(workflow, 'Check frontend workflows', True),
            'frontend-full-gate.log': 'Tests 688 passed\n',
            'image-inspect.json': json.dumps([{'Os': 'linux', 'Architecture': 'amd64', 'Id': 'sha256:test', 'RepoTags': ['adobe-relay:' + self.sha]}]),
            'image-version.txt': 'Sub2API 0.2.19 commit ' + self.sha,
            'local-verification.json': json.dumps({'application_sha': self.sha, 'backend_failed': 0, 'backend_passed_test_cases': 61, 'ss_hop_removal_rejected_direct_fallback': True, 'transport_ciphers': ['a', 'b', 'c', 'd'], 'mihomo_version': 'v1.19.31', 'frontend_types_passed': True, 'frontend_build_passed': True, 'go_vet_passed': True}),
        })
        for name in ['backend', 'frontend', 'image']:
            data[name + '-execution.json'] = json.dumps({'exit_code': 0})
        for name, value in data.items():
            (self.root / name).write_text(value)
        (self.root / 'release-images.tar.gz').write_bytes(b'isolated-test-archive')
        checksum = digest(self.root / 'release-images.tar.gz')
        (self.root / 'release-images.tar.gz.sha256').write_text(checksum + '  release-images.tar.gz\n')
        self.proof = {'mode': 'local_full_checks_exact_build', 'application_sha': self.sha, 'official_workflow_completed': False, 'files': {name: digest(self.root / name) for name in FILES}, 'image_id': 'sha256:test', 'archive_sha256': checksum, 'passed': True}

    def change(self, name, value):
        (self.root / name).write_text(value)
        self.proof['files'][name] = digest(self.root / name)

    def test_accepts_all_complete_checks_and_exact_image(self):
        self.assertTrue(validate(self.proof, self.sha, self.root))

    def test_failed_or_incomplete_execution_is_rejected(self):
        for code in [1, None]:
            self.change('backend-execution.json', json.dumps({'exit_code': code}))
            with self.assertRaises(AssertionError): validate(self.proof, self.sha, self.root)

    def test_removing_check_commands_is_rejected_even_after_rehash(self):
        self.change('backend-full-gate.sh', 'set -euo pipefail\ntrue\n')
        with self.assertRaises(AssertionError): validate(self.proof, self.sha, self.root)

    def test_changed_workflow_or_missing_tests_is_rejected(self):
        self.change('frontend-full-gate.log', 'Tests 687 passed\n')
        with self.assertRaises(AssertionError): validate(self.proof, self.sha, self.root)

    def test_wrong_architecture_or_application_is_rejected(self):
        self.change('image-inspect.json', json.dumps([{'Os':'linux','Architecture':'arm64','Id':'sha256:test','RepoTags':['adobe-relay:'+self.sha]}]))
        with self.assertRaises(AssertionError): validate(self.proof, self.sha, self.root)

    def test_tampered_archive_or_evidence_is_rejected(self):
        (self.root / 'release-images.tar.gz').write_bytes(b'changed')
        with self.assertRaises(AssertionError): validate(self.proof, self.sha, self.root)

    def test_cannot_claim_github_completed(self):
        self.proof['official_workflow_completed'] = True
        with self.assertRaises(AssertionError): validate(self.proof, self.sha, self.root)

    def test_missing_receipt_is_rejected(self):
        del self.proof['files']['backend-execution.json']
        with self.assertRaises(AssertionError): validate(self.proof, self.sha, self.root)
