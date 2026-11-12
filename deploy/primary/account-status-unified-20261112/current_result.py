"""Read-only verifier of the four result labels in returned account snapshots."""
from datetime import datetime

def obj(value): return value if isinstance(value,dict) else {}
def code(value):
 try:return int(value or 0)
 except (ValueError,TypeError):return 0
def stamp(value):
 try:return int(datetime.fromisoformat(value.replace('Z','+00:00')).timestamp()*1000)
 except (ValueError,TypeError,AttributeError):return 0

def result_status(account):
 extra=obj(account.get('extra'));adobe=account.get('platform')=='adobe'
 if adobe and obj(extra.get('okad_connection_test')).get('state') in ['pending','running','proxy_pending']:return None
 latest=obj(obj(extra.get('adobe_health')).get('latest'))
 if adobe:
  if isinstance(latest.get('success'),bool) and stamp(latest.get('checked_at'))>stamp(extra.get('last_check_at')):
   test=latest.get('source') in ['test','okad']
   return 'test_passed' if latest['success'] else ('test_failed' if test else 'request_failed')
  if extra.get('last_check_success') is True:return 'test_passed'
 # The legacy label is hidden by current authentication/timeout errors.
 import re,json
 error=account.get('error_message') or ''
 if not error and stamp(account.get('temp_unschedulable_until'))>int(datetime.now().timestamp()*1000):error=account.get('temp_unschedulable_reason') or ''
 try:reason=obj(json.loads(error))
 except (ValueError,TypeError):reason={}
 auth=(adobe and bool(re.search(r'adobe cookie is no longer valid|\binvalid_credentials\b',error,re.I))) or bool(re.search(r'(?:^\s*|\(|\b(?:http|oauth|status(?:\s+code)?|returned|rejected|upstream|failed)\s*[:=]?\s*\(?)401\b',error,re.I)) or code(reason.get('status_code'))==401
 if not error:auth=any(code(extra.get(k))==401 for k in ['last_status_code','status_code','upstream_status_code','adobe_last_status_code'])
 timeout=bool(re.search(r'\b408\b',error)) or code(extra.get('last_check_status_code'))==408
 if auth or timeout:return None
 if extra.get('last_check_success') is True:return 'test_passed'
 if extra.get('last_check_success') is False:return 'test_failed'
 return None
