/**
 * WelcomeContent Component
 * Landing page shown when starting a new chat session
 * Features refined example query cards with premium animations
 */

import { motion } from 'framer-motion'
import { ChartBar, FileText, Lightbulb, Sparkle, type Icon } from '@phosphor-icons/react'

interface ExamplePrompt {
  category: string
  icon: Icon
  text: string
  gradient: string
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
    gradient: 'from-violet-500 to-purple-600'
  },
  {
    category: 'Reports',
    icon: FileText,
    text: 'Generate a summary of customer feedback',
    gradient: 'from-purple-500 to-indigo-600'
  },
  {
    category: 'Insights',
    icon: Lightbulb,
    text: 'What are the top performing products?',
    gradient: 'from-indigo-500 to-violet-600'
  }
]

// Animation variants - refined timing
const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.12,
      delayChildren: 0.1
    }
  }
}

const itemVariants = {
  hidden: { opacity: 0, y: 24 },
  visible: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.6,
      ease: [0.25, 0.1, 0.25, 1]
    }
  }
}

const cardVariants = {
  hidden: { opacity: 0, y: 20, scale: 0.95 },
  visible: {
    opacity: 1,
    y: 0,
    scale: 1,
    transition: {
      duration: 0.5,
      ease: [0.25, 0.1, 0.25, 1]
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
      className="w-full max-w-4xl mx-auto px-6 py-12 text-center"
      variants={containerVariants}
      initial="hidden"
      animate="visible"
    >
      {/* Decorative sparkle */}
      <motion.div
        className="flex justify-center mb-6"
        variants={itemVariants}
      >
        <div className="p-3 rounded-2xl bg-gradient-to-br from-primary-100 to-primary-200 dark:from-primary-900/40 dark:to-primary-800/30">
          <Sparkle size={28} weight="duotone" className="text-primary-600 dark:text-primary-400" />
        </div>
      </motion.div>

      {/* Title - with gradient text option */}
      <motion.h1
        className="text-4xl sm:text-5xl font-bold text-gray-900 dark:text-white mb-5 tracking-tight"
        variants={itemVariants}
      >
        {title}
      </motion.h1>

      {/* Description */}
      <motion.p
        className="text-lg text-gray-600 dark:text-gray-400 mb-14 max-w-2xl mx-auto leading-relaxed"
        variants={itemVariants}
      >
        {description}
      </motion.p>

      {/* Quick Start Section */}
      <motion.div variants={itemVariants}>
        <div className="flex items-center justify-center gap-3 mb-8">
          <div className="h-px w-12 bg-gradient-to-r from-transparent to-gray-300 dark:to-gray-700" />
          <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
            Quick Start
          </h2>
          <div className="h-px w-12 bg-gradient-to-l from-transparent to-gray-300 dark:to-gray-700" />
        </div>

        <motion.div
          className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3"
          variants={containerVariants}
        >
          {EXAMPLE_PROMPTS.map((prompt, index) => (
            <motion.button
              key={index}
              onClick={() => handlePromptClick(prompt.text)}
              disabled={disabled}
              className="card-elevated group relative flex flex-col items-start text-left p-6 rounded-2xl focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:focus:ring-offset-gray-900 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:transform-none disabled:hover:shadow-md"
              variants={cardVariants}
              whileHover={disabled ? {} : { y: -4 }}
              whileTap={disabled ? {} : { scale: 0.98 }}
            >
              {/* Icon with gradient background */}
              <div className={`w-11 h-11 rounded-xl bg-gradient-to-br ${prompt.gradient} flex items-center justify-center mb-4 shadow-sm group-hover:shadow-md transition-shadow`}>
                <prompt.icon
                  size={22}
                  weight="fill"
                  className="text-white"
                />
              </div>

              {/* Category Badge */}
              <span className="inline-block px-2.5 py-1 rounded-lg text-xs font-semibold text-primary-700 dark:text-primary-300 bg-primary-50 dark:bg-primary-900/30 mb-3">
                {prompt.category}
              </span>

              {/* Prompt Text */}
              <p className="text-[15px] font-medium text-gray-700 dark:text-gray-300 group-hover:text-gray-900 dark:group-hover:text-white transition-colors leading-relaxed">
                {prompt.text}
              </p>

              {/* Hover arrow indicator */}
              <div className="absolute bottom-5 right-5 opacity-0 group-hover:opacity-100 transition-all duration-200 transform translate-x-1 group-hover:translate-x-0">
                <svg className="w-5 h-5 text-primary-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 8l4 4m0 0l-4 4m4-4H3" />
                </svg>
              </div>
            </motion.button>
          ))}
        </motion.div>
      </motion.div>
    </motion.div>
  )
}
