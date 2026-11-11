import json,re,subprocess

def validate_webp(data, dimensions, decoder):
    result=subprocess.run([str(decoder)],input=data,capture_output=True,timeout=35,check=True)
    image=json.loads(result.stdout)
    assert image["fully_decoded"] and (image["width"],image["height"])==dimensions
    return image

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
