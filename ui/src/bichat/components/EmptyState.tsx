/**
 * EmptyState Component
 * Reusable empty state display with icon, title, description, and action
 */

import { memo, type ReactNode } from 'react'
import { motion } from 'framer-motion'
import { fadeInVariants } from '../animations/variants'

export interface EmptyStateProps {
  /** Optional icon to display */
  icon?: ReactNode
  /** Main title text */
  title: string
  /** Optional description text */
  description?: string
  /** Optional action element (button, link, etc.) */
  action?: ReactNode
  /** Additional CSS classes */
  className?: string
  /** Size variant */
  size?: 'sm' | 'md' | 'lg'
}

const sizeClasses = {
  sm: {
    container: 'py-6 px-3',
    title: 'text-sm',
    description: 'text-xs',
  },
  md: {
    container: 'py-8 px-4',
    title: 'text-base',
    description: 'text-sm',
  },
  lg: {
    container: 'py-12 px-6',
    title: 'text-lg',
    description: 'text-base',
  },
}

function EmptyState({
  icon,
  title,
  description,
  action,
  className = '',
  size = 'md',
}: EmptyStateProps) {
  const sizes = sizeClasses[size]

  return (
    <motion.div
      className={`flex items-center justify-center ${sizes.container} ${className}`}
      variants={fadeInVariants}
      initial="initial"
      animate="animate"
      exit="exit"
    >
      <div className="text-center max-w-md">
        {/* Icon */}
        {icon && (
          <motion.div
            className="mb-4 flex justify-center"
            initial={{ opacity: 0, scale: 0.8 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ duration: 0.4, delay: 0.1 }}
          >
            {icon}
          </motion.div>
        )}

        {/* Title */}
        <motion.h3
          className={`${sizes.title} font-medium text-gray-900 dark:text-white mb-2`}
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4, delay: 0.2 }}
        >
          {title}
        </motion.h3>

        {/* Description */}
        {description && (
          <motion.p
            className={`${sizes.description} text-gray-500 dark:text-gray-400 mb-4`}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.4, delay: 0.3 }}
          >
            {description}
          </motion.p>
        )}

        {/* Action */}
        {action && (
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.4, delay: 0.4 }}
          >
            {action}
          </motion.div>
        )}
      </div>
    </motion.div>
  )
}

export default memo(EmptyState)
