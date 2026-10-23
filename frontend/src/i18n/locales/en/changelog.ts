import recoveryDemo from './accountRecoveryDemo'

export default {
  daily: {
    recoveryDemo,
    previewTitle: 'Proposal previews',
    previewBadge: 'Scheduling proposal · Rules not implemented',
    "title": "Changelog",
    "timezone": "Daily summaries · Beijing time (UTC+8)",
    "intro": "Updates and fixes grouped by day, with changes from the same date together and the newest day first.",
    "latest": "Latest",
    "count": "{count} updates",
    "archiveTitle": "Oct 17 detailed archive · Resources, proxies and load replays",
    "types": {
      "added": "Added",
      "improved": "Improved",
      "fixed": "Fixed"
    },
    measurements: "Release preparation measurements and sample scope",
    overviews: {
      proxyVerificationPriority: { title: 'Prioritize business capacity and lend proxies for 408 checks', body: 'Today fixes verification accounts waiting despite reclaimable capacity. Business demand comes first, followed by eligible pending verification. Unresolved 408 accounts release regular proxies and borrow one only for checks, keeping a binding after successful recovery. Today’s v0.2.17 upgrade and independent subscription management remain, with existing configuration and manual-pause rules preserved.' },
      clashSubscriptions: {"title": "v0.2.17 features and independent subscription updates", "body": "Today: integrate upstream v0.2.17 model, protocol and payment fixes while retaining Adobe, Okad and custom scheduling protections; remove the eight-URL cap, add row editing and checks alongside bulk input, and use bounded concurrency with independent source outcomes. Successful sources can update while failed sources retain saved nodes and bindings. Status icons, colored counts and scrolling improve; existing node budgets, manual account state and configuration remain protected."},
      "releaseSpeed": {
        "title": "Shorter release waits with rollout guards retained",
        "body": "Today: parallel preparation, reusable caches and compressed transfers. Cached preparation measured about 68% shorter. Full rollout still includes backups, draining and verification. The figures below describe specific samples, not a fixed service commitment."
      }
    },
    "updates": {
      manualAccountIP: {
        "title": "Manual IP settings per account",
        "body": "Pin a selected proxy per account, including accounts with 408 failures. Automatic allocation and IP switching skip manual accounts.",
        "sections": {
          "reason": {
            "title": "Keep operator-selected routes",
            "body": "Useful for network diagnosis, fixed egress and manual routing of 408 accounts. Existing allocation and 408 release rules could overwrite a chosen binding, so each account gains an explicit routing-mode switch."
          },
          "control": {
            "title": "Save with the account edit",
            "body": "Enable Manual IP settings beside the proxy selector, choose a proxy and click Update. Cancel discards the edit; No proxy explicitly selects direct routing. The switch defaults off and existing accounts keep their mode."
          },
          "routing": {
            "title": "Skip automatic routing changes",
            "body": "Manual accounts skip ordinary allocation, latency optimization, failover, expiry fallback and 408 proxy release. Checks, quota queries and renewal use the selected route without borrowing or replacing it. Manual bindings still consume node capacity and cannot be reclaimed. Turning the switch off restores normal management."
          },
          "protection": {
            "title": "Routing ownership preserves account protection",
            "body": "Keep model failure evidence, credentials, operator pauses and verification and scheduling requirements. Enabling the mode does not resume an account or clear errors. Finishing an older check or saving an older background snapshot cannot take over the manual route or mode."
          },
          "validation": {
            "title": "Validation scope",
            "body": "Local regressions cover save and reload, manual 408 bindings, maintenance, shared capacity, in-flight check completion and stale snapshots. Isolated tests do not prove real Adobe generation or peak throughput; production status belongs in the actual deployment record."
          },
          "boundary": {
            "title": "Operating boundary",
            "body": "Administrators select and maintain manual routes; invalid nodes are not repaired automatically. Explicit deletion of a proxy or subscription retains existing cleanup protections. Models, quota, concurrency, alerts and Cookie recovery rules are unchanged."
          }
        }
      },
      proxyVerificationPriority: {
        "title": "Prioritize pending verification and lend proxies to 408 recovery checks",
        "body": "Fix verification waiting despite reclaimable capacity. With automatic allocation enabled, unresolved 408 accounts leave the regular allocation queue and borrow a proxy only for checks. Real generation on the affected model must succeed and all remaining 408 failures must recover before the binding becomes permanent. Failure or cancellation releases it. The panel and account details explain capacity and recovery conditions.",
        "sections": {
          "reason": {
            "title": "Why verification could remain stuck",
            "body": "An account with a saved new cookie may still need a proxy for verification. Treating it as an ordinary error account prevented it from reclaiming capacity occupied by inactive accounts."
          },
          "priority": {
            "title": "Allocation order",
            "body": "Retain and serve enabled business accounts first, assign healthy proxies to eligible pending or retrying verification next, then handle other stopped, expired or error accounts. Pending verification can reclaim those inactive slots. The limit remains three accounts per node."
          },
          "quarantine": {
            "title": "Release capacity held by unresolved 408 accounts",
            "body": "408 failures persist per model; later failures such as 500 cannot overwrite them. Unresolved accounts leave the regular proxy queue and release existing bindings in background maintenance. Cookie updates, credit checks and success on another model cannot clear the original failure. Historical aliases use the existing model catalog; Flare and Sunburst remain separate models."
          },
          "leases": {
            "title": "Borrow for checks, keep only after successful recovery",
            "body": "Manual checks, existing schedules and new-cookie checks share temporary routing, protect business demand and retain the three-account limit. Every affected model must recover. Failure and cancellation release the slot; abandoned leases expire after five minutes and are reclaimed by maintenance. This feature adds no recurring image generation and does not replay a possibly submitted check. Credit queries and background token refresh also borrow a proxy and always release it; neither clears model 408 failures nor restores a permanent binding."
          },
          "protection": {
            "title": "Assignment still requires verification",
            "body": "Priority changes proxy bindings only; it never clears errors or enables an account. Manual pauses, expired accounts, missing cookies and definitive authentication, identity, quota or entitlement failures do not qualify. Existing verification, recovery and model checks remain in control."
          },
          "wording": {
            "title": "Understand capacity and waiting counts",
            "body": "Free slots are immediately assignable. Slots that can be freed are occupied by other accounts until priority accounts need them. Regular waiting, unresolved 408 accounts and temporary checks are shown separately. Account details list remaining models and explain which model to test. Connection and switching rules are expandable."
          },
          "validation": {
            "title": "Validation scope",
            "body": "Disposable database checks cover reclamation, concurrent leases, failure and cancellation, expiry, stale credentials/results, multiple models and historical aliases. Manual pauses, verification gates and scheduled recovery policy are checked. Frontend coverage includes counts, account hints, older API compatibility and bilingual layouts. Isolated data and simulated generation do not establish real account recovery."
          },
          "boundary": {
            "title": "Existing configuration and behavior",
            "body": "No configuration keys are added. New state tables track unresolved 408 failures, allocator pause intent and temporary test leases. Existing credentials, operator state, groups, subscriptions, SMTP and alert settings remain. Check intervals, failover and latency-switch cooldown are preserved. Releasing bindings does not guarantee distinct egress IPs; proxy connectivity is not proof of successful generation."
          }
        }
      },
      upstreamV0217: {
        title: 'Upstream v0.2.17 feature upgrade',
        body: 'Integrates official v0.2.17 features and fixes with adaptations for existing Adobe, Okad, proxy and scheduling behavior. Existing configuration and business data are retained; new capabilities depend on accounts, endpoints and explicit settings.',
        sections: {
          reason: { title: 'Why upgrade', body: 'Synchronizes upstream model support, protocol compatibility and payment fixes while retaining the custom business capabilities built on the official project. The application version label is v0.2.17.' },
          models: { title: 'Models and platforms', body: 'Adds Gemini 4 Argon, Claude Haiku 5.5, a dynamic platform catalog and multi-protocol providers, alongside group translations and explicit Gemini context caching. Availability still depends on accounts, mappings, upstream endpoints and existing configuration. No accounts or paid capabilities are enabled automatically.' },
          gateway: { title: 'Gateway and client compatibility', body: 'Integrates Responses timestamps, Gemini thinking levels and streamed summaries, tool-result images, protocol endpoint selection and workspace-aware imports. OpenAI API-key Messages requests can use configured web-search emulation, and Gemini shared-capacity shortages use same-account backoff.' },
          payments: { title: 'Payments, refunds and quotas', body: 'Integrates EasyPay order confirmation, deduction protection during pending refunds, Alipay refund queries, order-type validation and Stripe payment credential retrieval, together with usage-display and billing fixes. Existing merchant and payment settings are retained.' },
          compatibility: { title: 'Custom business adaptations', body: 'Registers Adobe and Kiro in the new catalog while retaining model listings, manual proxy selection, automatic failover, Okad test and manual-pause protections, default pagination and shared controls. New in-flight balance reservations and API-key creation limits require explicit configuration, preserving existing admission and concurrency behavior. Adobe image endpoints, 451 responses, usage, quality labels and memory protection retain the existing custom implementation.' },
          validation: { title: 'Validation scope', body: 'Development checks cover frontend and backend code, platform and gateway compatibility, existing business regressions, rollout guards and an isolated PostgreSQL upgrade. Fixtures are synthetic, and inherited baseline failures are recorded separately. Production cutover separately requires backups, draining, runtime checks and data verification. These checks do not establish peak throughput or upstream account recovery.' },
          boundary: { title: 'Upgrade limits and rollout', body: 'Upstream removes gpt-5.5 scheduling and listings for ChatGPT OAuth / SetupToken accounts; API-key accounts are outside that retirement rule. Adobe GPT Image models are outside its scope. Deployment still requires reviewing current mappings, running versions and configuration, followed by backup and controlled draining. Old databases and manual account states must not be restored or overwritten.' },
        },
      },
      clashSubscriptions: {
        "title": "Individual subscription management, independent fetches and scrolling fixes",
        "body": "Removes the eight-URL limit and adds a list editor alongside bulk input and individual feed checks. Foreground fetches and background refresh share a concurrency limit, report failures per source and retain existing nodes and bindings for failed sources. Status icons, colored node counts and page scrolling are improved. Existing node, proxy-binding and configuration protections remain in effect.",
        "sections": {
          "reason": {
            "title": "Why this changed",
            "body": "The fixed eight-URL cap rejected larger subscription lists. Serial fetching also caused one failed feed to abort the entire preview or import. The new flow reports each source separately to reduce interference between unrelated feeds."
          },
          "fetching": {
            "title": "Independent fetching and fair refresh",
            "body": "Foreground and background work share at most four network fetches. Each feed has eight seconds after receiving a slot; previews have a 35-second overall budget. Unfinished sources are identified separately, and background work prioritizes the oldest observations. Fetches run outside SQL transactions and completed observations are saved. Existing exhaustion, expiry and failure backoff rules remain."
          },
          "editing": {
            "title": "List, bulk input and individual checks",
            "body": "Add, edit or remove URLs in the list and check each feed download and parse result. Bulk mode keeps one URL per line and preserves edits when switching modes. Duplicate URLs are merged and visibility controls remain. Individual checks do not save configuration, probe node connectivity or contact Adobe."
          },
          "status": {
            "title": "Status icons and colored node counts",
            "body": "Expand, collapse, manage and refresh controls include icons. Reachable counts use green, failed counts red and unconfirmed counts amber, with a separate total and accessible text. Reachability still does not prove account authorization, credits or generation availability."
          },
          "scrolling": {
            "title": "Long lists and window scrolling",
            "body": "The IP management page uses natural document scrolling so expanded subscription health does not squeeze out the table or pagination. The subscription dialog uses one vertical body scroll area with a fixed header and actions. Wide node tables retain horizontal scrolling."
          },
          "validation": {
            "title": "Development validation and scope",
            "body": "Coverage includes more than eight feeds, shared concurrency, independent failures, duplicates, cancellation, reordered URLs, preserved proxy bindings, all-failure no-op behavior, list/bulk switching and individual checks. Validation uses synthetic feeds, a disposable database and local pages; it does not establish a production rollout, real proxy throughput or upstream recovery."
          },
          "boundary": {
            "title": "Preserved behavior and limits",
            "body": "The 8 MiB per-feed, 512 combined-node, request text size, URL safety, revision conflict and runtime rollback protections remain. Failed or unfinished sources keep their saved nodes. Explicitly deleting a URL or deselecting nodes from a successful feed follows the existing removal flow. All-failed imports do not save. Resource limits, account state, scheduling, SMTP and alert configuration are outside this change."
          }
        }
      },
      releaseSpeed: {
        "title": "Faster release preparation and compressed transfers",
        "body": "Run backend checks, frontend checks and image builds in parallel, reuse dependency and compilation caches, and keep image transfers compressed. Formal artifacts are published only after all checks pass. Backups, draining accepted work, cutover and rollback guards remain.",
        "metrics": {
          "baseline": {
            "label": "Before · one formal build",
            "value": "17 min 29 sec",
            "note": "A historical complete preparation run, including checks, image build and artifact publication. Server rollout is excluded."
          },
          "cold": {
            "label": "First new-cache validation",
            "value": "11 min 34 sec",
            "note": "The first parallel validation started with an empty new Go cache. All checks passed; production deployment was skipped."
          },
          "warm": {
            "label": "Cached validation · a later commit",
            "value": "5 min 31 sec",
            "note": "Complete preparation and the artifact gate on another commit, about 68% shorter than the historical sample. The backend check step fell from 10 min 45 sec to 1 min 29 sec."
          },
          "transfer": {
            "label": "Image transfer size · historical sample",
            "value": "About 148 → 48 MB",
            "note": "About 68% fewer transfer bytes with compression, using decimal MB. This is a size comparison, not a measured production transfer duration."
          }
        },
        "sections": {
          "reason": {
            "title": "Why this changed",
            "body": "Backend checks, frontend checks and image builds previously waited for one another, so their times added up. Existing Go caches were not fully reused across later commits, leaving repeated compilation work. The image archive was already compressed but was decompressed locally before transfer, increasing network bytes."
          },
          "parallel": {
            "title": "Parallel preparation with a final gate",
            "body": "Three independent jobs now prepare concurrently. A failed check prevents publication of the formal release-images artifact and blocks deployment. Build-only validation has a separate queue and can cancel obsolete runs on the same branch. Production releases remain serialized, and later commits do not automatically cancel a rollout in progress."
          },
          "cache": {
            "title": "Reuse dependencies and compilation; execute the checks",
            "body": "Go caches are saved and restored by environment, Go version, dependency files and commit, with compatible fallback keys. Frontend jobs reuse the pnpm dependency cache; images have a dedicated build cache. Each run clears cached Go test results before executing the original test commands. Savings come from less repeated preparation and compilation."
          },
          "transfer": {
            "title": "Compressed transfer and failure handling",
            "body": "The gzip archive stays compressed over SSH and is decompressed and loaded on the primary server. Existing SHA-256 and host identity checks remain. A remote decompression or image-load failure fails the operation instead of reporting success. Loading an image alone does not switch the running application."
          },
          "scenarios": {
            "title": "When it helps and what it costs",
            "body": "Routine code changes, frontend or changelog releases, and build-only validation can all spend less time waiting. Stable dependencies and toolchains favor cache reuse. Initial runs, expired caches and major dependency changes benefit less. Parallel jobs require more concurrent CI capacity, and queueing or network conditions affect timing. Production resource allocations remain unchanged."
          },
          "validation": {
            "title": "Measured results and verification scope",
            "body": "All 44 local release-script tests passed, including 6 covering compressed transfers and error propagation. Workflow syntax passed validation, and the original test commands were compared and retained. Two complete cloud checks and builds succeeded; the second reused caches on a different commit and passed the formal artifact gate. Both were build-only validations with production deployment skipped. Their timings do not measure a live cutover."
          },
          "boundary": {
            "title": "Limits and preserved production behavior",
            "body": "5 min 31 sec is one cached preparation measurement, not a fixed duration for every release or a complete rollout. Actual deployment still requires backups, natural completion of accepted work, rolling cutover and verification; busy periods may take longer to drain. This change covers preparation and changelog presentation. Existing configuration, data, scheduling, operator pauses, SMTP, alerts, resource allocations and released features remain. It makes no claim of faster generation, higher business throughput or account recovery."
          }
        }
      },
      "adobeManualRevalidation": {"title": "Independent manual revalidation", "body": "Forced manual revalidation no longer inherits an in-flight background query's result or narrower renewal policy. An older background failure cannot replace newer validation evidence. Equivalent checks still share work. Operator pauses, login attempt limits and image validation requirements remain."},
      "adobeQuotaDiagnostic": {"title": "Quota revalidation for quarantined Adobe accounts", "body": "Manual quota revalidation can renew a quarantined Adobe session. A successful balance check retains error and scheduling state; image recovery still requires its existing checks. Operator pauses and disabled or expired account limits remain."},
      "adobeSessionRenewal": {"title": "Adobe session renewal", "body": "Retain renewed Adobe sessions after token refresh and avoid appending stale cached login cookies. Quota revalidation and a definitively rejected image submission may renew and retry once; Recovery never regenerates accepted or unknown work. Late responses cannot overwrite newer sessions, and operator pauses and proxy-change guards remain. Real authentication, quota and rate limits still follow existing handling."},
      "recoveryDemoPublished": { "title": "Complete scheduling scenarios in the changelog", "body": "Explore 11 scenarios, 1–3 example extra checks, current and proposed flows, benefits, costs and limits, with bilingual text and mobile layouts. Controls affect only this browser demo. Actual scheduling, check retries and account settings are unchanged." },
      "accountRecoveryDemo": { "title": "Interactive account recovery scenarios", "body": "Compare temporary failures, authentication, quota, operator pauses and historical resume permissions. Adjust example retry counts and inspect benefits, costs and limits. Eleven scenarios cover accepted jobs and unknown submission outcomes as well. This is a proposal for review; actual scheduling rules are unchanged." },
      "accountCheckState": { "title": "Align account checks and recovery", "body": "Account lists show the latest completed check or request with its model, without contradictory historical error badges. Bulk and individual full image checks share guarded recovery while preserving manual pauses and credential or proxy changes." },
      "okadGroups": {
        "title": "Okad custom groups and filters across both apps",
        "body": "Create and choose groups when importing external accounts, or regroup existing accounts in bulk. Combine groups with company, Vietnam and non-company types. Sub2 filters, exports and bulk actions use the same group selection."
      },
      "subscriptionUI": {
        "title": "Subscription input and status cards",
        "body": "A six-line, resizable subscription input and green, amber or red status watermarks make links and availability easier to read. Node counts and specific failure reasons remain visible."
      },
      "dailyHistory": {
        "title": "Daily release summaries",
        "body": "Updates and fixes are grouped by release date in Beijing time, newest first. Releases on the same day share one section; the latest day is expanded, with earlier entries and detailed archives retained."
      },
      "adobeModels": {
        "title": "Correct Adobe model lists",
        "body": "An Adobe group without schedulable accounts still lists Adobe image models, respecting mappings and allowlists. A listed model does not guarantee that generation is currently available."
      },
      "qualityLabel": {
        "title": "Preserve the xhigh quality label",
        "body": "GPT Image 2.5 responses preserve xhigh when requested; max stays max. Both retain the same upstream highest-detail setting, with other model limits unchanged."
      },
      "proxyRouting": {
        "title": "Proxy health and account fallback",
        "body": "Identify exhausted, expired or failing subscriptions, switch away from persistently slow exits and prioritize enabled accounts. An initial empty pool gets a bounded same-group recheck, preserving manual pauses and retry limits."
      },
      "capacity": {
        "title": "Higher request capacity",
        "body": "Sub2 uses a 54 GiB container limit, 42 GiB Go target, 37.8 GiB image budget and parallelism 16, with higher ingress limits. Successful isolated replays are not a guarantee of upstream throughput."
      },
      "groupIDs": {
        "title": "Group IDs visible by default",
        "body": "Group IDs appear beside names. Existing browsers receive the new default on their first updated load, preserving other column preferences and later manual changes."
      },
      "imageAdmission": {
        "title": "Reduce premature image rejections",
        "body": "Staged reservations and disk buffering reduce capacity rejections when memory remains available. OOM protection, the absence of a new capacity queue and same-job download retries are preserved."
      },
      "imageSafety": {
        "title": "Consistent image content refusals",
        "body": "Explicit prompt or reference-image content refusals use 451. If streaming has already started, the error object carries the status code so clients can distinguish content refusal from parameter errors."
      },
      "imageDelivery": {
        "title": "Image delivery and download retries",
        "body": "Failed downloads continue the original job and image URL, retrying or changing exits under existing rules. Valid real usage is retained; random simulated values used when usage is missing do not represent actual token consumption."
      },
      "imageMemory": {
        "title": "Lower memory pressure during image bursts",
        "body": "Uploads reserve memory incrementally and release temporary allowances, while outputs use bounded disk buffering and reusable heap. Capacity guards remain; passing tests does not imply zero rejections for every input."
      },
      "quietAlerts": {
        "title": "Complete reports and quieter alerts",
        "body": "Reports show full details with red 401 and amber 408 entries and improved mobile layouts. Notifications require sustained confirmation, send once per event, and wait at least 60 minutes between new notifications or failed-send retries."
      },
      "imageURL": {
        "title": "Image URL delivery",
        "body": "Explicit URL requests prefer object storage, then a validated HTTPS image URL whose downloaded bytes were unchanged. When these conditions are not met, delivery falls back to Base64."
      },
      "accountDetails": {
        "title": "Clearer account and integration errors",
        "body": "Account details show Cookie verification and model errors, with company filters. Linked external accounts support error-state viewing and filtering; unknown states are not presented as healthy."
      },
      "accountEvidence": {
        "title": "Live account state and success evidence",
        "body": "Account pages reflect recorded changes and distinguish failures, pending verification and actual success evidence. Reports use existing records without generating images or signing in; quiet windows do not clear old failures."
      },
      "reliableLink": {
        "title": "Reliable primary-site integration",
        "body": "Okad and Sub2 use two-way authentication and reliable synchronization. Account lists hide integration details by default with an option to reveal them, preserving existing settings and manual scheduling."
      },
      "recoveryProtection": {
        "title": "Safer automatic account recovery",
        "body": "Authentication, quota and entitlement errors are distinguished, with existing Cookies reused first. Timeouts do not trigger sign-in; automatic sign-ins use persistent account cooldowns and preserve manual pauses."
      },
      "adobeAlerts": {
        "title": "Dedicated Adobe error alerts",
        "body": "Business and account-check failures are aggregated, deduplicated by account and evaluated by error ratio. Later dated entries describe sustained confirmation, success evidence and notification frequency improvements."
      }
    }
  },
  description: 'Changes, practical scenarios and validation evidence',
  edition: 'Night of Oct 16 – early Oct 17, 2026 · Beijing time',
  headline: 'More capacity, more reliable routing',
  intro: 'This edition covers resource capacity, proxy health and account selection. Start with delivery status, then explore the scenarios and evidence.',
  archive: 'Archived edition · Production observations through Oct 17, 03:04 Beijing time; this is not live monitoring.',
  overview: 'Overview',
  roadmap: 'Scenarios & flow',
  stress: 'Load tests',
  documents: 'Markdown documents',
  live: 'In effect',
  pending: 'Included in this release',
  verified: 'Isolated validation',
  resourceTitle: 'Resource and ingress capacity',
  resourceSummary: 'Sub2 54 GiB / Go 42 GiB, parallelism 16; higher Nginx connection and file descriptor limits.',
  resourceTime: 'Canonical instance switched at 02:52 on Oct 17',
  proxyTitle: 'Proxy selection and account fallback',
  proxySummary: 'Subscription diagnostics, sustained latency switching, business account priority and one bounded same-group recheck.',
  proxyTime: 'Based on f3a465685 · Included in this release',
  testTitle: '13 pressure replay cases',
  testSummary: 'Compare resource profiles using the same size samples; check admission, output handling and temporary file cleanup.',
  testTime: 'Synthetic input and output · No Adobe calls',
  resourceHeading: 'How resources are allocated',
  resourceHint: 'Final configuration in this record. The 54 / 42 / 37.8 GiB limits are nested, not additive.',
  container: 'Sub2 container limit',
  go: 'Go runtime target',
  image: 'Logical image budget',
  parallel: 'Go parallelism',
  memoryDiagram: 'The container includes the Go target, which includes the logical image budget',
  resourceFootnote: 'Used on demand, not preallocated. About 12 GiB remains for non-Go overhead and 3.29 GiB for the host. Okad stays at 3 GiB; PostgreSQL / Redis containers at 1 GiB / 256 MiB. Database internal settings and connection pools are unchanged.',
  ingress: 'Ingress connection limits',
  ingressHint: 'Nginx connections per worker: 768 → 8192; file descriptors: 1024 → 65536. Connections include client and upstream sockets, not completed image concurrency.',
  routingHeading: 'From subscription health to request fallback',
  routingHint: 'Proxy and account routing behavior in this release. Manual pauses, customer groups and in-flight tasks are preserved.',
  step1Title: 'Read subscriptions',
  step1Body: 'Record quota, expiry and fetch errors; label unknown quota.',
  step2Title: 'Filter exits',
  step2Body: 'Exclude confirmed exhausted or expired sources from new bindings; check connectivity.',
  step3Title: 'Select steadily',
  step3Body: 'Compare smoothed latency, success and occupancy; require a meaningful improvement.',
  step4Title: 'Prioritize business',
  step4Body: 'Serve enabled accounts first; reclaim inactive binding slots when needed.',
  step5Title: 'Bounded fallback',
  step5Body: 'On an initial empty pool before any upstream attempt, repair and recheck once in the same group.',
  fallbackTitle: 'Still unavailable after the recheck',
  busy: 'Concurrency slots full',
  busyBody: 'Return 429 with a suggested 2-second retry, preserving concurrency limits.',
  unavailable: 'No eligible account',
  unavailableBody: 'Return 503 with a reason and Retry-After. Background recovery continues; paused accounts stay paused.',
  fallbackBound: 'Repair and recheck take at most 1.2 seconds combined. No cross-group fallback, new waiting queue, request-time login or duplicate generation.',
  scenarios: 'Practical scenarios',
  before: 'Previous problem',
  after: 'New behavior',
  scene1Title: 'Subscription quota exhausted or expired',
  scene1Before: 'Imported nodes could remain allocation candidates until a connection failed.',
  scene1After: 'Confirmed evidence excludes the source from new bindings and identifies it in IP management. Later maintenance moves accounts to eligible exits. Unknown quota is never presented as positive.',
  scene2Title: 'Subscription URL cannot be fetched',
  scene2Before: 'A fetch failure was difficult to distinguish from failed nodes.',
  scene2After: 'Show the subscription index, redacted URL and error category. Imported nodes with working connections remain usable, avoiding mass unbinding after one fetch error.',
  scene3Title: 'An IP stays slow',
  scene3Before: 'A reachable but slow node could keep delaying subsequent requests.',
  scene3After: 'Require 3 successful checks over 2 seconds, an alternative at least 30% and 500 ms faster, and a 5-minute account cooldown. Keep a working slow node if no better candidate exists.',
  scene4Title: 'Paused accounts occupy healthy node slots',
  scene4Before: 'Business accounts could wait while inactive accounts occupied bindings.',
  scene4After: 'Allocate to business accounts first and reclaim inactive bindings if necessary. Keep 3 accounts per node, credentials and manual scheduling state.',
  scene5Title: 'Cache is empty but an account has recovered in the database',
  scene5Before: 'Stale snapshots or repairable bindings could immediately cause a 503.',
  scene5After: 'Perform one bounded repair and same-group database recheck on the initial empty selection. Preserve model, quota, concurrency and exclusion predicates.',
  scene6Title: 'One account fails or a generation job was already submitted',
  scene6Before: 'An inappropriate retry could expand attempts or duplicate generation.',
  scene6After: 'Keep the original same-group switch budget. Once a job ID exists, continue that job with the existing download retries and exit fallback.',
  scene7Title: 'Every account is actually unavailable',
  scene7Before: 'A plain 503 did not explain whether to wait or intervene.',
  scene7After: 'Return no_available_accounts, a reason and a suggested retry delay; keep IP management warnings. Retry timing is advice, not a recovery promise.',
  limitsTitle: 'Costs and limits',
  limitsBody: 'Subscription metadata may be missing and background checks have delay. IP changes can affect sessions, so stable bindings and cooldowns remain. Connectivity is not proof of account entitlement or successful generation. Monitoring does not add or remove imported nodes.',
  stressHeading: 'What tonight’s load tests actually cover',
  stressHint: 'Oct 17, 2026 · Isolated replay built from the running application source, not real Adobe throughput.',
  groups: 'Replay cases',
  oom: 'OOM / OOM kills',
  leftovers: 'Temporary files left',
  comparison: 'Comparable 1600-request large-output bursts',
  comparisonHint: '16-core quota · JSON · one synthetic 16 MiB image per request · mixed memory paths · reduced disk reservation',
  completed: 'Admitted and completed',
  rejected: 'Admission rejections',
  selected: 'Selected',
  previous: 'Previous',
  candidate: 'Comparison candidate',
  eventMax: 'Hard-limit events',
  selectedReason: 'Go 42 and 44 both completed 1600 requests. Go 42 had fewer hard-limit events and more non-Go headroom, so 42 was chosen. Parallelism also changed between the 52 and 54 GiB profiles; the difference cannot be attributed entirely to memory.',
  boundary: 'Extreme cases reached the container limit. memory.events.max is neither failed requests nor GC cycles. The 1600-request cases used reduced disk reservation; the production 160 MiB physical reservation was validated only at 256 requests.',
  extraChecks: 'Additional checks on the selected profile',
  mixedCheck: 'multipart · synthetic 8 MiB output · reduced reservation',
  fullCheck: 'Production 160 MiB physical reservation per image · 8 MiB output',
  method: 'Samples and methodology',
  methodBody: '340 real normalized body sizes, repeated into 680 samples: P50 about 1.03 MiB, P95 19.08 MiB, max 21.73 MiB. Only sizes were used, with synthetic input/output and an assumed 40% original-memory path. No Adobe, production account or business database access. These are not HTTP wire sizes, and bursts are not sustained production peaks.',
  allResults: 'Expand all 13 results',
  case: 'Scenario / output',
  config: 'Container / Go / parallelism',
  cores: 'Core quota',
  elapsed: 'Seconds per run',
  jsonCase: 'JSON',
  mixedCase: 'Mixed multipart',
  fullCase: 'Production disk reservation',
  onlineTitle: 'Production checks and business recovery are separate',
  onlineBody: 'After the 02:52 switch, 13 passive samples over about two minutes were healthy with no OOM or restart. The 03:02 query still found no successful images recorded today and upstream 403 / routing 503 errors remained. Capacity and replay checks do not prove account or Adobe recovery.',
  docsHeading: 'Read or download the documents',
  docsHint: 'Original Chinese documents archived with this edition, without subscription secrets, production credentials or customer details.',
  summaryDoc: 'Tonight’s update overview',
  summaryDocHint: 'Delivery status, resource changes, benefits and limits',
  roadmapDoc: 'Proxy and account scenario roadmap',
  roadmapDocHint: 'All 7 scenarios, switching conditions and fallback behavior',
  stressDoc: 'Tonight’s load test report',
  stressDocHint: 'All 13 cases, conditions and validation scope',
  read: 'Read',
  download: 'Download .md',
  closeDoc: 'Close document',
  docReading: 'Reading',
  original: 'Original Chinese archive',
}
