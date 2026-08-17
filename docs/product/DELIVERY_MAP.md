# Alzette product-to-code delivery map

**Baseline reviewed:** 2026-08-17

**Product baseline:** the two customer-facing modules and enabling inference-operations capability approved by the founder

**Maintenance owner:** Coordinator, with evidence supplied by Product, Platform, QA, and Review

## 1. Purpose

This document connects the intended Alzette product to the implementation that exists, the evidence supporting it, and the work still required.

It answers four different questions:

1. What customer feature are we trying to deliver?
2. What relevant implementation exists now?
3. Does that implementation fit the intended product, or must it change?
4. What evidence and customer outcome are still missing?

This map tracks delivery. It does not define product scope and does not itself prove that a feature is shipped.

Authority remains separated:

- [`PRODUCT.md`](PRODUCT.md) defines what Alzette is willing to ship.
- [`POC_BOUNDARY.md`](POC_BOUNDARY.md) defines what Alzette can currently claim and demonstrate.
- Focused PRDs define detailed workflows and acceptance criteria.
- Code, tests, reviewed pull requests, and real-environment evidence prove delivery.

The feature IDs in this first pass are provisional until the coordinator reconciles them into `PRODUCT.md`.

## 2. Baseline warning

At this review, repository `HEAD` is `4fd574e` (`docs: organize product, growth, and assurance material`). The implementation under `cmd/`, `internal/`, `migrations/`, deployment files, and most web application files is present in the workspace but untracked by Git.

Consequences:

- The paths below are workspace evidence, not committed or merged evidence.
- A passing local test does not mean the implementation has been reviewed or shipped.
- No implementation row may become **Shipped** until its code is committed through a reviewed PR and the required release evidence exists.
- The first engineering-governance task is to place the intended implementation baseline under version control without mixing unrelated changes.

## 3. Tracking vocabulary

### Delivery state

| State | Meaning |
| --- | --- |
| Not started | No implementation of the customer outcome was found. |
| Foundation | Reusable structures or lower-level capabilities exist, but the customer feature does not. |
| Partial | A meaningful part of the customer workflow exists, but important requirements or evidence remain. |
| Complete in code | The agreed acceptance criteria are implemented and pass deterministic tests. |
| Pilot validated | The feature passed its agreed test with a representative customer or real external environment. |
| Shipped | The feature is released to its intended users with its operational and support boundary in force. |
| Production validated | The shipped feature has passed the required security, reliability, recovery, and operating gates. |

### Implementation fit

| Fit | Meaning |
| --- | --- |
| Keep | The implementation matches the intended feature and needs no material product change. |
| Extend | The implementation has the right shape and needs additional behaviour or evidence. |
| Transform | Useful foundations exist, but the customer workflow or responsibility boundary must materially change. |
| Replace | The implementation conflicts with the intended product and should be superseded. |
| New | No meaningful implementation exists. |
| Unknown | The implementation has not been audited sufficiently. |

### Evidence level

Evidence progresses independently of delivery state:

`none → local test → deterministic integration → live external test → customer pilot → production evidence`

A schema, type, route, mock, catalogue row, quote, payment, or passing unit test is never by itself proof that the corresponding customer service is available.

## 4. Baseline summary

| Module | Foundation | Partial | Not started | Shipped |
| --- | ---: | ---: | ---: | ---: |
| Company and Workforce Access | 6 | 1 | 0 | 0 |
| Managed Dedicated Inference | 1 | 8 | 3 | 0 |
| Inference Operations | 3 | 1 | 0 | 0 |
| **Total** | **10** | **10** | **3** | **0** |

The strongest workspace foundations are tenant-safe routing and metering, service-account credentials, catalogue and endpoint control-plane records, quotes and billing states, and usage visibility.

The largest product gaps are employee federated access, the employee model space, an actual capacity recommendation engine, real dedicated inference operations, the private interaction vault, and company-specific model improvement.

## 5. Company and Workforce Access

