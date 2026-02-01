# Package Architecture - @iotauz/design-tokens

```
┌─────────────────────────────────────────────────────────────────┐
│                    @iotauz/design-tokens                        │
│                         v1.0.0                                  │
└─────────────────────────────────────────────────────────────────┘
                                │
                                │ npm install
                                ▼
                    ┌──────────────────────┐
                    │    Consumer Project   │
                    │   (Next.js, React,    │
                    │   Vue, etc.)          │
                    └──────────────────────┘
                                │
                                │ @import
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                         index.css                                │
│                      (Main Entry Point)                          │
│  @import "tailwindcss";                                         │
│  @import "./theme.css";                                         │
│  @import "./base.css";                                          │
│  @import "./components.css";                                    │
│  @import "./utilities.css";                                     │
└─────────────────────────────────────────────────────────────────┘
           │              │             │             │
           ▼              ▼             ▼             ▼
     ┌─────────┐   ┌──────────┐  ┌────────────┐  ┌──────────┐
     │ theme   │   │  base    │  │ components │  │utilities │
     │  .css   │   │  .css    │  │   .css     │  │  .css    │
     └─────────┘   └──────────┘  └────────────┘  └──────────┘
           │              │             │             │
           ▼              ▼             │             ▼
     ┌─────────┐   ┌──────────┐        │      ┌──────────────┐
     │ @theme  │   │@font-face│        │      │@layer utils  │
     │ imports │   │:root vars│        │      │ .hide-scroll │
     └─────────┘   │html.dark │        │      │ .no-trans    │
           │       │::selection│        │      │ @keyframes   │
           │       └──────────┘        │      └──────────────┘
           ▼                            │
     ┌─────────┐                       ▼
     │ tokens/ │              ┌──────────────────┐
     │ ├colors │              │@layer components │
     │ ├typo   │              │  .btn variants   │
     │ └space  │              │  .form-control   │
     └─────────┘              │  .dialog         │
                              │  .table          │
                              │  .tab-slider     │
                              │  .sidebar-*      │
                              └──────────────────┘

═══════════════════════════════════════════════════════════════════

Import Flow:

1. Consumer imports:
   @import "@iotauz/design-tokens";

2. index.css imports:
   - tailwindcss (Tailwind v4 core)
   - theme.css (design tokens)
   - base.css (fonts, variables)
   - components.css (UI components)
   - utilities.css (helper classes)

3. theme.css imports:
   - tokens/colors.css (39 color tokens)
   - tokens/typography.css (font families)
   - tokens/spacing.css (spacing tokens)

4. Consumer can extend:
   @theme {
     --color-custom: oklch(50% 0.2 180);
   }

═══════════════════════════════════════════════════════════════════

File Dependencies:

index.css
├── tailwindcss (external)
├── theme.css
│   └── tokens/
│       ├── colors.css
│       ├── typography.css
│       └── spacing.css
├── base.css
├── components.css
└── utilities.css

═══════════════════════════════════════════════════════════════════

Token Flow: Design Tokens → Tailwind Utilities

tokens/colors.css:
  --color-brand-500: oklch(58.73% 0.23 279.66)

↓ (via @theme)

Tailwind generates:
  .bg-brand-500
  .text-brand-500
  .border-brand-500
  .ring-brand-500
  etc.

Consumer uses:
  <div className="bg-brand-500 text-white">...</div>

═══════════════════════════════════════════════════════════════════

Component Architecture:

base.css (:root variables)
         ↓
    --primary-500
    --clr-btn-bg
    --clr-form-control-border
         ↓
components.css (.btn, .form-control)
         ↓
    Consumes :root variables
    oklch(var(--primary-500))
         ↓
Consumer HTML
    <button class="btn btn-primary">

═══════════════════════════════════════════════════════════════════

Package Structure by Purpose:

Design Tokens (What)
├── tokens/colors.css      → Color palette
├── tokens/typography.css  → Font families
└── tokens/spacing.css     → Spacing scale

Theme Layer (How - Tailwind)
└── theme.css              → @theme wrapper

Foundation (Base Styles)
└── base.css               → Fonts, variables, globals

Presentation (Components)
└── components.css         → UI components

Helpers (Utilities)
└── utilities.css          → Helper classes

Integration (Entry)
└── index.css              → Orchestrator

═══════════════════════════════════════════════════════════════════

Module Sizes:

tokens/colors.css      ▓▓░░░░░░░░  2.1 KB (7%)
tokens/typography.css  ░░░░░░░░░░  73 B  (0%)
tokens/spacing.css     ░░░░░░░░░░  168 B (1%)
theme.css              ░░░░░░░░░░  189 B (1%)
base.css               ▓▓▓▓▓░░░░░  9.9 KB (33%)
components.css         ▓▓▓▓▓▓▓▓░░  15 KB (50%)
utilities.css          ▓▓░░░░░░░░  2.4 KB (8%)
index.css              ░░░░░░░░░░  210 B (0%)
                       ──────────
Total:                 ~30 KB

═══════════════════════════════════════════════════════════════════

Usage Patterns:

Pattern 1: Full Import (Recommended)
┌──────────────────────────────────────┐
│ @import "@iotauz/design-tokens";     │
│ @source "./app/**/*.{js,ts,jsx,tsx}";│
└──────────────────────────────────────┘
Result: All features available

Pattern 2: Partial Import (Minimal)
┌──────────────────────────────────────┐
│ @import "tailwindcss";               │
│ @import "@iotauz/design-tokens/      │
│          theme.css";                 │
│ @import "@iotauz/design-tokens/      │
│          base.css";                  │
└──────────────────────────────────────┘
Result: Tokens + base styles only

Pattern 3: Extended (Custom)
┌──────────────────────────────────────┐
│ @import "@iotauz/design-tokens";     │
│ @theme {                             │
│   --color-custom: oklch(...);        │
│ }                                    │
└──────────────────────────────────────┘
Result: All features + custom tokens

═══════════════════════════════════════════════════════════════════

Distribution Model:

┌──────────────┐
│  IOTA SDK    │ (Source)
│  Repository  │
└──────┬───────┘
       │ git push
       ▼
┌──────────────┐
│   GitHub     │ (Version Control)
│  iota-uz/    │
│  iota-sdk    │
└──────┬───────┘
       │ npm publish
       ▼
┌──────────────┐
│  NPM Registry│ (Distribution)
│  @iotauz/    │
│design-tokens │
└──────┬───────┘
       │ npm install
       ▼
┌──────────────┐
│  Consumer    │ (Usage)
│  Projects    │
└──────────────┘

═══════════════════════════════════════════════════════════════════
```
