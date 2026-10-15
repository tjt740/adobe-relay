"""Guard the staged image-memory reservation release, preserving the existing 12/9 GiB limits."""
import importlib.util
from pathlib import Path

spec = importlib.util.spec_from_file_location('image_budget_guard', Path(__file__).with_name('adobe-memory-burst-release.py'))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)
release = guard.release
release.RID = 'adobe-memory-staged-20261015'
release.ROOT = Path('/opt/adobe-relay/releases') / release.RID
release.OLD_IMAGE = 'adobe-relay:8db3c34e862dfe07d556ef9b882d0d351113de0f'
release.ROLLBACK_IMAGE = 'adobe-relay:before-' + release.RID
snapshot = guard.snapshot
check_preserved = guard.check_preserved

if __name__ == '__main__':
    release.main()
