"""Finish the explicitly authorized release once the current business batch is idle."""
import datetime
import json
from pathlib import Path
import sqlite3
import subprocess
import time

root = Path('/opt/adobe-relay/releases/okad-company-mobile-mail-20261014')
started = time.monotonic()
while time.monotonic() - started < 5400:
    now = time.time()
    with sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True,timeout=5) as db:
        jobs = db.execute("SELECT id,type,status,target,success,fail FROM job_records WHERE deleted=0 AND status IN ('running','pausing','paused','cancelling')").fetchall()
        recoveries = db.execute("SELECT count(*) FROM integration_recovery_jobs WHERE state IN ('queued','running')").fetchone()[0]
        deliveries = db.execute("SELECT count(*) FROM integration_deliveries WHERE lease_until>? OR (state IN ('pending','retry') AND next_at<=?)",(now,now+60)).fetchone()[0]
    status = {'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'jobs':jobs,'active_recoveries':recoveries,'active_or_imminent_deliveries':deliveries,'state':'waiting'}
    if not jobs and not recoveries and not deliveries:
        # The release helper independently rechecks all locks, identities, source
        # hashes, config and task state immediately before changing any service.
        result = subprocess.run(['python3',str(root/'company-okad-release.py'),'deploy'],capture_output=True,text=True)
        with (root/'okad-deploy.log').open('a') as output:
            output.write(result.stdout+result.stderr)
        if result.returncode==0:
            status['state']='deployed'
        elif 'Wait for current login/delivery tasks:' not in result.stderr:
            status['state']='stopped_for_inspection'
        (root/'release-window.json').write_text(json.dumps(status,indent=2))
        if status['state']!='waiting':
            raise SystemExit(0 if status['state']=='deployed' else 1)
    (root/'release-window.json').write_text(json.dumps(status,indent=2))
    time.sleep(20)
raise SystemExit('No safe release window within 90 minutes; no forced restart performed')
