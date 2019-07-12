package destination

import (
	"time"

	"go.uber.org/zap"

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
				zap.L().Error("can't deserialize metric", zap.ByteString("payload", v), zap.Error(err))
			} else {
				c <- d
			}
			time.Sleep(sleep)
		}
		close(c)
	}(c)
	return c
}
