# Alzette OpenRouter PoC — maximum-confidence QA test plan

## Endpoint acquisition addendum — 2026-08-14

The current offline release gate includes the authenticated Models, Endpoints,
and Billing control plane defined by `../prd/ENDPOINTS_PRD.md`. The implementation is
acceptable only when all of these invariants hold:

- catalogue visibility is organisation-scoped and only operator-published
  offers are selectable;
- clients cannot supply or change a target URL, provider credential, approved
  alias, authoritative price, currency, payment state, or runtime state;
- shared evaluation, paid shared, and dedicated private acquisition retain
  separate eligibility and activation rules;
- dedicated configurations become immutable after submission and require an
  immutable operator quote before payment can begin;
- commercial, payment, and runtime states remain separately visible and no
  payment event alone represents a ready endpoint;
- payment and quote entry require recent human password authentication,
  independently from one-time workload API keys;
- every mutation is tenant-scoped, CSRF-protected, and idempotent; replay
  returns the same resource and a changed request under the same key conflicts;
- key-issuance retry safety prevents a second active same-named credential
  after an interrupted one-time reveal;
- hosted checkout and billing-portal redirects accept only HTTPS Stripe hosts
  (or an explicitly supported same-origin test location);
- capacity requests are allowed only for an eligible ready/degraded dedicated
  endpoint, persist bounded sizing intent under immutable server-side records,
  and never mutate hardware or routing directly;
- empty or unknown price, capacity, route, and evidence fields remain unknown;
  the UI must not infer availability from a catalogue row;
- customer APIs omit target URLs, provider secrets, provider attempts, prompts,
  outputs, and operator-only evidence references;
- catalogue, configurator, endpoint, progress, and billing views are keyboard
  usable and have no document-level horizontal overflow at `390px`.

The dated results are recorded at the top of `QA_REPORT.md`. The offline gate
passed on a fresh PostgreSQL race run and a visible-Chromium acceptance flow.
Live Stripe/provider evidence, public signup, MeluXina provisioning, measured
dedicated capacity, TLS/backup/rate controls, and independent production
security review remain separate unpassed gates.

## Standalone public surface addendum — 2026-08-14

The current Compose acceptance scope now includes a database-independent
`public` process in addition to gateway, control, worker, migration, and
PostgreSQL. Its required checks are:

- exact public allow-list for landing, docs, stylesheet, and river mark;
- source, portal-asset, and traversal paths return `404`;
- `/healthz` and `/readyz` return `200` without PostgreSQL;
- `/client` redirects only to a validated absolute HTTP(S) portal URL;
- control/portal does not serve the public landing/docs root;
- the public container receives no database or provider credential variables;
- CSP, framing, referrer, permissions, MIME-sniffing, method, and cache headers;
- landing and docs at 1440, 1024, and 390 widths with no overflow, console/page
  error, broken primary navigation, missing keyboard skip path, or unsupported
  unconditional technical or contractual claim.

The executed evidence for these checks is recorded at the top of
[`QA_REPORT.md`](QA_REPORT.md). Live-provider, TLS ingress, and Slice 3
production gates remain separate.

## Slice 1/2 independent QA readiness/results — 2026-08-13

This is the current Slice 1/2 evidence snapshot; the historical Slice 0
sections below are retained. No real `OPENROUTER_API_KEY` was read, printed, or
used, and no public provider was called. The isolated project was
`alzette-qa-slice12-20260813t171500z` with PostgreSQL `29532`, gateway `29180`,
control `29181`, static fallback `29182`, and a local fake target on `29990`.
The default `alzette-poc` stack and `alzette-poc_alzette-postgres` volume were
outside the test scope.

**Readiness verdict:** the Slice 0–2 offline software gate is **PASS**. Current
Compose build/migrate/health, full race tests, PostgreSQL portal/worker/schema
integration, and final live-browser closure all passed. Live-provider evidence
and Slice 3 production controls remain separate, intentionally unpassed gates.

The connected portal contract is **`External pilot / Shared pilot`**. It must
not claim “via OpenRouter”, MeluXina, dedicated capacity, or any other provider
fact inferred from `execution_class`. OpenRouter remains operator configuration;
the opt-in live-provider gate is still pending and was not run.

### Evidence matrix

| Area | Result | Concrete evidence |
|---|---|---|
| Formatting/module/static analysis | PASS | `gofmt -l .` empty; `env -u OPENROUTER_API_KEY -u OPENROUTER_API_KEY_FILE go mod verify`; `go vet ./...` — all passed. |
| Unit/default/race | PASS | `env -u OPENROUTER_API_KEY -u OPENROUTER_API_KEY_FILE go test ./...`; same environment with `go test -race ./...` — all packages passed. |
| Migration lifecycle | PASS | Isolated PostgreSQL used transactional `0001/0002` baselines, upgraded through `0003/0004`, then current migrations through `0005/0006/0007`; down/reapply and existing-schema upgrade checks passed. |
| Human login/session/security | PASS | `/tmp/alzette-slice12-20260813t171500z-http.sh`: valid/invalid login `303/401`, `/api/portal/me` `200`, session revocation after password rotation `401`, `HttpOnly`/`SameSite=Lax`, CSRF rejection `403`, role denial `403`, and multi-membership context switch `200`. |
| Two-tenant portal isolation | PASS | Same HTTP run: A/B dashboard responses were `200`, cross-tenant data, target URL, secret ref, and org identifiers were absent from the other tenant’s response; viewer could not mutate service accounts. |
| Service-account lifecycle | PASS | Unit, HTTP, PostgreSQL, and browser evidence covers provisioning, named expiring keys, overlap, one-time reveal, explicit revoke, password/session separation, and session revocation. Plaintext is never listed or persisted. |
| Fake-provider request/accounting | PASS | Isolated fake target with first-attempt timeout; a fresh isolated key produced HTTP `200` after retry. Ledger/rollup query reconciled `1` logical request, `2` provider attempts, `1` retry, finality `partial`. No provider secret or raw target URL was emitted. |
| Usage truth states | PASS | Unit, PostgreSQL, export, and connected-browser evidence covers exact zero, partial, failed-only, stale/unavailable separation, direct-ledger truth, rollup freshness, and nullable token finality. |
| Rollup/checkpoint/repeatability | PASS | Two bounded `worker` runs exited at the expected timeout (`124`); `worker-health --maximum-age 2m` passed after each. The retry ledger reconciled on repeated refresh without duplicate rollup rows. |
| Probe gating | PASS | Global-off/per-target opt-in, multi-target failure continuation, missing credentials, response-model validation, PostgreSQL observation isolation, and stale evidence all fail closed. Default Compose probes remain off. |
| JSON/CSV export | PASS | Authenticated CSRF-protected exports returned JSON `200` (`application/json`) and CSV `200` (`text/csv`); structural and secret/URL/header redaction checks passed. Artifacts: `/tmp/alzette-qa-slice12-20260813t171500z/export.json`, `export.csv`. |
| Browser portal | PASS / harness corrections recorded | Playwright visible-Chromium run under `xvfb-run` covered connected/static fallback at `1440`, `1024`, `390`, and `320`; marker/state, tenant data, navigation/ARIA, context/Escape, copy, exports, screenshots, and overflow passed. Console, page, and request-failure arrays were empty. The actual overlap button after navigation was one visible/enabled element: viewport `1440x1000`, rect `x=1203.40625,y=716.46875,w=120,h=28`; no force-click was used. |
| Compose | PASS | `docker compose config --quiet` and a clean current-image `docker compose up --build -d` passed; migrate exited `0`, and PostgreSQL, gateway, control, public, and worker were healthy with the three HTTP services bound to `0.0.0.0`. |
| Live provider / customer release | BLOCKED / NOT RUN | No live call and no ambient credential access. This remains a distinct external-provider gate. |

