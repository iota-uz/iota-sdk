import { describe, expect, it } from 'vitest'
import { QueryError } from '../runtime/query'
import { failureReason, panelFailureCopy } from './panelFailure'

/** A host catalogue: keys it has translated win, keys it lacks fall back. */
function translator(messages: Record<string, string>) {
  return (key: string, fallback: string) => messages[key] ?? fallback
}

const russianHost = translator({
  'panel.error': 'Не удалось отобразить панель',
  'panel.error.timeout': 'Данных за выбранный период слишком много',
  'panel.error.canceled': 'Загрузка была прервана',
})

/** A host on an older catalogue: it knows the generic sentence and no more. */
const legacyHost = translator({ 'panel.error': 'Не удалось отобразить панель' })

describe('failureReason', () => {
  it('reads the reason a classified server failure carries', () => {
    expect(failureReason(new QueryError('internal', 'panel execution failed', 200, 'timeout'))).toBe('timeout')
  })

  it('has no reason for an unclassified failure', () => {
    expect(failureReason(new QueryError('internal', 'panel execution failed', 200))).toBeUndefined()
    expect(failureReason(new Error('panel request failed'))).toBeUndefined()
    expect(failureReason(null)).toBeUndefined()
  })

  it('ignores a reason outside the contract vocabulary', () => {
    expect(failureReason({ reason: 'quota_exceeded' })).toBeUndefined()
    expect(failureReason({ reason: 7 })).toBeUndefined()
  })
})

describe('panelFailureCopy', () => {
  it('explains a timeout in terms of the period the reader chose', () => {
    const copy = panelFailureCopy(new QueryError('internal', 'panel execution failed', 200, 'timeout'), russianHost)
    expect(copy.message).toBe('Данных за выбранный период слишком много')
    expect(copy.alert).toBe(true)
  })

  it('does not announce a load the reader cancelled', () => {
    const copy = panelFailureCopy(new QueryError('internal', 'panel execution failed', 200, 'canceled'), russianHost)
    expect(copy.message).toBe('Загрузка была прервана')
    expect(copy.alert).toBe(false)
  })

  it('keeps the generic sentence when the server named no reason', () => {
    expect(panelFailureCopy(new Error('boom'), russianHost).message).toBe('Не удалось отобразить панель')
  })

  it('keeps the generic sentence for a reason this build does not know', () => {
    expect(panelFailureCopy({ reason: 'quota_exceeded' }, russianHost).message).toBe('Не удалось отобразить панель')
  })

  // An older host is entitled to keep working unchanged: an untranslated key
  // must land on its own localized sentence, never on the English default.
  it('falls back to the host catalogue rather than English copy', () => {
    const copy = panelFailureCopy(new QueryError('internal', 'panel execution failed', 200, 'timeout'), legacyHost)
    expect(copy.message).toBe('Не удалось отобразить панель')
  })

  // A host with no catalogue is already reading English, so it gets the
  // English that says something rather than the sentence that says nothing.
  it('uses the built-in reason copy when the host has no catalogue', () => {
    const copy = panelFailureCopy(new QueryError('internal', 'panel execution failed', 200, 'timeout'), translator({}))
    expect(copy.message).toContain('shorter period')
    expect(panelFailureCopy(new QueryError('internal', 'x', 200, 'canceled'), translator({})).message)
      .toContain('stopped before it finished')
  })

  // The server never puts its developer string on screen: the panel says what
  // kind of failure happened, not what the query called itself.
  it('never shows the developer message', () => {
    const failure = new QueryError('internal', 'analytics.repository.trends: context deadline exceeded', 200, 'timeout')
    expect(panelFailureCopy(failure, russianHost).message).not.toContain('deadline')
  })
})
