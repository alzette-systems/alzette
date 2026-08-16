# Alzette OpenRouter PoC QA report

## Endpoint acquisition control-plane verification — 2026-08-14

### Verdict

The self-service endpoint-acquisition vertical slice is **PASS for the offline
PoC scope**. An authenticated organisation can browse an operator-published
catalogue, configure and acquire an eligible shared evaluation endpoint,
submit a dedicated request for an immutable operator quote, complete a
recent-human-authentication gate, follow separate commercial/payment/runtime
states, inspect endpoints and billing, and request capacity for an eligible
dedicated endpoint. Prices, aliases, targets, ownership, and activation state
remain server-owned.

This is not a production-payment, public-signup, dedicated-capacity, or
MeluXina approval. Stripe is disabled in the running PoC; no provider or Stripe
call was made. Paid shared activation requires a verified billing event, and a
dedicated endpoint still requires operator-reviewed capacity, quote, payment,
and runtime evidence. The customer pays Alzette's merchant account and does
not need a Stripe account.

The requested final read-only teammate review could not start because that
existing reviewer exhausted its execution allowance. This report therefore
does not claim an independent endpoint security sign-off; the production
security review remains an explicit release gate.

### Checks and results

- **PASS — full deterministic baseline:** `gofmt -w cmd internal migrations`,
  `go vet ./...`, `go mod verify`, `node --check portal.js`,
  `./scripts/verify-gitignore.sh`, and `docker compose config --quiet` passed.
- **PASS — fresh PostgreSQL race suite:** with a task-specific fresh
  PostgreSQL instance, `go test -race ./... -count=1 -timeout=5m` passed for
  every package. Coverage includes clean migration, down/reapply, upgrades
  from valid `0001` and `0002` schemas, deterministic repair of legacy
  duplicate key display names, two-tenant isolation, endpoint configuration,
  quote/payment replay, state guards, immutable capacity-request sizing intent,
  changed-payload idempotency conflict, and the existing buffered and streaming
  gateway contracts. The live database then upgraded additively through
  `0010_capacity_request_intent` with migration exit `0`.
- **PASS — authentication and mutation safety:** customer mutations require a
  human portal session, CSRF protection, scoped membership, and stable
  idempotency keys. Quote acceptance and payment entry require recent password
  authentication. Workload API keys remain distinct one-time credentials;
  interrupted key reveal recovery cannot create a second same-named active key.
- **PASS — state and data boundaries:** shared and dedicated acquisition use
  distinct workflows. Offer price, currency, alias, target, and eligibility
  are resolved from server records. Commercial, payment, and runtime state are
  not collapsed. Customer responses omit target URLs, provider credentials,
  attempts, prompts, and outputs.
- **PASS — browser acceptance:** visible Chromium exercised the real served
  portal shell with deterministic APIs at desktop and `390px`: catalogue,
  model detail, dedicated configuration, server-backed draft save/reload,
  submit, same-key recent-auth retry, capacity request, usage deep link,
  hosted-URL rejection, malformed-route handling, and horizontal-overflow
  checks all passed. `portal.js` restored every persisted configurator field,
  distinguished a loading draft from a restored draft, reset scroll on SPA
  route changes, and rendered the immutable capacity/sizing record. Inspected
  artifacts: `/tmp/alzette-capacity-request.png`,
  `/tmp/alzette-endpoints-mobile.png`, and
  `/tmp/alzette-implementation-docs.png`.
- **PASS — Compose/LAN smoke:** the current image rebuilt successfully;
  migration exited `0`; PostgreSQL, gateway, control, public, worker, and the
  disabled billing-webhook process are healthy. Readiness returns `200` on
  ports `19080`–`19083`. Unauthenticated portal and catalogue access fail
  closed, and the disabled webhook returns `503`.
- **PASS — truthful live catalogue seed:** the running organisation contains
  only the existing `alzette-chat` shared-evaluation offer, published from the
  operator-reviewed pilot route with a hard 100-logical-request allowance. No
  paid or dedicated offer, price, hardware, or availability was invented.

### Remaining release gates

Public self-signup, invitations, live Stripe checkout/webhook evidence,
approved provider smoke, automated MeluXina provisioning, measured dedicated
capacity, TLS ingress, backup/restore automation, rate/concurrency enforcement,
and an independent production security review remain unclaimed. These do not
block the offline endpoint-acquisition PoC, but they do block a production or
paid-capacity launch.