Browser artifacts and inspection: `/tmp/alzette-qa-slice12-20260813t171500z/browser-result.json`, `browser-clean-rerun.log`, `portal-connected-1440.png`, and `portal-connected-390.png`. The inspected images show the connected contract copy `External pilot / Shared pilot`; they do not establish OpenRouter, MeluXina, or dedicated capacity.

### Remaining external and production gates

No Slice 0–2 offline software blocker remains. Keep the live OpenRouter
response test opt-in and separately gated; it was intentionally not run in
this credential-neutral audit. This is not a customer/production approval:
rate/concurrency enforcement, TLS ingress/remote auth, backup/restore,
retention/runbooks, and production security review are Slice 3 work.

## Slice 0 final Gate A — 2026-08-13

**Gate A: PASS.** Independent QA reran formatting, source-trackability,
module, vet, default, race, PostgreSQL migration/rollback/upgrade/integrity,
Compose, and deterministic acceptance gates on the current tree. The committed
`scripts/slice0-smoke.sh` proved two explicitly bound tenants, one timeout and
retry as one logical request/two attempts, cross-route denial, shared-capacity
truthfulness, and metadata redaction. It also scanned the smoke output and
gateway/fake-target process logs for prompt/output, credential, bearer-key, and
raw-target-URL canaries before printing the safe result. Exact focused coverage
now includes an authorised absent route, upstream `403`/`404`, disabled and
undeletable bound targets, query/header/body overrides, and usage finality.

Gate B remains **BLOCKED / NOT RUN** until a previously exposed OpenRouter key
is revoked and a newly rotated key is supplied by file for the bounded opt-in
live test. The running LAN stack is an HTTP PoC on an explicitly requested
trusted-network bind; TLS ingress remains later-slice production work.

## Post-hardening final Gate A readiness — 2026-08-12

**Gate A: PASS — offline software PoC only.** A newly built, credential-neutral isolated Compose run (`alzette-qa-20260812t171000z`) used the operator CLI, an inert fake provider, and a fresh ledger. The corrected harness passed **80/80** at 1440/1024/390/320 widths, covering API retry/partial/unknown/outage accounting, two-tenant isolation, auth duplication, redaction/containment, `Vary: Authorization`, connected/static fallback behavior, keyboard/anchors/copy, no overflow/errors, and formula-safe CSV. Artifacts: `/tmp/alzette-qa-20260812T171000Z-postfix-corrected/`.

`gofmt`, `go vet ./...`, `go mod verify`, default tests, and race tests passed. The post-fix PostgreSQL evidence passed fresh migration, down/reapply, valid-0001/0002 upgrades, A→B→A generation attribution, all 0002 direct-SQL invariants, concurrent retarget resolution, and route-before-target lock order (`/tmp/alzette-qa-20260812T163348Z-postfix-final/postgres-full.log`). Independent review found no P0/P1/P2 defects. The prior unreadable-`0600` temporary-secret harness failure is retained as fixture evidence, not a product failure.

Customer/production release is **BLOCKED**; **Gates B/C are BLOCKED / NOT RUN**. No live OpenRouter call or real credential was used. Worker/rollup, rate/concurrency, TLS/remote auth, backup/restore, production security, and other deferred deployment/live gates remain unpassed. This section supersedes the older readiness snapshot below while retaining its history.

Status: executable acceptance plan for the OpenRouter forwarding PoC

Authority: `../product/POC_BOUNDARY.md`; `../prd/PORTAL_PRD.md` is supporting product context

Owner: independent QA
Date: 2026-08-12

This plan tests the exact PoC boundary: a stable Alzette OpenAI-compatible ingress, server-controlled OpenRouter routing, PostgreSQL-backed tenant and usage records, a tenant-safe dashboard, one-machine Docker Compose deployment, deterministic fake-provider integration, and an opt-in real OpenRouter smoke. It does not turn fixture data or UI copy into evidence of live capacity, MeluXina hosting, dedicated OpenRouter capacity, residency, an SLA, or product-market fit.

## Decision vocabulary

- **Mandatory** — must pass for the PoC release gate. A missing test harness or missing implementation is `BLOCKED`, not a pass.
- **Deferred** — explicitly outside this PoC’s release gate. It may not be advertised as supported until its own gate passes.
- **Opt-in** — intentionally excluded from default CI because it uses a real provider, secret, cost, or external state. It is required as release evidence when declaring a real external pilot, but never as a default offline test.
- **Baseline** — current-repository health evidence. Baseline success does not waive a mandatory PoC test.

Every case has a stable ID. Test names, CI output, screenshots, logs, database snapshots, and defect reports must retain the ID. If an implementation chooses a different package or route name, preserve the ID and update the command mapping; do not silently drop the case.

## Current verification readiness/results

Independent QA run `20260812T150601Z` exercised the integrated Go-backed slice in worktree `/root/code/alzette`. No real OpenRouter credential or live provider call was used. The disposable Compose project was `alzette-qa-20260812t150601z`, with gateway/control/PostgreSQL ports `18580/18581/55483`; all generated Alzette key files were mode `0600` and were kept outside the repository.

### Passed evidence

- `test -z "$(gofmt -l $(rg --files -g '*.go'))"` — PASS; zero files.
- `go vet ./...` — PASS.
- `go test ./... -count=1` — PASS for all packages; this is the unit/package baseline.
- `go test -race ./... -count=1` — PASS for all packages; no race report.
- `ALZETTE_TEST_DATABASE_URL=<isolated Compose URL> go test ./internal/store/postgres -run '^TestPostgres' -count=1 -v` — PASS: `TestPostgresMigrationProvisioningIsolationAndAccounting`, `TestPostgresMigrationDownIsSafeInIsolatedSchema`, and `TestPostgresBackedHTTPVerticalSlice`. This directly exercised migration, down/reapply, constraints, provisioning rollback, two-tenant isolation, retry/accounting, and PostgreSQL-backed HTTP behavior.
- `docker compose -p alzette-qa-20260812t150601z -f compose.yaml -f /tmp/alzette-qa-compose-override-20260812T150601Z.yaml config --quiet` — PASS.
- The unique image build and `up -d --wait` — PASS; PostgreSQL, gateway, and control reported healthy. `/api/healthz`, `HEAD /api/healthz`, and `/readyz` returned `200`.
- Operator CLI provisioning created synthetic tenants A/B, an empty tenant, a limited-scope tenant, and a formula-export tenant without direct application-table writes. The host-local deterministic fake received 11 calls: success `3`, retry `2` (first failure plus success), partial `1`, unknown `1`, outage `4`; every call had valid fake-provider auth, server-selected provider model, and an Alzette request correlation marker.
- API/accounting: tenant A initially reconciled to 5 logical requests (`4` successful, `1` failed), input token value `28` and output token value `7` with partial finality; tenant B had exactly 1 request; B’s lookup of A’s request returned `404`; empty tenant showed `0` and route `unknown`; limited dashboard access returned `403`; final outage returned `503` and the shared route became `degraded`.
- Basic-auth portal: unauthenticated access challenged with `401`; authenticated `/` and `/dashboard.html` served the Go-injected marker `window.__ALZETTE_API__ = true`; same-origin `/api/dashboard` returned schema `alzette.client_dashboard.v1`, partial source/finality, and no export for the partial snapshot. Security headers, `no-store`, and `Vary: Authorization` were present.
- Static containment: allowlisted CSS/SVG assets returned `200`; `/index.html`, `/docs.html`, `/catalog.json`, `/go.mod`, `/cmd/alzette/main.go`, `/internal/server/http.go`, and `/migrations/0001_openrouter_poc.up.sql` returned safe `404` responses.
- Browser: Playwright was run under `xvfb-run` with `HEADLESS=false` at `1440×900`, `1024×768`, `390×844`, and `320×760`. The 71 API/browser assertions were all PASS: connected marker/contract, route/partial/outage truth, landmarks/heading/tables/button names, hash navigation, keyboard focus, copy toast, formula-safe CSV, zero console/page/request errors, and no horizontal overflow (`1425≤1440`, `1009≤1024`, `375≤390`, `320=320`).
- Static fallback bounded retry (screenshots disabled) passed at desktop and mobile: marker false, `data-api-state=fallback`, `Preview fallback`, no `/api/dashboard` request, no browser errors, and no overflow. A separate one-page static-mobile screenshot also succeeded.

