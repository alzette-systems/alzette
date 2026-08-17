---
name: Alzette Scoped Control Room
description: A quiet, provenance-first client portal for scoped inference operations.
colors:
  primary: "#0d9e63"
  primary-deep: "#087c4e"
  neutral-bg: "#faf9f6"
  neutral-paper: "#f2f0ea"
  neutral-surface: "#ffffff"
  ink: "#10151a"
  dim: "#4f5b64"
  faint: "#6b747c"
  line: "#d9ddd8"
  dark: "#0d1114"
  dark-raised: "#151d22"
  dark-line: "#2a363e"
  dark-dim: "#b7c2c9"
  warning: "#8b5b17"
  warning-bg: "#fff4de"
  danger: "#9f3034"
  danger-bg: "#fff0ef"
typography:
  display:
    fontFamily: "-apple-system, BlinkMacSystemFont, Inter, Segoe UI, Roboto, Helvetica, Arial, sans-serif"
    fontSize: "clamp(42px, 5vw, 72px)"
    fontWeight: 640
    lineHeight: 0.98
    letterSpacing: "-0.045em"
  title:
    fontFamily: "-apple-system, BlinkMacSystemFont, Inter, Segoe UI, Roboto, Helvetica, Arial, sans-serif"
    fontSize: "22px"
    fontWeight: 630
    lineHeight: 1.15
    letterSpacing: "-0.025em"
  body:
    fontFamily: "-apple-system, BlinkMacSystemFont, Inter, Segoe UI, Roboto, Helvetica, Arial, sans-serif"
    fontSize: "15px"
    fontWeight: 400
    lineHeight: 1.55
  label:
    fontFamily: "SF Mono, ui-monospace, JetBrains Mono, Menlo, Consolas, monospace"
    fontSize: "10px"
    fontWeight: 650
    lineHeight: 1.35
    letterSpacing: "0.12em"
    textTransform: uppercase
rounded:
  none: "0"
  xs: "2px"
  sm: "4px"
  md: "6px"
spacing:
  xs: "8px"
  sm: "16px"
  md: "24px"
  lg: "32px"
  xl: "48px"
  section: "72px"
components:
  button-primary:
    backgroundColor: "{colors.dark}"
    textColor: "#ffffff"
    typography: "{typography.label}"
    rounded: "{rounded.sm}"
    padding: "9px 14px"
    height: "40px"
  button-secondary:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    rounded: "{rounded.sm}"
    padding: "9px 14px"
    height: "40px"
  portal-panel:
    backgroundColor: "{colors.neutral-surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.sm}"
    padding: "24px 26px"
  route-surface:
    backgroundColor: "{colors.dark}"
    textColor: "#ffffff"
    rounded: "{rounded.sm}"
    padding: "27px 30px"
  text-field:
    backgroundColor: "{colors.neutral-surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.xs}"
    padding: "8px 10px"
    height: "40px"
  status-ribbon:
    backgroundColor: "{colors.neutral-paper}"
    textColor: "{colors.dim}"
    rounded: "{rounded.none}"
    padding: "10px 13px"
---

# Design System: Alzette Scoped Control Room

## Overview

**Creative North Star: "The Scoped Ledger"**

The portal is a calm control room for one authenticated project/environment. It borrows the incumbent Alzette ink, paper, river-green, and mono-data language, but gives the customer a persistent wayfinding rail and separate work areas: Overview, Usage, Routes, Access, and Docs. The visual hierarchy is operational rather than promotional: a dark route surface answers the callability question, a ledger records what is known, and provenance sits beside every important measurement.

The system is deliberately flat-by-default. Thin rules, paper tonal shifts, and the dark route surface create depth without turning every fact into a tile. The green accent is scarce and functional: focus, active navigation, affirmative boundary, and a safe next action. Connected values must come from server-supplied scope and route/service-plan data; static fallback semantics are labelled as fallback and never masquerade as customer facts.

**Key Characteristics:**

- Scoped context is persistent and legible: organisation / project / environment stays in the top bar and rail.
- Provenance is a first-class visual field: source, finality, freshness, and as-of appear near data.
- A route-led hero and ledger rows replace an equal-size KPI-card wall.
- Tables are the durable alternative to charts, and safe metadata excludes prompts, outputs, provider URLs, and secrets.
- Credentials are progressive: service account first, scoped expiring key second, plaintext once in memory.

