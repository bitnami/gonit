// Package utils provides commonly used utility fuctions
package utils

import "time"

// WaitUntil Waits until cb function returns true or the timeout is reached
func WaitUntil(cb func() bool, args ...time.Duration) (success bool) {
	var (
		timeout       = 60 * time.Second
		checkInterval = 500 * time.Millisecond
	)
	if len(args) > 0 {
		timeout = args[0]
	}
	if len(args) > 1 {
		checkInterval = args[1]
	}

	timoutTimer := time.NewTimer(timeout)
	defer timoutTimer.Stop()

	iteratorTimer := time.NewTimer(checkInterval)
	defer iteratorTimer.Stop()
	i := 0

	for {
		if cb() {
			return true
		}
		select {
		case <-iteratorTimer.C:
			i++
			iteratorTimer.Reset(checkInterval)
		case <-timoutTimer.C:
			return false
		}
	}
}
