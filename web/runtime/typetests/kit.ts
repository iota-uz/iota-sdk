// A miniature developer kit: platform-owned (native) entities, lists and public contracts.
import {
  command, event, field, foreign, nativeEntity, nativeList, operation, query, slotRef, t,
} from '../src/index.ts'

export const User = nativeEntity('core.user', { name: field.text(), email: field.text().optional() }, { key: 'int64' })
export const Client = nativeEntity('crm.client', { name: field.text(), phone: field.text().optional() }, { key: 'uuid' })
export const Clients = nativeList('crm.lists.Clients')

export const RenameUser = foreign(
  operation('core.users.rename', { input: t.object({ user: t.ref(User), name: t.string() }) }),
)
export const MarkClientContacted = foreign(
  operation('crm.clients.markContacted', { input: t.object({ client: t.ref(Client) }), output: t.object({ at: t.datetime() }) }),
)
export const RejectIncoming = foreign(
  command('core.commands.rejectIncoming', {
    atomic: true,
    input: t.object({ client: t.ref(Client), reason: t.string() }),
    output: t.object({ rejected: t.bool() }),
  }),
)
export const SendInvoice = foreign(
  command('core.commands.sendInvoice', { input: t.object({ client: t.ref(Client) }), output: t.object({ number: t.string() }) }),
)
export const ClientCreated = foreign(event('crm.clientCreated.v1', { subject: Client, payload: t.object({ name: t.string() }) }))
export const ClientToolbar = slotRef<{ client: { key: string } }>('crm.slots.clientToolbar')
