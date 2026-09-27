# Semantic border tokens

Border utilities describe purpose, not a palette shade. Components with visible borders must choose a token explicitly; the global border color is only a compatibility fallback.

| Utility | Use |
| --- | --- |
| `border-subtle` | cards, nested containers, ordinary row and section dividers |
| `border-default` | form controls, table frames, interactive containers and sticky dividers |
| `border-strong` | deliberately emphasized or draggable regions |
| `border-brand` | focus, active and selected states |
| `border-danger`, `border-warning`, `border-success` | status states |
| `border-disabled` | disabled controls, with opacity as an additional signal |

All tokens have explicit light and dark mappings in `styles/tailwind/iota.css`. Consumers may override brand colors, but must not redefine the neutral semantic meanings.

`border-primary` and `--clr-border-primary` are deprecated compatibility aliases. Migrate them by purpose: cards and dividers to `subtle`, controls and tables to `default`, and selected states to `brand`. New production UI must not use `border-gray-*` for cards, controls, tables, dialogs, drawers, or layout dividers.
