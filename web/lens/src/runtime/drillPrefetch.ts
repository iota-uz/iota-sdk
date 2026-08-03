import type { DashboardDocument, Frame, NodePath } from "../contract";
import type { QueryClient } from "./query";

interface IdleDrillPrefetchRoot {
  panelId: string;
  path: NodePath;
  perspective: string;
}

export function idleDrillPrefetchRoots(
  document: DashboardDocument,
): Array<IdleDrillPrefetchRoot> {
  const result: Array<IdleDrillPrefetchRoot> = [];
  const seen = new Set<string>();
  for (const row of document.layout.rows) {
    for (const item of row.panels) {
      const panel = document.panels.find(
        (candidate) => candidate.id === item.panelId,
      );
      const host = panel?.drillRoot
        ? document.drill.edges[panel.drillRoot]
        : undefined;
      if (!panel || !host) continue;
      for (const perspective of document.perspectives) {
        if (perspective.semantics === "evidence" || seen.has(perspective.id))
          continue;
        const root = document.drill.edges[perspective.root];
        if (!root || !host.path.every((key, index) => root.path[index] === key))
          continue;
        seen.add(perspective.id);
        result.push({
          panelId: panel.id,
          path: [...root.path],
          perspective: perspective.id,
        });
        if (result.length === 8) return result;
      }
    }
  }
  return result;
}

interface IdleDrillPrefetchOptions {
  document: DashboardDocument;
  queryClient: QueryClient;
  signal: AbortSignal;
  onChildren: (path: NodePath, frame: Frame) => void;
}

/**
 * Warm bounded first-level drill states after the useful root row is ready.
 * This module is loaded dynamically so speculative traversal never delays the
 * initial dashboard bundle or its critical panel requests.
 */
export async function prefetchIdleDrillStates({
  document,
  queryClient,
  signal,
  onChildren,
}: IdleDrillPrefetchOptions): Promise<void> {
  for (const candidate of idleDrillPrefetchRoots(document)) {
    if (signal.aborted) return;
    try {
      const response = await queryClient.query(
        {
          snapshotId: document.snapshotId,
          path: candidate.path,
          perspective: candidate.perspective,
          prefetch: true,
          idlePrefetch: true,
        },
        { signal },
      );
      const frame = Object.values(response.frames)[0];
      if (!frame) continue;
      if (frame.children) onChildren(candidate.path, frame);
      const children =
        frame.children?.filter(({ target }) => Boolean(target)) ?? [];
      if (children.length === 0 || children.length > 4) continue;
      for (const child of children) {
        if (!child.target || signal.aborted) return;
        await queryClient.query(
          {
            snapshotId: document.snapshotId,
            path: [...candidate.path, child.key, child.target],
            perspective: candidate.perspective,
            prefetch: true,
            idlePrefetch: true,
          },
          { signal },
        );
      }
    } catch {
      if (signal.aborted) return;
    }
  }
}