Harness note: the first attempt was a setup failure because the background fake-provider process was reaped before its JSONL log existed; subsequent PTY/secret-permission/selector corrections were confined to `/tmp`. The final runner’s only non-zero status was Chromium’s screenshot-capture protocol error after the connected/formula images were saved; the bounded screenshot-disabled fallback retry passed and the capture limitation is not treated as a product failure.

### Readiness decision

The exercised offline vertical slice is **PASS**, but the PoC release gate is **FAIL / BLOCKED**, not “all planned tests passed.” The 115-case plan remains only partially implemented/evidenced. The following mandatory or claim-enabling areas are `BLOCKED`/`DEFERRED` and must not be counted as passes: worker health probes and rollups/retention (including stale telemetry production), rate limiting and concurrency controls, TLS/remote-auth ingress, backup/restore automation and reconciliation, clean restart/restore operations, full automated QA service coverage, and production security/dependency review. Live OpenRouter smoke is `BLOCKED BY POLICY` because the exposed credential is compromised; no external-pilot claim is made.

The current portal has section links and an export-format select, not the earlier fixture dashboard’s concept switcher, date/options menus, or observatory inspector. Those interactions were not silently treated as passes; they are outside this integrated portal surface and remain deferred/not applicable until an approved product requirement exposes them.

## Test environment and evidence protocol

### Required tools

- Go version compatible with `go.mod`; `go test`, `go test -race`, `go vet`, and `gofmt`.
- Docker Engine and Docker Compose v2.
- PostgreSQL client tools (`psql`, `pg_dump`, `pg_restore`) or the equivalent Compose QA container.
- `curl`, `jq`, and a JSON-capable log scanner.
- Node.js and Playwright for browser checks. Use `xvfb-run -a` where no display is available.
- A secret-injection mechanism that does not echo `OPENROUTER_API_KEY` or database credentials in shell traces.

### Run variables

Use a unique UTC run ID and keep all evidence outside the repository by default:

```sh
export RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-${GIT_COMMIT:-local}"
export ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/alzette-poc-qa-${RUN_ID}}"
mkdir -p "$ARTIFACT_DIR"
export ALZETTE_BASE_URL="${ALZETTE_BASE_URL:-http://127.0.0.1:8080}"
export FAKE_OPENROUTER_URL="${FAKE_OPENROUTER_URL:-http://127.0.0.1:18080}"
```

Do not use `set -x` while secrets are present. Do not place prompt, output, bearer token, database password, or provider response bodies in a committed artifact. Redact any accidental output before attaching evidence.

### Canonical command set

These commands are the minimum offline run. They must be run from the repository root and their stdout, stderr, exit code, commit, Go version, Compose version, and schema version recorded under `$ARTIFACT_DIR/run/`.

```sh
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
test -z "$(gofmt -l $(rg --files -g '*.go'))"
docker compose config --quiet
```

After the implementation provides its QA-tagged tests and Compose profile:

```sh
go test -tags=integration ./... -count=1 -run 'TestQA_'
docker compose --profile qa up -d --build --wait
docker compose --profile qa run --rm qa
docker compose --profile qa down
```

The `qa` service must start a fresh fake-provider scenario and disposable database, seed through the operator surface, run the IDs, emit machine-readable results, and fail if a requested ID was not found. A command that matches zero tests is a failure of the QA harness, not a successful run.

For browser evidence, use the project’s version-pinned Playwright command once added; the canonical shape is:

```sh
xvfb-run -a npx playwright test qa/browser --reporter=line
```

Screenshots, traces, accessibility reports, HAR/request summaries, and console/page-error summaries belong under `$ARTIFACT_DIR/browser/`.

### Deterministic fixtures

The QA seed must create all data through the operator provisioning API/command, never by mutating production tables directly:

| Fixture | Required properties |
|---|---|
| Tenant A | `org-a`, project/environment A, service account/key A, alias `client-model`, dedicated test target `target-a` owned exclusively by A |
| Tenant B | `org-b`, project/environment B, service account/key B, alias `client-model`, shared test target `target-shared` explicitly allow-listed for B and any other permitted tenant |
| External target label | All OpenRouter-backed fixtures use execution class `external_pilot`; absent evidenced provider capacity, capacity mode is `shared`. |
| Fake model mapping | Client alias is distinct from provider slug. The fake target records the received provider slug and rejects an unexpected slug. |
| Request canaries | Normal tests use non-sensitive fixed text. Redaction tests use unique prompt/output/secret canaries that must never occur in persisted metadata or QA logs. |
| Clock | Inject a fixed UTC clock for rollup, freshness, retry, and retention tests; also run one real-time health/freshness check. |

The fixture must support a target response matrix: success with complete usage; success with no usage; success with partial usage; success with cached/reasoning fields; `400`, `401`, `403`, `404`, `408`, `429`, and `503`; `Retry-After`; slow response; malformed JSON; connection reset before output; connection reset after output begins; duplicate/late provider response; and an upstream correlation ID (`X-Generation-Id` or response `id`).

## Stable test matrix

### 1. Baseline and unit contracts — `U-*`

**Classification: Mandatory, except explicitly marked deferred.**  Unit tests must be deterministic and must not call the public OpenRouter service.

| ID | Test | Canonical command | Expected evidence / pass condition |
|---|---|---|---|
| U-001 | Model-alias resolution | `go test ./... -run '^TestQA_U001$' -count=1` | An authorised client alias resolves only to the server-controlled provider slug; unknown, disabled, and tenant-incompatible aliases fail without target access. |
| U-002 | API-key hashing and lifecycle | `go test ./... -run '^TestQA_U002$' -count=1` | Plaintext is never stored or returned; valid key authenticates, wrong key fails, revoked/expired key fails immediately, and verification does not log the key. |
| U-003 | Tenant/project/environment authorization | `go test ./... -run '^TestQA_U003$' -count=1` | Authenticated scope is derived from the credential/session; client-supplied tenant/project IDs cannot widen scope. |
| U-004 | Request validation and limits | `go test ./... -run '^TestQA_U004$' -count=1` | Malformed JSON, unknown/unsafe fields, empty invalid messages, oversized bodies, invalid limits, and unsupported options produce stable safe errors before provider access. |
| U-005 | Provider error mapping | `go test ./... -run '^TestQA_U005$' -count=1` | Each upstream status maps to a documented Alzette error class/status; safe correlation is retained, provider body/secret is not leaked. |
| U-006 | Usage parsing and finality | `go test ./... -run '^TestQA_U006$' -count=1` | Prompt, completion, cached, reasoning, cost, provider ID, and presence/absence are parsed without overflow or coercion; fields are `complete`, `partial`, or `unknown` as defined by the contract. Unknown is never silently converted to zero. |
| U-007 | Retry decision policy | `go test ./... -run '^TestQA_U007$' -count=1` | Only bounded, eligible pre-output failures retry; `Retry-After` is honored within a configured cap; non-retryable errors and post-output failures do not retry. |
| U-008 | Logical-request/attempt accounting | `go test ./... -run '^TestQA_U008$' -count=1` | One client call creates exactly one logical request; each provider call creates one attempt; terminal state and attempt count are deterministic across success, retry, and failure. |
| U-009 | Rollup arithmetic | `go test ./... -run '^TestQA_U009$' -count=1` | Hour/day summaries reconcile to logical requests, not provider attempts; late events, UTC boundaries, partial usage, and zero requests are handled without double counting. |
| U-010 | Redaction functions | `go test ./... -run '^TestQA_U010$' -count=1` | Prompt/output, bearer/API keys, database credentials, raw target URLs with secrets, and sensitive upstream bodies are removed from every metadata/log/audit representation. |
| U-011 | Target-binding invariants | `go test ./... -run '^TestQA_U011$' -count=1` | Dedicated ownership, shared allow-list, execution class, capacity mode, secret reference, model slug, and no-cross-offer-fallback checks are enforced at the domain boundary. |
| U-012 | Idempotent provisioning and identifiers | `go test ./... -run '^TestQA_U012$' -count=1` | Repeating the same operator provisioning request does not create duplicate tenant/route/key bindings; request IDs, attempt IDs, and audit IDs are unique and stable where the contract requires idempotency. |
| U-013 | Streaming safety | `go test ./... -run '^TestQA_U013$' -count=1` | **Deferred** unless streaming is implemented. If enabled, cancellation, partial output, final usage, and no transparent retry after bytes are sent must pass before advertising streaming. |

