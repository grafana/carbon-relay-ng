package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const aggregatorNamespace = "aggregator"

const cacheSystem = "cache"

type CacheMetrics struct {
	Hits   prometheus.Counter
	Misses prometheus.Counter
}

func NewCacheMetrics(namespace string, labels prometheus.Labels) *CacheMetrics {
	cm := CacheMetrics{}

	cVec := promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   cacheSystem,
		Name:        "requests_total",
		Help:        "Total number of requests processed by cache",
		ConstLabels: labels,
	}, []string{"status"})
	cm.Hits = cVec.WithLabelValues("hit")
	cm.Misses = cVec.WithLabelValues("miss")

	return &cm
}

type AggregatorMetrics struct {
	Cache                   *CacheMetrics
	Dropped                 prometheus.Counter
	lowestTimestampCounter  prometheus.Gauge
	highestTimestampCounter prometheus.Gauge
	highTs                  uint32
	lowTs                   uint32
	tsch                    chan uint32
}

func NewAggregatorMetrics(id string, labels prometheus.Labels) *AggregatorMetrics {
	namespace := aggregatorNamespace
	am := AggregatorMetrics{}

	if labels == nil {
		labels = prometheus.Labels{}
	}
	labels["id"] = id

	am.Cache = NewCacheMetrics(namespace, labels)
	am.Dropped = promauto.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Name:        "dropped_metrics_total",
		Help:        "Total number of metrics dropped because of their age",
		ConstLabels: labels,
	})
	tsVec := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "timestamp_value",
		Help:        "Lowest and Highest Timestamp registered",
		ConstLabels: labels,
	}, []string{"type"})
	am.highestTimestampCounter = tsVec.WithLabelValues("highest")
	am.lowestTimestampCounter = tsVec.WithLabelValues("lowest")

	am.tsch = make(chan uint32)
	go func() {
		for ts := range am.tsch {
			am.handleTs(ts)
		}

	}()

	return &am
}

func (am *AggregatorMetrics) handleTs(ts uint32) {
	if am.lowTs > ts || am.lowTs == 0 {
		am.lowTs = ts
		am.lowestTimestampCounter.Set(float64(ts))
		return
	}
	if am.highTs < ts {
		am.highTs = ts
		am.highestTimestampCounter.Set(float64(ts))
	}
}

func (am *AggregatorMetrics) ObserveTimestamp(ts uint32) {
	am.tsch <- ts
}
