/**
 * ActionButton Primitive
 * Compound component for icon buttons with tooltips
 */

import {
  createContext,
  useContext,
  useState,
  forwardRef,
  type ButtonHTMLAttributes,
  type HTMLAttributes,
} from 'react'
import { Slot, type AsChildProps } from './Slot'

/* -------------------------------------------------------------------------------------------------
 * ActionButtonContext
 * -----------------------------------------------------------------------------------------------*/

interface ActionButtonContextValue {
  isHovered: boolean
  isFocused: boolean
  isPressed: boolean
  isDisabled: boolean
}

const ActionButtonContext = createContext<ActionButtonContextValue | undefined>(undefined)

function useActionButtonContext() {
  const context = useContext(ActionButtonContext)
  if (!context) {
    throw new Error('ActionButton components must be used within ActionButton.Root')
  }
  return context
}

/* -------------------------------------------------------------------------------------------------
 * ActionButton.Root
 * -----------------------------------------------------------------------------------------------*/

type ActionButtonRootProps = AsChildProps<ButtonHTMLAttributes<HTMLButtonElement>>

const ActionButtonRoot = forwardRef<HTMLButtonElement, ActionButtonRootProps>((props, ref) => {
  const { asChild, disabled, children, onMouseEnter, onMouseLeave, onFocus, onBlur, onMouseDown, onMouseUp, ...domProps } = props
  const Comp = asChild ? Slot : 'button'

  const [isHovered, setIsHovered] = useState(false)
  const [isFocused, setIsFocused] = useState(false)
  const [isPressed, setIsPressed] = useState(false)

  const handleMouseEnter = (e: React.MouseEvent<HTMLButtonElement>) => {
    setIsHovered(true)
    onMouseEnter?.(e)
  }

  const handleMouseLeave = (e: React.MouseEvent<HTMLButtonElement>) => {
    setIsHovered(false)
    setIsPressed(false)
    onMouseLeave?.(e)
  }

  const handleFocus = (e: React.FocusEvent<HTMLButtonElement>) => {
    setIsFocused(true)
    onFocus?.(e)
  }

  const handleBlur = (e: React.FocusEvent<HTMLButtonElement>) => {
    setIsFocused(false)
    onBlur?.(e)
  }

  const handleMouseDown = (e: React.MouseEvent<HTMLButtonElement>) => {
    setIsPressed(true)
    onMouseDown?.(e)
  }

  const handleMouseUp = (e: React.MouseEvent<HTMLButtonElement>) => {
    setIsPressed(false)
    onMouseUp?.(e)
  }

  return (
    <ActionButtonContext.Provider
      value={{
        isHovered,
        isFocused,
        isPressed,
        isDisabled: !!disabled,
      }}
    >
      <Comp
        ref={ref}
        type="button"
        disabled={disabled}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        onFocus={handleFocus}
        onBlur={handleBlur}
        onMouseDown={handleMouseDown}
        onMouseUp={handleMouseUp}
        {...domProps}
      >
        {children}
      </Comp>
    </ActionButtonContext.Provider>
  )
})

ActionButtonRoot.displayName = 'ActionButton.Root'

/* -------------------------------------------------------------------------------------------------
 * ActionButton.Icon
 * -----------------------------------------------------------------------------------------------*/

type ActionButtonIconProps = AsChildProps<HTMLAttributes<HTMLSpanElement>>

const ActionButtonIcon = forwardRef<HTMLSpanElement, ActionButtonIconProps>((props, ref) => {
  const { asChild, children, ...domProps } = props
  const Comp = asChild ? Slot : 'span'

  return (
    <Comp ref={ref} aria-hidden="true" {...domProps}>
      {children}
    </Comp>
  )
})

ActionButtonIcon.displayName = 'ActionButton.Icon'

/* -------------------------------------------------------------------------------------------------
 * ActionButton.Label
 * -----------------------------------------------------------------------------------------------*/

type ActionButtonLabelProps = AsChildProps<HTMLAttributes<HTMLSpanElement>> & {
  /** Visually hidden but accessible to screen readers */
  srOnly?: boolean
}

const ActionButtonLabel = forwardRef<HTMLSpanElement, ActionButtonLabelProps>((props, ref) => {
  const { asChild, srOnly = false, children, className, ...domProps } = props
  const Comp = asChild ? Slot : 'span'

  const srOnlyClass = srOnly
    ? 'absolute w-px h-px p-0 -m-px overflow-hidden whitespace-nowrap border-0'
    : ''

  return (
    <Comp ref={ref} className={[srOnlyClass, className].filter(Boolean).join(' ')} {...domProps}>
      {children}
    </Comp>
  )
})

ActionButtonLabel.displayName = 'ActionButton.Label'

/* -------------------------------------------------------------------------------------------------
 * ActionButton.Tooltip
 * -----------------------------------------------------------------------------------------------*/

type ActionButtonTooltipProps = AsChildProps<HTMLAttributes<HTMLSpanElement>> & {
  /** Position relative to button */
  position?: 'top' | 'bottom' | 'left' | 'right'
  /** Only show when hovered */
  showOnHover?: boolean
}

const ActionButtonTooltip = forwardRef<HTMLSpanElement, ActionButtonTooltipProps>((props, ref) => {
  const { asChild, position = 'top', showOnHover = true, children, ...domProps } = props
  const context = useActionButtonContext()
  const Comp = asChild ? Slot : 'span'

  // If showOnHover is true, only render when hovered or focused
  if (showOnHover && !context.isHovered && !context.isFocused) {
    return null
  }

  return (
    <Comp ref={ref} role="tooltip" data-tooltip-position={position} {...domProps}>
      {children}
    </Comp>
  )
})

ActionButtonTooltip.displayName = 'ActionButton.Tooltip'

/* -------------------------------------------------------------------------------------------------
 * Exports
 * -----------------------------------------------------------------------------------------------*/

export const ActionButton = {
  Root: ActionButtonRoot,
  Icon: ActionButtonIcon,
  Label: ActionButtonLabel,
  Tooltip: ActionButtonTooltip,
}

export { useActionButtonContext }
export type {
  ActionButtonRootProps,
  ActionButtonIconProps,
  ActionButtonLabelProps,
  ActionButtonTooltipProps,
}