### 2. HTTP and OpenAI-compatible contract — `HTTP-*`

The exact response field/header names must be frozen in the API contract. The tests below require a documented Alzette request ID, not a particular undocumented spelling; the preferred evidence is a response `X-Request-ID` plus the same ID in structured error/audit records.

| ID | Test | Command / procedure | Expected evidence / pass condition |
|---|---|---|---|
| HTTP-001 | Method and content-type contract | `go test -tags=integration ./... -run '^TestQA_HTTP001$' -count=1` | `POST /v1/chat/completions` accepts only the documented method/content type; unsupported methods return documented `405`/`Allow`; JSON success and error content types are correct. |
| HTTP-002 | Valid non-streaming chat completion | `go test -tags=integration ./... -run '^TestQA_HTTP002$' -count=1` | A client-shaped request receives a compatible response, an Alzette request ID, provider model/usage normalization, and no client control over the upstream URL or credential. |
| HTTP-003 | Alias rewrite and allow-list | `go test -tags=integration ./... -run '^TestQA_HTTP003$' -count=1` | Fake target sees the configured provider slug, never the raw client alias as an uncontrolled target selector; unsupported model/options are rejected before forwarding. |
| HTTP-004 | Auth and authorization statuses | `go test -tags=integration ./... -run '^TestQA_HTTP004$' -count=1` | Missing, malformed, wrong-tenant, revoked, expired, and insufficient-scope credentials receive stable safe statuses; valid key reaches only its route. |
| HTTP-005 | Malformed/unknown/unsafe request fields | `go test -tags=integration ./... -run '^TestQA_HTTP005$' -count=1` | Invalid JSON, unknown fields, raw `base_url`, provider `model` override, target credentials, and unsafe options are rejected or stripped according to the frozen contract; fake target evidence proves no unsafe field crossed the boundary. |
| HTTP-006 | Body, token, timeout, and concurrency limits | `go test -tags=integration ./... -run '^TestQA_HTTP006$' -count=1` | Size, request deadline, output-size, per-tenant concurrency, and rate/allowance limits are enforced; rejection creates safe metadata and does not create a false successful usage event. |
| HTTP-007 | Cancellation and disconnect | `go test -tags=integration ./... -run '^TestQA_HTTP007$' -count=1` | Client cancellation reaches the adapter/attempt, bounded cleanup occurs, no goroutine/connection leak is observed, and final request/attempt state is truthful. |
| HTTP-008 | Request ID and provider correlation | `go test -tags=integration ./... -run '^TestQA_HTTP008$' -count=1` | Every accepted/rejected request has an Alzette correlation ID; provider response ID/generation ID is stored only as safe metadata and can correlate without content. |
| HTTP-009 | Error and header normalization | `go test -tags=integration ./... -run '^TestQA_HTTP009$' -count=1` | `429`/`503` and other upstream failures map to the documented client contract; unsafe upstream headers/body are not copied; retry hints are bounded and truthful. |
| HTTP-010 | Health/readiness semantics | `go test -tags=integration ./... -run '^TestQA_HTTP010$' -count=1` | Process health, dependency readiness, and target health are distinct; a target is not `ready` merely because a process is alive, and a failed real health/inference probe is visible. |
| HTTP-011 | Control/portal/usage/export contracts | `go test -tags=integration ./... -run '^TestQA_HTTP011$' -count=1` | Provisioning, key lifecycle, tenant-safe dashboard/query, request metadata, and CSV/JSON export endpoints enforce methods, authz, scope, content type, pagination/range, source, freshness, and finality. |
| HTTP-012 | Current fixture compatibility | `go test ./... -run 'TestDashboardEndpoint|TestStaticHandler|TestGoServerMarksDashboard' -count=1` | Existing fixture/static checks remain green while the real gateway/portal is introduced; fixture values are not accepted as live PoC evidence. **Baseline compatibility, mandatory during integration.** |

### 3. Two-tenant isolation — `ISO-*`

Run every case with synthetic tenants A and B, distinct keys, overlapping project names where useful, and both same-alias and different-alias routes. Capture response status, request ID, query/export payload, fake-target observations, and database row ownership. A pass requires zero cross-tenant records, not merely a `403` on the first request.

| ID | Test | Command / procedure | Expected evidence / pass condition |
|---|---|---|---|
| ISO-001 | Independent authorised calls | `go test -tags=integration ./... -run '^TestQA_ISO001$' -count=1` | A and B each complete a valid call; each logical request, attempt, usage row, and dashboard total is owned by the correct organisation. |
| ISO-002 | Wrong key and tenant/project/environment IDs | `go test -tags=integration ./... -run '^TestQA_ISO002$' -count=1` | B’s key cannot call A’s route or retrieve A’s project/environment; client-provided tenant IDs are ignored/rejected; no target call occurs on denial. |
| ISO-003 | Alias and route enumeration | `go test -tags=integration ./... -run '^TestQA_ISO003$' -count=1` | Unknown aliases and A-only aliases do not reveal existence, provider slug, target URL, capacity, or secret details to B. |
| ISO-004 | Usage filters and dashboard scope | `go test -tags=integration ./... -run '^TestQA_ISO004$' -count=1` | Organisation/project/environment/model/key-prefix filters cannot return or aggregate another tenant’s rows; totals equal only the selected scope. |
| ISO-005 | Request detail and exports | `go test -tags=integration ./... -run '^TestQA_ISO005$' -count=1` | B cannot fetch A’s request by ID, guessable ID, pagination cursor, date range, or export; A’s export contains no B rows and no content/secrets. |
| ISO-006 | Cache/session/cursor isolation | `go test -tags=integration ./... -run '^TestQA_ISO006$' -count=1` | Shared caches, browser sessions, pagination tokens, background jobs, and stale authorization context never reuse A’s data for B. |
| ISO-007 | Operator support boundary | `go test -tags=integration ./... -run '^TestQA_ISO007$' -count=1` | Any operator/support access is explicit, least-privilege, audited, tenant-scoped, time-bounded where supported, and still content-free by default. |

### 4. Dedicated/shared binding invariants — `BIND-*`

