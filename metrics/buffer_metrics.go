package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const bufferSystem = "buffer"

const FlushTypeTicker = "tick"
const FlushTypeManual = "manual"

type BufferMetrics struct {
	Size           prometheus.Gauge
	DroppedMetrics prometheus.Counter

	BufferedMetrics prometheus.Gauge
	WriteDuration   prometheus.Summary
	FlushSize       *prometheus.SummaryVec
	FlushDuration   *prometheus.SummaryVec
}

func NewBufferMetrics(namespace, id string, additionnalLabels prometheus.Labels, WriteDurationBuckets []float64) *BufferMetrics {
	if additionnalLabels == nil {
		additionnalLabels = prometheus.Labels{}
	}
	additionnalLabels["id"] = id
	bm := BufferMetrics{}
	bm.Size = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   bufferSystem,
		Name:        "size",
		Help:        "The current size of the buffer",
		ConstLabels: additionnalLabels,
	})
	bm.WriteDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace:   namespace,
		Subsystem:   bufferSystem,
		Name:        "write_duration_ns",
		Help:        "Summary about time spent writing in buffer",
		ConstLabels: additionnalLabels,
		Buckets:     WriteDurationBuckets,
	})
	bm.FlushSize = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:   namespace,
		Subsystem:   bufferSystem,
		Name:        "flush_size",
		Help:        "Summary about the buffer's flushes size",
		ConstLabels: additionnalLabels,
		BufCap:      50000,
	}, []string{"flush_type"})
	bm.FlushDuration = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:   namespace,
		Subsystem:   bufferSystem,
		Name:        "flush_duration_ns",
		Help:        "Summary about the buffer's flushes duration",
		ConstLabels: additionnalLabels,
		BufCap:      50000,
	}, []string{"flush_type"})
	bm.BufferedMetrics = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   bufferSystem,
		Name:        "metrics_number_current",
		Help:        "Current number of buffered metrics",
		ConstLabels: additionnalLabels,
	})
	bm.DroppedMetrics = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   bufferSystem,
		Name:        "dropped_total",
		Help:        "Total number of dropped metrics",
		ConstLabels: additionnalLabels,
	})
	return &bm
}

func (bm *BufferMetrics) ObserveFlush(duration time.Duration, size int64, flush_type string) {
	if flush_type == "" {
		flush_type = "basic"
	}
	bm.FlushDuration.WithLabelValues(flush_type).Observe(float64(duration.Nanoseconds()))
	bm.FlushSize.WithLabelValues(flush_type).Observe(float64(size))
}
