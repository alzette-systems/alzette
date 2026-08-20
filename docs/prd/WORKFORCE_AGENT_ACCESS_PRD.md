# Alzette workforce agent access PRD

**Status:** proposed implementation contract; not yet implemented

**Date:** 2026-08-17

**Owners:** product, platform, security, and quality

**Related documents:** [`PRODUCT.md`](../product/PRODUCT.md) defines the product promise;
[`PORTAL_PRD.md`](PORTAL_PRD.md) defines the complete customer portal;
[`ACCOUNT_ONBOARDING_PRD.md`](ACCOUNT_ONBOARDING_PRD.md) owns business signup,
invitation, and membership state; [`POC_BOUNDARY.md`](../product/POC_BOUNDARY.md) remains
the controlling statement of what the current software actually proves; and
[`ALZETTE_CONNECT_PRD.md`](ALZETTE_CONNECT_PRD.md) owns the employee-facing CLI,
desktop launcher, application adapters, packaging, and supported client/protocol
matrix built on the access contract defined here.

This document controls the narrower but critical path from an accepted
employee invitation to authenticated use of an Alzette inference endpoint. If
another product document says that a new external employee must receive or
create a permanent personal API key, or that new external users must have an
Alzette-managed local password, this document supersedes that statement.

## 1. Decision

Alzette will provide **human agent access** in addition to the existing
service-account API-key path.

Every organisation has exactly one current **owner**. Every other human is an
**employee**. The owner invites an exact email address and assigns the employee
to one or more owner-managed access groups. Each group grants a set of active
Alzette model endpoints; there is no invitation role picker and P0 has no
direct per-employee model exception. The employee accepts the invitation,
authenticates through Alzette's self-hosted Casdoor identity service, and can
then connect a supported agent without copying a permanent remote credential.
The owner can manage and use every active model endpoint in the organisation.

The first implementation uses the following separation:

1. Casdoor authenticates the person through OAuth/OIDC.
2. Alzette remains authoritative for invitations, users, ownership, employee
   status, groups, model entitlement, tenant routing, usage, and revocation.
3. A valid Casdoor identity alone grants no Alzette access.
4. The person login is durable across client restarts through a rotating
   Casdoor refresh session. Casdoor access tokens remain short-lived and
   memory-only; the refresh credential is stored under the explicit protected
   local-storage contract in this document and is never sent to Alzette.
5. After login or automatic identity refresh, the Alzette control service
   validates the Casdoor access token, resolves an enabled local identity and
   company relationship, computes the current owner-or-group entitlement, and
   returns a random, ten-minute Alzette human-agent token with an `alz_u_`
   prefix.
6. The inference gateway accepts that short-lived token through a distinct
   authentication path and derives one exact organisation/project/environment
   scope from it.
7. P0 uses a local `alzette-agent` compatibility proxy for one named operating
   system and agent version. Native-provider and headless-device integrations
   remain extensions until separately selected and tested.
8. Service-account keys with the existing `alz_k_` prefix remain the correct
   credential for applications, CI, shared servers, and unattended workloads.

The gateway will **not** accept Casdoor JWTs directly. It will not authorize
from email domains, Casdoor organisations, Casdoor groups, or client-supplied
tenant claims. The extra Alzette token-mint step is deliberate: it binds the
human to one current Alzette membership, preserves next-request local
revocation, and keeps identity-provider logic off the gateway hot path.

Casdoor is the selected P0 identity provider, subject to the acceptance spike
and pinned-version gate in this document. Alzette will not implement passwords,
MFA, OAuth authorization, device authorization, or identity federation from
scratch.

## 2. Product outcome

The required customer experience is:

```text
Company owner invites employee
      -> employee accepts and signs in
      -> Alzette membership becomes active
      -> employee selects Connect your agent
      -> browser PKCE login
      -> local compatibility proxy starts the named agent
      -> stable Alzette endpoint
      -> company route: shared or dedicated
```

An invited employee must be able to reach a first authenticated inference call
without asking the owner to create, reveal, copy, rotate, or recover a personal
API key. The owner must be able to identify the human actor's
metadata-only consumption and revoke that person's access without disrupting
separately owned production service accounts.

After the first browser login, later `alzette-agent` launches ordinarily reuse
the protected login session and refresh identity automatically. The employee
does not see or manage the ten-minute `alz_u_` credential, and its expiry is not
presented as the length of the employee's login. The default identity-session
policy is 30 days of inactivity with a 90-day absolute maximum; an organisation
may shorten those bounds after the P1 policy controls exist.

The customer-facing endpoint is not a secret. An invitation may state an
evidenced gateway base URL and model alias, for example:

```text
Gateway: https://inference.alzette.systems/v1
Model:   alzette-chat
```

It must never contain the private target host, MeluXina LAN address, provider
credential, OAuth token, Alzette bearer token, or local proxy credential. If no
callable route is evidenced, the email and portal say that connection details
will become available after activation; they do not fabricate an endpoint.

## 3. Current implementation baseline

The current repository already provides:

- local username/password portal users, organisation/project/environment
  memberships, revocable server-side sessions, and CSRF protection;
- service accounts with one-time-reveal, hashed, scoped, expiring `alz_k_`
  API keys;
- server-controlled model-alias routing to shared or dedicated targets;
- bounded OpenAI Chat Completions, OpenAI Responses, and Anthropic Messages
  text/function-tool subsets with buffered and SSE responses; Responses and
  Messages translate through the same server-owned Chat execution path;
- one immutable logical request separated from internal provider attempts;
- tenant-safe usage, request, route, and portal APIs;
- exact owner/employee/group authority plus group model grants in migration
  `0012`;
- digest-only exact-email invitations, setup sessions, federated identities,
  OIDC transactions, manual delivery, resend/revoke, and atomic acceptance in
  migration `0013` and the portal/workforce services;
- digest-pinned local Casdoor with deterministic organisation/application/demo
  bootstrap, Authorization Code plus PKCE, strict issuer/audience/RS256 checks,
  access-token introspection, and a real browser invitation acceptance path;
- group-filtered agent contexts, digest-only human-agent grants and maximum
  ten-minute `alz_u_` credentials in migration `0014`;
- strict `alz_k_`/`alz_u_` gateway dispatch, current group-policy rechecks,
  revocation, and service-or-human request-ledger actor constraints;
- a separate `alzette-agent` Go helper with browser Authorization Code plus
  PKCE, safe context selection, in-process OAuth refresh, automatic short-token
  remint, an authenticated ephemeral loopback proxy, grant revocation on exit,
  an isolated Pi 0.84.2 provider adapter, and verified local Linux Jan Desktop
  0.8.4 and Goose Desktop 1.46.0 custom-provider paths;
- one-machine Docker Compose with PostgreSQL, Casdoor, gateway, control,
  public, worker, migration, and optional billing processes.

The current repository does **not** provide:

- production email invitation delivery or canonical remote HTTPS evidence;
- a protected local rotating-refresh credential store or durable agent login;
- Device Authorization, a native client-owned OAuth provider, automatic
  native-client configuration, or broader named version/OS adapters;
- complete Casdoor refresh-rotation/reuse-family, logout, MFA, signing-key
  rotation, restart/restore, and operator recovery evidence;
- complete employee disable/reactivate, ownership transfer/recovery, and
  group-change transaction-invalidation behavior;
- TLS ingress suitable for remote employee authentication.

Therefore the implemented company/group, invitation/OIDC, and human inference
slices are local loopback evidence only. The current Compose deployment may
claim its tested pinned-Casdoor invitation, public-PKCE, group-filtered mint,
real inference, attribution, and revocation behavior. It must not claim
production email, remote employee OAuth, durable client login, general desktop
client compatibility, production identity recovery, or pilot readiness until
those separate gates pass.

The eventual implementation is guarded by a server capability whose default
is disabled. When disabled—or when the exact Casdoor, canonical HTTPS,
invitation/mail, gateway, or migration prerequisites are absent—the workforce
routes are not mounted, well-known metadata advertises no workforce login
mode, and portal/email/docs render the feature as unavailable rather than
partially working. Enabling the capability with incomplete configuration makes
the affected service fail startup.

## 4. Scope and priorities

### P0 — invited employee to authenticated agent

P0 is evaluation/named-pilot workforce access for one evidenced client/OS
combination. It is not a claim of general enterprise onboarding, customer-
federated SSO, or universal OpenAI-client compatibility.

P0 includes:

- one self-hosted, pinned Casdoor replica in the single-machine deployment;
- one confidential portal OIDC client and one public `alzette-agent` OAuth
  client;
- owner-created, exact-email invitations with an exact initial group set;
- exactly one current organisation owner with explicit atomic transfer and
  audited operator-assisted recovery;
- owner-managed access groups, employee group assignment, and default-deny
  group-to-model grants;
- Casdoor-backed identity creation/login and Alzette identity linking;
- browser Authorization Code with PKCE S256;
- a maximum-one-hour Casdoor access token plus a rotating refresh session with
  a default 30-day inactivity limit and 90-day absolute limit;
- protected refresh-session custody with keyring, explicit restricted-file,
  and process-memory modes; there is no silent plaintext-file fallback;
- an Alzette agent context and short-token API;
- ten-minute, membership-bound, alias-bounded `alz_u_` credentials;
- separate gateway authentication for `alz_k_` and `alz_u_` credentials;
- human-agent request lineage and per-employee metadata-only attribution;
- the `alzette-agent` loopback proxy for key-only OpenAI-compatible clients;
- an Access → People and Access → Agent sessions management surface;
- a Docs → Connect your agent workflow;
- next-request user, membership, grant, and token revocation at the Alzette
  boundary;
- deterministic fake-IdP tests plus an opt-in real Casdoor integration gate;
- preservation of every existing service-account behavior and ledger invariant.

P0 is complete only for the operating systems and agent clients named by the
first pilot. The generic protocol contract is broader than the first supported
installer matrix.

### P1 — enterprise lifecycle and broader native support

P1 includes, only when required by a signed pilot:

- Device Authorization for SSH or otherwise headless sessions;
- federation from the customer's OIDC or SAML identity provider into Casdoor;
- mandatory organisation-specific MFA policy and phishing-resistant methods;
- SCIM/JIT or external-directory group lifecycle integration;
- additional signed installers and reviewed auto-update for agreed desktop
  platforms beyond the named P0 artifact;
- native providers for additional agents that pass credential-storage review;
- sender-constrained tokens such as DPoP if the threat model requires them;
- organisation access reviews, active-agent-session inventory, and export;
- customer-configured session length and permitted credential-store modes
  within Alzette's safe maximums;
- container/remote-development compatibility with an explicit private IPC
  design rather than a publicly bound proxy.

### Explicit non-goals

- replacing service accounts for CI, applications, servers, or automation;
- giving every employee a permanent personal API key;
- using a portal password, portal cookie, invitation token, Casdoor ID token,
  device code, or local proxy key directly at the inference gateway;
