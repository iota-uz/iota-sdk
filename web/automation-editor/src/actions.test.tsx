import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library'
import { createSignal } from 'solid-js'
import { describe, expect, it } from 'vitest'
import { actionsCatalog } from '../fixture/catalog'
import { describeItem } from './names'
import { dispatchedType, itemOutcomes, nodeSpec } from './graph'
import { NodePanel } from './NodePanel'
import { Palette } from './Palette'
import type { AutomationGraph } from './types'

const generic = (params: Record<string, unknown>): AutomationGraph => ({
  name: 'g',
  run_as: 'owner',
  triggers: [{ id: 't', event: 'crm.client.created' }],
  nodes: [{ id: 'block', type: 'action', params }],
  edges: [{ from: { node: 't', outcome: 'fired' }, to: 'block' }],
})

describe('registered actions', () => {
  it('a generic action node behaves as the action it names: outcomes, title, output', () => {
    const graph = generic({ action: 'core.user.block', args: { user_id: 'event.user_id', reason: 'spam' } })
    const node = graph.nodes[0]!
    expect(dispatchedType(actionsCatalog.nodes.find((n) => n.type === 'action'), node.params)).toBe('action.core.user.block')
    expect(nodeSpec(actionsCatalog, node)?.type).toBe('action.core.user.block')
    expect(itemOutcomes(graph, actionsCatalog, 'block')).toEqual(['done', 'failed'])
    expect(describeItem(graph, actionsCatalog, 'block', 'en').title).toBe('Block user')
    // Without a key (or with an unknown one) it stays the front type.
    expect(nodeSpec(actionsCatalog, { type: 'action', params: {} })?.type).toBe('action')
    expect(nodeSpec(actionsCatalog, { type: 'action', params: { action: 'nope.x' } })?.type).toBe('action')
  })

  it('the palette lists each action under its module, not the generic node; the key finds it', async () => {
    const { container } = render(() => (
      <Palette catalog={actionsCatalog} graph={generic({})} mode={{ kind: 'add' }} locale="en" onPick={() => {}} onClose={() => {}} />
    ))
    const keys = [...container.querySelectorAll('[data-choice]')].map((e) => e.getAttribute('data-choice'))
    expect(keys).toContain('n:action.core.user.block')
    expect(keys).toContain('n:action.crm.chat.send_message')
    expect(keys).not.toContain('n:action')
    expect(container.textContent).toContain('Users and organization')
    const input = container.querySelector<HTMLInputElement>('input[type="search"]')!
    fireEvent.input(input, { target: { value: 'chat.send' } })
    await waitFor(() => expect([...container.querySelectorAll('[data-choice]')].map((e) => e.getAttribute('data-choice'))).toEqual(['n:action.crm.chat.send_message']))
  })

  it('the generic node picks its action from a list and shows that action’s fields; the saved shape stays {action, args}', async () => {
    const [graph, setGraph] = createSignal(generic({ action: 'core.user.block', args: { user_id: 'event.user_id', reason: 'spam' } }))
    const { container } = render(() => (
      <NodePanel graph={graph()} catalog={actionsCatalog} id="block" locale="en" readOnly={false} errors={[]} onChange={(g) => setGraph(g)} onClose={() => {}} />
    ))
    // A picker, not a free-text key; the action's own args, not a raw object.
    const picker = container.querySelector<HTMLInputElement>('[data-param="action"] input[role="combobox"]')!
    expect(picker).toBeTruthy()
    expect(container.querySelector('[data-param="action"]')!.textContent).toContain('Block user · core.user.block')
    expect(container.querySelector('[data-param="args"]')).toBeTruthy()
    expect(container.textContent).toContain('reason')
    fireEvent.focus(picker)
    const option = await screen.findByRole('option', { name: /Unblock user · core\.user\.unblock/ })
    fireEvent.click(option)
    await waitFor(() => expect(graph().nodes[0]).toEqual({ id: 'block', type: 'action', params: { action: 'core.user.unblock', args: { user_id: 'event.user_id' } } }))
  })
})
