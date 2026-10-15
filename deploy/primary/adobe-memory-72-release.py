"""Guard the 7.2 GiB image budget release, preserving the existing 12/9 GiB limits."""
import importlib.util
from pathlib import Path

spec = importlib.util.spec_from_file_location('image_budget_guard', Path(__file__).with_name('adobe-memory-burst-release.py'))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)
release = guard.release
release.RID = 'adobe-memory-72-20261015'
release.ROOT = Path('/opt/adobe-relay/releases') / release.RID
release.OLD_IMAGE = 'adobe-relay:6868148a26779e94c097eb9626cdd51233a5ac98'
release.ROLLBACK_IMAGE = 'adobe-relay:before-' + release.RID
snapshot = guard.snapshot
check_preserved = guard.check_preserved

if __name__ == '__main__':
    release.main()