- accepting a Casdoor role, group, organisation, email, or domain as Alzette
  tenant authority;
- mirroring Alzette customer organisations into Casdoor as the source of truth;
- letting an employee choose an upstream target URL, provider credential,
  private host, model slug, raw machine, or capacity allocation;
- exposing the local compatibility proxy to the LAN or Internet;
- changing request/response semantics to compensate for an agent that does not
  support Alzette's tested OpenAI-compatible API subset;
- promising compatibility with agents that cannot configure a base URL;
- prompt/output history, employee productivity scoring, or conversation review;
- SMS authentication, consumer social login, dynamic OAuth client registration,
  or a general-purpose third-party OAuth platform;
- Redis, Kubernetes, Kafka, or multiple Casdoor replicas for the single-machine
  P0.

## 5. Actors and authority

| Actor | Can do | Cannot do |
|---|---|---|
| Company owner | Manage the company, employees, groups, model grants, endpoints, billing, and application access; use all active company model endpoints; transfer ownership explicitly | Create a second current owner, leave the company ownerless, grant a direct employee model exception, see credentials, select a raw target, or silently take ownership of an employee session |
| Employee | Accept their own invitation, authenticate, discover and use models granted through enabled groups, view their own safe usage, and revoke their own agent session | Invite or manage people/groups, receive direct model exceptions, join by domain, mint service credentials, or select infrastructure |
| Service account | Run unattended inference with an `alz_k_` key | Sign in, accept invitations, or inherit a human's membership automatically |
| Casdoor | Authenticate identities, run OAuth flows, apply identity/MFA policy, issue and introspect identity access tokens | Create an Alzette membership, choose a route, or authorize a model |
| Alzette operator | Configure Casdoor trust, mail, policies, models, routes, and break-glass recovery | Learn a user's password or silently represent the customer in an agent session |

P0 has no generalised customer role system. `org_admin`, `project_admin`,
`developer`, and `viewer` are legacy implementation values to migrate, not
customer choices. Model access has two explicit branches:

```text
owner = active company entitlement ∩ active company route

employee = active company entitlement
  ∩ active route in the group's server-owned project/environment scope
  ∩ enabled employee
  ∩ enabled group membership
  ∩ enabled group model grant
```

Employee group grants form a union only inside that intersection. P0 has no
direct employee model grant and no deny rule. Removing an employee from a group,
removing a model from a group, disabling either object, or retiring the route
must block the next new request and the next token mint. Adding access requires
fresh discovery/mint; an existing token never silently gains privilege.

## 6. Credential taxonomy

| Credential | Purpose | Storage | Remote validity |
|---|---|---|---|
| Alzette invitation token | Enter one invitation acceptance flow | SHA-256 digest in PostgreSQL; plaintext only in delivery/link | Control service only; single use; default 72 hours |
| Casdoor password/passkey/MFA | Authenticate the person | Casdoor only | Casdoor only |
| OAuth authorization code | Complete browser PKCE | Transient Casdoor/client state | Casdoor token endpoint only; single use |
| Device/user code (deferred) | Complete an explicitly enabled headless login | Transient Casdoor/client memory | Casdoor device flow only; maximum 10 minutes |
| Casdoor access token | Prove identity to Alzette's token broker | Client memory only; never Alzette persistence or child-process exposure | Agent identity APIs only; maximum one hour |
| Casdoor rotating refresh credential | Renew identity without repeated browser login | OS credential store by default; explicit restricted file or process-memory mode; never Alzette persistence or child-process exposure | One rotation use at a time; session family defaults to 30-day inactivity and 90-day absolute limits |
| Alzette `alz_u_` token | Invoke inference as one human membership | SHA-256 digest in PostgreSQL; plaintext client memory only | Gateway inference only; maximum 10 minutes |
| Local proxy key | Satisfy an agent's API-key field | Proxy memory and the launched child's environment only in P0 | Loopback proxy only; process lifetime |
| Alzette `alz_k_` key | Authenticate a non-human workload | SHA-256 digest in PostgreSQL; plaintext revealed once to workload owner | Existing scoped machine APIs and gateway until expiry/revoke |
| Provider secret | Authenticate Alzette to a target | Server-side file-backed secret reference | Target only |

The product must call these by their exact names. “Password,” “API key,”
“agent session,” and “provider credential” are not interchangeable labels.

## 7. System architecture

```text
Company owner browser
    │  portal session
    ▼
Alzette control ── invitation/outbox ──> email
    │                                      │
    │                                      ▼
    │                           employee accepts invitation
    │                                      │
    └──────── OIDC callback <────── Casdoor identity
                    │
                    └── (issuer, subject) -> Alzette person + company access

Employee workstation
    │
    ├── browser PKCE ─────────────────────> Casdoor
    │                                         │ identity access token
    │                                         ▼
    ├── alzette-agent ────────────────────> Alzette agent identity API
    │                                         │ membership/alias validation
    │                                         ▼
    │                                  ten-minute alz_u_ token
    │
    └── agent ─> loopback proxy if needed ─> Alzette gateway
                                                │ token-derived tenant context
                                                ▼
                                         existing tenant route
                                                │
                                                ▼
                                      shared or dedicated target
```

Casdoor is not in the inference hot path. Its outage prevents a new login or
identity refresh and therefore prevents a new `alz_u_` mint after the current
Casdoor access token expires. An already issued ten-minute Alzette token remains
valid until its own expiry unless Alzette revokes it sooner.

## 8. Identity and invitation contract

### 8.1 Source-of-truth boundary

Casdoor owns:

- passwords, passkeys, MFA, recovery, and upstream identity federation;
- browser authentication and consent;
- Authorization Code with PKCE, Device Authorization, signing keys,
  discovery, access-token issuance, rotating refresh-session families,
  introspection, logout/revocation, and IdP sessions.

Alzette owns:

- business invitation state and exact acceptance transaction;
- user-to-identity links;
- exactly-one-owner company authority, employee state, access groups, group
  membership, and group model grants;
- server-owned project/environment enforcement scopes derived from groups;
- model aliases, endpoint entitlement, routing, and service mode;
- human-agent grants/tokens, usage attribution, and audit.

Alzette never stores or receives a Casdoor refresh credential. It may retain a
safe opaque session-family reference only if the pinned Casdoor contract needs
that reference for audited server-side revocation; the reference is not bearer
authority and cannot renew identity.

Alzette may create a one-use, email-bound Casdoor registration invitation to
gate identity signup. That Casdoor object is only an authentication bootstrap.
The Alzette invitation remains the sole membership authority, and a partial
failure between the systems must never activate access.

### 8.2 Identity linking

An external identity is identified by the exact pair `(issuer, subject)`.
Email is used only during a verified invitation or self-service linking
transaction. After linking:

- changing an email does not silently change the identity link;
- a second subject with the same email is not linked automatically;
- a Casdoor organisation/group/role claim does not create a membership;
- an existing local user must recently authenticate before explicitly linking
  a Casdoor identity;
- account merge, email change, and identity transfer are operator-reviewed
  recovery actions until a dedicated safe workflow exists.

### 8.3 Invitation acceptance

1. The current owner enters an exact email, optional display name, and at least
   one enabled Alzette access group in Access → People. There is no role,
   project, environment, or direct-model picker. Before send, a human-readable
   review shows the company, employee status, groups, effective assigned model
   endpoints, current route readiness, the supported client/OS, and that no
   personal API key will be created.
2. Alzette derives the company and owner authority from the current portal
   session, resolves every group server-side, snapshots the intended group IDs,
   and stores a digest-only, expiring invitation.
3. Email delivery contains the inviting organisation, inviter, exact groups,
   expiry, canonical HTTPS acceptance link, and a non-secret gateway/model
   summary only when runtime evidence exists.
4. `GET /accept-invite?token=...` may exchange the URL credential for a
   short-lived, non-authorising setup session and responds with `303` to the
   clean `/accept-invite` URL. It cannot accept, consume, revoke, or rotate the
   invitation; the invitation remains redeemable until the deliberate
   authenticated POST succeeds or it otherwise expires or is revoked.
5. The user deliberately continues to Casdoor and authenticates or creates an
   invited identity.
6. The callback validates issuer, signature, audience, state, nonce, expiry,
   and an exact verified-email match for initial linking.
7. One PostgreSQL transaction locks the invitation, rechecks expiry/revocation,
   links or resolves the user, creates/enables the employee record and exact
   group memberships, records audit, marks accepted, and invalidates other
   setup sessions.
8. The user lands in the invited company and sees only models derived from the
   accepted group set.

Concurrent acceptance, replay, resend rotation, revoked invitations, expired
invitations, wrong identities, group substitution, ownership escalation, and
cross-tenant IDs fail
closed. Acceptance never creates an `alz_k_` key or an `alz_u_` token.

Membership invitation remains possible before a route is ready, because team
membership does not own infrastructure. In that state the UI and email say
that agent connection is not yet enabled and omit a runnable setup command.
Any “Invite and connect” presentation is disabled until an eligible model
alias, current route evidence, workforce capability, and the named client/OS
are all enabled for the assigned company and groups.

The setup cookie is `__Host-alzette_setup`, `Secure`, `HttpOnly`,
`SameSite=Strict`, `Path=/`, has no `Domain`, and expires within 15 minutes. The
GET and clean page send `Cache-Control: no-store`, `Referrer-Policy:
no-referrer`, a restrictive CSP with no third-party subresources, and no
analytics or tracking pixels. Before exact Casdoor authentication the clean
page discloses no organisation, inviter, group, model, project, or environment. A
setup cookie alone grants no identity, membership, portal session, agent grant,
or inference credential. Repeated scanner GETs may create bounded independent
setup sessions but do not invalidate the invitation; successful acceptance,
resend, revoke, or expiry invalidates every associated setup session. Only an
authenticated, CSRF-protected, deliberate POST can accept.

### 8.4 Portal login migration

New external self-service and invited users authenticate through Casdoor and
do not receive an Alzette-managed password. Existing local users continue to
work during a bounded migration period.

The schema must permit a federated user with no password hash and a local user
with a valid password hash, but never a user with neither an enabled federated
identity nor a valid local authentication method. A fake password hash must not
be manufactured to satisfy the old schema.

Casdoor owns recovery for federated users. Alzette's proposed local password
reset path remains only for still-enabled legacy local accounts. Operator
break-glass identity must be separately controlled, auditable, and unavailable
at the inference gateway.

## 9. Agent authentication workflows

### 9.1 Browser login and durable session

The primary P0 command reuses a valid protected login session or starts browser
login when none is available. Access and inference tokens remain in the process
that owns the local proxy and child agent:

```console
$ alzette-agent run --verify -- my-agent
Opening the Alzette sign-in page…
Signed in as alice@example.lu
Organisation: Example Bank
Context: Research / Development
Login refreshes automatically · reauthentication by: 13 November 2026
Verifying alzette-chat… success · request req_… · 412 ms
Starting my-agent through http://127.0.0.1:43127/v1…
```

