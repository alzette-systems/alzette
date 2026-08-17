# Client Software Migration Fit

**Growth evidence brief — reviewed 2026-08-16**
**Scope:** migrating customer-facing software whose backend calls a model API or
AI SaaS. This is not an employee-chat integration brief.

> Correction to the adjacent research: [CORPORATE_AI_RUNTIME_FIT.md](CORPORATE_AI_RUNTIME_FIT.md)
> covers employee-facing Microsoft 365 Copilot, ChatGPT Work, and desktop
> runtimes. It remains useful background, but it is not the answer to this
> migration problem and is not used as evidence for the conclusions below.

## Executive answer

Alzette has a credible wedge when a company owns the application calling an
OpenAI-style endpoint and can change the endpoint, credential, and model alias
through configuration. The first offer should be a bounded **application
migration assessment plus compatibility gateway**, followed—only after the
technical and infrastructure gates pass—by a managed dedicated endpoint.

The promise is:

> Move the inference boundary of an existing customer-facing application with
> the least invasive change that its real API contract permits; preserve a
> tested request/response contract, then operate the selected private capacity
> with an explicit data and rollback boundary.

This is not a universal drop-in replacement for OpenAI, Claude, Azure, or every
AI SaaS. A simple Chat Completions application may be configuration-only. A
provider-specific feature may need an Alzette protocol façade or an application
rewrite. A locked SaaS with no documented custom endpoint or BYOM path is
blocked unless its vendor supports the change or the customer replaces that
workflow.

**Confidence:** medium that this is a real, technically legible buyer problem;
medium-low that Alzette can sell the full production offer today. The
repository proves a narrow compatible gateway and control-plane boundary, not
live provider availability, MeluXina access, dedicated capacity, production
SLOs, or customer demand.

## Evidence grammar

- **Verified:** directly stated in a current primary source or demonstrated in
  the repository.
- **Inference:** a reasonable product or sales conclusion from verified facts.
- **Unknown:** requires the customer’s code, tenant, contract, or a PoC.
- **Claim guardrail:** wording Alzette must not use until the unknown is proved.

Provider documentation changes. Every provider-specific statement in this
brief records the access date; re-check the exact SDK and API version during
each assessment.

## 1. The actual customer problem

### Buyer, user, and job

The buyer is the organisation that owns a customer-facing product, its
confidential data path, and the supplier/procurement decision. Typical
economic buyers are a CTO, CIO, product/engineering director, or regulated
business sponsor with security and procurement authority. The day-to-day user
is the application engineering/IT team; security, privacy, legal, and
operations approve the route.

The job is not “give employees another chatbot.” It is:

1. keep the application’s customer workflow and interface;
2. move model execution away from an incumbent public API, Azure resource, or
   unsuitable SaaS;
3. avoid an unnecessary rewrite while preserving quality, latency, tool, and
   failure semantics;
4. gain an explicit operational and contractual boundary for prompts, outputs,
   credentials, capacity, and support; and
5. retain a reversible path while the new model/runtime is evaluated.

“ChatGPT API” is not precise enough for qualification. OpenAI’s primary
documentation refers to the OpenAI API, while ChatGPT is a separate product
surface. Always identify the actual hostname, path, SDK, package version,
model/deployment identifier, and features used.

### Best initial segment

**Most plausible initial segment:** a regulated or knowledge-intensive
organisation that owns a production application, has an engineering owner, and
uses a simple OpenAI-compatible text or tool-calling request contract. It can
be a financial-company customer portal, internal case-management product, or
software company serving regulated clients.

Why this segment is attractive:

- the application owner can actually change configuration;
- the value is attached to a live customer workflow rather than employee
  curiosity;
- a stable endpoint and controlled capacity have an obvious operational buyer;
- migration can be measured with the customer’s own fixtures and acceptance
  tests; and
- the buyer can start with synthetic/redacted traffic before a data-processing
  decision.

Do not begin with a locked SaaS, a full agent platform, or a workflow that
depends on one provider’s hosted state and tools. Those can become advisory or
replacement work, not a credible low-friction migration promise.

## 2. Current Alzette evidence and boundary

