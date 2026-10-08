import '@testing-library/jest-dom/vitest'
import { cleanup, configure } from '@solidjs/testing-library'
import { afterEach } from 'vitest'

// waitFor/findBy* default to a 1s deadline, which chained-roundtrip tests miss
// when the worker pool saturates every core. The deadline only bounds failure —
// passing tests still resolve as soon as the DOM settles.
configure({ asyncUtilTimeout: 5000 })

afterEach(() => {
  cleanup()
})