When a reusable login exists, the opening-browser line is replaced by a calm
`Using your saved Alzette login` status. The CLI does not expose access-token,
refresh-token, or `alz_u_` values or make the employee reason about their
different expiries.

When exactly one inference-enabled context exists, `run` selects and displays
it. When several exist, it presents a keyboard-accessible numbered choice with
human-readable company, group, project/environment, and model labels. The employee does
not need to know or type an opaque membership ID. `--context <opaque-id>` is an
advanced copy/paste override and remains fully re-authorized.

The client is a public OAuth client with no embedded secret. It uses:

- Authorization Code only;
- PKCE S256 with a transaction-specific high-entropy verifier;
- transaction-specific `state` and OIDC `nonce`;
- an exact loopback IP-literal redirect on an ephemeral port;
- the exact configured Casdoor issuer and Alzette agent resource audience;
- a maximum-one-hour access token; and
- the exact configured refresh scope, including `offline_access` when required
  by the accepted Casdoor contract.

The returned refresh credential establishes a bounded login-session family,
not inference authority. Its P0 defaults are a 30-day inactivity timeout and a
90-day absolute timeout. Every successful refresh rotates the credential and
invalidates the prior value. Reuse of an invalidated value revokes the family
and requires browser login. Extending either default requires a later explicit
customer/security policy; shortening either value is safe.

Before an access token expires, the client refreshes it automatically against
the exact discovered token endpoint. Refresh for one local profile is
serialized under a cross-process lock: the client reads the latest credential
while holding the lock, completes one rotation, and atomically replaces the
stored value before releasing it. A crash after server rotation but before the
replacement commits intentionally loses the local session and requires browser
login; the client never retries an ambiguous old credential. Missing rotation,
malformed token responses, reuse detection, expiry, revocation, issuer or
audience mismatch, and storage failure all fail closed without starting the
proxy.

Only the rotating refresh credential may survive a client process. The
Casdoor access token, ID token, authorization code, `alz_u_` token, OAuth
transaction values, and local proxy key remain memory-only and are never
written to the persistent login store.

Credential-store modes are explicit:

- `keyring` is the P0 default and uses the named operating system's credential
  store;
- `memory` keeps the refresh credential only for the current process and
  therefore requires browser login after exit; and
- `file` is a company-policy-controlled, explicit opt-in for systems
  without a usable keyring. Its directory is owner-only mode `0700`, its file
  is `0600`, writes are atomic, and symlink, hard-link, wrong-owner, or broader
  permission states fail closed.

The client never silently falls back from keyring to a plaintext file. A file
credential is treated like a password: it is excluded from backups, support
bundles, version control, and normal diagnostics. Organisation policy may
disable `file` or require `memory` even when the user requests another mode.
Refresh material never enters browser storage, a child environment, process
arguments, stdout, logs, crash reports, or Alzette APIs.

The supported lifecycle commands are:

```text
alzette-agent login
alzette-agent login status
alzette-agent logout
```

`run` performs `login` implicitly when needed. `login status` reports only the
identity, selected issuer/profile, store kind, and safe idle/absolute expiry
times. `logout` first requests Casdoor session-family revocation when available,
revokes the user's active Alzette agent grants, and then deletes the local
refresh credential. Local deletion still occurs if remote revocation is
unavailable, with a clear safe warning rather than a false success claim.

Minting another ten-minute `alz_u_` token is not OAuth refresh. It always
requires a current Casdoor access token and a fresh Alzette authorization check.

The loopback callback listener exists only for the authorization response and
closes immediately. `localhost` is not used where an IP literal can be used.
Non-loopback HTTP redirect URIs and private-client secrets in the binary are
forbidden.

### 9.2 Headless login — deferred extension

Device Authorization is not enabled or advertised in baseline P0. If the first
named pilot requires a headless path, it becomes an explicitly enabled P1
capability and the employee runs:

```console
$ alzette-agent run --device --context research-dev -- my-agent
Open https://auth.alzette.systems/device
Enter code: ABCD-EFGH
Waiting for approval…
```

The client discovers Casdoor's device authorization and token endpoints from
the pinned issuer. It displays the canonical verification URI, user code,
expiry, polling interval, and selected Alzette context. It implements pending,
`slow_down`, denial, expiry, cancellation, and success exactly. Codes never
enter logs, analytics, process arguments, shell history, or support output.

The approval screen shows the employee, organisation, project, environment,
requested inference scope, client name, and expiry so that a user does not
approve an unexplained device.

### 9.3 Context selection and Alzette token mint

After Casdoor authentication, the client asks the Alzette agent identity API
for contexts. It receives only current effective contexts linked to the
authenticated `(issuer, subject)`: every active company alias for the owner or
group-derived aliases for an employee, with safe company, group where
applicable, permitted-alias, and customer-facing gateway labels.

If exactly one inference-enabled context exists, the client may select it
automatically while displaying it. If several exist, the user must choose. A
remembered context is only a selector and is re-authorized on every token mint.

For one `run` process, the client creates a random 128-bit
`client_instance_id`. It submits that value, the opaque membership ID, and a
subset of permitted model aliases. The value is a correlation handle, not
authority, and is re-authorized on every call. The control service:

1. validates the Casdoor access token against one configured issuer;
2. verifies signature, algorithm, `iss`, resource-bound `aud`, `sub`, `exp`,
   `nbf`, and authorised client ID with bounded clock skew;
3. performs configured token introspection before creating a grant or minting
   its next short token;
4. resolves the exact active local identity and person, then the current
   owner/all-active-endpoint or employee/group-derived alias entitlement;
5. creates an auditable agent grant or mints its next bounded short token;
6. generates a 256-bit random `alz_u_` token, stores only its SHA-256 digest,
   and returns plaintext once with a maximum ten-minute expiry.

There is at most one active `alz_u_` token for one agent grant. A successful
mint atomically revokes the previous token for that grant before committing the
new token. Different `run` processes have different grants and do not rotate
one another's credentials.

Unknown fields, duplicate authorization headers, missing aliases, invalid
identity tokens, disabled identities, cross-membership identifiers, and
unauthorised aliases fail closed. Cross-tenant absent/forbidden resources use
the same safe external response.

### 9.4 Native agent provider — deferred extension

A later native integration may connect directly only when it supports:

- the configured Alzette base URL and API subset;
- browser or device OAuth callbacks;
- custom short-token mint logic through the Alzette agent identity API;
- secure enough credential custody for the customer's policy;
- the exact model alias and no arbitrary upstream selection.

Pi's provider extension interface can expose browser/device login and token
minting. That makes Pi a possible later integration proof, but its actual
credential storage must be reviewed. A native provider must not persist the
Casdoor access token or `alz_u_` token merely because its plugin API calls the
field a refresh token. It may delegate the accepted rotating-refresh custody
contract to `alzette-agent`; otherwise its own keyring, rotation, concurrency,
logout, and leak behavior must pass the same tests.

OpenAI-compatible request shape does not imply native OAuth support. Codex-like
login is the target experience, not a claim that every existing agent can use
Alzette without an adapter.

## 10. Local compatibility proxy

### 10.1 Product shape

[`ALZETTE_CONNECT_PRD.md`](ALZETTE_CONNECT_PRD.md) defines the product name,
desktop surface, application adapters, and migration from this prototype CLI.
This section remains authoritative for credential custody, loopback proxy
security, and launch-lifetime behavior.

Ship a separate minimal Go binary from the same repository, tentatively:

```text
cmd/alzette-agent/
internal/agentauth/
internal/agentproxy/
```

Do not distribute the database/operator server CLI as the employee client.

Required commands:

```text
alzette-agent configure --control https://portal.example.lu \
  --credential-store keyring
alzette-agent login
alzette-agent login status
alzette-agent logout
alzette-agent contexts
alzette-agent run [--context <opaque-id>] [--verify] -- <agent command...>
```

The primary path is `run`: it starts the proxy on a random loopback port,
creates a random process-lifetime compatibility key, launches the child with a
local base URL and that local key, and destroys both when the child exits.
Authentication occurs inside that long-running process. P0 has no background
daemon. The only standalone bearer state allowed to persist between commands
is the rotating Casdoor refresh credential in the selected protected store;
access tokens, `alz_u_` tokens, grants, and local proxy keys are not cached
there. `contexts` reuses or refreshes the login, displays authorised choices,
and discards its access token on exit. An independently running `proxy` command,
config-file inference-credential installation, and agents that cannot be
launched as a child are deferred until an exact local IPC/credential-delivery
design is reviewed.

An agent that expects conventional variables sees values equivalent to:

```text
OPENAI_BASE_URL=http://127.0.0.1:<random-port>/v1
OPENAI_API_KEY=<process-lifetime local compatibility key>
```

The local key is not an API key in the Alzette product sense. It is never
accepted by the remote gateway and is never forwarded upstream.

`--verify` performs one deliberate, bounded Chat Completions request with fixed
Alzette-authored non-customer test content through the selected real tenant
route before launching the child. It counts as one logical request and against
any shared allowance. The CLI prints only model alias, success or a coarse
failure, safe request ID, and latency; it does not log or persist the request or
response body. Success means authenticated inference completed—not merely that
the proxy listened. On failure the child is not launched and the CLI gives the
route-not-ready, unsupported-client, authentication, or support-safe next step.

`run` generates at least 256 random bits and passes the resulting Base64URL
capability to the child only as `OPENAI_API_KEY`; it never appears in an
argument, URL, stdout, config file, or parent environment. The local listener
accepts exactly one `Authorization: Bearer <capability>` header. Missing,
duplicate, malformed, or incorrect capabilities receive the same generic 401.
The child receives the loopback base URL through `OPENAI_BASE_URL`. The launcher
removes inherited Casdoor, Alzette remote-token, and provider-secret variables
before starting the child. Agents that ignore those two variables require a
separately tested adapter and are not supported by baseline P0.

### 10.2 Proxy security contract

The proxy must:

- bind only an explicit `127.0.0.1` or `::1` socket, never `0.0.0.0`;
- use a random port by default and fail if the actual listener is not loopback;
- require the exact random per-launch bearer capability even on loopback;
- require `Host` to equal the actual bound IP literal and port; reject every
  `Origin` header, hostname/DNS form, absolute-form URL, proxy request,
  forwarded header, cookie, duplicate credential, and unsupported method/path;
- expose only `POST /v1/chat/completions` for the baseline tested gateway
  contract and emit no CORS permission;
- use an operator-built or signed discovery profile and exact HTTPS upstream,
  never a request-selected issuer, JWKS URI, redirect, or inference target;
- strip the inbound `Authorization`, `Proxy-Authorization`, connection-specific,
  forwarding, and cookie headers before setting one `alz_u_` authorization;
- keep Casdoor access and Alzette `alz_u_` tokens in memory; only the rotating
  Casdoor refresh credential may use the protected store defined in section
  9.1, with no silent plaintext-file fallback;
