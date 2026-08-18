# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Primary: decision-makers across Luxembourg's financial centre — banks, fund administrators, insurers, PSFs, asset managers, fiduciaries — and knowledge-intensive advisory firms serving regulated clients, specifically the partners, directors, and compliance-responsible staff evaluating whether AI can be used on confidential client work without erasing the organisation's distinctive methods and language. They read fast, are risk-averse, and forward URLs to boards. Fiduciaries and large consultancies are segments, not the whole target.

Secondary: developers and IT partners/integrators who connect the endpoint to existing applications. Surfaces route them to technical sections; the decision-maker leads.

## Product Purpose

Alzette Systems is building managed AI infrastructure for Luxembourg's financial centre and other regulated organisations through two related service branches. **Managed Inference** provides stable OpenAI-compatible APIs to approved models, dedicated customer deployments by default, optional explicitly shared service, predictable usage visibility, stable model versions, customer-controlled private interaction custody, and a direct operational relationship. **Model Improvement** is the Alzette-operated lifecycle for turning customer-authorised prompts, outputs, and other permitted data into private evaluation evidence and, where justified, an adapted model release. The intended production environment is MeluXina. Success begins when a financial organisation can evaluate the inference product without a sales gate, then acquire a model endpoint whose dedicated GPU capacity, expected service capacity, price, execution boundary, data-custody policy, and expansion path are explicit before activation.

For a subscribed dedicated customer, prompts and outputs can be retained in a
private, tenant-isolated interaction vault under that company's recorded
policy. The company chooses which projects, people, applications, and
interaction classes are retained; who may inspect, export, select, or delete
them; how long they remain; and whether they may be used for improvement.
Alzette makes no independent or cross-customer use of that content. Contract
and applicable law control any ownership, client-confidentiality, employee,
legal-hold, backup, and deletion qualifications.

Model Improvement is a distinct managed branch rather
than a customer self-service training console. The customer controls the
business objective, permitted source data, evaluation criteria, and release
approval. Alzette prepares the private dataset, runs evaluation and approved
adaptation work, safeguards artefacts, recommends a release, deploys the
approved version, and operates rollback. It is a gated product direction, not
current implementation evidence and never a default use of customer data.

The product is infrastructure. It is not an employee-facing AI workspace, an accounting application, an agent marketplace, or a compliance consultancy.

## Positioning

The intended product is the missing middle between prohibiting AI and accepting an uncontrolled public-cloud relationship: customer-scoped endpoints that existing tools call, with dedicated managed model deployments as the primary offer. Control stays with the customer; operations stay with Alzette. Hosting location, service levels, retention, model availability, and dedicated/shared mode are contractual—defined in the service agreement and evidenced by the running route, not inferred from marketing text.

The strategic value goes beyond infrastructure locality. A company that sends
all of its work through generic AI without preserving its approved
interactions, evaluations, terminology, and release decisions risks outsourcing
part of its institutional differentiation. Alzette gives the customer a
private custody and improvement path so AI can reinforce the firm's methods
rather than flatten them into generic provider behaviour.

The commercial unit is an **endpoint capacity unit**, not a raw machine picker: one approved model release, one validated runtime/hardware profile, a declared number of dedicated accelerators, evidenced capacity metrics, and a versioned price. A customer chooses the model, deployment mode, and required capacity; Alzette assigns and operates the physical infrastructure. Buying additional capacity units expands the endpoint without changing its customer URL or credential contract. Exact physical hardware remains visible as operational/contract evidence and an advanced constraint, not the default selection workflow.

The public landing page sells the finished service, not the temporary implementation route: private model infrastructure operated in Luxembourg, with dedicated customer deployments as the primary production offer. The first viewport must make that commercial promise legible to a financial-industry decision-maker and should not read like a release report. Exact current execution evidence remains visible in the authenticated portal and implementation documentation, where it matters operationally.

The current PoC is narrower and intentionally honest in those technical surfaces: Alzette owns the client endpoint, tenant routing, credentials, metering, and dashboard, and is configured to forward through OpenRouter. Deterministic compatible-target evidence exists; a live OpenRouter call does not yet. The connected customer route is labelled **External pilot / Shared pilot**; OpenRouter remains operator configuration rather than a browser-inferred availability claim. It is not MeluXina-hosted or dedicated compute.

## Operating Context

Evaluation happens under compliance and procurement scrutiny: security packs, service specifications, and board-level forwarding of claims. Integration is done by the customer's developers or IT partner using the OpenAI SDK or standard HTTP tooling. Contracts define the model, dedicated/shared mode, allocation or allowance, limits, retention policy, execution location, and support per tenant.

## Capabilities and Constraints

