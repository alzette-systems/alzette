# Alzette customer account onboarding PRD

**Status:** proposed implementation contract; not yet implemented

**Date:** 2026-08-15

**Owners:** product, platform, and security

**Related documents:** [`PORTAL_PRD.md`](PORTAL_PRD.md) defines the complete
portal product; [`POC_BOUNDARY.md`](POC_BOUNDARY.md) remains the controlling
description of what the current PoC actually exposes. This document makes the
identity, evaluation-account, and team-onboarding workflows in sections E1 and
E2 of the portal PRD implementable. [`ENDPOINTS_PRD.md`](ENDPOINTS_PRD.md)
defines the catalogue, quote, payment, deployment-request, and endpoint-capacity
lifecycle. Both features share the same organisation boundary; endpoint
ownership never depends on the individual who signed up, accepted a quote, or
paid. [`WORKFORCE_AGENT_ACCESS_PRD.md`](WORKFORCE_AGENT_ACCESS_PRD.md) controls
new external human authentication and invited-employee inference access.

## 2026-08-15 identity decision

New external self-service and invited users authenticate through self-hosted
Casdoor. Casdoor owns their password/passkey, MFA, recovery, OAuth/OIDC, and
upstream federation; Alzette owns the invitation, user link, membership, role,
tenant context, and route authority. Existing local password users remain a
bounded migration path.

Where this document says that a new external person chooses, stores, confirms,
or resets an Alzette-managed local password, that implementation detail is
superseded by [`WORKFORCE_AGENT_ACCESS_PRD.md`](WORKFORCE_AGENT_ACCESS_PRD.md).
The invitation, evaluation-organisation, role, mail, abuse, qualification, and
membership state requirements here remain controlling. A portal/OIDC session
still never authenticates inference directly: interactive employees receive
automated short-lived human-agent access, while applications and unattended
workloads use separate service-account keys.

## 1. Decision

Alzette will use **hybrid self-service B2B onboarding**.

An external person can verify a business email, create or authenticate their
Casdoor identity, and enter an isolated evaluation organisation without payment
or operator approval. That organisation receives only the explicitly shared,
hard-capped evaluation offer configured by Alzette. The user can inspect the
real portal and curated catalogue, connect an interactive agent through
short-lived human access or create a separately scoped application key for a
workload, make a bounded first request, and accumulate their own truthful
evaluation usage. No sample customer data is presented as live evidence.

The same identity can then submit business and workload details, select an
eligible model/deployment profile, and request a dedicated private endpoint.
Alzette verifies business authority, supplies a versioned price/capacity quote,
and approves physical capacity before the organisation becomes a production
customer. Dedicated activation is not a side effect of signup or email
verification.

The lifecycle events must remain distinct:

1. A person starts signup and verifies control of a business mailbox.
2. The person creates or authenticates the matching Casdoor identity.
3. One Alzette transaction links that identity and creates an isolated
   evaluation organisation, development project, hard-capped shared service
   plan, allow-listed route, and human membership.
4. The person connects an interactive agent with short-lived human access or
   explicitly creates a separate one-time-reveal application key for a
   workload, then explores the curated catalogue and may prepare a deployment
   configuration or request business qualification.
5. Alzette approves the business and a versioned quote before allocating or
   deploying dedicated infrastructure.
6. An authorised organisation administrator invites colleagues, who create or
   authenticate their own Casdoor identities.

A human login is never an inference key. Email verification proves control of
a mailbox, not authority to represent a business. An evaluation organisation
is not a verified legal customer. A catalogue entry, deployment request, quote,
or payment record is not a running target. Public registration cannot choose an
upstream URL, claim another organisation, or activate dedicated compute.

## 2. Why this is needed

The current implementation can provision a human user only through the
operator CLI. The operator either supplies a password file or receives a
generated password and must transfer it to the user. There is no email identity,
invitation acceptance, password recovery, member-management workflow, or public
access request.

That is adequate for a closed LAN demonstration but not for onboarding a real
financial-services client. It creates avoidable password-handling work, makes
account ownership ambiguous, and leaves the product workflow described by the
main portal PRD incomplete.

This work should produce the following outcomes:

- a qualified prospect can reach a real first shared inference request without
  a sales conversation, payment, or operator-issued password;
- every evaluation organisation is isolated, explicitly shared, hard-capped,
  and distinguishable from an approved customer organisation;
- the first authorised customer administrator creates or authenticates their
  Casdoor identity without Alzette handling a password;
- a customer administrator can add and remove colleagues within their own
  authority;
- a person can use Casdoor recovery without turning an inference credential
  into a login credential; legacy local-account recovery remains isolated;
- a prospect can browse a curated catalogue and submit a non-binding dedicated
  deployment configuration without selecting a raw machine or target address;
- business approval and quote acceptance can promote the same identity and
  organisation without copying usage or weakening tenant boundaries;
- every signup, verification, evaluation provisioning, approval, invitation,
  acceptance, role assignment, recovery, and revocation is attributable and
  tenant-safe.

## 3. Current implementation baseline

The design extends the current stack rather than replacing it.

Already implemented:

- one Go codebase and image with `gateway`, `control`, `public`, and `worker`
  process modes;
- PostgreSQL-backed `human_users`, `human_memberships`, and
  `portal_sessions`;
- bcrypt password hashes, SHA-256 session-token digests, revocable server-side
  sessions, HttpOnly cookies, SameSite policy, and CSRF protection;
- generic login failure responses and session revocation when a user is
  disabled or their password changes;
- operator provisioning of organisation, project, environment, service plan,
  target, route, service account, API key, human user, and membership;
- organisation/project/environment membership switching;
- administrator management of non-human service accounts and scoped,
  one-time-reveal application keys.

Not implemented:

- email as a verified human identity;
- invitations or invitation acceptance;
- customer-visible member management;
- forgot/reset-password;
- public signup, business-email verification, or automatic evaluation-tenant
  provisioning;
- a customer-facing catalogue, quote, or deployment-request API;
- login/recovery/signup throttling;
- transactional email;
- MFA, OIDC, SAML, or SCIM;
- TLS ingress suitable for an Internet-facing account surface.

Until this PRD is implemented, `/login` must not claim that a new user can
register, receive an evaluation route, or recover access. `alzette user
provision` remains the only supported human-user creation path. Additive schema
objects alone do not make signup, shared credits, catalogue availability,
pricing, or dedicated capacity operational.

## 4. Product boundary and priorities

### P0 — verified self-service evaluation

P0 removes the sales and operator-password gate from product evaluation while
preserving the infrastructure boundary:

- a person submits a business email, display name, organisation name, and
  privacy/acceptable-use acknowledgement;
- verification is single-use, expiring, non-enumerating, and required before
  creating the human user or evaluation organisation;
- the user creates or authenticates their Casdoor identity and atomically
  receives an `org_admin` membership in one new organisation marked
  `evaluation`;