- never log, trace, crash-dump, or persist prompt/output bodies or credentials;
- pass streaming bytes, SSE flushes, cancellation, response headers, safe
  request IDs, status, and tool-call deltas without semantic transformation;
- close the listener, revoke the process grant where possible, best-effort
  overwrite owned access/agent/local-key byte buffers, and release process
  credential state on exit. Explicit `logout` also revokes and deletes the
  durable refresh session as defined in section 9.1.

A malicious process running as the same operating-system user, debugger,
administrator, or compromised agent can still read memory or invoke a local
process. The proxy reduces copied-secret risk; it does not make a compromised
workstation trustworthy. That limitation must be stated in the security pack.

### 10.3 Short-token mint and request replay

The proxy mints the next `alz_u_` token before forwarding when the current token
has insufficient remaining lifetime. It does so with a valid in-memory Casdoor
access token. If that identity token has expired or is near expiry, the client
first performs the serialized rotating refresh from section 9.1. If no valid
refresh session remains, it requires browser authentication before sending the
inference request. Neither operation relaxes the fresh Alzette membership and
alias checks performed by the credential broker.

After a gateway call starts, the proxy may replace the short token and retry
the inference request once only when all of the following are true: the status
is `401`, the response came from the configured Alzette HTTPS origin, the
response includes `X-Alzette-Request-State: not-created`, the OpenAI-compatible
error code is `human_token_inactive`, and no response body byte has yet been
passed to the child. The marker means gateway authentication rejected the call
before a logical request ledger row or provider attempt was created; the safe
correlation `X-Alzette-Request-ID` may still exist. Missing, conflicting, or
malformed evidence means no retry. No upstream response may set this marker
because the gateway strips it from provider responses.

It must never replay an inference POST after:

- an ambiguous network error;
- a gateway timeout;
- an upstream/provider error;
- creation of a logical request;
- receipt of any downstream response byte or SSE event.

Authentication occurs before the gateway creates the logical request ledger
row. Provider retries remain internal attempts under the existing one-logical-
request contract. A future generic idempotency feature may broaden safe retry,
but P0 does not assume it.

## 11. Gateway, routing, and accounting contract

### 11.1 Separate authenticators

The gateway accepts exactly one credential representation and dispatches by
strict Alzette credential prefix. Chat Completions and Responses use
`Authorization: Bearer`; Anthropic Messages accepts either that form or the
SDK-compatible `X-Api-Key` form, never both:

- `alz_k_`: existing service-account/API-key validation, unchanged;
- `alz_u_`: new human-agent token validation.

Do not loosen the existing `alz_k_` format validator to accept arbitrary JWTs.
Do not attempt one credential type after a parse/validation failure in the
other. Both paths otherwise return the same safe public authentication
envelope while recording a safe operator-only failure class. The sole P0
exception is the narrowly defined `human_token_inactive` plus
`X-Alzette-Request-State: not-created` signal in section 10.3; it is emitted
only for a structurally valid `alz_u_` credential rejected before ledger
creation and reveals no tenant, user, or revocation reason.

Every `alz_u_` authentication joins the active token, grant, external identity,
human user, and enabled company relationship. It derives organisation,
project, environment, allowed aliases, and `inference:write` from the current
owner/all-active-endpoint branch or employee/group branch. The request cannot
submit a tenant, membership, raw model slug, target, or provider.

`alz_u_` tokens authenticate only the supported inference routes. They do not
authenticate portal APIs, operator APIs, billing APIs, or the existing
Bearer-only `/api/v1/dashboard`, usage, and request-detail machine APIs.

### 11.2 Principal and ledger lineage

The platform principal becomes a discriminated actor:

```text
credential kind: service_account_key | human_agent_token
tenant scope:    organisation / project / environment
actor lineage:   exactly one service tuple OR one human-agent tuple
scopes:          server-derived intersection
```

The inference ledger must support exactly one of:

```text
service_account_id + api_key_id + key_prefix
```

or:

```text
human_user_id + human_membership_id + agent_grant_id + agent_token_id
```

An XOR database constraint and composite foreign keys enforce the distinction.
Do not create a synthetic hidden service account for each employee merely to
avoid this migration. That would misstate identity, ownership, revocation, and
usage.

Route resolution, dedicated-target ownership, shared-route bindings,
request/model validation, provider-secret injection, and provider-attempt
semantics remain unchanged. A human call is one logical customer request even
if Alzette performs several internal attempts.

### 11.3 Customer-visible attribution

Usage totals must reconcile across both actor kinds. The portal exposes:

- total organisation/project/environment consumption;
- a breakdown by `Human agent` and `Service account`;
- for the owner, safe human display name, company context, requests,
  tokens with finality, errors, last-used time, and revocation state;
- for an employee, their own safe attribution;
- no prompt/output content, conversation titles, productivity score, or
  behavioural ranking.

The actor shown is the identity that authenticated the inference request, not
the human who happened to view the dashboard or originally created a service
account. Historical actor attribution remains immutable after offboarding.

### 11.4 Offboarding and revocation race

“Immediate at the Alzette boundary” has one precise P0 meaning: after the
Alzette disable/revoke transaction commits, every **new** gateway request joins
the current identity/user/membership/grant/token state from PostgreSQL and is
rejected before creation of a logical request. The gateway and token broker
have no positive Alzette-authorization cache that can outlive that transaction;
the durable Casdoor login proves identity only.

An already authenticated in-flight buffered request or SSE stream may complete
under the gateway's bounded request timeout; P0 does not claim distributed
mid-stream cancellation. The audit records that it began before the cutoff.
This limitation appears in the owner confirmation and offboarding
runbook.

On a cached-token rejection, the proxy discards that `alz_u_` value. It may
attempt the exact pre-request mint/retry protocol once; a disabled local user,
identity, person, group membership, or grant makes the broker fail, so no
logical request or provider attempt is created. Unrelated organisation-owned service accounts and
other memberships remain active.

A locally cached Casdoor refresh credential does not weaken this guarantee. It
may renew proof of identity, but cannot create or restore an Alzette membership,
model allowance, grant, or route. Every context read and `alz_u_` mint resolves
current Alzette state, and the gateway still joins that state on every new
request. Offboarding therefore blocks the next request even when the person's
Casdoor refresh session has not yet expired.

Disabling an account only in the Casdoor administration console prevents later
authentication but is not immediate Alzette authorization revocation. Customer
offboarding and the operator command must commit the Alzette disable/revoke
transaction first, then best-effort disable the Casdoor identity. P0 does not
rely on an IdP webhook for this guarantee. Customer-federation deprovisioning
cannot be claimed until that synchronization path is tested.

## 12. Portal, email, and copy contract

### 12.1 Access workspace

Access contains four visibly separate areas:

1. **People** — owner-protected company roster, employees, invitations,
   resend/revoke/disable, employee groups/effective model endpoints, and the
   owner's all-active-endpoints relationship.
2. **Groups** — owner-managed employee membership and model-endpoint grants;
   employees see only their own groups and derived models.
3. **Your agent sessions** — identity method, context, client, created/last-used,
   expiry, and revoke/logout.
4. **Application access** — service accounts and one-time-reveal `alz_k_` keys.

Use “Invite an employee,” “Connect your agent,” “Sign in to Alzette,” and “Create
application key.” Do not use the ambiguous action “Create credentials.”

The portal reports whether browser login, device login, proxy, or a named
native provider is enabled from server capability data. It does not render a
button merely because a design exists.

The Go control service owns normal-link, server-rendered routes:

```text
/app/access
/app/access/people
/app/access/people/invite
/app/access/people/{person-id}
/app/access/groups
/app/access/groups/new
/app/access/groups/{group-id}
/app/access/agent-sessions
/app/access/applications
```

Core forms work without JavaScript through CSRF-protected `POST` plus
Post/Redirect/Get. Small vanilla modules may add list filtering, checked-item
summaries, focus management, and confirmation enhancement; they never become
the authority or a client-side router.

The owner People view separates active people from pending invitations. The
owner row is protected and offers no disable/remove action. Employee rows show
status, groups, derived model endpoint access, safe last sign-in, and a detail
action. The employee view shows only their own company access, groups, models,
and agent-session link; it does not expose a coworker directory. “Invite an
employee” asks only for exact work email, optional name, and one or more
server-supplied enabled groups. Review shows the company, groups, derived model
endpoints, readiness separately from assignment, fixed expiry, and the fact
that no personal API key is created.

The Groups view says “Groups decide which people can use which model
endpoints.” A group has a unique company-scoped name, optional description,
enabled/disabled state, employee memberships, and zero or more model endpoint
grants. A group with no model grant is valid but grants no inference. Group
detail uses independent forms for people and model endpoints so one edit cannot
overwrite the other. Endpoint removal and group disable show a reviewed impact
summary; records are retained for audit rather than hard-deleted. Assignment
and runtime readiness remain separate states.

The server renders explicit capabilities rather than one broad admin boolean:
`people.read_self`, `people.read_company`, `people.invite`, `people.disable`,
`groups.read_self`, `groups.read_company`, `groups.manage`,
`invitations.enabled`, `identity_login.enabled`, and
`workforce_access.enabled`. Only the current owner receives company-wide read
or mutation capabilities. When workforce identity, canonical HTTPS, mail, or
migration prerequisites are absent, mutation routes are not mounted and the UI
shows a truthful unavailable state instead of an Invite/Connect action.

People and Groups tables and forms remain usable at 320, 390, 1024, and 1440
CSS pixels, 200% zoom, keyboard-only, screen reader, forced-colour, and
reduced-motion settings. Native fieldsets/checkboxes are the baseline for
group/model selection. Every validation response provides an error summary
linked to inline errors; delivery status uses one restrained polite live
region. Email addresses, aliases, and group names wrap without hiding status or
actions.

### 12.2 Invitation email

Required copy:

- subject: `You have been invited to <Organisation> on Alzette`;
- Alzette and the inviting organisation in the first line;
- inviter, employee status, assigned groups, and expiry;
- one canonical HTTPS acceptance button/link;
- a statement that the employee will sign in as a person and will not receive
  an API key;
- a non-secret Alzette gateway/model summary only when evidenced;
- support route and safe expired-link guidance.

Recommended customer copy is deliberately non-technical:

> `<Inviter>` invited you to `<Organisation>` on Alzette as an employee in
> `<groups>`. Use the invited email address to sign in or
> create your Alzette identity. You will not receive or need a personal API
> key. Next, accept the invitation, sign in, then connect the supported AI tool
> for this workspace. This invitation expires `<date, time, and timezone>`.

Forbidden content:

- passwords, API keys, OAuth/device/agent tokens, provider secrets;
- provider target URLs, MeluXina LAN addresses, raw model slugs;
- usage figures, prompts, outputs, financial data, or tracking pixels;
- links derived from an untrusted request Host header.

