# Alzette Systems

This repository contains the Alzette public site, client portal, an OpenAI-compatible inference-platform proof of concept through delivery Slice 2, and the first endpoint-acquisition control-plane increment. The public marketing landing and exact implementation documentation run as a standalone process with no database or provider credentials: the landing presents the intended private Luxembourg service, while the docs and authenticated portal report the current PoC route. A human signs in to the separate portal with a username and password; applications use separate, hashed, scoped API keys. Each key resolves a server-owned organisation/project/environment/model route, and PostgreSQL records customer-visible logical requests separately from operator-only provider attempts.

The PoC supports bounded non-streaming responses and a tested text/function-tool SSE subset for coding agents. It does not claim broad OpenAI API compatibility, MeluXina hosting, dedicated OpenRouter capacity, production availability, residency, or an SLA.

Customer account onboarding is specified separately in
[`ACCOUNT_ONBOARDING_PRD.md`](docs/prd/ACCOUNT_ONBOARDING_PRD.md). The intended flow is
hybrid self-service B2B: a verified business-email user enters a hard-capped
shared evaluation organisation, while business approval and a versioned quote
gate dedicated service; the one company owner can invite colleagues. Public
signup and recovery are not implemented. Owner-created invitations are
implemented with manual one-time-link delivery; transactional email delivery
is not.

Invited-employee inference access is specified in
[`WORKFORCE_AGENT_ACCESS_PRD.md`](docs/prd/WORKFORCE_AGENT_ACCESS_PRD.md). The target
workflow uses self-hosted Casdoor for human authentication, a short-lived
membership-bound Alzette token for inference, and eventually a loopback
compatibility proxy for agents that only accept a base URL and API-key field.
The local Compose stack now includes digest-pinned Casdoor, exact-email manual
invitations, OAuth Authorization Code with PKCE, employee context discovery,
10-minute `alz_u_` credentials, strict gateway dispatch, revocation, and
per-employee request attribution and the memory-only compatibility proxy. A
protected refresh-token client, transactional mail, remote TLS, and production
recovery/offboarding evidence remain unimplemented. Service-account keys
remain the unattended-workload credential.

## What the delivery slices mean

A slice is an end-to-end risk/evidence increment, not a product edition or a page count. Each one leaves a runnable path and closes a specific uncertainty:

- **Slice 0:** can Alzette route an isolated request safely and meter one customer call separately from internal attempts?
- **Slice 1:** can an operator provision the service and can a real human sign in, manage a workload identity/key, and make the first call without confusing the password and API key?
- **Slice 2:** can the customer trust the company-consumption, attribution, deployment evidence, zero/partial states, and exports?
- **Slice 3:** can the single-machine pilot be operated safely under real failure and security conditions?
- **Slice 4:** can the same customer contract move to a private MeluXina model target without reintegration?

P0 is the launch requirement set; the slices are the order used to build and prove it. Completing Slice 2 does not claim the Slice 3 production controls or the Slice 4 MeluXina deployment.

## Backend boundary

The customer controls only the supported chat request and the configured model alias. The server controls all of the following:

- organisation, project, environment, and service-account scope derived from the hashed bearer key;
- alias-to-target and alias-to-provider-model routing;
- compatible target base URL, execution class, capacity mode, timeout, and retry count;
- provider credential lookup through a server-side reference, preferring
  `<REFERENCE>_FILE` and falling back to `<REFERENCE>` only when no file is
  configured; the runtime resolver accepts only the comma-separated
  `ALZETTE_PROVIDER_SECRET_REFS` allow-list (Compose defaults to
  `OPENROUTER_API_KEY,DEEPSEEK_API_KEY`), so
  a database value cannot select an unrelated process environment variable;
- request IDs, retry policy, error normalization, and the metadata-only usage ledger.

No customer API accepts a tenant ID, project ID, raw target URL, provider credential, or arbitrary provider model slug. Prompt and completion bodies are held only long enough to proxy the request and response; they are not database fields, logs, audit metadata, or dashboard records.

