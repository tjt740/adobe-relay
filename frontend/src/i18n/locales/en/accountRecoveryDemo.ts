export default {
  "title": "Account scheduling recovery scenarios",
  "previewNotice": "Proposal preview only. Controls affect this diagram alone. No live accounts are read; no checks, generations, sign-ins or scheduling changes are performed.",
  "scenarioLabel": "Scenario",
  "budgetLabel": "Temporary errors: maximum extra full checks per round",
  "retryCount": "{count} checks (example)",
  "seconds": "{count} seconds",
  "budgetHint": "Retry counts, delays and global concurrency are not finalized. The example waits of 60 / 120 / 240 seconds are not recovery deadlines. Choices are stored only in this browser and do not change scheduling parameters.",
  "currentTitle": "Current mechanism",
  "currentHint": "Cookie verification and existing guards",
  "proposedTitle": "Candidate proposal",
  "proposedHint": "Bounded extra checks for temporary failures that are safe to retry",
  "temporaryReason": "408 timeout or overload",
  "rateReason": "Temporary 429 rate limit",
  "gainLabel": "Benefit",
  "costLabel": "Cost",
  "limitLabel": "Remaining limitation",
  "retryGain": "After a temporary fault clears and a retry passes, scheduling may resume under the original recovery conditions.",
  "noRecoveryGain": "Failure reasons and retry progress become explicit instead of an indefinite “waiting for check” message.",
  "extraCost": "At most {count} additional full-check attempts per account per round. They may consume generation credits and concurrency and compete with real traffic for upstream capacity.",
  "retryLimit": "Recovery takes time. One successful check proves only that model at that moment; later requests may still receive 401, 408 or 429.",
  "exhaustedLimit": "The account remains unavailable when the budget is exhausted, awaiting valid credentials or review. Bounded retries cannot guarantee recovery.",
  "reviewTitle": "Limits and tradeoffs still to be decided",
  "boundariesLabel": "Preserved boundaries",
  "boundaries": {
    "manual": "Keep operator pauses",
    "login": "Keep the existing sign-in budget",
    "dedup": "Deduplicate the same Cookie and version",
    "concurrency": "Cap global check concurrency",
    "originalTask": "Never regenerate an accepted job"
  },
  "scenarios": {
    "temporary": {
      "label": "New Cookie check gets a temporary 408, then recovers",
      "detail": "Simulation: no generation was accepted; the second extra check is assumed to pass. This is not a prediction of how many live accounts will recover."
    },
    "rate": {
      "label": "New Cookie check gets a temporary 429 rate limit",
      "detail": "Simulation: ordinary rate limiting, with no accepted job; the second extra check is assumed to pass. Actual waiting must respect Retry-After."
    },
    "persistent": {
      "label": "408 persists and retries keep failing",
      "detail": "Simulation: no job was accepted by any attempt; all extra checks fail and this round ends at its limit."
    },
    "auth": {
      "label": "Confirmed Cookie failure / invalid_credentials",
      "detail": "Only confirmed credential failures belong here; an arbitrary HTTP 401 is not enough.",
      "gain": "Avoid repeatedly checking known-invalid credentials or resetting the sign-in budget.",
      "cost": "Temporary-error retries do not directly restore this class of account.",
      "limit": "A shared upstream fault misclassified as a credential failure could still quarantine accounts incorrectly; that needs separate investigation."
    },
    "quota": {
      "label": "429 actually means exhausted credits",
      "detail": "The same status can have different causes; exhausted credits and ordinary rate limiting need separate paths.",
      "gain": "Avoid extra generation checks when an account is known to have no credits.",
      "cost": "Classification must be more precise; ambiguous cases recover more conservatively.",
      "limit": "Retries cannot replenish credits or grant model entitlements."
    },
    "manual": {
      "label": "An operator-paused account passes a check",
      "detail": "Existing results may update; this proposal does not schedule extra automatic recovery probes for operator-paused accounts.",
      "gain": "Intentionally paused accounts stay out of the business pool.",
      "cost": "Successful checks can outnumber schedulable accounts; “passed but manually paused” must be explicit.",
      "limit": "An operator must still remove the pause through the existing control."
    },
    "later": {
      "label": "A new business 401 follows a successful check",
      "detail": "An observed sequence: success first, then a new business authentication failure.",
      "gain": "Old successful checks cannot mask newer authentication failures.",
      "cost": "Availability may still drop quickly. This proposal does not fix the reason a Token becomes invalid again.",
      "limit": "The cause and classification of the new 401 require a separate investigation."
    },
    "stale": {
      "label": "Cookie or proxy changes during a check",
      "detail": "Old-version success cannot release the current version; new-version checks still require existing verification and deduplication.",
      "gain": "Stale credentials or routes cannot authorize the current state.",
      "cost": "Frequent changes invalidate results, delay recovery and may add check attempts.",
      "limit": "Continuous changes can prevent current-version verification; retries must not bypass version guards."
    },
    "ownership": {
      "label": "A check passes without permission to resume",
      "detail": "An absent manual-pause marker does not authorize recovery; historical proxy, scheduling or other pauses may remain.",
      "gain": "Verification cannot accidentally enable accounts that should stay disabled.",
      "cost": "Some accounts that pass still will not rejoin the business pool automatically.",
      "limit": "Review historical pause ownership; resume only a pause explicitly owned by this verification."
    },
    "accepted": {
      "label": "Generation accepted, then polling or download gets 408",
      "detail": "Once a job identifier exists, retry only the original job or image URL; do not submit another generation.",
      "gain": "Prevent duplicate images and credit consumption after one failed check.",
      "cost": "The original job may still take time; same-job retries remain bounded.",
      "limit": "If the original job cannot continue, inspect it rather than creating a new job and calling that a recovery."
    },
    "unknown": {
      "label": "Connection loss or restart leaves acceptance unknown",
      "detail": "No successful receipt does not prove the upstream rejected the job; restarting must not replay generation unconditionally.",
      "gain": "Reduce duplicate generation and charges when acceptance is uncertain.",
      "cost": "Some accounts stay pending review and recover slower than an immediate regeneration would.",
      "limit": "Without a correlatable receipt, the original outcome cannot be established safely; do not replay automatically."
    }
  },
  "steps": {
    "cookie": {
      "title": "Receive new Cookie",
      "body": "Temporarily leave business scheduling"
    },
    "temporaryFailure": {
      "title": "First check fails",
      "body": "{reason}"
    },
    "noRetry": {
      "title": "Check ends",
      "body": "Failed checks are not automatically reclaimed"
    },
    "awaitingCheck": {
      "title": "Remain unavailable",
      "body": "Wait for new credentials or another check"
    },
    "classifyTemporary": {
      "title": "Classify temporary failure",
      "body": "{reason}; confirm no job was accepted"
    },
    "backoff": {
      "title": "Check again after backoff",
      "body": "Example waits: {delays}; at most {count} extra checks"
    },
    "recovered": {
      "title": "Second retry succeeds",
      "body": "Resume only if existing recovery conditions hold"
    },
    "budgetEnded": {
      "title": "Retry limit reached",
      "body": "End this round and retain pending verification"
    },
    "request": {
      "title": "Request or check",
      "body": "Use the current credentials and proxy"
    },
    "invalidCredentials": {
      "title": "Confirmed credential failure",
      "body": "Confirmed invalid_credentials"
    },
    "authRecovery": {
      "title": "Use existing auth recovery",
      "body": "Quarantine or recover only when its conditions hold"
    },
    "noAuthRetry": {
      "title": "No temporary-error retry",
      "body": "Do not increase or reset the sign-in budget"
    },
    "waitCredentials": {
      "title": "Wait for valid credentials",
      "body": "Retain existing verification requirements"
    },
    "quota": {
      "title": "429 means exhausted credits",
      "body": "Separate it from ordinary rate limiting"
    },
    "quotaCooldown": {
      "title": "Keep quota protection",
      "body": "Retain existing quota and cooldown conditions"
    },
    "quotaWait": {
      "title": "Wait for quota recovery",
      "body": "Do not bypass credits or model entitlements"
    },
    "noQuotaRetry": {
      "title": "No extra generation check",
      "body": "Avoid spending checks on exhausted accounts"
    },
    "manualPause": {
      "title": "Operator pause",
      "body": "Scheduling is disabled"
    },
    "success": {
      "title": "Check succeeds",
      "body": "Evidence for this check and its model only"
    },
    "keepManualPause": {
      "title": "Preserve operator pause",
      "body": "Do not override the operator’s intent"
    },
    "awaitOperator": {
      "title": "Remain paused",
      "body": "Wait for the operator to enable scheduling"
    },
    "new401": {
      "title": "Later business request gets 401",
      "body": "A new failure after the successful check"
    },
    "oldProof": {
      "title": "Old success cannot clear new failure",
      "body": "Retain time and model checks"
    },
    "versionA": {
      "title": "Start check",
      "body": "Cookie or proxy version A"
    },
    "versionB": {
      "title": "Version changes during check",
      "body": "Cookie or proxy is now version B"
    },
    "oldSuccess": {
      "title": "Version A check succeeds",
      "body": "Its state snapshot is now stale"
    },
    "rejectOldProof": {
      "title": "Reject stale recovery evidence",
      "body": "The current version still needs verification"
    },
    "verifyVersionB": {
      "title": "Verify the current version",
      "body": "Deduplicate it and preserve budget limits"
    },
    "historicalPause": {
      "title": "Previously paused",
      "body": "No permission to resume automatically"
    },
    "noResumePermission": {
      "title": "Success does not automatically enable",
      "body": "This verification does not own the pause"
    },
    "inspectPauseOwner": {
      "title": "Check who owns the pause",
      "body": "Resume only a pause owned by verification"
    },
    "leavePaused": {
      "title": "Review historical pause",
      "body": "An absent manual marker does not authorize enabling"
    },
    "acceptedJob": {
      "title": "Generation accepted upstream",
      "body": "A job identifier already exists"
    },
    "pollTimeout": {
      "title": "Polling or download fails",
      "body": "For example, 408 or a dropped connection"
    },
    "sameJob": {
      "title": "Retry within the original job",
      "body": "Keep the same job or image URL"
    },
    "sameJobResult": {
      "title": "Finish or inspect the same job",
      "body": "Do not submit a new generation for this failure"
    },
    "unknownResult": {
      "title": "Submission outcome unknown",
      "body": "Connection lost before acceptance could be confirmed"
    },
    "interrupted": {
      "title": "Stop automatic replay",
      "body": "Avoid duplicate generation or charges"
    },
    "verifyReceipt": {
      "title": "Check the original receipt",
      "body": "Continue only if the original job can be identified"
    },
    "holdForEvidence": {
      "title": "Hold for review if uncertain",
      "body": "Do not treat an unknown result as unsubmitted"
    }
  },
  "review": {
    "classification": {
      "title": "Status codes alone are insufficient",
      "body": "Separate overload, rate limits, exhausted credits, authentication, model entitlement and unknown outcomes. Only temporary failures confirmed safe to retry enter this path."
    },
    "acceptedTask": {
      "title": "Acceptance is the first boundary",
      "body": "Continue the same accepted job. Investigate uncertain acceptance after connection loss or restart instead of generating again and potentially spending credits twice."
    },
    "budget": {
      "title": "Per-account limits do not control total load",
      "body": "Decide global, group and upstream concurrency and credit caps, reserve business capacity, use jittered backoff and Retry-After, and stop amplifying persistent upstream failures. Restarts and concurrent checks must not reset budgets. Repeated delivery of the same Cookie must not repeatedly remove scheduling or reset budgets merely because its synchronization version changed."
    },
    "eligibility": {
      "title": "Success does not authorize unconditional scheduling",
      "body": "Retain current Cookie, proxy, failure ordering, cooldown, quota, expiry and operator checks. Deferred probes must not occupy a business-request queue or increase the sign-in budget."
    },
    "ownership": {
      "title": "A pause needs an explicit recovery owner",
      "body": "Resume only pauses owned by this verification. Review missing historical recovery permissions; an absent manual-pause marker is not an enable instruction."
    },
    "modelScope": {
      "title": "One model’s success is limited evidence",
      "body": "Preserve the exact model and time. Old success cannot clear a new 401, and retries do not replace investigating rapid Token invalidation."
    },
    "display": {
      "title": "Display the actual lifecycle",
      "body": "Distinguish queued, running, waiting for retry, exhausted and needs-review states; keep failure history. Demo controls do not change production settings or describe already-configured retry values."
    },
    "exhaustion": {
      "title": "Decide what follows an exhausted retry budget",
      "body": "A short retry window cannot cover a prolonged upstream outage. Decide between low-frequency review with a daily credit cap and operator review. Do not loop indefinitely or silently leave the account stuck forever."
    }
  }
}