## Standalone public surface verification — 2026-08-14

### Verdict

The rewritten marketing landing and public implementation documentation are
**PASS**. They run under a standalone `alzette public` process, not under the
authenticated control/portal process. The landing presents the intended
private, locally operated Luxembourg offer to financial-industry buyers; the
documentation and authenticated portal preserve the exact current Slice 0–2
PoC boundary. The public container receives only the configured portal login
URL: it has no `DATABASE_URL`, provider key, or provider-secret reference. No
provider call was made during this change or verification.

### Checks and results

- **PASS — separated product and implementation truth:** the landing restores
  the intended private Luxembourg offer, dedicated production model, financial
  approval story, company visibility, governance questions, and pilot route.
  It avoids temporary provider and release-state language. The implementation
  docs continue to label the connected product `External pilot / Shared pilot`,
  distinguish compatible offline evidence from a live-provider result, and
  treat MeluXina as a gated deployment path. Contract-dependent location,
  retention, support, service level, capacity, and model commitments are
  explicitly subject to the applicable client agreement.
- **PASS — process/static isolation:** the image copies protected portal assets
  to `/app/portal` and public assets to `/app/public`. The public handler serves
  only `/`, `/index.html`, `/docs`, `/docs.html`, `/site.css`, and the river
  mark. `/portal.js`, `/.env`, and `/go.mod` return `404`; the control service
  still returns `404` for `/index.html`. `/client` returns `303` to the
  configured gentle human login.
- **PASS — deterministic regression:** `go mod verify`, `go vet ./...`, default
  tests, and the full PostgreSQL-backed `go test -race ./... -count=1
  -timeout=5m` passed. New tests cover the public allow-list, method policy,
  readiness, security headers, redirect validation, and portal/source
  containment.
- **PASS — Compose/LAN:** `docker compose config --quiet` and `docker compose up
  --build -d` passed. Migration exited `0`; PostgreSQL, gateway, control,
  public, and worker are healthy. The public service is published on
  `0.0.0.0:19082`; PostgreSQL remains loopback-only.
- **PASS — responsive browser:** visible Chromium through the Playwright skill
  loaded the landing at 1440×1000, 1024×900, and 390×844 and the docs at desktop
  and mobile widths. Every checked page/viewport had no horizontal overflow,
  console error, or page error. Keyboard skip navigation, required pilot-form
  validation, internal anchors, the separate `/client` redirect, and the
  paired mobile responsibility charter passed. Desktop, tablet, mobile, and
  full-page screenshots were visually inspected.
- **PASS — frontend craft gate:** the Impeccable context and one-time detector
  were run. The public surface was aligned to the committed paper/ink/river
  palette, display ramp, compact status vocabulary, flat-ledger composition,
  responsive breakpoints, keyboard focus, and reduced-motion behavior; the
  public extension is recorded in `DESIGN.md`.
- **PASS — independent finish closure:** review findings were resolved with a
  POST-only, disabled-without-JavaScript pilot form, direct-email fallback,
  white contact-section focus treatment, semantic service-charter column
  headings, and risk-sensitive headline copy. The complete browser suite passed
  again after those changes.

Current browser artifacts are `/tmp/alzette-landing-{desktop,tablet,mobile}.png`,
`/tmp/alzette-landing-{desktop,tablet,mobile}-fold.png`, and
`/tmp/alzette-landing-desktop-hero.png`. This verification does not change the
existing live-provider and Slice 3 production gates.

A canonical public URL and branded social-share card remain P2 publication work
until the production domain and asset URL are verified; the LAN-hosted PoC does
not invent absolute public metadata.

## Slice 1/2 independent verification — 2026-08-13

### Verdict

The Slice 0–2 offline software gate is **PASS**. This is not a customer or
production approval: approved live-provider evidence and Slice 3 production
controls remain separate gates. No real OpenRouter credential was accessed or
printed, and no public provider call was made.

The isolated run was `alzette-qa-slice12-20260813t171500z` (PostgreSQL
`29532`, gateway `29180`, control `29181`, static fallback `29182`, fake target
`29990`). The default `alzette-poc` stack and
`alzette-poc_alzette-postgres` volume were preserved. The control process was
restarted after the portal UI edits so it loaded the current assets.

