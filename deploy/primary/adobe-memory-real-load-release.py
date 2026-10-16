"""Code-only incident repair guard; preserve the user's later group enablement."""
import importlib.util
from pathlib import Path
spec=importlib.util.spec_from_file_location('guard',Path(__file__).with_name('adobe-memory-burst-release.py'))
guard=importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)
release=guard.release
release.RID='adobe-memory-real-load-20261016'
release.ROOT=Path('/opt/adobe-relay/releases')/release.RID
release.OLD_IMAGE='adobe-relay:c574dcc0dc6369cd05d94b25f75d4ea66c1e4bb1'
release.ROLLBACK_IMAGE='adobe-relay:before-'+release.RID
if __name__=='__main__':release.main()