Internet targets must use HTTPS at both provisioning and request time. Plain HTTP is accepted only when `ALZETTE_ALLOW_INSECURE_TARGETS=true` is explicitly set for an isolated local/private compatibility test.

## Run with Docker Compose

The default Compose stack starts PostgreSQL, applies migrations once, bootstraps a digest-pinned loopback Casdoor instance, and runs separate gateway, control/portal, public-site, and usage-worker processes from one image. The public process serves only an exact allow-list from `/app/public`; it receives no database URL or provider credential and redirects `/client` to the configured portal login. The stack never needs a real provider key merely to build, start, pass health checks, render an honest zero-usage portal, or run deterministic tests. Gateway and optional target probes resolve file-backed credential references such as `OPENROUTER_API_KEY_FILE=/run/secrets/openrouter_api_key` and `DEEPSEEK_API_KEY_FILE=/run/secrets/deepseek_api_key`; Compose mounts those paths from their corresponding host-side `*_SECRET_FILE` settings and never interpolates a provider key into its model. `/dev/null` is the inert default, and probes are globally disabled by default.

```bash
cp .env.example .env
docker compose up --build -d
docker compose ps
```

- Gateway: <http://localhost:8080>
- Client portal: <http://localhost:8081/login>
- Public landing page: <http://localhost:8082/>
- Public implementation docs: <http://localhost:8082/docs>
- Local Casdoor: <http://casdoor.localhost:19084>
- PostgreSQL: bound to `127.0.0.1:55432` for local integration tests

For an explicitly trusted-LAN demo, set the published origins in `.env` rather
than editing Compose:

```dotenv
ALZETTE_HTTP_BIND_ADDRESS=0.0.0.0
ALZETTE_GATEWAY_PORT=19080
ALZETTE_CONTROL_PORT=19081
ALZETTE_PUBLIC_PORT=19082
ALZETTE_PUBLIC_GATEWAY_URL=http://LAN_HOST:19080
ALZETTE_PUBLIC_PORTAL_URL=http://LAN_HOST:19081/login
```

That makes the landing page available at `http://LAN_HOST:19082/`, public docs
at `/docs`, and the human portal at `http://LAN_HOST:19081/login`. The public
page’s `/client` link uses `ALZETTE_PUBLIC_PORTAL_URL`; it does not guess a port
from the browser origin.

The local database password has a development fallback. Set a unique `POSTGRES_PASSWORD` before using a shared host. Published HTTP ports bind to `127.0.0.1` by default and can be changed with `ALZETTE_HTTP_BIND_ADDRESS`; PostgreSQL remains loopback-only. This PoC has no TLS listener. The committed local configuration therefore uses non-secure portal cookies only on loopback; an explicit trusted-LAN override may be used for a demo, but any customer, remote, or shared-host deployment requires a reviewed TLS terminator and secure cookies.

The portal deliberately separates identities:

- a human portal password creates a bounded server-side session and is never an inference credential;
- a service-account API key is revealed once, belongs in an application secret store, and is never reused as a portal password;
- an invited person authenticates with Casdoor and exchanges that identity proof
  for a digest-only, alias-bounded `alz_u_` credential that expires within ten
  minutes; the gateway never accepts the Casdoor token itself;
- inference consumption records exactly one actor: either a service account or
  the authenticated human membership and agent grant/token lineage.

The local bootstrap creates `employee@example.test` (`employee` /
`employee-demo-password`) only as an integration fixture. It grants no Alzette
membership by itself: the owner must first create an invitation for that exact
email in **Access → People**, choose the initial groups, and deliver the shown
one-time link manually. These loopback credentials and the default Casdoor
client secret must be replaced before any shared or remote deployment.

A newly provisioned client therefore starts with **0 logical requests**. Signing in, browsing, exporting an empty period, or managing a key does not create inference consumption. Statistics appear only after an application calls the Alzette gateway; tests use isolated tenants and never seed fake customer activity into the live demo scope.

## Model catalogue and endpoint acquisition