OpenRouter is an external provider. Unless independently evidenced, every OpenRouter target must be labelled `external_pilot` and `shared`; an Alzette-exclusive route to a shared external API is not dedicated capacity. Dedicated cases below validate the binding policy with a synthetic target and must not become customer-facing OpenRouter claims.

| ID | Test | Command / procedure | Expected evidence / pass condition |
|---|---|---|---|
| BIND-001 | Immutable target contract | `go test -tags=integration ./... -run '^TestQA_BIND001$' -count=1` | Target records execution class, capacity mode, base URL/private address, provider model, secret reference, timeout, retry policy, and health; clients can see only approved safe fields. |
| BIND-002 | Dedicated target exclusivity | `go test -tags=integration ./... -run '^TestQA_BIND002$' -count=1` | A target declared `dedicated` has exactly one owning tenant; provisioning a second owner is rejected transactionally; no traffic or dashboard state can bind B to it. |
| BIND-003 | Shared target allow-list | `go test -tags=integration ./... -run '^TestQA_BIND003$' -count=1` | A shared target accepts only explicitly allow-listed tenant/project/environment bindings; an unlisted tenant cannot infer or call it. |
| BIND-004 | No silent capacity-mode fallback | `go test -tags=integration ./... -run '^TestQA_BIND004$' -count=1` | Dedicated outage/queue does not silently route to shared; shared exhaustion does not silently route to dedicated; any policy-approved change requires an explicit binding/contract state and audit event. |
| BIND-005 | No client target override | `go test -tags=integration ./... -run '^TestQA_BIND005$' -count=1` | Body/query/header attempts to provide a raw URL, provider slug, secret, execution class, capacity mode, or target ID are rejected or ignored with safe evidence; fake target receives only server configuration. |
| BIND-006 | Truthful external labels | `go test -tags=integration ./... -run '^TestQA_BIND006$' -count=1` | Customer status and export say `External pilot` and `Shared pilot` for the external fixture; the browser does not infer a provider name, and no `MeluXina`, `on-premise`, `private`, or `dedicated` claim appears without evidence. |
| BIND-007 | Real readiness check | `go test -tags=integration ./... -run '^TestQA_BIND007$' -count=1` | Target health becomes ready only after a bounded real compatible check; auth failure, wrong model, timeout, and malformed response produce `degraded`/`unavailable` with freshness. |

### 5. Retry and logical-request/provider-attempt accounting — `RETRY-*`

The customer ledger counts logical client calls. Provider attempts are operator reliability/COGS evidence. A retry is allowed only before output begins, is bounded, and must not duplicate customer usage.

| ID | Test | Command / procedure | Expected evidence / pass condition |
|---|---|---|---|
| RETRY-001 | One successful call | `go test -tags=integration ./... -run '^TestQA_RETRY001$' -count=1` | One client response, one logical request, one successful provider attempt, complete usage, and one customer usage event. |
| RETRY-002 | Timeout then success | `go test -tags=integration ./... -run '^TestQA_RETRY002$' -count=1` | Fake target times out before output then succeeds; client sees one success; exactly one logical request and two attempts; attempt 1 is timed out, attempt 2 successful; customer usage counts once. |
| RETRY-003 | `429`/`503` and `Retry-After` | `go test -tags=integration ./... -run '^TestQA_RETRY003$' -count=1` | Retryable status respects a bounded `Retry-After`/backoff, records each attempt, and stops at the configured maximum; observed delay and attempt count are evidence. |
| RETRY-004 | Terminal provider failure | `go test -tags=integration ./... -run '^TestQA_RETRY004$' -count=1` | Non-retryable `4xx`, exhausted retries, malformed response, and provider auth failure return one safe terminal error; no phantom success/usage row is created. |
| RETRY-005 | No retry after output begins | `go test -tags=integration ./... -run '^TestQA_RETRY005$' -count=1` | Once response bytes have been sent to the client, the gateway never transparently replays to another attempt/target. If streaming is not implemented, keep the test at the adapter/writer boundary and mark streaming behavior deferred. |
| RETRY-006 | Attempt/request state transitions | `go test -tags=integration ./... -run '^TestQA_RETRY006$' -count=1` | State transitions are monotonic and queryable: pending → running → success/failure/cancelled; no attempt remains running after timeout/cancel; logical request finality is explicit. |
| RETRY-007 | Retry and latency accounting | `go test -tags=integration ./... -run '^TestQA_RETRY007$' -count=1` | End-to-end latency is measured once for the logical request; per-attempt latency is separate; dashboards/exports do not sum attempts as customer requests. |
| RETRY-008 | Concurrent duplicate/idempotent submission | `go test -tags=integration ./... -run '^TestQA_RETRY008$' -count=1` | **Mandatory if an idempotency key is advertised; otherwise deferred.** Repeated client submission with the same documented key has the documented single-execution behavior and no double billing. |

### 6. Complete, partial, and missing usage — `USAGE-*`

Usage fields are facts with provenance and finality, not guessed zeros. Every result must be checked in the raw logical request, attempts, rollups, dashboard response, and CSV/JSON export.

| ID | Test | Command / procedure | Expected evidence / pass condition |
|---|---|---|---|
| USAGE-001 | Complete OpenRouter usage | `go test -tags=integration ./... -run '^TestQA_USAGE001$' -count=1` | Prompt/input, completion/output, cached, reasoning, provider cost, model, provider ID, and total fields map correctly where supplied; source/provider and `complete` finality are preserved. |
| USAGE-002 | Missing usage object | `go test -tags=integration ./... -run '^TestQA_USAGE002$' -count=1` | Successful response without usage succeeds at the API layer but reports unknown fields/finality `missing` or equivalent; no field is rendered as zero unless zero is authoritative. |
| USAGE-003 | Partial usage object | `go test -tags=integration ./... -run '^TestQA_USAGE003$' -count=1` | Only supplied components are populated; absent components remain unknown; finality is `partial`; dashboard/export visibly distinguishes partial from zero and complete. |
| USAGE-004 | Invalid/inconsistent usage | `go test -tags=integration ./... -run '^TestQA_USAGE004$' -count=1` | Negative, overflowing, non-numeric, contradictory, or impossible totals are rejected/quarantined and never create misleading customer totals; provider correlation remains safe. |
| USAGE-005 | Failed/cancelled request usage | `go test -tags=integration ./... -run '^TestQA_USAGE005$' -count=1` | Failure/cancel rows distinguish no usage, partial usage, and unknown usage; no failed request is counted as successful; attempt metadata remains available to operators without content. |
| USAGE-006 | Retry aggregation | `go test -tags=integration ./... -run '^TestQA_USAGE006$' -count=1` | Two attempts with usage produce one logical customer event according to the defined authoritative policy; provider-attempt usage is separate and never double-counts the dashboard. |
| USAGE-007 | Dashboard/export reconciliation | `go test -tags=integration ./... -run '^TestQA_USAGE007$' -count=1` | Summary cards, time series, project/model breakdowns, recent request rows, and CSV/JSON export reconcile to logical rows for complete, partial, missing, failed, and retried fixtures; every view has source, `as of`, and finality. |

### 7. Deterministic fake OpenRouter integration — `FAKE-*`

The fake provider is the default integration target. It must assert received method, path, auth, provider slug, allow-listed body, timeout, retry order, and request correlation. It must never be replaced by a public network call in default CI.

