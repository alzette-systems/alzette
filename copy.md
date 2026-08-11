# Alzette Systems — Homepage Copy

## Positioning

Alzette Systems provides reliable private inference endpoints for Luxembourg's financial centre and other regulated businesses.

The product is infrastructure: OpenAI-compatible APIs, controlled model access, dedicated capacity, predictable metering, and a direct operational relationship. It is not an employee-facing AI workspace, accounting application, agent marketplace, or compliance consultancy.

## Information architecture

1. Header and navigation
2. Hero
3. The problem
4. The controlled endpoint
5. Capacity options
6. Models and compatibility
7. Onboarding
8. Security and data handling
9. Final call to action
10. Footer

---

## 1. Header and navigation

### Logo

Alzette Systems

### Navigation

- Product
- Security
- Models
- Documentation
- Contact

### Header CTA

Request access

---

## 2. Hero

### Eyebrow

PRIVATE INFERENCE INFRASTRUCTURE

### Headline

Use AI on confidential client work — without giving up control.

### Supporting copy

Private, Luxembourg-hosted AI infrastructure for the financial sector — connected by your IT partner, controlled by your organisation.

### Primary CTA

Discuss a pilot

### Secondary CTA

See how it connects

### Hero fact line

Luxembourg-based · Luxembourg-hosted · Guaranteed contractually

(The full hosting statement lives in the security section; the hero stays light.)

### Responsibility panel (hero visual)

Two columns — caption: "Control stays with you. Operations stay with us."

| Your organisation decides | Alzette operates |
| --- | --- |
| Approved workloads and users | Private inference endpoint |
| Models and retention policy | Capacity and monitoring |
| Budgets and access rules | Incidents and technical support |

The architecture diagram (Application → Alzette Gateway → Approved models → Private GPU capacity) appears in the controlled-endpoint section, where the technical reader engages.

---

## 3. The problem

### Section heading

AI adoption should not require a new risk category.

### Body copy

Teams across Luxembourg's financial centre work with sensitive financial, payroll, tax, corporate, and personal information. They also have repetitive work where modern language and document models can save time.

The usual choices are uncomfortable:

- prohibit AI and lose the productivity benefit;
- let employees choose public tools without a consistent control layer;
- build and operate GPU infrastructure internally;
- accept a large cloud relationship for a relatively small workload.

Alzette provides the missing middle: production-ready private endpoints that your existing tools can call.

### Closing line

No new employee product to roll out. No GPU cluster for your team to operate. Just a controlled inference service behind the applications you already use.

---

## 4. The controlled endpoint

### Section heading

A controlled endpoint, operated locally.

### Supporting copy

Alzette gives your developers and IT partners a stable interface to approved inference capacity — and makes the operational layer visible to the people responsible for your systems. The organisation decides who can call it, which models are available, how long data is retained, and how much can be spent.

### Capability cards

#### Data boundaries — Private and tenant-isolated

Requests are processed through controlled, tenant-isolated infrastructure — separate credentials, quotas, usage records, and policies per organisation. Prompt and response content is not retained by default unless an agreed policy requires it.

#### Integration — OpenAI-compatible API

Use familiar SDKs, streaming responses, structured outputs, embeddings, and standard HTTP tooling. Move an existing integration without rebuilding your application around a new product.

#### Cost control — Budgets and hard caps

Set organisation- and project-level budgets, quotas, rate limits, and hard monthly caps before the first request is made. See tokens and spend by organisation, user, application, or API key.

#### Change control — Stable model releases

Models are versioned, changes are communicated, and release records are kept. Production integrations do not silently move to a different model or behaviour.

#### Operational evidence — Reliability you can inspect

Published service levels, health and status visibility, request, latency, error, and capacity metrics, and usage exports for finance, operations, and internal review.

#### Accountability — Someone responsible

Named technical support, incident procedures, and clear maintenance and change communication — a local provider that understands the cost of an unreliable dependency.

### CTA

Ask for the service specification

### Footnote

Production commitments, retention periods, data location, and model availability are defined in the applicable service agreement and technical documentation.

---

## 5. Capacity options

### Section heading

Start small. Reserve what production needs.

### Supporting copy

Choose the capacity model that matches your workload. Every option uses the same API and operational controls.

### Capacity cards

#### Pilot

For evaluating a model, integration, or internal workload — on shared capacity with a defined usage allowance.

- Limited users and applications
- Defined usage allowance
- Usage and latency reporting
- Short onboarding cycle

