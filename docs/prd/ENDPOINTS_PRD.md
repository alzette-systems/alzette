# Alzette model catalogue and endpoints PRD

**Status:** PoC vertical slice implemented; live Stripe, catalogue-supply, and production-fulfilment gates remain

**Date:** 2026-08-14

**Owners:** product, platform, design, operations, finance, and security

**Related documents:** [`PRODUCT.md`](../product/PRODUCT.md) defines the product and
commercial unit; [`PORTAL_PRD.md`](PORTAL_PRD.md) defines the complete portal;
[`POC_BOUNDARY.md`](../product/POC_BOUNDARY.md) controls claims about the running PoC;
[`ACCOUNT_ONBOARDING_PRD.md`](ACCOUNT_ONBOARDING_PRD.md) owns signup, recovery,
membership, and invitations. This document makes the customer-facing model
catalogue, endpoint acquisition, payment, and endpoint-management journey
implementable.

## 1. Decision

Alzette will add two first-class authenticated workspaces:

- **Models** is the curated catalogue where a customer discovers reviewed model
  releases and the service modes for which their organisation is eligible.
- **Endpoints** is where a customer creates, pays for, monitors, and expands an
  Alzette endpoint backed by one approved model release and service profile.

The portal must not ask customers to choose an upstream URL, provider slug,
container, host, LAN address, or raw machine. The commercial unit is an
**endpoint capacity unit**: a versioned model/runtime/hardware profile with an
explicit accelerator count, evidenced capacity, and price. Alzette selects and
operates the physical infrastructure.

The customer-facing action is not “subscribe to a model.” That phrase collapses
catalogue, billing, and runtime state. The launch actions are:

- **Create shared endpoint** for an immediately eligible, operator-approved
  shared offer;
- **Configure dedicated endpoint** for a private capacity request and quote;
- **Add capacity** for a quoted revision to an existing dedicated endpoint.

Stripe is the default payment provider. Stripe collects payment details,
payments, subscriptions, and invoices. Alzette remains authoritative for tenant
eligibility, catalogue publication, quote content, endpoint entitlement,
physical allocation, deployment, route binding, and runtime readiness.

The customer does **not** create or connect a Stripe account. Alzette's legal
operating company is the seller and owns the Stripe merchant account into which
the customer pays. Stripe then settles funds to the company's configured payout
account. A `Stripe Customer` is only a server-side billing record created under
Alzette's merchant account; it is not a Stripe login, wallet, marketplace
account, or commercial relationship between the client and Stripe.

These states are deliberately separate:

```text
catalogue entry -> eligible offer -> endpoint configuration -> quote
       -> quote acceptance -> payment requirement -> payment confirmation
       -> allocation -> deployment -> validation -> route ready -> serving
```

No redirect page, Stripe event, accepted quote, paid invoice, database row, or
catalogue card may independently claim that a dedicated endpoint is ready.

## 2. Product outcome

The feature is successful when an authorised person at a business can:

1. discover which models Alzette has actually reviewed;
2. understand which models are available as shared or dedicated endpoints;
3. compare capabilities, execution boundary, capacity evidence, and price;
4. create an eligible shared endpoint without contacting Alzette;
5. configure dedicated endpoint intent without selecting raw infrastructure;
6. receive and accept a versioned capacity/price quote;
7. satisfy the applicable payment requirement through Stripe;
8. see commercial, provisioning, route, and health states separately;
9. create an application key and call the ready endpoint through a stable
   Alzette alias; and
10. add dedicated capacity later without changing that alias, URL, model,
    service mode, or credential contract.

The feature must make Alzette feel like an inference provider rather than a
dashboard attached to one operator-provisioned route.

## 3. Current baseline

### Implemented

- organisation/project/environment-scoped human sessions;
- service accounts and one-time-reveal application keys;
- operator-owned model aliases, inference targets, and tenant-route bindings;
- dedicated-target ownership and shared-target allow-list constraints;
- service-plan, usage, route, health, and request-ledger views;
- PostgreSQL migration `0008_self_service_catalogue`, which creates schema
  groundwork for catalogue models/versions, deployment profiles and metrics,
  prices, evaluation offers, business qualification, immutable quotes, model
  deployments, deployment requests, and capacity revisions;
- a single Go image deployed as multiple Docker Compose processes;
- PostgreSQL migration `0009_endpoint_billing_control_plane`, which adds
  customer configurations/endpoints, offer publication, server-owned payment
  requirements and Stripe mappings, checkout/subscription/invoice facts,
  idempotent webhook receipts, and append-only lifecycle guards;
- PostgreSQL migration `0010_capacity_request_intent`, which preserves bounded
  workload-sizing facts and hashed retry identity on immutable deployment and
  capacity requests;
- PostgreSQL migration `0011_endpoint_team_size`, which adds a bounded,
  nullable people-count intent to endpoint drafts and immutable submitted
  deployment requests without deriving it from concurrency;
- authenticated Models, model detail, Endpoints, endpoint/request detail,
  configurator, capacity-request, and Billing workspaces;
- tenant-scoped customer APIs for catalogue reads, resumable endpoint drafts,
  submission, immutable quote reads/acceptance, hosted checkout, billing
  summary, hosted billing portal, endpoint reads, and capacity requests;
- an audited operator CLI for catalogue seeding, immutable quote issuance, and
  deterministic dedicated fulfilment transitions;
- immediate allow-listed shared-evaluation activation, paid-shared payment
  gating, and dedicated quote/payment/provisioning state rails;
- a provider-neutral billing service, Stripe SDK adapter, separately exposed
  signed-webhook process, idempotent/out-of-order event ledger, and
  server-authoritative paid activation;
- recent-password confirmation for commercial mutations, CSRF enforcement,
  stable idempotency keys, one-time application-key display, and permanent
  per-service-account key-name uniqueness to prevent hidden retry duplicates.

### Remaining release gates

- publish at least one operator-reviewed offer in each mode intended for a
  pilot; an empty catalogue remains an honest empty state;
- run the opt-in real-provider smoke and a Stripe test-mode end-to-end checkout
  against the company's merchant account; neither is exercised by default;
- add scheduled reconciliation/retry for retained billing events when immediate
  application fails, plus finance/legal review of invoice, tax, refund,
  dispute, suspension, and cancellation policy;
- implement MeluXina allocation, deployment, validation, and production
  dedicated-capacity fulfilment;
- implement the public signup/recovery and organisation invitation flows owned
  by [`ACCOUNT_ONBOARDING_PRD.md`](ACCOUNT_ONBOARDING_PRD.md);
