// Wire types of the management API (pkg/extruntime/manage, JSON as served by
// pkg/extruntime/runtime/api.go). Workflow instances are the engine's types
// and keep their Go field names.

export interface PackageInfo {
  namespace: string
  npmName?: string
  provenance: 'git' | 'runtime'
  repoUrl?: string
  repoCommit?: string
  draftRev: number
  createdBy: number
  createdAt: string
  updatedBy: number
  updatedAt: string
  activeVersion?: string
}

export interface RemoteFile {
  path: string
  hash: string
  size: number
  updatedBy: number
  updatedAt: string
}

export type Severity = 'error' | 'warning'

export interface Diagnostic {
  file: string
  line?: number | undefined
  column?: number | undefined
  severity: Severity
  code: string
  message: string
}

export interface CheckReport {
  diagnostics: Diagnostic[]
  hasErrors: boolean
  kitHash: string
}

export interface BuildReport extends CheckReport {
  sourceHash?: string
  artifactHash?: string
  manifest?: unknown
}

export interface TestCase {
  name: string
  file: string
  passed: boolean
  message?: string
  durationMs: number
}

export interface TestReport {
  passed: boolean
  cases: TestCase[]
}

export interface Review {
  id: number
  approve: boolean
  reviewerId: number
  comment?: string
  sourceHash: string
  artifactHash: string
  at: string
}

export interface VersionInfo {
  id: string
  namespace: string
  version: string
  sourceHash: string
  treeHash: string
  artifactHash: string
  kitHash: string
  provenance: 'git' | 'runtime'
  repoUrl?: string
  repoCommit?: string
  diagnostics: Diagnostic[]
  authorId: number
  createdAt: string
  latestReview?: Review
  approved: boolean
  active: boolean
}

export interface BindingInfo {
  id: string
  slotKind: 'command' | 'screen' | 'shell'
  targetId: string
  package: string
  version: string
  export: string
  activatedBy?: number
  activeFrom: string
  activeTo?: string
}

export interface TriggerOwner {
  triggerId: string
  userId: number
  assignedAt: string
}

export interface AuditEntry {
  id: number
  at: string
  actorUserId: number
  credential?: string
  action: string
  namespace?: string
  version?: string
  target?: string
  sourceHash?: string
  artifactHash?: string
  outcome: 'ok' | 'failed' | 'denied'
  detail?: Record<string, unknown>
}

export interface KitInfo {
  hash: string
  modules: string[]
}

export interface TypeFiles {
  hash: string
  files: Record<string, string>
}

export interface WorkflowInstance {
  ID: string
  Workflow: string
  Package: string
  Version: string
  Status: string
  BlockedReason?: string
  BlockedDetail?: string
  Input?: unknown
  Result?: unknown
  Error?: unknown
  RunCount: number
  CreatedAt: string
  UpdatedAt: string
  FinishedAt?: string | null
  PredecessorID?: string | null
}

export interface WorkflowOccurrence {
  Name: string
  N: number
  Kind: string
  Status: string
  Attempts: number
  Result?: unknown
  Error?: unknown
  UpdatedAt: string
}

export interface WorkflowTask {
  ID: string
  Name: string
  AssigneeUserID: number
  DecisionSchema?: unknown
  Status: string
  Decision?: unknown
  CreatedAt: string
}

export interface WorkflowSignal {
  ID: string
  Name: string
  Payload?: unknown
  Key?: string
  CreatedAt: string
  ConsumedAt?: string | null
}

export interface WorkflowInvocation {
  ID: string
  Contract: string
  BindingPackage: string
  BindingVersion: string
  Outcome: string
  StartedAt: string
}

export interface WorkflowHistory {
  Instance: WorkflowInstance
  Occurrences: WorkflowOccurrence[] | null
  Tasks: WorkflowTask[] | null
  Signals: WorkflowSignal[] | null
  Invocations: WorkflowInvocation[] | null
}

export interface ActivationResult {
  activation: { id: string; namespace: string; version: string; state: string }
  schemaSteps?: string[]
  bound?: BindingInfo[]
  rebound?: BindingInfo[]
  released?: BindingInfo[]
}
