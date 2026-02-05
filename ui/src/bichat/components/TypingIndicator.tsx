/**
 * TypingIndicator Component
 * Displays animated dots or rotating text to indicate AI is processing
 */

import { useState, useEffect, memo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'

export type TypingIndicatorVariant = 'dots' | 'text' | 'pulse'

export interface TypingIndicatorProps {
  /** Display variant */
  variant?: TypingIndicatorVariant
  /** Custom thinking messages (for text variant) */
  messages?: string[]
  /** Message rotation interval in ms (for text variant, defaults to 3000) */
  rotationInterval?: number
  /** Size */
  size?: 'sm' | 'md' | 'lg'
  /** Additional CSS classes */
  className?: string
}

// Default thinking messages
const DEFAULT_MESSAGES = [
  'Thinking...',
  'Processing...',
  'Analyzing...',
  'Computing...',
  'Working on it...',
]

// Check if user prefers reduced motion
const prefersReducedMotion = () => {
  if (typeof window === 'undefined') return false
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

// Random selector without immediate repeat
const getRandomMessage = (messages: string[], current: string): string => {
  const available = messages.filter((m) => m !== current)
  return available[Math.floor(Math.random() * available.length)]
}

const sizeConfig = {
  sm: { dot: 'w-1.5 h-1.5', gap: 'gap-1', text: 'text-sm' },
  md: { dot: 'w-2 h-2', gap: 'gap-1.5', text: 'text-base' },
  lg: { dot: 'w-2.5 h-2.5', gap: 'gap-2', text: 'text-lg' },
}

function DotsIndicator({ size = 'md' }: { size: 'sm' | 'md' | 'lg' }) {
  const config = sizeConfig[size]
  return (
    <div className={`flex ${config.gap}`} role="status" aria-live="polite">
      {[0, 1, 2].map((index) => (
        <div
          key={index}
          className={`${config.dot} bg-gray-400 dark:bg-gray-500 rounded-full animate-bounce`}
          style={{ animationDelay: `${index * 0.15}s` }}
        />
      ))}
      <span className="sr-only">Loading...</span>
    </div>
  )
}

function PulseIndicator({ size = 'md' }: { size: 'sm' | 'md' | 'lg' }) {
  const config = sizeConfig[size]
  return (
    <div className="flex items-center" role="status" aria-live="polite">
      <div className={`${config.dot} bg-primary-500 rounded-full animate-pulse`} />
      <span className="sr-only">Loading...</span>
    </div>
  )
}

function TextIndicator({
  messages = DEFAULT_MESSAGES,
  rotationInterval = 3000,
  size = 'md',
}: {
  messages: string[]
  rotationInterval: number
  size: 'sm' | 'md' | 'lg'
}) {
  const config = sizeConfig[size]
  const [message, setMessage] = useState(() => messages[Math.floor(Math.random() * messages.length)])

  useEffect(() => {
    if (prefersReducedMotion()) return

    const interval = setInterval(() => {
      setMessage((prev) => getRandomMessage(messages, prev))
    }, rotationInterval)

    return () => clearInterval(interval)
  }, [messages, rotationInterval])

  return (
    <div role="status" aria-live="polite" className="overflow-hidden h-6 relative">
      <AnimatePresence mode="wait">
        <motion.span
          key={message}
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -10 }}
          transition={{ duration: 0.2 }}
          className={`${config.text} text-gray-500 dark:text-gray-400 block`}
        >
          {message}
        </motion.span>
      </AnimatePresence>
    </div>
  )
}

function TypingIndicator({
  variant = 'dots',
  messages = DEFAULT_MESSAGES,
  rotationInterval = 3000,
  size = 'md',
  className = '',
}: TypingIndicatorProps) {
  return (
    <div className={`flex items-center ${className}`}>
      {variant === 'dots' && <DotsIndicator size={size} />}
      {variant === 'pulse' && <PulseIndicator size={size} />}
      {variant === 'text' && (
        <TextIndicator messages={messages} rotationInterval={rotationInterval} size={size} />
      )}
    </div>
  )
}

const MemoizedTypingIndicator = memo(TypingIndicator)
MemoizedTypingIndicator.displayName = 'TypingIndicator'

export { MemoizedTypingIndicator as TypingIndicator }
export default MemoizedTypingIndicator
