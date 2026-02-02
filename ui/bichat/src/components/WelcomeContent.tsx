/**
 * WelcomeContent Component
 * Landing page shown when starting a new chat session
 * Clean, professional design for enterprise BI applications
 *
 * Uses branding configuration for customization:
 * - Title and description from branding.welcome or translations
 * - Example prompts from branding.welcome.examplePrompts
 */

import { motion } from 'framer-motion'
import { ChartBar, FileText, Lightbulb, type Icon } from '@phosphor-icons/react'
import { useBranding } from '../hooks/useBranding'
import { useTranslation } from '../hooks/useTranslation'

interface WelcomeContentProps {
  onPromptSelect?: (prompt: string) => void
  /** Override title (takes precedence over branding) */
  title?: string
  /** Override description (takes precedence over branding) */
  description?: string
  disabled?: boolean
}

/**
 * Map icon name strings to Phosphor Icon components.
 */
const iconMap: Record<string, Icon> = {
  'chart-bar': ChartBar,
  'file-text': FileText,
  lightbulb: Lightbulb,
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
  title: titleOverride,
  description: descriptionOverride,
  disabled = false,
}: WelcomeContentProps) {
  const branding = useBranding()
  const { t } = useTranslation()

  // Use overrides, then branding, then translation fallbacks
  const title = titleOverride || branding.welcome?.title || t('welcome.title')
  const description =
    descriptionOverride || branding.welcome?.description || t('welcome.description')
  const examplePrompts = branding.welcome?.examplePrompts || []

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
      {examplePrompts.length > 0 && (
        <motion.div variants={itemVariants}>
          <p className="text-xs font-medium text-gray-400 dark:text-gray-500 uppercase tracking-wide mb-4">
            {t('welcome.tryAsking')}
          </p>

          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {examplePrompts.map((prompt, index) => {
              const IconComponent = prompt.icon ? iconMap[prompt.icon] || ChartBar : ChartBar

              return (
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
                    <IconComponent
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
              )
            })}
          </div>
        </motion.div>
      )}
    </motion.div>
  )
}
