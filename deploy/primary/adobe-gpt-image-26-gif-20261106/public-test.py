"""Authorized customer-endpoint matrix; one POST per case, never replay unknown work."""
import base64
import datetime
import hashlib
import json
import os
from pathlib import Path
import struct
import subprocess
import sys
import time
import urllib.error
import urllib.request

ROOT = Path('/opt/adobe-relay/releases/adobe-gpt-image-26-gif-20261106')
MODELS = ['gpt-image-2', 'gpt-image-2.5-flare', 'gpt-image-2.5-sunburst',
          'gpt-image-2.6-star-swirl', 'gpt-image-2.6-aurora']


def image_info(data):
    out = {'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()}
    if data.startswith(b'\x89PNG\r\n\x1a\n'):
        w, h = struct.unpack('>II', data[16:24])
        return dict(out, format='png', width=w, height=h)
    if data.startswith(b'\xff\xd8'):
        i = 2
        while i < len(data):
            assert data[i] == 255
            while data[i] == 255:
                i += 1
            marker = data[i]
            i += 1
            if marker in (0x01, *range(0xD0, 0xD9)):
                continue
            size = int.from_bytes(data[i:i+2], 'big')
            assert size >= 2
            if marker in (0xC0, 0xC1, 0xC2):
                h, w = struct.unpack('>HH', data[i+3:i+7])
                return dict(out, format='jpeg', width=w, height=h)
            i += size
        raise ValueError('JPEG dimensions missing')
    assert data[:6] in (b'GIF87a', b'GIF89a'), 'Unexpected output format'
    w, h = struct.unpack('<HH', data[6:10])
    i = 13 + (3 * (2 ** ((data[10] & 7) + 1)) if data[10] & 128 else 0)
    delays, delay = [], 0

    def skip_blocks(pos):
        while True:
            size = data[pos]
            pos += 1
            if not size:
                return pos
            pos += size
            assert pos <= len(data)

    while True:
        marker = data[i]
        i += 1
        if marker == 0x3B:
            assert delays and i == len(data), 'Incomplete GIF structure'
            return dict(out, format='gif', width=w, height=h, frame_count=len(delays), delays_centiseconds=delays)
        if marker == 0x21:
            label = data[i]
            i += 1
            if label == 0xF9:
                assert data[i] == 4
                delay = int.from_bytes(data[i+2:i+4], 'little')
            i = skip_blocks(i)
        elif marker == 0x2C:
            packed = data[i+8]
            i += 9
            if packed & 128:
                i += 3 * (2 ** ((packed & 7) + 1))
            i += 1  # LZW minimum code size
            i = skip_blocks(i)
            delays.append(delay)
            delay = 0
        else:
            raise ValueError('Invalid GIF block')


def multipart(payload, reference):
    boundary = 'adobe-release-gif-20261106'
    chunks = []
    for key, value in payload.items():
        if key == 'images':
            continue
        chunks.append((f'--{boundary}\r\nContent-Disposition: form-data; name="{key}"\r\n\r\n{value}\r\n').encode())
    chunks += [(f'--{boundary}\r\nContent-Disposition: form-data; name="image[]"; filename="reference.png"\r\nContent-Type: application/octet-stream\r\n\r\n').encode(), reference, b'\r\n', (f'--{boundary}--\r\n').encode()]
    return b''.join(chunks), 'multipart/form-data; boundary=' + boundary


def main(sha):
    os.umask(0o077)
    assert len(sha) == 40 and all(c in '0123456789abcdef' for c in sha)
    assert (ROOT / 'expected-sha').read_text().strip() == sha
    assert (ROOT / 'rolling-finished.json').exists()
    c = json.loads(subprocess.check_output(['docker', 'inspect', 'adobe-relay-app-1']))[0]
    assert c['Config']['Image'] == 'adobe-relay:' + sha
    assert c['State']['Health']['Status'] == 'healthy'
    query = "BEGIN READ ONLY; SELECT key FROM api_keys WHERE id=12 AND user_id=1 AND group_id=7 AND status='active' AND deleted_at IS NULL; COMMIT;"
    key = subprocess.run(['docker', 'exec', '-i', 'adobe-relay-postgres-1', 'psql', '-U', 'sub2api', '-d', 'sub2api', '-Atq', '-v', 'ON_ERROR_STOP=1'], input=query, text=True, capture_output=True, check=True, timeout=25).stdout.strip()
    assert key and len(key) > 10
    reference = (ROOT / 'preflight/matrix-image2-png.png').read_bytes()
    image_url = 'data:image/png;base64,' + base64.b64encode(reference).decode()
    cases = []
    for model in MODELS:
        extra = {'complexity': 'high'} if model.startswith('gpt-image-2.6-') else {}
        for edit in (False, True):
            payload = dict(model=model, prompt='A blue ceramic mug on a plain white background, no text.', n=1, size='1024x1024', quality='low', output_format='jpeg' if edit else 'png', response_format='b64_json', **extra)
            if edit:
                payload['images'] = [{'image_url': image_url}]
            cases.append((model + ('-edit' if edit else '-generate'), '/v1/images/edits' if edit else '/v1/images/generations', payload, False, False))
    for model in MODELS[-2:]:
        frames = 4 if model.endswith('star-swirl') else 16
        payload = dict(model=model, prompt='A tiny bird waves one wing in a looping animation on white, no text.', n=1, quality='low', size='896x896' if frames == 4 else 'auto', output_format='gif', frame_count=frames, frame_duration_ms=200, response_format='url' if frames == 16 else 'b64_json', complexity='medium')
        cases.append((model + '-gif', '/v1/images/generations', payload, False, False))
        for variant in ('json', 'multipart', 'generation-reference'):
            edit_payload = dict(payload, images=[{'image_url': image_url}], response_format='b64_json')
            cases.append((model + '-reject-' + variant, '/v1/images/generations' if variant == 'generation-reference' else '/v1/images/edits', edit_payload, variant == 'multipart', True))
    results = []
    for name, endpoint, payload, form, rejection in cases:
        result_file, marker = ROOT / ('public-' + name + '-result.json'), ROOT / ('public-' + name + '-started.json')
        if result_file.exists():
            out = json.loads(result_file.read_text())
            assert out['application_sha'] == sha
            results.append(out)
            continue
        started = datetime.datetime.now(datetime.timezone.utc).isoformat()
        with os.fdopen(os.open(marker, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'w') as f:
            json.dump({'at': started, 'application_sha': sha, 'case': name, 'api_key_id': 12, 'outer_posts': 1, 'never_replay_if_interrupted': True}, f)
        data, content_type = multipart(payload, reference) if form else (json.dumps(payload).encode(), 'application/json')
        req = urllib.request.Request('https://creative.cheap' + endpoint, data=data, headers={'Authorization': 'Bearer ' + key, 'Content-Type': content_type}, method='POST')
        out = {'at': started, 'case': name, 'model': payload['model'], 'endpoint': endpoint, 'application_sha': sha, 'api_key_id': 12, 'outer_posts': 1, 'expected_rejection': rejection, 'success': False}
        begin = time.monotonic()
        try:
            with urllib.request.urlopen(req, timeout=300) as h:
                out['status'] = h.status
                body = json.load(h)
            assert not rejection and len(body['data']) == 1
            item = body['data'][0]
            if item.get('b64_json'):
                image = base64.b64decode(item['b64_json'], validate=True)
                out['delivery'] = 'b64_json'
            else:
                assert item['url'].startswith('https://')
                with urllib.request.urlopen(item['url'], timeout=60) as h:
                    image = h.read(64 * 1024 * 1024 + 1)
                out['delivery'] = 'url'
            assert 1024 < len(image) <= 64 * 1024 * 1024
            info = image_info(image)
            assert info['format'] == payload['output_format']
            if info['format'] == 'gif':
                assert info['frame_count'] == payload['frame_count']
                assert all(d == 20 for d in info['delays_centiseconds'])
                assert body.get('output_format') == 'gif'
            (ROOT / ('public-' + name + '.' + info['format'])).write_bytes(image)
            out.update(success=True, image=info, response_quality=body.get('quality'))
        except urllib.error.HTTPError as e:
            out['status'] = e.code
            try:
                error = json.loads(e.read(32768)).get('error', {})
                out['error'] = {k: error[k] for k in ['type', 'code', 'param', 'stage', 'source', 'upstream_status', 'request_id'] if k in error}
                out['success'] = bool(rejection and e.code == 400 and error.get('code') == 'gif_edit_unsupported' and error.get('param') == 'image' and error.get('source') == 'request' and error.get('stage') == 'validation')
            except (ValueError, TypeError):
                out['error'] = {'type': 'unparsed_http_error'}
        except Exception as e:
            out['exception'] = type(e).__name__
        finally:
            out['duration_seconds'] = round(time.monotonic() - begin, 3)
            result_file.write_text(json.dumps(out, indent=2))
            results.append(out)
            print(json.dumps(out), flush=True)
    summary = {'application_sha': sha, 'all_passed': len(results) == 18 and all(x['success'] for x in results), 'source': 'Public customer Images API with existing administrator-owned key 12/group 7; no customer keys or account changes', 'cases': results}
    (ROOT / 'post-release-tests.json').write_text(json.dumps(summary, indent=2))
    assert summary['all_passed'], 'Public matrix has failures; inspect evidence without replaying requests'


if __name__ == '__main__':
    main(sys.argv[1])