### Checks and results

- **PASS — baseline:** `gofmt -l .` was empty; `go mod verify`, `go vet ./...`,
  `go test ./...`, and `go test -race ./...` passed with
  `OPENROUTER_API_KEY` and `OPENROUTER_API_KEY_FILE` explicitly unset.
- **PASS — migrations:** isolated databases were taken from transactional
  `0001/0002` through `0003/0004`, upgraded with `go run ./cmd/alzette migrate`
  through `0005/0006/0007`, then taken down and reapplied. Existing-schema
  upgrades and fresh/reapply checks passed. The complete PostgreSQL package
  passed under the race detector, including human/session/key lifecycle,
  service-plan isolation and capacity-drift guards, rollup/checkpoint
  reconciliation, probes, exports, and route-evidence isolation.
- **PASS — human HTTP contract:**
  `/tmp/alzette-slice12-20260813t171500z-http.sh` produced valid/invalid login
  `303/401`, `/api/portal/me` `200`, CSRF rejection `403`, viewer mutation
  denial `403`, multi-membership context switch `200`, and session invalidation
  `401` after password rotation. Cookies were `HttpOnly` and `SameSite=Lax`.
- **PASS — tenant isolation/redaction:** A and B dashboard responses were
  `200`; cross-tenant organisation, target, raw URL, secret reference, and
  sensitive key material were absent. No fixture credential or fake-provider
  secret appeared in the portal output.
- **PASS — key and auth unit coverage:**
  `TestPasswordAndOpaqueTokens`, the memory provisioning/rotate/revoke tests,
  the portal strict-key/CSRF/privacy/export tests, and the control auth,
  duplicate-header, scope, route-observation, and logical-reconciliation
  tests passed.
- **PASS — retry/accounting:** the local fake target timed out its first
  attempt and the isolated gateway returned `200` after retry. The ledger and
  repeated worker refresh reconciled one logical request to two provider
  attempts, one retry, and one partial current-hour rollup; no attempt was
  counted as a second customer request.
- **PASS — worker/probe behavior:** two bounded worker runs exited with the
  expected timeout (`124`) and `worker-health --maximum-age 2m` passed after
  each. Global-off/no-target-opt-in probe tests made no outbound request;
  target failure and missing-credential tests remained fail-closed.
- **PASS — exports:** CSRF-protected JSON and CSV downloads returned `200`
  with `application/json` and `text/csv`. Structural checks passed and neither
  export contained the fake secret, raw target URL, authorization material, or
  API-key field. Artifacts: `/tmp/alzette-qa-slice12-20260813t171500z/export.json`
  and `export.csv`.
- **PASS — browser/responsive:** the Playwright skill runner used visible
  Chromium under `xvfb-run` for the Go-connected and static-fallback portals at
  `1440`, `1024`, `390`, and `320`. Marker/state, tenant context, route data,
  navigation/ARIA, context dialog/Escape, copy, exports, screenshots, and
  horizontal-overflow checks passed. Console, page-error, and request-failure
  collections were empty. Screenshots inspected:
  `/tmp/alzette-qa-slice12-20260813t171500z/portal-connected-1440.png` and
  `portal-connected-390.png`.
- **PASS — review closure:** the current LAN stack was rebuilt from one image;
  migration exited `0`, PostgreSQL/gateway/control/worker became healthy, and
  the final browser run passed truthful zero/user identity, final-vs-partial
  export gating, Access API outage semantics, revoked-key actions, mobile
  drawer focus/inert/outside-dismiss behavior, overflow, and logout.

The overlap-control harness check was independently reduced to one locator
after the navigation sequence. Its exact JSON evidence was: viewport
`1440x1000`, one matching element, visible/enabled, rect
`{x:1203.40625,y:716.46875,width:120,height:28}`. The clean browser run did
not force-click a hidden control. Earlier browser failures were harness
expectation/timing issues: the token regex excluded the implementation’s `.`,
the session cookie is `alzette_session` (not the harness’s expected name), the
clear check raced the close event, and mobile navigation state is represented
by `body.nav-open`. These remain harness corrections, not product defects;
they are not overstated as full PostgreSQL portal-lifecycle evidence.

