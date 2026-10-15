"""Guard the simulated Adobe image usage release and code-only rollback."""
import importlib.util
from pathlib import Path

path = Path(__file__).with_name('image-response-release.py')
spec = importlib.util.spec_from_file_location('image_response_release', path)
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)

release.RID = 'adobe-simulated-usage-20261015'
release.ROOT = Path('/opt/adobe-relay/releases') / release.RID
release.OLD_IMAGE = 'adobe-relay:9ff382000550d0c9010df5c2d5b90c18c950310b'
release.ROLLBACK_IMAGE = 'adobe-relay:before-' + release.RID

if __name__ == '__main__':
    release.main()
