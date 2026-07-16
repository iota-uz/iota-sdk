# Keyed in-place drill trees

`panel.DrillTree` adds precomputed, in-place detail navigation to Lens Pie and
Donut panels. The complete tree is sent with the chart, so moving between
levels is immediate; only an optional leaf action performs a request or page
navigation.

## Stable identity

The initial dataset needs a string ID field. Set it with `IDField`, then use
the same value as the branch `TriggerKey`:

```go
navigate := action.Navigate("/policies?year=2025&quarter=1")

tree := panel.DrillTree{Branches: []panel.DrillBranch{{
    TriggerKey: "earned",
    Label:      "Earned premium",
    Children: []panel.DrillNode{{
        Key:   "year:2025",
        Label: "2025",
        Value: 125_000,
        Children: []panel.DrillNode{{
            Key:    "quarter:2025:q1",
            Label:  "Q1",
            Value:  125_000,
            Action: &navigate,
        }},
    }},
}}}

chart := panel.Pie("premium", "Premium", "premium_breakdown").
    LabelField("segment").
    ValueField("amount").
    IDField("segment_key").
    DrillTree(tree).
    Build()
```

Every initial `premium_breakdown` row must have a unique, nonblank
`segment_key`, and exactly one row must use `earned`. Initial rows without a
matching branch continue to use the panel action, when one is configured.

Keys are identity, not display text. Keep them stable and independent of
locale and ordering. Branch trigger keys must be unique. Node keys must be
unique among siblings, while the same short key may be reused under another
parent because the complete key path identifies the view.

## Tree and action rules

- Branches must have children.
- Node labels are required; values must be finite and nonnegative.
- A node may have children or an action, but not both.
- A leaf without an action is valid and remains informational.
- Leaf actions support `action.Navigate`, `action.HtmxSwap`, and an
  `action.Spec` with `Kind: action.KindEmitEvent`.
- Node actions cannot use row field value sources because detail nodes are not
  dataset rows. Use a concrete URL and literal or variable action values.
- A panel cannot combine `DrillTree` with the Bar-specific `DrillHierarchy`.
