package clock

import (
	"time"
)

// AlignedTick returns a tick channel that ticks slightly after*
// offset after the whole interval.
// Examples
// interval   offset         ticks
// 1s         0              every whole second
// 10s        1s             00:00:01, 00:00:11, etc
// 90m        15m            00:00:15, 00:01:45, etc
// [*] in my testing about .0001 to 0.0002 seconds later due
// to scheduling etc.
func AlignedTick(period, offset time.Duration, bufSize int) <-chan time.Time {
	// note that time.Ticker is not an interface,
	// and that if we instantiate one, we can't write to its channel
	// hence we can't leverage that type.
	c := make(chan time.Time, bufSize)
	go func() {
		for {
			unix := time.Now().UnixNano()
			adjusted := time.Duration(unix) - offset
			diff := time.Duration(period - adjusted%period)
			time.Sleep(diff)
			select {
			case c <- time.Now():
			default:
			}
		}
	}()
	return c
}
