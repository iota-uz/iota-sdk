package services

import "time"

const (
	DefaultTitleQueueStream          = "bichat:title:jobs"
	DefaultTitleQueueDedupePrefix    = "bichat:title:dedupe"
	DefaultTitleQueueDedupeTTL       = 30 * time.Minute
	DefaultTitleQueueMaxStreamLen    = 10000
	DefaultTitleQueueGroup           = "bichat-title-workers"
	DefaultTitleQueueConsumerPrefix   = "consumer"
	DefaultTitleQueuePollInterval    = 300 * time.Millisecond
	DefaultTitleQueueReadBlock       = 2 * time.Second
	DefaultTitleQueueBatchSize       = 16
	DefaultTitleQueueMaxRetries      = 3
	DefaultTitleQueueRetryBaseDelay  = 5 * time.Second
	DefaultTitleQueueRetryMaxDelay   = 2 * time.Minute
	DefaultTitleQueuePendingIdle     = 30 * time.Second
	DefaultTitleQueueReconcileEvery  = 1 * time.Minute
	DefaultTitleQueueReconcileBatch  = 200
	DefaultTitleQueueJobTimeout      = 20 * time.Second
	DefaultTitleQueueRetryKeySuffix  = ":retry"
)
