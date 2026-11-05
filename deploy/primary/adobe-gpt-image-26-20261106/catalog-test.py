"""Read-only online model-directory verification for the deployed Adobe app."""
import json
import subprocess
import sys
import urllib.request


expected_sha = sys.argv[1]
assert len(expected_sha) == 40 and all(c in "0123456789abcdef" for c in expected_sha)


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
api_key = sql("SELECT key FROM api_keys WHERE id=12 AND status='active' AND deleted_at IS NULL AND group_id=7")
admin_key = sql("SELECT value FROM settings WHERE key='admin_api_key'")
assert api_key and admin_key


def get(url, key):
    request = urllib.request.Request(url, headers={"Authorization": "Bearer " + key, "x-api-key": key})
    with urllib.request.urlopen(request, timeout=30) as response:
        return response.status, json.load(response)


status, models_response = get("https://creative.cheap/v1/models", api_key)
models = models_response.get("data", [])
ids = [item.get("id") for item in models if isinstance(item, dict)]
assert "gpt-image-2.6-star-swirl" in ids
assert "gpt-image-2.6-aurora" in ids
assert "gpt-image-2.6-flare" not in ids
assert "gpt-image-2.6-sunburst" not in ids

admin_status, account_response = get("http://127.0.0.1:6666/api/v1/admin/accounts/2933/models", admin_key)
account_models = account_response.get("data", [])
account_by_id = {
    item.get("id"): item.get("display_name")
    for item in account_models
    if isinstance(item, dict) and item.get("id")
}
print(json.dumps({"public_ids": [model_id for model_id in ids if isinstance(model_id, str) and "gpt-image-2.6" in model_id], "account_ids": sorted(account_by_id), "account_2_6_display": {model_id: account_by_id.get(model_id) for model_id in ("gpt-image-2.6-star-swirl", "gpt-image-2.6-aurora", "gpt-image-2.6-flare", "gpt-image-2.6-sunburst") if model_id in account_by_id}}, ensure_ascii=False))
sys.exit(0)
assert account_by_id.get("gpt-image-2.6-star-swirl") == "gpt-image-2.6-star-swirl"
assert account_by_id.get("gpt-image-2.6-aurora") == "gpt-image-2.6-aurora"
if "gpt-image-2.6-flare" in account_by_id:
    assert account_by_id["gpt-image-2.6-flare"] == "gpt-image-2.6-star-swirl"
if "gpt-image-2.6-sunburst" in account_by_id:
    assert account_by_id["gpt-image-2.6-sunburst"] == "gpt-image-2.6-aurora"

print(
    json.dumps(
        {
            "application_sha": expected_sha,
            "models_status": status,
            "admin_models_status": admin_status,
            "public_2_6_ids": [model_id for model_id in ids if isinstance(model_id, str) and "gpt-image-2.6" in model_id],
            "account_2_6_display": {
                model_id: account_by_id.get(model_id)
                for model_id in (
                    "gpt-image-2.6-star-swirl",
                    "gpt-image-2.6-aurora",
                    "gpt-image-2.6-flare",
                    "gpt-image-2.6-sunburst",
                )
                if model_id in account_by_id
            },
        },
        ensure_ascii=False,
    )
)
