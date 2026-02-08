/**
 * BiChat Primitives
 * Unstyled compound components for building custom chat UIs
 */

// Slot utility
export { Slot, type SlotProps, type AsChildProps, getValidChildren } from './Slot'

// Turn primitive
export {
  Turn,
  useTurnContext,
  type TurnRootProps,
  type TurnUserProps,
  type TurnAssistantProps,
  type TurnTimestampProps,
  type TurnActionsProps,
} from './Turn'

// Avatar primitive
export {
  Avatar,
  useAvatarContext,
  type AvatarRootProps,
  type AvatarImageProps,
  type AvatarFallbackProps,
  type ImageLoadingStatus,
} from './Avatar'

// Bubble primitive
export {
  Bubble,
  useBubbleContext,
  type BubbleRootProps,
  type BubbleContentProps,
  type BubbleHeaderProps,
  type BubbleFooterProps,
  type BubbleMetadataProps,
  type BubbleVariant,
} from './Bubble'

// ActionButton primitive
export {
  ActionButton,
  useActionButtonContext,
  type ActionButtonRootProps,
  type ActionButtonIconProps,
  type ActionButtonLabelProps,
  type ActionButtonTooltipProps,
} from './ActionButton'
