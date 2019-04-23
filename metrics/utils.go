package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const summaryBufCap = 50000

func ObserveSince(obs prometheus.Observer, t time.Time) {
	obs.Observe(float64(time.Since(t)))
}