| ID | Product feature | State | Fit | Current workspace evidence | Remaining customer outcome |
| --- | --- | --- | --- | --- | --- |
| WA-01 | Company account | Foundation | Transform | Organisation, human-user, membership, and session records in [identity migration](../../migrations/0005_portal_identity_and_service_plans.up.sql) and [portal store](../../internal/store/postgres/portal.go); tenant-isolation coverage in [portal integration tests](../../internal/store/postgres/portal_worker_integration_test.go). | Replace operator-led account creation with verified B2B establishment, first-admin setup, recovery, and auditable administrator replacement. |
| WA-02 | Employee management | Foundation | Transform | Membership roles, session revocation, user disablement, and operator provisioning in [identity migration](../../migrations/0005_portal_identity_and_service_plans.up.sql), [portal store](../../internal/store/postgres/portal.go), and [operator command](../../cmd/alzette/main.go). | Add company-admin invitations, acceptance, groups, access profiles, employee lifecycle UI, and complete access-change audit. |
| WA-03 | Employee model space | Foundation | Transform | Authenticated portal shell and catalogue endpoints in [portal application](../../internal/portal/site.go), model catalogue service in [catalogue](../../internal/catalogue/catalogue.go), and portal contract coverage in [endpoint tests](../../internal/portal/endpoints_test.go). | Build the employee-specific model discovery and direct-query experience with permitted activity and workflow instructions. |
| WA-04 | Model access policy | Foundation | Transform | Company/project/environment roles and workload scopes in [platform types](../../internal/platform/types.go), route enforcement in [gateway](../../internal/gateway/gateway.go), and isolation tests in [gateway tests](../../internal/gateway/gateway_test.go). | Add employee/group/project model entitlements, policy categories, limits, approvals, filtered discovery, and safe denial explanations. |
| WA-05 | Human authentication | Foundation | Transform | Password hashing and opaque sessions in [human authentication](../../internal/humanauth/humanauth.go), portal login/session handling in [portal application](../../internal/portal/site.go), and session tests in [portal tests](../../internal/portal/site_test.go). | Add company-controlled federated login, agent-initiated login, short-lived employee inference authority, reauthentication rules, and complete offboarding behaviour. |
| WA-06 | Workflow access | Foundation | Transform | Agent-compatible streaming and tool-call subset in [gateway agent tests](../../internal/gateway/agent_test.go) and stable routed request handling in [gateway](../../internal/gateway/gateway.go). | Bind agent and embedded-workflow access to the authenticated employee and their model entitlements; document supported and impossible host integrations. |
| WA-07 | Workload identities | Partial | Extend | Service-account creation, one-time keys, scope, rotation, revocation, and attribution in [portal application](../../internal/portal/site.go), [credentials](../../internal/credentials/credentials.go), [portal store](../../internal/store/postgres/portal.go), and [store tests](../../internal/store/postgres/portal_worker_integration_test.go). | Commit and review the implementation, complete customer UX and audit coverage, enforce production lifetimes and limits, and pass the external-pilot security gate. |

### Workforce transformation hotspot

The existing portal is primarily an administrator and application-access control surface. It should be evolved rather than mistaken for the intended employee product. Password sessions and service-account keys are useful foundations, but they do not satisfy employee federated login or employee-authenticated agent access.

## 6. Managed Dedicated Inference

