/**
 * WelcomeContent Component
 * Landing page shown when starting a new chat session
 * Features example query cards with icons and animations
 */

import { motion } from 'framer-motion'
import { ChartBar, FileText, Lightbulb, type Icon } from '@phosphor-icons/react'

interface ExamplePrompt {
  category: string
  categoryColor: string
  icon: Icon
  text: string
}

interface WelcomeContentProps {
  onPromptSelect?: (prompt: string) => void
  title?: string
  description?: string
}

const EXAMPLE_PROMPTS: ExamplePrompt[] = [
  {
    category: 'Data Analysis',
    categoryColor: 'bg-primary-600 text-white',
    icon: ChartBar,
    text: 'Show me sales trends for the last quarter'
  },
  {
    category: 'Reports',
    categoryColor: 'bg-primary-500 text-white',
    icon: FileText,
    text: 'Generate a summary of customer feedback'
  },
  {
    category: 'Insights',
    categoryColor: 'bg-primary-700 text-white',
    icon: Lightbulb,
    text: 'What are the top performing products?'
  }
]

// Animation variants
const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.1,
      delayChildren: 0.2
    }
  }
}

const itemVariants = {
  hidden: { opacity: 0, y: 20 },
  visible: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.4,
      ease: 'easeOut'
    }
  }
}

const cardHoverVariants = {
  rest: { scale: 1 },
  hover: {
    scale: 1.02,
    transition: {
      duration: 0.2,
      ease: 'easeInOut'
    }
  }
}

export default function WelcomeContent({
  onPromptSelect,
  title = 'Welcome to BiChat',
  description = 'Your intelligent business analytics assistant. Ask questions about your data, generate reports, or explore insights.'
}: WelcomeContentProps) {
  const handlePromptClick = (prompt: string) => {
    if (onPromptSelect) {
      onPromptSelect(prompt)
    }
  }

  return (
    <motion.div
      className="flex-1 flex items-center justify-center p-8"
      variants={containerVariants}
      initial="hidden"
      animate="visible"
    >
      <div className="max-w-4xl w-full">
        {/* Header */}
        <motion.div className="text-center mb-12" variants={itemVariants}>
          <h1 className="text-4xl font-bold text-gray-900 dark:text-white mb-4">
            {title}
          </h1>
          <p className="text-lg text-gray-600 dark:text-gray-400 max-w-2xl mx-auto">
            {description}
          </p>
        </motion.div>

        {/* Example Prompts Grid */}
        <motion.div
          className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4"
          variants={itemVariants}
        >
          {EXAMPLE_PROMPTS.map((prompt, index) => {
            const IconComponent = prompt.icon
            return (
              <motion.button
                key={index}
                onClick={() => handlePromptClick(prompt.text)}
                className="group relative p-6 bg-white dark:bg-gray-800 rounded-2xl border-2 border-gray-200 dark:border-gray-700 hover:border-primary-500 dark:hover:border-primary-600 transition-all text-left focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:focus:ring-offset-gray-900"
                variants={cardHoverVariants}
                initial="rest"
                whileHover="hover"
                whileTap={{ scale: 0.98 }}
              >
                {/* Category badge */}
                <div className="mb-4">
                  <span
                    className={`inline-block px-3 py-1 rounded-full text-xs font-semibold ${prompt.categoryColor}`}
                  >
                    {prompt.category}
                  </span>
                </div>

                {/* Icon */}
                <div className="mb-4 text-primary-600 dark:text-primary-500 group-hover:scale-110 transition-transform">
                  <IconComponent size={32} />
                </div>

                {/* Prompt text */}
                <p className="text-gray-700 dark:text-gray-300 font-medium leading-relaxed">
                  {prompt.text}
                </p>

                {/* Hover indicator */}
                <div className="absolute bottom-4 right-4 opacity-0 group-hover:opacity-100 transition-opacity">
                  <svg
                    className="w-5 h-5 text-primary-600 dark:text-primary-500"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M14 5l7 7m0 0l-7 7m7-7H3"
                    />
                  </svg>
                </div>
              </motion.button>
            )
          })}
        </motion.div>

        {/* Footer hint */}
        <motion.div
          className="mt-12 text-center text-sm text-gray-500 dark:text-gray-400"
          variants={itemVariants}
        >
          <p>Or start typing your own question below</p>
        </motion.div>
      </div>
    </motion.div>
  )
}
