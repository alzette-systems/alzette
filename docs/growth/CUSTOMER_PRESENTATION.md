# Alzette Systems

## Dedicated AI inference for regulated organisations

**Discussion draft — 2026-08-16**

Alzette gives regulated businesses dedicated infrastructure for running approved open-weight AI models, with the performance, access controls, and operating accountability needed for serious workloads.

> Choose a model. Choose who needs it and how fast it must respond. Alzette recommends, deploys, monitors, and bills the right dedicated capacity.

## The problem

Large financial organisations face an uncomfortable choice:

- prohibit AI on sensitive work;
- give employees generic AI tools that may not fit the organisation’s risk and identity;
- build and operate GPU infrastructure internally; or
- accept a broad cloud relationship for a focused workload.

Alzette provides a managed middle path: a controlled inference endpoint behind the applications the organisation already uses.

## What the customer buys

| Customer need | Alzette provides |
| --- | --- |
| A defined operating boundary | Dedicated capacity in an agreed jurisdiction and tenant boundary |
| The right model | A curated set of approved open-weight models and versioned releases |
| Reliable serving | The serving runtime, model configuration, health checks, and performance optimisation |
| Simple administration | Model, employee, role, endpoint, and policy management in one portal |
| A private company AI memory | Customer-controlled custody of approved prompts and outputs, isolated from every other organisation |
| Predictable economics | A monthly price for agreed dedicated capacity, rather than default per-token billing |
| Operational accountability | One provider responsible for deployment, monitoring, changes, and support |

The customer chooses the approved model and desired workload. Alzette handles the infrastructure decisions behind it.

## A simple owner experience

The dashboard owner should not need to understand GPUs, quantisation, batching, or inference runtimes.

1. Select an approved model.
2. Select the teams or employees who need access.
3. Describe the workload and desired responsiveness.
4. Receive a recommended capacity profile and monthly price.
5. Approve deployment.
6. Monitor whether the service remains healthy for the assigned workload.

Employee count is a useful starting point. Alzette also sizes for model, concurrency, context size, workload type, and response-time objectives.

## What Alzette operates

Behind the simple interface, Alzette manages:

- model release and runtime selection;
- hardware and capacity assignment;
- concurrency, queueing, and request protection;
- deployment, health, restart, and rollback procedures;
- endpoint authentication and access boundaries;
- request, latency, error, and capacity evidence;
- agreed maintenance, support, and incident processes.

The customer sees business-ready status such as **Healthy**, **At risk**, or **Operator review required**. Technical diagnostics remain available to IT and engineering teams.

## Your prompts and outputs are valuable company assets

The questions employees ask, the corrections they make, the terminology they
use, and the outputs they approve contain institutional knowledge. Sending all
of that through a generic service without a deliberate custody policy can mean
that the organisation pays for AI while failing to preserve the learning that
could make its own service better.

For a subscribed dedicated service, Alzette can retain approved prompts and
outputs in a private interaction vault for that company. The customer decides:

- which people, applications, projects, or interaction classes are retained;
- who may inspect, export, select, or delete them;
- the permitted purpose, location, and retention period;
- which material may enter a separate model-improvement programme; and
- when data or derived artefacts must be returned or deleted.

Alzette operates the custody layer but does not appropriate the content or use
it across customers. Exact rights, client confidentiality, employee notice,
legal holds, backups, and deletion are defined in the customer agreement.

This is an important part of the subscription: the organisation is not only
renting answers. It is preserving the governed interaction record from which a
more distinctive and useful private AI capability can be built.

This is the target dedicated-service contract. The current PoC does not yet
persist prompt/output content; the vault becomes available only after its
tenant isolation, encryption, access, export, retention/deletion, backup, and
contract gates pass.

## Your organisation’s AI should retain its identity

Generic AI can produce generic language, generic assumptions, and behaviour that does not reflect a firm’s standards. For a highly branded organisation, that can become a quality and reputational risk.

With a dedicated endpoint, the organisation can choose to turn approved employee and client interactions into private improvement data. This creates a governed path to:

- learn the organisation’s terminology and working methods;
- improve tone, consistency, and domain behaviour;
- build a private evaluation set;
- fine-tune approved open-weight models;
- compare and approve new versions; and
- roll back when a change is not good enough.

The organisation controls which retained data may contribute, under its
data-processing and client obligations. Vault retention alone never authorises
training. Learning is separately approved, policy-controlled, versioned,
evaluated, and separate from ordinary operational telemetry.

> Your employees’ expertise becomes governed training capital for your company’s own AI.

## Built for financial-sector control

Each customer engagement defines the actual contractual and technical boundary, including:

- hosting location and data path;
- tenant, network, storage, and credential isolation;
- model version, licence, and approved use;
- prompt/output retention and deletion;
- whether approved interactions may enter an improvement dataset;
- employee and application permissions;
- audit and usage records;
- support, incident handling, maintenance, and service levels.

Human access and application access are separate. Employees use revocable authenticated sessions; unattended applications use managed service credentials. Personal long-lived API keys are not the employee access model.

## Designed for existing systems

Applications connect through a stable, OpenAI-compatible API using familiar HTTP and SDK patterns. The customer’s developers or IT partner can integrate the endpoint without introducing a new employee-facing chat product.

The portal provides the control plane. The customer’s applications and approved workflows remain the user experience.

## Commercial model

The default commercial unit is a **dedicated endpoint capacity plan**:

- one approved model release;
- one validated serving and hardware profile;
- an agreed workload and capacity target;
- a defined jurisdiction and operating boundary; and
- a predictable monthly price.

Usage remains visible for governance and capacity planning, but the primary buying question is not “how many tokens did we consume?” It is “what capacity does this service need, and what does it cost to keep it available?”

For a dedicated subscription, the private interaction vault and governed Model
Improvement branch protect another form of value: the organisation's growing
body of approved AI interactions, evaluation evidence, and differentiated
model behaviour.

Capacity can be expanded behind a stable customer endpoint, subject to the agreed model, location, and contract.

## Why consider this model seriously

Alzette addresses a strategic gap between generic AI subscriptions and internal platform ownership:

- **Control:** the organisation defines the approved model, users, data policy, and learning policy.
- **Performance:** Alzette optimises serving for the selected model and workload.
- **Identity:** the organisation can develop a private model behaviour instead of accepting generic output.
- **Predictability:** dedicated capacity is priced and governed as a monthly service.
- **Accountability:** a focused provider operates the inference layer and owns the operational relationship.
- **Integration:** existing applications call an endpoint rather than asking every employee to adopt another chat tool.

This is particularly relevant to firms whose brand, client confidentiality, and operating standards are valuable assets.

## A controlled first pilot

The first engagement should prove one meaningful workflow, not promise an organisation-wide transformation.

The pilot will define and measure:

- one approved open-weight model and version;
- one customer workflow and agreed user group;
- dedicated endpoint and access boundaries;
- cold-start and steady-state latency;
- throughput, concurrency, errors, and recovery;
- actual allocated or metered infrastructure cost;
- data path, retention, logging, and secret handling;
- prompt-improvement permissions and training-data controls; and
- evaluation results, deployment record, and rollback procedure.

The initial Luxembourg infrastructure path is intended to use MeluXina/LuxProvide. Access, allocation, endpoint operation, economics, support, and contractual terms are confirmed through the technical PoC and pilot agreement rather than assumed from the provider name.

## What Alzette is not

Alzette is not:

- a consumer chatbot;
- a generic employee productivity suite;
- a public model marketplace;
- a raw GPU-rental console; or
- an ungoverned system that silently trains on employee or client work.

It is a managed inference provider for organisations that want their own models, their own operating boundary, and a responsible path to improving AI over time.

## The first conversation

We would start with five questions:

1. Which confidential workflow should AI improve first?
2. Which employees or applications need access?
3. Which data may be processed, retained, or used for model improvement?
4. What responsiveness and concurrent-user level does the workflow require?
5. What evidence would allow security, procurement, and the business owner to approve a pilot?

The outcome is a scoped model, capacity recommendation, operating boundary, pilot plan, and monthly commercial proposal.