- the system creates one development project/environment and binds only the
  configured shared evaluation plan, model alias, and target;
- the shared plan has restart-safe request/token/rate/concurrency limits and a
  hard exhaustion state; signup cannot override them;
- the user can inspect the catalogue and real portal, connect an interactive
  agent through short-lived human access or create a scoped one-time-reveal
  application key for a workload, make a first call, and see their own usage;
- Casdoor recovery is available for new external users; legacy local-account
  recovery remains available during migration;
- all paths are throttled, audited, tenant-isolated, and safe under retries,
  duplicate signup, and concurrent verification;
- an organisation administrator can invite teammates only within the
  evaluation organisation and the launch role ceiling.

P0 does **not** activate dedicated compute, accept payment, prove business
authority, select an upstream target, or let a user alter the shared evaluation
allowance. When no safe shared evaluation target is configured, signup fails
closed before organisation creation rather than creating a misleading account.

### P1 — business qualification and dedicated conversion

P1 converts an evaluation organisation into an approved customer boundary
without creating a second identity or copying tenant data:

- the administrator submits bounded legal/business and workload information;
- the organisation can select an eligible curated model and deployment profile
  and request a versioned price/capacity quote;
- an Alzette operator reviews authority, fit, model licence, capacity supply,
  execution location, and contractual requirements;
- approval changes the organisation lifecycle but does not itself activate a
  target or charge the customer;
- the customer explicitly accepts a still-valid quote before any commercial or
  infrastructure allocation step;
- physical allocation, deployment, validation, and route binding remain
  operator-owned until the MeluXina automation gate passes;
- the applicant can withdraw a request, reject a quote, or continue using the
  bounded evaluation route under its retention policy;
- invitation, team membership, recovery, privacy, and audit requirements remain
  identical after conversion.

P1 requires transactional email and reviewed Internet ingress. It is a real
self-service acquisition path, but not anonymous compute or automatic dedicated
capacity.

### Production identity gate

New external users use the Casdoor identity/OAuth contract in
[`WORKFORCE_AGENT_ACCESS_PRD.md`](WORKFORCE_AGENT_ACCESS_PRD.md). Before remote
production use by a financial-sector client, Alzette must additionally enforce
the MFA or customer OIDC/SAML federation method required by the signed pilot.
Federation changes authentication, not Alzette's organisation/project/
environment authorisation model. SCIM/JIT is added only when a customer
lifecycle requirement justifies it.

### Non-goals

- consumer signup or social login;
- email-domain auto-join;
- anonymous creation of organisations, routes, targets, model deployments, or
  inference keys;
- customer-controlled upstream URLs or provider credentials;
- an uncapped shared trial or a customer-controlled evaluation allowance;
- presenting an evaluation organisation as a verified company account;
- customer selection of a raw machine, accelerator host, LAN address, runtime
  image, or provider model slug;
- automatic dedicated allocation or a commercial commitment from signup alone;
- arbitrary model uploads, fine-tuning, or marketplace publication;
- implementing SAML parsing or an identity provider from scratch;
- support impersonation;
- using an invitation, password-reset token, or portal session as a data-plane
  bearer key;
- adding Redis, Kafka, Kubernetes, a workflow engine, or a second application
  database for onboarding.

## 5. Actors and authority

| Actor | Can do | Cannot do |
|---|---|---|
| Unverified visitor | Submit one bounded signup and complete mailbox verification | Sign in, reserve a company name, consume inference, or create any tenant resource before verification |
| Evaluation admin | Administer their isolated evaluation organisation; browse the catalogue; create a scoped evaluation key; submit qualification/deployment intent; invite teammates within policy | Claim verified-company status, change the shared allowance, select a target/raw machine, accept another organisation's quote, or activate dedicated compute |
| Alzette operator | Configure the shared evaluation offer; review business authority and deployment requests; issue quotes; approve conversion; provision dedicated capacity | Learn any user's identity-provider password; silently accept a quote/contract for the customer; expose provider or infrastructure credentials |
| Organisation admin | List members and pending invitations in their organisation; invite any launch role; revoke an invitation; disable an authorised membership subject to last-admin protection | Act in another organisation; select an upstream target; grant a role outside the launch role set |
| Project admin | List project members; invite `developer` or `viewer` into a project/environment they administer | Grant `org_admin` or `project_admin`; invite into another project; change an organisation role |
| Developer/viewer | Accept their own invitation and manage their own login/recovery | Invite users, change roles, or infer membership outside their authorised context |
| Existing user | Accept another membership after authenticating as the exact invited email identity | Use an invitation addressed to a different email; create a duplicate identity to bypass policy |
| Application/service account | Authenticate inference calls with a scoped API key | Sign in to the human portal, accept an invitation, or manage members |

The current database has no separate `org_owner` role. For P0, the first
authorised owner is represented as `org_admin`. Last-admin protection prevents
an organisation from disabling or demoting its only enabled `org_admin`
without an operator-assisted transfer. A distinct owner role may be added only
if a real governance requirement appears.

## 6. Lifecycle state machines

### 6.1 Self-service registration

```text
pending_verification ──verified POST──> verified ──atomic setup──> completed
        ├──expiry──> expired                ├──policy block──> blocked
        └──superseded──> superseded         └──failure/retry──> verified
```

`completed` means one verified human user, one evaluation organisation, one
default development scope, and one exact membership exist. The registration
does not contain a password, API key, target secret, raw provider selection, or
dedicated-capacity promise. Setup is idempotent and all-or-nothing.

### 6.2 Organisation lifecycle

```text
evaluation ──submit qualification──> qualification_pending
     │                                      ├──approve──> approved
     │                                      │                └──activate contract──> active
     │                                      ├──reject──> evaluation
     │                                      └──withdraw──> evaluation
     ├──policy suspend──> suspended
     └──close/retention──> closed

approved ──contract expiry/cancel──> suspended or closed
active   ──contract expiry/cancel──> suspended or closed
```

Organisation lifecycle and runtime lifecycle are separate. `approved` or
`active` never means a model deployment is ready. An evaluation organisation
cannot be promoted merely because its email domain matches a known customer.

### 6.3 Invitation

```text
pending_delivery -> pending -> accepted
                         ├──> revoked
                         ├──> expired
                         └──resend/rotate──> pending with a new token digest
```

Only `accepted` creates or enables the specified membership. Delivery failure
does not activate anything. Revocation and expiry are terminal; resend rotates
the credential and remains auditable.

### 6.4 Human access

```text
self_service_verified -> active -> membership_disabled
invited --------------> active -> user_disabled
```

An invitation is stored independently of `human_users`. An invited user record
is created only during successful acceptance. A self-service user is created
only during the atomic evaluation setup after email verification and receives a
real evaluation membership in that transaction, preserving the current portal
invariant that an authenticated session always has a membership context.

An existing verified user can hold multiple memberships. Disabling one
membership removes only that scope; disabling the user revokes every portal
session and prevents all login.

