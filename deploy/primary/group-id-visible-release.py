"""Guard the group ID default-visibility release; preserve current business data."""
import importlib.util
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "image_response_release", Path(__file__).with_name("image-response-release.py")
)
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)
release.RID = "group-id-visible-20261016"
release.ROOT = Path("/opt/adobe-relay/releases") / release.RID
release.OLD_IMAGE = "adobe-relay:c138d73dc67b358a72ffcc8e09f5d62ff8d791dc"
release.ROLLBACK_IMAGE = "adobe-relay:before-" + release.RID

if __name__ == "__main__":
    release.main()
