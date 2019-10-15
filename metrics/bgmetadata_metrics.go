package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type BgMetadataMetrics struct {
	AddedMetrics    prometheus.Counter
	FilteredMetrics prometheus.Counter
}

const bgMetadataNamespace = "metadata"

func NewBgMetadataMetrics(id string) BgMetadataMetrics {
	namespace := bgMetadataNamespace
	mm := BgMetadataMetrics{}
	labels := prometheus.Labels{
		"id": id,
	}
	mm.AddedMetrics = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "added_metrics_total",
		Help:        "total number of metrics added to metadata",
		ConstLabels: labels,
	})
	mm.FilteredMetrics = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "filtered_metrics_total",
		Help:        "total number of metrics filtered from adding to metadata",
		ConstLabels: labels,
	})
	return mm
}
