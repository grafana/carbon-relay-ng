package route

import (
	"context"
	"fmt"

	"github.com/graphite-ng/carbon-relay-ng/encoding"
	"github.com/graphite-ng/carbon-relay-ng/matcher"
	"github.com/graphite-ng/carbon-relay-ng/metrics"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Kafka struct {
	baseRoute
	router *RoutingMutator
	Writer *kafka.Writer
	ctx    context.Context
}

func NewKafkaRoute(key, prefix, sub, regex string, config kafka.WriterConfig, routingMutator *RoutingMutator) (*Kafka, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %s", err)
	}
	k := Kafka{
		baseRoute: *newBaseRoute(key, "kafka"),
		router:    routingMutator,
		Writer:    kafka.NewWriter(config),
		ctx:       context.TODO(),
	}
	if err := metrics.RegisterKafkaMetrics(key, k.Writer); err != nil {
		return nil, fmt.Errorf("can't register kafka metrics: %s", err)
	}
	k.rm = metrics.NewRouteMetrics(key, "kafka", nil)
	k.logger = k.logger.With(zap.String("kafka_topic", config.Topic))

	// Don't remember why it's required
	m, err := matcher.New(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	k.config.Store(baseConfig{*m, nil})

	return &k, nil
}

func (k *Kafka) Shutdown() error {
	k.logger.Info("shutting down kafka writer")
	return k.Writer.Close()
}

func (k *Kafka) Dispatch(dp encoding.Datapoint) {
	k.rm.InMetrics.Inc()
	key := []byte(dp.Name)
	if newKey, ok := k.router.HandleBuf(key); ok {
		key = newKey
	}

	err := k.Writer.WriteMessages(k.ctx, kafka.Message{Key: key, Value: []byte(dp.String()), Headers: getKafkaHeader(dp.Tags)})
	if err != nil {
		k.logger.Error("error writing to kafka", zap.Error(err))
		k.rm.Errors.WithLabelValues(err.Error())
	} else {
		k.rm.OutMetrics.Inc()
	}
}
func getKafkaHeader(tags map[string]string) []kafka.Header {
	headers := make([]kafka.Header, len(tags))
	i := 0
	for key, value := range tags {
		header := kafka.Header{Key: key, Value: []byte(value)}
		headers[i] = header
		i++
	}
	return headers
}

func (k *Kafka) Snapshot() Snapshot {
	return makeSnapshot(&k.baseRoute)
}