- complete Slice 3 TLS ingress, rate/concurrency enforcement, backup/restore,
  retention, and independent production security review.

Migrations `0008` through `0010` plus the application code provide a runnable
control-plane path. They do not by themselves publish an offer, prove Stripe or
provider availability, allocate hardware, or establish MeluXina execution.

## 4. Scope and priorities

### P0 — credible provider journey

The software path below is implemented. Operational P0 is not passed until the
selected catalogue offers, Stripe test-mode flow, compatible inference target,
and dedicated fulfilment evidence are configured and exercised together.

P0 includes:

- authenticated Models catalogue and model-release detail;
- authenticated Endpoints list and endpoint detail;
- organisation/project/environment eligibility resolution;
- one real shared evaluation offer with explicit hard limits;
- one paid shared offer using fixed recurring pricing and a hard allowance;
- dedicated endpoint configuration, submission, operator review, and immutable
  quote presentation;
- Stripe sandbox and live-ready integration for Checkout, invoices,
  subscriptions, webhooks, and a restricted customer portal;
- endpoint provisioning progress with separate commercial and runtime states;
- one ready shared endpoint exercised through the real gateway;
- one deterministic dedicated fulfilment path using a fake/private-compatible
  target until MeluXina is available;
- quoted capacity expansion behind a stable route;
- complete two-tenant, payment-replay, and state-separation tests.

### P1 — production fulfilment

P1 includes:

- MeluXina allocation/deployment/validation and private target registration;
- production dedicated fulfilment and capacity increases;
- contract-specific invoice terms and payment methods;
- controlled cancellation, renewal, suspension, and recovery;
- model release notices and migration/rollback;
- organisation team invitations and member management from
  [`ACCOUNT_ONBOARDING_PRD.md`](ACCOUNT_ONBOARDING_PRD.md).

### Explicitly deferred

- arbitrary model uploads, fine-tuning, LoRA, or marketplace publishing;
- raw GPU/machine selection;
- token-based or request-based Stripe metered billing;
- automatic dedicated allocation solely because money was received;
- multiple payment processors in the first implementation;
- coupons, promotional credits, affiliate plans, and consumer checkout;
- procurement/ERP integrations, purchase-order automation, or invoice APIs;
- cross-organisation model sharing;
- team invitations in this implementation slice.

Team invitations are critical but adjacent. The endpoint domain must be owned
by the organisation, never by the purchasing human, so invitations can be added
later without migrating endpoints, Stripe Customers, quotes, or usage.

## 5. Product vocabulary and invariants

| Term | Meaning | Must not imply |
|---|---|---|
| Catalogue model | Customer-readable model family | Availability, a route, or a price |
| Model release | Immutable reviewed version with licence/support evidence | A deployed runtime |
| Deployment profile | Internal model/runtime/hardware/service bundle selected by Alzette | A customer-facing choice or customer control of a target or host |
| Endpoint capacity unit | Profile-defined capacity increment | Linear scaling unless the metric says so |
| Offer | Profile and commercial terms for an eligible organisation/scope | Runtime readiness |
| Endpoint configuration | Customer intent for model, service, scope, and people count | A customer-selected profile, quote, automated recommendation, or commitment |
| Quote | Immutable, expiring organisation-specific price/capacity snapshot | Payment, allocation, or readiness |
| Payment requirement | Commercial condition attached to an accepted offer/quote | Endpoint activation by itself |
| Deployment | Actual provisioned model/runtime lifecycle | A healthy route until validated |
| Endpoint | Stable Alzette invocation identity and its authorised route | Exposure of the target URL |
| Capacity revision | Historical change in active dedicated units | A changed customer endpoint contract |

Binding invariants:

1. Every resource is organisation-scoped; endpoint configuration is also
   project/environment-scoped.
2. The browser never submits a target ID, upstream URL, provider model ID,
   provider credential, Stripe Price ID, amount, or currency as authority.
3. A published model release is selectable only through a server-approved,
   eligible profile.
4. A shared endpoint binds only an explicitly allow-listed shared target.
5. A dedicated endpoint binds only a validated target exclusively owned by the
   same organisation.
6. Accepted quotes are immutable historical evidence.
7. Payment state and service state never share one status field.
8. Capacity is not multiplied unless each displayed metric declares its scaling
   semantics for the selected unit count.
9. Expanding capacity preserves the endpoint URL, alias, model release, service
   mode, execution class, and credential contract unless a separate migration
   is accepted.
10. Prompts, outputs, provider secrets, target URLs, card data, and webhook
    secrets are never persisted in endpoint/customer-facing records.

## 6. Actors and authority

| Actor | Allowed | Forbidden |
|---|---|---|
| Evaluation administrator | Browse eligible catalogue, use free shared evaluation, prepare dedicated configuration, submit qualification | Activate dedicated capacity, alter prices/limits, select targets, or access another tenant |
| Organisation administrator | Create eligible shared endpoint, submit dedicated request, accept quote, initiate payment, request capacity change | Change server-owned amount/currency/Stripe IDs, bind target, or bypass qualification |
| Finance role — future | View/accept commercial terms and manage payment/invoices under policy | Change model/runtime/target or grant roles |
| Developer | Inspect eligible models/endpoints and create scoped application keys when authorised | Accept quote, initiate payment, cancel commercial service, or see restricted billing fields |
| Viewer/auditor | Read permitted catalogue, endpoint, evidence, usage, and commercial summaries | Mutate configuration, payment, keys, or membership |
| Alzette operator | Publish reviewed entries/profiles/prices, qualify businesses, issue quotes, fulfil/validate deployments, bind targets | Act as the customer to accept a quote or fabricate payment/readiness |
| Stripe | Collect payment details and emit commercial events | Authenticate portal users, choose entitlements, targets, models, or endpoint state |

Until invitations/roles are implemented, the one existing `org_admin` may
perform customer commercial actions. Every such action requires recent
reauthentication and records the exact actor. This temporary role collapse must
not be represented as a completed team/finance permission model.

## 7. Information architecture

The authenticated navigation becomes:

```text
Overview
Models
Endpoints
Usage
Access
Docs
```

`Routes` becomes `Endpoints` in customer language. Route binding remains an
internal/runtime concept shown as evidence inside an endpoint detail page.

### Routes

| Route | Purpose |
|---|---|
| `/app/models` | Eligible curated catalogue |
| `/app/models/{model-slug}` | Reviewed model release and available profiles |
| `/app/endpoints` | Existing endpoints and in-progress requests |
| `/app/endpoints/new?model={slug}` | Shared/dedicated endpoint configurator |
| `/app/endpoints/requests/{id}` | Configuration, qualification, quote, payment, and fulfilment progress |
| `/app/endpoints/{id}` | Ready/degraded endpoint operations, capacity, commercial status, and usage links |
| `/app/billing` | Billing summary and redirect to Stripe-hosted customer portal; may remain an endpoint subview in P0 |

