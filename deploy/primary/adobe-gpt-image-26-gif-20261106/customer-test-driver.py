"""Run a distinct current-group matrix and always remove its temporary access."""
import importlib.util
from pathlib import Path
import sys

ROOT = Path('/opt/adobe-relay/releases/adobe-gpt-image-26-gif-20261106')


def module(name, file):
    spec = importlib.util.spec_from_file_location(name, ROOT / file)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


access = module('access', 'customer-test-access.py')
public = module('public', 'public-test.py')
assert sys.argv[1] == access.SHA
assert not (ROOT / 'customer-post-release-tests.json').exists()
try:
    access.prepare()
    public.main(sys.argv[1], current_group=True)
finally:
    if (ROOT / 'customer-test-access-started.json').exists():
        access.cleanup()
