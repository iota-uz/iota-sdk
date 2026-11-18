import { lazy } from "solid-js"
import { navigation, page } from "@iota-uz/sdk/runtime/client"
import * as c from "./contracts"

export const notes = page("e2e.notes.notes", {
  path: "/notes",
  nav: { label: { en: "Notes", ru: "Заметки", uz: "Eslatmalar" }, group: "e2e.notes", icon: "note" },
  permission: c.Write,
  component: lazy(() => import("./NotesScreen")),
})

export const nav = navigation({
  groups: [{ id: "e2e.notes", label: { en: "Notebook", ru: "Блокнот" } }],
})
