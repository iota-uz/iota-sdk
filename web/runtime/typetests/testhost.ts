// Type tests of the in-memory test host: rows read from a table and fixtures are typed from their entity.
import type { Ref } from '../src/index.ts'
import { createTestHost, fixture } from '../src/testing.ts'
import * as c from './acme/contracts.ts'

const host = createTestHost()
const policies = host.table(c.Policy)

// A row has the entity's fields with their types, and the system members.
const number: string = policies[0]!.number
const key: string = policies[0]!.key
const ref: Ref<typeof c.Policy> = policies[0]!.ref
const version: number = policies[0]!.rowVersion
void [number, key, ref, version]

// @ts-expect-error a row has no field the entity does not declare
void policies[0]!.nonexistent
// @ts-expect-error `number` is a string, not a number
const wrong: number = policies[0]!.number
void wrong
// @ts-expect-error a customer's field is not a policy's
void host.table(c.Policy)[0]!.phone
const phone: string | null = host.table(c.Customer)[0]!.phone
void phone

// A fixture exposes its key directly, and it is the ref's key.
const ali = fixture(c.Customer, { name: 'Ali' })
const aliKey: string = ali.key
void aliKey
// @ts-expect-error a key is a string
const numericKey: number = ali.key
void numericKey
