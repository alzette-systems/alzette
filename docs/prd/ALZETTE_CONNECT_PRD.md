# Alzette Connect PRD

**Status:** implementation in progress; desktop launcher and custody core implemented, named-client and signed-release acceptance not complete

**Date:** 2026-08-19

**Owners:** product, desktop, platform, security, design, quality, and operations

**Related documents:** [`WORKFORCE_AGENT_ACCESS_PRD.md`](WORKFORCE_AGENT_ACCESS_PRD.md)
owns employee identity, entitlement, credential, proxy, revocation, and accounting
invariants; [`PORTAL_PRD.md`](PORTAL_PRD.md) owns company administration;
[`PRODUCT.md`](../product/PRODUCT.md) defines the overall product promise; and
[`POC_BOUNDARY.md`](../product/POC_BOUNDARY.md) controls claims about what the
running proof of concept currently proves.

This document owns the employee-facing connection product: the
`alzette-connect` command-line interface, the Alzette Connect desktop launcher,
application adapters, supported protocol matrix, local lifecycle, packaging,
and release evidence. It does not weaken or replace the workforce-access
security contract.

## 1. Decision

Alzette will ship **Alzette Connect**, a small desktop application with a
matching CLI that signs an employee in and launches an approved AI application
against the models assigned by their company.

Alzette Connect is a launcher and connection controller, not another general
chat client. The employee keeps using a supported application such as Pi, Jan,
Goose, ChatGPT, or another explicitly qualified client. Connect
owns the difficult boundary around that application:

1. browser-based Alzette login;
2. company, project/environment, and full model-catalogue discovery;
3. application and model delivery limited to current employee entitlements;
4. creation of an authenticated loopback connection;
5. application-specific, reversible configuration;
6. launch and lifecycle supervision;
7. short-credential renewal while the session is active; and
8. proxy closure and grant revocation when the session ends.

The desktop application and CLI use one launcher core. They must not implement
separate login, authorization, proxy, configuration, or revocation behavior.
The current `alzette-agent` binary is the prototype of that shared core. The
product command and employee-facing name become `alzette-connect` once the
migration, compatibility alias, packaging, and documentation are ready.

Ollama's open-source launcher is useful implementation precedent for
application detection, isolated profiles, reversible configuration, model
catalog generation, and cross-platform process launch. Alzette does not adopt
Ollama account authentication or make Ollama a required runtime. Any reused
MIT-licensed source is recorded in the distributed third-party notices.

## 2. Product outcome

An invited and entitled employee can install Alzette Connect, sign in through
their browser, and launch an approved application with every compatible model
currently assigned by their company, without seeing or copying an Alzette
remote credential. A model is selected inside the launched application when
it supports a catalogue; Connect asks for one model only when that
application's adapter requires a single launch model.

The primary desktop journey is:

```text
Open Alzette Connect
      -> sign in in browser
      -> choose company context when more than one is available
      -> Connect synchronises all currently assigned models
      -> see installed/supported applications and compatible-model counts
      -> double-click an application or select it and choose Launch
      -> Connect supplies a catalogue, primary-plus-catalogue, or one model
         according to the reviewed application adapter
      -> use the selected application normally
      -> quit or disconnect
      -> local proxy closes and Alzette grant is revoked
```

The equivalent terminal journey is:

```console
$ alzette-connect launch pi
Opening Alzette sign-in...
Signed in as alice@example.lu
Company: Example Bank
Context: Research / Development
Models available to Pi: 4
Launching Pi through Alzette...
```

The employee does not need to understand API keys, OAuth, Casdoor, proxy ports,
provider URLs, raw model targets, or Alzette credential types. Company owners
continue to control people, groups, and model access in the portal; Connect
does not create or override entitlement.

## 3. Current baseline

### Implemented local prototype and desktop candidate

The repository currently provides:

- a separate Go `alzette-agent` executable;
- browser Authorization Code login with PKCE;
- group-filtered company context and model-alias discovery;
- interactive context selection;
- maximum-ten-minute `alz_u_` minting and reminting;
- an authenticated random-port loopback proxy;
- process-scoped `OPENAI_BASE_URL` and `OPENAI_API_KEY` injection;
- grant revocation and proxy shutdown when the launched child exits;
- an isolated Pi provider extension; and
- verified local Linux text-stream paths for Pi 0.84.2, Jan Desktop 0.8.4,
  and Goose Desktop 1.46.0;
