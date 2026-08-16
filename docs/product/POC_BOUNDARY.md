# Alzette OpenRouter PoC boundary

**Status:** Slice 0–2 and endpoint-control-plane offline software proof passed; live provider/Stripe evidence and Slice 3 production controls pending

**Date:** 2026-08-14

**Future identity contract:** [`WORKFORCE_AGENT_ACCESS_PRD.md`](../prd/WORKFORCE_AGENT_ACCESS_PRD.md)
defines invited-employee OAuth, short-lived human-agent tokens, and a local
compatibility proxy. Those capabilities are not current PoC evidence.

## Outcome

The PoC gives one B2B pilot client an honest view of an Alzette-managed inference service. The current software is configured to forward inference through an operator-controlled OpenRouter target, while default verification uses a deterministic compatible target and makes no provider call.

The client integrates with a stable Alzette OpenAI-compatible endpoint. Alzette authenticates the request, resolves a tenant-authorised route, forwards it to a server-controlled compatible target, records authoritative usage metadata, and exposes a credential-scoped dashboard. A separately authorised live smoke is required before describing this as a live OpenRouter-backed pilot. When MeluXina becomes available, the same target contract can point to a private model server address and port without changing the client API.

The authenticated portal also contains the first endpoint-provider journey: a
curated eligible model catalogue, resumable shared/dedicated configuration,
immutable quote and hosted-payment state, independent provisioning rails, and
dedicated capacity-change requests. These paths are backed by PostgreSQL and
deterministic tests, but no offer, price, checkout, private machine, or
MeluXina deployment is treated as live merely because the UI and schema exist.

This PoC validates the Alzette software boundary and makes the product concept demonstrable. It does not validate customer value, product-market fit, live OpenRouter availability, live Stripe settlement, MeluXina hosting, or dedicated compute.

## Product boundary

### In scope

- One operator-provisioned client organisation with projects and environments.
- One stable Alzette `/v1/chat/completions` endpoint.
- One or more client-facing model aliases mapped to operator-controlled OpenRouter model slugs.
- Scoped, revocable API credentials; target credentials never leave Alzette.
- Bounded non-streaming chat plus the tested text/function-tool SSE subset required by Pi 0.84.2; cancellation, pre-output retry, partial-response, tool-delta, and final/missing-usage behavior is deterministic-test evidence, not a claim of broad OpenAI compatibility.
- One logical customer request record and one or more internal target-attempt records.
- PostgreSQL as the authoritative tenant, route, request, usage, and audit store.
- A human-authenticated client portal with separate Overview, Models, Endpoints,
  Usage, Access, Billing, and Docs workspaces; exact project/environment
  consumption, service-account/model attribution, safe recent requests,
  source/finality, route evidence, and bounded server-generated exports.
- Authenticated Models, Endpoints, request/configuration detail, and Billing
  workspaces with server-derived eligibility, safe price/evidence finality,
  resumable drafts, idempotent submissions, recent-password commercial
  confirmation, and provider-neutral execution labels.
- Operator-seeded shared evaluation/paid-shared/dedicated offers; immediate
  allow-listed shared evaluation activation; immutable dedicated quotes and
  deterministic operator fulfilment transitions; capacity requests preserve
  immutable bounded sizing intent, the customer alias, and endpoint contract.
- A Stripe adapter and isolated signed-webhook listener that are disabled by
  default; customer payment goes to Alzette's merchant account and never
  requires a customer-owned Stripe account.
- A standalone public landing page and implementation-documentation process with no database or provider credential, an exact public-asset allow-list, and a configured link to the separate client login.
- Operator-entered route-bound shared allowance or dedicated allocation context; missing contractual values remain unknown.
- Reconciled hourly logical-request rollups and per-scope worker checkpoints.
- Metadata-only compatible probes, disabled globally and per-target by default.
- A minimal operator surface or command/API that provisions the tenant, target, route, model alias, and key without a database edit.
- Docker Compose deployment on one machine.
- Deterministic fake-target integration tests plus an opt-in live OpenRouter smoke test.

