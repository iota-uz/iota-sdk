/**
 * EmptyState Component
 * Consistent empty states across ChatList/Sidebar with illustrations and actions
 */

import { memo, type ReactNode } from 'react'
import { motion } from 'framer-motion'
import { fadeInVariants } from '../animations/variants'

interface EmptyStateProps {
  icon?: ReactNode
  title: string
  description?: string
  action?: ReactNode
  className?: string
}

function EmptyState({ icon, title, description, action, className = '' }: EmptyStateProps) {
  return (
    <motion.div
      className={`flex items-center justify-center py-8 px-4 ${className}`}
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
          className="text-base font-medium text-gray-900 dark:text-white mb-2"
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4, delay: 0.2 }}
        >
          {title}
        </motion.h3>

        {/* Description */}
        {description && (
          <motion.p
            className="text-sm text-gray-500 dark:text-gray-400 mb-4"
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