### 12.3 Connect your agent

The authenticated page shows:

- current company and effective model endpoints—all active company endpoints
  for the owner, or group-derived endpoints for an employee—and internal
  project/environment context only where it helps identify the endpoint;
- service mode and execution evidence separately from authentication;
- gateway base URL and approved model aliases;
- recommended command for the supported client/OS;
- browser login, plus device login only when that deferred capability is
  actually enabled;
- the P0 local-proxy path, with native providers shown only after a named
  integration is evidenced;
- current login/session expiry and revoke action;
- an explicit application/service-account path for CI and servers;
- unsupported-client criteria and a safe support path.

Core copy:

> Sign in as yourself for interactive agent use. Alzette issues short-lived
> access automatically. Application keys are separate and remain intended for
> software and unattended workloads.

In the primary employee journey, **agent** means “the supported AI tool or
command-line client you want to use.” User-facing primary actions do not expose
`Casdoor`, OIDC, PKCE, `alz_u_`, provider, target, grant, or membership jargon.
Those exact terms remain in technical and operator documentation.

Required customer-facing states include:

- wrong identity: “This invitation is for a different invited email address.
  Sign out and try again, or ask the inviter to resend it.”;
- route unavailable: “Your organisation has not enabled an agent route for
  this workspace yet. Contact your company owner.”;
- unsupported client: “This client is not enabled for this workspace. Use
  `<named client/version>` or contact your company owner.”;
- expired identity session: “Your interactive session expired. Sign in again;
  no application key is required.”;
- connected, not yet verified: “Agent connection started. No successful
  inference has been observed yet.”;
- first success: “Connection verified,” with safe request ID, alias, and
  latency, and no prompt/output content.

Every state works with keyboard and screen reader, does not rely on colour,
announces polling/progress safely, honours reduced motion, and remains usable
at 320, 390, 1024, and 1440 CSS pixels.

## 13. Data model and migration

Use additive migrations beginning after the current `0011` schema. The planned
split is `0012_company_people_groups` for company authority and group policy,
`0013_workforce_identity_invitations` for federated identity and invitations,
and `0014_human_agent_access` for grants, short tokens, mint idempotency, and
ledger actor changes. Confirm the next unused numbers at implementation time;
do not rewrite the already-applied `0008` catalogue/onboarding groundwork or
`0011_endpoint_team_size`.

### 13.1 Company authority and group policy

Minimum additive records:

- `organisation_ownerships`: append-only ownership periods with
  appointment/transfer/recovery actor, evidence, and timestamps. A partial
  unique index permits at most one current owner per organisation; company
  creation, transfer/recovery, restore, and close transactions enforce exactly
  one at commit for every active organisation. Transfer atomically closes the
  current period, starts one for an eligible existing employee, and retains the
  prior owner as an employee.
- `organisation_people`: the enabled/disabled link between a human user and an
  organisation. Owner versus employee authority is derived from
  `organisation_ownerships`, not a browser-supplied role or group.
- `access_groups`: organisation-owned, stable ID, unique name, optional
  description, internal project/environment enforcement scope, enabled state,
  and lifecycle/audit timestamps. These are Alzette groups, never Casdoor or
  customer-directory claims.
- `access_group_people`: enabled person-to-group membership with composite
  organisation foreign keys and audit lineage.
- `access_group_models`: enabled group-to-customer-endpoint/model-alias grant
  with composite organisation/project/environment foreign keys. It references
  the Alzette route/endpoint identity, never a provider slug, target, host,
  credential, profile, or machine.
- `human_invitation_groups`: immutable initial-group snapshot for an
  invitation. Acceptance revalidates every group under the invited company.

One shared policy resolver computes effective model access for portal model
discovery, People/Groups previews, agent contexts, token minting, and gateway
authorization. No caller reimplements the join. Group/person/model disablement
or removal revokes affected active human grants/tokens in the same transaction,
or advances a checked policy generation that makes them unusable on the next
request. Service accounts remain independent.

Legacy `human_memberships.role` values remain readable during migration but do
not authorize new customer actions. Migration assigns an owner only through an
explicit deterministic and audited rule; ambiguous organisations require
operator reconciliation and the workforce feature remains disabled for them.

Ownership transfer is a high-assurance company-settings operation, never a
People-row role edit. The current owner must recently authenticate, select one
enabled same-company employee with a linked identity, and explicitly confirm.
The transaction locks company, ownership, and target; ends the old ownership,
starts the new one, retains the former owner as employee, revokes both portal
sessions and affected human-agent tokens, and audits both IDs and reason. One
concurrent transfer wins; failure leaves the old owner intact. The current
owner cannot be disabled, removed, or leave outside that transaction.
Operator-assisted recovery uses the same atomic replacement invariant with
evidence and audit; it never impersonates the owner or issues inference access.

### 13.2 `human_federated_identities`

Minimum fields:

- stable ID and linked `human_user_id`;
- exact normalized issuer and subject;
- provider kind and enabled status;
- safe email snapshot and verification time for linkage evidence only;
- linked, last-authenticated, disabled, and updated times;
- safe link source/evidence and no token material.

`(issuer, subject)` is globally unique. Email is not a uniqueness substitute.
Database guards prevent an enabled external user from being left without an
enabled identity unless an approved local authentication method remains.
No Casdoor access or refresh token is stored in this or any other Alzette
table. A safe opaque Casdoor session-family reference may be recorded only when
required for audited revocation and is never sufficient to refresh a session.

### 13.3 `human_agent_grants`

Minimum fields:

- stable ID, user ID, membership ID, OAuth client ID, and digest-only random
  client-instance identifier;
- organisation/project/environment copied under composite foreign keys;
- bounded scopes and explicit permitted model aliases;
- authentication, creation, absolute-expiry, last-used, revoked times;
- revocation actor/reason and safe client/device label.

A grant is not a route and cannot contain a target ID, target URL, provider
secret, provider model slug, hardware record, or price.

### 13.4 `human_agent_access_tokens`

Minimum fields:

- stable ID and grant ID;
- display-safe `alz_u_` prefix and SHA-256 token digest;
- issued, expires, last-used, and revoked times;
- generation and safe replacement lineage; P0 permits no active overlap within
  one grant.

Plaintext is returned once and never stored. Token rows referenced by the
immutable ledger remain safe historical evidence after expiry/revocation.

### 13.5 `human_agent_credential_mints`

Minimum fields:

- stable ID, grant ID, and SHA-256 idempotency-key digest;
- canonical request hash covering identity, OAuth client, client instance,
  membership, sorted alias set, and schema version;
- created token ID, operation state, creation/completion/replay times, and
  bounded safe error state;
- no plaintext idempotency key or agent token.

The idempotency-key digest is unique for the authenticated identity/client for
at least 24 hours. It coordinates the one-time-reveal response; it is not an
authentication credential.

### 13.6 Existing-table changes

- `human_users.password_hash` becomes nullable only with an authentication-
  method XOR constraint; existing bcrypt hashes remain valid during migration.
- `portal_sessions` records authentication method, optional federated identity,
  and IdP authentication time for step-up/recent-auth decisions.
- `inference_requests` receives the human-agent tuple and the service/human XOR
  constraint described above; existing service-account rows remain valid.
- rollup dimensions support nullable service-account attribution and explicit
  human-agent attribution without changing organisation totals.
- disablement triggers revoke portal sessions, human-agent grants, and agent
  tokens at the correct user or membership scope.
- append-only audit events accept safe federated-human and agent-grant actors.

The migration must preserve all existing users, sessions, service accounts,
API keys, logical requests, attempts, rollups, routes, targets, and endpoint
commercial records. Applying it does not enable Casdoor or invitations.

## 14. HTTP and CLI contracts

### 14.1 Portal and invitation routes

```text
GET  /login/oidc
GET  /login/oidc/callback
POST /logout
GET  /accept-invite?token=...
POST /accept-invite

GET  /api/portal/people
GET  /api/portal/invitations
POST /api/portal/invitations
POST /api/portal/invitations/resend
POST /api/portal/invitations/revoke
POST /api/portal/employees/disable
POST /api/portal/employees/reactivate
GET  /api/portal/access-groups
POST /api/portal/access-groups
POST /api/portal/access-groups/{id}/disable
PUT  /api/portal/access-groups/{id}/people/{person-id}
DELETE /api/portal/access-groups/{id}/people/{person-id}
PUT  /api/portal/access-groups/{id}/models/{endpoint-id}
DELETE /api/portal/access-groups/{id}/models/{endpoint-id}
POST /api/portal/ownership/transfer
GET  /api/portal/agent-grants
POST /api/portal/agent-grants/revoke
```

Cookie-authenticated mutations retain the existing CSRF and session-derived
tenant rules. Invitation input contains only `email`, optional `display_name`,
and `group_ids`; ownership, role, project/environment, endpoint, and tenant
authority fields are rejected. OIDC callback parameters never become inference
credentials.

### 14.2 Agent identity API

```text
GET  /.well-known/alzette-agent-configuration
GET  /api/agent/contexts
POST /api/agent/credentials
POST /api/agent/credentials/revoke
```

The well-known response contains only the canonical control/gateway origins,
configured Casdoor issuer, OAuth client identifier, resource audience,
supported login modes, API subset, and safe version metadata.

Agent identity APIs are bearer-only with a Casdoor access token bound to the
Alzette agent resource. They accept no cookies and emit no permissive CORS.

`POST /api/agent/credentials` accepts exactly:

```json
{
  "client_instance_id": "aci_random-process-id",
  "membership_id": "mem_opaque",
  "model_aliases": ["alzette-chat"]
}
```

It also requires exactly one `Idempotency-Key: agm_<random>` header containing
at least 128 random bits, with a bounded maximum length. The client generates a
new key for each intended mint and never logs it. The server hashes it before
persistence and binds it to the canonical request described in section 13.5.

It returns plaintext once:

```json
{
  "schema": "alzette.agent-credential.v1",
  "credential": {
    "access_token": "alz_u_…",
    "token_type": "Bearer",
    "expires_at": "2026-08-15T09:10:00Z",
    "scope": ["inference:write"]
  },
  "context": {
    "membership_id": "mem_opaque",
    "organisation": "Example Bank",
    "project": "Research",
    "environment": "Development"
  },
  "gateway_base_url": "https://inference.alzette.systems/v1",
  "model_aliases": ["alzette-chat"]
}
```

The token value is excluded from every retry/read endpoint. Mint behavior is
exact:

1. A new idempotency key and valid request atomically create one token, revoke
   the prior active token for that grant, store the completed mint record, and
   return plaintext once.
2. The same key with a different canonical request returns `409
   idempotency_conflict` and changes no token.
3. The same key with the same completed request returns `409
   credential_response_unrecoverable`; in the same transaction the server
   revokes the token produced by that mint if it is still active. No plaintext
   is returned.
4. After case 3, the client generates a new key and retries the intended mint.
   The single-active-token rule ensures an undisclosed earlier credential is
   not left usable.
