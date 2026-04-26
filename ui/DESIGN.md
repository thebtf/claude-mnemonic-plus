---
version: alpha
name: Engram
description: Persistent memory infrastructure for AI coding agents. A trusted senior colleague that always knows the answer.
colors:
  primary: "#EE7410"
  primary-action: "#B85400"
  on-primary: "#FFFFFF"
  secondary: "#71717A"
  on-secondary: "#FAFAFA"
  tertiary: "#DC2626"
  on-tertiary: "#FFFFFF"
  neutral: "#FAFAFA"
  on-neutral: "#09090B"
  surface: "#FFFFFF"
  on-surface: "#09090B"
  surface-dim: "#F4F4F5"
  on-surface-variant: "#71717A"
  outline: "#E4E4E7"
  outline-variant: "#D4D4D8"
  error: "#DC2626"
  on-error: "#FFFFFF"
typography:
  headline-lg:
    fontFamily: Inter
    fontSize: 36px
    fontWeight: 700
    lineHeight: 1.1
    letterSpacing: -0.025em
  headline-md:
    fontFamily: Inter
    fontSize: 24px
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: -0.02em
  headline-sm:
    fontFamily: Inter
    fontSize: 20px
    fontWeight: 600
    lineHeight: 1.3
  body-lg:
    fontFamily: Inter
    fontSize: 18px
    fontWeight: 400
    lineHeight: 1.6
  body-md:
    fontFamily: Inter
    fontSize: 16px
    fontWeight: 400
    lineHeight: 1.5
  body-sm:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.5
  label-lg:
    fontFamily: Inter
    fontSize: 16px
    fontWeight: 500
    lineHeight: 1.4
  label-md:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: 500
    lineHeight: 1.4
  label-sm:
    fontFamily: Inter
    fontSize: 12px
    fontWeight: 500
    lineHeight: 1.4
    letterSpacing: 0.025em
  mono-md:
    fontFamily: JetBrains Mono
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.6
  mono-sm:
    fontFamily: JetBrains Mono
    fontSize: 12px
    fontWeight: 400
    lineHeight: 1.5
rounded:
  none: 0px
  sm: 6px
  md: 8px
  lg: 12px
  xl: 16px
  full: 9999px
spacing:
  xs: 4px
  sm: 8px
  md: 16px
  lg: 24px
  xl: 32px
  2xl: 48px
  sidebar-expanded: 256px
  sidebar-collapsed: 48px
  content-max-width: 1280px
components:
  button-primary:
    backgroundColor: "{colors.primary-action}"
    textColor: "{colors.on-primary}"
    typography: "{typography.label-md}"
    rounded: "{rounded.md}"
    height: 36px
    padding: 0 16px
  button-primary-hover:
    backgroundColor: "{colors.primary}"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.on-surface}"
    typography: "{typography.label-md}"
    rounded: "{rounded.md}"
    height: 36px
    padding: 0 16px
  button-secondary-hover:
    backgroundColor: "{colors.surface-dim}"
  button-ghost:
    backgroundColor: transparent
    textColor: "{colors.secondary}"
    typography: "{typography.label-md}"
    rounded: "{rounded.md}"
    height: 36px
  button-ghost-hover:
    backgroundColor: "{colors.surface-dim}"
    textColor: "{colors.on-surface}"
  button-destructive:
    backgroundColor: "{colors.tertiary}"
    textColor: "{colors.on-tertiary}"
    typography: "{typography.label-md}"
    rounded: "{rounded.md}"
    height: 36px
    padding: 0 16px
  card-standard:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.on-surface}"
    rounded: "{rounded.md}"
    padding: "{spacing.lg}"
  card-neutral:
    backgroundColor: "{colors.neutral}"
    textColor: "{colors.on-neutral}"
    rounded: "{rounded.md}"
    padding: "{spacing.lg}"
  badge-default:
    backgroundColor: "{colors.surface-dim}"
    textColor: "{colors.on-surface}"
    typography: "{typography.label-sm}"
    rounded: "{rounded.full}"
    padding: 2px 10px
  badge-primary:
    backgroundColor: "{colors.primary-action}"
    textColor: "{colors.on-primary}"
    typography: "{typography.label-sm}"
    rounded: "{rounded.full}"
    padding: 2px 10px
  badge-secondary:
    backgroundColor: "{colors.outline-variant}"
    textColor: "{colors.on-surface}"
    typography: "{typography.label-sm}"
    rounded: "{rounded.full}"
    padding: 2px 10px
  input-field:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.on-surface}"
    typography: "{typography.body-md}"
    rounded: "{rounded.md}"
    height: 36px
    padding: 0 12px
  input-field-focus:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.on-surface}"
  sidebar-container:
    backgroundColor: "{colors.neutral}"
    textColor: "{colors.on-neutral}"
  sidebar-item:
    textColor: "{colors.secondary}"
    typography: "{typography.label-md}"
    rounded: "{rounded.md}"
    padding: 8px 12px
  sidebar-item-active:
    textColor: "{colors.primary}"
  caption:
    textColor: "{colors.on-secondary}"
    typography: "{typography.mono-sm}"
  code-block:
    backgroundColor: "{colors.surface-dim}"
    textColor: "{colors.on-surface}"
    typography: "{typography.mono-md}"
    rounded: "{rounded.md}"
    padding: "{spacing.md}"
  table-row:
    textColor: "{colors.on-surface}"
    typography: "{typography.body-sm}"
    padding: 12px 16px
  table-header:
    textColor: "{colors.on-surface-variant}"
    typography: "{typography.label-sm}"
    padding: 8px 16px
  table-border:
    backgroundColor: "{colors.outline}"
  dialog-overlay:
    backgroundColor: "{colors.on-surface}"
  error-message:
    textColor: "{colors.error}"
    typography: "{typography.body-sm}"
  error-badge:
    backgroundColor: "{colors.error}"
    textColor: "{colors.on-error}"
    typography: "{typography.label-sm}"
    rounded: "{rounded.full}"
