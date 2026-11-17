# Lens: methodology formulas

A measure (`cube.MeasureSpec.Formula`, JSON `formula`) or a panel
(`spec.PanelSpec.Formula`, `panel.Builder.Formula`) may carry a `formula.Spec`:

- `latex` — the formula. Never shown to readers. Tag a symbol with
  `\htmlData{var=KEY}{…}` to make it hoverable/focusable; `KEY` is `[A-Za-z0-9-]+`.
- `alt` — optional localized plain-language reading (else derived from `latex`).
- `variables[]` — `key`, `label`, `ref` (`measure` | `field` | `stage` + name),
  `source` (sentence shown in the list), `value` (pre-formatted, for pages with no frame).
- `version`, `date` — methodology revision.

**Values.** The client resolves `ref` from the frame the panel already loaded
(first row), formatted with the panel's field formats — no extra request, same
filters/period as the figure.

**Server markup.** `document.Formula.Markup` is MathML rendered in Go for print,
PDF and no-JS; invalid LaTeX never fails a document: `error: true` is set, the
event is logged and the panel shows a message.

**Outside Lens.** `solid.Formula(props)` (templ) emits `<lens-formula>`; the
Lens bundle registers the element (`registerLensFormulaElement`).

**Rendering.** KaTeX lives only in `web/math` (`@iota-uz/sdk-math-internal`),
shared with chat-ui, loaded by dynamic import into its own chunk.

**Print / export.** Print appendix B has «Calculation methodology»; Excel gets a
«Methodology» sheet (formula in words, variables, values, sources, version).