The authenticated portal now exposes `/app/models`, `/app/endpoints`, and
`/app/billing`. An authorised customer can inspect only offers eligible for the
server-derived organisation/project/environment scope, save and resume an
endpoint configuration, submit it, follow separate commercial/payment/runtime
states, accept an immutable dedicated quote after password confirmation, and
request a capacity revision for a ready dedicated endpoint. The customer never
submits a host, provider model, provider credential, Stripe object, amount, or
return URL.

Shared evaluation submission atomically binds only the operator-published
shared target and hard allowance. A paid shared offer stays non-callable until
a verified billing event activates it. A dedicated request stays non-callable
through quote, payment, allocation, deployment, and validation; only the
operator's evidenced `ready` transition binds its owned target. Stripe is
disabled by default, and the UI states that hosted payment is unavailable
rather than simulating checkout.

After provisioning the route shown below, seed a customer-safe evaluation
catalogue entry from the same immutable model/target registry:

```bash
docker compose run --rm control catalogue seed \
  --model-alias alzette-chat \
  --target-name openrouter-pilot \
  --model-slug alzette-chat \
  --model-name "Alzette Chat" \
  --model-family "Compatible chat" \
  --description "Operator-reviewed shared evaluation route." \
  --release-version poc \
  --source operator_catalogue \
  --evidence-ref operator-catalogue-poc-v1 \
  --evaluation-request-limit 100
```

The command is idempotent. Paid and dedicated offers are optional flags and
must be supplied only with real server-owned Stripe price mapping, capacity
profile, and evidence values; `alzette catalogue seed --help` lists them. A
catalogue row is not runtime evidence by itself.

For a dedicated submission, the operator uses the request ID shown in the
portal to issue a contractual quote, then advances only the allowed fulfilment
states:

```bash
docker compose run --rm control endpoint quote \
  --request-id "$DEPLOYMENT_REQUEST_ID" \
  --recurring-unit-amount-minor "$CONTRACTUAL_UNIT_AMOUNT_MINOR" \
  --currency EUR \
  --collection-mode not_required \
  --source operator_quote \
  --evidence-ref quote-example-v1

docker compose run --rm control endpoint transition \
  --request-id "$DEPLOYMENT_REQUEST_ID" --state approved
```

Subsequent transitions are `allocating`, `deploying`, `validating`, then
`ready`; the final transition additionally requires an organisation-owned
target name and validation evidence. Payment-required quotes cannot advance to
approval until server-authoritative billing state is paid.

The committed Slice 0 operator smoke uses a deterministic compatible target,
two explicitly bound tenants, and PostgreSQL without any external credential:

```bash
./scripts/slice0-smoke.sh
```

It uses a unique Compose project and ephemeral loopback ports, verifies one
timeout/retry as one logical request and two attempts, proves cross-route
denial and metadata redaction, and scans its output plus gateway/fake-target
logs for content, credential, bearer-key, and raw-target-URL canaries before it
prints the safe result. It then removes only its isolated containers and
volume. It reads `.env.example`, not the working `.env`, and cannot address an
external host. It does not touch the default `alzette-poc` volume.

Before an explicitly approved provider call, revoke any previously exposed credential and create a new file readable by the operator and the container's non-root runtime user. On a single-user development host this is typically mode `0640` with a dedicated runtime group; plain `0600 root:root` is intentionally unreadable inside the non-root image. Point the provider-specific host setting, such as `DEEPSEEK_API_KEY_SECRET_FILE`, at that file. If a `<REFERENCE>_FILE` variable is configured but the file is missing, unreadable, empty, too large, or header-unsafe, lookup fails closed and does not fall back to the environment value. Provider base URL and model remain immutable operator-target fields in PostgreSQL; they are not taken from customer input or inferred from the credential.

Provision a tenant, its shared external-pilot route, and its route-bound service-plan evidence through the supported operator command. The command is idempotent and reveals a new application API key only on the first call:

```bash
docker compose run --rm gateway provision \
  --organisation-name "Pilot Client" \
  --organisation-slug pilot-client \
  --project-name "Document Workflow" \
  --project-slug document-workflow \
  --model-alias alzette-chat \
  --provider-model "$OPENROUTER_MODEL" \
  --service-plan-code external-pilot \
  --service-plan-name "External pilot" \
  --service-plan-source operator_poc_boundary \
  --service-plan-finality declared
```

