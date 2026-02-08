/**
 * TypingIndicator Component
 * Displays rotating verbs with shimmer animation to show AI is thinking/processing.
 * Verbs are configurable via props.
 */

import { useState, useEffect, memo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { verbTransitionVariants } from '../animations/variants'

export interface TypingIndicatorProps {
  /** Custom thinking verbs to rotate through */
  verbs?: string[]
  /** Verb rotation interval in ms (defaults to 3000) */
  rotationInterval?: number
  /** Additional CSS classes */
  className?: string
}

// Default thinking verbs
const DEFAULT_VERBS = [
  'Thinking',
  'Processing',
  'Analyzing',
  'Synthesizing',
  'Computing',
  'Working on it',
]

// Check if user prefers reduced motion
const prefersReducedMotion = () => {
  if (typeof window === 'undefined') return false
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

// Random selector without immediate repeat
const getRandomVerb = (verbs: string[], current: string): string => {
  const available = verbs.filter((v) => v !== current)
  if (available.length === 0) {
    return current || verbs[0] || 'Thinking'
  }
  return available[Math.floor(Math.random() * available.length)]
}

function TypingIndicator({
  verbs = DEFAULT_VERBS,
  rotationInterval = 3000,
  className = '',
}: TypingIndicatorProps) {
  const [verb, setVerb] = useState(() => verbs[Math.floor(Math.random() * verbs.length)])

  useEffect(() => {
    if (prefersReducedMotion()) return

    const interval = setInterval(() => {
      setVerb((prev) => getRandomVerb(verbs, prev))
    }, rotationInterval)

    return () => clearInterval(interval)
  }, [verbs, rotationInterval])

  return (
    <div
      role="status"
      aria-live="polite"
      className={`flex items-center gap-2.5 text-gray-500 dark:text-gray-400 ${className}`}
    >
      <div className="flex items-center gap-1" aria-hidden="true">
        <span className="w-1.5 h-1.5 rounded-full bg-gray-400 dark:bg-gray-500 animate-bounce motion-reduce:animate-none [animation-delay:0ms]" />
        <span className="w-1.5 h-1.5 rounded-full bg-gray-400 dark:bg-gray-500 animate-bounce motion-reduce:animate-none [animation-delay:150ms]" />
        <span className="w-1.5 h-1.5 rounded-full bg-gray-400 dark:bg-gray-500 animate-bounce motion-reduce:animate-none [animation-delay:300ms]" />
      </div>
      <div className="overflow-hidden h-6 relative">
        <AnimatePresence mode="wait">
          <motion.span
            key={verb}
            variants={verbTransitionVariants}
            initial="initial"
            animate="animate"
            exit="exit"
            className="text-sm bichat-thinking-shimmer block"
            aria-label={`AI is ${verb}`}
          >
            {verb}...
          </motion.span>
        </AnimatePresence>
      </div>
    </div>
  )
}

const MemoizedTypingIndicator = memo(TypingIndicator)
MemoizedTypingIndicator.displayName = 'TypingIndicator'

export { MemoizedTypingIndicator as TypingIndicator }
export default MemoizedTypingIndicator
