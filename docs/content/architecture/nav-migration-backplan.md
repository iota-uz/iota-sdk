# Descriptor-Derived Nav Back-Repo Migration Plan

Use this checklist after iota-sdk PR #826 is merged and `back/` bumps its SDK pin to a commit that includes descriptor-derived navigation.

## Preflight

- [ ] Bump `back/` to the SDK commit containing `application.ControllerDescriptor.Nav`, `Descriptor().WithNav(...)`, `application.NavNode`, `application.NavAction`, route auth helpers, and `composition.AddNavNodes`.
- [ ] Run the current `back/` test suite and record a baseline before nav changes.
- [ ] Add a back-specific nav parity harness equivalent to `pkg/composition/nav_parity_test.go`.
- [ ] Generate the baseline golden before removing any legacy `ContributeNavItems` or `AddQuickLinks` registrations.
- [ ] Inventory all `Key()` implementations. Expect roughly 130 controllers to move from `Key()` to `Descriptor()`.
- [ ] Inventory legacy nav registrations in `back/modules/shell_component.go`, `back/modules/list.go`, and each module `links.go`.
- [ ] Inventory per-module spotlight datasources and keep them separate from nav-derived quick links.

## Per-Controller Migration

- [ ] Replace every controller `Key()` with `Descriptor()` using the existing key string as the descriptor ID unless a controller already has a stable descriptor.
- [ ] Add the controller base route to `Descriptor(...)` using `application.Route(...)` or the typed route helpers.
- [ ] Mirror legacy nav permissions only. Do not infer permissions from handler bodies.
- [ ] For each navigable leaf, add `Descriptor().WithNav(application.NavNode{...})`.
- [ ] Reuse the controller's existing base-path constant or field for `NavNode.Path`.
- [ ] Preserve legacy `NavigationItem.Name` as `NavNode.TitleKey`.
- [ ] Preserve legacy `Key`, `Keywords`, `IsBeta`, parent group, and sibling order.
- [ ] Convert old quick-create links such as `X.List.New` into `NavAction`s on the owning leaf.
- [ ] Keep query-string tab variants as separate leaves sharing the same underlying route path.

## Module Nav Registration

- [ ] Convert pure grouping items from module `links.go` files into pathless `application.NavNode`s.
- [ ] Register grouping nodes with `composition.AddNavNodes(builder, ...)`.
- [ ] Remove module `composition.AddNavItems(...)` and `composition.AddQuickLinks(...)` blocks once equivalent descriptor nav is present.
- [ ] Preserve `ContributeNavItems` behavior from `back/modules/shell_component.go` and `back/modules/list.go` by converting those registrations to descriptor grouping nodes or runtime `NavProvider`s as appropriate.
- [ ] Keep per-module spotlight datasource/provider registration intact; only remove quick links that are now generated from descriptor nav.

## Parity Gate

- [ ] After each module, run `GOWORK=off go vet ./...`.
- [ ] Run the back nav parity test and compare structure, href, title, keywords, order, and beta flags.
- [ ] Require actual permissions to be equal or more precise than the golden; regenerate goldens only for deliberate permission tightening.
- [ ] Commit each module separately with `feat(nav): migrate <module> to descriptor-derived nav`.
- [ ] Do not batch unrelated route enforcement hardening into the nav migration.

## Recommended Order

- [ ] `accounting`
- [ ] `edo`
- [ ] `integration`
- [ ] `website`
- [ ] `crm`
- [ ] `reinsurance`
- [ ] `napp`
- [ ] `campaign`
- [ ] `logsys`
- [ ] `reserve`
- [ ] `claim`
- [ ] `underwriting`
- [ ] `reclaim`

## Final Checks

- [ ] Confirm no migrated module still calls `ContributeNavItems`, `AddNavItems`, or `AddQuickLinks` for descriptor-owned nav.
- [ ] Confirm all remaining legacy nav registrations are intentional runtime/dynamic providers.
- [ ] Confirm spotlight results include descriptor-derived leaves and `NavAction`s.
- [ ] Confirm sidebar order matches the baseline golden.
- [ ] File follow-up issues for any enforcement hardening discovered during migration.
