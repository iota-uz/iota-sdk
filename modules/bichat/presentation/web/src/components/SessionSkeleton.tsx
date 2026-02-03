/**
 * SessionSkeleton Component
 * Loading skeleton for session list using reusable Skeleton from @iotauz/iota-sdk/bichat
 */
import { SkeletonGroup, ListItemSkeleton } from '@iotauz/iota-sdk/bichat'

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