### Explicitly out of scope

- MeluXina allocation, model deployment, Slurm/OpenStack automation, or claims of Luxembourg execution.
- Dedicated-compute claims for OpenRouter. The pilot is `external_pilot` and `shared` unless OpenRouter supplies evidenced dedicated capacity.
- Customer-selected upstream URLs, arbitrary model slugs, or provider credentials.
- Silent fallback across customer, model, service mode, or execution location.
- Self-service model deployment, training, fine-tuning, evaluation, or marketplace workflows.
- Full SSO/SCIM, complex RBAC, Kubernetes, Redis, Kafka, ClickHouse, or multi-host high availability.
- Invitation acceptance, customer-managed human membership, password recovery,
  transactional email, or public self-registration. The current human user and
  membership are operator-provisioned; the target hybrid self-service
  evaluation/invitation workflow is specified in
  [`ACCOUNT_ONBOARDING_PRD.md`](../prd/ACCOUNT_ONBOARDING_PRD.md).
- Casdoor, portal OIDC, PKCE/device login, human-agent `alz_u_` tokens, a local
  credential proxy, native employee-agent login, or per-employee inference
  attribution. Their separate target contract is
  [`WORKFORCE_AGENT_ACCESS_PRD.md`](../prd/WORKFORCE_AGENT_ACCESS_PRD.md).
- Prompt/output history, prompt analytics, or support access to content.
- Live Stripe checkout/settlement evidence, scheduled retry/reconciliation of
  deferred billing events, production invoice/tax/refund/dispute operations,
  or MeluXina infrastructure-cost reconciliation. Contract charges are shown
  only if authoritative data exists.

## System boundary

```text
client application
      │  Alzette API key
      ▼
alzette gateway
  auth → tenant/project/environment/model-alias route → validation → request ledger
      │  OpenRouter credential + configured model slug
      ▼
OpenRouter target
      │
      ▼
model response + usage

client browser → human login/session → alzette control/portal → membership-scoped queries → PostgreSQL
                                      ├→ service accounts / one-time API keys
                                      ├→ route registry / inference evidence
                                      └→ rollup checkpoint / optional probe evidence
```

For the first-client seam, the control service exposes only the login, application workspaces, exact allow-listed assets, and `/api/portal/*`. It does not expose signup, invitation acceptance, or password recovery. A human password creates a server-side session; it is not an Alzette API key. Session and CSRF cookies protect browser mutations. Application inference keys remain one-time-reveal service-account secrets. The unchanged `/api/v1` machine APIs are Bearer-only and separate. The PoC listener is HTTP; remote use requires TLS, while the explicitly requested trusted-LAN demo uses a visible insecure-transport warning and non-secure cookies. Legacy dashboard and source paths are not served. The rewritten public landing page and documentation run under `alzette public` from a different static root and are not reachable through the authenticated control service.

Migration `0008_self_service_catalogue` supplies the catalogue/deployment base.
Migration `0009_endpoint_billing_control_plane` supplies the runnable customer
configuration, endpoint, payment, Stripe-mapping, and webhook ledgers. The
additive `0010_capacity_request_intent` migration preserves immutable bounded
workload-sizing intent and hashed retry identity for deployment/capacity
requests. The control process exposes these only through a human membership/session and
server-derived tenant scope; the billing process exposes only its signed
webhook endpoint. These migrations do not enable public signup, publish an
offer, configure Stripe, allocate a machine, or change a route on their own.
Existing targets, tenant routes, service plans, logical requests, and provider
attempts remain runtime and usage truth; a dedicated endpoint becomes callable
only after the explicit validated target/route transition.

