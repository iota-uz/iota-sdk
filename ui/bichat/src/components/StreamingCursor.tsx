/**
 * StreamingCursor Component
 * Animated cursor shown during AI response streaming
 */


export default function StreamingCursor() {
  return (
    <span
      className="inline-block w-1.5 h-4 ml-0.5 bg-primary-600 dark:bg-primary-500 animate-pulse"
      aria-label="AI is typing"
    />
  )
}
