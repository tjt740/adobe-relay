"""Guard the second image-memory repair, preserving 12/9 GiB and business state."""
import importlib.util
from pathlib import Path

spec = importlib.util.spec_from_file_location('image_response_release', Path(__file__).with_name('image-response-release.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)
release.RID = 'adobe-memory-burst-20261015'
release.ROOT = Path('/opt/adobe-relay/releases') / release.RID
release.OLD_IMAGE = 'adobe-relay:f16489d9a0ed4f9b4ebcd78816e6bb157d582fe8'
release.ROLLBACK_IMAGE = 'adobe-relay:before-' + release.RID
_original_snapshot = release.snapshot
_original_check = release.check_preserved


def snapshot():
    out = _original_snapshot()
    app = release.inspect(release.APP)
    out['memory_limits'] = {
        'memory': app['HostConfig']['Memory'],
        'memory_swap': app['HostConfig']['MemorySwap'],
        'go_memory_limit': next(v.split('=', 1)[1] for v in app['Config']['Env'] if v.startswith('GOMEMLIMIT=')),
    }
    assert out['memory_limits'] == {'memory': 12 * 1024**3, 'memory_swap': 24 * 1024**3, 'go_memory_limit': '9216MiB'}, 'Review unexpected memory configuration'
    return out


def check_preserved(before, current):
    checks = _original_check(before, current)
    assert current['memory_limits'] == before['memory_limits'], 'Memory configuration changed'
    checks['memory_limits_preserved'] = True
    return checks


release.snapshot = snapshot
release.check_preserved = check_preserved

if __name__ == '__main__':
    release.main()
