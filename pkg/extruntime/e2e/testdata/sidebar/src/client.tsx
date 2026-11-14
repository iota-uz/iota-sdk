// Contributions to slots of the platform (the CRM client sidebar) and of another package (the board).
import { slot } from "@iota-uz/sdk/runtime/client"
import * as board from "@iota-kit/e2e.board"
import * as crm from "@iota-kit/crm"
import * as c from "./contracts"

// Mounted in order: early (-5), profile (10), intern (20, needs a permission).
export const profile = slot(crm.ClientSidebar, (p) => <div data-testid="sidebar-profile">Profile of client {p.clientId}</div>, { order: 10 })
export const early = slot(crm.ClientSidebar, (p) => <div data-testid="sidebar-early">Early {p.clientId}</div>, { order: -5 })
export const internNote = slot(crm.ClientSidebar, (p) => <div data-testid="sidebar-intern">Intern note {p.clientId}</div>, { order: 20, permission: c.Intern })

export const boardNote = slot(board.Widgets, (p) => <section data-testid="board-note">Note on {p.title}</section>, { order: 20 })
