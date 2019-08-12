package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const routeNamespace = "route"

type RouteMetrics struct {
	Buffer     *BufferMetrics
	InMetrics  prometheus.Counter
	OutMetrics prometheus.Counter
	OutBatches prometheus.Counter
	Errors     *prometheus.CounterVec
}

func NewRouteMetrics(id, routeType string, additionnalLabels prometheus.Labels) *RouteMetrics {
	namespace := routeNamespace
	if additionnalLabels == nil {
		additionnalLabels = prometheus.Labels{}
	}
	additionnalLabels["id"] = id
	rm := RouteMetrics{}

	additionnalLabels["type"] = routeType

	rm.Buffer = NewBufferMetrics(namespace, id, additionnalLabels, nil)
	// TODO: Add again once package route is refactored
	rm.InMetrics = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "incoming_metrics_total",
		Help:        "total number of incoming metrics in a route",
		ConstLabels: additionnalLabels,
	})
	rm.Errors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "errors_total",
		Help:      "total number of errors",
	}, []string{"error"})
	rm.OutMetrics = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "outgoing_metrics_total",
		Help:        "total number of outgoing metrics in a route",
		ConstLabels: additionnalLabels,
	})
	rm.OutBatches = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "outgoing_batches_total",
		Help:        "total number of outgoing batches to remote service in a route",
		ConstLabels: additionnalLabels,
	})
	return &rm
}
