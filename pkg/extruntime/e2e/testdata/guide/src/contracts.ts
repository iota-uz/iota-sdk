// A package that adds a note field to CRM clients, nothing else: whoever may read clients sees the
// package (and so its documentation); anyone else does not.
import { field, overlay } from "@iota-uz/sdk/runtime"
import * as crm from "@iota-kit/crm"

export const ClientNote = overlay("e2e.guide.clientNote", crm.Client, {
  note: field.text().optional(),
})
