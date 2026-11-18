// The slice of the generated @iota-kit/core the conformance package uses, for the TS test host run
// (vitest and tsc map "@iota-kit/core" here). The Go run builds the package against the real kit.
import { field } from '../../src/index.ts'
import { nativeEntity } from '../../src/kit.ts'

export const User = nativeEntity(
  'core.user',
  { firstName: field.text(), lastName: field.text(), email: field.text() },
  { key: 'int64' },
)
