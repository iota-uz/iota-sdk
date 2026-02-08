/**
 * Bubble Primitive
 * Compound component for message bubble containers
 */

import {
  createContext,
  useContext,
  forwardRef,
  type HTMLAttributes,
} from 'react'
import { Slot, type AsChildProps } from './Slot'

/* -------------------------------------------------------------------------------------------------
 * BubbleContext
 * -----------------------------------------------------------------------------------------------*/

type BubbleVariant = 'user' | 'assistant' | 'system'

interface BubbleContextValue {
  variant?: BubbleVariant
}

const BubbleContext = createContext<BubbleContextValue | undefined>(undefined)

function useBubbleContext() {
  const context = useContext(BubbleContext)
  if (!context) {
    throw new Error('Bubble components must be used within Bubble.Root')
  }
  return context
}

/* -------------------------------------------------------------------------------------------------
 * Bubble.Root
 * -----------------------------------------------------------------------------------------------*/

type BubbleRootProps = AsChildProps<HTMLAttributes<HTMLDivElement>> & {
  /** Bubble variant (affects data attribute for styling) */
  variant?: BubbleVariant
}

const BubbleRoot = forwardRef<HTMLDivElement, BubbleRootProps>((props, ref) => {
  const { asChild, variant, children, ...domProps } = props
  const Comp = asChild ? Slot : 'div'

  return (
    <BubbleContext.Provider value={{ variant }}>
      <Comp ref={ref} data-bubble-variant={variant} {...domProps}>
        {children}
      </Comp>
    </BubbleContext.Provider>
  )
})

BubbleRoot.displayName = 'Bubble.Root'

/* -------------------------------------------------------------------------------------------------
 * Bubble.Content
 * -----------------------------------------------------------------------------------------------*/

type BubbleContentProps = AsChildProps<HTMLAttributes<HTMLDivElement>>

const BubbleContent = forwardRef<HTMLDivElement, BubbleContentProps>((props, ref) => {
  const { asChild, children, ...domProps } = props
  const Comp = asChild ? Slot : 'div'

  return (
    <Comp ref={ref} data-bubble-part="content" {...domProps}>
      {children}
    </Comp>
  )
})

BubbleContent.displayName = 'Bubble.Content'

/* -------------------------------------------------------------------------------------------------
 * Bubble.Header
 * -----------------------------------------------------------------------------------------------*/

type BubbleHeaderProps = AsChildProps<HTMLAttributes<HTMLDivElement>>

const BubbleHeader = forwardRef<HTMLDivElement, BubbleHeaderProps>((props, ref) => {
  const { asChild, children, ...domProps } = props
  const Comp = asChild ? Slot : 'div'

  return (
    <Comp ref={ref} data-bubble-part="header" {...domProps}>
      {children}
    </Comp>
  )
})

BubbleHeader.displayName = 'Bubble.Header'

/* -------------------------------------------------------------------------------------------------
 * Bubble.Footer
 * -----------------------------------------------------------------------------------------------*/

type BubbleFooterProps = AsChildProps<HTMLAttributes<HTMLDivElement>>

const BubbleFooter = forwardRef<HTMLDivElement, BubbleFooterProps>((props, ref) => {
  const { asChild, children, ...domProps } = props
  const Comp = asChild ? Slot : 'div'

  return (
    <Comp ref={ref} data-bubble-part="footer" {...domProps}>
      {children}
    </Comp>
  )
})

BubbleFooter.displayName = 'Bubble.Footer'

/* -------------------------------------------------------------------------------------------------
 * Bubble.Metadata
 * -----------------------------------------------------------------------------------------------*/

type BubbleMetadataProps = AsChildProps<HTMLAttributes<HTMLDivElement>>

const BubbleMetadata = forwardRef<HTMLDivElement, BubbleMetadataProps>((props, ref) => {
  const { asChild, children, ...domProps } = props
  const Comp = asChild ? Slot : 'div'

  return (
    <Comp ref={ref} data-bubble-part="metadata" {...domProps}>
      {children}
    </Comp>
  )
})

BubbleMetadata.displayName = 'Bubble.Metadata'

/* -------------------------------------------------------------------------------------------------
 * Exports
 * -----------------------------------------------------------------------------------------------*/

export const Bubble = {
  Root: BubbleRoot,
  Content: BubbleContent,
  Header: BubbleHeader,
  Footer: BubbleFooter,
  Metadata: BubbleMetadata,
}

export { useBubbleContext }
export type {
  BubbleRootProps,
  BubbleContentProps,
  BubbleHeaderProps,
  BubbleFooterProps,
  BubbleMetadataProps,
  BubbleVariant,
}