## 7. Required workflows

### 7.1 Create and activate an evaluation account

1. The public site links to the control service's canonical `/signup` route.
   The public process remains static and database-free.
2. The form collects a business email, display name, proposed organisation
   name, privacy-notice acceptance, and acceptable-use acceptance. It does not
   collect a password, prompt, model file, provider credential, payment detail,
   or raw infrastructure preference at this stage.
3. The POST returns the same neutral response for new, existing, blocked, and
   rate-limited identities. A transactional message carries a single-use
   verification URL whose digest—not plaintext—is stored.
4. The first valid GET establishes a short-lived setup session and redirects to
   a clean URL without accepting the account. A user-initiated action confirms
   the display/organisation names and completes Casdoor authentication under
   the workforce-agent identity contract.
5. One PostgreSQL transaction locks the registration, rechecks state and the
   configured evaluation offer, then creates the user, evaluation organisation,
   development project/environment, `org_admin` membership, organisation-bound
   shared service plan, and allow-listed route to the preconfigured shared
   target. Every ID and policy value is server-derived.
6. A portal session is issued only after commit. The first screen labels the
   organisation `Evaluation`, the route `Shared evaluation`, the execution
   class actually in use, and the exact remaining request/token allowance.
7. The user explicitly connects an interactive agent through short-lived human
   access or creates a service account and one-time-reveal application key for
   a workload, follows a real first-call example, and sees the resulting
   logical request and usage appear in the portal.

Retries return the same completed organisation to the same verified identity;
they never create duplicate organisations, memberships, plans, routes, or free
allowances. A single verified identity may own only the configured number of
active evaluation organisations. The same email domain never joins or reveals
another organisation automatically.

### 7.2 Qualify the business and configure a dedicated endpoint

1. An evaluation administrator opens Catalogue and selects an available model
   release. The catalogue shows capabilities, licence/support state, context
   limit, lifecycle, and eligible deployment modes; it does not imply that
   capacity is allocated.
2. The administrator selects `Dedicated private` and supplies workload intent:
   environment, expected concurrency, request rate, context size, latency
   priority, and desired capacity units. Customer data or prompts are not
   required.
3. Alzette recommends compatible deployment profiles. Each versioned profile
   is a validated model/runtime/hardware bundle and states accelerators per
   capacity unit, min/max units, capacity metrics with finality/evidence, region
   eligibility, and price availability. Raw machines, LAN addresses, images,
   provider slugs, and secrets remain operator-owned.
4. Submission creates a non-binding deployment request. Business authority,
   model licence, infrastructure supply, data boundary, and support terms are
   reviewed before a quote can become contractual.
5. Alzette issues a versioned quote that snapshots model/profile, units,
   accelerator class/count, capacity metrics, recurring and one-time price,
   currency, billing period, execution boundary, expiry, and evidence status.
6. An authorised customer explicitly accepts the valid quote after
   reauthentication. Acceptance records intent; payment, allocation,
   deployment, and route readiness remain separate states.
7. The operator assigns infrastructure, deploys and validates the pinned model
   runtime, registers a dedicated target owned by the organisation, and binds
   the stable tenant route. Only runtime evidence can change the endpoint to
   `Ready`.
8. To expand capacity, the customer selects the existing endpoint and a larger
   unit count. A new delta/full quote and capacity-change request go through the
   same approval and validation path. Successful expansion preserves the
   endpoint URL, alias, key contract, model, tenancy mode, and execution class.

Shared evaluation uses a pre-existing shared deployment and can activate
automatically within its configured hard limits. Dedicated private deployment
is customer-configured but operator-fulfilled until MeluXina allocation and
deployment automation have passed their own release gate.

### 7.3 First administrator of an operator-created business

The invitation path remains available for an already approved organisation or
a customer that must not use public signup:

1. The operator provisions or confirms the organisation and exact membership
   scope, then creates a first-administrator invitation.
2. The invitee follows a single-use link that establishes a clean setup
   session and creates or authenticates their Casdoor identity.
3. One transaction creates or resolves the user, creates the exact membership,
   marks the invitation accepted, writes audit events, and invalidates every
   setup session for the invitation.
4. The user enters the portal in the invited context. No password, application
   key, target address, provider name, or capacity claim is sent by the
   operator.

Legacy username reconciliation, replay, expiry, revoke, resend, and concurrent
acceptance retain the existing fail-closed invitation rules.

### 7.4 Invite a colleague

1. An authorised admin opens Access → Members and selects `Invite member`.
2. The admin enters an email, optional display name, role, project, and
   environment. The UI explains the effective permissions before confirmation.
3. The server derives the inviter's organisation and allowed scopes from the
   current session. It ignores no client-supplied tenant authority and rejects
   elevation beyond the inviter's policy.
4. If the person is already a member, the server returns an explicit
   authenticated-admin result and does not create an invitation. If an active
   invitation exists for the same email and scope, the operation is idempotent.
5. Delivery uses email when configured. In manual mode, an acceptance URL is
   revealed once to the inviting admin for out-of-band delivery.
6. The invitee follows the same acceptance path as the first administrator.
7. The inviter can see `delivery pending`, `invited`, `accepted`, `expired`, or
   `revoked`, resend with a rotated token, or revoke a pending invitation.

An email domain match never grants membership automatically.

### 7.5 Recover a local account

This workflow applies only to legacy users whose local password authentication
remains enabled. Casdoor owns password/passkey/MFA recovery for new external
and linked federated users; a recovery in either system never creates an
inference credential.

1. `/forgot-password` accepts an email or legacy username and always returns a
   generic response with comparable timing.
2. Existing eligible accounts receive a single-use reset URL. Requests are
   throttled per account and source; a new request invalidates older unused
   reset tokens after a small bounded overlap only if mail delivery requires it.
3. The initial GET exchanges the URL credential for a clean, short-lived setup
   session. The final password change is a CSRF-protected POST.
4. A successful reset updates the password hash and password-changed time,
   consumes the token, revokes all existing portal sessions, writes an audit
   event, and sends a confirmation notice. It does not issue an inference key.
5. The user signs in normally after reset. The reset flow does not silently
   preserve an older authenticated session.

Operator password rotation remains a break-glass command, not the normal
customer workflow.

### 7.6 Remove access

- Revoking a pending invitation invalidates every setup session associated with
  it immediately.
- Disabling a membership removes it from context selection and revokes sessions
  currently using that membership.
- Disabling a human revokes all of their portal sessions.
- Removing a human never revokes a service-account key implicitly. The UI must
  show separately owned application credentials and require an explicit key
  decision so production workloads are not accidentally broken.
- The last enabled organisation admin cannot remove or demote themselves
  without transferring authority or using the operator recovery path.

## 8. UX and content contract

### Public and unauthenticated pages