The target interface contains an execution class, evidenced capacity mode, base URL/private address, model identifier, secret reference, timeout, last-observed state, retry policy, and explicit probe policy. OpenRouter uses `https://openrouter.ai/api/v1`; a later MeluXina target uses a private LAN base URL. Both implement the same Alzette target contract. Registry policy, tenant-scoped inference observations, and target-shared probe observations remain distinct. A route becomes ready only from fresh opted-in probe evidence; registry-enabled alone never means ready.

## Reference deployment

The current PoC uses one repository, one Go module, and one application image
with separate `gateway`, `control`, `public`, `billing-webhook`, and `worker` process modes plus a one-shot `migrate`
command:

- `alzette gateway`: inference ingress, API-key authentication, route resolution,
  request validation, forwarding, error normalization, and logical
  request/provider-attempt metering.
- `alzette control`: human login/session handling, the multi-view client portal,
  service-account/key lifecycle, catalogue/endpoint/commercial APIs, safe
  exports, Bearer machine APIs, and membership-scoped route/usage reads.
- `alzette billing-webhook`: bounded Stripe signature verification, immutable
  event receipt, and server-authoritative commercial-state application; it has
  no portal or gateway routes.
- `alzette public`: serves only the public landing page, implementation docs,
  stylesheet, and river mark; process readiness is independent of PostgreSQL.
- `alzette worker`: reconciles hourly logical-request rollups and, only when
  globally enabled, executes individually opted-in compatible probes.
- `alzette migrate`: applies the embedded PostgreSQL migration before the two
  long-running services start.
- `alzette provision`, `alzette key`, `alzette user`, `alzette catalogue`, and
  `alzette endpoint`: ad-hoc operator commands from the same
  image; they are not additional long-running Compose services.

Docker Compose currently contains PostgreSQL, the one-shot migration service,
gateway, control, public site, and worker. PostgreSQL remains loopback-only. HTTP services
bind to loopback by default and may use an explicit trusted-LAN bind for this
demo. PostgreSQL logical request rows remain the exact customer totals; the
worker produces query-optimised rollups and independently labelled freshness
evidence. TLS ingress, retention/recovery jobs, admission controls, and optional
infrastructure telemetry remain Slice 3 work.

## Client top dashboard

The first screen answers four questions in order.

### 1. Can my application call Alzette now?

- Latest evidenced route state: `operational`, `degraded`, `unavailable`, `stale`, or `unknown`.
- Stable Alzette endpoint and approved model alias.
- Actual connected execution label: **External pilot / Shared pilot**. OpenRouter is the operator-configured target for this PoC, but the browser does not infer a provider name from an execution class and no live provider response has been evidenced.
- Service mode: **Shared pilot** unless dedicated capacity is evidenced.
- Last successful inference, latest current-binding inference observation, optional probe observation, and independent freshness.
- Active incident or next action before any chart.

This is a registry plus evidence view, not an SLA or capacity guarantee. Active compatible probes appear only after the two explicit opt-ins; otherwise callability remains unknown.

### 2. What is this authenticated project/environment consuming?

- Logical requests, successful requests, error rate, and blocked requests.
- Input, output, cached, and reasoning tokens when OpenRouter reports them.
- p50/p95 end-to-end latency; time to first token only after streaming is supported.
- Throughput and peak concurrency derived from represented logical-request intervals; spend is absent unless an authoritative source is implemented.
- Shared allowance or dedicated allocation appears only from the operator-entered route-bound service plan; absent values remain unknown.
- The browser is scoped by the current server-authorised human membership. A user with multiple memberships can switch explicit contexts; no client-supplied tenant selector can widen access.

### 3. Where is consumption coming from?

- Time series for requests, tokens, errors, and latency.
- The authenticated project/environment is named explicitly; the current membership cannot aggregate unauthorised sibling projects.
- Breakdown by model alias and exact executed model returned by OpenRouter.
- Service-account breakdown is present. Key prefixes are shown only in the Access lifecycle view, not used as customer billing dimensions.
- No global OpenRouter pool data and no other-tenant data.

