/**
 * Framer Motion animation variants for BiChat
 * Centralized configuration for all motion components
 * Respects prefers-reduced-motion for accessibility
 */

/**
 * Session list container with stagger
 */
export const sessionListContainerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.05,
      delayChildren: 0.1,
    },
  },
}

/**
 * Session item animation
 */
export const sessionItemVariants = {
  initial: { opacity: 0, x: -12 },
  animate: {
    opacity: 1,
    x: 0,
    transition: { duration: 0.25, ease: [0.4, 0, 0.2, 1] },
  },
  hover: {
    // Subtle hover - no x-translate to avoid jarring effect in dense lists
    transition: { duration: 0.15 },
  },
  exit: {
    opacity: 0,
    x: -12,
    transition: { duration: 0.15 },
  },
}
