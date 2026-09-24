---
name: ui-design-system
description: Apply the approved navy/Plus Jakarta visual language and Energy Management semantic tokens to every user-facing change, validated against the canonical HTML reference.
---

# UI Design System

## Use When

Creating or significantly modifying any user-facing UI: pages, shell/navigation, components, forms, tables, badges, alerts, charts, states, theming, or responsive behavior.

## Read First

1. `AGENTS.md`
2. `.agents/context/project-context.md`
3. `docs/design/design-system.md` (implementation contract)
4. `docs/design/reference/design-system-v3.html` (canonical visual reference — open it in a browser)
5. The relevant product requirements in `docs/product/`
6. The active phase document

## Responsibilities

Compose screens from semantic tokens and existing primitives so the product matches the reference visual language while expressing Energy Management meaning (normal, informational, warning, critical, data-quality).

## Required Rules

- Do not invent a second design system or replace the approved language with a default framework theme.
- Use semantic design tokens only; no random hex colors or raw reference values in components.
- Reuse primitives before creating variants; add a shared component only when it is reused.
- Keep severity, anomaly type, and destructive actions as separate concepts; never use destructive/danger styling merely for importance.
- Color is never the only carrier of meaning (text, icon, pattern, or value accompany it).
- Support loading, empty, recoverable error (with retry), and disabled states.
- Support hover, focus, active, and disabled interaction states; focus indicators always visible.
- Keyboard navigation remains fully usable; forms have explicit labels and nearby validation messages.
- Tables and charts have responsive behavior; small screens get explicit navigation.
- Charts use the token-based ECharts theme and integrate visually with the rest of the UI.
- Keep data density readable; visual polish must not hide poor information hierarchy.

## Prohibited

Treating the reference's illustrative values as real data or requirements; adding navigation items or modules the product does not require; competing palettes or a second component library; gradients other than the navy sidebar; heavy shadows; gratuitous motion; inconsistent status colors; success/danger for consumption up/down.

## Completion Checklist

- Tokens and semantic mapping from the design contract applied.
- Visual validation done at ~1440, ~768, and ~390 px: hierarchy, typography, spacing, surfaces, forms, buttons, tables, charts, navigation, states, no horizontal overflow.
- Implementation compared side by side with the HTML reference; notable deviations justified in the phase document.
- Component tests cover states and accessibility basics; Playwright screenshots attached as evidence when useful.
