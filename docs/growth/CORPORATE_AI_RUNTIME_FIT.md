# Corporate AI runtime fit

**Owner:** Growth
**Access date:** 2026-08-16
**Scope:** customer guidance for adopting Alzette alongside the software a firm already uses.
**Explicit non-goal:** this is not a new Alzette product branch and not a portal PRD. It does not propose replacing Microsoft 365, Claude, ChatGPT, Google Workspace, an IDE, or a creative suite. The separate [client-software migration brief](CLIENT_SOFTWARE_MIGRATION_FIT.md) covers customer-owned applications that call model APIs; this document covers employee-facing runtimes and must not be used as its answer.

## Executive decision

Alzette should fit around the client’s existing employee stack: keep the familiar shell, route only an approved workflow to an Alzette endpoint when the host supports it, and use a customer-controlled companion when the host cannot change its model path. The commercial story is **“add a governed private inference lane without an office-suite migration,”** not “replace Copilot, Claude, or ChatGPT.”

The least-disruptive route depends on who controls the request path:

1. **Customer-owned application or configurable provider endpoint:** use **Direct endpoint**. This is the cleanest model replacement because the customer’s code or approved host sends inference to Alzette.
2. **Host exposes a custom-engine agent:** use **Embedded agent**. The agent may use Alzette models, but the host still owns some UI, orchestration, identity, or cloud path.
3. **Host exposes only connectors, MCP, plugins, or actions:** use those for data and approved operations, but call the result **Embedded agent / connector-only**, never a model replacement. The host model remains in the loop unless a separately documented custom-engine path says otherwise.
4. **No supported endpoint or embedded-agent surface:** use a **Companion** such as a customer-managed browser shell or an existing internal application. This is an adoption aid, not a new Alzette product branch.
5. **Locked third-party SaaS:** classify as **Application transformation** only when the customer can change the surrounding workflow, or **Not viable** without vendor support/replacement. Do not promise an endpoint swap because a connector exists.

Strongest first routes:

- **Microsoft-heavy firms:** one narrow declarative agent/action for low-risk work, or a custom-engine agent when the customer explicitly wants its own model/orchestrator. SharePoint, Teams, Outlook, and Windows remain the employee surfaces.
- **Claude/Cowork firms:** Cowork connectors and plugins can add Alzette tools, but do not replace Cowork’s model. A separate official Anthropic path exists for Claude for Microsoft 365 through an organisation’s LLM gateway; compatibility and contract boundaries must be tested.
- **ChatGPT Work firms:** an MCP server or GPT custom action can invoke Alzette; it does not make ChatGPT Work run wholly on an Alzette model. Use a companion for an Alzette-only interaction.
- **Google Workspace firms:** a Workspace add-on or Chat app can call Alzette over HTTPS; built-in Gemini is not documented as accepting an arbitrary endpoint. Use an add-on first, companion second.
- **Developer teams:** VS Code Custom Endpoint and GitHub Copilot Enterprise BYOK are the most credible direct routes. Both require endpoint-compatibility, policy, key-custody, and preview/tenant checks.
- **Finance/accounting and creative workflows:** keep Excel, Sheets, ERP, email, document, and creative tools; select an action, add-on, IDE adapter, or specialist companion for the narrow task. Do not treat a text Chat Completions endpoint as image, audio, video, or accounting-system integration.

No current repository evidence establishes Alzette customer adoption, production SSO, production employee-agent integrations, MeluXina access, dedicated capacity, or the complete modality contracts. The current repository evidence is a strict buffered and text/function-tool SSE Chat Completions subset, scoped organisation/project/environment routing and credentials, a PostgreSQL request/attempt ledger, a protected dashboard, operator provisioning, and single-machine Compose. It does not prove a live production provider response or an employee-facing runtime. See [`PRODUCT.md`](../product/PRODUCT.md), [`POC_BOUNDARY.md`](../product/POC_BOUNDARY.md), and [`PORTAL_PRD.md`](../prd/PORTAL_PRD.md).

## Evidence discipline and route vocabulary

| Label | Meaning here |
| --- | --- |
| **Verified** | Stated in a current primary product document or official project repository, accessed 2026-08-16. |
| **Inference** | A reasonable conclusion from verified integration mechanics; it still needs a tenant, security, or compatibility test. |
| **Unknown** | Not established by reviewed public material, or dependent on plan, tenant policy, region, licensing, network, or contract. |
| **Repository evidence** | What this repository implements or documents. It is not customer traction, production-provider evidence, or a claim that a fixture dashboard is a real service. |
| **Vendor statement** | A vendor’s current documentation about its own route or data handling. It is not a contractual commitment to the customer or proof of Alzette compatibility. |

| Route | Use this label when | What it does not mean |
| --- | --- | --- |
| **Direct endpoint** | The host or customer application actually sends model requests to an Alzette URL/base URL or gateway. | It does not prove SSO, data residency, feature parity, or that the host’s other AI features use Alzette. |
| **Embedded agent** | The host displays or invokes a custom agent, action, plugin, add-in, or bot that calls Alzette. | It does not remove the host model, host cloud, host policy, or host data path. |
| **Companion** | A separate customer-approved shell calls Alzette while the incumbent suite remains in place. | It is not a reason to build a new Alzette employee product before demand and boundaries are proven. |
| **Application transformation** | The customer must modify orchestration, protocol, UX, permissions, or workflow code to use Alzette. | It is not “configuration only”; it needs an owned change surface and regression testing. |
| **Not viable** | No supported route is evidenced under the customer’s constraints. | It does not mean the vendor can never add one; it means do not sell one now. |

Connectors, MCP, plugins, and actions are integration surfaces. They carry data or invoke operations; they do not, by themselves, change the host product’s base model.

## Portfolio decision matrix

