/**
 * Turn Primitive
 * Compound component for rendering a conversation turn (user + assistant)
 */

import {
  createContext,
  useContext,
  forwardRef,
  type HTMLAttributes,
} from 'react'
import { Slot, type AsChildProps } from './Slot'

/* -------------------------------------------------------------------------------------------------
 * TurnContext
 * -----------------------------------------------------------------------------------------------*/

interface TurnContextValue {
  /** Turn identifier */
  turnId?: string
}

const TurnContext = createContext<TurnContextValue | undefined>(undefined)

function useTurnContext() {
  const context = useContext(TurnContext)
  if (!context) {
    throw new Error('Turn components must be used within Turn.Root')
  }
  return context
}

/* -------------------------------------------------------------------------------------------------
 * Turn.Root
 * -----------------------------------------------------------------------------------------------*/

type TurnRootProps = AsChildProps<HTMLAttributes<HTMLDivElement>> & {
  /** Turn identifier for tracking */
  turnId?: string
}

const TurnRoot = forwardRef<HTMLDivElement, TurnRootProps>((props, ref) => {
  const { asChild, turnId, children, ...domProps } = props
  const Comp = asChild ? Slot : 'div'

  return (
    <TurnContext.Provider value={{ turnId }}>
      <Comp ref={ref} data-turn-id={turnId} {...domProps}>
        {children}
      </Comp>
    </TurnContext.Provider>
  )
})

TurnRoot.displayName = 'Turn.Root'

/* -------------------------------------------------------------------------------------------------
 * Turn.User
 * -----------------------------------------------------------------------------------------------*/

type TurnUserProps = AsChildProps<HTMLAttributes<HTMLDivElement>>

const TurnUser = forwardRef<HTMLDivElement, TurnUserProps>((props, ref) => {
  const { asChild, children, ...domProps } = props
  const Comp = asChild ? Slot : 'div'

  return (
    <Comp ref={ref} data-turn-role="user" {...domProps}>
      {children}
    </Comp>
  )
})

TurnUser.displayName = 'Turn.User'

/* -------------------------------------------------------------------------------------------------
 * Turn.Assistant
 * -----------------------------------------------------------------------------------------------*/

type TurnAssistantProps = AsChildProps<HTMLAttributes<HTMLDivElement>>

const TurnAssistant = forwardRef<HTMLDivElement, TurnAssistantProps>((props, ref) => {
  const { asChild, children, ...domProps } = props
  const Comp = asChild ? Slot : 'div'

  return (
    <Comp ref={ref} data-turn-role="assistant" {...domProps}>
      {children}
    </Comp>
  )
})

TurnAssistant.displayName = 'Turn.Assistant'

/* -------------------------------------------------------------------------------------------------
 * Turn.Timestamp
 * -----------------------------------------------------------------------------------------------*/

type TurnTimestampProps = AsChildProps<HTMLAttributes<HTMLTimeElement>> & {
  /** ISO date string or Date object */
  date?: string | Date
  /** Custom formatter */
  formatter?: (date: Date) => string
}

const defaultFormatter = (date: Date) =>
  date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })

const TurnTimestamp = forwardRef<HTMLTimeElement, TurnTimestampProps>((props, ref) => {
  const { asChild, date, formatter = defaultFormatter, children, ...domProps } = props
  const Comp = asChild ? Slot : 'time'

  const dateObj = date ? (typeof date === 'string' ? new Date(date) : date) : null
  const formattedTime = dateObj ? formatter(dateObj) : ''
  const isoString = dateObj?.toISOString()

  return (
    <Comp ref={ref} dateTime={isoString} {...domProps}>
      {children ?? formattedTime}
    </Comp>
  )
})

TurnTimestamp.displayName = 'Turn.Timestamp'

/* -------------------------------------------------------------------------------------------------
 * Turn.Actions
 * -----------------------------------------------------------------------------------------------*/

type TurnActionsProps = AsChildProps<HTMLAttributes<HTMLDivElement>>

const TurnActions = forwardRef<HTMLDivElement, TurnActionsProps>((props, ref) => {
  const { asChild, children, ...domProps } = props
  const Comp = asChild ? Slot : 'div'

  return (
    <Comp ref={ref} role="group" aria-label="Message actions" {...domProps}>
      {children}
    </Comp>
  )
})

TurnActions.displayName = 'Turn.Actions'

/* -------------------------------------------------------------------------------------------------
 * Exports
 * -----------------------------------------------------------------------------------------------*/

export const Turn = {
  Root: TurnRoot,
  User: TurnUser,
  Assistant: TurnAssistant,
  Timestamp: TurnTimestamp,
  Actions: TurnActions,
}

export { useTurnContext }
export type { TurnRootProps, TurnUserProps, TurnAssistantProps, TurnTimestampProps, TurnActionsProps }