| Route | Purpose | Required content |
|---|---|---|
| `/login` | Existing-user sign-in | `Continue with Alzette` through Casdoor for external users; bounded legacy username/password fallback; explicit separation from application API keys and human-agent access |
| `/signup` | Begin self-service evaluation | Business email, person/organisation names, privacy and acceptable-use acknowledgement; exact shared-evaluation/no-payment boundary |
| `/signup/verify` | Complete verified setup | Clean-URL setup session, Casdoor identity authentication/linking, evaluation-organisation label, expiry/error/retry state |
| `/accept-invite` | New-user setup or existing-user membership acceptance | Inviting company, exact scope/role, expiry, Casdoor sign-in guidance, support path |
| `/forgot-password` | Optional legacy-only recovery, when explicitly enabled | One identifier field and non-enumerating result; new external users are directed to Casdoor recovery |
| `/reset-password` | Optional legacy-only recovery completion | New password, confirmation, expiry/error state, no account metadata beyond what is needed |
| `/qualification` | Submit business qualification for the current evaluation organisation | Business/workload fields, review and retention expectation, no dedicated-capacity or contract promise |

These pages use the current Alzette visual system and gentle login style. They
must work without client-side JavaScript for the core form action, meet WCAG
2.2 AA, preserve user input on validation errors except passwords, and show a
request/correlation ID for support-safe failures.

### Authenticated Access workspace

The existing Access view gains two separate sections:

- **People:** active members, role, project/environment scope, invitation
  status, last sign-in when authorised, and invite/revoke actions.
- **Application access:** the current service-account and one-time API-key
  management UI.

The separation must be visible in navigation and copy. Use “Invite a person”
and “Create application key”; never use the ambiguous label “Create
credentials” for both.

Every authenticated surface in an evaluation organisation displays a persistent
`Evaluation · Shared` context label and the hard-cap state. Catalogue and
deployment-request views distinguish `Available to configure`, `Quoted`,
`Provisioning`, and `Running`; none uses `Available` as a synonym for deployed.

### Email and link copy

- Subject and first line identify Alzette and the inviting organisation.
- Invitation mail states the exact role/scope, expiry, and who initiated it.
- Recovery mail never includes account scope, customer consumption, provider,
  model, target, or application-key information.
- Emails contain no prompt/output content and no tracking pixel.
- Links use only the configured canonical HTTPS control origin; they are never
  constructed from an untrusted request `Host` header.
- Expired/invalid links disclose no account existence and provide a safe
  resend/support route.

## 9. Functional requirements

### P0 requirements

| ID | Requirement | Acceptance |
|---|---|---|
| ONB-P0-001 | A visitor can submit and verify a business-email signup without enumeration | New/existing/blocked responses are neutral; verification tokens are random, expiring, single-use, and digest-only |
| ONB-P0-002 | Verified setup atomically creates one human, evaluation organisation, development scope, and exact membership | Failure or replay creates no partial/duplicate tenant resources or additional allowance |
| ONB-P0-003 | Evaluation provisioning uses only the operator-configured shared offer and target | Signup fields cannot alter model slug, target, route address, limits, execution class, plan, or price; missing/unsafe offer fails closed |
| ONB-P0-004 | Evaluation access is visibly shared and hard-capped | Gateway enforcement survives restart; exhaustion blocks before a provider attempt and the portal shows remaining/final usage honestly |
| ONB-P0-005 | A verified evaluation admin can connect an interactive agent with short-lived human access or create a separately scoped application key for a workload and make a real first request | Human login/session never authenticates inference directly; the exact actor kind appears once in the logical-request ledger and current organisation usage |
| ONB-P0-006 | Catalogue browsing and configuration do not claim runtime availability | Catalogue, quote, deployment, target, and route states are rendered separately and contain no fabricated capacity or price |
| ONB-P0-007 | Operator-created first-admin and teammate invitations remain supported | Exact scope/role acceptance is atomic; replay, expiry, revoke, cross-email, and privilege escalation fail closed |
| ONB-P0-008 | Authorised admins can list, invite, resend, revoke, and disable within their authority | Two-tenant, role-ceiling, and last-admin tests cover every action |
| ONB-P0-009 | New external users use Casdoor recovery while remaining local users retain a bounded, explicitly legacy recovery path | Neither path accepts or issues an inference credential; applicable Alzette sessions are revoked or revalidated after identity recovery |
| ONB-P0-010 | Login, signup, verification, invitation, identity callback, and any enabled legacy-recovery paths are restart-safe throttled | Account, source, organisation-name, and free-allocation abuse is bounded across control-process restart |
| ONB-P0-011 | Setup and portal sessions are separate and rotated after authentication or privilege change | URL token is removed before form entry; session-fixation and scanner-GET tests pass |
| ONB-P0-012 | Human identity credentials, short agent access, application keys, action tokens, and provider credentials remain separate | No endpoint accepts one in place of another; storage, API, audit, and copy tests prove separation |
| ONB-P0-013 | Identity, evaluation-provisioning, invitation, and recovery actions are append-only audited without secrets | Events contain actor/scope/action/result/correlation ID and safe resource IDs only |
| ONB-P0-014 | Existing operator-provisioned users continue to sign in during migration | Username login and bcrypt verification remain compatible until controlled retirement |
| ONB-P0-015 | External exposure fails closed without production security configuration | Missing canonical HTTPS origin, secure cookies, supported toolchain, mail, throttle key, or configured evaluation offer prevents Internet signup |

### P1 requirements

| ID | Requirement | Acceptance |
|---|---|---|
| ONB-P1-001 | An evaluation admin can submit business qualification and dedicated deployment intent for their own organisation | Submission grants no new runtime authority; operator review records evidence and cannot target another organisation |
| ONB-P1-002 | Approval promotes the existing organisation lifecycle without duplicating identity, usage, or resources | Tenant ID remains stable; approved/customer states still do not imply a ready deployment |
| ONB-P1-003 | A versioned price/capacity quote requires explicit authorised acceptance | Expired/superseded/cross-tenant quotes fail; acceptance never directly creates a target, binding, charge, or readiness state |
| ONB-P1-004 | Transactional mail delivery is retryable and observable without storing plaintext action tokens | Delivery survives worker restart; uncertain retries rotate credentials safely; customer-safe status is visible |
| ONB-P1-005 | Applicant can withdraw and unneeded signup/qualification PII expires under policy | Retention worker and deletion tests preserve only required audit/legal records and never delete usage belonging to an active organisation |
| ONB-P1-006 | Local-account MFA or the first customer's OIDC/SAML method gates production access and quote acceptance | Authentication, recovery, step-up, role mapping, and deprovisioning are tested end to end |

## 10. Technical architecture

### 10.1 Component boundary

```text
public site ──link only──> control/signup/auth ──transactions──> PostgreSQL
                                  │                                ▲
                                  ├──evaluation provision──────────┤
                                  └──delivery request──> worker ───┘
                                                          │
                                                          └──mail adapter

customer application ──service-account key──> gateway ──existing route/ledger──> target
interactive employee ──short human token────> gateway ──same route/ledger──────> target

customer portal ──catalogue/configuration──> quote/request ──operator fulfilment
                                                               │
                                                               └──existing target/route
```