- a separate `alzette-connect` Wails desktop repository and product binary;
- macOS Keychain, Windows Credential Manager, and Linux Secret Service refresh
  storage implementations with no plaintext fallback;
- the approved signed-out, context-choice, launcher, preparing, running,
  disconnect, and recovery UI driven only by native snapshots;
- passive Pi/Jan/Goose discovery with presence distinct from qualification;
- Pi 0.84.2 explicit-launch qualification and isolated provider launch;
- reversible Jan 0.8.4 and Goose 1.46.0 configuration with protected local
  capability storage, backup, atomic writes, and stale-safe rollback;
- a disabled-by-default reversible ChatGPT desktop candidate on macOS, generated
  all-model catalogue, child-process-only loopback capability, supervised
  launch, and bounded local Responses forwarding;
- per-launch human credential and loopback proxy creation separated from
  browser sign-in;
- supervised process handles, explicit disconnect, grant revocation attempt,
  and truthful incomplete-cleanup state; and
- an integrity-checked internal update channel and unsigned internal packages.

### Not implemented

The current implementation does not yet provide:

- the complete `alzette-connect` CLI command contract or `alzette-agent`
  compatibility migration;
- durable crash-journal recovery for an interrupted application-profile
  restore;
- Claude Code, Codex CLI, VS Code, or other launcher adapters;
- complete ChatGPT Responses named-client compatibility beyond the currently
  bounded text/tool protocol and generated model catalogue;
- signed/notarized installers, production update signing, managed deployment,
  or independently evidenced rollback;
- production TLS and remote-pilot evidence; or
- a supported cross-platform/version matrix.

No product surface may present a missing adapter or protocol as available just
because its executable can be found on the workstation.

## 4. Scope and priorities

### P0 — credible Connect product

P0 includes:

- one shared launcher core used by CLI and desktop;
- the `alzette-connect` CLI with a temporary `alzette-agent` compatibility
  path;
- a small signed desktop launcher for one named operating system;
- browser login, protected login reuse, status, and logout;
- safe context selection and synchronisation of the assigned model catalogue;
- application discovery with truthful installed/supported/blocked states;
- one-click launch for Pi;
- automatic reversible connection for the named Jan and Goose versions;
- reversible ChatGPT desktop connection for one named macOS version, with the
  compatible company catalogue over the bounded Responses contract;
- active-session status and explicit disconnect;
- per-launch proxy, local capability, child/profile lifecycle, and revocation;
- structured launcher events shared by terminal and desktop views;
- exact protocol and client-version capability gates; and
- local deterministic tests plus a real named-client acceptance run.

### P1 — broader agent launcher

P1 includes, one adapter at a time:

- Codex CLI after OpenAI Responses compatibility passes;
- OpenCode, Qwen Code, VS Code, and other selected employee tools;
- a second and third named desktop operating system;
- enterprise-managed installation and update policy;
- multiple concurrent launched clients with one isolated grant and proxy per
  session;
- optional headless/device authorization where an approved pilot requires it;
  and
- company policy restricting which Connect adapters employees may use.

### Explicit non-goals

Connect does not:

- provide its own general chat, document, coding, or autonomous-agent UI;
- install or update third-party applications without explicit user or managed
  IT approval;
- make every OpenAI-compatible application supported;
- expose provider credentials, raw targets, OAuth tokens, `alz_u_` tokens, or
  reusable application keys;
- let an employee select a model not assigned by current Alzette policy;
- translate arbitrary unsupported protocols under a generic compatibility
  claim;
- authorize from local configuration, client claims, or an Ollama device key;
- run an unattended workload under an employee identity; or
- replace the `alz_k_` service-account path for applications, CI, servers, and
  scheduled jobs.

## 5. Product surfaces

### 5.1 Desktop application

The desktop application is the primary employee experience. It contains only:

- current sign-in and company context;
- synchronised assigned-model count and inspectable catalogue;
- detected application choices, compatible-model counts, and support state;
- launch controls;
- active session status; and
- disconnect, sign-out, help, version, and diagnostic actions.

It may remain in the system tray while a launched application is active. A
window close must have an unambiguous policy: either keep Connect running with
a visible tray state or ask whether to disconnect. It must never silently stop
supervising a live proxy while presenting itself as closed.

