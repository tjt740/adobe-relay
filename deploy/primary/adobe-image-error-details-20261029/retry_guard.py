"""Read-only source invariants for the paid-generation submission budget."""
import re
import subprocess


def function(source, signature):
    start = source.index(signature)
    following = re.search(r'(?m)^func ', source[start + len(signature):])
    return source[start:start + len(signature) + following.start()] if following else source[start:]


def validate_retry_guards(base, candidate):
    def read(ref, path):
        return subprocess.check_output(['git', 'show', ref + ':' + path], text=True)
    path = 'backend/internal/pkg/adobe/route_retry.go'
    assert read(base, path) == read(candidate, path), 'Single submission and explicit rejection rules changed'
    for path, signature in [
        ('backend/internal/pkg/adobe/client.go', 'func (c *Client) postSubmit('),
        ('backend/internal/pkg/adobe/errors.go', 'func IsRotatable('),
        ('backend/internal/pkg/adobe/errors.go', 'func IsRetryableStatus('),
        ('backend/internal/handler/adobe_images.go', 'func (h *GatewayHandler) runAdobeImagesFailover('),
    ]:
        assert function(read(base, path), signature) == function(read(candidate, path), signature), 'Submission guard changed: ' + signature
    path = 'backend/internal/service/adobe_error_classifier.go'
    def accepted_guard(source):
        start = source.index('\t\tvar operation *adobe.OperationError')
        return source[start:source.index('\t}()', start)]
    assert accepted_guard(read(base, path)) == accepted_guard(read(candidate, path)), 'Accepted/unknown result stop guard changed'


if __name__ == '__main__':
    import sys
    validate_retry_guards(*sys.argv[1:])