| Item | Status | What it supports |
| --- | --- | --- |
| OpenAI-compatible endpoint | **Verified in repository** | A narrow target for a customer application that uses the supported contract. |
| Protocol | **Verified in repository** | Strict buffered and text/function-tool SSE Chat Completions subset. This is not full OpenAI compatibility. |
| Target proof | **Verified in repository** | Deterministic compatible-target/offline proof. |
| Routing and credentials | **Verified in repository** | Operator-controlled tenant/project/environment routing and scoped workload credentials. |
| Request accounting | **Verified in repository** | PostgreSQL request/attempt ledger and metadata-level usage evidence. |
| Browser/control plane | **Verified in repository** | Protected truthful dashboard and operator provisioning paths. |
| Deployment | **Verified in repository** | Single-machine Docker Compose evidence. |
| Live OpenRouter response | **Absent** | The real provider smoke is still skipped pending a rotated key. |
| Responses API, native Anthropic Messages, Azure, multimodal, Realtime | **Absent** | No current repository evidence for these contracts. |
| MeluXina access, dedicated capacity, Luxembourg execution, production SLA | **Absent** | Product direction or infrastructure hypothesis, not migration evidence. |
| Customer demand, revenue, workflow impact | **Absent** | No traction may be inferred from the fixture or gateway. |

The local product boundary says that the current target can later point at a
private model server without changing the client API, but that is a future
integration contract, not proof of a private provider. See
[PRODUCT.md](../product/PRODUCT.md) and
[POC_BOUNDARY.md](../product/POC_BOUNDARY.md).

**Growth implication:** sell the first migration as a compatibility and
evidence service, not as “all provider APIs already work.” The first
production-shaped customer should use the proven subset or fund the explicit
adapter work.

## 3. Migration classification

The classification is about the customer’s real application contract, not the
brand on the model.

| Class | Customer condition | Change needed | Alzette treatment |
| --- | --- | --- | --- |
| **Configuration-only** | Customer-owned code sends a supported standard HTTP contract and the endpoint, credential, and model alias are configurable. | Change environment/configuration, secret, and usually model alias; run contract and quality tests. | **Offer first.** Do not call it zero-risk or zero-testing. |
| **Provider SDK with configurable base URL** | The exact SDK/package supports an endpoint override and the application uses a supported subset. | Change base URL, credential, model/deployment alias, and possibly headers. | **Offer after package/version inspection.** Do not infer support from a different SDK or version. |
| **Alzette protocol façade/adapter** | The application is tied to a provider-shaped path or message schema, but its semantics fit a bounded translation. | Alzette exposes the expected façade and translates a documented subset; customer app stays stable or changes minimally. | **Offer as a scoped migration project.** Fail closed on unsupported fields; never silently discard them. |
| **Application transformation** | The application depends on provider-hosted state, tools, files, agents, multimodal/realtime events, or provider-specific semantics. | Customer code and tests change: state, output parsing, tool loop, assets, retries, policy, and evaluation. | **Offer advisory/engineering work only with a named scope.** Do not sell as a URL swap. |
| **Blocked / impossible without vendor support or replacement** | A third-party SaaS owns the backend path and provides no documented custom endpoint, BYOM, export, or replacement seam. | Vendor permission/support or a replacement application is required. | **Qualify out of a low-friction migration.** Do not proxy, scrape, or imply a safe override. |

### Decision rule

Configuration-only means that the existing application can point at the Alzette
URL and still satisfy its own tests. It does not mean that a model is
equivalent, that token limits or tool semantics match, or that the data path is
contractually approved.

An Alzette façade is useful only when the façade has a published, tested
compatibility matrix. A façade cannot reproduce a provider’s private control
plane, hosted conversation store, safety service, tool runtime, or model
behaviour merely by copying a URL.

## 4. Provider evidence and lock-in surfaces

### 4.1 OpenAI API

