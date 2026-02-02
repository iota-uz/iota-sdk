/**
 * Framer Motion animation variants for BiChat
 * Centralized configuration for all motion components
 * Respects prefers-reduced-motion for accessibility
 */

// Helper to check if user prefers reduced motion
const prefersReducedMotion = () => {
  if (typeof window === 'undefined') return false
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/**
 * Fade in animation for content
 */
export const fadeInVariants = {
  initial: { opacity: 0 },
  animate: {
    opacity: 1,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.4,
    },
  },
  exit: {
    opacity: 0,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.2,
    },
  },
}

/**
 * Modal backdrop animation
 */
export const backdropVariants = {
  initial: { opacity: 0 },
  animate: {
    opacity: 1,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.2,
    },
  },
  exit: {
    opacity: 0,
    transition: {
      duration: 0.2,
    },
  },
}

/**
 * Button animation for hover and tap
 */
export const buttonVariants = {
  hover: {
    scale: 1.05,
    transition: { duration: 0.2 },
  },
  tap: {
    scale: 0.95,
  },
}

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

/**
 * Message animations - smooth entrance and exit
 * Used for chat messages sliding in/out
 */
export const messageVariants = {
  initial: {
    opacity: 0,
    y: 20,
    scale: 0.95
  },
  animate: {
    opacity: 1,
    y: 0,
    scale: 1,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.3,
      ease: 'easeOut',
    },
  },
  exit: {
    opacity: 0,
    scale: 0.95,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.2,
    },
  },
}

/**
 * Container for staggered message animations
 */
export const messageContainerVariants = {
  initial: { opacity: 0 },
  animate: {
    opacity: 1,
    transition: {
      staggerChildren: 0.1,
      delayChildren: 0.1,
    },
  },
}

/**
 * Typing indicator dot animations
 */
export const typingDotVariants = {
  initial: { y: 0 },
  animate: {
    y: [-6, 0, -6],
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.6,
      repeat: Infinity,
      ease: 'easeInOut',
    },
  },
}

/**
 * Scroll to bottom button animations
 */
export const scrollButtonVariants = {
  initial: {
    opacity: 0,
    scale: 0.8,
    y: 20,
  },
  animate: {
    opacity: 1,
    scale: 1,
    y: 0,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.3,
      ease: 'easeOut',
    },
  },
  exit: {
    opacity: 0,
    scale: 0.8,
    y: 20,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.2,
    },
  },
  hover: {
    scale: 1.1,
    transition: {
      duration: 0.2,
    },
  },
}

/**
 * Dropdown menu animation
 */
export const dropdownMenuVariants = {
  initial: { opacity: 0, scale: 0.95, y: -10 },
  animate: {
    opacity: 1,
    scale: 1,
    y: 0,
    transition: { duration: prefersReducedMotion() ? 0 : 0.2 },
  },
  exit: {
    opacity: 0,
    scale: 0.95,
    y: -10,
    transition: { duration: 0.15 },
  },
}

/**
 * Error message slide in from top
 */
export const errorMessageVariants = {
  initial: { opacity: 0, y: -20 },
  animate: {
    opacity: 1,
    y: 0,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.3,
    },
  },
  exit: {
    opacity: 0,
    y: -20,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.2,
    },
  },
}