5. An in-progress duplicate returns bounded `409 operation_in_progress`; the
   client waits and then follows case 3. It never silently changes the key on a
   transport timeout.

This protocol deliberately prefers an extra login/mint round trip over storing
recoverable plaintext or leaving an ambiguous credential valid.

### 14.3 Gateway

The customer data-plane contracts are:

```text
POST /v1/chat/completions
POST /v1/responses
Authorization: Bearer <alz_k_ service key OR alz_u_ human-agent token>

POST /v1/messages
Authorization: Bearer <alz_k_ service key OR alz_u_ human-agent token>
OR X-Api-Key: <alz_k_ service key OR alz_u_ human-agent token>
```

Chat Completions and Responses errors remain OpenAI-compatible and generic;
Messages errors use the corresponding Anthropic envelope. Safe detailed
identity errors belong to the local CLI/agent-identity API, not the public
gateway. The adapters support bounded text and function-tool semantics only;
stateful Responses, hosted tools, media/files, extended thinking, computer use,
and prompt-cache semantics fail closed.

### 14.4 Operator commands

```text
alzette identity casdoor configure --issuer ... --client-secret-file ...
alzette user invite --email ... --organisation-slug ... \
  --group group_...
alzette identity disable --user-id ...
alzette ownership recover --organisation-id ... --new-owner-user-id ... \
  --evidence-ref ...
alzette agent-grant revoke --grant-id ...
```

Operator output contains safe IDs/status only. Invitation URLs may be revealed
once in explicit manual-delivery mode; no OAuth, agent, service, or provider
credential appears in routine output.

## 15. One-machine deployment and stack

The cheapest P0 deployment remains one host:

```text
TLS ingress
  ├── public
  ├── portal/control
  ├── inference gateway
  └── Casdoor login endpoints

Docker Compose
  ├── gateway
  ├── control
  ├── public
  ├── worker
  ├── billing-webhook when enabled
  ├── migrate
  ├── casdoor (one replica)
  └── PostgreSQL
        ├── alzette database / least-privilege role
        └── casdoor database / separate least-privilege role
```

Only TLS ingress publishes remote ports. Casdoor does not receive the Alzette
database role; the gateway receives no Casdoor client secret and does not call
Casdoor. Casdoor signing material, database credential, portal client secret,
mail credential, and provider credentials are file-mounted and never
interpolated into Compose output.

One Casdoor replica is intentional, so P0 adds no Redis. If Casdoor becomes
multi-replica—or a deferred flow introduces cross-replica transient state—its
shared-state requirements and failure-mode testing become a new architecture
decision.

The employee's `alzette-agent` process runs on their workstation outside the
server Compose network.

The implementation introduces one all-or-nothing capability gate and explicit
configuration, tentatively:

```text
ALZETTE_WORKFORCE_ACCESS_ENABLED=false
ALZETTE_CASDOOR_ISSUER=
ALZETTE_CASDOOR_PORTAL_CLIENT_ID=
ALZETTE_CASDOOR_PORTAL_CLIENT_SECRET_FILE=
ALZETTE_CASDOOR_AGENT_CLIENT_ID=
ALZETTE_CASDOOR_AGENT_AUDIENCE=
```

The public agent client has no secret. The portal secret is file-backed. When
the capability is true, control, gateway, migration version, canonical HTTPS
origins, issuer discovery/audience, and required mail/invitation configuration
must all agree or the affected processes fail startup. Compose output and
well-known metadata never contain a secret.

Remote use requires canonical HTTPS origins, Secure cookies, HSTS, reviewed
proxy-header trust, certificate validation, backup/restore for both databases,
and a restricted Casdoor administration surface. The current LAN HTTP demo is
not sufficient evidence.

## 16. Casdoor configuration and acceptance spike

Before application implementation depends on Casdoor, pin one version/image
digest and prove:

- standard OIDC discovery and application-specific signing/JWKS behavior;
- Authorization Code with mandatory PKCE S256 for a public client without a
  client secret;
- exact loopback redirect handling with an ephemeral port;
- resource/audience binding for the Alzette agent identity API;
- access-token introspection and logout/revocation behavior;
- maximum-one-hour access tokens and the exact refresh scope required by the
  accepted provider contract;
- public-client refresh without an embedded or distributed client secret;
- refresh issuance, rotation on every use, prior-token invalidation,
  concurrent-use/reuse detection, family revocation, 30-day inactivity and
  90-day absolute expiry, logout, and administrative revocation;
- signing-key rotation and unknown-key fail-closed behavior;
- one-use, quota-one, exact-email invitation-controlled registration;
- minimal claims without unnecessary Casdoor user profile data;
- disabling implicit, resource-owner-password, client-credentials, dynamic
  registration, and unused providers for the public agent client;
- clean Compose startup, restart, backup, restore, and credential rotation.

If any property fails, do not compensate by accepting broader tokens, embedding
a client secret, trusting email claims, lengthening Alzette tokens, or bypassing
local membership checks. Change the pinned configuration/provider or add one
narrow, reviewed broker behavior.

## 17. Security and privacy requirements

- OAuth validation uses one configured HTTPS issuer. No request can choose the
  issuer, discovery URL, JWKS URL, introspection URL, or redirect origin.
- PKCE is S256-only; state, nonce, verifier, and authorization code are
  transaction-specific, bounded, single-use, and never logged. The same rule
  applies to device codes only if that deferred mode is enabled.
- Token validation allow-lists algorithms and validates signature, issuer,
  audience, authorised client, subject, expiry, not-before, and bounded skew.
- ID tokens and access tokens are not interchangeable. The token broker accepts
  only the configured resource-bound Casdoor access-token contract.
- Invitation acceptance requires an exact verified identity and an active local
  invitation. Domain matching never grants access.
- User or identity disablement blocks every local person/group membership and
  grant on the next new request. Group-person or group-model removal blocks
  only the affected access. Grant/token revoke blocks that agent session. The
  current owner cannot be disabled outside atomic transfer/recovery. In-flight
  behavior follows section 11.4.
- Gateway authentication checks current local state on every call, so
  offboarding does not wait for a Casdoor access token to expire.
- A refresh credential proves only an ongoing Casdoor identity session. It
  never bypasses current Alzette user, person, owner-or-group model policy,
  grant, token, or route checks and never enters the gateway.
- Persistent refresh storage follows section 9.1. Keyring failure does not
  silently create a file; explicit file mode rejects links, wrong ownership,
  broad permissions, non-atomic replacement, and backup/support inclusion.
- Refresh rotation is serialized per local profile. Reuse, ambiguous rotation,
  corrupt state, and revocation fail closed and require browser login.
- Logging, audit, analytics, traces, error bodies, support bundles, crash output,
  and customer APIs contain no password, token, authorization/device/user code,
  OAuth state/nonce, idempotency key, cookie, prompt, output, raw email where
  unnecessary, provider secret, or target URL. Enumerated machine-safe error
  classes are not secret codes.
- The token broker and proxy follow bounded body, timeout, redirect, and egress
  policies. They never fetch a user-provided URL.
- Local proxy HTTP is acceptable only on loopback with its own per-launch
  capability. Loopback locality alone is not authentication.
- There is no wildcard CORS. Unexpected browser Origins are rejected.
- Every distributed binary or package has a version, checksum, provenance, and
  pilot-approved distribution path; production customers require signed
  artifacts for their agreed platform.
- Metadata-only per-employee attribution is disclosed to the organisation and
  subject to an approved retention policy. It is used for access, security,
  support, and cost visibility, not covert productivity monitoring.

## 18. Observability

Safe metrics/events cover:

- invitations created, delivered, accepted, expired, revoked, resent, and
  rejected by safe reason;
- OIDC starts/callbacks and, only when enabled, device pending/slow-down/
  denied/expired/succeeded;
- identity links, login method, issuer identifier, subject-safe reference, and
  membership context without tokens or full claims;
- identity refresh succeeded/failed/reuse-detected, store kind, and safe
  idle/absolute expiry class without credential values or filesystem paths;
- agent grants/tokens created, minted, expired, revoked, and blocked;
- token-broker validation/introspection latency and unavailable/error class;
- local proxy starts/stops, login method, safe version, short-token mint,
  and rejected Host/Origin/path counts;
- human versus service-account logical requests, model alias, result, latency,
  token finality, and safe request ID;
- offboarding-to-denial latency;
- Casdoor/gateway/control readiness, signing-key refresh, mail/outbox state,
  and both-database backup/restore evidence.

Do not use high-cardinality raw subjects/emails as metric labels. Audit may
store safe internal IDs and a protected issuer/subject reference.

The smallest product funnel records only explicit, safe state transitions:

1. invitation delivery succeeded or failed;
2. the employee deliberately started acceptance after the clean page (scanner
   GETs and email opens never count);
3. membership activated;
4. the proxy-backed agent session became ready;
5. first authenticated inference succeeded or failed with a coarse reason;
6. a second successful inference occurred within seven days.

Aggregate reporting covers invitation-to-acceptance and acceptance-to-first-
success conversion, median/p90 time to first success, failure rate by stage and
named client/OS version, and organisations with at least one activated
employee. There is no email-open tracking, behavioural profiling, prompt/
output analytics, or employee productivity ranking.

## 19. Functional requirements

### P0 requirements