- `alzette control` owns public account forms, authenticated member APIs,
  setup/session cookies, authorisation, and operator onboarding commands.
- PostgreSQL is the authoritative state and concurrency boundary.
- `alzette worker` claims delivery jobs, generates action credentials just in
  time, invokes a mail adapter, records safe delivery outcome, expires records,
  and prunes throttle buckets.
- `alzette public` remains static and database-free. Its only account action is
  a configured link to the canonical control-service signup route.
- Existing `/v1/chat/completions` route and target resolution remain unchanged.
  The workforce-agent contract adds a distinct short-lived human-token
  authenticator that checks current identity/user/membership/grant state on
  every call; existing scoped application-key behavior remains authoritative
  for service accounts.
- Evaluation provisioning calls one server-owned transactional operation that
  copies an enabled evaluation-offer template into an isolated tenant plan and
  route. It never accepts a target, model, allowance, or price from the browser.
- Catalogue/configuration and physical fulfilment are separate domains. A
  deployment request may eventually produce a normal target and route binding;
  it never bypasses the current routing invariants.

This remains one image, one PostgreSQL database, and Docker Compose on one
machine. These are code/domain boundaries, not new microservices.

Mail delivery, rollup reconciliation, and target probes need separate worker
checkpoints and failure handling. A mail-provider outage may defer mail but
must not terminate usage reconciliation or make inference health appear green;
one failed job is retried or dead-lettered without killing the worker process.

### 10.2 Go package layout

Recommended additions:

```text
internal/onboarding/                 domain validation, tokens, state policy
internal/catalogue/                  model/profile/quote/request policy
internal/mailer/                     small provider-neutral delivery interface
internal/store/postgres/onboarding.go transactional persistence
internal/store/postgres/catalogue.go  catalogue and deployment-request persistence
internal/portal/                     unauthenticated forms + member APIs
internal/worker/                     delivery/expiry/throttle maintenance
cmd/alzette/                          invite, offer, qualification, quote operator commands
```

Do not place mail-provider code, token policy, or SQL in the browser. Do not
create a custom Alzette identity protocol/service for P0; the separately
deployed, pinned Casdoor instance owns human authentication while Alzette owns
onboarding and membership state.

### 10.3 Database migration

Migration `0008_self_service_catalogue` is the additive domain foundation. It
must preserve every existing organisation, user, membership, route, target,
request, attempt, and service plan. Applying it does not enable signup or
publish an offer; no evaluation account can be created until the control,
mailer, throttle, gateway-limit, and operator-configuration gates are complete.

Repository reality now controls the numbering: `0008` has already supplied
email/self-service/catalogue groundwork but did not create invitation tables or
make federated password-less users possible. The invitation, federated
identity, human-agent grant/token, and request-actor changes are planned in the
next additive migration after `0010`, as defined by
[`WORKFORCE_AGENT_ACCESS_PRD.md`](WORKFORCE_AGENT_ACCESS_PRD.md). The detailed
`human_invitations` and action-session shapes below remain requirements for
that future migration; they are not evidence that `0008` implemented them.

#### Changes to `organisations`

- add `account_kind` (`evaluation` or `customer`), lifecycle status, and
  creation source;
- mark existing rows as operator-created active customers without fabricating
  a business-approval record;
- add optional business approval time/evidence for conversion from
  self-service evaluation to customer;
- require evaluation and customer lifecycle combinations to remain valid;
- preserve the same organisation ID through qualification and conversion.

#### Changes to `human_users`

- add nullable `email` and `email_normalized` during migration;
- add `email_verified_at`;
- add an identity origin that distinguishes legacy/operator and self-service
  creation;
- add a unique index on non-null `email_normalized`;
- keep `username` as a unique legacy/internal identifier;
- permit login by verified email or existing username during transition;
- backfill real emails through an explicit operator command with a verification
  source/evidence reference—never manufacture or infer a customer email;
- make verified email mandatory for users created through self-service or
  invitation.

Store the strictly validated single address, trimmed only at form boundaries,
as `email` for display/delivery. Store `strings.ToLower(email)` separately as
`email_normalized` for uniqueness and login comparison. Do not apply
provider-specific email transformations such as removing dots or plus tags.
Reject display-name syntax, control characters, and overlong addresses.

#### New `self_service_registrations`

Minimum fields are the normalized business email, proposed person/organisation
names, privacy/acceptable-use versions, digest-only verification credential,
expiry/generation/state, and the resulting user/organisation IDs after atomic
completion. No password or API key is stored in the registration. A partial
unique index permits only one active registration per normalized email, while
completed registrations remain immutable evidence.

#### New `evaluation_offer_templates`

An enabled template binds one published shared-evaluation deployment profile to
one existing shared target/routable model and declares restart-safe request,
token, rate, concurrency, and lifetime limits. It is operator-owned, versioned,
and disabled by default. Signup copies its policy into tenant-scoped plan/route
records; it never accepts these fields from the browser. A database trigger
rejects a dedicated target or non-evaluation deployment profile.

#### Catalogue and deployment-commercial tables

The same migration introduces the catalogue/profile/metric/price, business
qualification, quote, deployment request, deployment, and capacity-revision
tables specified in `PORTAL_PRD.md`. These are intent and commercial records;
the existing inference target and tenant route remain runtime truth.

#### New `human_invitations`

Minimum fields:

- stable ID and status;
- normalized recipient email and optional intended display name;
- organisation, project, environment, and role with composite scope foreign
  keys;
- SHA-256 token digest, token generation number, creation and expiry times;
- inviter type/ID, accepted user ID, accepted/revoked times, and safe reason;
- delivery mode/status and idempotency key.

A partial unique index allows at most one active invitation for an
email/scope/role tuple. Resend increments the generation and replaces the token
digest. Database checks make `accepted`, `revoked`, and `expired` mutually
exclusive terminal states.

#### New `human_action_sessions`

This table stores digest-only, short-lived setup sessions created when an
invitation, verification, or reset URL is opened. It includes action type,
resource ID, token digest, creation/expiry/consumption/revocation times, and no
tenant data in the browser value. Accepting or revoking the underlying action
invalidates all associated setup sessions.

#### Deferred legacy-only `password_reset_requests`

This table is not required for new external users or the initial Casdoor
migration. If Alzette later enables self-service recovery for remaining local
accounts, minimum fields are ID, user ID, SHA-256 token digest, created/expiry/
consumed/revoked times, generation, and safe request metadata. There is no
email or password plaintext in the token row.

#### New `business_qualification_requests` (P1)

