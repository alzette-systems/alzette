# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Alzette serves regulated organisations in Luxembourg. Company owners and IT
administrators define access to approved model endpoints; invited employees use
those models from supported third-party tools without handling permanent
personal API keys. Developers and integrators also call the same scoped
inference service from applications.

## Product Purpose

Alzette provides managed, customer-scoped model infrastructure with stable
endpoints, explicit shared or dedicated service boundaries, controlled model
access, and metadata-only operational evidence. Alzette Connect extends that
service to employees: it signs a person in, discovers their current company and
model entitlements, configures and launches a qualified AI application through
an authenticated local proxy, and revokes the connection when the session
ends.

The detailed product and implementation contract remains in
[`docs/product/PRODUCT.md`](docs/product/PRODUCT.md). The employee connection
surface is specified in
[`docs/prd/ALZETTE_CONNECT_PRD.md`](docs/prd/ALZETTE_CONNECT_PRD.md).

## Positioning

Control stays with the customer; operations stay with Alzette. The company
decides which people, applications, projects, and approved model aliases may be
used. Alzette Connect is a connection launcher and supervisor, not a new chat
workspace or agent marketplace. Employees keep using qualified tools while
Alzette owns the identity, entitlement, short-credential, route, accounting,
and revocation boundary.

## Operating Context

Owners invite employees and assign model access through groups in the client
portal. An employee opens Alzette Connect, authenticates in the system browser,
selects a company context and supported installed application, then launches
that application. Connect supplies every compatible assigned model when the
adapter supports a catalogue and asks for one model only when the adapter
requires it. Connect remains active while supervising the local
connection and offers an explicit disconnect. Application and protocol support
is versioned and evidenced; detecting an executable does not make it supported.

The Connect launcher is platform-neutral and cross-platform. Shared content
composition remains consistent while window chrome, browser handoff,
credential storage, tray behavior, and close conventions follow the host OS.
Its separate repository now contains the Wails native shell, Go custody core,
and the approved launcher UI. macOS is the named demo target; Windows and Linux
remain internal build targets until their clean-machine acceptance passes.

## Capabilities and Constraints

The current implementation includes the Go `alzette-agent` CLI prototype and a
separate Alzette Connect desktop repository. Connect implements browser PKCE
login, protected refresh storage, group-filtered context/model discovery,
per-launch short human credentials and loopback proxy, Pi 0.84.2 qualification
and isolated launch, reversible Jan 0.8.4 and Goose 1.46.0 configuration,
process supervision, disconnect, recovery state, tray behavior, internal
update checks, and a disabled-by-default reversible macOS ChatGPT candidate
over the bounded Responses path. ChatGPT named-version/native-client acceptance
and Windows Store integration are not yet implemented, and Claude Code is
outside the current Connect scope.
Signed/notarized macOS evidence and Windows/Linux
clean-machine acceptance remain open.

Remote credentials, provider targets, prompts, and outputs must not appear in
Connect UI, arguments, events, logs, diagnostics, or unprotected persistent
state. Every client claim is the intersection of adapter, application version,
OS, protocol, model capability, company policy, and acceptance evidence.

## Brand Commitments

The name is Alzette Systems and the product surface is Alzette Connect. The
river mark in [`alzette-mark.svg`](alzette-mark.svg), the existing scoped-ledger
design language in [`DESIGN.md`](DESIGN.md), and a calm, contract-first,
evidence-over-claims voice are binding. Avoid hype, invented availability,
purple gradients, glassmorphism, productivity scoring, and decorative
infrastructure imagery.

## Evidence on Hand

The running portal and gateway, current PRDs, deterministic gateway and Connect
tests, local Casdoor employee flow, and bounded Pi/Jan/Goose local evidence are
available. No signed/notarized Connect artifact, clean-machine cross-platform
matrix, production remote employee path, accepted ChatGPT native-client run,
customer testimonial, certification, or production MeluXina deployment may be
fabricated in design or copy.

## Product Principles

1. Current entitlement is the source of truth; cached preference is never
   authorization.
2. A person launches a familiar tool without receiving a reusable remote
   credential.
3. Installed is not the same as supported; every readiness state explains its
   evidence and next action.
4. Connect supervises one explicit connection lifecycle and makes cleanup and
   revocation visible.
5. Protocol, hosting, capacity, and privacy claims remain exact and scoped.

## Accessibility & Inclusion

Connect must be keyboard-complete, screen-reader legible, usable at 200% zoom,
compatible with reduced motion and high contrast, and never communicate state
by color alone. The compact desktop composition must remain operable at its
minimum supported window size with internal scrolling rather than hidden
controls.