### Contract and gaps

Connected UI assertions require exactly **`External pilot / Shared pilot`**.
They must not require “External pilot via OpenRouter” or infer OpenRouter,
MeluXina, or dedicated capacity from `execution_class`. The inspected connected
screenshots show the corrected copy. Live OpenRouter evidence remains a
separate opt-in gate and is **BLOCKED / NOT RUN**.

No Slice 0–2 offline software blocker remains. An approved file-backed provider
credential and bounded live call are still required before claiming a real
OpenRouter-backed pilot. Rate/concurrency enforcement, TLS ingress/remote auth,
backup/restore, retention/runbooks, and an independent production security
review remain Slice 3 work.

### Evidence and cleanup

Primary artifacts are under
`/tmp/alzette-qa-slice12-20260813t171500z/`, including
`postgres-tests.log`, `browser-result.json`, `browser-clean-rerun.log`, the
two inspected screenshots, worker logs, retry status, and sanitized export
files. Temporary provider secrets and generated Alzette keys were outside the
repository; no secret value is reproduced here. Exact isolated-resource
cleanup is performed after this report is written; the default stack/volume is
not a cleanup target.

## Slice 0 final Gate A — 2026-08-13

Verdict: **PASS for the offline Slice 0 software proof.** Independent QA passed
the full credential-neutral baseline and isolated PostgreSQL suite, including
migration `0004`, down/reapply, prior-schema upgrades, ledger constraints,
route-binding concurrency, and bound-target deletion protection. The hardened
Compose smoke passed with four logical requests and three provider attempts:
tenant A's timeout/retry reconciled as two attempts for one success, tenant B
used only its explicit binding, both cross-route calls failed closed, and the
safe result contained no content, credential, or target URL. A process-log
canary scan passed before the result was emitted. Focused absent-route and
upstream `403`/`404` mapping/accounting tests also passed. Evidence is under
`/tmp/alzette-qa-slice0-final-20260813/` and
`/tmp/alzette-qa-slice0-closure-20260813/` on the verification host.

The default stack was rebuilt from the verified source, migration `0004` is
applied, and gateway/control/PostgreSQL are healthy. Gate B is not claimed: no
public provider call or previously exposed credential was used. One bounded
live OpenRouter smoke with a newly rotated file-backed key remains the only
unmet Slice 0 external-response criterion. TLS ingress, automated stranded-row
reconciliation, workers/rollups, rate/concurrency limits, and backup/restore
remain later-slice production gates.

## Post-hardening final Gate A — 2026-08-12

Verdict: **Gate A offline software PoC PASS.** The fresh isolated project `alzette-qa-20260812t171000z` was rebuilt from the current tree and the corrected Playwright harness passed **80/80** assertions. Evidence: `/tmp/alzette-qa-20260812T171000Z-postfix-corrected/` (API/accounting, retry/partial/unknown/outage, tenant isolation, duplicate Bearer/Basic rejection, redaction/containment, `Vary: Authorization`, connected and static fallback browser checks at 1440/1024/390/320, zero browser errors/failed requests, screenshots, and formula-safe CSV).

Credential-neutral baseline checks passed: gofmt, `go vet ./...`, `go mod verify`, `go test ./...`, and `go test -race ./...`. The post-fix PostgreSQL suite passed migration fresh/down/reapply, valid-0001 and 0002 upgrades, A→B→A historical binding, 0002 integrity invariants, concurrent retarget resolution, and route-before-target lock ordering (`/tmp/alzette-qa-20260812T163348Z-postfix-final/postgres-full.log`). Independent migration review confirmed current routes migrate to generation 2, legacy attempted history remains generation 1, and reported no P0/P1/P2 defects.

The preceding 22-failure harness attempt was a preserved QA-fixture failure: the inert provider secret was `0600 root:root` and unreadable by the nonroot runtime, so no fake-provider calls occurred. After changing only that temporary secret to runtime-readable `0644` and recreating the ledger, the corrected run passed. No live OpenRouter credential or external call was used.

