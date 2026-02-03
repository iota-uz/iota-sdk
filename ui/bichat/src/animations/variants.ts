/**
 * Framer Motion animation variants for BiChat UI
 * Subtle, professional animations for enterprise applications
 * Respects prefers-reduced-motion for accessibility
 */

const prefersReducedMotion = () => {
  if (typeof window === 'undefined') return false
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/**
 * Fade in animation
 */
export const fadeInVariants = {
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
      duration: prefersReducedMotion() ? 0 : 0.15,
    },
  },
}

/**
 * Fade in with subtle slide up
 */
export const fadeInUpVariants = {
  initial: { opacity: 0, y: 8 },
  animate: {
    opacity: 1,
    y: 0,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.2,
      ease: [0.4, 0, 0.2, 1],
    },
  },
  exit: {
    opacity: 0,
    y: 8,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.15,
    },
  },
}

/**
 * Scale fade for modals and popups
 */
export const scaleFadeVariants = {
  initial: { opacity: 0, scale: 0.98 },
  animate: {
    opacity: 1,
    scale: 1,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.15,
    },
  },
  exit: {
    opacity: 0,
    scale: 0.98,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.1,
    },
  },
}

/**
 * Modal backdrop
 */
export const backdropVariants = {
  initial: { opacity: 0 },
  animate: {
    opacity: 1,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.15,
    },
  },
  exit: {
    opacity: 0,
    transition: {
      duration: 0.15,
    },
  },
}

/**
 * Button press feedback
 */
export const buttonVariants = {
  tap: {
    scale: 0.98,
  },
}

/**
 * Stagger container for lists
 */
export const staggerContainerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.03,
      delayChildren: 0.05,
    },
  },
}

/**
 * List item animation
 */
export const listItemVariants = {
  initial: { opacity: 0, x: -8 },
  animate: {
    opacity: 1,
    x: 0,
    transition: { duration: 0.2 },
  },
  exit: {
    opacity: 0,
    x: -8,
    transition: { duration: 0.15 },
  },
}

/**
 * Message entrance animation
 */
export const messageVariants = {
  initial: {
    opacity: 0,
    y: 8,
  },
  animate: {
    opacity: 1,
    y: 0,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.2,
      ease: [0.4, 0, 0.2, 1],
    },
  },
  exit: {
    opacity: 0,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.15,
    },
  },
}

/**
 * Container for staggered messages
 */
export const messageContainerVariants = {
  initial: { opacity: 0 },
  animate: {
    opacity: 1,
    transition: {
      staggerChildren: 0.05,
      delayChildren: 0.05,
    },
  },
}

/**
 * Typing indicator dots
 */
export const typingDotVariants = {
  initial: { opacity: 0.4 },
  animate: {
    opacity: [0.4, 1, 0.4],
    transition: {
      duration: prefersReducedMotion() ? 0 : 1,
      repeat: Infinity,
      ease: 'easeInOut',
    },
  },
}

/**
 * Floating button (scroll to bottom, etc.)
 */
export const floatingButtonVariants = {
  initial: {
    opacity: 0,
    scale: 0.9,
  },
  animate: {
    opacity: 1,
    scale: 1,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.2,
    },
  },
  exit: {
    opacity: 0,
    scale: 0.9,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.15,
    },
  },
}

/**
 * Dropdown menu
 */
export const dropdownVariants = {
  initial: { opacity: 0, y: -4 },
  animate: {
    opacity: 1,
    y: 0,
    transition: { duration: prefersReducedMotion() ? 0 : 0.15 },
  },
  exit: {
    opacity: 0,
    y: -4,
    transition: { duration: 0.1 },
  },
}

/**
 * Toast notification
 */
export const toastVariants = {
  initial: { opacity: 0, y: -8 },
  animate: {
    opacity: 1,
    y: 0,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.2,
    },
  },
  exit: {
    opacity: 0,
    y: -8,
    transition: {
      duration: prefersReducedMotion() ? 0 : 0.15,
    },
  },
}