Minimum fields are ID, evaluation organisation, submitting user, legal/display
name, title, optional website, bounded workload description, state, review
actor/reason/evidence/time, and created/updated/withdrawn times. Free text is
never forwarded to a model. Approval can change organisation lifecycle in the
same transaction but creates no target, deployment, quote acceptance, payment,
or inference activity.

#### New `mail_deliveries`

Minimum fields are ID, purpose, related resource ID, normalized recipient,
template version, state, attempt count, next-attempt/lease/sent times, provider
message ID if safe, and a bounded safe error class.

The outbox stores **no plaintext invitation/reset/verification token**. A worker
claims a delivery, creates a fresh action token with `crypto/rand`, commits its
digest and generation, then keeps the plaintext only in memory while calling
the mail adapter. A failed or uncertain retry rotates the token before sending
again, so an older delivered link cannot remain silently valid. No email body
or action URL is logged.

#### New `auth_throttle_buckets`

Persist bounded counters/expiry for action type plus HMAC-pseudonymised account
and source-network buckets. Do not retain raw IP addresses indefinitely. The
worker deletes expired buckets. The server applies both per-identity and
per-source limits so an attacker cannot lock one account cheaply or bypass all
limits with identifier rotation.

### 10.4 Token and lifetime defaults

All action tokens use at least 32 random bytes from `crypto/rand`, Base64URL
encoding without padding, SHA-256 digest-only storage, and constant-time
verification after lookup where applicable.

| Credential/state | Default | Rule |
|---|---:|---|
| Invitation URL | 72 hours | Single use; resend/revoke invalidates previous generation |
| Email-verification URL | 24 hours | Single use; no access granted |
| Legacy password-reset URL, if enabled | 30 minutes | Single use; success revokes all portal sessions |
| Clean-URL setup session | 15 minutes | HttpOnly, Secure, SameSite=Strict; bound to one action |
| Portal session | Existing configured TTL, maximum 12 hours for pilot | Server-side expiry/revocation; rotate after authentication/privilege change |
| Unverified signup | 24 hours | Expire if email is never verified; resend rotates the credential |
| Rejected/expired prospect PII | 90 days by default | Shorter/longer only under documented legal/CRM policy |

Lifetimes are configuration with safe upper bounds, not per-request inputs.

### 10.5 Legacy password compatibility guardrail

P0 does not add a new Alzette password path for external users. Existing bcrypt
users remain compatible during the bounded migration, and operator recovery is
the default legacy fallback. If a separately approved self-service legacy
reset is implemented, all of the following rules apply:

- Accept 16–128 Unicode characters, including spaces; do not silently trim.
- Do not impose composition rules or periodic forced changes.
- Screen new passwords against a local blocklist of common/compromised values
  without sending the candidate password to a third party.
- Hash new and changed passwords with versioned Argon2id PHC strings. Benchmark
  on the deployment host. Start at the current OWASP baseline of 19 MiB memory,
  two iterations, one lane, a 16-byte random salt, and 32-byte output; raise the
  work factor when the deployment benchmark permits it and cap concurrent hash
  work. Retain bcrypt verification for existing hashes and rehash after
  successful login.
- Use a dummy hash path and generic response for unknown identities.
- Passwords, reset tokens, and API keys never enter logs, analytics, audit
  metadata, email subject lines, or support tools.

The current bcrypt hashes remain valid during migration. Only an approved
legacy-reset implementation may add Argon2id encodings; Casdoor password hashes
must never be copied into the Alzette database.

### 10.6 HTTP contracts

Unauthenticated HTML routes:

```text
GET  /login
POST /login
GET  /signup
POST /signup
GET  /signup/verify?token=...
POST /signup/verify
GET  /accept-invite?token=...
POST /accept-invite
GET  /qualification                     # authenticated P1 page
POST /qualification                     # authenticated P1 mutation
```

Optional `/forgot-password` and `/reset-password` routes are legacy-only and
remain disabled unless the separately approved compatibility path in section
10.5 is implemented. New external users recover through Casdoor.

Authenticated portal APIs:

```text
GET  /api/portal/members
GET  /api/portal/account-stage
GET  /api/portal/evaluation-allowance
GET  /api/portal/invitations
POST /api/portal/invitations
POST /api/portal/invitations/resend
POST /api/portal/invitations/revoke
POST /api/portal/memberships/role
POST /api/portal/memberships/disable
POST /api/portal/qualification
```

Operator CLI:

```text
alzette user invite
alzette user invitation-resend
alzette user invitation-revoke
alzette user email-backfill
alzette user reset-link                 # break-glass/manual delivery
alzette evaluation-offer configure      # disabled until explicit operator setup
alzette qualification list              # P1
alzette qualification approve           # P1
alzette qualification reject            # P1
alzette qualification expire            # P1
```

All mutations are bounded, reject unknown fields, use CSRF protection when
cookie-authenticated, carry a correlation ID, and are idempotent or reject a
replayed state transition deterministically. Customer-supplied organisation
IDs never establish authority; the server resolves scope from the current
membership and checks any selected project/environment against it.

The bearer-only `/api/v1/*` machine APIs and inference gateway contracts do not
gain signup or human-session authentication.

### 10.7 Email adapter and delivery modes

Use a narrow Go interface such as:

```go
type Mailer interface {
	Send(ctx context.Context, message Message) (safeProviderID string, err error)
}
```

Implement two modes first:

- `manual`: return a plaintext action URL once to the authorised operator or
  inviting admin. This is sufficient for the closed PoC and deterministic
  tests.
- `transactional`: the worker sends through one reviewed HTTPS API or SMTP
  adapter. Secrets are file-backed, redirects are disabled, destinations are
  allow-listed/configured, timeouts are bounded, and errors are mapped to safe
  retry/permanent classes.

Public signup remains disabled in `manual` mode because mailbox verification
cannot be trusted as self-service. Operator invitations may still use manual
delivery. A local fake mailer captures messages in memory for deterministic
integration/browser tests without contacting the Internet or creating provider
cost.

### 10.8 Configuration additions

Recommended configuration:

```text
ALZETTE_PUBLIC_CONTROL_ORIGIN=https://portal.example.lu
ALZETTE_ONBOARDING_ENABLED=false
ALZETTE_PUBLIC_SIGNUP_ENABLED=false
ALZETTE_EVALUATION_OFFER_CODE=
ALZETTE_MAIL_MODE=manual
ALZETTE_MAIL_FROM=
ALZETTE_MAIL_SECRET_FILE=
ALZETTE_AUTH_THROTTLE_HMAC_FILE=
ALZETTE_INVITATION_TTL=72h
ALZETTE_PASSWORD_RESET_TTL=30m
ALZETTE_EMAIL_VERIFICATION_TTL=24h
```

The public origin is one exact configured HTTPS origin. External onboarding
must fail to start if it is absent/invalid, secure cookies are disabled, or the
service is knowingly running on raw public HTTP. Development/manual mode may
use loopback HTTP but must render the existing insecure-transport warning.
Public signup additionally fails closed unless the named offer is enabled and
resolves to a shared target, eligible model/profile, hard allowances, enforced
rate/concurrency limits, and an approved acceptable-use version.