Customer/production release remains **BLOCKED**. Gates B/C are **BLOCKED / NOT RUN**: live provider smoke is forbidden, and worker/rollup, rate/concurrency, TLS/remote-auth, backup/restore, production security, and other deployment evidence remain deferred or unbuilt. Prior QA history below is retained and superseded for Gate A by this dated section.

## Prior customer/production release verdict — superseded for Gate A

Release verdict: **FAIL / BLOCKED for a customer or production pilot release.** The exercised offline Go/API/portal slice passed its available checks, but the implementation does not yet provide evidence for every mandatory release-gate area in `POC_TEST_PLAN.md`. No real OpenRouter credential was used and no live OpenRouter smoke was attempted.

## Summary

Independent run: `20260812T150601Z`, worktree `/root/code/alzette`.

The disposable Compose project was `alzette-qa-20260812t150601z` with ports gateway `18580`, control `18581`, PostgreSQL `55483`, and a host-local fake provider on `18582`. Provisioning was performed through the operator CLI; no application tables were written directly for the end-to-end fixture.

The deterministic API/browser harness reached 71 assertions and all 71 assertions passed. Its process exited non-zero only because Chromium raised `Page.captureScreenshot` during the final static-mobile capture after all connected and formula-export images had already been produced. A bounded no-screenshot static fallback retry passed for desktop and mobile, and a separate one-page static-mobile screenshot succeeded. This is harness evidence, not a product failure.

Harness retries retained as non-product setup evidence: the first attempt stopped before assertions because the background fake-provider process was reaped and no JSONL log was created; the next PTY attempt initially failed closed because the disposable provider-secret file was unreadable by the non-root gateway; a later run stopped on an overly broad strict Playwright selector. Each was corrected in `/tmp` only. The final run used a persistent fake-provider PTY, fresh operator-provisioned tenants, and the exact application stack.

## Checks run and results

### Deterministic code and database checks

| Command | Result |
|---|---|
| `test -z "$(gofmt -l $(rg --files -g '*.go'))"` | PASS; zero unformatted Go files |
| `go vet ./...` | PASS; no output |
| `go test ./... -count=1` | PASS; all packages green |
| `go test -race ./... -count=1` | PASS; no race report |
| `ALZETTE_TEST_DATABASE_URL=<isolated Compose URL> go test ./internal/store/postgres -run '^TestPostgres' -count=1 -v` | PASS: migration/provisioning/isolation/accounting, migration down/reapply, and PostgreSQL-backed HTTP vertical slice |

The PostgreSQL tests verified one-way key storage, no content columns, dedicated-target ownership, shared binding, transactional rollback, immutable completed records/audit, key revocation/rotation, two-tenant isolation, retry attempt accounting, and the down/up migration cycle.

### Compose and HTTP/security checks

Commands run:

```sh
docker compose -p alzette-qa-20260812t150601z \
  -f compose.yaml -f /tmp/alzette-qa-compose-override-20260812T150601Z.yaml config --quiet
docker compose -p alzette-qa-20260812t150601z \
  -f compose.yaml -f /tmp/alzette-qa-compose-override-20260812T150601Z.yaml \
  build --pull=false migrate gateway control
docker compose -p alzette-qa-20260812t150601z \
  -f compose.yaml -f /tmp/alzette-qa-compose-override-20260812T150601Z.yaml up -d --wait
```

All three passed. PostgreSQL, gateway, and control became healthy. `GET /api/healthz`, `HEAD /api/healthz`, and `GET /readyz` returned `200`.

Operator-provisioned fake-provider evidence:

- 11 provider calls: 3 success, 2 retry attempts, 1 partial-usage success, 1 missing-usage success, and 4 outage attempts.
- Every fake call had valid server-side fake auth, the configured provider model, and an Alzette request correlation marker.
- Tenant A: 5 logical requests, 4 successful and 1 failed; known input tokens `28`, output tokens `7`, both explicitly partial.
- Tenant B: 1 logical request; B could not retrieve A’s request (`404`).
- Empty tenant: 0 requests and route `unknown`.
- Limited-scope dashboard: `403`.
- Final outage: gateway `503`; route became `degraded`.
- Provider secret, prompt canaries, target URL, and unsafe provider failure detail were absent from API responses and portal payloads.

Protected portal/static checks passed:

