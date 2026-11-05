"""Run one real, non-replayed GPT Image 2.6 request against the primary site."""
import base64
import datetime
import hashlib
import json
import os
import pathlib
import struct
import subprocess
import sys
import time
import urllib.error
import urllib.request


MODEL_TO_FAMILY = {
    "gpt-image-2.6-flare": "firefly-gpt-image-2-6-flare",
    "gpt-image-2.6-sunburst": "firefly-gpt-image-2-6-sunburst",
}


model, expected_sha = sys.argv[1:]
assert model in MODEL_TO_FAMILY
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
key = sql("SELECT key FROM api_keys WHERE id=12 AND status='active' AND deleted_at IS NULL AND group_id=7")
assert key and len(key) > 10

# The marker is deliberately exclusive: an interrupted request must never be replayed.
marker = root / (model + "-live-test-started.json")
started = datetime.datetime.now(datetime.timezone.utc).isoformat()
with os.fdopen(os.open(marker, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w") as handle:
    json.dump(
        {
            "at": started,
            "application_sha": expected_sha,
            "model": model,
            "family": MODEL_TO_FAMILY[model],
            "api_key_id": 12,
            "request_count": 1,
            "never_replay_if_interrupted": True,
        },
        handle,
    )

payload = {
    "model": model,
    "prompt": "A single red ceramic coffee mug on a plain light gray studio background, no text.",
    "quality": "medium",
    "size": "1024x1024",
    "n": 1,
    "response_format": "b64_json",
}
request = urllib.request.Request(
    "https://creative.cheap/v1/images/generations",
    data=json.dumps(payload).encode(),
    headers={"Authorization": "Bearer " + key, "Content-Type": "application/json"},
    method="POST",
)
out = {
    "at": started,
    "application_sha": expected_sha,
    "model": model,
    "family": MODEL_TO_FAMILY[model],
    "requested_quality": "medium",
    "requested_size": "1024x1024",
    "api_key_id": 12,
    "outer_posts": 1,
    "success": False,
}
begin = time.monotonic()
try:
    with urllib.request.urlopen(request, timeout=240) as response:
        out["status"] = response.status
        body = json.load(response)
    out["response_quality"] = body.get("quality")
    items = body.get("data", [])
    assert len(items) == 1 and out["response_quality"] == "medium"
    data = base64.b64decode(items[0]["b64_json"], validate=True)
    assert data.startswith(b"\x89PNG\r\n\x1a\n")
    width, height = struct.unpack(">II", data[16:24])
    assert (width, height) == (1024, 1024) and len(data) > 1024
    (root / (model + "-live-test.png")).write_bytes(data)
    out.update(
        success=True,
        image={
            "bytes": len(data),
            "sha256": hashlib.sha256(data).hexdigest(),
            "width": width,
            "height": height,
        },
        request_id=body.get("request_id"),
    )
except urllib.error.HTTPError as error:
    out["status"] = error.code
    try:
        value = json.loads(error.read(32768)).get("error", {})
        out["error"] = {
            key: value[key]
            for key in ["type", "code", "param", "stage", "source", "upstream_status", "request_id"]
            if key in value
        }
    except Exception:
        out["error"] = {"type": "unparsed_http_error"}
except Exception as error:
    out["exception"] = type(error).__name__
finally:
    out["duration_seconds"] = round(time.monotonic() - begin, 3)
    (root / (model + "-live-test.json")).write_text(json.dumps(out, ensure_ascii=False, indent=2))
    print(json.dumps(out, ensure_ascii=False), flush=True)

assert out["success"], "Live generation did not pass; inspect the single recorded attempt before any retry"