CTA: Start a pilot

#### Dedicated

Inference capacity allocated to your organisation and isolated from other tenants — for production workloads that need predictable performance.

- Dedicated inference capacity
- Tenant-specific limits and policies
- Production support
- Agreed service level

CTA: Discuss dedicated capacity

#### Reserved

A fixed minimum capacity allocation, held for your organisation over a contractual term — for teams that need guaranteed availability and predictable budgeting.

- Guaranteed minimum capacity
- Held for a contractual term
- Capacity and spend planning
- Contracted service level

CTA: Plan reserved capacity

### Pricing note

Usage is metered by tokens and other agreed resources. Your account can enforce hard spending limits, and production contracts can include a fixed platform commitment for predictable budgeting.

---

## 6. Models and compatibility

### Section heading

The right model for the workload.

### Supporting copy

Alzette maintains a curated catalogue of open-weight models selected for practical production use. Each model is presented with its intended use, context limits, supported capabilities, version, and operating characteristics.

Choose for the job—not for the marketing leaderboard.

### Model catalogue fields

- Model name and version
- Text, vision, embedding, or other capability
- Context limit
- Structured output and tool-calling support
- Performance profile
- Availability tier
- Input and output pricing
- Change and deprecation policy

### Compatibility note

If your application already uses the OpenAI SDK, integration should require a base URL, an API key, and an approved model identifier—not a rewrite. Tools that cannot point to a custom endpoint can be connected by your IT partner or integrator as part of onboarding.

### Model specialization, on request

For larger workloads, Alzette can evaluate confidential model adaptation using customer-approved data. Training data, model artifacts, access, retention, and deletion are governed by the applicable tenant-isolation and contractual controls. Available following a workload and feasibility review.

### CTA

Request the model catalogue

---

## 7. Onboarding

### Section heading

From first request to production endpoint.

### Steps

#### 1. Define the workload

Tell us what you need to run, who will call it, what data it touches, and what response time and volume you expect.

#### 2. Select the environment

Choose the appropriate model, retention policy, capacity tier, data location, and service level.

#### 3. Connect your application

Receive credentials, endpoint documentation, model access, and a test environment. Use your existing SDK or HTTP client.

#### 4. Validate and launch

Test quality, latency, usage, and failure handling. Move to production with agreed limits and operating procedures.

### Onboarding CTA

Talk to an engineer

---

## 8. Security and data handling

### Section heading

Clear answers for confidential workloads.

### Supporting copy

Alzette is responsible for the infrastructure layer: controlled execution, tenant isolation, retention limits, and documented operations. Your organisation remains responsible for how its applications and employees use AI — we provide the controls, technical documentation, and operational evidence to evaluate the service and operate it responsibly.

### Contractual Luxembourg hosting

Inference workloads are hosted in Luxembourg on high-performance European computing infrastructure, and the hosting location is guaranteed contractually. The service agreement defines what the boundary covers: model execution, prompt and response content, logs and usage records, backups, and administrative access. Subprocessors and support access are documented.

### Answer cards

#### What is retained?

Prompt and response content is not retained by default. Retention, deletion, and logging policies are defined per customer environment and documented in the service agreement.

#### Who can access what?

Credentials, quotas, usage records, and operational policies are isolated per tenant. Administrative and support access is bounded, logged, and documented.

#### What is guaranteed?

Hosting location, service levels, incident procedures, retention periods, and model availability are defined in the applicable service agreement — not in marketing text.

### Security CTA

Request the security pack

### Short disclaimer

Alzette provides infrastructure and operational controls. It does not provide legal, tax, accounting, or regulatory advice.

---

## 9. Final call to action

### Eyebrow

READY TO CONNECT?

### Headline

Give your applications a private inference layer.

### Supporting copy

Tell us what you need to run, how confidential the workload is, and what a reliable production endpoint means for your team. We will propose a focused pilot with clear limits and measurable success criteria.

### Primary CTA

Discuss a pilot

### Secondary CTA

Ask a technical question

### Contact line

Alzette Systems — Luxembourg-based private inference infrastructure for regulated work.

---

## 10. Footer

### Footer navigation

- Product
- Models
- Documentation
- Security
- Contact

### Footer statement

Private inference endpoints for Luxembourg's financial centre and other organisations that need control over sensitive AI workloads.

### Footer contact

hello@alzette.lu

© Alzette Systems. All rights reserved.