### 4. Can we investigate safely?

- Recent request rows: Alzette request ID, timestamp, project, alias, executed model, status/error class, latency, and token counts.
- No prompts or outputs.
- Copyable request ID for support correlation.
- Server-generated CSV/JSON export of the represented bounded snapshot, including safe scope, period/timezone, units, route/deployment/model context, service mode, service-plan context, generated-at, source, and finality.

Every panel has a source and `as of` timestamp. Zero usage, partial usage, stale telemetry, and an outage are different states. Unknown token fields remain unknown rather than becoming zero.

## Authoritative data model

- `organisations`: tenant and contract boundary.
- `projects` and `environments`: workload/lifecycle scope.
- `service_accounts` and `api_keys`: non-human identity and revocable authentication.
- `models`: stable client alias plus approved capabilities/version policy.
- `inference_targets`: operator-controlled execution class, capacity mode, base URL, model, secret reference, and health.
- `tenant_routes`: tenant/project/environment/model alias to an authorised target; clients cannot override it.
- `inference_requests`: one row per logical client call and the customer usage source.
- `provider_attempts`: one row per outbound target attempt; operator-only reliability/COGS evidence.
- `service_plans` and `tenant_service_plans`: operator-entered, route-bound allowance/allocation evidence with explicit source/finality.
- `human_users`, `human_memberships`, and `portal_sessions`: bcrypt human identity and digest-only bounded portal sessions.
- `usage_rollups_hourly_v2`: worker-reconciled logical-request aggregates; provider-attempt counts remain internal.
- `worker_checkpoints`: per-scope rollup freshness/finality, including truthful zero-row runs.
- `target_health_observations`: metadata-only results from explicitly opted-in compatible probes.
- `audit_events`: append-only administrative changes without secrets or content.

One client call remains one customer request even if a pre-output retry creates multiple attempts. Once response bytes have been sent, the gateway does not transparently replay the call.

## OpenRouter contract

- Upstream endpoint: `POST https://openrouter.ai/api/v1/chat/completions`.
- Authentication: a server-side bearer token resolved from the operator-controlled target secret reference, with the mounted file authoritative when configured.
- Alzette replaces the client model alias with the configured OpenRouter model slug.
- Alzette forwards only an allow-listed Chat Completions subset: text messages, bounded sampling/max-token fields, function tools/tool choice, assistant tool-call history, tool results, and OpenAI-style SSE. Unknown, multimodal, provider-extension, structured-output, or routing fields fail closed rather than being discarded.
- OpenRouter response usage supplies prompt, completion, reasoning, and cached token fields when present. Provider cost is not an implemented customer meter in this slice.
- Streaming requires validated text/function-call chunks, a supported terminal finish reason, and `[DONE]`. Terminal usage is final/partial according to reported fields; absent or interrupted usage remains unknown. Retry is forbidden after the first downstream write.
- `X-Generation-Id` or response `id` is stored as the upstream request identifier when available.
- OpenRouter error metadata is normalized into stable Alzette error classes while preserving a safe upstream correlation identifier.
- `429`/`503` and `Retry-After` are respected. Automatic retry is bounded, only before response output, and recorded as a new attempt.

Official contract references:

- <https://openrouter.ai/docs/quickstart>
- <https://openrouter.ai/docs/cookbook/administration/usage-accounting>
- <https://openrouter.ai/docs/api/reference/errors-and-debugging>
- <https://openrouter.ai/docs/api/reference/streaming>

## Security and privacy invariants

- Tenant scope comes from the authenticated credential/session, never a client-supplied tenant ID.
- A client cannot choose or discover a raw target URL, OpenRouter credential, or another tenant's model binding.
- API keys are stored as hashes, revealed once, revocable, and never logged.
- Provider secret references resolve an operator-mounted `<REFERENCE>_FILE`
  before environment fallback; a configured but invalid file fails closed.
