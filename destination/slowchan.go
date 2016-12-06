package destination

import (
	"time"
)

// note: goroutine doesn't get cleaned until backend closes.  don't instantiate gazillions of these
func NewSlowChan(backend chan []byte, sleep time.Duration) chan []byte {
	c := make(chan []byte)
	go func(c chan []byte) {
		time.Sleep(sleep)
		for v := range backend {
			c <- v
			time.Sleep(sleep)
		}
		close(c)
	}(c)
	return c
}
