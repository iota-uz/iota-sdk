/**
 * WelcomeContent Component
 * Landing page shown when starting a new chat session
 * Clean, professional design for enterprise BI applications
 */

import { motion, useReducedMotion } from 'framer-motion'
import { ChartBar, FileText, Lightbulb, type Icon } from '@phosphor-icons/react'
import { useTranslation } from '../hooks/useTranslation'

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
  },
]

const CATEGORY_STYLES: Record<string, { badge: string; icon: string }> = {
  'Data Analysis': {
    badge: 'bg-sky-50 text-sky-600 ring-sky-600/10 dark:bg-sky-400/10 dark:text-sky-400 dark:ring-sky-400/20',
    icon: 'text-sky-500 dark:text-sky-400',
  },
  Reports: {
    badge: 'bg-teal-50 text-teal-600 ring-teal-600/10 dark:bg-teal-400/10 dark:text-teal-400 dark:ring-teal-400/20',
    icon: 'text-teal-500 dark:text-teal-400',
  },
  Insights: {
    badge: 'bg-amber-50 text-amber-600 ring-amber-600/10 dark:bg-amber-400/10 dark:text-amber-400 dark:ring-amber-400/20',
    icon: 'text-amber-500 dark:text-amber-400',
  },
}

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

const reducedContainerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0,
      delayChildren: 0
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

const reducedItemVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0
    }
  }
}

function WelcomeContent({
  onPromptSelect,
  title,
  description,
  disabled = false
}: WelcomeContentProps) {
  const { t } = useTranslation()
  const shouldReduceMotion = useReducedMotion()
  const resolvedTitle = title || t('welcome.title')
  const resolvedDescription = description || t('welcome.description')

  const handlePromptClick = (prompt: string) => {
    if (onPromptSelect && !disabled) {
      onPromptSelect(prompt)
    }
  }

  const activeContainerVariants = shouldReduceMotion ? reducedContainerVariants : containerVariants
  const activeItemVariants = shouldReduceMotion ? reducedItemVariants : itemVariants

  return (
    <motion.div
      className="relative w-full max-w-5xl mx-auto px-6 text-center"
      variants={activeContainerVariants}
      initial="hidden"
      animate="visible"
    >
      {/* Ambient glow */}
      <div
        className="pointer-events-none absolute inset-x-0 -top-8 flex justify-center overflow-hidden h-56"
        aria-hidden
      >
        <div className="w-[420px] h-[260px] -mt-16 rounded-full bg-primary-300/[0.08] blur-[80px] dark:bg-primary-400/[0.05]" />
      </div>

      {/* Title */}
      <motion.h1
        className="relative text-2xl sm:text-3xl font-semibold text-gray-900 dark:text-white mb-4"
        variants={activeItemVariants}
      >
        {resolvedTitle}
      </motion.h1>

      {/* Description */}
      <motion.p
        className="text-base text-gray-500 dark:text-gray-400 mb-10 max-w-2xl mx-auto leading-relaxed"
        variants={activeItemVariants}
      >
        {resolvedDescription}
      </motion.p>

      {/* Example prompts */}
      <motion.div variants={activeItemVariants}>
        <div className="flex items-center gap-4 mb-5">
          <div className="h-px flex-1 bg-gradient-to-r from-transparent to-gray-200 dark:to-gray-700/70" />
          <span className="text-[11px] font-semibold uppercase tracking-[0.12em] text-gray-400 dark:text-gray-500 select-none">
            {t('welcome.quickStart')}
          </span>
          <div className="h-px flex-1 bg-gradient-to-l from-transparent to-gray-200 dark:to-gray-700/70" />
        </div>

        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {EXAMPLE_PROMPTS.map((prompt, index) => {
            const style = CATEGORY_STYLES[prompt.category]
            return (
              <motion.button
                key={index}
                onClick={() => handlePromptClick(prompt.text)}
                disabled={disabled}
                className="cursor-pointer group flex flex-col items-start text-left p-4 rounded-xl bg-white dark:bg-gray-800/80 border border-gray-200/80 dark:border-gray-700/60 hover:border-gray-300 dark:hover:border-gray-600 hover:bg-gray-50/80 dark:hover:bg-gray-700/30 shadow-sm hover:shadow-md transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-primary-500/40 focus:ring-offset-2 dark:focus:ring-offset-gray-900 disabled:opacity-50 disabled:cursor-not-allowed"
                variants={activeItemVariants}
                whileHover={disabled || shouldReduceMotion ? {} : { y: -3 }}
                whileTap={disabled || shouldReduceMotion ? {} : { scale: 0.98 }}
                aria-label={`${prompt.category}: ${prompt.text}`}
              >
                <div className="mb-3 flex items-center gap-2">
                  <prompt.icon
                    size={16}
                    weight="duotone"
                    className={style?.icon ?? 'text-gray-500'}
                  />
                  <span
                    className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-[11px] font-medium ring-1 ring-inset ${style?.badge ?? 'bg-gray-100 text-gray-600 ring-gray-500/10'}`}
                  >
                    {prompt.category}
                  </span>
                </div>

                <p className="text-[13px] text-gray-700 dark:text-gray-200 leading-relaxed group-hover:text-gray-900 dark:group-hover:text-white transition-colors duration-200">
                  {prompt.text}
                </p>
              </motion.button>
            )
          })}
        </div>
      </motion.div>
    </motion.div>
  )
}

export { WelcomeContent }
export default WelcomeContent
