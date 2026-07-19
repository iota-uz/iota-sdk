import type { DashboardDocument, QueryRequest, QueryResponse } from '../contract'
import { replayNavigation, rootNavigation } from './drill'
import type { NavigationView } from './navigation'
import { SnapshotGoneError } from './query'

export interface SnapshotRecoveryOptions {
  request: QueryRequest
  navigation: NavigationView
  loadDocument: () => Promise<DashboardDocument>
  query: (request: QueryRequest) => Promise<QueryResponse>
}

export type SnapshotRecoveryResult =
  | { document?: undefined; navigation: NavigationView; response: QueryResponse; reset: false }
  | { document: DashboardDocument; navigation: NavigationView; response: QueryResponse; reset: false }
  | { document: DashboardDocument; navigation: NavigationView; response?: undefined; reset: true }

export async function queryWithSnapshotRecovery(options: SnapshotRecoveryOptions): Promise<SnapshotRecoveryResult> {
  try {
    return {
      navigation: options.navigation,
      response: await options.query(options.request),
      reset: false,
    }
  } catch (error) {
    if (!(error instanceof SnapshotGoneError)) throw error
  }

  const document = await options.loadDocument()
  const replayed = replayNavigation(document, options.navigation)
  if (!replayed) {
    return {
      document,
      navigation: rootNavigation(document, options.navigation.panelId),
      reset: true,
    }
  }
  return {
    document,
    navigation: replayed,
    response: await options.query({
      ...options.request,
      snapshotId: document.snapshotId,
      path: replayed.path,
      ...(replayed.perspectiveId ? { perspective: replayed.perspectiveId } : { perspective: undefined }),
    }),
    reset: false,
  }
}
