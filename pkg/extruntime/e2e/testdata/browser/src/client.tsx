import { For, Show, lazy } from "solid-js"
import { navigation, page, shellRenderer, slot } from "@iota-uz/sdk/runtime/client"
import * as crm from "@iota-kit/crm"
import * as c from "./contracts"
import { probe } from "./probe"

export const lab = page("e2e.browser.lab", {
  path: "/browser/lab",
  nav: { label: { en: "Browser lab", ru: "Лаборатория", uz: "Laboratoriya" }, group: "e2e.browser.menu", icon: "flask" },
  permission: c.Use,
  component: lazy(() => import("./Lab")),
})

// A page on a typed route (`/browser/items/:id`).
export const item = page(c.ItemRoute, { component: lazy(() => import("./Item")) })

// A page whose component chunk the suite makes unreachable.
export const fragile = page("e2e.browser.fragile", {
  path: "/browser/fragile",
  nav: { label: { en: "Fragile page" }, group: "e2e.browser.menu", icon: "bug" },
  component: lazy(() => import("./Fragile")),
})

export const nav = navigation({
  groups: [{ id: "e2e.browser.menu", label: { en: "Lab", ru: "Лаба", uz: "Lab" } }],
  workspaces: [{ id: "e2e.browser.main", label: { en: "Main", ru: "Главная" } }],
  palette: [{ id: "e2e.browser.openLab", label: { en: "Open the lab", ru: "Открыть лабораторию" }, target: "e2e.browser.lab" }],
})

// Contributions to the lab page's own slot: one for everybody, one that needs the audit permission.
export const labNote = slot(c.Panel, (p) => {
  probe("lab-note")
  return <p data-testid="panel-note">Note for {p.title}</p>
}, { order: 10 })
export const labAudit = slot(c.Panel, (p) => {
  probe("lab-audit")
  return <p data-testid="panel-audit">Audit for {p.title}</p>
}, { order: 20, permission: c.Audit })

// Contributions to the CRM client sidebar (a Templ/HTMX page): the probe has no permission, the audit one has.
export const clientProbe = slot(crm.ClientSidebar, (p) => {
  probe("client-probe")
  return <div data-testid="client-probe">Probe for client {p.clientId}</div>
}, { order: 30 })
export const clientAudit = slot(crm.ClientSidebar, (p) => <div data-testid="client-audit">Audit for client {p.clientId}</div>, { order: 40, permission: c.Audit })

// The whole shell: navigation, user, theme and locale controls, and the host-owned outlet.
export const shell = shellRenderer((p) => {
  probe("shell")
  return (
    <div data-testid="lab-shell" data-theme-resolved={p.theme.resolved} data-locale={p.locale}>
      <header>
        <span data-testid="lab-shell-user">{p.user.name}</span>
        <span data-testid="lab-shell-locale">{p.locale}</span>
        <span data-testid="lab-shell-theme">{p.theme.resolved}</span>
        <button type="button" data-testid="lab-shell-light" onClick={() => p.theme.set("light")}>Light</button>
        <button type="button" data-testid="lab-shell-dark" onClick={() => p.theme.set("dark")}>Dark</button>
      </header>
      <nav data-testid="lab-shell-nav">
        <For each={p.nav.groups}>
          {(group) => (
            <section>
              <Show when={group.label}><h2>{group.label}</h2></Show>
              <For each={group.items}>{(i) => <a href={i.href} data-active={i.active ? "true" : "false"}>{i.label}</a>}</For>
            </section>
          )}
        </For>
        <For each={p.nav.palette}>{(entry) => <a data-testid="lab-shell-palette" href={entry.href}>{entry.label}</a>}</For>
      </nav>
      <main>{p.outlet}</main>
    </div>
  )
})