## 11. Open-source and platform choices

Keep the present stack:

- Go `net/http`, `html/template` or the existing controlled static assets, and
  `crypto/rand`, `crypto/sha256`, `crypto/subtle`;
- PostgreSQL 16 for identity state, transactions, locks, outbox, throttling,
  and audit;
- `github.com/jackc/pgx/v5`, already in use;
- existing bcrypt verification for bounded legacy compatibility; add
  `golang.org/x/crypto/argon2` only if the optional legacy-reset path is
  separately approved;
- the existing Docker Compose process topology.

Before exposing authentication beyond a trusted PoC network, upgrade the
module and image from Go 1.19 to a currently supported Go release and pin its
latest security patch. Go supports a major release only until two newer major
releases exist, so 1.19 is no longer a production option.

Use the pinned self-hosted Casdoor deployment selected in
[`WORKFORCE_AGENT_ACCESS_PRD.md`](WORKFORCE_AGENT_ACCESS_PRD.md) for new external
human authentication, OAuth/OIDC, device authorization, MFA, recovery, and the
future customer-federation seam. Use standard OIDC/OAuth validation in Alzette;
do not copy Casdoor's user/organisation authorization model into the product
database and do not implement SAML, WebAuthn, or OAuth protocols locally.

No extra library is needed for the database-backed throttle. A small in-process
token bucket may smooth bursts, but PostgreSQL state remains the restart-safe
enforcement source for this one-machine design.

## 12. Security, privacy, and abuse requirements

- External use requires reviewed TLS ingress, HSTS, Secure/HttpOnly/SameSite
  cookies, no mixed content, and no action credential in local/session storage.
- All unauthenticated account/recovery responses are generic and have
  comparable practical timing. Authenticated administrators may receive
  explicit duplicate-member/invite results inside their authorised tenant.
- URL credentials are never logged. On first valid GET they are exchanged for
  a setup cookie and removed through a redirect; `Referrer-Policy: no-referrer`,
  `Cache-Control: no-store`, strict CSP, and no third-party assets apply.
- A GET from an email security scanner must not accept an invitation, verify an
  address, or reset a password. The consequential transition requires a
  user-initiated POST.
- Login and every action-token issue/consume path have per-account and
  per-source throttles, bounded body sizes, deadlines, and alertable abuse
  counters. CAPTCHA is deferred unless measured abuse justifies the privacy and
  accessibility cost.
- Source address is `RemoteAddr` unless the connection came from an exact
  configured trusted ingress. Forwarded headers from any other peer are
  ignored, so a client cannot choose its own throttle identity.
- Business qualification and evaluation-to-customer conversion record the
  operator and an evidence reference. They never rely only on email domain,
  catalogue selection, usage volume, or quote acceptance.
- The mail provider receives only what is necessary to deliver the message.
  No prompt/output, target address, provider key, API key, usage, or financial
  telemetry is included.
- Audit stores safe IDs and masked email where display is necessary, never
  action tokens, password material, full session values, or message bodies.
- Data-subject deletion/withdrawal respects contracted audit/legal retention
  and records what was deleted versus retained and why.
- Password reset, email change, MFA change, and first-admin transfer are
  high-risk actions requiring fresh authentication once those settings exist.

## 13. Observability

Metrics and safe structured events must cover:

- signups submitted, verified, completed, blocked, superseded, and expired;
- evaluation organisations provisioned, first-key created, allowance exhausted,
  and first real request completed;
- qualification requests submitted, approved, rejected, withdrawn, and expired;
- invitations created, delivery pending/succeeded/failed, accepted, expired,
  revoked, resent;
- acceptance and reset failure classes without token or account disclosure;
- login/recovery throttles and suspected enumeration bursts;
- mail queue depth, oldest age, lease recovery, attempts, and permanent failure;
- membership/role changes and last-admin protection failures;
- time from signup verification to portal entry and first successful gateway
  request, plus invitation-to-acceptance time.

Customer-visible delivery status is safe and coarse. Raw mail-provider
responses, recipient lists across tenants, and security-abuse details are
operator-only.

## 14. Test plan

### Domain/unit tests

- email normalization, conservative equality, invalid input, and no
  provider-specific rewriting;
- legacy bcrypt compatibility; password-policy/Argon2id tests only if the
  optional legacy-reset path is enabled;
- random token length, digest-only representation, expiry, generation rotation,
  replay, and constant-time comparison;
- every allowed and forbidden state transition;
- role ceiling, last-admin, and existing-user acceptance policy;
- safe mail templates and absence of forbidden fields.

### PostgreSQL integration tests

- migration `0008` up/down/reapply from the current `0007` schema with existing
  organisations, users, routes, targets, plans, and ledger rows;
- catalogue/profile/price/quote scope constraints, dedicated target ownership,
  accepted-quote immutability, and one-active-capacity-revision guards;
- unique active signup and atomic evaluation organisation/plan/route creation
  under concurrent verification;
- unique active invitation under concurrent creation;
- exactly one accepted transaction under concurrent/replayed POSTs;
- rollback at every user/membership/invitation/audit write boundary;
- composite tenant/project/environment foreign-key rejection;
- session revocation after reset, user disablement, and membership disablement;
- outbox claim/retry after worker crash and token rotation on uncertain delivery;
- database/log scan proving no plaintext action token/password/API key;
- durable throttle behavior across process restart and expired-bucket cleanup.

### HTTP and browser tests

- complete signup, verification, evaluation first-call, qualification,
  new-user invite, existing-user invite, resend, revoke, expiry, and recovery;
- generic unknown/known forgot-password and signup responses with no meaningful
  enumeration signal;
- CSRF, setup-cookie, session fixation, hostile Host header, open redirect,
  oversized body, duplicate field, unknown field, and unsafe content-type tests;
- link-scanner GET does not consume an action;
- keyboard, screen-reader semantics, narrow viewport, validation focus, and
  password-manager behavior;
- CSP/referrer/cache/cookie headers and no action token after the clean redirect.

### Isolation and release tests

- two organisations, multiple projects/environments, every role, guessed IDs,
  reused invitation IDs, and cross-email attempts;
- a portal user cannot select a target/raw machine/provider and cannot obtain an
  application key without the existing access-management permission;
- signup and qualification create zero inference requests; only the user's
  explicit first call creates one logical request against the shared route;
- repeated signup cannot mint additional organisations, allowances, routes, or
  keys for the same verified identity;
- Docker Compose clean start, migration, worker restart, backup/restore, and
  deterministic fake-mail test;
- `go test -race ./...`, `go vet ./...`, migration tests, and an independent
  security review before Internet exposure.

## 15. Delivery increments

These are implementation increments inside the existing roadmap, not new
product “slices.” They add the self-service acquisition path while preserving
the Slice 3 remote-pilot hardening gate.