| ID | Requirement | Testable acceptance |
|---|---|---|
| WAA-P0-001 | The current owner can invite an exact employee with an exact initial group set | Company authority is session-derived; role/owner/tenant/project/endpoint fields are rejected; group IDs are same-company and revalidated; idempotency, expiry, resend rotation, revoke, two-tenant tests, and truthful assignment/readiness review pass |
| WAA-P0-002 | Invitation acceptance uses Casdoor identity but Alzette membership authority | Exact verified identity and active invitation are required; GET, domain match, Casdoor role, replay, and concurrent acceptance grant nothing |
| WAA-P0-003 | New external employees do not receive an Alzette password or personal `alz_k_` key | Casdoor owns authentication/recovery; acceptance creates no data-plane credential |
| WAA-P0-004 | Browser login uses a public client with Authorization Code, mandatory PKCE S256, and one bounded rotating refresh session | Wrong/plain/missing/replayed verifier, state, nonce, redirect, issuer, audience, client, or refresh response fails closed; only the exact accepted refresh scope is requested; code exchange and refresh work without an embedded client secret |
| WAA-P0-005 | Only evidenced login and client modes are advertised | Baseline P0 exposes browser PKCE plus one named proxy-backed client; device and native modes remain absent until their separate acceptance contract passes |
| WAA-P0-006 | Casdoor identity is mapped only through `(issuer, subject)` | Same-email alternate subjects and unlinked identities receive no contexts or credential |
| WAA-P0-007 | Agent context discovery returns only current effective model aliases | The owner receives all active company aliases; employees receive only current group-derived aliases. Cross-tenant, disabled, guessed, zero-group, disabled-group, removed-model, and unavailable-route contexts are absent/forbidden without enumeration; portal discovery, mint, and gateway policy sets match |
| WAA-P0-008 | Alzette mints a random ten-minute `alz_u_` credential bound to one membership and alias set | 256-bit token is returned once, only SHA-256 digest persists, one token is active per grant, and the exact replay/revoke/remint protocol leaves no ambiguous credential usable |
| WAA-P0-009 | Gateway auth keeps `alz_k_` and `alz_u_` paths separate | One exact header and strict size/prefix/encoding; Casdoor JWT, portal cookie, Basic password, duplicate/mixed headers, malformed/oversized values, and invalid whitespace/case variants fail |
| WAA-P0-010 | A human token resolves only its server-bound tenant route | No inference tenant selector exists; dedicated/shared and alias ownership tests remain fail-closed |
| WAA-P0-011 | Disablement and revocation stop the next new request at the Alzette boundary | User, identity, membership, grant, or token disablement prevents the next gateway call and cached-proxy remint; an already authenticated bounded stream may finish and is audited; Casdoor-only disablement is not misrepresented as local revocation |
| WAA-P0-012 | Service accounts remain unchanged for non-human workloads | Existing key issue/rotate/revoke, machine APIs, routing, and usage tests pass without altered behavior |
| WAA-P0-013 | The ledger represents service and human actors truthfully | Actor XOR/composite FKs pass; historical rows migrate; totals reconcile; one logical request remains separate from attempts |
| WAA-P0-014 | The owner and each employee can see permitted safe consumption | Owner company view and employee self view are tenant-safe, reconcile to totals, show finality, and contain no content/productivity ranking |
| WAA-P0-015 | The local proxy satisfies one named key-only client without exporting a remote credential | `run` passes a 256-bit per-launch capability only through the child environment; loopback-only listener, exact Bearer check/upstream, header stripping, no argv/file/log persistence, cleanup, and remote-bind failure tests pass |
| WAA-P0-016 | Proxying preserves the supported agent protocol | Buffered/SSE text and tool calls, cancellation, first byte, terminal usage, errors, and request IDs match direct gateway behavior |
| WAA-P0-017 | Short-token minting cannot duplicate inference | Only `401` plus the exact `not-created` marker and `human_token_inactive` code may trigger one mint/retry; ambiguous transport, provider error, missing/conflicting marker, or post-byte paths never replay |
| WAA-P0-018 | Access and Docs explain the credential boundary and current capability | People, agent sessions, and application access are separate; endpoint/model evidence is truthful; disabled workforce/device/native modes do not mount or render as enabled |
| WAA-P0-019 | P0 runs on one machine without Redis | Pinned one-replica Casdoor and separate DB role start/restart/restore under Compose; secrets are file-mounted and only ingress publishes remote ports |
| WAA-P0-020 | Remote employee use fails closed without production transport/configuration | Canonical HTTPS, TLS ingress, Secure cookies, issuer/resource configuration, mail, backup, and security review are release prerequisites |
| WAA-P0-021 | One named employee journey reaches a truthful first inference without security jargon or opaque-ID handling | With one context, the recommended `run --verify` command authenticates, selects it, creates exactly one verification request, reports alias/request ID/latency, and starts the named agent only on success; multiple contexts use a human-readable choice |
| WAA-P0-022 | Employee login survives client restart without persisting a gateway credential | Access token is at most one hour and memory-only; the refresh family defaults to 30-day inactivity/90-day absolute bounds, rotates on every use, detects reuse, and is stored in keyring by default, explicit restricted-file mode, or memory; concurrent refresh, crash, logout, revoke, leak, and restart tests pass |

### P1 requirements

| ID | Requirement | Testable acceptance |
|---|---|---|
| WAA-P1-001 | A contracted customer can federate through its required OIDC/SAML method | Issuer mapping, MFA, recovery, identity linking, deprovisioning, and break-glass tests pass without creating ownership or group authority from identity-provider claims |
| WAA-P1-002 | Directory lifecycle may synchronize employees/groups through SCIM/JIT when required | User/group changes produce explainable local employee/group changes within the contracted mapping and are audited; only the Alzette owner can transfer ownership |
| WAA-P1-003 | Company owners may enforce stricter session and credential-custody policy | Idle/absolute duration and allowed keyring/file/memory modes can only narrow platform ceilings; managed policy, audit, recovery, and two-tenant tests pass |
| WAA-P1-004 | Additional native agents are supported only by tested adapters | Base URL, OAuth, storage, refresh, API subset, streaming/tool, and revocation contract passes per version |
| WAA-P1-005 | Company owners can review active human-agent access | Inventory/export shows safe actor, context, client, creation/last use/expiry/revoke and excludes secrets/content |
| WAA-P1-006 | Sender-constrained tokens may be required for higher-risk customers | DPoP or equivalent proof, replay prevention, key rotation, recovery, and proxy/native support pass before being claimed |
| WAA-P1-007 | A named pilot may enable Device Authorization for headless use | Discovery, pending interval, `slow_down`, denial, expiry, cancellation, approval context, code secrecy, restart behavior, and success pass before the mode is advertised |

## 20. Verification plan

QA is an ordered release flow, not a collection of optional checks. A required
gate cannot pass by skipping because PostgreSQL, a browser, or Casdoor was not
configured. Every automated gate uses at least two synthetic companies, fixed
clocks/keys where deterministic, and canary scans across logs, audit, database,
HTTP responses, browser history, screenshots, and traces. Any cross-company
visibility, reusable credential leak, partial ownership/invitation mutation,
stale group entitlement, or unsupported capability claim fails the release.

| Gate | Required evidence | Pass boundary |
|---|---|---|
| Q0 — contract and capability | Documentation consistency, fresh/upgrade/down-reapply migration plan, disabled workforce routes absent, incomplete enabled configuration refuses startup | Proves only that unimplemented/unsafe capability cannot be advertised |
| Q1 — company authority and policy | PostgreSQL/domain tests for exactly one owner, owner all-active-endpoint access, atomic transfer/recovery, employee lifecycle, groups, group union, zero employee access, tenant composite FKs, audit, and policy-generation invalidation | Owner/People/Groups persistence and policy are deterministic and fail closed; no invitation, OAuth, or inference claim yet |
| Q2 — identity and HTTP | Fixed-key fake OIDC discovery/JWKS/token/introspection plus real disposable PostgreSQL; PKCE/state/nonce/replay, exact identity, scanner-safe invitation, CSRF, non-enumeration, and portal-session tests | Exact invited employee reaches only the invited company/groups; still no real-Casdoor or remote claim |
| Q3 — browser and gateway | Browser journey owner invite → OAuth → deliberate acceptance → group-filtered Models; gateway rejects OAuth JWT and accepts only digest-backed alias-bounded `alz_u_`; group removal/offboarding denies the next call; `alz_k_` regressions pass | Proves the complete deterministic software journey for the named fake IdP/target |
| Q4 — pinned Casdoor acceptance | Digest-pinned isolated Casdoor with discovery/JWKS, public-client PKCE, resource audience, refresh rotation/reuse-family revoke, introspection/logout, disablement, signing-key rotation, restart, and backup/restore | Proves provider compatibility, not production remote OAuth |
| Q5 — named pilot | Canonical HTTPS origins, Secure cookies, actual mail/sender, scanner behavior, real browser on named OS/client, signed artifact, both-database restore, measured offboarding, support/recovery runbooks, and independent review with no unresolved critical/high issue | Only this gate permits a remotely usable employee OAuth/invitation pilot claim |

At Q1/Q2 the UI may expose only the capabilities proved at that gate. Until Q5,
customer-facing surfaces must not claim production invitations, remote employee
OAuth, customer federation/SCIM, general agent/OS support, or an offboarding
SLA. A local Casdoor container is protocol evidence, not availability evidence.

### Deterministic default tests

- fake OIDC discovery/JWKS/introspection with fixed test keys and no network;
- issuer, algorithm, signature, audience, client, subject, expiry, not-before,
  skew, unknown-key, inactive-token, and IdP-unavailable cases;
- PKCE/state/nonce/code replay and loopback redirect tests;
- exact refresh-scope request; refresh issuance and rotation; invalidated-token
  reuse/family revoke; idle/absolute expiry; logout/operator revoke; malformed or
  missing rotation response; concurrent processes; and a crash between remote
  rotation and atomic local replacement requiring browser login;
- keyring success/unavailable/locked behavior; memory-mode restart; explicit
  file-mode owner/mode/link/atomic-write checks; organisation-policy override;
  and leak scans proving that access/ID/`alz_u_`/local keys never persist and
  refresh material appears only in the selected protected store;
- invitation new/existing user, scanner GET, wrong identity, replay, concurrent
  accept, expiry, revoke, resend generation, delivery failure, rejected role or
  owner fields, same-company group validation, and exactly-one-owner cases;
- redirect-following scanner GETs create no acceptance, identity, membership,
  grant, token, third-party request, referrer leak, cacheable response, or
  reusable authorising cookie; repeated GETs do not consume the invitation;
- identity-link duplicate subject, duplicate email/different subject, disabled
  user/identity/person/group, unverified email, Casdoor role/group/organisation
  claims, valid-but-unlinked identity, and legacy-link cases;
- agent context/credential exact JSON, unknown fields, duplicate headers,
  canonical request hashing, same-key conflict/replay/in-progress behavior,
  lost response, new-key remint, alias subsets, expiry, digest-at-rest,
  single-active-token rule, and revoke;
- gateway service/human credential dispatch with duplicate headers,
  whitespace/case variants, Basic/JWT/cookie input, malformed/oversized
  `alz_k_`/`alz_u_` values, cross-tenant routes, dedicated ownership, shared
  explicit bindings, absent aliases, and scope denial;
- human/service actor XOR migration and ledger/rollup reconciliation;
- one request/multiple attempt, missing/partial usage, timeout, cancellation,
  streaming first-byte and no-retry tests for both actor kinds;
- proxy loopback bind, random port/capability, Host/Origin/CORS, absolute URL,
  unsupported method/path, duplicate auth, header stripping, shutdown, and
  token/log/request-body leak scans, including child environment versus argv,
  stdout, config files, inherited secret variables, and remote-bind failure;
- proxy buffered/SSE byte-equivalence, tool deltas, cancellation, request ID,
  proactive short-token mint, exact-marker single auth retry, conflicting/
  absent marker, lost response, and ambiguous failure no-replay;
- offboarding with an already issued token and cached proxy credential denies
  the next new call/remint even when a refresh session remains usable,
  preserves unrelated service accounts, and records the documented bounded
  in-flight-stream behavior;
- group union, zero-group, zero-model, add/remove person, add/remove model,
  group disable, concurrent policy change, and portal/agent/gateway effective-
  access equivalence; additions require remint and removals deny the next call;