Deep links must resolve server-side and fail closed when the authenticated
organisation/project/environment cannot access the resource.

## 8. Primary journeys

### 8.1 Free shared evaluation

```text
verified evaluation organisation
  -> Models
  -> default shared evaluation model
  -> review allowance, execution boundary, retention and expiry
  -> Create evaluation endpoint
  -> server binds only the configured evaluation offer
  -> create separate application key
  -> make first call
  -> see real usage and remaining allowance
```

No payment method is required. Repeated signup or endpoint submission cannot
multiply the allowance. Exhaustion blocks before any provider attempt.

### 8.2 Paid shared endpoint

```text
Models -> eligible shared profile -> choose project/environment and alias
  -> review fixed plan, allowance, execution boundary and renewal terms
  -> Confirm and pay -> server creates Stripe Checkout Session
  -> Stripe-hosted Checkout -> return to Alzette pending page
  -> verified webhook + reconciliation confirms initial invoice paid
  -> Alzette creates entitlement and allow-listed route atomically
  -> endpoint ready -> create/use application key
```

The initial paid shared offer is a fixed recurring amount with an enforced
allowance and no automatic overage. Token/request metering may inform the
dashboard but is not a billing source until reconciliation and finality are
proven.

### 8.3 Dedicated endpoint

```text
Models -> reviewed release -> Dedicated private
  -> choose project/environment and stable alias
  -> state how many people need to use the endpoint
  -> Alzette applies the reviewed server-owned default context during managed review
  -> Alzette selects and attaches an eligible deployment profile and capacity floor
  -> submit request -> business/operations review
  -> receive immutable expiring quote
  -> reauthenticate and accept quote
  -> satisfy quote-specific Stripe payment/invoice requirement
  -> operator approves allocation
  -> allocate -> deploy -> validate -> register exclusive target -> bind route
  -> endpoint ready -> test through the customer gateway URL
```

For the default launch policy, dedicated allocation starts only after the
required setup charge or first invoice is confirmed. A quote may instead carry
approved invoice terms; that policy is stored on the payment requirement, not
inferred from a Stripe status. Receiving payment never skips operator approval,
capacity evidence, target validation, or exclusive binding.

### 8.4 Capacity increase

```text
Endpoint detail -> Add capacity
  -> compare current and supported total units
  -> review full resulting capacity/price snapshot
  -> accept immutable scale-up quote
  -> satisfy payment requirement
  -> operator adds and validates resources
  -> activate new capacity revision behind the same endpoint
```

The UI shows current and requested total capacity, not only the delta. A metric
that is not evidenced as scalable is not multiplied in the quote.

### 8.5 Cancellation and payment failure

- Shared subscriptions may be scheduled to stop at period end when policy
  permits. The portal states the exact end time and request behavior afterward.
- Dedicated cancellation is a commercial request, not an immediate Stripe
  Customer Portal switch, unless the contract explicitly allows self-service
  cancellation.
- `invoice.payment_failed` marks billing attention. It does not silently reroute
  a dedicated customer to shared capacity or immediately destroy a production
  endpoint.
- Alzette applies the service plan's grace/suspension policy in a separate,
  audited control-plane transition.
- Refund, dispute, and chargeback states require operator/finance review; they
  do not mutate historical usage or accepted quote evidence.

## 9. Stripe payment contract

### 9.1 Default integration choices

- Use Stripe-hosted Checkout for paid shared subscriptions and eligible
  immediate charges.
- Use Stripe Invoicing/hosted invoice payment for dedicated B2B charges where
  invoice terms are appropriate.
- Use one server-side Stripe Customer object per Alzette organisation, never
  per human user. Creating it requires no customer Stripe account or Stripe
  credentials.
- Use a restricted Stripe Customer Portal for billing details, payment methods,
  and invoices. Do not enable plan changes or cancellation that bypass Alzette
  endpoint policy.
- All charges and invoices are issued through Alzette's Stripe merchant account
  and settle to Alzette's configured payout account. The client pays as an
  ordinary business customer by an enabled payment method.
- Keep Alzette's immutable deployment quote authoritative. Stripe Quotes may be
  used as a delivery adapter later but are not required for quote truth or
  acceptance.
- Use fixed recurring prices for P0. Stripe usage meters are deferred.
- Create Stripe objects server-side with deterministic idempotency keys.
- Treat signed webhook events plus explicit object reconciliation as commercial
  evidence. A browser `success_url` is never payment authority.

### 9.2 Payment sequence

1. Customer confirms an eligible server-owned offer or accepts a still-valid
   Alzette quote after reauthentication.
2. Alzette creates or resolves the organisation's internal Stripe Customer
   object under Alzette's merchant account. The payer creates no Stripe account.
3. Alzette creates a payment requirement and one Checkout Session,
   subscription, or invoice using server-owned amount/currency/line items.
4. The browser is redirected to Stripe. Stripe payment fields never transit the
   Alzette application.
5. The customer returns to a pending page that says confirmation is still in
   progress.
6. A dedicated webhook listener verifies the Stripe signature against the raw,
   bounded body and stores an idempotent event receipt.
7. The worker reconciles the current Stripe object, updates the payment
   requirement, and emits an audit event.
8. A paid shared entitlement may then be activated atomically. A dedicated
   request may advance only to its next operator-controlled fulfilment state.

### 9.3 Required Stripe events

At minimum, handle and reconcile the events needed for the selected flows:

- `checkout.session.completed`;
- `checkout.session.expired`;
- `invoice.finalized`;
- `invoice.paid`;
- `invoice.payment_failed`;
- `customer.subscription.created`;
- `customer.subscription.updated`;
- `customer.subscription.deleted`;
- applicable refund/dispute events when those payment modes are enabled.

Event delivery is duplicate-tolerant and order-independent. Event IDs are
deduplicated; object ID plus event type is also considered during
reconciliation. The worker retrieves current Stripe state when an event is
missing, delayed, or arrives out of order.

### 9.4 Tax and invoicing

- Billing legal name, address, and tax ID belong to the organisation billing
  profile.
- Checkout may collect tax IDs and billing addresses; Alzette stores only the
  minimum safe references/status needed for customer and audit views.
- Currency, tax treatment, reverse-charge handling, tax codes, and automatic
  tax are operator/finance configuration. They must not be inferred from the
  browser locale or from “Luxembourg.”
