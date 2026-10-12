// GENERATED — do not edit

import { z } from 'zod'
import { CONTRACT_VERSION } from './types'
import type * as Contract from './types'

const CONTRACT_MAJOR_VERSION = CONTRACT_VERSION.split('.', 1)[0]!

function contractMajor(version: string): string {
  return version.split('.', 1)[0]!
}

export class ContractVersionMismatchError extends Error {
  readonly code = 'CONTRACT_VERSION_MISMATCH'
  readonly expectedMajor = CONTRACT_MAJOR_VERSION

  constructor(readonly actualVersion: string) {
    super(`Lens contract major version ${contractMajor(actualVersion)} is incompatible with expected major ${CONTRACT_MAJOR_VERSION}`)
    this.name = 'ContractVersionMismatchError'
  }
}

export const ContractVersionSchema: z.ZodType<string> = z.string().refine(
  (version) => contractMajor(version) === CONTRACT_MAJOR_VERSION,
  { message: `Expected Lens contract major version ${CONTRACT_MAJOR_VERSION}` },
)

export const CapabilitiesSchema: z.ZodType<Contract.Capabilities> = z.object({
  filters: z.array(z.string()),
  views: z.array(z.string()),
  drill: z.boolean(),
  save: z.boolean(),
  export: z.boolean(),
}).strict()

export const CompositionSchema: z.ZodType<Contract.Composition> = z.lazy(() => z.object({
  id: z.string(),
  revision: z.string(),
  title: z.string(),
  parameters: z.lazy(() => ParametersSchema).optional(),
  rows: z.array(z.array(z.lazy(() => PlacementSchema))),
}).strict())

export const DefinitionSchema: z.ZodType<Contract.Definition> = z.lazy(() => z.object({
  id: z.string(),
  revision: z.string(),
  title: z.string(),
  source: z.lazy(() => SourceRefSchema),
  view: z.string(),
  parameters: z.lazy(() => ParametersSchema).optional(),
}).strict())

export const ParametersSchema: z.ZodType<Contract.Parameters> = z.lazy(() => z.record(z.string(), z.unknown()))

export const PlacementSchema: z.ZodType<Contract.Placement> = z.object({
  id: z.string(),
  reportId: z.string(),
  title: z.string().optional(),
  span: z.number().int().optional(),
  parameters: z.lazy(() => ParametersSchema).optional(),
}).strict()

export const SourceDescriptorSchema: z.ZodType<Contract.SourceDescriptor> = z.object({
  id: z.string(),
  version: z.number().int(),
  revision: z.string(),
  capabilities: z.lazy(() => CapabilitiesSchema),
}).strict()

export const SourceRefSchema: z.ZodType<Contract.SourceRef> = z.object({
  id: z.string(),
  version: z.number().int(),
}).strict()

