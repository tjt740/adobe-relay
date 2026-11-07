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

CASES = [
 {'name':'flare-xhigh','model':'gpt-image-2.6-flare','quality':'xhigh','effective':'xhigh','size':'3840x2160','tier':'4K','price':0.031},
 {'name':'flare-max','model':'gpt-image-2.6-flare','quality':'max','effective':'max','size':'3840x2160','tier':'4K','price':0.042},
 {'name':'sunburst-xhigh','model':'gpt-image-2.6-sunburst','quality':'xhigh','effective':'xhigh','size':'3840x2160','tier':'4K','price':0.053},
 {'name':'sunburst-max','model':'gpt-image-2.6-sunburst','quality':'max','effective':'max','size':'3840x2160','tier':'4K','price':0.064},
 {'name':'flare-max-zero','model':'gpt-image-2.6-flare','quality':'max','effective':'max','size':'2048x2048','tier':'2K','price':0.0},
 {'name':'sunburst-auto-inherit','model':'gpt-image-2.6-sunburst','quality':'auto','effective':'medium','size':'1024x1024','tier':'1K','price':0.09},
]
PRICES = {model:{size:{quality:(mi*15+si*5+qi+1)/1000 for qi,quality in enumerate(['low','medium','high','xhigh','max'])} for si,size in enumerate(['1K','2K','4K'])} for mi,model in enumerate(MODELS)}
for case in CASES:
    if case['quality']=='auto':del PRICES[case['model']][case['tier']][case['effective']]
    else:PRICES[case['model']][case['tier']][case['effective']]=case['price']

def validate(cases, sha):
    assert len(cases)==len(CASES) and {r['name'] for r in cases}=={c['name'] for c in CASES}
    for row in cases:
        case=next(c for c in CASES if c['name']==row['name'])
        assert row['application_sha']==sha and row['success'] and row['http_status']==200
        assert row['outer_posts']==1 and row['response_quality']==case['effective']
        image,=row['images'];width,height=map(int,case['size'].split('x'))
        assert image['format']=='png' and (image['width'],image['height'])==(width,height)
        assert image['png_fully_validated'] and image['bytes']>1024 and len(image['sha256'])==64
        applied,=row['applied_submissions']
        assert applied['submitted_model_version']==MODELS[case['model']]
        assert applied['submitted_detail_level']=={'xhigh':6,'max':7,'medium':3}[case['effective']]
        assert applied['submit_attempts']==1
        usage=row['usage']
        assert row['pipeline_correlated'] and usage['image_count']==1 and usage['model']==case['model']
        assert usage['image_size']==case['tier'] and usage['image_quality']==case['effective']
        assert abs(usage['total_cost']-case['price'])<1e-8 and abs(usage['actual_cost']-case['price']*2)<1e-8
        assert usage['rate_multiplier']==2
        assert pipeline_matches(dict(row['pipeline'],msg='adobe_images.pipeline_completed'),usage,row['http_request_id'],row['api_key_id'])
    return {'models':list(MODELS),'cases':6,'all_passed':True,'actual_levels':[3,6,7],
            'real_4k_pngs':4,'real_2k_pngs':1,'real_1k_pngs':1,'silent_downgrades':0,'outer_posts':6,
            'replayed_requests':0,'independent_prices':True,'unset_price_inherits':True,'zero_price_free':True,'actual_quality_recorded':True}