- A quote must distinguish subtotal, tax treatment/finality, setup amount,
  recurring amount, billing period, term, and total due now.
- Stripe Tax is optional until legal/tax configuration is reviewed. The portal
  must display `Tax not yet determined` instead of calculating a value locally.

### 9.5 Payment security

- Stripe secret and webhook secrets use file-backed Docker secrets.
- No secret key, webhook secret, card number, bank detail, client secret, or
  complete webhook payload appears in logs, audit, analytics, URLs, or customer
  APIs.
- Only a short-lived Stripe-hosted URL is returned to the authorised browser.
- Webhook bodies are size-bounded and signature-verified before parsing or DB
  effects.
- Stripe API writes use operation-derived idempotency keys; customer retries do
  not create duplicate Customers, sessions, subscriptions, or invoices.
- Production payment requires HTTPS portal return URLs and HTTPS webhook
  ingress. The current LAN HTTP PoC may use Stripe sandbox/CLI forwarding only
  and must not claim live payment readiness.

## 10. Page requirements

### 10.1 Models catalogue

**Primary question:** Which approved model can this organisation deploy, and in
which service mode?

The page contains:

- organisation/project/environment context;
- current evaluation/qualification state;
- search and restrained filters for capability and service mode;
- ruled model rows showing model family, exact recommended release, reviewed
  capability labels, lifecycle, and source freshness;
- separate availability labels for `Shared evaluation`, `Paid shared`, and
  `Dedicated private`;
- price labels only when an effective visible price exists;
- one primary action per eligible mode;
- a truthful request/review action when no profile is activatable.

The page does not become a generic marketplace card wall. It inherits the
portal's paper/ink ledger system and uses rows/tables for comparable facts.

Required states:

- no catalogue entries published;
- published model but no eligible profile;
- checking eligibility;
- shared available now;
- dedicated available to configure/quote;
- licence or support review pending;
- deprecated release with migration guidance;
- unavailable/retired without disappearing from historical endpoint records.

### 10.2 Model detail

The detail page shows:

- model family and exact release;
- supported modalities and tested API capabilities;
- context window and other verified limits;
- licence/support/lifecycle status and source timestamp;
- eligible deployment profiles grouped by service mode;
- execution class and location wording supported by evidence;
- capacity metrics with unit, finality, evidence source, measurement time, and
  scaling semantics;
- effective indicative/contractual price and billing period where available;
- CTA `Create shared endpoint`, `Configure dedicated endpoint`, or
  `Request review`.

Provider benchmark claims, uncited leaderboards, invented prices, and
unverified MeluXina availability are forbidden.

### 10.3 Endpoint configurator

The configurator is a resumable, server-backed workflow:

1. **Model:** exact release, immutable after submission.
2. **Service:** shared or dedicated, with boundary explanation. Unavailable
   services are disabled; when only one is eligible it is selected by default.
3. **Scope:** organisation/project/environment and stable alias.
4. **Team size:** exactly one editable field:
   - label: **How many people need to use this endpoint?**
   - helper: **Count everyone who may use it, not only people using it at the
     same time. Alzette uses this when reviewing the managed service. The
     selected model’s default context is used.**
   - validation: **Enter a whole number between 1 and 10,000.**
5. **Review:** customer intent, price/term finality when available, and what
   happens next.
6. **Submit/Pay:** create a request or redirect to Stripe depending on service
   and eligibility.

Every step is validated server-side. Back/refresh preserves a safe draft. A
draft contains no secret, prompt, target URL, or payment detail.

Step 4 renders no use-case, customer-selected context, concurrency, request
volume, or workload-priority control. The people count is required to advance
and submit in the revised UI, while an incomplete draft may exist without it.
The server uses a reviewed default belonging to the selected offer/profile;
it must not infer that default from the model's maximum context window. The
wording above describes Alzette's managed review, not evidence that automated
sizing or default-context machinery exists today.

Deployment profiles, offer codes, profile codes, aliases, and capacity-unit
floors are resolved and attached by Alzette. They do not render as customer
controls and the revised browser payload cannot select them. The model picker
only enables models with at least one currently eligible service; the server
rechecks that eligibility when it creates the draft.

This amendment does not change the separate post-creation capacity-increase
flow in section 8.4, its dialog, or its API contract.

### 10.4 Request and fulfilment progress

This page presents four independent rails:

| Rail | Examples |
|---|---|
| Configuration | draft, submitted, needs information, rejected |
| Commercial | quote pending, offered, accepted, expired |
| Payment | not required, action required, processing, paid, past due |
| Infrastructure/runtime | awaiting approval, allocating, deploying, validating, ready, degraded, failed |

The main message is derived from the next customer action, not from collapsing
the four rails. Examples:

- “Quote accepted — payment is required before allocation.”
- “Payment received — capacity allocation has not started.”
- “Deployment validating — the endpoint is not callable yet.”
- “Endpoint ready — create an application key or make a test call.”

For a request created under the revised contract, the submitted-intent summary
shows **People using this endpoint** and the stored people count. Historical or
legacy-client requests without that field show **Not recorded**. The customer
progress UI does not substitute concurrency for people and does not expose the
legacy technical workload fields.

### 10.5 Endpoints list

The list includes ready endpoints and in-progress requests. Each row shows:

- stable alias and project/environment;
- model family and exact release;
- shared/dedicated mode;
- execution class;
- endpoint/runtime status;
- commercial attention state without exposing restricted amounts;
- current capacity units or shared allowance;
- latest validated/observed time;
- primary next action.

Filters include status, project/environment, service mode, and model. Empty
state leads to Models rather than instructing the user to contact an operator.

### 10.6 Endpoint detail

The endpoint detail answers, in order:

1. Can applications call it now?
2. Which model release and service boundary back it?
3. What stable URL/alias should applications use?
4. What capacity/allowance is active and how fresh is the evidence?
5. What is the commercial/payment state?
6. What usage has this organisation recorded?
7. What action is safe: test, create key, pay, add capacity, or request help?

Provider URLs, target members, provider credentials, and internal attempts stay
operator-only. Application key lifecycle remains in Access.

### 10.7 Billing access

P0 may use a compact billing panel instead of a full custom billing product. It
shows:

- organisation billing name and safe tax-ID status;
- active fixed subscriptions/invoices tied to endpoints;
- payment action required and due date;
- invoice/receipt links supplied by Stripe;
- `Manage billing in Stripe` redirect.

`Manage billing in Stripe` opens a short-lived hosted session created by
Alzette. It does not ask the client to register for or connect a Stripe account.
The Stripe Customer Portal must not be embedded in an iframe. The portal return
lands back on the exact Alzette endpoint/request context.