| ID | Product feature | State | Fit | Current workspace evidence | Remaining customer outcome |
| --- | --- | --- | --- | --- | --- |
| MI-01 | Curated model catalogue | Partial | Extend | Customer-safe models, releases, profiles, offers, prices, allowances, and evidence fields in [catalogue service](../../internal/catalogue/catalogue.go), [catalogue store](../../internal/store/postgres/catalogue.go), and [catalogue schema](../../migrations/0008_self_service_catalogue.up.sql); integration coverage in [PostgreSQL tests](../../internal/store/postgres/integration_test.go). | Publish real qualified open-weight models and tenant-specific availability; add employee filtering, business guidance, lifecycle management, and reviewed evidence for every offer. |
| MI-02 | Guided capacity recommendation | Foundation | Transform | Workload-intent fields for use case, context, concurrency, request rate, latency priority, and monthly volume in [endpoint service](../../internal/endpoints/endpoints.go) and [capacity migration](../../migrations/0010_capacity_request_intent.up.sql). | Build and validate the recommendation logic that converts business demand into a model, operating profile, capacity, safety margin, and understandable alternatives. |
| MI-03 | Monthly offer | Partial | Transform | Recurring prices, setup prices, allowances, offer states, and billing periods in [catalogue service](../../internal/catalogue/catalogue.go), [billing service](../../internal/billing/billing.go), and [commercial schema](../../migrations/0009_endpoint_billing_control_plane.up.sql). | Replace seeded or hypothetical pricing with approved offers derived from validated capacity and operating cost; complete buyer-facing comparison and contractual terms. |
| MI-04 | Advanced transparency | Partial | Extend | Model version, context, operating profile, accelerator class/count/memory, metrics, capacity finality, source, and evidence fields in [catalogue service](../../internal/catalogue/catalogue.go) and [catalogue schema](../../migrations/0008_self_service_catalogue.up.sql). | Build the advanced customer view and connect planned configuration to actual reserved machine evidence, measured service ranges, location, isolation, and safety margin. |
| MI-05 | Quote and commitment | Partial | Extend | Versioned quotes, expiry, acceptance, payment requirements, and separated runtime/commercial/payment rails in [endpoint service](../../internal/endpoints/endpoints.go), [endpoint workflow store](../../internal/store/postgres/endpoints_workflow.go), and [billing integration tests](../../internal/store/postgres/endpoint_billing_integration_test.go). | Complete customer review UX, live commercial validation, tax and invoice decisions, idempotent real payment evidence, and the contractual acceptance boundary. |
| MI-06 | Deployment lifecycle | Partial | Transform | Customer and operator deployment/request state machines in [endpoint service](../../internal/endpoints/endpoints.go), [endpoint workflow store](../../internal/store/postgres/endpoints_workflow.go), and [deployment schema](../../migrations/0008_self_service_catalogue.up.sql). | Connect state transitions to real capacity and model operations, customer-safe next actions, incident states, retirement, and verified transition evidence. |
| MI-07 | Stable model service | Partial | Transform | Stable aliases, server-controlled routes, model versions, target isolation, retries, and safe errors in [gateway](../../internal/gateway/gateway.go), [platform types](../../internal/platform/types.go), and [gateway tests](../../internal/gateway/gateway_test.go). | Replace deterministic/external forwarding evidence with an operated dedicated model service; complete production compatibility, promotion, rollback, limits, and service commitments. |
| MI-08 | Usage and service visibility | Partial | Extend | Logical request ledger, provider-attempt separation, usage attribution, rollups, probes, exports, and route evidence in [control](../../internal/control/client_dashboard.go), [portal](../../internal/portal/site.go), [worker](../../internal/worker/worker.go), and their tests. | Add employee attribution, real infrastructure evidence, production telemetry, alerting, contractual service views, and validated customer-facing accuracy. |
| MI-09 | Capacity changes | Partial | Transform | Capacity request intent, versioned scale quotes, request transitions, and capacity revisions in [endpoint workflow store](../../internal/store/postgres/endpoints_workflow.go), [endpoint service](../../internal/endpoints/endpoints.go), and [endpoint integration tests](../../internal/store/postgres/endpoint_billing_integration_test.go). | Fulfil changes against real reserved capacity, preserve the stable endpoint, expose progress, validate rollback/failure behaviour, and reconcile price with the active service. |
| MI-10 | Private interaction vault | Not started | New | No prompt/output persistence or vault domain was found; the current ledger intentionally retains metadata only. | Define and implement tenant-isolated content capture, trace structure, encryption, search, attribution, policy enforcement, export, deletion, and recovery. |
| MI-11 | Vault governance | Not started | New | No vault policy, retention, employee notice, legal-hold, selection, or improvement-consent implementation was found. | Implement company-controlled retention classes, access roles, employee transparency, redaction, holds, export/deletion, and a consent boundary separate from retention. |
| MI-12 | Company-specific model improvement | Not started | New | No private dataset, evaluation, adaptation, candidate, approval, release, or rollback implementation was found. | Define the managed engagement, data-rights gate, dataset curation, evaluation baseline, candidate comparison, release approval, private deployment, lineage, and rollback. |

### Managed-inference transformation hotspot

The workspace has a credible endpoint-acquisition control plane. It does not yet have a business-facing capacity recommender or a real dedicated service behind that control plane. Forms and state records must become a trustworthy managed journey driven by validated operating profiles and real capacity evidence.

## 7. Inference Operations

