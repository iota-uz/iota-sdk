package events

// SetLiveLimit lowers the number of live slots one pass enumerates.
func SetLiveLimit(s *Scheduler, n int) { s.liveLimit = n }
