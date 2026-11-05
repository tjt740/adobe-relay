"""Run one administrator-selected Adobe model test for one existing account."""
import datetime
import hashlib
import json
import os
import pathlib
import subprocess
import sys
import time
import urllib.error
import urllib.request


email, model, expected_sha = sys.argv[1:]
assert model in ("gpt-image-2.6-star-swirl", "gpt-image-2.6-aurora")
assert len(expected_sha) == 40 and all(c in "0123456789abcdef" for c in expected_sha)
os.umask(0o077)
root = pathlib.Path("/opt/adobe-relay/releases/adobe-gpt-image-26-20261106")
root.mkdir(mode=0o700, parents=True, exist_ok=True)


def sql(query):
    return subprocess.run(
        [
            "docker",
            "exec",
            "-i",
            "adobe-relay-postgres-1",
            "psql",
            "-U",
            "sub2api",
            "-d",
            "sub2api",
            "-Atq",
            "-v",
            "ON_ERROR_STOP=1",
        ],
        input="BEGIN READ ONLY; SET LOCAL statement_timeout='15s'; " + query + "; COMMIT;",
        text=True,
        capture_output=True,
        check=True,
        timeout=25,
    ).stdout.strip()


container = json.loads(subprocess.check_output(["docker", "inspect", "adobe-relay-app-1"]))[0]
assert container["Config"]["Image"] == "adobe-relay:" + expected_sha
assert container["State"].get("Health", {}).get("Status") == "healthy"
needle = email.lower().replace("'", "''")
row = sql(
    "SELECT id || E'\\t' || platform || E'\\t' || type || E'\\t' || status "
    "FROM accounts WHERE deleted_at IS NULL AND LOWER(COALESCE(credentials->>'email', extra->>'adobeteam_external_email',''))='"
    + needle
    + "' ORDER BY id LIMIT 2"
)
rows = [line.split("\t") for line in row.splitlines() if line]
assert len(rows) == 1, "account email must resolve to exactly one active record"
account_id, platform, account_type, account_status = rows[0]
assert platform == "adobe" and account_type == "oauth"
admin_key = sql("SELECT value FROM settings WHERE key='admin_api_key'")
assert admin_key and len(admin_key) > 10

# One exclusive marker protects the account test from accidental replay.
marker = root / ("account-" + account_id + "-" + model + "-test-started.json")
started = datetime.datetime.now(datetime.timezone.utc).isoformat()
with os.fdopen(os.open(marker, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w") as handle:
    json.dump(
        {
            "at": started,
            "application_sha": expected_sha,
            "account_id": int(account_id),
            "account_email_sha256": hashlib.sha256(email.lower().encode()).hexdigest(),
            "model": model,
            "request_count": 1,
            "never_replay_if_interrupted": True,
        },
        handle,
    )

payload = json.dumps({"model_id": model, "prompt": "", "mode": "default"}).encode()
request = urllib.request.Request(
    "http://127.0.0.1:6666/api/v1/admin/accounts/" + account_id + "/test",
    data=payload,
    headers={
        "x-api-key": admin_key,
        "Content-Type": "application/json",
        "Accept": "text/event-stream",
    },
    method="POST",
)
out = {
    "at": started,
    "application_sha": expected_sha,
    "account_id": int(account_id),
    "account_email_sha256": hashlib.sha256(email.lower().encode()).hexdigest(),
    "account_status_before": account_status,
    "model": model,
    "outer_posts": 1,
    "success": False,
}
events = []
begin = time.monotonic()
try:
    with urllib.request.urlopen(request, timeout=240) as response:
        out["status"] = response.status
        for raw_line in response:
            line = raw_line.decode("utf-8", errors="replace").strip()
            if not line.startswith("data:"):
                continue
            try:
                event = json.loads(line[5:].strip())
            except json.JSONDecodeError:
                continue
            event_type = event.get("type")
            if event_type not in ("test_complete", "error"):
                continue
            safe = {"type": event_type}
            for field in ("success", "status_code", "error", "request_id"):
                if field in event and isinstance(event[field], (str, int, float, bool)):
                    safe[field] = event[field]
            events.append(safe)
    out["terminal_events"] = events[-3:]
    terminal = next((event for event in reversed(events) if event["type"] in ("test_complete", "error")), None)
    out["success"] = bool(terminal and terminal["type"] == "test_complete" and terminal.get("success") is True)
    if terminal and "status_code" in terminal:
        out["status_code"] = terminal["status_code"]
except urllib.error.HTTPError as error:
    out["status"] = error.code
    out["terminal_events"] = [{"type": "http_error", "status_code": error.code}]
except Exception as error:
    out["exception"] = type(error).__name__
finally:
    out["duration_seconds"] = round(time.monotonic() - begin, 3)
    (root / ("account-" + account_id + "-" + model + "-test.json")).write_text(
        json.dumps(out, ensure_ascii=False, indent=2)
    )
    print(json.dumps(out, ensure_ascii=False), flush=True)

assert out["success"], "Account model test did not pass; inspect the single recorded attempt before any retry"
