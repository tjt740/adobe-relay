"""Guard the Adobe prompt/content HTTP 451 code-only release and rollback."""
import importlib.util
from pathlib import Path

path = Path(__file__).with_name('image-response-release.py')
spec = importlib.util.spec_from_file_location('image_response_release', path)
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)

release.RID = 'adobe-prompt-451-20261015'
release.ROOT = Path('/opt/adobe-relay/releases') / release.RID
release.OLD_IMAGE = 'adobe-relay:a2b46f5eac2bd04ddf6dc2d08843ec4843a086ad'
release.ROLLBACK_IMAGE = 'adobe-relay:before-' + release.RID

if __name__ == '__main__':
    release.main()
