package shell

import "time"

func SetDefaultExecuteTimeout(d time.Duration) func() {
	prev := defaultExecuteTimeout
	defaultExecuteTimeout = d
	return func() { defaultExecuteTimeout = prev }
}
