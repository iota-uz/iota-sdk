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
  initial: { opacity: 0, x: -20 },
  animate: {
    opacity: 1,
    x: 0,
    transition: { duration: 0.3 },
  },
  hover: {
    x: 4,
    transition: { duration: 0.2 },
  },
  exit: {
    opacity: 0,
    x: -20,
    transition: { duration: 0.2 },
  },
}
