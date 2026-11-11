"""Verify the documented manual-release path without claiming GitHub completed.

All original backend/frontend command blocks must finish successfully. The
unchanged Dockerfile builds the same application revision for linux/amd64; the
image identity, embedded version and archive checksum are checked separately.
Production locks, backup, preservation and rolling drain guards remain required.
"""
import hashlib
import json
from pathlib import Path
import re

WORKFLOW_SHA256 = '9dc5f49cd70b99a9bdffecaab62cf15987f5208c2931e8d73a68712506472c17'
FILES = {
    'workflow.yml', 'backend-full-gate.sh', 'backend-full-gate.log', 'backend-execution.json',
    'frontend-full-gate.sh', 'frontend-full-gate.log', 'frontend-execution.json',
    'local-image-build.log', 'image-execution.json', 'image-inspect.json', 'image-version.txt',
    'local-verification.json',
}


def digest(path):
    h = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def commands(workflow, name, frontend=False):
    lines = workflow.splitlines()
    start = next(i for i, line in enumerate(lines) if '- name: ' + name in line)
    start = next(i for i in range(start, len(lines)) if lines[i].strip() == 'run: |') + 1
    block = []
    for line in lines[start:]:
        if line and not line.startswith('          '):
            break
        block.append(line[10:] if line else '')
    script = 'set -euo pipefail\n' + '\n'.join(block) + '\n'
    if frontend:
        script = script.replace('pnpm --dir frontend', 'npx --yes pnpm@9.15.9 --dir frontend')
    return script


def validate(proof, sha, directory):
    directory = Path(directory)
    assert re.fullmatch('[0-9a-f]{40}', sha)
    assert proof['mode'] == 'local_full_checks_exact_build'
    assert proof['application_sha'] == sha
    assert proof['official_workflow_completed'] is False
    assert set(proof['files']) == FILES, 'Missing or unexpected gate evidence'
    for name, checksum in proof['files'].items():
        assert digest(directory / name) == checksum, 'Changed evidence: ' + name
    assert proof['files']['workflow.yml'] == WORKFLOW_SHA256, 'Release check commands changed'
    workflow = (directory / 'workflow.yml').read_text()
    assert (directory / 'backend-full-gate.sh').read_text() == commands(workflow, 'Check deployment scripts and backend')
    assert (directory / 'frontend-full-gate.sh').read_text() == commands(workflow, 'Check frontend workflows', True)
    for name in ['backend', 'frontend', 'image']:
        assert json.loads((directory / (name + '-execution.json')).read_text())['exit_code'] == 0, name + ' execution failed'
    local = json.loads((directory / 'local-verification.json').read_text())
    assert local['application_sha'] == sha and local['backend_failed'] == 0
    assert local['backend_passed_test_cases'] == 61 and local['ss_hop_removal_rejected_direct_fallback']
    assert len(local['transport_ciphers']) == 4 and local['mihomo_version'] == 'v1.19.31'
    assert local['frontend_types_passed'] and local['frontend_build_passed'] and local['go_vet_passed']
    front = re.sub(r'\x1b\[[0-?]*[ -/]*[@-~]', '', (directory / 'frontend-full-gate.log').read_text())
    assert sum(map(int, re.findall(r'Tests\s+(\d+) passed', front))) == 688
    image = json.loads((directory / 'image-inspect.json').read_text())[0]
    assert image['Os'] == 'linux' and image['Architecture'] == 'amd64'
    assert image['Id'] == proof['image_id'] and 'adobe-relay:' + sha in image['RepoTags']
    version = (directory / 'image-version.txt').read_text()
    assert 'Sub2API 0.2.19' in version and sha in version
    archive = directory / 'release-images.tar.gz'
    assert digest(archive) == proof['archive_sha256']
    assert (directory / 'release-images.tar.gz.sha256').read_text().split() == [proof['archive_sha256'], archive.name]
    assert proof['passed'] is True
    return True