**Verified endpoint and SDK facts.** OpenAI’s API reference documents
Responses, Realtime, Administration, Chat Completions, audio, video, image,
embedding, files, batches, fine-tuning, and other API surfaces. It says an
application can use an official client library or call HTTP directly; examples
use the OpenAI API v1 surface and bearer credentials. ([OpenAI API reference
overview](https://developers.openai.com/api/reference/overview), accessed
2026-08-16.)

The reviewed official OpenAI documentation does **not** establish that an
official OpenAI SDK universally supports an arbitrary third-party base URL.
The customer’s exact package and version may provide an endpoint override, but
that is an implementation fact to inspect and test, not a blanket OpenAI
guarantee. Direct HTTP code with a configurable URL is easier to qualify.

**Important lock-in surfaces.** OpenAI’s migration guide describes Responses
as a different object model with typed output items, built-in tools such as
web search, file search, computer use, code interpreter, and remote MCP,
multi-turn state, and a different structured-output and function-calling
shape. It also documents response storage/state choices and the migration
from /v1/chat/completions to /v1/responses. Chat Completions remains
supported. ([Migrate to the Responses API](https://developers.openai.com/api/docs/guides/migrate-to-responses),
accessed 2026-08-16.)

**Migration treatment.**

- Simple direct HTTP Chat Completions: configuration-only candidate.
- OpenAI SDK with proven endpoint injection and the same subset: configuration
  candidate; verify the exact library.
- Responses, Realtime, hosted files/vector stores, built-in tools, hosted
  agents, or provider state: adapter or application transformation.
- OpenAI model aliases and snapshots: pin the agreed model/version and run
  application evals; a compatible JSON shape is not behavioural equivalence.

**Claim guardrail:** “OpenAI-compatible” must name the endpoint and supported
  features. It must not mean Responses, Realtime, hosted tools, or all model
  families by default.

### 4.2 Anthropic / Claude API

**Verified endpoint and SDK facts.** Anthropic documents official client SDKs
as general-purpose Messages API clients with streaming, retries, and error
handling. The native surface has its own message and streaming contract.
([Claude SDKs and libraries](https://platform.claude.com/docs/en/cli-sdks-libraries/overview),
accessed 2026-08-16; [Claude Messages API](https://platform.claude.com/docs/en/api/messages),
accessed 2026-08-16; [Claude streaming](https://platform.claude.com/docs/en/build-with-claude/streaming),
accessed 2026-08-16.)

Anthropic explicitly documents an OpenAI SDK compatibility layer: change the
base URL to the Claude API, replace the key, and change the model name. The
same page says the layer is primarily for testing and comparison and is not a
long-term or production-ready solution for most use cases. It documents
material limits: audio input is stripped, strict function-call schema is not
guaranteed, prompt caching is unavailable, system/developer messages are
hoisted into one initial system message, and most unsupported fields are
silently ignored. Native Claude features such as PDF processing, citations,
thinking, prompt caching, and Structured Outputs require the native API.
([Anthropic OpenAI SDK compatibility](https://platform.claude.com/docs/en/cli-sdks-libraries/libraries/openai-sdk),
accessed 2026-08-16.)

**Migration treatment.**

- Simple OpenAI-style chat through the documented compatibility layer:
  configuration-only for an experiment, but not a production-equivalence
  claim.
- Native Messages applications: Alzette Anthropic façade/adapter only for a
  separately tested subset.
- Thinking, citations/PDF, prompt caching, native Structured Outputs, native
  tools, or Claude-specific streaming: application transformation or an
  explicit native-protocol implementation.
- Never silently accept fields that the target ignores; return a
  compatibility error or make the limitation visible.

**Claim guardrail:** “Claude-compatible” must identify native Messages versus
the limited OpenAI compatibility layer. Anthropic’s own warning prevents
Alzette from presenting a thin URL change as a universal production path.

### 4.3 Microsoft Azure OpenAI and Microsoft Foundry

**Current product naming.** Microsoft’s current documentation uses **Microsoft
Foundry**; older pages and “classic” routes may still use Azure AI Foundry
naming. The customer’s resource type and endpoint must be recorded exactly.
([Get started with Microsoft Foundry SDKs and endpoints](https://learn.microsoft.com/en-us/azure/foundry/how-to/develop/sdk-overview?view=azureml-api-1),
accessed 2026-08-16.)

**Verified endpoint and SDK facts.** Microsoft documents the Azure OpenAI v1
route as a configurable base URL ending in /openai/v1/. The OpenAI SDK can be
used with that base URL; OPENAI_BASE_URL is documented; api-version is not
required for the v1 route; and the model parameter is the model deployment
name. Microsoft’s examples show API-key and Microsoft Entra authentication.
([Azure OpenAI in Microsoft Foundry Models v1 API](https://learn.microsoft.com/en-us/azure/foundry/openai/api-version-lifecycle),
accessed 2026-08-16; [Endpoints for Microsoft Foundry Models](https://learn.microsoft.com/en-us/azure/foundry/foundry-models/concepts/endpoints),
accessed 2026-08-16.)

Older Azure-shaped applications commonly use a resource hostname, a
deployment path, and an api-version. The current Foundry documentation states
that deployments carry a model name, version, capacity/provisioning type,
content-filter configuration, and rate-limit configuration. It also states
that unsupported Responses requests can fail and require Chat Completions
instead. A deployment name is therefore not just a cosmetic model label.
([Endpoints for Microsoft Foundry Models](https://learn.microsoft.com/en-us/azure/foundry/foundry-models/concepts/endpoints),
accessed 2026-08-16.)

Microsoft distinguishes the OpenAI-compatible endpoint from the Foundry
project endpoint. The project endpoint provides Foundry project APIs and
platform features such as agents, evaluations, tracing, and
Foundry-specific tools; it is not equivalent to a bare OpenAI-compatible
inference URL. The same SDK overview recommends the OpenAI SDK for maximum
OpenAI compatibility and the Foundry SDK for Foundry-specific features.
([Get started with Microsoft Foundry SDKs and endpoints](https://learn.microsoft.com/en-us/azure/foundry/how-to/develop/sdk-overview?view=azureml-api-1),
accessed 2026-08-16.)

As of this review, Microsoft’s endpoint page says the Azure AI Inference beta
SDK is deprecated and scheduled for retirement on 2026-08-26. An assessment
must therefore capture the exact SDK and migration version rather than
building around a soon-retired client. ([Endpoints for Microsoft Foundry
Models](https://learn.microsoft.com/en-us/azure/foundry/foundry-models/concepts/endpoints),
accessed 2026-08-16.)

**Migration treatment.**

- Azure OpenAI v1 basic Chat Completions/Responses with a configurable
  base_url, deployment alias, and compatible auth: configuration-only
  candidate, subject to supported-model tests.
- Legacy Azure deployment-style path, api-version, AzureOpenAI client, or
  deployment-specific headers: Alzette façade/adapter or a small customer
  change; do not pretend a generic /v1/chat/completions route accepts the
  Azure control-plane path.
- Foundry project APIs, agents, evaluations, Foundry tools, project data
  resources, or Azure-specific tracing/governance: application
  transformation or a separate integration project.
- Azure content filters, model availability, region/data-zone choice, quota,
  private networking, and Entra/RBAC are deployment and contract concerns,
  not proof supplied by an OpenAI-shaped response.

Microsoft’s data/privacy documentation says prompts and completions are not
available to other customers or model providers and are not used to train
foundation models without permission or instruction; it also says processing
location varies by deployment type and geography. This is useful context, not
evidence that an Alzette route inherits Azure’s controls or geography.
([Data, privacy, and security for Azure Direct Models](https://learn.microsoft.com/en-us/azure/foundry/responsible-ai/openai/data-privacy),
accessed 2026-08-16.)

**Claim guardrail:** say “Azure OpenAI v1-compatible subset” only after
testing the exact request, auth, deployment alias, response, streaming, error,
and policy behaviour. Do not say “Azure-compatible” means Foundry agents,
project resources, or Azure operations can move by changing one URL.

## 5. Segment-by-segment fit

| Customer situation | Likely result | Smallest credible Alzette response |
| --- | --- | --- |
| Customer-owned backend, endpoint in environment/config, simple standard text request | **Configuration-only** | Provide endpoint, scoped credential, model alias, compatibility test, rollback variable, and an evidence report. |
| Customer-owned backend, official/third-party SDK accepts a base URL | **Configuration-only or small adapter** | Inspect package/version and actual URL construction; test headers, retries, streaming, errors, usage, and model naming. |
| Customer-owned backend, OpenAI Chat Completions but code reads provider-specific fields | **Façade/adapter or application change** | Implement only the needed response/error/tool subset, or change the customer parser. |
| Customer-owned backend, OpenAI Responses with hosted tools/state | **Application transformation** | Replace state/tool/file dependencies or agree a new Alzette-owned contract; do not promise a Chat Completions façade can reproduce it. |
| Customer-owned backend, native Anthropic Messages | **Façade/adapter or application transformation** | Choose native Messages support for a named subset, or migrate the app to an Alzette OpenAI-style contract. |
| Azure OpenAI v1 with configurable base URL and basic supported call | **Configuration-only candidate** | Map deployment alias and auth; test model availability and exact v1 features. |
| Legacy Azure deployment path or Foundry project/agent application | **Adapter or application transformation** | Preserve the application’s contract only if the required Azure semantics are implemented and tested. |
| Third-party SaaS with documented BYOM/custom endpoint | **Vendor-specific integration** | Confirm the vendor’s supported URL/auth/BYOM contract; treat as a separate adapter, not generic compatibility. |
| Third-party SaaS with no custom endpoint, BYOM, export, or vendor support | **Blocked** | Qualify out or propose replacing the SaaS workflow. Do not intercept traffic or promise a migration. |

## 6. Recommended migration offer

### The offer

**Alzette Application Migration Assessment and Managed Inference Lane**

1. **Contract inventory.** Record the exact endpoint, SDK/package/version,
   authentication, model/deployment name, request and response shapes,
   streaming, tools, state, files, modalities, retries, timeout, data path,
   and acceptance tests.
2. **Synthetic replay.** The customer supplies redacted or synthetic fixtures
   and expected behaviour. Alzette runs a compatibility matrix against the
   candidate endpoint. No production secret or customer content is required
   for this stage.
3. **Least-invasive route.** Classify the application as configuration-only,
   façade/adapter, transformation, or blocked. Provide a bounded diff and
   explicit unsupported fields. The gateway must fail closed rather than
   silently discard semantics.
4. **Side-by-side and canary.** Compare agreed quality, schema validity,
   latency, throughput, error/retry behaviour, and data/retention events.
   Route a small approved share only after rollback is demonstrated.
5. **Managed endpoint.** If the Alzette and infrastructure evidence gates
   pass, bind the application to an operated endpoint with an explicit model
   release, capacity plan, tenant boundary, credential lifecycle, retention
   policy, support path, and commercial commitment.

The paid unit should be a bounded assessment or migration project first. The
recurring offer can then be a dedicated endpoint/capacity commitment with
operations and evidence. Exact price, infrastructure location, availability,
SLA, and capacity are contract fields—not assumptions from this brief.

### What the customer receives

- a one-page migration classification and supported-feature matrix;
- a redacted request/response contract test pack;
- a configuration diff or adapter boundary;
- quality, latency, reliability, and capacity observations;
- a data-path and retention decision record;
- rollback and incident runbook;
- a clear list of work that remains with the customer;
- a recommendation: ready for pilot, PoC only, or blocked.

### What Alzette must not promise

- universal drop-in compatibility;
- identical output quality or model behaviour;
- preservation of provider-hosted state, tools, agent runtimes, or files by
  changing a base URL;
- that a provider-shaped response proves Luxembourg execution, dedicated
  capacity, data isolation, or a production SLA;
- that a locked SaaS can be redirected without vendor support;
- that retained prompts automatically become training data. Retention and any
  Model Improvement use require separate customer authorisation, purpose,
  controls, and contractual treatment.

## 7. Sales qualification questions

Ask for architecture facts and synthetic fixtures, never production
credentials. A call should end with one of the four migration classes.

### Ownership and change authority

1. Do you own and deploy the backend code that calls the model, or does a
   third-party SaaS own it?
2. Can your team change the endpoint URL, credential, model/deployment alias,
   headers, and timeout by environment?
3. Can you deploy a staging build and roll back the previous version?
4. Who approves application, security, data-protection, and procurement
   changes?
5. Can you provide a dependency lockfile, a request/response fixture, and
   synthetic or redacted inputs?

### Exact provider contract

6. What hostname and path does the application call today? Is it
   api.openai.com, an Azure resource, an Anthropic endpoint, or a SaaS-owned
   URL?
7. Which SDK, language, package, and exact version constructs the request?
   Is the URL override documented for that version?
8. Which API is used: Chat Completions, Responses, native Anthropic Messages,
   Azure deployment-style, Realtime, batch, embeddings, image, audio, or
   another surface?
9. What does the application parse: choices/message, typed output items,
   content blocks, tool calls, citations, usage, headers, or custom fields?
10. Does it use streaming? Which SSE/event types are required, and how does it
    handle partial output, cancellation, and errors?

### Lock-in and workflow semantics

11. Does the application use provider-hosted conversations, files, vector
    stores, assistants/agents, web/search/code tools, MCP, prompt caching,
    reasoning/thinking, structured outputs, fine-tuning, moderation, or
    provider callbacks?
12. Does it rely on model-specific token limits, refusal fields, safety
    results, tool schemas, citations, or content filters?
13. For Azure: what resource type, API version, deployment name, auth mode,
    region/data-zone, quota, private endpoint, and RBAC role are involved?
14. For Anthropic: is the app using the native Messages API or the documented
    OpenAI compatibility layer? Which native features are business-critical?
15. For an SaaS: does the vendor document BYOM, a custom inference base URL,
    an API/webhook, data export, or a supported replacement path? If not, what
    vendor permission would be required?

### Production and data acceptance

16. What customer-facing workflow is at risk if inference fails?
17. What are request volume, concurrency, payload/context size, latency,
    throughput, error-rate, availability, and recovery requirements?
18. Which prompts, outputs, files, identifiers, or regulated data cross the
    boundary? May the first test use only synthetic/public data?
19. What retention, deletion, export, legal-hold, training/improvement, and
    cross-customer-use terms are required?
20. What is the acceptance threshold for quality and schema correctness versus
    the incumbent, and who signs it?
21. What is the target migration date, rollback window, support expectation,
    budget owner, and pilot success criterion?

### Qualification outcomes

- **Green:** configuration-only, supported contract, owned code, staging and
  rollback available.
- **Amber:** bounded façade/adapter, with named fields and tests.
- **Red:** application transformation or provider-specific project; sell only
  with explicit engineering scope.
- **Blocked:** locked SaaS or missing change authority; stop promising
  migration.

## 8. Product inputs for acceptance

### Accept now

1. Position Alzette as the managed inference migration and operations layer
   for customer-facing software—not as a replacement for employee ChatGPT,
   Microsoft 365 Copilot, or Claude workspaces.
2. Make a tested OpenAI-compatible Chat Completions subset the first
   migration contract, with versioned request/response/error/streaming
   fixtures and a visible unsupported-feature list.
3. Build the sales and product flow around endpoint injection, model aliases,
   scoped workload credentials, migration evidence, canary, and rollback.
4. Treat the compatibility gateway as a façade with a strict contract, not a
   promise to emulate every provider control plane.
5. Package the first commercial motion as a bounded assessment/PoC followed by
   managed capacity only when the provider and infrastructure gates pass.

### Defer or reject

1. Do not promise Responses, native Anthropic Messages, Azure Foundry project
   features, Realtime, multimodal, or hosted agent portability in P0.
2. Do not build a generic “SaaS traffic redirector” for a vendor that does not
   support BYOM or custom endpoints.
3. Do not claim exact output quality, model equivalence, dedicated capacity,
   Luxembourg execution, or production readiness from the current fixture
   gateway.
4. Do not use employee-runtime integrations as the migration answer.
5. Do not silently drop unsupported request fields, turn off safety/content
   behaviour, or retain customer content for Model Improvement without a
   separate authorisation.

### Founder decisions still needed

- Is the first offer limited to the current Chat Completions subset, or is
  funded adapter work part of the first pilot?
- Which provider protocols, if any, will Alzette own as supported façades:
  OpenAI Chat Completions, OpenAI Responses, Anthropic Messages, Azure v1, or
  only one?
- What is the first supported model/capacity contract once MeluXina access and
  the technical PoC are real?
- Which retention modes, customer-controlled vault terms, and separately
  authorised Model Improvement terms are implementable and contractual?
- What quality/latency/availability evidence is required before a migration
  can be called a production pilot?
- Is application transformation sold, partnered, or explicitly outside
  Alzette’s service?

## 9. Proposed validation gates

These are proposed tests, not completed evidence.

### Gate M0 — five real application inventories

Obtain five target applications without production secrets. Require at least
two with customer-owned code and configurable endpoints. Record the exact
provider feature set and classify each. **Pass:** at least two green or amber
cases with a named technical owner, staging access, synthetic fixtures, and a
rollback route. **Change the thesis:** if most prospects are locked SaaS or
provider-hosted agent workflows, reposition toward migration consultancy or a
different wedge.

### Gate M1 — compatibility replay

Run each green/amber case against the Alzette endpoint using synthetic or
public data. Test buffered and required SSE behaviour, tools, errors, retries,
timeouts, usage metadata, auth denial, tenant separation, and unsupported
fields. **Pass:** every required field is either preserved and tested or
explicitly rejected; no silent loss; customer’s acceptance fixture passes.

### Gate M2 — operational migration

Demonstrate staging canary, stable customer URL, scoped credential rotation,
rollback, incident correlation, and a declared data/retention path. **Pass:**
the customer’s application owner can perform the change from documented
configuration and restore the incumbent route without Alzette editing the
application.

### Gate M3 — infrastructure and commercial evidence

Only after the application contract passes, prove the selected backend,
including authorised provider access, capacity, location/data path, restart
and recovery, support escalation, cost, and contract terms. **Pass:** a
written recommendation says suitable for PoC only, suitable for the first
pilot, or unsuitable. Until then, the migration is a software compatibility
PoC, not a production private-inference promise.

## 10. Source register

All sources below are primary/current documentation reviewed on 2026-08-16.

### OpenAI

- [API reference overview](https://developers.openai.com/api/reference/overview)
- [Developer quickstart](https://developers.openai.com/api/docs/quickstart)
- [Migrate to the Responses API](https://developers.openai.com/api/docs/guides/migrate-to-responses)
- [Models](https://developers.openai.com/api/docs/models)

### Anthropic / Claude Platform

- [CLI, SDKs, and libraries overview](https://platform.claude.com/docs/en/cli-sdks-libraries/overview)
- [OpenAI SDK compatibility](https://platform.claude.com/docs/en/cli-sdks-libraries/libraries/openai-sdk)
- [Python SDK](https://platform.claude.com/docs/en/cli-sdks-libraries/sdks/python)
- [Messages API](https://platform.claude.com/docs/en/api/messages)
- [Streaming messages](https://platform.claude.com/docs/en/build-with-claude/streaming)

### Microsoft / Azure

- [Microsoft Foundry SDKs and endpoints](https://learn.microsoft.com/en-us/azure/foundry/how-to/develop/sdk-overview?view=azureml-api-1)
- [Endpoints for Microsoft Foundry Models](https://learn.microsoft.com/en-us/azure/foundry/foundry-models/concepts/endpoints)
- [Azure OpenAI in Microsoft Foundry Models v1 API](https://learn.microsoft.com/en-us/azure/foundry/openai/api-version-lifecycle)
- [Deploy models using Azure CLI and Bicep](https://learn.microsoft.com/en-us/azure/foundry/foundry-models/how-to/create-model-deployments)
- [Microsoft Foundry REST API reference](https://ai.azure.com/api-reference)
- [Data, privacy, and security for Azure Direct Models](https://learn.microsoft.com/en-us/azure/foundry/responsible-ai/openai/data-privacy)

## 11. Unknowns and facts not verified

- No customer application has yet been qualified in this repository.
- No customer has supplied a real SDK lockfile, contract fixture, staging
  deployment, or acceptance threshold.
- The current Alzette proof is not a live OpenAI, Anthropic, Azure, or
  MeluXina call.
- A customer’s exact OpenAI SDK endpoint override remains package/version
  specific; the reviewed official OpenAI docs do not establish a universal
  arbitrary third-party base URL.
- Anthropic’s documented OpenAI compatibility layer is explicitly not a
  general production portability guarantee.
- Azure endpoint, deployment, auth, quota, model availability, region,
  private networking, and Foundry project behaviour must be verified against
  the customer’s resource and chosen API version.
- No locked SaaS is assumed to permit BYOM, custom base URLs, export, or
  replacement. Absence of documentation is a qualification blocker, not
  proof of impossibility in every contract.
- No MeluXina allocation, access, cost, availability, production suitability,
  or dedicated endpoint has been established here.
- No SLA, certification, customer data contract, or production support
  commitment may be inferred from a compatible response.

**Bottom line:** the strongest first sale is a measured migration of
customer-owned software with a portable, standard request contract. Alzette
should earn the right to claim private, dedicated, and production-grade
inference through the application compatibility, infrastructure, and contract
gates rather than through provider branding.