| ID | Scenario | Command / procedure | Expected evidence / pass condition |
|---|---|---|---|
| FAKE-001 | Normal successful Chat Completions response | `go test -tags=integration ./... -run '^TestQA_FAKE001$' -count=1` | Fake sees server-side bearer token and configured slug, returns deterministic OpenAI-compatible JSON, and records no client secret leakage. |
| FAKE-002 | Upstream `400/401/403/404` | `go test -tags=integration ./... -run '^TestQA_FAKE002$' -count=1` | Stable Alzette error classes/statuses, no retry where prohibited, safe correlation, and no raw provider body/credential disclosure. |
| FAKE-003 | Upstream `429/503` with retry hint | `go test -tags=integration ./... -run '^TestQA_FAKE003$' -count=1` | Retry policy and bounded wait match `RETRY-003`; all attempts are visible in operator evidence and one logical request remains in the customer ledger. |
| FAKE-004 | Slow response and timeout | `go test -tags=integration ./... -run '^TestQA_FAKE004$' -count=1` | Deadline is enforced at gateway and target client; no leaked connection/goroutine; attempt finality is recorded. |
| FAKE-005 | Malformed JSON/schema | `go test -tags=integration ./... -run '^TestQA_FAKE005$' -count=1` | Invalid provider payload is a safe provider/protocol error; no guessed usage or success is recorded. |
| FAKE-006 | Disconnect before output | `go test -tags=integration ./... -run '^TestQA_FAKE006$' -count=1` | Eligible bounded retry behavior works; request/attempt evidence is complete and no duplicate customer event occurs. |
| FAKE-007 | Disconnect/flush after output begins | `go test -tags=integration ./... -run '^TestQA_FAKE007$' -count=1` | No transparent replay after output; client sees the documented incomplete/error state; streaming remains deferred unless all streaming cases pass. |
| FAKE-008 | Usage variants and provider correlation | `go test -tags=integration ./... -run '^TestQA_FAKE008$' -count=1` | Complete/partial/missing usage and `X-Generation-Id`/response ID fixtures reconcile to the usage matrix and safe audit metadata. |
| FAKE-009 | Target URL and secret boundary | `go test -tags=integration ./... -run '^TestQA_FAKE009$' -count=1` | Fake target receives only the operator-configured URL and credential; no client-provided URL, key, raw model slug, prompt/output persistence, or unsafe headers cross the boundary. |

### 8. PostgreSQL constraints, migrations, rollups, and backup/restore — `DB-*`

PostgreSQL is authoritative for tenant, route, request, usage, and audit data. Prometheus/metrics are not acceptable as the customer usage source. All database tests use a disposable database and a pinned schema version.

| ID | Test | Command / procedure | Expected evidence / pass condition |
|---|---|---|---|
| DB-001 | Clean migration | `docker compose --profile qa run --rm qa db-test DB-001` | Empty database migrates to the expected schema version from scratch; migration output and schema hash are recorded; required indexes/constraints exist. |
| DB-002 | Migration idempotence and compatibility | `docker compose --profile qa run --rm qa db-test DB-002` | Reapplying the current migration is safe or explicitly rejected without corruption; upgrade from the previous supported version preserves rows and invariants. |
| DB-003 | Safe down migration policy | `docker compose --profile qa run --rm qa db-test DB-003` | Reversible migrations are tested up/down in a disposable DB; destructive production down migrations are prohibited or require an explicit backup/approval guard. |
| DB-004 | Foreign keys and tenant constraints | `docker compose --profile qa run --rm qa db-test DB-004` | Orphan projects/routes/attempts/rollups are rejected; tenant ownership is non-null and consistent; a request cannot point to another tenant’s project/route. |
| DB-005 | Uniqueness/check constraints | `docker compose --profile qa run --rm qa db-test DB-005` | Key hashes, aliases within their scope, target ownership, request IDs, attempt IDs, and audit IDs meet documented unique constraints; execution class/capacity mode/finality/status enums reject invalid values. |
| DB-006 | Idempotent provisioning transaction | `docker compose --profile qa run --rm qa db-test DB-006` | Repeated provisioning yields one intended resource set; injected failure rolls back all related tenant/project/target/route/key rows, with no half-bound route or plaintext secret. |
| DB-007 | Request/attempt persistence | `docker compose --profile qa run --rm qa db-test DB-007` | Logical request and provider attempts persist independently with FK linkage, timestamps, state/finality, safe correlation, and no body/content columns unless explicitly approved (default is none). |
| DB-008 | Hourly rollup reconciliation | `docker compose --profile qa run --rm qa db-test DB-008` | Rollups are UTC-hour scoped, tenant/project/model scoped, idempotent, and reconcile exactly to logical request rows across retries, failures, late usage, partial usage, and zero-volume hours. |
| DB-009 | Rollup rerun/concurrency | `docker compose --profile qa run --rm qa db-test DB-009` | Two workers rerunning the same period do not double count; unique/upsert strategy and transaction isolation are evidenced. |
| DB-010 | Retention and deletion boundary | `docker compose --profile qa run --rm qa db-test DB-010` | Prompt/output are not retained by default; metadata retention/deletion follows the configured policy; deletion does not corrupt usage/audit totals or violate required audit retention. |
| DB-011 | Backup and restore | `docker compose --profile qa run --rm qa db-test DB-011` | `pg_dump`/`pg_restore` or the supported equivalent restores a clean database; row counts, request/attempt links, rollups, tenant totals, key revocation state, and audit records reconcile; secrets/content are absent from backup evidence. |
| DB-012 | Restore after service restart | `docker compose --profile qa run --rm qa db-test DB-012` | A service/database restart and restore preserve the authoritative ledger and route policy; the gateway fails closed while dependencies are unavailable and recovers without duplicate accounting. |

### 9. Race and concurrency — `RACE-*`

These are correctness tests, not a claim of production scale. Use a deterministic fake target and a disposable PostgreSQL instance. Set a finite test deadline; a hang is a failure with goroutine/DB diagnostics attached.

| ID | Test | Command / procedure | Expected evidence / pass condition |
|---|---|---|---|
| RACE-001 | Go race detector | `go test -race ./... -count=1` | No race reports, data races, or test-only skips in gateway/auth/adapter/ledger/rollup code. |
| RACE-002 | Concurrent calls from two tenants | `CONCURRENCY=50 go test -tags=integration ./... -run '^TestQA_RACE002$' -count=1 -timeout=2m` | A and B calls complete within deadline; each has exactly the expected logical rows/attempts and no cross-tenant data; fake target observes correct bindings. |
| RACE-003 | Concurrent request state/usage updates | `CONCURRENCY=50 go test -tags=integration ./... -run '^TestQA_RACE003$' -count=1 -timeout=2m` | Concurrent attempt completion, timeout, cancellation, and usage finalization do not lose updates, produce duplicate usage, or leave impossible states. |
| RACE-004 | Concurrent provisioning/revocation | `CONCURRENCY=20 go test -tags=integration ./... -run '^TestQA_RACE004$' -count=1 -timeout=2m` | Key issuance/revocation and route provisioning are serialized by constraints/transactions; no revoked key remains usable after the defined consistency point. |
| RACE-005 | Concurrent rollup workers | `WORKERS=4 go test -tags=integration ./... -run '^TestQA_RACE005$' -count=1 -timeout=2m` | Repeated workers yield one deterministic rollup result and no deadlock/lock leak. |

### 10. Docker Compose and one-machine operations — `CMP-*`

Compose is part of the PoC boundary. It must be possible to rebuild, restore, and operate the slice without manually editing a database. Do not run these tests against a developer’s persistent production-like volume; use a disposable project/volume and record the Compose project name.

