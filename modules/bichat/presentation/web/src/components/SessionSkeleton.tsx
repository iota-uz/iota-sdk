import type { JSX } from 'solid-js'
import { SkeletonGroup } from '../ui/primitives'

export function SessionSkeleton(): JSX.Element {
  return (
    <div class="px-2 pt-2">
      <SkeletonGroup count={5} />
    </div>
  )
}