- ownership creation, fresh-auth transfer, concurrent/stale transfer, rollback,
  current-owner disable rejection, session/token invalidation, and audited
  operator recovery; every commit leaves exactly one current owner;
- portal two-tenant member/session/usage visibility, keyboard, screen-reader,
  reduced motion, 320/390/1024/1440 layout, feature disabled/misconfigured
  startup behavior, and truthful unavailable states;
- migration current-upgrade, fresh-up, rollback, down/reapply, concurrent
  operations, and preservation of all historical actor rows;
- `go test -race ./...`, `go vet ./...`, dependency/license scan, Compose config,
  clean build/start/restart, and backup/restore.

### Opt-in Casdoor integration evidence

- one pinned Casdoor container with real discovery, JWKS, PKCE,
  resource-bound maximum-one-hour access token, rotating refresh issuance,
  prior-token invalidation/reuse response, idle/absolute expiry, introspection,
  logout/family revocation, disablement, and signing-key rotation;
- one real invitation-controlled Casdoor signup linked to an Alzette invitation;
- browser login from the first supported workstation OS;
- one proxy-backed agent streaming tool call through the deterministic target;
- mail delivery through the selected transactional adapter;
- TLS ingress using the canonical portal/auth/gateway origins;
- no provider credential is required for this identity release gate.

### Independent release review

Before an external financial-client pilot, an independent reviewer must find
no unresolved critical/high issue in invitation, OIDC validation, token mint,
tenant isolation, proxy binding/header policy, offboarding, secret leakage,
request replay, Compose isolation, or recovery.

## 21. Delivery increments

Implementation checkpoint (2026-08-18): the local W1–W3 vertical slice is
runnable. It includes owner-managed People/Groups/Application access, manual
invitations, pinned-Casdoor browser acceptance, public PKCE token exchange,
group-filtered contexts, `alz_u_` mint/revoke, strict gateway use, and human
ledger attribution. The increment descriptions below remain the completion
contract; ownership transfer/recovery, mail, the durable agent/proxy, the full
Casdoor lifecycle matrix, and W5 remote-pilot evidence are still open.

### Increment W0 — Casdoor acceptance spike

- pin the image/version and minimal one-replica configuration;
- prove PKCE, audience/resource, access-token expiry, refresh issuance/rotation/
  reuse detection/family revoke, idle/absolute limits, introspection,
  invitation, disablement, JWKS rotation, restart, and backup/restore;
- record exact enabled/disabled grants and token claim contract.

**Exit:** Casdoor passes section 16 without weakening any Alzette invariant.

### Increment W1 — company authority, Access foundation, and Groups

- additive exactly-one-owner, company-person, group membership, and
  group-to-endpoint policy schema with tenant constraints;
- normal-link server-rendered Access shell, owner-protected People read view,
  and Groups list/create/detail/disable with separate people and model forms;
- one effective-access resolver contract and implementation, used immediately
  by People and Models and required for later agent-context, token, and gateway
  consumers;
- ownership transfer/recovery, audit, policy-generation invalidation, and
  feature flags.

**Exit:** the owner can manage groups and active company endpoints; the owner
sees every active company endpoint, employee access resolves only from enabled
groups, and no invitation or OAuth capability is advertised yet.

### Increment W2 — federated identity and employee invitations

- additive federated-identity, invitation, setup-session, OIDC, and mail-outbox
  schema and services;
- exact invitation acceptance, existing-user linking, and immutable initial
  group snapshot with same-company revalidation;
- People invite/resend/revoke/disable/reactivate UI and employee self view;
- retain legacy local login during controlled migration.

**Exit:** the owner invites an employee; the exact employee authenticates
through Casdoor and enters only that company with the recorded initial groups,
without an Alzette password or any path to ownership. Removing group access
blocks the next discovery or remint.

### Increment W3 — human-agent token and accounting

- agent context/credential API and short `alz_u_` token generator;
- distinct gateway authenticator and generalized principal;
- ledger/rollup actor XOR migration and usage attribution;
- disable/revoke behavior and service-key regressions.

**Exit:** a short human token calls only its bound model/context, appears once in
usage under the human actor, and the next new request is denied after the local
offboarding transaction.

### Increment W4 — local proxy and first client

- separate `alzette-agent` Go binary;
- browser login, automatic in-process identity refresh, context selection,
  short-token mint, memory-only `login` diagnostic, `run`, and Pi shorthand;
  no background daemon or persistent gateway token;
- loopback/header/privacy/retry hardening;
- Pi 0.84.2, Jan Desktop 0.8.4, and Goose Desktop 1.46.0 text-stream
  compatibility evidence on local Linux.

Implemented local-demo subset: the helper plus the named Pi, Jan, and Goose
paths satisfy credential custody and first text inference. A protected
rotating-refresh store, durable `login status`/`logout`, automatic native-
client configuration, signed cross-platform packaging, function-tool
compatibility through the human path, and broader client version/OS evidence
remain before this increment's full exit.

**Exit:** an invited employee starts an agreed key-only agent through one login
without copying a remote credential; streaming/tool behavior and accounting
match a direct call, and the employee reaches the first verified request without
knowing a context ID, provider, target, token type, Casdoor, or OAuth term.

### Increment W5 — remote pilot release

- canonical TLS ingress, secure cookies, transactional mail, signed artifact,
  authenticated sender domain, delivery/failure/resend evidence, monitoring,
  backup/restore, support and offboarding runbooks;
- opt-in Casdoor/browser evidence and independent security review;
- truthful portal/docs capability flags.

**Exit:** the invited-employee workflow is safe to expose to the named pilot;
the current LAN HTTP demo is no longer part of the access path.

## 22. Release gates

### Documentation gate

This PRD and its references are internally consistent. Current product
documentation labels the invitation/OAuth/human-credential path as implemented
for the tested local Compose configuration and keeps mail, TLS, durable client,
recovery, and remote-pilot claims explicitly unavailable.

### Offline software gate

W0–W4 deterministic tests pass with a fake IdP and fake inference target. No
real mail, provider, or Casdoor claim is made from deterministic evidence.

### External pilot gate

Go only when W5 passes, one supported OS/agent/version is named, the customer
accepts the per-employee metadata policy, the actual Casdoor/mail/TLS paths are
tested, and offboarding denial is measured.

### Financial-client production gate

The external pilot gate plus contracted MFA/federation, retention, access
review, incident/support, artifact distribution, recovery, and independent
security requirements pass. This gate is separate from proving a dedicated
MeluXina target.

## 23. Open decisions and owners

| Decision | Owner | Needed by |
|---|---|---|
| Canonical portal, auth, inference, and mail origins | Founder/platform | W0/W2 |
| First supported employee OS and agent/version | Founder/first pilot/platform | W4 |
| Fixed verification request, allowance treatment, and route-readiness rule | Product/platform | W3/W4 |
| Confirm the provisional one-hour access-token, 30-day inactivity, and 90-day absolute refresh-session ceilings against pinned Casdoor behavior | Security/product | W0 |
| Transactional mail provider and sender domain | Founder/platform | W2 |
| Initial per-employee usage visibility and retention policy | Founder/customer/legal | W3/pilot gate |
| Pilot funnel thresholds for invite acceptance, first success, and repeat use | Founder/growth/first pilot | W4/W5 |
| Binary distribution, signing, checksum, and upgrade owner | Platform/security | W4/W5 |
| Whether a later Pi native OAuth path delegates to the proxy or passes direct storage review | Platform/security | P1 |
| Casdoor patch, signing-key, backup, restore, and break-glass owner | Platform/security | W0/W5 |
| Whether a production customer requires shorter sessions, keyring-only custody, DPoP, customer SSO, or SCIM | Customer/security | Production gate |

None of these decisions permits a temporary personal API key as a silent
substitute. A pilot may remain proxy-only, but the default employee experience
reuses the protected refresh session until its idle/absolute bound or revoke.

## 24. Definition of done

This feature is done for P0 when:

- the current owner invites an exact employee into an exact initial group set;
- the employee authenticates through Casdoor and becomes an Alzette member only
  after the atomic invitation transaction;
- the employee can run the first supported agent through browser login without
  seeing or copying a remote long-lived credential;
- after a successful login, closing and reopening the client does not require
  another browser login while the rotating refresh session remains valid;
- only the protected refresh credential persists; access, `alz_u_`, and local
  proxy credentials remain memory-only and automatic rotation/logout pass;
- the gateway derives the exact tenant/model scope from a short `alz_u_` token
  and never accepts a Casdoor token directly;
- the local proxy is loopback-only, process-scoped, protocol-transparent, and
  content/credential silent;
- disabling the employee or removing their group access blocks the next
  inference request;
- service accounts continue to operate independently;
- company totals reconcile across human and service actors, with safe
  per-employee attribution and no prompt/output persistence;
- invitation, OAuth, token, tenant, proxy, replay, ledger, migration, browser,
  Compose, backup/restore, and independent security gates pass;
- portal, email, and docs clearly distinguish human login, short agent access,
  local proxy compatibility, service-account keys, and provider credentials;
- no current product surface claims the feature before its corresponding live
  evidence exists.

## 25. Primary references

- [OpenAI Codex authentication](https://learn.chatgpt.com/docs/auth#login-caching) —
  product comparator for cached login, automatic access-token refresh, explicit
  keyring/file credential-store modes, login status, and logout behavior
  (accessed 2026-08-15). Alzette adopts the durable experience, with stricter
  no-silent-file-fallback and next-request local authorization checks.
- [Casdoor repository and licence](https://github.com/casdoor/casdoor) —
  self-hosted Go identity server, OAuth/OIDC, MFA, organisations, and Docker
  support (accessed 2026-08-15).
- [Casdoor OAuth documentation](https://casdoor.ai/docs/how-to-connect/oauth/) —
  Authorization Code, PKCE, Device Authorization, resource indicators, refresh,
  and token verification behavior (accessed 2026-08-15). The public page shows
  refresh issuance/expiry but does not establish rotation, reuse-family
  revocation, or secretless public-client refresh; W0 must prove those exact
  properties rather than infer them.
- [Casdoor invitation documentation](https://casdoor.ai/docs/invitation/overview/) —
  application/organisation invitation codes, single-use quota, and exact email
  fields (accessed 2026-08-15).
- [Pi custom-provider OAuth contract](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/custom-provider.md) —
  browser/device callbacks, access-token renewal, and `/login` integration
  (accessed 2026-08-15).
- [RFC 9700, OAuth 2.0 Security Best Current Practice](https://www.rfc-editor.org/rfc/rfc9700.html) —
  PKCE, redirect, replay, token, and client security requirements.
- [RFC 8252, OAuth 2.0 for Native Apps](https://www.rfc-editor.org/rfc/rfc8252.html) —
  public clients and loopback IP redirect guidance.
- [RFC 8628, OAuth 2.0 Device Authorization Grant](https://www.rfc-editor.org/rfc/rfc8628.html) —
  device/user code, polling, expiry, denial, and `slow_down` behavior.
