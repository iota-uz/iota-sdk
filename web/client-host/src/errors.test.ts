import { describe, expect, it } from 'vitest'
import { asHostError } from './errors'

describe('host errors', () => {
  it('maps JSON-RPC data into typed server field errors', () => {
    const error = asHostError({
      code: -32000,
      message: 'Validation failed',
      data: { code: 'field_validation', fieldErrors: { 'terms.rate': 'Too low' } },
    })
    expect(error.code).toBe('field_validation')
    expect(error.fieldErrors).toEqual({ 'terms.rate': 'Too low' })
  })
})
