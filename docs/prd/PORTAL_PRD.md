# Alzette Systems — B2B Inference Portal PRD

**Status:** founder-aligned implementation draft

**Date:** 2026-08-12

**Owner:** Alzette growth/product strategy

**Scope:** public product/docs surface, customer portal, inference gateway/control boundary, and the contract for a later operator-only MeluXina Operations module

**Current PoC implementation boundary:** [`POC_BOUNDARY.md`](../product/POC_BOUNDARY.md) is the controlling delivery contract for the OpenRouter-compatible first-client PoC. Where this broader PRD describes later portal or MeluXina capabilities, the narrower PoC boundary controls current scope.

**Account onboarding companion:** [`ACCOUNT_ONBOARDING_PRD.md`](ACCOUNT_ONBOARDING_PRD.md)
defines verified self-service evaluation signup, organisation conversion,
invitation acceptance, teammate onboarding, and local-account recovery. The companion is a product
and technical contract; these flows are not part of the current Slice 2
implementation unless this PRD explicitly says otherwise.

**Workforce agent access companion:**
[`WORKFORCE_AGENT_ACCESS_PRD.md`](WORKFORCE_AGENT_ACCESS_PRD.md) defines
Casdoor-backed invited-employee identity, short-lived human-agent inference
tokens, per-employee attribution, and the local compatibility proxy. It
supersedes permanent personal API keys and new external Alzette-local passwords
as the target employee workflow; none of it is current Slice 2 evidence.

**Current provider research brief:**
[`research/LUXPROVIDE_STARTUP_ACCESS.md`](../../research/LUXPROVIDE_STARTUP_ACCESS.md)
condenses the current startup-access, allocation, and experiment evidence for
LuxProvide/MeluXina. It is a research aid; the evidence boundary and release
requirements in this PRD remain controlling.

**Research rule:** competitor claims below use public primary sources wherever available. Public documentation was reviewed on 2026-08-12. Competitor consoles that require login were not inspected.

## 2026-08-13 implementation checkpoint

The repository now contains the offline implementation through Slice 2 plus a bounded agent gateway seam: a strict text/function-tool `POST /v1/chat/completions` subset with buffered and SSE responses; server-controlled tenant/project/environment/model-alias routing; separate logical-request/provider-attempt ledgers; human portal identity and sessions; operator-provisioned service plans; service accounts with one-time, expiring, overlap-rotatable keys; a multi-view client portal; exact usage, attribution, route evidence, hourly rollups, opt-in probes, and safe CSV/JSON export; a standalone public marketing/docs process; and one-machine PostgreSQL/migration/gateway/control/public/worker Compose deployment. Deterministic compatible-target, two-tenant, retry/accounting, migration, race, Compose, and browser evidence is recorded in [`QA_REPORT.md`](../assurance/QA_REPORT.md).

## 2026-08-14 public-surface checkpoint

The public landing page and implementation documentation are now served by
`alzette public` from `/app/public`, independently of the authenticated portal
assets under `/app/portal`. The public process has its own readiness endpoint
and port, takes only a customer-visible portal-login URL, and has no database or
provider credential. The two public surfaces have different jobs: the landing
page markets the intended private, locally operated service for Luxembourg's
financial industry, while the implementation documentation states the narrower
external/shared PoC boundary, its tested text/function-tool streaming subset, and gated path to MeluXina.

That checkpoint does **not** prove a live OpenRouter response, cross-membership organisation-wide aggregation, rate/concurrency enforcement, TLS ingress, SSO/MFA, backup/restore automation, MeluXina access, dedicated capacity, production operations, customer demand, or product-market fit. The current portal view represents one server-authorised project/environment membership at a time. Compatible probes exist but remain globally and per-target disabled unless explicitly opted in; registry-enabled alone never means ready. The live OpenRouter smoke is pending a newly rotated credential. The public process is database-independent, and the marketing landing is not runtime evidence; implementation documentation and authenticated portal state remain the source of truth for the current route.

### Why the slices exist

The numbered slices are end-to-end risk/evidence increments, not UI tiers or separate products. Slice 0 proves isolation, forwarding, and accounting; Slice 1 proves provisioning, human access, workload credentials, and the first-call workflow; Slice 2 proves trustworthy company-consumption and deployment evidence. Slice 3 adds the operational controls required for a real pilot, and Slice 4 replaces the external target with an evidenced private MeluXina deployment. P0 is the launch requirement set; slices are the order in which that set is built and proven.

Several competitor-matrix and appendix “current repository” cells below preserve the earlier fixture-era audit that informed the build. They are historical research provenance and are superseded as implementation evidence by this checkpoint, `POC_BOUNDARY.md`, `README.md`, and `QA_REPORT.md`; their product recommendations remain useful unless explicitly changed.

## 2026-08-14 self-service account and endpoint decision

The approved product direction is hybrid self-service B2B. A person verifies a
business email, creates or authenticates their Casdoor identity, and receives
one isolated evaluation organisation backed only by an explicitly shared,
hard-capped offer.
They can inspect the real portal and curated catalogue, connect an interactive
agent through short-lived human access or create a separate scoped application
key for a workload, make a bounded first request, and see their own usage
without paying or waiting for a sales conversation.

Dedicated private inference remains the primary commercial offer. The customer
chooses a model and a validated deployment/capacity profile, not a raw machine.
Alzette supplies a versioned quote that identifies dedicated accelerators,
capacity metrics, price, region/execution boundary, and expansion increments.
Business approval, quote acceptance, physical allocation, deployment,
validation, and route readiness remain separate states. Adding capacity buys
additional hardware-backed units behind the same endpoint.

The exact identity lifecycle, security, delivery, and rollout contract is in
[`ACCOUNT_ONBOARDING_PRD.md`](ACCOUNT_ONBOARDING_PRD.md). The current repository
still provisions humans through `alzette user provision`; it has no signup,
email verification, automatic evaluation-organisation provisioning,
invitation, recovery, member-management, or transactional-email workflow.
Those identity controls form a dedicated post-Slice-2 increment and part of the
Slice 3 remote-pilot gate.

## 2026-08-15 workforce agent access decision

Interactive employee access is now a separate identity path from workload
access. An authorised employer invites an exact person into an exact
organisation/project/environment role. The employee authenticates through a
self-hosted Casdoor instance and uses an automatically issued, ten-minute,
membership-bound Alzette `alz_u_` token; they do not receive a permanent
personal API key. Applications, CI, and unattended automation continue to use
service accounts and `alz_k_` keys.

Casdoor proves identity but never chooses an Alzette tenant, model, endpoint, or
route. Alzette owns invitation acceptance, `(issuer, subject)` identity links,
membership, role, aliases, token mint/revoke, routing, usage, and audit. The
gateway accepts short Alzette tokens through a distinct path and never accepts
Casdoor JWTs directly. A separate Go `alzette-agent` process supplies a
loopback-only, process-lifetime compatibility key to agents that require an
API-key field. It forwards with the short Alzette token; Casdoor access and
`alz_u_` tokens remain memory-only. Only the rotating Casdoor refresh
credential may survive process restart, under the protected local-store
contract in the workforce PRD.

The first implementation remains one-machine Docker Compose: one pinned
Casdoor replica, existing Alzette processes, and one PostgreSQL server with
separate least-privilege Alzette/Casdoor databases. Redis is not introduced for
the single-replica P0. Canonical TLS ingress, transactional mail, the Casdoor
acceptance spike, a named client/OS, and independent security review gate any
external pilot. [`WORKFORCE_AGENT_ACCESS_PRD.md`](WORKFORCE_AGENT_ACCESS_PRD.md)
is the controlling technical, UX, security, test, and rollout contract.

## 2026-08-14 endpoint-acquisition implementation checkpoint

The catalogue and endpoint-control portion of the self-service decision is now
implemented for authenticated organisations. Models and Endpoints are distinct
workspaces; a customer can inspect eligible reviewed releases/offers, save and
resume a server-backed configuration, create a hard-capped shared evaluation
endpoint, submit a paid-shared or dedicated request, inspect an immutable
operator quote, confirm their human password before commercial action, follow
commercial/payment/runtime states independently, and request a capacity change
for a ready dedicated endpoint. Deployment and expansion requests preserve
their bounded sizing intent and hashed retry identity as immutable customer
records. Billing is provider-neutral in the domain,
uses Stripe-hosted flows when explicitly configured, and otherwise renders an
honest unavailable state. The client pays Alzette's merchant account and does
not need a Stripe account.

This checkpoint is software evidence, not supply evidence. No model, price,
provider, private machine, or MeluXina execution is available until an operator
publishes the corresponding evidenced catalogue/target records. Live Stripe
test-mode checkout, live compatible-provider inference, scheduled billing
reconciliation, MeluXina fulfilment, public signup, and team invitations remain
gated. [`ENDPOINTS_PRD.md`](ENDPOINTS_PRD.md) is the detailed implementation
contract; [`POC_BOUNDARY.md`](../product/POC_BOUNDARY.md) remains controlling for claims.

## Reading the requirements

The words **MUST**, **SHOULD**, and **MAY** are normative:

- **MUST** means a launch requirement. The MVP is not ready without it or an explicitly approved equivalent.
- **SHOULD** means the default product decision. An owner may defer it only with a recorded reason and a replacement safety or usability measure.
- **MAY** means optional implementation latitude, not a promise to customers.

Priority labels are separate: **P0** is launch MVP, **P1** is required for the first enterprise pilots, and **P2** is scale or expansion work.

## Executive verdict

**Decision:** continue the implemented narrow B2B inference gateway and customer portal as a provider-agnostic pilot. The software can forward requests to approved compatible endpoints; the current evidence is deterministic/offline, not a live OpenRouter result. The intended production destination remains Alzette-operated model serving on MeluXina over its private network, which is not yet available to this project.

**Product-market/product-readiness confidence:**
**Plausible problem thesis; early problem/solution fit unproven; narrow offline implementation confidence is high, production readiness is low.** Repository evidence now proves scoped tenant isolation, compatible-target inference, logical-request accounting, a protected dashboard, and a one-machine deployment path. It still does not prove customer demand, paid use, live OpenRouter behavior, production recovery/security, reconciled billing, MeluXina operations, or reliability. This document remains a product decision and evidence plan, not a traction claim.

**MVP recommendation:** finish the live-provider gate for the implemented single-machine vertical slice: authenticate a client, resolve its credential-scoped route, forward one compatible request, record authoritative logical usage, and show the authenticated project/environment's route evidence and consumption. External pilot routes MUST be labelled as external; they MUST NOT be presented as MeluXina-hosted.

**MeluXina operating decision:** Alzette intends to operate its gateway/control software and deploy customer model servers on LuxProvide’s MeluXina infrastructure. MeluXina is the target compute and network environment, not the customer-facing API contract and not a global provider selected by the client. Access, allocation, commercial terms, and production serving suitability remain unproven; that uncertainty blocks MeluXina hosting claims, not development of the forwarding pilot.

---

# A. Executive product decision

## A1. One-sentence product definition

Alzette is an organisation-scoped managed AI service with two connected
branches: Managed Inference lets a B2B prospect evaluate a capped shared
endpoint and acquire an Alzette-operated dedicated endpoint with optional
company-controlled private interaction custody, while Model Improvement lets
an approved dedicated customer ask Alzette to turn permitted private data into
evaluation evidence and a controlled, versioned model release.

## A2. Exact MVP customer promise

For a verified business-email prospect, the product **MUST** support this
evaluation promise without sales approval or payment:

> Create one isolated evaluation organisation, use its explicitly shared and
> hard-capped model route, issue a separate project/service credential, make a
> successful OpenAI-compatible test request, and see the resulting real usage.

For an approved organisation, the product **MUST** support this dedicated
capacity promise without an engineer manually editing the database:

> Choose an approved model and workload-sized deployment profile; review and
> explicitly accept a versioned quote that states GPU units, evidenced capacity,
> price, execution boundary, and expansion increments; then use one stable route
> whose deployment health, requests, tokens, latency, errors, allocation, and
> capacity utilisation are visible without prompt/output content by default.

The first pilot target MAY forward to OpenRouter, Kimi, OpenAI, or another approved compatible API. The client-facing URL and authentication remain Alzette-owned. A later MeluXina deployment changes the server-side target binding to a private LAN address and port; it does not require the customer to reintegrate. The portal MUST display the actual execution class (`external pilot`, `MeluXina`, or another approved location) and MUST not infer an SLA, retention rule, residency claim, or dedicated compute. Unless the external service supplies verified dedicated capacity, the pilot is labelled shared/external even if its Alzette route configuration is exclusive to one client.

## A3. Target organisation and buying/user roles

### Initial organisation segment (hypothesis)

The most plausible first segment is a small or mid-sized Luxembourg-regulated financial-services organisation—such as a fiduciary, fund administrator, PSF, asset manager, insurer operations team, or specialist back office—with one repetitive confidential text/document workflow, an existing application or IT partner, and no desire to operate GPU infrastructure. This is a **segment hypothesis**, not verified demand.

Large banks may have stronger budgets but longer procurement, architecture, and vendor-risk cycles. Individual consumers and employee-facing chat use cases are out of scope.

An adjacent high-value segment hypothesis is a knowledge-intensive consultancy
or advisory firm whose terminology, methods, review standards, and approved
client-work patterns are material intellectual capital. Its risk is not only
confidentiality: relying exclusively on generic provider behaviour without
preserving its own governed interaction and evaluation record may erode the
distinctiveness clients pay for. This hypothesis requires direct customer
validation and must not be presented as established demand.

### Buying and operating roles

| Role | Job in the buying/use cycle | Portal need |
|---|---|---|
| Economic buyer | CTO/CIO, COO, managing partner, technology-owning director, or operations leader | Contract boundary, commitment, expected cost, service evidence, outcome |
| Risk gatekeeper | Compliance, DPO, information security, operational risk, procurement | Data location and handling, subprocessors, access, audit, retention, incident and SLA evidence |
| Platform owner | Customer IT/platform lead or approved integrator | Projects, environments, endpoints, credentials, health, quotas, support |
| Developer/integrator | Connects the existing application using OpenAI SDK or HTTP | Model selection, endpoint URL, API key lifecycle, test call, errors, docs |
| Finance/contract owner | Reviews commitment, usage, invoices, caps, and exports | Contract source, invoice status, cost attribution, budget controls |
| Auditor/viewer | Reviews evidence without changing runtime | Read-only health, usage, audit, exports, contract documents |
| Alzette operator/support | Internal service owner; not a customer role | Capacity, deployment, incident, support, and audited support access |

## A4. What Alzette is and is not

**Alzette is:**

- an operator-managed inference service and customer control plane for organisations;
- a dedicated-private deployment service by default, with a bounded shared
  evaluation/contract offer when isolated capacity is not required;
- a catalogue and configurator for buying an endpoint capacity unit: approved
  model release, validated runtime/hardware profile, dedicated accelerator
  count, evidenced capacity metrics, and versioned price;
- a contract-aware interface around an approved, curated model catalogue;
- a stable OpenAI-compatible integration path whose server-side target can move from an external pilot endpoint to a private MeluXina model server;
- a place to make customer-controlled access, allocation, versions, usage, health, and support visible;
- for a validated dedicated workflow, an optional operator-assisted path to
  turn explicitly approved interactions into a private evaluation set and a
  versioned model-improvement release under customer control;
- a local operational relationship hypothesis that must be substantiated by contracts, operators, and evidence.

**Alzette is not:**

- a consumer chat application, employee productivity suite, or generic AI workspace;
- a model marketplace, arbitrary model-upload platform, automatic training on
  customer traffic, or general-purpose training laboratory at MVP;
- a raw GPU-host picker or customer-controlled infrastructure console;
- a promise that all customers share one model, or that a dedicated route may silently fall back to shared capacity;
- legal, tax, accounting, compliance, or regulatory advice;
- proof that a model, GPU, region, SLA, certification, or capacity allocation exists merely because a card or fixture displays it;
- a replacement for the customer’s own application governance or human accountability.

## A5. Evidence status and assumptions

| Statement | Status | Consequence for the portal |
|---|---|---|
| Alzette targets private managed inference for Luxembourg/regulatory workloads | **Repository claim** in `PRODUCT.md`, `index.html`, `docs.html` | Use as positioning input, not proof of demand or hosting |
| Customers need an approved, bounded path from an existing application to inference | **Reasonable product hypothesis** | Validate with recent workflows, blockers, buyer, budget, and technical review |
| OpenAI-compatible chat/streaming/structured-output/embeddings API exists | **Partially implemented** | A strict text/function-tool Chat Completions subset is tested for buffered responses and SSE with Pi 0.84.2; `/v1/models`, multimodal input, structured output, embeddings, and compatibility beyond that subset remain unsupported and MUST NOT be advertised |
| Tenant-isolated credentials, route bindings, usage, limits, versioning, and default non-retention exist | **Partially implemented and tested** | Scoped hashed keys, operator routes, metadata-only request usage, and model aliases exist; rate/concurrency enforcement, human auth, richer version lifecycle, and production retention operations remain gates |
| The pilot may forward requests to approved external endpoints | **Accepted founder decision** | Implement a provider-compatible adapter and label the route `external pilot`; do not claim MeluXina execution |
| Dedicated customer model deployments are the primary offer; shared inference is optional | **Accepted founder decision** | Every route MUST resolve through a tenant-authorised target binding with an explicit tenancy mode |
| Alzette software and customer model servers are intended to run on MeluXina | **Accepted founder direction; not yet implemented** | Keep network targets configurable as private addresses/ports and qualify the MeluXina operating path separately |
| Luxembourg hosting, retention, service levels, and model availability are contractual | **Repository claim/design intent** | Portal MUST read and label the executed agreement; it MUST NOT infer terms from marketing copy |
| Capacity exists and is operational; first clients are being signed | **Founder/operator claim** | Requires operator evidence: endpoint, ownership, capacity, monitoring, incident process, and access procedure |
| The earlier fixture dashboard proves live account usage, H100 capacity, success rate, sites, or spend | **False for repository evidence** | Those historical values are fixtures and MUST never be reused as traction or customer metrics; the current dashboard omits them |
| The current worktree contains a production inference service | **False; it contains a narrow implementation PoC** | Compatible-target inference and persistence exist, but live-provider, TLS, rate enforcement, recovery, worker, dependency review, and production operations gates remain |
| There is product-market fit | **Not established** | Do not use “PMF,” customer count, usage, or willingness-to-pay claims without records |

### A5.1 MeluXina infrastructure evidence status

The following table is the controlling evidence boundary for the infrastructure decision. “Official source” means a current public LuxProvide, MeluXina, or EuroHPC publication; it is not a substitute for an Alzette-specific agreement or a completed test.

