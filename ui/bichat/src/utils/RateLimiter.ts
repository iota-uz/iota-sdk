/**
 * Per-session rate limiter
 * Prevents excessive requests within a time window
 */

export interface RateLimiterConfig {
  maxRequests: number
  windowMs: number
}

export class RateLimiter {
  private timestamps: number[] = []
  private maxRequests: number
  private windowMs: number

  constructor(config: RateLimiterConfig) {
    this.maxRequests = config.maxRequests
    this.windowMs = config.windowMs
  }

  /**
   * Check if a request can be made
   * Updates internal state if request is allowed
   */
  canMakeRequest(): boolean {
    const now = Date.now()

    // Remove timestamps outside the current window
    this.timestamps = this.timestamps.filter(t => now - t < this.windowMs)

    // Check if limit exceeded
    if (this.timestamps.length >= this.maxRequests) {
      return false
    }

    // Add current timestamp
    this.timestamps.push(now)
    return true
  }

  /**
   * Get milliseconds until next request is allowed
   * Returns 0 if request can be made immediately
   */
  getTimeUntilNextRequest(): number {
    if (this.timestamps.length < this.maxRequests) {
      return 0
    }

    const now = Date.now()
    const oldestTimestamp = this.timestamps[0]
    const timeElapsed = now - oldestTimestamp
    const timeRemaining = this.windowMs - timeElapsed

    return Math.max(0, timeRemaining)
  }

  /**
   * Reset the rate limiter state
   */
  reset(): void {
    this.timestamps = []
  }
}
