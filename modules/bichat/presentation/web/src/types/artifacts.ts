/**
 * Artifact types for BiChat frontend
 * Matches GraphQL schema at modules/bichat/presentation/graphql/schema.graphql
 */

export interface Artifact {
  id: string
  sessionID: string
  messageID?: string
  type: string  // Extensible: "code_output", "chart", "export", etc.
  name: string
  description?: string
  mimeType?: string
  url?: string
  sizeBytes: number
  metadata?: Record<string, any>  // JSON metadata (chart spec, row counts, etc.)
  createdAt: string
}