| Statement | Classification | What is currently supported | What remains unproven / portal consequence |
|---|---|---|---|
| LuxProvide/MeluXina is the intended Alzette operating environment | **Founder strategic decision** | The founder intends Alzette to run its service and customer model deployments on MeluXina. LuxProvide identifies MeluXina as a Luxembourg-hosted EuroHPC supercomputer and documents AI/HPC use. [MeluXina](https://www.luxprovide.lu/meluxina/) and [EuroHPC system profile](https://www.eurohpc-ju.europa.eu/supercomputers/our-supercomputers_en) (accessed 2026-08-12). | No Alzette access, reservation, allocation, contract, private serving address, or deployment has been produced. This blocks MeluXina hosting claims and production binding, but not the external-forwarding pilot. |
| MeluXina can run an open-weight inference experiment | **Official capability evidence; not Alzette evidence** | Current documentation includes container support and LLM-serving examples using NVIDIA TensorRT-LLM/Triton and vLLM through Slurm jobs. [Triton example](https://docs.lxp.lu/howto/llama3-triton/) and [vLLM example](https://docs.lxp.lu/howto/llama3-vllm/) (accessed 2026-08-12). | The examples use HPC-style jobs and tunnels; they do not prove a managed, stable, customer-facing API, always-on service, or Alzette-owned gateway. The PoC MUST reproduce one supported path. |
| Commercial/startup access is possible in principle | **Official access evidence; Alzette eligibility unknown** | LuxProvide documents commercial access for industry, public administration, public research, and academia, plus special tracks for Luxembourg startups. It also documents national-share and EuroHPC access paths. The current EuroHPC AI Factories Industrial Innovation terms include commercial companies/SMEs/startups in eligible Member/associated countries, allow SME/startup commercial exploitation of results, and impose proposal, civilian-purpose, AI Act, reporting, and publication conditions. [Access procedures](https://docs.lxp.lu/access/gaining_access/) and [EuroHPC Industrial Innovation Terms](https://www.eurohpc-ju.europa.eu/document/download/bd7aa666-bdf3-4436-b5ec-0b34a781e817_en?filename=Terms+of+Reference-AIF+Access+Calls.pdf&prefLang=fi) (accessed 2026-08-12). | Alzette’s exact eligibility, route, approval time, allocation, and required agreement are unknown. No application or contact was made for this assessment. |
| MeluXina is cost-effective for Alzette’s first MeluXina-hosted pilot | **PoC hypothesis** | LuxProvide’s current startup/SME page advertises INITIATE free access up to six months and Cashback80 up to 80% cashback, subject to eligibility, terms, and hardware availability. [Startup and SME programmes](https://www.luxprovide.lu/programs/) (accessed 2026-08-12). | No current rate card, credit amount, allocation, cash timing, repeatability, or Alzette eligibility was verified. Older official programme PDFs contain numeric terms that differ from the current live page and MUST NOT be treated as current entitlement. Cost passes only after actual metering/terms and a non-subsidised scenario are recorded. |
| Alzette can sustain its customer-facing service on MeluXina | **Unknown / operating-boundary risk** | The system overview describes a Cloud module with temporary or persistent VMs; OpenStack docs show services exposed through a VPN. LuxProvide also says MeluXina Cloud gateways and Kubernetes are being engineered. [System overview](https://docs.lxp.lu/system/overview/), [OpenStack](https://docs.lxp.lu/cloud/openstack/openstack/), and [Web-services introduction](https://docs.lxp.lu/web_services/welcome/) (accessed 2026-08-12). | Public evidence does not prove that Alzette can run an always-on gateway plus stable private model-server targets, load balancing, service discovery, or autoscaling. The MeluXina Operations PoC MUST prove the selected topology. |
| GPU capacity is available when Alzette needs it | **Unknown** | Public documentation lists a 200-node GPU partition with four NVIDIA A100-40 GPUs per node; the marketing page reports 800 GPU-AI accelerators. [System overview](https://docs.lxp.lu/system/overview/) and [MeluXina](https://www.luxprovide.lu/meluxina/) (accessed 2026-08-12). | Inventory is not an allocation. Queue time, current availability, model fit, entitlement to cloud GPUs, and sustained capacity for a pilot remain unknown and MUST be measured. |
| Data is Luxembourg/EU-hosted and suitable for regulated customer data | **Official marketing/legal-language claim; not Alzette approval** | LuxProvide’s MeluXina page states Luxembourg operation, ISO 27001, Tier IV, isolation, private connectivity, and anonymisation; the public Terms of Use includes EU processing and a GDPR processor annex. [MeluXina](https://www.luxprovide.lu/meluxina/) and [Terms of Use (version 2023-06-12)](https://docs.lxp.lu/assets/LUXPROVIDE%20MeluXina%20terms%20of%20use%20-%20v10%20-%20final.pdf) (accessed 2026-08-12). | Exact scope, current certificate, subprocessors, logs, backups, retention, ingress/gateway path, customer audit rights, and DPA/service commitment must be reviewed. Geography alone MUST NOT be presented as regulatory suitability. |
| MeluXina provides a production SLA/support commitment | **Unknown** | The public status page exposes service categories and current status; docs identify a Service Desk and commercial-request route. [Status](https://status.lxp.lu/) and [Documentation welcome](https://docs.lxp.lu/) (accessed 2026-08-12). | The public Terms of Use is versioned 2023-06-12, says allocations’ start time cannot be guaranteed, and disclaims uninterrupted/error-free operation unless a service commitment/contract provides otherwise. Commercial response targets, remedies, and serving SLA are unknown. The portal MUST show only signed terms. |
| A next-generation MeluXina-AI service is available now | **False for current availability** | LuxProvide’s page says MeluXina-AI launches at the end of 2026; EuroHPC’s 22 July 2026 announcement says installation is expected to start in fall 2026. [MeluXina-AI](https://www.luxprovide.lu/meluxina-ai/) and [EuroHPC announcement](https://www.eurohpc-ju.europa.eu/eurohpc-ju-signs-contract-meluxina-ai-new-ai-optimised-supercomputer-luxembourg-ai-factory-2026-07-22_en) (accessed 2026-08-12). | Future AIaaS, multi-tenant, regulated-sector, and deployment language is roadmap evidence, not current MeluXina availability. It MUST NOT be used as a launch dependency or customer claim. |
| Alzette has produced the MeluXina PoC | **Not produced** | This assignment performed public research and PRD work only; no account, allocation, application, contact, credential entry, reservation, or deployment occurred. | There is no migration evidence in the repository. Slice 4 and Gate 4 define the required work; they are not evidence that it has happened. |

### A5.2 Current MeluXina offering shape relevant to Alzette

The official material supports the following **infrastructure** picture as of the research date:

- **HPC/AI compute:** the documentation describes a Slurm-managed system with CPU, GPU, FPGA, and large-memory partitions; its system overview lists 573 CPU nodes and 200 accelerator nodes with four NVIDIA A100-40 GPUs per GPU node. Jobs are scheduled on full compute nodes, and login nodes are explicitly not for intensive or long-running processes. [System overview](https://docs.lxp.lu/system/overview/), [Quick start](https://docs.lxp.lu/first-steps/quick_start/), and [Platform usage policy](https://docs.lxp.lu/access/PoliciesSummary/) (accessed 2026-08-12).
- **Storage and data movement:** the documentation lists Scratch, Project, Backup, and Archive tiers with project quotas and project-member access controls. Scratch is temporary; Project is the core project tier; Backup/Archive are separate retention mechanisms. This is storage infrastructure, not a customer prompt-retention policy. [System overview](https://docs.lxp.lu/system/overview/) and [Managing data](https://docs.lxp.lu/first-steps/managing_data/) (accessed 2026-08-12).
- **Software and serving:** Apptainer/NGC container workflows and official vLLM/Triton inference examples are available. The examples launch serving processes in scheduled GPU jobs and query them through forwarding/tunnel techniques. This is sufficient to justify a serving PoC, not a managed endpoint claim. [Containers](https://docs.lxp.lu/containerization/introduction/), [vLLM](https://docs.lxp.lu/howto/llama3-vllm/), and [Triton](https://docs.lxp.lu/howto/llama3-triton/) (accessed 2026-08-12).
- **Cloud and networking:** the system overview describes temporary or persistent VM services, while the OpenStack instructions use a VPN and state that users remain responsible for service authentication/security. S3, Slurm REST, Keycloak, JupyterLab, and Open OnDemand are documented, but the web-services landing page describes them as interfaces being implemented/test access. This leaves stable ingress, public reachability, load balancing, and production support open. [System overview](https://docs.lxp.lu/system/overview/), [OpenStack](https://docs.lxp.lu/cloud/openstack/openstack/), and [Web-services introduction](https://docs.lxp.lu/web_services/welcome/) (accessed 2026-08-12).
- **Access and commercial routes:** access is project/allocation based through Luxembourg national share or EuroHPC; commercial access and startup tracks are documented. LuxProvide’s current startup page advertises INITIATE and Cashback80, but the amount, price, allocation, and contract terms need confirmation. [Access procedures](https://docs.lxp.lu/access/gaining_access/), [Allocations](https://docs.lxp.lu/access/allocation_monitoring/), and [programmes](https://www.luxprovide.lu/programs/) (accessed 2026-08-12).
- **Onboarding, lead time, and operator responsibility:** the public national-share documentation does not publish a commercial approval lead time or minimum allocation. It says a project must first be approved, after which LuxProvide creates user accounts, users accept the Terms of Use and upload SSH keys through the Service Desk, and a Project ID tracks allocations. The public EuroHPC Large Scale AI Factory call advertises approval within 10 working days after a cut-off, but targets industry applications above 50,000 GPU hours and grants 3/6/12-month allocations; it is not evidence that this is the right or available route for Alzette’s small PoC. The public Terms of Use (version 2023-06-12) places responsibility for project software, data, access controls, backups, and timely data export on the Project Manager, while LuxProvide manages accounts and agreed allocations; it says allocation start times cannot be guaranteed. [Access procedures](https://docs.lxp.lu/access/gaining_access/), [EuroHPC Large Scale Access](https://www.eurohpc-ju.europa.eu/large-scale-access-ai-factories_en), and [Terms of Use](https://docs.lxp.lu/assets/LUXPROVIDE%20MeluXina%20terms%20of%20use%20-%20v10%20-%20final.pdf) (accessed 2026-08-12).

The resulting conclusion is narrow: MeluXina is a credible **target operating substrate for an authorised inference PoC**. The current public evidence does not establish that Alzette can yet run an always-on gateway and dedicated model servers there. The product can still validate its gateway, tenant routing, metering, and portal by forwarding to external compatible endpoints. MeluXina qualification is a separate infrastructure migration gate.

## A6. Explicit decisions

1. **B2B organisation first, self-service entry.** The portal model is organisation/tenant → project → environment → route/endpoint. A verified person may create one isolated evaluation organisation; it is not a personal dashboard or proof of company authority.
2. **Curated catalogue first.** Customers choose from models Alzette has approved and can support. Arbitrary model upload is deferred.
3. **One safe activation path.** The MVP optimises for a verified first request, not for a large console.
4. **Dedicated is the default paid offer.** A customer normally receives its own model deployment and GPU capacity. Shared inference is an explicit evaluation or contracted alternative, never an invisible implementation detail. MeluXina-hosted customer-specific infrastructure is labelled `dedicated private`; `on-premises` is reserved for equipment at the customer's own site.
5. **The sellable unit is endpoint capacity.** A capacity unit freezes one model release, validated runtime/hardware profile, accelerator count, evidenced capacity metrics, and versioned unit price. The customer chooses model/mode/units; Alzette assigns physical machines. Additional accepted units expand the stable endpoint.
6. **Configuration is self-service; supply remains evidenced.** A customer may prepare a deployment or scale-up request. Only an approved quote, assigned infrastructure, validated runtime, dedicated target, and active route make it real.
7. **Control plane/data plane separation.** The portal manages intent, credentials, configuration, and evidence. It does not pretend that a UI state means inference is serving.
8. **Customer-controlled private interaction custody.** A subscribed dedicated
   customer may retain prompts and outputs as a tenant-isolated company asset.
   The organisation chooses a recorded retention mode and controls authorised
   access, purpose, export, deletion, and eligibility for Model Improvement.
   Alzette does not reuse content independently or across customers. Until the
   feature is implemented and configured, inference remains metadata-only.
9. **Tenant-scoped routing is the core invariant.** The gateway resolves an authenticated tenant/project/model alias to an authorised inference target. Customers cannot submit raw upstream URLs.
10. **No cross-offer fallback.** Dedicated traffic stays on targets owned by that customer. Shared traffic stays within an explicitly authorised shared pool. Any fallback that changes tenancy mode requires an explicit contract and policy.
11. **Usage before billing.** Customer P0 reporting focuses on requests, tokens, errors, latency, throughput, allocation, and utilisation. Detailed invoices and infrastructure COGS are not prerequisites for a dedicated-capacity pilot.
12. **Logical requests and runtime attempts are separate.** A customer call counts once even if Alzette retries or fails over internally; each target attempt remains observable to operators.
13. **MeluXina is the target operating environment.** Alzette plans to run the gateway/control software and model servers there. A Project ID, Slurm account, VM, or allocation is infrastructure, not a customer tenant.
14. **External forwarding unlocks the software PoC.** Before MeluXina access exists, targets may point to approved external APIs. The route MUST be labelled honestly and remain replaceable by a private LAN target.
15. **MeluXina operations are a separate module.** Customers configure model/capacity intent; model deployment, allocation, image/runtime lifecycle, private endpoints, restart, and capacity operations remain in the Alzette operator surface until safely automated.
16. **Contract is a source of truth.** Region, retention, model availability, commitments, rates, caps, and service levels come from contract/operations systems and show provenance.
17. **Cost advantage is not assumed.** Any INITIATE, Cashback80, or EuroHPC access benefit is a scenario to verify, not recurring unit economics or customer pricing.
18. **Model Improvement is an Alzette-operated product branch.** The customer
    governs objective, permitted data, evaluation criteria, and release
    approval; Alzette operates dataset preparation, evaluation/adaptation,
    artefact custody, deployment, and rollback. It is separate from ordinary
    inference and is not exposed as a general customer training console.
19. **MeluXina-AI is a strategic front-row objective, not a current claim.**
    Alzette should seek early technical, commercial, and ecosystem
    qualification as a specialised inference and Model Improvement operator.
    No surface may imply access, partnership, preferred status, allocation, or
    production capability before direct evidence exists.

## A7. Unresolved founder/operator decisions

The following are launch blockers for any corresponding promise and must be answered with evidence, not design preference:

- Which external OpenAI-compatible endpoint and model will power the first forwarding pilot, and which API features are in its tested compatibility subset?
- Which shared model, lifetime allowance, rate/concurrency limits, abuse policy,
  and cost owner make the verified evaluation offer safe to enable?
- Which initial segment and named workflow will receive the first pilot?
- What hosting boundary can be contractually committed today: Luxembourg, another EU location, customer site, or per-customer choice?
- What data is stored in the data plane, gateway, queues, metrics, logs, backups, support tooling, and portal? For how long, where, and who can access it?
- What is the actual tenant-isolation design and test evidence? How are customer support accesses approved and logged?
- Which first model/version is assigned to each pilot customer, and is each route dedicated or explicitly shared?
- Which per-capacity-unit performance metrics are measured well enough to quote,
  how do they scale with added GPUs, and which workloads invalidate linear
  scaling assumptions?
- Which quote acceptance or payment/contract event authorises physical capacity
  allocation, and who can reverse or expire that commitment?
- What are the pricing units, overage rules, budget/cap semantics, invoice process, currencies, taxes, and commitment renewal/expiry rules?
- What availability target, incident severity model, support response, maintenance notice, and escalation path can be signed?
- Which identity features are required by the first pilot: email login, MFA, SSO/SAML, OIDC, SCIM, customer-managed groups?
- Is the first target binding created by an Alzette operator or by an operator-approved workflow? Which customer actions may alter it?
- Which gateway event is authoritative for company consumption, and how are retries, streaming completion, and incomplete upstream usage represented?
- What legal entity, DPA, subprocessor list, privacy notice, security pack, certifications, and model licence evidence may be shared?
- Which MeluXina access path is valid for Alzette: LuxProvide commercial/national share, INITIATE, Cashback80, EuroHPC AI Factory industrial access, or another route? What documents, project owner, and obligations apply?
- What exact MeluXina resource will run the Alzette services and each customer model server: GPU partition/job, persistent OpenStack VM, or another service? What stable private address/port, service discovery, restart, and on-call mechanism is supported?
- What current rate card, allocation/credit amount, term, support cost, storage/egress cost, taxes, and invoice source apply? Which programme terms supersede the older public PDFs?
- Can the planned path sustain a customer-facing endpoint outside a VPN or SSH tunnel, and what availability, queue, cold-start, scaling, maintenance, and recovery commitments can be contracted?
- Which data paths, logs, backups, subprocessors, retention/deletion controls, encryption, audit rights, and incident notifications apply to prompts, outputs, model artefacts, and telemetry? Does the complete Alzette gateway path remain within the contracted region?

---

# B. Competitive research and product decisions

## B1. Research boundary

The comparison uses official documentation, official pricing/trust/status pages, API references, and official changelogs accessed 2026-08-12. It does not claim to have inspected authenticated dashboards. Where a public page says a console has a feature, that proves the documented product concept, not the exact current UI behavior for every account tier.

## B2. Capability/workflow matrix

The “Current Alzette repository evidence” column is the fixture-era discovery snapshot used to choose the first slice. It is intentionally retained to show why each decision was made, but it is not the current implementation inventory. Use the implementation checkpoint above and the controlling PoC documents for current evidence.

| Capability or workflow | Baseten — verified public pattern | Fireworks — verified public pattern | Fixture-era Alzette evidence (historical) | Recommended Alzette treatment |
|---|---|---|---|---|
| Organisation/workspace structure | Organisation roles are Admin/Member; Enterprise Teams add isolated resources, members, secrets, keys, and Team Admin/Member roles. Billing remains organisation-level and public docs say there is no team-level billing breakdown or budget control. [Access control](https://docs.baseten.co/organization/access) and [Teams](https://docs.baseten.co/organization/teams) (accessed 2026-08-12). | An account has users with Admin, User, Contributor, and Inference User roles; service accounts are account-scoped. Public docs do not establish a project/workspace hierarchy comparable to Alzette’s proposed project/environment model. [Managing users](https://docs.fireworks.ai/accounts/users) (accessed 2026-08-12). | `PRODUCT.md` claims organisation/project controls. The dashboard shows a “Workspace” and account; there is no auth, membership, or tenant persistence in the implementation. `README.md:21-26` says those still need to be added. | **ADAPT NOW:** org/tenant, project, environment context, and a small role set. **REJECT NOW:** copying a large team hierarchy. Make billing and contract scope explicit at org level and runtime scope explicit at project/environment level. |
| Onboarding and first request | Model APIs require an API key and support OpenAI/Anthropic-compatible calls; `/v1/models` exposes current metadata. Custom models use Truss/management API. [Model APIs](https://docs.baseten.co/inference/model-apis/overview) and [Inference API](https://docs.baseten.co/reference/inference-api/overview) (accessed 2026-08-12). | Onboarding directs a user to create an API key, use the model library/playground, and call serverless; on-demand quickstart creates a deployment and then calls it through the same OpenAI-compatible API. [Onboarding](https://docs.fireworks.ai/getting-started/onboarding), [Inference introduction](https://docs.fireworks.ai/guides/inference-introduction), and [On-demand quickstart](https://docs.fireworks.ai/getting-started/ondemand-quickstart) (accessed 2026-08-12). | `docs.html` provides proposed base URL and snippets. No `/v1/models`, chat, embeddings, or authentication handler is implemented; only `/api/healthz` and `/api/dashboard` exist. | **ADAPT NOW:** guided first-call checklist, live model discovery, copy-safe credential issuance, curl/OpenAI examples, request ID and visible failure. Make “first successful request” the activation event. |
| API keys and lifecycle | Personal keys are user-bound; team keys are available with Teams. Docs show creation date/last used, no automatic expiry, and rotation by creating a new key then revoking the old one; keys can be environment/model scoped. [API keys](https://docs.baseten.co/organization/api-keys) (accessed 2026-08-12). | Users can manage their own keys; Admins can create/delete service-account keys. Keys may include an expiry in the API reference. Service accounts cannot log into the web UI; usage is tracked at account level. [Service accounts](https://docs.fireworks.ai/accounts/service-accounts), [Create API key](https://docs.fireworks.ai/api-reference/create-api-key), and [Users](https://docs.fireworks.ai/accounts/users) (accessed 2026-08-12). | Proposed docs say bearer keys per organisation/project. No key store or lifecycle exists. | **ADAPT NOW:** service accounts for workloads, least-privilege project/environment/model scopes, one-time reveal, expiry/last-used, overlap rotation, immediate revoke, and audit. **ADAPT FOR PEOPLE:** short-lived membership-bound human-agent access under [`WORKFORCE_AGENT_ACCESS_PRD.md`](WORKFORCE_AGENT_ACCESS_PRD.md), never permanent personal keys. Never show plaintext again. |
| Model catalogue/model garden | Model APIs expose a fixed supported list, feature support, prices, context, and `/v1/models`; custom models use Truss. [Model APIs](https://docs.baseten.co/inference/model-apis/overview) (accessed 2026-08-12). | Model Library lists base models and user models; model metadata includes `supportsServerless`, deprecation date, tuning flags, and other capability data. Serverless availability is explicitly tagged. [Models](https://docs.fireworks.ai/models/overview), [List models](https://docs.fireworks.ai/api-reference/list-models), and [Serverless overview](https://docs.fireworks.ai/serverless/overview) (accessed 2026-08-12). | `catalog.json` is a “sneak peek” with nine illustrative entries, tier labels, licences, and no live availability or price source. | **ADAPT NOW:** narrow, searchable approved catalogue with exact version, modality, capabilities, licence review, region/capacity eligibility, support status, price basis, deprecation/change notice, and “request a model” path. **REJECT NOW:** broad marketplace parity. |
| Shared/serverless inference | Baseten Model APIs run on shared infrastructure, require no deployment, and bill per million tokens; dedicated deployment is the alternative. [Model APIs](https://docs.baseten.co/inference/model-apis/overview) (accessed 2026-08-12). | Fireworks Serverless is multi-tenant, per-token, no GPU sizing, with standard/priority/fast tiers and rate limits; not every model is serverless. [Serverless overview](https://docs.fireworks.ai/serverless/overview) (accessed 2026-08-12). | Product copy proposes shared/dedicated/reserved tiers; no data-plane implementation. | **ADAPT NOW:** capacity mode as an explicit contract/eligibility choice; surface rate/cold-start/availability implications. **REJECT NOW:** silent fallback from dedicated to shared or vice versa. |
| Dedicated/on-demand inference | Baseten deployments are containerised model versions with configurable hardware/autoscaling; Model APIs can be a path before dedicated deployment. [Deployments](https://docs.baseten.co/deployment/deployments) and [Pricing](https://www.baseten.co/pricing/) (accessed 2026-08-12). | On-demand uses dedicated GPUs, broader model choice, GPU-second billing, configurable replicas/autoscaling, and region placement. [On-demand deployments](https://docs.fireworks.ai/guides/ondemand-deployments) (accessed 2026-08-12). | Fixture labels a dedicated machine/H100 and “operational,” but `main.go` returns those as hard-coded fixture values. | **ADAPT NOW:** preset dedicated plans and an operator-reviewed request flow. **ADAPT LATER:** customer-tunable hardware/autoscaler knobs only after real capacity APIs and cost guardrails exist. |
| Reserved/committed capacity | Baseten public sources reviewed describe dedicated compute, priority access, volume discounts, custom SLAs, and enterprise options, but a public reservation lifecycle was not verified. [Pricing](https://www.baseten.co/pricing/) (accessed 2026-08-12). | Enterprise reserved capacity is normally a fixed commitment, guarantees capacity/higher quotas/lower GPU-hour prices, is invoiced separately, and remains billable until term end. [Reserved capacity](https://docs.fireworks.ai/deployments/reservations) (accessed 2026-08-12). | `PRODUCT.md` and dashboard copy discuss dedicated/reserved capacity; no contract or invoice evidence. | **ADAPT NOW:** read-only commitment ledger, renewal/expiry warnings, reserved-vs-overage explanation, and support request. **ADAPT LATER:** self-serve purchase/renewal; commercial changes require human approval. |
| Endpoint/deployment lifecycle | Baseten separates mutable development deployments from stable environments; promotion preserves a stable endpoint. Rolling deployments can pause, resume, cancel, or force-cancel/roll-forward; old deployments remain available for rollback. [Deployment concepts](https://docs.baseten.co/deployment/concepts), [Environments](https://docs.baseten.co/deployment/environments), and [Rolling deployments](https://docs.baseten.co/deployment/rolling-deployments) (accessed 2026-08-12). | Fireworks deployment API exposes states, active/target model versions, replica status, and default deployments. Routers can split traffic by replica count for migration/A/B and can be rolled back by changing replicas; public docs do not establish a Baseten-style environment promotion abstraction. [Get deployment](https://docs.fireworks.ai/api-reference/get-deployment), [Default deployments](https://docs.fireworks.ai/deployments/managing-default-deployments), and [Routers](https://docs.fireworks.ai/deployments/routers) (accessed 2026-08-12). | No endpoint lifecycle or runtime route exists. Dashboard buttons are demo actions/toasts. | **ADAPT NOW:** immutable release/version record, stable route alias, dev test → explicit production promotion, last-known-good rollback, and clear states. **ADAPT LATER:** canary/weighted routing. **REJECT NOW:** exposing raw deployment mechanics as the primary customer model. |
| Scaling, regions, availability | Baseten documents autoscaling, scale-to-zero, multi-cloud capacity management, environments, and regional environments with configured geographic restrictions. [Overview](https://docs.baseten.co/overview), [Regional environments](https://docs.baseten.co/deployment/regional-environments), and [How Baseten works](https://docs.baseten.co/concepts/howbasetenworks) (accessed 2026-08-12). | Fireworks supports multi-region and explicit GLOBAL/US/EUROPE/APAC groupings; region is a creation-time choice in the on-demand guide, and scale-to-zero may return `503 DEPLOYMENT_SCALING_UP`. [Regions](https://docs.fireworks.ai/deployments/regions), [Deployments](https://docs.fireworks.ai/guides/ondemand-deployments), and [Autoscaling](https://docs.fireworks.ai/deployments/autoscaling) (accessed 2026-08-12). | Copy claims Luxembourg and dedicated capacity; no live placement/health data. | **ADAPT NOW:** show contractual region, actual placement/eligibility, capacity state, and cold-start/scale behavior. **ADAPT LATER:** multi-region failover and customer-selected regions. **REJECT NOW:** any “Luxembourg” label not backed by a live/contract source. |
| Health, logs, metrics, traces, incidents | Baseten documents status states, request-ID log filtering, metrics by environment/deployment and HTTP class, real-time logs/traces, and Prometheus export. [Health](https://docs.baseten.co/observability/health), [Logs](https://docs.baseten.co/observability/logs), [Metrics](https://docs.baseten.co/observability/metrics), and [Overview](https://docs.baseten.co/overview) (accessed 2026-08-12). | Fireworks documents Prometheus metrics for on-demand latency/throughput/errors, exportable billing metrics, status page, and Enterprise audit/access logs. Public sources reviewed do not establish customer-facing inference request logs/traces equivalent to Baseten’s request-ID workflow. [Exporting metrics](https://docs.fireworks.ai/deployments/exporting-metrics), [Billing metrics](https://docs.fireworks.ai/accounts/exporting-billing-metrics), [Audit logs](https://docs.fireworks.ai/guides/security_compliance/audit_logs), and [Status](https://status.fireworks.ai/) (accessed 2026-08-12). | Fixture has synthetic node/throughput/success values and “all operational.” `GET /api/dashboard` is the only data response. | **ADAPT NOW:** actionable health, freshness timestamp, request ID, HTTP/error class, latency/throughput where live, and status/incident links. **ADAPT LATER:** traces and external metrics export. **REJECT NOW:** synthetic deployment maps and telemetry. |
| Usage, attribution, cost, invoices, budgets, quotas | Baseten Model API budgets can be enforced or notification-only, with email at 75/90/100%; docs explicitly say budgets do not cover dedicated deployments. Pricing is per token for Model APIs and compute for dedicated. [Rate limits and budgets](https://docs.baseten.co/inference/model-apis/rate-limits-and-budgets) and [Pricing](https://www.baseten.co/pricing/) (accessed 2026-08-12). Public invoice detail was not verified. | Fireworks separates serverless token, on-demand GPU-second, fine-tuning, and reserved billing. Account quotas expose request limits, monthly spend budget, and GPU quotas; reaching the budget pauses API requests. Billing can be exported to CSV and invoices/credits are documented. [Account quotas](https://docs.fireworks.ai/guides/quotas_usage/account-quotas), [Billing management](https://docs.fireworks.ai/faq/billing-pricing-usage/billing/billing-management), and [Billing metrics](https://docs.fireworks.ai/accounts/exporting-billing-metrics) (accessed 2026-08-12). | Fixture shows €2,480.40, 82,140 requests, cap, commitment and history; all are synthetic. Product docs claim metering/budgets/caps but no source. | **ADAPT NOW:** company-consumption ledger with logical requests, token/latency/error attribution, dedicated utilisation or shared allowance, source/finality, and CSV/JSON export. **ADAPT WHEN CONTRACTED:** monetary budgets and invoices. **REJECT NOW:** displaying invented numbers, counting retries as customer requests, or promising enforcement the gateway cannot provide. |
| Enterprise access/trust | Baseten documents SSO/SCIM for Enterprise, JIT provisioning, directory groups, RBAC, audit logging, regional restrictions, SOC 2 Type II/HIPAA and trust documents. [SSO and SCIM](https://docs.baseten.co/organization/sso-and-scim), [Access](https://docs.baseten.co/organization/access), [Secure model inference](https://docs.baseten.co/observability/security), and [Trust Center](https://trust.baseten.co/) (accessed 2026-08-12). | Fireworks documents enterprise OIDC/SAML/JIT/enforced SSO, audit logs, zero-data-retention statements for open models, workload isolation, ISO 27001/27701/42001, SOC 2 Type II, HIPAA, and data-residency/private-networking claims. Feature scope and contract terms still require customer review. [Custom SSO](https://docs.fireworks.ai/accounts/sso), [Data security](https://docs.fireworks.ai/guides/security_compliance/data_security), and [Enterprise](https://fireworks.ai/enterprise) (accessed 2026-08-12). | Repo has security copy and TODOs for privacy URL, SLA figure, and certification status; no DPA, audit, data-flow, subprocessor, or certification evidence. | **ADAPT NOW:** evidence centre, least privilege, audit trail, access review, retention/region display, and incident/support path. **ADAPT LATER:** SAML/SCIM/private networking once a pilot requires it. **REJECT:** certification or compliance badges without current evidence. |
| API/SDK/developer experience | OpenAI and Anthropic compatibility, streaming, structured outputs, tool calling, async inference, management API, OpenAPI, Truss CLI and SDK are publicly documented. [Inference overview](https://docs.baseten.co/inference/overview) and [Reference](https://docs.baseten.co/reference/overview) (accessed 2026-08-12). | OpenAI-compatible inference, Python SDK, REST API, firectl, model playground, streaming and serverless/on-demand paths are documented. [API introduction](https://docs.fireworks.ai/api-reference/introduction) and [Onboarding](https://docs.fireworks.ai/getting-started/onboarding) (accessed 2026-08-12). | Proposed docs have curl/Python snippets, but the endpoint is not implemented. | **ADAPT NOW:** OpenAI-compatible happy path, exact error mapping, docs/versioning, copyable request, SDK smoke tests. **ADAPT LATER:** async/batch/streaming only after data-plane compatibility is proven. |
| Batch, async, fine-tuning, evaluation | Baseten publicly documents async inference, training/fine-tuning, chains, evaluations, and other workflows. [Inference overview](https://docs.baseten.co/inference/overview) (accessed 2026-08-12). | Fireworks documents batch inference, fine-tuning, evaluation, custom models, and server apps. These are not required for a first inference control plane. [On-demand quickstart](https://docs.fireworks.ai/getting-started/ondemand-quickstart) and [Changelog](https://docs.fireworks.ai/updates/changelog) (accessed 2026-08-12). | Product copy mentions adaptation/training as future feasibility; no implementation. | **REJECT FOR P0:** do not add training to the first inference release. **ADAPT AS A MANAGED BRANCH:** after dedicated-workflow value is proven, Alzette operates the customer's governed dataset, evaluation, adaptation, release, and rollback lifecycle. |

## B3. Patterns to adapt, defer, or reject

### ADAPT NOW

1. **Account/project context.** Make it difficult to operate the wrong tenant, project, or environment. Baseten’s organisation/team model and Fireworks’ account/user roles show that access context is a first-class product concept.
2. **First-call activation.** Both competitors reduce time to a working API call through a key, model catalogue/playground, and compatible client. Alzette should measure first success, not page views.
3. **A curated catalogue with availability metadata.** “Available” must mean available for this contract, mode, region, and version—not merely listed in a public garden.
4. **Make deployment tenancy explicit.** Dedicated is the primary offer; shared is optional. The customer needs a plain-language explanation of isolation, allocation, rate limits, predictability, and commitment before a route is activated.
5. **Stable route plus versioned release.** The customer application should not change its URL for every release. Promotion and rollback must be explicit and auditable.
6. **Operational observability.** Show live/stale state, request IDs, error class, latency/throughput, endpoint state, and incidents. Do not show a green dot without a data timestamp and source.
7. **Capacity safety.** Support request, token, and concurrency limits with clear scope and precedence. Cost budgets are required only where the customer contract is usage-based.
8. **Service identities.** Production workloads need service accounts or equivalent, not a departing employee’s personal secret.
9. **Enterprise evidence centre.** Give a risk gatekeeper a controlled place to view the agreement, data-handling policy, access/audit evidence, support path, and current status.
10. **Target adapter and tenant-route registry.** Resolve each request through a server-controlled tenant route. The target can be an external pilot API, a dedicated private model server, or an authorised shared pool without changing the customer API.
11. **Operator-reviewed deployment binding.** Dedicated/shared assignment and target addresses are operator-controlled. A requested model is not `ready` until the gateway can reach the assigned target and complete a health/inference check.

### ADAPT LATER

- SAML/OIDC SSO, SCIM/JIT provisioning, directory groups, and advanced RBAC after the first pilot’s identity requirements are known.
- Customer-facing canary/weighted traffic, multi-region routing, private networking, Prometheus/OpenTelemetry export, traces, and automated evaluations.
- Batch/async inference, fine-tuning, custom model import, LoRA, model publishing, and customer-managed storage.
- Granular cost allocation by application/team, webhooks, approval workflows, and self-serve commitment changes.
- Rich invoice/payment integrations once an actual billing system and finance workflow exist.
- A MeluXina Operations module for allocation intake, model-image deployment, private endpoint registration, health/restart, capacity inventory, and rollout automation. Define its contract during the forwarding pilot; implement it when authorised MeluXina access is available.

### REJECT FOR ALZETTE’S FIRST RELEASE

- Consumer accounts, chat history, or a general employee AI workspace.
- A broad public model marketplace and arbitrary customer model uploads.
- A synthetic “observatory” that maps nodes or reports throughput without a live telemetry source.
- Silent fallback between shared, dedicated, reserved, or regions; it could violate a customer’s contract and data boundary.
- Automatic budget bypass, hidden overage, or model rerouting after a cap.
- Prompt/output inspection as the default debugging mechanism.
- Copying competitor branding, UI wording, source code, or trade dress.
- A customer-operated MLOps console, or any Model Improvement implementation
  before one inference workflow is demonstrably valuable and its data rights
  are approved.
- A full HPC scheduler console, Slurm job authoring, OpenStack administration, or raw infrastructure credential management in the customer portal. These remain Alzette operator/LuxProvide surfaces.
- Direct customer access to an SSH tunnel, login node, or batch job as the production API. Those are PoC techniques only; a stable gateway/ingress boundary requires Slice 0 evidence.

## B4. Pricing, packaging, and enterprise-trust lessons

The public competitor pattern is a ladder: low-friction token-based shared access, a dedicated compute mode for performance/control, and enterprise or reserved commitments for predictable capacity and commercial terms. Fireworks explicitly separates serverless tokens, on-demand GPU time, and reserved capacity; Baseten separates Model API token pricing from dedicated compute and offers enterprise customisation. Alzette reverses the emphasis: dedicated managed inference is the primary offer, while a shared pool is an optional lower-isolation product for suitable workloads. This is a **packaging decision**, not a price recommendation.

For every offer, Alzette should make three operational concepts distinguishable:

1. **Actual consumption:** requests, tokens, concurrency, latency, errors, and time period from the gateway ledger.
2. **Allocated service:** the dedicated capacity or shared allowance promised by the agreement.
3. **Commercial record:** the fixed commitment, usage charge, overage, or invoice where the contract uses one.

The portal MUST show the source and timestamp for each. It MUST NOT turn a dashboard fixture, a catalogue placeholder, or an operator estimate into a price, SLA, or commitment. A dedicated customer may have no meaningful per-token customer charge; usage is still valuable for capacity planning, allocation, and service review.

MeluXina adds a material packaging constraint: its public allocation documentation describes compute in GPU/CPU node-hours and storage in GiB/files, while the customer promise is an inference API measured in requests/tokens and bounded by latency or availability. The infrastructure allocation is therefore an internal COGS/capacity input, not a customer invoice by itself. The product MUST preserve both layers:

| Economic layer | Required treatment |
|---|---|
| Infrastructure allocation and charge | Record the MeluXina Project ID, partition/VM, node-hours, storage, egress/support charges, grant/credit, invoice or allocation source, and finality in the operator module. [Resource allocations and monitoring](https://docs.lxp.lu/access/allocation_monitoring/) (accessed 2026-08-12). |
| Alzette customer meter | Attribute logical requests, tokens, route time, allocation, and any agreed overage to the organisation/project/environment/key/deployment. The Alzette gateway ledger is authoritative for customer consumption; MeluXina does not need to provide token-level billing. |
| Subsidy or programme benefit | Show INITIATE/Cashback80/EuroHPC support as a time-bounded, eligibility-dependent scenario with source and expiry. Do not build recurring customer pricing or margin assumptions on a benefit that has not been granted in writing. |
| Sustainable pilot economics | Calculate fully loaded cost with and without subsidy, including gateway, storage, network, operator/support, idle/queue waste, and recovery overhead. A “cheap” label is forbidden until the founder approves a baseline and actual records pass the PoC. |

The current live LuxProvide startup page advertises free access up to six months and up to 80% cashback, but does not publish a current numeric credit amount or full rate card. An official 2025 PDF contains older numeric INITIATE terms, and older PDFs contain still different durations/credit language. Treat the live page as a programme lead, not an entitlement; resolve the controlling terms before any commercial model or customer promise.

Enterprise trust is also a product workflow. Baseten and Fireworks expose or document identity, audit, data handling, region/residency, isolation, compliance, status, and support concepts. Alzette’s potential difference is not a badge; it is a narrower Luxembourg/EU contractual and operational boundary, local accountability, procurement-ready evidence, predictable metering, dedicated/reserved capacity, tenant isolation, and a curated catalogue. Every one of those is a **product hypothesis or operator claim until evidenced**.

### Alzette differentiation hypotheses to prove

| Intended difference | What the portal should eventually make visible | Evidence required before it is a claim |
|---|---|---|
| Luxembourg/EU hosting and jurisdiction | Exact processing/serving region, subprocessors, contract scope, and any customer-site option | Data-flow diagram, provider/placement records, DPA/contract, subprocessor list |
| Contractual commitments | Versioned agreement terms for availability, support, retention, models, capacity, and remedies | Executed agreement and an accountable commercial/operator owner |
| Local operational accountability | Named support owner, escalation path, status updates, incident history, and response terms | Runbook, on-call ownership, support records, and signed support policy |
| Regulated-customer procurement | Evidence pack, access review, audit export, data policy, model licence, and safe diagnostics | Current security/privacy/legal documents and a pilot procurement checklist |
| Predictable metering | Unit, estimate/finality, allowance, cap enforcement, commitment and invoice reconciliation | Live ledger, enforcement tests, invoice source, reconciliation report |
| Dedicated/reserved capacity | Eligibility, allocation, term, utilisation, expiry, overage and renewal state | Capacity ledger and contract; never a fixture card |
| Tenant isolation | Effective organisation/project/environment/key boundary and support-access record | Backend authorization tests, isolation review, and incident procedure |
| Narrow curated catalogue | Approved model/version, capability, licence, support, region and deprecation state | Model registry owner, licence review, release/change process |
| Preserved institutional AI identity | Company-controlled prompt/output vault, private evaluation corpus, approved terminology/method evidence, and versioned specialised model behaviour | Validated customer need, contracted data rights, vault isolation, evaluation baseline, customer release decision, and evidence that the specialised release outperforms the generic baseline for the named workflow |

These are the proposed wedge, not established moats. If validation shows that a target buyer does not value one of them, the product should revise the segment or promise rather than retain the claim for positioning symmetry.

---

# C. Portal product model

## C1. Entity model

| Entity | Purpose | Owned by | Minimum fields / invariants |
|---|---|---|---|
| Organisation / tenant | Evaluation or customer security/commercial boundary | Alzette account service; administered by organisation owner | Stable ID, display/legal name, account kind (`evaluation`/`customer`), lifecycle, creation source, approval evidence, contract IDs, timezone. The same ID survives qualification; every customer-visible resource resolves to exactly one tenant. |
| Self-service registration | Digest-only pre-account mailbox-verification state | Alzette account service | Normalized email, proposed names, notice/acceptable-use versions, token digest/generation/expiry/state, and completed user/organisation IDs. No password, API key, or capacity credential. |
| User | Human portal identity | Person; authorised through memberships | Stable identity, verified email/IdP subject, origin, status, roles, last sign-in, MFA/SSO state. Mailbox verification is not company authority. Deactivation revokes portal sessions. |
| Federated identity | Casdoor-authenticated identity link | Casdoor authenticates; Alzette links and enables | Exact issuer/subject, linked user, safe email-link evidence, status, authentication timestamps. Email/domain/IdP roles never replace local membership. |
| Role/membership | Permission assignment | Organisation and project admins | Organisation role plus project/environment memberships. Effective permission MUST be explainable. |
| Project | Workload and ownership boundary | Organisation | Stable ID, name, owner, status, purpose/tags, default environment. A project MUST NOT access another project’s secrets, routes, or usage without an explicit org-level permission. |
| Environment | Lifecycle context | Project | At minimum `development` and `production`; optional `staging` only if backed by the data plane. Environment policy includes model/capacity eligibility, endpoint alias, budgets, and promotion controls. |
| Service account | Non-human workload identity | Organisation/project admin | Name, owner, status, role, scope, expiry policy, last used, keys. It MUST NOT be a portal login. |
| API key | Revocable secret for a non-human workload | Service account | Key ID, non-secret `alz_k_` prefix, hash/reference, scope, created/expiry/revoked/last-used timestamps. Plaintext MUST be returned once only. |
| Human-agent grant/token | Short interactive inference authority | Person through one active membership | Grant binds user, membership, OAuth client, scope, and allowed aliases; random `alz_u_` token is digest-only, returned once, maximum ten minutes, and never a portal/service credential. |
| Catalogue model | Customer-readable model family | Alzette operator/model registry | Stable ID/slug, name/family, modality/capabilities, description, publication and lifecycle. Listing proves review intent, not licence, supply, price, or deployment. |
| Catalogue model version | Immutable approved release | Alzette operator/model registry | Version/digest, licence/support state, context limit, release/deprecation dates, compatible API, optional mapping to the existing routable `models` alias. |
| Deployment profile | Sellable technical capacity unit | Alzette model/runtime/finance owners | Model version, service mode, execution class, validated runtime/hardware class, accelerators and memory per unit, min/max units, region eligibility, evidence/finality, lifecycle. Customers choose profile/units, not a host. |
| Profile capacity metric | Evidence-backed expected endpoint capacity | Alzette performance owner | Metric/unit, min/target/max, per-unit/scaling semantics, measured/estimated/contractual finality, evidence reference. Metrics do not silently scale linearly. |
| Profile price | Time-bounded list/indicative unit price | Alzette finance | Currency, billing period, per-unit and setup amounts, visibility, effective dates, finality, source. A list price is not the customer's accepted quote. |
| Evaluation offer template | Server-owned free shared activation policy | Alzette operator/finance/security | One eligible shared profile, existing shared target/routable model, request/token/rate/concurrency/lifetime caps, terms versions, enabled/default state. Disabled until all enforcement gates pass. |
| Business qualification | Review of an evaluation organisation's authority and fit | Customer submits; Alzette approves | Bounded legal/workload data, state, reviewer/reason/evidence, timestamps. Approval changes organisation lifecycle only. |
| Deployment quote | Versioned customer-specific commercial/capacity snapshot | Alzette finance/operator; customer accepts | Tenant scope, profile, units, accelerator total, capacity snapshot, recurring/setup price, currency/period, execution boundary, finality/evidence, expiry, acceptance. Accepted quotes are immutable evidence, not runtime state. |
| Deployment request | Customer intent for a new endpoint or capacity change | Customer submits; Alzette fulfils | Scope, kind (`new_endpoint`, `scale_up`, `scale_down`), profile, requested/current units, quote, existing deployment when scaling, state, actors/timestamps. It cannot select target URL or raw machine. |
| Service plan | Commercial/technical offer | Alzette operator + contract | Tenancy mode (`dedicated` or `shared`), allocation/allowance, region, limits, price basis if applicable, contract link, status. Dedicated is the default; shared must be explicit. |
| Inference target | Internal model-serving destination | Alzette operator | ID, execution class (`external_pilot`, `meluxina`, or approved alternative), evidenced capacity mode (`dedicated` or `shared`), optional exclusive binding owner, OpenAI-compatible base URL or private LAN address/port, upstream model, secret reference, health, capacity, status. The URL and credentials are never customer-controlled. Exclusive configuration alone MUST NOT be presented as dedicated compute. |
| Target member / replica | One reachable server inside a target | Alzette operator/runtime | Target ID, address, port, health, weight, model digest, runtime, last heartbeat. All members of a dedicated target MUST belong to the same customer deployment. |
| Tenant route binding | Authorisation and routing decision | Alzette control plane | Tenant/project/environment/model alias, service plan, target ID, enabled state, effective dates, fallback policy, audit metadata. A binding to a dedicated target is exclusive; a shared target requires an explicit allow-listed binding per tenant. |
| Endpoint / route | Stable customer invocation target | Project/environment | Alias, model version, service plan, region policy, status, customer-facing endpoint URL, request schema, key scope, active target binding, last-known-good binding. |
| Deployment/release | Actual model/runtime deployment for one organisation scope | Data plane; viewed by customer, fulfilled by operator module | Profile, target/route only when assigned, state, validation/evidence, timestamps/error. Requested/quoted is distinct from allocating/deploying/validating/ready. |
| Deployment capacity revision | Immutable history of purchased and activated endpoint units | Contract/operator runtime | Deployment, quote, unit count, state/effective/end times, resource evidence. Exactly one active revision; expansion creates a new revision and preserves route identity. |
| MeluXina allocation | Internal infrastructure record | Alzette MeluXina Operations module | Project/allocation reference, partition/VM/service, resource class, term/expiry, placement, queue/capacity state, source timestamps, and cost/credit reference. It is infrastructure evidence, not a customer tenant or credential. |
| Model Improvement programme | Contracted boundary for one customer improvement objective | Customer approves; Alzette Model Improvement operates | Organisation/project/endpoint, objective, permitted source classes, prohibited uses, legal/contract evidence, approvers, retention/deletion policy, evaluation criteria, state, and term. It grants no inference or target authority. |
| Private interaction vault policy | Customer decision controlling prompt/output custody | Organisation; enforced by Alzette | Organisation/project/environment scope; mode (`none`, `selected`, or `policy_matched`); eligible actor/application/route/content classes; permitted purposes; viewer/exporter/selector/deleter roles; region, encryption/key policy, retention/deletion/legal-hold/backup rules; employee/client notice evidence; effective version and timestamps. Browser input cannot weaken the active policy. |
| Retained interaction | One tenant-isolated prompt/output exchange retained under policy | Customer-controlled vault operated by Alzette | Organisation/project/environment, logical request and actor lineage, ordered request/response content or encrypted object reference, completeness/finality, policy version and purpose, created/expiry/deletion/hold state, integrity digest, and access history. It is excluded from logs, analytics, support, and cross-customer use. |
| Private improvement dataset | Versioned tenant-isolated material prepared for evaluation/adaptation | Alzette custody under customer policy | Programme, immutable version, source/provenance references, inclusion approvals, redaction/transformation record, rights/purpose evidence, access policy, storage boundary, retention/deletion state, and digest. It is never populated automatically from ordinary telemetry. |
| Improvement run | Reproducible Alzette-operated evaluation or adaptation job | Alzette Model Improvement | Programme, base model release, dataset version, method/configuration, runtime/hardware evidence, operator, timestamps, safe logs, cost, artefact digests, result, and failure class. Customers do not submit raw training infrastructure. |
| Candidate model release and evaluation | Decision evidence for promotion or rejection | Alzette prepares; authorised customer approves | Base/candidate releases, evaluation-set version, baseline and candidate results, acceptance thresholds, limitations, model/licence evidence, approval/rejection, deployment record, and last-known-good rollback target. A candidate is not routable before approval and validation. |
| Inference request | One logical customer call and the customer usage source | Gateway/metering service | Request/time, tenant/project/env, route, deployment, identity/key attribution, result/status, input/output/cache/reasoning tokens where available, end-to-end latency, usage finality, optional customer charge. No prompt/output content by default. It counts once regardless of internal retries. |
| Provider attempt | One outbound execution attempt made for an inference request | Gateway/operator telemetry | Request ID, attempt number, target/member, upstream request ID, model, start/end, status/error, tokens where returned, latency, internal cost where relevant. Attempts are operator evidence and MUST NOT inflate customer request counts. |
| Usage rollup | Query-optimised company consumption aggregate | Metering service | Hour, tenant/project/env/route/deployment/model, request/error counts, tokens, latency percentiles, peak concurrency, source/finality. It MUST reconcile to inference requests. |
| Budget/cap | Spend/rate guardrail | Organisation/project/environment admin | Scope, period, currency/unit, threshold, soft/hard mode, enforcement state, effective time, precedence, notification targets. |
| Commitment | Reserved capacity or fixed commercial term | Contract service | Contract ID, mode, resources, region, start/end/renewal, allowance, price, overage, invoice schedule, status. Read-only to customers at MVP. |
| Invoice/statement | Finance record | Billing service | Number, period, status, due date, currency, line items, source document, usage reconciliation status. |
| Audit event | Immutable action record | Audit service | Time, actor, role/source, tenant, project/env/resource, action, result, correlation ID, safe metadata. Never include secrets or prompt/output content. |
| Alert | User-visible threshold/health notification | Alert service | Rule, scope, severity, state, triggered/resolved time, delivery, acknowledgement. |
| Incident | Service disruption or customer case | Alzette support/status service | Status, severity, affected services/regions/routes, start/update/resolution, customer-safe detail, references. |
| Support request | Account or technical help | Alzette support service | Requester, scope, category, severity, request IDs, safe diagnostics, status, owner, timestamps, audit link. |

## C2. Ownership and isolation boundaries

- **Organisation boundary:** contract, users, billing, commitments, approved catalogue, audit, support, and organisation-wide budgets belong here. Customer administrators can see only their organisation.
- **Project boundary:** application/workload separation, project members, service identities, routes, environments, and project budgets belong here.
- **Environment boundary:** development/test and production must have distinct credentials or scopes, endpoint aliases, policies, and promotion controls. A development key MUST NOT call production unless explicitly scoped and audited.
- **Data-plane boundary:** the runtime enforces tenant/project/route authorization. The portal is not the enforcement point by itself. A UI filter is not isolation.
- **Target boundary:** an inference target is operator-controlled infrastructure. A dedicated target has one owning tenant; a shared target has explicit tenant-route bindings and independent limits. No customer supplies an upstream address.
- **Fallback boundary:** retry/failover may choose another healthy member of the same authorised target. Crossing to another target, tenancy mode, model, or execution location requires an explicit route policy and contract; the default is fail closed.
- **Support boundary:** Alzette support access is time-bounded, least-privileged, customer-visible in audit, and disabled by default. The exact mechanism is an open founder decision.
- **Content boundary:** the current PoC remains metadata-only. In the target
  dedicated service, prompt/output content may enter only the organisation's
  tenant-isolated private interaction vault under the effective server-side
  policy. Vault content is never a log, analytic event, support attachment, or
  Alzette/cross-customer asset. The customer controls permitted access,
  purpose, export, selection, retention, and deletion subject to recorded
  contractual/legal/backup/hold limits.
- **Model Improvement boundary:** improvement content exists only inside a
  separately contracted, tenant-isolated programme with explicit permitted
  sources, purpose, custody, access, retention/deletion, and approval. Alzette
  operates the work; portal users cannot turn ordinary traffic into training
  data, launch arbitrary jobs, or promote a candidate directly.
- **Commercial boundary:** contracts may make region, model, retention, capacity, and price different per tenant. A catalogue item can be globally known but unavailable to a tenant.
- **Evaluation boundary:** self-service creates a new isolated organisation bound
  only to the configured shared offer. It cannot join an existing company by
  domain, alter its allowance, or obtain dedicated capacity. Evaluation usage
  is real but is not production/customer-traction evidence without its label.
- **Intent/evidence boundary:** catalogue, profile, quote, request, allocation,
  deployment, target, route, and observed health are distinct. No earlier state
  can be rendered as proof of a later one.

### C2.1 Runtime boundary: external pilot now, private MeluXina targets later

The customer-facing path is stable while the internal destination changes:

`customer application → Alzette gateway/auth/policy/metering → tenant route → inference target → model`

During the pilot, the inference target forwards to an approved external OpenAI-compatible API. In the intended production topology, Alzette services run inside the MeluXina operating environment and the target resolves to an Alzette-managed model server on a private LAN address and port. The gateway contract, customer URL, API key, request ID, and usage semantics remain stable across that migration.

Therefore:

- Authentication MUST resolve `tenant → project → environment → model alias` before any target is selected.
- The gateway MUST load a server-controlled tenant-route binding and MUST reject unknown, disabled, cross-tenant, or incompatible bindings. A request body cannot override the target URL.
- A target evidenced as dedicated MUST have exactly one owning tenant. A shared target MAY have one or many tenant bindings, but each binding has independent authorisation, limits, usage attribution, and contract status. An exclusive Alzette binding to a shared external API remains shared capacity.
- External pilot targets MUST be labelled `external pilot` in operator and authorised customer evidence. They MUST NOT be described as on-premise, private, or MeluXina-hosted.
- Production targets on MeluXina SHOULD use private addresses/service discovery. Raw LAN addresses, provider credentials, SSH keys, Slurm tokens, OpenStack credentials, and external API secrets remain operator-side data.
- One logical inference request MAY create multiple runtime attempts before output begins. The customer ledger counts the logical request once; the operator records each attempt. After streamed output begins, transparent cross-target retry is forbidden unless the protocol proves safe continuation.
- Scheduling, allocation exhaustion, node/VM failure, maintenance, and model-server failure MUST translate into customer-safe states and machine-readable errors. A deployment is not `ready` until the gateway completes a real health/inference check.
- Customers configure model, service mode, profile, and unit intent in the
  portal. The separate MeluXina Operations module owns physical allocation,
  model/runtime deployment, private target registration, health/restart,
  capacity inventory, and rollout evidence until those operations are safely
  automated. The customer never selects a host or private address.

### C2.2 Product modules

| Module | Primary user | P0 responsibility | Later responsibility |
|---|---|---|---|
| Inference gateway | Customer applications | Strict text/function-tool OpenAI-compatible ingress with buffered and SSE responses, authentication, tenant-route resolution, forwarding, request validation, pre-output-only retry, and request/attempt metering | Target pools, policy-driven failover, multimodal/structured/other API capabilities |
| Customer portal | Evaluation/customer administrators, developers, finance/viewers | Shared evaluation first call, catalogue/configuration, projects, credentials, route identity, deployment health, company consumption, basic service/contract context | Enterprise identity, private interaction vault policy/access, richer exports, alerts, commercial workflows |
| Catalogue and commercial control | Customers configure; Alzette model/finance/operator owners approve | Published models/profiles, evidenced capacity metrics, versioned prices/quotes, deployment and capacity-change requests | Automated quoting, payment/invoice integration, more profiles |
| Control service | Alzette software | Signup/tenant lifecycle, tenant/project/route/target registry, policy, audit, usage APIs | Approval and fulfilment automation, richer catalogue/release policy |
| Public site | Prospective customers and integrators | Finished-offer positioning, exact PoC implementation documentation, and configured links to signup/login; no tenant data or runtime credentials | Published catalogue/service specifications after evidence and approval |
| MeluXina Operations | Alzette operators | Contract/schema and manual target registration only; no MeluXina dependency for the forwarding pilot | Allocation, model deployment, runtime images, private endpoint/service discovery, restart, capacity, rollout, infrastructure evidence |
| Model Improvement | Customer business/data/model approvers; Alzette improvement operators | Product boundary and unavailable-state only; no prompt/output capture or training in P0 | Contracted programme intake, private dataset preparation, evaluation, approved adaptation, artefact custody, customer release decision, deployment handoff, and rollback evidence |

All P0 components MAY be processes from one Go codebase and one image, backed by one PostgreSQL database and deployed with Docker Compose on one machine. Module boundaries are ownership and API boundaries, not a requirement for independent distributed systems.

### C2.3 P0 deployment topology

The reference deployment is deliberately small:

`internet/TLS → ingress → gateway → tenant-route registry → inference target`

`customer browser → ingress → control/portal → PostgreSQL`

`public browser → standalone public site (no PostgreSQL or provider credential)`

The application SHOULD ship as one Go module and image with four long-running process modes:

- `alzette gateway`: customer inference ingress, auth, route resolution, bounded buffered or text/function-tool SSE forwarding, validation, and request/attempt ledger writes;
- `alzette control`: customer/operator APIs, portal assets, provisioning, keys, routes, usage queries, audit;
- `alzette public`: public landing and implementation docs from a static root isolated from portal assets and secrets;
- `alzette worker`: health probes, rollups, exports, retention and other asynchronous maintenance.

Docker Compose SHOULD contain ingress/TLS, those application processes, PostgreSQL, and operational telemetry. Customer usage and any contract-relevant meter MUST come from PostgreSQL, not Prometheus. Prometheus/OpenTelemetry/Grafana MAY observe service operations but are not the customer ledger. Redis, Kafka, Kubernetes, ClickHouse, and a workflow engine are out of scope until measured load or reliability evidence requires them.

## C3. Role and permission matrix

The launch role set is intentionally smaller than a full enterprise IAM product.

| Permission | Org Owner | Org Admin | Finance/Contract | Project Admin | Developer | Auditor/Viewer |
|---|---:|---:|---:|---:|---:|---:|
| View organisation status, contract summary, approved models | ✓ | ✓ | ✓ | ✓ | limited | ✓ |
| Manage organisation members and org roles | ✓ | ✓ | — | — | — | — |
| Manage project membership and project settings | ✓ | ✓ | — | ✓ | — | — |
| Create/revoke service accounts and production keys | ✓ | ✓ | — | ✓* | — | — |
| Use a permitted endpoint/inference route | ✓ | ✓ | — | ✓ | ✓ | optional read-only/test |
| Create/test a development route | ✓ | ✓ | — | ✓ | ✓** | — |
| Promote to production or rollback | ✓ | ✓ | — | ✓*** | — | — |
| Configure budgets, caps, alerts | ✓ | ✓ | ✓ for org budgets | ✓ for project budgets | — | — |
| View/export usage and invoices | ✓ | ✓ | ✓ | project scope | project scope | ✓ |
| View audit events | ✓ | ✓ | ✓ for commercial events | project scope | own actions where allowed | ✓ |
| Change contract/commitment terms | request only | request only | request only | — | — | — |
| Open support/incident request | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

`*` Production key creation SHOULD require Project Admin plus an explicit project policy; `**` developer actions MUST be limited to development and approved test routes; `***` a separate production-approval policy MAY require Org Admin for regulated pilots.

The permission service MUST return an explanation such as “allowed by Project Admin membership on project P, environment development” or “blocked because the model is not in this contract.”

---

# D. Information architecture

## D1. Proposed navigation

| Surface | Job to be done | Scope |
|---|---|---|
| Overview | Tell the user account stage, evaluation allowance or customer service state, endpoint health, cap/budget, active incident, first-call/setup progress, and contract context | Organisation with project/environment filters |
| Projects | Create and select workload boundaries; show environment and route readiness | Organisation → project |
| Catalogue | Compare approved model releases and compatible deployment profiles; understand capability, licence, mode, capacity evidence, and price availability | Public/authenticated catalogue; tenant eligibility is explicit |
| Deployments / Routes | Configure a new endpoint or capacity change, review quote/request state, then inspect/test the assigned deployment and stable route | Project → environment |
| Usage | Understand company consumption: requests, tokens, latency, errors, concurrency, allocation/utilisation, and export | Organisation or project; time range |
| Access | Manage members, roles, service accounts, API keys, and access review | Organisation and project |
| Service & contract | Read evaluation limits or customer service plan, quotes/commitments, dedicated/shared mode, allocation/allowance, region/retention terms, support, and available invoices/statements | Organisation only |
| Activity / audit | Explain who changed what, when, where, and result | Organisation or project, permission-gated |
| Status & support | See current incidents, service status, safe diagnostics, and open/request support | Organisation/service |
| Documentation | Make the first call and interpret errors without contacting support for basics | Global, with project-specific endpoint values |

## D2. Organisation-level versus project-level controls

**Organisation-level:** members, roles, identity policy, contract and data terms, approved catalogue, capacity commitments, organisation budget/cap, invoices, compliance evidence, audit search, service status, support contacts, currency/timezone.

**Project-level:** project members, environments, model eligibility, endpoint aliases, release promotion, service accounts/keys, project budgets/caps, request/usage attribution, route health, test calls, project export.

The shell MUST keep both contexts visible. A user should never see “Production” without the project name and organisation name. Destructive actions MUST state the full scope: “Revoke key `orders-prod` in Organisation A / Project B / Production.”

## D3. Account switcher and environment context

- The organisation switcher appears globally only for users with more than one organisation and always displays the current organisation name.
- Account kind and lifecycle are persistent context: `Evaluation · Shared` can
  never look like `Customer · Dedicated`, and qualification approval cannot
  make a requested deployment look ready.
- The project switcher is required on project surfaces; the current project is shown in the URL or equivalent navigational state.
- Environment is a required context on endpoint, key, usage, and promotion surfaces. `Development` and `Production` are not interchangeable labels.
- A context change MUST reset or revalidate filters and MUST NOT silently carry a selected key, endpoint, or destructive action into the new context.
- The portal SHOULD show a context breadcrumb such as `Organisation / Project / Production` and a data freshness timestamp on live panels.

---

# E. Critical end-to-end workflows

The common state contract applies to all workflows: every live panel has `loading`, `ready`, `empty`, `stale`, and `error` states; every write has an idempotency/correlation ID; every permission failure explains the required role; no failure reports success merely because a request was accepted by the portal.

## E1. Organisation provisioning

The exact signup, evaluation provisioning, invitation, and first-administrator
contract is defined in
[`ACCOUNT_ONBOARDING_PRD.md`](ACCOUNT_ONBOARDING_PRD.md). Until that companion
is implemented, this workflow remains an intended product flow and the current
PoC uses operator-provisioned usernames/passwords.

**Preconditions:** self-service evaluation requires verified transactional
email, abuse controls, and an enabled server-owned shared evaluation offer with
enforced hard limits. Operator-assisted onboarding requires an approved
organisation and exact invitation scope. Dedicated production additionally
requires business approval and explicit contract/quote evidence.

**Evaluation happy path:** verify business email → authenticate or create the
Casdoor identity → atomically link it and create the evaluation organisation/
development scope/membership and server-owned shared route → enter the portal
labelled `Evaluation · Shared` → connect an interactive agent with short human
access or create a separate workload application key → make the first real call
→ see own usage.

**Approved-customer path:** submit qualification → operator approves business
with evidence → retain the same organisation ID → review contract/data boundary
→ configure and accept an eligible dedicated endpoint quote → operator fulfils
and validates infrastructure → active route becomes available. An
operator-created organisation may instead begin through the scoped invitation
path.

**Dangerous/destructive confirmation:** provisioning MUST NOT activate paid capacity or production traffic without an explicit confirmation showing plan, estimated/contracted commercial effect, region, and owner. Leaving or cancelling setup must preserve the pending record; deleting an organisation is not a P0 customer action.

**Empty/loading/error/permission states:**

- Loading: show which provisioning source is being read; do not show placeholder contract terms.
- Empty evaluation: show first-call guidance and remaining shared allowance,
  not an empty production dashboard. Empty customer: “No approved deployment is
  attached yet”; link to Catalogue/configuration.
- Error: preserve the setup step and correlation ID; allow retry; do not create duplicate org/project records.
- Permission: invited user may accept and view assigned setup; only owner/admin can confirm contract or create project.

**Audit event:** `organisation.provisioned`, `organisation.contract_viewed`, `project.created`, `environment.created`, with actor/source and result.

**Acceptance criteria:**

1. A new tenant cannot see another tenant’s projects, catalogue eligibility,
   quote, contract, route, allowance, or usage in API or UI tests.
2. Verification/provisioning is idempotent: retrying creates no duplicate
   tenant, membership, project, route, plan, allowance, or key.
3. Signup cannot change the configured shared target/model/limits and creates no
   dedicated or paid capacity.
4. Production actions remain disabled when approval/quote/contract lacks
   required region, capacity, retention, model, or authority fields.
5. The evaluation owner can reach a real first-call checklist immediately.

## E2. Invite a team member and assign a role

This workflow is specified technically in
[`ACCOUNT_ONBOARDING_PRD.md`](ACCOUNT_ONBOARDING_PRD.md), with employee identity
and agent connection controlled by
[`WORKFORCE_AGENT_ACCESS_PRD.md`](WORKFORCE_AGENT_ACCESS_PRD.md). An invitation
grants only its exact membership after acceptance; it never provisions
inference infrastructure or creates an application API key or human-agent
token.

**Preconditions:** owner/admin has a verified organisation; the invitee email/domain is permitted; the target project/environment is known.

**Happy path:** admin enters email → selects org role and project/environment membership → sees permission summary → sends invite → invitee accepts → membership becomes active → admin can review last sign-in and effective permissions.

**Dangerous/destructive confirmation:** changing an admin to viewer, removing a member, or disabling a user MUST state affected projects and that their short human-agent grants will stop working. Separately owned service-account keys are not revoked implicitly; the portal MUST inventory them so production workloads are not accidentally broken. Revoke invite is confirmable and reversible only by sending a new invite.

**States:**

- Empty: no members beyond owner; show invite CTA and least-privilege guidance.
- Loading: show invite delivery and membership provisioning separately.
- Error: distinguish invalid domain, existing member, expired invitation, directory/IdP error, and service failure.
- Permission: Finance/Auditor can view permitted members but cannot change them; Project Admin cannot alter org roles.

**Audit event:** `member.invited`, `member.accepted`, `member.role_changed`, `member.removed`, `membership.project_changed`.

**Acceptance criteria:**

1. A member with Project Admin on Project A cannot list, mutate, or infer secrets/routes/usage for Project B.
2. An invitation expires according to an operator-defined policy and can be revoked; the UI shows its status.
3. Every role change records old role, new role, scope, actor, and result.
4. Removed users cannot create new portal sessions; existing sessions are invalidated within the documented security target.
5. Removed users cannot mint or use a human-agent token for that membership; service-account keys remain independent until explicitly revoked.

## E3. Create a project

**Preconditions:** organisation is provisioned; actor has org admin/owner role; contract permits another project or operator approval is available.

**Happy path:** select Create project → enter stable name/description/purpose → choose default environment policy → select approved model/capacity eligibility or “choose later” → review budget inheritance → create → land on project readiness page.

**Dangerous/destructive confirmation:** project deletion is not P0. Archive/deactivate requires confirmation, impact summary, route/key count, and a recovery policy. A project with production routes MUST require operator/admin review before archival.

**States:**

- Empty: no projects; explain why a project exists and offer one default.
- Loading: show policy/catalogue resolution.
- Error: duplicate name, contract limit, invalid policy, or unavailable capacity are distinct.
- Permission: Project Admin can edit project settings but cannot create organisation-level projects unless granted.

**Audit event:** `project.created`, `project.settings_changed`, `project.archived`.

**Acceptance criteria:**

1. Project IDs are immutable and unique within the tenant.
2. A project can be created without creating paid runtime capacity.
3. Inherited org budgets and model policies are displayed before creation and remain traceable after creation.
4. Archive is blocked or explicitly reviewed when active production routes exist.

## E4. Generate, rotate, and revoke credentials

**Preconditions:** actor has permitted role; target identity, project, environment, model/route scope, and expiry policy are known; secret store/key issuer is live.

**Happy path:** choose service account or approved development identity → name key → choose minimum scopes → choose expiry/rotation date → confirm → receive plaintext once → copy/download instructions → see masked key prefix, created time, expiry, last-used state, and status.

**Dangerous/destructive confirmation:** revoke MUST show the exact key, identity, project/environment, last-used time, and likely affected routes. Rotation MUST create the replacement before revocation and support a short overlap. The portal MUST never ask the user to paste a secret to verify it.

**States:**

- Empty: “No production credential exists”; recommend service account.
- Loading: key issuance progress must not display a fake key.
- Error: secret-store failure means no key is considered created; duplicate submission is idempotent.
- Permission: developers may create only permitted development keys; viewers never see reveal/copy controls.

**Audit event:** `service_account.created`, `api_key.created`, `api_key.revealed`, `api_key.rotated`, `api_key.revoked`, `api_key.expired`.

**Acceptance criteria:**

1. Plaintext is returned exactly once and is not retrievable from UI, API response history, audit, analytics, or logs afterward.
2. A revoked key fails data-plane authentication within the documented revocation target.
3. A key cannot call a project/environment/model outside its scope, even if the user who created it can.
4. Rotation can be completed without downtime when both keys are valid; the old key is then revoked and visibly confirmed.
5. Secret values never appear in URLs, browser history, error messages, screenshots generated by automated tests, or support records.

## E5. Inspect or request a model deployment and service mode

**Preconditions:** the forwarding gateway has passed the software PoC; at least
one catalogue release/profile has an operator owner and evidence status;
project/environment and actor authority are resolved. Shared evaluation also
requires one enabled hard-capped offer. Dedicated fulfilment requires a
qualified organisation and capacity source; MeluXina allocation is required
only when that execution class is promised.

**Shared evaluation path:** signup binds the server-selected shared evaluation
profile/route → user sees exact allowance and execution label → user creates a
separate key and makes a first call → usage reduces the enforced allowance →
exhaustion blocks before a provider attempt. The user cannot pick a different
shared target or multiply free allowance by resubmitting signup.

**Dedicated configuration path:** open Catalogue → choose model release → select
`Dedicated private` → enter workload/concurrency/context/latency intent → review
compatible profiles and their evidence → choose capacity units → submit
deployment request → receive versioned price/capacity quote → authorised actor
accepts while valid → operator allocates hardware, deploys pinned runtime,
validates it, registers a dedicated target, and binds the stable route → customer
tests the endpoint.

**Capacity expansion:** open an existing dedicated endpoint → compare current
active units with supported larger configurations → request new total units →
review delta and resulting full price/capacity snapshot → accept → operator adds
and validates resources → activate one new capacity revision behind the same
route. A profile must explicitly state which metrics scale per unit; the UI
cannot multiply a benchmark that is not marked scalable.

**Dangerous/commercial confirmation:** quote acceptance MUST show organisation,
project/environment, endpoint/new endpoint, model/version, service mode,
execution boundary, current/requested units, total accelerators, capacity metric
finality/evidence, recurring/setup price, tax/term status, expiry, and what
happens next. Acceptance records commitment intent but MUST NOT claim payment,
allocation, deployment, or readiness. Changing a production model/version uses
promotion/rollback, not an inline selector.

**States:**

- Empty: no model/profile is eligible; explain which eligibility, licence,
  region, or capacity evidence is missing and provide a request/support action.
- Loading: catalogue cards show `checking eligibility`; quotes and deployment
  state load independently.
- Error: distinguish catalogue unavailable, licence review pending, profile
  evidence missing, quote expired, business review pending, capacity
  unavailable, allocation failed, deployment failed, and health validation
  failed.
- Permission: prospects/viewers can inspect published entries; evaluation admins
  can configure; only authorised customer roles can accept a quote; only
  operators can assign targets or physical resources.

**Audit event:** `catalogue_model.viewed`, `deployment_profile.selected`,
`deployment.requested`, `deployment_quote.offered`,
`deployment_quote.accepted`, `deployment.capacity_change_requested`,
`deployment.capacity_revision_activated`, `target_binding.created`,
`target_binding.changed`.

**Acceptance criteria:**

1. Every selectable model has an exact catalogue version, licence/support state,
   lifecycle, source timestamp, and at least one eligible profile; an alias alone
   is insufficient.
2. Every quotable dedicated profile declares accelerator class/count per unit,
   min/max units, capacity metric finality/evidence, and price availability.
3. A customer cannot activate a model/profile/target unavailable to their
   organisation by manipulating IDs, units, aliases, URLs, quote, or request.
4. Catalogue, eligible, configured, quoted, accepted, approved, allocating,
   deploying, validating, target reachable, route ready, and serving are
   separate states in API and UI.
5. A ready dedicated deployment binds only a dedicated target owned by the same
   organisation; shared evaluation binds only the configured shared target.
6. Scale-up produces a new capacity revision and retains the same route URL,
   alias, credential contract, model, execution class, and tenancy mode unless a
   separate explicit migration is approved.
7. Quote and metric snapshots remain historically readable after catalogue
   price/profile changes; accepted quotes are immutable.

## E6. Create, test, promote, and roll back an endpoint/route

**Preconditions:** project/environment exists; model/version, service plan, and tenant-route binding are eligible; the assigned inference target is configured; credential scope is available; test input policy is displayed.

**Happy path:** create or receive a draft route with stable alias → operator/control service attaches an authorised target binding → gateway verifies target health → user sends a redacted/synthetic test request through the actual customer path → portal shows response status, latency, request ID, model/version, execution class, deployment state, and usage metadata without storing content → authorised actor promotes the tested binding/release where promotion is supported → portal shows the stable production route and last-known-good binding.

Rollback: select a previous healthy release → view impact and reason → confirm → runtime shifts/attaches the previous release → portal verifies health and records result.

**Dangerous/destructive confirmation:** production promotion and rollback MUST show route, current release, candidate/target release, traffic impact, expected capacity/contract effect, and confirmation text. Deleting/deactivating a production route is not P0 self-serve; “pause” must explain whether requests fail, queue, or stop.

**States:**

- Empty: no route; provide “create approved route” or operator request.
- Loading: `requested`, `provisioning`, `testing`, `ready`, `promoting`, `active`, `rolling_back` with timestamp and retry guidance.
- Error: `capacity_unavailable`, `model_incompatible`, `build_failed`, `health_failed`, `permission_denied`, `budget_blocked`, `unknown`; preserve logs/request ID.
- Permission: developer may test development; production promotion requires policy role; viewer can inspect only.

**Audit event:** `route.created`, `release.provisioning_started`, `route.tested`, `route.promoted`, `route.rollback_requested`, `route.rollback_completed`, `route.deactivated`.

**Acceptance criteria:**

1. The route displayed as active is verified by a gateway health and inference response, not merely by a successful portal/control write; target, queue, or allocation state is visible to the appropriate role when relevant.
2. A first successful test request is made through the real OpenAI-compatible endpoint and returns a request ID/correlation ID.
3. Production URL/alias remains stable across a release promotion.
4. Rollback is possible to the last-known-good target binding/release without editing a database or affecting another tenant.
5. A failed provisioning/promotion cannot leave the portal showing `active`.
6. Test content is synthetic or explicitly consented; the portal does not retain it by default.

## E7. Inspect deployment health and company consumption

**Preconditions:** route exists; live gateway telemetry and the request ledger identify tenant, project, deployment, model, source, and freshness; actor has read permission.

**Happy path:** open Usage → select organisation/project/environment and period → see service plan, dedicated/shared mode, execution class, deployment health, last heartbeat, model version, allocation or allowance, and freshness → inspect logical requests, success/error classes, input/output/cache/reasoning tokens where reported, latency, time to first token where streaming, throughput, and peak concurrency → compare actual use with dedicated capacity or shared limits → break down by project/application, deployment/model, route, and service account → open safe request metadata by request ID → export permitted records.

For a dedicated offer, the primary commercial question is utilisation of purchased capacity, not an invented per-token bill. For a shared offer, the portal MAY show allowance, overage, or estimated charge if the contract and meter define them. Internal MeluXina node-hours, external-provider charges, and retry costs belong to the operator view unless the agreement explicitly exposes them.

**Logical request versus attempt:** one client call creates one `inference_request`. Before output begins, the gateway MAY create multiple `provider_attempts` because of retry or same-policy failover. The customer request count remains one. The successful response usage, end-to-end result, and contract policy determine customer consumption; every attempt remains available to operators for reliability and internal cost analysis.

**Dangerous/destructive confirmation:** no destructive action is needed for viewing. Export MUST warn if data is partial, estimated, delayed, or includes personal identifiers. Any prompt/output view later introduced requires a separate policy and confirmation; it is not an MVP feature.

**States:**

- Empty: zero usage is a valid state and must say “No requests recorded in this period,” not “No data available.”
- Loading: skeleton with scope/time range; never show old values as current without a stale label.
- Stale: show last successful timestamp, age, source, and retry; do not use green `healthy` as a substitute for fresh telemetry.
- Partial: upstream token detail is missing or a stream did not finish; show known fields and `partial`/`unknown`, never zero by assumption.
- Error: distinguish telemetry unavailable, metering delayed, permission denied, and deployment unavailable.
- Permission: users see only permitted projects; contract amounts may be org-admin/finance only; internal target attempts and COGS are operator-only.

**Audit event:** `health.viewed`, `usage.viewed`, `request_metadata.viewed`, `usage.exported`.

**Acceptance criteria:**

1. Every chart/table states scope, period, timezone, unit, source, and freshness.
2. A request metadata view contains no prompt/output by default and supports safe correlation to the endpoint log.
3. Customer totals reconcile to logical gateway requests. Retries do not inflate request counts, and attempt totals are separately reconcilable by operators.
4. Dedicated customers see allocation/utilisation and shared customers see allowance/limit context without exposure of other tenants or global shared-pool capacity.
5. A zero-usage period, partial period, delayed ledger, and service outage each render distinct copy and state.
6. A user cannot infer another tenant’s deployment, target address, usage, or shared-pool membership by changing filters or resource IDs.

## E8. Set usage limits, budgets, and alerts

**Preconditions:** gateway metering and enforcement services exist; any target capacity/allocation constraint has an identified source and precedence; actor can set the requested scope; unit and period are known. Currency is required only for a monetary budget.

**Happy path:** choose org/project/environment → configure an allowed request/token/concurrency limit and, where contracted, a monetary budget → select thresholds and recipients → review precedence with inherited and deployment limits → confirm → portal displays effective limit and enforcement state → alerts fire and resolve with deduplication.

**Dangerous/destructive confirmation:** reducing a cap below current usage, enabling a hard cap, disabling alerts, or removing a limit MUST show the routes/projects affected and whether requests may be rejected immediately. Hard cap enforcement MUST be explicit; a notification-only budget cannot be labelled a cap.

**States:**

- Empty: no budget configured; show contract/default state and consequence.
- Loading: show source of effective policy; writes are disabled during conflict resolution.
- Error: invalid amount/currency, conflicting scopes, enforcement unavailable, or notification failure are distinct.
- Permission: finance may set org spending policy if allowed; project admin may set project policy; developer cannot bypass.

**Audit event:** `budget.created`, `budget.updated`, `cap.enforcement_changed`, `alert.created`, `alert.triggered`, `alert.acknowledged`.

**Acceptance criteria:**

1. The portal shows the effective customer limit and target allocation/allowance after all inherited policies, and states which rule wins.
2. A hard-cap test rejects new eligible requests at the data plane and records a machine-readable reason; it does not silently switch models or capacity.
3. Alert delivery is observable and idempotent; repeated polling does not duplicate alerts.
4. Every budget/cap change records old/new value, scope, actor, effective time, and result.

## E9. View service plan, commitment, and available commercial records

**Preconditions:** contract/billing source is connected; actor has finance or permitted read access; invoice/commitment identifiers exist or the portal explicitly says none exists.

**Happy path:** open Service & contract → view agreement version and effective dates → see dedicated/shared mode, assigned model/deployment, region, retention, allocation/allowance, overage if any, support, and SLA fields with contractual source → compare company usage with the service commitment → open an invoice/statement only when a billing integration exists.

**Dangerous/commercial confirmation:** configuring a capacity increase and
accepting its versioned quote are self-service when the actor is authorised and
reauthenticated. Activation remains separate: no payment, renewal,
cancellation, allocation, deployment, or revised-term acceptance may be
inferred or performed without its explicit workflow and source of authority.

**States:**

- Empty: “No invoice/commitment is published for this organisation”; do not fill with fixture values.
- Loading: separate agreement, commitment, and invoice loading states.
- Error: billing unavailable, document missing, permissions, or reconciliation pending.
- Permission: finance sees invoices and commercial amounts; developer sees only project allowance/cost needed to operate, if policy permits.

**Audit event:** `contract.viewed`, `commitment.viewed`, `invoice.viewed`, `invoice.exported`, `commercial_request.created`.

**Acceptance criteria:**

1. Every contractual value links to an agreement/document version or is labelled unavailable.
2. An expired commitment is visually and semantically distinct from a healthy active commitment.
3. Operational consumption, allocation/allowance, and invoiced amount cannot be shown in the same field without labels.
4. A customer can export a service/usage record without exposing another project’s data. Invoice reconciliation is P1 unless required by the pilot contract.

## E10. Report an incident or request support

**Preconditions:** user is authenticated; support/status service exists; safe request metadata can be attached.

**Happy path:** user selects service/route/project → chooses severity/category → portal pre-fills endpoint status, last refresh, request IDs, error class, release, and time range → user adds a description without prompt/output content → submits → receives case/incident ID and expected update path → status and support history remain visible.

**Dangerous/destructive confirmation:** submitting a support request is not a commercial commitment. Before attaching a payload or screenshot, the portal MUST warn that it may contain sensitive data and default to metadata-only diagnostics. Any emergency contact instruction must be explicit and contract-specific.

**States:**

- Empty: show status page and documentation before asking for a case; still allow “no matching incident.”
- Loading: show safe-diagnostics collection and submission status.
- Error: preserve a local draft without secrets/content; provide manual contact and correlation ID.
- Permission: any permitted member may report a technical issue; commercial cases require finance/owner or are routed for review.

**Audit event:** `support_request.created`, `support_request.updated`, `incident.linked`, `diagnostics.attached`.

**Acceptance criteria:**

1. A case can be submitted with metadata only and includes a request ID when one exists.
2. Prompt/output content is not automatically attached.
3. The user can distinguish platform incident, route-specific failure, budget block, and account/support request.
4. Submission result has a durable ID and is visible to permitted members.

## E11. Govern the company interaction vault

**Preconditions:** the organisation has a subscribed dedicated endpoint;
content custody, location, encryption, retention/deletion, backup, legal-hold,
employee/client notice, and Alzette operator-access terms are contracted; the
actor has the dedicated data-governance permission.

**Happy path:** open Data custody → choose organisation/project/environment
scope → select `Retain none`, `Retain selected interactions`, or a bounded
`Retain interactions matching this policy` mode → define eligible people,
applications, routes, and content classes → review purpose, location, access,
retention, deletion, backup and legal-hold consequences → reauthenticate and
activate a versioned policy → affected users and applications see the effective
state → authorised custodians inspect metadata, view permitted content, export,
select material for a separately approved improvement programme, or request
deletion.

Retention is not training consent. Moving a retained interaction into Model
Improvement requires a separate programme, purpose, source approval, and
customer release process. Alzette operators access vault content only through
an approved, time-bounded, audited task; ordinary support and telemetry remain
metadata-only.

**Dangerous/destructive confirmation:** enabling or widening retention,
exporting content, changing viewer/selector roles, placing/removing a hold,
bulk deletion, or approving improvement use requires recent authentication and
shows affected scope, data classes, existing records, future behavior, backup
expiry, and irreversible consequences. A policy change is prospective unless
the customer explicitly approves a bounded historical operation.

**States:**

- Unavailable: current PoC, shared evaluation, or contract has no vault; say
  `Prompt and output retention is not enabled for this scope`.
- None: metadata-only; no content object is created.
- Selected: only deliberate, authorised selections are retained.
- Policy matched: server-side policy records eligible interactions; the exact
  policy version remains attached to each retained record.
- Held: expiry/deletion is paused for the recorded legal/contract reason and
  visible only to authorised roles.
- Deletion pending: active storage and backup expiry are separate, dated states.
- Partial: an interrupted response is labelled incomplete and never presented
  as a complete company record.

**Audit events:** `vault_policy.activated`, `vault_policy.changed`,
`interaction.retained`, `interaction.viewed`, `interaction.exported`,
`interaction.selected_for_improvement`, `interaction.deletion_requested`,
`interaction.deleted`, `interaction.hold_changed`, `operator_content_accessed`.

**Acceptance criteria:**

1. Two-tenant tests prove that policies, indexes, objects, exports, encryption
   context, backups, and deletion jobs cannot cross organisation scope.
2. Every retained interaction traces to one logical request, actor, endpoint,
   model release, effective policy version, completeness state, and integrity
   digest without changing customer request accounting.
3. `none`, `selected`, and `policy_matched` modes are enforced at the data
   plane and survive restart; the browser cannot weaken policy or choose a
   storage destination.
4. Authorised customer roles can discover, export, and delete content according
   to policy; every content read and export is audited, including Alzette
   operator access.
5. Retained content never appears in logs, traces, analytics, billing, support
   bundles, cross-customer search, or an improvement dataset merely because it
   was retained.
6. Expiry/deletion tests cover active data, indexes, derived previews,
   improvement selections, backups, legal holds, and immutable safe audit
   evidence, with truthful completion dates.
7. Affected employees and application owners can determine whether their
   current scope retains content and where the approved policy can be reviewed.

---

# F. Requirements

## F1. Target P0 launch MVP functional requirements

This table is the future launch contract, not a statement that every row is
implemented. The dated implementation checkpoint and `POC_BOUNDARY.md` control
current evidence; absent capabilities remain server-gated and hidden/labelled
unavailable.

| ID | Requirement | Testable acceptance criteria |
|---|---|---|
| P0-FR-001 | The portal MUST authenticate a human user and resolve an organisation, role, project, and environment context before showing customer data. | Unauthenticated requests cannot read customer endpoints; changing a resource ID never crosses tenant/project scope; context is visible on every runtime surface. |
| P0-FR-002 | Backend policy MUST enforce tenant/project/environment/key scope. | Two-tenant adversarial tests prove that IDs, aliases, filters, exports, keys, and usage queries cannot cross scope. |
| P0-FR-003 | An operator MUST provision an organisation, default project, service plan, inference target, and tenant-route binding through a supported control operation rather than a manual database edit. | Provisioning is idempotent and audited; the assigned target is never exposed as a customer-controlled URL. |
| P0-FR-004 | Every inference target MUST declare execution class and evidenced capacity mode. | `external_pilot`, `meluxina`, or another approved class is source-labelled; `dedicated` requires evidence and one owner tenant; `shared` requires explicit tenant bindings. Exclusive configuration on a shared external service does not render as dedicated. |
| P0-FR-005 | The gateway MUST expose the tested OpenAI-compatible subset and resolve each call from authenticated tenant/project/model alias to an authorised target. | Request-body or URL manipulation cannot select an unbound target; unknown/disabled bindings fail closed with a stable error. |
| P0-FR-006 | The forwarding pilot MUST support at least one real external compatible endpoint without coupling the customer API to it. | Changing the operator target configuration to another compatible endpoint requires no client URL or credential change; the UI labels the execution class accurately. |
| P0-FR-007 | Dedicated and shared offers MUST remain distinct. | Dedicated requests use only the owning tenant’s target/members; shared requests use only an allow-listed shared target; no silent cross-mode fallback occurs. |
| P0-FR-008 | The gateway MUST create one logical inference-request record per client call and separate attempt records for outbound executions. | A timeout followed by a successful retry appears as one customer request and two operator attempts; request/token totals follow the documented finality policy. |
| P0-FR-009 | The portal MUST support service accounts and scoped API keys, including one-time reveal, expiry policy, rotation, and revoke. | Plaintext is returned once; scope is enforced in the gateway; revoked keys fail within the target; audit contains no secret. |
| P0-FR-010 | The portal MUST expose the tenant-approved model/deployment and a stable customer route. | The exact model/version, dedicated/shared mode, execution class, route status, and source timestamp are visible; clients cannot activate an unapproved catalogue item. |
| P0-FR-011 | The portal MUST support a real first test request through the customer-facing route. | A synthetic request returns a real response or precise error, request ID, end-to-end latency, model/version, and usage metadata; no fixture response is involved. |
| P0-FR-012 | The portal MUST display runtime-backed deployment health and freshness. | Target reachability and inference readiness drive state; stale telemetry is labelled stale/unknown; `operational` is never hard-coded. |
| P0-FR-013 | The portal MUST show company consumption by period and permitted scope. | Summary and breakdowns include logical requests, success/errors, tokens where known, latency, throughput/concurrency, project, route, deployment/model, and service account; zero/partial/unknown states are distinct. |
| P0-FR-014 | Dedicated customers MUST see allocation/utilisation context; shared customers MUST see allowance/limit context. | Neither mode exposes other tenants, raw target addresses, internal retry costs, or global shared-pool capacity. |
| P0-FR-015 | The portal MUST provide safe usage export. | CSV/JSON includes scope, period, timezone, units, source/finality, deployment/model, and no unauthorised content or data; export is audited. |
| P0-FR-016 | Consequential access, credential, project, route, target-binding, limit, and export actions MUST be audited. | Events contain actor/scope/action/result/correlation ID and safe before/after identifiers, excluding secrets and prompt/output content. |
| P0-FR-017 | Product data MUST be labelled live, stale, partial, estimated, final, contractual, operator-entered, or illustrative where applicable. | No unsupported `MeluXina`, `on-premise`, `Luxembourg-hosted`, `dedicated`, `operational`, SLA, or certification claim renders without an authoritative source. |
| P0-FR-018 | The whole P0 system MUST run on one machine under Docker Compose. | Gateway, control/portal, worker if used, PostgreSQL, ingress, and telemetry start from versioned configuration; no Kubernetes, Kafka, Redis, or separate analytics database is required. |
| P0-FR-019 | The first administrator of an approved organisation MUST accept a scoped, expiring invitation and authenticate through the selected human identity layer. | Alzette provisions no normal customer password; Casdoor authentication plus deliberate acceptance creates the exact identity link/user/membership atomically; replay, expiry, revoke, wrong identity, and cross-scope tests fail closed. |
| P0-FR-020 | Authorised customer administrators MUST be able to invite, inspect, resend, and revoke teammate invitations within a server-enforced role ceiling. | Organisation and project authority is derived from the session; project admins cannot grant organisation/peer-admin access; two-tenant tests cover every mutation. |
| P0-FR-021 | Human account recovery MUST remain distinct from inference credentials. | Casdoor owns recovery for new external identities; bounded legacy-account recovery changes only the human authentication method, revokes applicable portal sessions, and never issues or accepts an inference credential. |
| P0-FR-022 | A verified business-email user MUST be able to create one isolated evaluation organisation without payment or operator-issued credentials. | Atomic/replayed signup creates exactly one evaluation tenant and membership; email/domain does not grant access to an existing company. |
| P0-FR-023 | Evaluation provisioning MUST bind only the enabled server-owned shared offer and enforce hard lifetime/rate/concurrency/allowance limits. | Browser fields cannot change model, target, plan, limits, or execution class; exhaustion blocks before provider attempt and duplicate signup creates no extra allowance. |
| P0-FR-024 | The evaluation admin MUST be able to connect an interactive agent or create a separate scoped workload key, make a real first call, and see its truthful usage. | Casdoor tokens and portal sessions never authenticate the data plane; a short human token or application key produces one logical request and decrements the exact evaluation allowance. |
| P0-FR-025 | The portal MUST expose a curated catalogue whose model, version, profile, metric, price, eligibility, quote, deployment, and route states remain distinct. | A listing or price never renders as available/contractual/ready without the corresponding evidence; unsupported IDs fail closed. |
| P0-FR-026 | A customer MUST configure dedicated endpoint intent by model, workload, deployment profile, and capacity units rather than raw infrastructure. | Customer input cannot select target URL, host, image, provider slug, or secret; profiles declare accelerator count, capacity evidence/finality, min/max units, and price availability. |
| P0-FR-027 | Versioned dedicated endpoint quotes MUST snapshot price and capacity before acceptance. | Cross-tenant/expired/superseded quotes fail; acceptance is reauthenticated, immutable, and does not itself claim payment, allocation, deployment, or readiness. |
| P0-FR-028 | An authorised employer MUST be able to invite an exact employee into an exact organisation/project/environment role. | Casdoor authenticates the person, but only atomic Alzette invitation acceptance creates membership; domain/IdP claims, GET, replay, wrong identity, expiry, revoke, and role elevation grant nothing. |
| P0-FR-029 | An interactive employee MUST authenticate inference without receiving a permanent personal API key. | Alzette mints a digest-only, maximum-ten-minute `alz_u_` token bound to one active membership and alias set; Casdoor tokens and portal sessions are rejected by the gateway. |
| P0-FR-030 | A key-only compatible agent MUST be usable through a local Alzette proxy. | The separate Go client binds loopback only, uses a process-lifetime local key, keeps access and `alz_u_` tokens in memory, preserves the supported streaming/tool contract, and never forwards or persists the local key. |
| P0-FR-031 | Human-agent and service-account consumption MUST reconcile without conflating actors. | The immutable ledger enforces exactly one actor tuple, totals match, authorised per-employee metadata is tenant-safe, and no prompt/output or productivity score is stored. |
| P0-FR-032 | Service accounts MUST remain the independent credential for applications and unattended workloads. | Existing `alz_k_` issue/rotate/revoke, machine API, routing, retry, and accounting behavior passes unchanged; offboarding a human does not silently revoke workload keys. |
| P0-FR-033 | Employee agent login MUST survive client restart without making a gateway credential long-lived. | A maximum-one-hour Casdoor access token is memory-only; a rotating refresh session uses the protected keyring/explicit-file/memory policy and 30-day inactivity/90-day absolute defaults; `alz_u_` remains maximum ten minutes and every mint/request rechecks Alzette authorization. |

## F2. P1 first enterprise-pilot requirements

| ID | Requirement | Testable acceptance criteria |
|---|---|---|
| P1-FR-001 | The portal SHOULD support SSO via the identity method required by the first signed pilot, prioritising OIDC/SAML before SCIM. | Pilot IdP can sign in; role mapping and deprovisioning are tested; fallback/recovery is documented. |
| P1-FR-002 | The portal SHOULD support SCIM/JIT or an equivalent directory lifecycle when a pilot requires it. | User/group lifecycle changes reach the portal within the agreed target; effective permission is explainable and audited. |
| P1-FR-003 | The portal SHOULD support organisation-level access reviews and exportable audit evidence. | Admin can filter/export a review period and see actor, action, scope, result, and source. |
| P1-FR-004 | The portal SHOULD support read-only dedicated capacity/shared allowance utilisation, expiry, renewal request, and overage explanation where applicable. | A service-plan end warning identifies affected routes and contract consequence; no renewal is committed automatically. |
| P1-FR-005 | The system SHOULD fulfil accepted dedicated endpoint and scale-up requests through operator approval/allocation/deployment/validation/target binding. | Request status preserves quote/profile/units, actor and commercial effect; only a validated dedicated target owned by the tenant becomes ready. |
| P1-FR-006 | The portal SHOULD support model release notes, deprecation/change notices, customer acknowledgement, and last-known-good version. | A version can be marked deprecated without disappearing from historical usage; affected routes are enumerated. |
| P1-FR-007 | The portal SHOULD support configurable notifications through at least the channels contractually promised. | Delivery, acknowledgement, failure, and escalation are visible; no channel is implied by UI before integration exists. |
| P1-FR-008 | The portal SHOULD support cost attribution by project/environment/route/service account/key where the metering source provides it. | Totals reconcile to org billing; no attribution is fabricated when the source only has account-level data. |
| P1-FR-009 | The portal SHOULD support live incident banner/status integration and customer-safe incident history. | Affected scope and update/resolution times are shown with source; unrelated incidents do not appear as customer impact. |
| P1-FR-010 | The portal SHOULD support data export/deletion request workflow for portal metadata and customer records, subject to contract/legal policy. | Request has scope, owner, status, result, and audit; deletion does not silently erase required billing/audit records. |
| P1-FR-011 | The portal SHOULD support a controlled staging environment if the pilot’s data plane has a safe isolation model. | Staging credentials/routes cannot reach production; promotion records test result and approver. |
| P1-FR-012 | The MeluXina Operations module SHOULD register authorised allocations, deploy a pinned model/runtime, register private target members, and expose health/restart evidence to the control service. | One approved model can be deployed reproducibly and substituted for an external pilot target without changing the customer API; customer secrets and raw infrastructure credentials remain separated. |
| P1-FR-013 | The portal SHOULD support evaluation-to-customer business qualification without changing tenant identity. | Approval records evidence and lifecycle transition; usage/memberships remain scoped to the same ID and no deployment becomes ready automatically. |
| P1-FR-014 | Dedicated endpoints SHOULD support quoted capacity increases as immutable revisions behind the stable route. | New units are not active until resource evidence and health validation pass; model, tenancy mode, execution class, URL, alias, and credential contract remain unchanged. |
| P1-FR-015 | A subscribed dedicated organisation MUST be able to choose and operate a server-enforced private interaction-vault policy for its prompts and outputs. | An authorised customer selects `none`, explicit `selected`, or bounded `policy_matched` retention by project/environment/actor/application/route/content class; the portal shows the effective policy to affected users. Retained interactions are tenant-isolated, encrypted, integrity-protected, linked to the exact logical request/actor/policy version, and accessible/exportable/deletable only by authorised roles. Alzette cannot reuse them independently or across tenants; logs, analytics, support, and evaluation datasets remain separate. Retention expiry, deletion, backup expiry, legal hold, and export are tested and audited. |
| P1-FR-016 | Alzette MUST provide a distinct Alzette-operated Model Improvement branch for approved dedicated customers, introduced through one bounded managed cycle after inference workflow value is proven. | The customer separately approves the objective, source interactions or source policy, rights/purpose, evaluation criteria, and release; vault retention alone never authorises improvement. Alzette prepares a tenant-isolated versioned dataset, runs reproducible evaluation or approved adaptation, safeguards artefacts, and presents baseline-versus-candidate evidence. Only an authorised customer decision and validated deployment can promote the candidate behind the stable endpoint, and the last-known-good release can be restored. |

## F3. P2 scale requirements

| ID | Requirement | Rationale |
|---|---|---|
| P2-FR-001 | The portal MAY support customer-visible weighted/canary traffic and automated rollback gates | Only after safe release telemetry and enough traffic exist |
| P2-FR-002 | The portal MAY support multi-region routing/failover and customer-selectable region policies | Must not conflict with residency or contract boundaries |
| P2-FR-003 | The portal MAY support Prometheus/OpenTelemetry export and trace correlation | Valuable for mature platform teams; not required for first call |
| P2-FR-004 | The portal MAY support batch/async jobs with retention, status, cost, and cancellation | Add only for a validated workload |
| P2-FR-005 | Alzette MAY automate and scale its managed Model Improvement operations after the bounded P1 branch is proven | Preserve Alzette operator ownership, tenant isolation, provenance, customer approval, evaluation, release, retention/deletion, and rollback; do not expose raw training infrastructure as customer self-service |
| P2-FR-006 | The portal MAY support customer-managed networking/private connectivity | Enterprise demand and operator capability must precede it |
| P2-FR-007 | The portal MAY support advanced cost allocation, webhooks, invoice APIs, and procurement integrations | Scale customer operations after core ledger is trustworthy |
| P2-FR-008 | The portal MAY support multi-organisation partner/integrator administration | Avoid becoming a reseller platform prematurely |

## F4. Non-functional requirements

### Security and privacy

- The portal and data plane MUST enforce tenant isolation server-side and test cross-tenant reads, writes, usage queries, and key use.
- Authentication MUST use the selected, pinned Casdoor identity mechanism for new external users; MFA policy and any customer federation are configured there without transferring Alzette membership authority.
- Sessions MUST use secure, http-only, same-site protections, bounded lifetime, reauthentication for high-risk actions, and revocation on deprovisioning.
- Secrets MUST be encrypted at rest, transmitted only over TLS, hashed or stored in a dedicated secret system, redacted from logs/analytics/support, and returned once.
- Least privilege MUST apply to customer and Alzette operator access. Support access MUST be time-bounded or explicitly approved and audited.
- The current PoC and any scope without an active vault policy MUST remain
  metadata-only. The target dedicated service MUST store prompt/output content
  only under the organisation's versioned vault policy, with purpose/rights,
  tenant isolation, encryption, least-privilege access, visible user notice,
  export, retention, deletion, backup/hold rules, and complete access audit.
- Vault content MUST remain technically and operationally separate from logs,
  traces, analytics, support attachments, billing, and improvement datasets.
  Retention never implies permission to train, evaluate, or cross-use.
- Audit records MUST be append-only to customers and protected against modification; access to audit data is itself auditable.
- The product MUST not publish a certification, compliance, residency, retention, or SLA claim unless a current source and owner are recorded.
- The target registry and MeluXina Operations module MUST treat raw target URLs, MeluXina Project IDs, Slurm accounts, VM identifiers, SSH keys, API tokens, and allocation records as operator-side data. None may be customer-visible unless an explicit contract and redaction policy approve it.
- External-pilot, gateway, and MeluXina data paths MUST be documented separately. A LuxProvide statement about the supercomputer does not prove the location, retention, or controls of an external pilot route, Alzette gateway, logs, queues, backups, support system, or customer client path.

### Availability and failure isolation

- The portal MUST distinguish portal availability, control-plane availability, and data-plane availability.
- A portal outage MUST not be described as inference outage and an inference outage MUST not be hidden by a cached green portal state.
- Read-only cached views MAY be available during telemetry outages but MUST display age and source.
- The target uptime/SLO and support response are founder/operator decisions; until signed, the portal MUST say `not contracted` rather than invent a percentage.
- Writes MUST be idempotent or safely retryable; no duplicate keys, projects, routes, charges, or support cases from client retries.
- Inference retry MUST distinguish the logical client request from runtime attempts. Once response streaming has begun, the gateway MUST NOT transparently replay the request to a different target unless safe continuation is explicitly supported and tested.
- If a MeluXina deployment is batch/scheduled or allocation-limited, the portal MUST expose queue/pending/blocked states and MUST NOT claim endpoint availability while a job, VM, ingress, or model is not ready.

### Performance

Proposed product targets for pilot review, not measured claims:

- shell and navigation usable within 2 seconds on a normal broadband connection after authentication;
- normal metadata/read requests p95 under 2 seconds when the backend is healthy;
- dashboard first meaningful live state under 4 seconds, with progressive loading for slower telemetry;
- key reveal action returns only after secret issuance is committed and never re-requests plaintext;
- large usage exports are asynchronous and show progress rather than blocking the browser.

These targets MUST be measured in a pilot-like environment and revised if the contract/data plane requires different limits.

### Accessibility and responsive behaviour

- The portal MUST target WCAG 2.2 AA: keyboard operation, visible focus, semantic headings, labelled controls, error association, colour-independent status, reduced-motion respect, and accessible tables/charts.
- Charts MUST have tabular or textual equivalents. “Green”/“amber”/“red” MUST never be the only status signal.
- The portal MUST work at narrow mobile widths for status, credentials, support, and urgent actions; dense usage/contract tables MAY require horizontal scrolling with accessible headers.
- Copy/paste code and key actions MUST be usable by keyboard and screen readers.

### Observability and supportability

- Every portal request and backend operation MUST have a correlation ID visible in support-safe errors.
- Control-plane events MUST include latency, outcome, dependency, retry, and permission-denial metrics.
- Data freshness, ledger finality, and source status MUST be stored and displayed.
- Alerts MUST cover authentication failures, tenant-scope denial, key issuance/revoke failures, route state divergence, metering delay, export failure, and notification failure.
- Operators MUST have a runbook for stuck provisioning, limit enforcement, stale telemetry, key compromise, tenant isolation/route-misbinding incidents, and rollback.

### Privacy, retention, and deletion

- Retention for portal metadata, audit, usage, support, and billing records where present MUST be configured by policy/contract and displayed to authorised users.
- Customer content is not a portal field at MVP. Diagnostics use IDs/status/timing/model/version/usage metadata and optional contract-relevant charge fields.
- Required billing/audit records MUST not be deleted merely because a customer requests content deletion; the portal must show the applicable legal/contractual reason and scope.
- Data export MUST identify whether records are complete, provisional, or final.

### Internationalisation, timezones, and currency

- Every timestamp MUST include an explicit timezone or ISO-8601 offset; the organisation’s configured timezone is the display default and UTC is available.
- Currency MUST come from the contract/billing source. EUR may be the initial default only if the operator confirms it; no currency symbol is inferred from locale.
- Numeric formatting, pluralisation, date ranges, and decimal precision MUST be locale-aware.
- Translation is not required for P0 unless the first pilot requires it, but labels MUST avoid encoding business meaning only in English abbreviations.

### Data export

- Usage exports MUST include organisation/project/environment/route/deployment/model/version, service mode, period, timestamp/timezone, logical request/error counts, token/compute units where known, allocation/allowance context, source/finality, and export generation time. Charge and currency fields are included only when the contract defines them.
- Audit exports MUST include actor, actor type, role/source, action, scope, result, correlation ID, and safe metadata.
- Export permissions MUST be scope-aware and export events MUST be audited.

## F5. Backend capability status after Slice 2

The earlier prototype lacked the P0 vertical slice. The repository now implements the following Slice 0–2 capabilities in one Go codebase and one PostgreSQL-backed Compose stack:

1. Human session/authentication plus tenant/project membership and a small role policy.
2. PostgreSQL-backed organisation, project, environment, model, service-plan, inference-target, target-member, and tenant-route registry.
3. Service-account/API-key issuance, hashing, scope, rotation, revoke, and one-time reveal.
4. OpenAI-compatible gateway with request IDs, non-streaming chat first, streaming only when tested, machine-readable errors, and a configurable target adapter.
5. Server-side route resolution and policy that enforces dedicated ownership or explicit shared-pool membership and forbids arbitrary upstream URLs.
6. Authoritative `inference_requests` and `provider_attempts` ledger plus hourly usage rollups in PostgreSQL.
7. Health probing and request-derived deployment telemetry with freshness.
8. Customer APIs for deployment/route status, company usage, safe request metadata, and CSV/JSON export.
9. Append-only administrative audit events and metadata-retention/redaction policy.
10. Versioned Docker Compose deployment containing gateway, control/portal, worker, migrations, and PostgreSQL. TLS ingress and production telemetry remain Slice 3 work.
11. Contract tests using deterministic fake targets. The separately authorised real external compatible-endpoint smoke remains pending a fresh file-backed credential.

Items 1–9 are implemented through Slice 2. Item 10 is complete for the documented single-machine LAN PoC boundary, not for a remote or production deployment. Item 11 is complete for deterministic software evidence and incomplete for live-provider evidence. The portal is therefore an implemented offline-validated product PoC, not merely a design fixture and not yet a production-ready or live-provider pilot.

Human authentication in item 1 currently means operator-created username,
bcrypt password, membership, and server-side session only. Invitation
acceptance, verified email, password recovery, member management, throttling,
transactional mail, public signup/evaluation provisioning, catalogue APIs,
quotes, and deployment-request fulfilment are explicitly unimplemented;
their contract is [`ACCOUNT_ONBOARDING_PRD.md`](ACCOUNT_ONBOARDING_PRD.md).
Migration `0008_self_service_catalogue` provides additive schema groundwork
only; it does not change that runtime statement.

P1 introduces the separate **MeluXina Operations module**:

1. Authorised project/allocation and terms record.
2. Pinned model, licence, weight digest, runtime image, and deployment manifest.
3. Allocation/job/VM state and capacity inventory.
4. Private model-server address/port registration and health/restart controls.
5. Controlled rollout/rollback and last-known-good evidence.
6. Infrastructure metrics, incident/runbook evidence, and internal COGS records.
7. A migration test proving that an external pilot target can be replaced by a MeluXina LAN target without changing the customer API or crossing tenant boundaries.

Until the Slice 3 operating controls and the live-provider gate pass, the portal MUST continue to label the deployment as an HTTP/LAN PoC with unknown live readiness where appropriate. Until the MeluXina path passes its infrastructure gate, no route may be labelled MeluXina-hosted.

---

# G. UX and content requirements

## G1. Dashboard hierarchy

The first screen should answer, in order:

1. **Can I safely call my route now?** Current endpoint/data-plane state, affected incident, stale marker, and route/environment.
2. **What needs action?** Failed provisioning, unhealthy release, budget threshold, expired key, missing contract field, or support case.
3. **What are we consuming?** Current requests/tokens, errors, latency, allocation/utilisation for dedicated service or allowance/limits for shared service, with source and freshness.
4. **What changed?** Recent release, key, policy, contract, or incident event.
5. **Where do I go next?** First-call checklist, route test, docs, support, or evidence export.

Charts are secondary. A chart without a decision, source, unit, period, and freshness is not useful P0 content. The dashboard MUST NOT lead with total requests or a visually impressive node map when a route is failing or a cap is near enforcement.

## G2. Status vocabulary

The following vocabulary is proposed and must be used consistently:

| Status | Meaning |
|---|---|
| `pending` | Configuration accepted; no runtime operation confirmed yet |
| `provisioning` | Backend is allocating/building/attaching capacity |
| `awaiting_capacity` | Request is valid but capacity is not yet confirmed |
| `testing` | Route exists and is undergoing a test/readiness check |
| `ready` | Route passed readiness but is not serving production traffic |
| `active` | Runtime health source confirms it serves the selected environment |
| `degraded` | It serves but a monitored condition is outside target or an incident affects it |
| `stale` | Last telemetry is older than the configured freshness target; health is unknown |
| `budget_blocked` | Enforced budget/cap prevents new requests |
| `rate_limited` | Gateway or capacity limit rejects/throttles requests |
| `paused` | Operator/customer action stopped or suspended serving; consequence is stated |
| `failed` | Last operation or runtime health failed; cause/action is shown when safe |
| `retiring` | Release/model/commitment is scheduled for removal or expiry |
| `expired` | Key, commitment, contract, or policy is past its effective end |
| `unknown` | Source unavailable or insufficient evidence; never imply healthy |

Avoid using `operational`, `production-ready`, `private`, `Luxembourg-hosted`, `certified`, or `SLA-backed` as generic visual status. Those are claims requiring source/contract context.

## G3. Credential reveal and copy behaviour

- Show a warning before issuance: “This secret will be shown once. Store it in your secret manager.”
- Display the full key only after explicit reveal and only in a one-time response.
- Provide copy and download-to-safe-file guidance, not an automatic clipboard write without user action.
- Show a non-secret prefix and key ID everywhere else.
- Never put the secret in query strings, referrers, telemetry, screenshots, support forms, or browser local storage.
- Rotation creates the replacement first, displays the overlap window, then permits revoke.
- Revoke uses a typed or explicit confirmation for production keys and immediately displays the resulting state.

## G4. Required edge-state copy

| Condition | Required meaning |
|---|---|
| Permission denied | “You can view this area, but your role cannot perform this action. Ask [role/owner] for access.” |
| Stale telemetry | “Last confirmed at [time]. Current health is unknown until telemetry refreshes.” |
| Partial telemetry | “Some sources are delayed; totals may be incomplete. See source/freshness.” |
| Zero usage | “No requests were recorded in [period]. This is not an error.” |
| Over budget | “Notification threshold reached; requests continue under the current policy.” |
| Hard cap reached | “Requests are blocked by the [scope] cap. No alternate model or capacity was selected.” |
| Evaluation account | “Evaluation · Shared — this organisation uses the bounded shared evaluation offer and is not an approved dedicated deployment.” |
| Evaluation exhausted | “The evaluation allowance is exhausted. No provider attempt was made and no paid or alternate capacity was activated.” |
| Catalogue only | “Available to configure — no capacity, price commitment, deployment, or route has been assigned.” |
| Indicative profile price | “Indicative unit price — your accepted versioned quote, if issued, controls.” |
| Quote accepted | “Quote accepted — allocation and deployment have not started unless shown separately.” |
| Capacity increase pending | “The endpoint continues on [current units]. Requested units are not active until deployment and health validation complete.” |
| Degraded service | “The route may respond slowly or fail. Incident [ID] affects [scope].” |
| Expired commitment | “This commitment ended on [date]. Affected capacity/overage terms require operator confirmation.” |
| No model available | “No model is approved and available for this project, mode, and region. Request review.” |
| External pilot route | “This pilot currently executes through an approved external inference service. It is not running on MeluXina.” |
| MeluXina migration not passed | “The MeluXina deployment path is not yet verified for customer use. This route remains on its labelled pilot target.” |
| Target allocation/queue | “Capacity is pending or unavailable. This route is not ready; see deployment state and the next operator action.” |
| Usage detail partial | “The request was recorded, but some token or runtime detail is not final. Unknown values are not counted as zero.” |
| Vault unavailable | “Prompt and output retention is not enabled for this scope. Requests remain metadata-only.” |
| Vault retention active | “This scope retains prompt and output content under company policy [version]. Review access, purpose, retention, and deletion terms.” |
| Retained, not approved for improvement | “This interaction is in your company vault. It is not part of a Model Improvement dataset.” |
| Deletion pending | “Active content deletion is pending/completed; encrypted backup expiry is tracked separately for [date].” |
| Contract unknown | “The portal cannot verify this term. Contact Alzette before relying on it.” |
| Fixture/demo | “Illustrative preview — not live account data.” |

## G5. Contractual versus live operational evidence

Every important value needs one of these labels:

- **Live:** returned by a current health/runtime/metering source, with `as of` time.
- **Stale:** last successful source response exceeds the freshness target.
- **Estimated:** calculated from provisional metering or forecast.
- **Final/invoiced:** reconciled billing record with invoice/statement source.
- **Contractual:** copied from a versioned agreement or policy document.
- **Operator-entered:** manually maintained by an authorised Alzette operator, with owner and update time.
- **Unknown:** not available or not verified.
- **Illustrative:** fixture/demo only; never mixed with live data.

The content system MUST prevent a `contractual` field from silently falling back to a marketing string or fixture. If a live status is unavailable, `unknown` is safer than green.

For infrastructure data, the product SHOULD distinguish at least `public capability`, `Alzette allocation/usage`, `Alzette operational observation`, and `Alzette contractual commitment`. A LuxProvide page can support the first label; only an authorised record/test/contract can support the latter three. MeluXina’s public statement that infrastructure is in Luxembourg MUST NOT automatically label an external pilot, Alzette gateway, logs, backups, or customer route as Luxembourg-hosted.

---

# H. Scope control

## H1. Target P0/P1/P2 feature table

| Area | P0 launch MVP | P1 first enterprise pilots | P2 scale |
|---|---|---|---|
| Identity/access | Verified self-service business-email signup, Casdoor-backed human auth, evaluation/customer tenant lifecycle, invitation/recovery, short human-agent access, loopback compatibility proxy, small role set, service accounts and scoped workload keys | Pilot-required customer SAML/OIDC federation or stronger MFA, SCIM/JIT, directory groups, access reviews, additional native agents | Delegated partner admin, fine-grained policy engine |
| Onboarding | One isolated hard-capped shared evaluation tenant, first-call checklist, first-admin/team invitations, qualification and dedicated configuration | Operator-reviewed customer conversion and accepted-request fulfilment | Automated qualification only with evidence; partner onboarding |
| Models | Curated catalogue/model versions, licence/capability lifecycle, deployment profiles with eligible modes | Release notes, acknowledgements, deprecation migration, more evidenced profiles; distinct Alzette-operated Model Improvement branch proven through one governed cycle | Scaled managed evaluation/adaptation and customer model onboarding operations |
| Capacity | Explicit shared evaluation allowance; dedicated profile units, accelerator count, evidenced metrics, versioned quote/request; no raw machine selection | MeluXina allocation/deployment/private target registration and stable-endpoint capacity revisions | Automated fulfilment, reservations, private networking, advanced hardware constraints |
| Commercial | Indicative profile prices and immutable customer quote snapshots; acceptance distinct from charge/allocation/readiness | Contract/payment/invoice linkage and capacity-change commercial workflow | Procurement APIs, discounts, automated renewals under policy |
| Runtime | Tenant-route registry, forwarding, readiness, test call, stable alias, last-known-good target binding | Promotion/rollback workflow against MeluXina model deployments | Canary/weighted routing, automated rollback, A/B |
| Observability | Health/freshness, request metadata, status/error classes, basic latency/throughput, incident link | Alerts, incident history, external status integration, better attribution | Traces, Prometheus/OTel export, SLO analytics |
| Usage | Logical requests, tokens, errors, latency, concurrency, deployment/model/project attribution, dedicated utilisation or shared allowance, CSV/JSON export | Notifications, richer service-plan/commitment records, invoice detail where required | Forecasting, chargeback, webhooks, procurement integration |
| Private data custody | Metadata-only; no prompt/output persistence in the first release | Dedicated customer interaction vault with `none`, `selected`, and bounded policy-matched retention, access/export/deletion controls, and separation from improvement authority | Advanced policy automation, customer-managed keys/storage where required |
| Trust/support | Agreement/evidence links, retention/region labels, audit events, metadata-only support | SSO/audit exports, deletion workflow, customer support history | Private connectivity, customer-managed keys/storage, advanced compliance evidence |
| Inference modes | Synchronous compatible text chat plus the tested Pi-oriented function-tool SSE subset after retry/metering semantics passed | Embeddings, multimodal input, structured output, additional tool/provider extensions, or async/batch only for a validated pilot; one governed evaluation/adaptation cycle only when contracted | Automated fine-tuning/evaluation pipelines and custom apps |

## H2. Non-goals

The following are explicitly outside the first portal release:

- consumer chat, prompt library, employee analytics, or end-user conversation history;
- arbitrary model upload, default capture of inference content, ungoverned model
  training/fine-tuning, a general-purpose dataset/evaluation laboratory, or
  public model publishing;
- a generic cloud infrastructure console with every GPU, replica, autoscaling, networking, and storage knob;
- public marketplace billing or multi-vendor routing;
- anonymous activation of a tenant, membership, route, target, plan, model
  deployment, API key, or inference credit;
- automated legal/compliance advice or a certification portal;
- storing or browsing prompts/responses as the default debugging path;
- claiming production readiness based on the current fixture dashboard;
- changing implementation language/framework or visual style through this PRD.

## H3. Dependencies

- One approved external OpenAI-compatible endpoint/model for the forwarding pilot, with credentials, usage semantics, retention/location disclosure, and a deterministic fake for tests.
- A versioned gateway API subset and target-adapter contract.
- Tenant/project/target/route/key/usage persistence and gateway policy enforcement.
- Reviewed Internet identity/mail/throttle controls and one enabled shared
  evaluation target whose hard request/token/rate/concurrency limits are
  enforced by the gateway, not only displayed.
- At least one curated model release and deployment profile with licence owner,
  runtime/hardware validation, capacity evidence/finality, price owner, and
  versioned acceptable-use terms.
- An operator-owned service plan and target binding for every pilot organisation.
- A first pilot organisation, workflow, success metric, test-data policy, sponsor, and support owner.
- Contract/service facts needed for the chosen pilot: dedicated/shared mode, model, limits/allocation, location disclosure, retention, support, and any usage charge.
- Docker Compose host, PostgreSQL backup/restore procedure, TLS/ingress, secret handling, telemetry, and runbook.
- Operator capacity and on-call ownership.
- Security pack: architecture/data flow, retention/deletion, access, subprocessors, incident, DPA, model licences, and current certification status.
- Product/legal decisions on region, contract, SLA, and pricing where applicable.

A governed model-improvement pilot additionally depends on:

- a dedicated inference workflow with a named owner, baseline, measured value,
  and an explicit customer decision to enter an improvement phase;
- recorded authority for every permitted source-data class, including client
  confidentiality, data-processing purpose, model licence, retention,
  deletion, access, and subprocessor constraints;
- tenant-isolated dataset and artefact custody, a versioned evaluation set and
  acceptance threshold, authorised release approval, reproducibility, and a
  tested rollback path. Only content selected from the governed vault or
  supplied through another equally controlled source can satisfy these
  dependencies; metadata-only telemetry cannot.

The private interaction vault additionally depends on:

- a contractually defined customer/Alzette role boundary and reviewed wording
  for customer rights, Alzette non-appropriation/non-cross-use, client and
  employee data, legal holds, subprocessors, and termination/export;
- tenant-isolated encrypted object/index storage, key ownership, region,
  backup/restore, deletion/expiry, integrity, access-audit, and breach response;
- a server-enforced versioned policy and role model, visible employee/
  application notice, content-safe export, operator just-in-time access, and
  tests proving that logs, analytics, billing, support, and Model Improvement
  remain separate purposes.

The MeluXina migration additionally depends on:

- an authorised LuxProvide/MeluXina access path and infrastructure record: project owner, Project ID, allocation/term, eligible resource, accepted terms/service commitment, and operator;
- a reproducible long-lived model-serving path and stable private network address/port reachable by the Alzette gateway;
- current commercial and trust facts for customer-serving use, including data paths, retention, support, maintenance, termination, and any relevant rates/credits;
- a passing MeluXina infrastructure PoC and migration test. No provider logo or public page substitutes for this evidence.

## H4. Risks and mitigations

| Risk | Consequence | Mitigation / go-no-go signal |
|---|---|---|
| Portal is built against fixtures while data plane is absent | False readiness and unsafe customer claims | Backend contract first; no `live` labels until end-to-end tests pass |
| Tenant isolation is only a UI filter | Cross-customer disclosure | Backend authz tests, two-tenant adversarial test, independent review |
| A route resolves to the wrong customer target | Confidentiality breach and dedicated-service violation | Server-controlled tenant bindings, ownership constraints, fail-closed resolution, adversarial routing tests |
| External pilot execution is mistaken for MeluXina/on-premise | False residency and procurement claim | Explicit execution class in target registry, UI, exports, and evidence; copy tests forbid unsupported hosting claims |
| Retries inflate customer consumption | Misleading dashboard or billing dispute | Separate logical requests and provider attempts; reconciliation and retry tests |
| Usage totals do not reconcile | Operational distrust and capacity mistakes | Source/finality labels, ledger-to-rollup reconciliation, safe export before pilot |
| Model/version changes break customer applications | Production incident | Immutable releases, change notice, compatibility test, rollback |
| Dedicated capacity is accidentally shared or oversold | Isolation/SLA failure | Exclusive target ownership invariant, operator allocation evidence, quote/request/deployment separation, and no readiness before validated binding |
| Free signup is farmed for inference capacity | Unbounded provider cost and poor service | Verified mailbox, durable identity/source throttles, one evaluation tenant per identity, hard lifetime/rate/concurrency caps, cost alerts, and operator kill switch |
| Catalogue benchmark is multiplied as if GPU scaling were linear | False capacity quote and underprovisioned endpoint | Per-metric finality/evidence and explicit scaling semantics; validate each supported unit count rather than extrapolating by default |
| Profile price changes after customer selection | Commercial dispute | Versioned effective prices and immutable customer quote snapshots with expiry, currency, term, units, and evidence |
| Quote acceptance is rendered as running capacity | False operational status | Separate accepted/approved/allocating/deploying/validating/ready states; only route/runtime evidence can render ready |
| MeluXina’s scheduled HPC model cannot provide the required endpoint semantics | Queue delays, cold starts, downtime, or an impossible customer promise | PoC must test a stable external/private path, warm/cold behavior, restart/recovery, and 24-hour probe; classify batch/tunnel-only as PoC-only |
| MeluXina allocation is available on paper but GPUs are queued/unavailable | Migration cannot meet its workload or capacity target | Record actual allocation, partition, queue, repeated start attempts, capacity visibility, and fail-closed no-capacity state |
| MeluXina Project/Slurm isolation is mistaken for customer tenant isolation | Cross-customer disclosure or shared credentials | Enforce tenant policy at Alzette gateway; test two synthetic scopes and wrong-key/project denial |
| MeluXina subsidy or credit is one-off, capped, or ineligible | Apparent margin disappears after the PoC | Model with and without subsidy; obtain current written terms; never present programme copy as recurring price |
| Applicable MeluXina terms or commercial support are unsuitable for customer-serving workloads | Data, liability, support, or procurement failure | Legal/operator review of current terms, service commitment, DPA, commercial support, termination/export, and SLA before migration |
| Gateway/ingress leaves the contracted region or has different logging/retention | Residency and confidentiality claim is false | Inventory every hop and processor; prove location/retention; show exact scope in portal evidence centre |
| Infrastructure updates, maintenance, or job eviction break a long-running model | Unplanned inference outage | Pin image/model/runtime, record maintenance/status, test restart/recovery, and define customer-safe degraded/blocked states |
| Luxembourg/EU hosting claim is broader than actual data path | Procurement/compliance failure | Data-flow and subprocessor evidence; display exact contractual scope |
| Prompt/output appears in logs/support | Privacy breach | Metadata-only diagnostics, redaction tests, retention policy |
| Vault content crosses a tenant, role, region, retention boundary, or leaks into logs/support | Confidentiality breach and loss of the product's core trust promise | Separate encrypted tenant custody/indexes, policy-derived authorization, JIT operator access, per-read/export audit, isolation tests, key/backup/deletion evidence, and no secondary system ingestion |
| Alzette claims unconditional customer ownership where employee, client, third-party, or legal rights differ | Contractual misrepresentation | Promise customer control and Alzette non-appropriation/non-cross-use; record applicable rights, purpose, holds, and exceptions in the customer agreement |
| Customer interactions are reused for improvement without valid authority or provenance | Confidentiality breach, unlawful processing, and loss of customer trust | Vault retention alone grants no improvement authority; use a separate programme and versioned dataset with recorded purpose/rights/source, approval, deletion, and audit before any training or evaluation |
| A fine-tuned release improves style but degrades safety or workflow quality | Reputational harm and production regression | Versioned baseline/evaluation set, explicit acceptance thresholds and customer approver, staged promotion, immutable evidence, and last-known-good rollback |
| Budget cap is decorative | Unbounded spend | Enforce at gateway; integration test rejects requests at cap |
| Customer federation/SCIM consumes the whole MVP | Pilot delay | Keep the pinned Casdoor base identity layer; implement only the customer-federation or directory feature the first signed pilot requires |
| Dashboard becomes vanity telemetry | Low operational value | Every panel must answer a decision and show source/freshness |
| Broad catalogue increases licence/support burden | Operational and legal exposure | Curated approval registry with owner and deprecation process |

## H5. Open questions

1. What is the minimum first-pilot workflow: document classification, extraction, summarisation, coding, search/embeddings, or another job?
2. Which external endpoint/model powers the first forwarding pilot, and which exact OpenAI-compatible features are P0?
3. What is the retry policy before streaming starts, and which upstream usage fields are authoritative or may remain partial?
4. Is `project` a customer concept already enforced by the service or only a portal abstraction?
5. Which operator provisions the first target binding, and what safe workflow replaces direct database edits?
6. Which limits apply to dedicated capacity versus shared allowance, and what happens when each is reached?
7. Does a reserved plan guarantee capacity, price, region, availability, or only a commercial commitment? Each must be separate in the data model.
8. What is the support route and response target for a pilot? What is the emergency route?
9. Which SSO/SCIM/retention/region requirements are written into the first customer’s procurement checklist?
10. Is the first offer a fixed dedicated commitment, usage-based shared plan, or hybrid, and which commercial values actually need to appear in P0?
11. What is the retention period for request metadata, logs, audit, billing, and support? What legal records cannot be deleted?
12. Which model licences permit the proposed commercial service and customer data handling?
13. What is the smallest contract between the control service and the future MeluXina Operations module?
14. Which analytics are permitted, and how will the product prove that vault
    content never enters analytics, logs, support, or billing?
15. Will Alzette gateway/control processes run on the same MeluXina host/network segment as model servers, and what service discovery is available for private target addresses?
16. Which model/profile is the default shared evaluation offer, what is its
    maximum lifetime cost per verified organisation, and which hard gateway
    limits bound abuse?
17. What exactly is one dedicated capacity unit for the first sellable profile:
    GPU class/count, memory, validated context/concurrency/throughput range,
    billing period, price, and upgrade lead time?
18. Which capacity metrics can scale with added units, and must scaling add
    replicas, tensor parallelism, or a replacement deployment?
19. Does the first validated workflow create enough repeatable value to justify
    a separate model-improvement phase, or should prompt/process changes remain
    the intervention?
20. Which exact employee/client interactions may enter an improvement dataset,
    who can approve them, what legal/contract authority applies, and how are
    withdrawal, retention, deletion, and subject/client restrictions enforced?
21. What private evaluation set, baseline, acceptance thresholds, approver,
    model licence, artefact custody, promotion, and rollback evidence are
    required before a derived release may serve production traffic?
22. Which dedicated organisations and scopes may use `selected` or
    `policy_matched` retention, who can activate or widen it, and what notice
    must employees, clients, and application owners receive?
23. What customer-rights and Alzette non-appropriation/non-cross-use language
    can be contracted without overstating ownership of employee, client, or
    third-party material?
24. What storage region, encryption/key model, operator-access procedure,
    export format, deletion SLA, backup expiry, legal-hold process, and
    termination handover make the private vault credible?

### Questions to ask LuxProvide later (not contacted for this PRD)

These questions are deliberately exact because public pages do not answer them. They should be asked only after Alzette has an approved reason to open a provider discussion; they are not a substitute for the PoC.

**Access and eligibility**

- Which route should Alzette use for a commercial inference PoC: Luxembourg national share, the startup track, INITIATE, Cashback80, EuroHPC AI Factory Industrial Innovation, or another programme?
- Is Alzette eligible as a Luxembourg company/startup/SME under each route? What proof is required (legal entity/VAT, PIC, project owner, employment/PI, civilian purpose, AI Act compliance, or other documents)?
- What is the application/approval/onboarding lead time, minimum project term, allocation size, renewal process, and hardware-availability rule? Can access be extended without reapplying?
- For EuroHPC or AI Factory access, may models and results be used in a commercial customer-serving pilot? What publication, reference, final-report, data/model ownership, or non-double-awarding obligations apply?

**Economics and procurement**

- What current rate card applies to CPU/GPU node-hours, cloud VMs, storage tiers, egress, private connectivity, support, and any managed service? Are taxes and minimum commitments separate?
- What are the current INITIATE entitlements (node-hours, storage, support, duration, expiry, exclusions), and which document controls where the live page and older PDFs differ?
- What exactly does Cashback80 cover: eligible cost base, state-aid application, approval timing, reimbursement timing, cap, term, clawback, and repeatability? Is it available to a startup as well as an SME?
- How are usage, allocation, credits, idle time, queued jobs, VM runtime, storage, and overage measured and invoiced? Can LuxProvide provide machine-readable records suitable for reconciliation?
- Are commercial customer-serving inference workloads allowed under the applicable national/commercial terms, programme rules, and Terms of Use, or is a separate service contract required?

**Serving, network, and operations**

- May Alzette run a long-lived inference process on a GPU compute allocation, or must it run in an OpenStack/cloud VM or provider-managed service? What are the maximum job/VM lifetimes, requeue/eviction rules, and idle policies?
- Can a serving process be reached from an Alzette gateway over a supported private or public path? Is VPN-only access required? What are the supported ingress, static-address/load-balancer, TLS, firewall, allow-list, egress, DDoS, and bandwidth options?
- Who owns authentication, API gateway, tenant isolation, request logging, metering, autoscaling, restart, patching, model rollout, backup, and incident response at each layer?
- Which GPU models, memory sizes, drivers, CUDA/runtime versions, container formats, and inference servers are currently supported for the selected model? Are A100-40 nodes and any cloud GPU entitlement actually available to this project?
- What are the observed/contractual queue time, cold start, warm latency, throughput, capacity reservation, maintenance window, recovery, and availability commitments for a customer-facing route?
- What is the current status and supported contract scope of MeluXina Cloud gateways, Kubernetes, AIaaS, and MeluXina-AI? Which of those can Alzette use now rather than after a future launch?

**Security, data, and support**

- For every hop (customer, Alzette gateway, network, VM/job, model artefact, logs, metrics, backups, support tools), where is data processed and stored, which subprocessors can access it, and what is the retention/deletion schedule?
- What current DPA/processor terms, confidentiality terms, subprocessor list, audit rights, encryption/key-management options, access controls, breach-notification timing, and customer-data restrictions apply?
- What is the current scope and evidence for ISO/IEC 27001, Tier IV, isolation, anonymisation, and EU/Luxembourg processing? Does the public 99.995% availability statement form part of an Alzette service commitment, and what remedies apply?
- What commercial support tier, response targets, escalation contacts, maintenance notice, incident feed, and post-incident report process are available? The public policy page’s support section is explicitly for non-commercial projects, so commercial scope must be confirmed.
- On termination or allocation expiry, how long are data, model artefacts, logs, backups, and billing records recoverable, and how is destruction evidenced? The public ToU’s default 30-day recovery language must be confirmed for the applicable service contract.

## H6. Decision log

| ID | Decision | Status / owner |
|---|---|---|
| D-001 | Portal is B2B organisation control plane, not consumer product | Accepted from founder direction; founder/operator |
| D-002 | Control plane and data plane are separate; portal state is never proof of runtime state | Accepted in this PRD; engineering/operator |
| D-003 | P0 uses a curated approved catalogue; arbitrary upload, automatic content capture, and fine-tuning remain deferred | Proposed; founder/operator |
| D-004 | Contract/operations sources are authoritative for region, retention, model availability, cost, caps, and SLA | Proposed; founder/legal/operator |
| D-005 | The current PoC remains metadata-only, but the target subscribed dedicated service includes a customer-controlled private interaction vault. The company chooses the versioned retention/access/purpose/export/deletion policy; Alzette does not appropriate or cross-use content, and vault retention does not authorise Model Improvement. | Accepted user/product direction, 2026-08-16; product/customer/legal/security/operator |
| D-006 | Runtime P0 remains one approved target/project/stable route/verified first call/usage; acquisition P0 adds one hard-capped shared evaluation path | Accepted founder direction; product/engineering |
| D-007 | Dedicated configuration and quote acceptance are customer self-service; physical allocation/activation remains operator-fulfilled until safely automated | Accepted founder direction; founder/finance/operator |
| D-008 | Existing dashboard fixture values cannot be used as customer evidence | Accepted from repository audit; all team members |
| D-009 | First target is a small/mid-sized regulated organisation with one workflow | Hypothesis to validate; growth/founder |
| D-010 | Customer federation/SCIM priority depends on the first signed pilot; this does not defer the selected Casdoor base identity layer | Proposed; founder/customer |
| D-011 | Profiles may expose evidenced indicative unit pricing; the customer-specific versioned quote is authoritative for acceptance | Accepted direction; exact prices unresolved; founder/finance |
| D-012 | Exact hosting/residency/SLA/certification claims are unresolved | Founder/legal/operator |
| D-013 | Alzette intends to operate its software and customer model servers on MeluXina; MeluXina is infrastructure, not the customer API | Accepted founder direction; founder/operator |
| D-014 | The software PoC proceeds now against approved external compatible targets; MeluXina qualification blocks only MeluXina migration and hosting claims | Accepted founder direction; founder/engineering |
| D-015 | The Alzette gateway owns customer authz, tenant routing, limits, request IDs, and customer metering | Accepted founder direction; founder/security/engineering |
| D-016 | MeluXina subsidy/credits are not baseline economics until current written terms and actual records exist | Proposed; founder/finance |
| D-017 | Existing MeluXina HPC/AI tutorials prove an experiment path, not a production endpoint or SLA | Accepted research interpretation; product/operator |
| D-018 | Dedicated customer deployments are the primary paid offer; shared targets are explicit evaluation or contracted alternatives | Accepted founder direction; founder/product |
| D-019 | Every request resolves through a tenant-route binding to a dedicated or shared inference target; customers never supply target URLs | Accepted founder direction; engineering/security |
| D-020 | Logical customer requests and provider/runtime attempts are distinct ledger entities | Accepted founder direction; engineering/product |
| D-021 | MeluXina model deployment and infrastructure lifecycle form a separate operator module | Accepted founder direction; product/engineering/operator |
| D-022 | P0 runs on one machine with Docker Compose and PostgreSQL; module boundaries do not imply microservices | Accepted founder direction; engineering |
| D-023 | Customer onboarding is hybrid self-service B2B: verified signup creates one isolated evaluation organisation; operator invitation remains available for controlled customers/teammates | Accepted founder direction; product/security/engineering |
| D-024 | Verified signup may activate only the configured hard-capped shared evaluation offer; it never proves business authority or activates dedicated/paid capacity | Accepted founder direction; product/security/engineering |
| D-025 | Customers choose model, deployment mode, workload profile, and capacity units; Alzette assigns physical infrastructure and never accepts customer target URLs | Accepted founder direction; product/engineering/operator |
| D-026 | One dedicated endpoint capacity unit is a versioned model/runtime/hardware bundle with accelerator count, evidenced capacity metrics, and unit price | Accepted founder direction; exact first profile unresolved; product/finance/operator |
| D-027 | Capacity expansion purchases additional quoted units behind the same stable endpoint and preserves model, tenancy mode, execution class, alias, and credential contract unless separately migrated | Accepted founder direction; product/engineering/operator |
| D-028 | `Dedicated private` describes MeluXina/customer-specific infrastructure; `on-premises` is reserved for equipment at the customer's site | Accepted terminology; founder/product/legal |
| D-029 | New external humans authenticate through self-hosted Casdoor; Alzette remains authoritative for invitations, memberships, roles, models, routes, and usage | Accepted founder direction; product/security/engineering |
| D-030 | Interactive employees use maximum-ten-minute membership-bound `alz_u_` tokens, never permanent personal `alz_k_` API keys; service accounts remain for workloads | Accepted founder direction; product/security/engineering |
| D-031 | Key-only agents use a separate loopback-only Go compatibility proxy with a process-lifetime local credential; Casdoor access and `alz_u_` tokens remain memory-only, while only the rotating identity refresh credential may persist under the protected-store contract | Accepted founder direction; platform/security/product |
| D-032 | Employee login survives client restart through a rotating Casdoor refresh session; P0 defaults are a maximum-one-hour access token, 30-day inactivity and 90-day absolute refresh-session limits, keyring storage by default, explicit restricted-file or memory modes, and no silent plaintext-file fallback | Accepted founder direction; product/security/platform |
| D-033 | Model Improvement is a distinct Alzette-operated product branch for dedicated customers after workflow value is proven. Customers govern objective, permitted data, evaluation criteria, and release approval; Alzette operates dataset preparation, evaluation/adaptation, artefact custody, deployment, and rollback. It remains separate from ordinary metadata-only inference and is not a customer self-service training console. | Accepted user/product direction, 2026-08-16; product/customer/legal/security/operator |
| D-034 | Retained prompts and outputs are treated as valuable company-controlled assets. Alzette provides tenant-isolated custody and policy enforcement as part of the dedicated subscription while making no independent or cross-customer use; exact ownership and legal exceptions remain contractual. | Accepted user/product direction, 2026-08-16; product/customer/legal/security/operator |
| D-035 | Alzette will pursue a front-row role in the MeluXina-AI ecosystem as a Luxembourg specialised inference and Model Improvement operator. This is a strategic objective only; access, partnership, allocation, technical suitability, and commercial status remain unproven until directly evidenced. | Accepted user/product direction, 2026-08-16; founder/growth/product/operator |

---

# I. Delivery plan and success measurement

## I1. Thin vertical slices in build order

### Slice 0 — Forwarding gateway and metering contract

**Status (2026-08-13): Gate A offline proof passed.** The deterministic
compatible-target, PostgreSQL, isolation, retry/accounting, redaction, race,
and Compose gates are green. The full Slice 0 exit still requires one approved
live external response using a newly rotated, file-backed provider key.

Implement the smallest real data-plane path in Docker Compose:

`synthetic tenant client → Alzette gateway → tenant-route binding → fake or approved external target → model`

Start with the non-streaming `/v1/chat/completions` subset. Define provider adapter, request ID, error mapping, timeout, retry-before-output, token finality, redaction, and health semantics. Use deterministic fake targets for failure tests and one approved real external endpoint for compatibility evidence.

**Required tests:** valid call; invalid/revoked key; wrong tenant/project/model alias; missing/disabled target; timeout then retry; upstream 4xx/5xx; partial/missing usage; target health failure; attempted raw URL override; dedicated target bound to a second tenant; shared target without an allow-listed binding.

**Exit:** two synthetic tenants can each call only their authorised route; a real response traverses the gateway; one timeout/retry produces one logical request and two attempts; no prompt/output or secret enters logs; external execution is labelled truthfully.

### Slice 1 — Operator provisioning and customer first call

**Status (2026-08-13): offline software exit passed.** Operator provisioning, human sign-in, workload identities, one-time scoped keys, overlap rotation/revocation, first-call documentation, and two-tenant isolation are implemented and verified. A successful call through the approved real external endpoint remains part of the separate live-provider gate.

Add PostgreSQL-backed organisations, projects, service plans, inference targets, tenant-route bindings, minimal roles, service accounts/keys, operator provisioning, customer sign-in, and copy-safe first-call documentation.

**Exit:** an operator provisions an evidenced exclusive/dedicated test target for synthetic tenant A and an allow-listed shared test target for synthetic tenant B without editing the database; each signs in, creates/uses a scoped key, reaches one successful request, and cannot inspect or call the other tenant’s unauthorised target. A real external pilot remains labelled shared unless dedicated capacity is actually evidenced.

### Slice 2 — Company usage and deployment health

**Status (2026-08-13): offline software exit passed.** Exact logical-ledger usage, attribution, zero/partial/unknown states, current route and plan context, hourly reconciliation, opt-in probe semantics, and safe CSV/JSON export are implemented and verified. The live demo correctly reports zero requests and unknown callability until an application call or explicitly enabled probe supplies evidence.

Persist logical requests and provider attempts, produce hourly rollups, probe target/deployment health, and build the primary Usage page with organisation/project/environment/time filters, summary cards, time series, model/deployment and project breakdowns, recent safe request metadata, allocation/allowance context, and CSV/JSON export.

**Exit:** customer totals reconcile to logical requests across success, failure, and retry fixtures; dedicated and shared views show the correct service context; zero, partial, stale, and unavailable states are truthful; no fixture value remains in the customer path.

### Slice 3 — Self-service evaluation and operational hardening

Add idempotency, concurrency/rate/hard evaluation limits, backup/restore,
migrations, TLS/ingress, telemetry, audit search, incident/support path,
retention configuration, and runbooks. Complete verified signup, transactional
mail, atomic evaluation-tenant provisioning, recovery, invitation/member
management, and abuse/cost controls from
[`ACCOUNT_ONBOARDING_PRD.md`](ACCOUNT_ONBOARDING_PRD.md). Complete the bounded
Casdoor acceptance spike, federated identity links, short human-agent tokens,
actor-safe accounting, and first loopback proxy path from
[`WORKFORCE_AGENT_ACCESS_PRD.md`](WORKFORCE_AGENT_ACCESS_PRD.md). Publish only a
curated catalogue/profile whose evidence owner exists.

**Exit:** a verified prospect enters one isolated shared evaluation tenant,
connects an interactive agent through short-lived human access or creates a
separate scoped workload key, makes a real bounded call, and sees its exact
actor attribution; duplicate signup cannot create extra capacity. The Compose
deployment rebuilds and restores cleanly, limits and revocation are enforced,
and an operator can investigate without content.

### Slice 4 — Dedicated endpoint catalogue, fulfilment, and MeluXina migration

After authorised access exists, validate one sellable deployment profile and
its capacity unit, issue a versioned price/capacity quote, and implement the
operator-side module for allocation records, pinned model/runtime deployment,
private address/port registration, health/restart, capacity inventory, and
rollout evidence. Fulfil one accepted customer request and support one quoted
capacity increase as a new capacity revision.

**Exit:** a customer configures a model/profile/units without choosing a raw
machine; the accepted quote identifies dedicated GPUs, evidenced capacity, and
price; one approved model server becomes an owned MeluXina target; the stable
customer route/key moves from the external pilot to it; adding validated units
preserves the endpoint contract. Isolation, performance, recovery, data path,
commercial, and hosting evidence pass.

### Slice 5 — Customer-controlled private interaction vault

For a subscribed dedicated organisation, add a separately gated content path
behind the existing logical request. Implement versioned `none`, `selected`,
and bounded `policy_matched` retention; tenant-isolated encrypted content and
indexes; customer roles for policy/view/export/select/delete; visible policy
notice; integrity and completeness evidence; time-bounded audited Alzette
operator access; expiry, backup deletion, hold, export, and termination paths.
Keep logs, analytics, billing, support, and improvement datasets content-free.

**Exit:** one dedicated customer can activate a scoped policy, retain and find
only eligible interactions, export/delete them, prove every access, and return
to `none` without cross-tenant, cross-purpose, backup, or hidden-copy leakage.
The customer agreement states Alzette's non-appropriation/non-cross-use promise
and any ownership, client/employee, hold, or deletion qualifications.

### Slice 6 — Alzette-operated Model Improvement branch

After one dedicated inference workflow has measured value and the customer
approves an improvement phase, create one contracted programme. Alzette
prepares a tenant-isolated versioned dataset from expressly permitted sources,
records provenance and rights, establishes a private evaluation baseline,
runs one reproducible evaluation or approved adaptation, and presents the
candidate and limitations for customer release approval. Deployment uses the
existing versioned route lifecycle and preserves a last-known-good rollback.

**Exit:** one customer-approved programme proceeds from separately authorised
vault interactions or another permitted source to baseline-versus-candidate
evidence; vault retention alone grants no training/evaluation authority;
artefact custody, retention/deletion, operator access, model licence, cost,
promotion, and rollback are evidenced; the customer can approve or reject the
release without operating training infrastructure.

### Slice 7 — Scale only after product evidence

Prioritise only measured needs among SSO/SCIM, shared-pool fairness, target
replicas, staged rollout, private networking, async/batch, advanced usage
allocation, invoice integration, additional models, or automation of the
managed Model Improvement branch.

**Exit:** one P1/P2 capability has an owner, data source, security/contract
policy, success metric, and customer demand evidence.

## I2. Privacy-safe analytics/product-success events

Analytics MUST contain metadata, not prompt/output content or raw secrets.

| Event | Meaning | Important properties |
|---|---|---|
| `signup_submitted/verified/completed` | Self-service acquisition | identity/tenant pseudonym, duration, result/blocker; no email or token |
| `evaluation_provisioned/allowance_exhausted` | Shared evaluation lifecycle | tenant/offer pseudonym, limit class, result; no target URL or cross-tenant pool data |
| `org_setup_started/completed` | Provisioning funnel | tenant pseudonym, actor role, duration, blocker class |
| `project_created` | Workload activation | project pseudonym, environment, inherited policy |
| `catalogue_model_viewed/selected` | Model decision | model/version, mode, eligibility result; no prompt |
| `deployment_profile_selected` | Capacity configuration | profile/version, service mode, units, metric finality; no raw host |
| `deployment_quote_offered/accepted/expired` | Commercial conversion | quote pseudonym, units, currency/period, finality, actor/result; no payment secret |
| `deployment_capacity_requested/activated` | Endpoint expansion | deployment pseudonym, previous/new units, evidence state/result |
| `service_account_created` | Production identity setup | project/env, scope class, role |
| `api_key_created/revealed/rotated/revoked` | Credential lifecycle | key pseudonym, actor type, scope class, result; never secret |
| `target_binding_created/changed` | Operator routing setup | tenant/route/target pseudonyms, dedicated/shared mode, execution class, actor/result; no raw URL |
| `route_requested/ready/tested` | Runtime setup | route pseudonym, model/version, tenancy mode, execution class, latency/status |
| `first_request_succeeded/failed` | Activation | route, deployment, status/error class, latency, request ID pseudonym |
| `route_promoted/rolled_back` | Release operation | source/target version, approver role, result |
| `vault_policy_activated/changed` | Company content-custody decision | tenant/scope pseudonym, policy version/mode, actor/result; no content |
| `interaction_retained/exported/deleted` | Private-vault lifecycle | tenant/interaction pseudonym, policy version, content class, completeness, retention/hold/result; no content |
| `operator_content_accessed` | Accountable Alzette custody operation | tenant/interaction/task pseudonym, approved purpose, actor, start/end/result; no content |
| `improvement_programme_started/closed` | Managed improvement engagement | tenant/programme pseudonym, objective class, policy/evidence version, actor/result; no customer content |
| `improvement_dataset_versioned/deleted` | Private dataset custody | programme/dataset pseudonym, source class, item count, rights/retention state, digest/result; no content |
| `improvement_run_started/completed` | Alzette-operated evaluation/adaptation | programme/run pseudonym, base release, dataset/evaluation version, method class, cost/finality, result; no content or artefact secret |
| `candidate_release_approved/rejected/rolled_back` | Customer-governed release decision | programme, base/candidate releases, evaluation version, approver role, decision/result |
| `usage_viewed/exported` | Company consumption use | scope, period, deployment/model, source/finality |
| `budget_created/threshold_triggered/cap_blocked` | Spend control | scope, threshold class, result |
| `contract_viewed/invoice_reconciled` | Commercial confidence | document/period, status |
| `support_request_created/incident_viewed` | Support need | category/severity, route/incident pseudonym |
| `meluxina_access_verified` | MeluXina access evidence | project pseudonym, access route, allocation class, terms version; no credentials |
| `meluxina_deployment_started/completed` | Infrastructure evidence | model/runtime/container digest, resource class, result; no prompt/output |
| `target_migration_started/completed` | Runtime migration evidence | old/new execution class, route/target pseudonyms, compatibility/isolation result |

## I3. KPIs

### Activation and value

- median and p90 time from verified signup to first successful evaluation request;
- signup submitted → verified → portal entered → key created → first request conversion, with blocker classes;
- evaluation cost and allowance exhaustion per verified organisation, including duplicate/abuse blocks;
- median and p90 time from provisioned invitation to first successful request for operator-assisted customers;
- percentage of evaluation and provisioned organisations reaching a first call within one business day;
- percentage of first calls that succeed without support intervention;
- evaluation → qualification → quote offered → quote accepted → dedicated ready conversion and elapsed time;
- quoted versus activated capacity units and scale-up lead time;
- percentage of active pilot organisations with a named workflow, baseline, and outcome measure;
- percentage of subscribed dedicated organisations with an explicit vault
  policy decision, plus retained/exported/deleted interaction counts and
  policy-to-improvement conversion without measuring content or employees;
- weekly active project/route count, interpreted only with customer context.

### Operational trust

- percentage of route states backed by a live source and freshness timestamp;
- logical-request-to-usage-rollup reconciliation error rate and finalisation time;
- percentage of retries correctly represented as one customer request and multiple operator attempts;
- dedicated target ownership and shared binding isolation test pass rate;
- key revoke propagation time and zero secret-leak test failures;
- cross-tenant authorization test pass rate;
- vault cross-tenant/cross-purpose isolation pass rate, unauthorised content
  access count, deletion/backup-expiry completion, and operator-access audit
  coverage;
- budget/cap enforcement test pass rate;
- incident detection, acknowledgement, and resolution times against signed support terms once those exist.

### Product usability

- first-call checklist completion rate;
- route setup abandonment by reason (capacity, model, permission, contract, error);
- successful key rotation rate without outage;
- usage export success rate;
- percentage of active organisations that view company usage or export it during the period;
- support requests per active route and avoidable documentation questions.

No vanity KPI—page views, synthetic requests, fixture spend, or demo clicks—may be reported as customer traction.

## I4. Go/no-go gates

### Gate 0 — Forwarding software PoC

**Go only if:** two synthetic tenants pass target-routing isolation; a scoped credential reaches one real external compatible model through the Alzette gateway; target URLs and secrets remain server-side; deterministic timeout/retry and error tests pass; one logical request is distinct from its attempts; usage is persisted; and external execution is labelled accurately.

**No-go:** fixture-only responses, arbitrary upstream URL selection, cross-tenant target access, retry double-counting, prompt/output or secret leakage, or a customer-visible claim that the route is on MeluXina.

### Gate 1 — Technical safety gate

**Go only if:** two synthetic tenants pass isolation tests; dedicated ownership and shared allow-list constraints pass; key creation/rotation/revoke works; production scopes are enforced by the gateway; audit excludes secrets/content; limits are enforced where promised; backup/restore succeeds.
**No-go:** the portal is the only enforcement layer or values are hard-coded.

### Gate 2 — Pilot activation gate

**Go for public evaluation only if:** transactional verification, durable abuse
limits, one enabled shared offer, cost owner/kill switch, hard gateway
allowance/rate/concurrency enforcement, real first-call/usage path, privacy and
acceptable-use terms, and backup/recovery pass. **Go for a customer pilot only
if:** a target organisation, workflow, sponsor, test-data policy, assigned
model/deployment, service plan, location disclosure, support route, and
first-call success threshold are agreed.
**No-go:** a broad “AI platform” demo with no workflow or decision owner.

### Gate 3 — Commercial/trust gate

**Go only if:** a pilot can see exact contractual scope, service mode,
allocation/allowance, retention/region terms, model licence status, and
escalation path. Any accepted dedicated quote snapshots model/profile, units,
accelerator count, capacity metrics/finality, currency/period, total price,
execution boundary, expiry, and source; acceptance is not rendered as payment
or readiness.
**No-go:** unsupported SLA, residency, certification, or pricing claims.

### Gate 4 — MeluXina infrastructure and migration gate

**Go only if:** authorised access/terms exist; one quoted capacity unit maps to
real assigned GPU resources; one pinned model deployment is reproducible; its
published capacity metrics are validated for the quoted unit count; the gateway
reaches it through a supported private address/port; target registration,
health, restart, scale-up, and rollback work; the complete data path is
documented; and binding/adding units preserves customer API compatibility and
isolation.

**Limited result:** a working Slurm job, SSH tunnel, VPN experiment, or batch invocation without stable service/recovery evidence is infrastructure-PoC-only. It does not authorise a MeluXina production or hosting claim.

**No-go:** no authorised access, customer-serving use prohibited, no stable private route, irreproducible deployment, failed isolation/migration, unknown data path, or unavailable capacity.

### Gate 5 — Private interaction-vault gate

**Go only if:** a subscribed dedicated customer has approved the exact policy;
tenant-isolated encrypted content/index storage, region and keys, policy-derived
roles, visible notice, per-read/export/operator audit, integrity/completeness,
export, expiry/deletion, backup expiry, legal hold, termination handover, and
breach response pass. Contracts state permitted purpose, customer control,
Alzette non-appropriation/non-cross-use, subprocessors, and legal exceptions.
**No-go:** silent or browser-controlled retention, unclear purpose/rights,
cross-tenant search or keys, content in logs/analytics/support/billing, hidden
copies, unaudited operator reads, or deletion claims that ignore backups/holds.

### Gate 6 — Model Improvement gate

**Go only if:** a dedicated workflow has measured value; the customer has
explicitly authorised the improvement objective and every permitted data
source; client/data/model rights, retention/deletion, tenant-isolated custody,
operator access, evaluation baseline and thresholds, artefact handling,
release approver, deployment, and rollback are documented and tested.
**No-go:** automatic transfer from inference traffic or the vault into an
improvement dataset without separate authority, unclear rights or purpose,
shared datasets/artefacts, no baseline, customer-operated raw infrastructure,
or a candidate promoted without explicit approval and validation.

### Gate 7 — Scale gate

**Go only if:** at least two qualified organisations complete the first-call workflow, one proceeds to a technical/security review or paid next step, and operational/support evidence is stable.
**No-go:** interest without time, data, technical review, or budget signal.

## I5. Definition of done

### This PRD is done when

- the product decision and target segment are explicit;
- the repository/claim/evidence boundary is recorded;
- competitor workflows are source-linked and marked verified/inferred/unknown;
- entity, roles, information architecture, workflows, P0/P1/P2 requirements, NFRs, states, dependencies, risks, decisions, and acceptance criteria exist;
- every unresolved commercial/operational promise has an owner and decision path;
- no requirement depends on a fixture being mistaken for production.

### The eventual P0 MVP is done when

- a verified prospect can create exactly one isolated evaluation organisation,
  enter without an operator-issued password, create a separate key, make one
  real hard-capped shared call, and see its usage;
- duplicate signup cannot mint extra allowance, public signup can be disabled
  instantly, and no shared target is exposed as dedicated/private;
- the portal exposes a curated model/version/profile catalogue and lets an
  authorised organisation configure units without selecting raw infrastructure;
- a versioned quote snapshots dedicated GPU count, capacity evidence/finality,
  price and execution boundary, while accepted/allocating/deploying/ready remain
  separate;
- an operator can provision a tenant, service plan, inference target, and tenant-route binding without a database edit;
- the first approved administrator chooses their own portal credential through
  a scoped invitation, and authorised admins can onboard teammates without
  handling their passwords or confusing them with inference API keys;
- an authenticated pilot tenant can create/use a project and safe scoped credential;
- the tenant-approved model and target binding drive routing; dedicated and shared invariants pass;
- the tenant can make a real compatible first request and see request ID/status/usage metadata through an honestly labelled external pilot target;
- a stable route can be tested and its target binding changed or rolled back without changing the customer API or affecting another tenant;
- production credentials are scoped, one-time revealed, rotatable, and revocable;
- logical requests and provider attempts are persisted separately and retry tests do not inflate customer consumption;
- health, freshness, errors, requests, tokens, latency, concurrency, and dedicated allocation/shared allowance are live or explicitly unavailable;
- company usage rollups and exports reconcile to logical requests within policy;
- role, tenant isolation, audit, privacy, export, accessibility, and support tests pass;
- all production claims displayed by the portal can be traced to a live operational or contractual source;
- the complete P0 system starts through Docker Compose on one machine and has tested backup/restore and an accountable support path.

The MeluXina module is done for the first migration when one approved model can be deployed reproducibly, registered through a private LAN target, monitored/restarted, and substituted for the external target through an audited binding change while the same customer URL and credential continue to work.

The private interaction vault is done for its first subscription when one
dedicated organisation can activate a versioned policy, retain only eligible
complete/partial interactions, control and audit every view/export/selection,
enforce expiry/deletion/hold/backup rules, and verify that Alzette has made no
independent, cross-customer, or cross-purpose copy or use.

The Model Improvement branch is done for its first engagement when Alzette can
operate one customer-approved programme from permitted private source data to a
versioned evaluation decision and, if approved, a validated candidate release,
while preserving tenant isolation, evidence, deletion, and last-known-good
rollback without giving the customer raw training infrastructure.

---

# J. Appendix

## J1. Source register

All sources in this register were accessed on **2026-08-12**. Links are primary/official sources unless noted as the official status or trust service.

### Baseten

- [Overview](https://docs.baseten.co/overview) — platform, Model APIs, autoscaling, observability, and multi-cloud concepts.
- [Model APIs](https://docs.baseten.co/inference/model-apis/overview) — shared OpenAI/Anthropic-compatible inference, catalogue, pricing basis, `/v1/models`.
- [Inference overview](https://docs.baseten.co/inference/overview) — synchronous, streaming, async, structured outputs, tool calling, and client configuration.
- [Inference API reference overview](https://docs.baseten.co/reference/inference-api/overview) — Model APIs versus deployed custom endpoints and routes.
- [Reference documentation](https://docs.baseten.co/reference/overview) — inference/management API, CLI, and SDK surfaces.
- [Access control](https://docs.baseten.co/organization/access) — organisation RBAC and audit statement.
- [Teams](https://docs.baseten.co/organization/teams) — Enterprise Teams, team roles, isolation, key scope, and org-level billing.
- [API keys](https://docs.baseten.co/organization/api-keys) — personal/team keys, scope, last use, rotation, revocation.
- [SSO and SCIM](https://docs.baseten.co/organization/sso-and-scim) — Enterprise SAML/SCIM, JIT, directory groups, deprovisioning.
- [Deployment concepts](https://docs.baseten.co/deployment/concepts) — deployment versions, environments, endpoints, and rollback concept.
- [Deployments](https://docs.baseten.co/deployment/deployments) — development/published deployments, rolling/canary notes, deletion/deactivation.
- [Environments](https://docs.baseten.co/deployment/environments) — promotion, stable endpoints, environment metrics, traffic controls.
- [Rolling deployments](https://docs.baseten.co/deployment/rolling-deployments) — staged traffic, pause/resume/cancel/force actions and states.
- [Status and health](https://docs.baseten.co/observability/health) — deployment health states.
- [Logs](https://docs.baseten.co/observability/logs) — request IDs and per-request log filtering.
- [Metrics](https://docs.baseten.co/observability/metrics) — inference volume, status classes, environment/deployment scope.
- [Export metrics](https://docs.baseten.co/observability/export-metrics/overview) — Prometheus-format export and rate limit.
- [Rate limits and budgets](https://docs.baseten.co/inference/model-apis/rate-limits-and-budgets) — Model API limits, budget notifications/enforcement, dedicated exclusion.
- [Regional environments](https://docs.baseten.co/deployment/regional-environments) — region-restricted inference routing.
- [Secure model inference](https://docs.baseten.co/observability/security) — data privacy and default input/output handling claims.
- [Pricing](https://www.baseten.co/pricing/) — public Model API/dedicated pricing basis, plans, enterprise options, support and compliance claims.
- [Status](https://status.baseten.co/) — public service status, uptime and incident history.
- [Trust Center](https://trust.baseten.co/) — public compliance/trust document index and access process.

### Fireworks AI

- [Onboarding](https://docs.fireworks.ai/getting-started/onboarding) — account/key/model-library/playground/serverless flow.
- [Inference introduction](https://docs.fireworks.ai/guides/inference-introduction) — serverless, dedicated, reserved, playground, and API paths.
- [Serverless overview](https://docs.fireworks.ai/serverless/overview) — multi-tenant model, tiers, token billing, lifecycle, and rate limits.
- [Models overview](https://docs.fireworks.ai/models/overview) — base/LoRA models, serverless/dedicated availability and privacy statements.
- [List models API](https://docs.fireworks.ai/api-reference/list-models) — model metadata including serverless support and deprecation fields.
- [On-demand quickstart](https://docs.fireworks.ai/getting-started/ondemand-quickstart) — key → CLI → deployment → query workflow.
- [On-demand deployments](https://docs.fireworks.ai/guides/ondemand-deployments) — dedicated GPU billing, regions, placement, autoscaling, and lifecycle.
- [Autoscaling](https://docs.fireworks.ai/deployments/autoscaling) — replica policy and scale-from-zero behavior.
- [Regions](https://docs.fireworks.ai/deployments/regions) — global/multi-region and residency-oriented placement concepts.
- [Get deployment API](https://docs.fireworks.ai/api-reference/get-deployment) — active/target model versions and replica status.
- [Default deployments](https://docs.fireworks.ai/deployments/managing-default-deployments) — default selection, rollout and replacement pattern.
- [Routers](https://docs.fireworks.ai/deployments/routers) — weighted deployment routing and migration/A-B pattern.
- [Exporting metrics](https://docs.fireworks.ai/deployments/exporting-metrics) — Prometheus metrics for on-demand deployments.
- [Exporting billing metrics](https://docs.fireworks.ai/accounts/exporting-billing-metrics) — billing CSV fields and coverage.
- [Account quotas](https://docs.fireworks.ai/guides/quotas_usage/account-quotas) — spend limits, budget enforcement, request and GPU quotas.
- [Billing management](https://docs.fireworks.ai/faq/billing-pricing-usage/billing/billing-management) — credits, invoices, and billing behaviour.
- [Users](https://docs.fireworks.ai/accounts/users) — Admin/User/Contributor/Inference User roles.
- [Service accounts](https://docs.fireworks.ai/accounts/service-accounts) — non-human identities, keys, roles, billing and audit attribution.
- [Create API key API](https://docs.fireworks.ai/api-reference/create-api-key) — key fields including expiry/last-used metadata.
- [Custom SSO](https://docs.fireworks.ai/accounts/sso) — OIDC/SAML/JIT/enforced SSO for enterprise accounts.
- [Audit and access logs](https://docs.fireworks.ai/guides/security_compliance/audit_logs) — Enterprise audit/data access logging.
- [Data security](https://docs.fireworks.ai/guides/security_compliance/data_security) — zero-data-retention, isolation, security and compliance claims.
- [Reserved capacity](https://docs.fireworks.ai/deployments/reservations) — commitment, guaranteed capacity, invoicing, expiry and overage behaviour.
- [Pricing](https://fireworks.ai/pricing) — serverless token and on-demand GPU pricing/enterprise packaging.
- [Enterprise](https://fireworks.ai/enterprise) — enterprise collaboration, SSO, observability, residency and compliance claims.
- [API reference introduction](https://docs.fireworks.ai/api-reference/introduction) — REST authentication and account management APIs.
- [Inference error codes](https://docs.fireworks.ai/guides/inference-error-codes) — rate/capacity error distinctions and request-ID guidance.
- [Changelog](https://docs.fireworks.ai/updates/changelog) — official product changes, audit UI, billing/contracts, deployment status, and model-library updates.
- [Status](https://status.fireworks.ai/) — public service status, uptime, and incidents.

### LuxProvide / MeluXina / EuroHPC

- [MeluXina](https://www.luxprovide.lu/meluxina/) — current official overview, published capability/security/sovereignty statements, standard batch/CLI access, and the statement that cloud gateways/Kubernetes are being engineered (accessed 2026-08-12).
- [MeluXina system documentation](https://docs.lxp.lu/) — current documentation landing page, updates, status/support links, and AI/HPC/container/web-service navigation (accessed 2026-08-12).
- [System overview](https://docs.lxp.lu/system/overview/) — HPC/Cloud/compute/storage architecture; GPU partition and storage facts; project-folder isolation behavior (accessed 2026-08-12).
- [Project access](https://docs.lxp.lu/access/gaining_access/) — national-share and EuroHPC access paths, commercial/startup eligibility language, project onboarding, SSH-key activation, Project ID, and Service Desk process (accessed 2026-08-12).
- [Allocations and monitoring](https://docs.lxp.lu/access/allocation_monitoring/) — project allocation, monthly node-hour grants, storage quotas, `myquota`, and Slurm/Lustre usage units (accessed 2026-08-12).
- [Platform usage policy](https://docs.lxp.lu/access/PoliciesSummary/) — login-node prohibition on intensive/long-running work, Slurm compute scheduling, production software stack, and non-commercial support scope (accessed 2026-08-12).
- [Quick start](https://docs.lxp.lu/first-steps/quick_start/) — SSH, Slurm, GPU partition, QOS, job and node behavior (accessed 2026-08-12).
- [LLM inference with Triton](https://docs.lxp.lu/howto/llama3-triton/) and [vLLM](https://docs.lxp.lu/howto/llama3-vllm/) — official inference-serving tutorials using containers, GPU jobs, Slurm, and SSH forwarding (accessed 2026-08-12).
- [Container introduction](https://docs.lxp.lu/containerization/introduction/) — Apptainer/GPU/container registry concepts and operator responsibilities (accessed 2026-08-12).
- [Cloud/OpenStack](https://docs.lxp.lu/cloud/openstack/openstack/) — VPN-only dashboard path, VM/GPU entitlement language, service exposure through security groups, and user responsibility for service authentication/security (accessed 2026-08-12).
- [Web-services introduction](https://docs.lxp.lu/web_services/welcome/) — S3, JupyterLab, Slurm REST API, and Open OnDemand described as test access/interfaces being implemented (accessed 2026-08-12).
- [Slurm REST API](https://docs.lxp.lu/web_services/slurmrestd/) — job-control API and short-lived user token documentation; not an inference gateway (accessed 2026-08-12).
- [MeluXina status](https://status.lxp.lu/) — public service categories, current status, and maintenance/status surface (accessed 2026-08-12).
- [LuxProvide Terms of Use](https://docs.lxp.lu/assets/LUXPROVIDE%20MeluXina%20terms%20of%20use%20-%20v10%20-%20final.pdf), version 2023-06-12 — project/allocation precedence, responsibilities, suspension/termination, data handling, EU processing language, confidentiality, and availability disclaimers (accessed 2026-08-12).
- [Startup and SME programmes](https://www.luxprovide.lu/programs/) — current INITIATE and Cashback80 public eligibility/benefit language; live page does not publish a complete current rate card or numeric credit entitlement (accessed 2026-08-12).
- [Official 2025 programme PDF](https://www.luxprovide.lu/wp-content/uploads/2025/09/LuxProvide_Word_Website_Pages_Print_Template_Doc-1.pdf) — older/currently conflicting INITIATE numeric terms; retained only to flag the need to confirm controlling terms, not as entitlement (accessed 2026-08-12).
- [MeluXina-AI](https://www.luxprovide.lu/meluxina-ai/) and [EuroHPC announcement](https://www.eurohpc-ju.europa.eu/eurohpc-ju-signs-contract-meluxina-ai-new-ai-optimised-supercomputer-luxembourg-ai-factory-2026-07-22_en) — future AI-optimised system and launch/installation timeline; not current MeluXina availability (accessed 2026-08-12).
- [EuroHPC access policy FAQ](https://www.eurohpc-ju.europa.eu/supercomputers/supercomputers-access-policy-and-faq_en) — current access-call routes and broad European industry/SME eligibility language (accessed 2026-08-12).
- [EuroHPC AI Factories Industrial Innovation Terms of Reference](https://www.eurohpc-ju.europa.eu/document/download/bd7aa666-bdf3-4436-b5ec-0b34a781e817_en?filename=Terms+of+Reference-AIF+Access+Calls.pdf&prefLang=fi) — eligibility, commercial exploitation of results, resource limits, reporting/publication obligations, and AI Act/civilian-purpose conditions (version 31-03-2026; accessed 2026-08-12).
- [EuroHPC Large Scale Access to AI Factories](https://www.eurohpc-ju.europa.eu/large-scale-access-ai-factories_en) — current open industrial call, >50,000 GPU-hour scope, cutoffs, and approval/term language; likely not the small first-PoC route (accessed 2026-08-12).

### Repository evidence audited

- [`POC_BOUNDARY.md`](../product/POC_BOUNDARY.md) — controlling current software, dashboard, evidence-gate, and MeluXina boundary.
- [`ACCOUNT_ONBOARDING_PRD.md`](ACCOUNT_ONBOARDING_PRD.md) — hybrid self-service
  evaluation/customer account, invitation, recovery, technical-stack, security,
  test, and rollout contract; not current implementation evidence.
- [`WORKFORCE_AGENT_ACCESS_PRD.md`](WORKFORCE_AGENT_ACCESS_PRD.md) — selected
  Casdoor identity boundary, invited-employee short-token path, local proxy,
  human/service actor accounting, security, test, and rollout contract; not
  current implementation evidence.
- [`README.md`](../../README.md) — executable gateway/control/provisioning/key/Compose contracts, browser seam, secret-file handling, live-smoke command, and explicit deferrals.
- [`cmd/alzette/main.go`](../../cmd/alzette/main.go) and [`internal/`](../../internal/) — process modes, strict Chat Completions gateway, authentication, routing, portal/control contracts, secret resolution, provisioning, stores, and tests.
- [`migrations/`](../../migrations/) — embedded PostgreSQL tenant, credential, route, logical-request, provider-attempt, rollup-reservation, and append-only audit schema plus integrity hardening.
- [`portal.html`](../../portal.html), [`portal.css`](../../portal.css), and [`portal.js`](../../portal.js) — protected multi-view client portal; no spend, hardware, invoice, prompt/output, target URL, or locality fiction.
- [`QA_REPORT.md`](../assurance/QA_REPORT.md) and [`POC_TEST_PLAN.md`](../assurance/POC_TEST_PLAN.md) — executed offline evidence, harness limitations, blocked/deferred production gates, and opt-in live-provider gate.
- [`PRODUCT.md`](../product/PRODUCT.md) — target users, positioning, exact current implementation contract, and absent evidence.
- [`MARKET_FIT.md`](../growth/MARKET_FIT.md) — current candid market/funding evidence assessment; treated as repository input, not independent traction proof.
- [`index.html`](../../index.html), [`docs.html`](../../docs.html), and
  [`site.css`](../../site.css) — public marketing and exact
  implementation-documentation surfaces served by the standalone public
  process and excluded from the protected portal static root.
- [`.herdr/team.yaml`](../../.herdr/team.yaml) — committed role ownership for
  coordination, product design, platform implementation, independent review,
  QA, and growth/funding research.

## J2. Competitor and MeluXina facts not verified publicly

The following were not treated as facts because the official public sources reviewed did not establish them or because the relevant console requires login:

- exact Baseten and Fireworks authenticated dashboard screen flows, loading/error copy, and per-plan UI permissions;
- exact contractual SLA values, support response terms, credit remedies, and customer-specific commitments;
- exact invoice formats, tax handling, payment methods, and commercial approval flows for every plan;
- whether Baseten exposes customer service accounts as a separate public concept beyond personal/team keys;
- a complete Baseten public reservation/commitment lifecycle equivalent to Fireworks reservations;
- Fireworks’ complete project/workspace hierarchy, project-level budgets, and project-level cost attribution;
- Fireworks customer-facing inference request logs or traces equivalent to Baseten request-ID logs;
- exact region/datacenter availability, failover guarantees, and residency scope for a particular customer contract;
- the full scope, exceptions, and current contractual terms behind competitor zero-retention, compliance, HIPAA, GDPR, ISO, SOC, private networking, and data-residency claims;
- actual production latency, throughput, error rates, capacity, or customer-specific availability beyond public status pages and docs;
- whether any documented feature is included for a new/small account rather than an enterprise plan.

The following MeluXina/LuxProvide facts were also not treated as established for Alzette:

- Alzette’s eligibility, access route, approval lead time, project/allocation size, hardware entitlement, queue priority, renewal, or contract scope;
- current commercial pricing for node-hours, storage, VM/cloud resources, egress, private networking, support, or managed serving;
- current numeric INITIATE credits, support-ticket allowance, term, or Cashback80 amount/payment mechanics; public programme pages and older official PDFs do not align on all details;
- whether a startup/SME subsidy or EuroHPC allocation can fund a customer-facing commercial inference service without additional contract, reporting, publication, state-aid, or double-awarding constraints;
- an always-on public or customer-private inference endpoint, managed load balancer, autoscaler, production Kubernetes service, or provider-operated OpenAI-compatible gateway on the currently available MeluXina system;
- actual availability of A100-40 GPU nodes or OpenStack GPU flavours for Alzette, queue/cold-start behavior, sustained capacity, network bandwidth, or recovery under pilot load;
- the complete location, processors, retention, deletion, backup, log, support, and incident path for an Alzette gateway connected to MeluXina; the provider’s infrastructure page does not prove every downstream hop;
- the current scope of ISO/IEC 27001, Tier IV, isolation, anonymisation, the public 99.995% statement, or any commercial SLA/remedy applicable to Alzette;
- commercial support response targets, on-call escalation, maintenance notice, incident-report rights, and uptime commitments. The public support policy explicitly describes non-commercial tiers, and the public Terms of Use is not an Alzette service commitment;
- whether the 2026 MeluXina-AI roadmap, AIaaS, multi-tenant, regulated-sector, MLOps, or deployment language is available before its stated end-2026 launch/installation timeline;
- any PoC result. No MeluXina access, deployment, measurement, cost ledger, or failure test was produced in this repository or during this assignment.

## J3. Concise glossary

- **Control plane:** authenticated management surface for identity, configuration, policy, credentials, deployment intent, usage, billing, and support evidence.
- **Data plane:** the runtime gateway and model-serving infrastructure that authenticates and executes inference requests.
- **Organisation/tenant:** the customer security and commercial boundary.
- **Project:** a workload/application boundary inside an organisation.
- **Environment:** lifecycle context such as development or production with distinct policy and route identity.
- **Catalogue model:** customer-readable model family; **catalogue model version** is an immutable reviewed release and is not itself a deployment.
- **Deployment profile:** validated model-version/runtime/hardware bundle from which Alzette can quote one or more endpoint capacity units.
- **Endpoint capacity unit:** profile-defined increment containing an explicit accelerator count, capacity metrics/finality, and unit price; additional units expand an endpoint only after fulfilment and validation.
- **Deployment quote:** immutable, expiring customer-specific snapshot of profile, units, price, capacity, and execution boundary; acceptance is not runtime readiness.
- **Endpoint/route:** stable customer-facing invocation identity; server-side policy binds it to an authorised inference target.
- **Inference target:** operator-controlled model-serving destination. It may be an external pilot API, a dedicated private model server, or an authorised shared pool.
- **Tenant-route binding:** audited mapping from tenant/project/environment/model alias to a permitted target and service plan.
- **Execution class:** truthful location/type label such as `external_pilot` or `meluxina`; it is not inferred from the model name.
- **Shared/serverless:** multi-tenant managed capacity generally metered by usage and constrained by policy/rate limits.
- **Dedicated:** model-serving target and capacity owned by one customer deployment; it cannot accept another tenant’s route.
- **Dedicated private:** customer-specific managed inference capacity on the contractually declared Alzette/MeluXina boundary.
- **On-premises:** infrastructure physically operated at the customer's site; it is a separate future execution class and never a synonym for MeluXina hosting.
- **Reserved:** contractually committed capacity with explicit term/availability/price rules; not automatically synonymous with dedicated or Luxembourg-hosted.
- **Service account:** non-human identity for an application or automation.
- **API key:** revocable secret used by an identity to authenticate to the data plane.
- **Inference request:** one logical customer call and the authoritative unit for customer request counts.
- **Provider attempt:** one outbound runtime execution for an inference request; retries/failover can create several attempts without creating several customer requests.
- **Usage rollup:** time-bucketed aggregate reconciled from inference requests for company dashboards and exports.
- **MeluXina Operations module:** operator-only module for infrastructure allocation, model/runtime deployment, private target registration, capacity, health/restart, rollout, and evidence.
- **Budget:** notification or spending target; **hard cap** is an enforced request-blocking limit.
- **Audit event:** immutable record of an administrative or security-relevant action.
- **Freshness:** age of the source data behind a status or metric.
- **SLA:** contractual service-level agreement. A UI target or status dot is not an SLA.
- **SLO:** internal/service objective used for operations; it is not customer-facing commitment unless contracted.

## J4. Current-repository evidence audit: decision summary

The worktree proves a coherent product narrative and a functioning offline infrastructure vertical slice. It does **not** prove a live provider pilot or production service. Specifically:

- `POST /v1/chat/completions` implements a strict buffered and text/function-tool SSE allow-list. An authenticated key determines organisation, project, environment, service account, and model alias; the client cannot supply a target URL, provider credential, or raw provider model. Streaming retries stop at the first downstream write, terminal usage remains nullable/finality-labelled, and interrupted streams are never replayed.
- PostgreSQL stores hashed one-time Alzette keys, operator targets/routes, one row per logical request, separate provider attempts, nullable token usage/finality, and append-only safe audit metadata. Prompt/output bodies are not schema fields.
- Operator commands provision tenant/project/environment/route/key state and rotate or revoke keys without a manual database edit.
- Bearer machine APIs derive scope from an application key; the separate human portal uses a bcrypt-backed password, revocable server-side session, membership context, and CSRF protection. The current browser view represents one authorised project/environment membership at a time, not organisation-wide cross-membership consumption.
- The multi-view portal labels the connected execution boundary **External pilot / Shared pilot**, distinguishes zero/unknown/partial/stale/unavailable states, and exposes only safe request metadata. A provider name is not inferred in the browser. Route policy, current-binding tenant inference evidence, and optional compatible probes remain separate.
- The Compose path builds one image and runs PostgreSQL, one-shot migrations, gateway, control/portal, and the rollup/probe worker. A deterministic compatible target provides offline failure/retry/accounting evidence without a real provider key.
- Offline unit, race, PostgreSQL, tenant-isolation, retry/accounting, static-containment, protected-browser, responsive, and CSV-safety evidence exists. The opt-in live OpenRouter smoke was not run.
- Human sessions, service plans, server-generated exports, hourly rollups, worker checkpoints, disabled-by-default compatible probes, and the tested Pi 0.84.2 text/function-tool streaming seam are present. Rate/concurrency enforcement, TLS ingress, SSO/MFA where required, backup/restore automation, compatibility beyond that agent subset, stranded-ledger recovery, production telemetry, and a production security/dependency review remain Slice 3 or later work.
- MeluXina remains an explicit strategic destination with no Alzette access, allocation, private serving endpoint, deployment, performance, recovery, cost, or contractual evidence.

This distinction is a hard product requirement: deterministic tests prove the forwarding software boundary, not provider availability, traction, production readiness, capacity, hosting, reliability, or revenue. External execution remains explicit until the live-provider gate passes; MeluXina or dedicated claims remain prohibited until a tested and contracted private deployment replaces it.
