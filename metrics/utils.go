package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func ObserveSinceSeconds(obs prometheus.Observer, t time.Time) {
	obs.Observe(time.Since(t).Seconds())
}
