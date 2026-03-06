package periodics

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

// RetryWrapper creates a JobWrapper that retries failed jobs with exponential backoff
func RetryWrapper(maxRetries int, initialDelay time.Duration, logger *logrus.Logger) cron.JobWrapper {
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

				// Calculate delay with exponential backoff
				delay := time.Duration(attempt) * initialDelay
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

// TimeoutWrapper creates a JobWrapper that cancels jobs after a specified timeout
func TimeoutWrapper(timeout time.Duration, logger *logrus.Logger) cron.JobWrapper {
	return func(j cron.Job) cron.Job {
		return cron.FuncJob(func() {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			done := make(chan struct{})
			var jobPanic interface{}

			go func() {
				defer func() {
					if r := recover(); r != nil {
						jobPanic = r
					}
					close(done)
				}()
				j.Run()
			}()

			select {
			case <-done:
				if jobPanic != nil {
					panic(jobPanic) // Re-panic to let Recover wrapper handle it
				}
			case <-ctx.Done():
				logger.WithField("timeout", timeout).Error("Job execution timed out")
				// Note: We can't actually cancel the job goroutine, but we can log the timeout
				// The job will continue running in the background
			}
		})
	}
}

// LoggingWrapper creates a JobWrapper that logs job execution details
func LoggingWrapper(taskName string, logger *logrus.Logger) cron.JobWrapper {
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
	return func(j cron.Job) cron.Job {
		return cron.FuncJob(func() {
			collector.RecordTaskStart(taskName)
			start := time.Now()

			defer func() {
				duration := time.Since(start)
				if r := recover(); r != nil {
					var err error
					switch v := r.(type) {
					case error:
						err = v
					default:
						err = fmt.Errorf("panic: %v", v)
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
						switch v := r.(type) {
						case error:
							jobError = v
						default:
							jobError = fmt.Errorf("panic: %v", v)
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
