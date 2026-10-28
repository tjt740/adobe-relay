import collections
import datetime
import hashlib
import json
import re
import subprocess
import urllib.error
import urllib.request


def command(args, data=None, timeout=25):
    result = subprocess.run(args, input=data, text=True, capture_output=True, timeout=timeout)
    if result.returncode:
        raise RuntimeError('command_failed:' + args[0] + ':' + str(result.returncode))
    return result.stdout


def sql(query):
    return command(
        ['docker', 'exec', '-i', 'adobe-relay-postgres-1', 'psql', '-U', 'sub2api', '-d', 'sub2api', '-Atq', '-v', 'ON_ERROR_STOP=1'],
        "BEGIN READ ONLY; SET LOCAL statement_timeout='8s'; " + query + '; COMMIT;',
    ).strip()


def runtime():
    value = json.loads(command(['docker', 'inspect', 'adobe-relay-app-1']))[0]
    return {
        'image': value['Config']['Image'],
        'image_id': value['Image'],
        'started_at': value['State']['StartedAt'],
        'restarts': value['RestartCount'],
    }


before = runtime()
report = {
    'checked_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'runtime': before,
    'storage_setting': json.loads(sql("""SELECT json_build_object(
      'saved_setting_exists', EXISTS(SELECT 1 FROM settings WHERE key='image_storage_config'),
      'saved_enabled', (SELECT value::jsonb->'enabled' FROM settings WHERE key='image_storage_config'),
      'reuse_backup_s3', (SELECT value::jsonb->'reuse_backup_s3' FROM settings WHERE key='image_storage_config'))""")),
    'production_writes': 0,
    'generation_requests': 0,
}

# The key and complete response stay only in this process. Emit flags, never credentials.
key = sql("SELECT value FROM settings WHERE key='admin_api_key'")
if key:
    request = urllib.request.Request(
        'http://127.0.0.1:6666/api/v1/admin/backups/image-storage',
        headers={'x-api-key': key, 'X-Request-ID': 'image-response-default-readonly-20261028'},
        method='GET',
    )
    try:
        with urllib.request.urlopen(request, timeout=15) as response:
            body = json.load(response)
        data = body.get('data') or {}
        cfg = data.get('config') or {}
        report['running_app_storage_config'] = {
            'http_status': 200,
            'enabled': cfg.get('enabled'),
            'reuse_backup_s3': cfg.get('reuse_backup_s3'),
            'bucket_configured': bool(cfg.get('bucket')),
            'access_key_configured': bool(cfg.get('access_key_id')),
            'secret_configured': data.get('secret_configured'),
            'public_base_url_configured': bool(cfg.get('public_base_url')),
        }
    except urllib.error.HTTPError as error:
        report['running_app_storage_config'] = {'http_status': error.code}
    except Exception as error:
        report['running_app_storage_config'] = {'error_class': type(error).__name__}
else:
    report['running_app_storage_config'] = {'error': 'admin_key_unavailable'}

logs = command(['docker', 'logs', '--since', '2h', '--tail', '15000', 'adobe-relay-app-1'], timeout=25)
deliveries = re.findall(r'(?i)(?:"delivery"\s*:\s*"|delivery[=:]\s*)(b64_json|storage_url|upstream_url)', logs)
report['recent_delivery_counts'] = dict(collections.Counter(deliveries))
report['recent_log_lines_examined'] = len(logs.splitlines())
report['recent_storage_warning_counts'] = {
    name: logs.count(name) for name in [
        'image_storage.settings_load_failed',
        'image_storage.client_build_failed',
        'image_storage is enabled but not fully configured',
    ]
}
report['runtime_unchanged'] = before == runtime()
print(json.dumps(report, ensure_ascii=False, indent=2))