### 5.2 Command-line interface

The CLI is a first-class surface for developers, support, automation of
interactive launches, and deterministic verification. It is not merely a
debug back door for the desktop application.

The target command contract is:

```text
alzette-connect login
alzette-connect login status
alzette-connect logout
alzette-connect contexts
alzette-connect models [--context <opaque-id>]
alzette-connect applications
alzette-connect launch <application> [--model <alias>] [--context <opaque-id>]
                          [--verify] [-- <application arguments...>]
alzette-connect disconnect <session-id>
alzette-connect version
alzette-connect doctor
```

Interactive `launch` may ask the employee to choose among safe labels. The
`--model` option selects or overrides the primary model for adapters that
require or support one; it does not silently discard other compatible models
from catalogue-capable adapters. A non-interactive invocation must provide
every adapter-required ambiguous selection explicitly and fail closed; it must
not select an arbitrary company context or single launch model.

For a bounded migration period, `alzette-agent` may remain as an alias that
prints the replacement command. Removal requires documented notice and pilot
confirmation that managed scripts no longer depend on it.

### 5.3 Shared launcher core

The core is a Go package, not duplicated shell behavior. It owns:

- login/session access;
- context and model resolution;
- application adapter registry;
- proxy and grant lifecycle;
- process/profile supervision;
- structured events and errors;
- cleanup and restoration; and
- support-safe diagnostics.

The desktop shell may call the core in-process or supervise a bundled CLI over
a private structured channel. If it uses a child process, secrets must never
appear in arguments, stdout, general JSON events, crash reports, or desktop
logs.

## 6. Identity and credential boundary

The workforce-agent PRD remains authoritative. In summary:

- the employee authenticates using Alzette's configured OIDC provider through
  Authorization Code with PKCE;
- a valid identity-provider account alone grants no Alzette access;
- Alzette resolves current local membership and group/model entitlement;
- identity-provider access tokens and `alz_u_` credentials remain memory-only;
- only the approved rotating refresh session may persist in the selected
  protected credential store;
- every launch gets a new local capability, grant, and proxy;
- the remote gateway never accepts the local capability; and
- the next request is denied after membership, group, endpoint, or grant
  revocation according to the workforce-access contract.

Connect must not replace this with Ollama's Ed25519 device-key account flow or
another application vendor's login. Third-party application login, sync, and
telemetry are separate data paths and must be disclosed for each adapter.

## 7. Launch lifecycle

Every launch follows one state machine:

```text
idle
  -> authenticating
  -> resolving_context
  -> resolving_model
  -> checking_application
  -> preparing_profile
  -> starting_proxy
  -> launching
  -> active
  -> disconnecting
  -> restoring_profile
  -> revoked
```

Any failure after `preparing_profile` enters cleanup before returning to idle.
Cleanup is idempotent and attempts, in order, to stop accepting new local
requests, terminate or detach according to the adapter contract, revoke the
Alzette grant, restore owned configuration, delete temporary material, and
release locks. A failed restoration is surfaced with exact safe file names and
manual recovery guidance; it must not be hidden behind a generic launch error.

Application crashes, Connect crashes, workstation suspend/resume, network
loss, expired identity session, and forced operating-system shutdown all need
explicit tests. Short remote credential expiry limits exposure when immediate
revocation cannot complete.

## 8. Application adapter contract

Each supported application has a versioned adapter that declares:

- stable adapter ID and user-facing name;
- supported operating systems, architectures, application versions, and
  installation forms;
- detection rules that do not execute untrusted candidates;
- required inference protocol and features;
- supported model capabilities and context requirements;
- model-delivery mode: `catalogue`, `primary_plus_catalogue`, `single`, or
  reviewed `autodiscovery`;
- whether configuration is environment-only, isolated profile, managed file,
  plugin/extension, or application API;
- files, keychain entries, environment variables, and processes it may touch;
- backup, locking, restoration, and crash-recovery behavior;
- launch arguments and conflicting arguments it rejects;
- full-exit detection and disconnect behavior;
- vendor telemetry, account, and external-service disclosures; and
- acceptance evidence and last-qualified version.

The adapter interface conceptually separates:

```text
Detect -> Validate -> Prepare -> Launch -> Observe -> Restore
```

Detection is not support. An installed application is selectable only when its
adapter, version/OS range, required Alzette protocol, and company policy all
pass. The UI distinguishes at least:

