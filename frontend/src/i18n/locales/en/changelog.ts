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
      adobeGroupMultiPricing: {title: 'Adobe model selection and quality billing modes', body: 'Select multiple models, edit four-decimal prices in model tabs, and switch real billing modes. Quality billing defaults on; disabling retains saved prices and restores legacy billing. Website model tiers and custom names remain available.'},
      adobePricingRevert: {"title": "Restore independent Image 2.6 pricing tables", "body": "Choose the 2.6 pricing model independently, retaining the legacy/new editor switch and saved prices. Withdraw 2/2.5 quality pricing while retaining the website model directory, naming and filters."},
      adobeGroupNaming: {"title": "Adobe model filters and quality tables for three generations", "body": "Model names prefer Adobe discovery and allow custom input. Image 2, 2.5 and 2.6 prices follow the selected model, with the legacy editor selected by default."},
      adobeImage26Pricing: {"title": "Separate resolution and quality prices for both Image 2.6 models", "body": "Adobe groups now have independent Flare and Sunburst quality price tables. Extra High and Max can be priced separately; blank cells inherit existing rates. Today’s generation quality, official names, documentation reading theme and shared login remain available."},
      adobeImage26Quality: {"title": "Image 2.6 quality, website names and API documentation updated", "body": "2.6 Flare / Sunburst support distinct Extra High and Max levels without silent downgrades. Model lists, previous-name compatibility and examples are updated, alongside today’s documentation reading theme, shared login and protected downloads."},
      docsReadingTheme: {"title": "More readable docs with updated parameters and shared sign-in", "body": "Stronger body/table contrast and dark syntax-highlighted code improve reading, with the same tab icon as the main site. Green navigation, Sub2 shared sign-in and the latest 2.6 parameters/examples remain; original downloads and copied text stay intact."},
      docsCapabilities: {"title": "Document verified 2.6 parameters and working examples", "body": "The API guide now includes a live-tested support table for both 2.6 models, transparent PNG, JPEG compression and GIF timing examples, and explicit WebP, SVG, streaming and preview limitations. Shared Sub2 sign-in, green styling, copying and protected downloads remain available."},
      docsSharedLogin: {"title": "Shared documentation sign-in, updated parameters and green navigation", "body": "Signing into Sub2 in the same browser now opens the API documentation without another login. Signed-out visitors still see the login page. Green navigation and softer parameter and code colors accompany the updated parameters and examples, with copying and downloads available."},
      docsColors: {"title": "Coordinated green navigation and softer documentation colors", "body": "The independent /docs site uses green navigation and table headings, soft teal parameters, and subdued status and restriction colors. Mint, blue-grey and warm sand code highlighting preserves complete copying, document content and access rules."},
      adobeGptImage26Gif: { title: 'Adobe 2.6 models, text-only GIFs and planning controls', body: 'Today exposes the actual Adobe gpt-image-2.6-star-swirl and gpt-image-2.6-aurora names, GIF text generation, frame controls and independent complexity planning. Static generation and editing retain the established workflow; GIF reference editing is explicitly unsupported. Image 2/2.5 calls and submission protections remain unchanged.' },
      adobeGptImage26: { title: 'Add Adobe gpt-image-2.6-star-swirl and gpt-image-2.6-aurora', body: 'Expose Adobe’s actual GPT Image 2.6 interface versions gpt-image-2.6-star-swirl and gpt-image-2.6-aurora through the same image workflow as 2.5, with exact upstream routing, 2.6 size validation and explicit quality limits. Legacy Flare and Sunburst aliases remain accepted without appearing in the model directory or UI. Existing models, defaults, checks and submission protections remain unchanged.' },
      upstreamV0219: { title: 'Upgrade to v0.2.19 with Adobe medium defaults retained', body: 'Integrates v0.2.18–v0.2.19 scoped administrator keys, streaming usage, image billing and provider fixes. Adobe auto quality remains medium. Manual IPs, batch concurrency, subscription protection and Okad integration remain available. The new per-image switch defaults off; existing Adobe billing is retained.' },
      adobeAutoMedium: { title: 'Adobe auto quality now defaults to medium', body: 'Native Adobe GPT Image requests with auto or omitted quality use medium and report the submitted tier. Explicit choices and model caps remain intact, account checks retain low quality, and scheduling, billing and submission protection are preserved.' },
      bulkTestConcurrency: { title: 'Choose bulk-test concurrency and remove the fixed subscription node cap', body: 'Bulk account tests now offer 1–15 simultaneous accounts, default 3, fixed for the batch with background progress. Subscription parsing, merging and import remove the 512-node cap. Upstream, proxy and resource constraints still apply, alongside download, port and existing business safeguards.' },
      adobeImageErrorDetails: { title: 'Specific image errors with safe submission protection', body: 'Today’s update separates Adobe parameter rejections, dependency failures, failed jobs and image fetch errors, adding codes, parameters, failure stages and request IDs. Existing settings and submission budgets remain. Accepted jobs and uncertain outcomes must not be resubmitted, and diagnostic details remain redacted.' },
      bulkAccountTestModels: { title: 'Bulk model selection and Flare defaults for Cookie and automatic tests', body: 'Today adds bulk model selection and extends the Flare default to Cookie delivery and background tests without an explicit model. Batch model snapshots, concurrency, progress, explicit choices and queue protection remain. A catalog does not prove upstream recovery; real requests record their actual outcomes.' },
      clashBackgroundTests: {"title": "Resume ordinary scheduling after 408 and keep node tests in background", "body": "Remove indefinite scheduling holds caused by 408 and safely release owned historical check gates, preserving operator pauses, quota and route protection. Imported-node tests continue after closing the dialog, with progress and individual results retained."},
      adobeRouteReview: {"title": "Check subscription traffic and review Adobe routes across sources", "body": "Today adds manual IP settings per account, visible source traffic, bounded three-source review, generation cooldown and safe submission handling. Existing features, operator pauses and configuration remain protected; network reachability and account credits are not generation proof."},
      proxyVerificationPriority: { title: 'Prioritize business capacity and lend proxies for 408 checks', body: 'Today fixes verification accounts waiting despite reclaimable capacity. Business demand comes first, followed by eligible pending verification. Unresolved 408 accounts release regular proxies and borrow one only for checks, keeping a binding after successful recovery. Today’s v0.2.17 upgrade and independent subscription management remain, with existing configuration and manual-pause rules preserved.' },
      clashSubscriptions: {"title": "v0.2.17 features and independent subscription updates", "body": "Today: integrate upstream v0.2.17 model, protocol and payment fixes while retaining Adobe, Okad and custom scheduling protections; remove the eight-URL cap, add row editing and checks alongside bulk input, and use bounded concurrency with independent source outcomes. Successful sources can update while failed sources retain saved nodes and bindings. Status icons, colored counts and scrolling improve; existing node budgets, manual account state and configuration remain protected."},
      "releaseSpeed": {
        "title": "Shorter release waits with rollout guards retained",
        "body": "Today: parallel preparation, reusable caches and compressed transfers. Cached preparation measured about 68% shorter. Full rollout still includes backups, draining and verification. The figures below describe specific samples, not a fixed service commitment."
      }
    },
    "updates": {
      adobeGroupMultiPricing: {"title": "Adobe model selection and pricing tabs", "body": "Select multiple website or custom model names and edit prices in model tabs with four decimal places.", "sections": {"selection": {"title": "Multiple models and a name preview", "body": "Choose models without closing the list. Names are joined with +, retain website and fallback discovery and custom input, and can be filtered by any selected model."}, "pricing": {"title": "Separate prices with preserved drafts", "body": "The new editor uses model tabs, resolution rows and quality columns in USD. Switching tabs or reselecting models preserves drafts; creation saves only selected models. Editing retains access to other saved model prices. Blank inherits and zero is free. Rows and columns follow website model parameters: Image 2 uses the website default size and three qualities, while 2.5/2.6 expose three resolutions and five qualities. Other model tiers are not invented, and hidden saved prices remain intact."}, "billing": {"title": "Restore Image 2 / 2.5 quality billing", "body": "Billing matches GPT Image 2, 2.5 and 2.6 prices by model, resolution and quality, with applicable multipliers. Image 2.5 Extra High / Max use tiers 6/7 to match billing. Unknown model quality tables are not guessed."}, "boundary": {"title": "Validation and scope", "body": "New and unset Adobe groups default to quality billing; saved off states remain. Website discovery is not an entitlement list or support promise. Billing regressions and two local real generations passed; this does not verify the full pool, every tier or production debits."}, "mode": {"title": "Switch actual billing modes", "body": "New and unset Adobe groups default to quality billing; saved choices are retained. Click the exclamation icon beside the title for details. Enabled quality prices take priority, blank cells inherit legacy rates, and disabling preserves prices while using legacy billing after saving."}}},
      adobeGroupNameLayout: {
        title: 'Simplify Adobe group naming',
        body: 'Local preview: hide the category selector, place the customer / group name next to the model, and rename Final name to Name preview.',
        sections: {
          layout: { title: 'Enter the model and group name side by side', body: 'Creating an Adobe group requires a model and customer / group name, with a live name preview below. The fields stack on narrow screens.' },
          preservation: { title: 'Retain drafts and model input', body: 'Category text from existing drafts remains editable within the name field. Website model discovery, fallback names, custom models and group filters remain available.' },
          boundary: { title: 'Compatibility', body: 'Existing group prices and account bindings are retained. Names organize and filter groups; they do not grant model access.' },
        },
      },
      adobePricingRevert: {
        "title": "Restore independent Image 2.6 pricing tables",
        "body": "Choose the 2.6 pricing model independently, retaining the legacy/new editor switch and saved prices. Withdraw 2/2.5 quality pricing while retaining the website model directory, naming and filters.",
        "sections": {
                "reason": {
                        "title": "Withdraw the model-bound pricing change",
                        "body": "Withdraw the November 9 group-name lock and its subsequent pricing extensions. Historical release entries remain; this entry describes current behavior."
                },
                "editor": {
                        "title": "Edit both 2.6 models independently",
                        "body": "Any Adobe group can open the new editor and independently select Flare or Sunburst, including groups with ordinary, custom or mixed names. Switching retains both drafts and saves them together. The default is legacy; closing only collapses the editor and preserves saved prices."
                },
                "billing": {
                        "title": "Restore the earlier billing and parameter behavior",
                        "body": "Quality prices apply only to the two 2.6 models again; Image 2/2.5 use their earlier base and per-model rules. For 2.5, xhigh/max again submit tier 7 and retain their request labels. The 2.6 tiers remain 6/7 with their existing generation and billing capabilities."
                },
                "preservation": {
                        "title": "Keep unrelated features and customer data",
                        "body": "Keep the Adobe website API, fallback names, custom entry, structured naming and filtering before pagination. Retain saved 2.6 prices, group names, account bindings and historical usage. Do not restore an old database or rebill past usage."
                },
                "boundary": {
                        "title": "Verification scope",
                        "body": "Check independent selection, saving both models, preservation when closed, blank inheritance and zero pricing; restore backend behavior to the prior source. Real upstream generation and billing acceptance remains a separate result, not implied by configuration or simulated checks."
                }
        }
},
      adobeGroupNaming: {
        "title": "Adobe website model filters, structured names and quality prices for all three generations",
        "body": "Model names prefer Adobe’s public website API with fallback names and custom entry. Image 2, 2.5 and 2.6 offer legacy/new pricing editors, defaulting to legacy; the new table follows the group model.",
        "sections": {
          "reason": {
            "title": "Prefer model names from the API",
            "body": "Sub2 relays the public discovery API used by Firefly, fetching current image model names on each load rather than presenting its built-in catalog as live data. Disabled, deprecated and non-image entries are excluded, while new models remain available. Failed or unusable responses use five backup names with an explicit source label. Custom input is always available."
          },
          "filter": {
            "title": "Apply filters before pagination",
            "body": "The server combines model, platform, status, search and exclusivity before counting, sorting and pagination. Official names and legacy version or Flare/Sunburst names are recognized. Custom names match a literal fixed name segment rather than a regular expression."
          },
          "naming": {
            "title": "Compose names with full model labels",
            "body": "Select or enter a model, category and customer to preview Adobe GPT Image 2.6 Flare - category - customer. Ordinary groups omit category, and the 6-cent category may omit customer. Names do not configure model availability."
          },
          "pricing": {
            "title": "Retain legacy entry with an optional quality table",
            "body": "Creation and editing default to legacy base and per-model pricing. The new table follows the named model: three qualities for Image 2 and five for 2.5/2.6 across 1K/2K/4K. Image 2.5 Extra High now submits website level 6 separately from Max level 7. Switching models preserves drafts; saving changes only the current model and preserves other stored prices. Closing the editor retains stored prices. Blank inherits; zero is free."
          },
          "compatibility": {
            "title": "Normalize only unambiguous names",
            "body": "Only names with a single identifiable model receive a normalization preview. Mixed Flare/Sunburst names, ambiguous groups and utility groups are skipped, including ordinary Image 2.6 names the user asked to retain. Naming does not change account bindings, model access or existing rates."
          },
          "validation": {
            "title": "Isolated preview and focused regression checks",
            "body": "Adobe public schemas and current website quality conversion were checked. Regression coverage includes five-model submission, usage and cost matching, zero/inheritance, model switching, legacy defaults, pagination and API fallback. Mock preview writes do not affect production groups; automated checks are not live upstream generation or billing evidence."
          },
          "boundary": {
            "title": "Scope and verification limits",
            "body": "The public directory is not an account entitlement list or a promise that Sub2 supports every listed model. Unknown or ambiguous models receive no guessed price table. No prefilled customer prices or new database migrations. November 7 live billing verification remains pending upstream recovery."
          }
        }
      },
      adobeImage26Pricing: {"title": "Separate resolution and quality prices for both Image 2.6 models", "body": "Adobe groups now have independent Flare and Sunburst quality price tables. Extra High and Max can be priced separately; blank cells inherit existing rates. Today’s generation quality, official names, documentation reading theme and shared login remain available.", "sections": {"reason": {"title": "Independent prices for higher image qualities", "body": "Image billing previously distinguished models and resolutions but could not price Extra High and Max separately. Adobe group image billing now includes dedicated price tables for both Image 2.6 variants."}, "pricing": {"title": "Fifteen price cells per model", "body": "Set a per-image USD price for each combination of 1K, 2K or 4K and Low, Medium, High, Extra High or Max. A matching cell takes priority; the applicable image billing multiplier still applies."}, "fallback": {"title": "Blank inherits; zero is free", "body": "Deployment does not populate prices or change existing customer rates. Unset cells retain the existing model, group, channel and built-in pricing order. Clearing restores inheritance; explicit zero means free. Legacy model names share the corresponding variant prices."}, "usage": {"title": "Record effective image quality", "body": "New usage records include effective image quality alongside resolution, image count and costs. Auto and omitted quality use Medium. Historical records are not guessed or rebilled; the API guide explains the settings and rules."}, "validation": {"title": "Check billing, persistence and retrieval", "body": "Regression checks cover both models, current and legacy names, three resolutions, five qualities and defaults, correlating submitted quality with usage and cost. Coverage includes blank cells, zero, multipliers, fallback, auth cache and group copying."}, "boundary": {"title": "Pricing preserves generation quality", "body": "Extra High and Max retain their actual requested quality even when a price is unset. This applies to supported native Adobe Image 2.6 paths, without expanding GIF editing or other capabilities. Relay responses without effective quality retain existing billing."}}},
      adobeImage26Quality: {
        "title": "Distinct Extra High / Max quality and Adobe website names for Image 2.6",
        "body": "GPT Image 2.6 Flare and Sunburst now use separate Extra High and Max levels, removing the former High cap. Model directories, connection-test choices and API documentation use the website names.",
        "sections": {
          "reason": {
            "title": "Follow current verified upstream capabilities",
            "body": "Adobe now offers five quality levels for both 2.6 variants. The former relay accepted xhigh/max but capped them at High. Applied submission levels now determine the result; a successful response alone is not evidence that a higher level was used."
          },
          "mapping": {
            "title": "Apply Extra High and Max separately",
            "body": "quality=xhigh selects Extra High and quality=max selects Max; responses preserve xhigh and max. Low, medium, high and the medium default for auto remain unchanged. Unknown 2.6 quality values and high-level 2.6 requests mapped to another model are rejected explicitly instead of silently downgraded."
          },
          "names": {
            "title": "Match the Flare and Sunburst names on Adobe’s website",
            "body": "The interface shows GPT Image 2.6 Flare and GPT Image 2.6 Sunburst. New API integrations use gpt-image-2.6-flare and gpt-image-2.6-sunburst. Existing star-swirl and aurora names remain accepted; equivalent names work with existing allowlists for the same variant."
          },
          "usage": {
            "title": "Update examples and parameter guidance",
            "body": "API documentation includes the new names, the quality support table and two 4K Extra High / Max examples. The website’s Extra High setting uses quality=xhigh. Complexity still accepts only low, medium and high; JPEG encoding quality is a separate control."
          },
          "compatibility": {
            "title": "Preserve existing calls and explicit restrictions",
            "body": "Previous names route to their original variants, explicit account mappings retain priority, and permissions do not cross between Flare and Sunburst. Static generation, edits, URL and Base64 delivery share the quality rule. Image 2, 1.5, 2.5 and submission protections are preserved."
          },
          "validation": {
            "title": "Verify submitted levels and actual outputs",
            "body": "Before release, both variants were tested once with Extra High and once with Max. All four real upstream generations returned complete 3840×2160 PNGs with the submitted levels verified. Regressions cover names, allowlists, explicit mappings, quality rejection and delivery paths; generation logs record applied levels for public-endpoint verification."
          },
          "boundary": {
            "title": "Keep the supported scope clear",
            "body": "Higher levels may take longer and remain subject to upstream entitlements, load and account state. Static 4K samples do not prove high-level GIFs, live edits, all-account or peak-load reliability. Accepted or uncertain requests remain protected against repeat submission."
          }
        }
      },
      docsReadingTheme: {
        "title": "Improve documentation readability and unify its favicon",
        "body": "Make body text and code easier to read, with clearer separation between parameters, strings and keywords.",
        "sections": {
          "reason": {
            "title": "Distinguish text and code more clearly",
            "body": "The previous green-grey tones were too similar, with small code and parameter text. Reading feedback led to stronger separation between prose, tables and examples while retaining green navigation."
          },
          "reading": {
            "title": "Dark text on white with clear table headings",
            "body": "Body text increases to 16px, with larger inline parameters and table text. Current navigation and headings use deep green with white text; parameters have distinct light chips. Wide tables and code scroll within their own containers."
          },
          "code": {
            "title": "Separate, colorful code blocks",
            "body": "Examples use a dark surface and brighter text, with distinct colors for keys, strings, keywords, functions and numbers. Code is 14px on desktop and 13px on narrow screens, with clearer language labels and copy controls. Highlighting changes presentation only; copying preserves the complete source."
          },
          "icon": {
            "title": "Match the main-site tab icon",
            "body": "Documentation reuses the same creative.cheap SVG favicon on its login page, document and section links."
          },
          "validation": {
            "title": "Content, presentation and access checks",
            "body": "Chrome at 1440/390 widths passed all 21 exact copies, table-text comparisons, search and section navigation. Administrator, regular-user, two-factor and shared-session checks passed. Checked text contrast exceeds 4.5:1 and code colors reach at least about 9.1:1. Public checks verified the theme, favicon, download and logout."
          },
          "boundary": {
            "title": "Preserve content and access rules",
            "body": "API parameters, all 21 examples and the original Markdown remain unchanged. Sub2 shared sign-in and independent documentation login remain available. A 390-pixel Chrome viewport is not physical-phone or other-browser acceptance."
          }
        }
      },
      docsCapabilities: {
        "title": "Document live-tested 2.6 parameters and limitations",
        "body": "Describe verified public-endpoint behavior, including effective controls, explicit rejections and parameters that are accepted without producing the requested feature.",
        "sections": {
          "reason": {
            "title": "Keep other providers’ capabilities separate",
            "body": "Both 2.6 models were checked through the public image endpoint. The guide covers this Adobe-native channel and does not infer WebP, SVG or streaming support from familiar parameter names."
          },
          "parameters": {
            "title": "Separate planning, rendering and encoding controls",
            "body": "complexity accepts only low, medium and high, independently of quality and JPEG output_compression. Transparency uses PNG. GIF supports 2–16 frames, defaults to 9 frames and 200ms, requires at least 20ms and rounds 25ms to 30ms. Fixed 896×896 supports at most 9 frames; larger counts use auto."
          },
          "examples": {
            "title": "Add three complete request examples",
            "body": "New curl examples cover transparent PNG, JPEG encoding quality 25/90 and a 16-frame GIF, with file-saving and model-substitution guidance. The Python format list is corrected. The guide now contains 21 code examples."
          },
          "unsupported": {
            "title": "Make unavailable or ineffective controls explicit",
            "body": "WebP, SVG, svg.model, stream=true and progressive previews are marked unsupported. Errors are documented for complexity none/xhigh/max, transparent JPEG and custom GIF compression. partial_images=2 on a non-streaming static request still returns only a complete image."
          },
          "validation": {
            "title": "Inspect output bytes and document interactions",
            "body": "The two models produced 14 real outputs and 26 expected parameter rejections. Checks inspected PNG alpha, JPEG encoding, GIF frame counts and per-frame delays. All 21 examples passed syntax and parameter checks, with Chrome desktop/narrow viewport copying, content, download and sign-in regressions."
          },
          "boundary": {
            "title": "Document existing capabilities only",
            "body": "An earlier upstream authentication 401 is retained separately and is not counted as a pass. These samples do not prove whole-pool availability or peak capacity. Only the independent documentation service changes; business application, configuration and billing rules remain unchanged. This documentation release submits no new image generations."
          }
        }
      },
      docsSharedLogin: {
        "title": "Open API documentation with your existing Sub2 sign-in",
        "body": "Avoid repeated sign-ins between the console and documentation while keeping document and download access authenticated.",
        "sections": {
          "reason": {
            "title": "Reuse an existing sign-in",
            "body": "Opening /docs after using the console reuses a valid Sub2 session in the same browser and origin. Without a session, email/password login, registration and recovery links remain available."
          },
          "access": {
            "title": "Administrator and regular-user access",
            "body": "Active administrators, regular users and completed two-factor sign-ins are supported. The server validates each document read and Markdown download. Pending verification, invalid sessions and disabled accounts cannot gain access from browser profile data."
          },
          "refresh": {
            "title": "Renew an expired session",
            "body": "Expired access credentials can be renewed when a valid refresh session remains, coordinated with the console and other documentation tabs. Late responses cannot restore a signed-out or replaced account. Failed authentication returns to login."
          },
          "logout": {
            "title": "Defined sign-out scope",
            "body": "Documentation opened through Sub2 responds to Sub2 sign-out or account switching in the same browser. Signing out of documentation displays its login page while retaining the console session; reopening can reuse it again. Independent email/password sign-in retains its own documentation session. A 502 error during console sign-out has also been corrected."
          },
          "validation": {
            "title": "Sign-in, download and renewal checks",
            "body": "Local Chrome checks passed administrator, regular-user, two-factor, multi-tab renewal, sign-out and account-switch flows. Public Chrome verified real administrator console sign-in, automatic entry, exact Markdown download, one controlled renewal and both sign-out scopes. Widths of 1440 and 390 had no page overflow."
          },
          "boundary": {
            "title": "A valid session in the same browser",
            "body": "Private windows, other browsers and unavailable browser storage still require sign-in. The live renewal check replaced access credentials in an isolated test session rather than waiting for natural expiry. Regular-user and two-factor checks used a local simulated upstream; a 390-pixel viewport is not a physical-phone test."
          }
        }
      },
      docsColors: {
        "title": "Coordinate API documentation around green navigation",
        "body": "Color distinguishes sections, parameters and model capabilities while reading the reference.",
        "sections": {
          "reason": {
            "title": "Coordinate navigation and supporting colors",
            "body": "Following reading feedback, navigation uses green with coordinated parameter, status and code accents to reduce distracting color changes and keep the page simple."
          },
          "presentation": {
            "title": "Color follows content",
            "body": "Navigation and table headings are green, parameters soft teal, supported states muted green and restrictions restrained warm amber. Text labels remain, and the current reading section is highlighted."
          },
          "examples": {
            "title": "Highlight and copy complete examples",
            "body": "Bash, JSON and Python use mint, sage, blue-grey and warm sand highlighting on a dark green-grey surface, with separate language labels and copy controls. Copying preserves original text; wide tables and code scroll inside their containers on narrow screens."
          },
          "access": {
            "title": "Keep concise login and existing access",
            "body": "Login retains email, password, the sign-in button and recovery/registration links. Administrators, regular users and existing two-factor authentication retain the same rules. Documents and Markdown downloads still require authentication."
          },
          "validation": {
            "title": "Browser and content verification",
            "body": "Chrome at 1440 and 390 widths passed administrator, regular-user and two-factor flows. All 18 examples retain exact display and clipboard text, all table cells remain unchanged, and search, current-section and page-width checks passed."
          },
          "boundary": {
            "title": "Presentation only",
            "body": "API parameters, examples, model capabilities and authentication logic are unchanged. A 390-pixel viewport check is not physical-phone or other-browser acceptance."
          }
        }
      },
      adobeGptImage26: {
        title: 'Add Adobe GPT Image 2.6 models',
        body: 'Expose Adobe’s actual gpt-image-2.6-star-swirl and gpt-image-2.6-aurora versions through the existing image generation and editing flow.',
        sections: {
          reason: { title: 'Expose models already enabled upstream', body: 'Adobe’s discovery service reports two enabled and healthy GPT Image 2.6 versions while the Firefly menu may omit them. Sub2 exposes these versions using their actual Adobe interface names, star-swirl and aurora.' },
          models: { title: 'Show the exact Adobe interface version names', body: 'The public names are gpt-image-2.6-star-swirl and gpt-image-2.6-aurora, and each routes to the matching Adobe version. Legacy Flare and Sunburst aliases still resolve to the same internal families, while model lists and allowlist presets use only the exact version names.' },
          workflow: { title: 'Reuse the established image workflow', body: 'Generation, edits, reference images, task polling, downloads, output formatting, no-replay handling, image counting and account scheduling reuse the existing GPT Image path. Detection defaults, Adobe auto medium and existing account mappings remain unchanged unless an operator explicitly enables a new model.' },
          quality: { title: 'Respect the 2.6 quality and size schema', body: '2.6 keeps auto and omitted quality at detailLevel 3, maps low to 1 and high, xhigh or max to the model cap of 5. Explicit sizes are checked for 480–3840 dimensions, multiples of 16, total pixels between 655,360 and 8,294,400 and an aspect ratio no wider than 3:1.' },
          validation: { title: 'Validation and live rollout evidence', body: 'Focused backend, race, frontend, type and production-build checks cover mapping, payloads, quality labels, pricing, model lists and size boundaries. After the exact application was healthy on the primary server, the requested Adobe account completed one Flare and one Sunburst account-level test with HTTP 200 and test_complete=true. Separate public-key samples returned upstream 408 at submit and were recorded without replay; these samples verify the requested account and deployed chain, not every account or peak throughput.' },
          boundary: { title: 'Preserved limits', body: 'The 2.6 discovery record is alpha and healthy, which does not guarantee every Adobe account is entitled or that every upstream task succeeds. No database migration, login, cookie push, proxy, scheduler or default detection change is included. Unsupported 2.6 sizes fail locally and are never silently resized.' },
        },
      },
      adobeGptImage26Gif: {
        title: 'Adobe GPT Image 2.6 GIF and planning controls',
        body: 'Accept complexity, frame_count and frame_duration_ms for gpt-image-2.6-star-swirl and gpt-image-2.6-aurora while keeping GPT Image 2 and 2.5 unchanged.',
        sections: {
          complexity: { title: 'Control planning independently of quality', body: 'complexity accepts low, medium or high for generation and static edits, including JSON and multipart. The 2.6 path recognizes PNG/JPEG files uploaded with a generic MIME type by common SDKs. Omission preserves the upstream default. Explicit planning values reach Adobe while rendering quality remains independently controlled by quality. This new option is enabled only for the two 2.6 models.' },
          reason: { title: 'Match the public GIF contract', body: 'The image endpoint now recognizes the documented output_format=gif request and the two frame controls instead of dropping them before the Adobe path.' },
          parameters: { title: 'Validate before spending an upstream request', body: 'frame_count accepts 2–16 and defaults to 9; frame_duration_ms accepts integer values of at least 20 milliseconds, defaults to 200, rounds to 10 milliseconds and caps the encoded delay at 655350 milliseconds. Omitted or auto size lets Adobe choose a frame size that fits. Fixed 896x896 supports up to 9 frames; use auto for 10–16. Other explicit sizes, over-budget combinations, masks, streaming, partial images and custom output compression are rejected before submission.' },
          delivery: { title: 'Preserve the animated bytes', body: '2.6 GIF output is forwarded in the Adobe model-specific payload, bypasses PNG/JPEG transcoding, is verified as GIF after download and is delivered as Base64 or a configured image URL. The response format is derived from the delivered bytes. Each animation counts as one image and its billing tier uses the actual frame dimensions.' },
          compatibility: { title: 'Keep older image workflows stable', body: 'GPT Image 2 and 2.5 retain their existing PNG/JPEG conversion and size behavior. Frame controls are forwarded through the compatible Responses request builder as well.' },
          validation: { title: 'Validation before release', body: 'Live-account samples pass static generation and JSON edits for Image 2, both 2.5 variants and both 2.6 variants, plus 2.6 multi-reference, multipart static edits and 2K high-quality/high-complexity generation. Text-only GIF samples cover 2/4/9/16 frames, defaults, delay rounding and delivered bytes. Regression checks cover invalid inputs, local rejection and older-model behavior. These samples do not establish pool-wide availability or peak throughput.' },
          boundary: { title: 'Provider limits remain visible', body: 'GIF currently supports text-only generation. GIF reference edits returned upstream 500 for both models, so GIF edit endpoints and reference-image requests return 400 / gif_edit_unsupported before upload or generation. Static PNG/JPEG edits remain supported. SVG is outside this release. Older models do not gain GIF support.' },
        },
      },
      upstreamV0219: {
        title: 'Integrate upstream v0.2.19 features and fixes',
        body: 'Updates the v0.2.17 code with two upstream releases while retaining this site’s Adobe, Kiro, proxy and integration features.',
        sections: {
          reason: { title: 'Upgrade the current application', body: 'Integrates official v0.2.18 and v0.2.19 changes for streaming accounting, image uploads and provider compatibility, and adds scoped administrator keys. The application version is 0.2.19 and existing features remain available.' },
          access: { title: 'Authorize administrator keys by operation and scope', body: 'Keys can restrict operations, groups, account types, providers, expiration and source IPs. Unselected operations are denied. Shared account changes still affect associated groups. Full keys appear only at creation or rotation, and revoked records remain auditable. The upgrade creates no keys and expands no existing grants. The interface uses an in-app confirmation dialog and individual group and provider checkboxes.' },
          images: { title: 'Apply image billing by endpoint and configuration', body: 'OpenAI and Composite groups gain an opt-in switch to bill the number of images actually returned. It defaults off and applies only to dedicated image endpoints. Mixed Responses and Chat requests retain text token costs. Fixes Gemini fileData image accounting and lost token usage on per-request channel image tiers. Native Adobe images retain this site’s image count, resolution and multiplier billing. Auto quality stays medium and account checks stay low.' },
          streaming: { title: 'Handle interrupted streams and incomplete responses', body: 'Anthropic compatibility streams continue collecting upstream usage after client disconnection and report incomplete upstream responses as protocol errors. Gemini records usage collected before disconnects. Fixes negative input usage on cached responses, shared rate-limit snapshot writes and duplicate model refreshes during cold starts. Accepted Adobe tasks and requests with unknown results remain protected from resubmission.' },
          models: { title: 'Refresh model catalogs and provider compatibility', body: 'Adds GPT-6.1 Terra support and Grok catalog diagnostics with source, trust state and draft preservation, preventing old requests from overwriting newer results. Gemini defaults remove retired 2.x and 3-pro-preview models; checks use gemini-3.5-flash. Legacy models can be explicitly mapped or allowlisted. Fixes DeepSeek caching, pricing and developer-role compatibility while retaining Kiro’s own catalog, quota and cache features.' },
          preservation: { title: 'Retain current accounts, configuration and scheduling', body: 'Retains Adobe mappings, Cookie recovery rules, manual pauses and IPs, business priority and three accounts per proxy, batch model selection and concurrency from 1 to 15, and subscription nodes and traffic protection. Additive migrations introduce administrator keys and the image billing switch while retaining historical records. Upstream migration enables image generation for existing Gemini and Antigravity groups; Composite groups still require explicit opt-in.' },
          validation: { title: 'Validation scope', body: 'An isolated environment checks migration replay, historical checksums and existing business records, permission scope isolation, image billing, Adobe quality defaults, manual proxies and integration regressions. Frontend types and production builds are also checked. Full test results and existing failures are recorded separately, without deleting checks or changing production data to hide differences.' },
          boundary: { title: 'Scope and limits', body: 'New provider features require suitable accounts and group configuration. Gemini image generation in Composite groups follows the image-generation switch. Grok 4.7 Fast uses the corrected official Fast pricing. Downloads reject non-public destinations and oversized uploads. Catalog or connection checks do not guarantee all models work, and upgrade validation does not establish pool-wide recovery or peak throughput.' },
        },
      },
      adobeAutoMedium: {
        title: 'Default auto image quality to medium',
        body: 'Text generation and image edits with auto or omitted quality submit the medium tier and return medium on success.',
        sections: {
          reason: { title: 'Raise the automatic quality default', body: 'Previously, auto and omitted quality selected the lowest tier and returned low. Requests that rely on the default now use medium while explicit quality choices continue to control generation.' },
          default: { title: 'Keep submission and response labels consistent', body: 'Native Adobe GPT Image 1.5, 2 and 2.5 requests with auto, blank or omitted quality submit detailLevel=3 and return quality=medium on success. Case and surrounding whitespace are normalized. Generation, edits, multiple images, Base64, URLs and storage fallback share this rule. The label describes the submitted setting rather than grading the resulting image.' },
          explicit: { title: 'Retain explicit choices and model caps', body: 'Explicit low remains tier 1, medium remains 3 and high remains 5. GPT Image 2.5 retains tier 7 for xhigh and max and preserves their distinct response labels; older models keep their existing caps. Unknown quality values retain the low fallback. Other models and relay accounts keep their existing behavior.' },
          detection: { title: 'Keep account checks at low quality', body: 'Manual, bulk, Cookie-triggered and background account checks explicitly use low, retaining their existing 1K test and workload. Raising the business default does not raise the detection tier or trigger batch retests.' },
          validation: { title: 'Validation scope and live request limits', body: 'Isolated regressions inspect upstream settings and response labels for defaults, explicit tiers, model mapping, edits, multiple images and delivery paths, and verify low-tier account checks. One pre-release production request with explicit medium successfully returned a 1K image. That sample verifies the existing medium path and cannot alone prove the new auto default, pool recovery or peak throughput.' },
          boundary: { title: 'Scope and practical costs', body: 'The default applies to native Adobe GPT Image business requests handled by the new version. Historical images remain unchanged. Per-image and resolution billing, model mappings, manual pauses, proxies, concurrency, resources and alerts are preserved. Higher quality may affect latency and Adobe credit consumption; actual results determine the impact. Accepted or uncertain generation requests must still never be resubmitted.' },
        },
      },
      bulkTestConcurrency: {
        title: 'Choose 1–15 simultaneous bulk account tests',
        body: 'Choose the number of simultaneous account tests before starting, default 3, for sequential diagnosis or faster checks when capacity permits.',
        sections: {
          reason: { title: 'Match test speed to the task', body: 'Each page previously allowed at most three simultaneous account tests, with no way to reduce this to one or increase it to match available exits and upstream capacity. The new dropdown offers integers from 1 to 15 without duplicating tests for selected accounts.' },
          selection: { title: 'Fix settings when the batch starts', body: 'Each new batch defaults to 3. Choose a model and concurrency before starting; both are then fixed. Actual concurrency cannot exceed the unique target count. Closing and reopening the dialog retains the same model, concurrency and results, and the background card displays batch concurrency.' },
          queue: { title: 'Fill each freed slot', body: 'The queue uses the chosen concurrency and continues after individual failures. Stopping or an expired admin session prevents queued tests and aborts current client requests; leaving the page keeps existing cancellation behavior. An accepted upstream generation may continue, so stopping does not cancel its cost or justify resubmission.' },
          risk: { title: 'Costs of higher concurrency', body: 'Tests make real upstream requests; Adobe generates test images. Higher concurrency concentrates quota use and increases potential 429, 408, proxy, image-transfer and memory pressure. Three accounts on one node do not provide three independent exits. Other pages, Cookie checks, scheduled tests and customer traffic can add to the load.' },
          validation: { title: 'Validation scope', body: 'Isolated regression checks cover 1, 3 and 15 workers, slot refill, deduplication, small target sets, running locks, background display, reopening, new-batch defaults, invalid input, stopping and session expiry. Requests are simulated; this does not prove production fifteen-way generation, upstream recovery or peak throughput.' },
          boundary: { title: 'Only the manual batch on this page', body: 'This is not a site-wide testing limit and does not change account business concurrency, the three-accounts-per-node capacity, automatic checks or scheduled tests. Model mappings, operator pauses, manual proxies, quota, subscription traffic and expiry, credentials and submission protections continue to apply. The default of 3 retains previous intensity; 15 does not guarantee five times the speed.' },
        },
      },
      clashNodeCount: {
        title: 'Remove the 512-node subscription limit',
        body: 'Remove fixed node-count checks from parsing, merging, selection and failed-source preservation so larger pools can be previewed in full and imported as needed.',
        sections: {
          reason: { title: 'Valid merged subscriptions should not fail on count alone', body: 'The early implementation limited processing to 512 nodes. Combining valid feeds or retaining nodes from a temporarily unavailable source could therefore reject the entire operation. That threshold did not reflect download size, network concurrency or available ports.' },
          behavior: { title: 'Parse, merge and select the full pool', body: 'Clash YAML and Base64 feeds no longer fail because they contain more than 512 nodes. Merged feeds can still be previewed, selected and imported. When a source fails temporarily, its enabled nodes remain available and its previously disabled nodes stay disabled.' },
          protection: { title: 'Keep resource and business safeguards', body: 'Subscription URL text remains limited to 64 KiB, each feed response to 8 MiB, and shared fetching to four concurrent requests. Existing timeouts and request-size limits remain. Node validation, source identity, duplicate selection, revision conflicts, manual pauses, account bindings, credentials, subscription traffic and expiry checks follow their existing rules.' },
          validation: { title: 'Validation scope', body: 'Local synthetic fixtures cover 1200-node YAML and Base64 parsing, merging three 400-node feeds, and previewing and importing 520 new nodes alongside failed-source nodes in isolated PostgreSQL. Checks cover disabled state, listener credentials, account bindings, manual pauses and no writes when every source fails. The simulated controller and TCP listeners do not establish real Mihomo forwarding or production upstream availability.' },
          boundary: { title: 'Removing a count limit does not make resources unlimited', body: 'Preview and merging have no fixed node-count ceiling. Import still requires an available dedicated listener port per node. The existing allocation range remains at most 4096 ports from the configured start, never above port 65535. Retained nodes and old listeners reserved during replacement occupy ports. Port exhaustion and request-size limits still produce errors, and larger pools may take longer to load and apply.' },
        },
      },
      adobeImageErrorDetails: {
        title: 'Specific image request errors and failure reasons',
        body: 'Add explicit error codes, affected parameters, failure stages and request correlation to Adobe image errors, separating parameter rejection from upstream processing failures.',
        sections: {
          reason: { title: 'Separate failures previously grouped under HTTP 400', body: 'Some dependency failures, failed jobs and incomplete upstream responses previously returned 400, while fixed messages hid useful validation rules. Clients could not reliably decide whether to change parameters or investigate the upstream service.' },
          classification: { title: 'Classify status and cause independently', body: 'Confirmed parameter rejections remain 400. Upstream dependency failures, missing task or image URLs and failed generation jobs return 502. Network failures and timeouts fetching input images return 502 and 504 respectively. Local processing faults are separated from parameter errors; existing authentication, quota, entitlement and content safety policies remain.' },
          details: { title: 'Expose codes, parameters and failure stages', body: 'Return a stable code, a confirmed param when available, and the validation, prepare, input_fetch, upload, submit, poll or download stage, plus the source, available original upstream status and gateway request ID. Recognized dimension errors explain the applicable size rule. Unknown errors do not guess a parameter. OpenAI errors retain their envelope; Gemini adds details within its existing Google format.' },
          privacy: { title: 'Extract specific reasons through controlled rules', body: 'Only confirmed rules, controlled codes and necessary numeric values are exposed, never raw provider payloads. Cookies, tokens, signed image URLs and internal addresses remain hidden. Administrative diagnostics retain redacted evidence with additional searchable code, parameter and source fields.' },
          protection: { title: 'Status changes do not authorize new generation attempts', body: 'A 502 or 504 response does not authorize another generation. Existing duplicate submission protection remains for accepted jobs, poll or download failures and unknown submission outcomes. Same-job GET retries, proven pre-acceptance submission budgets, account settings, manual pauses, proxies, concurrency, resources and alerts remain unchanged.' },
          validation: { title: 'Validation scope', body: 'Isolated regressions cover size rejection, 424, incomplete upload and generation responses, failed jobs, download failures, local parameters and input image network errors, bilingual notes, protocol compatibility and redaction. They count generation POSTs and check that accepted or uncertain requests are not resubmitted. Mocked upstream tests do not prove production generation or provider recovery.' },
          boundary: { title: 'Applicability and diagnostic boundaries', body: 'Requests handled by the new version receive detailed errors; historical errors are not rewritten. Native Adobe requests use this classification while relay accounts retain their existing protocol handling. Matching timestamps or status codes cannot establish the cause of a screenshot; correlate complete evidence using the request ID. Clients must honor terminal no-retry markers, which do not replace existing server submission protection.' },
        },
      },
      cookieRecoveryRevert: {
        title: 'Revert the recent Cookie recovery changes',
        body: 'Restore the previous Cookie recovery, retry and login-profile behavior while retaining multiple Supply tokens.',
        sections: {
          scope: { title: 'Restore the previous behavior', body: 'Revert the increased retry limit, cooldown changes and default-profile override during automatic recovery. The existing behavior is restored without adding another automatic login flow.' },
          preservation: { title: 'Keep current business data', body: 'Accounts, Cookies, tokens, Sub2 links, manual pauses, selected proxies and configuration are retained. Existing recovery counts are not reset and no old database snapshot is restored. Independent token saving, tests, email routing and hourly checks remain available.' },
        },
      },
      cookieRecoveryRetries: {
        title: 'Two fallback retries for expired Cookie recovery',
        body: 'Confirmed expired Cookies automatically call Okad, with two retries after the first failure and at most three logins. Recovery uses the default profile of the Adobe login session.',
        sections: {
          retry: { title: 'Three attempts, five minutes between failures', body: 'Requests and background checks share durable attempt counts. Restarts, Cookie updates and duplicate requests do not reset them. Historical Cookie failure messages are recognized. Cooldown rejections spend no attempt; when no deadline is provided, the next check is in five minutes rather than fifteen. Accounts at the former two-attempt limit retain their count and receive one final opportunity.' },
          profile: { title: 'Use the default profile during recovery', body: 'Automatic recovery ignores personal or organization preferences and does not force a profile switch. Manual logins keep the saved choice. The Cookie, token and current identity remain consistent.' },
          protection: { title: 'Preserve account and integration safeguards', body: 'Manual pauses, disabled or expired accounts, entitlement and quota failures do not trigger logins. Multiple-token mailbox routing, existing Cookies, account IDs, Sub2 links and concurrency deduplication remain in place. Successful recovery still requires credential and connection checks.' },
        },
      },
      adobeAutomaticTestFlare: {
        title: 'Flare by default for Cookie delivery and automatic tests',
        body: 'Extend the bulk-test default to connection tests after Cookie delivery, background tests without an explicit model, and single-account tests: GPT Image 2.5 Flare.',
        sections: {
          reason: { title: 'Fix the remaining Image 2 backend default', body: 'The first release today added the bulk-dialog model picker. Cookie delivery queues and background tests still passed an empty model to the backend, which selected GPT Image 2 as the first catalog entry. They therefore did not inherit the bulk default.' },
          default: { title: 'Use one default across unspecified-model tests', body: 'Adobe tests now have a dedicated GPT Image 2.5 Flare default shared by ordinary Cookie delivery queues, bounded cross-source review, background tests and test API requests without a model. The single-account dialog selects Flare when the account catalog includes it, otherwise retaining the available account choices.' },
          consistency: { title: 'Keep submission and health records aligned', body: 'The actual submission, test lease scope, per-model health and route review resolve the same default model, so a Flare test is not recorded against Image 2. Account model mappings still apply. Catalog ordering and defaults for customer business requests remain unchanged.' },
          protection: { title: 'Keep explicit models and existing queue rules', body: 'Image 2, Sunburst and other explicitly selected models remain unchanged, including models saved in existing plans. Cookie and proxy ownership, manual pauses, 408 scheduling, quota and verification gates stay in place. No bulk enqueue or extra login is added, and accepted or uncertain generation is not replayed to change models.' },
          validation: { title: 'Validation scope', body: 'Isolated tests check real submission parameters for empty and whitespace models, Cookie delivery queues, bounded cross-source review, explicit background models, custom account mappings and test lease scopes. Frontend regressions cover the Flare default and explicit selection. Mock upstream results do not prove production generation or upstream recovery.' },
          boundary: { title: 'When the default takes effect', body: 'Once the new version takes over, pending tests without an explicit model use Flare. Tests already started on the old version finish naturally, and historical Image 2 results remain. Availability still depends on account entitlement, mapping, routing and the actual upstream response; a new default does not clear errors for other models or force recovery.' }
        }
      },
      bulkAccountTestModels: {
        title: 'Choose a model for bulk account tests',
        body: 'Choose a model before starting a bulk account test. Adobe defaults to GPT Image 2.5 Flare.',
        sections: {
          reason: { title: 'Verify the model you need', body: 'Bulk tests previously sent an empty model parameter, so Adobe used GPT Image 2. The selector allows separate checks of Flare, Sunburst, Image 2 and other models. A result for one model does not establish that another model works.' },
          selection: { title: 'Choose before starting', body: 'Opening the dialog only prepares the selected accounts and loads the first account’s model list. GPT Image 2.5 Flare is selected when available; choose another model or each account’s default instead. Only Start testing sends upstream tests. The chosen model applies to every account in the batch and is locked once testing starts.' },
          progress: { title: 'Keep progress and existing controls', body: 'The three-request concurrency limit, account deduplication, individual failure details, stopping, session expiry handling and list refresh remain. Closing the dialog continues the same batch, the background card shows its model, and reopening does not duplicate tests. Failed model loading can be retried without sending tests early.' },
          validation: { title: 'Validation scope', body: 'Local isolated regressions cover the Flare default, selected model request parameters, consistent queued requests, default model fallback, model loading and stale responses, closing and reopening, bounded concurrency, stopping and session expiry. A local browser with simulated data also verified model switching, locking during a run, background results, Chinese light mode and English dark mode. No production image generation or mobile layout acceptance was performed; actual upstream availability still depends on real test results.' },
          boundary: { title: 'Model support and quota', body: 'The model list comes from the first selected account; choose a model supported by all accounts in the batch. Unsupported models produce real failures for the affected accounts. Tests use upstream quota and Adobe generates a test image. Refreshing, closing the browser page or leaving account management stops pending tests. This change applies to bulk tests only; defaults for single-account tests, scheduled tests and business requests stay unchanged.' }
        }
      },
      adobe408TimeoutScheduling: {"title": "Stop indefinitely disabling accounts after 408 timeouts", "body": "Remove the persistent 408 scheduling hold and the requirement for a successful generation test before scheduling can resume. Timeout observations and independent account restrictions remain.", "sections": {"reason": {"title": "Timeouts do not prove invalid credentials", "body": "An upstream 408 or local network/check timeout does not establish that a Cookie is invalid. The previous hold could indefinitely prevent ordinary traffic even when other account conditions were satisfied."}, "behavior": {"title": "Use ordinary scheduling rules", "body": "408 no longer creates a persistent scheduling block or automatically releases a regular proxy binding. Temporary source reviews return only the eligibility they borrowed on failure, cancellation or expiry, without requiring generation success. A node cooldown caused by multiple accounts expires so ordinary traffic can retry it."}, "existing": {"title": "Release owned historical timeout pauses", "body": "The upgrade returns only pauses with recorded 408 resume ownership, plus completed timeout checks matching the current Cookie and credential-verification requirements. Accounts already paused are not bulk-enabled; other errors and newer checks remain protected. Existing temporary leases retain ownership until completion."}, "legacy": {"title": "Safely handle old proxy-waiting gates and missing check start times", "body": "Release combined proxy-waiting and failed-408 gates only with the current Cookie, verified identity and recorded resume ownership. Legacy manual results without a start time need a complete queue-to-finish window containing that result; incomplete evidence stays gated. New manual checks carry their actual request start so older results cannot finish a newer queue. Operator pauses, quota restrictions and independent cooldowns remain, with no new generation or fabricated success."}, "protection": {"title": "Preserve independent restrictions", "body": "Manual pauses, 401 authentication failures, quota and entitlement, Cookie identity checks, expiry, 429 cooldowns, subscription traffic, excluded nodes and the three-account node limit still apply. Accepted or uncertain image submissions are never replayed."}, "validation": {"title": "Validation scope", "body": "Isolated PostgreSQL regressions cover continued scheduling after timeouts, historical migration, temporary lease expiry and concurrent ownership, manual pauses and credential changes, alongside related frontend/backend checks, types and builds. A sanitized live snapshot is replayed locally to compare eligibility and IP allocation; this does not replace actual generation verification."}, "boundary": {"title": "Retain actual timeout evidence", "body": "408 codes, models, stages and alert history remain. Removing a scheduling restriction does not convert failure to success or guarantee upstream recovery. Missing proxy capacity, bounded cooldowns and independent failures may still prevent scheduling."}}},
      clashBackgroundTests: {"title": "Run imported-node tests in the background", "body": "The application now owns the test queue, so closing its dialog no longer discards pending nodes.", "sections": {"reason": {"title": "Free the dialog during long test runs", "body": "With many nodes, the previous workflow required the dialog to stay open; removing the component stopped pending tests. Use Run in background, the close icon or Esc to dismiss it and keep using the page."}, "progress": {"title": "Return to the same progress and results", "body": "IP Management displays completed and total counts with a progress bar, followed by available and failed counts. Open progress / results or reopen Clash subscriptions to see the same run, including individual latency and failure details. Navigating within the app also retains the task."}, "protection": {"title": "Keep bounded concurrency and editing guards", "body": "One batch runs per browser session, with at most four concurrent nodes. Only nodes imported and enabled at the start are tested, and duplicate starts are ignored. Importing or replacing subscriptions within the dialog stays disabled during testing; closing guards during loading, previewing and saving remain. Signing out or changing users stops further queued tests from that session."}, "validation": {"title": "Validation scope", "body": "Regression coverage includes a seven-node queue after dialog closure and destruction, four workers, reopening progress and results, duplicate starts, isolated failures, user changes and list refresh after background completion. Isolated connection checks do not establish Adobe image availability or production peak throughput."}, "boundary": {"title": "Limits of browser background work", "body": "Keep the browser page open. Reloading or closing it stops tests that have not started; completed server health records retain their existing behavior. Background testing does not change proxy bindings, manual account state, subscription traffic protections, scheduling, credentials, concurrency limits or alert settings. Release preparation allows more time for cold builds and bounds cache-export waits. Formal checks and artifact verification remain required; cache availability and network conditions affect release time."}}},
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
            "body": "Manual mode pins the proxy selection; its provider may still change the public egress IP. Administrators maintain the selected route. Exhausted or expired subscriptions, the exclusion of node 1290 and generation cooldown remain enforced. Explicit deletion of a proxy or subscription retains cleanup protections. Models, quota, concurrency, alerts and Cookie recovery rules are unchanged."
          }
        }
      },
      adobeRouteReview: {
  "title": "Subscription traffic visibility and Adobe review across sources",
  "body": "IP management shows remaining traffic, expiry and observation time for every Clash source, with source IDs on individual nodes. Explicitly unaccepted Adobe 408/429 submissions can be reviewed through up to three different sources; generation health is independent of basic reachability.",
  "sections": {
    "reason": {
      "title": "Why credits and website success can coexist with failures",
      "body": "Adobe image credits and proxy subscription traffic are separate allowances. A funded account can still receive 408 through a specific route even when website generation works. HEAD reachability is also insufficient proof of real generation. Subscription quota, network probes and generation evidence now remain separate."
    },
    "balance": {
      "title": "Find the actual source and remaining traffic",
      "body": "Collapsed source cards also show traffic, expiry and observation time. Each node shows its saved source identity and remaining traffic, rather than inferring ownership from an outdated name prefix. Known exhausted or expired sources are excluded from new automatic allocation and native Adobe submissions; existing allocation seeks another eligible route. Missing or older-than-15-minute traffic information is unknown. Previously confirmed exhaustion or expiry remains protected until valid new evidence replaces it. Values represent provider-reported traffic bytes, not money or Adobe credits."
    },
    "review": {
      "title": "Review up to three different sources",
      "body": "Manual, batch, scheduled and post-cookie full tests share the same review. For explicit unaccepted submit-stage 408/429 responses, the same account, cookie, model and parameters can use another subscription. Such rejections receive one submission per source, with at most three different sources and immediate stop on success. Three genuine failures produce a failed test with actual evidence. Insufficient eligible sources or capacity is reported as pending review without inventing three failures. Disabled automatic allocation and operator-fixed routing are respected."
    },
    "circuit": {
      "title": "Generation failures are independent of HEAD probes",
      "body": "Three different accounts receiving generation-stage 408 on the same node revision and model within five minutes pause new Adobe generation for at least five minutes. HEAD success cannot clear this state. After cooldown, only one temporary test may review the route; a newer real success on the affected model is required. New allocation favors recent real success after quota, capacity and reachability requirements. Existing healthy bindings retain affinity. One failing account does not block an entire subscription, and no generation polling loop is added."
    },
    "safety": {
      "title": "Accepted or uncertain jobs are never resubmitted",
      "body": "Poll or download failures, submit transport timeouts, 5xx and error responses carrying a job location do not prove that generation never started. Native generation therefore cannot be replayed on another account for these outcomes; existing polling and same-image download retries stay on the original job. Explicit unaccepted 401 retains the existing single session-renewal boundary. Cross-source review does not multiply the old three submissions per node or add Okad login attempts."
    },
    "protection": {
      "title": "Capacity, credentials and operator pauses remain protected",
      "body": "Reviews retain the three-accounts-per-node limit, temporary ownership and business priority. Failure or cancellation returns borrowed routing, while interrupted leases expire. Permanent retention requires genuine success, matching credential and route snapshots, recovery of every blocked model and an enabled recovery policy. Operator pauses, expiry, verification gates and scheduled AutoRecover choices remain intact. Existing subscriptions, configuration, mail, alerts and resource limits are not reset. Failed transient reviews without a real 408 return only scheduling temporarily taken by that lease; 429 keeps its existing cooldown instead of becoming a permanent 408 hold. Operator pauses, new verification gates and genuine 408 blocks remain enforced."
    },
    "validation": {
      "title": "Validation and its limits",
      "body": "Isolated database and concurrent regressions cover exhaustion, excluding sibling nodes from one source, third-source success, three failures, insufficient sources, lease release, one half-open review, operator pauses and uncertain jobs. Frontend checks cover always-visible balances, unknown traffic and retained evidence. A production case completed submission, polling and download after changing subscription; that proves only the tested account, model and route at that time, not pool-wide or all-model recovery."
    },
    "boundary": {
      "title": "Different sources do not guarantee different public IPs",
      "body": "Subscriptions can share networks and a node can change its egress IP. This phase separates reviews by subscription identity, without presenting source counts as independent IP counts. Missing traffic remains unknown. Three failures mean this test did not pass; 408 alone cannot establish invalid cookies. This bounded review extends the single-attempt test policy described on October 22 while retaining capacity reclamation and real-success requirements."
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