### Increment A — schema, identity, and evaluation foundation

- supported Go toolchain upgrade;
- additive email/registration/invitation/action-session/federated-identity/
  throttle schema;
- pinned Casdoor, portal OIDC, exact identity linking, and bounded legacy bcrypt
  compatibility without creating new Alzette-managed customer passwords;
- transactional signup verification plus Casdoor-owned external recovery;
- atomic evaluation organisation/plan/route provisioning from one disabled-by-
  default offer template;
- restart-safe shared allowance/rate/concurrency enforcement;
- operator invite/resend/revoke/backfill/reset commands and manual invite mode;
- audit, isolation, concurrency, migration, and browser tests.

**Exit:** a verified prospect authenticates through Casdoor, enters one isolated
evaluation tenant, connects through short human access or creates a separate
workload key, completes one real capped shared call, and sees its usage;
duplicate or abusive signup cannot create more free capacity.

### Increment B — delegated team onboarding

- People section in Access;
- member/invitation APIs;
- role ceiling, last-admin, resend/revoke, existing-user acceptance;
- fake mailer and optional transactional adapter/outbox.

**Exit:** the customer admin can onboard and remove a teammate without Alzette
handling the teammate's password and without crossing project/tenant scope.

### Increment C — qualification and dedicated-endpoint configuration

- catalogue/profile browsing and workload-based capacity configuration;
- business qualification, review, retention, and organisation conversion;
- versioned price/capacity quotes and explicit acceptance;
- dedicated deployment and capacity-increase requests handed to the operator
  fulfilment path.

**Exit:** an evaluation customer can configure and accept an evidenced offer,
but only an approved and validated target becomes a dedicated ready endpoint;
capacity expansion preserves the endpoint contract.

### Increment D — enterprise authentication

- first contracted customer's MFA or OIDC/SAML requirement;
- identity-subject linking, role mapping, recovery, deprovisioning, and access
  review;
- SCIM/JIT only if required.

**Exit:** the signed customer's identity lifecycle and production access policy
pass joint acceptance and deprovisioning tests.

## 16. Release gates

### Closed PoC gate

Manual invitation delivery may run on a trusted LAN with the existing visible
HTTP warning. It must not be represented as an Internet-ready account system.

### Internet onboarding gate

Go only when all are true:

- a supported patched Go toolchain is in the image;
- reviewed TLS ingress and canonical public origin are configured;
- Secure cookies and HSTS are enabled;
- durable login/action throttles and alerting pass;
- invitation acceptance, recovery, resend/revoke, and backup/restore pass;
- a configured shared evaluation offer has enforced lifetime/rate/concurrency
  limits, a cost owner, abuse alerts, and a tested kill switch;
- transactional mail, sender-domain records, bounce/failure handling, and
  support ownership are operational;
- privacy notice and retention are approved;
- an independent security review finds no unresolved critical/high issue.

### Financial-client production gate

The Internet gate plus the customer's contracted MFA/SSO, deprovisioning,
access-review, support, incident, and retention requirements must pass. This is
separate from proving a MeluXina model target or dedicated capacity.

## 17. Success measures

- at least 90% of valid invitations are accepted without operator password
  intervention;
- at least 80% of verified signups reach the real portal without operator help;
- median verified-signup-to-first-real-request time is under ten minutes;
- free evaluation cost, exhaustion, abuse blocks, and duplicate-allocation
  attempts are measurable per organisation;
- median accepted-invite setup time is under five minutes once the link is
  opened;
- zero customer passwords or action tokens appear in operator output except
  explicitly authorised one-time manual links;
- zero cross-tenant/member privilege failures in adversarial tests and pilot;
- invitation acceptance to first successful application request is measurable;
- support-assisted account recovery and failed-mail rates are visible;
- conversion is measured from verified evaluation account to first request,
  qualification, accepted quote, and ready dedicated endpoint—not page views.

## 18. Open decisions and owners

| Decision | Owner | Needed by |
|---|---|---|
| Canonical portal hostname and TLS ingress | Platform/security | Increment A Internet mode |
| Customer-facing sender domain and transactional mail provider/SMTP route | Founder/platform | Increment B mail mode |
| Default shared evaluation model, hard allowance, rate/concurrency limits, lifetime, and cost owner | Founder/platform/finance | Increment A |
| Privacy/acceptable-use versions, prospect fields, and retention | Founder/legal | Increment A |
| Which dedicated model/profile capacity metrics and prices have enough evidence to publish or quote | Founder/operator/finance | Increment C |
| Who may approve a first administrator and what business evidence is recorded | Founder/operator | Increment A |
| Whether the first signed client requires local MFA, OIDC, or SAML | Customer/founder/security | Production gate |
| Email-change and first-admin transfer support process | Operator/security | Increment B |
| Exact throttle thresholds and alert route after load/abuse testing | Security/operator | Internet gate |

None of these decisions blocks the additive schema and deterministic migration
tests. They do block enabling public signup or publishing an offer.

## 19. Definition of done

This PRD is implemented for self-service evaluation P0 when:

- no sales action or operator-created password is needed for a verified prospect
  to enter one isolated evaluation organisation;
- the evaluation route is explicitly shared, hard-capped, and backed by a real
  configured target; signup cannot change or duplicate its policy;
- the human connects an interactive agent through short-lived human access or
  creates a separate application key for a workload, and a real first call
  appears under the exact actor kind in the truthful usage ledger;
- a new and an existing user can accept an exact invitation safely;
- a customer admin can manage people separately from application credentials;
- recovery, resend, revoke, expiry, disablement, and last-admin protection work;
- plaintext action credentials exist only in the browser/email/manual one-time
  response for the minimum required lifetime;
- tenant and privilege boundaries pass database, API, and browser adversarial
  tests;
- current CLI-provisioned users migrate without lockout;
- the docs and login/signup pages truthfully distinguish evaluation/customer,
  shared/dedicated, Casdoor identity, short human-agent access, application
  credentials, provider credentials, and LAN/Internet modes.

Dedicated conversion is done only when qualification, quote, allocation,
deployment, validation, and route-binding states remain separate; an accepted
quote or approved company must never render as a ready endpoint without runtime
evidence.

## 20. Security references

The requirements above follow current primary guidance:

- [NIST SP 800-63B, Authentication and Authenticator Management](https://pages.nist.gov/800-63-4/sp800-63b.html)
  for password length, blocklists, rate limiting, and phishing-resistant
  authentication considerations;
- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
  for generic failures, throttling, MFA, and identity handling;
- [OWASP Forgot Password Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html)
  for uniform responses, single-use expiring tokens, side-channel delivery, and
  reset behavior;
- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
  for any separately approved Argon2id and legacy bcrypt migration path;
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
  for cookie-only session exchange, TLS, rotation, expiry, and safe logging;
- [Go release policy](https://go.dev/doc/devel/release) for the supported
  toolchain gate.
