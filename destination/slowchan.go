package destination

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/graphite-ng/carbon-relay-ng/encoding"
)

// note: goroutine doesn't get cleaned until backend closes.  don't instantiate gazillions of these
func NewSlowChan(backend chan []byte, sleep time.Duration) chan encoding.Datapoint {
	handler := encoding.NewPlain(false, true)
	c := make(chan encoding.Datapoint)
	go func(c chan encoding.Datapoint) {
		time.Sleep(sleep)
		for v := range backend {
			d, err := handler.Load(v)
			if err != nil {
				logrus.Errorf("can't deserialize metric `%s`", string(v))
			} else {
				c <- d
			}
			time.Sleep(sleep)
		}
		close(c)
	}(c)
	return c
}