## Colors

The palette is a high-contrast paper-and-ink field with one river-green signal and restrained amber/red state channels.

### Primary

- **River Green** (#0d9e63): Focus, current navigation, safe action, and affirmative provenance.
- **River Green Deep** (#087c4e): Text links and readable accent text on paper.

### Neutral

- **Warm Paper** (#faf9f6): Primary page canvas and login field.
- **Paper Deep** (#f2f0ea): Context bands, secondary panels, and quiet state surfaces.
- **Clean Surface** (#ffffff): Data panels and form controls.
- **Ink** (#10151a): Primary text, primary actions, and strong type.
- **Graphite** (#4f5b64): Body support copy and secondary values.
- **Faint Graphite** (#6b747c): Labels, timestamps, and unavailable metadata.
- **Rule Grey** (#d9ddd8): Table rules and panel boundaries.
- **Night** (#0d1114): Rail, route hero, code blocks, and primary depth plane.
- **Night Raised** (#151d22): Execution context surface.
- **Night Rule** (#2a363e): Dark-surface dividers.
- **Night Dim** (#b7c2c9): Dark-surface support copy.
- **Quiet Amber** (#8b5b17): LAN notice, stale/partial state, and permission caution.
- **Amber Wash** (#fff4de): Quiet warning background.
- **Quiet Red** (#9f3034): Error and destructive state.
- **Red Wash** (#fff0ef): Error background.

**The One Voice Rule.** River-green should signal a decision or affordance, not decorate every panel. Keep the majority of a screen paper, ink, or rule grey.

## Typography

**Display Font:** System sans stack (`-apple-system`, BlinkMacSystemFont, Inter, Segoe UI, Roboto, Helvetica, Arial, sans-serif)

**Body Font:** The same system sans stack, chosen for dependable LAN and offline rendering.

**Label/Mono Font:** SF Mono / ui-monospace / JetBrains Mono / Menlo / Consolas

**Character:** Sans type carries the human reading layer; mono type marks system state, scopes, IDs, timestamps, and filters. Tight display tracking gives the control room a composed editorial edge without becoming ornamental.

### Hierarchy

- **Display** (640, `clamp(42px, 5vw, 72px)`, `.98): Workspace titles and the first-view decision.
- **Headline** (630, 25–38px, 1.05–1.15): Route decision, next action, and credential explanation.
- **Title** (630, 22px, 1.15): Panel headings and ledger group names.
- **Body** (400, 15px, 1.55): Explanations, boundary copy, and empty-state guidance.
- **Label** (650, 10–12px, `.12em`, uppercase): Eyebrows, fields, source labels, and data vocabulary.

**The Two-Layer Type Rule.** Use sans for what a customer needs to understand; use mono for what a customer needs to verify or copy.

## Layout

The desktop shell uses a 248px persistent dark rail and a fluid content column capped at 1380px. The top bar stays visible while a workspace scrolls. Overview begins with one asymmetrical route/next-action pair, then an execution/service-plan band, then a ledger and provenance panel. Usage favors wide ledger panels and scrollable tables over a uniform card matrix.

At 1160px the rail contracts and the content gutters tighten. At 900px the rail becomes a keyboard-addressable drawer with a compact menu button; content panels collapse to one column. At 640px filters become stacked, tables retain horizontal scroll rather than hiding fields, and dialogs become edge-to-edge within a 15px viewport margin. The intended checkpoints are 1440px, 1024px, and 390px.

Spacing follows an 8px base: 8 / 16 / 24 / 32 / 48px, with 72px used for the large portal breathing zone. Panels are separated by 16px; internal panel padding is 24–26px on desktop and 18px on mobile.

### Public product and documentation surface

The standalone public service extends the same scoped-ledger world into a Persuade surface without pretending to be an authenticated workspace. The landing page presents the finished private Luxembourg offer to financial-industry decision-makers; current gateway/provider status belongs in implementation docs and the client portal. Its decision-maker landing page uses the existing 42–72px display ramp and a 25–48px editorial section-headline ramp; implementation documentation caps section headlines at 38px. Lead copy is 18px, body copy remains 15px, and compact support/navigation text may use 10–14px where it stays above the contrast floor. The landing page uses a maximum 1180px reading shell, while documentation narrows its article to 760px.

Public surfaces keep the committed palette unchanged. The hero’s two-column service charter makes “your organisation decides / Alzette operates” the signature object. The private-endpoint flow, capability ledger, dedicated offer, governance questions, client-visibility ledger, and documentation tables use rules and paper shifts before elevation. Corners use the established 2px/4px/6px scale; only compact status tags use a full capsule because they are small state controls, not content containers.

## Elevation & Depth

Alzette is flat-by-default. Depth comes from the paper / deep-paper / night tonal stack and rules, not floating cards. A restrained dialog shadow is reserved for an interruptive one-time secret or admin form. Hover lift is a one-pixel cue on buttons only, and reduced-motion users receive no meaningful movement.

### Shadow Vocabulary

- **Dialog:** `0 20px 70px rgba(16, 21, 26, .2)` for modal separation from the scoped ledger.
- **Rail drawer:** `12px 0 30px rgba(13, 17, 20, .16)` only while the mobile navigation drawer is open.
- **Toast:** `0 10px 30px rgba(13, 17, 20, .2)` for transient confirmation.

**The Flat Ledger Rule.** If a fact can be separated with a rule or a tonal surface, do that before adding a shadow.

## Shapes

The form language is squared and quiet: 4px panel/button corners, 2px field corners, 1px rules, and no pill-shaped KPI containers. The route hero and execution band are rectangular dark instruments; the state pill is a compact outlined label, not a decorative badge. Focus uses a 3px river-green outline with a 3px offset so keyboard position is never conveyed by color alone.

## Components

### Persistent rail

- **Character:** A dependable workspace index without decorative numbering.
- **Shape:** Dark full-height plane, 2px active left rule, 42px minimum navigation target.
- **States:** Active uses night-raised background and river-green rule; hover changes text and tonal fill; keyboard focus is always visible.

### Route surface

- **Character:** One deliberate answer to “Can I call now?”
- **Shape:** Night background, 4px corners, signal mark, facts separated by night rules.
- **States:** Text labels name unknown, stale, degraded, unavailable, or ready; no state is color-only.

### Ledger rows

- **Character:** A readable record, not a KPI tile.
- **Shape:** Definition-list rows with a top rule, sans labels, mono values, and explicit unknown/unavailable text.
- **Behavior:** Zero is a valid value; unknown token or contract values never become zero.

### Data table

- **Character:** The source of truth for investigation.
- **Shape:** Full-width ruled table with compact mono headers and horizontal scroll on narrow screens.
- **Behavior:** Chart data always has an open table alternative; request rows contain safe metadata only.

### Buttons and fields

- **Character:** Tactile, direct, and low drama.
- **Primary:** Night fill, white text, 40px minimum height, 4px corner.
- **Secondary:** Transparent paper fill, rule border, ink text.
- **Focus:** River-green outline; hover does not rely on color alone.

### Credential dialogs

- **Character:** Progressive disclosure with an explicit boundary.
- **Behavior:** Create service-account name first; choose backend-allowed least-privilege scopes and a required 1-hour–365-day expiry when issuing a key; show plaintext only after a successful issue/rotation in an in-memory one-time panel.

## Do's and Don'ts

### Do:

- **Do** keep Org / Project / Environment visible at the top of every workspace.
- **Do** show source, finality, freshness, and as-of near a value that can be partial or stale.
- **Do** hydrate execution and capacity from the selected route and service plan; ask the customer to select a model when routing is ambiguous.
- **Do** say “No requests were recorded in this period. This is not an error.” for a connected zero ledger.
- **Do** show the LAN PoC notice when the transport is not HTTPS without blocking a gentle sign-in.
- **Do** preserve a normal-link fallback for `/app/overview`, `/app/usage`, `/app/routes`, `/app/access`, and `/app/docs`.

### Don't:

- **Don't** invent throughput, peak concurrency, allowance, dedicated allocation, spend, hosting location, or provider health.
- **Don't** call the human portal password an API key or call an inference API key a password.
- **Don't** place plaintext keys, prompts, outputs, or provider URLs in HTML, JSON exports, localStorage, or sessionStorage.
- **Don't** turn every measure into an equal-size KPI card or hide the accessible table alternative behind a chart-only view.
- **Don't** use decorative section numbering as if it were a required task sequence.