## 11. Functional requirements

| ID | Requirement | Acceptance criterion |
|---|---|---|
| EP-P0-001 | The portal MUST expose Models and Endpoints as distinct workspaces. | A first-time administrator can discover and acquire an endpoint; existing route operations remain available under Endpoints. |
| EP-P0-002 | Catalogue responses MUST be filtered by server-derived organisation/project/environment eligibility. | Cross-tenant/profile IDs fail closed and do not reveal entry existence or price. |
| EP-P0-003 | Every selectable model MUST have an exact reviewed release and at least one server-resolvable eligible deployment profile. | The browser selects no profile; Alzette resolves one transactionally or fails closed. |
| EP-P0-004 | Shared and dedicated offers MUST remain visibly and technically distinct. | Shared activation cannot bind a dedicated target; dedicated fulfilment cannot bind a shared target or silently fall back. |
| EP-P0-005 | Customers MUST configure endpoints without raw infrastructure fields. | Browser/API schemas contain no target URL, host, provider slug/model ID, secret, image, or arbitrary hardware ID. |
| EP-P0-006 | The revised endpoint configurator MUST capture an expected people count while Alzette owns profile and capacity selection. | Step 4 exposes exactly one required integer from 1 through 10,000; no profile, offer code, or capacity-unit control appears in endpoint creation. |
| EP-P0-007 | Quotes MUST be immutable, expiring, and organisation/scope-bound. | Expired, superseded, changed, or cross-tenant quote acceptance fails; historical accepted quotes remain readable. |
| EP-P0-008 | Quote acceptance MUST require an authorised recent human session. | API keys and stale/CSRF-less sessions cannot accept commercial terms; actor and snapshot are audited. |
| EP-P0-009 | Stripe amounts and object mappings MUST be server-owned. | Manipulating amount, currency, Product/Price ID, Customer ID, quote ID, return URL, or metadata cannot change the purchase. |
| EP-P0-010 | Payment confirmation MUST use signed webhook/reconciled Stripe state. | Visiting or replaying a success URL never marks a requirement paid. |
| EP-P0-011 | Stripe event handling MUST be idempotent and order-independent. | Duplicate, delayed, missing, replayed, and out-of-order fixture events converge on one correct state. |
| EP-P0-012 | Paid shared activation MUST be atomic and allow-listed. | Exactly one entitlement/route is created after valid payment; retries create no duplicate allowance, route, or subscription. |
| EP-P0-013 | Dedicated payment MUST NOT activate runtime capacity. | Paid state advances only to the permitted fulfilment gate; target/route readiness still requires operator allocation and validation. |
| EP-P0-014 | Endpoint readiness MUST require target and route evidence. | `ready` is impossible without validated target ownership/binding and freshness evidence. |
| EP-P0-015 | Capacity expansion MUST create an immutable revision behind the same endpoint. | Active units change only after payment policy, resource evidence, validation, and atomic revision activation. |
| EP-P0-016 | Payment failures/cancellations MUST follow an explicit service policy. | No silent fallback or immediate dedicated destruction occurs directly from a Stripe webhook. |
| EP-P0-017 | The portal MUST show truthful independent state rails. | Paid/provisioning, quote-accepted/payment-pending, and route-ready/payment-attention render distinctly. |
| EP-P0-018 | Commercial and endpoint operations MUST be audited without secrets. | Events include opaque org/quote/request/endpoint/payment references, actor/result, and no card, prompt, output, or provider secret. |
| EP-P0-019 | P0 billing MUST use fixed pricing and enforced allowances. | Unknown/partial token usage cannot change an invoice; overage is blocked or explicitly unsupported. |
| EP-P0-020 | Invitation absence MUST not make endpoint ownership personal. | Every endpoint, quote, Stripe Customer mapping, and invoice link belongs to the organisation; the actor is attribution only. |

## 12. API contract

Implemented resource paths follow the existing control API conventions.

### Customer/session API

```text
GET  /api/portal/catalogue/models
GET  /api/portal/catalogue/models/{slug}
GET  /api/portal/endpoints
GET  /api/portal/endpoints/{id}
POST /api/portal/endpoint-configurations
GET  /api/portal/endpoint-configurations/{id}
PATCH /api/portal/endpoint-configurations/{id}
POST /api/portal/endpoint-configurations/{id}/submit
GET  /api/portal/deployment-requests/{id}
GET  /api/portal/deployment-quotes/{id}
POST /api/portal/deployment-quotes/{id}/accept
POST /api/portal/payment-requirements/{id}/checkout-session
GET  /api/portal/billing
POST /api/portal/billing/portal-session
POST /api/portal/endpoints/{id}/capacity-requests
POST /api/portal/reauthenticate
```

All mutation APIs require the human session, CSRF protection, server-derived
scope, role enforcement, and an idempotency token. Quote acceptance and payment
initiation additionally require recent reauthentication.

The revised browser configuration payload contains only customer intent:

```json
{
  "model_slug": "alzette-chat",
  "service_mode": "shared",
  "workload": {
    "expected_user_count": 20
  }
}
```

For this managed-selection shape, `service_mode` is `shared` or `dedicated`.
Alzette selects the published eligible offer/profile, derives the approved
alias, and applies the profile's minimum capacity units inside the database
transaction. `offer_code`, `profile_code`, `endpoint_alias`, and
`capacity_units` must be absent. Legacy clients may continue using the complete
explicit shape for compatibility, but the customer portal never does.

The domain field is `Workload.ExpectedUserCount`; its JSON name is
`workload.expected_user_count`. When supplied, it is a JSON integer from `1`
through `10000`, inclusive. Zero, negative values, fractions, strings, and
larger values fail validation. The revised UI requires the field before it can
advance or submit, but the API keeps it optional so existing clients and
records remain compatible.

The legacy `use_case`, `expected_context_tokens`, `expected_concurrency`,
`expected_requests_per_minute`, `latency_priority`, and
`expected_monthly_requests` members remain accepted, persisted, and returned
for existing API clients. They are not rendered or sent by the revised
endpoint-creation UI and do not substitute for `expected_user_count`. A UI
update to an existing draft must preserve omitted legacy values rather than
clear them. Existing immutable deployment requests are not backfilled or
rewritten; a missing people count remains `null` and is presented as **Not
recorded**. The people count participates in idempotency comparison and becomes
immutable when copied into a submitted deployment request.

The revised endpoint-creation UI does not send a context size. Managed review
must use a reviewed server-owned default for the selected offer/profile; a
catalogue maximum context window is not that default. Legacy API clients may
still send `expected_context_tokens` for compatibility, but it does not
override the revised flow's default. The API must not claim an automated
recommendation until the corresponding machinery and evidence exist. The
separate `POST /api/portal/endpoints/{id}/capacity-requests` contract is
unchanged by this amendment.