- **Ready** — installed and qualified with at least one current compatible
  assigned model;
- **Not installed** — supported adapter, application absent;
- **Update required** — detected version outside the qualified range;
- **Protocol unavailable** — Alzette does not currently serve what it needs;
- **Blocked by company** — adapter not allowed by current policy; and
- **Not yet supported** — known product without release evidence.

An adapter prefers a temporary isolated application profile. If it must edit a
normal user configuration, it uses an application-specific lock, preserves
unknown fields and formatting where feasible, creates a private backup,
performs an atomic write, records no remote secret, and restores only the
fields it owns. It must detect concurrent user or application edits and avoid
blind overwrite.

Some desktop applications persist their provider API-key field. Where a
qualified adapter cannot avoid that behavior, it may supply only the random
loopback capability. The capability is useless after disconnect, is never
accepted remotely, and is removed or replaced during restoration when the
application permits it. OAuth and `alz_u_` values are never supplied to the
application.

## 9. Protocol and application matrix

Application support is the intersection of launcher behavior and inference
protocol behavior. The baseline matrix is:

| Application | Model delivery | Client protocol/configuration | Current evidence | Connect release condition |
|---|---|---|---|---|
| Pi | Catalogue with a primary default | Isolated Alzette provider over Chat Completions | Local Linux text stream verified | Convert existing shorthand into the adapter contract; verify multi-model discovery, switching, streaming, tools, cleanup, and packaged launch |
| Jan Desktop | Catalogue when supported by the qualified version | OpenAI-compatible Chat Completions provider | Local Linux manual configuration verified | Automatic reversible configuration, catalogue refresh, full-exit handling, and named-version acceptance |
| Goose Desktop | Catalogue when supported by the qualified version | OpenAI-compatible Chat Completions provider | Local Linux manual configuration verified | Automatic reversible configuration, catalogue refresh, full-exit handling, and named-version acceptance |
| Codex CLI | Primary plus generated compatible catalogue when the reviewed client contract permits it | OpenAI Responses API and isolated profile/model catalog | Bounded gateway Responses text/tool buffered/SSE contract implemented; Connect adapter and named client acceptance remain absent | Implement the isolated profile/catalogue adapter; verify exact Codex version, tools, restore, and packaged launch |
| ChatGPT desktop | Primary plus generated compatible catalogue | Reversible custom provider over the OpenAI Responses path | Disabled-by-default macOS candidate, protected per-launch capability boundary, catalogue generation, stale-safe rollback, and bounded proxy tests implemented; named native client acceptance absent; Windows Store integration absent | Verify one exact ChatGPT version on signed macOS: text, streaming, tools, model switching, errors, disconnect, crash recovery, and profile restoration. Add Windows only after package-aware discovery/activation and its own acceptance |
| Other agents/editors | Product/version-specific | Product-specific | No claim | Add only through a reviewed adapter and named acceptance matrix |

Chat Completions compatibility must never be used as evidence for Anthropic
Messages, OpenAI Responses, embeddings, images, audio, web search, MCP, or
application-specific tool semantics. Each protocol extension keeps one logical
Alzette request, safe streaming/cancellation, tenant routing, current-policy
checks, and metadata-only accounting.

## 10. Desktop interaction requirements

The signed-out view offers one primary action: **Sign in with Alzette**. After
login, Connect resolves the company context, synchronises every currently
assigned model, and makes the installed application launcher the dominant
surface. The catalogue is inspectable but does not become a required selection
step for catalogue-capable applications.

The application list does not become a promotional catalog. Each row shows its
support state and compatible-model count. Ready applications launch on double
click, Enter, or an explicit **Launch** action. Double click is never the only
accessible activation path. A `single` adapter opens a compact model choice;
other adapters launch with the current compatible catalogue and an optional
remembered primary that is revalidated. Unavailable entries explain the exact
reason and safe next step. Connect may link to an approved vendor installation
page, but unmanaged P0 does not silently install third-party software or
execute a network-fetched installer.

The active view shows:

- application, model-delivery mode, and the active/default Alzette alias when
  the adapter reports it;
- company and project/environment;
- connection state and start time;
- whether Connect must remain running;
- a primary **Disconnect** action; and
- a support-safe error or request identifier when attention is required.

