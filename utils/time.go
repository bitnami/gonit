package utils

import (
	"math"
	"time"
)

// RoundDuration returns a new duration truncated to seconds
func RoundDuration(duration time.Duration) time.Duration {
	return time.Duration(math.Floor(duration.Seconds())) * time.Second

}
