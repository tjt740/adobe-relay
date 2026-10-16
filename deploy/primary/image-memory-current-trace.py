"""Export only size/timing metadata for the reported 2026-10-16 traffic window."""
import datetime, json
from pathlib import Path

start = datetime.datetime.fromisoformat('2026-10-16T11:55:00+08:00')
end = datetime.datetime.fromisoformat('2026-10-16T12:07:00+08:00')
requests = {}
completions = {}
path = Path('/opt/adobe-relay/shared/data/logs/sub2api.log')
with path.open('rb') as f:
    f.seek(max(0, path.stat().st_size - 150_000_000))
    for raw in f:
        if b'security_audit.gateway_check_start' not in raw and b'adobe_images.pipeline_completed' not in raw:
            continue
        try:
            line = raw.decode('utf-8')
            at = datetime.datetime.fromisoformat(line.split('\t', 1)[0])
            row = json.loads(line[line.index('{'):])
        except (ValueError, UnicodeError):
            continue
        request_id = row.get('request_id')
        if not request_id:
            continue
        if 'security_audit.gateway_check_start' in line and start <= at < end and row.get('path', '').endswith('/images/edits'):
            requests.setdefault(request_id, {'at': at, 'audit_bytes': int(row.get('body_bytes', 0))})
        elif 'adobe_images.pipeline_completed' in line:
            completions[request_id] = {'duration_ms': int(row.get('processing_ms', 0)), 'image_count': int(row.get('image_count', 1)), 'legacy': not row.get('timings', {}).get('disk_buffer', False)}

out = []
for request_id, value in sorted(requests.items(), key=lambda r: r[1]['at']):
    matched = completions.get(request_id)
    out.append({'offset_ms': int((value['at']-start).total_seconds()*1000), 'audit_bytes': value['audit_bytes'],
                'duration_ms': max(1, matched['duration_ms']) if matched else 180000,
                'duration_observed': bool(matched), 'image_count': matched['image_count'] if matched else 1,
                'legacy': matched['legacy'] if matched else None})
assert len(out) > 400, 'The fixed incident window is no longer in the current log; use the saved sanitized trace'
print(json.dumps({'window_start': start.isoformat(), 'window_end': end.isoformat(),
                  'note': 'Size-only synthetic replay. Audit JSON bytes are not HTTP wire bytes; unmatched durations use 180 seconds. No bodies, URLs, IDs, keys or account identifiers exported.',
                  'requests': out}))