Current implementation contract: one OpenAI-compatible Chat Completions text/function-tool subset with bounded buffered and SSE responses; operator-controlled tenant/project/environment/model-alias routing; separate human sessions and one-time workload keys; explicit operator-reconciled company ownership plus owner-managed, scope-bound employee access groups; a unified server-rendered Access workspace for People, Groups, and owner-only Application access; exact-email employee invitation creation, manual one-time-link delivery, resend/revoke, scanner-safe setup, and OIDC/PKCE acceptance; a digest-pinned loopback Casdoor service and deterministic bootstrap; group-filtered employee agent-context discovery; OAuth-token validation and introspection; digest-only, alias-bounded `alz_u_` credentials expiring within ten minutes; strict `alz_k_`/`alz_u_` gateway dispatch, current-policy rechecks, revocation, and human request attribution; a memory-only `alzette-agent` demo helper that owns browser PKCE, in-process identity refresh, context selection, short-token remint, an authenticated ephemeral loopback proxy, exit revocation, an isolated Pi 0.84.2 provider, and verified local Linux custom-provider paths for Jan Desktop 0.8.4 and Goose Desktop 1.46.0; one logical customer request separated from internal target attempts; metadata-only usage records; route-bound service plans; exact scoped usage, attribution, rollups, opt-in probes, and safe exports; an authenticated curated Models catalogue, resumable shared/dedicated endpoint configurations, immutable dedicated quotes, hosted-payment state, immutable sizing intent for deployment/capacity requests, and separate commercial/payment/runtime evidence across Overview, Usage, Endpoints, Models, Billing, Access, and Docs; no prompt/output persistence in the current PoC; a standalone database-independent public landing/docs process; single-machine Docker Compose deployment. Transactional email, ownership transfer/recovery, durable protected-refresh storage, automatic native-client configuration, signed cross-platform client packaging, remote TLS, broader client version/OS support, and production Casdoor/offboarding/restore evidence are not implemented. Without complete OIDC configuration, invitations remain pending and the acceptance page says sign-in is unavailable.

Current human accounts are operator-provisioned usernames/passwords. The target
customer-account experience is hybrid self-service B2B: a person verifies a
business email, authenticates through Alzette's self-hosted identity service,
and becomes the organisation's one current owner inside an isolated evaluation
organisation with a hard-capped shared route; approval converts that same
identity and organisation into a customer boundary eligible for dedicated
deployment. The owner invites employees and assigns each employee to one or
more owner-managed access groups. Groups, not employee-selected roles or direct
per-person exceptions, determine which active Alzette model endpoints an
employee can discover and use. The owner can manage and use every active model
endpoint in the organisation. Interactive people authenticate agents with
short-lived, membership-bound human access rather than permanent personal API
keys; service-account keys remain available for applications and unattended
workloads. Ownership transfer is explicit and atomic: an organisation never
has two current owners or intentionally becomes ownerless.
[`ACCOUNT_ONBOARDING_PRD.md`](../prd/ACCOUNT_ONBOARDING_PRD.md) defines the account
lifecycle and [`WORKFORCE_AGENT_ACCESS_PRD.md`](../prd/WORKFORCE_AGENT_ACCESS_PRD.md)
defines invited-employee agent access. Implemented slices are evidence only for
their tested local configuration; the broader roadmap remains non-evidence.

Service stage: **offline-validated OpenRouter-compatible product PoC for the first client**. The opt-in live OpenRouter smoke is pending a newly rotated credential, so a real-provider pilot is not yet evidenced. MeluXina access, allocation, model deployment, private LAN serving, Luxembourg execution, dedicated compute, production SLA, and production capacity are not yet established as live operational evidence. The customer portal and technical documentation must continue to label that evidence exactly; the public landing page describes the intended private Luxembourg service and keeps contract-specific details subject to the applicable client agreement.

Strategic infrastructure objective: Alzette intends to qualify early for
MeluXina-AI and seek a front-row role as a Luxembourg specialised inference and
Model Improvement operator for regulated organisations. This is an ecosystem
and roadmap objective, not evidence of access, partnership, allocation,
preferred status, production suitability, or commercial terms.

Deferred until tested or required by the first workflow: structured output, embeddings, image/audio input, arbitrary model uploads, automatic MeluXina allocation, customer selection of raw machines, customer federation/SCIM, scheduled billing reconciliation, production invoice/tax operations, and multi-host orchestration. The implemented curated catalogue, hard-capped shared evaluation, customer-authored deployment requests, and priced dedicated-capacity quotes are control-plane capability only until an operator publishes evidenced offers and the selected provider/payment/fulfilment paths pass their opt-in gates. Physical provisioning remains operator-approved until its infrastructure and commercial evidence are automated.

