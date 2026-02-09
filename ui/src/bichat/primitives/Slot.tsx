/**
 * Slot Primitive
 * Utility component for the asChild pattern (similar to @radix-ui/react-slot)
 */

import {
  Children,
  cloneElement,
  forwardRef,
  isValidElement,
  type HTMLAttributes,
  type ReactNode,
  type ReactElement,
} from 'react'

type AnyProps = Record<string, unknown>

function mergeProps(slotProps: AnyProps, childProps: AnyProps): AnyProps {
  const overrideProps: AnyProps = { ...childProps }

  for (const propName in childProps) {
    const slotPropValue = slotProps[propName]
    const childPropValue = childProps[propName]

    const isHandler = /^on[A-Z]/.test(propName)
    if (isHandler) {
      // Merge event handlers
      if (slotPropValue && childPropValue) {
        overrideProps[propName] = (...args: unknown[]) => {
          ;(childPropValue as (...a: unknown[]) => void)(...args)
          ;(slotPropValue as (...a: unknown[]) => void)(...args)
        }
      } else if (slotPropValue) {
        overrideProps[propName] = slotPropValue
      }
    } else if (propName === 'style') {
      // Merge styles
      overrideProps[propName] = { ...(slotPropValue as object), ...(childPropValue as object) }
    } else if (propName === 'className') {
      // Merge classNames
      overrideProps[propName] = [slotPropValue, childPropValue].filter(Boolean).join(' ')
    }
  }

  return { ...slotProps, ...overrideProps }
}

export interface SlotProps extends HTMLAttributes<HTMLElement> {
  children?: ReactNode
}

/**
 * Slot component that merges its props with its child element's props
 * Used for the asChild pattern to allow consumers to customize the rendered element
 */
export const Slot = forwardRef<HTMLElement, SlotProps>((props, forwardedRef) => {
  const { children, ...slotProps } = props

  if (!isValidElement(children)) {
    return null
  }

  const childrenRef = (children as ReactElement & { ref?: unknown }).ref

  return cloneElement(children as ReactElement, {
    ...mergeProps(slotProps, children.props as AnyProps),
    ref: forwardedRef
      ? composeRefs(forwardedRef, childrenRef as React.Ref<unknown>)
      : childrenRef,
  } as AnyProps)
})

Slot.displayName = 'Slot'

/**
 * Compose multiple refs into one
 */
function composeRefs<T>(...refs: (React.Ref<T> | undefined)[]): React.RefCallback<T> {
  return (node) => {
    refs.forEach((ref) => {
      if (typeof ref === 'function') {
        ref(node)
      } else if (ref != null) {
        ;(ref as React.MutableRefObject<T | null>).current = node
      }
    })
  }
}

/**
 * Helper type for components that support asChild
 * Extends the HTML attributes while adding asChild option
 */
export type AsChildProps<T extends HTMLAttributes<HTMLElement> = HTMLAttributes<HTMLElement>> = T & {
  /** Merge props with child element instead of rendering wrapper */
  asChild?: boolean
}

/**
 * Get children count (flattens fragments)
 */
export function getValidChildren(children: ReactNode): ReactElement[] {
  return Children.toArray(children).filter(isValidElement) as ReactElement[]
}