| ID | Product feature | State | Fit | Current workspace evidence | Remaining customer outcome |
| --- | --- | --- | --- | --- | --- |
| IO-01 | Model qualification and operating profiles | Foundation | Transform | Model/release/profile/metric evidence structures and validation in [catalogue service](../../internal/catalogue/catalogue.go), [catalogue store](../../internal/store/postgres/catalogue.go), and [catalogue schema](../../migrations/0008_self_service_catalogue.up.sql). | Create the repeatable qualification workflow, benchmark and quality evidence, approval authority, versioning rules, and publication gate for real model profiles. |
| IO-02 | Capacity planning and fulfilment | Foundation | Transform | Workload intent, profile capacity ranges, deployment requests, quotes, and capacity revision records in [endpoint service](../../internal/endpoints/endpoints.go) and [endpoint workflow store](../../internal/store/postgres/endpoints_workflow.go). | Implement demand-to-capacity planning, safety margins, provider availability, reservation/release, shortage handling, cost reconciliation, and truthful fulfilment evidence. |
| IO-03 | Deployment operation | Foundation | Transform | Target validation, routed inference, health probes, deployment states, and background work in [provisioning validation](../../internal/provisioning/validate.go), [gateway](../../internal/gateway/gateway.go), [worker](../../internal/worker/worker.go), and [fake target](../../internal/faketarget/faketarget.go). | Implement real model preparation, capacity attachment, validation, activation, request scheduling, isolation, recovery, upgrade, rollback, and retirement on the intended compute environment. |
| IO-04 | Operational evidence | Partial | Extend | Immutable request/attempt lineage, route-binding evidence, health observations, usage finality, rollups, and exports in [platform types](../../internal/platform/types.go), [control](../../internal/control/client_dashboard.go), [worker store](../../internal/store/postgres/worker.go), and integration tests. | Add infrastructure telemetry, reservation identity, deployment events, capacity truth, service alerts, retention, production observability, and reconciliation with customer commitments. |

### Operations transformation hotspot

Current operations evidence proves a bounded forwarding and control-plane design. It does not prove automatic capacity reservation, model deployment, dedicated execution, production recovery, or the intended compute-provider integration.

## 8. Product dependency path

The implementation should be evaluated in this order:

1. `IO-01` qualifies a model and operating profile.
2. `MI-01` publishes the approved model and `MI-02` recommends capacity.
3. `MI-03` and `MI-05` turn the plan into an accepted commercial commitment.
4. `IO-02` and `IO-03` fulfil and validate the service.
5. `MI-06` to `MI-09` expose and operate the customer service truth.
6. `WA-01` to `WA-07` make the service safely available to the company workforce and applications.
7. `MI-10` and `MI-11` preserve authorised company interactions.
8. `MI-12` uses separately approved evidence to create a private model candidate.

This dependency path does not imply that all work must be sequential. It identifies which upstream evidence downstream customer claims depend on.

## 9. First coordination priorities

### Priority 0 — establish Git truth

- Review and commit the intended implementation baseline through focused PRs.
- Keep unrelated website, product-document, implementation, and environment changes separate.
- Record the merge PR for each row when it first becomes committed evidence.
- Add the provisional feature IDs to `PRODUCT.md` once Product approves the reconciliation.

### Priority 1 — close the workforce product gap

- Decide and specify the B2B company identity and employee-login contract.
- Build company-admin employee invitations and lifecycle management.
- Transform the portal into a role-appropriate employee model space.
- Bind agent access to short-lived employee authority and model entitlements.

### Priority 2 — turn endpoint configuration into managed onboarding

- Establish real qualified model profiles.
- Implement workload-based model and capacity recommendation.
- Connect monthly offers and advanced details to the same validated profile.
- Prove the quote and service-state separation with a real fulfilment path.

### Priority 3 — prove dedicated operations

- Obtain the required compute-provider access and operating constraints.
- Prove capacity reservation, deployment, validation, routing, recovery, and retirement.
- Feed actual capacity and deployment evidence back into customer surfaces.

### Priority 4 — add company AI memory after the service boundary is stable

- Define vault rights, employee transparency, retention, export, deletion, and recovery.
- Implement the vault before accepting any model-improvement use of customer content.
- Run the first improvement engagement only after evaluation, approval, private release, and rollback gates exist.

## 10. Pull-request update rule

Every material implementation PR should reference at least one feature ID and update this map when its state, fit, evidence, or remaining gap changes.

Recommended PR description fields:

```text
Product feature: WA-05
Customer outcome:
Scope included:
Scope excluded:
Implementation paths:
Acceptance evidence:
Delivery-map change:
Remaining gap:
```

Status changes require evidence:

- Platform identifies the implementation paths and recommends **Keep**, **Extend**, **Transform**, or **Replace**.
- Product confirms that the customer outcome and remaining gap are accurate.
- QA supplies independent test evidence.
- Review confirms that the evidence supports the claimed state.
- Coordinator records the merged PR and maintains the cross-feature sequence.

Do not calculate completion percentages from these rows. A small missing policy or operational gate can block an otherwise large feature from being usable or shippable.

## 11. Baseline verification

The 2026-08-17 workspace audit found relevant implementation under `cmd/`, `internal/`, and `migrations/` and ran:

```text
go test ./...
```

All discovered packages passed locally. This is deterministic workspace evidence only: database-backed tests may use their own controlled fixtures, and no live inference provider, dedicated compute allocation, customer pilot, or production environment was proven by this command.
