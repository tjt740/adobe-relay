"""Guard the image download egress fallback release and code-only rollback."""
import importlib.util
from pathlib import Path

path = Path(__file__).with_name('image-response-release.py')
spec = importlib.util.spec_from_file_location('image_response_release', path)
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)

release.RID = 'adobe-download-fallback-20261015'
release.ROOT = Path('/opt/adobe-relay/releases') / release.RID
release.OLD_IMAGE = 'adobe-relay:5bfea0b97224f2a3dcdf976b2a6857f3ee176a10'
release.ROLLBACK_IMAGE = 'adobe-relay:before-' + release.RID

if __name__ == '__main__':
    release.main()