A first Model Improvement engagement is deferred until a dedicated inference
workflow has value evidence, the customer has approved the exact source data
and purpose, applicable client/data/model rights are recorded, retention and
deletion are enforceable, and a private evaluation baseline, release approval,
and rollback path exist. Arbitrary uploads, automatic transfer of retained
prompts/outputs into improvement without a separate approval, and a
customer-operated general-purpose training/MLOps workspace remain out of
scope; Alzette operates this branch for the customer.

Model availability is tenant- and route-specific. A catalogue entry describes a reviewed model; a deployment profile describes a model/runtime/hardware combination Alzette can quote; a quote records price and expected capacity; only a validated deployment and active tenant-route binding prove that an endpoint is available. During the PoC, stable Alzette aliases map to operator-approved external model identifiers. Catalogue, quote, deployment, target, and route state must never be collapsed into one optimistic status.

Legal identity (confirmed 2026-08-11, published in footer): "Alzette Systems" is a commercial name of DUCHENE S.à r.l.-S; RCS Luxembourg B258532; VAT LU33413731; registered address 7, route de Mamer, Holzem, Luxembourg (per owner — the RCS record may still show the previous Prince Henri address; a registered-office change filing may be needed). The company no longer retains a fiduciary. The privacy policy URL, committed SLA figure, certification status, and a verified model catalogue remain undecided, so the public page does not link or claim them.

## Brand Commitments

Name: Alzette Systems (alzette.systems). The mark is a trace of the Alzette river through Luxembourg City (`alzette-mark.svg`) — the one ownable visual asset; it anchors header, footer, favicon, and the final-CTA motif.

Voice (binding): calm, contract-first, no hype adjectives, evidence over claims. Anti-references: purple gradients, glassmorphism, "boost your productivity" SaaS-speak, leaderboard marketing.

## Evidence on Hand

`POC_BOUNDARY.md` (controlling current implementation contract), `PORTAL_PRD.md` (broader product requirements), `ENDPOINTS_PRD.md` (implemented endpoint-acquisition contract and remaining gates), `ACCOUNT_ONBOARDING_PRD.md` (future account-onboarding contract), `WORKFORCE_AGENT_ACCESS_PRD.md` (invited-employee OAuth/proxy contract), the running Slice 0–2 portal/gateway, deterministic compatible-target and endpoint/billing integration evidence, the local pinned-Casdoor invitation/OAuth/human-credential/Pi vertical-slice evidence, and the verified request contract. Absent and not to be fabricated: customer names, testimonials, case studies, certifications, real benchmark numbers, SLA figures, MeluXina or MeluXina-AI access/deployments/preferred status, live catalogue supply/prices/capacity unless actually seeded from reviewed evidence, dedicated OpenRouter capacity, live Stripe checkout evidence, an implemented self-registration/evaluation workflow, production mail/TLS/remote employee access, durable protected-refresh storage or a signed cross-platform client, support for untested desktop clients, an implemented private interaction vault, or an implemented private improvement-dataset, evaluation, or fine-tuning workflow. Schema support, a catalogue card, or a customer-facing concept alone is not supply or capability evidence.

## Product Principles

1. A product page, not a changelog — lead with the finished private Luxembourg offer; keep implementation-state evidence in technical and authenticated surfaces.
2. The decision-maker leads; developers are routed, not courted.
3. Contract-first — every production commitment lives in the service agreement; the site points at it rather than paraphrasing it.
4. Control stays with the customer — every surface reinforces who decides versus who operates.
5. Luxembourg is the production differentiator—the public page sells that operating model, while every authenticated runtime surface reports the actual execution class until the private route is tested and contracted.
6. Buy endpoint capacity, not infrastructure trivia—the customer selects a model and an evidenced capacity/price profile; Alzette owns machine placement and operations unless a contract explicitly requires customer-site hardware.
7. Preserve the endpoint while capacity changes—adding dedicated capacity updates the deployment behind the stable route and never silently changes model, tenancy mode, location, or commercial commitment.
8. Separate people from workloads—interactive people use short-lived, revocable human-agent access; applications and unattended automation use service accounts. Neither credential can impersonate the other.
9. Keep company authority simple—one current owner manages the company,
   employees, groups, endpoints, billing, and application access; every other
   person is an employee whose model access comes only from enabled group
   membership; the owner can use every active company endpoint. Ownership
   transfer is explicit, atomic, and recoverable.
10. Customer interactions are valuable company assets—a subscribed company can
   retain prompts and outputs in its private interaction vault under its own
   recorded access, purpose, retention, export, deletion, and improvement
   policy. Alzette does not appropriate or cross-use them.
11. Model Improvement is a distinct Alzette-operated branch—content moves from
    the private vault into an evaluation or adaptation dataset only under a
    separate recorded purpose and approval, with versioned evidence and a
    rollback decision.
