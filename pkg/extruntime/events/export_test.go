package events

// SetLiveLimit lowers the number of live slots one pass enumerates.
func SetLiveLimit(s *Scheduler, n int) { s.liveLimit = n }

// SetRetryLimit lowers the number of due retries one pass handles.
func SetRetryLimit(s *Scheduler, n int) { s.retryLimit = n }