It does not show tokens, local keys, raw provider URLs, raw target names,
prompt/output content, token countdowns, or infrastructure internals. Advanced
diagnostics may show Connect version, adapter version, client version, OS,
safe context labels, loopback reachability, gateway reachability, and coarse
protocol capability.

Accessibility requirements include keyboard completion, visible focus,
screen-reader names and state announcements, 200% zoom, reduced motion,
high-contrast support, and no color-only status. The app must remain usable at
the smallest supported desktop window.

## 11. Structured events and desktop control

If the desktop shell supervises the CLI, the CLI exposes a versioned event
stream such as `alzette.connect.events.v1`. Events contain safe identifiers and
labels only. Required event kinds include:

```text
login_required
login_opened
login_complete
contexts_loaded
selection_required
application_checked
profile_prepared
proxy_ready
application_started
application_exited
disconnect_started
profile_restored
grant_revoked
launch_failed
cleanup_incomplete
```

The desktop-to-core control channel supports selection, launch cancellation,
disconnect, logout, and acknowledgement of a recovery action. It is local to
the current operating-system user and authenticated when it crosses a process
boundary. General stdout remains human-readable in normal CLI mode; structured
mode never mixes logs into the event stream.

No event includes an authorization URL containing transient secrets after it
has been handed to the system browser, an identity-provider token, Alzette
credential, loopback capability, prompt/output body, third-party provider
credential, or raw private target.

## 12. Local state and configuration

Connect persists only what is required for a stable employee experience:

- selected control origin from a signed/operator-provided product profile;
- non-secret preferred context and model IDs, revalidated on every use;
- adapter preferences and safe last-used application ID;
- protected rotating login state under the workforce-access contract;
- versioned application-configuration backup metadata; and
- support-safe application version and cleanup records.

It does not persist identity access tokens, `alz_u_` values, grants, loopback
capabilities, prompts, outputs, or private targets. A cached context/model is a
preference, never authorization. If it is no longer returned by the server,
Connect clears it and requires a new selection.

Configuration and backup directories use owner-only permissions. Protected
credentials use the operating-system credential store by default. There is no
silent fallback to a plaintext file. Explicit restricted-file or memory-only
modes remain governed by the workforce-access PRD.

## 13. Packaging, installation, and update

Every distributed CLI and desktop artifact has:

- a version tied to source revision;
- a reproducible or independently verifiable build record;
- platform signing/notarization where applicable;
- published checksums and provenance;
- a software bill of materials and third-party notices;
- an approved download or managed-deployment path;
- an update signature verified before installation; and
- a rollback and minimum-supported-version policy.

The desktop package bundles or installs the exact compatible launcher core; it
must not resolve an arbitrary executable from `PATH` for privileged internal
operations. Third-party applications are detected separately and remain under
their vendors' licences and update mechanisms.

Updates never interrupt an active inference session without explicit notice.
An unsupported Connect version fails with a safe upgrade requirement; it does
not bypass server policy or silently lengthen credential lifetime.

## 14. Security and privacy requirements

In addition to the workforce-access requirements:

- desktop navigation, deep links, and IPC inputs are treated as untrusted;
- executable detection rejects writable-path substitution and records the
  resolved application identity/version before launch;
- child environments remove inherited provider, Alzette, and identity secrets
  before adding the minimal adapter-specific values;
- launch arguments cannot override the managed base URL, credential, profile,
  or model unless the adapter explicitly validates an equivalent safe value;
- configuration writes are private, atomic, locked, bounded in size, and
  recoverable;
- application output is not ingested into Connect telemetry;
- crash reporting excludes environments, arguments containing customer data,
  configuration contents, prompts, outputs, URLs with queries, and secrets;
- diagnostics are locally reviewable before export;
- disconnect is available even when the launched application is hung;
- an abandoned local profile cannot contain a remotely usable credential; and
- adapters that introduce vendor-cloud sync, telemetry, MCP, connectors, web
  search, or other external data paths disclose them before launch and may be
  disabled by company policy.

Connect reduces credential-copying and configuration error. It does not make a
compromised workstation, same-user malicious process, third-party application,
or enabled external tool trustworthy. That boundary appears in customer and
security documentation.

## 15. Functional requirements

