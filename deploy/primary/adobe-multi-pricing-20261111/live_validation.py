import json,re,struct,zlib,hashlib
def validate_png(data, dimensions):
    assert data[:8]==b'\x89PNG\r\n\x1a\n'
    at=8;compressed=bytearray();ihdr=None;ended=False
    while at<len(data):
        size=struct.unpack('>I',data[at:at+4])[0];kind=data[at+4:at+8];body=data[at+8:at+8+size]
        assert at+12+size<=len(data)
        assert zlib.crc32(kind+body)&0xffffffff==struct.unpack('>I',data[at+8+size:at+12+size])[0]
        if kind==b'IHDR':
            assert ihdr is None and at==8
            ihdr=struct.unpack('>IIBBBBB',body)
        if kind==b'IDAT':compressed.extend(body)
        at+=12+size
        if kind==b'IEND':ended=True;break
    assert ended and at==len(data) and ihdr and ihdr[:2]==dimensions
    w,h,depth,color,compression,filtering,interlace=ihdr
    assert compression==filtering==interlace==0 and depth in (8,16) and color in (0,2,4,6)
    channels={0:1,2:3,4:2,6:4}[color];stride=(w*channels*depth+7)//8+1
    dec=zlib.decompressobj();raw=dec.decompress(compressed,128*1024*1024)
    assert dec.eof and not dec.unconsumed_tail and not dec.unused_data and len(raw)==h*stride
    assert all(raw[row*stride]<=4 for row in range(h))
    return {'format':'png','width':w,'height':h,'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest(),'png_fully_validated':True}

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
