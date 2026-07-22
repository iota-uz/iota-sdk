import { describe, expect, it } from 'vitest'
import fixture from '../../fixtures/explore.json'
import { parseDocument } from '../contract'
import { levelForPath, pathResolves, queryPathForNavigation, resolveDrillPath, withFrameChildren } from './drill'

const document = parseDocument(fixture)

const compositionRoot = [
  'profitability',
  'profitability/operating-margin',
  'profitability/operating-margin/composition',
  'profitability/operating-margin/composition/root',
]
const servicesKey = 'profitability/operating-margin/composition/root/services'
const productsKey = 'profitability/operating-margin/composition/root/products'
const costCentersKey = 'profitability/operating-margin/composition/cost-centers'
const salesKey = 'profitability/operating-margin/composition/cost-centers/sales'
const transactionsKey = 'profitability/operating-margin/composition/transactions'

describe('resolveDrillPath', () => {
  it('resolves a canonical level path to its own level without rewriting it', () => {
    const resolved = resolveDrillPath(document, compositionRoot)
    expect(resolved?.level.path).toEqual(compositionRoot)
    expect(resolved?.queryPath).toEqual(compositionRoot)
  })

  it('resolves a path ending at a point selection to the level its edge opens', () => {
    const path = [...compositionRoot, servicesKey]
    expect(levelForPath(document, path)?.path.at(-1)).toBe(costCentersKey)
    expect(pathResolves(document, path, 'profitability/operating-margin/composition')).toBe(true)
  })

  it('keeps sibling point selections distinct instead of collapsing them onto the node', () => {
    // Both points expand into the same node; the level it renders is
    // parameterised by which one was entered, so their paths must differ.
    const services = queryPathForNavigation(document, [...compositionRoot, servicesKey])
    const products = queryPathForNavigation(document, [...compositionRoot, productsKey])
    expect(services).toEqual([...compositionRoot, servicesKey, costCentersKey])
    expect(products).toEqual([...compositionRoot, productsKey, costCentersKey])
    expect(services).not.toEqual(products)
  })

  it('interleaves every selection with the node it selects into across multiple hops', () => {
    const path = [...compositionRoot, servicesKey, salesKey]
    expect(levelForPath(document, path)?.path.at(-1)).toBe(transactionsKey)
    expect(queryPathForNavigation(document, path)).toEqual([
      ...compositionRoot, servicesKey, costCentersKey, salesKey, transactionsKey,
    ])
  })

  it('does not repeat a branch step whose key already names the node it opens', () => {
    const branchPath = ['profitability', 'profitability/operating-margin']
    const resolved = resolveDrillPath(document, branchPath)
    expect(resolved?.level.path).toEqual(branchPath)
    expect(resolved?.queryPath).toEqual(branchPath)
  })

  it('keeps legacy node-ancestry paths resolvable so stored URLs survive', () => {
    const legacy = [
      'profitability',
      'profitability/operating-margin',
      'profitability/operating-margin/composition',
      costCentersKey,
    ]
    expect(levelForPath(document, legacy)?.path).toEqual(legacy)
    expect(queryPathForNavigation(document, legacy)).toEqual(legacy)
  })

  it('rejects a path whose selection carries no edge', () => {
    const leafPath = [
      'profitability',
      'profitability/operating-margin',
      'profitability/operating-margin/composition',
      transactionsKey,
      'profitability/operating-margin/composition/transactions/TX-1042',
    ]
    // TX-1042 is a record with a leaf action, not a level; the path must not
    // resolve, and the query path passes through for the server to reject.
    expect(levelForPath(document, leafPath)).toBeUndefined()
    expect(queryPathForNavigation(document, leafPath)).toEqual(leafPath)
  })
})

describe('withFrameChildren', () => {
  const dynamicDocument = parseDocument({
    ...fixture,
    drill: {
      inlineDepth: 0,
      edges: {
        root: {
          path: ['root'], label: 'Root', children: [], perspectives: [],
          dynamicChildren: {
            key: { kind: 'field', name: 'id' }, label: { kind: 'field', name: 'label' },
            target: { kind: 'field', name: 'target' },
          },
        },
        detail: { path: ['root', 'detail'], label: 'Detail', children: [], perspectives: [] },
      },
    },
  })
  const frame = {
    columns: [
      { name: 'id', type: 'string' as const },
      { name: 'label', type: 'string' as const },
      { name: 'target', type: 'string' as const },
    ],
    rows: [['year-2025', '2025', 'detail']],
    children: [{ key: 'year-2025', path: ['root', 'year-2025'], label: '2025', target: 'detail' }],
  }

  it('merges resolved children onto the dynamic level', () => {
    const merged = withFrameChildren(dynamicDocument, ['root'], frame)
    expect(merged).not.toBe(dynamicDocument)
    expect(merged.drill.edges['root']?.children).toBe(frame.children)
  })

  it('is an identity no-op once the same frame children are merged', () => {
    // The runtime re-applies the merge whenever the resolved document changes,
    // and the query cache returns the same frame object. Without reference
    // identity here the merge → new document → effect → merge cycle never
    // settles and the render loops forever.
    const merged = withFrameChildren(dynamicDocument, ['root'], frame)
    expect(withFrameChildren(merged, ['root'], frame)).toBe(merged)
  })
})
