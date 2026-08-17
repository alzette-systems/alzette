# Alzette documentation

This directory separates authoritative product requirements from commercial
collateral and delivery evidence. A document's location indicates its primary
owner; it does not make customer-facing claims true without the evidence and
gates defined by the product boundary.

## Product

Owned by Product.

- [`product/PRODUCT.md`](product/PRODUCT.md) — authoritative, technology-neutral
  product feature contract: target users, modules, supported features,
  dependencies, and boundaries.
- [`product/POC_BOUNDARY.md`](product/POC_BOUNDARY.md) — controlling statement
  of what the current proof of concept does and does not prove.
- [`product/DELIVERY_MAP.md`](product/DELIVERY_MAP.md) — product-to-code
  traceability: implementation state, fit, evidence, remaining gaps, and
  delivery sequencing.

## Product requirements

Owned by Product. These documents define workflows, prioritised requirements,
acceptance criteria, dependencies, and release gates.

- [`prd/PORTAL_PRD.md`](prd/PORTAL_PRD.md) — master portal and service PRD.
- [`prd/ENDPOINTS_PRD.md`](prd/ENDPOINTS_PRD.md) — endpoint acquisition and
  lifecycle.
- [`prd/ACCOUNT_ONBOARDING_PRD.md`](prd/ACCOUNT_ONBOARDING_PRD.md) — company
  signup, evaluation, approval, and account lifecycle.
- [`prd/WORKFORCE_AGENT_ACCESS_PRD.md`](prd/WORKFORCE_AGENT_ACCESS_PRD.md) —
  invited employee and agent access.

## Growth

Owned by Growth. These documents translate the product into market evidence,
positioning, presentations, and speeches. They must remain consistent with the
product contract and must not create unapproved capability or partnership
claims.

- [`growth/CUSTOMER_PRESENTATION.md`](growth/CUSTOMER_PRESENTATION.md)
- [`growth/MELUXINA_AI_SPEECH.md`](growth/MELUXINA_AI_SPEECH.md)
- [`growth/MARKET_FIT.md`](growth/MARKET_FIT.md)
- [`growth/CLIENT_SOFTWARE_MIGRATION_FIT.md`](growth/CLIENT_SOFTWARE_MIGRATION_FIT.md)
  — evidence brief for migrating customer-owned applications to an Alzette
  inference boundary.
- [`growth/CORPORATE_AI_RUNTIME_FIT.md`](growth/CORPORATE_AI_RUNTIME_FIT.md)
  — integration guidance for employee-facing software and existing workplace
  runtimes.
- [`growth/copy.md`](growth/copy.md)

## Assurance

Owned independently by QA and Review.

- [`assurance/POC_TEST_PLAN.md`](assurance/POC_TEST_PLAN.md)
- [`assurance/QA_REPORT.md`](assurance/QA_REPORT.md)

## Authority and annotations

For current capability claims, `product/POC_BOUNDARY.md` controls. For target
product scope, `product/PRODUCT.md` controls. Focused PRDs translate that
contract into delivery requirements and acceptance criteria; they cannot
silently expand it. Growth copy cannot expand the product boundary, and
assurance documents report evidence rather than define product scope.

Make proposed edits directly when the intended wording is known. For a private
inline review note that should not render, use:

```md
<!-- Founder note: explain the requested change or open decision here. -->
```

Keep unresolved product decisions in the relevant PRD rather than in growth
collateral. Once resolved, update dependent growth and assurance documents as
needed.
