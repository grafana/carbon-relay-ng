package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const SpoolSystem = "spool"

type SpoolMetrics struct {
	Buffer          *BufferMetrics
	IncomingMetrics *prometheus.CounterVec
	WriteDuration   prometheus.Histogram
}

func NewSpoolMetrics(namespace, id string, additionnalLabels prometheus.Labels) *SpoolMetrics {
	if additionnalLabels == nil {
		additionnalLabels = prometheus.Labels{}
	}
	additionnalLabels["id"] = id
	sm := SpoolMetrics{}
	sm.Buffer = NewBufferMetrics(fmt.Sprintf("%s_%s", namespace, SpoolSystem), id, additionnalLabels, nil)
	sm.IncomingMetrics = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   SpoolSystem,
		Name:        "incoming_metrics_total",
		Help:        "total number of incoming metrics in spool",
		ConstLabels: additionnalLabels,
	}, []string{"status"})
	sm.WriteDuration = promauto.NewSummary(prometheus.SummaryOpts{
		Namespace:   namespace,
		Subsystem:   SpoolSystem,
		Name:        "write_duration_ns",
		ConstLabels: additionnalLabels,
		BufCap:      summaryBufCap,
	})
	return &sm
}