Provision a human membership separately. Omitting `--password-file` generates a strong password and prints it once only when the user is first created:

```bash
docker compose run --rm control user provision \
  --username pilot-admin \
  --display-name "Pilot Administrator" \
  --organisation-slug pilot-client \
  --project-slug document-workflow \
  --environment-slug production \
  --role project_admin
```

Use the returned username/password at `/login`. PostgreSQL stores only a bcrypt password hash and a SHA-256 session-token digest. Rotate a human password with `user rotate-password`; rotation and disablement revoke that user's active portal sessions.

This operator-created password flow is the current PoC behavior, not the target
customer onboarding experience. There is presently no `/signup`, invitation,
forgot-password, member-invite, or external self-registration endpoint. The
implementation-ready replacement and its security gates are defined in
[`ACCOUNT_ONBOARDING_PRD.md`](docs/prd/ACCOUNT_ONBOARDING_PRD.md); commands and routes in
that document must not be treated as available until implemented.

The provider credential remains server-side. Copy the returned Alzette `api_key` once into the client secret store; PostgreSQL contains only its SHA-256 digest and a display-safe prefix. Re-running provisioning returns the existing prefix without plaintext.

Rotate or revoke a scoped Alzette key:

```bash
docker compose run --rm gateway key rotate \
  --organisation-slug pilot-client \
  --project-slug document-workflow \
  --environment-slug production \
  --service-account application

docker compose run --rm gateway key revoke --prefix alz_k_exampleprefix
```

## API contracts

The inference and machine control APIs use `Authorization: Bearer <Alzette API key>`. Error bodies are stable JSON envelopes with a server-generated `request_id`; the same ID is returned in `X-Alzette-Request-ID` and `X-Request-ID`.

### `POST /v1/chat/completions`

Supported request fields are `model`, `messages`, `stream`, `stream_options.include_usage`, `temperature`, `top_p`, `max_tokens`, `tools`, and `tool_choice`. Text messages use `system`, `user`, and `assistant` roles with either a string or text-only `{type:"text",text:"..."}` parts. The agent subset also accepts function-tool definitions, assistant `tool_calls`, and immediately following `role:"tool"` results with matching IDs. `tool_choice` may be `auto`, `none`, `required`, or a declared named function. Function names, call IDs, tool arguments, and object JSON Schemas are bounded and validated.

Unknown top-level or nested fields and all query parameters are rejected, so request manipulation cannot choose an upstream URL, tenant, project, environment, provider model, or provider credential. Image/audio content, custom or grammar tools, client reasoning extensions, `max_completion_tokens`, structured-output controls, embeddings, and `/v1/models` are outside this tested subset and fail closed rather than being dropped.

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $ALZETTE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "alzette-chat",
    "messages": [{"role": "user", "content": "Reply with OK"}],
    "stream": true
  }'
