from pathlib import Path
import json
p=Path('/opt/adobe-relay/releases/subscription-health-ui-20261018/progress.json')
try:
 result=json.loads(p.read_text())
except (FileNotFoundError,json.JSONDecodeError):
 result={'step':'starting-or-updating','state':'running'}
print(json.dumps(result))
