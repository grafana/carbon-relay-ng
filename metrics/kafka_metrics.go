package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/segmentio/kafka-go"
)

const (
	namespace = "kafka"
	subsystem = "writer"
)

type KafkaMetrics struct {
	ID     string
	Writer *kafka.Writer
}

func RegisterKafkaMetrics(id string, w *kafka.Writer) error {
	m := NewKafkaMetricsFromWriter(id, w)
	return prometheus.Register(m)
}

func NewKafkaMetricsFromWriter(id string, w *kafka.Writer) *KafkaMetrics {
	return &KafkaMetrics{
		id, w,
	}
}

func (k *KafkaMetrics) Describe(chan<- *prometheus.Desc) {
	return
}

func (k *KafkaMetrics) Collect(c chan<- prometheus.Metric) {
	s := k.Writer.Stats()
	labels := prometheus.Labels{
		"id":        k.ID,
		"client_id": s.ClientID,
		"topic":     s.Topic,
	}

	g := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "dials_total",
		Help:        "total number of dials to kafka brokers",
		ConstLabels: labels,
	})
	g.Add(float64(s.Dials))
	c <- g

	g = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "writes_total",
		Help:        "total number of writes to kafka brokers",
		ConstLabels: labels,
	})
	g.Add(float64(s.Writes))
	c <- g

	g = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "messages_total",
		Help:        "total number of messages to kafka brokers",
		ConstLabels: labels,
	})
	g.Add(float64(s.Messages))
	c <- g

	g = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "bytes_total",
		Help:        "total number of bytes written to kafka brokers",
		ConstLabels: labels,
	})
	g.Add(float64(s.Bytes))
	c <- g

	g = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "rebalance_total",
		Help:        "total number of rebalance operations",
		ConstLabels: labels,
	})
	g.Add(float64(s.Rebalances))
	c <- g

	g = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "errors_total",
		Help:        "total number of errors encountered",
		ConstLabels: labels,
	})
	g.Add(float64(s.Errors))
	c <- g

	sums := map[string]kafka.SummaryStats{
		"batch_bytes": s.BatchBytes,
		"batch_size":  s.BatchSize,
	}

	for name, s := range sums {
		types := map[string]int64{
			"min": s.Min,
			"avg": s.Avg,
			"max": s.Max,
		}
		for suffix, value := range types {
			m := prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace:   namespace,
				Subsystem:   subsystem,
				Name:        fmt.Sprintf("%s_%s", name, suffix),
				Help:        fmt.Sprintf("%s value for %s", suffix, name),
				ConstLabels: labels,
			})
			m.Set(float64(value))
			c <- m
		}
	}
}
