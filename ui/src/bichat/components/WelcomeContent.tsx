/**
 * WelcomeContent Component
 * Landing page shown when starting a new chat session
 * Clean, professional design for enterprise BI applications
 */

import { motion } from 'framer-motion'
import { ChartBar, FileText, Lightbulb, type Icon } from '@phosphor-icons/react'

interface ExamplePrompt {
  category: string
  icon: Icon
  text: string
}

interface WelcomeContentProps {
  onPromptSelect?: (prompt: string) => void
  title?: string
  description?: string
  disabled?: boolean
}

const EXAMPLE_PROMPTS: ExamplePrompt[] = [
  {
    category: 'Data Analysis',
    icon: ChartBar,
    text: 'Show me sales trends for the last quarter',
  },
  {
    category: 'Reports',
    icon: FileText,
    text: 'Generate a summary of customer feedback',
  },
  {
    category: 'Insights',
    icon: Lightbulb,
    text: 'What are the top performing products?',
  }
]

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.08,
      delayChildren: 0.05
    }
  }
}

const itemVariants = {
  hidden: { opacity: 0, y: 12 },
  visible: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.3,
      ease: [0.4, 0, 0.2, 1]
    }
  }
}

export default function WelcomeContent({
  onPromptSelect,
  title = 'Welcome to BiChat',
  description = 'Your intelligent business analytics assistant. Ask questions about your data, generate reports, or explore insights.',
  disabled = false
}: WelcomeContentProps) {
  const handlePromptClick = (prompt: string) => {
    if (onPromptSelect && !disabled) {
      onPromptSelect(prompt)
    }
  }

  return (
    <motion.div
      className="w-full max-w-3xl mx-auto px-6 py-12 text-center"
      variants={containerVariants}
      initial="hidden"
      animate="visible"
    >
      {/* Title */}
      <motion.h1
        className="text-3xl sm:text-4xl font-semibold text-gray-900 dark:text-white mb-4"
        variants={itemVariants}
      >
        {title}
      </motion.h1>

      {/* Description */}
      <motion.p
        className="text-base text-gray-500 dark:text-gray-400 mb-12 max-w-xl mx-auto leading-relaxed"
        variants={itemVariants}
      >
        {description}
      </motion.p>

      {/* Example prompts */}
      <motion.div variants={itemVariants}>
        <p className="text-xs font-medium text-gray-400 dark:text-gray-500 uppercase tracking-wide mb-4">
          Try asking
        </p>

        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {EXAMPLE_PROMPTS.map((prompt, index) => (
            <motion.button
              key={index}
              onClick={() => handlePromptClick(prompt.text)}
              disabled={disabled}
              className="group flex flex-col items-start text-left p-5 rounded-xl bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 shadow-sm hover:shadow-md hover:border-gray-300 dark:hover:border-gray-600 transition-all duration-150 focus:outline-none focus:ring-2 focus:ring-primary-500/50 focus:ring-offset-2 dark:focus:ring-offset-gray-900 disabled:opacity-50 disabled:cursor-not-allowed"
              variants={itemVariants}
              whileHover={disabled ? {} : { y: -2 }}
              whileTap={disabled ? {} : { scale: 0.98 }}
            >
              {/* Icon */}
              <div className="w-9 h-9 rounded-lg bg-gray-100 dark:bg-gray-700 flex items-center justify-center mb-3">
                <prompt.icon
                  size={18}
                  weight="regular"
                  className="text-gray-600 dark:text-gray-300"
                />
              </div>

              {/* Category */}
              <span className="text-xs font-medium text-gray-400 dark:text-gray-500 mb-1.5">
                {prompt.category}
              </span>

              {/* Prompt text */}
              <p className="text-sm text-gray-700 dark:text-gray-200 leading-relaxed">
                {prompt.text}
              </p>
            </motion.button>
          ))}
        </div>
      </motion.div>
    </motion.div>
  )
}