| Existing surface | Can the underlying model endpoint be Alzette? | Alzette-powered agent/action or connector | Primary classification | Least-disruptive customer route | Boundary and confidence |
| --- | --- | --- | --- | --- | --- |
| Windows itself | **No generic endpoint override found.** Windows is the device and policy shell; each application controls its AI path. | Use the application’s supported extension, an internal web app, or a browser companion. | **Companion** | Keep Windows and deploy one approved app/extension or browser tab. | **Verified/inference:** no reviewed Microsoft source establishes a Windows-wide inference switch. |
| Microsoft 365 Copilot declarative agent | **No.** Declarative agents use Copilot’s infrastructure, model, and orchestrator. | REST/OpenAPI tools, Power Platform connectors, MCP, actions, and Microsoft 365 data can call Alzette. | **Embedded agent** | One narrow agent/action in Copilot Chat, Teams, Outlook, Word, Excel, or SharePoint. | **Verified:** [agents overview](https://learn.microsoft.com/en-us/microsoft-365/copilot/extensibility/agents-overview), accessed 2026-08-16. |
| Microsoft 365 Copilot custom-engine agent | **Yes for the custom agent’s own model/orchestrator, not for standard Copilot.** | Custom engine agent can use an external model/orchestrator and surface in Microsoft channels. | **Embedded agent** | Use the customer’s existing Copilot/Teams distribution with Alzette as the model service, subject to preview/licensing/hosting checks. | **Verified, but deployment is a separate engineering and compliance path:** [custom-engine overview](https://learn.microsoft.com/en-us/microsoft-365/copilot/extensibility/agents-overview), accessed 2026-08-16. |
| Microsoft Graph synced/federated connector | **No.** It is a data/retrieval surface, not a model provider. | Federated connectors can expose real-time, user-scoped retrieval; synced connectors index into Microsoft Graph. | **Embedded agent** (connector-only) | Start with a read-only retrieval or action tool; show data path and permissions. | **Verified:** [federated connectors](https://learn.microsoft.com/en-us/microsoft-365/copilot/connectors/federated-connectors-overview), accessed 2026-08-16. |
| Teams, Outlook, SharePoint, Word, Excel, and Windows M365 clients | **Not as a generic client setting.** A custom engine agent or supported add-in can be surfaced there. | Agents can retrieve, summarize, and take actions such as sending mail or updating records; distribution is tenant-controlled. | **Embedded agent** | One app/channel with a measurable task and explicit “sent to Alzette” status. | **Verified/inference:** [publish agents](https://learn.microsoft.com/en-us/microsoft-365-copilot/extensibility/publish), accessed 2026-08-16; exact tenant availability is unknown. |
| Claude Cowork | **No arbitrary Alzette endpoint is documented.** Cowork supports Claude or the listed own-cloud options, not a general OpenAI-compatible base URL. | Plugins, MCP connectors, skills, tools, computer use, approvals, and scheduled tasks can add Alzette operations. | **Companion** or **Embedded agent** (connector-only) | Keep Cowork for broad work; expose one reviewed Alzette tool. Use a companion for Alzette-owned inference. | **Verified/inference:** [Cowork](https://claude.com/product/cowork), [connectors](https://support.claude.com/en/articles/11176164-use-connectors-to-extend-claude-s-capabilities), accessed 2026-08-16. |
| Claude for Microsoft 365 add-in through an LLM gateway | **Potentially yes.** Anthropic documents an organisation-selected LLM gateway path; exact supported wire format and compatibility with Alzette are not proven. | Office add-in remains the employee surface; gateway carries inference. | **Direct endpoint** if compatible; otherwise **Application transformation** | Qualify the add-in’s gateway contract before promising a direct route. | **Vendor-documented, tenant/contract/compatibility dependent:** [Claude for M365 third-party platforms](https://support.claude.com/en/articles/13945233-use-claude-for-microsoft-365-with-third-party-platforms), accessed 2026-08-16. |
| ChatGPT Work | **No arbitrary private base-model replacement found.** | Plugins, MCP servers, connectors, and GPT custom actions can call Alzette. | **Embedded agent** (connector/action) or **Companion** | Add one governed MCP/action for a selected task; use a separate shell when the whole interaction must run on Alzette. | **Verified/inference:** [ChatGPT Work Overview](https://learn.chatgpt.com/docs/enterprise/chatgpt-work-overview), [GPTs and Sharing](https://learn.chatgpt.com/docs/enterprise/gpts-and-sharing), accessed 2026-08-16. |
| Google Workspace built-in Gemini | **No arbitrary endpoint override found in reviewed public docs.** | Built-in Gemini has admin controls and Workspace context; custom add-ons are a separate surface. | **Not viable** for direct replacement; **Embedded agent** via add-on | Keep Gemini; add one Workspace add-on or Chat app calling Alzette. | **Unknown rather than an absolute impossibility:** [Workspace with Gemini](https://knowledge.workspace.google.com/admin/generative-ai/workspace-with-gemini/google-workspace-with-gemini), accessed 2026-08-16. |
| Gmail, Drive, Calendar, Docs, Sheets, Slides, Meet, and Chat add-on | **The add-on can call Alzette’s HTTPS API; this is not a Gemini model swap.** | Workspace add-ons and Apps Script/HTTP services can call external APIs with user OAuth or service identity. | **Embedded agent** | Start with one add-on or Chat app and narrow OAuth scopes. | **Verified/inference:** [Workspace add-ons](https://developers.google.com/workspace/add-ons/how-tos/building-workspace-addons), [Apps Script external APIs](https://developers.google.com/apps-script/guides/services/external), accessed 2026-08-16. |
| VS Code Custom Endpoint | **Yes.** VS Code documents custom Chat Completions, Responses, and Anthropic Messages endpoints. | MCP and custom agents add tools/context; they do not change the endpoint semantics. | **Direct endpoint** | Customer-managed endpoint configuration or an enterprise-managed proxy; test capability flags. | **Verified:** [VS Code language models](https://code.visualstudio.com/docs/agent-customization/language-models), accessed 2026-08-16. |
| GitHub Copilot Enterprise BYOK | **Yes, public preview.** Enterprise owners can add OpenAI-compatible providers for Copilot Chat, CLI, and IDEs. | MCP/custom agents add tools; enterprise BYOK is the model route. | **Direct endpoint** | Pilot one organisation with server-side enterprise BYOK; reject employee-local keys as the default. | **Verified, preview and policy dependent:** [Copilot BYOK](https://docs.github.com/en/copilot/concepts/models/bring-your-own-key), [custom models](https://docs.github.com/en/copilot/how-tos/administer-copilot/manage-for-enterprise/enable-custom-models), accessed 2026-08-16. |
| Customer-owned finance/accounting application | **Usually yes if its code/configuration owns the provider URL; otherwise requires an adapter.** | Alzette can be called from the customer’s application or an approved gateway. | **Direct endpoint** or **Application transformation** | Migrate one route with replay tests; keep business logic and approvals in the application. | **Use the separate migration brief:** no assumption about a specific ERP/SaaS is made here. |
| Locked third-party finance or creative SaaS | **No public replacement path unless the vendor supports custom providers.** | Vendor connectors may import/export data but do not establish Alzette inference. | **Not viable** or **Application transformation** | Keep the SaaS; add an approved external workflow or replace the vendor only when the business case supports it. | **Unknown per vendor; qualify before sale.** |

## Stack guidance

### Windows plus Microsoft 365, Copilot, Teams, Outlook, and SharePoint

#### Verified mechanics

Microsoft distinguishes two agent types. A **declarative agent** uses Copilot’s infrastructure, model, and orchestrator, while a **custom engine agent** brings its own orchestrator and models and needs additional hosting and its own compliance/security work. Microsoft documents both as able to surface in Microsoft 365 Copilot and applications such as Teams, Outlook, Word, Excel, SharePoint, and Edge. ([Agents for Microsoft 365 Copilot](https://learn.microsoft.com/en-us/microsoft-365/copilot/extensibility/agents-overview), accessed 2026-08-16.)

Microsoft also documents Copilot connectors, Copilot APIs, and Copilot Retrieval API. The Retrieval API can ground a custom engine agent in Microsoft 365 data while respecting user and tenant governance; that is a retrieval route, not proof that Microsoft’s standard Copilot model is redirected to Alzette. ([Extend Microsoft 365 Copilot](https://learn.microsoft.com/en-us/microsoft-365/copilot/extensibility/overview), accessed 2026-08-16.)

Copilot Studio can connect an existing MCP server, but its current documentation says the supported transport is **Streamable HTTP** and SSE is no longer supported for MCP after August 2025. It also documents authentication and Power Platform policy boundaries. ([Connect an existing MCP server](https://learn.microsoft.com/en-us/microsoft-copilot-studio/mcp-add-existing-server-to-agent), accessed 2026-08-16.) The current Alzette repository’s text/function-tool SSE Chat Completions subset is not itself a Streamable HTTP MCP server. An adapter is required.

Agent publication and availability are tenant-controlled. Microsoft documents organisation catalogues, Teams/Microsoft 365 distribution, admin approval, and license-dependent access; exact availability depends on the customer’s Copilot, Copilot Studio, Entra, Power Platform, DLP, and app policies. ([Publish agents](https://learn.microsoft.com/en-us/microsoft-365-copilot/extensibility/publish), accessed 2026-08-16.)

#### Recommended customer route

1. Select one low-risk workflow, such as a redacted document classification or drafting operation.
2. Build one narrow adapter with a named operation, model alias, identity context, content policy, timeout, request ID, retention mode, and audit record. Do not expose an unrestricted model router to Copilot.
3. Start with a declarative agent/action if the customer accepts Copilot’s model and orchestration. Use a custom engine agent only when the customer explicitly needs its own model/orchestrator and accepts the additional hosting and review.
4. Surface the agent in the customer’s preferred Teams, Outlook, SharePoint, or Copilot entry point. Keep Windows unchanged.
5. Show the full route in the pilot: employee input and selected context → Microsoft host/orchestrator or custom agent → Alzette adapter → Alzette endpoint → response. The UI must say which route was used.

#### Guardrails

- Do not say “Alzette replaces Copilot” for a declarative agent or connector.
- Do not say “the prompt stayed in Luxembourg” merely because the endpoint is operated in Luxembourg; the Microsoft host, Graph, connector, telemetry, and tenant policy may remain in the path.
- Do not require employees to paste permanent Alzette keys. Prefer Entra/OAuth or a customer-managed integration identity at the adapter; use scoped, revocable application credentials only for machine-to-machine routes.
- Treat write actions—sending an email, changing a SharePoint file, updating a record, or posting to Teams—as approval-gated until a customer policy and test prove otherwise.
- Treat a SharePoint or Graph connector as permission-aware retrieval only; it is not a model provider.

### Claude and Cowork

#### Cowork facts

Anthropic describes Claude Cowork as a desktop-first work surface, available on Windows and macOS, with web/mobile in beta. It supports files, tools, connectors, plugins, sub-agents, approvals, and scheduled tasks. Enterprise controls include feature access, spend, tool permissions, and OpenTelemetry activity streams. Anthropic lists Claude’s own account or Amazon Bedrock, Google Cloud, and Microsoft Foundry as cloud-provider choices; it does not document a generic arbitrary OpenAI-compatible Cowork base URL. ([Claude Cowork](https://claude.com/product/cowork), accessed 2026-08-16.)

Connectors inherit each user’s source permissions and can retrieve data or take actions. Custom remote MCP connectors are reached from Anthropic’s cloud infrastructure, not the local device, and must be reachable from the relevant public network/allowlist. ([Use connectors](https://support.claude.com/en/articles/11176164-use-connectors-to-extend-claude-s-capabilities), [custom remote MCP](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp), accessed 2026-08-16.) Cowork’s approval modes and scheduled tasks are documented, but those controls govern Cowork actions; they do not turn a connector into Alzette inference. ([Cowork approvals](https://support.claude.com/en/articles/13345190-get-started-with-claude-cowork), [scheduled tasks](https://support.claude.com/en/articles/13854387-schedule-recurring-tasks-in-claude-cowork), accessed 2026-08-16.)

#### Important M365 add-in distinction

Anthropic’s current Claude-for-Microsoft-365 documentation describes an **LLM gateway** route for the Excel, PowerPoint, Word, and Outlook add-ins. It says the organisation can point the add-in at an LLM gateway, and that prompts/responses in the third-party-platform configuration travel to the chosen gateway/provider. The same page names requirements, gateway URL/token, supported provider formats, and network domains. ([Use Claude for Microsoft 365 with third-party platforms](https://support.claude.com/en/articles/13945233-use-claude-for-microsoft-365-with-third-party-platforms), accessed 2026-08-16.)

This is the best Anthropic-related direct-endpoint hypothesis, but not yet an Alzette capability. Alzette must test the exact gateway API format, streaming/tool requirements, headers, authentication, add-in deployment, Microsoft Graph consent, error behaviour, and whether the customer accepts Anthropic’s UI/telemetry boundary. The fact that Anthropic documents a gateway does not prove that an Alzette OpenAI-compatible endpoint is compatible.

#### Recommended route

- Keep Cowork for general employee work; expose one reviewed Alzette operation through a connector/plugin when that host model and cloud path are acceptable.
- Use the Claude for M365 gateway path only for a customer who wants to preserve the Office add-in and is willing to qualify the gateway protocol and data contract.
- Use a customer-managed companion for an Alzette-only conversation or for data that cannot cross the Anthropic cloud connector path.
- Use approvals for write actions and make the remote MCP/cloud path visible in the customer’s data-flow record.

### ChatGPT Work

The current official work documentation uses the name **ChatGPT Work** for the work execution experience associated with Business/Enterprise workspace controls. The work overview describes cloud execution for web/mobile tasks and permissions/workspace policy as part of the boundary. ([ChatGPT Work Overview](https://learn.chatgpt.com/docs/enterprise/chatgpt-work-overview), accessed 2026-08-16.)

The supported integration vocabulary is:

- **Connectors/MCP:** external data and tools; OpenAI’s documentation covers tools, connectors, and remote MCP servers. ([Tools, connectors, and MCP](https://developers.openai.com/api/docs/guides/tools-connectors-mcp), accessed 2026-08-16.)
- **Plugins:** reusable skills, MCP servers, and optional UI for ChatGPT/Codex. ([Plugins](https://learn.chatgpt.com/docs/plugins), accessed 2026-08-16.)
- **GPT custom actions or connected apps:** a GPT can use approved connected apps or custom actions; the reviewed workspace documentation says the two are not combined in one GPT. ([GPTs and Sharing](https://learn.chatgpt.com/docs/enterprise/gpts-and-sharing), accessed 2026-08-16.)

OpenAI’s MCP guidance describes Streamable HTTP, OAuth, public HTTPS reachability/proxy patterns, logging, and reliability. Its authentication guidance documents an end-user OAuth boundary rather than employee-supplied machine credentials. ([Build an MCP server](https://developers.openai.com/plugins/build/mcp-server), [MCP authentication](https://developers.openai.com/plugins/build/auth), accessed 2026-08-16.)

#### Recommended route

Offer one named MCP tool or GPT action such as “run approved private classification” or “draft using the firm’s approved private model.” Keep the operation narrow, return a request ID and provenance, and show that ChatGPT remains the host. Do not call it private ChatGPT, an Alzette-powered ChatGPT model, or a replacement for the ChatGPT backend.

If the customer’s requirement is “the entire prompt, context, inference, and retention boundary must be Alzette-controlled,” the supported answer is a companion or a customer-owned application—not a ChatGPT connector. A ChatGPT MCP call also needs a reviewed public reachability/proxy and OAuth design; a private network alone is not evidence of accessibility from ChatGPT’s execution environment.

### Google Workspace

Google’s built-in Workspace Gemini experience is integrated into Gmail, Docs, Meet, Sheets, Slides, Drive, and Chat, with admin controls over access to Workspace data. The reviewed public documentation does not provide an arbitrary model endpoint or base-URL override for that native surface. ([Workspace with Gemini](https://knowledge.workspace.google.com/admin/generative-ai/workspace-with-gemini/google-workspace-with-gemini), [Control Workspace Intelligence](https://knowledge.workspace.google.com/admin/generative-ai/workspace-with-gemini/control-workspace-intelligence), accessed 2026-08-16.) Classify direct replacement as **Not viable on the reviewed public surface**, while retaining **Unknown** for future/plan-specific features.

Workspace add-ons can run in Gmail, Calendar, Chat, Docs, Drive, Meet, Sheets, and Slides. Google documents Apps Script and HTTP add-ons that call external APIs; authorization scopes, OAuth verification, and user/admin consent remain material. ([Build Workspace add-ons](https://developers.google.com/workspace/add-ons/how-tos/building-workspace-addons), [Extend Workspace](https://developers.google.com/workspace/extend), [Apps Script external APIs](https://developers.google.com/apps-script/guides/services/external), accessed 2026-08-16.) Google’s Workspace add-on sample demonstrates a multi-app agent/add-on pattern, but a sample or MCP connection is not a Gemini model swap. ([Workspace add-on sample](https://developers.google.com/workspace/add-ons/samples/travel-concierge), accessed 2026-08-16.)

#### Recommended route

Build or configure one customer-owned add-on/Chat app that calls an Alzette adapter over HTTPS. Start with narrow scopes and one selected operation, such as extracting a redacted invoice into a review card or preparing a source-linked meeting brief. The add-on should identify what context is sent to Alzette, require confirmation for writes, and respect user/admin OAuth.

The customer must accept that the Google add-on/Apps Script execution path and Google Workspace data remain part of the route. Exact execution location, logging, retention, OAuth verification, and third-party service processing need a tenant test and contract review.

### Developer tools and Windows usability

VS Code’s current Custom Endpoint provider supports Chat Completions, Responses, and Anthropic Messages APIs, explicit endpoint URLs, model capability metadata, and custom headers. This is a genuine **Direct endpoint** surface for customer-controlled developer environments. ([AI language models in VS Code](https://code.visualstudio.com/docs/agent-customization/language-models), accessed 2026-08-16.) It still leaves key custody, SSO, policy, extension permissions, code context, and audit responsibility to the customer or an enterprise proxy.

GitHub documents two BYOK paths. Local BYOK is client-side and can be disabled by enterprise policy. **Enterprise BYOK** is server-side, applies to Copilot Chat, CLI, and IDEs, supports OpenAI-compatible providers, and is currently public preview. ([Bring your own key for GitHub Copilot](https://docs.github.com/en/copilot/concepts/models/bring-your-own-key), [enable custom models](https://docs.github.com/en/copilot/how-tos/administer-copilot/manage-for-enterprise/enable-custom-models), accessed 2026-08-16.) Copilot CLI’s custom provider also requires tool calling and streaming. ([Copilot CLI BYOK](https://docs.github.com/en/enterprise-cloud%40latest/copilot/how-tos/copilot-cli/customize-copilot/use-byok-models), accessed 2026-08-16.)

Continue remains a useful targeted IDE companion: its official documentation supports self-hosted OpenAI-compatible model configuration, but the reviewed material does not establish enterprise SSO, source ACL synchronisation, or central Alzette governance. ([Continue self-host a model](https://docs.continue.dev/guides/how-to-self-host-a-model), accessed 2026-08-16.)

#### Recommended route

- First test VS Code Custom Endpoint and GitHub Enterprise BYOK with one organisation-managed Alzette route.
- Use a gateway or short-lived enterprise identity; do not distribute personal long-lived API keys to developers.
- Record the exact API shape required by the coding host. The repository’s current buffered/SSE subset may not satisfy an IDE agent that requires streaming, tool calls, Responses semantics, or large context.
- Treat GitHub’s UI, policies, network, and hosted context as part of the route even when enterprise BYOK selects Alzette.

## Representative customer workflows

The table describes adoption routes, not claims that Alzette currently implements each modality or connector. Workflow choices are **inference** until a customer and test dataset validate them.

| Workflow | Existing surface | Endpoint question | Least-disruptive route | Data/action guardrail |
| --- | --- | --- | --- | --- |
| Cash-flow forecast, budget-versus-actuals, or management reporting | Excel, Google Sheets, or an existing finance application | If the workbook/app calls a configurable API, direct endpoint is possible; native Copilot/Gemini model replacement is not evidenced. | **Embedded agent** in M365/Workspace for a selected calculation/explanation, or **Direct endpoint** in customer-owned code. | Use structured outputs, citations, deterministic calculation outside the model, and human review before publishing. |
| Invoice/expense extraction and coding | Outlook/Gmail plus ERP/accounting workflow | A customer-owned intake service can call Alzette; a locked accounting SaaS may not. | **Embedded agent** or **Application transformation** with a review queue. | Narrow OAuth/file scope; no automatic posting or payment without approval and audit. |
| Audit evidence, policy lookup, or client-file review | SharePoint/OneDrive, Google Drive, Gmail, or a knowledge system | Connectors can retrieve permitted context; they do not alter the host model. | **Embedded agent** with user-scoped retrieval, or **Companion** with a permission-aware connector. | Prove ACL inheritance, revocation, source links, indexing/retention, and prompt-injection handling. |
| Email and meeting brief | Outlook/Teams or Gmail/Calendar | Add-ins, agents, MCP, or Workspace add-ons can call an external action; the host/cloud remains in path. | **Embedded agent** for one read/draft workflow. | Default read-only; explicit confirmation before send/create/update. |
| Client-facing report or proposal drafting | Word/PowerPoint/Google Docs/Slides or ChatGPT/Claude | Native suite base model generally cannot be redirected; customer-owned document application can. | **Embedded agent** for context selection or **Companion** for Alzette-only drafting. | Show source and route; retention is not training consent. Model-improvement use requires separate authorisation. |
| Code review, refactoring, or test generation | VS Code, JetBrains, GitHub Copilot CLI/IDE | VS Code and GitHub enterprise BYOK provide direct endpoint paths; other IDE/SaaS routes vary. | **Direct endpoint** or **Application transformation** through an organisation proxy. | Never leak secrets; test tool calling/streaming/context; attribute usage by user and repository. |
| Image generation or brand-asset review | Adobe/creative tool, specialist web UI, or asset management workflow | No universal endpoint override is evidenced; image APIs, file storage, rights, and moderation are separate. | **Companion** or future **Application transformation** around a specialist workflow. | Current Alzette repo does not prove image capability, rights, asset lifecycle, or third-party licensing. |
| Audio transcription/voice and video generation/analysis | Meeting/audio/video tools and asset stores | These are usually different APIs and asynchronous jobs, not text Chat Completions. | **Companion** or customer-owned job adapter if demand is proven. | Define upload, storage, deletion, job status, speaker/biometric handling, rights, and recovery first. |

The sales message should be about the customer’s workflow and control requirement: “keep the employee’s existing tool; route this approved task through the private model lane.” It should not imply that Alzette can make every model, tool, or media format available inside every host.

## Companion-shell evaluation

These are existing projects a customer could self-host or approve as a temporary employee surface. They are not proposed as Alzette product branches. Their feature claims are project capabilities, not Alzette security or contractual commitments.

| Candidate | Arbitrary OpenAI-compatible endpoint | SSO/RBAC | Per-user source permissions / ACL sync | Email, calendar, files | Windows usability | Actions / approvals | Scheduled work | Licence and operations | What still traverses third parties | Growth decision |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| **Onyx** | **Verified:** custom inference provider accepts OpenAI-compatible base URL and model configuration. ([Custom inference provider](https://docs.onyx.app/admins/ai_models/custom_inference_provider), accessed 2026-08-16.) | **Verified:** Google/Okta/Entra OAuth/OIDC/SAML; document/resource RBAC is Enterprise. SCIM is an Enterprise feature. ([Access controls](https://docs.onyx.app/security/architecture/access_controls), [SCIM](https://docs.onyx.app/deployment/authentication/scim), accessed 2026-08-16.) | **Strongest reviewed option:** Enterprise permission-syncing for Google Drive, Gmail, SharePoint, Slack, Salesforce, GitHub, Jira, and others; source data is persistently indexed and sync/prune timing matters. ([Connector overview](https://docs.onyx.app/admins/connectors/overview), accessed 2026-08-16.) | Gmail, Google Drive, SharePoint, Teams, and files are documented; no Google Calendar permission-aware connector was verified in the reviewed source list. | Browser/self-hosted use is the safe assumption. An Onyx desktop app is described as a development/preview build in release notes; do not make it the Windows requirement. ([Release notes](https://docs.onyx.app/changelog), accessed 2026-08-16.) | MCP/OpenAPI actions are documented. Scheduled/event-triggered workflows and approval/reporting controls are marked “Coming Soon”; do not promise them. ([Actions](https://docs.onyx.app/admins/actions/overview), [workflows](https://docs.onyx.app/overview/core_features/workflows), accessed 2026-08-16.) | **Not verified as generally available.** | Core is described as free/open source; Business/Enterprise add SSO/RBAC, permission syncing, usage limits, SCIM, support/SLA and other controls; self-host licensing and current edition terms require legal review. ([Plans](https://docs.onyx.app/admins/billing/overview), [open source statement](https://docs.onyx.app/overview/miscellaneous/open_source_statement), accessed 2026-08-16.) | Self-hosting avoids Onyx Cloud, but connectors fetch/index source data, the chosen LLM endpoint sees prompts/context, and remote MCP/OpenAPI systems see invoked data. Onyx Cloud adds Onyx’s service boundary. | **Adapt/benchmark now** for an enterprise knowledge companion if the customer accepts indexed copies and Enterprise terms. Do not make Onyx the Alzette control plane. |
| **LibreChat** | **Verified:** custom endpoints support arbitrary OpenAI-compatible APIs and base URLs. ([AI endpoints](https://www.librechat.ai/docs/configuration/librechat_yaml/ai_endpoints), [compatibility](https://www.librechat.ai/docs/compatibility), accessed 2026-08-16.) | **Verified:** OAuth/OIDC, SAML, LDAP, local auth, groups, roles, feature permissions, and resource ACLs are documented. Entra group synchronisation is documented for the OIDC path. ([Access control](https://www.librechat.ai/en/docs/features/access_control), [authentication](https://www.librechat.ai/docs/configuration/authentication), accessed 2026-08-16.) | **Good shell ACLs.** Per-user Google Workspace MCP OAuth follows the authorising user’s Google permissions; this is not the same as broad source ACL synchronisation for every connector. ([Google Workspace MCP](https://www.librechat.ai/docs/mcp_servers/google_workspace), accessed 2026-08-16.) | Gmail, Drive, Calendar, People, and Chat via Google’s remote MCP are documented as Developer Preview; files are supported. M365/Outlook/SharePoint parity was not verified. | **Verified:** browser/self-hosted app; official docs say it is not a native Windows app and recommend Docker Desktop on Windows. ([LibreChat docs](https://www.librechat.ai/docs), accessed 2026-08-16.) | OpenAPI Actions and MCP are documented. Fine-grained confirmation/approval behaviour for consequential writes is not established by the reviewed docs; add a customer policy and test. ([Agents](https://www.librechat.ai/docs/features/agents), [MCP](https://www.librechat.ai/docs/features/mcp), accessed 2026-08-16.) | No current scheduled-work feature was verified. | Official repository is MIT. Self-hosting still requires database, secrets, upgrades, OAuth, and tool-policy operations. ([LibreChat LICENSE](https://github.com/danny-avila/LibreChat/blob/main/LICENSE), accessed 2026-08-16.) | Self-hosting avoids a LibreChat cloud dependency; Google remote MCP, any external MCP/action endpoint, the model endpoint, SMTP, and source systems remain third-party boundaries. | **Benchmark now** as a flexible companion; choose only after SSO, ACL, audit, retention, action confirmation, and upgrade tests. |
| **Open WebUI** | **Verified:** connects to any OpenAI-compatible server and can optionally use direct connections that bypass its backend for inference. ([OpenAI-compatible provider](https://docs.openwebui.com/getting-started/quick-start/connect-a-provider/starting-with-openai-compatible/), [Direct Connections](https://docs.openwebui.com/features/chat-conversations/direct-connections/), accessed 2026-08-16.) | **Verified:** SSO/OIDC/LDAP, SCIM, groups, RBAC, and model/resource permissions are documented. ([Authentication and access](https://docs.openwebui.com/features/authentication-access/), [SSO](https://docs.openwebui.com/features/authentication-access/auth/sso/), accessed 2026-08-16.) | Group/model/resource permission controls are documented; source-system ACL synchronisation comparable to Onyx was not verified. | Files, tools, MCP, Functions/Pipelines, and external integrations are possible; permission-aware Gmail/Calendar/SharePoint/Drive connectors were not verified in the reviewed docs. | Browser use on Windows is a reasonable inference from its self-hosted web deployment; native Windows employee packaging was not verified. | Tools, Functions, Pipelines, and Actions are documented; approval semantics and scheduled employee work were not verified. Treat executable pipelines as high-risk code. ([Pipelines](https://docs.openwebui.com/features/extensibility/pipelines/), accessed 2026-08-16.) | Not verified. | Current repository uses a custom Open WebUI licence with branding restrictions and multiple historical licences; do not assume MIT or white-label rights. ([Open WebUI LICENSE](https://github.com/open-webui/open-webui/blob/main/LICENSE), [license notice](https://github.com/open-webui/open-webui/blob/main/LICENSE_NOTICE), accessed 2026-08-16.) | Self-hosting can keep inference and stored chat within the customer deployment, but external MCP/tools and the selected model endpoint remain in path; direct connections change where request logs and policy enforcement occur. | **Use as a small controlled benchmark**, not a default regulated-company recommendation until licence, ACL, direct-connection, audit, and pipeline risks pass review. |
| **Continue** (developer adapter, not a general employee shell) | **Verified:** self-hosted OpenAI-compatible model/base URL configuration. ([Self-host a model](https://docs.continue.dev/guides/how-to-self-host-a-model), accessed 2026-08-16.) | No reviewed evidence of organisation-wide SSO, SCIM, or central role/model governance in the IDE adapter. | Local IDE/project permissions; no source ACL sync evidenced. | Local repository/files and MCP can be used; email/calendar connectors were not verified. | Runs as a VS Code/JetBrains extension, so Windows usability follows the IDE; central policy remains the customer’s responsibility. | MCP/tool support is documented; approvals and schedules are not a central Continue control in the reviewed material. | Not verified. | Project licence, support, and extension release policy must be checked for the exact customer deployment; do not sell it as Alzette support. | Code/context goes to the configured endpoint and any external MCP/tool. | **Adapt later** for a developer team after key custody, code policy, tool semantics, and attribution are proven. |

### Shell recommendation

For a regulated or large professional-services pilot, the order is:

1. **Existing host with direct endpoint:** VS Code Custom Endpoint or GitHub Enterprise BYOK where the customer already owns that policy surface.
2. **Existing host with embedded agent:** Microsoft custom/declarative agent, a Google Workspace add-on, ChatGPT MCP/GPT action, or a Cowork connector when the host cloud boundary is acceptable.
3. **Customer-managed companion benchmark:** Onyx where source ACL synchronisation and enterprise knowledge are central; LibreChat where flexible model/tool breadth matters; Open WebUI only after its current licence and executable-extension risks are accepted.
4. **No shell commitment:** do not select a companion just to make a demo look complete. Prove a customer workflow first.

## Least-disruptive route by client stack

| Client stack | Start with | Escalate to | Do not promise |
| --- | --- | --- | --- |
| Windows + M365/Teams/Outlook/SharePoint | One Copilot Studio action/declarative agent or a custom engine agent if the customer truly needs its own model. | Customer-managed companion or application route for Alzette-only interaction. | A Windows-wide model switch, Copilot base-model replacement, or Luxembourg-only data path. |
| Claude/Cowork | Keep Cowork; add a narrow connector/plugin action. For Office users, qualify Anthropic’s LLM gateway path. | Companion for full Alzette interaction. | Arbitrary Cowork endpoint replacement or local-network-only remote MCP. |
| ChatGPT Work | One MCP tool or GPT custom action with OAuth and a named operation. | Companion or customer application for whole-session Alzette inference. | “Private ChatGPT” or a ChatGPT Work base-model swap. |
| Google Workspace | Workspace add-on/Chat app with narrow OAuth scopes and one action. | Companion or customer application. | Gemini base-model replacement through a connector/add-on. |
| GitHub/VS Code developer stack | VS Code Custom Endpoint or enterprise BYOK, with a governed proxy. | Continue or customer internal developer portal. | Personal long-lived keys, untested tool/streaming parity, or automatic code-data residency. |
| Finance/accounting SaaS | Customer-owned application route or a small adapter around a selected workflow. | Companion or transformation if the SaaS cannot change provider. | A connector as proof of endpoint replacement. |
| Creative suite | Keep existing creative application; use a narrow action or specialist companion. | Customer-owned media/job adapter after rights and modality PoC. | Text endpoint parity for image/audio/video, or automatic ownership of generated assets. |

## Adoption enablement offer

This is a Growth/service motion around the existing Alzette endpoint and control-plane direction, not a new product branch.

### What the client receives

1. **Stack and workflow inventory:** identify the host, tenant/plan, application owner, data classes, model route, write actions, and endpoint authority.
2. **Route classification:** Direct endpoint, Embedded agent, Companion, Application transformation, or Not viable, with the host/cloud boundaries written down.
3. **One integration contract:** model alias, supported wire format, user/session identity, request ID, retention mode, content limits, error semantics, and revoke path.
4. **One narrow pilot:** a redacted or synthetic workflow with baseline quality, latency, reliability, approval, and data-flow measurements.
5. **Decision pack:** keep, expand, transform, or stop; no promise of suite replacement.

### Minimum customer qualification questions

- Which exact product, desktop/web surface, plan, tenant, and region does the employee use?
- Who owns the tenant and who can approve an agent, add-on, connector, endpoint, or enterprise BYOK policy?
- Is the requirement an actual model replacement, a tool/action, retrieval, or a separate private conversation?
- Does the customer own the application code/configuration or is it a locked SaaS?
- Which protocol is required: OpenAI Chat Completions, Responses, Anthropic Messages, native Azure, MCP, REST/OpenAPI, streaming, tool calling, structured output, or async jobs?
- Which context may leave the host, and which parties may retain it? Is a cloud-hosted connector or add-on acceptable?
- How are employees authenticated and revoked? Can the customer provide Entra/Google/IdP context without personal permanent keys?
- Which source permissions must follow the employee? Is indexed copying acceptable, or is real-time retrieval required?
- Which actions can change a record, send a message, publish a file, or create a financial commitment? What approval is required?
- What is the minimum acceptable quality, latency, availability, and cost compared with the incumbent route?
- Are image, audio, video, fine-tuning, or asynchronous jobs actually required, and what are the rights/retention obligations?
- Does the customer require an Alzette-only interaction boundary, or is the incumbent host allowed to remain in the path?

### Qualification outcomes

- **Green — Direct endpoint:** customer controls the base URL and protocol; run replay and security tests.
- **Amber — Embedded agent:** host supports the required agent/action/add-on, but host model or cloud remains; contract the boundary plainly.
- **Amber — Companion:** no direct route, but a customer-approved shell can meet the workflow with SSO and source permissions.
- **Red — Application transformation:** customer owns enough code to change orchestration/protocol, but effort and regression scope must be priced.
- **Red — Not viable:** locked SaaS, unsupported protocol, prohibited data path, or no approval/identity route. Do not sell around the blocker.

## Validation experiments

These experiments are designed to falsify the route before a customer promise. Use synthetic/public data first, then customer-approved redacted data only after the relevant contract and security review.

| ID | Experiment | Pass condition | Evidence to retain |
| --- | --- | --- | --- |
| E1 | Build a host inventory for one target firm: M365, Claude, ChatGPT, Google, IDE, finance, creative. | Every target workflow has an owner, endpoint authority, data class, protocol, identity path, and route classification. | Signed-off route map; no “connector = model swap” assumptions. |
| E2 | Replay one text/function-tool workload against Alzette from VS Code Custom Endpoint and one customer-owned application. | Required model, tool, streaming/buffered semantics, errors, timeouts, request IDs, and key revocation pass without personal permanent keys. | Raw request/response contract, latency/error report, redaction review. |
| E3 | Configure a Microsoft Copilot Studio declarative agent with a narrow REST/MCP action. | Tenant admin can publish, user identity reaches the adapter, the action invokes Alzette, and the UI identifies the host/model boundary. | Tenant settings, consent, data-flow diagram, audit event, revoke test. |
| E4 | Test a Microsoft custom-engine agent or existing Teams bot with an Alzette model route. | Custom model/orchestrator path works in the target tenant and the customer accepts the added hosting/compliance boundary. | Preview/licence status, model route, latency, permissions, failure/recovery evidence. |
| E5 | Test Claude for M365’s LLM gateway route with the exact Alzette adapter. | Add-in accepts the gateway URL/auth, required wire format and tools work, and data/telemetry boundaries are contractually acceptable. | Manifest, network trace, supported API matrix, token rotation, add-in deployment record. |
| E6 | Add one Alzette remote MCP tool to Claude/Cowork and one GPT/MCP action to ChatGPT Work. | OAuth, public reachability/proxy, user scoping, tool result, error state, and host disclosure work; no claim of base-model replacement survives review. | OAuth scopes, network path, host display, request ledger, denial/revoke tests. |
| E7 | Build one Google Workspace add-on/Chat app action. | Narrow OAuth scopes, admin consent, selected Gmail/Drive/Calendar/Docs context, Alzette call, confirmation, and deletion/revocation work. | Manifest/scopes, OAuth verification status, data-flow/retention record, action audit. |
| E8 | Run Onyx, LibreChat, and Open WebUI as customer-managed shells against the same Alzette tenant. | SSO, group/model entitlement, request attribution, key custody, source permissions, export/deletion, upgrade, and degraded endpoint states pass. | Version/licence, configuration, security review, ACL test results, operator runbook. |
| E9 | Run a finance/accounting workflow and a creative workflow side by side with the incumbent. | Agreed quality, latency, approval, audit, rights, and operational thresholds are met; users do not need to understand inference internals. | Blind evaluation, task time, error/override counts, user/admin interviews. |
| E10 | Execute failure tests across each chosen route. | Denied/revoked user, expired token, unavailable endpoint, malformed tool result, stale connector, and partial response fail closed and recover visibly. | Screenshots/logs, incident timeline, customer-visible message, recovery proof. |

### Suggested decision thresholds

Thresholds must be agreed with the customer, but the first gate should require: 100% of test identities denied after revocation; zero cross-user or cross-tenant context in the test corpus; all write actions explicitly confirmed; no secret in client-visible logs; a documented third-party data path; and parity on the required protocol features for the selected host. Quality, latency, and availability thresholds must be workflow-specific rather than invented globally.

## Buyer objections and positioning guardrails

| Objection | Credible answer | Do not say |
| --- | --- | --- |
| “We already pay for Copilot, Claude, ChatGPT, or Gemini.” | “Keep it. Alzette is for the selected workflow where a private model route, dedicated capacity, contractual data custody, or a different model is worth the added control.” | “Alzette replaces the suite.” |
| “Will employees have to learn GPUs?” | “No. The customer selects an approved workflow and capacity requirement; the serving profile remains an operator concern.” This is a future product promise until the capacity recommender is proven. | “Any model runs anywhere with one click.” |
| “Can you put your model inside our software?” | “Sometimes: directly when the application owns the endpoint, through a custom engine/add-on when the host supports it, or via a companion. We will classify the route before quoting it.” | “Every connector changes the underlying model.” |
| “Will our prompts stay private?” | “We will show the route, identity, isolation, retention, and contractual parties. Host/cloud processing may remain.” | “The endpoint URL or Luxembourg geography proves privacy.” |
| “Can you train on our prompts?” | “Only through a separately authorised Model Improvement branch with selected corpus, evaluation, approval, provenance, rollback, and deletion. Retention is not training consent.” | “Retention automatically gives Alzette permission to train.” |
| “Can we use the new model for payments or client communications?” | “Only with explicit action scope, approval, audit, and customer workflow controls.” | “The agent can safely act on its own.” |
| “Can every employee use a key?” | “No personal long-lived provider keys. Use SSO/short-lived user access or customer-managed scoped service identities.” | “Paste an API key into the desktop app.” |
| “Can you serve image, audio, and video too?” | “Only where a tested capability contract, model licence, storage, job lifecycle, and rights policy exist.” | “OpenAI-compatible means all modalities.” |

## Product inputs for accept/reject

These are Growth inputs for Product to accept or reject. They are not PRD requirements in this document.

### Accept as positioning and enablement rules

- **Route classification is mandatory:** every sales or integration request is labelled Direct endpoint, Embedded agent, Companion, Application transformation, or Not viable.
- **Host boundary is visible:** every customer-facing integration names the host model/orchestrator, Alzette model/endpoint, identity provider, data stores, third-party services, and write-action boundary.
- **One narrow adapter contract:** model alias, supported protocol/capabilities, user context, retention mode, request ID, error semantics, audit event, and revoke path are required before an integration is called supported.
- **No employee permanent keys:** use SSO/OAuth/short-lived access or customer-owned service identities; keep application credentials scoped and revocable.
- **Capability flags are evidence-based:** text, tools, streaming, structured output, vision, image, audio, video, and async jobs are separately tested per route/model.
- **Retention and improvement are separate decisions:** customer-controlled none/selected/policy-matched retention does not authorise Model Improvement. Improvement needs separate consent and controls.
- **Pilot existing shells before building:** benchmark native host routes and the three companion candidates against one workflow before proposing any new employee-facing surface.

### Reject or defer

- **Reject suite replacement** as the first sale or product branch.
- **Reject a universal “Alzette model for Copilot/ChatGPT/Gemini” claim** without an official supported path and tenant test.
- **Reject employee personal long-lived API keys** for interactive use.
- **Reject connector-only success criteria** that measure retrieval but not model path, data flow, identity, approval, and revocation.
- **Defer a general media platform** until image/audio/video workloads, rights, storage, and async operations are evidenced.
- **Reject open-source shell RBAC as Alzette isolation:** shell permissions complement but do not replace Alzette tenant, ledger, retention, or provider controls.
- **Reject custom integration for a locked SaaS** unless the customer controls a supported API/application change or is willing to transform/replace the workflow.

### Open Product questions

- Which first customer stack is real: M365, Claude/Cowork, ChatGPT Work, Google Workspace, GitHub/VS Code, or a customer-owned application?
- Is the first objective a true base-model replacement, a private action, a private employee conversation, or customer software migration? These are different sales and technical routes.
- What Alzette endpoint contract will be supported for the first pilot: Chat Completions, Responses, Anthropic Messages, MCP, REST/OpenAPI, or an adapter combination? Which streaming/tool semantics are proven?
- Which identity model can Alzette operate without personal keys, and which party owns the integration secret and revocation?
- Which third-party host/cloud paths are contractually acceptable for the target data class?
- Is a customer-managed companion acceptable, and who patches, operates, supports, and licenses it?
- Which workflow-specific quality, latency, availability, cost, retention, and approval thresholds decide expand/stop?
- Which modality beyond text is a real near-term demand rather than a roadmap assumption?
- Can Alzette currently evidence the private/dedicated provider path and Model Improvement controls, or must the offer remain PoC-only?

## Source register

All external sources below were accessed 2026-08-16. Links are primary product documentation or the official project repository.

### Microsoft

- [Agents for Microsoft 365 Copilot](https://learn.microsoft.com/en-us/microsoft-365/copilot/extensibility/agents-overview) — declarative versus custom-engine agents, models, orchestrators, channels, and hosting.
- [Extend Microsoft 365 Copilot](https://learn.microsoft.com/en-us/microsoft-365/copilot/extensibility/overview) — agents, connectors, Copilot APIs, Retrieval API, and governance.
- [Publish Agents for Microsoft 365 Copilot](https://learn.microsoft.com/en-us/microsoft-365/copilot/extensibility/publish) — organisation catalogue and distribution.
- [Connect an existing MCP server](https://learn.microsoft.com/en-us/microsoft-copilot-studio/mcp-add-existing-server-to-agent) — Streamable HTTP, SSE deprecation, authentication, and Power Platform boundaries.
- [Federated connectors overview](https://learn.microsoft.com/en-us/microsoft-365/copilot/connectors/federated-connectors-overview) — real-time retrieval and connector boundary.

### Anthropic

- [Claude Cowork](https://claude.com/product/cowork) — desktop/web availability, plugins, connectors, controls, and listed provider options.
- [Connectors](https://support.claude.com/en/articles/11176164-use-connectors-to-extend-claude-s-capabilities) — user permission inheritance and connector actions.
- [Custom remote MCP connectors](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp) — Anthropic-cloud connection path and beta status.
- [Cowork approvals](https://support.claude.com/en/articles/13345190-get-started-with-claude-cowork) — manual/automatic/skip action approval modes.
- [Scheduled Cowork tasks](https://support.claude.com/en/articles/13854387-schedule-recurring-tasks-in-claude-cowork) — remote scheduled sessions and connector/file scope.
- [Claude for Microsoft 365 with third-party platforms](https://support.claude.com/en/articles/13945233-use-claude-for-microsoft-365-with-third-party-platforms) — LLM gateway, Bedrock, Vertex, and Foundry paths for Office add-ins.

### OpenAI / ChatGPT Work

- [ChatGPT Work Overview](https://learn.chatgpt.com/docs/enterprise/chatgpt-work-overview) — current Work terminology and cloud/tool boundary.
- [GPTs and Sharing](https://learn.chatgpt.com/docs/enterprise/gpts-and-sharing) — connected apps, custom actions, and workspace/user controls.
- [Plugins](https://learn.chatgpt.com/docs/plugins) — plugin, skill, connector, and MCP surface.
- [Tools, connectors, and MCP](https://developers.openai.com/api/docs/guides/tools-connectors-mcp) — official integration terminology.
- [Build an MCP server](https://developers.openai.com/plugins/build/mcp-server) — remote MCP transport, reachability, reliability, and proxy guidance.
- [MCP authentication](https://developers.openai.com/plugins/build/auth) — user OAuth and client-credential/custom-key limits for ChatGPT MCP.

### Google

- [Build Workspace add-ons](https://developers.google.com/workspace/add-ons/how-tos/building-workspace-addons) — Gmail, Calendar, Chat, Docs, Drive, Meet, Sheets, and Slides surfaces.
- [Extend Google Workspace](https://developers.google.com/workspace/extend) — add-on and Chat app extension model.
- [Apps Script external APIs](https://developers.google.com/apps-script/guides/services/external) — UrlFetch, OAuth, and service-identity options.
- [Workspace with Gemini](https://knowledge.workspace.google.com/admin/generative-ai/workspace-with-gemini/google-workspace-with-gemini) — native Gemini product surface.
- [Control Workspace Intelligence](https://knowledge.workspace.google.com/admin/generative-ai/workspace-with-gemini/control-workspace-intelligence) — admin controls for Workspace data access.
- [Workspace add-on sample](https://developers.google.com/workspace/add-ons/samples/travel-concierge) — multi-app agent/add-on pattern; a sample is not Alzette compatibility evidence.

### Developer tools

- [VS Code AI language models](https://code.visualstudio.com/docs/agent-customization/language-models) — Custom Endpoint provider and supported API types.
- [GitHub Copilot BYOK](https://docs.github.com/en/copilot/concepts/models/bring-your-own-key) — local versus Enterprise BYOK.
- [Enable custom Copilot models](https://docs.github.com/en/copilot/how-tos/administer-copilot/manage-for-enterprise/enable-custom-models) — OpenAI-compatible providers and preview status.
- [Copilot CLI BYOK](https://docs.github.com/en/enterprise-cloud%40latest/copilot/how-tos/copilot-cli/customize-copilot/use-byok-models) — tool-calling/streaming requirements.
- [Continue self-host a model](https://docs.continue.dev/guides/how-to-self-host-a-model) — self-hosted endpoint configuration.

### Companion projects

- [Onyx custom inference provider](https://docs.onyx.app/admins/ai_models/custom_inference_provider), [access controls](https://docs.onyx.app/security/architecture/access_controls), [SCIM](https://docs.onyx.app/deployment/authentication/scim), [connectors](https://docs.onyx.app/admins/connectors/overview), [actions](https://docs.onyx.app/admins/actions/overview), [workflows](https://docs.onyx.app/overview/core_features/workflows), and [plans](https://docs.onyx.app/admins/billing/overview).
- [LibreChat custom endpoints](https://www.librechat.ai/docs/configuration/librechat_yaml/ai_endpoints), [MCP](https://www.librechat.ai/docs/features/mcp), [access control](https://www.librechat.ai/en/docs/features/access_control), [Google Workspace MCP](https://www.librechat.ai/docs/mcp_servers/google_workspace), [Windows/browser deployment](https://www.librechat.ai/docs), and [official MIT licence](https://github.com/danny-avila/LibreChat/blob/main/LICENSE).
- [Open WebUI OpenAI-compatible provider](https://docs.openwebui.com/getting-started/quick-start/connect-a-provider/starting-with-openai-compatible/), [SSO/SCIM/RBAC](https://docs.openwebui.com/features/authentication-access/), [Direct Connections](https://docs.openwebui.com/features/chat-conversations/direct-connections/), [Pipelines](https://docs.openwebui.com/features/extensibility/pipelines/), and [current licence](https://github.com/open-webui/open-webui/blob/main/LICENSE).
- [Onyx release notes](https://docs.onyx.app/changelog) — desktop preview and generic OpenAI-compatible provider release notes; preview material is not a production guarantee.

## Facts not verified and limits

- No customer tenant, Microsoft environment, Claude/Cowork workspace, ChatGPT Work workspace, Google Workspace domain, GitHub Enterprise account, or third-party SaaS was accessed. No account, credential, connector, plugin, add-on, or form was created or submitted.
- No public source reviewed here proves that standard Microsoft 365 Copilot, ChatGPT Work, Claude Cowork, or native Google Workspace Gemini accepts an arbitrary Alzette base URL. Custom-engine, gateway, add-on, action, and BYOK routes are separate and must not be conflated with base-model replacement.
- Anthropic’s Claude-for-M365 gateway documentation does not prove that Alzette’s current endpoint implements the exact gateway protocol, headers, tool semantics, or commercial route.
- Google add-on/Apps Script and remote MCP patterns do not prove the final execution location, retention, OAuth verification, or regulated-data suitability for a particular tenant.
- The reviewed shell documentation does not establish a production-grade, Alzette-compatible combination of SSO, source ACL synchronisation, audit, retention, action approvals, scheduled work, and Windows packaging. Onyx is strongest for permission-synced knowledge but has edition/licence and scheduled-work limits; LibreChat is flexible but needs operational testing; Open WebUI has current licence/branding and executable-extension considerations.
- No source establishes Alzette customer traction, willingness to pay, dedicated capacity, MeluXina access, production SSO, production employee-agent integrations, full modality support, or a private interaction-vault contract. The repository remains implementation/PoC evidence, not market or production proof.
- No source reviewed here supports a blanket claim that retaining customer prompts authorises training or Model Improvement. Any such use remains separately authorised and contractually bounded.