| ID | Test | Command / procedure | Expected evidence / pass condition |
|---|---|---|---|
| CMP-001 | Compose configuration | `docker compose config --quiet` | All services, profiles, health checks, volumes, networks, env references, and secret references resolve; no secret value is embedded in YAML/image/config output. |
| CMP-002 | Clean build/start/health | `docker compose --profile qa up -d --build --wait` | Gateway, control, worker, PostgreSQL, and required ingress become healthy in a clean environment; health is distinct from target readiness. |
| CMP-003 | Default offline integration | `docker compose --profile qa run --rm qa` | Default QA uses fake OpenRouter only and has no public-provider dependency; all mandatory fake/integration IDs run and emit evidence. |
| CMP-004 | Network and exposure boundary | `docker compose --profile qa run --rm qa compose-test CMP-004` | PostgreSQL is not unnecessarily public; only documented ingress/portal ports are reachable; service-to-service auth/network policy is enforced. |
| CMP-005 | Restart and persistence | `docker compose --profile qa restart gateway control worker` | After restart, routes, revoked keys, request ledger, attempts, rollups, and audit state remain correct; no duplicate worker effects occur. |
| CMP-006 | Rebuild and clean-machine restore | `docker compose --profile qa down` followed by clean volume/restore procedure | A fresh machine/project can rebuild, migrate, restore a documented backup, and pass health plus reconciliation checks. |
| CMP-007 | TLS/ingress boundary | `docker compose --profile qa run --rm qa compose-test CMP-007` | If TLS/ingress is in scope for the claimed deployment, certificates, forwarded headers, request IDs, body limits, and HTTP-to-HTTPS policy are tested. If not yet implemented, the release is blocked from an external customer claim, not silently passed. |
| CMP-008 | Shutdown/backup runbook | `docker compose --profile qa run --rm qa compose-test CMP-008` | Operator can stop safely, capture a consistent backup, restore, and verify request/usage totals using documented commands; evidence is attached to the release record. |

### 11. Browser, responsive, and accessibility — `UI-*`

Use a seeded tenant session or test login, never a real customer account. Required viewports: desktop `1440×900`, tablet `1024×768`, and mobile `390×844`; also test 320 CSS px reflow/zoom where supported. Capture screenshots only after the dashboard reaches a stable data state.

| ID | Test | Canonical command / procedure | Expected evidence / pass condition |
|---|---|---|---|
| UI-001 | First-call/status hierarchy | `xvfb-run -a npx playwright test qa/browser --grep UI-001` | Dashboard answers whether the route can be called first: operational/degraded/unavailable/unknown, endpoint/alias, `External pilot`, `Shared pilot`, last success, health, and freshness. No fixture-only or provider-inferred claim is presented as live. |
| UI-002 | Usage and attribution | `xvfb-run -a npx playwright test qa/browser --grep UI-002` | Logical requests, successes/errors/blocked, known tokens, latency, project/model/executed-model/key-prefix breakdowns, and contract/limit context match API/export values. |
| UI-003 | Safe recent requests and export | `xvfb-run -a npx playwright test qa/browser --grep UI-003` | Recent rows show only safe metadata and copyable Alzette request ID; CSV/JSON export has scope, period, units, source, freshness, and finality and contains no prompt/output/secret. |
| UI-004 | Zero/partial/stale/outage states | `xvfb-run -a npx playwright test qa/browser --grep UI-004` | Zero usage, missing/partial usage, stale telemetry, target outage, and unavailable API are visually and textually distinct; unknown is not shown as zero; stale data has an `as of` timestamp. |
| UI-005 | Filters and tenant scope | `xvfb-run -a npx playwright test qa/browser --grep UI-005` | Organisation/project/environment/model/time filters preserve server authorization, update all dependent panels consistently, and cannot reveal another tenant through URL/query manipulation. |
| UI-006 | Responsive layouts | `xvfb-run -a npx playwright test qa/browser --grep UI-006` | Desktop/tablet/mobile screenshots show no horizontal overflow or clipped essential content; tables/charts have usable alternatives; no interaction is hidden behind a fixed overlay. |
| UI-007 | Keyboard and focus | `xvfb-run -a npx playwright test qa/browser --grep UI-007` | All controls are reachable in logical order; menus/dialogs close with Escape; focus is visible and restored; no keyboard trap; tab/arrow behavior is documented and works. |
| UI-008 | Accessibility semantics | `xvfb-run -a npx playwright test qa/browser --grep UI-008` | Automated accessibility scan has no critical/serious violations; landmarks/headings/labels/table headers/status announcements are semantic; charts expose text/table summaries; status is not color-only; contrast meets the chosen WCAG AA target. |
| UI-009 | Reduced motion/zoom/reflow | `xvfb-run -a npx playwright test qa/browser --grep UI-009` | `prefers-reduced-motion`, 200% zoom, and 320 CSS px reflow remain usable without lost actions or horizontal scroll. |
| UI-010 | Browser network/console health | `xvfb-run -a npx playwright test qa/browser --grep UI-010` | No page errors, uncaught console errors, broken local assets, failed required API calls, or unhandled fallback state; request trace confirms correct tenant/API scope. |

### 12. Secrets, content, and security redaction — `SEC-*`

Use unique canaries such as `PROMPT_CANARY_${RUN_ID}`, `OUTPUT_CANARY_${RUN_ID}`, and a fake provider token. Search all service logs, structured events, traces, audit rows, database tables, exports, error responses, browser-visible text, and QA artifacts. The fake provider may see the prompt because it is the test target; every other default persistence/diagnostic surface must not.

| ID | Test | Command / procedure | Expected evidence / pass condition |
|---|---|---|---|
| SEC-001 | Key storage/reveal/revoke | `go test -tags=integration ./... -run '^TestQA_SEC001$' -count=1` | Key is shown once if that is the contract, then only prefix/metadata; database/log/export/dashboard never contain plaintext. Revocation is immediate at the documented consistency point. |
| SEC-002 | Prompt/output non-persistence | `go test -tags=integration ./... -run '^TestQA_SEC002$' -count=1` | Canary prompt/output is absent from request/attempt/audit/rollup tables and standard logs/traces; body retention is off by default. |
| SEC-003 | Log/trace/audit redaction | `go test -tags=integration ./... -run '^TestQA_SEC003$' -count=1` | Authorization, cookies, provider tokens, database credentials, raw target URLs, request bodies, response bodies, and unsafe upstream error fields are redacted; audit remains useful metadata only. |
| SEC-004 | Error/HTTP/header redaction | `go test -tags=integration ./... -run '^TestQA_SEC004$' -count=1` | Client errors contain stable class, message, and correlation ID only; no upstream secret/body or internal target address leaks through status, headers, JSON, or stack traces. |
| SEC-005 | Support/export/dashboard boundary | `go test -tags=integration ./... -run '^TestQA_SEC005$' -count=1` | Customer/operator views and exports expose only approved metadata and scope; no prompt/output, token, secret, or other-tenant data appears. |
| SEC-006 | Static/dependency/security checks | `go test -tags=security ./... -run '^TestQA_SEC006$' -count=1` plus the approved dependency/vulnerability scanner | No known release-blocking vulnerability, unsafe dependency, accidental debug endpoint, source exposure, or insecure default is accepted without an explicit risk decision. |

### 13. Static-file containment — `STATIC-*`

Run against the Go server and against any ingress/proxy path that is part of the deployment. Static hosting may serve the preview, but only the Go route may advertise API capability. The test must cover encoded and normalized variants, not just the obvious paths.

