/**
 * SessionSkeleton Component
 * Loading skeleton for session list using reusable Skeleton from @iota-uz/bichat-ui
 */
import { SkeletonGroup, ListItemSkeleton } from '@iota-uz/bichat-ui'

interface SessionSkeletonProps {
  count?: number
}

export default function SessionSkeleton({ count = 5 }: SessionSkeletonProps) {
  return (
    <SkeletonGroup count={count} gap="sm" className="px-2">
      {() => <ListItemSkeleton />}
    </SkeletonGroup>
  )
}