| ID | Requirement | Acceptance criterion |
|---|---|---|
| CON-P0-001 | Desktop and CLI share one launcher implementation | The same fixture inputs produce the same context/model decision, adapter plan, lifecycle events, and cleanup result through both surfaces |
| CON-P0-002 | Employees launch without receiving a remote credential | Browser login through first real request and disconnect completes with no OAuth or `alz_u_` value in UI, argv, stdout, events, logs, application config, or exported diagnostics |
| CON-P0-003 | Every delivered model respects current entitlement | Removed membership/group/model access disappears from every adapter catalogue and blocks the next mint/request; cached preferences and third-party client configuration do not preserve access |
| CON-P0-004 | Application support is truthful | Executable presence alone never yields Ready; adapter, version, OS, protocol, model capability, and company-policy checks must all pass |
| CON-P0-004A | Model delivery matches the adapter | Catalogue adapters receive all compatible assigned models, primary-plus-catalogue adapters receive the revalidated preferred model first, and single adapters require exactly one current compatible model before launch |
| CON-P0-005 | Each launch is isolated | Every active client has a distinct random loopback capability, listener, grant, and lifecycle record; a capability cannot invoke another session or the remote gateway |
| CON-P0-006 | Application configuration is reversible | Success, cancellation, child crash, Connect crash recovery, and forced disconnect restore the qualified application profile without overwriting unrelated concurrent edits |
| CON-P0-007 | Disconnect fails closed | The listener stops accepting requests immediately, grant revocation is attempted, temporary material is removed, and incomplete restoration is visibly recoverable |
| CON-P0-008 | Desktop state is understandable | Signed-out, selection, preparing, active, degraded, disconnecting, and cleanup-incomplete states have accessible labels and one safe primary next action |
| CON-P0-009 | CLI supports deterministic launch | Explicit context/application/model invocations do not prompt; ambiguity fails with safe choices and a non-zero exit rather than guessing |
| CON-P0-010 | Protocol claims are exact | Named request, streaming, cancellation, tool, error, usage, and revocation fixtures pass for the protocol advertised by each adapter |
| CON-P0-011 | Local state contains no inference secret | Restart and forensic fixture checks find no access token, `alz_u_`, loopback capability, prompt, output, or provider secret in Connect-owned persistent state |
| CON-P0-012 | Distributed artifacts are supportable | The named OS installer passes signature, clean install, upgrade, rollback, uninstall, checksum, provenance, and third-party-notice checks |
| CON-P1-001 | New adapters cannot weaken existing ones | Adding an adapter requires its own capability manifest and acceptance suite while the complete existing adapter/security matrix remains green |
| CON-P1-002 | Concurrent sessions remain isolated | Two different supported clients can run and disconnect independently without shared local keys, grants, profiles, model selection, or cleanup state |

## 16. Verification plan

### Deterministic tests

- shared-core state-machine and event ordering;
- desktop/CLI parity against the same fake services;
- context/model ambiguity, stale preference, and current-policy removal;
- adapter detection under PATH, symlink, writable-directory, wrong-version,
  architecture, and installation-form cases;
- configuration parse/preserve/atomic-write/lock/restore and concurrent edit;
- random loopback bind, exact local bearer, header policy, origin rejection,
  cancellation, and cleanup;
- child exit, hung child, Connect crash journal recovery, suspend/resume,
  network loss, expired login, failed revoke, and failed profile restore;
- no-secret argv, environment inheritance, stdout, event, file, keychain,
  crash-report, and diagnostic-export checks;
- protocol fixtures for buffered and streaming text, tool calls, usage, safe
  errors, disconnect, and one-logical-request accounting; and
- two-company isolation throughout discovery, launch, inference, usage, and
  revocation.

### Named-client acceptance

Each released adapter records:

- Connect build and source revision;
- operating system, architecture, and patch level;
- application source, version, package hash, and installation form;
- Alzette protocol/capability version;
- model alias and safe route/service-plan identity;
- launch, first request, stream, tools when claimed, disconnect, restore, and
  offboarding results; and
- screenshots/logs containing no customer content or credential.

A prior version's evidence does not silently cover a new major client version,
operating system, protocol, or packaging form.

### Independent release review

Before an external financial-client pilot, an independent reviewer must find
no unresolved critical/high issue in login, local IPC, executable selection,
profile writes, proxy isolation, credential custody, protocol translation,
revocation, update provenance, diagnostics, or third-party data-path copy.

## 17. Delivery increments

### C0 — product-core extraction and naming

