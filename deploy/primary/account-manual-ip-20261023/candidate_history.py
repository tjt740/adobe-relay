"""Retire only an unstarted candidate, retaining immutable load evidence."""
def archive_candidate(release_root, new_sha):
    import datetime
    import json
    import pathlib
    import re
    import shutil

    root = pathlib.Path(release_root)
    assert re.fullmatch('[0-9a-f]{40}', new_sha)
    assert not (root / 'rolling-prepared.json').exists() and not (root / 'progress.json').exists(), 'Never replace a candidate after startup'
    (root / 'artifact').mkdir(parents=True, exist_ok=True, mode=0o700)
    previous = root / 'image-loaded.json'
    if not previous.exists():
        return
    evidence = json.loads(previous.read_text())
    old = evidence['sha']
    if old == new_sha:
        return
    assert re.fullmatch('[0-9a-f]{40}', old)
    history = root / 'candidate-history' / old
    history.mkdir(parents=True, exist_ok=True, mode=0o700)
    saved = history / previous.name
    if saved.exists():
        assert json.loads(saved.read_text()) == evidence, 'Historical candidate evidence differs'
    else:
        shutil.copy2(previous, saved)
    # A failed upload may already contain the next candidate's proof. Never
    # attach that newer proof to the previously loaded image or rewrite history.
    for name in ['expected-sha', 'expected-run', 'artifact/ci-proof.json']:
        source = root / name
        target = history / source.name
        if target.exists() or not source.exists():
            continue
        if name == 'artifact/ci-proof.json' and json.loads(source.read_text())['application_sha'] != old:
            continue
        if name == 'expected-sha' and source.read_text().strip() != old:
            continue
        if name == 'expected-run' and source.read_text().strip() != str(evidence['workflow_run']):
            continue
        shutil.copy2(source, target)
    attempt = history / ('replacement-attempt-' + new_sha + '.json')
    if not attempt.exists():
        attempt.write_text(json.dumps({'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'previous_sha': old, 'replacement_sha': new_sha, 'application_started': False, 'customer_traffic': False}))
        attempt.chmod(0o600)
    # load-candidate.py retains its rejection of any existing active receipt.
    # The archived receipt above makes this metadata removal recoverable.
    previous.unlink()