- Request bodies and response bodies are proxied but not persisted by default.
- Logs, traces, audit events, and usage rows contain metadata only.
- Body size, timeouts, and upstream response size are bounded. Admission/rate/concurrency enforcement is not implemented.
- Dashboard, request detail, and exports are protected by server-side tenant authorization.
- External execution and its data-location implications are visible and cannot render as MeluXina/on-premise/dedicated.

## Test and release boundary

The offline software slice is not complete until its implemented mandatory groups pass:

1. **Unit:** alias resolution, auth, key hashing, request limits, error mapping, usage parsing, finality, and redaction.
2. **HTTP contract:** methods, content types, size limits, malformed bodies, unknown fields, timeouts, cancellation, upstream status/header propagation, request IDs, and duplicate-auth rejection.
3. **Tenant isolation:** two tenants, cross-project IDs, wrong aliases, wrong keys, usage filters, request detail, exports, shared target allow-list, and dedicated target exclusivity.
4. **Retry/accounting:** timeout then success is one logical request/two attempts; terminal failures; missing/partial usage; no retry after output.
5. **Database:** migrations up/down/reapply, upgrade from the prior schema, constraints, idempotent provisioning, transaction rollback, and request/key/route/attempt ledger integrity.
6. **Integration:** deterministic fake compatible target for success, retry, outage, and usage variants, plus the real gateway/control/PostgreSQL path.
7. **Live smoke:** opt-in test with a newly rotated OpenRouter key and approved low-cost model; never required in default CI, but required before claiming a live external pilot.
8. **Browser:** protected access, dashboard hierarchy, empty/partial/outage states, export, keyboard use, safe CSV, static fallback, and responsive widths.
9. **Accessibility:** semantic landmarks/headings, accessible tables/chart alternatives, focus, contrast, and status not conveyed by colour alone.
10. **Security/review:** no secret/content logging, static-file containment, headers, authz review, race test, and independent code review. A current toolchain/dependency vulnerability review remains a production gate.

Required verification commands and the exact pass/deferred evidence are versioned in `../assurance/POC_TEST_PLAN.md` and `../assurance/QA_REPORT.md`.

The Slice 2 worker, bounded server-generated exports, and tested agent streaming subset are part of this release candidate. Backup/restore automation, stranded-row reconciliation, rate/concurrency enforcement, TLS ingress, compatibility beyond the tested text/function-tool subset, retention/runbooks, and production operations are explicit later gates. Their absence does not invalidate the narrow offline software proof, but it blocks production readiness and any claim that those capabilities exist.

## Evidence gates

### Gate A — offline software PoC

This gate passes when deterministic compatible-target, PostgreSQL, protected-browser, isolation, retry/accounting, migration, Compose, race, and security-containment checks pass without an external provider credential. It proves the software boundary, not OpenRouter availability, customer value, MeluXina hosting, or production operations.

### Gate B — live OpenRouter pilot

This gate passes when one provisioned client can:

1. call its Alzette model alias through a scoped key;
2. receive a real OpenRouter-backed response with an Alzette request ID using a newly rotated, file-mounted provider key;
3. open a tenant-safe dashboard showing truthful route evidence and the authenticated project/environment's consumption;
4. reconcile dashboard totals and export to logical request rows, including retry and partial-usage cases; and
5. see the execution route labelled **External pilot / Shared pilot**, with no MeluXina, dedicated-capacity, or unverified live-provider claim.

Until the opt-in live smoke is recorded, the release label is **offline/fake-target software PoC**, not **live OpenRouter-backed customer pilot**.

### Gate C — MeluXina deployment

After authorised access exists, the operator must deploy a compatible model server on MeluXina, register and verify its private LAN address/port, and replace the external target without changing the client's endpoint, alias, or key. Capacity, locality, dedicated mode, operations, recovery, and commercial terms require their own evidence. This gate is not part of the current PoC.
