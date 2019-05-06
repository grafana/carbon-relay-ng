package destination

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/graphite-ng/carbon-relay-ng/formats"
)

// note: goroutine doesn't get cleaned until backend closes.  don't instantiate gazillions of these
func NewSlowChan(backend chan []byte, sleep time.Duration) chan formats.Datapoint {
	handler := formats.NewPlain(false)
	c := make(chan formats.Datapoint)
	go func(c chan formats.Datapoint) {
		time.Sleep(sleep)
		for v := range backend {
			d, err := handler.Process(v)
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