- extract the current `alzette-agent` behavior into a shared launcher package;
- introduce `alzette-connect` CLI commands and structured lifecycle events;
- retain a bounded compatibility alias;
- keep current Pi behavior and deterministic tests green.

**Exit:** the new CLI launches Pi through the same security path with no
regression, and `alzette-agent` migration behavior is documented.

### C1 — adapter registry and truthful matrix

- implement adapter manifests, detection, support states, and capability gates;
- move Pi into the adapter contract;
- implement reversible Jan and Goose configuration for exact named versions;
- add `applications`, `models`, `doctor`, and support-safe diagnostics.

**Exit:** the CLI truthfully reports and launches the three qualified clients
without manual provider-value copying.

### C2 — desktop launcher

- build the signed-out, selection, preparing, active, error, and disconnect
  desktop flows on the shared core;
- add protected durable login and logout;
- add tray/session lifecycle and crash cleanup recovery;
- pass accessibility and desktop/CLI parity tests.

**Exit:** an invited employee on the named OS installs Connect, signs in,
launches a qualified GUI or terminal client, completes a request, disconnects,
and restarts without receiving a remote credential or manually editing a
provider.

### C3 — signed remote-pilot artifact

- canonical TLS and production identity profile;
- signed installer, checksums, provenance, update/rollback, and managed install
  instructions;
- named-client/OS acceptance and independent security review;
- support, cleanup recovery, offboarding, and incident runbooks.

**Exit:** Alzette Connect is supportable for the named external pilot; local
development flags and manual provider setup are not part of the employee path.

### C4 — protocol and adapter expansion

- implement one selected protocol extension;
- add the next selected adapter after the P0 ChatGPT acceptance, according to
  pilot demand;
- pass protocol, accounting, client, security, and regression gates;
- expand the signed OS matrix only with named evidence.

**Exit:** the new application is advertised only for the exact versions,
features, models, and operating systems that passed.

## 18. Success measures

Pilot measures are operational evidence, not employee productivity scoring:

- invitation accepted to first successful launched request;
- percentage of launches requiring manual support;
- application detection and configuration success by exact adapter/version;
- clean disconnect and profile-restoration rate;
- login reuse success without remote credential exposure;
- launch failure category and recovery completion;
- offboarding-to-next-request-denial time; and
- Connect crash/update rate by signed build.

No prompt/output body, semantic task judgment, employee ranking, keystroke
monitoring, or inferred productivity score is collected.

## 19. Open decisions

| Decision | Owner | Required by |
|---|---|---|
| Product bundle technology for the first desktop OS | Desktop/platform | C2 start |
| First signed operating system and architecture | Product/first pilot | C2 start |
| Exact Jan and Goose configuration mechanisms and qualified versions | Desktop/quality | C1 exit |
| Whether the desktop calls the core in-process or over private child IPC | Desktop/security | C2 architecture review |
| Protected credential-store implementations and explicit fallback policy per OS | Security/desktop | C2 exit |
| Update channel, signing identity, cadence, forced-minimum policy, and rollback | Operations/security | C3 start |
| Exact ChatGPT desktop version and supported Responses feature subset | Product/platform/first pilot | P0 exit |
| Company-level adapter allow/deny policy and portal owner | Product/security | P1 |
| Vendor telemetry/connectors disclosures required in the launch confirmation | Legal/security/product | First affected adapter |
| Multiple concurrent sessions in the desktop UX | Product/desktop | P1 |

## 20. Definition of done

Alzette Connect P0 is done only when:

- the desktop and CLI use the same reviewed launcher core;
- a named signed desktop artifact and CLI are reproducible and supportable;
- browser login is durable under the protected-store contract and logout is
  complete;
- context and model choices always derive from current Alzette entitlement;
- Pi, Jan, Goose, and ChatGPT meet their exact named adapter/version/OS acceptance
  contracts without manual credential or provider copying;
- configuration, crash recovery, disconnect, revocation, and offboarding tests
  pass;
- no remotely usable employee credential appears in a third-party application
  or Connect-owned unprotected persistent state;
- protocol and feature claims match real end-to-end evidence;
- the external pilot passes canonical TLS, signed-update, provenance,
  backup/recovery, support, and independent security gates; and
- all customer, portal, CLI, desktop, and operational copy describes the same
  supported application matrix and limitations.
