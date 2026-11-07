"""Assert real, distinct quality levels; a 200 response alone never passes."""
import json
import re
MODELS = {'gpt-image-2.6-flare': 'gpt-image-2.6-star-swirl', 'gpt-image-2.6-sunburst': 'gpt-image-2.6-aurora'}
LEVELS = {'xhigh': 6, 'max': 7}

def parse_pipeline_line(line):
    """Accept the existing Zap JSON or console encoder without changing logging."""
    try:
        entry = json.loads(line)
        return entry if isinstance(entry, dict) and entry.get('msg') == 'adobe_images.pipeline_completed' else None
    except (ValueError, TypeError):
        match = re.search(r'(?:^|\t)adobe_images\.pipeline_completed\t(\{.*\})$', line)
        if not match:
            return None
        try:
            entry = json.loads(match.group(1))
        except ValueError:
            return None
        if not isinstance(entry, dict):
            return None
        entry['msg'] = 'adobe_images.pipeline_completed'
        return entry

def pipeline_matches(entry, usage, http_request_id, api_key_id):
    if not entry or entry.get('msg') != 'adobe_images.pipeline_completed':
        return False
    if not http_request_id or entry.get('request_id') != http_request_id:
        return False
    if entry.get('api_key_id') != api_key_id or entry.get('account_id') != usage.get('account_id'):
        return False
    client_id = entry.get('client_request_id')
    billing_id = 'client:' + client_id if client_id else 'local:' + http_request_id
    return usage.get('request_id') == billing_id

def pipeline_evidence(entry):
    return {'request_id': entry['request_id'], 'client_request_id': entry.get('client_request_id'),
            'generation_request_id': entry['generation_request_id'], 'api_key_id': entry['api_key_id'],
            'account_id': entry['account_id'], 'processing_ms': entry['processing_ms'],
            'applied_submissions': [{key: row[key] for key in ('submitted_model_version','submitted_detail_level','submit_attempts')} for row in entry['timings']['images']]}

def validate(cases, sha):
    expected = {(model, quality) for model in MODELS for quality in LEVELS}
    assert len(cases) == 4 and {(r['model'], r['quality']) for r in cases} == expected
    for row in cases:
        assert row['application_sha'] == sha and row['success'] and row['http_status'] == 200
        assert row['outer_posts'] == 1 and row['response_quality'] == row['quality']
        image, = row['images']
        assert image['format'] == 'png' and image['width'] == 3840 and image['height'] == 2160
        assert image['png_fully_validated'] and image['bytes'] > 1024 and len(image['sha256']) == 64
        applied, = row['applied_submissions']
        assert applied['submitted_model_version'] == MODELS[row['model']]
        assert applied['submitted_detail_level'] == LEVELS[row['quality']]
        assert applied['submit_attempts'] == 1
        assert row['pipeline_correlated'] and row['usage']['image_count'] == 1
        assert row['usage']['model'] == row['model']
        assert pipeline_matches(dict(row['pipeline'],msg='adobe_images.pipeline_completed'),row['usage'],row['http_request_id'],row['api_key_id'])
    return {'models': list(MODELS), 'cases': 4, 'all_passed': True, 'actual_levels': [6, 7],
            'real_4k_pngs': 4, 'silent_downgrades': 0, 'outer_posts': 4, 'replayed_requests': 0}