```

With `stream:true`, the configured target must return `text/event-stream`. Alzette validates and flushes OpenAI-style text and function `tool_calls` deltas, requires a terminal supported `finish_reason` and `[DONE]`, and preserves `X-Alzette-Request-ID`. Terminal usage chunks are metered as final when both input and output totals exist, partial when only some reported dimensions exist, and unknown when usage is absent. An interrupted stream never invents token totals.

The gateway buffers the bounded non-streaming provider response before writing customer bytes. For either mode, timeouts, connection failures, `429`, and `503` may be retried only before the first downstream response write. Once an SSE frame is attempted, cancellation, timeout, malformed tail, or upstream disconnect ends that logical request without replay; the partial stream has no synthetic `[DONE]` or appended JSON error. Caller cancellation is recorded as `client_cancelled`, is not target-health evidence, and closes the upstream request context. `Retry-After` is honored within the five-second safety cap. Each retry creates a provider-attempt row while customer consumption remains one logical request. Provider error bodies, URLs, credentials, prompts, and outputs are never persisted or returned in normalized errors.

This contract is exercised with the Pi 0.84.2 `openai-completions` request shape: unconditional streaming, `max_tokens`, function tools/tool choice, streamed tool-call arguments, and assistant-tool/tool-result history. That evidence establishes this agent subset only; other SDKs and provider-specific extensions require their own contract tests.

### Tenant control APIs

- `GET /api/v1/dashboard`
- `GET /api/v1/usage?from=<RFC3339>&to=<RFC3339>&model=<alias>&limit=50`
- `GET /api/v1/requests/<Alzette request ID>`

These `/api/v1` routes remain Bearer-only and retain their machine response contracts. Usage ranges are limited to 31 days and 10,000 logical rows; callers must narrow larger periods rather than receive silent partial totals. The server rejects unknown query parameters, including client-supplied organisation or project selectors.

Responses contain the authenticated scope, logical success/failure/blocked counts, p50/p95 end-to-end latency, nullable token metrics with known/eligible counts and finality, safe recent-request metadata, registry policy plus authenticated-ledger route observations, source, and `as_of` timestamps. The route observation and last-success timestamps are derived from that project/environment's represented logical requests; target-global timestamps from a shared target are not presented as client activity. Registry disabled/unavailable policy may still make a route uncallable. This is inference evidence, not an active health probe. Responses never contain target URLs, provider credentials, another tenant's records, or provider-attempt detail. The connected PoC route renders as `External pilot` and `Shared pilot`; it does not infer or claim a live provider result.

Health endpoints are `GET /healthz` (process liveness) and `GET /readyz` (PostgreSQL readiness).

### Human client portal

`GET /` leads to the authenticated application and redirects unauthenticated browsers to the gentle `/login` page. Human credentials create a bounded, revocable server-side session with an HttpOnly session cookie and double-submit CSRF token. The portal never sends an application API key as a password and never derives tenant scope from browser input.

The portal has distinct `/app/overview`, `/app/usage`, `/app/routes`, `/app/access`, and `/app/docs` workspaces. Its `/api/portal/*` contracts expose only the current server-authorised membership. The Usage view reports exact logical-ledger totals, safe request metadata, service-account/model/project attribution, nullable token finality, latency, throughput, peak concurrency, service-plan context, rollup status, and safe CSV/JSON export. Provider attempts remain operator-only and never inflate customer totals.

The connected session contract is:

- `GET /api/portal/me`, `GET /api/portal/dashboard`, and `GET /api/portal/access`;
- `POST /api/portal/service-accounts` with exactly `name`;
- `POST /api/portal/keys/issue` with required `service_account_id`, `name`, `scopes`, and `expires_at`;
- `POST /api/portal/keys/rotate` with those fields plus the required explicit `rotated_from_prefix`;
- `POST /api/portal/keys/revoke` with exactly `prefix`;
- `POST /api/portal/context` with exactly `membership_id`;
- CSRF-protected `GET /api/portal/usage/export?format=csv|json` and `POST /logout`.

Unknown JSON fields fail closed. Key names and expiries are required; expiry is bounded from one hour through 365 days. Rotation creates an overlap and never revokes the predecessor implicitly. The caller explicitly revokes it after rollout. Plaintext is returned once at `key.api_key` only. A disabled human cannot be silently re-enabled by `user provision`; that command fails explicitly, while password rotation and disablement revoke active sessions.

Dashboard totals and trend come from the exact tenant-scoped `inference_requests` query performed at the response `as_of`, so a missing/stale rollup worker does not make that direct source unavailable. Rollup freshness and range coverage are separate. A completed empty period is exactly zero logical requests with token metrics labelled `not_applicable`; a failed-only period keeps tokens unknown. Export envelopes/preambles include authenticated scope, exact period/timezone, generated-at, units, direct-ledger source/finality, current route and route-bound plan/allocation context, and safe immutable per-request model/execution/capacity attribution where it exists. Current plan context is explicitly not inferred as historical context.

Route state keeps three sources separate: registry policy, current-binding tenant inference observations, and optional compatible probes. A registry-enabled route is not called ready. A fresh opted-in probe may report ready, degraded, or unavailable; stale/missing evidence remains unknown. Probes require a global operator flag and a per-target opt-in and are off by default.

Administrators can create service accounts and issue, overlap-rotate, or revoke scoped expiring API keys. Plaintext appears only in the successful issue/rotation response and is held in browser memory only until the one-time reveal dialog closes. The legacy Basic dashboard and source/static paths are not exposed by the control service. The rewritten landing page and public documentation are served only by the standalone public process; portal assets remain unavailable there. The Bearer-only `/api/v1` machine APIs remain separate.

## PostgreSQL schema

Migration `0001_openrouter_poc` creates:

- `organisations`, `projects`, and `environments` as the tenant/workload scope;
- `service_accounts` and hashed, scoped, revocable `api_keys`;
- approved `models`, operator-only `inference_targets`, and `tenant_routes`;
- one `inference_requests` row per authenticated logical call and one or more operator-only `provider_attempts` rows;
- the original reserved `usage_rollups_hourly` table and append-only `audit_events` for operator changes.

Migration `0002_ledger_integrity` upgrades fresh and existing 0001-only schemas. It binds each request to its service-account/API-key/prefix tuple, rollups to a composite tenant route, and project audit events to their organisation. It also enforces completion timestamps, contiguous request/attempt counts, target-matched attempts on in-progress routed requests, and completion only after every recorded attempt is terminal. A request route is attached once before attempts; active route scope/model/target changes are rejected. Critical target execution fields are frozen while a matching request is active and permanently after an attempt references the target. Operators replace such a target with a new record and retarget the route after active requests finish; enabled and registry observation fields remain mutable. Migration `0003_route_binding_observations` upgrades both fresh and existing 0002 schemas with a database-controlled route-binding generation and immutable per-request target/model attribution. Target, model, or tenant-scope binding changes advance the generation; idempotent reprovision does not. Migrated routes start in a new current epoch while pre-0003 request history remains in a legacy epoch, so even an old A -> B -> A route cycle cannot make stale evidence look current. Portal and machine route observations use only completed logical requests attributed to the current generation and exact target/model, so a same-ID route retarget remains unknown until the replacement binding records its own target-relevant inference. Provisioning and route resolution both lock an existing route before its target to avoid a route/target deadlock cycle. Migration `0004_slice0_contract_guards` makes the application-level Slice 0 target-mode and token-finality rules fail closed for new or changed database rows; it uses upgrade-safe `NOT VALID` checks so a legacy incompatible row cannot prevent deployment. All three hardening migrations have reversible down scripts.

Migration `0005_portal_identity_and_service_plans` adds human users, memberships, hashed portal sessions, key names/rotation lineage, organisation-scoped service plans, route-plan bindings, capacity-mode guards, and automatic session revocation after password/identity disablement. Migration `0006_usage_rollups_and_target_probes` adds reconciled hourly logical-request rollups, per-scope worker checkpoints, target probe opt-ins, and metadata-only health observations. Migration `0007_slice2_contract_closure` prevents direct plan edits and disabled-route target edits from drifting active service-plan capacity evidence. The original rollup table remains for upgrade compatibility; `usage_rollups_hourly_v2` is the Slice 2 schema.

Migration `0008_self_service_catalogue` is additive schema groundwork for the target hybrid self-service B2B flow. It separates catalogue model releases, deployment profiles, evidenced/versioned prices, quotes, deployments, requests, and capacity revisions so a customer can buy known endpoint capacity without selecting an operator host. Database guards require accepted matching quotes, tenant-owned dedicated targets, matching routes, and one active capacity revision. Applying this migration does **not** expose public signup, publish an offer, allocate hardware, or change a route; those application workflows remain unimplemented until their release gates pass.

Migration `0009_endpoint_billing_control_plane` adds the runnable endpoint
marketplace ledger: published offers, resumable configurations, customer
endpoints, payment requirements, Stripe customer/price/session/subscription/
invoice mappings, idempotent signed-webhook receipts, and independent
commercial/payment/runtime state guards. It also adds recent-authentication
timestamps to human sessions and permanently unique API-key names within a
service account so an ambiguous one-time-reveal retry cannot mint a hidden
duplicate. Applying it still does not publish an offer, configure Stripe,
allocate a machine, or prove provider/MeluXina availability.

Migration `0010_capacity_request_intent` closes the dedicated expansion seam:
initial deployment and later capacity requests retain their bounded sizing
intent, bind server-side retries to SHA-256 request/key digests, and protect
those facts from mutation while commercial and runtime states advance. Raw
idempotency keys, prompts, outputs, provider secrets, target hosts, and
customer-selected hardware identifiers are not stored in this ledger.

Together, the migrations keep project/environment/request records inside their organisation. Database triggers reject a route whose tenant/model alias does not match the logical request and a dedicated target binding owned by another organisation. Dedicated mode also requires a non-customer-visible operator evidence reference; external OpenRouter pilots remain shared. Shared targets are reachable only through an explicit tenant route. Completed requests/attempts and audit events are protected from mutation. The target table stores a secret reference such as `OPENROUTER_API_KEY`, not the credential itself.

## Local Go commands

With a PostgreSQL URL in the environment:

```bash
export DATABASE_URL='postgres://alzette:local-development-only@127.0.0.1:55432/alzette?sslmode=disable'
go run ./cmd/alzette migrate
ALZETTE_PUBLIC_GATEWAY_URL=http://localhost:8080 \
ALZETTE_ALLOW_INSECURE_PUBLIC_GATEWAY=true \
ALZETTE_PORTAL_COOKIE_SECURE=false \
  go run ./cmd/alzette serve --addr :8080 --static-dir .

# In another terminal; this process does not open PostgreSQL.
ALZETTE_PUBLIC_PORTAL_URL=http://localhost:8081/login \
  go run ./cmd/alzette public --addr :8082 --static-dir .
```

Production-like Compose uses separate `gateway`, `control`, `public`, and `worker` modes. `serve` combines gateway and control only for local development.

## Employee login and desktop clients

The employee client keeps OAuth and `alz_u_` credentials inside one
`alzette-agent` process. It starts a random loopback proxy, gives the selected
client only a process-scoped loopback capability, discovers the employee's
current group-assigned models, remints the maximum-ten-minute inference
credential when needed, and revokes the grant when the client exits. It does
not modify the employee's normal Pi configuration or print a reusable
credential.

Build the desktop-side helper, then start Pi through it:

```bash
go build -o /tmp/alzette-agent ./cmd/alzette-agent

# Reference Compose defaults:
ALZETTE_AGENT_ALLOW_INSECURE_LOCAL=true \
  /tmp/alzette-agent pi

# This checkout currently publishes control on 19081:
ALZETTE_AGENT_ALLOW_INSECURE_LOCAL=true \
  /tmp/alzette-agent pi --control http://127.0.0.1:19081
```

The command opens Casdoor in the browser, selects the only eligible context
automatically (or presents a numbered human-readable choice), registers an
isolated `alzette-employee` provider for that Pi process, and starts Pi on the
first assigned model. `alzette-agent login` performs the same browser login and
lists safe context/model labels as a diagnostic, but intentionally stores no
credential. `alzette-agent run -- <agent>` exposes the standard
`OPENAI_BASE_URL` and process-scoped `OPENAI_API_KEY` variables for a bounded
compatibility test with another OpenAI-compatible command-line client.

Packaged Jan Desktop 0.8.4 and Goose Desktop 1.46.0 have also completed real
local employee chats through that same authenticated loopback path. The helper
now prints the exact one-session provider values required by each desktop app.
See [the employee desktop guide](docs/EMPLOYEE_DESKTOP_CLIENTS.md) for the
short Jan and Goose setup.

This is a local demo client, not the remote-pilot artifact: the OAuth callback
uses registered loopback port `43127`, browser login is repeated for each run,
and the OAuth refresh token is memory-only. Durable keyring login, signed
cross-platform packaging, canonical TLS, mail delivery, automatic native-
client configuration, and broader version/OS evidence remain separate release
gates.

## Verification

Default verification is deterministic and never calls OpenRouter:

```bash
gofmt -w cmd internal migrations
./scripts/verify-gitignore.sh
go vet ./...
go test ./...
go test -race ./...
docker compose config --quiet
docker build -t alzette-poc:test .
```

Run migration, rollback, key-storage, tenant-trigger, transaction-rollback, and request/attempt database tests against the local Compose database:

```bash
docker compose up -d postgres
ALZETTE_TEST_DATABASE_URL='postgres://alzette:local-development-only@127.0.0.1:55432/alzette?sslmode=disable' \
  go test -v ./internal/store/postgres -count=1
```

The gateway suite uses deterministic fake targets for success, upstream rejection, `429`/`503`, timeout then success, target deadline during response buffering, terminal timeout, truncated/malformed and partial/missing usage, request limits, cancellation, injected ledger-completion failures, revocation, shared-route isolation, and dedicated-target exclusivity.

An optional live smoke test is the only test that uses OpenRouter:

```bash
chmod 0600 /absolute/path/to/newly-rotated-openrouter-key
OPENROUTER_LIVE_TEST=1 \
ALZETTE_EXTERNAL_SMOKE_APPROVED=1 \
OPENROUTER_API_KEY_FILE=/absolute/path/to/newly-rotated-openrouter-key \
OPENROUTER_MODEL='provider/model-slug' \
go test -run TestLiveOpenRouterSmoke -v ./internal/gateway
```

Populate that file through the approved secret manager without placing the key
in a command argument, shell history, environment dump, or process metadata.
The second opt-in is the coordinator's explicit approval gate; without both
flags, a file path, and a model, the test skips or fails before a provider call.
Ambient `OPENROUTER_API_KEY` alone is never accepted. The smoke sends one
bounded non-streaming request with `max_tokens`, verifies compatible model and
final usage, request correlation, one logical row/one attempt, external/shared
labels, and metadata redaction. No live provider test is part of default CI.

After the coordinator has explicitly approved the deployed client key and
DeepSeek call, the Pi-shaped gateway smoke reads that key only from a mode-0600
file and sends one bounded text/function-tool stream. For the current trusted
LAN endpoint, the exact opt-in command is:

```bash
chmod 0600 /absolute/path/to/approved-alzette-client-key
ALZETTE_PI_LIVE_TEST=1 \
ALZETTE_EXTERNAL_SMOKE_APPROVED=1 \
ALZETTE_INSECURE_LAN_SMOKE_APPROVED=1 \
ALZETTE_CLIENT_API_KEY_FILE=/absolute/path/to/approved-alzette-client-key \
ALZETTE_LIVE_GATEWAY_URL=http://befree:19080/v1 \
ALZETTE_CLIENT_MODEL_ALIAS=alzette-chat \
go test -run TestLivePiAgentGatewaySmoke -v ./internal/gateway
```

The default suite skips this test. Ambient `ALZETTE_CLIENT_API_KEY` alone is
rejected, response content is never printed on failure, and HTTP is accepted
only with the explicit insecure-LAN approval shown above. Remote use requires
TLS instead.

If completion-ledger persistence fails after an attempt starts, the gateway
fails closed and never returns buffered provider output. A logical request or
attempt can remain `in_progress`; automated reconciliation and stranded-row
recovery are an explicit Slice 3 requirement, not a Slice 0 exit condition.

Slices 1 and 2 add local human sessions, the usage/health worker, service-plan evidence, and bounded server-generated CSV/JSON exports. The remaining Slice 3 production work includes SSO/MFA integration where required, TLS ingress, rate/concurrency enforcement, stranded-ledger reconciliation, backup/restore automation, retention/runbooks, and any OpenAI compatibility beyond the tested text/function-tool streaming subset. A real OpenRouter- or DeepSeek-backed pilot claim also remains gated on an explicitly approved file-backed provider credential and a bounded live smoke.
