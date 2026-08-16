# Alzette Systems — market-fit and LHoFT assessment

Assessment date: 2026-08-12
Scope: product-market fit, initial segment, competitive position, and fit for the [LHoFT AI Experience Centre](https://lhoft.com/programs/ai-experience-centre/) application.
Decision status: no form submitted, no organisation contacted, and no external commitment made.

## Executive verdict

Alzette has a credible problem thesis—regulated financial organisations want to use AI on sensitive workloads without accepting uncontrolled public-tool risk or operating GPU infrastructure themselves—but there is still no verified customer demand, paid usage, repeat usage, willingness-to-pay signal, measurable workflow impact, or production service evidence. The repository now contains an infrastructure implementation PoC: a tested non-streaming OpenAI-compatible gateway, scoped tenant/project/environment credentials and routing, a PostgreSQL request/attempt ledger, a protected truthful dashboard, operator provisioning, and a single-machine Compose path. The opt-in real OpenRouter smoke remains skipped pending a newly rotated key, so no live provider response is counted. The honest market status is **only a plausible thesis**, with an unproven early problem/solution-fit hypothesis; it is not demonstrated product-market fit.

The LHoFT opportunity is strategically attractive but the current candidate is not yet a strong match. The form requires an already-built or very advanced, continuously demonstrable AI platform, an interactive hands-on environment, clear financial-services relevance, and measurable impact. The worktree now proves a narrow infrastructure PoC and a truthful protected control-plane dashboard, but the current portal is loopback-only and Basic-authenticated; it does not prove a live OpenRouter call, a public URL, or a continuously accessible visitor-facing financial workflow.

**Recommendation: APPLY AFTER SPECIFIC FIXES.** Apply only if the founder/operator can complete and verify the following gates before submission:

- a public, stable website and a live, resettable demo URL;
- one concrete financial-services workflow using synthetic/public data, with baseline and measured impact;
- an operator-approved live-provider smoke and evidence record (or a clearly documented reason it cannot run), without treating the deterministic fake-target tests as provider evidence;
- a continuously accessible interactive demo that can be ready by 2026-10-15 and run through the November go-live;
- a pitch deck, accessible demo video, model/licence permissions, and truthful maturity/user/revenue/team facts;
- a minimum security and operations evidence pack: data flow, hosting boundary, retention, access, incident process, service level, and model availability, including any MeluXina and dedicated-capacity evidence.

If any of those gates remains unverified by the internal go/no-go date, do not apply. The opportunity can create visibility and conversations; it is not a grant, a guaranteed customer channel, or evidence of PMF by itself.

Confidence ratings:

| Judgment | Confidence | Reason |
| --- | --- | --- |
| True PMF is not demonstrated | High — about 90% | No customers, revenue, repeat usage, testimonials, case studies, or signed pilots are present in the evidence reviewed. |
| The underlying problem is plausible in Luxembourg finance | Medium-high — about 75% | The pain is coherent and matches the Centre’s stated AI-adoption mission, but that is ecosystem context, not Alzette buyer evidence. |
| A narrow infrastructure implementation PoC exists in this repository | High — about 90% | Current source, PostgreSQL migration, provisioning path, deterministic gateway/control tests, protected portal, and Compose definition are observable; this is not production or provider evidence. |
| A live OpenRouter-backed service is evidenced | Low — about 10% | The real-provider smoke is present as an opt-in test but remains skipped pending a rotated key; deterministic fake-target tests do not establish a live provider call. |
| The described production service exists outside this repository | Unknown | PRODUCT.md and product copy make operator claims, while the worktree now contains only a narrow PoC boundary. The actual production deployment, owner, uptime, and access still require founder/operator proof. |
| LHoFT fit after the gates are met | Medium — about 60% | A verified controlled-inference demonstration could fit the Luxembourg Spotlight, Operations/Back Office, or Compliance themes; the current infrastructure PoC still lacks a visitor-facing financial workflow, measurable impact, public access, rights, and six-month operating evidence. |

### 2026-08-12 implementation checkpoint

The repository state is materially newer than the earlier fixture audit. Current implementation evidence is a non-streaming `POST /v1/chat/completions` gateway with OpenAI-compatible request/response handling, server-controlled model-alias routing, scoped hashed Bearer keys, a PostgreSQL-backed logical-request and provider-attempt ledger, truthful credential-scoped project/environment usage and route reads, a protected browser dashboard, operator `provision`/key lifecycle commands, and a one-machine PostgreSQL/migration/gateway/control Compose stack. Automated coverage uses deterministic fake compatible targets and includes tenant/routing, retry/accounting, credential, control, portal, and persistence paths.

This checkpoint does **not** establish a live OpenRouter response, real customer usage, provider cost, MeluXina access, dedicated capacity, public availability, or production readiness. The real OpenRouter smoke remains intentionally skipped until an operator supplies a newly rotated key through the documented secret-file path. The current browser portal is a loopback-only HTTP Basic seam for the PoC; it is not a public visitor demo.

## Source and evidence discipline

The report uses five labels:

- **Repository implementation evidence:** observable in files or in a local run.
- **PoC test evidence:** reproducible automated behavior against deterministic test targets or local infrastructure; it is narrower than live provider, production, or customer evidence.
- **Founder/operator claim:** stated in PRODUCT.md or product copy but not independently evidenced by the worktree or a customer record.
- **Assumption or thesis:** a testable interpretation, not a fact.
- **Missing proof:** evidence required before an application or sales claim is safe.

### Sources checked

| Source | What was checked | Result |
| --- | --- | --- |
| [LHoFT AI Experience Centre programme page](https://lhoft.com/programs/ai-experience-centre/) | Current programme description, zones, audiences, and exhibitor invitation | Page was live and current when accessed on 2026-08-12. It describes eight immersive zones, hands-on exploration, and an exhibitor call for interactive, implementation-ready AI solutions with six months of visibility. |
| [Official Zoho application form](https://forms.zohopublic.eu/lhoftfoundation/form/AIExperience/formperma/vaYw71l5D6hHPdoIOVNe_xQq6buM00uh5dvaBwp5LZ0) | Direct HTML fetch of the applicant form | The direct endpoint returned HTTP 200 and exposed the complete two-page form when fetched on 2026-08-12. The browser reader could not follow LHoFT’s zfrmz redirect safely, but the final Zoho URL was independently fetched directly. The form title is “AI Experience Centre - Call for Application”; the stated deadline is 2026-09-15. |
| [PRODUCT.md](../product/PRODUCT.md), [copy.md](copy.md), [README.md](../../README.md) | Positioning, claimed service capabilities, stage, known gaps, and evidence inventory | Clear product thesis and explicit disclosure that customer names, testimonials, case studies, certifications, real benchmarks, SLA figures, and legal-entity details are absent. README also describes the current narrow PoC boundary and its unsupported production/provider claims. |
| [index.html](../../index.html), [docs.html](../../docs.html), [catalog.json](../../catalog.json) | Public surface, API documentation, model catalogue, security copy, and TODOs | A coherent static product surface and integration examples exist. The security page still contains TODOs for the service-level figure and certifications; the privacy-policy link is also a TODO. |
| [cmd/alzette/main.go](../../cmd/alzette/main.go), [internal/gateway/gateway.go](../../internal/gateway/gateway.go), [internal/control/control.go](../../internal/control/control.go), [internal/control/client_dashboard.go](../../internal/control/client_dashboard.go), [internal/portal/site.go](../../internal/portal/site.go) | Current gateway, control API, portal authentication, and browser dashboard wiring | The current tree implements a non-streaming `POST /v1/chat/completions` compatible gateway, server-controlled alias routing, scoped Bearer credentials, a Basic-authenticated client portal/dashboard, tenant-scoped usage and route reads, truthful source/freshness/finality states, and safe metadata-only responses. The portal is a loopback-only PoC seam; it is not public access. |
| [internal/provisioning/validate.go](../../internal/provisioning/validate.go), [internal/store/postgres](../../internal/store/postgres), [migrations/0001_openrouter_poc.up.sql](../../migrations/0001_openrouter_poc.up.sql), [compose.yaml](../../compose.yaml) | Current provisioning, persistence, tenancy constraints, request/attempt ledger, and single-machine deployment | Operator provisioning creates or reuses organisation/project/environment, model alias, target, route, service account, and one-time key records; rotation/revocation are implemented. PostgreSQL stores logical requests separately from provider attempts, with tenant/route constraints and audit events. Compose defines PostgreSQL, migration, gateway, and control processes bound to loopback. This is implementation evidence, not production or provider evidence. |
| [POC_BOUNDARY.md](../product/POC_BOUNDARY.md), [POC_TEST_PLAN.md](../assurance/POC_TEST_PLAN.md), [QA_REPORT.md](../assurance/QA_REPORT.md) | Controlling PoC boundary, test intent, and executed evidence | The boundary separates the offline software, live OpenRouter, and later MeluXina gates. The test plan's current-readiness section and QA report record the passing deterministic slice while explicitly leaving live provider, production operations, MeluXina, dedicated capacity, and PMF unproven. |
| [AWS Bedrock data-protection documentation](https://docs.aws.amazon.com/bedrock/latest/userguide/data-protection.html) and [vLLM OpenAI-compatible server documentation](https://docs.vllm.ai/en/latest/serving/online_serving/openai_compatible_server/) | Competitive substitute check | Managed-cloud privacy/private-network controls and self-hosted OpenAI-compatible serving are established alternatives. OpenAI compatibility alone is not a moat. Accessed 2026-08-12. |
| [DeepSeek API change log](https://api-docs.deepseek.com/updates/) | One model-catalogue cross-check | DeepSeek’s official documentation lists V4 Pro/Flash and an OpenAI-format interface. This corroborates the supplier model names, not Alzette’s access, licence rights, deployment, or capacity. Accessed 2026-08-12. |

### Repository implementation and runtime checks

The following checks were performed locally on 2026-08-12; they establish repository behavior, not customer or provider traction:

- go test ./... passed.
- go vet ./... passed.
- docker compose -f compose.yaml config --quiet passed.
- Current automated tests exercise deterministic compatible fake targets for successful and failed non-streaming requests, retries, timeout/error handling, usage/accounting, credential revocation, route/tenant isolation, control responses, portal protection, and PostgreSQL-backed paths where the test environment permits. They do not call OpenRouter.
- The current Go application wires `POST /v1/chat/completions`, Bearer API authentication, operator provisioning/key commands, PostgreSQL storage, control APIs, and the protected dashboard. `stream: true` is explicitly rejected; the supported PoC is non-streaming.
- The current portal uses HTTP Basic with an Alzette API key as the password, requires `usage:read` and `routes:read`, serves a checked-in truthful dashboard through the control process, and exposes only the minimal dashboard assets. Compose publishes PostgreSQL, gateway, and control ports on `127.0.0.1`.
- The opt-in `TestLiveOpenRouterSmoke` path was not run: it requires an operator-supplied newly rotated key file and an approved provider model. No live OpenRouter response, live provider latency, live provider cost, or real-provider health observation is recorded here.
- DNS lookup for alzette.systems and www.alzette.systems failed from this audit environment. This does not prove a global outage, but it means a publicly accessible website was not verified.

No local server or background process was started during this refresh. The temporary server from the earlier audit was stopped after those historical checks; that audit verified `127.0.0.1:18081` and its corresponding `go run ./cmd/alzette` process were absent. The observation is retained below as historical context.

### Superseded earlier runtime observations

An earlier audit of a prior implementation snapshot recorded a temporary static/fixture server, a fixture `GET /api/dashboard`, fixture account/spend/capacity/request values, `GET /v1/models` returning 404, and no `POST /v1/chat/completions` handler. Those observations were accurate for that earlier snapshot and are **superseded**, not erased: the current tree now contains the gateway, scoped authentication and routing, PostgreSQL ledger, protected portal, operator provisioning, and Compose paths described above. They must not be quoted as the current repository state, and the newer implementation must not be treated as evidence that the earlier runtime ever had those capabilities.

The earlier fixture-only test-plan inventory has now been replaced by a dated current-readiness/results section. The larger 115-case matrix remains an acceptance backlog: unexecuted IDs are not passes, and its deferred production gates do not override the narrower offline PoC evidence.

## 1. Product, customer, buyer, user, and job-to-be-done

### What Alzette is

The repository positions Alzette as B2B infrastructure, not an employee-facing AI application: a managed, OpenAI-compatible endpoint for approved open-weight language, vision, and document models, with organisation/project controls, metering, versioning, and a local operational relationship. The declared promise is a controlled path from a customer application to inference for confidential work.

The current implementation is narrower than that product direction. It provides a non-streaming compatible gateway at `POST /v1/chat/completions`; server-controlled model-alias routing; tenant/project/environment-scoped hashed keys; a PostgreSQL logical-request/provider-attempt ledger; a protected, truthful dashboard; operator provisioning and key rotation/revocation; and a single-machine Compose deployment. It does not establish a live OpenRouter call, MeluXina access, dedicated capacity, public availability, budgets/quotas, streaming, SSO, or a production service-level commitment.

That positioning is internally coherent. It is also easy to confuse with a broad “sovereign AI” claim unless the exact contract boundary, hosting, operators, data retention, model rights, and service levels are shown.

### Customer problem

**Problem thesis:** a regulated financial organisation has a real AI or document workflow, but cannot approve it because data may be sensitive, public AI tools are difficult to govern, model behaviour changes, spend is unpredictable, or the organisation does not want to build and operate GPU infrastructure. It needs a controlled endpoint that its existing application can call, with a defined data and operational boundary.

The problem is specific enough to test, but the repository provides no buyer interview, lost-deal record, usage log, or customer quote. The wording about financial, payroll, tax, corporate, and personal information is product copy, not observed market evidence.

### Buyer and users

| Role | Job in the purchase | What must be proven |
| --- | --- | --- |
| Economic buyer | CTO/CIO, COO, managing partner, or technology-owning director at a regulated financial-services provider | That this person owns a budget and has an active workflow, rather than merely expressing interest in AI. |
| Risk gatekeeper | Compliance, DPO, information-security, operational-risk, and procurement stakeholders | That the data boundary, subprocessors, retention, incident process, model licences, and service level pass review. |
| Direct technical user | Customer developer, IT partner, or systems integrator | That a real application can connect, authenticate, select an approved model, handle errors, and observe usage. |
| Indirect end user | Analysts, operations staff, fund/accounting teams, compliance reviewers, or other staff using the customer’s existing application | That the workflow saves time or reduces risk without unacceptable quality or review burden. |

The repository says partners, directors, and compliance-responsible staff lead evaluation, with developers and integrators as a secondary audience. That is a positioning claim, not segmentation evidence.

### Job-to-be-done

> When our team has a confidential, repetitive text or document workflow, help us run an approved model behind the application we already use, inside a clear contractual and operational boundary, so we can launch AI without creating a new data, vendor, or infrastructure risk category.

The job is stronger than “give us an AI chatbot.” It is a platform adoption job, so the buying cycle, proof burden, and integration dependencies will be heavier than for a simple end-user application.

### Most plausible initial segment

**Hypothesis for the first segment:** Luxembourg-based mid-market fund administrators, asset-servicing providers, and other regulated PSFs with document-heavy operations, an internal developer or trusted IT integrator, and one sponsorable confidential workflow.

Start with one role and one workflow: **COO/IT lead responsible for document operations and AI governance**, initially testing document review, extraction, classification, or controlled summarisation on synthetic or approved data. Use Operations/Back Office and Compliance/Regulation as the likely LHoFT categories; add Risk Management only if the workflow genuinely supports it.

Why this segment is more plausible than “all financial institutions”:

- it has sensitive, repetitive document work;
- it can value a managed service without building a full platform;
- it may have shorter procurement paths than a large bank;
- a local provider and integrator relationship can matter;
- one workflow can produce a measurable time/quality baseline.

Do not initially target every bank, insurer, asset manager, fiduciary, and regulated business. Large banks may be important later, but they are likely to have stronger internal platforms, longer procurement, and a higher evidence threshold. PRODUCT.md also says fiduciaries are a segment, not the target; that should remain consistent until interviews show otherwise.

## 2. Evidence matrix

| Dimension | Repository implementation evidence | Founder/operator claim | Current conclusion and missing proof |
| --- | --- | --- | --- |
| Demand | Product page describes a plausible regulated-finance pain. LHoFT’s call confirms that financial institutions are actively being invited to explore AI, but that is programme context. | “Pilot, signing first clients.” | **No verified demand evidence.** Need 12–15 interviews, named design partners, problem recurrence, and next-step commitments. |
| Willingness to pay | Capacity cards describe Pilot/Dedicated/Reserved; no public price, quote, invoice, contract, or payment record. The earlier dashboard’s €18,000 monthly commitment was fixture data and is not a current usage or billing record. | Predictable metering, hard caps, and production contracts. | **Unproven.** Need a priced pilot offer and at least two qualified buyers willing to pay or sign a non-binding paid-pilot intent. |
| Product value | API documentation, architecture diagram, budget controls, model catalogue preview, and onboarding sequence are present. | Confidential-workload control, Luxembourg hosting, local accountability, stable releases, and support. | Value proposition is clear, but “value” is not the same as measured customer impact. Need a named workflow with baseline, quality guardrail, and quantified result. |
| Differentiation | No benchmark, cost comparison, latency test, contract, or customer comparison is in the repository. | Luxembourg-hosted private inference with open-weight choice and operational controls. | **Plausible positioning, not defensible differentiation.** Hyperscalers, direct model APIs, integrators, and self-hosting are credible substitutes. Prove a buyer-specific advantage in total approval time, data boundary, local accountability, or economics. |
| Implementation readiness | Current source, tests, PostgreSQL migration/store, operator provisioning, protected dashboard, and single-machine Compose define a tested non-streaming infrastructure PoC. Deterministic fake-target tests cover the gateway boundary; the opt-in real OpenRouter smoke remains skipped pending a rotated key. | “Capacity exists and is operational”; API, tenant isolation, budgets, versioning, retention, and contractual hosting are asserted. | **Infrastructure PoC exists; production service remains unknown.** A founder/operator must show the actual live provider path, access controls, hosting, measured capacity/cost, reset procedure, and operational owner. |
| Live provider evidence | The code can route an approved alias to a configured compatible target and records provider attempts, but no current test result calls OpenRouter. | OpenRouter and later private/MeluXina execution are intended routes. | **Unproven.** Do not describe a live OpenRouter-backed endpoint, model availability, latency, cost, or uptime until the rotated-key smoke and evidence record exist. |
| LHoFT demo readiness | The protected dashboard is interactive and truthful for the PoC’s tenant ledger and route state. It is loopback-only with HTTP Basic; it is not a visitor-facing AI task and no public URL or video exists in the repository. | A private inference pilot is available. | **Does not yet satisfy the Centre’s core demo gate.** Infrastructure evidence improves maturity credibility but does not replace a live, continuously demonstrable financial task. |
| Measurable impact | No real benchmark numbers, baseline, case study, or customer outcome. | Copy refers to productivity, reliability, and operational evidence. | **Missing.** Need controlled test data, baseline human time/cost, model quality metric, latency/cost, and a clear definition of “better.” |
| Security and compliance credibility | Security copy and docs describe intended controls; TODOs remain for SLA, certifications, and privacy URL. No security pack, DPA, data-flow diagram, subprocessors, incident record, or audit evidence is in the worktree. | Hosting, retention, access, and service levels are contractual. | **Claim only.** The application can state design intent, not certification or guaranteed controls, until the pack exists. |
| Model availability and rights | catalog.json is a versioned sneak peek and labels itself illustrative; licences and tiers are metadata only. | Curated catalogue including Kimi K3, GLM 5.2, DeepSeek V4, and others. | **Supplier facts are not Alzette deployment proof.** Verify model weight access, licences, redistribution/display rights, safety restrictions, version, capacity, and legal approval. |
| Credibility | Legal name, RCS, VAT, and address appear in PRODUCT.md/footer as owner-confirmed claims. No independent corporate record or live public domain was verified in this audit. | Luxembourg commercial identity and local operation. | **Moderate narrative credibility, low external proof.** Resolve address discrepancy, public website, founder/team, contact details, and legal documents. |
| Distribution | Static site has mailto CTAs; no CRM, analytics, partner pipeline, or referral record. | Direct operational relationship and potential IT partners. | **No repeatable channel shown.** Design a partner motion with Luxembourg integrators, finance associations, and one accountable customer sponsor. |

### What counts as evidence versus not

The passing test suite proves coded behavior against deterministic compatible targets and local stores. It does not prove a live OpenRouter response, inference quality on a real model, production security isolation, customer usage, service uptime, provider cost, MeluXina access, dedicated capacity, or payment. The sample values in the old fixture dashboard—requests, spend, H100 capacity, success rate, nodes, and agreement history—must never be entered as traction or customer metrics unless the operator can produce the underlying records and explicitly confirms that they are real. The current truthful dashboard’s zero/partial/stale/route states are implementation behavior, not customer outcome evidence.

## 3. Competitive frame and differentiation

Alzette is selling a managed control and operations layer around model inference. The relevant competitors are substitutes for that job, not only companies with the same product label.

| Substitute | Why a buyer may choose it | Alzette’s possible advantage | Proof required |
| --- | --- | --- | --- |
| Hyperscaler managed AI, such as AWS Bedrock, Azure AI, or Vertex AI | Existing enterprise contracts, IAM, networking, support, broad model catalogue, procurement familiarity, and documented security controls. AWS documents VPC/PrivateLink and model-provider access boundaries for Bedrock. | A narrower Luxembourg contractual boundary, local accountable operator, smaller-workload economics, and less cloud-platform overhead. | Contractual inference/hosting location, service-level terms, pricing, support response, and an approval-time comparison. |
| Direct model APIs | Fast start, low unit price, broad public documentation, and no infrastructure to operate. DeepSeek’s official docs, for example, expose V4 Pro/Flash through an OpenAI-format API. | Local governance and a service relationship around models, retention, budget, and operations. | Demonstrate why the customer cannot safely buy direct: residency, retention, jurisdiction, procurement, or operational requirements. |
| Self-hosting open-weight models with vLLM or a similar engine | Maximum infrastructure control, model freedom, and potentially better economics at enough volume. vLLM itself provides an OpenAI-compatible server. | Faster path to a pilot and less GPU/MLOps burden for a finance organisation. | Time-to-live, total cost of ownership, performance, on-call burden, and a reproducible deployment. |
| Local cloud, GPU host, or systems integrator | Existing relationship, implementation expertise, and ability to bundle infrastructure with consulting or compliance work. | A focused, productised endpoint and model catalogue rather than a bespoke project. | Partner margin, integration boundary, support model, and proof that Alzette is repeatable across customers. |
| Internal platform build | Best fit for large institutions with existing security, data, and ML teams. | Smaller providers can buy a managed capability without staffing a platform team. | A segment-specific economic case and an implementation plan that respects the buyer’s existing architecture. |

**Positioning conclusion:** OpenAI compatibility, model choice, budgets, and tenant controls are useful but increasingly table stakes. The credible wedge is “a small, locally accountable, contract-first inference service that lets a regulated organisation approve one real workflow quickly.” That is a testable position, not yet a proven moat.

## 4. Exact LHoFT opportunity and programme fit

### What the official programme says

The [official LHoFT programme page](https://lhoft.com/programs/ai-experience-centre/) describes a physical hub for hands-on exploration of AI in finance, with eight zones including Compliance Maze, Immersive Square, AI Co-Innovation Space, and Luxembourg Spotlight. It says startups and tech providers can join, and that applications are open for interactive, implementation-ready AI solutions with six months of visibility to financial institutions, investors, and ecosystem partners. Accessed 2026-08-12.

The directly fetched [application form](https://forms.zohopublic.eu/lhoftfoundation/form/AIExperience/formperma/vaYw71l5D6hHPdoIOVNe_xQq6buM00uh5dvaBwp5LZ0) adds the operative requirements:

- the solution must be continuously demonstrable through an interactive, hands-on experience;
- the applicant must provide platform access, a demo environment, an active subscription, or sufficient credits for visitors;
- the solution must already be available or be a very advanced prototype relevant to financial services;
- it must show a clear value proposition and measurable impact, such as efficiency, risk reduction, revenue, or client experience;
- selection considers relevance, maturity, demo stability, and fit with the Centre’s themes;
- the application deadline is 2026-09-15;
- shortlisted applicants are expected to be demo-ready by 2026-10-15;
- selected demos go live in November 2026;
- showcasing is stated to be free of charge.

The form says the Centre’s visibility may include financial institutions, associations, delegations, investors, public bodies, and ecosystem partners. It also says the opportunity may generate feedback, partnerships, and investment discussions. None of those outcomes is guaranteed.

### Programme-level mapping

Status meanings: **Strong answer** means supported by current evidence; **Weak answer** means a truthful narrative is possible but proof is thin; **Unknown** means founder/operator facts are required; **Blocker** means do not submit until the gap is closed.

| Requirement or selection criterion | Status | Assessment |
| --- | --- | --- |
| Interactive, hands-on experience | **Blocker** | The repository has an interactive, protected control-plane dashboard and a tested gateway boundary, but no visitor-facing financial task and no live provider result. Build and test a resettable workflow. |
| Continuous demonstration and visitor access | **Blocker** | The current portal is loopback-only and HTTP Basic-only. No public demo URL, visitor credential/access policy, uptime record, credits policy, concurrency plan, or reset procedure is evidenced. |
| Already-built platform or very advanced prototype | **Weak answer / blocker** | The current tree is a tested infrastructure PoC rather than the earlier fixture-only snapshot, but deterministic fake-target tests are not live OpenRouter evidence and the financial workflow is absent. The operator’s production-service claim still requires an endpoint and technical evidence. |
| Relevance to banking, insurance, asset management, payments, capital markets, sustainable finance, regtech, compliance, risk, or operations | **Weak answer** | The infrastructure is general-purpose. Fit becomes credible only when tied to one finance workflow, likely document operations or compliance. |
| Clear value proposition | **Weak answer** | “Controlled inference for confidential workloads” is clear and the control boundary is now implemented as a PoC, but the form is looking for an applied solution visitors can understand and use. |
| Measurable impact | **Blocker** | No baseline, benchmark, case study, or outcome exists. Add one task, one baseline, and one measured result. |
| Relevance/maturity/demo stability/thematic fit selection basis | **Weak answer** | The infrastructure PoC and local positioning may fit Luxembourg Spotlight, but thematic fit, real workflow relevance, visitor stability, and live-provider maturity are not evidenced. |
| Six-month showcase commitment | **Blocker / unknown** | The form says six months; operator must confirm budget, staffing, infrastructure, support, visitor access, data-reset logistics, and an operating/recovery owner. The current loopback PoC cannot satisfy continuous operation as-is. |
| Demo ready by 2026-10-15 | **Blocker / unknown** | The date is clear, but the current implementation lacks the public visitor workflow, URL, video, rights, provider evidence, and operating plan needed to meet it. |
| Go live in November 2026 | **Blocker / unknown** | Requires acceptance of LHoFT logistics, a stable visitor release, public/approved access, and six-month support readiness; none is evidenced. |
| Free of charge | **Strong answer** | The form states showcasing is free; do not assume related travel, build, hosting, or staffing costs are covered. |
| Rights and permissions to demonstrate | **Blocker** | Model licences, datasets, code, logos, and any third-party component permissions are not documented; the infrastructure PoC does not establish rights to use a real provider/model in a public demo. |
| Application information accurate and complete | **Blocker until verified** | Required facts about customers, users, revenue, team, legal identity, maturity, live provider status, and demo readiness are missing. |
| Consent to contact and reviewer sharing | **Unknown** | Founder/primary contact must choose and understand these declarations. |

### Form field mapping

The direct HTML exposed 38 applicant fields/questions across two pages, including optional fields, plus information blocks, a programme video, and declarations. The table below lists every discoverable applicant field. No hidden applicant question has been inferred.

| Form field/question | Status | Truthful answer or required action |
| --- | --- | --- |
| Name of Organization/Startup | **Weak answer** | “Alzette Systems” is the product/brand name. The founder must confirm the exact legal applicant name and commercial-name relationship. |
| Registered address (street, line 2, city, region, postal/ZIP, country) | **Blocker** | PRODUCT.md flags a possible old RCS address. Confirm the legally registered address and update the filing if needed before submission. |
| Country of Registration | **Strong answer, founder verify** | Luxembourg is stated in the repository’s legal-identity claim. Confirm against the current corporate record. |
| Founding year | **Unknown** | Founder-only fact; no year appears in the reviewed evidence. |
| Company website | **Blocker** | The repo names alzette.systems, but DNS did not resolve during this audit. Deploy/verify an accessible URL or provide the correct public URL. |
| Company social media URL | **Unknown** | Optional in the form; no verified company profile is in the repository. |
| Number of full-time employees | **Unknown** | Required numeric field; do not infer from the repository or agent activity. |
| Revenue: last month and last 12 months | **Unknown** | No financial record is present. The form’s matrix appears optional, but enter only founder-verified figures or zero where applicable. |
| Company/platform name and brief company description (placeholder says 200 words) | **Weak answer** | A truthful draft is supplied below; replace bracketed founder facts and disclose actual maturity and traction. |
| Previous participation in an LHoFT programme | **Unknown** | Founder-only; the repository contains no participation history. |
| Name of platform/solution | **Strong answer** | “Alzette Systems Private Inference Endpoint” is a truthful working name derived from the product positioning; use the official product name if different. |
| Describe the solution | **Weak answer / blocker** | The product narrative and narrow infrastructure PoC are clear, but a live provider-backed endpoint and visitor workflow are not proven. Use the draft below only after the demo, provider smoke, access boundary, and reset path exist. |
| Challenge or need addressed | **Weak answer** | The confidential-AI adoption problem is a thesis, not a customer quote. Add interview evidence and a named workflow. |
| AI techniques at the core | **Weak answer** | The current PoC implements a non-streaming compatible gateway, server-controlled routing, scoped credentials, and metadata-only ledger; it does not prove a deployed open-weight model or proprietary model technique. Confirm exact runtime, models, retrieval/adaptation, live provider, and licences; do not claim techniques that are not deployed. |
| Monthly platform users | **Blocker** | Required numeric field; no user telemetry exists. The operator must provide the real number, including zero if that is the truth. Do not use dashboard fixture requests. |
| Financial-services areas targeted | **Weak answer** | Recommend Operations/Back Office and Compliance/Regulation for the first workflow; add Risk Management only with evidence. |
| Current maturity: Fully Operational / Pilot with Clients / Advanced prototype / Research project with demonstrable prototype / Other | **Blocker** | The current repository supports an “advanced prototype” description for the narrow infrastructure PoC, not “Fully Operational” or “Pilot with Clients.” Select a maturity level only after the operator reconciles the skipped live-provider smoke, actual service, workflow, and support evidence. |
| Pitch deck upload, PDF, maximum 20 MB | **Blocker** | No deck is in the repository. Create one with architecture, workflow, measurement, evidence labels, team, roadmap, and risks. |
| Direct accessible demo video URL | **Blocker** | No video URL is present. Record the actual task and verify that an unauthenticated reviewer can open it. |
| Founder name/surname | **Unknown** | Founder-only fact. |
| Founder email address | **Unknown** | Founder-only fact; use a monitored address. |
| Founder telephone number | **Unknown** | Optional; founder-only fact. |
| Social media of founder | **Unknown / blocker if required** | Required by the form; founder must provide a truthful professional profile. |
| Is the founder different from the designated primary contact? | **Unknown** | Founder must select Yes or No. |
| Primary contact name/surname | **Unknown** | Required founder/operator fact. |
| Primary contact title | **Unknown** | Required founder/operator fact. |
| Primary contact email | **Unknown** | Required founder/operator fact. |
| Primary contact telephone | **Unknown** | Required founder/operator fact. |
| Primary contact social media | **Unknown** | Optional; founder/operator fact. |
| Core team with names and positions | **Blocker** | Required narrative; no team facts are in the repository. Supply names, roles, relevant experience, and actual commitment. |
| How did you hear about the AI Experience Centre? | **Unknown** | Choose the truthful option. The present audit route was the official LHoFT page/form, but the founder should answer based on how the applicant actually learned of it. |
| Why are you applying? | **Strong draft, proof-dependent** | The Centre’s stated visibility and feedback goals align with the validation need; draft below. |
| What are you looking mostly out of this collaboration? | **Strong draft, proof-dependent** | Ask for design-partner feedback, workflow validation, and ecosystem conversations; do not imply guaranteed leads or investment. |
| Rights and permissions declaration: Yes/No | **Blocker until verified** | Select Yes only after model, dataset, software, brand, and demo permissions are documented. |
| Accuracy/completeness declaration: Yes/No | **Blocker until verified** | Select Yes only after all founder facts and evidence have been reviewed. |
| Understanding that submission does not guarantee selection: Yes/No | **Strong answer** | Yes, if the applicant understands the statement. |
| Agreement to be contacted about the application/evaluation: Yes/No | **Unknown** | Founder/primary contact decision. |
| Consent to share application information with authorised reviewers: Yes/No | **Unknown** | Founder/operator decision after reviewing the form’s privacy terms. |

## 5. Truthful application-ready wording

The following drafts are deliberately conservative. Text in square brackets is a founder/operator fact or a required substitution. Do not remove the brackets by guessing. The only implementation facts safe to state today are the narrow non-streaming compatible gateway, scoped credentials/routing, PostgreSQL request/attempt ledger, protected loopback Basic portal, operator provisioning, and single-machine Compose path. The drafts do not turn deterministic fake-target tests, the old dashboard fixture, or the skipped live-provider smoke into traction, public access, or production readiness.

### Company/platform name and brief company description

The form’s placeholder asks for approximately 200 words. Use this only after the legal identity, maturity, public URL, and workflow have been confirmed:

> Alzette Systems is the commercial name of DUCHENE S.à r.l.-S, a Luxembourg company [FOUNDER: verify the exact registered name, RCS/address, founding year and website]. We are building controlled inference infrastructure for regulated organisations. Our current implementation proof of concept is a non-streaming OpenAI-compatible gateway through which an organisation can connect an existing application to an operator-approved model alias, with scoped credentials, server-controlled routing, PostgreSQL request/attempt metadata, and a protected tenant dashboard. Broader controls such as quotas, budgets, retention policy, live model availability, hosting location and service levels remain [FOUNDER: insert only if actually deployed and evidenced].
>
> For this application, we propose a demonstration of [FOUNDER: name one validated financial-services workflow] using [FOUNDER: synthetic/public/approved data]. Alzette is intended to sit behind the customer’s application; it is not an employee-facing chatbot or a compliance consultancy. At application date, the solution is [FOUNDER: verified maturity] and the demo is available at [FOUNDER: tested URL]. Customer count, monthly users, revenue, paid pilots, certifications, service levels and benchmark results are [FOUNDER: insert verified figures or state that none are yet available].

### Previous LHoFT programme participation

> [FOUNDER ONLY: enter “No” if there has been no prior participation. If “Yes”, name the programme, year, role, and outcome. The repository contains no verified history.]

### Platform/solution name

> Alzette Systems Private Inference Endpoint

Use the registered product name if the operator has one. Do not use “production-ready” unless the service, evidence pack, and support path are actually ready.

### Describe the solution

Use after a live demo has been built and tested:

> Alzette is building a managed inference endpoint for regulated organisations. A customer’s existing application or IT partner connects through an OpenAI-compatible API and selects an operator-approved model alias. The current infrastructure proof of concept applies scoped credentials, server-controlled routing, and metadata-only request/attempt accounting; live inference provider, capacity, quotas, budgets, retention, hosting location, and support commitments are [FOUNDER: insert only when verified].
>
> At the AI Experience Centre, visitors will use a resettable demonstration of [FOUNDER: named financial workflow] with [FOUNDER: synthetic/public dataset]. The demo will show the input, model output, structured result, control boundary and measured outcome: [FOUNDER: verified baseline and result]. No visitor-submitted confidential data will be used.

The second paragraph is a required implementation promise. Do not submit it until the environment, reset, data policy, and measured outcome exist.

### Challenge or need addressed

> Financial institutions often have repetitive text and document workflows but cannot move directly from interest in AI to approved use on sensitive data. Public tools may not satisfy the organisation’s requirements for data handling, retention, model governance, spend control or vendor accountability; building and operating a private GPU stack can be disproportionate for one workflow. Alzette addresses this gap with a controlled inference endpoint that existing applications can call.
>
> Our first validation focus is [FOUNDER: segment and workflow]. We will measure [FOUNDER: baseline metric], [FOUNDER: quality or risk guardrail], and [FOUNDER: latency/cost metric]. This is a problem hypothesis being tested with financial-services practitioners, not a claim that all institutions share the same need.

### AI techniques at the core

> Alzette is an inference and control layer rather than a model-training product. The current proof of concept accepts a bounded, non-streaming OpenAI-compatible chat request, applies tenant-scoped credentials and server-controlled model-alias routing, and records metadata for the logical request and provider attempts. [FOUNDER: insert only the exact deployed model(s), serving runtime, provider, capacity, retrieval/adaptation method, version, licence, evaluation method, quotas, budgets, retention, and support commitments.]

### Core team

> [FOUNDER ONLY: list each person as “Name — position — relevant experience and responsibility for this demo.” Include who owns infrastructure, security/compliance, customer discovery, visitor support, and six-month operations. No team member facts are present in the repository.]

### Why are you applying?

> We are applying to turn a controlled-inference thesis into a concrete financial-services demonstration. The AI Experience Centre is a relevant setting because visitors can see and use an AI workflow rather than evaluate infrastructure from a slide alone. We want to test whether a locally accountable, contract-first inference layer [FOUNDER: verify any Luxembourg/EU hosting or jurisdiction statement before using it] helps a financial-services team approve and operate one confidential workload. We are seeking evidence and informed feedback from financial institutions, practitioners, integrators and ecosystem partners; we are not assuming that visibility will become a customer or investment outcome.

### What are you looking mostly out of this collaboration?

> We are primarily seeking three things: first, structured feedback on whether our proposed workflow solves a material problem for financial-services teams; second, introductions or conversations with potential design partners and IT/integration partners who can evaluate the deployment boundary; and third, feedback on the evidence required for procurement, security review and a paid pilot. We would use the six-month setting to measure visitor comprehension, demo completion, workflow value, and qualified follow-up—not simply visitor volume.

### Non-narrative fields that cannot be drafted safely

The founder/operator must supply, verify, and retain evidence for the legal address, founding year, FTE count, revenue, user count, maturity, pitch deck, demo URL, founder and contact identities, team, model/data permissions, and declarations. The repository does not contain those facts. A blank or invented value is worse than a candid “none yet” where the form permits it.

## 6. Lean 30-day validation plan

The objective is not to manufacture traction before the deadline. It is to determine whether one narrow buyer will take a concrete next step and whether the product can show a credible finance outcome.

### Target interviewees

Recruit a role-balanced sample, without treating job titles as proof of need:

- 5–6 COO, operations, or document-process owners at Luxembourg fund administrators, asset-servicing firms, and regulated PSFs;
- 3–4 CTO, CIO, IT/security, DPO, compliance, or operational-risk stakeholders from the same segment;
- 3–4 Luxembourg or European IT integrators who already connect software into regulated financial institutions;
- 2–3 practitioners who have recently approved, rejected, or paused an AI/document automation project.

Target 12–15 completed conversations, with at least eight from the initial segment. Do not count a general networking conversation as a validated interview unless it covers a recent workflow, current workaround, buyer, budget, and next step.

### Interview questions

Ask about observed behaviour before presenting Alzette:

1. Tell me about the last AI or document-automation workflow your team evaluated. What triggered it?
2. What data would the workflow touch, and which data could not be sent to an unmanaged external tool?
3. What is the current workaround? How many people, hours, documents, or euros does it consume?
4. What stopped the project, or what allowed it to proceed?
5. Who owns the budget, who can block it, and what evidence do they require?
6. Which alternatives are you considering: public cloud, direct model API, self-hosting, or an integrator?
7. Does Luxembourg hosting or a locally accountable operator change the decision? Why or why not?
8. What would a 30-day paid pilot need to prove?
9. What quality, latency, retention, audit, and incident requirements would be non-negotiable?
10. After seeing a ten-minute demo, what is the smallest next step you would actually take?
11. What budget range or existing budget line could fund that next step?
12. Who else should evaluate this, and may we schedule a technical follow-up?

Avoid asking “Would you use this?” or counting compliments. The strongest evidence is a buyer naming a live workflow, introducing the approver, sharing a metric, providing test data, or accepting a paid pilot proposal.

### Falsifiable hypotheses and thresholds

| Hypothesis | Experiment | Success threshold by day 30 | What changes if it fails |
| --- | --- | --- | --- |
| H1: The problem is active, not theoretical | 12–15 problem interviews | At least 8/12 in the target segment report a current or recently blocked confidential-AI workflow; at least 6 identify a next-quarter decision. | Narrow or change the segment/workflow. If fewer than 5 do, pause infrastructure positioning and search for a different job. |
| H2: Local control is a purchase driver | Rank-order the decision criteria and test the architecture boundary | At least 5/12 rank residency, retention, local accountability, or governance in their top three; at least 4 accept a security/technical review. | Treat Luxembourg as distribution/credibility, not as the primary value proposition; compete on workflow or economics instead. |
| H3: An endpoint is enough for a buyer to act | Show the same workflow through a thin application and through the endpoint | At least 3 qualified prospects request a technical discovery, test access, or a scoped pilot; at least 2 can name the integration owner. | Add a vertical application or partner-led implementation. If nobody wants the endpoint, stop selling infrastructure alone. |
| H4: Buyers will pay for the service | Present one transparent pilot offer after the problem interview; ask for a paid next step | At least 2 qualified prospects accept a priced pilot proposal or provide a non-binding paid-pilot intent with buyer, scope, timing, and budget path. | Rework pricing, packaging, or the target segment. Do not use free interest as willingness to pay. |
| H5: A concrete workflow creates measurable value | Build a synthetic/public-data document workflow with a human baseline and model evaluation | At least 20–30% reduction in handling time while meeting a pre-agreed quality/error threshold, or another buyer-approved metric that is materially valuable. | Choose a different workflow/model or conclude the infrastructure does not create enough customer value by itself. |
| H6: The LHoFT demo can stand on its own | Five first-time testers use the demo without operator rescue | At least 4/5 complete the task in under five minutes, explain what the AI did, and identify the control boundary; reset works every time. | Redesign the demo around one task, add guided interaction, or do not apply. |
| H7: The service can support a six-month showcase | Seven-day technical soak test with monitoring, resets, access control, and a visitor-safe data policy | No critical failure; recorded uptime/latency/throughput; no confidential input required; support owner and recovery procedure documented. | Delay the application or use a non-live recorded demonstration only if LHoFT explicitly accepts it; do not claim continuous access. |

### Validation build

Use a small, safe demo, not a grand platform:

1. Select one workflow such as extraction and quality checking of a public fund factsheet or synthetic onboarding document.
2. Use only public or synthetic data; make the visitor session disposable and resettable.
3. Put a thin browser UI in front of the actual endpoint so visitors can see input, output, structured fields, confidence/uncertainty, and the control boundary.
4. Record latency, cost, failure rate, model version, quality score, and human baseline.
5. Prepare a one-page security/data-flow sheet and a model-rights register.
6. Record the same live workflow as the required demo video and use the same evidence in the pitch deck.

### 30-day sequence

| Period | Work | Exit evidence |
| --- | --- | --- |
| Days 1–3 | Freeze segment/workflow; verify legal identity, public URL, team facts, model licences, and actual service endpoint. | Founder fact sheet; evidence register; go/no-go on whether a live endpoint exists. |
| Days 4–12 | Conduct 12–15 structured interviews. | Interview notes coded by problem, buyer, trigger, current workaround, budget, and next step. |
| Days 7–16 | Build and measure the thin workflow demo with synthetic/public data. | Live URL, reset procedure, metrics, test transcript, failure handling. |
| Days 13–20 | Run technical/security reviews with qualified prospects and integrators. | At least two technical follow-ups or a documented reason for no follow-up. |
| Days 21–25 | Test a priced pilot and request a non-binding next step. | Offers, objections, budget path, and any written intent; no invented traction. |
| Days 26–30 | Rehearse LHoFT demo, record video, assemble deck and evidence pack. | Five-person usability result, pitch deck, video URL, rights register, operations checklist. |
| Day 30 | Apply the gates and decide. | APPLY only if all LHoFT blockers are cleared; otherwise DO NOT APPLY this cycle. |

### Verdict-change rules

- Move from “plausible thesis” to “early problem/solution fit” only if H1, H3, and H5 pass and at least two real design partners take a concrete next step.
- Move toward PMF only after repeated paid use: at least three paying organisations or equivalent paid pilots, repeat/expanded usage, a referenceable outcome, and evidence that acquisition does not rely on one-off founder access.
- Stay at “plausible thesis” if interviews are positive but nobody commits time, data, technical review, or budget.
- Stop or reposition the LHoFT application if H6 or H7 fails, even if the general problem is real. Programme fit is a separate gate from market fit.

## 7. Founder/operator questions and blockers

These are the highest-value unanswered questions. They should be answered in the evidence pack, not by optimistic application prose.

### P0 — required before submitting

1. Has an operator-approved live OpenRouter smoke been run against the current gateway with a newly rotated key? If not, record that it is skipped; if yes, retain the model, timestamp, request ID, response/usage evidence, latency, cost, and failure result without exposing the key.
2. Is there a real externally usable inference endpoint beyond the loopback PoC? What is the URL, authentication flow, supported route/model list, current capacity, operator, uptime history, and reset procedure? The current browser portal is Basic-only and loopback-bound.
3. If the production service is outside this repository, where is its source, deployment record, monitoring, and incident ownership? What can be shown to LHoFT without exposing secrets?
4. What is the exact legal applicant name, current registered address, founding year, RCS/VAT status, and commercial-name relationship? Has the possible old RCS address been resolved?
5. What are the actual monthly users, active organisations, paid pilots, revenue, invoices, renewal/expansion, and pipeline? If zero, enter zero.
6. Which single finance workflow and initial segment will be demonstrated? Who has confirmed that workflow is painful?
7. Can the workflow run continuously for six months with visitor-safe data, rate limits, reset, support, monitoring, and a documented failure mode?
8. What are the live model versions, licences, weights/access rights, restrictions, safety controls, and redistribution/display permissions? In particular, verify every model listed in catalog.json before using its name in an application.
9. What exact metric will demonstrate impact, and what is the baseline? Who supplied or approved the baseline?
10. Can a pitch deck and accessible video be produced before the 2026-09-15 deadline, showing the actual product rather than fixture data?
11. Who are the founder, primary contact, core team, and six-month support owner? Which contact/social/consent fields may be submitted?
12. Does the organisation have a security pack, DPA, retention/deletion policy, subprocessors list, support/incident process, service level, and certification status? If not, what can be honestly stated as design intent?
13. What evidence, if any, exists for MeluXina/LuxProvide access, commercial terms, allocation, GPU availability, networking, security/data handling, production suitability, support, uptime, or dedicated capacity? Until official terms or a signed agreement and a PoC record exist, describe each as unknown.

### P1 — required for a credible business case

1. What is the pilot price, minimum commitment, gross-margin model, GPU/capacity cost, and expected support load?
2. Why would the initial customer choose Alzette over AWS/Azure/Google, a direct model API, vLLM self-hosting, or a local integrator?
3. What integration partners can implement the endpoint, and how are incentives and support split?
4. Which model capabilities are genuinely differentiated for the chosen workflow, and which are replaceable catalogue entries?
5. What customer data may be used for evaluation or adaptation, and how are training data, model artifacts, access, retention, and deletion governed?
6. What is the public launch and lead-capture plan if alzette.systems remains unavailable?

## 8. Funding and ecosystem opportunities

### Highest-value current opportunity: LHoFT AI Experience Centre

The Centre is the clearest near-term market-access opportunity because it is local, directly aimed at financial-services AI adoption, free to showcase, and offers a six-month physical setting with visitors from institutions, associations, delegations, investors, public bodies, and ecosystem partners. It can help Alzette:

- turn an infrastructure thesis into a concrete finance use case;
- meet potential design partners and integrators;
- collect structured feedback under real visitor conditions;
- create a referenceable demonstration and an evidence trail for later funding;
- generate partnership or investment conversations if the demo earns them.

It does **not** provide funding, guarantee a pilot, substitute for security due diligence, or prove willingness to pay. Its value is customer access and credibility, contingent on demo quality and follow-up.

### Luxembourg R&D support — explore after defining a genuine R&D work package

[Luxinnovation’s current R&D project support page](https://luxinnovation.lu/fund/all-funding/industrial-research-projects) says Luxembourg private companies may seek aid for eligible industrial research or experimental-development projects, with maximum aid rates shown as up to 60% for industrial research and up to 40% for experimental development for small companies, subject to eligibility and the applicable process. Applications are made through MyGuichet.lu. Accessed 2026-08-12.

This is worth an eligibility discussion only if Alzette has a real technical R&D project—such as measurable inference optimisation, privacy-preserving evaluation, or a novel deployment/control problem—not simply resale or hosting of existing models. Do not describe ordinary product implementation as research without a defensible work package, cost plan, technical uncertainty, and before-work application timing.

### EIC Accelerator 2026 — later-stage funding watchlist, not an immediate application

The [European Innovation Council Accelerator](https://eic.ec.europa.eu/eic-funding-opportunities/eic-accelerator_en) is open to start-ups and SMEs developing innovations at roughly TRL 6–8; its 2026 offer includes a grant component below €2.5 million and up to €10 million in equity, with full-proposal batching dates including 2026-09-02 and 2026-11-04. Accessed 2026-08-12.

Alzette is not ready to pursue this as a hosting thesis. It becomes plausible only if the company can show a genuinely innovative, scalable technology or business model, a large European market, technical maturity, and customer evidence. A managed endpoint assembled from existing models is unlikely to be compelling without a proprietary control, privacy, efficiency, or deployment innovation.

### Fit 4 Start — valuable but closed for the current cycle

[Luxinnovation’s Fit 4 Start page](https://luxinnovation.lu/assess-and-accelerate/fit4start/) describes six months of coaching, up to €150,000 of equity-free funding, and network access, but states that the #17 call is closed; the current-cycle call closed on 2026-07-10. Accessed 2026-08-12. Monitor the next call rather than pretending it is available now. The published eligibility also includes an early-stage startup/team profile, so confirm timing, company age, team size, and innovation fit before investing in the next application.

### LHoFT Catapult programmes — monitor, do not force-fit

LHoFT’s [current acceleration-programme page](https://lhoft.com/programs/catapult-acceleration-programs/) says some 2026 industry programmes are closed while a financial-inclusion LAC programme is open. The closed FundTech programme is closer to Alzette’s Luxembourg finance context but is not an available current route; the open financial-inclusion route is not an obvious fit for a Luxembourg private-inference infrastructure product. Accessed 2026-08-12. Reassess the next FundTech or AI-relevant call after customer evidence exists.

## 9. Recommended next steps

1. Treat the LHoFT application as a conditional market-validation experiment, not a traction claim.
2. Resolve the P0 questions and create a fact/evidence register with an owner and source for every application field.
3. Deploy and verify one visitor-safe financial workflow on the actual service, or stop the application.
4. Run the 30-day interview and paid-pilot tests against the thresholds above.
5. Produce the pitch deck, demo video, security/data-flow sheet, model-rights register, and operations plan from the same verified demo.
6. Submit only if every LHoFT blocker is cleared and the founder can truthfully select the maturity, users, team, rights, and declaration fields.
7. If the gates fail, keep the product thesis, narrow the segment, and use the evidence to pursue customer discovery and Luxinnovation eligibility—not a broad “sovereign AI” funding narrative.

**Bottom line:** Alzette now has a credible infrastructure implementation PoC, but no evidence of PMF, demand, customers, revenue, measurable workflow impact, public demo, MeluXina access, dedicated capacity, or live OpenRouter use. LHoFT remains **APPLY AFTER SPECIFIC FIXES**: the continuously accessible visitor-facing financial workflow, impact evidence, public URL/video, rights, six-month operating plan, and complete application facts are still blockers. The smallest credible path is one verified live-provider path, one real workflow, one measurable outcome, one public/resettable demo, two qualified paid-pilot signals, and a verified legal/operations evidence pack.