---

# Engram Design System

## Overview

Engram is persistent memory infrastructure for AI coding agents. The visual identity reflects a trusted senior colleague — understated, reliable, with hidden depth. The UI prioritizes clarity over decoration, information density over whitespace waste, and functional beauty over aesthetic indulgence.

The personality is **Calm Competence**: no flashy gradients, no playful illustrations, no trendy glassmorphism. Instead, precise borders, restrained color, and typographic hierarchy do all the work. The emotional response should be confidence and trust — the user is looking at infrastructure that remembers everything.

The design supports both light and dark color modes. Light mode is the default, evoking professionalism and openness. Dark mode is available for low-light environments, using the same information hierarchy with inverted contrast. The primary brand color (burnt orange #EE7410) is used sparingly — only for the single most important action per screen and active navigation states. Everything else uses the neutral zinc palette.

## Colors

The palette is built on zinc neutrals with a single warm accent. The strategy is monochromatic restraint with one decisive pop of color.

- **Primary (#EE7410):** Burnt orange — used exclusively for primary actions, active navigation states, and brand identity. Never used for backgrounds or large surfaces. Evokes warmth and reliability.
- **On-Primary (#FFFFFF):** White text on primary surfaces for maximum contrast.
- **Secondary (#71717A):** Zinc-500 — the workhorse neutral for secondary text, metadata, captions, timestamps, and non-interactive borders.
- **Tertiary / Destructive (#EF4444):** Red reserved strictly for delete actions, error states, and critical severity badges. No other use.
- **Neutral (#FAFAFA):** Near-white background in light mode. Clean, recessive — content stands forward against it.
- **Surface (#FFFFFF):** Pure white for cards, dialogs, and elevated content. Combined with a 1px zinc-200 border to create visual layers without shadows.
- **Surface-Dim (#F4F4F5):** Zinc-100 for hover states, code blocks, muted backgrounds, and secondary UI elements.
- **Outline (#E4E4E7):** Zinc-200 border color for cards, inputs, table rows, and separators.

In dark mode, the palette inverts around the zinc scale: background becomes #09090B (zinc-950), surface becomes #18181B (zinc-900), and borders use #27272A (zinc-800). The primary orange stays identical in both modes.

## Typography

Two typefaces serve complementary roles with clear separation of duties.

- **Inter** — the primary typeface for all UI text. Clean, neutral, optimized for screens at all sizes. Used for headlines (Semi-Bold to Bold with negative letter-spacing for density), body text (Regular), and labels (Medium with slight positive letter-spacing for readability at small sizes).
- **JetBrains Mono** — strictly for machine-generated content: code snippets, terminal output, API responses, credential names, session IDs, and hash prefixes. Never for UI labels or prose.

The type scale uses a limited set of sizes (12, 14, 16, 18, 20, 24, 36px) to maintain visual consistency. Body text minimum is 14px; nothing smaller appears as readable content.

## Layout

The layout follows a sidebar-plus-content pattern optimized for dashboard workflows.

- **Sidebar:** Fixed left rail, 256px expanded with text labels, 48px collapsed (icon-only). On mobile (<768px) the sidebar converts to a Sheet overlay triggered by a hamburger button.
- **Main content:** Fluid width within the SidebarInset container, max-width 1280px for readability, centered with 24px horizontal padding.
- **Cards:** 8px border radius, 1px border (`outline` color), 24px inner padding. No shadows — hierarchy comes from border and background contrast only.
- **Stat cards:** 2-column grid on mobile, 4-column on desktop. Internal padding 16px.
- **Tables:** Full-width within cards, 16px cell padding, alternating row tints in dark mode.

Spacing follows a strict 4px base unit: 4, 8, 16, 24, 32, 48. No arbitrary values.

## Elevation & Depth

Depth is achieved through **Border Layering** and **Background Contrast**, not shadows. The visual hierarchy is flat and architectural.

1. **Base Layer:** The neutral background (#FAFAFA light / #09090B dark) serves as the ground plane.
2. **Surface Layer:** White cards (#FFFFFF light / #18181B dark) sit on the base with a 1px outline border to define their edges.
3. **Elevated Layer:** Dialogs and popovers use backdrop-filter blur (8px) on a semi-transparent overlay, creating depth through focus separation rather than drop shadows.

Shadows are prohibited. If two elements need visual separation, increase the spacing token or change the background tone. The matte, borderline-editorial aesthetic depends on this discipline.

## Shapes

The shape language is defined by **Functional Minimalism**. Corner radii are small and consistent — just enough softness to feel modern while maintaining a clean, engineered look.

- **Cards, inputs, dialogs:** 8px corner radius (`rounded.md`) — the default for all rectangular containers.
- **Buttons:** 8px corner radius to match cards. Primary and secondary share the same radius.
- **Badges and pills:** Full radius (9999px) for status indicators and filter pills — the only round elements in the system.
- **Sidebar items:** 8px radius on hover/active states.

No mixing of sharp and round corners within the same component. The 6px (`rounded.sm`) variant exists for small elements (tooltips, mini badges) but is rarely used.

## Components

All UI components are built from shadcn-vue (Radix Vue primitives). Custom components are prohibited unless shadcn has no equivalent. Every component follows the token system — no hardcoded colors or arbitrary values.

### Buttons

Four variants with strict hierarchy:
- **Primary:** Orange fill (#EE7410) + white text. Reserved for the single most important action per screen.
- **Secondary:** White fill + zinc border + dark text. For "confirm" actions and secondary CTAs.
- **Ghost:** Transparent background, zinc text. For toolbar actions, close buttons, and inline controls. Hover state uses surface-dim background.
- **Destructive:** Red fill (#EF4444) + white text. Only for delete/revoke actions, always behind a confirmation dialog.

Height is 36px. Small variant (28px) for inline table actions. No icon-only buttons without aria-label.

### Cards

White surface with 1px zinc-200 border, 24px internal padding, 8px radius. Cards never have shadows. Hover state: subtle border color shift to zinc-300. Cards are the primary grouping container — stats, credentials, issues, and health indicators all live in cards.

### Tables

Full-width within their parent card. Header row uses label-sm typography in muted-foreground color. Body rows use body-sm in standard foreground. Row borders are 1px zinc-100 (light) or zinc-800 (dark). No striped rows in light mode; subtle alternating tint in dark mode.

### Badges

Pill-shaped (full radius). Default: surface-dim background + on-surface text. Status variants:
- Critical/High: red background + white text
- Medium: orange background + white text (using primary)
- Low/Info: zinc background + zinc text (default)

### Inputs

36px height, 8px radius, 1px border. Focus state: border shifts to primary orange + ring shadow. Label text sits above the input using label-sm typography. Helper/error text below uses body-sm in muted or destructive color.

### Sidebar

Fixed left with collapsible state. Active item: primary orange text with no background fill. Inactive items: on-surface-variant color. Footer section: theme toggle icon, connection status dot, logout. Logo at top is a router-link to home.

## Do's and Don'ts

- Do use the primary orange only for the single most important action per screen
- Do maintain WCAG AA contrast ratios (4.5:1 for normal text, 3:1 for large text)
- Do use Inter for all UI text — never introduce additional sans-serif fonts
- Do use JetBrains Mono only for machine-generated content (code, IDs, hashes)
- Do use consistent 8px border radius on cards, inputs, and buttons
- Do test both light and dark modes before shipping any UI change
- Don't use shadows for elevation — use borders and background contrast
- Don't use more than two font weights in a single screen section
- Don't use the primary orange for backgrounds, large surfaces, or decorative elements
- Don't hardcode hex colors — always reference CSS variables or design tokens
- Don't add decorative elements (illustrations, gradients, patterns) that don't convey information
- Don't mix rounded and sharp corners within the same component
- Don't use emoji as icons — use Lucide SVG icons exclusively