### Stripe ingress

```text
POST /webhooks/stripe
GET  /healthz
```

The webhook listener exposes no portal pages or customer read API. It accepts
only the configured Stripe signature scheme and bounded content type/body.

### Operator API/CLI

Operators need supported, audited operations to:

- publish/retire catalogue models and releases;
- publish/retire profiles, metrics, and effective prices;
- enable/disable shared offers;
- review business qualification and deployment requests;
- issue/supersede/expire quotes;
- approve fulfilment;
- attach validation evidence, target, route, and capacity revision;
- reconcile/suspend/recover a commercial entitlement.

No workflow may require manual SQL editing.

### Error vocabulary

Machine-readable errors include:

```text
catalogue_unavailable
model_not_eligible
profile_not_eligible
profile_evidence_missing
business_qualification_required
capacity_unavailable
quote_not_found
quote_expired
quote_superseded
quote_already_accepted
payment_not_configured
payment_action_required
payment_processing
payment_failed
billing_reconciliation_pending
deployment_not_ready
capacity_change_in_progress
permission_denied
```

Errors never contain another tenant's resource, Stripe object, provider target,
or secret.

## 13. Technical architecture

The implementation stays inside the existing Go/PostgreSQL/Docker Compose
architecture.

```text
browser
  -> control/portal: catalogue, configuration, quote, payment-session APIs
  -> Stripe-hosted Checkout / Customer Portal

Stripe
  -> billing-webhook: signature verification + event receipt
  -> PostgreSQL

worker
  -> reconcile Stripe objects and commercial state
  -> advance eligible shared entitlement or dedicated fulfilment gate

operator control
  -> allocation/deployment/validation/target binding

gateway
  -> reads only authorised ready route/entitlement state
```

### Components

- `internal/catalogue`: eligibility, model/release/profile presentation, and
  price visibility;
- `internal/endpoints`: configuration, request, quote, deployment, and capacity
  state orchestration;
- `internal/billing`: provider-neutral payment requirements and commercial
  reconciliation;
- `internal/billing/stripe`: official Stripe Go SDK adapter pinned to a reviewed
  version;
- `alzette billing-webhook`: narrow process mode from the same image, holding
  only the webhook verification secret and database access;
- existing `control`: authenticated session/CSRF APIs and creation of
  server-owned payment operations;
- existing `worker`: PostgreSQL-backed reconciliation/checkpoints; no Redis,
  Kafka, or new workflow engine;
- existing `gateway`: no Stripe dependency and no direct reaction to raw
  webhook events.

If Stripe API reconciliation requires a secret in the worker, use a restricted
file-backed key. The webhook listener does not receive the general Stripe API
key unless an implemented reconciliation operation proves it needs one.

### Network boundary

- Portal/control and Stripe return URLs require HTTPS before live mode.
- Only `/webhooks/stripe` is exposed for Stripe ingress; operator services and
  model targets remain private.
- The billing webhook process has no provider/model secret and no customer
  session endpoint.
- Docker Compose health checks cover control, webhook, worker, gateway, and DB.

## 14. Data model additions

Migration `0008` remains the catalogue/deployment base. Migration
`0009_endpoint_billing_control_plane` introduces the payment and endpoint
control records without adding Stripe fields to routes or inference requests.
Migration `0010_capacity_request_intent` currently makes the legacy technical
workload fields and hashed retry identity on initial deployment and capacity
requests durable and immutable; it contains no prompt, output, target,
provider secret, or raw idempotency token.

Migration `0011_endpoint_team_size` adds a nullable `expected_user_count`
integer to endpoint configurations and deployment requests, constrained to
`1..10000` when present. It is copied into the immutable submitted-request
snapshot and included in the hashed request intent.
Existing rows remain `NULL`; no migration derives people count from concurrency
or any other legacy field. Existing legacy columns remain intact for API and
historical compatibility. A reviewed server-owned default context belongs to
the selected offer/profile and must be represented separately from the model's
maximum context-window capability; this PRD does not claim that representation
or an automated sizing engine is implemented.

### `billing_accounts`

- `id`, `organisation_id` unique;
- `provider` (`stripe` initially);
- provider customer-object ID under Alzette's merchant account, stored as an
  opaque server-side reference; this is not a customer-owned Stripe account;
- billing legal name/address/tax-status references;
- lifecycle state and timestamps;
- no card/bank detail.

### `payment_requirements`

- organisation/project/environment scope;
- purpose (`shared_activation`, `dedicated_setup`, `dedicated_recurring`,
  `capacity_change`);
- source offer or immutable quote;
- amount/currency/period/tax snapshot;
- collection mode (`checkout_subscription`, `checkout_payment`, `invoice`,
  `invoice_terms`, `not_required`);
- state (`pending`, `action_required`, `processing`, `paid`, `past_due`,
  `failed`, `cancelled`, `refunded`, `disputed`);
- provider object references and safe failure class;
- timestamps and immutable paid snapshot.

### `billing_checkout_sessions`

- payment requirement and organisation scope;
- Stripe Checkout Session reference;
- Alzette operation/idempotency key digest;
- state/expiry/completion timestamps;
- no return URL supplied by the browser.

### `billing_subscriptions`

- payment requirement/offer/endpoint scope;
- provider subscription/customer references;
- status, current period, cancel-at-period-end, and reconciliation timestamp;
- fixed price/version snapshot.

### `billing_invoices`

- payment requirement/quote/endpoint scope;
- provider invoice reference;
- safe number/status/amount/currency/due/paid timestamps;
- hosted invoice/PDF links returned only under authorised short-lived policy;
- no raw invoice payload by default.

### `stripe_event_receipts`

- unique Stripe event ID;
- event type and provider object ID;
- payload digest, signature verification time, processing state, retry count,
  safe failure class, and timestamps;
- minimal retained fields or encrypted/raw payload only if a reviewed retention
  need exists;
- idempotent processor checkpoint.

Composite foreign keys must enforce tenant/scope ownership. Accepted/paid
commercial snapshots and processed event receipts are protected against
destructive mutation. Provider IDs are never accepted as unscoped lookup
authority from customer requests.

## 15. Audit and analytics

Required audit/product events:

| Event | Purpose |
|---|---|
| `catalogue_model.viewed` / `catalogue_profile.selected` | Model discovery and eligibility |
| `endpoint_configuration.created/submitted` | Acquisition funnel |
| `deployment_request.created/reviewed` | Dedicated workflow |
| `deployment_quote.offered/accepted/expired` | Commercial decision |
| `payment_requirement.created` | Payment gate creation |
| `billing_checkout.started/completed/expired` | Checkout funnel |
| `billing_invoice.paid/failed` | Commercial status |
| `billing_subscription.changed` | Renewal/cancellation status |
| `endpoint.fulfilment_started/ready/failed` | Operational delivery |
| `endpoint.capacity_requested/activated` | Expansion |
| `endpoint.first_call_succeeded` | Activation outcome |

Analytics never contain prompt/output content, API keys, card/bank information,
Stripe secrets, raw webhook bodies, provider secrets, or target URLs.

Primary measures:

- catalogue view -> configuration start;
- configuration start -> submission;
- submission -> quote offered;
- quote offered -> accepted;
- accepted -> payment completed;
- payment completed -> endpoint ready;
- endpoint ready -> first successful call;
- time in each fulfilment state;
- paid shared allowance exhaustion and renewal;
- capacity requested versus activated.

## 16. UX states and content rules

Every page must support loading, empty, permission, validation, conflict,
stale-data, and backend-unavailable states. Commercially dangerous states need
specific language:

| State | Required language pattern |
|---|---|
| Catalogue only | “Available to configure — no endpoint has been assigned.” |
| Indicative price | “Indicative price — an accepted versioned quote controls.” |
| Quote accepted | “Terms accepted — payment and provisioning are separate.” |
| Checkout returned | “Payment confirmation is pending.” |
| Paid dedicated request | “Payment received — infrastructure is not yet allocated.” |
| Deploying | “Deployment in progress — the endpoint is not callable.” |
| Ready | “Validated route ready” plus evidence timestamp |
| Past due | Exact payment action and service/grace policy; no threat or surprise |
| Capacity requested | Current units, requested total units, and unchanged endpoint contract |

The portal must never say `Subscribed`, `Active`, or `Available` without naming
which domain is active: commercial subscription, entitlement, deployment,
route, or runtime.

The endpoint configurator uses the following exact Step 4 content:

- step title: **4. Team size**;
- field label: **How many people need to use this endpoint?**;
- helper: **Count everyone who may use it, not only people using it at the same
  time. Alzette uses this when reviewing the managed service. The selected
  model’s default context is used.**;
- validation: **Enter a whole number between 1 and 10,000.**

The field is a required whole-number control in the revised UI. Missing,
fractional, zero, negative, and over-limit values use the validation text above
and block progression and submission. Loading or restoring a legacy draft does
not expose its technical workload fields; it asks for the missing people count
before revised-UI submission. Request progress uses **People using this
endpoint** and either the value or **Not recorded**.

The creation UI never says or exposes **profile**, **offer code**, or
**capacity units**. It describes shared/dedicated as services and states that
Alzette selects and manages the compatible deployment configuration.

## 17. Security, privacy, and operational requirements

- All customer reads/mutations derive scope from the authenticated human
  session; IDs are secondary selectors, never authority.
- Every commercial mutation is CSRF-protected, idempotent, audited, and
  permission checked.
- Quote acceptance and payment initiation require recent reauthentication.
- Stripe webhook signature verification uses the raw bounded body and configured
  tolerance; invalid signatures have no DB effect.
- Event handling tolerates duplicates and out-of-order delivery.
- The billing adapter uses explicit timeouts, bounded retries, Stripe request
  IDs, and operation idempotency keys.
- Stripe/browser return URLs use fixed server configuration and opaque IDs; no
  secret or arbitrary redirect is accepted.
- Customer-visible APIs redact Stripe Customer/subscription/payment object IDs
  unless a safe support reference is specifically required.
- Billing PII retention, invoice retention, audit retention, and deletion
  exceptions are policy/contract fields, not undocumented constants.
- Backup/restore includes catalogue, quotes, payment requirements, event
  receipts, subscriptions/invoices, deployments, and capacity revisions.
- Payment or worker failure cannot corrupt route state. Reconciliation is
  restart-safe.
- No payment workflow is advertised live before HTTPS ingress, Stripe live-mode
  configuration, webhook delivery evidence, tax review, and restore testing.

## 18. Verification plan

### Unit and HTTP contract

- eligibility filtering and price visibility;
- model/profile/unit validation;
- `workload.expected_user_count` accepts only JSON integers from `1` through
  `10000`, while omission remains valid for legacy API clients;
- revised-UI creation sends the people count and none of the legacy technical
  workload members;
- legacy payloads continue to round-trip, and updating a legacy draft through
  the revised UI preserves omitted legacy values;
- the people count participates in idempotency comparison and submitted-request
  immutability; concurrency is never migrated or presented as people count;
- the revised UI exposes no context control and sends no context value; the
  reviewed default remains server-owned and separate from the model's maximum
  context-window capability;
- exact customer payload allow-list;
- quote expiry/supersede/immutability;
- role, CSRF, and recent-reauth enforcement;
- idempotent configuration, acceptance, checkout, and capacity requests;
- no arbitrary Stripe amount/currency/Price/Customer/return URL;
- safe error/redaction behavior.

### Stripe sandbox and deterministic fixtures

- Checkout success, cancellation, and expiry;
- browser return before webhook;
- webhook without browser return;
- invalid signature/raw-body mutation;
- duplicate event ID and duplicate object event;
- out-of-order subscription/invoice events;
- delayed/missing event reconciled by current-object fetch;
- initial invoice paid and failed;
- renewal paid and failed;
- subscription cancellation at period end;
- refund/dispute state;
- API timeout/retry with one Stripe object via idempotency key;
- amount/currency/quote mismatch fails closed;
- event payload and secrets absent from logs/audit.

### Database and tenant isolation

- migration up/down/reapply and rollback;
- additive nullable people-count columns preserve existing rows and enforce
  `1..10000` when present;
- historical workload fields remain unchanged, and submitted people counts
  cannot be mutated;
- two tenants cannot enumerate or mutate catalogue eligibility, drafts, quotes,
  payment requirements, invoices, endpoints, or capacity revisions;
- one Stripe Customer maps to one organisation;
- accepted quotes and paid snapshots are immutable;
- exactly one active capacity revision;
- dedicated target exclusivity and shared target allow-list remain enforced;
- duplicate payment cannot create a second route/allowance/deployment.

### End-to-end

1. Evaluation admin activates the configured free shared offer and makes a real
   first request.
2. Approved admin completes Stripe sandbox Checkout for a paid shared offer;
   verified webhook activates exactly one endpoint; usage appears in the
   existing dashboard.