| ID | Test | Command / procedure | Expected evidence / pass condition |
|---|---|---|---|
| STATIC-001 | Allowed public assets | `go test ./... -run '^TestQA_STATIC001$' -count=1` plus `curl` smoke | `/`, documented HTML/CSS/JS/image/font assets, docs, and dashboard load with correct content type; dashboard static mode retains fallback values and does not attempt an API call without capability. |
| STATIC-002 | Source/config containment | `go test ./... -run '^TestQA_STATIC002$' -count=1` | `/go.mod`, `/go.sum`, `/cmd/...`, test files, `.git/*`, environment/config files, SQL/migrations, and arbitrary repository files return `404`/safe denial; no directory listing. |
| STATIC-003 | Traversal and encoding | `go test ./... -run '^TestQA_STATIC003$' -count=1` | Plain, URL-encoded, double-encoded, slash/backslash, dot-segment, query, and path-normalization variants cannot escape the public static root. |
| STATIC-004 | Symlink and alternate path containment | `go test ./... -run '^TestQA_STATIC004$' -count=1` | A disposable symlink/alternate file outside the public root cannot be read; `HEAD`/`OPTIONS` behavior is documented and does not bypass containment. |
| STATIC-005 | Security headers and method policy | `go test ./... -run '^TestQA_STATIC005$' -count=1` | `X-Content-Type-Options: nosniff` and other required headers are present; API methods and static methods match the contract; errors do not disclose filesystem paths. |
| STATIC-006 | Capability marker integrity | `go test ./... -run '^TestQA_STATIC006$' -count=1` | Raw static dashboard advertises API capability false; Go-served dashboard injects true exactly once; marker cannot be enabled by a client query/header and is not cached across static/Go modes incorrectly. |

### 14. Opt-in real OpenRouter smoke — `LIVE-*`

**Classification: Opt-in.** Never run by default, on pull requests, or with a real customer prompt. It requires an approved low-cost model, budget cap, provider key, and explicit operator consent. The implemented smoke skips unless `OPENROUTER_LIVE_TEST=1` and fails closed when the file-resolved secret or model is missing.

```sh
OPENROUTER_LIVE_TEST=1 \
  OPENROUTER_API_KEY_FILE="/absolute/path/to/chmod-0600-newly-rotated-qa-key" \
  OPENROUTER_MODEL="<approved-low-cost-model>" \
  go test -run '^TestLiveOpenRouterSmoke$' -v ./internal/gateway
```

That command is the current executable single-call smoke. The broader `LIVE-*`
matrix below is additional manual release evidence; budget/cost evidence,
dashboard reconciliation, cleanup, and provider-drift capture are not implied
by the Go smoke alone.

| ID | Test | Expected evidence / pass condition |
|---|---|---|
| LIVE-001 | Explicit opt-in and budget guard | Without the opt-in flag, the test performs zero network calls. With it, model, maximum calls, timeout, and cost ceiling are printed without the secret; exceeding any guard aborts safely. |
| LIVE-002 | Real non-streaming compatible call | A synthetic non-sensitive request reaches OpenRouter through Alzette, returns a valid response and Alzette request ID, and records the provider model/ID and available usage. |
| LIVE-003 | Live accounting and labels | The request/attempt/usage row and dashboard/export reconcile; customer surfaces remain labelled `External pilot` and `Shared pilot` unless independent dedicated evidence exists, while operator-only smoke evidence identifies the provider target. |
| LIVE-004 | Secret/content cleanup | Provider key is not in logs, DB, traces, export, browser, or artifacts; synthetic prompt/output is not retained by default; temporary key/route is revoked or deleted and evidence is recorded. |
| LIVE-005 | Provider drift evidence | Response schema, usage semantics, error/correlation fields, and observed model are captured as redacted contract evidence. A provider schema change fails the smoke rather than silently changing accounting. |

## Release gate

### Gate G0 — Contract and harness readiness

**Mandatory before implementation QA begins.**

- The stable client route (`POST /v1/chat/completions`), auth scheme, alias semantics, request ID, error schema, usage/finality schema, status labels, export schema, and target-binding fields are documented and versioned.
- The fake OpenRouter server can express every `FAKE-*` scenario deterministically.
- Operator provisioning can create the two-tenant fixture without direct database edits.
- A disposable PostgreSQL database and Compose QA profile exist.
- Each ID above has a discoverable test or is explicitly recorded `BLOCKED` with owner and due milestone; zero-test matches are failures.

### Gate G1 — Offline correctness

**Mandatory and default CI.** All `U-*`, `HTTP-*`, `ISO-*`, `BIND-*`, `RETRY-*`, `USAGE-*`, `FAKE-*`, `DB-*`, `RACE-*`, `CMP-*`, `SEC-*`, and `STATIC-*` mandatory cases pass in a clean run.

Required command outcomes:

```sh
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
test -z "$(gofmt -l $(rg --files -g '*.go'))"
docker compose config --quiet
go test -tags=integration ./... -count=1 -run 'TestQA_'
```

No critical/high security defect, tenant-isolation defect, secret/content leak, incorrect logical-request count, silent dedicated/shared fallback, or data-loss/restore failure may remain open.

### Gate G2 — Database and deployment recovery

**Mandatory for any pilot environment.** `DB-001` through `DB-012` and `CMP-001` through `CMP-008` pass on a clean machine/project. Evidence includes schema version/hash, migration output, Compose health, restart result, backup checksum, restore result, and reconciliation report. A dashboard that reads fixture values while the authoritative store is unavailable is not a pass.

### Gate G3 — Browser and privacy evidence

**Mandatory for customer review.** `UI-001` through `UI-010`, `SEC-001` through `SEC-006`, and `STATIC-001` through `STATIC-006` pass at the required viewports and states. Accessibility and redaction findings are release blockers when they expose content, secrets, tenant data, or prevent a customer from understanding external/shared execution.

### Gate G4 — Real external pilot evidence

**Opt-in, required before claiming a real OpenRouter pilot, not required for default CI.** `LIVE-001` through `LIVE-005` pass with an approved low-cost model, bounded budget, redacted artifacts, and key cleanup. If live smoke is unavailable, the release may be called a fake-target/internal PoC only; it may not be called a real OpenRouter-backed customer pilot.

### Final decision rules

- **PASS:** G0–G3 pass, no mandatory case is blocked, evidence is reproducible from a clean checkout/Compose environment, and any live claim is backed by G4.
- **CONDITIONAL:** only explicitly deferred features or opt-in live evidence are absent; the release statement names the limitation and does not make the corresponding claim. Conditional status is not permitted for isolation, accounting, redaction, backup/restore, or static containment failures.
- **FAIL:** any mandatory case fails or is untestable, any critical/high defect remains, any tenant/content/secret boundary is violated, or any external/shared/dedicated/MeluXina claim is unsupported.

The final QA report must attach: commit and environment, complete ID/status list, commands and exit codes, fake scenario transcript, API contract samples with canaries removed, database reconciliation, migration/restore evidence, Compose health/restart evidence, browser screenshots/traces/accessibility output, redaction scan summary, static containment matrix, and—only when opted in—live smoke cost/model/request evidence.

## Explicit deferrals

These are not release blockers for the narrow non-streaming OpenRouter PoC, but must remain visibly unsupported:

- `DEFER-001`: streaming, cancellation after partial output, final stream usage, and any cross-target replay after bytes sent; promote only after `U-013` and `FAKE-007` plus browser stream checks pass.
- `DEFER-002`: MeluXina allocation, model deployment, Slurm/OpenStack automation, private serving, dedicated-compute evidence, and Luxembourg execution claims.
- `DEFER-003`: silent/policy-driven cross-target failover, multi-host HA, Kubernetes, Redis/Kafka/ClickHouse, autoscaling, and multi-region routing.
- `DEFER-004`: full invoicing, payments, infrastructure COGS reconciliation, tax, and commercial contract automation.
- `DEFER-005`: SSO/SCIM, broad RBAC/enterprise identity, self-service model deployment, training/fine-tuning/evaluation, marketplace, and arbitrary model/provider selection.
- `DEFER-006`: advanced batch/async APIs, embeddings, tool/structured-output breadth beyond the frozen non-streaming subset, and performance/SLO claims beyond the measured Compose pilot envelope.

Deferred does not mean “implemented but untested.” If any deferred behavior is exposed in UI/API, it must be disabled or clearly labelled unsupported.
