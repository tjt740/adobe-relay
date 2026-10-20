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
      "releaseSpeed": {
        "title": "Shorter release waits with rollout guards retained",
        "body": "Today: parallel preparation, reusable caches and compressed transfers. Cached preparation measured about 68% shorter. Full rollout still includes backups, draining and verification. The figures below describe specific samples, not a fixed service commitment."
      }
    },
    "updates": {
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