3. Approved admin configures a dedicated endpoint, receives/accepts a quote,
   pays, and sees `payment received / awaiting allocation` rather than `ready`.
4. Operator fulfils against a deterministic private-compatible target; the
   customer endpoint becomes ready only after validation and binding.
5. Capacity increase creates a new active revision while URL, alias, key, model,
   service mode, and execution class remain unchanged.
6. A second tenant fails every cross-scope ID and Stripe-reference attempt.

### Browser/accessibility

- keyboard-complete catalogue, configurator, quote acceptance, and return flow;
- Step 4 exposes exactly one editable, required, correctly labelled people-count
  field with the approved helper and validation wording;
- values `1` and `10000` pass; missing, fractional, zero, negative, and
  over-limit values block progression and submission;
- refresh/back restores the people count, legacy requests show **Not recorded**,
  and no use-case, context, concurrency, volume, or workload-priority control
  appears in endpoint creation;
- the post-creation capacity-increase dialog and request behavior remain
  unchanged;
- visible focus and meaningful headings/status regions;
- no colour-only status;
- 390px, 1024px, and 1440px layouts;
- refresh/back/retry preserve safe draft and payment state;
- Stripe redirect/return clearly leaves and re-enters Alzette;
- screen-reader language distinguishes payment, provisioning, route, and
  serving state.

## 19. Delivery plan

### Slice E0 — read-only provider surface

- Implement catalogue/store/control read APIs over migration `0008`.
- Add Models, Endpoints list, model detail, and endpoint detail/progress shells.
- Seed only an operator-reviewed current shared profile; other entries remain
  draft/request-review until evidence exists.

**Exit:** a customer can discover exactly what is eligible and understand why
anything else is not activatable. No payment or readiness fiction exists.

### Slice E1 — endpoint intent and quotes

- Implement drafts, submission, qualification link, quote issuance, view,
  expiry, acceptance, and operator workflows.
- Add dedicated configurator and capacity-change flow.

**Exit:** a customer can submit and accept one immutable dedicated quote without
choosing raw infrastructure; payment/allocation/readiness remain pending.

### Slice E2 — Stripe sandbox and paid shared endpoint

- Add billing migration/modules, webhook process, reconciliation worker,
  Checkout, Customer Portal, and fixed shared subscription.
- Keep free evaluation paymentless.

**Exit:** duplicate-safe sandbox payment activates exactly one allowed shared
endpoint, and failures/renewals reconcile correctly.

### Slice E3 — dedicated payment and deterministic fulfilment

- Tie accepted quote to setup/first-invoice requirement.
- Fulfil against a deterministic private-compatible target.
- Verify payment/provisioning/route separation and capacity revision.

**Exit:** paid does not mean ready; operator validation creates one exclusive
ready route; expansion preserves the endpoint contract.

### Slice E4 — production gates

- HTTPS ingress, Stripe live-mode keys/webhooks, tax/invoice review,
  backup/restore, alerts/runbooks, production security review, and one real
  payment/refund rehearsal.
- Replace deterministic dedicated fulfilment with MeluXina only after its
  allocation/network/runtime evidence passes.

**Exit:** finance, security, operations, and product sign off on one live
commercial flow. Until then all payments remain sandbox-labelled.

## 20. Team invitations seam

Inviting people is intentionally not implemented by this PRD, but the following
requirements prevent the endpoint feature from blocking it:

1. All commercial/runtime resources belong to `organisation_id`, never
   `human_user_id`.
2. Human IDs appear only as actor/approver attribution.
3. Permission checks use roles/capabilities rather than “the user who paid.”
4. The internal Stripe Customer object and billing identity are
   organisation-scoped. The client never needs a Stripe account.
5. Last-admin protection remains required before broad rollout.
6. A future finance role can view/pay invoices without receiving target,
   credential, or membership authority.
7. Invitation acceptance must not recreate or transfer endpoint/billing state.

The first-client PoC may operate with one operator-provisioned `org_admin`.
General self-service release is blocked until the invitation/member-management
contract in [`ACCOUNT_ONBOARDING_PRD.md`](ACCOUNT_ONBOARDING_PRD.md) and the
invited-employee identity/agent-access contract in
[`WORKFORCE_AGENT_ACCESS_PRD.md`](WORKFORCE_AGENT_ACCESS_PRD.md) are shipped, or
an explicit single-admin pilot exception is documented. Employee offboarding
revokes short human-agent grants; it does not silently revoke independent
organisation-owned service-account keys or transfer endpoint/billing ownership.

## 21. Open decisions before live payment

1. Exact first paid shared model/profile, fixed allowance, period, and price.
2. Exact first dedicated capacity unit, price, setup charge, term, and evidence.
3. Which actors can accept terms/pay before a separate finance role exists.
4. Card, SEPA Direct Debit, bank transfer, or invoice terms supported per offer.
5. Tax registration, Stripe Tax use, tax codes, VAT validation, and invoice
   wording reviewed by finance/legal.
6. Grace, suspension, cancellation, refund, dispute, and data-retention policy.
7. Whether dedicated payment is setup fee, first recurring invoice, deposit, or
   approved invoice terms.
8. Which billing fields developers/viewers may see.
9. Stripe Customer Portal configuration for shared versus dedicated customers.
10. Support/notification channel for payment and fulfilment events before team
    invitations and transactional email are complete.

These are launch gates, not fields for a builder to invent.

## 22. Reference sources

Stripe is an adapter, not the source of Alzette product truth. The initial
integration is based on current official Stripe documentation:

- [Checkout Sessions](https://docs.stripe.com/api/checkout/sessions)
- [Build subscriptions with Checkout](https://docs.stripe.com/payments/checkout/build-subscriptions)
- [Subscription webhooks](https://docs.stripe.com/billing/subscriptions/webhooks)
- [Webhook signatures, duplicates, and processing](https://docs.stripe.com/webhooks)
- [Idempotent API requests](https://docs.stripe.com/api/idempotent_requests)
- [Stripe Customer Portal](https://docs.stripe.com/customer-management)
- [Stripe Quotes](https://docs.stripe.com/quotes)
- [Stripe Invoicing](https://docs.stripe.com/no-code/invoices)
- [Checkout tax-ID collection](https://docs.stripe.com/tax/checkout/tax-ids)
- [Automatic tax in Checkout](https://docs.stripe.com/tax/checkout)
- [Usage meters](https://docs.stripe.com/billing/subscriptions/usage-based/meters/configure) — explicitly deferred for P0

Provider documentation is evidence for adapter behavior only. Alzette's
database, audit, contracts, capacity validation, and route invariants remain
authoritative for the endpoint product.