- Unauthenticated portal: Basic challenge with `401`.
- Authenticated `/` and `/dashboard.html`: `200`, connected marker injected exactly for Go-served content.
- Same-origin `/api/dashboard`: `alzette.client_dashboard.v1`, tenant-scoped partial finality, no export while partial.
- `no-store`, `Vary: Authorization`, CSP, `X-Content-Type-Options`, and frame/security headers present.
- Allowlisted CSS/SVG assets: `200`.
- `/index.html`, `/docs.html`, `/catalog.json`, `/go.mod`, `/cmd/alzette/main.go`, `/internal/server/http.go`, and `/migrations/0001_openrouter_poc.up.sql`: safe `404`.

### Browser checks

The required Playwright skill was read completely. Visible Chromium was run with `xvfb-run -a` and `HEADLESS=false` using `/root/.agents/skills/playwright-skill/run.js`.

Connected portal viewports all passed marker/contract, semantics, console/page/network, and overflow checks:

| Viewport | Scroll width / viewport | Console/page/request errors |
|---|---:|---:|
| 1440×900 | 1425 / 1440 | 0 / 0 / 0 |
| 1024×768 | 1009 / 1024 | 0 / 0 / 0 |
| 390×844 | 375 / 390 | 0 / 0 / 0 |
| 320×760 | 320 / 320 | 0 / 0 / 0 |

Also passed: section hash navigation, visible keyboard focus, semantic landmarks/headings/tables, no unnamed buttons, copy-request-ID toast, partial-state export disabled, complete-state export enabled, and formula-safe CSV. A formula-like project name was exported as a prefixed CSV value (`'=QA,Project`), preventing spreadsheet formula execution.

Static fallback bounded retry passed at 1440 and 390 widths: marker false, `data-api-state=fallback`, `Preview fallback`, zero `/api/dashboard` requests, no browser errors, and no horizontal overflow.

## Screenshots and temporary artifacts

Sanitized artifacts are under `/tmp/alzette-qa-20260812T150601Z/`:

- `dashboard-connected-desktop.png`
- `dashboard-connected-tablet.png`
- `dashboard-connected-mobile.png`
- `dashboard-connected-narrow.png`
- `dashboard-formula-export.png`
- `static-mobile.png`
- `formula-export.csv`
- `qa-evidence.json`
- `postgres-integration-verbose.log`
- `static-fallback-check.log` and `static-mobile-check.log`
- `integrated-browser-api-final2.log` (records the screenshot-only harness abort)
- Compose build/config/health logs and redacted fake-provider transcript

Visual inspection found the connected desktop/mobile views readable, the degraded route and partial token state visible, the formula export toast visible, and the static mobile view explicitly showing `Preview fallback`/`Unknown`. No clipped essential content or horizontal overflow was observed.

## Changed files

Implementation/product changes: **none**.

QA documentation changes authorized for this task: `QA_REPORT.md` and only the current verification-readiness/results section of `POC_TEST_PLAN.md`.

## Blockers

These items are **blocked/deferred, not passed**:

- Worker health probes, rollups, retention, and a reproducible stale-telemetry producer.
- Rate limiting, admission/concurrency controls, and production-scale concurrency evidence.
- TLS/remote-auth ingress and a safe external deployment boundary.
- Backup/restore automation, checksums, restore reconciliation, and clean restart/persistence runbook.
- Full automated QA service coverage for the complete 115-ID matrix.
- Production security/dependency review.
- Live OpenRouter smoke: forbidden because the exposed credential is compromised; no real-provider or customer-pilot claim is supported.

The current integrated portal has section navigation and an export-format select. The older fixture dashboard’s concept switcher, date/options menus, and observatory inspector are not part of this portal surface; they were not represented as passing checks.

## Recommendations

Implement the blocked worker/rate/TLS/backup surfaces and add clean-machine Compose QA gates before release. Add an operator-driven stale/outage fixture, full accessibility/zoom/reduced-motion coverage, restart/restore reconciliation, and a production security review. Keep the release label limited to an offline/fake-target PoC until an approved, bounded live smoke is possible with a fresh credential.

## Cleanup

The exact Compose project/volume/network, unique QA image, fake provider PTY, static server PTY, generated Alzette key files, fake provider secret, and temporary provisioning files were removed after evidence capture. Sanitized screenshots/logs remain under the artifact directory for review.
