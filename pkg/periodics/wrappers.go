package periodics

import (
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

// noopLogger returns a logger that discards all output
func noopLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

// RetryWrapper creates a JobWrapper that retries failed jobs with exponential backoff
func RetryWrapper(maxRetries int, initialDelay time.Duration, logger *logrus.Logger) cron.JobWrapper {
	if logger == nil {
		logger = noopLogger()
	}
	return func(j cron.Job) cron.Job {
		return cron.FuncJob(func() {
			var lastErr error

			for attempt := 1; attempt <= maxRetries; attempt++ {
				// Execute the job
				func() {
					defer func() {
						if r := recover(); r != nil {
							lastErr = fmt.Errorf("panic during job execution: %v", r)
							logger.WithField("attempt", attempt).Error(lastErr)
						}
					}()

					// Try to execute the job
					j.Run()
					lastErr = nil // Success
				}()

				// If successful, return
				if lastErr == nil {
					if attempt > 1 {
						logger.WithField("attempt", attempt).Info("Job succeeded after retry")
					}
					return
				}

				// If this was the last attempt, log the final failure
				if attempt == maxRetries {
					logger.WithFields(logrus.Fields{
						"attempt":     attempt,
						"max_retries": maxRetries,
						"error":       lastErr,
					}).Error("Job failed after all retry attempts")
					return
				}

				// Calculate delay with exponential backoff (2^(attempt-1) * initialDelay)
				delay := initialDelay * (1 << (attempt - 1))
				logger.WithFields(logrus.Fields{
					"attempt": attempt,
					"delay":   delay,
					"error":   lastErr,
				}).Warn("Job failed, retrying after delay")

				// Wait before retry
				time.Sleep(delay)
			}
		})
	}
}

// TimeoutWrapper creates a JobWrapper that cancels jobs after a specified timeout.
// Note: On timeout, the job goroutine is NOT cancelled and will continue running in the background
// until it completes naturally. This is a known limitation since Go goroutines are not preemptible.
// A warning is logged if the timed-out goroutine eventually completes.
func TimeoutWrapper(timeout time.Duration, logger *logrus.Logger) cron.JobWrapper {
	if logger == nil {
		logger = noopLogger()
	}
	return func(j cron.Job) cron.Job {
		return cron.FuncJob(func() {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			done := make(chan struct{})
			var jobPanic any
			timedOut := make(chan struct{})

			go func() {
				defer func() {
					if r := recover(); r != nil {
						jobPanic = r
					}
					close(done)
					// Warn if the wrapper already timed out
					select {
					case <-timedOut:
						logger.Warn("Job goroutine completed after timeout")
					default:
					}
				}()
				j.Run()
			}()

			select {
			case <-done:
				if jobPanic != nil {
					panic(jobPanic) // Re-panic to let Recover wrapper handle it
				}
			case <-ctx.Done():
				close(timedOut)
				logger.WithField("timeout", timeout).Error("Job execution timed out")
			}
		})
	}
}

// LoggingWrapper creates a JobWrapper that logs job execution details
func LoggingWrapper(taskName string, logger *logrus.Logger) cron.JobWrapper {
	if logger == nil {
		logger = noopLogger()
	}
	return func(j cron.Job) cron.Job {
		return cron.FuncJob(func() {
			start := time.Now()
			logger.WithField("task", taskName).Info("Starting periodic task execution")

			defer func() {
				duration := time.Since(start)
				if r := recover(); r != nil {
					logger.WithFields(logrus.Fields{
						"task":     taskName,
						"duration": duration,
						"error":    r,
					}).Error("Periodic task panicked")
					panic(r) // Re-panic to let other wrappers handle it
				}
				logger.WithFields(logrus.Fields{
					"task":     taskName,
					"duration": duration,
				}).Info("Periodic task completed")
			}()

			j.Run()
		})
	}
}

// MetricsWrapper creates a JobWrapper that collects metrics
func MetricsWrapper(taskName string, collector *MetricsCollector) cron.JobWrapper {
	if collector == nil {
		return func(j cron.Job) cron.Job { return j }
	}
	return func(j cron.Job) cron.Job {
		return cron.FuncJob(func() {
			collector.RecordTaskStart(taskName)
			start := time.Now()

			defer func() {
				duration := time.Since(start)
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					var err error
					switch v := r.(type) {
					case error:
						err = fmt.Errorf("%w\n\n%s", v, stack)
					default:
						err = fmt.Errorf("panic: %v\n\n%s", v, stack)
					}
					collector.RecordTaskFailure(taskName, duration, err)
					panic(r) // Re-panic to let other wrappers handle it
				}
			}()

			// Execute job and check for errors
			var jobError error
			func() {
				defer func() {
					if r := recover(); r != nil {
						stack := string(debug.Stack())
						switch v := r.(type) {
						case error:
							jobError = fmt.Errorf("%w\n\n%s", v, stack)
						default:
							jobError = fmt.Errorf("panic: %v\n\n%s", v, stack)
						}
					}
				}()
				j.Run()
			}()

			duration := time.Since(start)
			if jobError != nil {
				collector.RecordTaskFailure(taskName, duration, jobError)
				panic(jobError) // Re-panic to maintain error flow
			} else {
				collector.RecordTaskSuccess(taskName, duration)
			}
		})
	}
}
